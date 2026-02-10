package monitor

import (
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/liaoweijun/agent-team-monitor/pkg/parser"
	"github.com/liaoweijun/agent-team-monitor/pkg/types"
)

// Collector collects and aggregates monitoring data
type Collector struct {
	processMonitor *ProcessMonitor
	fsMonitor      *FileSystemMonitor
	state          *types.MonitorState
	stateMutex     sync.RWMutex
	updateChan     chan struct{}
}

// NewCollector creates a new data collector
func NewCollector() (*Collector, error) {
	c := &Collector{
		processMonitor: NewProcessMonitor(),
		state: &types.MonitorState{
			Teams:     []types.TeamInfo{},
			Processes: []types.ProcessInfo{},
			UpdatedAt: time.Now(),
		},
		updateChan: make(chan struct{}, 1),
	}

	// Create filesystem monitor with callback
	fsMonitor, err := NewFileSystemMonitor(func(event fsnotify.Event) {
		// Trigger state update on filesystem changes
		select {
		case c.updateChan <- struct{}{}:
		default:
		}
	})
	if err != nil {
		return nil, err
	}
	c.fsMonitor = fsMonitor

	return c, nil
}

// Start begins collecting data
func (c *Collector) Start() error {
	// Start filesystem monitoring
	if err := c.fsMonitor.Start(); err != nil {
		return err
	}

	// Initial data collection
	c.updateState()

	// Start periodic updates
	go c.periodicUpdate()

	// Start event-driven updates
	go c.eventDrivenUpdate()

	return nil
}

// periodicUpdate updates state periodically
func (c *Collector) periodicUpdate() {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		c.updateState()
	}
}

// eventDrivenUpdate updates state on filesystem events
func (c *Collector) eventDrivenUpdate() {
	for range c.updateChan {
		time.Sleep(100 * time.Millisecond) // Debounce
		c.updateState()
	}
}

// updateState collects and updates the current state
func (c *Collector) updateState() {
	c.stateMutex.Lock()
	defer c.stateMutex.Unlock()

	// Collect process information
	processes, err := c.processMonitor.FindClaudeProcesses()
	if err != nil {
		log.Printf("Error finding Claude processes: %v", err)
		processes = []types.ProcessInfo{}
	}

	// Collect team information
	homeDir, _ := os.UserHomeDir()
	teamsDir := filepath.Join(homeDir, ".claude", "teams")
	tasksDir := filepath.Join(homeDir, ".claude", "tasks")

	teams, err := parser.ScanTeams(teamsDir)
	if err != nil {
		log.Printf("Error scanning teams: %v", err)
		teams = []types.TeamInfo{}
	}

	// Load tasks for each team
	for i := range teams {
		tasks, err := parser.ScanTasks(tasksDir, teams[i].Name)
		if err != nil {
			log.Printf("Error scanning tasks for team %s: %v", teams[i].Name, err)
			continue
		}
		teams[i].Tasks = tasks

		// Update agent status based on task ownership
		c.updateAgentStatus(&teams[i])
	}

	// Update state
	c.state.Teams = teams
	c.state.Processes = processes
	c.state.UpdatedAt = time.Now()
}

// updateAgentStatus updates agent status based on their tasks
func (c *Collector) updateAgentStatus(team *types.TeamInfo) {
	// Create a map of agent names to their current tasks
	agentTasks := make(map[string]*types.TaskInfo)

	for i := range team.Tasks {
		task := &team.Tasks[i]
		if task.Owner != "" && task.Status == "in_progress" {
			agentTasks[task.Owner] = task
		}
	}

	// Update agent status
	for i := range team.Members {
		agent := &team.Members[i]
		if task, ok := agentTasks[agent.Name]; ok {
			agent.Status = "working"
			agent.CurrentTask = task.ID
			agent.LastActivity = task.UpdatedAt
		} else {
			// Check if agent has completed tasks
			hasCompletedTasks := false
			for _, task := range team.Tasks {
				if task.Owner == agent.Name && task.Status == "completed" {
					hasCompletedTasks = true
					if task.UpdatedAt.After(agent.LastActivity) {
						agent.LastActivity = task.UpdatedAt
					}
				}
			}

			if hasCompletedTasks {
				agent.Status = "idle"
			} else {
				agent.Status = "idle"
			}
		}
	}
}

// GetState returns the current monitoring state
func (c *Collector) GetState() types.MonitorState {
	c.stateMutex.RLock()
	defer c.stateMutex.RUnlock()

	// Return a copy of the state
	return *c.state
}

// Stop stops the collector
func (c *Collector) Stop() error {
	close(c.updateChan)
	return c.fsMonitor.Stop()
}

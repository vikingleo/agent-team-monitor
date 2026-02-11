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
	projectsDir := filepath.Join(homeDir, ".claude", "projects")

	teams, err := parser.ScanTeams(teamsDir)
	if err != nil {
		log.Printf("Error scanning teams: %v", err)
		teams = []types.TeamInfo{}
	}

	// Load tasks for each team
	for i := range teams {
		// Load visible tasks (excluding internal tasks)
		tasks, err := parser.ScanTasks(tasksDir, teams[i].Name)
		if err != nil {
			log.Printf("Error scanning tasks for team %s: %v", teams[i].Name, err)
			continue
		}
		teams[i].Tasks = tasks

		// Load all tasks (including internal) for status calculation
		allTasks, err := parser.ScanAllTasks(tasksDir, teams[i].Name)
		if err != nil {
			log.Printf("Error scanning all tasks for team %s: %v", teams[i].Name, err)
			allTasks = tasks // Fallback to visible tasks
		}

		// Load inbox messages for each agent
		c.loadAgentInboxes(&teams[i], teamsDir)

		// Load agent activities from jsonl logs
		c.loadAgentActivities(&teams[i], projectsDir)

		// Update agent status based on all tasks (including internal)
		c.updateAgentStatus(&teams[i], allTasks)
	}

	// Update state
	c.state.Teams = teams
	c.state.Processes = processes
	c.state.UpdatedAt = time.Now()
}

// loadAgentInboxes loads inbox messages for all agents in a team
func (c *Collector) loadAgentInboxes(team *types.TeamInfo, teamsDir string) {
	for i := range team.Members {
		agent := &team.Members[i]
		message, err := parser.ParseInbox(teamsDir, team.Name, agent.Name)
		if err != nil {
			log.Printf("Error parsing inbox for %s: %v", agent.Name, err)
			continue
		}
		if message != nil {
			agent.LatestMessage = message.Text
			agent.MessageSummary = message.Summary
			agent.LastMessageTime = message.Timestamp
		}
	}
}

// loadAgentActivities loads recent activities from agent jsonl logs
func (c *Collector) loadAgentActivities(team *types.TeamInfo, projectsDir string) {
	for i := range team.Members {
		agent := &team.Members[i]

		// Skip if no working directory
		if agent.Cwd == "" {
			continue
		}

		// Find agent's log file by matching working directory
		logPath, agentID, err := parser.FindAgentLogFileByCwd(projectsDir, agent.Cwd)
		if err != nil || logPath == "" || agentID == "" {
			continue
		}

		// Parse agent activity
		activity, err := parser.ParseAgentActivity(logPath)
		if err != nil {
			log.Printf("Error parsing activity for %s: %v", agent.Name, err)
			continue
		}

		if activity != nil {
			agent.LastThinking = activity.LastThinking
			agent.LastToolUse = activity.LastToolUse
			agent.LastToolDetail = activity.LastToolDetail
			agent.LastActiveTime = activity.LastActiveTime
		}
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// updateAgentStatus updates agent status based on their tasks
func (c *Collector) updateAgentStatus(team *types.TeamInfo, allTasks []types.TaskInfo) {
	// Create a map of agent names to their current tasks
	agentTasks := make(map[string]*types.TaskInfo)

	for i := range allTasks {
		task := &allTasks[i]
		if task.Status == "in_progress" {
			// Match by owner field
			if task.Owner != "" {
				agentTasks[task.Owner] = task
			} else if task.Subject != "" {
				// Also try to match by subject (for internal tasks)
				// Check if subject matches any agent name
				for _, agent := range team.Members {
					if agent.Name == task.Subject {
						agentTasks[agent.Name] = task
						break
					}
				}
			}
		}
	}

	// Update agent status
	for i := range team.Members {
		agent := &team.Members[i]
		if task, ok := agentTasks[agent.Name]; ok {
			agent.Status = "working"
			agent.CurrentTask = task.Subject
			agent.LastActivity = task.UpdatedAt
		} else {
			// Check if agent has completed tasks
			hasCompletedTasks := false
			for _, task := range allTasks {
				ownerMatch := task.Owner == agent.Name
				subjectMatch := task.Subject == agent.Name

				if (ownerMatch || subjectMatch) && task.Status == "completed" {
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

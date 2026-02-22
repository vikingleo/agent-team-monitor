package monitor

import (
	"log"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/liaoweijun/agent-team-monitor/pkg/narrative"
	"github.com/liaoweijun/agent-team-monitor/pkg/parser"
	"github.com/liaoweijun/agent-team-monitor/pkg/types"
)

var sessionIDPattern = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)
var windowsAbsPathPattern = regexp.MustCompile(`^[A-Za-z]:[\\/]`)
var exposeAbsolutePaths = readExposeAbsolutePaths()

// Collector collects and aggregates monitoring data
type Collector struct {
	processMonitor *ProcessMonitor
	fsMonitor      *FileSystemMonitor
	state          *types.MonitorState
	stateMutex     sync.RWMutex
	updateChan     chan struct{}
	stopOnce       sync.Once
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
	teams = mergeTaskOnlyTeams(teams, tasksDir)

	// Load tasks for each team
	for i := range teams {
		// Load visible tasks (excluding internal tasks)
		tasks, err := parser.ScanTasks(tasksDir, teams[i].Name)
		if err != nil {
			log.Printf("Error scanning tasks for team %s: %v", teams[i].Name, err)
			continue
		}
		teams[i].Tasks = tasks
		c.updateVirtualTeamTimestamp(&teams[i], tasks)
		c.populateTeamProjectCwd(&teams[i], projectsDir)

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

		// Build shared office narrative fields for TUI/Web
		c.buildAgentNarratives(&teams[i])
	}

	// Update state
	c.state.Teams = filterStaleTeams(teams, 30*time.Minute, tasksDir)
	c.state.Processes = processes
	c.state.UpdatedAt = time.Now()
}

// mergeTaskOnlyTeams creates virtual teams for task directories that have no team config.
func mergeTaskOnlyTeams(teams []types.TeamInfo, tasksDir string) []types.TeamInfo {
	existing := make(map[string]struct{}, len(teams))
	for _, team := range teams {
		existing[team.Name] = struct{}{}
	}

	entries, err := os.ReadDir(tasksDir)
	if err != nil {
		if !os.IsNotExist(err) {
			log.Printf("Error scanning task directories for virtual teams: %v", err)
		}
		return teams
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		teamName := entry.Name()
		if _, ok := existing[teamName]; ok {
			continue
		}

		createdAt := time.Now()
		if info, err := entry.Info(); err == nil {
			createdAt = info.ModTime()
		}

		leadSessionID := ""
		if sessionIDPattern.MatchString(teamName) {
			leadSessionID = teamName
		}

		teams = append(teams, types.TeamInfo{
			Name:          teamName,
			CreatedAt:     createdAt,
			LeadSessionID: leadSessionID,
			Members:       []types.AgentInfo{},
			Tasks:         []types.TaskInfo{},
		})
	}

	return teams
}

func (c *Collector) updateVirtualTeamTimestamp(team *types.TeamInfo, tasks []types.TaskInfo) {
	if len(team.Members) > 0 || len(tasks) == 0 {
		return
	}

	latest := team.CreatedAt
	for _, task := range tasks {
		if task.UpdatedAt.After(latest) {
			latest = task.UpdatedAt
		}
		if task.CreatedAt.After(latest) {
			latest = task.CreatedAt
		}
	}

	if latest.After(team.CreatedAt) {
		team.CreatedAt = latest
	}
}

func (c *Collector) populateTeamProjectCwd(team *types.TeamInfo, projectsDir string) {
	if team.ProjectCwd != "" {
		return
	}

	// Prefer team-lead cwd from config.
	for _, agent := range team.Members {
		if agent.Name == "team-lead" && agent.Cwd != "" {
			team.ProjectCwd = agent.Cwd
			return
		}
	}

	// Fallback to first available member cwd.
	for _, agent := range team.Members {
		if agent.Cwd != "" {
			team.ProjectCwd = agent.Cwd
			return
		}
	}

	// For task-only teams, recover cwd from lead session log.
	if team.LeadSessionID == "" {
		return
	}

	cwd, err := parser.FindSessionCwd(projectsDir, team.LeadSessionID)
	if err != nil {
		log.Printf("Error resolving project cwd for team %s (session %s): %v", team.Name, team.LeadSessionID, err)
		return
	}
	if cwd != "" {
		team.ProjectCwd = cwd
	}
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
	homeDir, _ := os.UserHomeDir()
	todosDir := filepath.Join(homeDir, ".claude", "todos")
	leadLogPath, _ := parser.FindLeadSessionLogFile(projectsDir, team.LeadSessionID)

	for i := range team.Members {
		agent := &team.Members[i]

		// Find agent's log file by member identity first, then cwd fallback
		logPath, agentID, sessionID, err := parser.FindAgentLogFileForMember(projectsDir, team.LeadSessionID, agent.Name, agent.Cwd, agent.JoinedAt)
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

		// Load TodoWrite items for this agent
		if sessionID != "" {
			todos, err := parser.LoadTodosForSession(todosDir, sessionID)
			if err != nil {
				log.Printf("Error loading todos for %s (session %s): %v", agent.Name, sessionID, err)
			}
			if len(todos) == 0 {
				// Fallback: extract from JSONL log
				todos, _ = parser.ExtractTodosFromLog(logPath)
			}
			if len(todos) > 0 {
				agent.Todos = todos
			}
		} else {
			// No session ID, try extracting directly from log
			todos, _ := parser.ExtractTodosFromLog(logPath)
			if len(todos) > 0 {
				agent.Todos = todos
			}
		}

		// For team-lead, prefer lead session root log when more recent.
		if agent.Name == "team-lead" && leadLogPath != "" {
			leadActivity, err := parser.ParseAgentActivity(leadLogPath)
			if err == nil && leadActivity != nil {
				if activity == nil || leadActivity.LastActiveTime.After(activity.LastActiveTime) {
					agent.LastThinking = leadActivity.LastThinking
					agent.LastToolUse = leadActivity.LastToolUse
					agent.LastToolDetail = leadActivity.LastToolDetail
					agent.LastActiveTime = leadActivity.LastActiveTime
				}
			}
			// Also try loading todos from lead session
			if len(agent.Todos) == 0 && team.LeadSessionID != "" {
				todos, _ := parser.LoadTodosForSession(todosDir, team.LeadSessionID)
				if len(todos) == 0 && leadLogPath != "" {
					todos, _ = parser.ExtractTodosFromLog(leadLogPath)
				}
				if len(todos) > 0 {
					agent.Todos = todos
				}
			}
		}
	}
}

// buildAgentNarratives populates shared fields used by both TUI and Web UI
func (c *Collector) buildAgentNarratives(team *types.TeamInfo) {
	tasksByOwner, _ := narrative.GroupTasksByOwner(team.Members, team.Tasks)
	now := time.Now()

	for i := range team.Members {
		agent := &team.Members[i]
		agent.RoleEmoji = narrative.RoleEmoji(agent.Name)
		agent.OfficeDialogues = narrative.BuildAgentDialogues(*agent, tasksByOwner[agent.Name], now)
	}
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
				agent.Status = "unknown"
			}
		}
	}
}

// GetState returns the current monitoring state
func (c *Collector) GetState() types.MonitorState {
	c.stateMutex.RLock()
	defer c.stateMutex.RUnlock()

	stateCopy := types.MonitorState{
		UpdatedAt: c.state.UpdatedAt,
		Processes: append([]types.ProcessInfo(nil), c.state.Processes...),
		Teams:     make([]types.TeamInfo, len(c.state.Teams)),
	}

	for i, team := range c.state.Teams {
		teamCopy := team
		teamCopy.Tasks = append([]types.TaskInfo(nil), team.Tasks...)
		teamCopy.Members = append([]types.AgentInfo(nil), team.Members...)

		for j, member := range teamCopy.Members {
			member.OfficeDialogues = append([]string(nil), member.OfficeDialogues...)
			member.Todos = append([]types.TodoItem(nil), member.Todos...)

			if !exposeAbsolutePaths {
				member.Cwd = sanitizeDisplayPath(member.Cwd)
			}
			teamCopy.Members[j] = member
		}

		if !exposeAbsolutePaths {
			teamCopy.ProjectCwd = sanitizeDisplayPath(teamCopy.ProjectCwd)
		}

		stateCopy.Teams[i] = teamCopy
	}

	return stateCopy
}

func readExposeAbsolutePaths() bool {
	raw := strings.TrimSpace(os.Getenv("ATM_EXPOSE_ABS_PATHS"))
	if raw == "" {
		return false
	}

	enabled, err := strconv.ParseBool(raw)
	if err == nil {
		return enabled
	}

	switch strings.ToLower(raw) {
	case "on", "yes", "y":
		return true
	default:
		return false
	}
}

func sanitizeDisplayPath(raw string) string {
	path := strings.TrimSpace(raw)
	if path == "" {
		return ""
	}

	cleaned := filepath.Clean(path)

	homeDir, err := os.UserHomeDir()
	if err == nil && homeDir != "" {
		homeDir = filepath.Clean(homeDir)
		if cleaned == homeDir {
			return "~"
		}
		homePrefix := homeDir + string(os.PathSeparator)
		if strings.HasPrefix(cleaned, homePrefix) {
			return "~" + cleaned[len(homeDir):]
		}
	}

	// For non-home absolute paths, keep only project/folder name.
	if filepath.IsAbs(cleaned) {
		return filepath.Base(cleaned)
	}
	if windowsAbsPathPattern.MatchString(cleaned) {
		replaced := strings.ReplaceAll(cleaned, "\\", "/")
		replaced = strings.TrimSuffix(replaced, "/")
		parts := strings.Split(replaced, "/")
		if len(parts) > 0 {
			return parts[len(parts)-1]
		}
	}

	return cleaned
}

// DeleteTeam removes a team's config and task directories.
func (c *Collector) DeleteTeam(teamName string) error {
	homeDir, _ := os.UserHomeDir()
	teamsDir := filepath.Join(homeDir, ".claude", "teams", teamName)
	tasksDir := filepath.Join(homeDir, ".claude", "tasks", teamName)

	// Remove team config directory
	if err := os.RemoveAll(teamsDir); err != nil && !os.IsNotExist(err) {
		return err
	}
	// Remove task directory
	if err := os.RemoveAll(tasksDir); err != nil && !os.IsNotExist(err) {
		return err
	}

	log.Printf("Deleted team %q (removed teams and tasks directories)", teamName)

	// Trigger state refresh
	select {
	case c.updateChan <- struct{}{}:
	default:
	}

	return nil
}

// Stop stops the collector
func (c *Collector) Stop() error {
	var err error
	c.stopOnce.Do(func() {
		close(c.updateChan)
		err = c.fsMonitor.Stop()
	})
	return err
}

// filterStaleTeams removes teams where all members have been inactive
// longer than the given threshold.
func filterStaleTeams(teams []types.TeamInfo, threshold time.Duration, tasksDir string) []types.TeamInfo {
	now := time.Now()
	result := make([]types.TeamInfo, 0, len(teams))

	for _, team := range teams {
		if isTeamActive(team, now, threshold) {
			result = append(result, team)
			continue
		}
		// Virtual team (no config file): remove orphaned task directory.
		if team.ConfigPath == "" {
			dir := filepath.Join(tasksDir, team.Name)
			if err := os.RemoveAll(dir); err != nil {
				log.Printf("Failed to remove orphaned task dir %s: %v", dir, err)
			} else {
				log.Printf("Removed orphaned task dir for stale virtual team %q", team.Name)
			}
		}
	}

	return result
}

// isTeamActive returns true if any member in the team has recent activity.
func isTeamActive(team types.TeamInfo, now time.Time, threshold time.Duration) bool {
	if len(team.Members) == 0 {
		// No members — check if team was created recently
		return now.Sub(team.CreatedAt) < threshold
	}

	for _, agent := range team.Members {
		// Check all available timestamps for recent activity
		timestamps := []time.Time{
			agent.LastActiveTime,
			agent.LastMessageTime,
			agent.LastActivity,
			agent.JoinedAt,
		}
		for _, ts := range timestamps {
			if !ts.IsZero() && now.Sub(ts) < threshold {
				return true
			}
		}
	}

	return false
}

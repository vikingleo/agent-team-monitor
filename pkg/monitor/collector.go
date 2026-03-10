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
var teamTokenPattern = regexp.MustCompile(`[a-z0-9]+`)
var teamTokenStopWords = map[string]struct{}{
	"team":      {},
	"dev":       {},
	"agent":     {},
	"project":   {},
	"projects":  {},
	"workspace": {},
	"work":      {},
	"works":     {},
	"home":      {},
	"users":     {},
	"user":      {},
	"code":      {},
}
var exposeAbsolutePaths = readBoolEnv("ATM_EXPOSE_ABS_PATHS", false)
var discoveryMetricsLoggingEnabled = readBoolEnv("ATM_DISCOVERY_METRICS", false)

const projectDiscoveryMaxAge = 2 * time.Hour
const codexSessionDiscoveryMaxAge = 8 * time.Hour
const codexWorkingRecentThreshold = 2 * time.Minute
const discoveryMetricsLogInterval = 30 * time.Second
const discoveryMetricsSlowThreshold = 500 * time.Millisecond

// CollectorOptions controls collector behavior.
type CollectorOptions struct {
	Provider ProviderMode
}

// Collector collects and aggregates monitoring data
type Collector struct {
	processMonitor          *ProcessMonitor
	fsMonitor               *FileSystemMonitor
	provider                ProviderMode
	state                   *types.MonitorState
	stateMutex              sync.RWMutex
	updateChan              chan struct{}
	stopOnce                sync.Once
	lastDiscoveryMetrics    parser.DiscoveryMetrics
	lastDiscoveryMetricsLog time.Time
}

// NewCollector creates a new data collector
func NewCollector() (*Collector, error) {
	return NewCollectorWithOptions(CollectorOptions{})
}

// NewCollectorWithOptions creates a collector with explicit options.
func NewCollectorWithOptions(options CollectorOptions) (*Collector, error) {
	provider := normalizeProviderMode(options.Provider)

	c := &Collector{
		processMonitor: NewProcessMonitor(),
		provider:       provider,
		state: &types.MonitorState{
			Teams:     []types.TeamInfo{},
			Processes: []types.ProcessInfo{},
			UpdatedAt: time.Now(),
		},
		updateChan: make(chan struct{}, 1),
	}

	// Create filesystem monitor with callback
	fsMonitor, err := NewFileSystemMonitor(FileSystemMonitorOptions{
		Provider: provider,
	}, func(event fsnotify.Event) {
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
	processes, err := c.processMonitor.FindProcesses(c.provider)
	if err != nil {
		log.Printf("Error finding monitored processes: %v", err)
		processes = []types.ProcessInfo{}
	}

	homeDir, _ := os.UserHomeDir()
	allTeams := make([]types.TeamInfo, 0)

	if c.provider.IncludesClaude() {
		claudeTeams := c.collectClaudeTeams(homeDir)
		allTeams = append(allTeams, claudeTeams...)
	}
	if c.provider.IncludesCodex() {
		codexTeams := c.collectCodexTeams(homeDir)
		allTeams = append(allTeams, codexTeams...)
	}

	// Update state
	c.state.Teams = allTeams
	c.state.Processes = processes
	c.state.UpdatedAt = time.Now()
}

func (c *Collector) collectClaudeTeams(homeDir string) []types.TeamInfo {
	teamsDir := filepath.Join(homeDir, ".claude", "teams")
	tasksDir := filepath.Join(homeDir, ".claude", "tasks")
	projectsDir := filepath.Join(homeDir, ".claude", "projects")

	teams, err := parser.ScanTeams(teamsDir)
	if err != nil {
		log.Printf("Error scanning teams: %v", err)
		teams = []types.TeamInfo{}
	}
	teams = mergeTaskOnlyTeams(teams, tasksDir)
	teams = mergeInboxOnlyTeams(teams, teamsDir)

	discoveryStart := time.Now()
	discoveredTeams, err := parser.DiscoverProjectTeams(projectsDir, projectDiscoveryMaxAge)
	discoveryElapsed := time.Since(discoveryStart)
	if err != nil {
		log.Printf("Error discovering teams from projects: %v", err)
	} else {
		teams = mergeDiscoveredProjectTeams(teams, discoveredTeams)
		c.logDiscoveryMetrics(discoveryElapsed, discoveredTeams)
	}

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
		markTeamProvider(&teams[i], "claude")
	}

	return filterStaleTeams(teams, time.Hour, tasksDir)
}

func (c *Collector) collectCodexTeams(homeDir string) []types.TeamInfo {
	sessionsDir := filepath.Join(homeDir, ".codex", "sessions")
	discovered, err := parser.DiscoverCodexSessions(sessionsDir, codexSessionDiscoveryMaxAge)
	if err != nil {
		log.Printf("Error discovering codex sessions: %v", err)
		return []types.TeamInfo{}
	}

	now := time.Now()
	teams := make([]types.TeamInfo, 0, len(discovered))
	for _, session := range discovered {
		lastActive := session.LastActiveAt
		if lastActive.IsZero() {
			lastActive = session.StartedAt
		}

		createdAt := session.StartedAt
		if createdAt.IsZero() {
			createdAt = lastActive
		}
		if createdAt.IsZero() {
			createdAt = now
		}

		status := "idle"
		if !lastActive.IsZero() && now.Sub(lastActive) <= codexWorkingRecentThreshold {
			status = "working"
		}

		messageSummary := session.LastAgentMessage
		latestMessage := session.LastAgentMessage
		if messageSummary == "" {
			messageSummary = session.LastUserMessage
		}
		if latestMessage == "" {
			latestMessage = session.LastUserMessage
		}

		agent := types.AgentInfo{
			Name:            "codex-agent",
			Provider:        "codex",
			AgentID:         session.SessionID,
			AgentType:       "codex",
			Status:          status,
			CurrentTask:     session.LastUserMessage,
			LastActivity:    lastActive,
			Cwd:             session.Cwd,
			LatestMessage:   latestMessage,
			MessageSummary:  messageSummary,
			LastMessageTime: lastActive,
			LastThinking:    session.LastReasoning,
			LastToolUse:     session.LastToolUse,
			LastToolDetail:  session.LastToolDetail,
			LastActiveTime:  lastActive,
		}

		team := types.TeamInfo{
			Name:          codexTeamName(session.SessionID, session.Cwd),
			Provider:      "codex",
			CreatedAt:     createdAt,
			LeadSessionID: session.SessionID,
			ProjectCwd:    session.Cwd,
			Members:       []types.AgentInfo{agent},
			Tasks:         []types.TaskInfo{},
		}
		c.buildAgentNarratives(&team)
		teams = append(teams, team)
	}

	return teams
}

func markTeamProvider(team *types.TeamInfo, provider string) {
	team.Provider = provider
	for i := range team.Members {
		if team.Members[i].Provider == "" {
			team.Members[i].Provider = provider
		}
	}
}

func codexTeamName(sessionID, cwd string) string {
	shortID := strings.TrimSpace(sessionID)
	if len(shortID) > 8 {
		shortID = shortID[:8]
	}
	if shortID == "" {
		shortID = "unknown"
	}

	cwdBase := strings.TrimSpace(filepath.Base(cwd))
	if cwdBase == "" || cwdBase == "." || cwdBase == string(filepath.Separator) {
		return "codex-" + shortID
	}
	cwdBase = strings.ReplaceAll(cwdBase, " ", "-")
	return "codex-" + cwdBase + "-" + shortID
}

func (c *Collector) logDiscoveryMetrics(elapsed time.Duration, discovered []parser.ProjectTeamDiscovery) {
	if !discoveryMetricsLoggingEnabled {
		return
	}

	snapshot := parser.SnapshotDiscoveryMetrics()
	delta := snapshot.Delta(c.lastDiscoveryMetrics)
	c.lastDiscoveryMetrics = snapshot

	now := time.Now()
	if !c.lastDiscoveryMetricsLog.IsZero() &&
		now.Sub(c.lastDiscoveryMetricsLog) < discoveryMetricsLogInterval &&
		elapsed < discoveryMetricsSlowThreshold {
		return
	}
	c.lastDiscoveryMetricsLog = now

	memberCount := 0
	for _, team := range discovered {
		memberCount += len(team.Members)
	}

	rootReq := delta.RootCacheHits + delta.RootCacheMisses
	subReq := delta.SubCacheHits + delta.SubCacheMisses

	log.Printf(
		"Project discovery metrics: elapsed=%s teams=%d members=%d runs=+%d root-cache=%d/%d(%.1f%%) sub-cache=%d/%d(%.1f%%)",
		elapsed.Round(time.Millisecond),
		len(discovered),
		memberCount,
		delta.Runs,
		delta.RootCacheHits,
		rootReq,
		discoveryHitRate(delta.RootCacheHits, rootReq),
		delta.SubCacheHits,
		subReq,
		discoveryHitRate(delta.SubCacheHits, subReq),
	)
}

func discoveryHitRate(hits, total int64) float64 {
	if total <= 0 {
		return 0
	}
	return float64(hits) * 100 / float64(total)
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

// mergeInboxOnlyTeams creates virtual teams for ~/.claude/teams/<team>/inboxes-only directories.
func mergeInboxOnlyTeams(teams []types.TeamInfo, teamsDir string) []types.TeamInfo {
	existing := make(map[string]struct{}, len(teams))
	for _, team := range teams {
		existing[team.Name] = struct{}{}
	}

	entries, err := os.ReadDir(teamsDir)
	if err != nil {
		if !os.IsNotExist(err) {
			log.Printf("Error scanning team directories for inbox-only teams: %v", err)
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

		configPath := filepath.Join(teamsDir, teamName, "config.json")
		if _, err := os.Stat(configPath); err == nil {
			continue
		}

		inboxesDir := filepath.Join(teamsDir, teamName, "inboxes")
		inboxEntries, err := os.ReadDir(inboxesDir)
		if err != nil || len(inboxEntries) == 0 {
			continue
		}

		createdAt := latestInboxesModTime(inboxesDir)
		if createdAt.IsZero() {
			if info, err := entry.Info(); err == nil {
				createdAt = info.ModTime()
			} else {
				createdAt = time.Now()
			}
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
		existing[teamName] = struct{}{}
	}

	return teams
}

func latestInboxesModTime(inboxesDir string) time.Time {
	entries, err := os.ReadDir(inboxesDir)
	if err != nil {
		return time.Time{}
	}

	latest := time.Time{}
	for _, entry := range entries {
		info, err := entry.Info()
		if err != nil {
			continue
		}
		if latest.IsZero() || info.ModTime().After(latest) {
			latest = info.ModTime()
		}
	}
	return latest
}

func mergeDiscoveredProjectTeams(teams []types.TeamInfo, discovered []parser.ProjectTeamDiscovery) []types.TeamInfo {
	if len(discovered) == 0 {
		return teams
	}

	nameToIndex := make(map[string]int, len(teams))
	sessionToIndex := make(map[string]int, len(teams))
	cwdToIndex := make(map[string]int, len(teams))

	for i, team := range teams {
		nameToIndex[team.Name] = i
		if team.LeadSessionID != "" {
			sessionToIndex[team.LeadSessionID] = i
		}
		if team.ProjectCwd != "" {
			cwdToIndex[team.ProjectCwd] = i
		}
	}

	for _, found := range discovered {
		targetIdx := -1

		if found.TeamNameHint != "" {
			if idx, ok := nameToIndex[found.TeamNameHint]; ok {
				targetIdx = idx
			}
		}
		if targetIdx == -1 && found.LeadSessionID != "" {
			if idx, ok := sessionToIndex[found.LeadSessionID]; ok {
				targetIdx = idx
			}
		}
		if targetIdx == -1 && found.ProjectCwd != "" {
			if idx, ok := cwdToIndex[found.ProjectCwd]; ok {
				targetIdx = idx
			}
		}

		resolvedName := resolveDiscoveredTeamName(found, teams)
		if targetIdx == -1 && resolvedName != "" {
			if idx, ok := nameToIndex[resolvedName]; ok {
				targetIdx = idx
			}
		}

		if targetIdx == -1 {
			// Strict mode: skip low-confidence orphan sessions to avoid
			// showing one-off delegated subagents as standalone teams.
			if resolvedName == "" && !isHighConfidenceDiscoveredTeam(found) {
				continue
			}
			if resolvedName == "" {
				resolvedName = discoveredFallbackTeamName(found.LeadSessionID, found.ProjectCwd)
			}
			createdAt := found.LastActiveAt
			if createdAt.IsZero() {
				createdAt = time.Now()
			}

			teams = append(teams, types.TeamInfo{
				Name:          resolvedName,
				CreatedAt:     createdAt,
				LeadSessionID: found.LeadSessionID,
				ProjectCwd:    found.ProjectCwd,
				Members:       append([]types.AgentInfo(nil), found.Members...),
				Tasks:         []types.TaskInfo{},
			})
			targetIdx = len(teams) - 1
			nameToIndex[resolvedName] = targetIdx
		} else {
			mergeDiscoveredTeamFields(&teams[targetIdx], found)
		}

		if teams[targetIdx].LeadSessionID != "" {
			sessionToIndex[teams[targetIdx].LeadSessionID] = targetIdx
		}
		if teams[targetIdx].ProjectCwd != "" {
			cwdToIndex[teams[targetIdx].ProjectCwd] = targetIdx
		}
	}

	return teams
}

func resolveDiscoveredTeamName(found parser.ProjectTeamDiscovery, teams []types.TeamInfo) string {
	if found.TeamNameHint != "" {
		return found.TeamNameHint
	}

	candidates := make([]types.TeamInfo, 0)
	for _, team := range teams {
		// Only match unresolved virtual teams (typically inbox-only).
		if team.ConfigPath != "" || team.LeadSessionID != "" || len(team.Members) > 0 {
			continue
		}
		candidates = append(candidates, team)
	}

	if len(candidates) == 0 {
		return ""
	}

	bestName := ""
	bestScore := 0
	ambiguous := false

	for _, candidate := range candidates {
		score := scoreTeamNameMatch(candidate.Name, found)
		if score > bestScore {
			bestName = candidate.Name
			bestScore = score
			ambiguous = false
			continue
		}
		if score == bestScore && score > 0 {
			ambiguous = true
		}
	}

	if bestScore > 0 && !ambiguous {
		return bestName
	}
	return ""
}

func isHighConfidenceDiscoveredTeam(found parser.ProjectTeamDiscovery) bool {
	if found.TeamNameHint != "" {
		return true
	}

	namedMembers := 0
	for _, member := range found.Members {
		if isMeaningfulDiscoveredMember(member) {
			namedMembers++
		}
	}

	if namedMembers >= 2 {
		return true
	}
	if namedMembers >= 1 && len(found.Members) >= 3 {
		return true
	}

	return false
}

func isMeaningfulDiscoveredMember(member types.AgentInfo) bool {
	name := strings.ToLower(strings.TrimSpace(member.Name))
	if name == "" {
		return false
	}
	if strings.HasPrefix(name, "agent-") {
		id := strings.ToLower(strings.TrimSpace(member.AgentID))
		return id != "" && name != "agent-"+id
	}
	return true
}

func scoreTeamNameMatch(teamName string, found parser.ProjectTeamDiscovery) int {
	tokens := tokenizeName(teamName)
	if len(tokens) == 0 {
		return 0
	}

	corpus := strings.ToLower(found.ProjectCwd)
	for _, member := range found.Members {
		if member.Name != "" {
			corpus += " " + strings.ToLower(member.Name)
		}
	}

	score := 0
	for token := range tokens {
		if strings.Contains(corpus, token) {
			score++
		}
	}
	return score
}

func tokenizeName(raw string) map[string]struct{} {
	result := make(map[string]struct{})
	name := strings.ToLower(raw)
	matches := teamTokenPattern.FindAllString(name, -1)
	for _, token := range matches {
		if len(token) < 3 {
			continue
		}
		if _, skip := teamTokenStopWords[token]; skip {
			continue
		}
		result[token] = struct{}{}
	}
	return result
}

func discoveredFallbackTeamName(leadSessionID, projectCwd string) string {
	if leadSessionID != "" {
		if len(leadSessionID) >= 8 {
			return "session-" + leadSessionID[:8]
		}
		return "session-" + leadSessionID
	}

	base := strings.TrimSpace(filepath.Base(projectCwd))
	if base != "" && base != "." && base != string(filepath.Separator) {
		return "session-" + base
	}

	return "session-unknown"
}

func mergeDiscoveredTeamFields(team *types.TeamInfo, found parser.ProjectTeamDiscovery) {
	if team.LeadSessionID == "" {
		team.LeadSessionID = found.LeadSessionID
	}
	if team.ProjectCwd == "" {
		team.ProjectCwd = found.ProjectCwd
	}

	if team.ConfigPath == "" && !found.LastActiveAt.IsZero() {
		if team.CreatedAt.IsZero() || found.LastActiveAt.After(team.CreatedAt) {
			team.CreatedAt = found.LastActiveAt
		}
	}

	team.Members = mergeDiscoveredMembers(team.Members, found.Members)
}

func mergeDiscoveredMembers(existing, discovered []types.AgentInfo) []types.AgentInfo {
	if len(discovered) == 0 {
		return existing
	}

	index := make(map[string]int, len(existing))
	for i, member := range existing {
		key := memberMergeKey(member)
		if key == "" {
			continue
		}
		index[key] = i
	}

	for _, member := range discovered {
		key := memberMergeKey(member)
		if key == "" {
			existing = append(existing, member)
			continue
		}
		if idx, ok := index[key]; ok {
			merged := existing[idx]
			if merged.Name == "" {
				merged.Name = member.Name
			}
			if merged.AgentID == "" {
				merged.AgentID = member.AgentID
			}
			if merged.AgentType == "" {
				merged.AgentType = member.AgentType
			}
			if merged.Cwd == "" {
				merged.Cwd = member.Cwd
			}
			if merged.JoinedAt.IsZero() {
				merged.JoinedAt = member.JoinedAt
			}
			if member.LastActiveTime.After(merged.LastActiveTime) {
				merged.LastActiveTime = member.LastActiveTime
				merged.LastActivity = member.LastActivity
			}
			existing[idx] = merged
			continue
		}

		index[key] = len(existing)
		existing = append(existing, member)
	}

	return existing
}

func memberMergeKey(member types.AgentInfo) string {
	name := strings.ToLower(strings.TrimSpace(member.Name))
	if name != "" && !strings.HasPrefix(name, "agent-") {
		return "name:" + name
	}

	if member.AgentID != "" {
		return "id:" + strings.ToLower(member.AgentID)
	}
	if name != "" {
		return "name:" + name
	}
	return ""
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

func readBoolEnv(key string, defaultValue bool) bool {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		return defaultValue
	}

	enabled, err := strconv.ParseBool(raw)
	if err == nil {
		return enabled
	}

	switch strings.ToLower(raw) {
	case "on", "yes", "y":
		return true
	case "off", "no", "n":
		return false
	default:
		return defaultValue
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
	lastActivity := latestTeamActivityTime(team)
	if lastActivity.IsZero() {
		return false
	}

	return now.Sub(lastActivity) < threshold
}

func latestTeamActivityTime(team types.TeamInfo) time.Time {
	var latest time.Time
	for _, agent := range team.Members {
		for _, ts := range []time.Time{agent.LastActiveTime, agent.LastMessageTime, agent.LastActivity} {
			if ts.IsZero() {
				continue
			}
			if latest.IsZero() || ts.After(latest) {
				latest = ts
			}
		}
	}

	return latest
}

package monitor

import (
	"log"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode"

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
const openClawSessionDiscoveryMaxAge = 8 * time.Hour
const openClawWorkingRecentThreshold = 2 * time.Minute
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
	if c.provider.IncludesOpenClaw() {
		openClawTeams := c.collectOpenClawTeams(homeDir)
		allTeams = append(allTeams, openClawTeams...)
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

	return c.buildCodexTeams(discovered, time.Now())
}

func (c *Collector) collectOpenClawTeams(homeDir string) []types.TeamInfo {
	agentsDir := filepath.Join(homeDir, ".openclaw", "agents")
	discovered, err := parser.DiscoverOpenClawSessions(agentsDir, openClawSessionDiscoveryMaxAge)
	if err != nil {
		log.Printf("Error discovering openclaw sessions: %v", err)
		discovered = nil
	}

	stateDir := filepath.Join(homeDir, ".openclaw")
	subagentRuns, runsErr := parser.DiscoverOpenClawSubagentRuns(stateDir, openClawSessionDiscoveryMaxAge)
	if runsErr != nil {
		log.Printf("Error discovering openclaw subagent runs: %v", runsErr)
	}

	if len(discovered) == 0 && len(subagentRuns) == 0 {
		return []types.TeamInfo{}
	}

	now := time.Now()
	team := types.TeamInfo{
		Name:      "openclaw",
		Provider:  "openclaw",
		CreatedAt: now,
		Members:   make([]types.AgentInfo, 0, len(discovered)),
		Tasks:     []types.TaskInfo{},
	}
	memberKeys := make(map[string]struct{}, len(discovered)+len(subagentRuns))

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

		if team.CreatedAt.IsZero() || createdAt.Before(team.CreatedAt) {
			team.CreatedAt = createdAt
		}

		if team.ProjectCwd == "" && session.Cwd != "" {
			team.ProjectCwd = session.Cwd
		}

		status := "idle"
		if !lastActive.IsZero() && now.Sub(lastActive) <= openClawWorkingRecentThreshold {
			status = "working"
		}

		currentTask := firstNonEmpty(
			session.LastUserMessage,
			session.Label,
			session.DisplayName,
		)
		latestMessage := firstNonEmpty(session.LastAgentMessage, session.LastUserMessage)
		messageSummary := firstNonEmpty(session.LastAgentMessage, session.LastUserMessage, session.Label)

		agentName := firstNonEmpty(session.DisplayName, session.Label, "openclaw-"+session.AgentID)
		agentType := firstNonEmpty(session.AgentID, "openclaw")
		memberKey := agentName + "\x00" + session.SessionID

		team.Members = append(team.Members, types.AgentInfo{
			Name:            agentName,
			Provider:        "openclaw",
			AgentID:         session.SessionID,
			AgentType:       agentType,
			Status:          status,
			CurrentTask:     currentTask,
			LastActivity:    lastActive,
			Cwd:             session.Cwd,
			LatestMessage:   latestMessage,
			MessageSummary:  messageSummary,
			LatestResponse:  session.FullAgentMessage,
			LastMessageTime: lastActive,
			LastThinking:    session.LastReasoning,
			LastToolUse:     session.LastToolUse,
			LastToolDetail:  session.LastToolDetail,
			LastActiveTime:  lastActive,
			RecentEvents:    convertOpenClawEvents(session.RecentEvents),
		})
		memberKeys[memberKey] = struct{}{}
	}

	for _, run := range subagentRuns {
		sessionAgentID, childToken := parseOpenClawChildSessionKey(run.ChildSessionKey)
		agentName := firstNonEmpty(run.Label, childToken, "openclaw-subagent")
		agentType := firstNonEmpty(sessionAgentID, "openclaw-subagent")
		agentID := firstNonEmpty(run.ChildSessionKey, run.RunID)
		memberKey := agentName + "\x00" + agentID
		if _, exists := memberKeys[memberKey]; exists {
			continue
		}

		lastActive := resolveOpenClawRunLastActive(run)
		createdAt := resolveOpenClawRunCreatedAt(run)
		if team.CreatedAt.IsZero() || (!createdAt.IsZero() && createdAt.Before(team.CreatedAt)) {
			team.CreatedAt = createdAt
		}
		if team.ProjectCwd == "" && run.WorkspaceDir != "" {
			team.ProjectCwd = run.WorkspaceDir
		}

		status := "idle"
		if !run.EndedAt.IsZero() {
			status = "completed"
		} else if !lastActive.IsZero() && now.Sub(lastActive) <= openClawWorkingRecentThreshold {
			status = "working"
		}

		currentTask := firstNonEmpty(run.Task, run.Label, childToken)
		messageSummary := firstNonEmpty(run.Label, run.Task)

		team.Members = append(team.Members, types.AgentInfo{
			Name:            agentName,
			Provider:        "openclaw",
			AgentID:         agentID,
			AgentType:       agentType,
			Status:          status,
			CurrentTask:     currentTask,
			LastActivity:    lastActive,
			Cwd:             run.WorkspaceDir,
			LatestMessage:   "",
			MessageSummary:  messageSummary,
			LatestResponse:  "",
			LastMessageTime: lastActive,
			LastThinking:    "",
			LastToolUse:     "",
			LastToolDetail:  "",
			LastActiveTime:  lastActive,
			RecentEvents:    nil,
		})
		memberKeys[memberKey] = struct{}{}
	}

	sort.SliceStable(team.Members, func(i, j int) bool {
		return team.Members[i].LastActiveTime.After(team.Members[j].LastActiveTime)
	})
	c.buildAgentNarratives(&team)
	return []types.TeamInfo{team}
}

func parseOpenClawChildSessionKey(sessionKey string) (agentID, childToken string) {
	parts := strings.Split(strings.TrimSpace(sessionKey), ":")
	if len(parts) < 4 {
		return "", strings.TrimSpace(sessionKey)
	}

	if len(parts) >= 2 && parts[0] == "agent" {
		agentID = strings.TrimSpace(parts[1])
	}

	lastToken := strings.TrimSpace(parts[len(parts)-1])
	if lastToken != "" {
		childToken = lastToken
	} else {
		childToken = strings.TrimSpace(sessionKey)
	}
	return agentID, childToken
}

func resolveOpenClawRunLastActive(run parser.OpenClawSubagentRunRecord) time.Time {
	if !run.EndedAt.IsZero() {
		return run.EndedAt
	}
	if !run.StartedAt.IsZero() {
		return run.StartedAt
	}
	return run.CreatedAt
}

func resolveOpenClawRunCreatedAt(run parser.OpenClawSubagentRunRecord) time.Time {
	if !run.CreatedAt.IsZero() {
		return run.CreatedAt
	}
	if !run.StartedAt.IsZero() {
		return run.StartedAt
	}
	return run.EndedAt
}

func markTeamProvider(team *types.TeamInfo, provider string) {
	team.Provider = provider
	for i := range team.Members {
		if team.Members[i].Provider == "" {
			team.Members[i].Provider = provider
		}
	}
}

type codexSessionEnvelope struct {
	session    parser.CodexSessionDiscovery
	agent      types.AgentInfo
	createdAt  time.Time
	lastActive time.Time
	cwdKey     string
	cwdDisplay string
	prefixKey  string
	teamLabel  string
}

type codexUnionFind struct {
	parent []int
	rank   []int
}

func newCodexUnionFind(size int) *codexUnionFind {
	parent := make([]int, size)
	rank := make([]int, size)
	for i := range parent {
		parent[i] = i
	}
	return &codexUnionFind{
		parent: parent,
		rank:   rank,
	}
}

func (uf *codexUnionFind) find(index int) int {
	if uf.parent[index] != index {
		uf.parent[index] = uf.find(uf.parent[index])
	}
	return uf.parent[index]
}

func (uf *codexUnionFind) union(a, b int) {
	rootA := uf.find(a)
	rootB := uf.find(b)
	if rootA == rootB {
		return
	}
	if uf.rank[rootA] < uf.rank[rootB] {
		rootA, rootB = rootB, rootA
	}
	uf.parent[rootB] = rootA
	if uf.rank[rootA] == uf.rank[rootB] {
		uf.rank[rootA]++
	}
}

func (c *Collector) buildCodexTeams(discovered []parser.CodexSessionDiscovery, now time.Time) []types.TeamInfo {
	if len(discovered) == 0 {
		return []types.TeamInfo{}
	}

	envelopes := make([]codexSessionEnvelope, 0, len(discovered))
	for _, session := range discovered {
		envelopes = append(envelopes, buildCodexSessionEnvelope(session, now))
	}

	unions := newCodexUnionFind(len(envelopes))
	keyOwners := make(map[string]int, len(envelopes)*2)

	for i, envelope := range envelopes {
		keys := []string{}
		if envelope.cwdKey != "" {
			keys = append(keys, "cwd:"+envelope.cwdKey)
		}
		if envelope.prefixKey != "" {
			keys = append(keys, "prefix:"+envelope.prefixKey)
		}

		for _, key := range keys {
			if existing, ok := keyOwners[key]; ok {
				unions.union(i, existing)
				continue
			}
			keyOwners[key] = i
		}
	}

	grouped := make(map[int][]codexSessionEnvelope, len(envelopes))
	order := make([]int, 0, len(envelopes))
	for i, envelope := range envelopes {
		root := unions.find(i)
		if _, ok := grouped[root]; !ok {
			order = append(order, root)
		}
		grouped[root] = append(grouped[root], envelope)
	}

	teams := make([]types.TeamInfo, 0, len(grouped))
	for _, root := range order {
		group := grouped[root]
		if len(group) == 0 {
			continue
		}

		team := buildCodexTeam(group)
		c.buildAgentNarratives(&team)
		teams = append(teams, team)
	}

	sort.SliceStable(teams, func(i, j int) bool {
		return latestTeamActivityTime(teams[i]).After(latestTeamActivityTime(teams[j]))
	})

	return teams
}

func buildCodexSessionEnvelope(session parser.CodexSessionDiscovery, now time.Time) codexSessionEnvelope {
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

	cleanCwd := cleanDisplayPath(session.Cwd)
	teamLabel := codexTeamLabel(cleanCwd)

	messageSummary := firstNonEmpty(session.LastAgentMessage, session.LastUserMessage)
	latestMessage := firstNonEmpty(session.LastAgentMessage, session.LastUserMessage)
	agentName := codexAgentName(session.DisplayName, session.SessionID, cleanCwd)

	agent := types.AgentInfo{
		Name:            agentName,
		Provider:        "codex",
		AgentID:         session.SessionID,
		AgentType:       "codex",
		Status:          status,
		CurrentTask:     session.LastUserMessage,
		LastActivity:    lastActive,
		Cwd:             cleanCwd,
		LatestMessage:   latestMessage,
		MessageSummary:  messageSummary,
		LatestResponse:  session.FullAgentMessage,
		LastMessageTime: lastActive,
		LastThinking:    session.LastReasoning,
		LastToolUse:     session.LastToolUse,
		LastToolDetail:  session.LastToolDetail,
		LastActiveTime:  lastActive,
		RecentEvents:    convertCodexEvents(session.RecentEvents),
	}

	return codexSessionEnvelope{
		session:    session,
		agent:      agent,
		createdAt:  createdAt,
		lastActive: lastActive,
		cwdKey:     normalizeComparablePath(cleanCwd),
		cwdDisplay: cleanCwd,
		prefixKey:  codexTeamPrefix(cleanCwd),
		teamLabel:  teamLabel,
	}
}

func buildCodexTeam(group []codexSessionEnvelope) types.TeamInfo {
	createdAt := time.Time{}
	leadSessionID := ""
	projectCwd := ""
	latestActive := time.Time{}
	members := make([]types.AgentInfo, 0, len(group))
	labelCounts := make(map[string]int)
	cwdCounts := make(map[string]int)
	cwdDisplay := make(map[string]string)

	for _, envelope := range group {
		if createdAt.IsZero() || envelope.createdAt.Before(createdAt) {
			createdAt = envelope.createdAt
		}
		if latestActive.IsZero() || envelope.lastActive.After(latestActive) {
			latestActive = envelope.lastActive
			leadSessionID = envelope.session.SessionID
		}
		if envelope.teamLabel != "" {
			labelCounts[envelope.teamLabel]++
		}
		if envelope.cwdKey != "" {
			cwdCounts[envelope.cwdKey]++
			if cwdDisplay[envelope.cwdKey] == "" && envelope.cwdDisplay != "" {
				cwdDisplay[envelope.cwdKey] = envelope.cwdDisplay
			}
		}
		members = append(members, envelope.agent)
	}

	sort.SliceStable(members, func(i, j int) bool {
		return members[i].LastActiveTime.After(members[j].LastActiveTime)
	})

	label := pickDominantCodexLabel(group, labelCounts)
	if label == "" {
		label = codexShortID(leadSessionID)
	}

	projectCwd = pickDominantCodexCwd(group, cwdCounts, cwdDisplay)

	return types.TeamInfo{
		Name:          "codex-" + label,
		Provider:      "codex",
		CreatedAt:     createdAt,
		LeadSessionID: leadSessionID,
		ProjectCwd:    projectCwd,
		Members:       members,
		Tasks:         []types.TaskInfo{},
	}
}

func pickDominantCodexLabel(group []codexSessionEnvelope, counts map[string]int) string {
	bestLabel := ""
	bestCount := 0
	bestActive := time.Time{}

	for _, envelope := range group {
		label := envelope.teamLabel
		if label == "" {
			continue
		}
		count := counts[label]
		if count > bestCount || (count == bestCount && envelope.lastActive.After(bestActive)) {
			bestLabel = label
			bestCount = count
			bestActive = envelope.lastActive
		}
	}

	return bestLabel
}

func pickDominantCodexCwd(group []codexSessionEnvelope, counts map[string]int, display map[string]string) string {
	bestKey := ""
	bestCount := 0
	bestActive := time.Time{}

	for _, envelope := range group {
		if envelope.cwdKey == "" {
			continue
		}
		count := counts[envelope.cwdKey]
		if count > bestCount || (count == bestCount && envelope.lastActive.After(bestActive)) {
			bestKey = envelope.cwdKey
			bestCount = count
			bestActive = envelope.lastActive
		}
	}

	if bestKey == "" {
		return ""
	}
	if value := strings.TrimSpace(display[bestKey]); value != "" {
		return value
	}
	return bestKey
}

func codexAgentName(displayName, sessionID, cwd string) string {
	if value := strings.TrimSpace(displayName); value != "" {
		return value
	}

	shortID := codexShortID(sessionID)
	label := codexTeamLabel(cwd)
	if label == "" {
		return "codex-" + shortID
	}
	return "codex-" + label + "-" + shortID
}

func codexShortID(sessionID string) string {
	shortID := strings.TrimSpace(sessionID)
	if len(shortID) > 8 {
		shortID = shortID[:8]
	}
	if shortID == "" {
		return "unknown"
	}
	return shortID
}

func codexTeamPrefix(cwd string) string {
	label := codexTeamLabel(cwd)
	if label == "" {
		return ""
	}
	return "codex-" + label
}

func codexTeamLabel(cwd string) string {
	base := strings.TrimSpace(filepath.Base(strings.TrimSpace(cwd)))
	if base == "" || base == "." || base == string(filepath.Separator) {
		return ""
	}
	return sanitizeCodexSlug(base)
}

func sanitizeCodexSlug(raw string) string {
	raw = strings.TrimSpace(strings.ToLower(raw))
	if raw == "" {
		return ""
	}

	var builder strings.Builder
	lastDash := false
	for _, r := range raw {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			builder.WriteRune(r)
			lastDash = false
			continue
		}
		if !lastDash {
			builder.WriteByte('-')
			lastDash = true
		}
	}

	return strings.Trim(builder.String(), "-")
}

func cleanDisplayPath(raw string) string {
	cleaned := strings.TrimSpace(raw)
	if cleaned == "" {
		return ""
	}
	cleaned = filepath.Clean(cleaned)
	if cleaned == "." {
		return ""
	}
	return cleaned
}

func normalizeComparablePath(raw string) string {
	cleaned := cleanDisplayPath(raw)
	if cleaned == "" {
		return ""
	}
	if windowsAbsPathPattern.MatchString(cleaned) {
		return strings.ToLower(cleaned)
	}
	return cleaned
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

		messages, err := parser.ParseInboxMessages(teamsDir, team.Name, agent.Name)
		if err != nil {
			log.Printf("Error parsing inbox history for %s: %v", agent.Name, err)
			continue
		}

		for j := len(messages) - 1; j >= 0 && len(agent.RecentEvents) < 8; j-- {
			text := strings.TrimSpace(messages[j].Text)
			if text == "" {
				continue
			}

			kind := "message"
			title := "消息"
			if messages[j].Summary != "" {
				title = "结果摘要"
			}

			agent.RecentEvents = append(agent.RecentEvents, types.AgentEvent{
				Kind:      kind,
				Title:     title,
				Text:      text,
				Source:    "inbox",
				Timestamp: messages[j].Timestamp,
			})
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
			if activity.LastResponse != "" {
				agent.LatestResponse = activity.LastResponse
			}
			agent.LastActiveTime = activity.LastActiveTime
			agent.RecentEvents = append(agent.RecentEvents, convertActivityEvents(activity.RecentEvents)...)
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
					if leadActivity.LastResponse != "" {
						agent.LatestResponse = leadActivity.LastResponse
					}
					agent.LastActiveTime = leadActivity.LastActiveTime
					agent.RecentEvents = append(agent.RecentEvents, convertActivityEvents(leadActivity.RecentEvents)...)
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

		agent.RecentEvents = compactAgentEvents(agent.RecentEvents, 10)
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
			member.RecentEvents = append([]types.AgentEvent(nil), member.RecentEvents...)

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

func convertActivityEvents(events []parser.AgentActivityEvent) []types.AgentEvent {
	if len(events) == 0 {
		return nil
	}

	converted := make([]types.AgentEvent, 0, len(events))
	for _, event := range events {
		converted = append(converted, types.AgentEvent{
			Kind:      event.Kind,
			Title:     event.Title,
			Text:      event.Text,
			Source:    "activity_log",
			Timestamp: event.Timestamp,
		})
	}
	return converted
}

func convertCodexEvents(events []parser.CodexSessionEvent) []types.AgentEvent {
	if len(events) == 0 {
		return nil
	}

	converted := make([]types.AgentEvent, 0, len(events))
	for _, event := range events {
		converted = append(converted, types.AgentEvent{
			Kind:      event.Kind,
			Title:     event.Title,
			Text:      event.Text,
			Source:    "codex_session",
			Timestamp: event.Timestamp,
		})
	}
	return converted
}

func convertOpenClawEvents(events []parser.OpenClawSessionEvent) []types.AgentEvent {
	if len(events) == 0 {
		return nil
	}

	converted := make([]types.AgentEvent, 0, len(events))
	for _, event := range events {
		converted = append(converted, types.AgentEvent{
			Kind:      event.Kind,
			Title:     event.Title,
			Text:      event.Text,
			Source:    "openclaw_session",
			Timestamp: event.Timestamp,
		})
	}
	return converted
}

func compactAgentEvents(events []types.AgentEvent, limit int) []types.AgentEvent {
	if len(events) == 0 || limit <= 0 {
		return nil
	}

	sort.SliceStable(events, func(i, j int) bool {
		return events[i].Timestamp.After(events[j].Timestamp)
	})

	compacted := make([]types.AgentEvent, 0, minCollector(limit, len(events)))
	seen := make(map[string]struct{})
	for _, event := range events {
		if strings.TrimSpace(event.Text) == "" {
			continue
		}

		key := event.Kind + "\x00" + event.Text
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		compacted = append(compacted, event)
		if len(compacted) >= limit {
			break
		}
	}

	return compacted
}

func minCollector(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
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

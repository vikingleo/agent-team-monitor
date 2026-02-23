package parser

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/liaoweijun/agent-team-monitor/pkg/types"
)

var discoverSessionIDPattern = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)
var discoverTeamNameHintPattern = regexp.MustCompile(`(?i)([a-z0-9]+(?:-[a-z0-9]+)*-team)\b`)
var discoverMemberCNPattern = regexp.MustCompile(`你是\s*([^\n，。,;；:：]+)`)
var discoverMemberENPattern = regexp.MustCompile(`(?i)you are\s*([^\n,.;:]+)`)
var discoverShutdownMemberPattern = regexp.MustCompile(`shutdown-[^@\s"]+@([a-zA-Z0-9_-]+)`)
var discoverSenderPattern = regexp.MustCompile(`"sender"\s*:\s*"([^"]+)"`)
var discoverFromPattern = regexp.MustCompile(`"from"\s*:\s*"([^"]+)"`)

const discoveredMemberRecentWindow = 45 * time.Minute
const discoveredMemberMaxCount = 12
const discoveryCacheMaxEntries = 4000

type discoveryRootCacheEntry struct {
	ModTimeUnixNano int64
	Size            int64
	Cwd             string
	HintCounts      map[string]int
}

type discoverySubagentCacheEntry struct {
	ModTimeUnixNano int64
	Size            int64
	Member          types.AgentInfo
	HintCounts      map[string]int
}

var discoveryParseCache = struct {
	mu        sync.RWMutex
	roots     map[string]discoveryRootCacheEntry
	subagents map[string]discoverySubagentCacheEntry
}{
	roots:     make(map[string]discoveryRootCacheEntry),
	subagents: make(map[string]discoverySubagentCacheEntry),
}

var discoveryMetricsCounter = struct {
	runs            atomic.Int64
	durationNS      atomic.Int64
	teams           atomic.Int64
	members         atomic.Int64
	rootCacheHits   atomic.Int64
	rootCacheMisses atomic.Int64
	subCacheHits    atomic.Int64
	subCacheMisses  atomic.Int64
}{}

// DiscoveryMetrics describes aggregated metrics for DiscoverProjectTeams.
type DiscoveryMetrics struct {
	Runs            int64
	TotalDuration   time.Duration
	TotalTeams      int64
	TotalMembers    int64
	RootCacheHits   int64
	RootCacheMisses int64
	SubCacheHits    int64
	SubCacheMisses  int64
}

// Delta returns metric differences between two snapshots.
func (m DiscoveryMetrics) Delta(previous DiscoveryMetrics) DiscoveryMetrics {
	return DiscoveryMetrics{
		Runs:            m.Runs - previous.Runs,
		TotalDuration:   m.TotalDuration - previous.TotalDuration,
		TotalTeams:      m.TotalTeams - previous.TotalTeams,
		TotalMembers:    m.TotalMembers - previous.TotalMembers,
		RootCacheHits:   m.RootCacheHits - previous.RootCacheHits,
		RootCacheMisses: m.RootCacheMisses - previous.RootCacheMisses,
		SubCacheHits:    m.SubCacheHits - previous.SubCacheHits,
		SubCacheMisses:  m.SubCacheMisses - previous.SubCacheMisses,
	}
}

// SnapshotDiscoveryMetrics returns current discovery metrics counters.
func SnapshotDiscoveryMetrics() DiscoveryMetrics {
	return DiscoveryMetrics{
		Runs:            discoveryMetricsCounter.runs.Load(),
		TotalDuration:   time.Duration(discoveryMetricsCounter.durationNS.Load()),
		TotalTeams:      discoveryMetricsCounter.teams.Load(),
		TotalMembers:    discoveryMetricsCounter.members.Load(),
		RootCacheHits:   discoveryMetricsCounter.rootCacheHits.Load(),
		RootCacheMisses: discoveryMetricsCounter.rootCacheMisses.Load(),
		SubCacheHits:    discoveryMetricsCounter.subCacheHits.Load(),
		SubCacheMisses:  discoveryMetricsCounter.subCacheMisses.Load(),
	}
}

// ProjectTeamDiscovery stores team-like activity inferred from ~/.claude/projects logs.
type ProjectTeamDiscovery struct {
	TeamNameHint  string
	LeadSessionID string
	ProjectCwd    string
	LastActiveAt  time.Time
	Members       []types.AgentInfo
}

// DiscoverProjectTeams scans ~/.claude/projects and infers active agent teams from root/subagent logs.
// A discovered team must have a root session log and at least one subagent log.
func DiscoverProjectTeams(projectsDir string, maxAge time.Duration) (discovered []ProjectTeamDiscovery, err error) {
	startAt := time.Now()
	defer func() {
		memberCount := 0
		for _, team := range discovered {
			memberCount += len(team.Members)
		}

		discoveryMetricsCounter.runs.Add(1)
		discoveryMetricsCounter.durationNS.Add(time.Since(startAt).Nanoseconds())
		discoveryMetricsCounter.teams.Add(int64(len(discovered)))
		discoveryMetricsCounter.members.Add(int64(memberCount))
	}()

	if projectsDir == "" {
		return nil, nil
	}

	rootLogs, err := filepath.Glob(filepath.Join(projectsDir, "*", "*.jsonl"))
	if err != nil {
		return nil, err
	}
	if len(rootLogs) == 0 {
		return nil, nil
	}

	now := time.Now()
	discovered = make([]ProjectTeamDiscovery, 0, len(rootLogs))

	for _, rootLogPath := range rootLogs {
		sessionID := strings.TrimSuffix(filepath.Base(rootLogPath), filepath.Ext(rootLogPath))
		if !discoverSessionIDPattern.MatchString(sessionID) {
			continue
		}

		rootInfo, err := os.Stat(rootLogPath)
		if err != nil || rootInfo.IsDir() {
			continue
		}

		subagentPattern := filepath.Join(filepath.Dir(rootLogPath), sessionID, "subagents", "agent-*.jsonl")
		subagentLogs, err := filepath.Glob(subagentPattern)
		if err != nil || len(subagentLogs) == 0 {
			continue
		}

		lastActive := rootInfo.ModTime()
		for _, subagentLogPath := range subagentLogs {
			if info, err := os.Stat(subagentLogPath); err == nil && info.ModTime().After(lastActive) {
				lastActive = info.ModTime()
			}
		}
		if maxAge > 0 && now.Sub(lastActive) > maxAge {
			continue
		}

		cwd, hintCounts := inspectDiscoveryRootLog(rootLogPath)
		members := make([]types.AgentInfo, 0, len(subagentLogs))
		memberIndex := make(map[string]int)

		for _, subagentLogPath := range subagentLogs {
			member, subHints, err := inspectDiscoverySubagentLog(subagentLogPath, cwd)
			if err != nil {
				continue
			}
			if member.AgentID == "" {
				continue
			}

			if member.LastActiveTime.After(lastActive) {
				lastActive = member.LastActiveTime
			}
			if member.Cwd == "" {
				member.Cwd = cwd
			}
			if member.LastActivity.IsZero() {
				member.LastActivity = member.LastActiveTime
			}

			key := memberKey(member)
			if key == "" {
				members = append(members, member)
				continue
			}
			if idx, ok := memberIndex[key]; ok {
				members[idx] = mergeDiscoveredAgent(members[idx], member)
			} else {
				memberIndex[key] = len(members)
				members = append(members, member)
			}

			for hint, count := range subHints {
				hintCounts[hint] += count
			}
		}

		if len(members) == 0 {
			continue
		}

		sort.SliceStable(members, func(i, j int) bool {
			return members[i].LastActiveTime.After(members[j].LastActiveTime)
		})
		members = compactDiscoveredMembers(members, lastActive)

		discovered = append(discovered, ProjectTeamDiscovery{
			TeamNameHint:  pickTeamNameHint(hintCounts),
			LeadSessionID: sessionID,
			ProjectCwd:    cwd,
			LastActiveAt:  lastActive,
			Members:       members,
		})
	}

	sort.SliceStable(discovered, func(i, j int) bool {
		return discovered[i].LastActiveAt.After(discovered[j].LastActiveAt)
	})

	return discovered, nil
}

func inspectDiscoveryRootLog(logPath string) (string, map[string]int) {
	info, err := os.Stat(logPath)
	if err != nil || info.IsDir() {
		return "", map[string]int{}
	}

	if cachedCwd, cachedHints, ok := loadRootDiscoveryCache(logPath, info); ok {
		return cachedCwd, cachedHints
	}

	file, err := os.Open(logPath)
	if err != nil {
		return "", map[string]int{}
	}
	defer file.Close()

	scanner := newLargeScanner(file)
	hintCounts := make(map[string]int)
	cwd := ""

	const maxScanLines = 160
	for i := 0; i < maxScanLines && scanner.Scan(); i++ {
		line := scanner.Text()
		collectTeamHints(hintCounts, line)

		var record activityRecord
		if err := json.Unmarshal([]byte(line), &record); err != nil {
			continue
		}

		if cwd == "" && record.Cwd != "" {
			cwd = record.Cwd
		}
		collectTeamHints(hintCounts, string(record.Message))
	}

	if err := scanner.Err(); err != nil {
		return cwd, hintCounts
	}

	saveRootDiscoveryCache(logPath, info, cwd, hintCounts)
	return cwd, hintCounts
}

func inspectDiscoverySubagentLog(logPath, fallbackCwd string) (types.AgentInfo, map[string]int, error) {
	info, err := os.Stat(logPath)
	if err != nil {
		return types.AgentInfo{}, nil, err
	}

	if cachedMember, cachedHints, ok := loadSubagentDiscoveryCache(logPath, info); ok {
		if cachedMember.Cwd == "" {
			cachedMember.Cwd = fallbackCwd
		}
		return cachedMember, cachedHints, nil
	}

	file, err := os.Open(logPath)
	if err != nil {
		return types.AgentInfo{}, nil, err
	}
	defer file.Close()

	scanner := newLargeScanner(file)
	hintCounts := make(map[string]int)

	agentID := strings.TrimSuffix(strings.TrimPrefix(filepath.Base(logPath), "agent-"), ".jsonl")
	memberName := ""
	cwd := ""
	joinedAt := time.Time{}

	const maxScanLines = 120
	for i := 0; i < maxScanLines && scanner.Scan(); i++ {
		line := scanner.Text()
		collectTeamHints(hintCounts, line)
		if memberName == "" {
			memberName = extractMemberNameFromRawText(line)
		}

		var record activityRecord
		if err := json.Unmarshal([]byte(line), &record); err != nil {
			continue
		}

		if record.AgentID != "" {
			agentID = record.AgentID
		}
		if cwd == "" && record.Cwd != "" {
			cwd = record.Cwd
		}
		if joinedAt.IsZero() {
			if ts, err := time.Parse(time.RFC3339, record.Timestamp); err == nil {
				joinedAt = ts
			}
		}

		collectTeamHints(hintCounts, string(record.Message))
		if memberName == "" {
			memberName = extractMemberNameFromRawText(string(record.Message))
		}
	}
	if err := scanner.Err(); err != nil {
		return types.AgentInfo{}, nil, err
	}

	if cwd == "" {
		cwd = fallbackCwd
	}
	if memberName == "" {
		memberName = "agent-" + agentID
	}

	lastActive := info.ModTime()
	member := types.AgentInfo{
		Name:           memberName,
		AgentID:        agentID,
		Status:         "unknown",
		JoinedAt:       joinedAt,
		LastActivity:   lastActive,
		LastActiveTime: lastActive,
		Cwd:            cwd,
	}

	saveSubagentDiscoveryCache(logPath, info, member, hintCounts)
	return member, hintCounts, nil
}

func collectTeamHints(hints map[string]int, raw string) {
	if raw == "" {
		return
	}

	matches := discoverTeamNameHintPattern.FindAllStringSubmatch(raw, -1)
	for _, match := range matches {
		if len(match) < 2 {
			continue
		}
		name := strings.ToLower(strings.TrimSpace(match[1]))
		if name == "" {
			continue
		}
		hints[name]++
	}
}

func pickTeamNameHint(hints map[string]int) string {
	bestName := ""
	bestCount := 0
	for name, count := range hints {
		if count > bestCount || (count == bestCount && (bestName == "" || name < bestName)) {
			bestName = name
			bestCount = count
		}
	}
	return bestName
}

func extractMemberNameFromRawText(raw string) string {
	if raw == "" {
		return ""
	}

	patterns := []*regexp.Regexp{
		discoverMemberCNPattern,
		discoverMemberENPattern,
		discoverShutdownMemberPattern,
		discoverSenderPattern,
		discoverFromPattern,
	}

	for _, pattern := range patterns {
		matches := pattern.FindStringSubmatch(raw)
		if len(matches) < 2 {
			continue
		}
		name := normalizeDiscoveredMemberName(matches[1])
		if name == "" || strings.EqualFold(name, "team-lead") {
			continue
		}
		return name
	}

	return ""
}

func normalizeDiscoveredMemberName(raw string) string {
	name := strings.TrimSpace(raw)
	name = strings.Trim(name, "\"'`")
	name = strings.Trim(name, " \t\r\n.,;:，。；：")
	if name == "" {
		return ""
	}
	if len([]rune(name)) > 64 {
		return ""
	}
	return name
}

func memberKey(agent types.AgentInfo) string {
	name := strings.ToLower(strings.TrimSpace(agent.Name))
	if name != "" && !strings.HasPrefix(name, "agent-") {
		return "name:" + name
	}

	if agent.AgentID != "" {
		return "id:" + strings.ToLower(agent.AgentID)
	}
	if name != "" {
		return "name:" + name
	}
	return ""
}

func mergeDiscoveredAgent(base, incoming types.AgentInfo) types.AgentInfo {
	if base.Name == "" {
		base.Name = incoming.Name
	}
	if base.Cwd == "" {
		base.Cwd = incoming.Cwd
	}
	if base.JoinedAt.IsZero() {
		base.JoinedAt = incoming.JoinedAt
	}

	if base.AgentID == "" {
		base.AgentID = incoming.AgentID
	}
	if incoming.LastActiveTime.After(base.LastActiveTime) {
		if incoming.Name != "" {
			base.Name = incoming.Name
		}
		if incoming.AgentID != "" {
			base.AgentID = incoming.AgentID
		}
		if incoming.Cwd != "" {
			base.Cwd = incoming.Cwd
		}
		if !incoming.JoinedAt.IsZero() {
			base.JoinedAt = incoming.JoinedAt
		}
		base.LastActiveTime = incoming.LastActiveTime
		base.LastActivity = incoming.LastActivity
	}
	return base
}

func compactDiscoveredMembers(members []types.AgentInfo, anchor time.Time) []types.AgentInfo {
	if len(members) == 0 {
		return members
	}

	recent := make([]types.AgentInfo, 0, len(members))
	if !anchor.IsZero() {
		cutoff := anchor.Add(-discoveredMemberRecentWindow)
		for _, member := range members {
			if member.LastActiveTime.IsZero() || !member.LastActiveTime.Before(cutoff) {
				recent = append(recent, member)
			}
		}
	}
	if len(recent) == 0 {
		recent = append(recent, members[0])
	}

	hasNamed := false
	for _, member := range recent {
		if !isFallbackMember(member) {
			hasNamed = true
			break
		}
	}

	compacted := make([]types.AgentInfo, 0, len(recent))
	seen := make(map[string]struct{})
	for _, member := range recent {
		if hasNamed && isFallbackMember(member) {
			continue
		}

		key := memberKey(member)
		if key == "" {
			continue
		}
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		compacted = append(compacted, member)

		if len(compacted) >= discoveredMemberMaxCount {
			break
		}
	}

	if len(compacted) == 0 {
		return recent[:1]
	}
	return compacted
}

func isFallbackMember(member types.AgentInfo) bool {
	name := strings.ToLower(strings.TrimSpace(member.Name))
	if name == "" {
		return true
	}
	if !strings.HasPrefix(name, "agent-") {
		return false
	}
	id := strings.ToLower(strings.TrimSpace(member.AgentID))
	return id == "" || name == "agent-"+id
}

func loadRootDiscoveryCache(path string, info os.FileInfo) (string, map[string]int, bool) {
	discoveryParseCache.mu.RLock()
	entry, ok := discoveryParseCache.roots[path]
	discoveryParseCache.mu.RUnlock()
	if !ok {
		discoveryMetricsCounter.rootCacheMisses.Add(1)
		return "", nil, false
	}
	// Claude JSONL logs are append-only in normal operation.
	// If file grows, previously parsed prefix remains valid.
	if info.Size() < entry.Size || info.ModTime().UnixNano() < entry.ModTimeUnixNano {
		discoveryMetricsCounter.rootCacheMisses.Add(1)
		return "", nil, false
	}
	discoveryMetricsCounter.rootCacheHits.Add(1)
	return entry.Cwd, cloneHintCounts(entry.HintCounts), true
}

func saveRootDiscoveryCache(path string, info os.FileInfo, cwd string, hints map[string]int) {
	discoveryParseCache.mu.Lock()
	defer discoveryParseCache.mu.Unlock()

	if len(discoveryParseCache.roots) > discoveryCacheMaxEntries {
		discoveryParseCache.roots = make(map[string]discoveryRootCacheEntry)
	}

	discoveryParseCache.roots[path] = discoveryRootCacheEntry{
		ModTimeUnixNano: info.ModTime().UnixNano(),
		Size:            info.Size(),
		Cwd:             cwd,
		HintCounts:      cloneHintCounts(hints),
	}
}

func loadSubagentDiscoveryCache(path string, info os.FileInfo) (types.AgentInfo, map[string]int, bool) {
	discoveryParseCache.mu.RLock()
	entry, ok := discoveryParseCache.subagents[path]
	discoveryParseCache.mu.RUnlock()
	if !ok {
		discoveryMetricsCounter.subCacheMisses.Add(1)
		return types.AgentInfo{}, nil, false
	}
	// Agent logs are append-only; reuse cached identity parse when file only grows.
	if info.Size() < entry.Size || info.ModTime().UnixNano() < entry.ModTimeUnixNano {
		discoveryMetricsCounter.subCacheMisses.Add(1)
		return types.AgentInfo{}, nil, false
	}
	discoveryMetricsCounter.subCacheHits.Add(1)
	return entry.Member, cloneHintCounts(entry.HintCounts), true
}

func saveSubagentDiscoveryCache(path string, info os.FileInfo, member types.AgentInfo, hints map[string]int) {
	discoveryParseCache.mu.Lock()
	defer discoveryParseCache.mu.Unlock()

	if len(discoveryParseCache.subagents) > discoveryCacheMaxEntries {
		discoveryParseCache.subagents = make(map[string]discoverySubagentCacheEntry)
	}

	discoveryParseCache.subagents[path] = discoverySubagentCacheEntry{
		ModTimeUnixNano: info.ModTime().UnixNano(),
		Size:            info.Size(),
		Member:          member,
		HintCounts:      cloneHintCounts(hints),
	}
}

func cloneHintCounts(source map[string]int) map[string]int {
	if len(source) == 0 {
		return map[string]int{}
	}

	cloned := make(map[string]int, len(source))
	for key, value := range source {
		cloned[key] = value
	}
	return cloned
}

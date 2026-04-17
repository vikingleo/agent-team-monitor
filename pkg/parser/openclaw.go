package parser

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
)

const openClawSessionTailLines = 320

var openClawSessionIDPattern = regexp.MustCompile(`[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}`)

// OpenClawSessionDiscovery is summarized runtime activity inferred from one OpenClaw session.
type OpenClawSessionDiscovery struct {
	AgentID          string
	SessionKey       string
	SessionID        string
	SessionPath      string
	Cwd              string
	Label            string
	DisplayName      string
	SpawnedBy        string
	StartedAt        time.Time
	LastActiveAt     time.Time
	LastUserMessage  string
	LastAgentMessage string
	FullAgentMessage string
	LastReasoning    string
	LastToolUse      string
	LastToolDetail   string
	RecentEvents     []OpenClawSessionEvent
}

// OpenClawSessionEvent is a recent structured event extracted from an OpenClaw transcript.
type OpenClawSessionEvent struct {
	Kind      string
	Title     string
	Text      string
	Timestamp time.Time
}

type openClawSessionIndexEntry struct {
	SessionID           string `json:"sessionId"`
	UpdatedAt           int64  `json:"updatedAt"`
	SessionFile         string `json:"sessionFile"`
	TranscriptPath      string `json:"transcriptPath"`
	Label               string `json:"label"`
	DisplayName         string `json:"displayName"`
	SpawnedBy           string `json:"spawnedBy"`
	SpawnedWorkspaceDir string `json:"spawnedWorkspaceDir"`
}

type openClawTranscriptEnvelope struct {
	Timestamp string                 `json:"timestamp"`
	Message   *openClawTranscriptMsg `json:"message"`
}

type openClawTranscriptMsg struct {
	Role       string      `json:"role"`
	Timestamp  interface{} `json:"timestamp"`
	StopReason string      `json:"stopReason"`
	Content    interface{} `json:"content"`
}

type openClawAssistantTextPart struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

type openClawAssistantToolCall struct {
	Type      string          `json:"type"`
	ID        string          `json:"id"`
	Name      string          `json:"name"`
	Arguments json.RawMessage `json:"arguments"`
}

type OpenClawSubagentRunRecord struct {
	RunID                string
	ChildSessionKey      string
	ControllerSessionKey string
	RequesterSessionKey  string
	Task                 string
	Label                string
	WorkspaceDir         string
	CreatedAt            time.Time
	StartedAt            time.Time
	EndedAt              time.Time
	SpawnMode            string
}

type openClawPersistedSubagentRegistry struct {
	Version int                               `json:"version"`
	Runs    map[string]openClawSubagentRunRaw `json:"runs"`
}

type openClawSubagentRunRaw struct {
	RunID                string `json:"runId"`
	ChildSessionKey      string `json:"childSessionKey"`
	ControllerSessionKey string `json:"controllerSessionKey"`
	RequesterSessionKey  string `json:"requesterSessionKey"`
	Task                 string `json:"task"`
	Label                string `json:"label"`
	WorkspaceDir         string `json:"workspaceDir"`
	CreatedAt            int64  `json:"createdAt"`
	StartedAt            int64  `json:"startedAt"`
	EndedAt              int64  `json:"endedAt"`
	SpawnMode            string `json:"spawnMode"`
}

// DiscoverOpenClawSessions scans ~/.openclaw/agents/*/sessions indexes and returns recent session activity.
func DiscoverOpenClawSessions(agentsDir string, maxAge time.Duration) ([]OpenClawSessionDiscovery, error) {
	if strings.TrimSpace(agentsDir) == "" {
		return nil, nil
	}

	entries, err := os.ReadDir(agentsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	now := time.Now()
	discovered := make([]OpenClawSessionDiscovery, 0)
	seenSessionIDs := make(map[string]struct{})

	for _, agentEntry := range entries {
		if !agentEntry.IsDir() {
			continue
		}

		agentID := strings.TrimSpace(agentEntry.Name())
		if agentID == "" {
			continue
		}

		sessionsDir := filepath.Join(agentsDir, agentID, "sessions")
		indexedSessions, err := discoverOpenClawIndexedSessions(agentID, sessionsDir, now, maxAge)
		if err == nil {
			for _, session := range indexedSessions {
				if session.SessionID == "" {
					continue
				}
				seenSessionIDs[session.SessionID] = struct{}{}
				discovered = append(discovered, session)
			}
		}

		globbedSessions, err := discoverOpenClawSessionFiles(agentID, sessionsDir, now, maxAge)
		if err != nil {
			continue
		}
		for _, session := range globbedSessions {
			if session.SessionID != "" {
				if _, exists := seenSessionIDs[session.SessionID]; exists {
					continue
				}
				seenSessionIDs[session.SessionID] = struct{}{}
			}
			discovered = append(discovered, session)
		}
	}

	sort.SliceStable(discovered, func(i, j int) bool {
		return discovered[i].LastActiveAt.After(discovered[j].LastActiveAt)
	})

	return discovered, nil
}

func discoverOpenClawIndexedSessions(agentID, sessionsDir string, now time.Time, maxAge time.Duration) ([]OpenClawSessionDiscovery, error) {
	storePath := filepath.Join(sessionsDir, "sessions.json")
	storeBytes, err := os.ReadFile(storePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	store := map[string]openClawSessionIndexEntry{}
	if err := json.Unmarshal(storeBytes, &store); err != nil {
		return nil, err
	}

	discovered := make([]OpenClawSessionDiscovery, 0, len(store))
	for sessionKey, entry := range store {
		session := OpenClawSessionDiscovery{
			AgentID:      agentID,
			SessionKey:   strings.TrimSpace(sessionKey),
			SessionID:    strings.TrimSpace(entry.SessionID),
			Cwd:          strings.TrimSpace(entry.SpawnedWorkspaceDir),
			Label:        strings.TrimSpace(entry.Label),
			DisplayName:  strings.TrimSpace(entry.DisplayName),
			SpawnedBy:    strings.TrimSpace(entry.SpawnedBy),
			LastActiveAt: parseOpenClawUnixMillis(entry.UpdatedAt),
		}
		if session.SessionID == "" {
			continue
		}

		sessionPath := resolveOpenClawSessionPath(sessionsDir, session.SessionID, entry)
		if sessionPath == "" {
			continue
		}

		discoveredSession, ok := buildOpenClawSessionFromPath(session, sessionPath, now, maxAge)
		if !ok {
			continue
		}
		discovered = append(discovered, discoveredSession)
	}

	return discovered, nil
}

func discoverOpenClawSessionFiles(agentID, sessionsDir string, now time.Time, maxAge time.Duration) ([]OpenClawSessionDiscovery, error) {
	sessionLogs, err := filepath.Glob(filepath.Join(sessionsDir, "*.jsonl"))
	if err != nil {
		return nil, err
	}
	if len(sessionLogs) == 0 {
		return nil, nil
	}

	discovered := make([]OpenClawSessionDiscovery, 0, len(sessionLogs))
	for _, sessionPath := range sessionLogs {
		sessionID := extractOpenClawSessionID(sessionPath)
		if sessionID == "" {
			continue
		}

		session := OpenClawSessionDiscovery{
			AgentID:   agentID,
			SessionID: sessionID,
		}
		sessionKey := inferOpenClawSessionKeyFromFilename(sessionPath, agentID)
		if sessionKey != "" {
			session.SessionKey = sessionKey
		}

		discoveredSession, ok := buildOpenClawSessionFromPath(session, sessionPath, now, maxAge)
		if !ok {
			continue
		}
		discovered = append(discovered, discoveredSession)
	}

	return discovered, nil
}

func buildOpenClawSessionFromPath(session OpenClawSessionDiscovery, sessionPath string, now time.Time, maxAge time.Duration) (OpenClawSessionDiscovery, bool) {
	info, err := os.Stat(sessionPath)
	if err != nil || info.IsDir() {
		return OpenClawSessionDiscovery{}, false
	}

	if session.LastActiveAt.IsZero() || info.ModTime().After(session.LastActiveAt) {
		session.LastActiveAt = info.ModTime()
	}
	if maxAge > 0 && now.Sub(session.LastActiveAt) > maxAge {
		return OpenClawSessionDiscovery{}, false
	}

	inspected, err := inspectOpenClawSessionLog(sessionPath)
	if err != nil {
		return OpenClawSessionDiscovery{}, false
	}

	session.SessionPath = sessionPath
	if inspected.StartedAt.After(time.Time{}) {
		session.StartedAt = inspected.StartedAt
	}
	if inspected.LastActiveAt.After(session.LastActiveAt) {
		session.LastActiveAt = inspected.LastActiveAt
	}
	if session.Cwd == "" {
		session.Cwd = inspected.Cwd
	}
	if session.Label == "" {
		session.Label = inspected.Label
	}
	if session.DisplayName == "" {
		session.DisplayName = inspected.DisplayName
	}
	session.LastUserMessage = inspected.LastUserMessage
	session.LastAgentMessage = inspected.LastAgentMessage
	session.FullAgentMessage = inspected.FullAgentMessage
	session.LastReasoning = inspected.LastReasoning
	session.LastToolUse = inspected.LastToolUse
	session.LastToolDetail = inspected.LastToolDetail
	session.RecentEvents = inspected.RecentEvents

	return session, true
}

func inspectOpenClawSessionLog(logPath string) (OpenClawSessionDiscovery, error) {
	file, err := os.Open(logPath)
	if err != nil {
		return OpenClawSessionDiscovery{}, err
	}
	defer file.Close()

	result := OpenClawSessionDiscovery{}
	ring := make([]string, openClawSessionTailLines)
	ringIdx := 0
	totalLines := 0

	scanner := newLargeScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		ring[ringIdx%openClawSessionTailLines] = line
		ringIdx++
		totalLines++
	}
	if err := scanner.Err(); err != nil {
		return OpenClawSessionDiscovery{}, err
	}

	count := totalLines
	if count > openClawSessionTailLines {
		count = openClawSessionTailLines
	}

	recentEvents := make([]OpenClawSessionEvent, 0, 10)

	for k := 0; k < count; k++ {
		idx := (ringIdx - 1 - k) % openClawSessionTailLines
		if idx < 0 {
			idx += openClawSessionTailLines
		}

		var envelope openClawTranscriptEnvelope
		if err := json.Unmarshal([]byte(ring[idx]), &envelope); err != nil {
			continue
		}
		if envelope.Message == nil {
			continue
		}

		ts := parseOpenClawTimestamp(envelope.Timestamp, envelope.Message.Timestamp)
		if !ts.IsZero() {
			if result.StartedAt.IsZero() || ts.Before(result.StartedAt) {
				result.StartedAt = ts
			}
			if result.LastActiveAt.IsZero() || ts.After(result.LastActiveAt) {
				result.LastActiveAt = ts
			}
		}

		role := strings.ToLower(strings.TrimSpace(envelope.Message.Role))
		switch role {
		case "user":
			text := extractOpenClawMessageText(envelope.Message.Content)
			if result.LastUserMessage == "" {
				result.LastUserMessage = normalizeCodexText(text, 120)
			}
			if text != "" {
				recentEvents = append(recentEvents, OpenClawSessionEvent{
					Kind:      "task",
					Title:     "用户请求",
					Text:      text,
					Timestamp: ts,
				})
			}
		case "assistant":
			fullText, thinking, toolUse, toolDetail := extractOpenClawAssistantData(envelope.Message.Content)
			if result.LastAgentMessage == "" {
				result.LastAgentMessage = normalizeCodexText(fullText, 150)
			}
			if result.FullAgentMessage == "" {
				result.FullAgentMessage = fullText
			}
			if result.LastReasoning == "" && thinking != "" {
				result.LastReasoning = normalizeCodexText(thinking, 150)
			}
			if result.LastToolUse == "" && toolUse != "" {
				result.LastToolUse = toolUse
				result.LastToolDetail = toolDetail
			}

			if fullText != "" {
				recentEvents = append(recentEvents, OpenClawSessionEvent{
					Kind:      "response",
					Title:     "输出",
					Text:      fullText,
					Timestamp: ts,
				})
			}
			if thinking != "" {
				recentEvents = append(recentEvents, OpenClawSessionEvent{
					Kind:      "thinking",
					Title:     "思路",
					Text:      thinking,
					Timestamp: ts,
				})
			}
			if toolUse != "" {
				text := normalizeToolEventText(toolUse, toolDetail)
				kind, title := classifyToolCall(toolUse, toolDetail)
				recentEvents = append(recentEvents, OpenClawSessionEvent{
					Kind:      kind,
					Title:     title,
					Text:      text,
					Timestamp: ts,
				})
			}
		case "toolresult", "tool":
			text := extractOpenClawToolResultText(envelope.Message.Content)
			if text != "" {
				kind, title := classifyToolResult("", text)
				recentEvents = append(recentEvents, OpenClawSessionEvent{
					Kind:      kind,
					Title:     title,
					Text:      text,
					Timestamp: ts,
				})
			}
		}
	}

	result.RecentEvents = dedupeOpenClawEvents(recentEvents, 24)
	return result, nil
}

func resolveOpenClawSessionPath(sessionsDir, sessionID string, entry openClawSessionIndexEntry) string {
	candidates := make([]string, 0, 5)

	if path := strings.TrimSpace(entry.SessionFile); path != "" {
		candidates = append(candidates, path)
	}
	if path := strings.TrimSpace(entry.TranscriptPath); path != "" {
		candidates = append(candidates, path)
	}
	candidates = append(candidates, filepath.Join(sessionsDir, sessionID+".jsonl"))

	for _, candidate := range candidates {
		if strings.TrimSpace(candidate) == "" {
			continue
		}

		resolved := candidate
		if !filepath.IsAbs(candidate) {
			resolved = filepath.Join(sessionsDir, candidate)
		}

		if info, err := os.Stat(resolved); err == nil && !info.IsDir() {
			return resolved
		}
	}

	return ""
}

func extractOpenClawSessionID(path string) string {
	base := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
	return openClawSessionIDPattern.FindString(base)
}

func inferOpenClawSessionKeyFromFilename(path, agentID string) string {
	sessionID := extractOpenClawSessionID(path)
	if sessionID == "" {
		return ""
	}

	trimmedAgentID := strings.TrimSpace(agentID)
	if trimmedAgentID == "" {
		return sessionID
	}
	return "agent:" + trimmedAgentID + ":" + sessionID
}

func DiscoverOpenClawSubagentRuns(stateDir string, maxAge time.Duration) ([]OpenClawSubagentRunRecord, error) {
	if strings.TrimSpace(stateDir) == "" {
		return nil, nil
	}

	registryPath := filepath.Join(stateDir, "subagents", "runs.json")
	raw, err := os.ReadFile(registryPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	var registry openClawPersistedSubagentRegistry
	if err := json.Unmarshal(raw, &registry); err != nil {
		return nil, err
	}

	now := time.Now()
	runs := make([]OpenClawSubagentRunRecord, 0, len(registry.Runs))
	for runID, entry := range registry.Runs {
		childSessionKey := strings.TrimSpace(entry.ChildSessionKey)
		if childSessionKey == "" {
			continue
		}

		run := OpenClawSubagentRunRecord{
			RunID:                firstNonEmptyOpenClaw(entry.RunID, runID),
			ChildSessionKey:      childSessionKey,
			ControllerSessionKey: strings.TrimSpace(entry.ControllerSessionKey),
			RequesterSessionKey:  strings.TrimSpace(entry.RequesterSessionKey),
			Task:                 strings.TrimSpace(entry.Task),
			Label:                strings.TrimSpace(entry.Label),
			WorkspaceDir:         strings.TrimSpace(entry.WorkspaceDir),
			CreatedAt:            parseOpenClawUnixMillis(entry.CreatedAt),
			StartedAt:            parseOpenClawUnixMillis(entry.StartedAt),
			EndedAt:              parseOpenClawUnixMillis(entry.EndedAt),
			SpawnMode:            strings.TrimSpace(entry.SpawnMode),
		}

		referenceTime := run.StartedAt
		if referenceTime.IsZero() {
			referenceTime = run.CreatedAt
		}
		if referenceTime.IsZero() {
			referenceTime = run.EndedAt
		}

		if maxAge > 0 && !referenceTime.IsZero() && now.Sub(referenceTime) > maxAge {
			continue
		}

		runs = append(runs, run)
	}

	sort.SliceStable(runs, func(i, j int) bool {
		return resolveOpenClawRunTime(runs[i]).After(resolveOpenClawRunTime(runs[j]))
	})

	return runs, nil
}

func resolveOpenClawRunTime(run OpenClawSubagentRunRecord) time.Time {
	if !run.StartedAt.IsZero() {
		return run.StartedAt
	}
	if !run.CreatedAt.IsZero() {
		return run.CreatedAt
	}
	return run.EndedAt
}

func firstNonEmptyOpenClaw(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}

func parseOpenClawUnixMillis(raw int64) time.Time {
	if raw <= 0 {
		return time.Time{}
	}
	return time.UnixMilli(raw)
}

func parseOpenClawTimestamp(primary string, fallback interface{}) time.Time {
	if ts := parseCodexTimestamp(primary); !ts.IsZero() {
		return ts
	}

	switch value := fallback.(type) {
	case float64:
		if value <= 0 {
			return time.Time{}
		}
		return time.UnixMilli(int64(value))
	case int64:
		return parseOpenClawUnixMillis(value)
	case json.Number:
		if millis, err := value.Int64(); err == nil {
			return parseOpenClawUnixMillis(millis)
		}
	case string:
		if ts := parseCodexTimestamp(value); !ts.IsZero() {
			return ts
		}
	}

	return time.Time{}
}

func extractOpenClawMessageText(content interface{}) string {
	switch value := content.(type) {
	case string:
		return sanitizeCodexStructuredText(value)
	case []interface{}:
		parts := make([]string, 0, len(value))
		for _, item := range value {
			switch part := item.(type) {
			case map[string]interface{}:
				kind := strings.ToLower(strings.TrimSpace(toString(part["type"])))
				if kind == "thinking" || kind == "toolcall" {
					continue
				}

				for _, key := range []string{"text", "content", "output", "summary", "result"} {
					text := sanitizeCodexStructuredText(toString(part[key]))
					if text != "" {
						parts = append(parts, text)
						break
					}
				}
			case string:
				text := sanitizeCodexStructuredText(part)
				if text != "" {
					parts = append(parts, text)
				}
			}
		}
		return strings.TrimSpace(strings.Join(parts, "\n\n"))
	case map[string]interface{}:
		for _, key := range []string{"text", "content", "output", "result", "summary"} {
			text := sanitizeCodexStructuredText(toString(value[key]))
			if text != "" {
				return text
			}
		}
	}

	return ""
}

func extractOpenClawAssistantData(content interface{}) (string, string, string, string) {
	items, ok := content.([]interface{})
	if !ok {
		text := extractOpenClawMessageText(content)
		return text, "", "", ""
	}

	textParts := make([]string, 0, len(items))
	thinkingParts := make([]string, 0, 2)
	toolUse := ""
	toolDetail := ""

	for _, rawItem := range items {
		part, ok := rawItem.(map[string]interface{})
		if !ok {
			if text := sanitizeCodexStructuredText(toString(rawItem)); text != "" {
				textParts = append(textParts, text)
			}
			continue
		}

		kind := strings.ToLower(strings.TrimSpace(toString(part["type"])))
		switch kind {
		case "text", "":
			if text := sanitizeCodexStructuredText(toString(part["text"])); text != "" {
				textParts = append(textParts, text)
			}
		case "thinking":
			if text := sanitizeCodexStructuredText(toString(part["text"])); text != "" {
				thinkingParts = append(thinkingParts, text)
			}
		case "toolcall":
			if toolUse == "" {
				toolUse = strings.TrimSpace(toString(part["name"]))
				toolDetail = summarizeOpenClawToolArguments(part["arguments"])
			}
		}
	}

	return strings.TrimSpace(strings.Join(textParts, "\n\n")), strings.TrimSpace(strings.Join(thinkingParts, "\n\n")), toolUse, toolDetail
}

func extractOpenClawToolResultText(content interface{}) string {
	switch value := content.(type) {
	case string:
		return sanitizeCodexStructuredText(value)
	case map[string]interface{}:
		for _, key := range []string{"text", "output", "content", "result", "error", "summary"} {
			text := sanitizeCodexStructuredText(toString(value[key]))
			if text != "" {
				return text
			}
		}
	case []interface{}:
		parts := make([]string, 0, len(value))
		for _, item := range value {
			if text := sanitizeCodexStructuredText(toString(item)); text != "" {
				parts = append(parts, text)
			}
		}
		return strings.TrimSpace(strings.Join(parts, "\n"))
	}
	return ""
}

func summarizeOpenClawToolArguments(raw interface{}) string {
	switch value := raw.(type) {
	case string:
		return summarizeCodexToolArguments(value)
	case map[string]interface{}:
		bytes, err := json.Marshal(value)
		if err != nil {
			return ""
		}
		return summarizeCodexToolArguments(string(bytes))
	default:
		bytes, err := json.Marshal(value)
		if err != nil {
			return ""
		}
		return summarizeCodexToolArguments(string(bytes))
	}
}

func dedupeOpenClawEvents(events []OpenClawSessionEvent, limit int) []OpenClawSessionEvent {
	if len(events) == 0 || limit <= 0 {
		return nil
	}

	deduped := make([]OpenClawSessionEvent, 0, minCodex(limit, len(events)))
	seen := make(map[string]struct{})
	for _, event := range events {
		key := event.Kind + "\x00" + event.Text
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		deduped = append(deduped, event)
		if len(deduped) >= limit {
			break
		}
	}

	return deduped
}

func toString(value interface{}) string {
	switch typed := value.(type) {
	case string:
		return typed
	case json.RawMessage:
		return string(typed)
	default:
		bytes, err := json.Marshal(typed)
		if err != nil {
			return ""
		}
		return string(bytes)
	}
}

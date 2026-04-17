package parser

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"
)

// ActivityLog represents a single log entry from agent's jsonl file
type ActivityLog struct {
	Type      string          `json:"type"`
	Timestamp string          `json:"timestamp"`
	AgentID   string          `json:"agentId"`
	Cwd       string          `json:"cwd"`
	Message   json.RawMessage `json:"message"`
}

// AssistantMessage represents an assistant's message with content
type AssistantMessage struct {
	Role    string          `json:"role"`
	Content json.RawMessage `json:"content"`
}

// ContentItem represents a single content item in a Claude activity record.
type ContentItem struct {
	Type      string          `json:"type"`
	Text      string          `json:"text,omitempty"`
	Name      string          `json:"name,omitempty"`        // For tool_use
	ID        string          `json:"id,omitempty"`          // For tool_use
	ToolUseID string          `json:"tool_use_id,omitempty"` // For tool_result
	Input     json.RawMessage `json:"input,omitempty"`
	Content   json.RawMessage `json:"content,omitempty"`
	Output    json.RawMessage `json:"output,omitempty"`
	Result    json.RawMessage `json:"result,omitempty"`
	Error     json.RawMessage `json:"error,omitempty"`
	IsError   bool            `json:"is_error,omitempty"`
}

// AgentActivity represents parsed agent activity
type AgentActivity struct {
	LastThinking   string    // Latest thinking/reasoning text
	LastToolUse    string    // Latest tool being used (e.g., "Read", "Edit", "Bash")
	LastToolDetail string    // Details about the tool use
	LastResponse   string    // Latest full assistant response text
	LastActiveTime time.Time // Last activity timestamp
	RecentEvents   []AgentActivityEvent
}

// AgentActivityEvent represents a recent parsed event from an activity log.
type AgentActivityEvent struct {
	Kind      string
	Title     string
	Text      string
	Timestamp time.Time
}

// AgentLogCandidate represents a candidate subagent log matched for a member
type AgentLogCandidate struct {
	Path          string
	AgentID       string
	SessionID     string
	Cwd           string
	FirstActiveAt time.Time
	LastActiveAt  time.Time
}

type activityRecord struct {
	Type      string          `json:"type"`
	Timestamp string          `json:"timestamp"`
	AgentID   string          `json:"agentId"`
	SessionID string          `json:"sessionId"`
	TeamName  string          `json:"teamName"`
	Cwd       string          `json:"cwd"`
	Message   json.RawMessage `json:"message"`
}

const activityLogTailLines = 200
const activityLogEventLimit = 24

// ParseAgentActivity parses the agent's jsonl log file and extracts recent activity
func ParseAgentActivity(logPath string) (*AgentActivity, error) {
	file, err := os.Open(logPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil // No log file is not an error
		}
		return nil, err
	}
	defer file.Close()

	activity := &AgentActivity{}
	scanner := newLargeScanner(file)

	// Use a ring buffer to keep only the last N lines in memory
	const tailSize = activityLogTailLines
	ring := make([]string, tailSize)
	ringIdx := 0
	totalLines := 0
	for scanner.Scan() {
		ring[ringIdx%tailSize] = scanner.Text()
		ringIdx++
		totalLines++
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	// Determine how many tail lines we have
	count := totalLines
	if count > tailSize {
		count = tailSize
	}

	recentEvents := make([]AgentActivityEvent, 0, activityLogEventLimit)
	pendingToolResults := make(map[string]int)

	// Process lines in reverse to get most recent activity first
	for k := 0; k < count; k++ {
		idx := (ringIdx - 1 - k) % tailSize
		if idx < 0 {
			idx += tailSize
		}

		var entry ActivityLog
		if err := json.Unmarshal([]byte(ring[idx]), &entry); err != nil {
			continue
		}

		// Parse timestamp
		timestamp, err := time.Parse(time.RFC3339, entry.Timestamp)
		if err != nil {
			continue
		}

		// Update last active time
		if activity.LastActiveTime.IsZero() || timestamp.After(activity.LastActiveTime) {
			activity.LastActiveTime = timestamp
		}

		// Only process assistant and user messages from activity logs.
		if entry.Type != "assistant" && entry.Type != "user" {
			continue
		}

		var msg AssistantMessage
		if err := json.Unmarshal(entry.Message, &msg); err != nil {
			continue
		}

		content := parseActivityContent(msg.Content)
		if len(content) == 0 {
			continue
		}

		// Extract thinking, tool use, and latest assistant response
		for _, item := range content {
			text := extractActivityItemText(item)

			if (item.Type == "thinking" || item.Type == "redacted_thinking") && activity.LastThinking == "" {
				text := normalizeActivitySummary(text, 150)
				activity.LastThinking = text
			}

			if item.Type == "text" && entry.Type == "assistant" && activity.LastResponse == "" {
				activity.LastResponse = sanitizeStructuredText(text)
			}

			if item.Type == "tool_use" && activity.LastToolUse == "" {
				activity.LastToolUse = item.Name
				// Try to extract tool details from the raw message
				activity.LastToolDetail = extractToolDetail(item.Name, entry.Message)
			}
		}

		recentEvents = appendActivityEvents(recentEvents, entry.Type, content, entry.Message, timestamp, pendingToolResults)

		// Stop once we have enough recent signals for the UI.
		if activity.LastToolUse != "" && activity.LastResponse != "" && len(recentEvents) >= activityLogEventLimit {
			break
		}
	}

	activity.RecentEvents = dedupeActivityEvents(recentEvents, activityLogEventLimit)

	return activity, nil
}

func normalizeActivityText(text string) string {
	text = sanitizeStructuredText(text)
	if text == "" {
		return ""
	}
	return strings.Join(strings.Fields(text), " ")
}

func normalizeActivitySummary(text string, maxRunes int) string {
	normalized := normalizeActivityText(text)
	if normalized == "" || maxRunes <= 0 {
		return normalized
	}

	if len(normalized) <= maxRunes {
		return normalized
	}

	cut := maxRunes
	for cut > 0 && !utf8.ValidString(normalized[:cut]) {
		cut--
	}
	if cut <= 0 {
		return "..."
	}

	return normalized[:cut] + "..."
}

func parseActivityContent(raw json.RawMessage) []ContentItem {
	var content []ContentItem
	if err := json.Unmarshal(raw, &content); err == nil {
		return content
	}

	var singleContent ContentItem
	if err := json.Unmarshal(raw, &singleContent); err == nil {
		return []ContentItem{singleContent}
	}

	var text string
	if err := json.Unmarshal(raw, &text); err == nil {
		return []ContentItem{{
			Type: "text",
			Text: text,
		}}
	}

	return nil
}

func appendActivityEvents(events []AgentActivityEvent, recordType string, content []ContentItem, rawMessage json.RawMessage, timestamp time.Time, pendingToolResults map[string]int) []AgentActivityEvent {
	for _, item := range content {
		switch item.Type {
		case "thinking", "redacted_thinking":
			text := normalizeActivityText(extractActivityItemText(item))
			if text == "" {
				continue
			}
			events = append(events, AgentActivityEvent{
				Kind:      "thinking",
				Title:     "思考",
				Text:      text,
				Timestamp: timestamp,
			})
		case "text":
			text := normalizeActivityText(item.Text)
			if text == "" {
				continue
			}
			kind := "response"
			title := "输出"
			if recordType == "user" {
				kind = "task"
				title = "用户指令"
			}
			events = append(events, AgentActivityEvent{
				Kind:      kind,
				Title:     title,
				Text:      text,
				Timestamp: timestamp,
			})
		case "tool_use":
			detail := extractToolDetail(item.Name, rawMessage)
			text := normalizeToolEventText(item.Name, detail)
			if text == "" {
				continue
			}
			kind, title := classifyToolCall(item.Name, detail)
			events = append(events, AgentActivityEvent{
				Kind:      kind,
				Title:     title,
				Text:      text,
				Timestamp: timestamp,
			})
			if item.ID != "" {
				if idx, ok := pendingToolResults[item.ID]; ok && idx >= 0 && idx < len(events) {
					events[idx].Kind, events[idx].Title = classifyToolResult(item.Name, events[idx].Text)
					delete(pendingToolResults, item.ID)
				}
			}
		case "tool_result":
			text := normalizeActivityText(extractActivityItemText(item))
			if text == "" {
				continue
			}
			kind, title := classifyToolResult(item.Name, text)
			events = append(events, AgentActivityEvent{
				Kind:      kind,
				Title:     title,
				Text:      text,
				Timestamp: timestamp,
			})
			if item.ToolUseID != "" {
				pendingToolResults[item.ToolUseID] = len(events) - 1
			}
		}
	}
	return events
}

func extractActivityItemText(item ContentItem) string {
	if text := sanitizeStructuredText(item.Text); text != "" {
		return text
	}
	for _, raw := range []json.RawMessage{item.Content, item.Output, item.Result, item.Error} {
		if text := extractActivityContentText(raw); text != "" {
			return text
		}
	}
	return ""
}

func extractActivityContentText(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}

	var value interface{}
	if err := json.Unmarshal(raw, &value); err != nil {
		return sanitizeStructuredText(strings.TrimSpace(string(raw)))
	}
	return extractActivityAnyText(value)
}

func extractActivityAnyText(value interface{}) string {
	switch typed := value.(type) {
	case string:
		return sanitizeStructuredText(typed)
	case []interface{}:
		parts := make([]string, 0, len(typed))
		for _, item := range typed {
			text := extractActivityAnyText(item)
			if text != "" {
				parts = append(parts, text)
			}
		}
		return strings.TrimSpace(strings.Join(parts, "\n"))
	case map[string]interface{}:
		kind := strings.ToLower(strings.TrimSpace(toString(typed["type"])))
		if kind == "tool_reference" {
			if name := sanitizeStructuredText(toString(typed["tool_name"])); name != "" {
				return name
			}
		}

		for _, key := range []string{"text", "content", "output", "result", "error", "summary", "tool_name"} {
			if text := sanitizeStructuredText(toString(typed[key])); text != "" {
				return text
			}
		}
	}
	return ""
}

func dedupeActivityEvents(events []AgentActivityEvent, limit int) []AgentActivityEvent {
	if len(events) == 0 || limit <= 0 {
		return nil
	}

	deduped := make([]AgentActivityEvent, 0, min(limit, len(events)))
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

func sanitizeStructuredText(text string) string {
	trimmed := strings.TrimSpace(strings.ReplaceAll(text, "\r\n", "\n"))
	if trimmed == "" {
		return ""
	}

	lines := strings.Split(trimmed, "\n")
	result := make([]string, 0, len(lines))
	blankCount := 0
	for _, line := range lines {
		cleanLine := strings.TrimRightFunc(line, unicode.IsSpace)
		if strings.TrimSpace(cleanLine) == "" {
			blankCount++
			if blankCount > 1 {
				continue
			}
			result = append(result, "")
			continue
		}

		blankCount = 0
		result = append(result, cleanLine)
	}

	return strings.TrimSpace(strings.Join(result, "\n"))
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// extractToolDetail extracts details about tool usage
func extractToolDetail(toolName string, rawMessage json.RawMessage) string {
	var msg map[string]interface{}
	if err := json.Unmarshal(rawMessage, &msg); err != nil {
		return ""
	}

	content, ok := msg["content"].([]interface{})
	if !ok {
		return ""
	}

	for _, item := range content {
		itemMap, ok := item.(map[string]interface{})
		if !ok {
			continue
		}

		if itemMap["type"] == "tool_use" && itemMap["name"] == toolName {
			input, ok := itemMap["input"].(map[string]interface{})
			if !ok {
				return ""
			}

			// Extract relevant details based on tool type
			switch toolName {
			case "Read":
				if filePath, ok := input["file_path"].(string); ok {
					return filepath.Base(filePath)
				}
			case "Edit":
				if filePath, ok := input["file_path"].(string); ok {
					return filepath.Base(filePath)
				}
			case "Write":
				if filePath, ok := input["file_path"].(string); ok {
					return filepath.Base(filePath)
				}
			case "Bash":
				if command, ok := input["command"].(string); ok {
					// Truncate long commands
					if len(command) > 50 {
						command = command[:50] + "..."
					}
					return command
				}
			case "Grep":
				if pattern, ok := input["pattern"].(string); ok {
					return "搜索: " + pattern
				}
			case "Glob":
				if pattern, ok := input["pattern"].(string); ok {
					return "查找: " + pattern
				}
			}
		}
	}

	return ""
}

// FindAgentLogFileByCwd finds the agent's log file by matching working directory
// Returns logFilePath, agentID, error
func FindAgentLogFileByCwd(projectsDir, cwd string) (string, string, error) {
	if cwd == "" {
		return "", "", nil
	}

	// Use filepath.Walk to recursively search for agent log files
	var foundLogPath, foundAgentID string

	err := filepath.Walk(projectsDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // Skip errors
		}

		// Check if this is an agent log file
		if !info.IsDir() && strings.HasPrefix(info.Name(), "agent-") && strings.HasSuffix(info.Name(), ".jsonl") {
			// Read first line to get cwd
			file, err := os.Open(path)
			if err != nil {
				return nil
			}
			defer file.Close()

			scanner := newLargeScanner(file)
			if scanner.Scan() {
				var log ActivityLog
				if err := json.Unmarshal(scanner.Bytes(), &log); err != nil {
					return nil
				}

				// Check if cwd matches
				if log.Cwd == cwd {
					foundLogPath = path
					foundAgentID = log.AgentID
					return filepath.SkipAll // Found it, stop walking
				}
			}
		}

		return nil
	})

	if err != nil && err != filepath.SkipAll {
		return "", "", err
	}

	return foundLogPath, foundAgentID, nil
}

// FindAgentLogFileForMember finds the best matching agent log file for a team member.
// Strategy:
// 1) Prefer logs under team's leadSessionId/subagents if available.
// 2) Match logs whose text content includes "你是 {memberName}" or "You are {memberName}".
// 3) Fallback to cwd match if no member-text match found.
// 4) Pick the most recently active candidate.
// Returns logFilePath, agentID, sessionID, error
func FindAgentLogFileForMember(projectsDir, leadSessionID, memberName, cwd string, joinedAt time.Time) (string, string, string, error) {
	if projectsDir == "" || memberName == "" {
		return "", "", "", nil
	}

	aliases := memberAliases(memberName)
	candidates, err := findMemberLogCandidates(projectsDir, leadSessionID, aliases, cwd, joinedAt)
	if err != nil {
		return "", "", "", err
	}
	if len(candidates) == 0 {
		return "", "", "", nil
	}

	sort.SliceStable(candidates, func(i, j int) bool {
		if !joinedAt.IsZero() {
			di := distanceFromJoinedAt(candidates[i], joinedAt)
			dj := distanceFromJoinedAt(candidates[j], joinedAt)
			if di != dj {
				return di < dj
			}
		}

		if candidates[i].LastActiveAt.Equal(candidates[j].LastActiveAt) {
			return candidates[i].Path < candidates[j].Path
		}
		return candidates[i].LastActiveAt.After(candidates[j].LastActiveAt)
	})

	return candidates[0].Path, candidates[0].AgentID, candidates[0].SessionID, nil
}

func findMemberLogCandidates(projectsDir, leadSessionID string, memberAliases []string, cwd string, joinedAt time.Time) ([]AgentLogCandidate, error) {

	searchRoots := []string{}
	if leadSessionID != "" {
		searchRoots = append(searchRoots, filepath.Join(projectsDir, "*", leadSessionID, "subagents"))
	}
	searchRoots = append(searchRoots, filepath.Join(projectsDir, "*", "*", "subagents"))

	seen := make(map[string]bool)
	allLogs := make([]string, 0)
	for _, pattern := range searchRoots {
		matches, err := filepath.Glob(filepath.Join(pattern, "agent-*.jsonl"))
		if err != nil {
			continue
		}
		for _, match := range matches {
			if !seen[match] {
				seen[match] = true
				allLogs = append(allLogs, match)
			}
		}
	}

	if len(allLogs) == 0 {
		return nil, nil
	}

	matchedByMember := make([]AgentLogCandidate, 0)
	fallbackByCwd := make([]AgentLogCandidate, 0)

	for _, logPath := range allLogs {
		candidate, memberMatched, cwdMatched, err := inspectLogCandidate(logPath, memberAliases, cwd)
		if err != nil {
			continue
		}

		if !joinedAt.IsZero() && !candidate.LastActiveAt.IsZero() && candidate.LastActiveAt.Before(joinedAt.Add(-2*time.Minute)) {
			// Ignore stale historical sessions that ended before this member joined.
			continue
		}

		if memberMatched {
			matchedByMember = append(matchedByMember, candidate)
			continue
		}

		if cwdMatched {
			fallbackByCwd = append(fallbackByCwd, candidate)
		}
	}

	if len(matchedByMember) > 0 {
		return matchedByMember, nil
	}

	return fallbackByCwd, nil
}

func inspectLogCandidate(logPath string, memberAliases []string, cwd string) (AgentLogCandidate, bool, bool, error) {
	file, err := os.Open(logPath)
	if err != nil {
		return AgentLogCandidate{}, false, false, err
	}
	defer file.Close()

	scanner := newLargeScanner(file)
	candidate := AgentLogCandidate{Path: logPath}
	memberMatched := false
	lineCount := 0

	// Member identity hints are typically in the first few messages.
	// Scan all lines for timestamps but only check member identity in the first 30 lines.
	const identityScanLimit = 30

	for scanner.Scan() {
		line := scanner.Bytes()
		var record activityRecord
		if err := json.Unmarshal(line, &record); err != nil {
			lineCount++
			continue
		}

		if candidate.AgentID == "" {
			candidate.AgentID = record.AgentID
		}
		if candidate.SessionID == "" {
			candidate.SessionID = record.SessionID
		}
		if candidate.Cwd == "" {
			candidate.Cwd = record.Cwd
		}

		if ts, err := time.Parse(time.RFC3339, record.Timestamp); err == nil {
			if candidate.FirstActiveAt.IsZero() || ts.Before(candidate.FirstActiveAt) {
				candidate.FirstActiveAt = ts
			}
			if candidate.LastActiveAt.IsZero() || ts.After(candidate.LastActiveAt) {
				candidate.LastActiveAt = ts
			}
		}

		if !memberMatched && lineCount < identityScanLimit && messageContainsMember(record.Message, memberAliases) {
			memberMatched = true
		}

		lineCount++
	}

	if err := scanner.Err(); err != nil {
		return AgentLogCandidate{}, false, false, err
	}

	if candidate.AgentID == "" {
		candidate.AgentID = strings.TrimSuffix(strings.TrimPrefix(filepath.Base(logPath), "agent-"), ".jsonl")
	}

	cwdMatched := cwd != "" && candidate.Cwd == cwd

	return candidate, memberMatched, cwdMatched, nil
}

func messageContainsMember(raw json.RawMessage, aliases []string) bool {
	if len(raw) == 0 || len(aliases) == 0 {
		return false
	}

	var msg struct {
		Content interface{} `json:"content"`
	}
	if err := json.Unmarshal(raw, &msg); err != nil {
		return false
	}

	checkText := func(text string) bool {
		if text == "" {
			return false
		}
		lower := strings.ToLower(text)
		for _, alias := range aliases {
			if alias == "" {
				continue
			}
			if strings.Contains(lower, fmt.Sprintf("你是 %s", alias)) ||
				strings.Contains(lower, fmt.Sprintf("you are %s", alias)) {
				return true
			}
		}
		return false
	}

	switch content := msg.Content.(type) {
	case string:
		return checkText(content)
	case []interface{}:
		for _, item := range content {
			itemMap, ok := item.(map[string]interface{})
			if !ok {
				continue
			}
			typeVal, _ := itemMap["type"].(string)
			if typeVal != "text" {
				continue
			}
			textVal, _ := itemMap["text"].(string)
			if checkText(textVal) {
				return true
			}
		}
	}

	// Fallback: structured content didn't match; skip raw string search to avoid false positives
	return false
}

func newLargeScanner(file *os.File) *bufio.Scanner {
	scanner := bufio.NewScanner(file)
	buf := make([]byte, 0, 64*1024)
	scanner.Buffer(buf, 10*1024*1024)
	return scanner
}

func distanceFromJoinedAt(candidate AgentLogCandidate, joinedAt time.Time) time.Duration {
	if joinedAt.IsZero() {
		return 0
	}

	base := candidate.FirstActiveAt
	if base.IsZero() {
		base = candidate.LastActiveAt
	}
	if base.IsZero() {
		return time.Duration(1<<63 - 1)
	}

	delta := base.Sub(joinedAt)
	if delta < 0 {
		delta = -delta
	}
	return delta
}

func memberAliases(memberName string) []string {
	name := strings.ToLower(strings.TrimSpace(memberName))
	if name == "" {
		return nil
	}

	seen := map[string]bool{name: true}
	aliases := []string{name}

	re := regexp.MustCompile(`-[0-9]+$`)
	base := re.ReplaceAllString(name, "")
	if base != "" && !seen[base] {
		aliases = append(aliases, base)
		seen[base] = true
	}

	return aliases
}

// FindLeadSessionLogFile returns the lead session log file path, if it exists.
func FindLeadSessionLogFile(projectsDir, leadSessionID string) (string, error) {
	if projectsDir == "" || leadSessionID == "" {
		return "", nil
	}

	pattern := filepath.Join(projectsDir, "*", leadSessionID+".jsonl")
	matches, err := filepath.Glob(pattern)
	if err != nil {
		return "", err
	}
	if len(matches) == 0 {
		return "", nil
	}
	if len(matches) == 1 {
		return matches[0], nil
	}

	best := matches[0]
	bestMod := time.Time{}
	for _, match := range matches {
		info, err := os.Stat(match)
		if err != nil {
			continue
		}
		if bestMod.IsZero() || info.ModTime().After(bestMod) {
			best = match
			bestMod = info.ModTime()
		}
	}

	return best, nil
}

// FindSessionCwd returns the absolute working directory (cwd) for a session.
// It reads the root session log and returns the first non-empty cwd found.
func FindSessionCwd(projectsDir, sessionID string) (string, error) {
	logPath, err := FindLeadSessionLogFile(projectsDir, sessionID)
	if err != nil || logPath == "" {
		return "", err
	}

	file, err := os.Open(logPath)
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil
		}
		return "", err
	}
	defer file.Close()

	scanner := newLargeScanner(file)
	const maxScanLines = 200

	for i := 0; i < maxScanLines && scanner.Scan(); i++ {
		var record activityRecord
		if err := json.Unmarshal(scanner.Bytes(), &record); err != nil {
			continue
		}
		if record.Cwd != "" {
			return record.Cwd, nil
		}
	}

	if err := scanner.Err(); err != nil {
		return "", err
	}

	return "", nil
}

// FindSessionTeamName returns the most recent non-empty runtime teamName for a session.
func FindSessionTeamName(projectsDir, sessionID string) (string, error) {
	logPath, err := FindLeadSessionLogFile(projectsDir, sessionID)
	if err != nil || logPath == "" {
		return "", err
	}

	file, err := os.Open(logPath)
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil
		}
		return "", err
	}
	defer file.Close()

	scanner := newLargeScanner(file)
	teamName := ""

	for scanner.Scan() {
		var record activityRecord
		if err := json.Unmarshal(scanner.Bytes(), &record); err != nil {
			continue
		}
		if strings.TrimSpace(record.TeamName) != "" {
			teamName = strings.TrimSpace(record.TeamName)
		}
	}

	if err := scanner.Err(); err != nil {
		return "", err
	}

	return teamName, nil
}

// FindAgentLogFile finds the agent's log file by matching agent ID (deprecated, use FindAgentLogFileByCwd)
func FindAgentLogFile(projectsDir, agentID string) (string, error) {
	// Search for agent log files
	pattern := filepath.Join(projectsDir, "*", "subagents", "agent-"+agentID+".jsonl")
	matches, err := filepath.Glob(pattern)
	if err != nil {
		return "", err
	}

	if len(matches) > 0 {
		return matches[0], nil
	}

	return "", nil
}

package parser

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
)

var codexSessionIDPattern = regexp.MustCompile(`[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}`)

const codexSessionTailLines = 240

// CodexSessionDiscovery is summarized runtime activity inferred from one codex session log.
type CodexSessionDiscovery struct {
	SessionID        string
	SessionPath      string
	Cwd              string
	DisplayName      string
	AgentRole        string
	StartedAt        time.Time
	LastActiveAt     time.Time
	LastUserMessage  string
	LastAgentMessage string
	FullAgentMessage string
	LastReasoning    string
	LastToolUse      string
	LastToolDetail   string
	RecentEvents     []CodexSessionEvent
}

// CodexSessionEvent is a recent structured event extracted from a codex session log.
type CodexSessionEvent struct {
	Kind      string
	Title     string
	Text      string
	Timestamp time.Time
}

type codexLogEntry struct {
	Timestamp string          `json:"timestamp"`
	Type      string          `json:"type"`
	Payload   json.RawMessage `json:"payload"`
}

type codexResponseMessagePart struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

type codexResponsePayload struct {
	Type      string                     `json:"type"`
	Role      string                     `json:"role"`
	Name      string                     `json:"name"`
	Arguments string                     `json:"arguments"`
	CallID    string                     `json:"call_id"`
	Output    string                     `json:"output"`
	Content   []codexResponseMessagePart `json:"content"`
}

type codexEventPayload struct {
	Type             string `json:"type"`
	Message          string `json:"message"`
	Text             string `json:"text"`
	CallID           string `json:"call_id"`
	AggregatedOutput string `json:"aggregated_output"`
	Stdout           string `json:"stdout"`
	Stderr           string `json:"stderr"`
}

type codexTurnContextPayload struct {
	Cwd string `json:"cwd"`
}

type codexSessionMetaPayload struct {
	ID            string                 `json:"id"`
	Timestamp     string                 `json:"timestamp"`
	Cwd           string                 `json:"cwd"`
	AgentNickname string                 `json:"agent_nickname"`
	AgentRole     string                 `json:"agent_role"`
	Source        codexSessionMetaSource `json:"source"`
}

type codexSessionMetaSource struct {
	Subagent codexSessionMetaSubagent `json:"subagent"`
}

type codexSessionMetaSubagent struct {
	ThreadSpawn codexSessionThreadSpawn `json:"thread_spawn"`
}

type codexSessionThreadSpawn struct {
	AgentNickname string `json:"agent_nickname"`
	AgentRole     string `json:"agent_role"`
}

// DiscoverCodexSessions scans ~/.codex/sessions and returns recent session activity.
func DiscoverCodexSessions(sessionsDir string, maxAge time.Duration) ([]CodexSessionDiscovery, error) {
	if sessionsDir == "" {
		return nil, nil
	}

	sessionLogs, err := filepath.Glob(filepath.Join(sessionsDir, "*", "*", "*", "*.jsonl"))
	if err != nil {
		return nil, err
	}
	if len(sessionLogs) == 0 {
		return nil, nil
	}

	now := time.Now()
	discovered := make([]CodexSessionDiscovery, 0, len(sessionLogs))
	for _, sessionPath := range sessionLogs {
		info, err := os.Stat(sessionPath)
		if err != nil || info.IsDir() {
			continue
		}
		if maxAge > 0 && now.Sub(info.ModTime()) > maxAge {
			continue
		}

		session, err := inspectCodexSessionLog(sessionPath)
		if err != nil {
			continue
		}
		if session.SessionID == "" {
			session.SessionID = extractCodexSessionID(sessionPath)
		}
		if session.SessionID == "" {
			continue
		}

		if session.StartedAt.IsZero() {
			session.StartedAt = info.ModTime()
		}
		if session.LastActiveAt.IsZero() || info.ModTime().After(session.LastActiveAt) {
			session.LastActiveAt = info.ModTime()
		}
		if maxAge > 0 && now.Sub(session.LastActiveAt) > maxAge {
			continue
		}

		session.SessionPath = sessionPath
		discovered = append(discovered, session)
	}

	sort.SliceStable(discovered, func(i, j int) bool {
		return discovered[i].LastActiveAt.After(discovered[j].LastActiveAt)
	})

	return discovered, nil
}

func inspectCodexSessionLog(logPath string) (CodexSessionDiscovery, error) {
	file, err := os.Open(logPath)
	if err != nil {
		return CodexSessionDiscovery{}, err
	}
	defer file.Close()

	result := CodexSessionDiscovery{}
	ring := make([]string, codexSessionTailLines)
	ringIdx := 0
	totalLines := 0

	scanner := newLargeScanner(file)
	firstTimestamp := time.Time{}
	for scanner.Scan() {
		line := scanner.Text()
		if totalLines == 0 {
			var firstEntry codexLogEntry
			if err := json.Unmarshal([]byte(line), &firstEntry); err == nil {
				if ts := parseCodexTimestamp(firstEntry.Timestamp); !ts.IsZero() {
					firstTimestamp = ts
				}
				if firstEntry.Type == "session_meta" {
					applyCodexSessionMeta(&result, firstEntry.Payload)
				}
			}
		}

		ring[ringIdx%codexSessionTailLines] = line
		ringIdx++
		totalLines++
	}
	if err := scanner.Err(); err != nil {
		return CodexSessionDiscovery{}, err
	}

	if result.StartedAt.IsZero() && !firstTimestamp.IsZero() {
		result.StartedAt = firstTimestamp
	}

	count := totalLines
	if count > codexSessionTailLines {
		count = codexSessionTailLines
	}

	recentEvents := make([]CodexSessionEvent, 0, 16)
	pendingToolOutputIdx := make(map[string]int)
	seenToolOutputCalls := make(map[string]struct{})

	for k := 0; k < count; k++ {
		idx := (ringIdx - 1 - k) % codexSessionTailLines
		if idx < 0 {
			idx += codexSessionTailLines
		}

		var entry codexLogEntry
		if err := json.Unmarshal([]byte(ring[idx]), &entry); err != nil {
			continue
		}

		ts := parseCodexTimestamp(entry.Timestamp)
		if !ts.IsZero() {
			if result.LastActiveAt.IsZero() || ts.After(result.LastActiveAt) {
				result.LastActiveAt = ts
			}
		}

		switch entry.Type {
		case "session_meta":
			applyCodexSessionMeta(&result, entry.Payload)
		case "turn_context":
			if result.Cwd == "" {
				var payload codexTurnContextPayload
				if err := json.Unmarshal(entry.Payload, &payload); err == nil {
					result.Cwd = strings.TrimSpace(payload.Cwd)
				}
			}
		case "event_msg":
			var payload codexEventPayload
			if err := json.Unmarshal(entry.Payload, &payload); err != nil {
				continue
			}

			switch payload.Type {
			case "user_message":
				if result.LastUserMessage == "" {
					result.LastUserMessage = normalizeCodexText(payload.Message, 120)
				}
				if text := sanitizeCodexStructuredText(payload.Message); text != "" {
					recentEvents = append(recentEvents, CodexSessionEvent{
						Kind:      "task",
						Title:     "用户请求",
						Text:      text,
						Timestamp: ts,
					})
				}
			case "agent_message":
				if result.LastAgentMessage == "" {
					result.LastAgentMessage = normalizeCodexText(payload.Message, 150)
				}
				if result.FullAgentMessage == "" {
					result.FullAgentMessage = sanitizeCodexStructuredText(payload.Message)
				}
				if text := sanitizeCodexStructuredText(payload.Message); text != "" {
					recentEvents = append(recentEvents, CodexSessionEvent{
						Kind:      "response",
						Title:     "输出",
						Text:      text,
						Timestamp: ts,
					})
				}
			case "agent_reasoning":
				if result.LastReasoning == "" {
					result.LastReasoning = normalizeCodexText(payload.Text, 150)
				}
				if text := sanitizeCodexStructuredText(payload.Text); text != "" {
					recentEvents = append(recentEvents, CodexSessionEvent{
						Kind:      "thinking",
						Title:     "思路",
						Text:      text,
						Timestamp: ts,
					})
				}
			case "task_started":
				recentEvents = append(recentEvents, CodexSessionEvent{
					Kind:      "status",
					Title:     "轮次开始",
					Text:      "开始处理当前请求",
					Timestamp: ts,
				})
			case "exec_command_end":
				if payload.CallID != "" {
					if _, ok := seenToolOutputCalls[payload.CallID]; ok {
						continue
					}
				}
				text := sanitizeCodexStructuredText(payload.AggregatedOutput)
				if text == "" {
					text = sanitizeCodexStructuredText(strings.TrimSpace(payload.Stdout + "\n" + payload.Stderr))
				}
				if text == "" {
					continue
				}
				kind, title := classifyToolResult("exec_command", text)
				recentEvents = append(recentEvents, CodexSessionEvent{
					Kind:      kind,
					Title:     title,
					Text:      text,
					Timestamp: ts,
				})
			}
		case "response_item":
			var payload codexResponsePayload
			if err := json.Unmarshal(entry.Payload, &payload); err != nil {
				continue
			}

			if payload.Type == "function_call" && result.LastToolUse == "" {
				result.LastToolUse = strings.TrimSpace(payload.Name)
				result.LastToolDetail = summarizeCodexToolArguments(payload.Arguments)
			}

			if payload.Type == "function_call" {
				detail := summarizeCodexToolArguments(payload.Arguments)
				text := normalizeToolEventText(payload.Name, detail)
				if text != "" {
					kind, title := classifyToolCall(payload.Name, detail)
					recentEvents = append(recentEvents, CodexSessionEvent{
						Kind:      kind,
						Title:     title,
						Text:      text,
						Timestamp: ts,
					})
				}
				if payload.CallID != "" {
					if idx, ok := pendingToolOutputIdx[payload.CallID]; ok && idx >= 0 && idx < len(recentEvents) {
						recentEvents[idx].Kind, recentEvents[idx].Title = classifyToolResult(payload.Name, recentEvents[idx].Text)
						delete(pendingToolOutputIdx, payload.CallID)
					}
				}
				continue
			}

			if payload.Type == "function_call_output" {
				text := sanitizeCodexStructuredText(payload.Output)
				if text != "" {
					kind, title := classifyToolResult("", text)
					recentEvents = append(recentEvents, CodexSessionEvent{
						Kind:      kind,
						Title:     title,
						Text:      text,
						Timestamp: ts,
					})
					if payload.CallID != "" {
						pendingToolOutputIdx[payload.CallID] = len(recentEvents) - 1
						seenToolOutputCalls[payload.CallID] = struct{}{}
					}
				}
				continue
			}

			if payload.Type == "message" && payload.Role == "assistant" && result.LastAgentMessage == "" {
				for _, part := range payload.Content {
					if part.Type != "output_text" {
						continue
					}
					if text := normalizeCodexText(part.Text, 150); text != "" {
						result.LastAgentMessage = text
						break
					}
				}
			}

			if payload.Type == "message" && payload.Role == "assistant" {
				fullText := collectCodexAssistantMessage(payload.Content)
				if result.FullAgentMessage == "" && fullText != "" {
					result.FullAgentMessage = fullText
				}
				if fullText != "" {
					recentEvents = append(recentEvents, CodexSessionEvent{
						Kind:      "response",
						Title:     "输出",
						Text:      fullText,
						Timestamp: ts,
					})
				}
			}
		}
	}

	result.RecentEvents = dedupeCodexEvents(recentEvents, 24)

	return result, nil
}

func applyCodexSessionMeta(target *CodexSessionDiscovery, payload json.RawMessage) {
	var meta codexSessionMetaPayload
	if err := json.Unmarshal(payload, &meta); err != nil {
		return
	}

	if target.SessionID == "" {
		target.SessionID = strings.TrimSpace(meta.ID)
	}
	if target.Cwd == "" {
		target.Cwd = strings.TrimSpace(meta.Cwd)
	}
	if target.StartedAt.IsZero() {
		target.StartedAt = parseCodexTimestamp(meta.Timestamp)
	}

	nickname := strings.TrimSpace(meta.AgentNickname)
	if nickname == "" {
		nickname = strings.TrimSpace(meta.Source.Subagent.ThreadSpawn.AgentNickname)
	}

	role := strings.TrimSpace(meta.AgentRole)
	if role == "" {
		role = strings.TrimSpace(meta.Source.Subagent.ThreadSpawn.AgentRole)
	}

	if target.DisplayName == "" {
		target.DisplayName = formatCodexDisplayName(nickname, role)
	}
	if target.AgentRole == "" {
		target.AgentRole = role
	}
}

func formatCodexDisplayName(nickname, role string) string {
	nickname = strings.TrimSpace(nickname)
	role = strings.TrimSpace(role)
	if nickname == "" {
		return ""
	}
	if role == "" {
		return nickname
	}

	lowerNickname := strings.ToLower(nickname)
	lowerRole := strings.ToLower(role)
	if strings.Contains(lowerNickname, "("+lowerRole+")") {
		return nickname
	}

	return nickname + " (" + role + ")"
}

func parseCodexTimestamp(raw string) time.Time {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return time.Time{}
	}
	ts, err := time.Parse(time.RFC3339, raw)
	if err != nil {
		return time.Time{}
	}
	return ts
}

func extractCodexSessionID(path string) string {
	base := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
	return codexSessionIDPattern.FindString(base)
}

func normalizeCodexText(raw string, maxRunes int) string {
	text := sanitizeCodexStructuredText(raw)
	if text == "" {
		return ""
	}
	text = strings.Join(strings.Fields(text), " ")
	if maxRunes <= 0 {
		return text
	}

	runes := []rune(text)
	if len(runes) <= maxRunes {
		return text
	}
	return string(runes[:maxRunes]) + "..."
}

func summarizeCodexToolArguments(arguments string) string {
	arguments = strings.TrimSpace(arguments)
	if arguments == "" {
		return ""
	}

	var payload map[string]interface{}
	if err := json.Unmarshal([]byte(arguments), &payload); err != nil {
		return normalizeCodexText(arguments, 96)
	}
	if len(payload) == 0 {
		return ""
	}

	preferredKeys := []string{
		"cmd", "command", "q", "pattern", "file_path", "path", "ref_id", "workdir", "location", "team",
	}
	for _, key := range preferredKeys {
		if value, ok := payload[key]; ok {
			return normalizeCodexText(fmt.Sprintf("%s=%v", key, value), 96)
		}
	}

	keys := make([]string, 0, len(payload))
	for key := range payload {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	first := keys[0]
	return normalizeCodexText(fmt.Sprintf("%s=%v", first, payload[first]), 96)
}

func collectCodexAssistantMessage(content []codexResponseMessagePart) string {
	parts := make([]string, 0, len(content))
	for _, part := range content {
		if part.Type != "output_text" {
			continue
		}
		text := sanitizeCodexStructuredText(part.Text)
		if text != "" {
			parts = append(parts, text)
		}
	}

	return strings.TrimSpace(strings.Join(parts, "\n\n"))
}

func dedupeCodexEvents(events []CodexSessionEvent, limit int) []CodexSessionEvent {
	if len(events) == 0 || limit <= 0 {
		return nil
	}

	deduped := make([]CodexSessionEvent, 0, minCodex(limit, len(events)))
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

func minCodex(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func sanitizeCodexStructuredText(text string) string {
	text = strings.TrimSpace(strings.ReplaceAll(text, "\r\n", "\n"))
	if text == "" {
		return ""
	}

	lines := strings.Split(text, "\n")
	result := make([]string, 0, len(lines))
	blankCount := 0
	for _, line := range lines {
		cleanLine := strings.TrimRight(line, " \t")
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

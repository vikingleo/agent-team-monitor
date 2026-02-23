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
	StartedAt        time.Time
	LastActiveAt     time.Time
	LastUserMessage  string
	LastAgentMessage string
	LastReasoning    string
	LastToolUse      string
	LastToolDetail   string
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
	Content   []codexResponseMessagePart `json:"content"`
}

type codexEventPayload struct {
	Type    string `json:"type"`
	Message string `json:"message"`
	Text    string `json:"text"`
}

type codexTurnContextPayload struct {
	Cwd string `json:"cwd"`
}

type codexSessionMetaPayload struct {
	ID        string `json:"id"`
	Timestamp string `json:"timestamp"`
	Cwd       string `json:"cwd"`
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
		if session.LastActiveAt.IsZero() {
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

	for k := 0; k < count; k++ {
		idx := (ringIdx - 1 - k) % codexSessionTailLines
		if idx < 0 {
			idx += codexSessionTailLines
		}

		var entry codexLogEntry
		if err := json.Unmarshal([]byte(ring[idx]), &entry); err != nil {
			continue
		}

		if ts := parseCodexTimestamp(entry.Timestamp); !ts.IsZero() {
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
			case "agent_message":
				if result.LastAgentMessage == "" {
					result.LastAgentMessage = normalizeCodexText(payload.Message, 150)
				}
			case "agent_reasoning":
				if result.LastReasoning == "" {
					result.LastReasoning = normalizeCodexText(payload.Text, 150)
				}
			}
		case "response_item":
			var payload codexResponsePayload
			if err := json.Unmarshal(entry.Payload, &payload); err != nil {
				continue
			}

			if payload.Type == "function_call" && result.LastToolUse == "" {
				result.LastToolUse = strings.TrimSpace(payload.Name)
				result.LastToolDetail = summarizeCodexToolArguments(payload.Arguments)
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
		}
	}

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
	text := strings.TrimSpace(raw)
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

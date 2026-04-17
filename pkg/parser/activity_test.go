package parser

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestFindAgentLogFileForMember_MatchesMemberPromptWithSameCwd(t *testing.T) {
	projectsDir := t.TempDir()
	leadSessionID := "lead-session-1"
	cwd := "/workspace/project"
	subagentsDir := filepath.Join(projectsDir, "project-a", leadSessionID, "subagents")

	apiLogPath := filepath.Join(subagentsDir, "agent-a111111.jsonl")
	adminLogPath := filepath.Join(subagentsDir, "agent-a222222.jsonl")

	mustWriteJSONL(t, apiLogPath, []any{
		logEntry("user", "2026-02-10T10:00:00Z", "a111111", leadSessionID, cwd,
			messageString("你是 api-developer，负责后端开发。")),
		logEntry("assistant", "2026-02-10T10:05:00Z", "a111111", leadSessionID, cwd,
			assistantTextMessage("已就绪。")),
	})

	mustWriteJSONL(t, adminLogPath, []any{
		logEntry("user", "2026-02-10T10:01:00Z", "a222222", leadSessionID, cwd,
			messageString("你是 admin-developer，负责管理后台开发。")),
		logEntry("assistant", "2026-02-10T10:06:00Z", "a222222", leadSessionID, cwd,
			assistantTextMessage("已开始熟悉代码。")),
	})

	path, agentID, _, err := FindAgentLogFileForMember(projectsDir, leadSessionID, "admin-developer", cwd, time.Time{})
	if err != nil {
		t.Fatalf("FindAgentLogFileForMember returned error: %v", err)
	}
	if filepath.Base(path) != "agent-a222222.jsonl" {
		t.Fatalf("unexpected log file: got %s", filepath.Base(path))
	}
	if agentID != "a222222" {
		t.Fatalf("unexpected agentID: got %s", agentID)
	}
}

func TestFindAgentLogFileForMember_UsesJoinedAtAndAliasForGeneration(t *testing.T) {
	projectsDir := t.TempDir()
	leadSessionID := "lead-session-2"
	cwd := "/workspace/project"
	subagentsDir := filepath.Join(projectsDir, "project-b", leadSessionID, "subagents")

	oldLogPath := filepath.Join(subagentsDir, "agent-old111.jsonl")
	newNearLogPath := filepath.Join(subagentsDir, "agent-new222.jsonl")
	newFarLogPath := filepath.Join(subagentsDir, "agent-new333.jsonl")

	mustWriteJSONL(t, oldLogPath, []any{
		logEntry("user", "2026-02-10T09:55:00Z", "old111", leadSessionID, cwd,
			messageString("你是 api-developer，继续之前任务。")),
		logEntry("assistant", "2026-02-10T09:58:00Z", "old111", leadSessionID, cwd,
			assistantTextMessage("旧会话完成。")),
	})

	mustWriteJSONL(t, newNearLogPath, []any{
		logEntry("user", "2026-02-11T10:01:00Z", "new222", leadSessionID, cwd,
			messageString("你是 api-developer，新的轮次继续。")),
		logEntry("assistant", "2026-02-11T10:05:00Z", "new222", leadSessionID, cwd,
			assistantTextMessage("新成员接手。")),
	})

	mustWriteJSONL(t, newFarLogPath, []any{
		logEntry("user", "2026-02-11T11:00:00Z", "new333", leadSessionID, cwd,
			messageString("你是 api-developer，另一个重启会话。")),
		logEntry("assistant", "2026-02-11T11:10:00Z", "new333", leadSessionID, cwd,
			assistantTextMessage("后续会话。")),
	})

	joinedAt := time.Date(2026, 2, 11, 10, 0, 0, 0, time.UTC)
	path, agentID, _, err := FindAgentLogFileForMember(projectsDir, leadSessionID, "api-developer-2", cwd, joinedAt)
	if err != nil {
		t.Fatalf("FindAgentLogFileForMember returned error: %v", err)
	}
	if filepath.Base(path) != "agent-new222.jsonl" {
		t.Fatalf("unexpected log file: got %s", filepath.Base(path))
	}
	if agentID != "new222" {
		t.Fatalf("unexpected agentID: got %s", agentID)
	}
}

func TestFindAgentLogFileForMember_HandlesLongJSONLLine(t *testing.T) {
	projectsDir := t.TempDir()
	leadSessionID := "lead-session-3"
	cwd := "/workspace/project"
	subagentsDir := filepath.Join(projectsDir, "project-c", leadSessionID, "subagents")

	longLogPath := filepath.Join(subagentsDir, "agent-long01.jsonl")
	veryLongPrompt := "你是 api-developer，" + strings.Repeat("超长内容", 20000)

	mustWriteJSONL(t, longLogPath, []any{
		logEntry("user", "2026-02-11T09:00:00Z", "long01", leadSessionID, cwd,
			messageString(veryLongPrompt)),
	})

	path, agentID, _, err := FindAgentLogFileForMember(projectsDir, leadSessionID, "api-developer", cwd, time.Time{})
	if err != nil {
		t.Fatalf("FindAgentLogFileForMember returned error: %v", err)
	}
	if filepath.Base(path) != "agent-long01.jsonl" {
		t.Fatalf("unexpected log file: got %s", filepath.Base(path))
	}
	if agentID != "long01" {
		t.Fatalf("unexpected agentID: got %s", agentID)
	}
}

func TestParseAgentActivity_HandlesLongAssistantLine(t *testing.T) {
	logPath := filepath.Join(t.TempDir(), "agent-long-activity.jsonl")
	longThinking := "开始分析：" + strings.Repeat("逻辑检查", 12000)

	mustWriteJSONL(t, logPath, []any{
		logEntry("assistant", "2026-02-11T09:10:00Z", "aa1234", "session-1", "/workspace/project",
			map[string]any{
				"role": "assistant",
				"content": []any{
					map[string]any{"type": "text", "text": longThinking},
					map[string]any{"type": "tool_use", "name": "Read", "input": map[string]any{"file_path": "/tmp/demo.txt"}},
				},
			}),
	})

	activity, err := ParseAgentActivity(logPath)
	if err != nil {
		t.Fatalf("ParseAgentActivity returned error: %v", err)
	}
	if activity == nil {
		t.Fatalf("ParseAgentActivity returned nil activity")
	}
	if activity.LastToolUse != "Read" {
		t.Fatalf("unexpected tool use: got %s", activity.LastToolUse)
	}
	if activity.LastResponse == "" {
		t.Fatalf("expected non-empty response")
	}
	if activity.LastThinking != "" {
		t.Fatalf("expected structured thinking to stay empty when log only contains text")
	}
}

func TestParseAgentActivity_ClassifiesTerminalAndToolResultEvents(t *testing.T) {
	logPath := filepath.Join(t.TempDir(), "agent-terminal-activity.jsonl")

	mustWriteJSONL(t, logPath, []any{
		logEntry("assistant", "2026-04-10T09:10:00Z", "aa1234", "session-1", "/workspace/project",
			map[string]any{
				"role": "assistant",
				"content": []any{
					map[string]any{
						"type": "tool_use",
						"id":   "call-bash-1",
						"name": "Bash",
						"input": map[string]any{
							"command": "git status --short",
						},
					},
				},
			}),
		logEntry("user", "2026-04-10T09:10:01Z", "aa1234", "session-1", "/workspace/project",
			map[string]any{
				"role": "user",
				"content": []any{
					map[string]any{
						"type":        "tool_result",
						"tool_use_id": "call-bash-1",
						"content":     " M web/static/js/app.js",
					},
				},
			}),
		logEntry("assistant", "2026-04-10T09:10:02Z", "aa1234", "session-1", "/workspace/project",
			map[string]any{
				"role": "assistant",
				"content": []any{
					map[string]any{"type": "text", "text": "状态已确认。"},
				},
			}),
	})

	activity, err := ParseAgentActivity(logPath)
	if err != nil {
		t.Fatalf("ParseAgentActivity returned error: %v", err)
	}
	if activity == nil {
		t.Fatalf("ParseAgentActivity returned nil activity")
	}

	foundTerminal := false
	foundTerminalOutput := false
	for _, event := range activity.RecentEvents {
		if event.Kind == "terminal" && strings.Contains(event.Text, "git status --short") {
			foundTerminal = true
		}
		if event.Kind == "terminal_output" && strings.Contains(event.Text, "web/static/js/app.js") {
			foundTerminalOutput = true
		}
	}

	if !foundTerminal {
		t.Fatalf("expected terminal event, got %#v", activity.RecentEvents)
	}
	if !foundTerminalOutput {
		t.Fatalf("expected terminal output event, got %#v", activity.RecentEvents)
	}
}

func TestFindSessionTeamName(t *testing.T) {
	projectsDir := t.TempDir()
	sessionID := "4d6e964f-7291-4c89-9e58-c12dd3ded563"
	logPath := filepath.Join(projectsDir, "project-a", sessionID+".jsonl")

	mustWriteJSONL(t, logPath, []any{
		map[string]any{
			"type":      "assistant",
			"timestamp": "2026-04-08T16:21:52Z",
			"sessionId": sessionID,
			"cwd":       "/home/test",
		},
		map[string]any{
			"type":      "user",
			"timestamp": "2026-04-08T16:21:57Z",
			"sessionId": sessionID,
			"teamName":  "default",
			"cwd":       "/home/test",
		},
	})

	teamName, err := FindSessionTeamName(projectsDir, sessionID)
	if err != nil {
		t.Fatalf("FindSessionTeamName returned error: %v", err)
	}
	if teamName != "default" {
		t.Fatalf("unexpected team name: %q", teamName)
	}
}

func logEntry(recordType, timestamp, agentID, sessionID, cwd string, message any) map[string]any {
	return map[string]any{
		"type":      recordType,
		"timestamp": timestamp,
		"agentId":   agentID,
		"sessionId": sessionID,
		"cwd":       cwd,
		"message":   message,
	}
}

func messageString(content string) map[string]any {
	return map[string]any{
		"role":    "user",
		"content": content,
	}
}

func assistantTextMessage(content string) map[string]any {
	return map[string]any{
		"role": "assistant",
		"content": []any{
			map[string]any{"type": "text", "text": content},
		},
	}
}

func mustWriteJSONL(t *testing.T, path string, records []any) {
	t.Helper()

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("failed to create dir: %v", err)
	}

	file, err := os.Create(path)
	if err != nil {
		t.Fatalf("failed to create file: %v", err)
	}
	defer file.Close()

	encoder := json.NewEncoder(file)
	for _, record := range records {
		if err := encoder.Encode(record); err != nil {
			t.Fatalf("failed to encode record: %v", err)
		}
	}
}

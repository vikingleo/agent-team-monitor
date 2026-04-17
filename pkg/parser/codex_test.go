package parser

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestDiscoverCodexSessions(t *testing.T) {
	root := t.TempDir()
	sessionsDir := filepath.Join(root, "sessions", "2026", "02", "23")
	if err := os.MkdirAll(sessionsDir, 0755); err != nil {
		t.Fatalf("mkdir failed: %v", err)
	}

	sessionID := "019c8b52-8e3f-7ff0-a072-2ab2e598b686"
	logPath := filepath.Join(sessionsDir, "rollout-2026-02-23T16-26-09-"+sessionID+".jsonl")
	content := "" +
		`{"timestamp":"2026-02-23T16:26:09.347Z","type":"session_meta","payload":{"id":"019c8b52-8e3f-7ff0-a072-2ab2e598b686","timestamp":"2026-02-23T16:26:09.343Z","cwd":"/home/test/work/demo"}}` + "\n" +
		`{"timestamp":"2026-02-23T16:26:10.000Z","type":"event_msg","payload":{"type":"user_message","message":"请实现 codex 监控"}}` + "\n" +
		`{"timestamp":"2026-02-23T16:26:12.000Z","type":"event_msg","payload":{"type":"agent_reasoning","text":"**Planning implementation**"}}` + "\n" +
		`{"timestamp":"2026-02-23T16:26:13.000Z","type":"response_item","payload":{"type":"function_call","name":"exec_command","arguments":"{\"cmd\":\"rg --files\"}","call_id":"call-1"}}` + "\n" +
		`{"timestamp":"2026-02-23T16:26:14.000Z","type":"response_item","payload":{"type":"function_call_output","call_id":"call-1","output":"Command: /usr/bin/zsh -lc 'rg --files'\nOutput:\nREADME.md\npkg/parser/codex.go"}}` + "\n" +
		`{"timestamp":"2026-02-23T16:26:15.000Z","type":"event_msg","payload":{"type":"agent_message","message":"我会先读取项目结构。"}}` + "\n"

	if err := os.WriteFile(logPath, []byte(content), 0644); err != nil {
		t.Fatalf("write log failed: %v", err)
	}
	now := time.Now()
	if err := os.Chtimes(logPath, now, now); err != nil {
		t.Fatalf("Chtimes failed: %v", err)
	}

	sessions, err := DiscoverCodexSessions(filepath.Join(root, "sessions"), 24*time.Hour)
	if err != nil {
		t.Fatalf("DiscoverCodexSessions error: %v", err)
	}
	if len(sessions) != 1 {
		t.Fatalf("expected 1 session, got %d", len(sessions))
	}

	session := sessions[0]
	if session.SessionID != sessionID {
		t.Fatalf("unexpected session id: %s", session.SessionID)
	}
	if session.Cwd != "/home/test/work/demo" {
		t.Fatalf("unexpected cwd: %s", session.Cwd)
	}
	if session.LastToolUse != "exec_command" {
		t.Fatalf("unexpected last tool use: %s", session.LastToolUse)
	}
	if session.LastToolDetail == "" {
		t.Fatal("expected non-empty tool detail")
	}
	if session.LastReasoning == "" {
		t.Fatal("expected non-empty last reasoning")
	}
	if session.LastAgentMessage == "" {
		t.Fatal("expected non-empty last agent message")
	}
	if session.LastUserMessage == "" {
		t.Fatal("expected non-empty last user message")
	}
	if session.LastActiveAt.IsZero() {
		t.Fatal("expected non-zero last active time")
	}
	foundTerminal := false
	foundTerminalOutput := false
	for _, event := range session.RecentEvents {
		if event.Kind == "terminal" && event.Title == "终端命令" {
			foundTerminal = true
		}
		if event.Kind == "terminal_output" && event.Title == "终端输出" {
			foundTerminalOutput = true
		}
	}
	if !foundTerminal {
		t.Fatalf("expected terminal event, got %#v", session.RecentEvents)
	}
	if !foundTerminalOutput {
		t.Fatalf("expected terminal output event, got %#v", session.RecentEvents)
	}
}

func TestDiscoverCodexSessions_ExtractsSubagentDisplayNameFromSessionMeta(t *testing.T) {
	root := t.TempDir()
	sessionsDir := filepath.Join(root, "sessions", "2026", "03", "22")
	if err := os.MkdirAll(sessionsDir, 0755); err != nil {
		t.Fatalf("mkdir failed: %v", err)
	}

	sessionID := "019d11a7-145a-7be3-bf0d-8939469bc2a2"
	logPath := filepath.Join(sessionsDir, "rollout-2026-03-22T02-27-35-"+sessionID+".jsonl")
	content := "" +
		`{"timestamp":"2026-03-21T18:27:35.654Z","type":"session_meta","payload":{"id":"019d11a7-145a-7be3-bf0d-8939469bc2a2","forked_from_id":"019d11a5-206e-7d01-a409-3a117be44622","timestamp":"2026-03-21T18:27:35.645Z","cwd":"/home/test/work/demo","source":{"subagent":{"thread_spawn":{"parent_thread_id":"019d11a5-206e-7d01-a409-3a117be44622","depth":1,"agent_nickname":"Gauss","agent_role":"explorer"}}},"agent_nickname":"Gauss","agent_role":"explorer"}}` + "\n" +
		`{"timestamp":"2026-03-21T18:27:36.000Z","type":"event_msg","payload":{"type":"user_message","message":"请排查 editor 页面问题"}}` + "\n" +
		`{"timestamp":"2026-03-21T18:27:37.000Z","type":"event_msg","payload":{"type":"agent_message","message":"我先快速扫一遍相关文件。"}}` + "\n"

	if err := os.WriteFile(logPath, []byte(content), 0644); err != nil {
		t.Fatalf("write log failed: %v", err)
	}
	now := time.Now()
	if err := os.Chtimes(logPath, now, now); err != nil {
		t.Fatalf("Chtimes failed: %v", err)
	}

	sessions, err := DiscoverCodexSessions(filepath.Join(root, "sessions"), 24*time.Hour)
	if err != nil {
		t.Fatalf("DiscoverCodexSessions error: %v", err)
	}
	if len(sessions) != 1 {
		t.Fatalf("expected 1 session, got %d", len(sessions))
	}

	session := sessions[0]
	if session.DisplayName != "Gauss (explorer)" {
		t.Fatalf("unexpected display name: %s", session.DisplayName)
	}
	if session.AgentRole != "explorer" {
		t.Fatalf("unexpected agent role: %s", session.AgentRole)
	}
}

func TestDiscoverCodexSessions_FiltersByAge(t *testing.T) {
	root := t.TempDir()
	sessionsDir := filepath.Join(root, "sessions", "2026", "02", "01")
	if err := os.MkdirAll(sessionsDir, 0755); err != nil {
		t.Fatalf("mkdir failed: %v", err)
	}

	logPath := filepath.Join(sessionsDir, "rollout-2026-02-01T16-26-09-019c8b52-8e3f-7ff0-a072-2ab2e598b686.jsonl")
	if err := os.WriteFile(logPath, []byte(`{"timestamp":"2026-02-01T16:26:09.347Z","type":"session_meta","payload":{"id":"019c8b52-8e3f-7ff0-a072-2ab2e598b686","cwd":"/tmp/demo"}}`+"\n"), 0644); err != nil {
		t.Fatalf("write log failed: %v", err)
	}

	old := time.Now().Add(-48 * time.Hour)
	if err := os.Chtimes(logPath, old, old); err != nil {
		t.Fatalf("Chtimes failed: %v", err)
	}

	sessions, err := DiscoverCodexSessions(filepath.Join(root, "sessions"), 2*time.Hour)
	if err != nil {
		t.Fatalf("DiscoverCodexSessions error: %v", err)
	}
	if len(sessions) != 0 {
		t.Fatalf("expected 0 sessions after age filter, got %d", len(sessions))
	}
}

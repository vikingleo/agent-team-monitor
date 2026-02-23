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
		`{"timestamp":"2026-02-23T16:26:13.000Z","type":"response_item","payload":{"type":"function_call","name":"exec_command","arguments":"{\"cmd\":\"rg --files\"}"}}` + "\n" +
		`{"timestamp":"2026-02-23T16:26:15.000Z","type":"event_msg","payload":{"type":"agent_message","message":"我会先读取项目结构。"}}` + "\n"

	if err := os.WriteFile(logPath, []byte(content), 0644); err != nil {
		t.Fatalf("write log failed: %v", err)
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

package parser

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestDiscoverOpenClawSessions(t *testing.T) {
	root := t.TempDir()
	sessionsDir := filepath.Join(root, "agents", "writer", "sessions")
	if err := os.MkdirAll(sessionsDir, 0755); err != nil {
		t.Fatalf("mkdir failed: %v", err)
	}

	sessionID := "717ebe9d-8ab7-4853-b3bc-61d191c8424a"
	sessionFile := filepath.Join(sessionsDir, sessionID+".jsonl")
	store := `{
  "agent:writer:main": {
    "sessionId": "` + sessionID + `",
    "updatedAt": 1774177680000,
    "sessionFile": "` + sessionID + `.jsonl",
    "label": "写文案任务",
    "displayName": "writer-main",
    "spawnedWorkspaceDir": "/home/test/work/docs"
  }
}`
	if err := os.WriteFile(filepath.Join(sessionsDir, "sessions.json"), []byte(store), 0644); err != nil {
		t.Fatalf("write store failed: %v", err)
	}

	content := "" +
		`{"timestamp":"2026-03-21T10:25:00.000Z","message":{"role":"user","content":"请生成面板文案说明"}}` + "\n" +
		`{"timestamp":"2026-03-21T10:25:05.000Z","message":{"role":"assistant","content":[{"type":"thinking","text":"先整理结构，再输出完整说明。"},{"type":"toolCall","name":"read_file","arguments":{"path":"README.md"}},{"type":"text","text":"我已经整理好一版完整说明。"}]}}` + "\n" +
		`{"timestamp":"2026-03-21T10:25:06.000Z","message":{"role":"toolResult","content":{"text":"README.md 已读取"}}}` + "\n"
	if err := os.WriteFile(sessionFile, []byte(content), 0644); err != nil {
		t.Fatalf("write transcript failed: %v", err)
	}

	now := time.Now()
	if err := os.Chtimes(sessionFile, now, now); err != nil {
		t.Fatalf("chtimes failed: %v", err)
	}

	discovered, err := DiscoverOpenClawSessions(filepath.Join(root, "agents"), 24*time.Hour)
	if err != nil {
		t.Fatalf("DiscoverOpenClawSessions error: %v", err)
	}
	if len(discovered) != 1 {
		t.Fatalf("expected 1 session, got %d", len(discovered))
	}

	session := discovered[0]
	if session.AgentID != "writer" {
		t.Fatalf("unexpected agent id: %s", session.AgentID)
	}
	if session.SessionKey != "agent:writer:main" {
		t.Fatalf("unexpected session key: %s", session.SessionKey)
	}
	if session.SessionID != sessionID {
		t.Fatalf("unexpected session id: %s", session.SessionID)
	}
	if session.Cwd != "/home/test/work/docs" {
		t.Fatalf("unexpected cwd: %s", session.Cwd)
	}
	if session.LastUserMessage == "" {
		t.Fatal("expected last user message")
	}
	if session.LastAgentMessage == "" {
		t.Fatal("expected last agent message")
	}
	if session.FullAgentMessage == "" {
		t.Fatal("expected full agent message")
	}
	if session.LastReasoning == "" {
		t.Fatal("expected reasoning")
	}
	if session.LastToolUse != "read_file" {
		t.Fatalf("unexpected tool use: %s", session.LastToolUse)
	}
	if len(session.RecentEvents) == 0 {
		t.Fatal("expected recent events")
	}
}

func TestDiscoverOpenClawSessions_FiltersByAge(t *testing.T) {
	root := t.TempDir()
	sessionsDir := filepath.Join(root, "agents", "main", "sessions")
	if err := os.MkdirAll(sessionsDir, 0755); err != nil {
		t.Fatalf("mkdir failed: %v", err)
	}

	sessionID := "11111111-2222-3333-4444-555555555555"
	if err := os.WriteFile(filepath.Join(sessionsDir, "sessions.json"), []byte(`{"agent:main:main":{"sessionId":"`+sessionID+`","updatedAt":1,"sessionFile":"`+sessionID+`.jsonl"}}`), 0644); err != nil {
		t.Fatalf("write store failed: %v", err)
	}
	if err := os.WriteFile(filepath.Join(sessionsDir, sessionID+".jsonl"), []byte(`{"timestamp":"2026-03-01T10:00:00.000Z","message":{"role":"user","content":"old"}}`+"\n"), 0644); err != nil {
		t.Fatalf("write transcript failed: %v", err)
	}

	old := time.Now().Add(-48 * time.Hour)
	if err := os.Chtimes(filepath.Join(sessionsDir, sessionID+".jsonl"), old, old); err != nil {
		t.Fatalf("chtimes failed: %v", err)
	}

	discovered, err := DiscoverOpenClawSessions(filepath.Join(root, "agents"), 2*time.Hour)
	if err != nil {
		t.Fatalf("DiscoverOpenClawSessions error: %v", err)
	}
	if len(discovered) != 0 {
		t.Fatalf("expected 0 sessions, got %d", len(discovered))
	}
}

func TestDiscoverOpenClawSessions_FallsBackWhenIndexedAbsolutePathIsStale(t *testing.T) {
	root := t.TempDir()
	sessionsDir := filepath.Join(root, "agents", "coder", "sessions")
	if err := os.MkdirAll(sessionsDir, 0755); err != nil {
		t.Fatalf("mkdir failed: %v", err)
	}

	sessionID := "ddb31b0f-1e86-40df-a2f7-42406a8d427f"
	sessionFile := filepath.Join(sessionsDir, sessionID+".jsonl")
	store := `{
  "agent:coder:main": {
    "sessionId": "` + sessionID + `",
    "updatedAt": 1774254300000,
    "sessionFile": "/home/legacy/.openclaw/agents/coder/sessions/` + sessionID + `.jsonl",
    "label": "补充实现",
    "displayName": "coder-main"
  }
}`
	if err := os.WriteFile(filepath.Join(sessionsDir, "sessions.json"), []byte(store), 0644); err != nil {
		t.Fatalf("write store failed: %v", err)
	}

	content := "" +
		`{"timestamp":"2026-03-22T08:10:00.000Z","message":{"role":"user","content":"继续补 OpenClaw 子 agent 监控"}}` + "\n" +
		`{"timestamp":"2026-03-22T08:10:05.000Z","message":{"role":"assistant","content":[{"type":"toolCall","name":"read_file","arguments":{"path":"pkg/parser/openclaw.go"}},{"type":"text","text":"我已经定位到子 agent 丢失的原因。"}]}}` + "\n"
	if err := os.WriteFile(sessionFile, []byte(content), 0644); err != nil {
		t.Fatalf("write transcript failed: %v", err)
	}

	now := time.Now()
	if err := os.Chtimes(sessionFile, now, now); err != nil {
		t.Fatalf("chtimes failed: %v", err)
	}

	discovered, err := DiscoverOpenClawSessions(filepath.Join(root, "agents"), 24*time.Hour)
	if err != nil {
		t.Fatalf("DiscoverOpenClawSessions error: %v", err)
	}
	if len(discovered) != 1 {
		t.Fatalf("expected 1 session, got %d", len(discovered))
	}

	if discovered[0].SessionPath != sessionFile {
		t.Fatalf("expected fallback session path %s, got %s", sessionFile, discovered[0].SessionPath)
	}
	if discovered[0].LastAgentMessage == "" {
		t.Fatal("expected parsed assistant message")
	}
}

func TestDiscoverOpenClawSessions_DiscoversJsonlWithoutIndex(t *testing.T) {
	root := t.TempDir()
	sessionsDir := filepath.Join(root, "agents", "writer", "sessions")
	if err := os.MkdirAll(sessionsDir, 0755); err != nil {
		t.Fatalf("mkdir failed: %v", err)
	}

	sessionID := "bba7cd05-63ab-4584-b272-50b08ef91aa1"
	sessionFile := filepath.Join(sessionsDir, sessionID+".jsonl")
	content := "" +
		`{"timestamp":"2026-03-22T09:20:00.000Z","message":{"role":"user","content":"整理本轮总结"}}` + "\n" +
		`{"timestamp":"2026-03-22T09:20:06.000Z","message":{"role":"assistant","content":[{"type":"thinking","text":"先列要点，再补风险。"},{"type":"text","text":"本轮总结已经整理完成。"}]}}` + "\n"
	if err := os.WriteFile(sessionFile, []byte(content), 0644); err != nil {
		t.Fatalf("write transcript failed: %v", err)
	}

	now := time.Now()
	if err := os.Chtimes(sessionFile, now, now); err != nil {
		t.Fatalf("chtimes failed: %v", err)
	}

	discovered, err := DiscoverOpenClawSessions(filepath.Join(root, "agents"), 24*time.Hour)
	if err != nil {
		t.Fatalf("DiscoverOpenClawSessions error: %v", err)
	}
	if len(discovered) != 1 {
		t.Fatalf("expected 1 session, got %d", len(discovered))
	}

	session := discovered[0]
	if session.AgentID != "writer" {
		t.Fatalf("unexpected agent id: %s", session.AgentID)
	}
	if session.SessionID != sessionID {
		t.Fatalf("unexpected session id: %s", session.SessionID)
	}
	if session.SessionPath != sessionFile {
		t.Fatalf("unexpected session path: %s", session.SessionPath)
	}
	if session.LastReasoning == "" {
		t.Fatal("expected reasoning from jsonl-only discovery")
	}
	if session.SessionKey == "" {
		t.Fatal("expected synthesized session key")
	}
}

func TestDiscoverOpenClawSubagentRuns(t *testing.T) {
	root := t.TempDir()
	subagentsDir := filepath.Join(root, "subagents")
	if err := os.MkdirAll(subagentsDir, 0755); err != nil {
		t.Fatalf("mkdir failed: %v", err)
	}

	now := time.Now()
	runsJSON := fmt.Sprintf(`{
  "version": 2,
  "runs": {
    "run-123": {
      "runId": "run-123",
      "childSessionKey": "agent:coder:subagent:leaf-1",
      "requesterSessionKey": "agent:main:main",
      "controllerSessionKey": "agent:main:subagent:orch-1",
      "task": "修复 openclaw 子 agent 监控",
      "label": "coder-leaf",
      "workspaceDir": "/home/test/work/project",
      "createdAt": %d,
      "startedAt": %d,
      "spawnMode": "run"
    }
  }
}`, now.Add(-2*time.Minute).UnixMilli(), now.Add(-1*time.Minute).UnixMilli())
	if err := os.WriteFile(filepath.Join(subagentsDir, "runs.json"), []byte(runsJSON), 0644); err != nil {
		t.Fatalf("write runs.json failed: %v", err)
	}

	runs, err := DiscoverOpenClawSubagentRuns(root, 24*time.Hour)
	if err != nil {
		t.Fatalf("DiscoverOpenClawSubagentRuns error: %v", err)
	}
	if len(runs) != 1 {
		t.Fatalf("expected 1 run, got %d", len(runs))
	}

	run := runs[0]
	if run.RunID != "run-123" {
		t.Fatalf("unexpected run id: %s", run.RunID)
	}
	if run.ChildSessionKey != "agent:coder:subagent:leaf-1" {
		t.Fatalf("unexpected child session key: %s", run.ChildSessionKey)
	}
	if run.Label != "coder-leaf" {
		t.Fatalf("unexpected label: %s", run.Label)
	}
	if run.WorkspaceDir != "/home/test/work/project" {
		t.Fatalf("unexpected workspaceDir: %s", run.WorkspaceDir)
	}
}

package parser

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestDiscoverClaudeSessions(t *testing.T) {
	root := t.TempDir()
	sessionsDir := filepath.Join(root, "sessions")
	if err := os.MkdirAll(sessionsDir, 0o755); err != nil {
		t.Fatalf("mkdir failed: %v", err)
	}

	sessionPath := filepath.Join(sessionsDir, "10217.json")
	content := `{"pid":10217,"sessionId":"9a88def1-6ad3-4d32-a378-1dd0979cfd49","cwd":"/home/test/work/demo","startedAt":1775665224316,"kind":"interactive","entrypoint":"cli"}`
	if err := os.WriteFile(sessionPath, []byte(content), 0o644); err != nil {
		t.Fatalf("write session failed: %v", err)
	}

	now := time.Now()
	if err := os.Chtimes(sessionPath, now, now); err != nil {
		t.Fatalf("chtimes failed: %v", err)
	}

	sessions, err := DiscoverClaudeSessions(sessionsDir, 24*time.Hour)
	if err != nil {
		t.Fatalf("DiscoverClaudeSessions error: %v", err)
	}
	if len(sessions) != 1 {
		t.Fatalf("expected 1 session, got %d", len(sessions))
	}

	session := sessions[0]
	if session.PID != 10217 {
		t.Fatalf("unexpected pid: %d", session.PID)
	}
	if session.SessionID != "9a88def1-6ad3-4d32-a378-1dd0979cfd49" {
		t.Fatalf("unexpected session id: %s", session.SessionID)
	}
	if session.Cwd != "/home/test/work/demo" {
		t.Fatalf("unexpected cwd: %s", session.Cwd)
	}
	if session.Kind != "interactive" {
		t.Fatalf("unexpected kind: %s", session.Kind)
	}
	if session.Entrypoint != "cli" {
		t.Fatalf("unexpected entrypoint: %s", session.Entrypoint)
	}
	if session.StartedAt.IsZero() {
		t.Fatal("expected non-zero startedAt")
	}
	if session.LastSeenAt.IsZero() {
		t.Fatal("expected non-zero lastSeenAt")
	}
}

func TestDiscoverClaudeSessions_FiltersByAge(t *testing.T) {
	root := t.TempDir()
	sessionsDir := filepath.Join(root, "sessions")
	if err := os.MkdirAll(sessionsDir, 0o755); err != nil {
		t.Fatalf("mkdir failed: %v", err)
	}

	sessionPath := filepath.Join(sessionsDir, "99999.json")
	content := `{"pid":99999,"sessionId":"aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee","cwd":"/tmp/demo","startedAt":1,"kind":"interactive","entrypoint":"cli"}`
	if err := os.WriteFile(sessionPath, []byte(content), 0o644); err != nil {
		t.Fatalf("write session failed: %v", err)
	}

	old := time.Now().Add(-48 * time.Hour)
	if err := os.Chtimes(sessionPath, old, old); err != nil {
		t.Fatalf("chtimes failed: %v", err)
	}

	sessions, err := DiscoverClaudeSessions(sessionsDir, 2*time.Hour)
	if err != nil {
		t.Fatalf("DiscoverClaudeSessions error: %v", err)
	}
	if len(sessions) != 0 {
		t.Fatalf("expected 0 sessions after age filter, got %d", len(sessions))
	}
}

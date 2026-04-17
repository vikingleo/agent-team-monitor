package monitor

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestCollectClaudeTeams_IncludesStandaloneSessionFromClaudeSessions(t *testing.T) {
	root := t.TempDir()
	sessionID := "9a88def1-6ad3-4d32-a378-1dd0979cfd49"

	sessionsDir := filepath.Join(root, ".claude", "sessions")
	projectsDir := filepath.Join(root, ".claude", "projects", "-home-test-work-demo")
	if err := os.MkdirAll(sessionsDir, 0o755); err != nil {
		t.Fatalf("mkdir sessions failed: %v", err)
	}
	if err := os.MkdirAll(projectsDir, 0o755); err != nil {
		t.Fatalf("mkdir projects failed: %v", err)
	}

	sessionJSON := `{"pid":10217,"sessionId":"` + sessionID + `","cwd":"/home/test/work/demo","startedAt":` + "1775665224316" + `,"kind":"interactive","entrypoint":"cli"}`
	if err := os.WriteFile(filepath.Join(sessionsDir, "10217.json"), []byte(sessionJSON), 0o644); err != nil {
		t.Fatalf("write session failed: %v", err)
	}

	logPath := filepath.Join(projectsDir, sessionID+".jsonl")
	now := time.Now()
	parserRecords := []any{
		map[string]any{
			"type":      "assistant",
			"timestamp": now.Add(-30 * time.Second).UTC().Format(time.RFC3339),
			"agentId":   "lead-agent",
			"sessionId": sessionID,
			"cwd":       "/home/test/work/demo",
			"message": map[string]any{
				"role": "assistant",
				"content": []any{
					map[string]any{"type": "text", "text": "我先检查当前 Claude CLI 会话。"},
				},
			},
		},
	}
	mustWriteJSONL(t, logPath, parserRecords)

	if err := os.Chtimes(logPath, now, now); err != nil {
		t.Fatalf("chtimes failed: %v", err)
	}

	collector := &Collector{}
	teams := collector.collectClaudeTeams(root)
	if len(teams) != 1 {
		t.Fatalf("expected 1 team, got %d", len(teams))
	}

	team := teams[0]
	if team.Name != "claude-demo" {
		t.Fatalf("unexpected team name: %s", team.Name)
	}
	if len(team.Members) != 1 {
		t.Fatalf("expected 1 member, got %d", len(team.Members))
	}

	member := team.Members[0]
	if member.Name != "team-lead" {
		t.Fatalf("unexpected member name: %s", member.Name)
	}
	if member.LatestResponse == "" {
		t.Fatal("expected lead session activity to populate latest response")
	}
	if member.LastActiveTime.IsZero() {
		t.Fatal("expected non-zero last active time")
	}
}

func mustWriteJSONL(t *testing.T, path string, records []any) {
	t.Helper()

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir failed: %v", err)
	}

	file, err := os.Create(path)
	if err != nil {
		t.Fatalf("create file failed: %v", err)
	}
	defer file.Close()

	encoder := json.NewEncoder(file)
	for _, record := range records {
		if err := encoder.Encode(record); err != nil {
			t.Fatalf("encode record failed: %v", err)
		}
	}
}

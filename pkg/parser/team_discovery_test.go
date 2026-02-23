package parser

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/liaoweijun/agent-team-monitor/pkg/types"
)

func TestDiscoverProjectTeams_FindsSubagentTeamWithoutConfig(t *testing.T) {
	projectsDir := t.TempDir()
	sessionID := "9636696a-c575-48e5-89d0-5cc5afb2278c"
	projectDir := filepath.Join(projectsDir, "-home-test-works-wog")
	rootLogPath := filepath.Join(projectDir, sessionID+".jsonl")
	subagentLogPath := filepath.Join(projectDir, sessionID, "subagents", "agent-a400c13.jsonl")

	writeDiscoveryJSONL(t, rootLogPath, []map[string]any{
		{
			"type":      "user",
			"timestamp": "2026-02-22T16:14:20Z",
			"sessionId": sessionID,
			"cwd":       "/home/test/works/wog",
			"message": map[string]any{
				"role":    "user",
				"content": "启动 agent team 继续任务",
			},
		},
	})

	writeDiscoveryJSONL(t, subagentLogPath, []map[string]any{
		{
			"type":      "user",
			"timestamp": "2026-02-22T18:45:47Z",
			"sessionId": sessionID,
			"agentId":   "a400c13",
			"cwd":       "/home/test/works/wog",
			"message": map[string]any{
				"role":    "user",
				"content": "你是 WOG 后台管理开发者，负责修复高危 Bug。",
			},
		},
	})

	discovered, err := DiscoverProjectTeams(projectsDir, 24*time.Hour)
	if err != nil {
		t.Fatalf("DiscoverProjectTeams returned error: %v", err)
	}
	if len(discovered) != 1 {
		t.Fatalf("expected 1 discovered team, got %d", len(discovered))
	}

	team := discovered[0]
	if team.LeadSessionID != sessionID {
		t.Fatalf("unexpected session id: got %s", team.LeadSessionID)
	}
	if team.ProjectCwd != "/home/test/works/wog" {
		t.Fatalf("unexpected project cwd: got %s", team.ProjectCwd)
	}
	if len(team.Members) != 1 {
		t.Fatalf("expected 1 member, got %d", len(team.Members))
	}
	if team.Members[0].Name != "WOG 后台管理开发者" {
		t.Fatalf("unexpected member name: got %s", team.Members[0].Name)
	}
	if team.Members[0].AgentID != "a400c13" {
		t.Fatalf("unexpected member agent id: got %s", team.Members[0].AgentID)
	}
}

func TestDiscoverProjectTeams_AppliesAgeFilter(t *testing.T) {
	projectsDir := t.TempDir()
	sessionID := "11111111-2222-3333-4444-555555555555"
	projectDir := filepath.Join(projectsDir, "-home-test-works-old")
	rootLogPath := filepath.Join(projectDir, sessionID+".jsonl")
	subagentLogPath := filepath.Join(projectDir, sessionID, "subagents", "agent-a111111.jsonl")

	writeDiscoveryJSONL(t, rootLogPath, []map[string]any{
		{
			"type":      "user",
			"timestamp": "2026-02-20T16:14:20Z",
			"sessionId": sessionID,
			"cwd":       "/home/test/works/old",
			"message":   map[string]any{"role": "user", "content": "old"},
		},
	})
	writeDiscoveryJSONL(t, subagentLogPath, []map[string]any{
		{
			"type":      "user",
			"timestamp": "2026-02-20T16:20:00Z",
			"sessionId": sessionID,
			"agentId":   "a111111",
			"cwd":       "/home/test/works/old",
			"message":   map[string]any{"role": "user", "content": "你是 old-agent"},
		},
	})

	oldTime := time.Now().Add(-48 * time.Hour)
	if err := os.Chtimes(rootLogPath, oldTime, oldTime); err != nil {
		t.Fatalf("failed to set root log mtime: %v", err)
	}
	if err := os.Chtimes(subagentLogPath, oldTime, oldTime); err != nil {
		t.Fatalf("failed to set subagent log mtime: %v", err)
	}

	discovered, err := DiscoverProjectTeams(projectsDir, 30*time.Minute)
	if err != nil {
		t.Fatalf("DiscoverProjectTeams returned error: %v", err)
	}
	if len(discovered) != 0 {
		t.Fatalf("expected 0 discovered teams with age filter, got %d", len(discovered))
	}
}

func TestDiscoverProjectTeams_DeduplicatesAndCompactsRecentMembers(t *testing.T) {
	projectsDir := t.TempDir()
	sessionID := "22222222-3333-4444-5555-666666666666"
	projectDir := filepath.Join(projectsDir, "-home-test-works-demo")
	rootLogPath := filepath.Join(projectDir, sessionID+".jsonl")

	writeDiscoveryJSONL(t, rootLogPath, []map[string]any{
		{
			"type":      "user",
			"timestamp": "2026-02-22T16:14:20Z",
			"sessionId": sessionID,
			"cwd":       "/home/test/works/demo",
			"message":   map[string]any{"role": "user", "content": "启动团队"},
		},
	})

	apiOlder := filepath.Join(projectDir, sessionID, "subagents", "agent-a100001.jsonl")
	apiNewer := filepath.Join(projectDir, sessionID, "subagents", "agent-a100002.jsonl")
	adminNew := filepath.Join(projectDir, sessionID, "subagents", "agent-a200001.jsonl")
	fallbackNew := filepath.Join(projectDir, sessionID, "subagents", "agent-a300001.jsonl")
	oldMember := filepath.Join(projectDir, sessionID, "subagents", "agent-a400001.jsonl")

	writeDiscoveryJSONL(t, apiOlder, []map[string]any{
		{
			"type":      "user",
			"timestamp": "2026-02-22T17:30:00Z",
			"sessionId": sessionID,
			"agentId":   "a100001",
			"cwd":       "/home/test/works/demo",
			"message":   map[string]any{"role": "user", "content": "你是 api-developer"},
		},
	})
	writeDiscoveryJSONL(t, apiNewer, []map[string]any{
		{
			"type":      "user",
			"timestamp": "2026-02-22T17:58:00Z",
			"sessionId": sessionID,
			"agentId":   "a100002",
			"cwd":       "/home/test/works/demo",
			"message":   map[string]any{"role": "user", "content": "你是 api-developer"},
		},
	})
	writeDiscoveryJSONL(t, adminNew, []map[string]any{
		{
			"type":      "user",
			"timestamp": "2026-02-22T17:59:00Z",
			"sessionId": sessionID,
			"agentId":   "a200001",
			"cwd":       "/home/test/works/demo",
			"message":   map[string]any{"role": "user", "content": "you are admin-developer"},
		},
	})
	writeDiscoveryJSONL(t, fallbackNew, []map[string]any{
		{
			"type":      "assistant",
			"timestamp": "2026-02-22T17:59:30Z",
			"sessionId": sessionID,
			"agentId":   "a300001",
			"cwd":       "/home/test/works/demo",
			"message": map[string]any{
				"role": "assistant",
				"content": []map[string]any{
					{"type": "text", "text": "无明确身份信息"},
				},
			},
		},
	})
	writeDiscoveryJSONL(t, oldMember, []map[string]any{
		{
			"type":      "user",
			"timestamp": "2026-02-22T15:00:00Z",
			"sessionId": sessionID,
			"agentId":   "a400001",
			"cwd":       "/home/test/works/demo",
			"message":   map[string]any{"role": "user", "content": "you are uniapp-developer"},
		},
	})

	now := time.Now()
	if err := os.Chtimes(rootLogPath, now, now); err != nil {
		t.Fatalf("failed to set root mtime: %v", err)
	}
	if err := os.Chtimes(apiOlder, now.Add(-30*time.Minute), now.Add(-30*time.Minute)); err != nil {
		t.Fatalf("failed to set api older mtime: %v", err)
	}
	if err := os.Chtimes(apiNewer, now.Add(-5*time.Minute), now.Add(-5*time.Minute)); err != nil {
		t.Fatalf("failed to set api newer mtime: %v", err)
	}
	if err := os.Chtimes(adminNew, now.Add(-2*time.Minute), now.Add(-2*time.Minute)); err != nil {
		t.Fatalf("failed to set admin mtime: %v", err)
	}
	if err := os.Chtimes(fallbackNew, now.Add(-time.Minute), now.Add(-time.Minute)); err != nil {
		t.Fatalf("failed to set fallback mtime: %v", err)
	}
	if err := os.Chtimes(oldMember, now.Add(-2*time.Hour), now.Add(-2*time.Hour)); err != nil {
		t.Fatalf("failed to set old member mtime: %v", err)
	}

	discovered, err := DiscoverProjectTeams(projectsDir, 24*time.Hour)
	if err != nil {
		t.Fatalf("DiscoverProjectTeams returned error: %v", err)
	}
	if len(discovered) != 1 {
		t.Fatalf("expected 1 discovered team, got %d", len(discovered))
	}

	members := discovered[0].Members
	if len(members) != 2 {
		t.Fatalf("expected 2 compacted members, got %d", len(members))
	}

	memberByName := make(map[string]types.AgentInfo)
	for _, member := range members {
		memberByName[member.Name] = member
	}

	apiMember, ok := memberByName["api-developer"]
	if !ok {
		t.Fatalf("expected api-developer to exist")
	}
	if apiMember.AgentID != "a100002" {
		t.Fatalf("expected latest api-developer agentID a100002, got %s", apiMember.AgentID)
	}

	if _, ok := memberByName["admin-developer"]; !ok {
		t.Fatalf("expected admin-developer to exist")
	}
	for _, member := range members {
		if len(member.Name) >= 6 && member.Name[:6] == "agent-" {
			t.Fatalf("fallback member should be filtered when named members exist, got %s", member.Name)
		}
	}
}

func TestDiscoverProjectTeams_UpdatesMetricsAndCacheHits(t *testing.T) {
	projectsDir := t.TempDir()
	sessionID := "33333333-4444-5555-6666-777777777777"
	projectDir := filepath.Join(projectsDir, "-home-test-works-metrics")
	rootLogPath := filepath.Join(projectDir, sessionID+".jsonl")
	subagentLogPath := filepath.Join(projectDir, sessionID, "subagents", "agent-a777777.jsonl")

	writeDiscoveryJSONL(t, rootLogPath, []map[string]any{
		{
			"type":      "user",
			"timestamp": "2026-02-22T16:14:20Z",
			"sessionId": sessionID,
			"cwd":       "/home/test/works/metrics",
			"message":   map[string]any{"role": "user", "content": "启动 metrics 团队"},
		},
	})
	writeDiscoveryJSONL(t, subagentLogPath, []map[string]any{
		{
			"type":      "user",
			"timestamp": "2026-02-22T16:15:00Z",
			"sessionId": sessionID,
			"agentId":   "a777777",
			"cwd":       "/home/test/works/metrics",
			"message":   map[string]any{"role": "user", "content": "you are metrics-agent"},
		},
	})

	before := SnapshotDiscoveryMetrics()

	first, err := DiscoverProjectTeams(projectsDir, 24*time.Hour)
	if err != nil {
		t.Fatalf("first DiscoverProjectTeams returned error: %v", err)
	}
	second, err := DiscoverProjectTeams(projectsDir, 24*time.Hour)
	if err != nil {
		t.Fatalf("second DiscoverProjectTeams returned error: %v", err)
	}
	if len(first) != 1 || len(second) != 1 {
		t.Fatalf("expected both discover runs to find 1 team, got %d and %d", len(first), len(second))
	}

	after := SnapshotDiscoveryMetrics()
	delta := after.Delta(before)

	if delta.Runs < 2 {
		t.Fatalf("expected at least 2 runs in metrics delta, got %d", delta.Runs)
	}
	if delta.TotalTeams < 2 {
		t.Fatalf("expected aggregated teams >= 2 in metrics delta, got %d", delta.TotalTeams)
	}
	if delta.RootCacheMisses < 1 || delta.RootCacheHits < 1 {
		t.Fatalf("expected root cache to have both miss and hit, got miss=%d hit=%d", delta.RootCacheMisses, delta.RootCacheHits)
	}
	if delta.SubCacheMisses < 1 || delta.SubCacheHits < 1 {
		t.Fatalf("expected sub cache to have both miss and hit, got miss=%d hit=%d", delta.SubCacheMisses, delta.SubCacheHits)
	}
	if delta.TotalDuration <= 0 {
		t.Fatalf("expected positive total duration, got %v", delta.TotalDuration)
	}
}

func writeDiscoveryJSONL(t *testing.T, path string, records []map[string]any) {
	t.Helper()

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("failed to create directory: %v", err)
	}

	f, err := os.Create(path)
	if err != nil {
		t.Fatalf("failed to create file: %v", err)
	}
	defer f.Close()

	encoder := json.NewEncoder(f)
	for _, record := range records {
		if err := encoder.Encode(record); err != nil {
			t.Fatalf("failed to write record: %v", err)
		}
	}
}

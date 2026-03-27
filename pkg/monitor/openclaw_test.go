package monitor

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/liaoweijun/agent-team-monitor/pkg/parser"
)

func TestConvertOpenClawEvents(t *testing.T) {
	events := []parser.OpenClawSessionEvent{
		{
			Kind:      "response",
			Title:     "输出",
			Text:      "任务已完成",
			Timestamp: time.Now(),
		},
	}

	converted := convertOpenClawEvents(events)
	if len(converted) != 1 {
		t.Fatalf("expected 1 converted event, got %d", len(converted))
	}
	if converted[0].Source != "openclaw_session" {
		t.Fatalf("unexpected source: %s", converted[0].Source)
	}
}

func TestFirstNonEmpty(t *testing.T) {
	if got := firstNonEmpty("", "  ", "writer", "fallback"); got != "writer" {
		t.Fatalf("unexpected first value: %s", got)
	}
}

func TestOpenClawWatcherPaths(t *testing.T) {
	fsm, err := NewFileSystemMonitor(FileSystemMonitorOptions{
		Provider: ProviderOpenClaw,
	}, nil)
	if err != nil {
		t.Fatalf("NewFileSystemMonitor error: %v", err)
	}

	if filepath.Base(fsm.openClawAgentsDir) != "agents" {
		t.Fatalf("unexpected openclaw agents dir: %s", fsm.openClawAgentsDir)
	}

	if filepath.Base(fsm.openClawDir) != ".openclaw" {
		t.Fatalf("unexpected openclaw root dir: %s", fsm.openClawDir)
	}
}

func TestCollectOpenClawTeams_IncludesSubagentRunsWithoutSessions(t *testing.T) {
	root := t.TempDir()
	subagentsDir := filepath.Join(root, ".openclaw", "subagents")
	if err := os.MkdirAll(subagentsDir, 0755); err != nil {
		t.Fatalf("mkdir failed: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(root, ".openclaw", "agents"), 0755); err != nil {
		t.Fatalf("mkdir agents failed: %v", err)
	}

	now := time.Now()
	runsJSON := fmt.Sprintf(`{
  "version": 2,
  "runs": {
    "run-subagent-1": {
      "runId": "run-subagent-1",
      "childSessionKey": "agent:writer:subagent:leaf-a",
      "requesterSessionKey": "agent:main:main",
      "task": "整理日报",
      "label": "writer-leaf",
      "workspaceDir": "/home/test/workspace",
      "createdAt": %d,
      "startedAt": %d,
      "spawnMode": "run"
    }
  }
}`, now.Add(-2*time.Minute).UnixMilli(), now.Add(-1*time.Minute).UnixMilli())
	if err := os.WriteFile(filepath.Join(subagentsDir, "runs.json"), []byte(runsJSON), 0644); err != nil {
		t.Fatalf("write runs.json failed: %v", err)
	}

	collector := &Collector{}
	teams := collector.collectOpenClawTeams(root)
	if len(teams) != 1 {
		t.Fatalf("expected 1 team, got %d", len(teams))
	}

	team := teams[0]
	if team.Name != "openclaw" {
		t.Fatalf("unexpected team name: %s", team.Name)
	}
	if len(team.Members) != 1 {
		t.Fatalf("expected 1 member, got %d", len(team.Members))
	}

	member := team.Members[0]
	if member.Name != "writer-leaf" {
		t.Fatalf("unexpected member name: %s", member.Name)
	}
	if member.AgentType != "writer" {
		t.Fatalf("unexpected agent type: %s", member.AgentType)
	}
	if member.CurrentTask != "整理日报" {
		t.Fatalf("unexpected current task: %s", member.CurrentTask)
	}
	if member.Cwd != "/home/test/workspace" {
		t.Fatalf("unexpected cwd: %s", member.Cwd)
	}
}

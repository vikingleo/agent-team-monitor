package monitor

import (
	"testing"
	"time"

	"github.com/liaoweijun/agent-team-monitor/pkg/parser"
)

func TestBuildCodexTeams_MergesSessionsWithSameWorkingDirectory(t *testing.T) {
	now := time.Now()
	collector := &Collector{}

	teams := collector.buildCodexTeams([]parser.CodexSessionDiscovery{
		{
			SessionID:        "aaaaaaaa-1111-2222-3333-444444444444",
			Cwd:              "/home/test/works/alpha",
			StartedAt:        now.Add(-20 * time.Minute),
			LastActiveAt:     now.Add(-5 * time.Minute),
			LastUserMessage:  "实现登录页",
			LastAgentMessage: "正在修改组件",
		},
		{
			SessionID:        "bbbbbbbb-1111-2222-3333-444444444444",
			Cwd:              "/home/test/works/alpha",
			StartedAt:        now.Add(-18 * time.Minute),
			LastActiveAt:     now.Add(-2 * time.Minute),
			LastUserMessage:  "补充测试",
			LastAgentMessage: "已经补了单测",
		},
	}, now)

	if len(teams) != 1 {
		t.Fatalf("expected 1 codex team, got %d", len(teams))
	}

	team := teams[0]
	if team.Name != "codex-alpha" {
		t.Fatalf("unexpected team name: %s", team.Name)
	}
	if team.ProjectCwd != "/home/test/works/alpha" {
		t.Fatalf("unexpected project cwd: %s", team.ProjectCwd)
	}
	if team.LeadSessionID != "bbbbbbbb-1111-2222-3333-444444444444" {
		t.Fatalf("expected latest active session as lead, got %s", team.LeadSessionID)
	}
	if len(team.Members) != 2 {
		t.Fatalf("expected 2 members, got %d", len(team.Members))
	}
	if team.Members[0].Name != "codex-alpha-bbbbbbbb" {
		t.Fatalf("unexpected first member name: %s", team.Members[0].Name)
	}
	if team.Members[1].Name != "codex-alpha-aaaaaaaa" {
		t.Fatalf("unexpected second member name: %s", team.Members[1].Name)
	}
}

func TestBuildCodexTeams_MergesBySharedPrefixBridge(t *testing.T) {
	now := time.Now()
	collector := &Collector{}

	teams := collector.buildCodexTeams([]parser.CodexSessionDiscovery{
		{
			SessionID:        "11111111-1111-2222-3333-444444444444",
			Cwd:              "/home/test/works/monorepo/api",
			StartedAt:        now.Add(-30 * time.Minute),
			LastActiveAt:     now.Add(-6 * time.Minute),
			LastUserMessage:  "调整 API",
			LastAgentMessage: "正在更新接口",
		},
		{
			SessionID:        "22222222-1111-2222-3333-444444444444",
			Cwd:              "/home/test/works/monorepo/api",
			StartedAt:        now.Add(-25 * time.Minute),
			LastActiveAt:     now.Add(-4 * time.Minute),
			LastUserMessage:  "清理文档",
			LastAgentMessage: "已整理说明",
		},
		{
			SessionID:        "33333333-1111-2222-3333-444444444444",
			Cwd:              "/tmp/worktrees/api",
			StartedAt:        now.Add(-15 * time.Minute),
			LastActiveAt:     now.Add(-1 * time.Minute),
			LastUserMessage:  "继续 API 联调",
			LastAgentMessage: "联调中",
		},
	}, now)

	if len(teams) != 1 {
		t.Fatalf("expected bridge merge into 1 codex team, got %d", len(teams))
	}

	team := teams[0]
	if team.Name != "codex-api" {
		t.Fatalf("unexpected merged team name: %s", team.Name)
	}
	if len(team.Members) != 3 {
		t.Fatalf("expected 3 members in merged team, got %d", len(team.Members))
	}
	if team.LeadSessionID != "33333333-1111-2222-3333-444444444444" {
		t.Fatalf("expected most recently active session as lead, got %s", team.LeadSessionID)
	}
	if team.ProjectCwd != "/home/test/works/monorepo/api" {
		t.Fatalf("expected dominant cwd selected, got %s", team.ProjectCwd)
	}
}

func TestBuildCodexTeams_SeparatesUnrelatedPrefixesWithoutSharedDirectory(t *testing.T) {
	now := time.Now()
	collector := &Collector{}

	teams := collector.buildCodexTeams([]parser.CodexSessionDiscovery{
		{
			SessionID:        "aaaaaaaa-aaaa-bbbb-cccc-111111111111",
			Cwd:              "/home/test/works/alpha",
			StartedAt:        now.Add(-20 * time.Minute),
			LastActiveAt:     now.Add(-3 * time.Minute),
			LastUserMessage:  "alpha-task",
			LastAgentMessage: "alpha-response",
		},
		{
			SessionID:        "bbbbbbbb-aaaa-bbbb-cccc-111111111111",
			Cwd:              "/home/test/works/beta",
			StartedAt:        now.Add(-10 * time.Minute),
			LastActiveAt:     now.Add(-2 * time.Minute),
			LastUserMessage:  "beta-task",
			LastAgentMessage: "beta-response",
		},
	}, now)

	if len(teams) != 2 {
		t.Fatalf("expected 2 separate codex teams, got %d", len(teams))
	}

	if teams[0].Name != "codex-beta" {
		t.Fatalf("expected most active team first, got %s", teams[0].Name)
	}
	if teams[1].Name != "codex-alpha" {
		t.Fatalf("expected second team to be codex-alpha, got %s", teams[1].Name)
	}
}

func TestBuildCodexTeams_PrefersParsedDisplayNameForSubagent(t *testing.T) {
	now := time.Now()
	collector := &Collector{}

	teams := collector.buildCodexTeams([]parser.CodexSessionDiscovery{
		{
			SessionID:        "019d11a7-145a-7be3-bf0d-8939469bc2a2",
			Cwd:              "/home/test/works/yxl-front",
			DisplayName:      "Gauss (explorer)",
			AgentRole:        "explorer",
			StartedAt:        now.Add(-12 * time.Minute),
			LastActiveAt:     now.Add(-30 * time.Second),
			LastUserMessage:  "请排查 editor 页面问题",
			LastAgentMessage: "我先快速扫一遍相关文件。",
		},
	}, now)

	if len(teams) != 1 {
		t.Fatalf("expected 1 codex team, got %d", len(teams))
	}

	if len(teams[0].Members) != 1 {
		t.Fatalf("expected 1 codex member, got %d", len(teams[0].Members))
	}

	if teams[0].Members[0].Name != "Gauss (explorer)" {
		t.Fatalf("expected parsed display name, got %s", teams[0].Members[0].Name)
	}
}

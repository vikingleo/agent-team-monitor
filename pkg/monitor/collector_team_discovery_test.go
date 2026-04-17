package monitor

import (
	"testing"
	"time"

	"github.com/liaoweijun/agent-team-monitor/pkg/parser"
	"github.com/liaoweijun/agent-team-monitor/pkg/types"
)

func TestMergeDiscoveredProjectTeams_MergesIntoInboxOnlyTeam(t *testing.T) {
	now := time.Now()
	teams := []types.TeamInfo{
		{
			Name:      "wog-dev-team",
			CreatedAt: now.Add(-2 * time.Hour),
			Members:   []types.AgentInfo{},
			Tasks:     []types.TaskInfo{},
		},
	}

	discovered := []parser.ProjectTeamDiscovery{
		{
			LeadSessionID: "9636696a-c575-48e5-89d0-5cc5afb2278c",
			ProjectCwd:    "/home/test/works/wog",
			LastActiveAt:  now,
			Members: []types.AgentInfo{
				{
					Name:           "WOG 后台管理开发者",
					AgentID:        "a400c13",
					Cwd:            "/home/test/works/wog",
					LastActivity:   now,
					LastActiveTime: now,
				},
			},
		},
	}

	merged := mergeDiscoveredProjectTeams(teams, discovered)
	if len(merged) != 1 {
		t.Fatalf("expected 1 team after merge, got %d", len(merged))
	}

	team := merged[0]
	if team.Name != "wog-dev-team" {
		t.Fatalf("unexpected team name: got %s", team.Name)
	}
	if team.LeadSessionID != "9636696a-c575-48e5-89d0-5cc5afb2278c" {
		t.Fatalf("unexpected lead session id: got %s", team.LeadSessionID)
	}
	if team.ProjectCwd != "/home/test/works/wog" {
		t.Fatalf("unexpected project cwd: got %s", team.ProjectCwd)
	}
	if len(team.Members) != 1 {
		t.Fatalf("expected members merged, got %d", len(team.Members))
	}
}

func TestMergeDiscoveredProjectTeams_UsesSessionFallbackName(t *testing.T) {
	now := time.Now()
	merged := mergeDiscoveredProjectTeams(nil, []parser.ProjectTeamDiscovery{
		{
			LeadSessionID: "12345678-1111-2222-3333-444444444444",
			ProjectCwd:    "/home/test/works/demo",
			LastActiveAt:  now,
			Members: []types.AgentInfo{
				{
					Name:           "api-developer",
					AgentID:        "a111111",
					Cwd:            "/home/test/works/demo",
					LastActivity:   now,
					LastActiveTime: now,
				},
				{
					Name:           "admin-developer",
					AgentID:        "a222222",
					Cwd:            "/home/test/works/demo",
					LastActivity:   now,
					LastActiveTime: now,
				},
			},
		},
	})

	if len(merged) != 1 {
		t.Fatalf("expected 1 team, got %d", len(merged))
	}
	if merged[0].Name != "session-12345678" {
		t.Fatalf("unexpected fallback team name: got %s", merged[0].Name)
	}
}

func TestMergeDiscoveredProjectTeams_MergesByLeadSessionID(t *testing.T) {
	now := time.Now()
	teams := []types.TeamInfo{
		{
			Name:          "existing-team",
			LeadSessionID: "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee",
			CreatedAt:     now.Add(-time.Hour),
			Members: []types.AgentInfo{
				{
					Name:    "team-lead",
					AgentID: "lead-1",
				},
			},
			Tasks: []types.TaskInfo{},
		},
	}

	discovered := []parser.ProjectTeamDiscovery{
		{
			LeadSessionID: "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee",
			ProjectCwd:    "/home/test/works/existing",
			LastActiveAt:  now,
			Members: []types.AgentInfo{
				{
					Name:           "api-developer",
					AgentID:        "a222222",
					Cwd:            "/home/test/works/existing",
					LastActivity:   now,
					LastActiveTime: now,
				},
			},
		},
	}

	merged := mergeDiscoveredProjectTeams(teams, discovered)
	if len(merged) != 1 {
		t.Fatalf("expected existing team to be merged, got %d teams", len(merged))
	}
	if len(merged[0].Members) != 2 {
		t.Fatalf("expected discovered member merged into existing team, got %d members", len(merged[0].Members))
	}
	if merged[0].ProjectCwd != "/home/test/works/existing" {
		t.Fatalf("expected project cwd to be backfilled, got %s", merged[0].ProjectCwd)
	}
}

func TestMergeDiscoveredProjectTeams_SkipsLowConfidenceOrphanSession(t *testing.T) {
	now := time.Now()
	teams := []types.TeamInfo{
		{
			Name:      "alpha-dev-team",
			CreatedAt: now.Add(-time.Hour),
			Members:   []types.AgentInfo{},
			Tasks:     []types.TaskInfo{},
		},
	}

	discovered := []parser.ProjectTeamDiscovery{
		{
			LeadSessionID: "5bb422f0-a5f9-4260-9ec7-757b7e757969",
			ProjectCwd:    "/home/test/works/wog/wog-for-vue3",
			LastActiveAt:  now,
			Members: []types.AgentInfo{
				{
					Name:           "agent-af22756",
					AgentID:        "af22756",
					Cwd:            "/home/test/works/wog/wog-for-vue3",
					LastActivity:   now,
					LastActiveTime: now,
				},
			},
		},
	}

	merged := mergeDiscoveredProjectTeams(teams, discovered)

	if len(merged) != 1 {
		t.Fatalf("expected low-confidence orphan session to be skipped, got %d teams", len(merged))
	}
	if merged[0].LeadSessionID != "" {
		t.Fatalf("expected existing team to remain unchanged, got lead session %s", merged[0].LeadSessionID)
	}
}

func TestMergeStandaloneClaudeSessions_CreatesStandaloneTeam(t *testing.T) {
	now := time.Now()
	merged := mergeStandaloneClaudeSessions(nil, []parser.ClaudeSessionDiscovery{
		{
			PID:        10217,
			SessionID:  "9a88def1-6ad3-4d32-a378-1dd0979cfd49",
			Cwd:        "/home/test/work/demo",
			StartedAt:  now.Add(-2 * time.Minute),
			LastSeenAt: now,
			Kind:       "interactive",
			Entrypoint: "cli",
		},
	})

	if len(merged) != 1 {
		t.Fatalf("expected 1 team, got %d", len(merged))
	}

	team := merged[0]
	if team.Name != "claude-demo" {
		t.Fatalf("unexpected team name: %s", team.Name)
	}
	if team.LeadSessionID != "9a88def1-6ad3-4d32-a378-1dd0979cfd49" {
		t.Fatalf("unexpected lead session id: %s", team.LeadSessionID)
	}
	if team.ProjectCwd != "/home/test/work/demo" {
		t.Fatalf("unexpected project cwd: %s", team.ProjectCwd)
	}
	if len(team.Members) != 1 {
		t.Fatalf("expected 1 member, got %d", len(team.Members))
	}
	if team.Members[0].Name != "team-lead" {
		t.Fatalf("unexpected member name: %s", team.Members[0].Name)
	}
}

func TestMergeStandaloneClaudeSessions_MergesIntoExistingTeamByLeadSession(t *testing.T) {
	now := time.Now()
	teams := []types.TeamInfo{
		{
			Name:          "existing-team",
			LeadSessionID: "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee",
			CreatedAt:     now.Add(-time.Hour),
			Members:       []types.AgentInfo{},
			Tasks:         []types.TaskInfo{},
		},
	}

	merged := mergeStandaloneClaudeSessions(teams, []parser.ClaudeSessionDiscovery{
		{
			PID:        1,
			SessionID:  "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee",
			Cwd:        "/home/test/work/existing",
			StartedAt:  now.Add(-5 * time.Minute),
			LastSeenAt: now,
		},
	})

	if len(merged) != 1 {
		t.Fatalf("expected 1 team, got %d", len(merged))
	}
	if merged[0].ProjectCwd != "/home/test/work/existing" {
		t.Fatalf("expected cwd backfilled, got %s", merged[0].ProjectCwd)
	}
	if len(merged[0].Members) != 1 {
		t.Fatalf("expected team-lead merged, got %d members", len(merged[0].Members))
	}
}

func TestMergeStandaloneClaudeSessions_KeepsMultipleSessionsInSameCwd(t *testing.T) {
	now := time.Now()
	merged := mergeStandaloneClaudeSessions(nil, []parser.ClaudeSessionDiscovery{
		{
			PID:        1,
			SessionID:  "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee",
			Cwd:        "/home/test/work/demo",
			StartedAt:  now.Add(-5 * time.Minute),
			LastSeenAt: now.Add(-4 * time.Minute),
		},
		{
			PID:        2,
			SessionID:  "ffffffff-1111-2222-3333-444444444444",
			Cwd:        "/home/test/work/demo",
			StartedAt:  now.Add(-3 * time.Minute),
			LastSeenAt: now,
		},
	})

	if len(merged) != 2 {
		t.Fatalf("expected 2 teams, got %d", len(merged))
	}
	if merged[0].LeadSessionID == merged[1].LeadSessionID {
		t.Fatal("expected distinct lead sessions")
	}
}

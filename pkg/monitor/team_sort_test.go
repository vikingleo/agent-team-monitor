package monitor

import (
	"testing"
	"time"

	"github.com/liaoweijun/agent-team-monitor/pkg/types"
)

func TestTeamSortKey_PrefersStableFields(t *testing.T) {
	team := types.TeamInfo{
		Name:          "beta",
		SortKey:       "config:/tmp/team-beta",
		ConfigPath:    "/tmp/other",
		ProjectCwd:    "/tmp/work",
		LeadSessionID: "bbbbbbbb-1111-2222-3333-444444444444",
	}

	if got := teamSortKey(team); got != "config:/tmp/team-beta" {
		t.Fatalf("expected explicit sort key to win, got %q", got)
	}
}

func TestTeamSortKey_FallsBackWithoutActivityOrdering(t *testing.T) {
	now := time.Now()
	teams := []types.TeamInfo{
		{
			Name:      "codex-beta",
			Provider:  "codex",
			CreatedAt: now,
			SortKey:   "codex:path:/tmp/beta",
			Members: []types.AgentInfo{
				{Name: "beta", LastActiveTime: now},
			},
		},
		{
			Name:      "codex-alpha",
			Provider:  "codex",
			CreatedAt: now.Add(-time.Hour),
			SortKey:   "codex:path:/tmp/alpha",
			Members: []types.AgentInfo{
				{Name: "alpha", LastActiveTime: now.Add(-24 * time.Hour)},
			},
		},
	}

	if teamSortKey(teams[0]) < teamSortKey(teams[1]) {
		t.Fatalf("test fixture invalid: beta should sort after alpha")
	}
}

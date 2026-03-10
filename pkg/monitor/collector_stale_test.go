package monitor

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/liaoweijun/agent-team-monitor/pkg/types"
)

func TestIsTeamActiveUsesLatestMemberActivity(t *testing.T) {
	now := time.Now()
	team := types.TeamInfo{
		Name:      "alpha",
		CreatedAt: now.Add(-24 * time.Hour),
		Members: []types.AgentInfo{
			{
				Name:           "agent-a",
				LastActiveTime: now.Add(-59 * time.Minute),
			},
		},
	}

	if !isTeamActive(team, now, time.Hour) {
		t.Fatal("expected team to stay active when latest activity is within threshold")
	}
}

func TestIsTeamActiveIgnoresCreatedAtAndJoinedAtWithoutRecentActivity(t *testing.T) {
	now := time.Now()
	team := types.TeamInfo{
		Name:      "alpha",
		CreatedAt: now.Add(-10 * time.Minute),
		Members: []types.AgentInfo{
			{
				Name:     "agent-a",
				JoinedAt: now.Add(-10 * time.Minute),
			},
		},
	}

	if isTeamActive(team, now, time.Hour) {
		t.Fatal("expected team without real activity timestamps to be stale")
	}
}

func TestFilterStaleTeamsRemovesOnlyTeamsOlderThanThreshold(t *testing.T) {
	now := time.Now()
	stale := types.TeamInfo{
		Name: "stale-team",
		Members: []types.AgentInfo{{
			Name:           "agent-a",
			LastActiveTime: now.Add(-2 * time.Hour),
		}},
	}
	fresh := types.TeamInfo{
		Name: "fresh-team",
		Members: []types.AgentInfo{{
			Name:           "agent-b",
			LastActiveTime: now.Add(-20 * time.Minute),
		}},
	}

	teams := filterStaleTeams([]types.TeamInfo{stale, fresh}, time.Hour, filepath.Join(t.TempDir(), "tasks"))
	if len(teams) != 1 {
		t.Fatalf("expected 1 team after filtering, got %d", len(teams))
	}
	if teams[0].Name != "fresh-team" {
		t.Fatalf("expected fresh team to remain, got %s", teams[0].Name)
	}
}

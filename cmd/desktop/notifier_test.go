package main

import (
	"testing"
	"time"

	"github.com/liaoweijun/agent-team-monitor/pkg/types"
)

func TestDesktopNotifierSnapshotTasks(t *testing.T) {
	notifier := newDesktopNotifier(nil, newInMemoryDesktopPreferencesStore())
	state := types.MonitorState{
		Teams: []types.TeamInfo{
			{
				Name: "alpha",
				Tasks: []types.TaskInfo{
					{ID: "t-1", Status: "in_progress", Owner: "Alice"},
				},
			},
		},
	}

	snapshot := notifier.snapshotTasks(state)
	task, ok := snapshot["alpha::t-1"]
	if !ok {
		t.Fatalf("expected task snapshot to include task key, got %#v", snapshot)
	}
	if task.Status != "in_progress" || task.Owner != "Alice" {
		t.Fatalf("unexpected task snapshot: %#v", task)
	}
}

func TestDesktopNotifierSnapshotStaleAgents(t *testing.T) {
	notifier := newDesktopNotifier(nil, newInMemoryDesktopPreferencesStore())
	now := time.Now()
	state := types.MonitorState{
		Teams: []types.TeamInfo{
			{
				Name: "alpha",
				Members: []types.AgentInfo{
					{Name: "Alice", Status: "working", LastActiveTime: now.Add(-20 * time.Minute)},
					{Name: "Bob", Status: "idle", LastActiveTime: now.Add(-30 * time.Minute)},
				},
			},
		},
	}

	stale := notifier.snapshotStaleAgents(state, now)
	if len(stale) != 1 {
		t.Fatalf("expected exactly one stale working agent, got %#v", stale)
	}
	if _, ok := stale["alpha::Alice"]; !ok {
		t.Fatalf("expected Alice to be marked stale, got %#v", stale)
	}
}

func TestLatestActivityTimeUsesMostRecentSignal(t *testing.T) {
	now := time.Now()
	agent := types.AgentInfo{
		LastActiveTime:  now.Add(-10 * time.Minute),
		LastMessageTime: now.Add(-5 * time.Minute),
		LastActivity:    now.Add(-2 * time.Minute),
	}

	if got := latestActivityTime(agent); !got.Equal(agent.LastActivity) {
		t.Fatalf("expected latest activity time %v, got %v", agent.LastActivity, got)
	}
}

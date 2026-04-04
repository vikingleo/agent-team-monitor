package main

import (
	"context"
	"fmt"
	"os/exec"
	"runtime"
	"strings"
	"time"

	"github.com/liaoweijun/agent-team-monitor/pkg/monitor"
	"github.com/liaoweijun/agent-team-monitor/pkg/types"
)

const (
	desktopNotificationPollInterval = 5 * time.Second
	staleAgentThreshold             = 12 * time.Minute
)

type desktopNotifier struct {
	collector    *monitor.Collector
	preferences  *desktopPreferencesStore
	lastSnapshot map[string]taskSnapshot
	staleAgents  map[string]time.Time
}

type taskSnapshot struct {
	Status string
	Owner  string
}

func newDesktopNotifier(collector *monitor.Collector, preferences *desktopPreferencesStore) *desktopNotifier {
	return &desktopNotifier{
		collector:    collector,
		preferences:  preferences,
		lastSnapshot: map[string]taskSnapshot{},
		staleAgents:  map[string]time.Time{},
	}
}

func (n *desktopNotifier) Start(ctx context.Context) {
	if n == nil || n.collector == nil || n.preferences == nil {
		return
	}

	ticker := time.NewTicker(desktopNotificationPollInterval)
	defer ticker.Stop()

	n.prime()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			n.poll()
		}
	}
}

func (n *desktopNotifier) prime() {
	state := n.collector.GetState()
	n.lastSnapshot = n.snapshotTasks(state)
	n.staleAgents = n.snapshotStaleAgents(state, time.Now())
}

func (n *desktopNotifier) poll() {
	prefs := n.preferences.Get()
	if !prefs.NotifyTaskCompletion && !prefs.NotifyStaleAgents {
		n.prime()
		return
	}

	state := n.collector.GetState()
	now := time.Now()

	if prefs.NotifyTaskCompletion {
		n.notifyTaskCompletions(state)
	} else {
		n.lastSnapshot = n.snapshotTasks(state)
	}

	if prefs.NotifyStaleAgents {
		n.notifyStaleAgents(state, now)
	} else {
		n.staleAgents = n.snapshotStaleAgents(state, now)
	}
}

func (n *desktopNotifier) notifyTaskCompletions(state types.MonitorState) {
	next := n.snapshotTasks(state)

	for key, task := range next {
		prev, ok := n.lastSnapshot[key]
		if !ok {
			continue
		}

		if normalizeTaskStatus(prev.Status) == "completed" || normalizeTaskStatus(task.Status) != "completed" {
			continue
		}

		taskID := key
		if idx := strings.Index(key, "::"); idx >= 0 {
			taskID = key[idx+2:]
		}

		title := "任务已完成"
		message := taskID
		if strings.TrimSpace(task.Owner) != "" {
			message = fmt.Sprintf("%s 已完成 %s", task.Owner, taskID)
		}
		n.send(title, message)
	}

	n.lastSnapshot = next
}

func (n *desktopNotifier) notifyStaleAgents(state types.MonitorState, now time.Time) {
	current := n.snapshotStaleAgents(state, now)

	for key, lastActive := range current {
		if _, seen := n.staleAgents[key]; seen {
			continue
		}

		teamName := key
		agentName := key
		if idx := strings.Index(key, "::"); idx >= 0 {
			teamName = key[:idx]
			agentName = key[idx+2:]
		}

		ageMinutes := int(now.Sub(lastActive).Minutes())
		n.send("成员长时间无活动", fmt.Sprintf("%s / %s 已超过 %d 分钟无活动", teamName, agentName, ageMinutes))
	}

	n.staleAgents = current
}

func (n *desktopNotifier) snapshotTasks(state types.MonitorState) map[string]taskSnapshot {
	result := make(map[string]taskSnapshot)

	for _, team := range state.Teams {
		for _, task := range team.Tasks {
			key := team.Name + "::" + strings.TrimSpace(task.ID)
			if strings.TrimSpace(task.ID) == "" {
				key = team.Name + "::" + strings.TrimSpace(task.Subject)
			}
			result[key] = taskSnapshot{
				Status: task.Status,
				Owner:  task.Owner,
			}
		}
	}

	return result
}

func (n *desktopNotifier) snapshotStaleAgents(state types.MonitorState, now time.Time) map[string]time.Time {
	result := make(map[string]time.Time)

	for _, team := range state.Teams {
		for _, agent := range team.Members {
			if !isWorkingStatus(agent.Status) {
				continue
			}

			lastActive := latestActivityTime(agent)
			if lastActive.IsZero() || now.Sub(lastActive) < staleAgentThreshold {
				continue
			}

			result[team.Name+"::"+agent.Name] = lastActive
		}
	}

	return result
}

func (n *desktopNotifier) send(title, message string) {
	nextTitle := strings.TrimSpace(title)
	nextMessage := strings.TrimSpace(message)
	if nextTitle == "" || nextMessage == "" {
		return
	}

	cmd := desktopNotificationCommand(nextTitle, nextMessage)
	if cmd == nil {
		return
	}

	if err := cmd.Run(); err != nil {
		// Ignore notification failures; desktop app should keep running.
	}
}

func desktopNotificationCommand(title, message string) *exec.Cmd {
	switch runtime.GOOS {
	case "darwin":
		script := fmt.Sprintf(`display notification %q with title %q`, message, title)
		return exec.Command("osascript", "-e", script)
	case "windows":
		return nil
	case "linux":
		return exec.Command("notify-send", title, message, "--app-name=Agent Team Monitor")
	default:
		return nil
	}
}

func normalizeTaskStatus(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}

func isWorkingStatus(value string) bool {
	status := strings.ToLower(strings.TrimSpace(value))
	return status == "working" || status == "busy"
}

func latestActivityTime(agent types.AgentInfo) time.Time {
	candidates := []time.Time{
		agent.LastActiveTime,
		agent.LastMessageTime,
		agent.LastActivity,
	}

	var latest time.Time
	for _, candidate := range candidates {
		if candidate.After(latest) {
			latest = candidate
		}
	}

	return latest
}

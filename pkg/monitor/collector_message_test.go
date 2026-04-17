package monitor

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/liaoweijun/agent-team-monitor/pkg/types"
)

func TestAppendInboxMessage_PreservesExistingFields(t *testing.T) {
	inboxPath := filepath.Join(t.TempDir(), "teams", "default", "inboxes", "team-lead.json")
	if err := os.MkdirAll(filepath.Dir(inboxPath), 0o755); err != nil {
		t.Fatalf("mkdir failed: %v", err)
	}

	initial := []map[string]interface{}{
		{
			"from":      "statusline_agent",
			"text":      "existing",
			"timestamp": "2026-04-08T16:21:56.299Z",
			"color":     "red",
			"read":      true,
		},
	}
	payload, err := json.Marshal(initial)
	if err != nil {
		t.Fatalf("marshal initial failed: %v", err)
	}
	if err := os.WriteFile(inboxPath, payload, 0o644); err != nil {
		t.Fatalf("write initial failed: %v", err)
	}

	if err := appendInboxMessage(inboxPath, "agent-team-monitor", "继续执行"); err != nil {
		t.Fatalf("appendInboxMessage error: %v", err)
	}

	updated, err := os.ReadFile(inboxPath)
	if err != nil {
		t.Fatalf("read updated inbox failed: %v", err)
	}

	var messages []map[string]interface{}
	if err := json.Unmarshal(updated, &messages); err != nil {
		t.Fatalf("unmarshal updated inbox failed: %v", err)
	}

	if len(messages) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(messages))
	}
	if messages[0]["color"] != "red" {
		t.Fatalf("expected existing color preserved, got %v", messages[0]["color"])
	}
	if messages[1]["from"] != "agent-team-monitor" {
		t.Fatalf("unexpected sender: %v", messages[1]["from"])
	}
	if messages[1]["text"] != "继续执行" {
		t.Fatalf("unexpected text: %v", messages[1]["text"])
	}
}

func TestSendAgentMessage_AppendsToResolvedInbox(t *testing.T) {
	root := t.TempDir()
	t.Setenv("HOME", root)

	collector, err := NewCollector()
	if err != nil {
		t.Fatalf("NewCollector error: %v", err)
	}

	collector.state.Teams = []types.TeamInfo{
		{
			Name:          "claude-demo",
			Provider:      "claude",
			InboxTeamName: "default",
			Members: []types.AgentInfo{
				{
					Name:             "team-lead",
					Provider:         "claude",
					CommandTransport: "claude_inbox",
				},
			},
		},
	}

	if err := collector.SendAgentMessage("claude-demo", "team-lead", "请继续执行"); err != nil {
		t.Fatalf("SendAgentMessage error: %v", err)
	}

	inboxPath := filepath.Join(root, ".claude", "teams", "default", "inboxes", "team-lead.json")
	data, err := os.ReadFile(inboxPath)
	if err != nil {
		t.Fatalf("read inbox failed: %v", err)
	}

	var messages []map[string]interface{}
	if err := json.Unmarshal(data, &messages); err != nil {
		t.Fatalf("unmarshal inbox failed: %v", err)
	}
	if len(messages) != 1 {
		t.Fatalf("expected 1 message, got %d", len(messages))
	}
	if messages[0]["text"] != "请继续执行" {
		t.Fatalf("unexpected message text: %v", messages[0]["text"])
	}
}

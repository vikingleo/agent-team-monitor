package parser

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"time"
	"unicode"
)

// InboxMessage represents a message in an agent's inbox
type InboxMessage struct {
	From      string    `json:"from"`
	Text      string    `json:"text"`
	Summary   string    `json:"summary"`
	Timestamp time.Time `json:"timestamp"`
	Read      bool      `json:"read"`
}

// ParseInbox parses an agent's inbox file and returns the latest message
func ParseInbox(teamsDir, teamName, agentName string) (*InboxMessage, error) {
	inboxPath := filepath.Join(teamsDir, teamName, "inboxes", agentName+".json")

	data, err := os.ReadFile(inboxPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil // No inbox file is not an error
		}
		return nil, err
	}

	var messages []InboxMessage
	if err := json.Unmarshal(data, &messages); err != nil {
		return nil, err
	}

	if len(messages) == 0 {
		return nil, nil
	}

	// Return the latest message (last in array)
	latest := messages[len(messages)-1]
	return &latest, nil
}

// ParseInboxMessages returns all inbox messages for an agent ordered from oldest to newest.
func ParseInboxMessages(teamsDir, teamName, agentName string) ([]InboxMessage, error) {
	inboxPath := filepath.Join(teamsDir, teamName, "inboxes", agentName+".json")

	data, err := os.ReadFile(inboxPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	var messages []InboxMessage
	if err := json.Unmarshal(data, &messages); err != nil {
		return nil, err
	}

	for i := range messages {
		messages[i].Text = sanitizeInboxDisplayText(messages[i].Text)
		messages[i].Summary = sanitizeInboxDisplayText(messages[i].Summary)
	}

	return messages, nil
}

func sanitizeInboxDisplayText(text string) string {
	trimmed := strings.TrimSpace(strings.ReplaceAll(text, "\r\n", "\n"))
	if trimmed == "" {
		return ""
	}

	lines := strings.Split(trimmed, "\n")
	result := make([]string, 0, len(lines))
	blankCount := 0
	for _, line := range lines {
		cleanLine := strings.TrimRightFunc(line, unicode.IsSpace)
		if strings.TrimSpace(cleanLine) == "" {
			blankCount++
			if blankCount > 1 {
				continue
			}
			result = append(result, "")
			continue
		}

		blankCount = 0
		result = append(result, cleanLine)
	}

	return strings.TrimSpace(strings.Join(result, "\n"))
}

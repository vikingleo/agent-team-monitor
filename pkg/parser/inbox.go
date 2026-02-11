package parser

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"
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

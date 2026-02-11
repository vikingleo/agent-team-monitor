package parser

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// ActivityLog represents a single log entry from agent's jsonl file
type ActivityLog struct {
	Type      string          `json:"type"`
	Timestamp string          `json:"timestamp"`
	AgentID   string          `json:"agentId"`
	Cwd       string          `json:"cwd"`
	Message   json.RawMessage `json:"message"`
}

// AssistantMessage represents an assistant's message with content
type AssistantMessage struct {
	Role    string          `json:"role"`
	Content json.RawMessage `json:"content"`
}

// ContentItem represents a single content item (text or tool_use)
type ContentItem struct {
	Type string `json:"type"`
	Text string `json:"text,omitempty"`
	Name string `json:"name,omitempty"` // For tool_use
	ID   string `json:"id,omitempty"`   // For tool_use
}

// AgentActivity represents parsed agent activity
type AgentActivity struct {
	LastThinking   string    // Latest thinking/reasoning text
	LastToolUse    string    // Latest tool being used (e.g., "Read", "Edit", "Bash")
	LastToolDetail string    // Details about the tool use
	LastActiveTime time.Time // Last activity timestamp
}

// ParseAgentActivity parses the agent's jsonl log file and extracts recent activity
func ParseAgentActivity(logPath string) (*AgentActivity, error) {
	file, err := os.Open(logPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil // No log file is not an error
		}
		return nil, err
	}
	defer file.Close()

	activity := &AgentActivity{}
	scanner := bufio.NewScanner(file)

	// Read all lines to get the most recent entries
	var lines []string
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	// Process lines in reverse to get most recent activity first
	for i := len(lines) - 1; i >= 0 && i >= len(lines)-50; i-- {
		var log ActivityLog
		if err := json.Unmarshal([]byte(lines[i]), &log); err != nil {
			continue
		}

		// Parse timestamp
		timestamp, err := time.Parse(time.RFC3339, log.Timestamp)
		if err != nil {
			continue
		}

		// Update last active time
		if activity.LastActiveTime.IsZero() || timestamp.After(activity.LastActiveTime) {
			activity.LastActiveTime = timestamp
		}

		// Only process assistant messages
		if log.Type != "assistant" {
			continue
		}

		var msg AssistantMessage
		if err := json.Unmarshal(log.Message, &msg); err != nil {
			continue
		}

		// Parse content
		var content []ContentItem
		if err := json.Unmarshal(msg.Content, &content); err != nil {
			// Try parsing as single object
			var singleContent ContentItem
			if err := json.Unmarshal(msg.Content, &singleContent); err != nil {
				continue
			}
			content = []ContentItem{singleContent}
		}

		// Extract thinking and tool use
		for _, item := range content {
			if item.Type == "text" && activity.LastThinking == "" {
				// Clean up and truncate thinking text
				text := strings.TrimSpace(item.Text)
				if len(text) > 150 {
					text = text[:150] + "..."
				}
				activity.LastThinking = text
			}

			if item.Type == "tool_use" && activity.LastToolUse == "" {
				activity.LastToolUse = item.Name
				// Try to extract tool details from the raw message
				activity.LastToolDetail = extractToolDetail(item.Name, log.Message)
			}
		}

		// Stop if we have both thinking and tool use
		if activity.LastThinking != "" && activity.LastToolUse != "" {
			break
		}
	}

	return activity, nil
}

// extractToolDetail extracts details about tool usage
func extractToolDetail(toolName string, rawMessage json.RawMessage) string {
	var msg map[string]interface{}
	if err := json.Unmarshal(rawMessage, &msg); err != nil {
		return ""
	}

	content, ok := msg["content"].([]interface{})
	if !ok {
		return ""
	}

	for _, item := range content {
		itemMap, ok := item.(map[string]interface{})
		if !ok {
			continue
		}

		if itemMap["type"] == "tool_use" && itemMap["name"] == toolName {
			input, ok := itemMap["input"].(map[string]interface{})
			if !ok {
				return ""
			}

			// Extract relevant details based on tool type
			switch toolName {
			case "Read":
				if filePath, ok := input["file_path"].(string); ok {
					return filepath.Base(filePath)
				}
			case "Edit":
				if filePath, ok := input["file_path"].(string); ok {
					return filepath.Base(filePath)
				}
			case "Write":
				if filePath, ok := input["file_path"].(string); ok {
					return filepath.Base(filePath)
				}
			case "Bash":
				if command, ok := input["command"].(string); ok {
					// Truncate long commands
					if len(command) > 50 {
						command = command[:50] + "..."
					}
					return command
				}
			case "Grep":
				if pattern, ok := input["pattern"].(string); ok {
					return "搜索: " + pattern
				}
			case "Glob":
				if pattern, ok := input["pattern"].(string); ok {
					return "查找: " + pattern
				}
			}
		}
	}

	return ""
}

// FindAgentLogFileByCwd finds the agent's log file by matching working directory
// Returns logFilePath, agentID, error
func FindAgentLogFileByCwd(projectsDir, cwd string) (string, string, error) {
	if cwd == "" {
		return "", "", nil
	}

	// Use filepath.Walk to recursively search for agent log files
	var foundLogPath, foundAgentID string

	err := filepath.Walk(projectsDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // Skip errors
		}

		// Check if this is an agent log file
		if !info.IsDir() && strings.HasPrefix(info.Name(), "agent-") && strings.HasSuffix(info.Name(), ".jsonl") {
			// Read first line to get cwd
			file, err := os.Open(path)
			if err != nil {
				return nil
			}
			defer file.Close()

			scanner := bufio.NewScanner(file)
			if scanner.Scan() {
				var log ActivityLog
				if err := json.Unmarshal(scanner.Bytes(), &log); err != nil {
					return nil
				}

				// Check if cwd matches
				if log.Cwd == cwd {
					foundLogPath = path
					foundAgentID = log.AgentID
					return filepath.SkipAll // Found it, stop walking
				}
			}
		}

		return nil
	})

	if err != nil && err != filepath.SkipAll {
		return "", "", err
	}

	return foundLogPath, foundAgentID, nil
}

// FindAgentLogFile finds the agent's log file by matching agent ID (deprecated, use FindAgentLogFileByCwd)
func FindAgentLogFile(projectsDir, agentID string) (string, error) {
	// Search for agent log files
	pattern := filepath.Join(projectsDir, "*", "subagents", "agent-"+agentID+".jsonl")
	matches, err := filepath.Glob(pattern)
	if err != nil {
		return "", err
	}

	if len(matches) > 0 {
		return matches[0], nil
	}

	return "", nil
}

package parser

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/liaoweijun/agent-team-monitor/pkg/types"
)

// todoFileItem represents a single item in a TodoWrite JSON file
type todoFileItem struct {
	Content    string `json:"content"`
	Status     string `json:"status"`
	ActiveForm string `json:"activeForm"`
}

// ParseTodoFile parses a TodoWrite JSON file and returns todo items
func ParseTodoFile(todoPath string) ([]types.TodoItem, error) {
	data, err := os.ReadFile(todoPath)
	if err != nil {
		return nil, err
	}

	var items []todoFileItem
	if err := json.Unmarshal(data, &items); err != nil {
		return nil, err
	}

	return convertTodoItems(items), nil
}

// LoadTodosForSession loads todo items for a given session ID from the todos directory.
// Falls back to extracting from JSONL log if the todo file is empty.
func LoadTodosForSession(todosDir, sessionID string) ([]types.TodoItem, error) {
	if todosDir == "" || sessionID == "" {
		return nil, nil
	}

	// Todo files are named: {sessionId}-agent-{sessionId}.json
	todoFile := filepath.Join(todosDir, fmt.Sprintf("%s-agent-%s.json", sessionID, sessionID))
	if _, err := os.Stat(todoFile); os.IsNotExist(err) {
		return nil, nil
	}

	return ParseTodoFile(todoFile)
}

// ExtractTodosFromLog extracts the last TodoWrite call from a JSONL log file
func ExtractTodosFromLog(logPath string) ([]types.TodoItem, error) {
	file, err := os.Open(logPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	defer file.Close()

	scanner := newLargeScanner(file)

	// Use a ring buffer to keep only the last N lines
	const tailSize = 100
	ring := make([]string, tailSize)
	ringIdx := 0
	totalLines := 0
	for scanner.Scan() {
		ring[ringIdx%tailSize] = scanner.Text()
		ringIdx++
		totalLines++
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	count := totalLines
	if count > tailSize {
		count = tailSize
	}

	// Scan in reverse to find the most recent TodoWrite call
	for k := 0; k < count; k++ {
		idx := (ringIdx - 1 - k) % tailSize
		if idx < 0 {
			idx += tailSize
		}

		line := ring[idx]
		todos := extractTodoWriteFromLine([]byte(line))
		if todos != nil {
			return todos, nil
		}
	}

	return nil, nil
}

// extractTodoWriteFromLine tries to extract TodoWrite data from a single JSONL line
func extractTodoWriteFromLine(line []byte) []types.TodoItem {
	var entry struct {
		Type    string          `json:"type"`
		Message json.RawMessage `json:"message"`
	}
	if err := json.Unmarshal(line, &entry); err != nil {
		return nil
	}

	if entry.Type != "assistant" {
		return nil
	}

	var msg struct {
		Content []struct {
			Type  string `json:"type"`
			Name  string `json:"name"`
			Input struct {
				Todos []todoFileItem `json:"todos"`
			} `json:"input"`
		} `json:"content"`
	}
	if err := json.Unmarshal(entry.Message, &msg); err != nil {
		return nil
	}

	for _, item := range msg.Content {
		if item.Type == "tool_use" && item.Name == "TodoWrite" && len(item.Input.Todos) > 0 {
			return convertTodoItems(item.Input.Todos)
		}
	}

	return nil
}

func convertTodoItems(items []todoFileItem) []types.TodoItem {
	if len(items) == 0 {
		return nil
	}

	todos := make([]types.TodoItem, 0, len(items))
	for _, item := range items {
		todos = append(todos, types.TodoItem{
			Content:    item.Content,
			Status:     item.Status,
			ActiveForm: item.ActiveForm,
		})
	}
	return todos
}

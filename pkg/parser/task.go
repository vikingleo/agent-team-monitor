package parser

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"

	"github.com/liaoweijun/agent-team-monitor/pkg/types"
)

// TaskFile represents the structure of a task JSON file
type TaskFile struct {
	ID          string                 `json:"id"`
	Subject     string                 `json:"subject"`
	Description string                 `json:"description"`
	Status      string                 `json:"status"`
	Owner       string                 `json:"owner"`
	CreatedAt   string                 `json:"created_at"`
	UpdatedAt   string                 `json:"updated_at"`
	Blocks      []string               `json:"blocks"`
	BlockedBy   []string               `json:"blocked_by"`
	Metadata    map[string]interface{} `json:"metadata"`
}

// ParseTaskFile parses a task JSON file
func ParseTaskFile(taskPath string) (*types.TaskInfo, error) {
	data, err := os.ReadFile(taskPath)
	if err != nil {
		return nil, err
	}

	var task TaskFile
	if err := json.Unmarshal(data, &task); err != nil {
		return nil, err
	}

	// Skip internal tasks
	if task.Metadata != nil {
		if internal, ok := task.Metadata["_internal"].(bool); ok && internal {
			return nil, nil
		}
	}

	// Parse timestamps
	createdAt := time.Now()
	if task.CreatedAt != "" {
		if t, err := time.Parse(time.RFC3339, task.CreatedAt); err == nil {
			createdAt = t
		}
	}

	updatedAt := createdAt
	if task.UpdatedAt != "" {
		if t, err := time.Parse(time.RFC3339, task.UpdatedAt); err == nil {
			updatedAt = t
		}
	}

	return &types.TaskInfo{
		ID:          task.ID,
		Subject:     task.Subject,
		Description: task.Description,
		Status:      task.Status,
		Owner:       task.Owner,
		CreatedAt:   createdAt,
		UpdatedAt:   updatedAt,
	}, nil
}

// ScanTasks scans the tasks directory for a specific team
func ScanTasks(tasksDir, teamName string) ([]types.TaskInfo, error) {
	return scanTasksInternal(tasksDir, teamName, true)
}

// ScanAllTasks scans all tasks including internal ones
func ScanAllTasks(tasksDir, teamName string) ([]types.TaskInfo, error) {
	return scanTasksInternal(tasksDir, teamName, false)
}

// scanTasksInternal is the internal implementation
func scanTasksInternal(tasksDir, teamName string, skipInternal bool) ([]types.TaskInfo, error) {
	var tasks []types.TaskInfo

	teamTasksDir := filepath.Join(tasksDir, teamName)
	entries, err := os.ReadDir(teamTasksDir)
	if err != nil {
		if os.IsNotExist(err) {
			return tasks, nil
		}
		return nil, err
	}

	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}

		taskPath := filepath.Join(teamTasksDir, entry.Name())
		task, err := ParseTaskFile(taskPath)
		if err != nil {
			continue
		}
		// Skip if task is nil (internal task) and skipInternal is true
		if task != nil {
			tasks = append(tasks, *task)
		} else if !skipInternal {
			// Re-parse without filtering for internal tasks
			task, err = parseTaskFileNoFilter(taskPath)
			if err == nil && task != nil {
				tasks = append(tasks, *task)
			}
		}
	}

	return tasks, nil
}

// parseTaskFileNoFilter parses task file without filtering internal tasks
func parseTaskFileNoFilter(taskPath string) (*types.TaskInfo, error) {
	data, err := os.ReadFile(taskPath)
	if err != nil {
		return nil, err
	}

	var task TaskFile
	if err := json.Unmarshal(data, &task); err != nil {
		return nil, err
	}

	// Parse timestamps
	createdAt := time.Now()
	if task.CreatedAt != "" {
		if t, err := time.Parse(time.RFC3339, task.CreatedAt); err == nil {
			createdAt = t
		}
	}

	updatedAt := createdAt
	if task.UpdatedAt != "" {
		if t, err := time.Parse(time.RFC3339, task.UpdatedAt); err == nil {
			updatedAt = t
		}
	}

	return &types.TaskInfo{
		ID:          task.ID,
		Subject:     task.Subject,
		Description: task.Description,
		Status:      task.Status,
		Owner:       task.Owner,
		CreatedAt:   createdAt,
		UpdatedAt:   updatedAt,
	}, nil
}


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
	ID          string   `json:"id"`
	Subject     string   `json:"subject"`
	Description string   `json:"description"`
	Status      string   `json:"status"`
	Owner       string   `json:"owner"`
	CreatedAt   string   `json:"created_at"`
	UpdatedAt   string   `json:"updated_at"`
	Blocks      []string `json:"blocks"`
	BlockedBy   []string `json:"blocked_by"`
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
		tasks = append(tasks, *task)
	}

	return tasks, nil
}

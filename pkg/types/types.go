package types

import "time"

// TeamInfo represents a Claude agent team
type TeamInfo struct {
	Name       string      `json:"name"`
	CreatedAt  time.Time   `json:"created_at"`
	Members    []AgentInfo `json:"members"`
	Tasks      []TaskInfo  `json:"tasks"`
	ConfigPath string      `json:"config_path"`
}

// AgentInfo represents an agent in a team
type AgentInfo struct {
	Name         string    `json:"name"`
	AgentID      string    `json:"agent_id"`
	AgentType    string    `json:"agent_type"`
	Status       string    `json:"status"` // idle, working, completed
	CurrentTask  string    `json:"current_task,omitempty"`
	LastActivity time.Time `json:"last_activity"`
}

// TaskInfo represents a task in the task list
type TaskInfo struct {
	ID          string    `json:"id"`
	Subject     string    `json:"subject"`
	Description string    `json:"description"`
	Status      string    `json:"status"` // pending, in_progress, completed
	Owner       string    `json:"owner,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// ProcessInfo represents a Claude Code process
type ProcessInfo struct {
	PID       int32     `json:"pid"`
	Command   string    `json:"command"`
	Team      string    `json:"team,omitempty"`
	StartedAt time.Time `json:"started_at"`
}

// MonitorState represents the overall monitoring state
type MonitorState struct {
	Teams     []TeamInfo    `json:"teams"`
	Processes []ProcessInfo `json:"processes"`
	UpdatedAt time.Time     `json:"updated_at"`
}

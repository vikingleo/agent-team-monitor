package types

import "time"

// TeamInfo represents a Claude agent team
type TeamInfo struct {
	Name          string      `json:"name"`
	Provider      string      `json:"provider,omitempty"` // claude, codex, openclaw
	CreatedAt     time.Time   `json:"created_at"`
	LeadSessionID string      `json:"lead_session_id,omitempty"`
	ProjectCwd    string      `json:"project_cwd,omitempty"`
	Members       []AgentInfo `json:"members"`
	Tasks         []TaskInfo  `json:"tasks"`
	ConfigPath    string      `json:"config_path"`
}

// AgentEvent represents a recent observable event for an agent.
type AgentEvent struct {
	Kind      string    `json:"kind"`                // response, message, thinking, tool, status
	Title     string    `json:"title,omitempty"`     // Short UI label
	Text      string    `json:"text"`                // Full display text
	Source    string    `json:"source,omitempty"`    // inbox, activity_log, codex_session, openclaw_session
	Timestamp time.Time `json:"timestamp,omitempty"` // Event time
}

// AgentInfo represents an agent in a team
type AgentInfo struct {
	Name            string    `json:"name"`
	Provider        string    `json:"provider,omitempty"` // claude, codex, openclaw
	AgentID         string    `json:"agent_id"`
	AgentType       string    `json:"agent_type"`
	Status          string    `json:"status"` // idle, working, completed
	CurrentTask     string    `json:"current_task,omitempty"`
	JoinedAt        time.Time `json:"joined_at,omitempty"`
	RoleEmoji       string    `json:"role_emoji,omitempty"`       // Office persona emoji
	OfficeDialogues []string  `json:"office_dialogues,omitempty"` // Shared office dialogue lines
	LastActivity    time.Time `json:"last_activity"`
	Cwd             string    `json:"cwd,omitempty"`             // Working directory
	LatestMessage   string    `json:"latest_message,omitempty"`  // Latest inbox message
	MessageSummary  string    `json:"message_summary,omitempty"` // Message summary
	LatestResponse  string    `json:"latest_response,omitempty"` // Latest full outbound response text
	LastMessageTime time.Time `json:"last_message_time,omitempty"`
	// Activity tracking from jsonl logs
	LastThinking   string       `json:"last_thinking,omitempty"`    // Latest thinking/reasoning
	LastToolUse    string       `json:"last_tool_use,omitempty"`    // Latest tool name (Read, Edit, Bash, etc.)
	LastToolDetail string       `json:"last_tool_detail,omitempty"` // Tool usage details
	LastActiveTime time.Time    `json:"last_active_time,omitempty"` // Last activity timestamp from logs
	RecentEvents   []AgentEvent `json:"recent_events,omitempty"`    // Full recent timeline for Web UI
	// TodoWrite items from ~/.claude/todos/
	Todos []TodoItem `json:"todos,omitempty"`
}

// TodoItem represents a single todo item from TodoWrite
type TodoItem struct {
	Content    string `json:"content"`
	Status     string `json:"status"`      // pending, in_progress, completed
	ActiveForm string `json:"active_form"` // Short display label
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
	Provider  string    `json:"provider,omitempty"` // claude, codex, openclaw
	StartedAt time.Time `json:"started_at"`
}

// MonitorState represents the overall monitoring state
type MonitorState struct {
	Teams     []TeamInfo    `json:"teams"`
	Processes []ProcessInfo `json:"processes"`
	UpdatedAt time.Time     `json:"updated_at"`
}

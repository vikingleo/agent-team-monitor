package parser

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"time"

	"github.com/liaoweijun/agent-team-monitor/pkg/types"
)

// TeamConfig represents the structure of a team config file
type TeamConfig struct {
	Name          string       `json:"name"`
	Description   string       `json:"description"`
	AgentType     string       `json:"agent_type"`
	Members       []TeamMember `json:"members"`
	CreatedAt     string       `json:"created_at"`
	CreatedAtMs   int64        `json:"createdAt"`
	LeadSessionID string       `json:"leadSessionId"`
}

// TeamMember represents a member in the team config
type TeamMember struct {
	Name      string `json:"name"`
	AgentID   string `json:"agentId"`
	AgentType string `json:"agentType"`
	Cwd       string `json:"cwd"`
	Prompt    string `json:"prompt"`
	JoinedAt  int64  `json:"joinedAt"`
}

// ParseTeamConfig parses a team config.json file
func ParseTeamConfig(configPath string) (*types.TeamInfo, error) {
	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, err
	}

	var config TeamConfig
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, err
	}

	// Parse created timestamp (support both created_at RFC3339 and createdAt milliseconds)
	createdAt := time.Now()
	if config.CreatedAtMs > 0 {
		createdAt = time.UnixMilli(config.CreatedAtMs)
	} else if config.CreatedAt != "" {
		if t, err := time.Parse(time.RFC3339, config.CreatedAt); err == nil {
			createdAt = t
		}
	}

	// Convert members
	agents := make([]types.AgentInfo, len(config.Members))
	for i, member := range config.Members {
		// Try to extract working directory from prompt first
		cwd := extractCwdFromPrompt(member.Prompt)
		// Fall back to cwd field if extraction failed
		if cwd == "" {
			cwd = member.Cwd
		}

		agents[i] = types.AgentInfo{
			Name:         member.Name,
			AgentID:      member.AgentID,
			AgentType:    member.AgentType,
			Status:       "unknown",
			JoinedAt:     parseMillisTimestamp(member.JoinedAt),
			LastActivity: time.Now(),
			Cwd:          cwd,
		}
	}

	return &types.TeamInfo{
		Name:          config.Name,
		CreatedAt:     createdAt,
		LeadSessionID: config.LeadSessionID,
		Members:       agents,
		Tasks:         []types.TaskInfo{},
		ConfigPath:    configPath,
	}, nil
}

func parseMillisTimestamp(ms int64) time.Time {
	if ms <= 0 {
		return time.Time{}
	}
	return time.UnixMilli(ms)
}

// ScanTeams scans the teams directory for all team configs
func ScanTeams(teamsDir string) ([]types.TeamInfo, error) {
	var teams []types.TeamInfo

	entries, err := os.ReadDir(teamsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return teams, nil
		}
		return nil, err
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		configPath := filepath.Join(teamsDir, entry.Name(), "config.json")
		if _, err := os.Stat(configPath); err == nil {
			team, err := ParseTeamConfig(configPath)
			if err != nil {
				continue
			}
			teams = append(teams, *team)
		}
	}

	return teams, nil
}

// extractCwdFromPrompt extracts working directory from agent prompt
// Looks for patterns like "你的工作目录是 /path/to/dir" or "working directory is /path/to/dir"
func extractCwdFromPrompt(prompt string) string {
	if prompt == "" {
		return ""
	}

	// Chinese pattern: "你的工作目录是 /path/to/directory"
	reCN := regexp.MustCompile(`你的工作目录是\s+(/[^\s\n]+)`)
	if matches := reCN.FindStringSubmatch(prompt); len(matches) > 1 {
		return matches[1]
	}

	// English pattern: "working directory is /path/to/directory"
	reEN := regexp.MustCompile(`(?i)working\s+directory\s+is\s+(/[^\s\n]+)`)
	if matches := reEN.FindStringSubmatch(prompt); len(matches) > 1 {
		return matches[1]
	}

	return ""
}

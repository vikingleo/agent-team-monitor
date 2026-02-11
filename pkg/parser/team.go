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
	Name        string        `json:"name"`
	Description string        `json:"description"`
	AgentType   string        `json:"agent_type"`
	Members     []TeamMember  `json:"members"`
	CreatedAt   string        `json:"created_at"`
}

// TeamMember represents a member in the team config
type TeamMember struct {
	Name      string `json:"name"`
	AgentID   string `json:"agentId"`
	AgentType string `json:"agentType"`
	Cwd       string `json:"cwd"`
	Prompt    string `json:"prompt"`
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

	// Parse created_at timestamp
	createdAt := time.Now()
	if config.CreatedAt != "" {
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
			LastActivity: time.Now(),
			Cwd:          cwd,
		}
	}

	return &types.TeamInfo{
		Name:       config.Name,
		CreatedAt:  createdAt,
		Members:    agents,
		Tasks:      []types.TaskInfo{},
		ConfigPath: configPath,
	}, nil
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
// Looks for patterns like "你的工作目录是 /path/to/directory"
func extractCwdFromPrompt(prompt string) string {
	if prompt == "" {
		return ""
	}

	// Pattern to match: "你的工作目录是 /path/to/directory"
	re := regexp.MustCompile(`你的工作目录是\s+(/[^\s\n]+)`)
	matches := re.FindStringSubmatch(prompt)
	if len(matches) > 1 {
		return matches[1]
	}

	return ""
}

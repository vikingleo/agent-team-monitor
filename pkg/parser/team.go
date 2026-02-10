package parser

import (
	"encoding/json"
	"os"
	"path/filepath"
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
		agents[i] = types.AgentInfo{
			Name:         member.Name,
			AgentID:      member.AgentID,
			AgentType:    member.AgentType,
			Status:       "unknown",
			LastActivity: time.Now(),
			Cwd:          member.Cwd,
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

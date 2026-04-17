package parser

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// ClaudeSessionDiscovery is summarized runtime metadata inferred from one
// ~/.claude/sessions/<pid>.json file.
type ClaudeSessionDiscovery struct {
	PID        int32
	SessionID  string
	Cwd        string
	StartedAt  time.Time
	LastSeenAt time.Time
	Kind       string
	Entrypoint string
}

type claudeSessionFile struct {
	PID        int32  `json:"pid"`
	SessionID  string `json:"sessionId"`
	Cwd        string `json:"cwd"`
	StartedAt  int64  `json:"startedAt"`
	Kind       string `json:"kind"`
	Entrypoint string `json:"entrypoint"`
}

// DiscoverClaudeSessions scans ~/.claude/sessions and returns standalone CLI sessions.
func DiscoverClaudeSessions(sessionsDir string, maxAge time.Duration) ([]ClaudeSessionDiscovery, error) {
	if strings.TrimSpace(sessionsDir) == "" {
		return nil, nil
	}

	sessionFiles, err := filepath.Glob(filepath.Join(sessionsDir, "*.json"))
	if err != nil {
		return nil, err
	}
	if len(sessionFiles) == 0 {
		return nil, nil
	}

	now := time.Now()
	discovered := make([]ClaudeSessionDiscovery, 0, len(sessionFiles))
	for _, sessionPath := range sessionFiles {
		info, err := os.Stat(sessionPath)
		if err != nil || info.IsDir() {
			continue
		}

		session, err := inspectClaudeSessionFile(sessionPath)
		if err != nil {
			continue
		}
		if session.SessionID == "" {
			continue
		}

		session.LastSeenAt = info.ModTime()
		if session.StartedAt.IsZero() {
			session.StartedAt = session.LastSeenAt
		}

		latest := session.LastSeenAt
		if latest.IsZero() || session.StartedAt.After(latest) {
			latest = session.StartedAt
		}
		if maxAge > 0 && !latest.IsZero() && now.Sub(latest) > maxAge {
			continue
		}

		discovered = append(discovered, session)
	}

	sort.SliceStable(discovered, func(i, j int) bool {
		return discovered[i].StartedAt.After(discovered[j].StartedAt)
	})

	return discovered, nil
}

func inspectClaudeSessionFile(sessionPath string) (ClaudeSessionDiscovery, error) {
	data, err := os.ReadFile(sessionPath)
	if err != nil {
		return ClaudeSessionDiscovery{}, err
	}

	var decoded claudeSessionFile
	if err := json.Unmarshal(data, &decoded); err != nil {
		return ClaudeSessionDiscovery{}, err
	}

	return ClaudeSessionDiscovery{
		PID:        decoded.PID,
		SessionID:  strings.TrimSpace(decoded.SessionID),
		Cwd:        strings.TrimSpace(decoded.Cwd),
		StartedAt:  parseMillisTimestamp(decoded.StartedAt),
		Kind:       strings.TrimSpace(decoded.Kind),
		Entrypoint: strings.TrimSpace(decoded.Entrypoint),
	}, nil
}

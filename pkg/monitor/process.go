package monitor

import (
	"sort"
	"strings"
	"time"

	"github.com/liaoweijun/agent-team-monitor/pkg/types"
	"github.com/shirou/gopsutil/v3/process"
)

// ProcessMonitor monitors Claude Code processes
type ProcessMonitor struct{}

// NewProcessMonitor creates a new process monitor
func NewProcessMonitor() *ProcessMonitor {
	return &ProcessMonitor{}
}

// FindProcesses finds all running Claude/Codex processes.
func (pm *ProcessMonitor) FindProcesses(provider ProviderMode) ([]types.ProcessInfo, error) {
	processes, err := process.Processes()
	if err != nil {
		return nil, err
	}

	mode := normalizeProviderMode(provider)
	var monitored []types.ProcessInfo

	for _, p := range processes {
		cmdline, err := p.Cmdline()
		if err != nil {
			continue
		}
		cmdLower := strings.ToLower(cmdline)

		matchedProvider := ""
		if mode.IncludesClaude() && isClaudeProcess(cmdLower) {
			matchedProvider = "claude"
		} else if mode.IncludesCodex() && isCodexProcess(cmdLower) {
			matchedProvider = "codex"
		}
		if matchedProvider == "" {
			continue
		}

		createTime, _ := p.CreateTime()
		startedAt := time.Unix(createTime/1000, 0)

		monitored = append(monitored, types.ProcessInfo{
			PID:       p.Pid,
			Command:   cmdline,
			StartedAt: startedAt,
			Provider:  matchedProvider,
		})
	}

	sort.SliceStable(monitored, func(i, j int) bool {
		return monitored[i].StartedAt.After(monitored[j].StartedAt)
	})

	return monitored, nil
}

// FindClaudeProcesses keeps backward compatibility with older callsites.
func (pm *ProcessMonitor) FindClaudeProcesses() ([]types.ProcessInfo, error) {
	return pm.FindProcesses(ProviderClaude)
}

// IsProcessRunning checks if a process with given PID is still running
func (pm *ProcessMonitor) IsProcessRunning(pid int32) bool {
	exists, err := process.PidExists(pid)
	if err != nil {
		return false
	}
	return exists
}

// isClaudeCodeBinary checks if the command line refers to a claude-code binary
func isClaudeCodeBinary(cmdLower string) bool {
	parts := strings.Fields(cmdLower)
	if len(parts) == 0 {
		return false
	}
	exe := parts[0]
	return exe == "claude" || strings.HasSuffix(exe, "/claude")
}

func isClaudeProcess(cmdLower string) bool {
	return strings.Contains(cmdLower, "@anthropic-ai/claude-code") ||
		strings.Contains(cmdLower, "claude-code") ||
		isClaudeCodeBinary(cmdLower)
}

func isCodexProcess(cmdLower string) bool {
	parts := strings.Fields(cmdLower)
	if len(parts) == 0 {
		return false
	}

	exe := parts[0]
	if exe == "codex" || strings.HasSuffix(exe, "/codex") {
		return true
	}

	if strings.Contains(cmdLower, "/codex app-server") ||
		strings.Contains(cmdLower, "/codex exec") ||
		strings.Contains(cmdLower, "@openai/codex") ||
		strings.Contains(cmdLower, "/linux-x86_64/codex ") ||
		strings.Contains(cmdLower, "/codex/codex ") {
		return true
	}

	// Node wrappers often run codex from package paths.
	if strings.Contains(exe, "node") {
		return strings.Contains(cmdLower, "/codex ") || strings.Contains(cmdLower, "/codex/")
	}

	return false
}

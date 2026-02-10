package monitor

import (
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

// FindClaudeProcesses finds all running Claude Code processes
func (pm *ProcessMonitor) FindClaudeProcesses() ([]types.ProcessInfo, error) {
	processes, err := process.Processes()
	if err != nil {
		return nil, err
	}

	var claudeProcesses []types.ProcessInfo
	for _, p := range processes {
		cmdline, err := p.Cmdline()
		if err != nil {
			continue
		}

		// Look for claude-code or node processes running Claude
		if strings.Contains(cmdline, "claude") ||
		   strings.Contains(cmdline, "claude-code") ||
		   strings.Contains(cmdline, "@anthropic-ai/claude-code") {

			createTime, _ := p.CreateTime()
			startedAt := time.Unix(createTime/1000, 0)

			claudeProcesses = append(claudeProcesses, types.ProcessInfo{
				PID:       p.Pid,
				Command:   cmdline,
				StartedAt: startedAt,
			})
		}
	}

	return claudeProcesses, nil
}

// IsProcessRunning checks if a process with given PID is still running
func (pm *ProcessMonitor) IsProcessRunning(pid int32) bool {
	exists, err := process.PidExists(pid)
	if err != nil {
		return false
	}
	return exists
}

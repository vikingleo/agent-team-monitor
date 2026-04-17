package parser

import "strings"

func normalizeToolEventText(toolName, detail string) string {
	toolName = strings.TrimSpace(toolName)
	detail = strings.TrimSpace(detail)

	if toolName == "" {
		return detail
	}
	if detail == "" {
		return toolName
	}
	return toolName + " · " + detail
}

func classifyToolCall(toolName, detail string) (string, string) {
	if isTerminalToolName(toolName) || isTerminalActivityText(detail) {
		return "terminal", "终端命令"
	}
	return "tool", "工具调用"
}

func classifyToolResult(toolName, text string) (string, string) {
	if isTerminalToolName(toolName) || isTerminalActivityText(text) {
		return "terminal_output", "终端输出"
	}
	return "tool_result", "工具结果"
}

func isTerminalToolName(toolName string) bool {
	switch strings.ToLower(strings.TrimSpace(toolName)) {
	case "bash", "exec_command", "terminal", "shell", "run_terminal_cmd", "powershell", "pwsh", "cmd", "sh", "zsh":
		return true
	}

	lower := strings.ToLower(strings.TrimSpace(toolName))
	return strings.Contains(lower, "terminal") || strings.Contains(lower, "shell")
}

func isTerminalActivityText(text string) bool {
	lower := strings.ToLower(strings.TrimSpace(text))
	if lower == "" {
		return false
	}

	terms := []string{
		"bash",
		"terminal",
		"shell",
		"powershell",
		"pwsh",
		"cmd",
		"zsh -lc",
		"sh -lc",
		"/bin/zsh",
		"/bin/sh",
		"exec_command",
		"unified_exec",
		"pty",
	}
	for _, term := range terms {
		if strings.Contains(lower, term) {
			return true
		}
	}

	return false
}

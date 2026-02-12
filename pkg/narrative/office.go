package narrative

import (
	"fmt"
	"strings"
	"time"

	"github.com/liaoweijun/agent-team-monitor/pkg/types"
)

func GroupTasksByOwner(members []types.AgentInfo, tasks []types.TaskInfo) (map[string][]types.TaskInfo, []types.TaskInfo) {
	agentNames := make(map[string]bool)
	for _, member := range members {
		agentNames[member.Name] = true
	}

	tasksByOwner := make(map[string][]types.TaskInfo)
	var unassigned []types.TaskInfo

	for _, task := range tasks {
		owner := task.Owner
		if owner == "" && task.Subject != "" && agentNames[task.Subject] {
			owner = task.Subject
		}

		if owner == "" {
			unassigned = append(unassigned, task)
			continue
		}

		tasksByOwner[owner] = append(tasksByOwner[owner], task)
	}

	return tasksByOwner, unassigned
}

func RoleEmoji(agentName string) string {
	name := strings.ToLower(agentName)

	switch {
	case strings.Contains(name, "lead"):
		return "🧑‍💼"
	case strings.Contains(name, "api"):
		return "👨‍💻"
	case strings.Contains(name, "admin"):
		return "🧑‍🔧"
	case strings.Contains(name, "vue"):
		return "🧑‍🎨"
	case strings.Contains(name, "uniapp"):
		return "🧑‍📱"
	default:
		return "🧑"
	}
}

func BuildAgentDialogues(agent types.AgentInfo, tasks []types.TaskInfo, now time.Time) []string {
	var dialogues []string

	showCurrentTask := agent.CurrentTask != "" && agent.CurrentTask != agent.Name
	activeTask := pickActiveTask(tasks)

	if showCurrentTask {
		dialogues = append(dialogues,
			fmt.Sprintf("我正在推进「%s」", NormalizeDialogText(agent.CurrentTask, 60)),
		)
	} else if activeTask != nil {
		dialogues = append(dialogues,
			fmt.Sprintf("我在处理任务 #%s：%s", activeTask.ID, NormalizeDialogText(activeTask.Subject, 52)),
		)
	}

	if agent.LastToolUse != "" {
		toolDetail := ""
		if agent.LastToolDetail != "" {
			toolDetail = fmt.Sprintf("（%s）", NormalizeDialogText(agent.LastToolDetail, 45))
		}
		dialogues = append(dialogues,
			fmt.Sprintf("我刚使用了 %s%s", agent.LastToolUse, toolDetail),
		)
	}

	if agent.LastThinking != "" {
		dialogues = append(dialogues,
			fmt.Sprintf("我在想：%s", NormalizeDialogText(agent.LastThinking, 90)),
		)
	}

	if agent.MessageSummary != "" {
		dialogues = append(dialogues,
			fmt.Sprintf("我刚收到：%s", NormalizeDialogText(agent.MessageSummary, 90)),
		)
	}

	if len(dialogues) == 0 {
		switch agent.Status {
		case "working":
			dialogues = append(dialogues, "我正专注处理中，稍后同步最新进展。")
		case "completed":
			dialogues = append(dialogues, "我这边已完成本轮工作，等待下一项安排。")
		default:
			dialogues = append(dialogues, "我这边空闲待命，随时可以接新任务。")
		}
	}

	if relative := FormatRelativeTime(agent.LastActiveTime, now); relative != "" {
		dialogues = append(dialogues, fmt.Sprintf("我最后一次动作是 %s", relative))
	}

	if len(dialogues) > 3 {
		return dialogues[:3]
	}
	return dialogues
}

func NormalizeDialogText(text string, maxLen int) string {
	if text == "" {
		return ""
	}

	normalized := strings.Join(strings.Fields(strings.TrimSpace(text)), " ")
	runes := []rune(normalized)
	if len(runes) <= maxLen {
		return normalized
	}

	return string(runes[:maxLen]) + "..."
}

func FormatRelativeTime(lastActive time.Time, now time.Time) string {
	if lastActive.IsZero() || lastActive.Year() <= 1971 {
		return ""
	}
	if now.IsZero() {
		now = time.Now()
	}

	delta := now.Sub(lastActive)
	if delta < 0 {
		delta = 0
	}

	seconds := int(delta.Seconds())
	if seconds < 60 {
		return fmt.Sprintf("%d秒前", seconds)
	}

	minutes := seconds / 60
	if minutes < 60 {
		return fmt.Sprintf("%d分钟前", minutes)
	}

	hours := minutes / 60
	if hours < 24 {
		return fmt.Sprintf("%d小时前", hours)
	}

	days := hours / 24
	return fmt.Sprintf("%d天前", days)
}

func pickActiveTask(tasks []types.TaskInfo) *types.TaskInfo {
	for i := range tasks {
		if tasks[i].Status == "in_progress" {
			return &tasks[i]
		}
	}
	if len(tasks) == 0 {
		return nil
	}
	return &tasks[0]
}

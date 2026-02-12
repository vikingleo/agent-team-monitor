package ui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/liaoweijun/agent-team-monitor/pkg/monitor"
	"github.com/liaoweijun/agent-team-monitor/pkg/narrative"
	"github.com/liaoweijun/agent-team-monitor/pkg/types"
)

var (
	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#7D56F4")).
			MarginBottom(1)

	teamStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("#874BFD")).
			Padding(1, 2).
			MarginBottom(1)

	agentStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#04B575")).
			MarginLeft(2)

	taskStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FFA500")).
			MarginLeft(4)

	processStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FF6B6B")).
			MarginLeft(2)

	statusWorkingStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#00FF00")).
				Bold(true)

	statusIdleStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FFFF00"))

	statusCompletedStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#888888"))

	officeSectionStyle = lipgloss.NewStyle().
				Underline(true).
				Foreground(lipgloss.Color("#A88CFF"))

	officeHintStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#888888")).
			MarginLeft(2)

	dialoguePrimaryStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#EFEFFF")).
				Background(lipgloss.Color("#2F335F")).
				Padding(0, 1).
				MarginLeft(4)

	dialogueSecondaryStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#D8DBF8")).
				Background(lipgloss.Color("#242846")).
				Padding(0, 1).
				MarginLeft(4)

	agentMetaStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#888888")).
			MarginLeft(4)

	taskTitleStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#9AA0D6")).
			MarginLeft(4)

	broadcastStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FFA500")).
			MarginLeft(2)

	taskOverviewStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#E0E0E0")).
				MarginLeft(2)
)

type model struct {
	collector *monitor.Collector
	state     types.MonitorState
	width     int
	height    int
}

type tickMsg time.Time

func tickCmd() tea.Cmd {
	return tea.Tick(time.Second, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

func NewModel(collector *monitor.Collector) model {
	return model{
		collector: collector,
		state:     collector.GetState(),
	}
}

func (m model) Init() tea.Cmd {
	return tickCmd()
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit
		case "r":
			// Manual refresh
			m.state = m.collector.GetState()
		}

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

	case tickMsg:
		// Update state periodically
		m.state = m.collector.GetState()
		return m, tickCmd()
	}

	return m, nil
}

func (m model) View() string {
	var b strings.Builder

	// Title
	title := titleStyle.Render("🤖 Claude Agent Team 监控器")
	b.WriteString(title)
	b.WriteString("\n\n")

	// Last updated
	lastUpdate := fmt.Sprintf("最后更新: %s", m.state.UpdatedAt.Format("15:04:05"))
	b.WriteString(lipgloss.NewStyle().Faint(true).Render(lastUpdate))
	b.WriteString("\n\n")

	// Processes section
	b.WriteString(lipgloss.NewStyle().Bold(true).Render("📊 Claude 进程"))
	b.WriteString(fmt.Sprintf(" (运行中: %d)\n", len(m.state.Processes)))
	if len(m.state.Processes) == 0 {
		b.WriteString(processStyle.Render("  未检测到 Claude 进程\n"))
	} else {
		for _, proc := range m.state.Processes {
			uptime := time.Since(proc.StartedAt).Round(time.Second)
			procInfo := fmt.Sprintf("  进程 ID: %d | 运行时间: %s", proc.PID, uptime)
			b.WriteString(processStyle.Render(procInfo))
			b.WriteString("\n")
		}
	}
	b.WriteString("\n")

	// Teams section
	b.WriteString(lipgloss.NewStyle().Bold(true).Render("👥 活动团队"))
	b.WriteString(fmt.Sprintf(" (共 %d 个)\n\n", len(m.state.Teams)))

	if len(m.state.Teams) == 0 {
		b.WriteString(teamStyle.Render("未找到活动团队"))
	} else {
		for _, team := range m.state.Teams {
			teamContent := m.renderTeam(team)
			b.WriteString(teamStyle.Render(teamContent))
			b.WriteString("\n")
		}
	}

	// Help
	b.WriteString("\n")
	help := lipgloss.NewStyle().Faint(true).Render("按 'r' 刷新 | 按 'q' 退出")
	b.WriteString(help)

	return b.String()
}

func (m model) renderTeam(team types.TeamInfo) string {
	var b strings.Builder

	b.WriteString(lipgloss.NewStyle().Bold(true).Render(fmt.Sprintf("团队: %s", team.Name)))
	b.WriteString("\n")
	b.WriteString(lipgloss.NewStyle().Faint(true).Render(fmt.Sprintf("创建时间: %s", team.CreatedAt.Format("2006-01-02 15:04"))))
	b.WriteString("\n\n")

	workingCount := 0
	for _, agent := range team.Members {
		if agent.Status == "working" {
			workingCount++
		}
	}

	tasksByOwner, unassignedTasks := narrative.GroupTasksByOwner(team.Members, team.Tasks)

	b.WriteString(officeSectionStyle.Render(
		fmt.Sprintf("🏢 办公区实况 (%d 位同事, %d 位忙碌中)", len(team.Members), workingCount)))
	b.WriteString("\n")
	b.WriteString(officeHintStyle.Render("每位成员用“人话”同步当前状态、思路和工具动作。"))
	b.WriteString("\n")

	if len(team.Members) == 0 {
		b.WriteString(agentStyle.Render("  无成员"))
		b.WriteString("\n")
	} else {
		for _, agent := range team.Members {
			b.WriteString(m.renderAgentDesk(agent, tasksByOwner[agent.Name]))
		}
	}

	if len(unassignedTasks) > 0 {
		b.WriteString(m.renderBroadcastDesk(unassignedTasks))
	}

	b.WriteString("\n")
	b.WriteString(officeSectionStyle.Render(fmt.Sprintf("📋 任务总览 (%d 项)", len(team.Tasks))))
	b.WriteString("\n")

	if len(team.Tasks) == 0 {
		b.WriteString(taskOverviewStyle.Render("  暂无任务"))
		b.WriteString("\n")
	} else {
		for _, task := range team.Tasks {
			owner := task.Owner
			if owner == "" {
				owner = "未分配"
			}
			statusStr := m.formatTaskStatus(task.Status)
			taskLine := fmt.Sprintf("  #%s %s %s · 负责人: %s",
				task.ID,
				statusStr,
				narrative.NormalizeDialogText(task.Subject, 40),
				owner,
			)
			b.WriteString(taskOverviewStyle.Render(taskLine))
			b.WriteString("\n")
		}
	}

	return b.String()
}

func (m model) renderAgentDesk(agent types.AgentInfo, tasks []types.TaskInfo) string {
	var b strings.Builder

	header := fmt.Sprintf("  %s %s [%s] · %s",
		m.agentRoleEmoji(agent),
		agent.Name,
		agent.AgentType,
		m.formatStatus(agent.Status),
	)
	b.WriteString(agentStyle.Render(header))
	b.WriteString("\n")

	dialogues := m.agentDialogues(agent, tasks)
	for i, dialogue := range dialogues {
		prefix := "💬"
		style := dialoguePrimaryStyle
		if i > 0 {
			prefix = "🗨"
			style = dialogueSecondaryStyle
		}

		b.WriteString(style.Render(fmt.Sprintf("%s %s", prefix, dialogue)))
		b.WriteString("\n")
	}

	if agent.Cwd != "" {
		b.WriteString(agentMetaStyle.Render(fmt.Sprintf("📁 %s", agent.Cwd)))
		b.WriteString("\n")
	}

	if len(tasks) > 0 {
		b.WriteString(taskTitleStyle.Render("我手上的任务"))
		b.WriteString("\n")
		for _, task := range tasks {
			statusStr := m.formatTaskStatus(task.Status)
			taskLine := fmt.Sprintf("    %s %s %s",
				task.ID,
				statusStr,
				narrative.NormalizeDialogText(task.Subject, 46),
			)
			b.WriteString(taskStyle.Render(taskLine))
			b.WriteString("\n")
		}
	}

	b.WriteString("\n")
	return b.String()
}

func (m model) renderBroadcastDesk(tasks []types.TaskInfo) string {
	var b strings.Builder

	b.WriteString(broadcastStyle.Render(fmt.Sprintf("  📣 前台广播 [%d 条待认领任务]", len(tasks))))
	b.WriteString("\n")
	b.WriteString(dialoguePrimaryStyle.Render(
		fmt.Sprintf("💬 有 %d 项任务暂未分配，欢迎同事主动认领。", len(tasks)),
	))
	b.WriteString("\n")

	for _, task := range tasks {
		statusStr := m.formatTaskStatus(task.Status)
		taskLine := fmt.Sprintf("    %s %s %s",
			task.ID,
			statusStr,
			narrative.NormalizeDialogText(task.Subject, 46),
		)
		b.WriteString(taskStyle.Render(taskLine))
		b.WriteString("\n")
	}

	b.WriteString("\n")
	return b.String()
}

func (m model) agentDialogues(agent types.AgentInfo, tasks []types.TaskInfo) []string {
	if len(agent.OfficeDialogues) > 0 {
		return agent.OfficeDialogues
	}

	return narrative.BuildAgentDialogues(agent, tasks, time.Now())
}

func (m model) agentRoleEmoji(agent types.AgentInfo) string {
	if agent.RoleEmoji != "" {
		return agent.RoleEmoji
	}

	return narrative.RoleEmoji(agent.Name)
}

func (m model) formatStatus(status string) string {
	switch status {
	case "working":
		return statusWorkingStyle.Render("工作中")
	case "idle":
		return statusIdleStyle.Render("空闲")
	case "completed":
		return statusCompletedStyle.Render("已完成")
	default:
		return status
	}
}

func (m model) formatTaskStatus(status string) string {
	switch status {
	case "in_progress":
		return statusWorkingStyle.Render("进行中")
	case "pending":
		return statusIdleStyle.Render("待处理")
	case "completed":
		return statusCompletedStyle.Render("已完成")
	default:
		return status
	}
}

// Run starts the TUI application
func Run(collector *monitor.Collector) error {
	p := tea.NewProgram(NewModel(collector), tea.WithAltScreen())
	_, err := p.Run()
	return err
}

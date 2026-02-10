package ui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/liaoweijun/agent-team-monitor/pkg/monitor"
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

	// Team header
	b.WriteString(lipgloss.NewStyle().Bold(true).Render(fmt.Sprintf("团队: %s", team.Name)))
	b.WriteString("\n")
	b.WriteString(lipgloss.NewStyle().Faint(true).Render(fmt.Sprintf("创建时间: %s", team.CreatedAt.Format("2006-01-02 15:04"))))
	b.WriteString("\n\n")

	// Agents
	b.WriteString(lipgloss.NewStyle().Underline(true).Render("成员:"))
	b.WriteString("\n")
	if len(team.Members) == 0 {
		b.WriteString(agentStyle.Render("  无成员"))
		b.WriteString("\n")
	} else {
		for _, agent := range team.Members {
			statusStr := m.formatStatus(agent.Status)
			agentInfo := fmt.Sprintf("  • %s [%s] - %s", agent.Name, agent.AgentType, statusStr)
			if agent.CurrentTask != "" {
				agentInfo += fmt.Sprintf(" (任务: %s)", agent.CurrentTask)
			}
			b.WriteString(agentStyle.Render(agentInfo))
			b.WriteString("\n")
		}
	}
	b.WriteString("\n")

	// Tasks
	b.WriteString(lipgloss.NewStyle().Underline(true).Render("任务:"))
	b.WriteString("\n")
	if len(team.Tasks) == 0 {
		b.WriteString(taskStyle.Render("    无任务"))
		b.WriteString("\n")
	} else {
		for _, task := range team.Tasks {
			statusStr := m.formatTaskStatus(task.Status)
			owner := task.Owner
			if owner == "" {
				owner = "未分配"
			}
			taskInfo := fmt.Sprintf("    [%s] %s - %s (%s)", task.ID, task.Subject, statusStr, owner)
			b.WriteString(taskStyle.Render(taskInfo))
			b.WriteString("\n")
		}
	}

	return b.String()
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

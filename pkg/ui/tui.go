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
	collector      *monitor.Collector
	state          types.MonitorState
	width          int
	height         int
	providerFilter string
	hideIdleAgents bool
}

type providerStats struct {
	AllTeams     int
	AllAgents    int
	ClaudeTeams  int
	ClaudeAgents int
	CodexTeams   int
	CodexAgents  int
}

type tickMsg time.Time

func tickCmd() tea.Cmd {
	return tea.Tick(time.Second, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

func NewModel(collector *monitor.Collector) model {
	return model{
		collector:      collector,
		state:          collector.GetState(),
		providerFilter: "all",
		hideIdleAgents: true,
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
		case "1", "a":
			m.providerFilter = "all"
		case "2", "c":
			m.providerFilter = "claude"
		case "3", "o":
			m.providerFilter = "codex"
		case "i":
			m.hideIdleAgents = !m.hideIdleAgents
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
	teams, processes, stats := m.filteredState()

	// Title
	title := titleStyle.Render("🤖 Claude + Codex Agent Team 监控器")
	b.WriteString(title)
	b.WriteString("\n\n")

	// Last updated
	lastUpdate := fmt.Sprintf("最后更新: %s", m.state.UpdatedAt.Format("15:04:05"))
	b.WriteString(lipgloss.NewStyle().Faint(true).Render(lastUpdate))
	b.WriteString("\n")
	b.WriteString(lipgloss.NewStyle().Faint(true).Render(m.filterSummary(stats)))
	b.WriteString("\n\n")

	// Processes section
	b.WriteString(lipgloss.NewStyle().Bold(true).Render("📊 代理进程"))
	b.WriteString(fmt.Sprintf(" (运行中: %d)\n", len(processes)))
	if len(processes) == 0 {
		b.WriteString(processStyle.Render("  未检测到代理进程\n"))
	} else {
		for _, proc := range processes {
			uptime := time.Since(proc.StartedAt).Round(time.Second)
			provider := detectProcessProvider(proc)
			procInfo := fmt.Sprintf("  进程 ID: %d | 来源: %s | 运行时间: %s", proc.PID, provider, uptime)
			b.WriteString(processStyle.Render(procInfo))
			b.WriteString("\n")
		}
	}
	b.WriteString("\n")

	// Teams section
	b.WriteString(lipgloss.NewStyle().Bold(true).Render("👥 活动团队"))
	b.WriteString(fmt.Sprintf(" (共 %d 个)\n\n", len(teams)))

	if len(teams) == 0 {
		b.WriteString(teamStyle.Render("未找到活动团队"))
	} else {
		for _, team := range teams {
			teamContent := m.renderTeam(team)
			b.WriteString(teamStyle.Render(teamContent))
			b.WriteString("\n")
		}
	}

	// Help
	b.WriteString("\n")
	help := lipgloss.NewStyle().Faint(true).Render("按 '1/2/3' 切换筛选 | 按 'i' 切换空闲隐藏 | 按 'r' 刷新 | 按 'q' 退出")
	b.WriteString(help)

	return b.String()
}

func (m model) renderTeam(team types.TeamInfo) string {
	var b strings.Builder

	provider := detectTeamProvider(team)
	if provider != "unknown" {
		b.WriteString(lipgloss.NewStyle().Bold(true).Render(fmt.Sprintf("团队: %s [%s]", team.Name, provider)))
	} else {
		b.WriteString(lipgloss.NewStyle().Bold(true).Render(fmt.Sprintf("团队: %s", team.Name)))
	}
	b.WriteString("\n")
	b.WriteString(lipgloss.NewStyle().Faint(true).Render(fmt.Sprintf("创建时间: %s", team.CreatedAt.Format("2006-01-02 15:04"))))
	if team.ProjectCwd != "" {
		b.WriteString("\n")
		b.WriteString(lipgloss.NewStyle().Faint(true).Render(fmt.Sprintf("工作目录: %s", team.ProjectCwd)))
	}
	b.WriteString("\n\n")

	displayMembers := visibleMembers(team.Members, m.hideIdleAgents)

	workingCount := 0
	for _, agent := range displayMembers {
		if agent.Status == "working" {
			workingCount++
		}
	}

	tasksByOwner, unassignedTasks := narrative.GroupTasksByOwner(displayMembers, team.Tasks)

	b.WriteString(officeSectionStyle.Render(
		fmt.Sprintf("🏢 办公区实况 (%d 位同事, %d 位忙碌中)", len(displayMembers), workingCount)))
	b.WriteString("\n")
	b.WriteString(officeHintStyle.Render("每位成员用“人话”同步当前状态、思路和工具动作。"))
	b.WriteString("\n")

	if len(displayMembers) == 0 {
		b.WriteString(agentStyle.Render("  无成员"))
		b.WriteString("\n")
	} else {
		for _, agent := range displayMembers {
			b.WriteString(m.renderAgentDesk(agent, tasksByOwner[agent.Name]))
		}
	}

	if len(unassignedTasks) > 0 {
		b.WriteString(m.renderBroadcastDesk(unassignedTasks))
	}

	return b.String()
}

func visibleMembers(members []types.AgentInfo, hideIdle bool) []types.AgentInfo {
	if !hideIdle {
		return append([]types.AgentInfo(nil), members...)
	}

	result := make([]types.AgentInfo, 0, len(members))
	for _, member := range members {
		if member.Status == "idle" {
			continue
		}
		result = append(result, member)
	}
	return result
}

func (m model) filteredState() ([]types.TeamInfo, []types.ProcessInfo, providerStats) {
	stats := providerStats{}
	teams := make([]types.TeamInfo, 0, len(m.state.Teams))

	for _, team := range m.state.Teams {
		provider := detectTeamProvider(team)
		members := visibleMembers(team.Members, m.hideIdleAgents)

		if !shouldKeepTeam(team, members, m.hideIdleAgents) {
			continue
		}

		stats.AllTeams++
		stats.AllAgents += len(members)
		switch provider {
		case "claude":
			stats.ClaudeTeams++
			stats.ClaudeAgents += len(members)
		case "codex":
			stats.CodexTeams++
			stats.CodexAgents += len(members)
		}

		if m.providerFilter != "all" && provider != m.providerFilter {
			continue
		}

		teamCopy := team
		teamCopy.Members = members
		teams = append(teams, teamCopy)
	}

	processes := make([]types.ProcessInfo, 0, len(m.state.Processes))
	for _, proc := range m.state.Processes {
		provider := detectProcessProvider(proc)
		if m.providerFilter != "all" && provider != m.providerFilter {
			continue
		}
		processes = append(processes, proc)
	}

	return teams, processes, stats
}

func (m model) filterSummary(stats providerStats) string {
	hideIdle := "开"
	if !m.hideIdleAgents {
		hideIdle = "关"
	}

	return fmt.Sprintf(
		"筛选: [1]全部(team:%d,agent:%d) [2]Claude(team:%d,agent:%d) [3]Codex(team:%d,agent:%d) | 当前:%s | 自动隐藏空闲:%s",
		stats.AllTeams,
		stats.AllAgents,
		stats.ClaudeTeams,
		stats.ClaudeAgents,
		stats.CodexTeams,
		stats.CodexAgents,
		strings.ToUpper(m.providerFilter),
		hideIdle,
	)
}

func shouldKeepTeam(team types.TeamInfo, members []types.AgentInfo, hideIdle bool) bool {
	if !hideIdle {
		return true
	}

	if len(members) > 0 {
		return true
	}

	for _, task := range team.Tasks {
		if strings.ToLower(strings.TrimSpace(task.Status)) != "completed" {
			return true
		}
	}

	return false
}

func detectTeamProvider(team types.TeamInfo) string {
	provider := strings.ToLower(strings.TrimSpace(team.Provider))
	if provider == "claude" || provider == "codex" {
		return provider
	}

	for _, member := range team.Members {
		memberProvider := strings.ToLower(strings.TrimSpace(member.Provider))
		if memberProvider == "claude" || memberProvider == "codex" {
			return memberProvider
		}
	}

	if strings.HasPrefix(strings.ToLower(strings.TrimSpace(team.Name)), "codex-") {
		return "codex"
	}
	return "unknown"
}

func detectProcessProvider(proc types.ProcessInfo) string {
	provider := strings.ToLower(strings.TrimSpace(proc.Provider))
	if provider == "claude" || provider == "codex" {
		return provider
	}

	cmd := strings.ToLower(strings.TrimSpace(proc.Command))
	if strings.Contains(cmd, "codex") {
		return "codex"
	}
	if strings.Contains(cmd, "claude") {
		return "claude"
	}
	return "unknown"
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

	if len(agent.Todos) > 0 {
		b.WriteString(taskTitleStyle.Render("📝 待办清单"))
		b.WriteString("\n")
		for _, todo := range agent.Todos {
			icon := "  "
			switch todo.Status {
			case "in_progress":
				icon = "🔄"
			case "completed":
				icon = "✅"
			default:
				icon = "⬜"
			}
			label := todo.Content
			if todo.ActiveForm != "" && todo.Status == "in_progress" {
				label = todo.ActiveForm
			}
			todoLine := fmt.Sprintf("    %s %s", icon, narrative.NormalizeDialogText(label, 46))
			b.WriteString(taskStyle.Render(todoLine))
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

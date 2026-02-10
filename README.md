# 🤖 Claude Agent Team Monitor

A real-time monitoring tool for Claude Code agent teams. Track all your agent teams, their members, tasks, and activity status in a beautiful terminal interface.

## ✨ Features

- 🔍 **Process Monitoring**: Automatically detects running Claude Code processes
- 👥 **Team Tracking**: Monitors all active agent teams from `~/.claude/teams/`
- 📋 **Task Management**: Displays task status and ownership for each team
- 🎨 **Beautiful TUI**: Clean, colorful terminal interface built with Bubble Tea
- ⚡ **Real-time Updates**: File system watching + periodic polling for instant updates
- 🖥️ **Cross-platform**: Works on macOS and Linux

## 🏗️ Architecture

```
agent-team-monitor/
├── cmd/monitor/          # Main application entry point
├── pkg/
│   ├── monitor/          # Core monitoring logic
│   │   ├── process.go    # Claude process detection
│   │   ├── filesystem.go # File system watching
│   │   ├── collector.go  # Data aggregation
│   │   └── types.go      # Data structures
│   ├── parser/           # Configuration parsers
│   │   ├── team.go       # Team config parser
│   │   └── task.go       # Task file parser
│   └── ui/               # Terminal UI
│       └── tui.go        # Bubble Tea interface
```

## 🚀 Quick Start

### Prerequisites

- Go 1.21 or higher
- Claude Code installed and configured

### Installation

```bash
# Clone the repository
git clone <your-repo-url>
cd agent-team-monitor

# Install dependencies
make install

# Build the application
make build

# Run the monitor
make run
```

### Usage

```bash
# Run directly
./bin/agent-team-monitor

# Or use make
make run

# Install globally (optional)
make install-global
agent-team-monitor
```

## 🎮 Controls

- `r` - Manual refresh
- `q` or `Ctrl+C` - Quit

## 📊 What It Monitors

### Process Information
- Running Claude Code processes (PID, uptime)
- Process command line

### Team Information
- Team name and creation time
- Team members (agents)
- Agent status: WORKING, IDLE, COMPLETED
- Current task assignment

### Task Information
- Task ID and subject
- Task status: PENDING, IN PROGRESS, COMPLETED
- Task owner (agent name)

## 🛠️ Technical Stack

- **Language**: Go 1.25
- **TUI Framework**: [Bubble Tea](https://github.com/charmbracelet/bubbletea) + [Lipgloss](https://github.com/charmbracelet/lipgloss)
- **Process Monitoring**: [gopsutil](https://github.com/shirou/gopsutil)
- **File System Watching**: [fsnotify](https://github.com/fsnotify/fsnotify)

## 🔧 Configuration

The monitor automatically watches:
- `~/.claude/teams/` - Team configuration files
- `~/.claude/tasks/` - Task status files

No additional configuration required!

## 📦 Building for Multiple Platforms

```bash
# Build for all supported platforms
make build-all

# Output:
# bin/agent-team-monitor-darwin-amd64  (macOS Intel)
# bin/agent-team-monitor-darwin-arm64  (macOS Apple Silicon)
# bin/agent-team-monitor-linux-amd64   (Linux x86_64)
# bin/agent-team-monitor-linux-arm64   (Linux ARM64)
```

## 🧪 Testing

```bash
make test
```

## 🤝 Contributing

Contributions are welcome! Please feel free to submit a Pull Request.

## 📝 License

MIT License

## 🙏 Acknowledgments

- Built for monitoring [Claude Code](https://github.com/anthropics/claude-code) agent teams
- UI powered by [Charm](https://charm.sh/) libraries

## 📸 Screenshots

The monitor displays:
- Real-time process information
- Team hierarchy with agent status
- Task lists with ownership and status
- Color-coded status indicators
- Auto-refreshing display

## 🐛 Troubleshooting

### No teams detected
- Ensure Claude Code has created teams in `~/.claude/teams/`
- Check that team config files exist and are valid JSON

### No processes detected
- Make sure Claude Code is running
- The monitor looks for processes containing "claude" or "claude-code"

### Permission errors
- Ensure you have read access to `~/.claude/` directory
- On Linux, you may need to adjust file permissions

## 🔮 Future Enhancements

- [ ] Web dashboard interface
- [ ] Historical data tracking
- [ ] Performance metrics
- [ ] Alert notifications
- [ ] Export to JSON/CSV
- [ ] Agent communication logs
- [ ] Resource usage monitoring

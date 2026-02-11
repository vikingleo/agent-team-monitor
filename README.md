# Claude Agent Team Monitor

[中文](#中文) | [English](#english)

---

<a id="中文"></a>

[Claude Code](https://docs.anthropic.com/en/docs/claude-code) 智能体团队实时监控面板。在终端或浏览器中追踪团队成员、任务状态、智能体思考过程、工具调用和进程信息。

## 截图

### Web 面板
![Web Dashboard](static/web.png)

### 终端界面 (TUI)
![Terminal UI](static/TUI.png)

## 功能特性

- **团队总览** — 查看所有活跃的智能体团队、成员、角色和状态
- **任务追踪** — 任务按负责人分组展示，实时状态更新
- **智能体活动** — 实时显示思考过程 (💭)、工具调用 (🔧)、消息摘要 (📨)
- **进程监控** — 追踪运行中的 Claude Code 进程及运行时长
- **双模式** — 终端 UI 和 Web 面板布局一致
- **文件监听** — 基于 fsnotify 监听 `~/.claude/teams/` 和 `~/.claude/tasks/`
- **自动刷新** — 两种模式均支持 1 秒智能更新

## 快速开始

```bash
git clone https://github.com/liaoweijun/agent-team-monitor.git
cd agent-team-monitor
make build
```

### TUI 模式（默认）

```bash
./bin/agent-team-monitor
```

| 按键 | 操作 |
|------|------|
| `r` | 手动刷新 |
| `q` / `Ctrl+C` | 退出 |

### Web 模式

```bash
./bin/agent-team-monitor -web

# 自定义端口
./bin/agent-team-monitor -web -addr :3000
```

浏览器打开 `http://localhost:8080`。

## API 接口

```
GET /api/state      # 完整监控状态
GET /api/teams      # 团队信息
GET /api/processes  # 进程信息
GET /api/health     # 健康检查
```

```bash
curl http://localhost:8080/api/state | jq
```

## 工作原理

监控器监听 Claude Code 智能体的文件系统：

```
~/.claude/
├── teams/{team-name}/config.json       # 团队配置与成员
├── tasks/{team-name}/*.json            # 任务定义与状态
├── teams/{team-name}/inboxes/          # 智能体收件箱
└── projects/*/activity.jsonl           # 智能体活动日志
```

## 项目结构

```
cmd/monitor/main.go              入口 & 模式选择
pkg/
├── types/types.go                共享数据结构
├── monitor/
│   ├── collector.go              数据聚合中心
│   ├── filesystem.go             fsnotify 文件监听
│   └── process.go                系统进程扫描
├── parser/
│   ├── team.go                   团队配置解析
│   ├── task.go                   任务文件解析
│   ├── inbox.go                  收件箱解析
│   └── activity.go               活动日志解析
├── api/
│   └── server.go                 HTTP 服务 & REST API
└── ui/
    └── tui.go                    终端 UI (Bubble Tea)
web/static/                       Web 前端 (HTML/CSS/JS)
```

## 跨平台构建

```bash
make build-all
```

输出 macOS (amd64/arm64) 和 Linux (amd64/arm64) 的二进制文件。

## 技术栈

- [Bubble Tea](https://github.com/charmbracelet/bubbletea) + [Lip Gloss](https://github.com/charmbracelet/lipgloss) — 终端 UI
- [fsnotify](https://github.com/fsnotify/fsnotify) — 文件系统监听
- [gopsutil](https://github.com/shirou/gopsutil) — 进程监控

## 常见问题

**未检测到团队** — 确认 Claude Code 已在 `~/.claude/teams/` 下创建团队，且 config.json 文件有效。

**未检测到进程** — 确认 Claude Code 正在运行。监控器扫描包含 "claude" 的进程。

**权限错误** — 确认对 `~/.claude/` 目录有读取权限。

## 许可证

MIT

---

<a id="english"></a>

## English

Real-time monitoring dashboard for [Claude Code](https://docs.anthropic.com/en/docs/claude-code) agent teams. Track team members, tasks, agent thinking, tool usage, and processes — in your terminal or browser.

## Screenshots

### Web Dashboard
![Web Dashboard](static/web.png)

### Terminal UI (TUI)
![Terminal UI](static/TUI.png)

## Features

- **Team Overview** — All active agent teams, members, roles, and status at a glance
- **Task Tracking** — Tasks grouped by assigned agent with real-time status
- **Agent Activity** — Live display of thinking (💭), tool usage (🔧), and messages (📨)
- **Process Monitoring** — Running Claude Code processes with uptime
- **Dual Mode** — Terminal UI and Web dashboard with consistent layout
- **File Watching** — fsnotify-based monitoring of `~/.claude/teams/` and `~/.claude/tasks/`
- **Auto Refresh** — 1-second smart updates in both modes

## Quick Start

```bash
git clone https://github.com/liaoweijun/agent-team-monitor.git
cd agent-team-monitor
make build
```

### TUI Mode (default)

```bash
./bin/agent-team-monitor
```

| Key | Action |
|-----|--------|
| `r` | Manual refresh |
| `q` / `Ctrl+C` | Quit |

### Web Mode

```bash
./bin/agent-team-monitor -web

# Custom port
./bin/agent-team-monitor -web -addr :3000
```

Open `http://localhost:8080` in your browser.

## API Endpoints

```
GET /api/state      # Complete monitoring state
GET /api/teams      # Team information
GET /api/processes  # Process information
GET /api/health     # Health check
```

```bash
curl http://localhost:8080/api/state | jq
```

## How It Works

The monitor watches the Claude Code agent filesystem:

```
~/.claude/
├── teams/{team-name}/config.json       # Team config & members
├── tasks/{team-name}/*.json            # Task definitions & status
├── teams/{team-name}/inboxes/          # Agent inbox messages
└── projects/*/activity.jsonl           # Agent activity logs
```

## Architecture

```
cmd/monitor/main.go              Entry point & mode selection
pkg/
├── types/types.go                Shared data structures
├── monitor/
│   ├── collector.go              Central data aggregation
│   ├── filesystem.go             fsnotify file watcher
│   └── process.go                OS process scanner
├── parser/
│   ├── team.go                   Team config parser
│   ├── task.go                   Task file parser
│   ├── inbox.go                  Agent inbox parser
│   └── activity.go               Activity log parser
├── api/
│   └── server.go                 HTTP server & REST API
└── ui/
    └── tui.go                    Terminal UI (Bubble Tea)
web/static/                       Web dashboard (HTML/CSS/JS)
```

## Cross-Platform Build

```bash
make build-all
```

Outputs binaries for macOS (amd64/arm64) and Linux (amd64/arm64).

## Tech Stack

- [Bubble Tea](https://github.com/charmbracelet/bubbletea) + [Lip Gloss](https://github.com/charmbracelet/lipgloss) — Terminal UI
- [fsnotify](https://github.com/fsnotify/fsnotify) — Filesystem watching
- [gopsutil](https://github.com/shirou/gopsutil) — Process monitoring

## Troubleshooting

**No teams detected** — Ensure Claude Code has created teams in `~/.claude/teams/` with valid config.json files.

**No processes detected** — Make sure Claude Code is running. The monitor scans for processes containing "claude".

**Permission errors** — Ensure read access to `~/.claude/` directory.

## License

MIT

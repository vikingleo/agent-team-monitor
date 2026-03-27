# Agent Team Monitor

[中文](#中文) | [English](#english)

---

<a id="中文"></a>

[Claude Code](https://docs.anthropic.com/en/docs/claude-code) + Codex 智能体团队实时监控面板。在桌面窗口、终端或 Web 页面中追踪团队成员、任务状态、智能体思考过程、工具调用和进程信息。

## 截图

### Web 面板

![Web Dashboard](static/web.png)

### 终端界面 (TUI)

![Terminal UI](static/TUI.png)

## 功能特性

- **团队总览** — 查看所有活跃的智能体团队、成员、角色和状态
- **任务追踪** — 任务按负责人分组展示，实时状态更新
- **智能体活动** — 实时显示思考过程 (💭)、工具调用 (🔧)、消息摘要 (📨)
- **进程监控** — 追踪运行中的 Claude Code / Codex 进程及运行时长
- **双模式** — 终端 UI 和 Web 面板布局一致
- **文件监听** — 基于 fsnotify 监听 `~/.claude/teams/`、`~/.claude/tasks/`、`~/.claude/projects/`、`~/.codex/sessions/`
- **自动刷新** — 两种模式均支持 1 秒智能更新

## 快速开始

```bash
git clone https://github.com/liaoweijun/agent-team-monitor.git
cd agent-team-monitor
make build
```

### 桌面应用

```bash
make build-desktop
./bin/agent-team-monitor-desktop
```

桌面版会直接启动一个桌面壳应用窗口，在应用内完整显示现有 Web 监控页面和办公场景，不再调用外部浏览器。

- 主窗口内完整加载 Web 监控面板和 `/game/` 办公场景
- 保持 Web 界面的原有布局、动画、主题和交互，不做裁剪重写
- 可通过 Ayatana AppIndicator 系统托盘在后台继续运行
- 关闭窗口时可按设置隐藏到托盘，或直接退出应用
- 桌面设置会持久化到本机配置目录，而不是依赖浏览器缓存
- 提供独立的原生“设置”与“关于”窗口
- 外部文档链接会交给系统默认浏览器打开
- 支持系统通知：任务完成、成员长时间无活动

当前桌面版与 Web 版共用同一套采集与监控数据，并直接复用同一套前端页面；桌面端额外补充托盘、桌面设置、系统通知和开机自启等能力。

如果你希望它像普通 Linux 桌面软件一样出现在应用菜单、任务栏和搜索结果中，可以安装桌面入口和图标：

```bash
make install-desktop-entry
```

安装后会注册图标名 `agent-team-monitor`，同时：

- Linux 桌面菜单会显示应用图标
- 桌面窗口会使用相同应用图标
- Web 页面标签会显示 favicon

### TUI 模式（默认）

```bash
./bin/agent-team-monitor

# 仅监控 codex
./bin/agent-team-monitor -provider codex

# 同时监控 claude + codex
./bin/agent-team-monitor -provider both
```

| 按键           | 操作     |
| -------------- | -------- |
| `r`            | 手动刷新 |
| `q` / `Ctrl+C` | 退出     |

### Web 模式

```bash
./bin/agent-team-monitor -web

# 自定义端口
./bin/agent-team-monitor -web -addr :3000

# 随机端口
./bin/agent-team-monitor -web -addr 127.0.0.1:0

# Web + 仅 codex
./bin/agent-team-monitor -web -provider codex
```

默认地址为 `http://localhost:8080`。如果使用随机端口，程序会打印实际地址，例如 `http://localhost:49152`。

- 默认暗色监控面板：`/`
- 办公场景游戏视图：`/game/`
- 也支持单地址切换：`/?view=game`、`/game/?view=dark`

### Linux 部署脚本

仓库内置了一个适合 Linux 服务器部署的管理脚本：

```bash
chmod +x ./scripts/service.sh

# 默认以 web 模式启动，端口 8080
./scripts/service.sh start

# 查看状态
./scripts/service.sh status

# 快速重启
./scripts/service.sh restart

# 停止
./scripts/service.sh stop

# 查看日志
./scripts/service.sh logs
```

也可以通过环境变量控制启动参数：

```bash
ATM_MODE=web ATM_PORT=3000 ATM_PROVIDER=both ./scripts/service.sh start
```

默认行为：

- PID 文件：`run/agent-team-monitor.pid`
- 标准日志：`logs/agent-team-monitor.out.log`
- 错误日志：`logs/agent-team-monitor.err.log`
- 默认直接执行 `scripts/agent-team-monitor`
- 可通过 `ATM_APP_BIN` 指定其他二进制路径

## API 接口

```
GET /api/state      # 完整监控状态
GET /api/teams      # 团队信息
GET /api/processes  # 进程信息
GET /api/health     # 健康检查
```

```bash
curl http://localhost:3000/api/state | jq
```

## 环境变量

- `ATM_EXPOSE_ABS_PATHS` — 默认 `false`，设置为 `true/yes/on` 后，API 返回绝对路径（否则脱敏）
- `ATM_DISCOVERY_METRICS` — 默认 `false`，设置为 `true/yes/on` 后，输出 team 发现链路性能日志（耗时、缓存命中率、命中数）

## 工作原理

监控器监听 Claude Code 智能体的文件系统：

```
~/.claude/
├── teams/{team-name}/config.json       # 团队配置与成员
├── tasks/{team-name}/*.json            # 任务定义与状态
├── teams/{team-name}/inboxes/          # 智能体收件箱
├── projects/{project}/{session}.jsonl  # team lead 根会话日志
└── projects/{project}/{session}/subagents/agent-*.jsonl # 成员会话日志

~/.codex/
└── sessions/YYYY/MM/DD/rollout-*.jsonl # Codex 会话日志
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

Linux 桌面应用可单独构建：

```bash
make build-desktop
```

Linux 桌面入口可一并安装：

```bash
make install-desktop-entry
```

也可以直接构建 Debian 安装包：

```bash
make package-deb
```

构建后的 `.deb` 文件会输出到 `dist/` 目录。

`.deb` 包内包含：

- `/usr/bin/agent-team-monitor`
- `/usr/bin/agent-team-monitor-desktop`
- Linux desktop entry
- 128/256/512 多尺寸图标
- Debian changelog 与 copyright

## 技术栈

- [Bubble Tea](https://github.com/charmbracelet/bubbletea) + [Lip Gloss](https://github.com/charmbracelet/lipgloss) — 终端 UI
- [fsnotify](https://github.com/fsnotify/fsnotify) — 文件系统监听
- [gopsutil](https://github.com/shirou/gopsutil) — 进程监控

## 常见问题

**未检测到团队** — 监控器会从 `~/.claude/teams/`、`~/.claude/tasks/`、`~/.claude/projects/` 和 `~/.codex/sessions/` 发现活动。若仍为空，请检查这些目录是否有最近数据。

**未检测到进程** — 确认 Claude Code 或 Codex 正在运行。监控器会扫描 `claude` / `codex` 相关进程。

**权限错误** — 确认对 `~/.claude/` 与 `~/.codex/` 目录有读取权限。

## 许可证

MIT

---

<a id="english"></a>

## English

Real-time monitoring dashboard for [Claude Code](https://docs.anthropic.com/en/docs/claude-code) + Codex agent teams. Track team members, tasks, agent thinking, tool usage, and processes in a desktop window, terminal, or browser.

## Screenshots

### Web Dashboard

![Web Dashboard](static/web.png)

### Terminal UI (TUI)

![Terminal UI](static/TUI.png)

## Features

- **Team Overview** — All active agent teams, members, roles, and status at a glance
- **Task Tracking** — Tasks grouped by assigned agent with real-time status
- **Agent Activity** — Live display of thinking (💭), tool usage (🔧), and messages (📨)
- **Process Monitoring** — Running Claude Code / Codex processes with uptime
- **Dual Mode** — Terminal UI and Web dashboard with consistent layout
- **File Watching** — fsnotify-based monitoring of `~/.claude/teams/`, `~/.claude/tasks/`, `~/.claude/projects/`, and `~/.codex/sessions/`
- **Auto Refresh** — 1-second smart updates in both modes

## Quick Start

```bash
git clone https://github.com/liaoweijun/agent-team-monitor.git
cd agent-team-monitor
make build
```

### Desktop App

```bash
make build-desktop
./bin/agent-team-monitor-desktop
```

The desktop build starts a native Linux GTK window and renders the monitoring panel directly inside the desktop app instead of launching an external browser.

- It reads monitoring data directly from the collector instead of depending on an embedded browser view
- The main team list, members, and task summaries are rendered by the desktop app itself
- External links still open in your system browser
- The app supports tray residency, close-to-tray, native settings/about windows, and desktop notifications
- You can enable `开机自动启动桌面应用` and `启动时直接驻留托盘` from the desktop settings window

To install a menu entry and icons for the current user:

```bash
make install-desktop-entry
```

### TUI Mode (default)

```bash
./bin/agent-team-monitor

# Codex only
./bin/agent-team-monitor -provider codex

# Claude + Codex
./bin/agent-team-monitor -provider both
```

| Key            | Action         |
| -------------- | -------------- |
| `r`            | Manual refresh |
| `q` / `Ctrl+C` | Quit           |

### Web Mode

```bash
./bin/agent-team-monitor -web

# Custom port
./bin/agent-team-monitor -web -addr :3000

# Random port
./bin/agent-team-monitor -web -addr 127.0.0.1:0

# Web + codex only
./bin/agent-team-monitor -web -provider codex
```

The default address is `http://localhost:8080`. When using a random port, the program prints the resolved address.

The browser tab uses the packaged app favicon from `web/static/assets/favicon.png`.

The Linux desktop app now runs as a desktop shell window that embeds the full Web dashboard and office scene instead of launching your browser.

## API Endpoints

```
GET /api/state      # Complete monitoring state
GET /api/teams      # Team information
GET /api/processes  # Process information
GET /api/health     # Health check
```

```bash
curl http://localhost:3000/api/state | jq
```

## Environment Variables

- `ATM_EXPOSE_ABS_PATHS` — default `false`; set `true/yes/on` to expose absolute paths in API output
- `ATM_DISCOVERY_METRICS` — default `false`; set `true/yes/on` to log discovery performance metrics (latency, cache hit rate, hit counts)

## How It Works

The monitor watches the Claude Code agent filesystem:

```
~/.claude/
├── teams/{team-name}/config.json       # Team config & members
├── tasks/{team-name}/*.json            # Task definitions & status
├── teams/{team-name}/inboxes/          # Agent inbox messages
├── projects/{project}/{session}.jsonl  # Team lead root session log
└── projects/{project}/{session}/subagents/agent-*.jsonl # Member session logs

~/.codex/
└── sessions/YYYY/MM/DD/rollout-*.jsonl # Codex session logs
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

The Linux desktop app can be built separately:

```bash
make build-desktop
```

Install the Linux desktop entry and icons:

```bash
make install-desktop-entry
```

Build a Debian package:

```bash
make package-deb
```

The generated `.deb` is written to `dist/`.

After installation, launch `Agent Team Monitor` from your desktop menu. In desktop mode it runs as a native Linux window rather than opening a browser tab, and it can continue running from the tray in the background.

## Tech Stack

- [Bubble Tea](https://github.com/charmbracelet/bubbletea) + [Lip Gloss](https://github.com/charmbracelet/lipgloss) — Terminal UI
- [fsnotify](https://github.com/fsnotify/fsnotify) — Filesystem watching
- [gopsutil](https://github.com/shirou/gopsutil) — Process monitoring

## Troubleshooting

**No teams detected** — The monitor discovers activity from `~/.claude/teams/`, `~/.claude/tasks/`, `~/.claude/projects/`, and `~/.codex/sessions/`. If still empty, verify recent activity exists in those directories.

**No processes detected** — Make sure Claude Code or Codex is running. The monitor scans `claude` / `codex` related processes.

**Permission errors** — Ensure read access to both `~/.claude/` and `~/.codex/` directories.

## License

MIT

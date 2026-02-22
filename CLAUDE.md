# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

请始终使用简体中文进行回复。

## 常用命令

```bash
make build              # 构建到 bin/agent-team-monitor
make test               # 运行测试 (go test -v ./...)
make run                # TUI 模式运行
make run-web            # Web 模式运行 (端口 8080)
make run-web-port PORT=3000  # 自定义端口
make build-all          # 跨平台构建 (macOS/Linux/Windows, amd64/arm64)
make release V=v1.3.0   # 一键发布到 GitHub Release
make install            # go mod download && go mod tidy
```

单个测试运行：`go test -v -run TestFuncName ./pkg/parser/`

## 架构概览

Go 项目，为 Claude Code 智能体团队提供实时监控面板，支持 TUI 和 Web 双模式。

### 数据流

```
~/.claude/teams/ & ~/.claude/tasks/
        │
        ▼
  FileSystemMonitor (fsnotify)  ──┐
  ProcessMonitor (gopsutil)     ──┼──▶ Collector ──▶ TUI (Bubble Tea)
  Parser (team/task/inbox/       ─┘       │              或
          activity/todo)                  └────▶ API Server ──▶ Web 前端
```

### 核心模块

- `cmd/monitor/main.go` — 入口，通过 `-web` 标志选择 TUI 或 Web 模式
- `pkg/monitor/collector.go` — 数据聚合中心，5 秒周期更新 + 100ms 防抖事件驱动更新
- `pkg/monitor/filesystem.go` — fsnotify 文件监听器，监听 `~/.claude/teams/` 和 `~/.claude/tasks/`
- `pkg/monitor/process.go` — 扫描系统中包含 "claude" 的进程
- `pkg/parser/` — 解析团队配置、任务、收件箱、活动日志 (JSONL)、TodoWrite 项
- `pkg/types/types.go` — 所有共享数据结构 (TeamInfo, AgentInfo, TaskInfo 等)
- `pkg/api/server.go` — HTTP 服务器，REST API (`/api/state`, `/api/teams`, `/api/processes`, `/api/health`)
- `pkg/ui/tui.go` — Bubble Tea 终端界面
- `pkg/narrative/office.go` — 智能体角色 emoji 和对话生成
- `web/static/` — 嵌入式 Web 前端 (HTML/CSS/JS)，通过 `web/embed.go` 嵌入二进制

### 关键设计

- Web 静态资源通过 Go `embed` 打包进二进制，无需外部文件
- Collector 使用 `sync.RWMutex` 保护状态，支持并发读取
- 文件变更通过 channel 通知，带防抖机制避免频繁刷新
- 过期团队自动清理（30 分钟无活动阈值）

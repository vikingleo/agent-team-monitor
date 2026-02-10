# Agent Team Monitor

## 项目概述

这是一个用于监控 Claude Code agent teams 的实时监控工具。它可以追踪所有活动的 agent team、成员状态、任务进度，并在终端中以美观的界面展示。

## ✨ 核心功能

- 🔍 **进程监控**: 自动检测运行中的 Claude Code 进程
- 👥 **团队追踪**: 监控 `~/.claude/teams/` 中的所有活动团队
- 📋 **任务管理**: 显示每个团队的任务状态和所有权
- 🎨 **美观的 TUI**: 使用 Bubble Tea 构建的彩色终端界面
- ⚡ **实时更新**: 文件系统监控 + 定期轮询，确保即时更新
- 🖥️ **跨平台**: 兼容 macOS 和 Linux

## 🏗️ 技术架构

### 技术栈
- **语言**: Go 1.25
- **TUI 框架**: Bubble Tea + Lipgloss
- **进程监控**: gopsutil
- **文件监控**: fsnotify

### 项目结构
```
agent-team-monitor/
├── cmd/monitor/          # 主程序入口
├── pkg/
│   ├── monitor/          # 核心监控逻辑
│   │   ├── process.go    # Claude 进程检测
│   │   ├── filesystem.go # 文件系统监控
│   │   └── collector.go  # 数据聚合器
│   ├── parser/           # 配置解析器
│   │   ├── team.go       # 团队配置解析
│   │   └── task.go       # 任务文件解析
│   ├── types/            # 数据类型定义
│   │   └── types.go      # 共享类型
│   └── ui/               # 终端界面
│       └── tui.go        # Bubble Tea 界面
├── Makefile              # 构建脚本
└── test-setup.sh         # 测试环境设置脚本
```

## 🚀 快速开始

### 前置要求
- Go 1.21 或更高版本
- Claude Code 已安装并配置

### 安装步骤

```bash
# 1. 克隆仓库
cd agent-team-monitor

# 2. 安装依赖
make install

# 3. 构建应用
make build

# 4. 运行监控器
make run
```

### 使用测试数据

如果你想在没有真实 Claude teams 的情况下测试监控器：

```bash
# 创建测试数据
./test-setup.sh

# 运行监控器
./bin/agent-team-monitor

# 清理测试数据
rm -rf ~/.claude/teams/test-team ~/.claude/tasks/test-team
```

## 🎮 操作说明

- `r` - 手动刷新
- `q` 或 `Ctrl+C` - 退出

## 📊 监控内容

### 进程信息
- 运行中的 Claude Code 进程（PID、运行时间）
- 进程命令行

### 团队信息
- 团队名称和创建时间
- 团队成员（agents）
- Agent 状态：WORKING（工作中）、IDLE（空闲）、COMPLETED（已完成）
- 当前任务分配

### 任务信息
- 任务 ID 和主题
- 任务状态：PENDING（待处理）、IN PROGRESS（进行中）、COMPLETED（已完成）
- 任务所有者（agent 名称）

## 🔧 配置

监控器自动监控以下目录：
- `~/.claude/teams/` - 团队配置文件
- `~/.claude/tasks/` - 任务状态文件

无需额外配置！

## 📦 多平台构建

```bash
# 构建所有支持的平台
make build-all

# 输出文件：
# bin/agent-team-monitor-darwin-amd64  (macOS Intel)
# bin/agent-team-monitor-darwin-arm64  (macOS Apple Silicon)
# bin/agent-team-monitor-linux-amd64   (Linux x86_64)
# bin/agent-team-monitor-linux-arm64   (Linux ARM64)
```

## 🧪 测试

```bash
make test
```

## 🛠️ 开发

### 构建命令
```bash
make build      # 构建应用
make run        # 运行应用
make clean      # 清理构建产物
make install    # 安装依赖
```

### 全局安装
```bash
make install-global
# 之后可以直接运行：
agent-team-monitor
```

## 🔍 工作原理

### 监控策略
监控器采用混合模式：

1. **文件系统监控**（实时）
   - 使用 fsnotify 监听 `~/.claude/teams/` 和 `~/.claude/tasks/`
   - 文件变化时立即触发更新

2. **定期轮询**（每 5 秒）
   - 确保不遗漏任何变化
   - 更新进程信息

3. **智能防抖**
   - 文件系统事件有 100ms 防抖
   - 避免频繁更新

### Agent 状态判断
- **WORKING**: Agent 拥有状态为 `in_progress` 的任务
- **IDLE**: Agent 没有进行中的任务
- **COMPLETED**: Agent 的任务已完成

## 🐛 故障排除

### 未检测到团队
- 确保 Claude Code 已在 `~/.claude/teams/` 中创建团队
- 检查团队配置文件是否存在且为有效 JSON

### 未检测到进程
- 确保 Claude Code 正在运行
- 监控器查找包含 "claude" 或 "claude-code" 的进程

### 权限错误
- 确保你有 `~/.claude/` 目录的读取权限
- 在 Linux 上，可能需要调整文件权限

## 🔮 未来增强

- [ ] Web 仪表板界面
- [ ] 历史数据追踪
- [ ] 性能指标
- [ ] 告警通知
- [ ] 导出为 JSON/CSV
- [ ] Agent 通信日志
- [ ] 资源使用监控
- [ ] 支持多个 Claude 实例
- [ ] 配置文件支持

## 📝 许可证

MIT License

## 🙏 致谢

- 为监控 [Claude Code](https://github.com/anthropics/claude-code) agent teams 而构建
- UI 由 [Charm](https://charm.sh/) 库提供支持

## 📧 反馈

如有问题或建议，欢迎提交 Issue 或 Pull Request！

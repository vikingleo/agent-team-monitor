# 🌐 Web Dashboard 使用指南

## 概述

Agent Team Monitor 现在支持两种运行模式：
1. **TUI 模式** - 终端界面（默认）
2. **Web 模式** - Web 仪表板（新增）

## 启动 Web 模式

### 方式 1: 使用 Make 命令

```bash
# 启动 Web 服务器（默认端口 8080）
make run-web

# 使用自定义端口
make run-web-port PORT=3000
```

### 方式 2: 直接运行

```bash
# 默认端口 8080
./bin/agent-team-monitor -web

# 自定义端口
./bin/agent-team-monitor -web -addr :3000

# 自定义地址和端口
./bin/agent-team-monitor -web -addr 0.0.0.0:8080
```

### 方式 3: 开发模式

```bash
# 开发模式（自动重载）
make dev-web
```

## 访问 Web 界面

启动后，在浏览器中访问：

```
http://localhost:8080
```

你会看到一个现代化的 Web 仪表板，包含：
- 实时更新的进程信息
- 所有活动团队的卡片视图
- Agent 状态和任务进度
- 自动刷新（每秒）

## Web API 端点

Web 模式提供以下 REST API 端点：

### 获取完整状态
```bash
GET /api/state
```
返回所有监控数据（teams + processes）

### 获取团队信息
```bash
GET /api/teams
```
只返回团队数据

### 获取进程信息
```bash
GET /api/processes
```
只返回进程数据

### 健康检查
```bash
GET /api/health
```
返回服务器健康状态

## 示例 API 调用

```bash
# 获取完整状态
curl http://localhost:8080/api/state | jq

# 获取团队列表
curl http://localhost:8080/api/teams | jq

# 获取进程列表
curl http://localhost:8080/api/processes | jq

# 健康检查
curl http://localhost:8080/api/health
```

## Web 界面功能

### 1. 实时监控
- 自动刷新（每秒）
- 连接状态指示器
- 最后更新时间显示

### 2. 进程视图
- 显示所有 Claude Code 进程
- PID 和运行时间
- 实时更新

### 3. 团队视图
- 卡片式布局
- 每个团队显示：
  - 团队名称和创建时间
  - Agent 列表和状态
  - 任务列表和进度

### 4. 状态指示器
- 🟢 **WORKING** - Agent 正在工作
- 🟡 **IDLE** - Agent 空闲
- ⚪ **COMPLETED** - 任务已完成

### 5. 响应式设计
- 支持桌面和移动设备
- 自适应布局
- 流畅的动画效果

## 性能优化

### 自动暂停
当浏览器标签页不可见时，自动停止更新以节省资源。

### 轻量级
- 纯 JavaScript（无框架）
- 最小化 HTTP 请求
- 高效的 DOM 更新

## 安全考虑

### CORS 支持
默认允许所有来源（开发模式）。生产环境建议配置特定来源。

### 只读 API
所有 API 端点都是只读的，不会修改任何数据。

### 本地访问
默认绑定到 `localhost`，只能本地访问。如需远程访问，使用：
```bash
./bin/agent-team-monitor -web -addr 0.0.0.0:8080
```

## 对比：TUI vs Web

| 特性 | TUI 模式 | Web 模式 |
|------|---------|---------|
| 界面 | 终端 | 浏览器 |
| 访问方式 | 本地终端 | HTTP 浏览器 |
| 远程访问 | ❌ | ✅ |
| 多用户 | ❌ | ✅ |
| API 访问 | ❌ | ✅ |
| 资源占用 | 低 | 中 |
| 适用场景 | 开发调试 | 团队协作 |

## 使用场景

### TUI 模式适合：
- 个人开发调试
- 快速查看状态
- 服务器环境（无 GUI）
- 低资源占用需求

### Web 模式适合：
- 团队协作监控
- 远程访问需求
- 多人同时查看
- 集成到其他系统
- 需要 API 访问

## 故障排除

### 端口被占用
```bash
# 使用其他端口
./bin/agent-team-monitor -web -addr :8081
```

### 无法访问
检查防火墙设置，确保端口开放。

### 数据不更新
检查浏览器控制台是否有错误，刷新页面重试。

### 连接断开
Web 界面会显示 "Disconnected" 状态，自动尝试重连。

## 高级配置

### 反向代理（Nginx）

```nginx
server {
    listen 80;
    server_name monitor.example.com;

    location / {
        proxy_pass http://localhost:8080;
        proxy_http_version 1.1;
        proxy_set_header Upgrade $http_upgrade;
        proxy_set_header Connection 'upgrade';
        proxy_set_header Host $host;
        proxy_cache_bypass $http_upgrade;
    }
}
```

### 使用 systemd 服务

```ini
[Unit]
Description=Claude Agent Team Monitor
After=network.target

[Service]
Type=simple
User=youruser
ExecStart=/usr/local/bin/agent-team-monitor -web -addr :8080
Restart=on-failure

[Install]
WantedBy=multi-user.target
```

## 命令行选项

```bash
# 显示帮助
./bin/agent-team-monitor -h

# 显示版本
./bin/agent-team-monitor -version

# TUI 模式（默认）
./bin/agent-team-monitor

# Web 模式
./bin/agent-team-monitor -web

# 自定义地址
./bin/agent-team-monitor -web -addr :3000
```

## 开发

### 修改前端代码

前端文件位于 `web/static/` 目录：
- `index.html` - HTML 结构
- `css/style.css` - 样式
- `js/app.js` - JavaScript 逻辑

修改后无需重新编译，刷新浏览器即可。

### 修改后端代码

后端文件位于 `pkg/api/` 目录：
- `server.go` - HTTP 服务器和 API 端点
- `websocket.go` - WebSocket 支持（预留）

修改后需要重新编译：
```bash
make build
```

## 未来增强

- [ ] WebSocket 实时推送
- [ ] 历史数据图表
- [ ] 导出功能（JSON/CSV）
- [ ] 用户认证
- [ ] 主题切换（亮色/暗色）
- [ ] 自定义刷新间隔
- [ ] 告警配置
- [ ] 性能指标图表

## 反馈

如有问题或建议，欢迎提交 Issue！

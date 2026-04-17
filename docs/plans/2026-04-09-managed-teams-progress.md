# 2026-04-09 Managed Teams Progress

## 本轮已完成

### 1. Claude 会话发现增强

- 新增 `~/.claude/sessions/*.json` 解析，standalone Claude CLI 会话不再只出现在进程列表，也会纳入团队发现。
- 支持从 root session log 回退读取 `team-lead` 活动，而不是只依赖 subagent log。
- 修复“同一 cwd 下多个 Claude CLI 被错误合并”的问题。

涉及文件：

- `pkg/parser/claude.go`
- `pkg/parser/claude_test.go`
- `pkg/monitor/collector.go`
- `pkg/monitor/collector_claude_sessions_test.go`
- `pkg/monitor/collector_team_discovery_test.go`

### 2. 监控退出时的 panic 修复

- 修复 collector 停止后，`fsnotify` 回调仍向已关闭 channel 写入导致的 `panic: send on closed channel`。

涉及文件：

- `pkg/monitor/collector.go`
- `pkg/monitor/collector_stop_test.go`

### 3. 管理员登录门禁

- 从**可执行文件所在目录**加载 `.env`。
- 新增管理员登录状态：
  - `ATM_ADMIN_USERNAME`
  - `ATM_ADMIN_PASSWORD`
- 未登录时：
  - 只能浏览
  - 不允许发送消息
  - 不允许清理团队
  - 不允许修改桌面设置
- Web 与 Desktop bridge 都统一受控。

涉及文件：

- `internal/app/env.go`
- `cmd/monitor/main.go`
- `cmd/desktop/main.go`
- `pkg/api/auth.go`
- `pkg/api/server.go`
- `cmd/desktop/bridge.go`
- `web/static/index.html`
- `web/static/js/app.js`
- `web/static/js/desktop-ui.js`
- `web/static/css/style.css`

当前开发配置：

- `bin/.env`
- 管理员账号：`admin`
- 管理员密码：`123456`

### 4. 自托管团队 V1 后端闭环

- 新增控制目录：`~/.agent-team-monitor/`
- 新增受管模型：
  - `WorkspaceConfig`
  - `TeamSpec`
  - `AgentSpec`
  - `RunState`
- 新增 `pkg/managed` 管理器：
  - 创建受管团队
  - 按 team 批量启动多个 Claude CLI agent
  - 按 team / agent 停止受管会话
  - 按主 agent 兼容发送消息，并支持定向发到指定 agent
  - 以 `teamID:agentID` 维度维护进程内 active 状态
  - 按 `runs/{teamID}/{agentID}.json` 保存 run 状态
  - 向后兼容读取旧 `runs/{teamID}.json` 并迁移
  - 恢复历史 run 状态并识别 detached 进程

涉及文件：

- `pkg/managed/manager.go`
- `pkg/managed/manager_test.go`
- `go.mod`
- `go.sum`

### 5. 自托管团队管理 API

新增接口：

- `GET /api/managed/teams`
- `POST /api/managed/teams`
- `POST /api/managed/teams/{id}/start`
- `POST /api/managed/teams/{id}/stop`
- `POST /api/managed/teams/{id}/message`
- `POST /api/managed/teams/{id}/agents/{agentID}/start`
- `POST /api/managed/teams/{id}/agents/{agentID}/stop`
- `POST /api/managed/teams/{id}/agents/{agentID}/message`

兼容策略：

- 旧 team 级 `message` 仍可用，默认路由到主 agent
- team 级 `message` body 现也可带 `agent_id`
- agent 级路由与 team 级路由都受管理员登录门禁控制

### 6. 自托管团队 Web UI

在主面板中新增了自托管团队创建区，支持：

- 团队名
- 工作目录
- 模型
- 权限模式
- 创建后立即启动
- 可选首条任务

本轮补充：

- agent 详情面板里，如果该成员来自 managed team，会把消息写入对应 managed agent，而不是固定写到 team lead
- 顶部 managed message form 也兼容 `agent_id` 透传
- 主团队列表中的 managed 成员卡片已补 per-agent 启动 / 停止入口
- managed 团队卡片顶部按钮已改成“全部启动 / 全部停止”批量语义
- managed 团队摘要栏会展示 `受管运行` / `本地可控` 计数
- agent 详情面板里的 managed 成员已补 start / stop，并在弹窗内显示本地操作反馈
- 顶部创建区已明确标注：顶部负责建 team，团队卡片负责批量控制，成员卡片与详情负责单 agent 控制

当前仍未完成：

- 创建区仍是“先建 team，再由列表或详情继续操作”的分段流

### 7. Managed 团队并入主状态视图

- 受管团队已并入 `/api/state.teams`
- managed team 现在会把 `spec.agents` 全部展开到 `members`
- 主团队卡片现在区分：
  - `managed`
  - `imported`
- 受管团队带运行态字段：
  - `control_mode`
  - `managed`
  - `managed_team_id`
  - `managed_status`
  - `controllable`
  - `log_path`
- managed member 现在也会带：
  - `agent_id`
  - `command_transport=managed_pty`
  - `command_reason`
  - `last_thinking` / `last_active_time`

## 实际验证结果

### 已验证通过

```bash
go test ./pkg/api ./pkg/managed
node --check web/static/js/app.js
```

此前已验证通过：

```bash
node --check web/static/js/app.js
node --check web/static/js/desktop-ui.js
node --check web/static/js/desktop-bridge.js
```

### 真实 smoke test 已完成

已实际跑通：

1. 管理员登录
2. 创建受管团队 `Managed Smoke`
3. 启动受管 Claude CLI
4. 下发首条任务
5. 在 `~/.agent-team-monitor/logs/*.log` 中看到任务文本写入 PTY
6. 停止团队

随后又验证了：

- `Managed Auto Start`
- 启动参数带 `--permission-mode acceptEdits`
- 下发任务文本成功进入受管会话

## 当前运行状态

- 后台 Web 服务地址：`http://127.0.0.1:8011`
- 管理员登录已启用
- 当前受管团队会出现在主 `/api/state` 数据中

## 当前边界

### 1. 仍只支持 Claude managed sessions

Codex / OpenClaw 仍然只有导入监控，没有自托管 launcher。

### 2. 多 agent 受管后端已落地，主操作入口已开始切到 per-agent

后端和状态模型已经支持一个 managed team 下多个 agent：

- `active` 已按 `teamID:agentID`
- run 文件已按 `runs/{teamID}/{agentID}.json`
- API 已支持 agent 级 start / stop / message
- `/api/state.teams[].members` 已能看到全部 managed agent

UI 现状：

- 主列表中的 managed member 已支持 per-agent start / stop
- agent 详情面板已支持向指定 managed agent 发消息
- agent 详情面板已支持 managed agent start / stop
- 团队卡片顶部按钮已改为 team 级批量 start / stop

但 UI 还没有完全统一成同一套操作流：

- 创建区仍是单独入口
- team 级批量控制与 agent 级细粒度控制仍分散在不同视图层级

### 3. 受管会话的 PTY 可控性是进程内的

如果监控服务重启：

- run 状态可以恢复识别
- 但已运行进程通常只会标为 `running_detached`
- 暂时不能无损重新接管 PTY

### 4. 当前主界面是“并入数据 + 保留创建区”

Managed 团队已经进入主团队列表，但创建区仍单独保留在顶部，没有完全融合成单一操作流。

## 已知问题

### 1. 仍有 stale virtual team 清理日志噪音

运行日志里仍会出现：

- `Removed orphaned task dir for stale virtual team "default"`
- `Removed orphaned task dir for stale virtual team "claude-law-office"`

这不是本轮 blocker，但属于后续需要单独收口的噪音问题。

### 2. 当前外部 Claude 会话依然多数不可直接发消息

结论已经比较明确：

- `claude-vscode`：私有 socket，当前监控器无法直接注入
- 普通 CLI 导入态：没有稳定可写 inbox 通道

因此“外部导入态会话可直接控制”仍未完成。

## 下一步建议

优先级最高的两项：

1. **把 Managed / Imported 真正统一成一套主操作流**
   目标：用户从主团队卡片直接看到：
   - 是否受管
   - 是否可控
   - 启动 / 停止 / 下发任务
   不再依赖顶部独立创建区理解系统。

2. **把创建区进一步并入主团队视图**
   目标：避免“先在顶部创建，再到下方管理”的两段式心智切换。

建议实现顺序：

1. 把创建区进一步并入主卡片操作流
2. 再考虑为 managed team 提供更紧凑的批量状态概览
3. 最后再评估 managed Codex / OpenClaw launcher

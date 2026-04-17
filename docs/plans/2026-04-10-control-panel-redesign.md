# 2026-04-10 Control Panel Redesign

## 目标

- 将现有“监控面板”升级为“操控面板 / command workspace”。
- 从“团队卡片堆叠 + 弹窗详情”切换为“左侧导航 + 右侧主控台”的常驻操作流。
- 对齐 Claude Code / Codex / Gemini CLI 的工作方式：先选团队或 agent，再查看活动流，最后直接下发指令。
- 保留现有 managed / imported 数据模型和 API，不重做 transport，而是在前端上统一操作入口。
- 新增硬要求：聊天界面必须尽可能呈现 agent 的完整活动，而不是只展示摘要。

## 全活动透明度

- 对所有类型统一要求：
  - Claude
  - Codex
  - OpenClaw
  - 未来接入的 Gemini 等 provider
- 聊天界面应优先展示完整活动流，而不是仅展示状态卡摘要。
- 目标事件集合至少包括：
  - 用户请求 / operator 指令
  - agent 输出
  - thinking / reasoning
  - 工具调用
  - 工具结果
  - 后台终端运行
  - 终端输出
  - 任务 / todo / 状态变化
- 前后端统一原则：
  - 只要底层日志里可观察到，就应尽量进入 `AgentEvent` 流
  - UI 应按时间线展开，而不是折叠到单条 summary
  - 不同 provider 的事件名字可以不同，但在 UI 层要归一成统一活动语义

## 信息架构

### 左侧导航

- 展示全部团队。
- 每个团队卡显示：
  - team 名称
  - provider / managed 标识
  - 成员数、运行中成员数、待处理任务数
- 选中团队后展开该团队的导航层级：
  - `全部活动`
  - 各 agent 条目
- 选中团队时，右侧显示 team 级活动流。
- 选中 agent 时，右侧显示该 agent 的活动流与定向操控区。

### 右侧主控台

- 上半部分：
  - 当前 selection 的详细信息
  - 状态摘要
  - start / stop 等上下文操作
  - 聊天式活动流
- 下半部分：
  - 固定输入区
  - `Enter` 发送
  - `Shift+Enter` 换行
  - 需兼容中文输入法合成态，避免选词时误发送

## 交互模型

### 选择状态

- 前端维护：
  - `selectedTeamName`
  - `selectedAgentKey`
  - `composerDrafts`
  - `pendingOperatorMessages`
- 默认选中第一个可见团队。
- 默认优先选中该团队的第一个可见 agent；用户也可切回 team 级“全部活动”视图。

### 消息路由

- 选中 managed agent：
  - 走 `/api/managed/teams/{teamID}/message`
  - body 带 `agent_id`
- 选中 managed team 且未选中 agent：
  - 走 team 级 `/message`
  - 默认发给主 agent / lead
- 选中 imported Claude agent：
  - 走现有 `/api/agents/message`
- 选中 team 级 imported 视图：
  - 不允许盲发
  - UI 明确提示“先选择具体 agent”

### 活动流

- 聊天流统一展示：
  - operator 下发的指令
  - agent 最近消息 / 输出
  - 工具调用
  - thinking
  - 任务 / 待办变化
- 由于后端尚未持久化 operator 指令历史，前端暂存本地已发送消息并插入活动流，保证对话感和可追溯性。

## 分阶段实施

### 阶段 1

- 新增计划文档。
- 替换 teams tab 主视图为 control workspace。
- 接入 team / agent 选择状态。
- 在右侧实现活动流 + 底部 composer。
- 让 composer 真正可将消息发往当前 selection。

### 阶段 2

- 补强视觉层级：
  - sidebar 导航
  - 主控信息卡
  - 聊天气泡
  - 固定底部输入区
- 统一 managed start / stop 操作入口到右侧 header / summary 区。

### 阶段 3

- 评估是否弱化旧弹窗详情。
- 评估是否把顶部 managed 创建区并入同一工作台流。

## 风险与边界

### 1. 现有 transport 能力不一致

- 当前真正适合“即时操控”的主要是：
  - `managed_pty`
  - `claude_inbox`
- 其他 provider / imported agent 可能仍只能浏览，不能统一支持发送。
- UI 需要显式暴露“可控 / 不可控 / 原因”。

### 2. 自动刷新会影响输入体验

- 当前前端每秒刷新。
- 如果每次刷新都全量重绘，会打断输入。
- 需要在 composer 上做：
  - draft 持久化
  - 焦点与光标恢复
  - IME 合成态下尽量避免重绘

### 3. 第一阶段不强求清理旧代码

- 旧 team card / agent detail modal 暂时保留，降低重构风险。
- 先保证 teams tab 的主工作流完成升级。

## 本轮预期交付

- `docs/plans/2026-04-10-control-panel-redesign.md`
- `web/static/index.html`
- `web/static/js/app.js`
- `web/static/css/style.css`

## 当前进展

### 已完成

- teams 页主视图已切换为“左侧团队 / 成员切换 + 右侧主控区 + 底部指令输入”的操控台布局。
- 输入区已支持：
  - `Enter` 发送
  - `Shift+Enter` 换行
  - 中文输入法合成态保护
  - draft 持久化与刷新后焦点恢复
- 消息路由已接通：
  - 受管团队总览默认发给主成员
  - 受管成员可直接定向发消息
  - imported Claude 成员继续走现有 inbox 通道
- “全活动透明度”已完成一轮后端语义收敛：
  - Claude / Codex / OpenClaw 的活动流统一归并为 `response`、`thinking`、`tool`、`tool_result`、`terminal`、`terminal_output`、`task`、`status`、`message`
  - 前端聊天流按统一语义展示，不再主要依赖 provider 特定文案猜测

### 本轮继续收敛

- 继续清理前端残留的英文 / 半英文用户文案。
- 重点把角色、状态、受管运行态、提示词统一映射为中文，避免直接暴露 `lead`、`working`、`running_detached`、`teammate-message` 等原始值。

## 验证方式

```bash
node --check web/static/js/app.js
```

手工验证重点：

1. 左侧 team / agent 切换是否稳定。
2. 右侧 feed 是否按 selection 更新。
3. `Enter` 发送 / `Shift+Enter` 换行是否符合预期。
4. 中文输入法选词时是否不会误发送。
5. managed agent 与 team lead 的消息路由是否正确。

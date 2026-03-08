# 游戏化办公场景测试指南

## 快速启动

```bash
make run-web
```

然后访问：http://localhost:8080

## 已实现功能

### 核心功能
- ✅ Phaser 3 游戏引擎集成
- ✅ 动态办公室布局（根据团队数量自动调整）
- ✅ Agent 实体渲染（简笔画风格）
- ✅ 状态可视化（busy=绿色，idle=灰色）
- ✅ 呼吸动画（busy 快速，idle 慢速）
- ✅ 功能区渲染（洗手间、茶水间、健身区、老板办公室）
- ✅ 实时数据同步（每秒轮询 /api/state）
- ✅ 增量更新（团队增删、agent 状态变化）

### 交互功能
- ✅ 鼠标滚轮缩放（0.5x - 2x）
- ✅ 鼠标拖拽平移视角
- ✅ 双击重置视角
- ✅ 自动适配视角（首次加载）

### 布局策略
- **小型公司**（1-2 团队）：800x600 场景，简单布局
- **中型公司**（3-5 团队）：1200x800 场景，网格布局
- **大型公司**（6+ 团队）：动态扩展场景，不规则布局

## 测试要点

### 1. 基础渲染测试
- [ ] 页面加载后看到灰色背景
- [ ] 控制台无错误信息
- [ ] 看到办公室和 agent 简笔画

### 2. 数据同步测试
- [ ] 打开浏览器开发者工具 Network 标签
- [ ] 确认每秒有 /api/state 请求
- [ ] 确认返回的团队数据正确显示

### 3. 交互测试
- [ ] 鼠标滚轮可以缩放场景
- [ ] 按住鼠标左键可以拖拽视角
- [ ] 双击鼠标可以重置视角

### 4. 动画测试
- [ ] Agent 有轻微的呼吸动画（缩放效果）
- [ ] Busy 状态的 agent 是绿色
- [ ] Idle 状态的 agent 是灰色

### 5. 布局测试
- [ ] 团队办公室按 2 列多行排列 agents
- [ ] 功能区显示正确的 emoji 和标签
- [ ] 场景大小根据团队数量自动调整

## 已知限制

1. **图片资源**：当前使用 Canvas 简笔画，未来可替换为图片
2. **Agent 活动模拟**：尚未实现 idle agent 的随机移动
3. **气泡显示**：尚未连接到真实的 activity 数据
4. **寻路算法**：尚未实现 agent 移动到功能区

## 下一步开发

如需继续开发，可以实现：
- ActivitySimulator（agent 随机活动模拟）
- CameraController（更高级的相机控制）
- 连接真实的 activity 数据显示气泡
- 图片资源替换系统测试

## 提交记录

```
d932161 feat: integrate all components into OfficeScene with camera controls
85d2546 feat: add DataSyncManager for API polling and state diff
b9ec551 feat: add LayoutManager for dynamic office layout
67d6a52 feat: add Office and Team entity classes
f5f9d87 feat: add Agent entity with state, animation and bubble
49316c4 feat: add OfficeScene with basic rendering test
71955f1 feat: add ResourceManager for image/canvas fallback system
52b5e15 feat: add DrawFunctions library for canvas rendering
630e1e8 feat: add Phaser 3 game framework and basic config
```

## 故障排查

**问题：页面空白**
- 检查浏览器控制台是否有 JavaScript 错误
- 确认 Phaser CDN 是否可访问
- 检查 /api/state 是否返回数据

**问题：无法缩放/拖拽**
- 确认鼠标事件是否被其他元素拦截
- 检查浏览器控制台是否有错误

**问题：数据不更新**
- 检查 Network 标签确认 API 请求正常
- 查看控制台日志确认 DataSyncManager 运行正常

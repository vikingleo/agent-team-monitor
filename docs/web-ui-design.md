# Web UI 设计文档

## 概述

Agent Team Monitor 的 Web 界面采用 Phaser 3 游戏引擎实现办公场景可视化，配合右侧固定侧栏展示详细信息。

## 整体布局

### 布局结构
```
┌─────────────────────────────────────┬──────────────┐
│                                     │  Team Tabs   │
│                                     │  (90px)      │
│         办公场景区域                 ├──────────────┤
│      (Phaser Canvas)                │              │
│      宽度: 100vw - 490px            │  Sidebar     │
│      高度: 100vh                    │  Content     │
│                                     │  (400px)     │
└─────────────────────────────────────┴──────────────┘
```

### 尺寸规范
- **侧栏总宽度**: 490px
  - Team Tabs: 90px (左侧)
  - Sidebar Content: 400px (右侧)
- **办公场景宽度**: `window.innerWidth - 490px`
- **办公场景高度**: `window.innerHeight`

## 办公场景设计

### 区域划分

办公区域分为两个主要区域：
- **Claude 区域**: 左侧，浅蓝色背景 (#E3F2FD)
- **Codex 区域**: 右侧，浅绿色背景 (#E8F5E9)

判断逻辑：团队名称包含 "claude" 或 "anthropic" 的归入 Claude 区域，其他归入 Codex 区域。

### 团队布局

#### 办公桌设计
- **形状**: 长方形桌子（长桌）
- **尺寸**:
  - 宽度: `Math.max(200, agentCount * 60 + 40)`
  - 高度: 80px
- **颜色**: 棕色 (#8B4513)，边框 (#654321)
- **摆放**: 水平放置，不旋转
- **桌面元素**:
  - 团队名称：白色粗体文字，居中显示在桌面上
  - 电脑：每个 agent 位置对应一台电脑（30x20 深灰色矩形）

#### Agent 布局规则

**少量 Agent (≤3 个)**:
- 只在桌子下方一排排列
- 间距: `deskWidth / (agentCount + 1)`
- 距离桌子: 60px

**多个 Agent (>3 个)**:
- 分布在桌子上下两侧（长边）
- 上方: `Math.ceil(agentCount / 2)` 个
- 下方: 剩余的 agent
- 距离桌子: 上方 -60px，下方 +60px

### Agent 设计

#### 头像系统

**Team Lead (第一个 agent 或名称包含 "lead")**:
- 尺寸: 20px 半径
- 颜色: 8 种配色方案，根据团队名称哈希选择
  - 红色 (#FF6B6B / #C92A2A)
  - 青色 (#4ECDC4 / #0D7377)
  - 黄色 (#FFE66D / #F4A261)
  - 薄荷绿 (#95E1D3 / #38A3A5)
  - 浅蓝 (#A8DADC / #457B9D)
  - 橙色 (#F4A261 / #E76F51)
  - 紫色 (#B8B8FF / #6A67CE)
  - 粉色 (#FFAFCC / #F72585)
- 边框宽度: 3px
- 名字: 14px 粗体黑色

**普通成员**:
- 尺寸: 18px 半径
- 颜色: 蓝色 (#4A90E2 / #2E5C8A)
- 边框宽度: 2px
- 名字: 13px 常规灰色

#### 面部特征
- **眼睛**: 两个小圆点，偏移 `size * 0.32`，大小 `size * 0.15`
- **嘴巴**: 半圆弧线，宽度 1.5px，半径 `size * 0.35`

#### 状态气泡

**气泡样式**:
- 白色圆角矩形 (50x35, 圆角 10px)
- 蓝色边框 (#4A90E2)
- 三角形尾巴指向头像

**Emoji 显示规则**:
- **Working**: 💻 ⚙️ 🔧 📝 🎯 (随机)
- **Busy**: ⏰ 🔥 ⚡ 💪 🚀 (随机)
- **Idle**: 😊 🏊 ⚽ 🎮 ☕ 🎵 🌴 🎨 📚 🌞 (随机)
- 每 3 秒自动切换一次 emoji
- 闪烁动画: alpha 0.5-1.0，800ms 循环

#### 图片替换支持

所有视觉元素都支持未来替换为图片：

```javascript
avatarConfig = {
    type: 'shape' | 'image',  // 当前使用 shape，未来可改为 image
    imageKey: null,            // 图片资源 key
    shape: 'circle',
    size: 18 | 20,
    color: 0x4A90E2,
    strokeColor: 0x2E5C8A,
    strokeWidth: 2 | 3
}
```

## 侧栏设计

### Team Tabs (90px)

**默认状态**:
- 宽度: 90px
- 背景: #f5f5f5
- 显示: 团队名称（截断）+ 成员数量

**Hover 状态**:
- 向左展开至 180px
- 使用 `position: absolute` + `transform: translateX(-90px)`
- 右边界保持不变
- z-index: 1001 (避免被裁剪)
- 添加阴影效果
- 完整显示团队名称

**Active 状态**:
- 白色背景
- 右侧 3px 蓝色边框 (#4A90E2)

### Sidebar Content (400px)

**团队信息区**:
- 团队名称 + 成员数量

**Agent 卡片**:
- 名称 + 状态标签 (工作中/空闲)
- 当前操作 (🔧 + last_tool_use)
- 思考内容 (💭 + last_thinking，最多 150 字符)
- 任务清单 (📋 + todos，显示前 5 项)

## 渲染优化

### 高 DPI 屏幕支持

**Canvas 配置**:
```javascript
scale: {
    mode: Phaser.Scale.RESIZE,
    autoCenter: Phaser.Scale.CENTER_BOTH,
    resolution: window.devicePixelRatio || 1
}
```

**文字渲染**:
- 所有文字添加 `resolution: window.devicePixelRatio || 2`
- 使用系统字体: `-apple-system, BlinkMacSystemFont, "Segoe UI", Arial, sans-serif`
- 字号适当增大以提高清晰度

### 相机设置

**初始视图**:
```javascript
camera.setZoom(1);
camera.centerOn((window.innerWidth - 490) / 2, window.innerHeight / 2);
```

**交互控制**:
- 鼠标滚轮: 缩放 (0.5x - 2x)
- 左键拖拽: 平移视图
- 双击: 重置视角

## 数据同步

### 轮询机制
- 间隔: 1000ms
- API: `/api/state?_ts={timestamp}`
- 防抖: 100ms

### 状态更新流程
```
DataSyncManager.fetchAndUpdate()
    ↓
processChanges(newState)
    ↓
OfficeScene.initializeState() / applyChanges()
    ↓
Game.events.emit('state-updated', state)
    ↓
Sidebar.updateState(state)
```

### 差异检测
- 新增团队: 完全重建场景
- 删除团队: 销毁对应 Team 对象
- Agent 状态变化: 更新 agent.state，触发动画

## 文件结构

```
web/static/
├── index.html                      # 入口页面
├── css/
│   └── sidebar.css                 # 侧栏样式
└── js/
    ├── components/
    │   └── Sidebar.js              # 侧栏组件
    └── game/
        ├── main.js                 # 游戏主入口
        ├── config.js               # Phaser 配置
        ├── scenes/
        │   └── OfficeScene.js      # 办公场景
        ├── entities/
        │   ├── Team.js             # 团队实体
        │   ├── Agent.js            # Agent 实体
        │   └── FacilityZone.js     # 功能区实体
        └── systems/
            ├── LayoutManager.js    # 布局管理器
            ├── DataSyncManager.js  # 数据同步管理器
            └── ResourceManager.js  # 资源管理器
```

## 关键实现细节

### 1. 宽度计算一致性

所有涉及宽度的地方必须统一减去侧栏宽度：
- `index.html`: `width: calc(100vw - 490px)`
- `config.js`: `width: window.innerWidth - 490`
- `LayoutManager.js`: `viewWidth = window.innerWidth - 490`
- `OfficeScene.js`: `camera.centerOn((window.innerWidth - 490) / 2, ...)`

### 2. Agent 角色识别

```javascript
const isLead = this.agents.size === 0 ||
               agentData.name.toLowerCase().includes('lead') ||
               agentData.role === 'lead';
```

### 3. 团队分类

```javascript
isClaudeTeam(team) {
    const name = team.name.toLowerCase();
    return name.includes('claude') || name.includes('anthropic');
}
```

### 4. Emoji 轮换

```javascript
startEmojiRotation() {
    this.emojiRotationTimer = setInterval(() => {
        if (this.bubbleText && (this.state === 'working' || this.state === 'busy' || this.state === 'idle')) {
            const newEmoji = this.getStateEmoji(this.state);
            this.bubbleText.setText(newEmoji);
        }
    }, 3000);
}
```

## 未来扩展

### 图片资源替换

1. 在 `ResourceManager.js` 中加载图片资源
2. 修改 `Agent.avatarConfig.type = 'image'`
3. 设置 `Agent.avatarConfig.imageKey = 'avatar_lead_1'`
4. 在 `Agent.create()` 中使用 `this.scene.add.image()`

### 动画增强

- Agent 移动到功能区（休息室、茶水间）
- 状态切换时的过渡动画
- 团队协作的视觉效果

### 交互功能

- 点击 Agent 显示详细信息
- 拖拽调整布局
- 自定义主题配色

## 注意事项

1. **避免重叠**: Agent 与桌子保持 60px 距离
2. **文字清晰**: 使用 devicePixelRatio 提高渲染分辨率
3. **Tab 展开**: 使用 absolute 定位避免被裁剪
4. **性能优化**: 使用对象池管理 Agent 实例
5. **响应式**: 所有尺寸基于窗口大小动态计算

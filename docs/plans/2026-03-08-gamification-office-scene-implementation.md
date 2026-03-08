# 游戏化办公场景实现计划

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** 将 Agent Team Monitor 的 Web 界面改造为基于 Phaser 3 的游戏化办公场景，支持动态布局、Agent 行为模拟和图片资源热替换。

**Architecture:** 使用 Phaser 3 游戏引擎渲染办公场景，通过 ResourceManager 实现 Canvas 绘制与图片资源的无缝切换，DataSyncManager 每秒轮询 API 并增量更新场景，ActivitySimulator 模拟 idle agent 的随机活动。

**Tech Stack:** Phaser 3.80+, 原生 JavaScript (ES6+), Canvas 2D API

---

## 阶段 1：基础框架搭建

### Task 1: 创建游戏入口和配置

**Files:**
- Create: `web/static/js/game/config.js`
- Create: `web/static/js/game/main.js`
- Create: `web/static/assets/.gitkeep`
- Modify: `web/static/index.html`

**Step 1: 创建游戏配置文件**

创建 `web/static/js/game/config.js`:

```javascript
export const GameConfig = {
    type: Phaser.AUTO,
    parent: 'game-container',
    width: 1920,
    height: 1080,
    backgroundColor: '#f5f5f5',
    physics: {
        default: 'arcade',
        arcade: {
            debug: false
        }
    },
    scale: {
        mode: Phaser.Scale.RESIZE,
        autoCenter: Phaser.Scale.CENTER_BOTH
    }
};

export const Constants = {
    POLL_INTERVAL: 1000,
    AGENT_SPEED: 100,
    AGENT_IDLE_SPEED: 50,
    AGENT_SIZE: 40,
    OFFICE_MIN_WIDTH: 200,
    OFFICE_MIN_HEIGHT: 150,
    ROOM_PADDING: 20,
    CORRIDOR_WIDTH: 150
};
```

**Step 2: 创建游戏主入口**

创建 `web/static/js/game/main.js`:

```javascript
import { GameConfig } from './config.js';

class Game {
    constructor() {
        this.game = null;
    }

    init() {
        this.game = new Phaser.Game(GameConfig);
        console.log('Phaser game initialized');
    }

    destroy() {
        if (this.game) {
            this.game.destroy(true);
        }
    }
}

// 全局实例
window.AgentMonitorGame = new Game();

// 页面加载后初始化
document.addEventListener('DOMContentLoaded', () => {
    window.AgentMonitorGame.init();
});
```

**Step 3: 修改 HTML 引入 Phaser 和游戏代码**

修改 `web/static/index.html`:

```html
<!DOCTYPE html>
<html lang="zh-CN">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Agent Team Monitor - 办公场景</title>
    <style>
        body {
            margin: 0;
            padding: 0;
            overflow: hidden;
            font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, sans-serif;
        }
        #game-container {
            width: 100vw;
            height: 100vh;
        }
    </style>
</head>
<body>
    <div id="game-container"></div>

    <!-- Phaser 3 from CDN -->
    <script src="https://cdn.jsdelivr.net/npm/phaser@3.80.1/dist/phaser.min.js"></script>

    <!-- Game modules -->
    <script type="module" src="js/game/main.js"></script>
</body>
</html>
```

**Step 4: 创建 assets 目录占位符**

```bash
touch web/static/assets/.gitkeep
```

**Step 5: 测试基础框架**

运行: `make run-web`
访问: `http://localhost:8080`
预期: 看到灰色背景的 Phaser 画布

**Step 6: 提交**

```bash
git add web/static/js/game/config.js web/static/js/game/main.js web/static/index.html web/static/assets/.gitkeep
git commit -m "feat: add Phaser 3 game framework and basic config"
```

---

### Task 2: 实现 DrawFunctions 绘制函数库

**Files:**
- Create: `web/static/js/game/systems/DrawFunctions.js`

**Step 1: 创建绘制函数库**

创建 `web/static/js/game/systems/DrawFunctions.js`:

```javascript
export class DrawFunctions {
    static drawAgent(graphics, x, y, state) {
        const color = state === 'busy' ? 0x4CAF50 : 0x9E9E9E;

        // 身体（圆形）
        graphics.fillStyle(color, 1);
        graphics.fillCircle(x, y, 20);

        // 眼睛
        graphics.fillStyle(0x000000, 1);
        graphics.fillCircle(x - 8, y - 5, 3);
        graphics.fillCircle(x + 8, y - 5, 3);

        // 微笑
        graphics.lineStyle(2, 0x000000);
        graphics.beginPath();
        graphics.arc(x, y + 5, 10, 0, Math.PI, false);
        graphics.strokePath();
    }

    static drawAgentName(graphics, x, y, name) {
        const text = graphics.scene.add.text(x, y + 30, name, {
            fontSize: '12px',
            color: '#333',
            align: 'center'
        });
        text.setOrigin(0.5, 0);
        return text;
    }

    static drawOffice(graphics, x, y, width, height, teamName) {
        // 墙壁
        graphics.fillStyle(0xE8E8E8, 1);
        graphics.fillRect(x, y, width, height);

        // 边框
        graphics.lineStyle(3, 0x666666);
        graphics.strokeRect(x, y, width, height);

        // 门（底部中央）
        graphics.fillStyle(0x8B4513, 1);
        graphics.fillRect(x + width/2 - 20, y + height - 5, 40, 5);

        // 窗户（左上角）
        graphics.lineStyle(2, 0x87CEEB);
        graphics.strokeRect(x + 10, y + 10, 40, 30);

        // 团队名称
        const text = graphics.scene.add.text(x + width/2, y + 20, teamName, {
            fontSize: '14px',
            fontStyle: 'bold',
            color: '#333',
            align: 'center'
        });
        text.setOrigin(0.5, 0);
        return text;
    }

    static drawFacility(graphics, x, y, width, height, type) {
        // 背景
        graphics.fillStyle(0xFFFFFF, 1);
        graphics.fillRect(x, y, width, height);

        // 边框
        graphics.lineStyle(2, 0x999999);
        graphics.strokeRect(x, y, width, height);

        // 根据类型绘制图标和文字
        let emoji, label;
        switch(type) {
            case 'restroom':
                emoji = '🚻';
                label = '洗手间';
                break;
            case 'cafe':
                emoji = '☕';
                label = '茶水间';
                break;
            case 'gym':
                emoji = '💪';
                label = '健身区';
                break;
            case 'boss':
                emoji = '🏢';
                label = '老板办公室';
                break;
            default:
                emoji = '📦';
                label = type;
        }

        const emojiText = graphics.scene.add.text(x + width/2, y + height/2 - 15, emoji, {
            fontSize: '32px',
            align: 'center'
        });
        emojiText.setOrigin(0.5, 0.5);

        const labelText = graphics.scene.add.text(x + width/2, y + height/2 + 20, label, {
            fontSize: '12px',
            color: '#666',
            align: 'center'
        });
        labelText.setOrigin(0.5, 0.5);

        return { emojiText, labelText };
    }

    static drawBubble(graphics, x, y, emoji) {
        // 白色圆角矩形
        graphics.fillStyle(0xFFFFFF, 0.95);
        graphics.fillRoundedRect(x - 20, y - 40, 40, 30, 8);

        // 边框
        graphics.lineStyle(1, 0xCCCCCC);
        graphics.strokeRoundedRect(x - 20, y - 40, 40, 30, 8);

        // Emoji
        const text = graphics.scene.add.text(x, y - 25, emoji, {
            fontSize: '20px',
            align: 'center'
        });
        text.setOrigin(0.5, 0.5);
        return text;
    }
}
```

**Step 2: 提交**

```bash
git add web/static/js/game/systems/DrawFunctions.js
git commit -m "feat: add DrawFunctions library for canvas rendering"
```

---

### Task 3: 实现 ResourceManager 资源管理器

**Files:**
- Create: `web/static/js/game/systems/ResourceManager.js`
- Create: `web/static/js/game/config/assets.json`

**Step 1: 创建资源配置文件**

创建 `web/static/js/game/config/assets.json`:

```json
{
  "agents": {
    "busy": {
      "type": "image",
      "path": "assets/agent-busy.png",
      "fallback": "drawAgent"
    },
    "idle": {
      "type": "image",
      "path": "assets/agent-idle.png",
      "fallback": "drawAgent"
    }
  },
  "rooms": {
    "office": {
      "type": "image",
      "path": "assets/office-room.png",
      "fallback": "drawOffice"
    }
  },
  "facilities": {
    "restroom": {
      "type": "image",
      "path": "assets/restroom.png",
      "fallback": "drawFacility"
    },
    "cafe": {
      "type": "image",
      "path": "assets/cafe.png",
      "fallback": "drawFacility"
    },
    "gym": {
      "type": "image",
      "path": "assets/gym.png",
      "fallback": "drawFacility"
    },
    "boss": {
      "type": "image",
      "path": "assets/boss-room.png",
      "fallback": "drawFacility"
    }
  }
}
```

**Step 2: 创建 ResourceManager 类**

创建 `web/static/js/game/systems/ResourceManager.js`:

```javascript
import { DrawFunctions } from './DrawFunctions.js';

export class ResourceManager {
    constructor(scene) {
        this.scene = scene;
        this.config = null;
        this.loadedImages = new Map();
        this.drawFunctions = new Map();

        // 注册绘制函数
        this.registerDrawFunction('drawAgent', DrawFunctions.drawAgent);
        this.registerDrawFunction('drawOffice', DrawFunctions.drawOffice);
        this.registerDrawFunction('drawFacility', DrawFunctions.drawFacility);
    }

    async init() {
        // 加载配置文件
        try {
            const response = await fetch('js/game/config/assets.json');
            this.config = await response.json();
            console.log('Assets config loaded:', this.config);
        } catch (error) {
            console.error('Failed to load assets config:', error);
            this.config = { agents: {}, rooms: {}, facilities: {} };
        }

        // 尝试加载所有图片
        await this.loadAllImages();
    }

    async loadAllImages() {
        const promises = [];

        for (const [category, items] of Object.entries(this.config)) {
            for (const [key, asset] of Object.entries(items)) {
                if (asset.type === 'image') {
                    const assetKey = `${category}_${key}`;
                    promises.push(this.tryLoadImage(asset.path, assetKey));
                }
            }
        }

        await Promise.all(promises);
        console.log('Image loading complete. Loaded:', this.loadedImages.size);
    }

    async tryLoadImage(path, key) {
        return new Promise((resolve) => {
            this.scene.load.image(key, path);
            this.scene.load.once('filecomplete-image-' + key, () => {
                this.loadedImages.set(key, true);
                console.log(`Image loaded: ${key}`);
                resolve();
            });
            this.scene.load.once('loaderror', () => {
                this.loadedImages.set(key, false);
                console.log(`Image not found, using fallback: ${key}`);
                resolve();
            });
            this.scene.load.start();
        });
    }

    getAsset(category, key) {
        const assetKey = `${category}_${key}`;

        if (this.loadedImages.get(assetKey) === true) {
            return { type: 'image', key: assetKey };
        } else {
            const fallbackName = this.config[category]?.[key]?.fallback || 'drawAgent';
            const func = this.drawFunctions.get(fallbackName);
            return { type: 'draw', func: func };
        }
    }

    registerDrawFunction(name, func) {
        this.drawFunctions.set(name, func);
    }

    hasImage(category, key) {
        const assetKey = `${category}_${key}`;
        return this.loadedImages.get(assetKey) === true;
    }
}
```

**Step 3: 提交**

```bash
git add web/static/js/game/systems/ResourceManager.js web/static/js/game/config/assets.json
git commit -m "feat: add ResourceManager for image/canvas fallback system"
```

---

## 阶段 2：场景和实体基础

### Task 4: 创建 OfficeScene 主场景

**Files:**
- Create: `web/static/js/game/scenes/OfficeScene.js`
- Modify: `web/static/js/game/config.js`
- Modify: `web/static/js/game/main.js`

**Step 1: 创建 OfficeScene 类**

创建 `web/static/js/game/scenes/OfficeScene.js`:

```javascript
import { ResourceManager } from '../systems/ResourceManager.js';

export class OfficeScene extends Phaser.Scene {
    constructor() {
        super({ key: 'OfficeScene' });
        this.resourceManager = null;
        this.teams = new Map();
        this.agents = new Map();
        this.facilities = new Map();
    }

    preload() {
        console.log('OfficeScene: preload');
        // Phaser preload 阶段
    }

    async create() {
        console.log('OfficeScene: create');

        // 初始化资源管理器
        this.resourceManager = new ResourceManager(this);
        await this.resourceManager.init();

        // 设置相机
        this.cameras.main.setBounds(0, 0, 2400, 1600);

        // 创建图形对象（用于绘制）
        this.graphics = this.add.graphics();

        // 测试绘制
        this.testDraw();

        console.log('OfficeScene: ready');
    }

    testDraw() {
        // 测试绘制一个办公室
        const asset = this.resourceManager.getAsset('rooms', 'office');
        if (asset.type === 'draw') {
            asset.func(this.graphics, 100, 100, 300, 200, 'Test Team');
        }

        // 测试绘制一个 agent
        const agentAsset = this.resourceManager.getAsset('agents', 'busy');
        if (agentAsset.type === 'draw') {
            agentAsset.func(this.graphics, 200, 200, 'busy');
        }
    }

    update(time, delta) {
        // 游戏循环
    }
}
```

**Step 2: 修改配置文件添加场景**

修改 `web/static/js/game/config.js`:

```javascript
import { OfficeScene } from './scenes/OfficeScene.js';

export const GameConfig = {
    type: Phaser.AUTO,
    parent: 'game-container',
    width: 1920,
    height: 1080,
    backgroundColor: '#f5f5f5',
    scene: [OfficeScene],  // 添加场景
    physics: {
        default: 'arcade',
        arcade: {
            debug: false
        }
    },
    scale: {
        mode: Phaser.Scale.RESIZE,
        autoCenter: Phaser.Scale.CENTER_BOTH
    }
};

// ... Constants 保持不变
```

**Step 3: 测试场景渲染**

运行: `make run-web`
访问: `http://localhost:8080`
预期: 看到一个简笔画办公室和一个 agent

**Step 4: 提交**

```bash
git add web/static/js/game/scenes/OfficeScene.js web/static/js/game/config.js
git commit -m "feat: add OfficeScene with basic rendering test"
```

---

### Task 5: 实现 Agent 实体类

**Files:**
- Create: `web/static/js/game/entities/Agent.js`

**Step 1: 创建 Agent 类**

创建 `web/static/js/game/entities/Agent.js`:

```javascript
export class Agent {
    constructor(scene, data) {
        this.scene = scene;
        this.id = data.name;
        this.name = data.name;
        this.state = data.status || 'idle'; // busy, idle, going_out
        this.teamName = data.teamName;

        this.x = 0;
        this.y = 0;
        this.targetX = 0;
        this.targetY = 0;

        this.graphics = null;
        this.nameText = null;
        this.bubble = null;
        this.bubbleText = null;

        this.moveTween = null;
        this.breatheTween = null;
        this.scale = 1.0;

        this.recentActivity = null;
    }

    create(x, y) {
        this.x = x;
        this.y = y;
        this.targetX = x;
        this.targetY = y;

        // 创建图形对象
        this.graphics = this.scene.add.graphics();
        this.render();

        // 创建名字文本
        this.nameText = this.scene.add.text(this.x, this.y + 30, this.name, {
            fontSize: '12px',
            color: '#333',
            align: 'center'
        });
        this.nameText.setOrigin(0.5, 0);

        // 启动呼吸动画
        this.startBreatheAnimation();
    }

    render() {
        if (!this.graphics) return;

        this.graphics.clear();

        const color = this.state === 'busy' ? 0x4CAF50 : 0x9E9E9E;
        const radius = 20 * this.scale;

        // 身体
        this.graphics.fillStyle(color, 1);
        this.graphics.fillCircle(this.x, this.y, radius);

        // 眼睛
        this.graphics.fillStyle(0x000000, 1);
        this.graphics.fillCircle(this.x - 8 * this.scale, this.y - 5 * this.scale, 3 * this.scale);
        this.graphics.fillCircle(this.x + 8 * this.scale, this.y - 5 * this.scale, 3 * this.scale);

        // 微笑
        this.graphics.lineStyle(2, 0x000000);
        this.graphics.beginPath();
        this.graphics.arc(this.x, this.y + 5 * this.scale, 10 * this.scale, 0, Math.PI, false);
        this.graphics.strokePath();
    }

    startBreatheAnimation() {
        if (this.breatheTween) {
            this.breatheTween.stop();
        }

        this.breatheTween = this.scene.tweens.add({
            targets: this,
            scale: this.state === 'busy' ? 1.05 : 1.02,
            duration: this.state === 'busy' ? 800 : 1200,
            yoyo: true,
            repeat: -1,
            onUpdate: () => {
                this.render();
            }
        });
    }

    moveTo(x, y, speed = 100) {
        this.targetX = x;
        this.targetY = y;

        const distance = Phaser.Math.Distance.Between(this.x, this.y, x, y);
        const duration = (distance / speed) * 1000;

        if (this.moveTween) {
            this.moveTween.stop();
        }

        this.moveTween = this.scene.tweens.add({
            targets: this,
            x: x,
            y: y,
            duration: duration,
            ease: 'Cubic.InOut',
            onUpdate: () => {
                this.render();
                if (this.nameText) {
                    this.nameText.setPosition(this.x, this.y + 30);
                }
                if (this.bubble) {
                    this.bubble.setPosition(this.x, this.y - 40);
                }
                if (this.bubbleText) {
                    this.bubbleText.setPosition(this.x, this.y - 25);
                }
            }
        });
    }

    updateState(newState) {
        if (this.state !== newState) {
            this.state = newState;
            this.startBreatheAnimation();
            this.render();
        }
    }

    showActivity(emoji) {
        // 移除旧气泡
        this.hideActivity();

        // 创建气泡背景
        this.bubble = this.scene.add.graphics();
        this.bubble.fillStyle(0xFFFFFF, 0.95);
        this.bubble.fillRoundedRect(this.x - 20, this.y - 40, 40, 30, 8);
        this.bubble.lineStyle(1, 0xCCCCCC);
        this.bubble.strokeRoundedRect(this.x - 20, this.y - 40, 40, 30, 8);

        // 创建 emoji 文本
        this.bubbleText = this.scene.add.text(this.x, this.y - 25, emoji, {
            fontSize: '20px',
            align: 'center'
        });
        this.bubbleText.setOrigin(0.5, 0.5);

        // 2秒后自动隐藏
        this.scene.time.delayedCall(2000, () => {
            this.hideActivity();
        });
    }

    hideActivity() {
        if (this.bubble) {
            this.bubble.destroy();
            this.bubble = null;
        }
        if (this.bubbleText) {
            this.bubbleText.destroy();
            this.bubbleText = null;
        }
    }

    destroy() {
        if (this.moveTween) this.moveTween.stop();
        if (this.breatheTween) this.breatheTween.stop();
        if (this.graphics) this.graphics.destroy();
        if (this.nameText) this.nameText.destroy();
        this.hideActivity();
    }
}
```

**Step 2: 提交**

```bash
git add web/static/js/game/entities/Agent.js
git commit -m "feat: add Agent entity with state, animation and bubble"
```

---

由于输出限制，我将计划分成多个文件。当前已完成基础框架部分。

**计划已保存到**: `docs/plans/2026-03-08-gamification-office-scene-implementation.md`

继续实现剩余部分（Office、Team、LayoutManager、DataSyncManager、ActivitySimulator 等）需要继续编写。

是否需要我继续完成剩余的实现计划？
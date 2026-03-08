export class Agent {
    constructor(scene, data) {
        this.scene = scene;
        this.id = data.name;
        this.name = data.name;
        this.state = data.status || 'idle'; // working, busy, idle, going_out
        this.teamName = data.teamName;
        this.role = data.role || 'member'; // 'lead' or 'member'

        this.x = 0;
        this.y = 0;
        this.targetX = 0;
        this.targetY = 0;

        this.avatar = null;
        this.avatarImage = null; // 用于图片头像
        this.leftEye = null;
        this.rightEye = null;
        this.mouth = null;
        this.nameText = null;
        this.bubble = null;
        this.bubbleText = null;

        this.moveTween = null;
        this.breatheTween = null;

        this.recentActivity = null;
        this.emojiRotationTimer = null;

        // 头像配置（支持图片替换）
        this.avatarConfig = this.getAvatarConfig();
    }

    // 获取头像配置（根据角色和团队）
    getAvatarConfig() {
        if (this.role === 'lead') {
            // Team lead 使用不同的颜色方案
            const leadColors = [
                { fill: 0xFF6B6B, stroke: 0xC92A2A }, // 红色
                { fill: 0x4ECDC4, stroke: 0x0D7377 }, // 青色
                { fill: 0xFFE66D, stroke: 0xF4A261 }, // 黄色
                { fill: 0x95E1D3, stroke: 0x38A3A5 }, // 薄荷绿
                { fill: 0xA8DADC, stroke: 0x457B9D }, // 浅蓝
                { fill: 0xF4A261, stroke: 0xE76F51 }, // 橙色
                { fill: 0xB8B8FF, stroke: 0x6A67CE }, // 紫色
                { fill: 0xFFAFCC, stroke: 0xF72585 }  // 粉色
            ];

            // 根据团队名称哈希选择颜色
            const hash = this.hashString(this.teamName);
            const colorScheme = leadColors[hash % leadColors.length];

            return {
                type: 'shape', // 'shape' or 'image'
                imageKey: null, // 未来可以设置为图片资源 key
                shape: 'circle',
                size: 20, // lead 头像稍大
                color: colorScheme.fill,
                strokeColor: colorScheme.stroke,
                strokeWidth: 3
            };
        } else {
            // 普通成员使用统一样式
            return {
                type: 'shape',
                imageKey: null,
                shape: 'circle',
                size: 18,
                color: 0x4A90E2,
                strokeColor: 0x2E5C8A,
                strokeWidth: 2
            };
        }
    }

    // 简单的字符串哈希函数
    hashString(str) {
        let hash = 0;
        for (let i = 0; i < str.length; i++) {
            const char = str.charCodeAt(i);
            hash = ((hash << 5) - hash) + char;
            hash = hash & hash;
        }
        return Math.abs(hash);
    }

    // 根据状态获取对应的 emoji
    getStateEmoji(state) {
        const emojiMap = {
            'working': ['💻', '⚙️', '🔧', '📝', '🎯'],
            'busy': ['⏰', '🔥', '⚡', '💪', '🚀'],
            'idle': ['😊', '🏊', '⚽', '🎮', '☕', '🎵', '🌴', '🎨', '📚', '🌞']
        };

        const emojis = emojiMap[state] || emojiMap['idle'];
        return emojis[Math.floor(Math.random() * emojis.length)];
    }

    create(x, y) {
        this.x = x;
        this.y = y;
        this.targetX = x;
        this.targetY = y;

        // 根据配置创建头像
        if (this.avatarConfig.type === 'image' && this.avatarConfig.imageKey) {
            // 使用图片头像（未来支持）
            this.avatarImage = this.scene.add.image(x, y, this.avatarConfig.imageKey);
            this.avatarImage.setDisplaySize(this.avatarConfig.size * 2, this.avatarConfig.size * 2);
            this.avatarImage.setDepth(2);
            this.avatar = this.avatarImage; // 保持引用一致性
        } else {
            // 使用形状头像（当前默认）
            this.avatar = this.scene.add.circle(
                x, y,
                this.avatarConfig.size,
                this.avatarConfig.color
            );
            this.avatar.setStrokeStyle(
                this.avatarConfig.strokeWidth,
                this.avatarConfig.strokeColor
            );
            this.avatar.setDepth(2);

            // 添加眼睛（仅形状模式）
            const eyeOffset = this.avatarConfig.size * 0.32;
            const eyeSize = this.avatarConfig.size * 0.15;
            this.leftEye = this.scene.add.circle(x - eyeOffset, y - 3, eyeSize, 0x000000);
            this.leftEye.setDepth(3);
            this.rightEye = this.scene.add.circle(x + eyeOffset, y - 3, eyeSize, 0x000000);
            this.rightEye.setDepth(3);

            // 添加嘴巴（仅形状模式）
            this.mouth = this.scene.add.graphics();
            this.mouth.lineStyle(1.5, 0x000000);
            this.mouth.beginPath();
            this.mouth.arc(x, y + 3, this.avatarConfig.size * 0.35, 0, Math.PI, false);
            this.mouth.strokePath();
            this.mouth.setDepth(3);
        }

        // 创建名字文本
        const nameYOffset = this.avatarConfig.size + 10;
        this.nameText = this.scene.add.text(x, y + nameYOffset, this.name, {
            fontSize: this.role === 'lead' ? '14px' : '13px',
            fontFamily: '-apple-system, BlinkMacSystemFont, "Segoe UI", Arial, sans-serif',
            color: this.role === 'lead' ? '#000000' : '#333333',
            fontStyle: this.role === 'lead' ? 'bold' : 'normal',
            align: 'center',
            resolution: window.devicePixelRatio || 2  // 提高文字渲染分辨率
        });
        this.nameText.setOrigin(0.5);
        this.nameText.setDepth(2);

        // 根据状态显示气泡
        if (this.state === 'working' || this.state === 'busy' || this.state === 'idle') {
            this.showThinkingBubble();
        }

        // 启动呼吸动画
        this.startBreatheAnimation();
    }

    showThinkingBubble() {
        // 移除旧气泡
        this.hideActivity();

        // 创建气泡背景
        this.bubble = this.scene.add.graphics();
        this.bubble.fillStyle(0xFFFFFF, 0.95);
        this.bubble.fillRoundedRect(this.x - 25, this.y - 50, 50, 35, 10);
        this.bubble.lineStyle(2, 0x4A90E2);
        this.bubble.strokeRoundedRect(this.x - 25, this.y - 50, 50, 35, 10);

        // 气泡尾巴
        this.bubble.fillStyle(0xFFFFFF, 0.95);
        this.bubble.fillTriangle(
            this.x - 5, this.y - 15,
            this.x + 5, this.y - 15,
            this.x, this.y - 5
        );
        this.bubble.setDepth(4);

        // 根据状态显示对应的 emoji
        const emoji = this.getStateEmoji(this.state);
        this.bubbleText = this.scene.add.text(this.x, this.y - 32, emoji, {
            fontSize: '24px',
            align: 'center'
        });
        this.bubbleText.setOrigin(0.5);
        this.bubbleText.setDepth(5);

        // 添加闪烁动画
        this.scene.tweens.add({
            targets: this.bubbleText,
            alpha: 0.5,
            duration: 800,
            yoyo: true,
            repeat: -1
        });

        // 启动 emoji 轮换定时器
        this.startEmojiRotation();
    }

    startEmojiRotation() {
        // 清除旧定时器
        if (this.emojiRotationTimer) {
            clearInterval(this.emojiRotationTimer);
        }

        // 每 3 秒切换一次 emoji
        this.emojiRotationTimer = setInterval(() => {
            if (this.bubbleText && (this.state === 'working' || this.state === 'busy' || this.state === 'idle')) {
                const newEmoji = this.getStateEmoji(this.state);
                this.bubbleText.setText(newEmoji);
            }
        }, 3000);
    }

    stopEmojiRotation() {
        if (this.emojiRotationTimer) {
            clearInterval(this.emojiRotationTimer);
            this.emojiRotationTimer = null;
        }
    }

    render() {
        // 更新位置
        if (this.avatar) {
            this.avatar.setPosition(this.x, this.y);

            // 根据状态改变颜色（仅形状模式）
            if (this.avatarConfig.type === 'shape') {
                const color = (this.state === 'working' || this.state === 'busy') ? 0x4CAF50 : this.avatarConfig.color;
                this.avatar.setFillStyle(color);
            }
        }

        if (this.leftEye) this.leftEye.setPosition(this.x - this.avatarConfig.size * 0.32, this.y - 3);
        if (this.rightEye) this.rightEye.setPosition(this.x + this.avatarConfig.size * 0.32, this.y - 3);

        if (this.mouth) {
            this.mouth.clear();
            this.mouth.lineStyle(1.5, 0x000000);
            this.mouth.beginPath();
            this.mouth.arc(this.x, this.y + 3, this.avatarConfig.size * 0.35, 0, Math.PI, false);
            this.mouth.strokePath();
        }
    }

    startBreatheAnimation() {
        if (this.breatheTween) {
            this.breatheTween.stop();
        }

        const isWorking = this.state === 'working' || this.state === 'busy';

        this.breatheTween = this.scene.tweens.add({
            targets: this.avatar,
            scale: isWorking ? 1.08 : 1.03,
            duration: isWorking ? 600 : 1000,
            yoyo: true,
            repeat: -1
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
                    this.nameText.setPosition(this.x, this.y + 35);
                }
                if (this.bubble) {
                    this.bubble.setPosition(this.x, this.y);
                }
                if (this.bubbleText) {
                    this.bubbleText.setPosition(this.x, this.y - 32);
                }
            }
        });
    }

    updateState(newState) {
        if (this.state !== newState) {
            this.state = newState;
            this.startBreatheAnimation();
            this.render();

            // 根据状态显示或隐藏气泡
            if (newState === 'working' || newState === 'busy') {
                this.showThinkingBubble();
            } else if (newState === 'idle') {
                this.showThinkingBubble(); // idle 状态也显示气泡，但 emoji 不同
            } else {
                this.hideActivity();
            }
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
        this.stopEmojiRotation();
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
        if (this.avatar) this.avatar.destroy();
        if (this.avatarImage) this.avatarImage.destroy();
        if (this.leftEye) this.leftEye.destroy();
        if (this.rightEye) this.rightEye.destroy();
        if (this.mouth) this.mouth.destroy();
        if (this.nameText) this.nameText.destroy();
        this.hideActivity();
    }
}

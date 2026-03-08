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

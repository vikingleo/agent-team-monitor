import * as PIXI from '../../vendor/pixi.js';

const LEAD_COLORS = [
    { fill: 0xFF6B6B, stroke: 0xC92A2A },
    { fill: 0x4ECDC4, stroke: 0x0D7377 },
    { fill: 0xFFE66D, stroke: 0xF4A261 },
    { fill: 0x95E1D3, stroke: 0x38A3A5 },
    { fill: 0xA8DADC, stroke: 0x457B9D },
    { fill: 0xF4A261, stroke: 0xE76F51 },
    { fill: 0xB8B8FF, stroke: 0x6A67CE },
    { fill: 0xFFAFCC, stroke: 0xF72585 }
];

const STATE_EMOJI = {
    working: ['💻', '⚙️', '🔧', '📝', '🎯'],
    busy: ['⏰', '🔥', '⚡', '💪', '🚀'],
    idle: ['😊', '🏊', '⚽', '🎮', '☕', '🎵', '🌴', '🎨', '📚', '🌞']
};

export class Agent {
    constructor(scene, data) {
        this.scene = scene;
        this.id = data.name;
        this.name = data.name;
        this.state = data.status || 'idle';
        this.teamName = data.teamName;
        this.role = data.role || 'member';
        this.x = 0;
        this.y = 0;
        this.targetX = 0;
        this.targetY = 0;
        this.container = new PIXI.Container();
        this.container.sortableChildren = true;
        this.avatar = null;
        this.nameText = null;
        this.bubble = null;
        this.bubbleText = null;
        this.emojiElapsed = 0;
        this.breatheElapsed = Math.random() * Math.PI * 2;
        this.avatarConfig = this.getAvatarConfig();
    }

    getAvatarConfig() {
        if (this.role === 'lead') {
            const hash = this.hashString(this.teamName);
            const scheme = LEAD_COLORS[hash % LEAD_COLORS.length];
            return { size: 20, color: scheme.fill, strokeColor: scheme.stroke, strokeWidth: 3 };
        }
        return { size: 18, color: 0x4A90E2, strokeColor: 0x2E5C8A, strokeWidth: 2 };
    }

    hashString(str) {
        let hash = 0;
        for (let i = 0; i < str.length; i += 1) {
            hash = ((hash << 5) - hash) + str.charCodeAt(i);
            hash |= 0;
        }
        return Math.abs(hash);
    }

    getStateEmoji(state) {
        const emojis = STATE_EMOJI[state] || STATE_EMOJI.idle;
        return emojis[Math.floor(Math.random() * emojis.length)];
    }

    create(x, y) {
        this.x = x;
        this.y = y;
        this.targetX = x;
        this.targetY = y;
        this.container.position.set(x, y);
        this.scene.world.addChild(this.container);

        this.avatar = new PIXI.Graphics();
        this.redrawAvatar();
        this.avatar.zIndex = 2;
        this.container.addChild(this.avatar);

        this.nameText = new PIXI.Text(this.name, {
            fontFamily: '-apple-system, BlinkMacSystemFont, "Segoe UI", Arial, sans-serif',
            fontSize: this.role === 'lead' ? 14 : 13,
            fontWeight: this.role === 'lead' ? '700' : '400',
            fill: this.role === 'lead' ? 0x000000 : 0x333333,
            resolution: 1.5
        });
        this.nameText.anchor.set(0.5, 0);
        this.nameText.position.set(0, this.avatarConfig.size + 10);
        this.nameText.zIndex = 2;
        this.container.addChild(this.nameText);

        if (['working', 'busy', 'idle'].includes(this.state)) {
            this.showThinkingBubble();
        }
    }

    redrawAvatar(scale = 1) {
        if (!this.avatar) {
            return;
        }

        const size = this.avatarConfig.size;
        const fillColor = (this.state === 'working' || this.state === 'busy') ? 0x4CAF50 : this.avatarConfig.color;
        this.avatar.clear();
        this.avatar.lineStyle(this.avatarConfig.strokeWidth, this.avatarConfig.strokeColor, 1);
        this.avatar.beginFill(fillColor, 1);
        this.avatar.drawCircle(0, 0, size);
        this.avatar.endFill();

        const eyeOffset = size * 0.32;
        const eyeSize = size * 0.15;
        this.avatar.beginFill(0x000000, 1);
        this.avatar.drawCircle(-eyeOffset, -3, eyeSize);
        this.avatar.drawCircle(eyeOffset, -3, eyeSize);
        this.avatar.endFill();
        this.avatar.lineStyle(1.5, 0x000000, 1);
        this.avatar.arc(0, 3, size * 0.35, 0, Math.PI);
        this.avatar.scale.set(scale);
    }

    showThinkingBubble() {
        this.hideActivity();

        this.bubble = new PIXI.Graphics();
        this.bubble.beginFill(0xffffff, 0.95);
        this.bubble.lineStyle(1, 0x4A90E2, 0.9);
        this.bubble.drawRoundedRect(-25, -60, 50, 35, 10);
        this.bubble.endFill();
        this.bubble.beginFill(0xffffff, 0.95);
        this.bubble.moveTo(-6, -25);
        this.bubble.lineStyle(1, 0x4A90E2, 0.9);
        this.bubble.lineTo(0, -16);
        this.bubble.lineTo(6, -25);
        this.bubble.endFill();
        this.bubble.zIndex = 4;
        this.container.addChild(this.bubble);

        this.bubbleText = new PIXI.Text(this.getStateEmoji(this.state), {
            fontSize: 20,
            resolution: 1.5
        });
        this.bubbleText.anchor.set(0.5);
        this.bubbleText.position.set(0, -42);
        this.bubbleText.zIndex = 5;
        this.container.addChild(this.bubbleText);
        this.emojiElapsed = 0;
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

    moveTo(x, y) {
        this.targetX = x;
        this.targetY = y;
        this.x = x;
        this.y = y;
        this.container.position.set(x, y);
    }

    updateState(newState) {
        if (this.state === newState) {
            return;
        }
        this.state = newState;
        if (['working', 'busy', 'idle'].includes(newState)) {
            this.showThinkingBubble();
        } else {
            this.hideActivity();
        }
        this.redrawAvatar(this.avatar?.scale.x || 1);
    }

    update(delta) {
        this.breatheElapsed += delta * (this.state === 'working' || this.state === 'busy' ? 0.12 : 0.08);
        const scale = 1 + Math.sin(this.breatheElapsed) * (this.state === 'working' || this.state === 'busy' ? 0.08 : 0.03);
        this.redrawAvatar(scale);

        if (this.bubbleText) {
            this.emojiElapsed += delta;
            this.bubble.alpha = 0.75 + (Math.sin(this.breatheElapsed * 1.8) + 1) * 0.125;
            if (this.emojiElapsed >= 180) {
                this.bubbleText.text = this.getStateEmoji(this.state);
                this.emojiElapsed = 0;
            }
        }
    }

    destroy() {
        this.hideActivity();
        this.container.destroy({ children: true });
    }
}

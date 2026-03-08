export class FacilityZone {
    constructor(scene, data) {
        this.scene = scene;
        this.type = data.type;
        this.x = data.x;
        this.y = data.y;
        this.width = data.width;
        this.height = data.height;
        this.graphics = null;
        this.emojiText = null;
        this.labelText = null;
    }

    create() {
        this.graphics = this.scene.add.graphics();
        this.render();
    }

    render() {
        if (!this.graphics) return;
        this.graphics.clear();

        // 背景
        this.graphics.fillStyle(0xFFFFFF, 1);
        this.graphics.fillRect(this.x, this.y, this.width, this.height);

        // 边框
        this.graphics.lineStyle(2, 0x999999);
        this.graphics.strokeRect(this.x, this.y, this.width, this.height);

        // 根据类型绘制图标和文字
        let emoji, label;
        switch(this.type) {
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
                label = this.type;
        }

        if (this.emojiText) this.emojiText.destroy();
        if (this.labelText) this.labelText.destroy();

        this.emojiText = this.scene.add.text(this.x + this.width/2, this.y + this.height/2 - 15, emoji, {
            fontSize: '32px',
            align: 'center'
        });
        this.emojiText.setOrigin(0.5, 0.5);

        this.labelText = this.scene.add.text(this.x + this.width/2, this.y + this.height/2 + 20, label, {
            fontSize: '12px',
            color: '#666',
            align: 'center'
        });
        this.labelText.setOrigin(0.5, 0.5);
    }

    destroy() {
        if (this.graphics) this.graphics.destroy();
        if (this.emojiText) this.emojiText.destroy();
        if (this.labelText) this.labelText.destroy();
    }
}

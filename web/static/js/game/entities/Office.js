export class Office {
    constructor(scene, x, y, width, height, teamName) {
        this.scene = scene;
        this.x = x;
        this.y = y;
        this.width = width;
        this.height = height;
        this.teamName = teamName;
        this.graphics = null;
        this.nameText = null;
    }

    create() {
        this.graphics = this.scene.add.graphics();
        this.render();
    }

    render() {
        if (!this.graphics) return;
        this.graphics.clear();

        // 墙壁
        this.graphics.fillStyle(0xE8E8E8, 1);
        this.graphics.fillRect(this.x, this.y, this.width, this.height);

        // 边框
        this.graphics.lineStyle(3, 0x666666);
        this.graphics.strokeRect(this.x, this.y, this.width, this.height);

        // 门
        this.graphics.fillStyle(0x8B4513, 1);
        this.graphics.fillRect(this.x + this.width/2 - 20, this.y + this.height - 5, 40, 5);

        // 窗户
        this.graphics.lineStyle(2, 0x87CEEB);
        this.graphics.strokeRect(this.x + 10, this.y + 10, 40, 30);

        // 团队名称
        if (this.nameText) this.nameText.destroy();
        this.nameText = this.scene.add.text(this.x + this.width/2, this.y + 20, this.teamName, {
            fontSize: '14px',
            fontStyle: 'bold',
            color: '#333'
        });
        this.nameText.setOrigin(0.5, 0);
    }

    destroy() {
        if (this.graphics) this.graphics.destroy();
        if (this.nameText) this.nameText.destroy();
    }
}

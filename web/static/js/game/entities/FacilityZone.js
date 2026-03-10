import * as PIXI from '../../vendor/pixi.js';

const FACILITY_META = {
    restroom: { emoji: '🚻', label: '洗手间' },
    cafe: { emoji: '☕', label: '茶水间' },
    gym: { emoji: '💪', label: '健身区' },
    boss: { emoji: '🏢', label: '老板办公室' }
};

export class FacilityZone {
    constructor(scene, data) {
        this.scene = scene;
        this.type = data.type;
        this.x = data.x;
        this.y = data.y;
        this.width = data.width;
        this.height = data.height;
        this.container = new PIXI.Container();
    }

    create() {
        const card = new PIXI.Graphics();
        card.beginFill(0xffffff, 0.96);
        card.lineStyle(2, 0x999999, 1);
        card.drawRoundedRect(this.x, this.y, this.width, this.height, 10);
        card.endFill();
        this.container.addChild(card);

        const meta = FACILITY_META[this.type] || { emoji: '📦', label: this.type };

        const emoji = new PIXI.Text(meta.emoji, { fontSize: 30, resolution: 1.5 });
        emoji.anchor.set(0.5);
        emoji.position.set(this.x + this.width / 2, this.y + this.height / 2 - 14);
        this.container.addChild(emoji);

        const label = new PIXI.Text(meta.label, {
            fontFamily: '-apple-system, BlinkMacSystemFont, "Segoe UI", Arial, sans-serif',
            fontSize: 12,
            fill: 0x666666,
            resolution: 1.5
        });
        label.anchor.set(0.5);
        label.position.set(this.x + this.width / 2, this.y + this.height / 2 + 20);
        this.container.addChild(label);

        this.scene.world.addChild(this.container);
    }

    destroy() {
        this.container.destroy({ children: true });
    }
}

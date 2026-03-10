import * as PIXI from '../../vendor/pixi.js';
import { Agent } from './Agent.js';

export class Team {
    constructor(scene, data) {
        this.scene = scene;
        this.name = data.name;
        this.agents = new Map();
        this.container = new PIXI.Container();
        this.container.sortableChildren = true;
        this.desk = null;
        this.nameText = null;
        this.x = 0;
        this.y = 0;
        this.rotation = 0;
    }

    create(x, y, rotation = 0) {
        this.x = x;
        this.y = y;
        this.rotation = rotation || 0;
        this.container.position.set(x, y);
        this.container.rotation = this.rotation * Math.PI / 180;
        this.scene.world.addChild(this.container);

        const agentCount = this.agents.size;
        const deskWidth = Math.max(200, agentCount * 60 + 40);
        const deskHeight = 80;

        this.desk = new PIXI.Graphics();
        this.desk.beginFill(0x8B4513, 1);
        this.desk.lineStyle(3, 0x654321, 1);
        this.desk.drawRoundedRect(-deskWidth / 2, -deskHeight / 2, deskWidth, deskHeight, 6);
        this.desk.endFill();
        this.desk.zIndex = 0;
        this.container.addChild(this.desk);

        const woodLines = new PIXI.Graphics();
        woodLines.lineStyle(1, 0x654321, 0.3);
        for (let i = 0; i < 3; i += 1) {
            const offsetY = -deskHeight / 2 + ((i + 1) * deskHeight) / 4;
            woodLines.moveTo(-deskWidth / 2, offsetY);
            woodLines.lineTo(deskWidth / 2, offsetY);
        }
        woodLines.zIndex = 1;
        this.container.addChild(woodLines);

        this.nameText = new PIXI.Text(this.name, {
            fontFamily: '-apple-system, BlinkMacSystemFont, "Segoe UI", Arial, sans-serif',
            fontSize: 16,
            fontWeight: '700',
            fill: 0xffffff,
            resolution: 1.5
        });
        this.nameText.anchor.set(0.5);
        this.nameText.position.set(0, 0);
        this.nameText.zIndex = 2;
        this.container.addChild(this.nameText);

        this.addComputers(deskWidth);
        this.layoutAgents(deskWidth, deskHeight);
    }

    addComputers(deskWidth) {
        const agentCount = this.agents.size;
        const spacing = deskWidth / (agentCount + 1);
        for (let i = 0; i < agentCount; i += 1) {
            const offsetX = -deskWidth / 2 + spacing * (i + 1);
            const computer = new PIXI.Graphics();
            computer.beginFill(0x333333, 1);
            computer.lineStyle(1, 0x666666, 1);
            computer.drawRoundedRect(offsetX - 15, -20, 30, 20, 4);
            computer.endFill();
            computer.zIndex = 1;
            this.container.addChild(computer);
        }
    }

    layoutAgents(deskWidth, deskHeight) {
        const agents = Array.from(this.agents.values());
        const agentCount = agents.length;
        if (agentCount === 0) {
            return;
        }

        if (agentCount <= 3) {
            const spacing = deskWidth / (agentCount + 1);
            agents.forEach((agent, index) => {
                const offsetX = -deskWidth / 2 + spacing * (index + 1);
                const offsetY = deskHeight / 2 + 60;
                agent.create(this.x + offsetX, this.y + offsetY);
            });
            return;
        }

        const topCount = Math.ceil(agentCount / 2);
        const bottomCount = agentCount - topCount;
        const topSpacing = deskWidth / (topCount + 1);
        for (let i = 0; i < topCount; i += 1) {
            const offsetX = -deskWidth / 2 + topSpacing * (i + 1);
            const offsetY = -deskHeight / 2 - 60;
            agents[i].create(this.x + offsetX, this.y + offsetY);
        }

        const bottomSpacing = deskWidth / (bottomCount + 1);
        for (let i = 0; i < bottomCount; i += 1) {
            const offsetX = -deskWidth / 2 + bottomSpacing * (i + 1);
            const offsetY = deskHeight / 2 + 60;
            agents[topCount + i].create(this.x + offsetX, this.y + offsetY);
        }
    }

    addAgent(agentData) {
        const isLead = this.agents.size === 0 ||
            agentData.name.toLowerCase().includes('lead') ||
            agentData.role === 'lead';

        const agent = new Agent(this.scene, {
            ...agentData,
            teamName: this.name,
            role: isLead ? 'lead' : 'member'
        });
        this.agents.set(agentData.name, agent);
        return agent;
    }

    update(delta) {
        this.agents.forEach((agent) => agent.update(delta));
    }

    destroy() {
        this.agents.forEach((agent) => agent.destroy());
        this.agents.clear();
        this.container.destroy({ children: true });
    }
}

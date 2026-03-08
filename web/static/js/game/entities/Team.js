import { Agent } from './Agent.js';
import { Office } from './Office.js';

export class Team {
    constructor(scene, data) {
        this.scene = scene;
        this.name = data.name;
        this.agents = new Map();
        this.office = null;
        this.x = 0;
        this.y = 0;
    }

    create(x, y) {
        this.x = x;
        this.y = y;

        // 计算办公室大小
        const agentCount = this.agents.size;
        const rows = Math.ceil(agentCount / 2);
        const width = 250;
        const height = Math.max(150, 80 + rows * 80);

        // 创建办公室
        this.office = new Office(this.scene, x, y, width, height, this.name);
        this.office.create();

        // 布局 agents（2列多行）
        this.layoutAgents();
    }

    layoutAgents() {
        const agents = Array.from(this.agents.values());
        const startX = this.x + 70;
        const startY = this.y + 60;
        const colSpacing = 120;
        const rowSpacing = 80;

        agents.forEach((agent, index) => {
            const col = index % 2;
            const row = Math.floor(index / 2);
            const agentX = startX + col * colSpacing;
            const agentY = startY + row * rowSpacing;
            agent.create(agentX, agentY);
        });
    }

    addAgent(agentData) {
        const agent = new Agent(this.scene, { ...agentData, teamName: this.name });
        this.agents.set(agentData.name, agent);
        return agent;
    }

    removeAgent(agentName) {
        const agent = this.agents.get(agentName);
        if (agent) {
            agent.destroy();
            this.agents.delete(agentName);
        }
    }

    destroy() {
        if (this.office) this.office.destroy();
        this.agents.forEach(agent => agent.destroy());
        this.agents.clear();
    }
}

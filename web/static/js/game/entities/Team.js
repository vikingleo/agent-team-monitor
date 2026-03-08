import { Agent } from './Agent.js';

export class Team {
    constructor(scene, data) {
        this.scene = scene;
        this.name = data.name;
        this.agents = new Map();
        this.desk = null;  // 办公桌对象
        this.nameText = null;
        this.x = 0;
        this.y = 0;
        this.rotation = 0;
    }

    create(x, y, rotation = 0) {
        this.x = x;
        this.y = y;
        this.rotation = rotation || 0;

        // 计算办公桌大小（长桌）
        const agentCount = this.agents.size;
        const deskWidth = Math.max(200, agentCount * 60 + 40);
        const deskHeight = 80;

        // 创建办公桌（长桌）
        this.desk = this.scene.add.rectangle(x, y, deskWidth, deskHeight, 0x8B4513);
        this.desk.setStrokeStyle(3, 0x654321);
        this.desk.setAngle(this.rotation);

        // 添加桌面纹理效果
        const graphics = this.scene.add.graphics();
        graphics.lineStyle(1, 0x654321, 0.3);
        for (let i = 0; i < 3; i++) {
            const offsetY = -deskHeight/2 + (i + 1) * deskHeight/4;
            graphics.lineBetween(
                x - deskWidth/2, y + offsetY,
                x + deskWidth/2, y + offsetY
            );
        }

        // 团队名称标签（在桌子中间）
        this.nameText = this.scene.add.text(x, y, this.name, {
            fontSize: '16px',
            fontFamily: '-apple-system, BlinkMacSystemFont, "Segoe UI", Arial, sans-serif',
            color: '#ffffff',
            fontStyle: 'bold',
            align: 'center',
            resolution: window.devicePixelRatio || 2  // 提高文字渲染分辨率
        });
        this.nameText.setOrigin(0.5);
        this.nameText.setAngle(this.rotation);
        this.nameText.setDepth(1);

        // 在桌子上添加电脑
        this.addComputers();

        // 布局 agents（围绕桌子坐）
        this.layoutAgents();
    }

    addComputers() {
        const agentCount = this.agents.size;
        const deskWidth = this.desk.width;
        const spacing = deskWidth / (agentCount + 1);

        // 为每个 agent 位置添加一台电脑
        for (let i = 0; i < agentCount; i++) {
            const offsetX = -deskWidth/2 + spacing * (i + 1);
            const offsetY = -10;  // 桌子上方一点

            // 应用旋转
            const rad = this.rotation * Math.PI / 180;
            const rotatedX = offsetX * Math.cos(rad) - offsetY * Math.sin(rad);
            const rotatedY = offsetX * Math.sin(rad) + offsetY * Math.cos(rad);

            const computerX = this.x + rotatedX;
            const computerY = this.y + rotatedY;

            // 绘制电脑（简单的矩形表示）
            const computer = this.scene.add.rectangle(computerX, computerY, 30, 20, 0x333333);
            computer.setStrokeStyle(1, 0x666666);
            computer.setAngle(this.rotation);
            computer.setDepth(1);
        }
    }

    layoutAgents() {
        const agents = Array.from(this.agents.values());
        const agentCount = agents.length;
        const deskWidth = this.desk.width;
        const deskHeight = this.desk.height;

        if (agentCount === 0) return;

        // 根据 agent 数量决定布局策略
        if (agentCount <= 3) {
            // 少量 agent：只在下方一排
            const spacing = deskWidth / (agentCount + 1);
            agents.forEach((agent, index) => {
                const offsetX = -deskWidth/2 + spacing * (index + 1);
                const offsetY = deskHeight/2 + 60;  // 桌子下方，增加距离避免重叠

                const agentX = this.x + offsetX;
                const agentY = this.y + offsetY;

                agent.create(agentX, agentY);
            });
        } else {
            // 多个 agent：分布在桌子两侧（长边）
            const topCount = Math.ceil(agentCount / 2);
            const bottomCount = agentCount - topCount;

            // 上方（桌子上边）
            const topSpacing = deskWidth / (topCount + 1);
            for (let i = 0; i < topCount; i++) {
                const offsetX = -deskWidth/2 + topSpacing * (i + 1);
                const offsetY = -deskHeight/2 - 60;  // 桌子上方

                const agentX = this.x + offsetX;
                const agentY = this.y + offsetY;

                agents[i].create(agentX, agentY);
            }

            // 下方（桌子下边）
            const bottomSpacing = deskWidth / (bottomCount + 1);
            for (let i = 0; i < bottomCount; i++) {
                const offsetX = -deskWidth/2 + bottomSpacing * (i + 1);
                const offsetY = deskHeight/2 + 60;  // 桌子下方

                const agentX = this.x + offsetX;
                const agentY = this.y + offsetY;

                agents[topCount + i].create(agentX, agentY);
            }
        }
    }

    addAgent(agentData) {
        // 检测是否为 team lead（第一个 agent 或名称包含 lead）
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

    removeAgent(agentName) {
        const agent = this.agents.get(agentName);
        if (agent) {
            agent.destroy();
            this.agents.delete(agentName);
        }
    }

    destroy() {
        if (this.desk) this.desk.destroy();
        if (this.nameText) this.nameText.destroy();
        this.agents.forEach(agent => agent.destroy());
        this.agents.clear();
    }
}

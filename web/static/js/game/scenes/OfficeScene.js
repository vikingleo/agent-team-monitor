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

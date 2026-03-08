import { ResourceManager } from '../systems/ResourceManager.js';
import { LayoutManager } from '../systems/LayoutManager.js';
import { DataSyncManager } from '../systems/DataSyncManager.js';
import { Team } from '../entities/Team.js';
import { FacilityZone } from '../entities/FacilityZone.js';

export class OfficeScene extends Phaser.Scene {
    constructor() {
        super({ key: 'OfficeScene' });
        this.resourceManager = null;
        this.layoutManager = null;
        this.dataSyncManager = null;
        this.teams = new Map();
        this.facilities = new Map();
    }

    preload() {
        console.log('OfficeScene: preload');
    }

    async create() {
        console.log('OfficeScene: create');

        // 初始化资源管理器
        this.resourceManager = new ResourceManager(this);
        await this.resourceManager.init();

        // 初始化布局管理器
        this.layoutManager = new LayoutManager(this);

        // 设置相机
        this.cameras.main.setBounds(0, 0, 2400, 1600);
        this.setupCameraControls();

        // 创建图形对象（用于绘制）
        this.graphics = this.add.graphics();

        // 启动数据同步
        this.dataSyncManager = new DataSyncManager(this);
        this.dataSyncManager.start();

        console.log('OfficeScene: ready');
    }

    setupCameraControls() {
        const camera = this.cameras.main;

        // 鼠标滚轮缩放
        this.input.on('wheel', (pointer, gameObjects, deltaX, deltaY, deltaZ) => {
            const zoomFactor = deltaY > 0 ? 0.9 : 1.1;
            const newZoom = Phaser.Math.Clamp(camera.zoom * zoomFactor, 0.5, 2);
            camera.setZoom(newZoom);
        });

        // 鼠标拖拽
        this.input.on('pointerdown', (pointer) => {
            if (pointer.leftButtonDown()) {
                this.isDragging = true;
                this.dragStartX = pointer.x;
                this.dragStartY = pointer.y;
                this.cameraStartX = camera.scrollX;
                this.cameraStartY = camera.scrollY;
            }
        });

        this.input.on('pointermove', (pointer) => {
            if (this.isDragging) {
                const deltaX = (pointer.x - this.dragStartX) / camera.zoom;
                const deltaY = (pointer.y - this.dragStartY) / camera.zoom;
                camera.scrollX = this.cameraStartX - deltaX;
                camera.scrollY = this.cameraStartY - deltaY;
            }
        });

        this.input.on('pointerup', () => {
            this.isDragging = false;
        });

        // 双击重置视角
        this.input.on('pointerdblclick', () => {
            this.resetCamera();
        });
    }

    resetCamera() {
        const camera = this.cameras.main;
        camera.setZoom(1);
        camera.centerOn(800, 400);
    }

    initializeState(state) {
        console.log('Initializing state:', state);

        // 清空现有场景
        this.clearScene();

        if (!state.teams || state.teams.length === 0) {
            console.log('No teams to display');
            return;
        }

        // 计算布局
        const layout = this.layoutManager.calculateLayout(state.teams);

        // 更新相机边界
        this.cameras.main.setBounds(0, 0, layout.bounds.width, layout.bounds.height);

        // 创建功能区
        layout.facilities.forEach(facilityData => {
            const facility = new FacilityZone(this, facilityData);
            facility.create();
            this.facilities.set(facilityData.type, facility);
        });

        // 创建团队和 agents
        state.teams.forEach(teamData => {
            const team = new Team(this, teamData);

            // 添加 agents
            if (teamData.agents) {
                teamData.agents.forEach(agentData => {
                    team.addAgent(agentData);
                });
            }

            // 获取布局位置
            const teamLayout = layout.teams.find(t => t.name === teamData.name);
            if (teamLayout) {
                team.create(teamLayout.x, teamLayout.y);
            }

            this.teams.set(teamData.name, team);
        });

        // 自动适配视角
        this.fitCameraToContent();
    }

    applyChanges(changes) {
        console.log('Applying changes:', changes);

        // 处理新增团队
        changes.teamsAdded.forEach(teamData => {
            console.log('Adding team:', teamData.name);
            // 重新计算布局并重建场景
            this.dataSyncManager.lastState = null; // 强制完全重建
        });

        // 处理删除团队
        changes.teamsRemoved.forEach(teamName => {
            console.log('Removing team:', teamName);
            const team = this.teams.get(teamName);
            if (team) {
                team.destroy();
                this.teams.delete(teamName);
            }
        });

        // 处理 agent 状态更新
        changes.agentsUpdated.forEach(agentData => {
            const team = this.teams.get(agentData.teamName);
            if (team) {
                const agent = team.agents.get(agentData.name);
                if (agent) {
                    agent.updateState(agentData.status);
                }
            }
        });
    }

    fitCameraToContent() {
        if (this.teams.size === 0) return;

        const camera = this.cameras.main;
        const padding = 100;

        // 计算所有团队的边界
        let minX = Infinity, minY = Infinity, maxX = -Infinity, maxY = -Infinity;

        this.teams.forEach(team => {
            if (team.office) {
                minX = Math.min(minX, team.office.x);
                minY = Math.min(minY, team.office.y);
                maxX = Math.max(maxX, team.office.x + team.office.width);
                maxY = Math.max(maxY, team.office.y + team.office.height);
            }
        });

        const contentWidth = maxX - minX + padding * 2;
        const contentHeight = maxY - minY + padding * 2;
        const centerX = (minX + maxX) / 2;
        const centerY = (minY + maxY) / 2;

        // 计算合适的缩放比例
        const zoomX = camera.width / contentWidth;
        const zoomY = camera.height / contentHeight;
        const zoom = Math.min(zoomX, zoomY, 1) * 0.9;

        camera.setZoom(zoom);
        camera.centerOn(centerX, centerY);
    }

    clearScene() {
        this.teams.forEach(team => team.destroy());
        this.teams.clear();

        this.facilities.forEach(facility => facility.destroy());
        this.facilities.clear();

        if (this.graphics) {
            this.graphics.clear();
        }
    }

    update(time, delta) {
        // 游戏循环
    }

    shutdown() {
        if (this.dataSyncManager) {
            this.dataSyncManager.stop();
        }
        this.clearScene();
    }
}

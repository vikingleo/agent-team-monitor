import * as PIXI from '../../vendor/pixi.js';
import { LayoutManager } from '../systems/LayoutManager.js';
import { DataSyncManager } from '../systems/DataSyncManager.js';
import { Team } from '../entities/Team.js';
import { FacilityZone } from '../entities/FacilityZone.js';
import { GameConfig } from '../config.js';

export class OfficeScene {
    constructor(app) {
        this.app = app;
        this.layoutManager = null;
        this.dataSyncManager = null;
        this.teams = new Map();
        this.facilities = new Map();
        this.zoneGraphics = [];
        this.world = new PIXI.Container();
        this.world.eventMode = 'static';
        this.world.sortableChildren = true;
        this.app.stage.addChild(this.world);
        this.onStateUpdated = null;
        this.currentState = null;
        this.bounds = { width: GameConfig.width(), height: GameConfig.height() };
        this.zoom = 1;
        this.minZoom = GameConfig.minZoom;
        this.maxZoom = GameConfig.maxZoom;
        this.isDragging = false;
        this.dragOrigin = null;
        this.dragWorldOrigin = null;
        this.lastTapTime = 0;
        this.boundTick = this.tick.bind(this);
    }

    async init() {
        this.layoutManager = new LayoutManager(this);
        this.setupInteractions();
        this.app.ticker.add(this.boundTick);
        this.dataSyncManager = new DataSyncManager(this);
        this.dataSyncManager.start();
        console.log('OfficeScene: ready (PixiJS)');
    }

    setupInteractions() {
        const view = this.app.view;
        view.addEventListener('wheel', (event) => {
            event.preventDefault();
            const nextZoom = event.deltaY > 0 ? this.zoom * 0.9 : this.zoom * 1.1;
            this.setZoom(nextZoom, event.offsetX, event.offsetY);
        }, { passive: false });

        view.addEventListener('pointerdown', (event) => {
            if (event.button !== 0) {
                return;
            }

            const now = Date.now();
            if (now - this.lastTapTime < 280) {
                this.resetCameraToFullView();
                this.lastTapTime = 0;
                return;
            }
            this.lastTapTime = now;

            this.isDragging = true;
            this.dragOrigin = { x: event.clientX, y: event.clientY };
            this.dragWorldOrigin = { x: this.world.x, y: this.world.y };
        });

        window.addEventListener('pointermove', (event) => {
            if (!this.isDragging || !this.dragOrigin || !this.dragWorldOrigin) {
                return;
            }
            const deltaX = event.clientX - this.dragOrigin.x;
            const deltaY = event.clientY - this.dragOrigin.y;
            this.world.x = this.dragWorldOrigin.x + deltaX;
            this.world.y = this.dragWorldOrigin.y + deltaY;
            this.clampWorldPosition();
        });

        const stopDrag = () => {
            this.isDragging = false;
            this.dragOrigin = null;
            this.dragWorldOrigin = null;
        };

        window.addEventListener('pointerup', stopDrag);
        window.addEventListener('pointercancel', stopDrag);
    }

    tick(delta) {
        this.teams.forEach((team) => team.update(delta));
    }

    initializeState(state) {
        this.currentState = state;
        this.clearScene();

        if (!state.teams || state.teams.length === 0) {
            this.emitStateUpdate(state);
            return;
        }

        const layout = this.layoutManager.calculateLayout(state.teams);
        this.bounds = layout.bounds;
        this.drawZones(layout.zones || []);

        layout.facilities.forEach((facilityData) => {
            const facility = new FacilityZone(this, facilityData);
            facility.create();
            this.facilities.set(facilityData.type, facility);
        });

        state.teams.forEach((teamData) => {
            const team = new Team(this, teamData);
            (teamData.members || []).forEach((agentData) => team.addAgent(agentData));
            const teamLayout = layout.teams.find((item) => item.name === teamData.name);
            if (teamLayout) {
                team.create(teamLayout.x, teamLayout.y, teamLayout.rotation || 0);
            }
            this.teams.set(teamData.name, team);
        });

        this.resetCameraToFullView();
        this.emitStateUpdate(state);
    }

    applyChanges(changes) {
        if (changes.teamsAdded.length > 0 || changes.teamsRemoved.length > 0) {
            this.dataSyncManager.lastState = null;
            return;
        }

        changes.agentsUpdated.forEach((agentData) => {
            const team = this.teams.get(agentData.teamName);
            const agent = team?.agents.get(agentData.name);
            if (agent) {
                agent.updateState(agentData.status);
            }
        });
    }

    emitStateUpdate(state) {
        if (typeof this.onStateUpdated === 'function') {
            this.onStateUpdated(state);
        }
    }

    drawZones(zones) {
        this.zoneGraphics.forEach((item) => item.destroy());
        this.zoneGraphics = [];

        zones.forEach((zoneData) => {
            const graphic = new PIXI.Graphics();
            graphic.beginFill(zoneData.color, 0.25);
            graphic.lineStyle(2, zoneData.color, 0.7);
            graphic.drawRect(zoneData.x, zoneData.y, zoneData.width, zoneData.height);
            graphic.endFill();
            graphic.zIndex = -10;
            this.world.addChild(graphic);

            const label = new PIXI.Text(zoneData.name, {
                fontFamily: '-apple-system, BlinkMacSystemFont, "Segoe UI", Arial, sans-serif',
                fontSize: 28,
                fontWeight: '700',
                fill: 0x666666,
                resolution: GameConfig.resolution()
            });
            label.anchor.set(0.5, 0.5);
            label.position.set(zoneData.x + zoneData.width / 2, zoneData.y + 30);
            label.zIndex = -9;
            this.world.addChild(label);
            this.zoneGraphics.push(graphic, label);
        });
    }

    setZoom(nextZoom, screenX = this.app.renderer.width / 2, screenY = this.app.renderer.height / 2) {
        const clamped = Math.min(this.maxZoom, Math.max(this.minZoom, nextZoom));
        const worldX = (screenX - this.world.x) / this.zoom;
        const worldY = (screenY - this.world.y) / this.zoom;

        this.zoom = clamped;
        this.world.scale.set(this.zoom);
        this.world.x = screenX - worldX * this.zoom;
        this.world.y = screenY - worldY * this.zoom;
        this.clampWorldPosition();
    }

    resetCameraToFullView() {
        this.zoom = 1;
        this.world.scale.set(1);
        this.world.x = 0;
        this.world.y = 0;
        this.centerWorld();
    }

    centerWorld() {
        const viewportWidth = this.app.renderer.width;
        const viewportHeight = this.app.renderer.height;
        const contentWidth = this.bounds.width * this.zoom;
        const contentHeight = this.bounds.height * this.zoom;

        this.world.x = contentWidth < viewportWidth ? (viewportWidth - contentWidth) / 2 : 0;
        this.world.y = contentHeight < viewportHeight ? (viewportHeight - contentHeight) / 2 : 0;
        this.clampWorldPosition();
    }

    clampWorldPosition() {
        const viewportWidth = this.app.renderer.width;
        const viewportHeight = this.app.renderer.height;
        const contentWidth = this.bounds.width * this.zoom;
        const contentHeight = this.bounds.height * this.zoom;

        const minX = Math.min(0, viewportWidth - contentWidth);
        const minY = Math.min(0, viewportHeight - contentHeight);
        const maxX = contentWidth < viewportWidth ? (viewportWidth - contentWidth) / 2 : 0;
        const maxY = contentHeight < viewportHeight ? (viewportHeight - contentHeight) / 2 : 0;

        this.world.x = Math.min(maxX, Math.max(minX, this.world.x));
        this.world.y = Math.min(maxY, Math.max(minY, this.world.y));
    }

    handleResize() {
        this.bounds.width = Math.max(this.bounds.width, this.app.renderer.width);
        this.bounds.height = Math.max(this.bounds.height, this.app.renderer.height);
        this.centerWorld();
    }

    clearScene() {
        this.teams.forEach((team) => team.destroy());
        this.teams.clear();
        this.facilities.forEach((facility) => facility.destroy());
        this.facilities.clear();
        this.zoneGraphics.forEach((item) => item.destroy());
        this.zoneGraphics = [];
    }

    destroy() {
        if (this.dataSyncManager) {
            this.dataSyncManager.stop();
        }
        this.app.ticker.remove(this.boundTick);
        this.clearScene();
        this.world.destroy({ children: true });
    }
}

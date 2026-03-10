import { LayoutManager } from '../systems/LayoutManager.js';
import { DataSyncManager } from '../systems/DataSyncManager.js';
import { GameConfig } from '../config.js';

const LEAD_COLORS = [
    { fill: '#ff6b6b', stroke: '#c92a2a' },
    { fill: '#4ecdc4', stroke: '#0d7377' },
    { fill: '#ffe66d', stroke: '#f4a261' },
    { fill: '#95e1d3', stroke: '#38a3a5' },
    { fill: '#a8dadc', stroke: '#457b9d' },
    { fill: '#f4a261', stroke: '#e76f51' },
    { fill: '#b8b8ff', stroke: '#6a67ce' },
    { fill: '#ffafcc', stroke: '#f72585' }
];

const MEMBER_COLOR = { fill: '#4a90e2', stroke: '#2e5c8a' };
const WORKING_COLOR = { fill: '#4caf50', stroke: '#2e7d32' };
const ZONE_LABEL_FONT = '700 28px -apple-system, BlinkMacSystemFont, "Segoe UI", Arial, sans-serif';
const TEAM_FONT = '700 16px -apple-system, BlinkMacSystemFont, "Segoe UI", Arial, sans-serif';
const AGENT_FONT = '13px -apple-system, BlinkMacSystemFont, "Segoe UI", Arial, sans-serif';

const STATE_EMOJI = {
    working: ['💻', '⚙️', '🔧', '📝', '🎯'],
    busy: ['⏰', '🔥', '⚡', '💪', '🚀'],
    idle: ['😊', '🏊', '⚽', '🎮', '☕', '🎵', '🌴', '🎨', '📚', '🌞'],
    completed: ['✅', '🎉', '🏁']
};

export class CanvasOfficeScene {
    constructor(canvas) {
        this.canvas = canvas;
        this.ctx = canvas.getContext('2d');
        this.layoutManager = null;
        this.dataSyncManager = null;
        this.onStateUpdated = null;
        this.currentState = null;
        this.layout = { teams: [], facilities: [], zones: [], bounds: { width: 1, height: 1 } };
        this.bounds = { width: 1, height: 1 };
        this.viewportWidth = GameConfig.width();
        this.viewportHeight = GameConfig.height();
        this.dpr = GameConfig.resolution();
        this.zoom = 1;
        this.offsetX = 0;
        this.offsetY = 0;
        this.cameraInitialized = false;
        this.pendingFit = false;
        this.dragging = false;
        this.dragState = null;
        this.lastTapTime = 0;
        this.userMovedCamera = false;
        this.animationFrame = null;

        this.handleWheel = this.handleWheel.bind(this);
        this.handlePointerDown = this.handlePointerDown.bind(this);
        this.handlePointerMove = this.handlePointerMove.bind(this);
        this.handlePointerUp = this.handlePointerUp.bind(this);
        this.handleDoubleClick = this.handleDoubleClick.bind(this);
        this.renderLoop = this.renderLoop.bind(this);
    }

    async init() {
        if (!this.ctx) {
            throw new Error('Canvas 2D context is unavailable');
        }

        this.layoutManager = new LayoutManager(this);
        this.resizeCanvas();
        this.setupInteractions();
        this.dataSyncManager = new DataSyncManager(this);
        this.dataSyncManager.start();
        this.animationFrame = window.requestAnimationFrame(this.renderLoop);
    }

    setupInteractions() {
        this.canvas.addEventListener('wheel', this.handleWheel, { passive: false });
        this.canvas.addEventListener('pointerdown', this.handlePointerDown);
        window.addEventListener('pointermove', this.handlePointerMove);
        window.addEventListener('pointerup', this.handlePointerUp);
        window.addEventListener('pointercancel', this.handlePointerUp);
        this.canvas.addEventListener('dblclick', this.handleDoubleClick);
        this.canvas.style.touchAction = 'none';
    }

    handleWheel(event) {
        event.preventDefault();
        const rect = this.canvas.getBoundingClientRect();
        const screenX = event.clientX - rect.left;
        const screenY = event.clientY - rect.top;
        const factor = event.deltaY > 0 ? 0.9 : 1.1;
        this.setZoom(this.zoom * factor, screenX, screenY);
        this.userMovedCamera = true;
    }

    handlePointerDown(event) {
        if (event.button !== 0) {
            return;
        }

        const now = Date.now();
        if (now - this.lastTapTime < 280) {
            this.resetCameraToFullView();
            this.userMovedCamera = false;
            this.lastTapTime = 0;
            return;
        }
        this.lastTapTime = now;

        this.dragging = true;
        this.dragState = {
            x: event.clientX,
            y: event.clientY,
            offsetX: this.offsetX,
            offsetY: this.offsetY
        };
    }

    handlePointerMove(event) {
        if (!this.dragging || !this.dragState) {
            return;
        }

        const deltaX = event.clientX - this.dragState.x;
        const deltaY = event.clientY - this.dragState.y;
        this.offsetX = this.dragState.offsetX + deltaX;
        this.offsetY = this.dragState.offsetY + deltaY;
        this.clampWorldPosition();

        if (Math.abs(deltaX) > 2 || Math.abs(deltaY) > 2) {
            this.userMovedCamera = true;
        }
    }

    handlePointerUp() {
        this.dragging = false;
        this.dragState = null;
    }

    handleDoubleClick() {
        this.resetCameraToFullView();
        this.userMovedCamera = false;
    }

    resizeCanvas() {
        this.viewportWidth = GameConfig.width();
        this.viewportHeight = GameConfig.height();
        this.dpr = GameConfig.resolution();
        this.canvas.width = Math.max(1, Math.floor(this.viewportWidth * this.dpr));
        this.canvas.height = Math.max(1, Math.floor(this.viewportHeight * this.dpr));
        this.canvas.style.width = `${this.viewportWidth}px`;
        this.canvas.style.height = `${this.viewportHeight}px`;
    }

    initializeState(state) {
        this.currentState = state;
        this.pendingFit = true;
        this.recalculateLayout(true);
    }

    applyChanges(changes) {
        if (changes.teamsAdded.length > 0 || changes.teamsRemoved.length > 0) {
            this.pendingFit = true;
        }
    }

    emitStateUpdate(state) {
        this.currentState = state;
        this.recalculateLayout(this.pendingFit || !this.cameraInitialized || !this.userMovedCamera);
        this.pendingFit = false;

        if (typeof this.onStateUpdated === 'function') {
            this.onStateUpdated(state);
        }
    }

    recalculateLayout(shouldFitCamera = false) {
        const teams = this.currentState?.teams || [];
        this.layout = this.layoutManager.calculateLayout(teams);
        this.bounds = this.layout.bounds || { width: this.viewportWidth, height: this.viewportHeight };

        if (shouldFitCamera) {
            this.resetCameraToFullView();
            return;
        }

        this.clampWorldPosition();
    }

    setZoom(nextZoom, screenX = this.viewportWidth / 2, screenY = this.viewportHeight / 2) {
        const clamped = Math.min(GameConfig.maxZoom, Math.max(GameConfig.minZoom, nextZoom));
        const worldX = (screenX - this.offsetX) / this.zoom;
        const worldY = (screenY - this.offsetY) / this.zoom;

        this.zoom = clamped;
        this.offsetX = screenX - worldX * this.zoom;
        this.offsetY = screenY - worldY * this.zoom;
        this.clampWorldPosition();
    }

    resetCameraToFullView() {
        const padding = 32;
        const contentWidth = Math.max(this.bounds.width, 1);
        const contentHeight = Math.max(this.bounds.height, 1);
        const fitScale = Math.min(
            (this.viewportWidth - padding * 2) / contentWidth,
            (this.viewportHeight - padding * 2) / contentHeight
        );

        this.zoom = Math.min(GameConfig.maxZoom, Math.max(GameConfig.minZoom, Number.isFinite(fitScale) ? fitScale : 1));
        this.offsetX = (this.viewportWidth - contentWidth * this.zoom) / 2;
        this.offsetY = (this.viewportHeight - contentHeight * this.zoom) / 2;
        this.cameraInitialized = true;
        this.clampWorldPosition();
    }

    clampWorldPosition() {
        const contentWidth = this.bounds.width * this.zoom;
        const contentHeight = this.bounds.height * this.zoom;

        if (contentWidth <= this.viewportWidth) {
            this.offsetX = (this.viewportWidth - contentWidth) / 2;
        } else {
            const minX = this.viewportWidth - contentWidth;
            this.offsetX = Math.max(minX, Math.min(0, this.offsetX));
        }

        if (contentHeight <= this.viewportHeight) {
            this.offsetY = (this.viewportHeight - contentHeight) / 2;
        } else {
            const minY = this.viewportHeight - contentHeight;
            this.offsetY = Math.max(minY, Math.min(0, this.offsetY));
        }
    }

    handleResize() {
        this.resizeCanvas();
        this.recalculateLayout(!this.userMovedCamera);
    }

    renderLoop(timestamp) {
        this.render(timestamp);
        this.animationFrame = window.requestAnimationFrame(this.renderLoop);
    }

    render(timestamp) {
        const ctx = this.ctx;
        ctx.setTransform(this.dpr, 0, 0, this.dpr, 0, 0);
        ctx.clearRect(0, 0, this.viewportWidth, this.viewportHeight);
        ctx.fillStyle = '#f5f5f5';
        ctx.fillRect(0, 0, this.viewportWidth, this.viewportHeight);

        if (!this.currentState || !Array.isArray(this.currentState.teams) || this.currentState.teams.length === 0) {
            this.drawEmptyState(ctx);
            return;
        }

        ctx.save();
        ctx.translate(this.offsetX, this.offsetY);
        ctx.scale(this.zoom, this.zoom);

        this.drawZones(ctx, this.layout.zones || []);
        this.drawFacilities(ctx, this.layout.facilities || []);
        this.drawTeams(ctx, timestamp, this.currentState.teams || [], this.layout.teams || []);

        ctx.restore();
    }

    drawEmptyState(ctx) {
        ctx.fillStyle = '#666';
        ctx.font = '600 18px -apple-system, BlinkMacSystemFont, "Segoe UI", Arial, sans-serif';
        ctx.textAlign = 'center';
        ctx.fillText('暂无团队数据', this.viewportWidth / 2, this.viewportHeight / 2);
    }

    drawZones(ctx, zones) {
        zones.forEach((zone) => {
            ctx.save();
            ctx.fillStyle = this.withAlpha(zone.color, 0.28);
            ctx.strokeStyle = this.withAlpha(zone.color, 0.7);
            ctx.lineWidth = 2;
            ctx.fillRect(zone.x, zone.y, zone.width, zone.height);
            ctx.strokeRect(zone.x, zone.y, zone.width, zone.height);
            ctx.fillStyle = '#666666';
            ctx.font = ZONE_LABEL_FONT;
            ctx.textAlign = 'center';
            ctx.fillText(zone.name, zone.x + zone.width / 2, zone.y + 36);
            ctx.restore();
        });
    }

    drawFacilities(ctx, facilities) {
        facilities.forEach((facility) => {
            const meta = this.getFacilityMeta(facility.type);
            ctx.save();
            this.roundRect(ctx, facility.x, facility.y, facility.width, facility.height, 10);
            ctx.fillStyle = 'rgba(255,255,255,0.96)';
            ctx.fill();
            ctx.strokeStyle = '#999999';
            ctx.lineWidth = 2;
            ctx.stroke();

            ctx.textAlign = 'center';
            ctx.font = '30px -apple-system, BlinkMacSystemFont, "Segoe UI Emoji", sans-serif';
            ctx.fillStyle = '#222222';
            ctx.fillText(meta.emoji, facility.x + facility.width / 2, facility.y + facility.height / 2 - 8);

            ctx.font = '12px -apple-system, BlinkMacSystemFont, "Segoe UI", Arial, sans-serif';
            ctx.fillStyle = '#666666';
            ctx.fillText(meta.label, facility.x + facility.width / 2, facility.y + facility.height / 2 + 24);
            ctx.restore();
        });
    }

    drawTeams(ctx, timestamp, teams, teamLayouts) {
        const layoutMap = new Map(teamLayouts.map((item) => [item.name, item]));
        teams.forEach((team) => {
            const layout = layoutMap.get(team.name);
            if (!layout) {
                return;
            }
            this.drawTeam(ctx, timestamp, team, layout);
        });
    }

    drawTeam(ctx, timestamp, team, layout) {
        const members = Array.isArray(team.members) ? team.members : [];
        const deskWidth = Math.max(200, members.length * 60 + 40);
        const deskHeight = 80;
        const rotation = ((layout.rotation || 0) * Math.PI) / 180;

        ctx.save();
        ctx.translate(layout.x, layout.y);
        ctx.rotate(rotation);

        this.roundRect(ctx, -deskWidth / 2, -deskHeight / 2, deskWidth, deskHeight, 6);
        ctx.fillStyle = '#8b4513';
        ctx.fill();
        ctx.strokeStyle = '#654321';
        ctx.lineWidth = 3;
        ctx.stroke();

        ctx.strokeStyle = 'rgba(101,67,33,0.35)';
        ctx.lineWidth = 1;
        for (let i = 0; i < 3; i += 1) {
            const offsetY = -deskHeight / 2 + ((i + 1) * deskHeight) / 4;
            ctx.beginPath();
            ctx.moveTo(-deskWidth / 2, offsetY);
            ctx.lineTo(deskWidth / 2, offsetY);
            ctx.stroke();
        }

        const spacing = deskWidth / Math.max(members.length + 1, 2);
        for (let i = 0; i < members.length; i += 1) {
            const offsetX = -deskWidth / 2 + spacing * (i + 1);
            this.roundRect(ctx, offsetX - 15, -20, 30, 20, 4);
            ctx.fillStyle = '#333333';
            ctx.fill();
            ctx.strokeStyle = '#666666';
            ctx.lineWidth = 1;
            ctx.stroke();
        }

        ctx.fillStyle = '#ffffff';
        ctx.font = TEAM_FONT;
        ctx.textAlign = 'center';
        ctx.fillText(team.name, 0, 6);
        ctx.restore();

        const positions = this.layoutAgentPositions(members.length, deskWidth, deskHeight);
        members.forEach((agent, index) => {
            const local = positions[index] || { x: 0, y: deskHeight / 2 + 60 };
            const world = this.rotatePoint(local.x, local.y, rotation);
            this.drawAgent(ctx, timestamp, team, agent, layout.x + world.x, layout.y + world.y, index);
        });
    }

    drawAgent(ctx, timestamp, team, agent, x, y, index) {
        const state = String(agent.status || 'idle').toLowerCase();
        const avatar = this.getAvatarConfig(team.name, agent, index);
        const phase = this.hashString(`${team.name}:${agent.name}`) % 360;
        const pulse = Math.sin(timestamp / 500 + phase);
        const amplitude = state === 'working' || state === 'busy' ? 0.08 : 0.03;
        const scale = 1 + pulse * amplitude;
        const radius = avatar.size * scale;
        const fill = state === 'working' || state === 'busy' ? WORKING_COLOR.fill : avatar.fill;
        const stroke = state === 'working' || state === 'busy' ? WORKING_COLOR.stroke : avatar.stroke;

        ctx.save();
        ctx.translate(x, y);

        ctx.beginPath();
        ctx.arc(0, 0, radius, 0, Math.PI * 2);
        ctx.fillStyle = fill;
        ctx.fill();
        ctx.lineWidth = avatar.strokeWidth;
        ctx.strokeStyle = stroke;
        ctx.stroke();

        const eyeOffset = radius * 0.32;
        const eyeSize = Math.max(2, radius * 0.15);
        ctx.beginPath();
        ctx.fillStyle = '#000000';
        ctx.arc(-eyeOffset, -3, eyeSize, 0, Math.PI * 2);
        ctx.arc(eyeOffset, -3, eyeSize, 0, Math.PI * 2);
        ctx.fill();

        ctx.beginPath();
        ctx.strokeStyle = '#000000';
        ctx.lineWidth = 1.5;
        ctx.arc(0, 3, radius * 0.35, 0, Math.PI, false);
        ctx.stroke();

        this.drawBubble(ctx, timestamp, team.name, agent.name, state, radius);

        ctx.fillStyle = avatar.labelColor;
        ctx.font = `${avatar.isLead ? '700' : '400'} ${avatar.isLead ? 14 : 13}px -apple-system, BlinkMacSystemFont, "Segoe UI", Arial, sans-serif`;
        ctx.textAlign = 'center';
        ctx.fillText(agent.name, 0, avatar.size + 28);
        ctx.restore();
    }

    drawBubble(ctx, timestamp, teamName, agentName, state, radius) {
        const emojis = STATE_EMOJI[state] || STATE_EMOJI.idle;
        const bucket = Math.floor(timestamp / 3000);
        const emoji = emojis[this.hashString(`${teamName}:${agentName}:${state}:${bucket}`) % emojis.length];

        const bubbleX = -25;
        const bubbleY = -60;
        this.roundRect(ctx, bubbleX, bubbleY, 50, 35, 10);
        ctx.fillStyle = 'rgba(255,255,255,0.95)';
        ctx.fill();
        ctx.strokeStyle = '#4a90e2';
        ctx.lineWidth = 1;
        ctx.stroke();

        ctx.beginPath();
        ctx.moveTo(-6, -25);
        ctx.lineTo(0, -16);
        ctx.lineTo(6, -25);
        ctx.closePath();
        ctx.fillStyle = 'rgba(255,255,255,0.95)';
        ctx.fill();
        ctx.stroke();

        ctx.font = '20px -apple-system, BlinkMacSystemFont, "Segoe UI Emoji", sans-serif';
        ctx.textAlign = 'center';
        ctx.fillStyle = '#222222';
        ctx.fillText(emoji, 0, -34);
    }

    layoutAgentPositions(agentCount, deskWidth, deskHeight) {
        if (agentCount <= 0) {
            return [];
        }

        const positions = [];
        if (agentCount <= 3) {
            const spacing = deskWidth / (agentCount + 1);
            for (let index = 0; index < agentCount; index += 1) {
                positions.push({
                    x: -deskWidth / 2 + spacing * (index + 1),
                    y: deskHeight / 2 + 60
                });
            }
            return positions;
        }

        const topCount = Math.ceil(agentCount / 2);
        const bottomCount = agentCount - topCount;
        const topSpacing = deskWidth / (topCount + 1);
        for (let index = 0; index < topCount; index += 1) {
            positions.push({
                x: -deskWidth / 2 + topSpacing * (index + 1),
                y: -deskHeight / 2 - 60
            });
        }

        const bottomSpacing = deskWidth / (bottomCount + 1);
        for (let index = 0; index < bottomCount; index += 1) {
            positions.push({
                x: -deskWidth / 2 + bottomSpacing * (index + 1),
                y: deskHeight / 2 + 60
            });
        }

        return positions;
    }

    getAvatarConfig(teamName, agent, index) {
        const isLead = index === 0 || String(agent.name || '').toLowerCase().includes('lead') || agent.role === 'lead';
        if (isLead) {
            const scheme = LEAD_COLORS[this.hashString(teamName) % LEAD_COLORS.length];
            return {
                ...scheme,
                size: 20,
                strokeWidth: 3,
                labelColor: '#000000',
                isLead: true
            };
        }

        return {
            ...MEMBER_COLOR,
            size: 18,
            strokeWidth: 2,
            labelColor: '#333333',
            isLead: false
        };
    }

    getFacilityMeta(type) {
        switch (type) {
            case 'restroom':
                return { emoji: '🚻', label: '洗手间' };
            case 'cafe':
                return { emoji: '☕', label: '茶水间' };
            case 'gym':
                return { emoji: '💪', label: '健身区' };
            case 'boss':
                return { emoji: '🏢', label: '老板办公室' };
            default:
                return { emoji: '📦', label: type };
        }
    }

    withAlpha(colorNumber, alpha) {
        const r = (colorNumber >> 16) & 255;
        const g = (colorNumber >> 8) & 255;
        const b = colorNumber & 255;
        return `rgba(${r}, ${g}, ${b}, ${alpha})`;
    }

    rotatePoint(x, y, angle) {
        const cos = Math.cos(angle);
        const sin = Math.sin(angle);
        return {
            x: x * cos - y * sin,
            y: x * sin + y * cos
        };
    }

    roundRect(ctx, x, y, width, height, radius) {
        const r = Math.min(radius, width / 2, height / 2);
        ctx.beginPath();
        ctx.moveTo(x + r, y);
        ctx.lineTo(x + width - r, y);
        ctx.quadraticCurveTo(x + width, y, x + width, y + r);
        ctx.lineTo(x + width, y + height - r);
        ctx.quadraticCurveTo(x + width, y + height, x + width - r, y + height);
        ctx.lineTo(x + r, y + height);
        ctx.quadraticCurveTo(x, y + height, x, y + height - r);
        ctx.lineTo(x, y + r);
        ctx.quadraticCurveTo(x, y, x + r, y);
        ctx.closePath();
    }

    hashString(value) {
        let hash = 0;
        for (let index = 0; index < value.length; index += 1) {
            hash = ((hash << 5) - hash) + value.charCodeAt(index);
            hash |= 0;
        }
        return Math.abs(hash);
    }

    destroy() {
        this.canvas.removeEventListener('wheel', this.handleWheel);
        this.canvas.removeEventListener('pointerdown', this.handlePointerDown);
        this.canvas.removeEventListener('dblclick', this.handleDoubleClick);
        window.removeEventListener('pointermove', this.handlePointerMove);
        window.removeEventListener('pointerup', this.handlePointerUp);
        window.removeEventListener('pointercancel', this.handlePointerUp);

        if (this.dataSyncManager) {
            this.dataSyncManager.stop();
        }
        if (this.animationFrame) {
            window.cancelAnimationFrame(this.animationFrame);
            this.animationFrame = null;
        }
    }
}

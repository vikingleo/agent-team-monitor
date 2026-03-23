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
const INACTIVE_THRESHOLD_MS = 20 * 60 * 1000;
const NO_TASK_WANDER_THRESHOLD_MS = 60 * 1000;
const BOSS_VISIT_MIN_MS = 2000;
const BOSS_VISIT_MAX_MS = 3000;
const LEISURE_VISIT_MIN_MS = 2200;
const LEISURE_VISIT_MAX_MS = 4200;
const LEISURE_COOLDOWN_MIN_MS = 12000;
const LEISURE_COOLDOWN_MAX_MS = 28000;
const MAX_FRAME_DELTA_MS = 48;
const INTERACTION_DURATION_MS = 5200;
const COMPLETION_BURST_MS = 2600;
const TURN_RESPONSE_MS = 180;
const ARRIVAL_DISTANCE_PX = 5;
const SLOT_MARKER_RADIUS = 5;
const STALE_SIGNAL_THRESHOLD_MS = 4 * 60 * 1000;
const LONG_RUNNING_THRESHOLD_MS = 8 * 60 * 1000;
const FOCUS_RING_DURATION_MS = 2200;

const FACILITY_EMOJI = {
    restroom: ['🚶', '🚻', '🫧'],
    cafe: ['☕', '🧋', '🍪'],
    gym: ['💪', '🏃', '🤸'],
    boss: ['📋', '🫡', '🧑‍💼']
};

const STATE_EMOJI = {
    working: ['💻', '⚙️', '🔧', '📝', '🎯'],
    busy: ['⏰', '🔥', '⚡', '💪', '🚀'],
    idle: ['😊', '🏊', '⚽', '🎮', '☕', '🎵', '🌴', '🎨', '📚', '🌞'],
    completed: ['✅', '🎉', '🏁']
};

const TOOL_EMOJI = {
    read: ['📖', '🔍', '🧭'],
    grep: ['🔎', '🧠', '🗺️'],
    search: ['🔎', '🌐', '🧭'],
    web: ['🌐', '📰', '🛰️'],
    browser: ['🌐', '🛰️', '🧭'],
    edit: ['✍️', '🛠️', '🧩'],
    write: ['✍️', '🛠️', '🧩'],
    bash: ['🧪', '🖥️', '⚙️'],
    terminal: ['🖥️', '⚙️', '🧪'],
    test: ['🧪', '✅', '📈'],
    todo: ['📝', '📋', '🎯']
};

const INTERACTION_STYLE = {
    assignment: {
        stroke: 'rgba(245, 158, 11, 0.9)',
        glow: 'rgba(251, 191, 36, 0.3)',
        fill: 'rgba(255, 247, 214, 0.96)'
    },
    sync: {
        stroke: 'rgba(59, 130, 246, 0.9)',
        glow: 'rgba(96, 165, 250, 0.28)',
        fill: 'rgba(235, 245, 255, 0.96)'
    }
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
        this.lastFrameTime = 0;
        this.actorStates = new Map();
        this.teamAnchors = new Map();
        this.facilityByType = new Map();
        this.previousAgentSnapshots = new Map();
        this.assignmentBursts = [];
        this.selectedActorKey = '';

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

    prepareState(state) {
        if (!state || !Array.isArray(state.teams)) {
            return state;
        }

        const now = Date.now();
        const teams = state.teams
            .map((team) => this.prepareTeamState(team, now))
            .filter(Boolean);

        return {
            ...state,
            teams
        };
    }

    prepareTeamState(team, now) {
        const members = Array.isArray(team?.members) ? team.members : [];
        const tasks = Array.isArray(team?.tasks) ? team.tasks : [];
        const visibleMembers = members.filter((agent) => this.shouldShowAgent(agent, tasks, now));
        const visibleTasks = tasks.filter((task) => this.shouldKeepTask(task, visibleMembers));
        const hasPendingTasks = this.hasPendingTasks(visibleTasks);

        if (visibleMembers.length === 0 && !hasPendingTasks) {
            return null;
        }

        return {
            ...team,
            members: visibleMembers,
            tasks: visibleTasks
        };
    }

    shouldKeepTask(task, visibleMembers) {
        if (String(task?.status || '').toLowerCase() !== 'completed') {
            return true;
        }

        const owner = String(task?.owner || '').trim();
        if (!owner) {
            return false;
        }

        return visibleMembers.some((member) => String(member?.name || '') === owner);
    }

    shouldShowAgent(agent, teamTasks, now) {
        if (!agent) {
            return false;
        }

        const status = String(agent.status || 'idle').toLowerCase();
        if (status === 'working' || status === 'busy') {
            return true;
        }

        const hasTask = this.agentHasTask(agent, teamTasks);
        if (hasTask) {
            return true;
        }

        const activityTs = this.getAgentActivityTimestamp(agent);
        if (!activityTs) {
            return false;
        }

        return now - activityTs <= INACTIVE_THRESHOLD_MS;
    }

    hasPendingTasks(tasks) {
        return (tasks || []).some((task) => String(task.status || '').toLowerCase() !== 'completed');
    }

    agentHasTask(agent, teamTasks) {
        if (String(agent.current_task || '').trim() !== '') {
            return true;
        }

        return this.getAgentTasks(agent, teamTasks).some((task) => String(task.status || '').toLowerCase() !== 'completed');
    }

    getAgentTasks(agent, teamTasks) {
        const agentName = String(agent?.name || '');
        return (teamTasks || []).filter((task) => {
            const owner = String(task.owner || '');
            const subject = String(task.subject || '');
            return owner === agentName || (!owner && subject === agentName);
        });
    }

    getAgentActivityTimestamp(agent) {
        const raw = agent?.last_active_time || agent?.last_message_time || agent?.last_activity;
        return this.toTimestamp(raw);
    }

    toTimestamp(value) {
        if (!value) {
            return 0;
        }
        if (typeof value === 'number' && Number.isFinite(value)) {
            return value;
        }
        const parsed = new Date(value);
        if (Number.isNaN(parsed.getTime())) {
            return 0;
        }
        return parsed.getTime();
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

        const hitActor = this.pickActorAt(event.clientX, event.clientY);
        if (hitActor) {
            this.selectActor(hitActor);
        } else {
            this.selectedActorKey = '';
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
        this.lastFrameTime = 0;
        this.recalculateLayout(true);
        this.syncActors(true);
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
        this.syncActors(false);

        if (typeof this.onStateUpdated === 'function') {
            this.onStateUpdated(state);
        }
    }

    recalculateLayout(shouldFitCamera = false) {
        const teams = this.currentState?.teams || [];
        this.layout = this.layoutManager.calculateLayout(teams);
        this.bounds = this.layout.bounds || { width: this.viewportWidth, height: this.viewportHeight };
        this.facilityByType = this.indexFacilities(this.layout.facilities || []);
        this.teamAnchors = this.buildTeamAnchors(teams, this.layout.teams || []);

        if (shouldFitCamera) {
            this.resetCameraToFullView();
            return;
        }

        this.clampWorldPosition();
    }

    indexFacilities(facilities) {
        const map = new Map();
        facilities.forEach((facility) => {
            const type = String(facility.type || '');
            if (!map.has(type)) {
                map.set(type, []);
            }
            map.get(type).push(facility);
        });
        return map;
    }

    buildTeamAnchors(teams, teamLayouts) {
        const anchors = new Map();
        const layoutMap = new Map(teamLayouts.map((item) => [item.name, item]));

        teams.forEach((team) => {
            const layout = layoutMap.get(team.name);
            if (!layout) {
                return;
            }

            const members = Array.isArray(team.members) ? team.members : [];
            const deskWidth = Math.max(200, members.length * 60 + 40);
            const deskHeight = 80;
            const rotation = ((layout.rotation || 0) * Math.PI) / 180;
            const positions = this.layoutAgentPositions(members.length, deskWidth, deskHeight);
            const memberAnchors = positions.map((local, index) => {
                const world = this.rotatePoint(local.x, local.y, rotation);
                return {
                    key: this.getAgentKey(team.name, members[index]?.name),
                    teamName: team.name,
                    agentName: members[index]?.name,
                    index,
                    isLead: this.isLeadAgent(members[index], index),
                    homeX: layout.x + world.x,
                    homeY: layout.y + world.y,
                    deskX: layout.x,
                    deskY: layout.y,
                    zone: layout.zone,
                    rotation
                };
            });

            anchors.set(team.name, {
                layout,
                deskWidth,
                deskHeight,
                rotation,
                members: memberAnchors
            });
        });

        return anchors;
    }

    getAgentKey(teamName, agentName) {
        return `${teamName || 'team'}::${agentName || 'agent'}`;
    }

    isLeadAgent(agent, index) {
        return index === 0 || String(agent?.name || '').toLowerCase().includes('lead') || agent?.role === 'lead';
    }

    syncActors(forceReset = false) {
        const now = Date.now();
        const activeKeys = new Set();

        for (const team of this.currentState?.teams || []) {
            const anchorGroup = this.teamAnchors.get(team.name);
            if (!anchorGroup) {
                continue;
            }

            const members = Array.isArray(team.members) ? team.members : [];
            members.forEach((agent, index) => {
                const anchor = anchorGroup.members[index];
                if (!anchor) {
                    return;
                }

                const key = anchor.key;
                activeKeys.add(key);
                const actor = this.ensureActorState(key, anchor, team, agent, forceReset);
                this.updateActorSignals(actor, team, agent, now, forceReset);
            });
        }

        Array.from(this.actorStates.keys()).forEach((key) => {
            if (!activeKeys.has(key)) {
                this.actorStates.delete(key);
                this.previousAgentSnapshots.delete(key);
            }
        });
    }

    ensureActorState(key, anchor, team, agent, forceReset) {
        let actor = this.actorStates.get(key);
        if (!actor) {
            actor = {
                key,
                x: anchor.homeX,
                y: anchor.homeY,
                homeX: anchor.homeX,
                homeY: anchor.homeY,
                deskX: anchor.deskX,
                deskY: anchor.deskY,
                state: String(agent.status || 'idle').toLowerCase(),
                behavior: 'desk',
                path: [],
                pathIndex: 0,
                waitUntil: 0,
                pauseType: '',
                facilityType: '',
                bubbleOverride: '',
                noTaskSince: 0,
                cooldownUntil: 0,
                lastBossSignal: '',
                isLead: anchor.isLead,
                velocityX: 0,
                velocityY: 0,
                facingX: anchor.isLead ? 0 : 1,
                facingY: 1,
                displayFacingX: anchor.isLead ? 0 : 1,
                displayFacingY: 1,
                bodyLean: 0,
                walkPhase: 0,
                interactionUntil: 0,
                interactionLabel: '',
                interactionKind: '',
                assignedSlot: null,
                leaderEscort: null,
                completionPulseUntil: 0,
                completionLabel: ''
            };
            this.actorStates.set(key, actor);
        }

        actor.teamName = team.name;
        actor.agentName = agent.name;
        actor.homeX = anchor.homeX;
        actor.homeY = anchor.homeY;
        actor.deskX = anchor.deskX;
        actor.deskY = anchor.deskY;
        actor.anchor = anchor;
        actor.team = team;
        actor.agent = agent;
        actor.state = String(agent.status || 'idle').toLowerCase();
        actor.isLead = anchor.isLead;
        actor.snapshot = this.buildAgentSnapshot(team, agent);
        actor.isBlocked = Boolean(actor.snapshot?.blocked);

        if (forceReset) {
            actor.x = anchor.homeX;
            actor.y = anchor.homeY;
            actor.path = [];
            actor.pathIndex = 0;
            actor.waitUntil = 0;
            actor.behavior = 'desk';
            actor.pauseType = '';
            actor.facilityType = '';
            actor.bubbleOverride = '';
            actor.velocityX = 0;
            actor.velocityY = 0;
            actor.facingX = anchor.isLead ? 0 : 1;
            actor.facingY = 1;
            actor.displayFacingX = actor.facingX;
            actor.displayFacingY = actor.facingY;
            actor.bodyLean = 0;
            actor.walkPhase = 0;
            actor.interactionUntil = 0;
            actor.interactionLabel = '';
            actor.interactionKind = '';
            actor.assignedSlot = null;
            actor.leaderEscort = null;
            actor.completionPulseUntil = 0;
            actor.completionLabel = '';
        }

        return actor;
    }

    updateActorSignals(actor, team, agent, now, forceReset) {
        const hasTask = this.agentHasTask(agent, team.tasks);
        if (hasTask) {
            actor.noTaskSince = 0;
        } else if (!actor.noTaskSince) {
            actor.noTaskSince = now;
        }

        const snapshot = this.buildAgentSnapshot(team, agent);
        const previous = this.previousAgentSnapshots.get(actor.key);
        this.previousAgentSnapshots.set(actor.key, snapshot);
        actor.snapshot = snapshot;
        actor.isBlocked = Boolean(snapshot.blocked);

        if (forceReset || !previous) {
            actor.lastBossSignal = snapshot.bossSignal;
            return;
        }

        const canInterrupt = actor.behavior !== 'pause-boss' && actor.behavior !== 'going-boss' && actor.behavior !== 'returning-boss';
        if (canInterrupt && this.shouldTriggerBossVisit(actor, snapshot, previous, now)) {
            this.emitTaskInteraction(team, actor, snapshot, previous, now);
            this.startBossVisit(actor, now, snapshot, previous);
            actor.lastBossSignal = snapshot.bossSignal;
            return;
        }

        if (actor.behavior === 'desk' && this.shouldTriggerLeisureWalk(actor, snapshot, now)) {
            this.startLeisureWalk(actor, now);
        }
    }

    buildAgentSnapshot(team, agent) {
        const latestMessage = String(agent.message_summary || agent.latest_message || '').trim();
        const currentTask = String(agent.current_task || '').trim();
        const lastActivity = this.getAgentActivityTimestamp(agent);
        const leadName = this.resolveLeaderName(team, agent);
        const toolName = String(agent.last_tool_use || '').trim();
        const toolDetail = String(agent.last_tool_detail || '').trim();
        const recentEvents = Array.isArray(agent.recent_events) ? agent.recent_events : [];
        const status = String(agent.status || 'idle').toLowerCase();
        return {
            teamName: team.name,
            agentName: agent.name,
            status,
            currentTask,
            latestMessage,
            lastActivity,
            leadName,
            toolName,
            toolDetail,
            lastSignalType: this.inferRecentSignalType(agent, recentEvents),
            blocked: this.isAgentBlocked(agent, team.tasks, lastActivity),
            bossSignal: `${currentTask}::${latestMessage}::${toolName}::${toolDetail}::${lastActivity}`
        };
    }

    inferRecentSignalType(agent, recentEvents) {
        const firstEvent = recentEvents.find((item) => String(item?.kind || '').trim() !== '');
        if (firstEvent?.kind) {
            return String(firstEvent.kind).toLowerCase();
        }
        if (agent.last_tool_use) {
            return 'tool';
        }
        if (agent.last_thinking) {
            return 'thinking';
        }
        if (agent.message_summary || agent.latest_message) {
            return 'message';
        }
        if (agent.current_task) {
            return 'task';
        }
        return 'idle';
    }

    isAgentBlocked(agent, teamTasks, lastActivity) {
        const status = String(agent?.status || '').toLowerCase();
        const now = Date.now();
        const hasTask = this.agentHasTask(agent, teamTasks);

        if ((status === 'working' || status === 'busy') && lastActivity && now - lastActivity > STALE_SIGNAL_THRESHOLD_MS) {
            return true;
        }

        if ((status === 'working' || status === 'busy') && hasTask && lastActivity && now - lastActivity > LONG_RUNNING_THRESHOLD_MS) {
            return true;
        }

        return false;
    }

    shouldTriggerBossVisit(actor, snapshot, previous, now) {
        if (snapshot.status !== 'working' && snapshot.status !== 'busy') {
            return false;
        }

        if (!previous) {
            return false;
        }

        if (now - snapshot.lastActivity > 90 * 1000) {
            return false;
        }

        const taskChanged = snapshot.currentTask && snapshot.currentTask !== previous.currentTask;
        const messageChanged = snapshot.latestMessage && snapshot.latestMessage !== previous.latestMessage;
        const recentSignalChanged = snapshot.bossSignal !== actor.lastBossSignal;

        return recentSignalChanged && (taskChanged || messageChanged);
    }

    resolveLeaderName(team, currentAgent) {
        const members = Array.isArray(team?.members) ? team.members : [];
        if (members.length === 0) {
            return '';
        }

        const explicitLead = members.find((member, index) => this.isLeadAgent(member, index));
        if (!explicitLead) {
            return '';
        }

        const leadName = String(explicitLead.name || '');
        const agentName = String(currentAgent?.name || '');
        if (leadName === agentName) {
            return '';
        }
        return leadName;
    }

    emitTaskInteraction(team, actor, snapshot, previous, now) {
        const targetName = snapshot.agentName;
        const leaderName = snapshot.leadName || this.resolveLeaderName(team, actor.agent);
        if (!leaderName || !targetName || leaderName === targetName) {
            return;
        }

        const leaderActor = this.actorStates.get(this.getAgentKey(team.name, leaderName));
        if (!leaderActor) {
            return;
        }

        const taskChanged = snapshot.currentTask && snapshot.currentTask !== previous.currentTask;
        const messageChanged = snapshot.latestMessage && snapshot.latestMessage !== previous.latestMessage;
        const kind = taskChanged ? 'assignment' : 'sync';
        const label = taskChanged ? '任务分配' : 'Leader 沟通';
        const detail = taskChanged
            ? snapshot.currentTask
            : snapshot.latestMessage || snapshot.currentTask || '同步进展';

        actor.interactionUntil = now + INTERACTION_DURATION_MS;
        actor.interactionLabel = label;
        actor.interactionKind = kind;

        leaderActor.interactionUntil = now + INTERACTION_DURATION_MS;
        leaderActor.interactionLabel = label;
        leaderActor.interactionKind = kind;

        this.assignmentBursts.push({
            id: `${team.name}:${targetName}:${now}`,
            kind,
            label,
            detail,
            fromKey: leaderActor.key,
            toKey: actor.key,
            createdAt: now,
            expiresAt: now + INTERACTION_DURATION_MS,
            completed: false
        });

        this.startLeaderEscort(leaderActor, actor, now, kind, label);
    }

    startLeaderEscort(leaderActor, targetActor, now, kind, label) {
        const destination = this.resolveFacilityVisitPoint('boss', `${leaderActor.key}:escort`);
        if (!destination) {
            return;
        }

        const escortPoint = {
            ...destination,
            x: destination.x + (targetActor.x >= leaderActor.x ? -18 : 18),
            y: destination.y - 10,
            queueX: destination.queueX + (targetActor.x >= leaderActor.x ? -16 : 16),
            queueY: destination.queueY
        };

        leaderActor.leaderEscort = {
            targetKey: targetActor.key,
            startedAt: now,
            label,
            kind
        };
        leaderActor.interactionUntil = now + INTERACTION_DURATION_MS;
        leaderActor.interactionLabel = kind === 'assignment' ? '带去交接' : '当面同步';
        leaderActor.interactionKind = kind;
        leaderActor.behavior = 'going-boss-lead';
        leaderActor.pauseType = 'boss-lead';
        leaderActor.facilityType = 'boss';
        leaderActor.bubbleOverride = this.pickEmoji(FACILITY_EMOJI.boss, `${leaderActor.key}:boss-lead:${Math.floor(now / 1600)}`);
        leaderActor.path = this.buildPath(leaderActor, escortPoint, 'boss');
        leaderActor.pathIndex = 0;
        leaderActor.waitUntil = 0;
    }

    shouldTriggerLeisureWalk(actor, snapshot, now) {
        if (snapshot.status === 'working' || snapshot.status === 'busy') {
            return false;
        }
        if (!actor.noTaskSince || now - actor.noTaskSince < NO_TASK_WANDER_THRESHOLD_MS) {
            return false;
        }
        if (actor.cooldownUntil && now < actor.cooldownUntil) {
            return false;
        }
        return true;
    }

    startLeisureWalk(actor, now) {
        const preferredType = this.resolveSemanticFacility(actor);
        const facilities = preferredType ? [preferredType] : ['restroom', 'gym', 'cafe'];
        const type = facilities[this.hashString(`${actor.key}:${Math.floor(now / 3000)}`) % facilities.length];
        const destination = this.resolveFacilityVisitPoint(type, actor.key);
        if (!destination) {
            actor.cooldownUntil = now + this.randomRange(actor.key, LEISURE_COOLDOWN_MIN_MS, LEISURE_COOLDOWN_MAX_MS);
            return;
        }

        actor.behavior = 'going-leisure';
        actor.pauseType = 'leisure';
        actor.facilityType = type;
        actor.bubbleOverride = this.pickEmoji(FACILITY_EMOJI[type], `${actor.key}:${type}:${Math.floor(now / 2000)}`);
        actor.path = this.buildPath(actor, destination, type);
        actor.pathIndex = 0;
        actor.waitUntil = 0;
    }

    resolveSemanticFacility(actor) {
        const snapshot = actor?.snapshot;
        const toolName = String(snapshot?.toolName || '').toLowerCase();
        const signalType = String(snapshot?.lastSignalType || '').toLowerCase();

        if (toolName.includes('bash') || toolName.includes('terminal') || toolName.includes('test')) {
            return 'gym';
        }
        if (toolName.includes('read') || toolName.includes('grep') || toolName.includes('search') || toolName.includes('web') || toolName.includes('browser')) {
            return 'cafe';
        }
        if (signalType === 'thinking' || signalType === 'message') {
            return 'cafe';
        }

        return null;
    }

    startBossVisit(actor, now, snapshot = null, previous = null) {
        const destination = this.resolveFacilityVisitPoint('boss', actor.key);
        if (!destination) {
            return;
        }

        actor.behavior = 'going-boss';
        actor.pauseType = 'boss';
        actor.facilityType = 'boss';
        actor.bubbleOverride = this.pickEmoji(FACILITY_EMOJI.boss, `${actor.key}:boss:${Math.floor(now / 2000)}`);
        const label = snapshot?.currentTask && snapshot.currentTask !== previous?.currentTask
            ? '新任务'
            : '接收任务';
        actor.interactionUntil = now + INTERACTION_DURATION_MS;
        actor.interactionLabel = label;
        actor.interactionKind = 'assignment';
        actor.path = this.buildPath(actor, destination, 'boss');
        actor.pathIndex = 0;
        actor.waitUntil = 0;
    }

    resolveFacilityVisitPoint(type, key) {
        const facilities = this.facilityByType.get(type) || [];
        if (!facilities.length) {
            return null;
        }

        const facility = facilities[this.hashString(`${key}:${type}`) % facilities.length];
        const slot = this.resolveFacilitySlot(facility, type, key);
        return {
            x: slot.x,
            y: slot.y,
            queueX: slot.queueX,
            queueY: slot.queueY,
            slotIndex: slot.slotIndex,
            facility
        };
    }

    buildPath(actor, destination, travelKind) {
        const corridorY = travelKind === 'boss'
            ? Math.max(96, Math.min(this.bounds.height * 0.32, this.bounds.height - 120))
            : Math.max(120, Math.min(this.bounds.height * 0.58, this.bounds.height - 120));
        const start = { x: actor.x, y: actor.y };
        const queueX = Number.isFinite(destination.queueX) ? destination.queueX : destination.x;
        const queueY = Number.isFinite(destination.queueY) ? destination.queueY : destination.y;
        const approachY = this.clampNumber(corridorY + this.routeOffset(actor.key, `${travelKind}:approachY`, 26), 80, this.bounds.height - 80);
        const startLaneX = this.clampNumber(start.x + this.routeOffset(actor.key, `${travelKind}:startLane`, 24), 40, this.bounds.width - 40);
        const queueLaneX = this.clampNumber(queueX + this.routeOffset(actor.key, `${travelKind}:queueLane`, 20), 40, this.bounds.width - 40);
        const points = [];

        if (Math.abs(start.x - startLaneX) > 8 || Math.abs(start.y - approachY) > 8) {
            points.push({ x: startLaneX, y: approachY });
        }
        if (Math.abs(queueLaneX - startLaneX) > 8) {
            points.push({ x: queueLaneX, y: approachY });
        }
        if (Math.abs(queueY - approachY) > 8) {
            points.push({ x: queueX, y: queueY });
        }
        points.push({ x: destination.x, y: destination.y });
        return this.smoothPath(start, points);
    }

    updateActors(deltaMs, now) {
        this.assignmentBursts = this.assignmentBursts.filter((burst) => burst.expiresAt > now);
        this.actorStates.forEach((actor) => {
            this.updateActorMotion(actor, deltaMs, now);
        });
    }

    updateActorMotion(actor, deltaMs, now) {
        if (actor.path && actor.pathIndex < actor.path.length) {
            const waypoint = actor.path[actor.pathIndex];
            const speed = actor.behavior.includes('boss') ? 150 : actor.behavior.includes('leisure') ? 92 : 120;
            const dx = waypoint.x - actor.x;
            const dy = waypoint.y - actor.y;
            const distance = Math.hypot(dx, dy);
            const maxStep = speed * (deltaMs / 1000);

            if (distance <= maxStep || distance < ARRIVAL_DISTANCE_PX) {
                actor.velocityX = dx;
                actor.velocityY = dy;
                actor.x = waypoint.x;
                actor.y = waypoint.y;
                actor.pathIndex += 1;
                if (actor.pathIndex >= actor.path.length) {
                    actor.path = [];
                    actor.pathIndex = 0;
                    this.handleActorArrival(actor, now);
                }
                this.updateActorFacing(actor, deltaMs, actor.velocityX, actor.velocityY);
                return;
            }

            const ratio = maxStep / distance;
            actor.velocityX = dx * ratio;
            actor.velocityY = dy * ratio;
            actor.x += actor.velocityX;
            actor.y += actor.velocityY;
            actor.walkPhase += deltaMs / 100;
            this.updateActorFacing(actor, deltaMs, actor.velocityX, actor.velocityY);
            return;
        }

        actor.velocityX = 0;
        actor.velocityY = 0;
        this.updateActorFacing(actor, deltaMs, 0, 0);

        if (actor.waitUntil && now >= actor.waitUntil) {
            actor.waitUntil = 0;
            this.startReturnToDesk(actor);
            return;
        }

        if (actor.behavior === 'desk') {
            const dx = actor.homeX - actor.x;
            const dy = actor.homeY - actor.y;
            const distance = Math.hypot(dx, dy);
            if (distance > 0.8) {
                const ease = Math.min(1, deltaMs / 180);
                actor.x += dx * ease;
                actor.y += dy * ease;
                actor.velocityX = dx * ease;
                actor.velocityY = dy * ease;
                this.updateActorFacing(actor, deltaMs, actor.velocityX, actor.velocityY);
            } else {
                actor.x = actor.homeX;
                actor.y = actor.homeY;
                actor.bubbleOverride = '';
                actor.facilityType = '';
                actor.behavior = 'desk';
                actor.assignedSlot = null;
                actor.leaderEscort = null;
            }
        }
    }

    handleActorArrival(actor, now) {
        if (actor.pauseType === 'boss') {
            actor.behavior = 'pause-boss';
            actor.waitUntil = now + this.randomRange(actor.key, BOSS_VISIT_MIN_MS, BOSS_VISIT_MAX_MS);
            actor.bubbleOverride = this.pickEmoji(FACILITY_EMOJI.boss, `${actor.key}:boss-wait:${Math.floor(now / 1000)}`);
            return;
        }

        if (actor.pauseType === 'boss-lead') {
            actor.behavior = 'pause-boss-lead';
            actor.waitUntil = now + this.randomRange(`${actor.key}:lead`, BOSS_VISIT_MIN_MS, BOSS_VISIT_MAX_MS);
            actor.bubbleOverride = '🧑‍💼';
            return;
        }

        if (actor.pauseType === 'leisure') {
            actor.behavior = 'pause-leisure';
            actor.waitUntil = now + this.randomRange(actor.key, LEISURE_VISIT_MIN_MS, LEISURE_VISIT_MAX_MS);
            actor.bubbleOverride = this.pickEmoji(FACILITY_EMOJI[actor.facilityType] || STATE_EMOJI.idle, `${actor.key}:pause:${Math.floor(now / 1000)}`);
        }
    }

    startReturnToDesk(actor) {
        const returnFromBoss = actor.pauseType === 'boss' || actor.pauseType === 'boss-lead';
        actor.behavior = returnFromBoss
            ? (actor.pauseType === 'boss-lead' ? 'returning-boss-lead' : 'returning-boss')
            : 'returning-leisure';
        actor.path = this.buildPath(actor, { x: actor.homeX, y: actor.homeY }, returnFromBoss ? 'boss' : 'desk');
        actor.pathIndex = 0;
        if (returnFromBoss) {
            this.markInteractionCompleted(actor, Date.now());
        }
        actor.pauseType = '';

        if (actor.behavior === 'returning-leisure') {
            actor.cooldownUntil = Date.now() + this.randomRange(actor.key, LEISURE_COOLDOWN_MIN_MS, LEISURE_COOLDOWN_MAX_MS);
        }
    }

    markInteractionCompleted(actor, now) {
        if (!actor || !actor.key) {
            return;
        }

        actor.completionPulseUntil = now + COMPLETION_BURST_MS;
        actor.completionLabel = actor.leaderEscort ? '交接完成' : '已接收';

        this.assignmentBursts.forEach((burst) => {
            const involvesActor = burst.toKey === actor.key || burst.fromKey === actor.key;
            if (involvesActor && !burst.completed) {
                burst.completed = true;
                burst.expiresAt = Math.max(now + COMPLETION_BURST_MS, now + 800);
                burst.label = '交接完成';
                burst.kind = 'sync';

                const counterpartKey = burst.toKey === actor.key ? burst.fromKey : burst.toKey;
                const counterpart = this.actorStates.get(counterpartKey);
                if (counterpart) {
                    counterpart.completionPulseUntil = now + COMPLETION_BURST_MS;
                    counterpart.completionLabel = '交接完成';
                }
            }
        });

        if (actor.leaderEscort) {
            actor.leaderEscort = null;
        }
    }

    updateActorFacing(actor, deltaMs, velocityX, velocityY) {
        const magnitude = Math.hypot(velocityX, velocityY);
        const desiredX = magnitude > 0.4 ? velocityX / magnitude : (actor.behavior === 'desk' ? 0 : actor.facingX);
        const desiredY = magnitude > 0.4 ? velocityY / magnitude : (actor.behavior === 'desk' ? 1 : actor.facingY);
        actor.facingX = desiredX;
        actor.facingY = desiredY;

        const response = Math.min(1, deltaMs / TURN_RESPONSE_MS);
        actor.displayFacingX += (desiredX - actor.displayFacingX) * response;
        actor.displayFacingY += (desiredY - actor.displayFacingY) * response;

        const displayMagnitude = Math.hypot(actor.displayFacingX, actor.displayFacingY);
        if (displayMagnitude > 0.001) {
            actor.displayFacingX /= displayMagnitude;
            actor.displayFacingY /= displayMagnitude;
        }

        const leanTarget = magnitude > 0.4 ? Math.max(-1, Math.min(1, velocityX / 24)) : 0;
        actor.bodyLean += (leanTarget - actor.bodyLean) * Math.min(1, deltaMs / 220);
    }

    smoothPath(start, points) {
        const result = [];
        let previous = { x: start.x, y: start.y };

        points.forEach((point, index) => {
            const current = { x: point.x, y: point.y };
            if (Math.hypot(current.x - previous.x, current.y - previous.y) < 6) {
                previous = current;
                return;
            }

            if (index > 0) {
                const blended = {
                    x: previous.x + (current.x - previous.x) * 0.45,
                    y: previous.y + (current.y - previous.y) * 0.45
                };
                if (Math.hypot(blended.x - previous.x, blended.y - previous.y) > 5) {
                    result.push(blended);
                }
            }

            result.push(current);
            previous = current;
        });

        return result;
    }

    resolveFacilitySlot(facility, type, key) {
        const slotCount = type === 'boss' ? 4 : 5;
        const slotIndex = this.hashString(`${key}:${type}:slot`) % slotCount;
        const baseX = facility.x + facility.width / 2;
        const baseY = facility.y + facility.height / 2;
        const horizontalSpread = Math.max(22, facility.width * 0.18);
        const verticalSpread = Math.max(18, facility.height * 0.16);

        let x = baseX;
        let y = baseY;
        let queueX = baseX;
        let queueY = baseY;

        if (type === 'boss') {
            x = baseX - facility.width * 0.24 + slotIndex * (horizontalSpread * 0.55);
            y = facility.y + facility.height + 18;
            queueX = x;
            queueY = y + 22 + slotIndex * 6;
        } else if (type === 'cafe') {
            x = facility.x + facility.width * 0.22 + (slotIndex % 2) * horizontalSpread;
            y = facility.y + facility.height * 0.72 + Math.floor(slotIndex / 2) * 12;
            queueX = facility.x - 24 - slotIndex * 6;
            queueY = facility.y + facility.height * 0.5;
        } else if (type === 'gym') {
            x = facility.x + facility.width * 0.28 + (slotIndex % 2) * horizontalSpread;
            y = facility.y + facility.height * 0.35 + Math.floor(slotIndex / 2) * verticalSpread;
            queueX = facility.x + facility.width + 24 + slotIndex * 5;
            queueY = facility.y + facility.height * 0.55;
        } else {
            x = facility.x + facility.width * 0.32 + (slotIndex % 2) * horizontalSpread;
            y = facility.y + facility.height * 0.35 + Math.floor(slotIndex / 2) * verticalSpread;
            queueX = facility.x + facility.width + 20 + slotIndex * 5;
            queueY = facility.y + facility.height * 0.45;
        }

        return { x, y, queueX, queueY, slotIndex };
    }

    routeOffset(key, salt, spread) {
        const half = Math.floor(spread / 2);
        return (this.hashString(`${key}:${salt}`) % (spread + 1)) - half;
    }

    clampNumber(value, min, max) {
        return Math.max(min, Math.min(max, value));
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

    pickActorAt(clientX, clientY) {
        const rect = this.canvas.getBoundingClientRect();
        const worldX = (clientX - rect.left - this.offsetX) / this.zoom;
        const worldY = (clientY - rect.top - this.offsetY) / this.zoom;

        let winner = null;
        let bestDistance = Infinity;

        this.actorStates.forEach((actor) => {
            const avatar = this.getAvatarConfig(actor.teamName, actor.agent, actor.anchor?.index || 0);
            const radius = (avatar.size || 18) + 12;
            const distance = Math.hypot(worldX - actor.x, worldY - actor.y);
            if (distance <= radius && distance < bestDistance) {
                winner = actor;
                bestDistance = distance;
            }
        });

        return winner;
    }

    selectActor(actor) {
        if (!actor) {
            return;
        }

        this.selectedActorKey = actor.key;
        actor.focusUntil = Date.now() + FOCUS_RING_DURATION_MS;
        this.focusCameraOn(actor.x, actor.y);
        this.userMovedCamera = true;

        if (typeof this.onActorSelected === 'function') {
            this.onActorSelected({
                teamName: actor.teamName,
                agentName: actor.agentName
            });
        }
    }

    focusCameraOn(worldX, worldY) {
        this.offsetX = this.viewportWidth / 2 - worldX * this.zoom;
        this.offsetY = this.viewportHeight / 2 - worldY * this.zoom;
        this.clampWorldPosition();
    }

    handleResize() {
        this.resizeCanvas();
        this.recalculateLayout(!this.userMovedCamera);
        this.syncActors(false);
    }

    renderLoop(timestamp) {
        if (!this.lastFrameTime) {
            this.lastFrameTime = timestamp;
        }
        const deltaMs = Math.min(MAX_FRAME_DELTA_MS, Math.max(0, timestamp - this.lastFrameTime));
        this.lastFrameTime = timestamp;

        this.updateActors(deltaMs, Date.now());
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
        this.drawTeams(ctx, timestamp, this.currentState.teams || []);
        this.drawInteractions(ctx, timestamp);

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

             const slotCount = facility.type === 'boss' ? 4 : 5;
            for (let slotIndex = 0; slotIndex < slotCount; slotIndex += 1) {
                const slot = this.resolveFacilitySlot(facility, facility.type, `${facility.type}:${slotIndex}`);
                ctx.beginPath();
                ctx.arc(slot.x, slot.y, SLOT_MARKER_RADIUS, 0, Math.PI * 2);
                ctx.fillStyle = facility.type === 'boss' ? 'rgba(251, 191, 36, 0.3)' : 'rgba(148, 163, 184, 0.28)';
                ctx.fill();
                ctx.beginPath();
                ctx.arc(slot.queueX, slot.queueY, SLOT_MARKER_RADIUS - 1, 0, Math.PI * 2);
                ctx.fillStyle = 'rgba(148, 163, 184, 0.18)';
                ctx.fill();
            }
            ctx.restore();
        });
    }

    drawTeams(ctx, timestamp, teams) {
        teams.forEach((team) => this.drawTeam(ctx, timestamp, team));
    }

    drawTeam(ctx, timestamp, team) {
        const anchorGroup = this.teamAnchors.get(team.name);
        if (!anchorGroup) {
            return;
        }

        const members = Array.isArray(team.members) ? team.members : [];
        const { layout, deskWidth, deskHeight, rotation } = anchorGroup;

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

        members.forEach((agent) => {
            const actor = this.actorStates.get(this.getAgentKey(team.name, agent.name));
            if (actor) {
                this.drawAgent(ctx, timestamp, actor);
            }
        });
    }

    drawInteractions(ctx, timestamp) {
        this.assignmentBursts.forEach((burst) => {
            const fromActor = this.actorStates.get(burst.fromKey);
            const toActor = this.actorStates.get(burst.toKey);
            if (!fromActor || !toActor) {
                return;
            }

            const alpha = Math.max(0, Math.min(1, (burst.expiresAt - Date.now()) / INTERACTION_DURATION_MS));
            const progress = 1 - alpha;
            const style = INTERACTION_STYLE[burst.kind] || INTERACTION_STYLE.sync;
            const startX = fromActor.x;
            const startY = fromActor.y - 8;
            const endX = toActor.x;
            const endY = toActor.y - 8;
            const midX = (startX + endX) / 2;
            const midY = Math.min(startY, endY) - 46 - progress * 12;

            ctx.save();
            ctx.strokeStyle = style.stroke.replace('0.9', `${Math.max(0.2, alpha)}`);
            ctx.lineWidth = 2.5;
            ctx.setLineDash([10, 8]);
            ctx.lineDashOffset = -(timestamp / 36);
            ctx.beginPath();
            ctx.moveTo(startX, startY);
            ctx.quadraticCurveTo(midX, midY, endX, endY);
            ctx.stroke();
            ctx.setLineDash([]);

            ctx.strokeStyle = style.glow.replace('0.3', `${Math.max(0.1, alpha * 0.28)}`);
            ctx.lineWidth = 8;
            ctx.beginPath();
            ctx.moveTo(startX, startY);
            ctx.quadraticCurveTo(midX, midY, endX, endY);
            ctx.stroke();

            const badgeX = midX;
            const badgeY = midY - 14;
            this.roundRect(ctx, badgeX - 46, badgeY - 15, 92, 30, 14);
            ctx.fillStyle = style.fill.replace('0.96', `${Math.max(0.5, alpha)}`);
            ctx.fill();
            ctx.strokeStyle = style.stroke.replace('0.9', `${Math.max(0.35, alpha)}`);
            ctx.lineWidth = 1.5;
            ctx.stroke();
            ctx.fillStyle = '#1f2937';
            ctx.font = '600 12px -apple-system, BlinkMacSystemFont, "Segoe UI", Arial, sans-serif';
            ctx.textAlign = 'center';
            ctx.fillText(burst.label, badgeX, badgeY + 4);
            ctx.restore();
        });
    }

    drawAgent(ctx, timestamp, actor) {
        const state = actor.state;
        const avatar = this.getAvatarConfig(actor.teamName, actor.agent, actor.anchor?.index || 0);
        const moving = Math.hypot(actor.velocityX, actor.velocityY) > 0.1;
        const phase = this.hashString(`${actor.teamName}:${actor.agentName}`) % 360;
        const pulse = Math.sin(timestamp / 520 + phase);
        const stride = moving ? Math.sin(actor.walkPhase + phase) * 2.8 : 0;
        const amplitude = state === 'working' || state === 'busy' ? 0.08 : moving ? 0.045 : 0.03;
        const scale = 1 + pulse * amplitude;
        const radius = avatar.size * scale;
        const fill = state === 'working' || state === 'busy' ? WORKING_COLOR.fill : avatar.fill;
        const stroke = state === 'working' || state === 'busy' ? WORKING_COLOR.stroke : avatar.stroke;
        const facingX = Number.isFinite(actor.displayFacingX) ? actor.displayFacingX : 0;
        const facingY = Number.isFinite(actor.displayFacingY) ? actor.displayFacingY : 1;
        const shoulderX = -facingY;
        const shoulderY = facingX;
        const noseX = facingX * radius * 0.58;
        const noseY = facingY * radius * 0.58;
        const leanOffsetX = actor.bodyLean * 2.8;

        ctx.save();
        ctx.translate(actor.x, actor.y + stride);
        ctx.translate(leanOffsetX, 0);

        this.drawFocusRing(ctx, actor, radius, timestamp);
        this.drawCompletionPulse(ctx, actor, radius, timestamp);

        if (moving) {
            const legSwing = Math.sin(actor.walkPhase + phase);
            ctx.strokeStyle = 'rgba(31, 41, 55, 0.7)';
            ctx.lineWidth = 2.2;
            ctx.lineCap = 'round';
            ctx.beginPath();
            ctx.moveTo(-shoulderX * 4, radius * 0.65);
            ctx.lineTo(-shoulderX * 8 + facingX * legSwing * 2, radius * 0.65 + 10);
            ctx.moveTo(shoulderX * 4, radius * 0.65);
            ctx.lineTo(shoulderX * 8 - facingX * legSwing * 2, radius * 0.65 + 10);
            ctx.stroke();
        }

        ctx.beginPath();
        ctx.arc(0, 0, radius, 0, Math.PI * 2);
        ctx.fillStyle = fill;
        ctx.fill();
        ctx.lineWidth = avatar.strokeWidth;
        ctx.strokeStyle = actor.isBlocked ? '#dc2626' : stroke;
        ctx.stroke();

        if (actor.isBlocked) {
            ctx.beginPath();
            ctx.arc(0, 0, radius + 6, 0, Math.PI * 2);
            ctx.strokeStyle = 'rgba(220, 38, 38, 0.42)';
            ctx.lineWidth = 3;
            ctx.stroke();
        }

        const eyeOffset = radius * 0.26;
        const eyeSize = Math.max(2, radius * 0.15);
        ctx.beginPath();
        ctx.fillStyle = '#000000';
        ctx.arc(-shoulderX * eyeOffset + facingX * 3, -shoulderY * eyeOffset + facingY * 2 - 3, eyeSize, 0, Math.PI * 2);
        ctx.arc(shoulderX * eyeOffset + facingX * 3, shoulderY * eyeOffset + facingY * 2 - 3, eyeSize, 0, Math.PI * 2);
        ctx.fill();

        ctx.beginPath();
        ctx.strokeStyle = '#000000';
        ctx.lineWidth = 1.5;
        ctx.moveTo(-shoulderX * radius * 0.24 + facingX * 2, shoulderY * radius * 0.24 + facingY * 4);
        ctx.quadraticCurveTo(facingX * 4, facingY * 6 + 4, shoulderX * radius * 0.24 + facingX * 2, -shoulderY * radius * 0.24 + facingY * 4);
        ctx.stroke();

        ctx.beginPath();
        ctx.arc(noseX, noseY, Math.max(1.8, radius * 0.11), 0, Math.PI * 2);
        ctx.fillStyle = 'rgba(255,255,255,0.85)';
        ctx.fill();

        this.drawBubble(ctx, timestamp, actor, radius);
        this.drawInteractionBadge(ctx, actor, radius, timestamp);

        ctx.fillStyle = avatar.labelColor;
        ctx.font = `${avatar.isLead ? '700' : '400'} ${avatar.isLead ? 14 : 13}px -apple-system, BlinkMacSystemFont, "Segoe UI", Arial, sans-serif`;
        ctx.textAlign = 'center';
        ctx.fillText(actor.agentName, 0, avatar.size + 28);
        ctx.restore();
    }

    drawFocusRing(ctx, actor, radius, timestamp) {
        const selected = this.selectedActorKey && actor.key === this.selectedActorKey;
        const active = selected || (actor.focusUntil && actor.focusUntil > Date.now());
        if (!active) {
            return;
        }

        const alpha = selected
            ? 0.95
            : Math.max(0, Math.min(1, (actor.focusUntil - Date.now()) / FOCUS_RING_DURATION_MS));
        const pulse = 1 + Math.sin(timestamp / 160 + this.hashString(actor.key)) * 0.08;

        ctx.save();
        ctx.beginPath();
        ctx.arc(0, 0, radius * 1.42 * pulse, 0, Math.PI * 2);
        ctx.strokeStyle = `rgba(8, 145, 178, ${0.55 * alpha})`;
        ctx.lineWidth = 3;
        ctx.stroke();
        ctx.beginPath();
        ctx.arc(0, 0, radius * 1.62 * pulse, 0, Math.PI * 2);
        ctx.strokeStyle = `rgba(8, 145, 178, ${0.18 * alpha})`;
        ctx.lineWidth = 5;
        ctx.stroke();
        ctx.restore();
    }

    drawCompletionPulse(ctx, actor, radius, timestamp) {
        if (!actor.completionPulseUntil || actor.completionPulseUntil <= Date.now()) {
            return;
        }

        const alpha = Math.max(0, Math.min(1, (actor.completionPulseUntil - Date.now()) / COMPLETION_BURST_MS));
        const pulse = 1 + (1 - alpha) * 0.32 + Math.sin(timestamp / 120 + this.hashString(actor.key)) * 0.03;
        ctx.save();
        ctx.beginPath();
        ctx.arc(0, 0, radius * pulse * 1.18, 0, Math.PI * 2);
        ctx.fillStyle = `rgba(16, 185, 129, ${0.14 * alpha})`;
        ctx.fill();
        ctx.beginPath();
        ctx.arc(0, 0, radius * pulse * 1.36, 0, Math.PI * 2);
        ctx.strokeStyle = `rgba(16, 185, 129, ${0.32 * alpha})`;
        ctx.lineWidth = 2;
        ctx.stroke();
        ctx.restore();
    }

    drawInteractionBadge(ctx, actor, radius, timestamp) {
        const now = Date.now();
        const completionActive = actor.completionPulseUntil && actor.completionPulseUntil > now;
        if ((!actor.interactionUntil || actor.interactionUntil <= now || !actor.interactionLabel) && !completionActive) {
            return;
        }

        const alpha = completionActive
            ? Math.max(0, Math.min(1, (actor.completionPulseUntil - now) / COMPLETION_BURST_MS))
            : Math.max(0, Math.min(1, (actor.interactionUntil - now) / INTERACTION_DURATION_MS));
        const floatY = Math.sin(timestamp / 220 + this.hashString(actor.key)) * 2;
        const style = completionActive
            ? INTERACTION_STYLE.sync
            : (INTERACTION_STYLE[actor.interactionKind] || INTERACTION_STYLE.sync);
        const text = completionActive ? (actor.completionLabel || '交接完成') : actor.interactionLabel;
        const width = Math.max(64, text.length * 14 + 16);
        const y = -radius - 38 + floatY;

        this.roundRect(ctx, -width / 2, y - 12, width, 24, 12);
        ctx.fillStyle = style.fill.replace('0.96', `${Math.max(0.5, alpha)}`);
        ctx.fill();
        ctx.strokeStyle = style.stroke.replace('0.9', `${Math.max(0.35, alpha)}`);
        ctx.lineWidth = 1.2;
        ctx.stroke();
        ctx.fillStyle = '#1f2937';
        ctx.font = '600 11px -apple-system, BlinkMacSystemFont, "Segoe UI", Arial, sans-serif';
        ctx.textAlign = 'center';
        ctx.fillText(text, 0, y + 5);
    }

    drawBubble(ctx, timestamp, actor) {
        const emoji = actor.bubbleOverride || this.resolveActorEmoji(actor, timestamp);
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
        ctx.fillStyle = actor.isBlocked ? '#b91c1c' : '#222222';
        ctx.fillText(emoji, 0, -34);

        if (actor.isBlocked) {
            ctx.beginPath();
            ctx.arc(16, -53, 7.5, 0, Math.PI * 2);
            ctx.fillStyle = '#fee2e2';
            ctx.fill();
            ctx.strokeStyle = '#ef4444';
            ctx.lineWidth = 1;
            ctx.stroke();
            ctx.fillStyle = '#b91c1c';
            ctx.font = '700 10px -apple-system, BlinkMacSystemFont, "Segoe UI", Arial, sans-serif';
            ctx.fillText('!', 16, -49.5);
        }
    }

    resolveActorEmoji(actor, timestamp) {
        if (actor.isBlocked) {
            return '🚨';
        }
        if (actor.behavior.includes('boss') || actor.pauseType === 'boss') {
            return this.pickEmoji(FACILITY_EMOJI.boss, `${actor.key}:boss:${Math.floor(timestamp / 1200)}`);
        }
        if (actor.behavior.includes('leisure') || actor.pauseType === 'leisure') {
            return this.pickEmoji(FACILITY_EMOJI[actor.facilityType] || STATE_EMOJI.idle, `${actor.key}:leisure:${Math.floor(timestamp / 1500)}`);
        }

        const semanticEmoji = this.resolveSemanticEmoji(actor, timestamp);
        if (semanticEmoji) {
            return semanticEmoji;
        }

        const state = STATE_EMOJI[actor.state] || STATE_EMOJI.idle;
        return this.pickEmoji(state, `${actor.key}:${actor.state}:${Math.floor(timestamp / 3000)}`);
    }

    resolveSemanticEmoji(actor, timestamp) {
        const snapshot = actor?.snapshot;
        if (!snapshot) {
            return '';
        }

        const signalType = String(snapshot.lastSignalType || '').toLowerCase();
        const toolName = String(snapshot.toolName || '').toLowerCase();

        if (signalType === 'thinking') {
            return '💭';
        }
        if (signalType === 'task') {
            return '🎯';
        }
        if (signalType === 'message') {
            return '📨';
        }
        if (signalType === 'tool') {
            for (const [toolKey, emojis] of Object.entries(TOOL_EMOJI)) {
                if (toolName.includes(toolKey)) {
                    return this.pickEmoji(emojis, `${actor.key}:${toolName}:${Math.floor(timestamp / 1800)}`);
                }
            }
            return '🔧';
        }

        return '';
    }

    pickEmoji(list, seed) {
        if (!Array.isArray(list) || list.length === 0) {
            return '🙂';
        }
        return list[this.hashString(seed) % list.length];
    }

    randomRange(seed, min, max) {
        if (max <= min) {
            return min;
        }
        const span = max - min;
        return min + (this.hashString(`${seed}:${Date.now()}:${span}`) % span);
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
        const isLead = this.isLeadAgent(agent, index);
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

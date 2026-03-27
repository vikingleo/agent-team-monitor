export class DataSyncManager {
    constructor(scene) {
        this.scene = scene;
        this.pollInterval = 1000;
        this.timer = null;
        this.lastState = null;
        this.desktopBridge = window.AgentMonitorDesktopBridge || null;
        this.isDesktopMode = Boolean(this.desktopBridge && this.desktopBridge.isDesktopModeEnabled());
    }

    start() {
        if (this.isDesktopMode && this.desktopBridge) {
            this.timer = this.desktopBridge.startDesktopPolling(() => this.fetchAndUpdate(), this.pollInterval);
            return;
        }

        this.fetchAndUpdate();
        this.timer = setInterval(() => this.fetchAndUpdate(), this.pollInterval);
    }

    stop() {
        if (this.timer) {
            if (typeof this.timer === 'function') {
                this.timer();
            } else {
                clearInterval(this.timer);
            }
            this.timer = null;
        }
    }

    async fetchAndUpdate() {
        try {
            let rawState;
            if (this.isDesktopMode && this.desktopBridge) {
                rawState = await this.desktopBridge.fetchDesktopState();
            } else {
                const response = await fetch('/api/state?_ts=' + Date.now());
                if (!response.ok) throw new Error('API error');
                rawState = await response.json();
            }
            const newState = typeof this.scene.prepareState === 'function'
                ? this.scene.prepareState(rawState)
                : rawState;
            this.processChanges(newState);
            this.lastState = newState;
        } catch (error) {
            console.error('Failed to fetch state:', error);
        }
    }

    processChanges(newState) {
        if (!this.lastState) {
            this.scene.initializeState(newState);
            this.emitStateUpdate(newState);
            return;
        }

        const changes = this.diffStates(this.lastState, newState);
        this.scene.applyChanges(changes);
        this.emitStateUpdate(newState);
    }

    emitStateUpdate(state) {
        if (typeof this.scene.emitStateUpdate === 'function') {
            this.scene.emitStateUpdate(state);
        }
    }

    diffStates(oldState, newState) {
        const changes = {
            teamsAdded: [],
            teamsRemoved: [],
            agentsUpdated: []
        };

        const oldTeams = new Map((oldState.teams || []).map(t => [t.name, t]));
        const newTeams = new Map((newState.teams || []).map(t => [t.name, t]));

        // 检测新增团队
        for (const [name, team] of newTeams) {
            if (!oldTeams.has(name)) {
                changes.teamsAdded.push(team);
            }
        }

        // 检测删除团队
        for (const [name] of oldTeams) {
            if (!newTeams.has(name)) {
                changes.teamsRemoved.push(name);
            }
        }

        // 检测 agent 状态变化（使用 members 而不是 agents）
        for (const [teamName, team] of newTeams) {
            const oldTeam = oldTeams.get(teamName);
            if (!oldTeam) continue;

            const oldAgents = new Map((oldTeam.members || []).map(a => [a.name, a]));
            const newAgents = new Map((team.members || []).map(a => [a.name, a]));

            for (const [agentName, agent] of newAgents) {
                const oldAgent = oldAgents.get(agentName);
                if (!oldAgent || oldAgent.status !== agent.status) {
                    changes.agentsUpdated.push({ ...agent, teamName });
                }
            }
        }

        return changes;
    }
}

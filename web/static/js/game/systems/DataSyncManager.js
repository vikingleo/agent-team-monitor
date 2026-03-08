export class DataSyncManager {
    constructor(scene) {
        this.scene = scene;
        this.pollInterval = 1000;
        this.timer = null;
        this.lastState = null;
    }

    start() {
        this.fetchAndUpdate();
        this.timer = setInterval(() => this.fetchAndUpdate(), this.pollInterval);
    }

    stop() {
        if (this.timer) {
            clearInterval(this.timer);
            this.timer = null;
        }
    }

    async fetchAndUpdate() {
        try {
            const response = await fetch('/api/state?_ts=' + Date.now());
            if (!response.ok) throw new Error('API error');
            const newState = await response.json();
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
        // 通知 Game 实例状态已更新
        if (this.scene.game && this.scene.game.events) {
            this.scene.game.events.emit('state-updated', state);
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

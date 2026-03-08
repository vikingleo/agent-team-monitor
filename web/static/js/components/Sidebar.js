export class Sidebar {
    constructor() {
        this.currentTeam = null;
        this.teams = [];
        this.container = null;
        this.init();
    }

    init() {
        // 创建侧栏 DOM
        this.container = document.createElement('div');
        this.container.className = 'sidebar';
        this.container.innerHTML = `
            <div class="team-tabs" id="team-tabs"></div>
            <div class="sidebar-content" id="sidebar-content"></div>
        `;
        document.body.appendChild(this.container);

        this.tabsContainer = document.getElementById('team-tabs');
        this.contentContainer = document.getElementById('sidebar-content');
    }

    updateState(state) {
        if (!state || !state.teams) {
            this.showEmptyState();
            return;
        }

        this.teams = state.teams;

        // 如果没有选中团队，默认选中第一个
        if (!this.currentTeam && this.teams.length > 0) {
            this.currentTeam = this.teams[0].name;
        }

        this.renderTabs();
        this.renderContent();
    }

    renderTabs() {
        this.tabsContainer.innerHTML = '';

        this.teams.forEach(team => {
            const tab = document.createElement('div');
            tab.className = `team-tab ${team.name === this.currentTeam ? 'active' : ''}`;
            tab.innerHTML = `
                <div class="team-tab-name">${this.truncateName(team.name)}</div>
                <div class="team-tab-count">${team.members ? team.members.length : 0} 人</div>
            `;
            tab.addEventListener('click', () => {
                this.currentTeam = team.name;
                this.renderTabs();
                this.renderContent();
            });
            this.tabsContainer.appendChild(tab);
        });
    }

    renderContent() {
        const team = this.teams.find(t => t.name === this.currentTeam);
        if (!team) {
            this.showEmptyState();
            return;
        }

        this.contentContainer.innerHTML = `
            <div class="team-header">
                <div class="team-title">${team.name}</div>
                <div class="team-meta">${team.members ? team.members.length : 0} 个成员</div>
            </div>
            <div id="agents-list"></div>
        `;

        const agentsList = document.getElementById('agents-list');

        if (!team.members || team.members.length === 0) {
            agentsList.innerHTML = `
                <div class="empty-state">
                    <div class="empty-state-icon">👤</div>
                    <div class="empty-state-text">暂无成员</div>
                </div>
            `;
            return;
        }

        team.members.forEach(agent => {
            const card = this.createAgentCard(agent);
            agentsList.appendChild(card);
        });
    }

    createAgentCard(agent) {
        const card = document.createElement('div');
        const status = agent.status || 'idle';
        const isWorking = status === 'working' || status === 'busy';

        card.className = `agent-card ${isWorking ? 'working' : 'idle'}`;

        let html = `
            <div class="agent-header">
                <div class="agent-name">${agent.name}</div>
                <div class="agent-status ${isWorking ? 'working' : 'idle'}">
                    ${isWorking ? '工作中' : '空闲'}
                </div>
            </div>
        `;

        // 当前操作
        if (agent.last_tool_use) {
            html += `
                <div class="agent-action">
                    🔧 ${agent.last_tool_use}${agent.last_tool_detail ? ': ' + this.truncateText(agent.last_tool_detail, 30) : ''}
                </div>
            `;
        }

        // 思考内容
        if (agent.last_thinking) {
            html += `
                <div class="agent-thinking">
                    💭 ${this.truncateText(agent.last_thinking, 150)}
                </div>
            `;
        }

        // 任务清单
        if (agent.todos && agent.todos.length > 0) {
            html += `
                <div class="agent-todos">
                    <div class="agent-todos-title">📋 任务清单 (${agent.todos.length})</div>
            `;
            agent.todos.slice(0, 5).forEach(todo => {
                html += `
                    <div class="todo-item ${todo.status === 'completed' ? 'completed' : ''}">
                        <div class="todo-status ${todo.status}"></div>
                        <div>${this.truncateText(todo.content, 40)}</div>
                    </div>
                `;
            });
            if (agent.todos.length > 5) {
                html += `<div class="todo-item" style="color: #999;">...还有 ${agent.todos.length - 5} 项</div>`;
            }
            html += `</div>`;
        }

        card.innerHTML = html;
        return card;
    }

    showEmptyState() {
        this.tabsContainer.innerHTML = '';
        this.contentContainer.innerHTML = `
            <div class="empty-state">
                <div class="empty-state-icon">🏢</div>
                <div class="empty-state-text">暂无团队数据</div>
            </div>
        `;
    }

    truncateName(name) {
        if (name.length <= 8) return name;
        return name.substring(0, 7) + '...';
    }

    truncateText(text, maxLength) {
        if (!text) return '';
        if (text.length <= maxLength) return text;
        return text.substring(0, maxLength) + '...';
    }

    destroy() {
        if (this.container) {
            this.container.remove();
        }
    }
}

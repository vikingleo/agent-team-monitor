export class Sidebar {
    constructor() {
        this.currentTeam = null;
        this.teams = [];
        this.container = null;
        this.tabsContainer = null;
        this.contentContainer = null;
        this.previewContainer = null;
        this.previewTeamName = null;
        this.hidePreviewTimer = null;
        this.init();
    }

    init() {
        this.container = document.createElement('div');
        this.container.className = 'sidebar';
        this.container.innerHTML = `
            <div class="team-tabs" id="team-tabs"></div>
            <div class="sidebar-content" id="sidebar-content"></div>
            <div class="team-tab team-tab-preview" id="team-tab-preview"></div>
        `;
        document.body.appendChild(this.container);

        this.tabsContainer = this.container.querySelector('#team-tabs');
        this.contentContainer = this.container.querySelector('#sidebar-content');
        this.previewContainer = this.container.querySelector('#team-tab-preview');

        this.tabsContainer.addEventListener('scroll', () => this.hideTabPreview());
        this.previewContainer.addEventListener('mouseenter', () => this.cancelHideTabPreview());
        this.previewContainer.addEventListener('mouseleave', () => this.hideTabPreview());
        this.previewContainer.addEventListener('click', () => {
            if (!this.previewTeamName) {
                return;
            }
            this.currentTeam = this.previewTeamName;
            this.hideTabPreview();
            this.renderTabs();
            this.renderContent();
        });
    }

    updateState(state) {
        if (!state || !state.teams) {
            this.showEmptyState();
            return;
        }

        this.teams = state.teams;

        if (!this.currentTeam && this.teams.length > 0) {
            this.currentTeam = this.teams[0].name;
        }

        if (this.currentTeam && !this.teams.some(team => team.name === this.currentTeam)) {
            this.currentTeam = this.teams.length > 0 ? this.teams[0].name : null;
        }

        this.hideTabPreview();
        this.renderTabs();
        this.renderContent();
    }

    renderTabs() {
        this.tabsContainer.innerHTML = '';

        this.teams.forEach(team => {
            const tab = document.createElement('div');
            tab.className = `team-tab ${team.name === this.currentTeam ? 'active' : ''}`;
            tab.title = team.name;
            tab.innerHTML = this.renderTabMarkup(team);

            tab.addEventListener('mouseenter', () => this.showTabPreview(tab, team));
            tab.addEventListener('mouseleave', () => this.scheduleHideTabPreview());
            tab.addEventListener('click', () => {
                this.currentTeam = team.name;
                this.hideTabPreview();
                this.renderTabs();
                this.renderContent();
            });

            this.tabsContainer.appendChild(tab);
        });
    }

    renderTabMarkup(team) {
        return `
            <div class="team-tab-name">${team.name}</div>
            <div class="team-tab-count">${team.members ? team.members.length : 0} 人</div>
        `;
    }

    showTabPreview(tab, team) {
        this.cancelHideTabPreview();

        const tabRect = tab.getBoundingClientRect();
        const sidebarRect = this.container.getBoundingClientRect();

        this.previewTeamName = team.name;
        this.previewContainer.className = `team-tab team-tab-preview visible ${team.name === this.currentTeam ? 'active' : ''}`;
        this.previewContainer.innerHTML = this.renderTabMarkup(team);
        this.previewContainer.style.top = `${tabRect.top - sidebarRect.top}px`;
        this.previewContainer.style.minHeight = `${tabRect.height}px`;
    }

    scheduleHideTabPreview() {
        this.cancelHideTabPreview();
        this.hidePreviewTimer = window.setTimeout(() => this.hideTabPreview(), 80);
    }

    cancelHideTabPreview() {
        if (this.hidePreviewTimer) {
            clearTimeout(this.hidePreviewTimer);
            this.hidePreviewTimer = null;
        }
    }

    hideTabPreview() {
        this.cancelHideTabPreview();
        this.previewTeamName = null;
        if (this.previewContainer) {
            this.previewContainer.className = 'team-tab team-tab-preview';
            this.previewContainer.innerHTML = '';
            this.previewContainer.style.top = '';
            this.previewContainer.style.minHeight = '';
        }
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

        const lastActivityText = this.formatRelativeTime(agent.last_active_time || agent.last_message_time || agent.last_activity);

        let html = `
            <div class="agent-header">
                <div class="agent-name">${agent.name}</div>
                <div class="agent-status ${isWorking ? 'working' : 'idle'}">
                    ${isWorking ? '工作中' : '空闲'}
                </div>
            </div>
            ${lastActivityText ? `<div class="agent-last-active">最后活动 ${lastActivityText}</div>` : ''}
        `;

        if (agent.last_tool_use) {
            html += `
                <div class="agent-action">
                    🔧 ${agent.last_tool_use}${agent.last_tool_detail ? ': ' + this.truncateText(agent.last_tool_detail, 30) : ''}
                </div>
            `;
        }

        if (agent.last_thinking) {
            html += `
                <div class="agent-thinking">
                    💭 ${this.truncateText(agent.last_thinking, 150)}
                </div>
            `;
        }

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
        this.hideTabPreview();
        this.tabsContainer.innerHTML = '';
        this.contentContainer.innerHTML = `
            <div class="empty-state">
                <div class="empty-state-icon">🏢</div>
                <div class="empty-state-text">暂无团队数据</div>
            </div>
        `;
    }

    formatRelativeTime(timestamp) {
        if (!timestamp) {
            return '';
        }

        const target = new Date(timestamp);
        if (Number.isNaN(target.getTime())) {
            return '';
        }

        const diffSeconds = Math.max(0, Math.floor((Date.now() - target.getTime()) / 1000));
        if (diffSeconds < 60) {
            return `${diffSeconds}秒前`;
        }

        const diffMinutes = Math.floor(diffSeconds / 60);
        if (diffMinutes < 60) {
            return `${diffMinutes}分钟前`;
        }

        const diffHours = Math.floor(diffMinutes / 60);
        if (diffHours < 24) {
            return `${diffHours}小时前`;
        }

        return `${Math.floor(diffHours / 24)}天前`;
    }

    truncateText(text, maxLength) {
        if (!text) return '';
        if (text.length <= maxLength) return text;
        return text.substring(0, maxLength) + '...';
    }

    destroy() {
        this.hideTabPreview();
        if (this.container) {
            this.container.remove();
        }
    }
}

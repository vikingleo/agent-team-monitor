export class Sidebar {
    constructor() {
        this.currentTeam = null;
        this.currentAgent = null;
        this.teams = [];
        this.container = null;
        this.tabsContainer = null;
        this.contentContainer = null;
        this.previewContainer = null;
        this.previewTeamName = null;
        this.hidePreviewTimer = null;
        this.handleContentWheel = this.handleContentWheel.bind(this);
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
        this.contentContainer.addEventListener('wheel', this.handleContentWheel, { passive: false });
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
        if (!state || !Array.isArray(state.teams)) {
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

        if (this.currentAgent) {
            const activeTeam = this.teams.find(team => team.name === this.currentTeam);
            const members = Array.isArray(activeTeam?.members) ? activeTeam.members : [];
            if (!members.some(agent => agent.name === this.currentAgent)) {
                this.currentAgent = null;
            }
        }

        this.hideTabPreview();
        this.renderTabs();
        this.renderContent();
    }

    focusAgent(teamName, agentName) {
        if (teamName && this.teams.some(team => team.name === teamName)) {
            this.currentTeam = teamName;
        }

        const team = this.teams.find(item => item.name === this.currentTeam);
        const members = Array.isArray(team?.members) ? team.members : [];
        this.currentAgent = members.some(agent => agent.name === agentName) ? agentName : null;
        this.hideTabPreview();
        this.renderTabs();
        this.renderContent();
    }

    renderTabs() {
        this.tabsContainer.innerHTML = '';

        this.teams.forEach((team) => {
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
        const members = Array.isArray(team.members) ? team.members : [];
        const tasks = Array.isArray(team.tasks) ? team.tasks : [];
        const workingCount = members.filter(member => this.isWorkingStatus(member.status)).length;
        const pendingTaskCount = tasks.filter(task => !this.isCompletedStatus(task.status)).length;

        return `
            <div class="team-tab-name">${this.escapeHtml(team.name || '未命名团队')}</div>
            <div class="team-tab-count">${members.length} 人</div>
            <div class="team-tab-meta">${workingCount} 忙碌 / ${pendingTaskCount} 待办</div>
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

    handleContentWheel(event) {
        if (event.defaultPrevented || event.ctrlKey || Math.abs(event.deltaY) < 0.01) {
            return;
        }

        const scrollTarget = this.findWheelScrollTarget(event.target, event.deltaY);
        if (!(scrollTarget instanceof HTMLElement)) {
            return;
        }

        const before = scrollTarget.scrollTop;
        const next = before + event.deltaY;
        const maxScrollTop = Math.max(0, scrollTarget.scrollHeight - scrollTarget.clientHeight);
        scrollTarget.scrollTop = Math.max(0, Math.min(maxScrollTop, next));

        if (scrollTarget.scrollTop !== before) {
            event.preventDefault();
            event.stopPropagation();
        }
    }

    findWheelScrollTarget(startNode, deltaY) {
        const fallback = this.contentContainer instanceof HTMLElement ? this.contentContainer : null;
        let node = startNode instanceof Element ? startNode : fallback;

        while (node && node !== this.container && node !== document.body && node !== document.documentElement) {
            if (node instanceof HTMLElement && this.canWheelScroll(node, deltaY)) {
                return node;
            }

            if (node === fallback) {
                break;
            }
            node = node.parentElement;
        }

        return fallback && this.canWheelScroll(fallback, deltaY) ? fallback : null;
    }

    canWheelScroll(node, deltaY) {
        const style = window.getComputedStyle(node);
        const overflowY = style.overflowY || '';
        if (!/(auto|scroll|overlay)/.test(overflowY)) {
            return false;
        }

        const maxScrollTop = node.scrollHeight - node.clientHeight;
        if (maxScrollTop <= 1) {
            return false;
        }

        if (deltaY > 0) {
            return node.scrollTop < maxScrollTop - 1;
        }

        if (deltaY < 0) {
            return node.scrollTop > 1;
        }

        return false;
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
        const team = this.teams.find(item => item.name === this.currentTeam);
        if (!team) {
            this.showEmptyState();
            return;
        }

        const members = Array.isArray(team.members) ? team.members : [];
        const tasks = Array.isArray(team.tasks) ? team.tasks : [];
        const provider = this.detectTeamProvider(team);
        const { tasksByOwner, unassignedTasks } = this.groupTasksByOwner(members, tasks);
        const activityFeed = this.buildActivityFeed(members, tasksByOwner);

        const workingCount = members.filter(member => this.isWorkingStatus(member.status)).length;
        const activeCount = members.filter(member => this.isAgentActive(member)).length;
        const toolCount = members.filter(member => Boolean(member.last_tool_use)).length;
        const pendingTaskCount = tasks.filter(task => !this.isCompletedStatus(task.status)).length;
        const latestActivityText = this.findLatestTeamActivity(members);

        this.contentContainer.innerHTML = `
            <div class="team-header">
                <div class="team-title-row">
                    <div class="team-title">${this.escapeHtml(team.name || '未命名团队')}</div>
                    ${provider !== 'unknown' ? `<span class="team-provider">${this.escapeHtml(provider.toUpperCase())}</span>` : ''}
                </div>
                <div class="team-meta">${members.length} 个成员 · ${workingCount} 个忙碌中 · ${pendingTaskCount} 个未完成任务</div>
                ${team.project_cwd ? `<div class="team-path" title="${this.escapeHtml(team.project_cwd)}">${this.escapeHtml(team.project_cwd)}</div>` : ''}
            </div>

            <div class="team-summary-grid">
                ${this.renderSummaryCard('活跃成员', `${workingCount}/${members.length || 0}`, '状态为 working / busy')}
                ${this.renderSummaryCard('最近有动作', String(activeCount), latestActivityText || '暂无最近动作')}
                ${this.renderSummaryCard('工具调用', String(toolCount), toolCount > 0 ? '本轮有工具信号' : '暂未观测到工具调用')}
                ${this.renderSummaryCard('团队任务', String(pendingTaskCount), tasks.length > 0 ? `${tasks.length} 条任务已同步` : '暂无任务')}
            </div>

            ${activityFeed.length > 0 ? `
                <section class="sidebar-section sidebar-section-activity">
                    <div class="sidebar-section-header">
                        <div class="sidebar-section-title">最近活动</div>
                        <div class="sidebar-section-meta">${activityFeed.length} 条信号</div>
                    </div>
                    <div class="activity-feed">
                        ${activityFeed.map(item => this.renderActivityItem(item)).join('')}
                    </div>
                </section>
            ` : ''}

            <section class="sidebar-section">
                <div class="sidebar-section-header">
                    <div class="sidebar-section-title">团队任务</div>
                    <div class="sidebar-section-meta">${tasks.length} 项</div>
                </div>
                ${this.renderTeamTasks(tasks)}
            </section>

            <section class="sidebar-section">
                <div class="sidebar-section-header">
                    <div class="sidebar-section-title">Agent 实况</div>
                    <div class="sidebar-section-meta">${members.length} 位同事</div>
                </div>
                <div class="agent-cards">
                    ${members.length > 0
                        ? members.map(agent => this.renderAgentCard(agent, tasksByOwner[agent.name] || [])).join('')
                        : this.renderInlineEmptyState('暂无成员数据')}
                    ${unassignedTasks.length > 0 ? this.renderBroadcastCard(unassignedTasks) : ''}
                </div>
            </section>
        `;
    }

    renderSummaryCard(label, value, detail) {
        return `
            <div class="summary-card">
                <div class="summary-card-label">${this.escapeHtml(label)}</div>
                <div class="summary-card-value">${this.escapeHtml(value)}</div>
                <div class="summary-card-detail">${this.escapeHtml(detail)}</div>
            </div>
        `;
    }

    renderActivityItem(item) {
        return `
            <div class="activity-item">
                <div class="activity-item-icon ${this.escapeHtml(item.kind)}">${this.escapeHtml(item.icon)}</div>
                <div class="activity-item-body">
                    <div class="activity-item-head">
                        <span class="activity-item-name">${this.escapeHtml(item.agentName)}</span>
                        <span class="activity-item-tag">${this.escapeHtml(item.label)}</span>
                    </div>
                    <div class="activity-item-text">${this.escapeHtml(item.content)}</div>
                    <div class="activity-item-time">${this.escapeHtml(item.relativeTime || '刚刚')}</div>
                </div>
            </div>
        `;
    }

    renderTeamTasks(tasks) {
        if (!tasks.length) {
            return this.renderInlineEmptyState('当前团队还没有同步到任务');
        }

        const orderedTasks = [...tasks].sort((left, right) => {
            const statusDelta = this.getTaskStatusRank(left.status) - this.getTaskStatusRank(right.status);
            if (statusDelta !== 0) {
                return statusDelta;
            }

            const leftTime = this.toTimestamp(left.updated_at || left.created_at);
            const rightTime = this.toTimestamp(right.updated_at || right.created_at);
            return rightTime - leftTime;
        });

        const visibleTasks = orderedTasks.slice(0, 10);

        return `
            <div class="team-task-list">
                ${visibleTasks.map(task => this.renderTaskRow(task)).join('')}
                ${orderedTasks.length > visibleTasks.length
                    ? `<div class="list-overflow-hint">还有 ${orderedTasks.length - visibleTasks.length} 项任务未展开</div>`
                    : ''}
            </div>
        `;
    }

    renderTaskRow(task) {
        const owner = task.owner || '待分配';
        const statusClass = this.normalizeStatusClass(task.status);
        const updatedText = this.formatRelativeTime(task.updated_at || task.created_at);

        return `
            <div class="team-task-item">
                <div class="team-task-main">
                    <div class="team-task-head">
                        <span class="team-task-id">${this.escapeHtml(task.id || 'TASK')}</span>
                        <span class="task-status-pill ${this.escapeHtml(statusClass)}">${this.escapeHtml(this.formatTaskStatus(task.status))}</span>
                    </div>
                    <div class="team-task-subject">${this.escapeHtml(task.subject || '未命名任务')}</div>
                    <div class="team-task-meta">
                        <span>${this.escapeHtml(owner)}</span>
                        ${updatedText ? `<span>${this.escapeHtml(updatedText)} 更新</span>` : ''}
                    </div>
                </div>
            </div>
        `;
    }

    renderAgentCard(agent, assignedTasks) {
        const status = String(agent.status || 'idle').toLowerCase();
        const statusClass = this.isWorkingStatus(status) ? 'working' : status === 'completed' ? 'completed' : 'idle';
        const selectedClass = this.currentAgent === agent.name ? 'selected' : '';
        const dialogues = this.buildAgentDialogues(agent, assignedTasks);
        const highlights = this.buildAgentHighlights(agent);
        const lastActivityText = this.formatRelativeTime(agent.last_active_time || agent.last_message_time || agent.last_activity);
        const provider = this.detectAgentProvider(agent);
        const agentType = agent.agent_type || 'agent';

        return `
            <div class="agent-card ${statusClass} ${selectedClass}">
                <div class="agent-header">
                    <div class="agent-header-main">
                        <div class="agent-name-row">
                            <div class="agent-name">${this.escapeHtml(agent.name || '未命名 Agent')}</div>
                            <span class="agent-badge subtle">${this.escapeHtml(agentType)}</span>
                            ${provider !== 'unknown' ? `<span class="agent-badge provider">${this.escapeHtml(provider)}</span>` : ''}
                        </div>
                        <div class="agent-meta-row">
                            <span class="agent-status ${statusClass}">${this.escapeHtml(this.formatAgentStatus(status))}</span>
                            ${lastActivityText ? `<span class="agent-last-active">最后活动 ${this.escapeHtml(lastActivityText)}</span>` : '<span class="agent-last-active">暂无活动时间</span>'}
                        </div>
                    </div>
                </div>

                ${dialogues.length > 0 ? `
                    <div class="agent-dialogues">
                        ${dialogues.map((dialogue, index) => `
                            <div class="agent-bubble ${index === 0 ? 'primary' : 'secondary'}">${this.escapeHtml(dialogue)}</div>
                        `).join('')}
                    </div>
                ` : ''}

                ${highlights.length > 0 ? `
                    <div class="agent-signal-list">
                        ${highlights.map(item => `
                            <div class="agent-signal-item">
                                <div class="agent-signal-icon">${this.escapeHtml(item.icon)}</div>
                                <div class="agent-signal-body">
                                    <div class="agent-signal-label">${this.escapeHtml(item.label)}</div>
                                    <div class="agent-signal-text">${this.escapeHtml(item.content)}</div>
                                </div>
                            </div>
                        `).join('')}
                    </div>
                ` : ''}

                ${assignedTasks.length > 0 ? `
                    <div class="agent-block">
                        <div class="agent-block-title">负责任务</div>
                        <div class="agent-task-list">
                            ${assignedTasks.slice(0, 4).map(task => this.renderCompactTask(task)).join('')}
                            ${assignedTasks.length > 4 ? `<div class="list-overflow-hint">还有 ${assignedTasks.length - 4} 项任务</div>` : ''}
                        </div>
                    </div>
                ` : ''}

                ${Array.isArray(agent.todos) && agent.todos.length > 0 ? `
                    <div class="agent-block">
                        <div class="agent-block-title">待办清单</div>
                        <div class="todo-list">
                            ${agent.todos.slice(0, 5).map(todo => this.renderTodoRow(todo)).join('')}
                            ${agent.todos.length > 5 ? `<div class="list-overflow-hint">还有 ${agent.todos.length - 5} 项待办</div>` : ''}
                        </div>
                    </div>
                ` : ''}

                ${agent.cwd ? `<div class="agent-cwd" title="${this.escapeHtml(agent.cwd)}">${this.escapeHtml(agent.cwd)}</div>` : ''}
            </div>
        `;
    }

    renderCompactTask(task) {
        const statusClass = this.normalizeStatusClass(task.status);
        return `
            <div class="compact-row">
                <span class="compact-id">${this.escapeHtml(task.id || 'TASK')}</span>
                <span class="compact-status ${this.escapeHtml(statusClass)}">${this.escapeHtml(this.formatTaskStatus(task.status))}</span>
                <span class="compact-text">${this.escapeHtml(this.truncateText(task.subject || '未命名任务', 44))}</span>
            </div>
        `;
    }

    renderTodoRow(todo) {
        const statusClass = this.normalizeStatusClass(todo.status);
        const label = todo.status === 'in_progress' && todo.active_form ? todo.active_form : todo.content;
        return `
            <div class="compact-row todo ${this.escapeHtml(statusClass)}">
                <span class="compact-status-dot ${this.escapeHtml(statusClass)}"></span>
                <span class="compact-text">${this.escapeHtml(this.truncateText(label || '未命名待办', 46))}</span>
            </div>
        `;
    }

    renderBroadcastCard(tasks) {
        return `
            <div class="agent-card broadcast">
                <div class="agent-header">
                    <div class="agent-header-main">
                        <div class="agent-name-row">
                            <div class="agent-name">前台广播</div>
                            <span class="agent-badge subtle">${this.escapeHtml(`${tasks.length} 条待认领任务`)}</span>
                        </div>
                        <div class="agent-meta-row">
                            <span class="agent-status working">待认领</span>
                            <span class="agent-last-active">团队内暂无明确负责人</span>
                        </div>
                    </div>
                </div>
                <div class="agent-dialogues">
                    <div class="agent-bubble primary">有 ${this.escapeHtml(String(tasks.length))} 项任务尚未分配，欢迎同事主动认领。</div>
                </div>
                <div class="agent-block">
                    <div class="agent-block-title">待认领任务</div>
                    <div class="agent-task-list">
                        ${tasks.slice(0, 5).map(task => this.renderCompactTask(task)).join('')}
                        ${tasks.length > 5 ? `<div class="list-overflow-hint">还有 ${tasks.length - 5} 项任务</div>` : ''}
                    </div>
                </div>
            </div>
        `;
    }

    renderInlineEmptyState(text) {
        return `
            <div class="inline-empty-state">
                <div class="inline-empty-state-text">${this.escapeHtml(text)}</div>
            </div>
        `;
    }

    buildActivityFeed(members, tasksByOwner) {
        const items = [];

        members.forEach((agent) => {
            const relativeTime = this.formatRelativeTime(agent.last_active_time || agent.last_message_time || agent.last_activity);
            const assignedTasks = tasksByOwner[agent.name] || [];

            if (agent.current_task) {
                items.push(this.createActivityFeedItem(agent, 'task', '任务', '🎯', `正在处理 ${this.truncateText(agent.current_task, 56)}`, relativeTime));
            } else if (assignedTasks.length > 0) {
                items.push(this.createActivityFeedItem(agent, 'task', '任务', '🎯', `负责 ${assignedTasks.length} 项任务：${this.truncateText(assignedTasks[0].subject || '', 46)}`, relativeTime));
            }

            if (agent.last_tool_use) {
                const detail = agent.last_tool_detail ? ` · ${this.truncateText(agent.last_tool_detail, 42)}` : '';
                items.push(this.createActivityFeedItem(agent, 'tool', '工具', '🔧', `${agent.last_tool_use}${detail}`, relativeTime));
            }

            if (agent.message_summary || agent.latest_message) {
                items.push(this.createActivityFeedItem(
                    agent,
                    'message',
                    '消息',
                    '📨',
                    this.truncateText(agent.message_summary || agent.latest_message, 64),
                    relativeTime
                ));
            }

            if (agent.last_thinking) {
                items.push(this.createActivityFeedItem(agent, 'thinking', '思路', '💭', this.truncateText(agent.last_thinking, 64), relativeTime));
            }
        });

        return items
            .sort((left, right) => right.timestamp - left.timestamp)
            .slice(0, 8);
    }

    createActivityFeedItem(agent, kind, label, icon, content, relativeTime) {
        return {
            agentName: agent.name || '未命名 Agent',
            kind,
            label,
            icon,
            content,
            relativeTime,
            timestamp: this.toTimestamp(agent.last_active_time || agent.last_message_time || agent.last_activity)
        };
    }

    buildAgentDialogues(agent, tasks) {
        if (Array.isArray(agent.office_dialogues) && agent.office_dialogues.length > 0) {
            return agent.office_dialogues
                .map(line => this.truncateText(this.normalizeWhitespace(line), 100))
                .filter(Boolean)
                .slice(0, 3);
        }

        const dialogues = [];
        const activeTask = tasks.find(task => this.normalizeStatusClass(task.status) === 'in-progress') || tasks[0];

        if (agent.current_task && agent.current_task !== agent.name) {
            dialogues.push(`我正在推进「${this.truncateText(agent.current_task, 56)}」`);
        } else if (activeTask) {
            dialogues.push(`我在处理任务 #${activeTask.id || 'TASK'}：${this.truncateText(activeTask.subject || '未命名任务', 50)}`);
        }

        if (agent.last_tool_use) {
            const detail = agent.last_tool_detail ? `（${this.truncateText(agent.last_tool_detail, 36)}）` : '';
            dialogues.push(`我刚使用了 ${agent.last_tool_use}${detail}`);
        }

        if (agent.message_summary) {
            dialogues.push(`我刚收到：${this.truncateText(agent.message_summary, 58)}`);
        } else if (agent.latest_message) {
            dialogues.push(`我刚同步：${this.truncateText(agent.latest_message, 58)}`);
        }

        if (dialogues.length === 0 && agent.last_thinking) {
            dialogues.push(`我在想：${this.truncateText(agent.last_thinking, 58)}`);
        }

        if (dialogues.length === 0) {
            if (this.isWorkingStatus(agent.status)) {
                dialogues.push('我正专注处理中，稍后同步最新进展。');
            } else if (this.normalizeStatusClass(agent.status) === 'completed') {
                dialogues.push('我这边已完成本轮工作，等待下一项安排。');
            } else {
                dialogues.push('我这边空闲待命，随时可以接新任务。');
            }
        }

        return dialogues.slice(0, 3);
    }

    buildAgentHighlights(agent) {
        const highlights = [];

        if (agent.current_task) {
            highlights.push({
                icon: '🎯',
                label: '当前任务',
                content: this.truncateText(agent.current_task, 72)
            });
        }

        if (agent.last_tool_use) {
            const detail = agent.last_tool_detail ? ` · ${this.truncateText(agent.last_tool_detail, 48)}` : '';
            highlights.push({
                icon: '🔧',
                label: '工具调用',
                content: `${agent.last_tool_use}${detail}`
            });
        }

        if (agent.message_summary || agent.latest_message) {
            highlights.push({
                icon: '📨',
                label: '最近消息',
                content: this.truncateText(agent.message_summary || agent.latest_message, 72)
            });
        }

        if (agent.last_thinking) {
            highlights.push({
                icon: '💭',
                label: '思路片段',
                content: this.truncateText(agent.last_thinking, 72)
            });
        }

        return highlights.slice(0, 4);
    }

    groupTasksByOwner(agents, tasks) {
        const agentNames = new Set((agents || []).map(agent => agent.name));
        const tasksByOwner = {};
        const unassignedTasks = [];

        (tasks || []).forEach((task) => {
            let owner = task.owner || '';
            if (!owner && task.subject && agentNames.has(task.subject)) {
                owner = task.subject;
            }

            if (!owner) {
                unassignedTasks.push(task);
                return;
            }

            if (!tasksByOwner[owner]) {
                tasksByOwner[owner] = [];
            }
            tasksByOwner[owner].push(task);
        });

        return { tasksByOwner, unassignedTasks };
    }

    findLatestTeamActivity(members) {
        const latestTimestamp = Math.max(...members.map(member => this.toTimestamp(member.last_active_time || member.last_message_time || member.last_activity)), 0);
        if (!latestTimestamp) {
            return '';
        }
        return `最近一次更新 ${this.formatRelativeTime(latestTimestamp)}`;
    }

    detectTeamProvider(team) {
        const direct = String((team && team.provider) || '').toLowerCase();
        if (direct === 'claude' || direct === 'codex' || direct === 'openclaw') {
            return direct;
        }

        const members = Array.isArray(team && team.members) ? team.members : [];
        for (const member of members) {
            const provider = this.detectAgentProvider(member);
            if (provider !== 'unknown') {
                return provider;
            }
        }

        const teamName = String((team && team.name) || '').toLowerCase();
        if (teamName.startsWith('codex-')) {
            return 'codex';
        }
        if (teamName.startsWith('openclaw')) {
            return 'openclaw';
        }

        return 'unknown';
    }

    detectAgentProvider(agent) {
        const direct = String((agent && agent.provider) || '').toLowerCase();
        if (direct === 'claude' || direct === 'codex' || direct === 'openclaw') {
            return direct;
        }

        const type = String((agent && agent.agent_type) || '').toLowerCase();
        if (type.includes('openclaw')) {
            return 'openclaw';
        }
        if (type.includes('codex')) {
            return 'codex';
        }
        if (type.includes('claude')) {
            return 'claude';
        }

        return 'unknown';
    }

    formatAgentStatus(status) {
        switch (String(status || '').toLowerCase()) {
            case 'working':
            case 'busy':
                return '工作中';
            case 'completed':
                return '已完成';
            case 'idle':
            default:
                return '空闲';
        }
    }

    formatTaskStatus(status) {
        switch (String(status || '').toLowerCase()) {
            case 'in_progress':
                return '进行中';
            case 'completed':
                return '已完成';
            case 'pending':
            default:
                return '待处理';
        }
    }

    getTaskStatusRank(status) {
        switch (String(status || '').toLowerCase()) {
            case 'in_progress':
                return 0;
            case 'pending':
                return 1;
            case 'completed':
                return 2;
            default:
                return 3;
        }
    }

    isCompletedStatus(status) {
        return String(status || '').toLowerCase() === 'completed';
    }

    isWorkingStatus(status) {
        const normalized = String(status || '').toLowerCase();
        return normalized === 'working' || normalized === 'busy';
    }

    isAgentActive(agent) {
        const age = this.getTimestampAgeSeconds(agent.last_active_time || agent.last_message_time || agent.last_activity);
        if (age !== null && age <= 180) {
            return true;
        }
        return this.isWorkingStatus(agent.status) && Boolean(agent.last_tool_use || agent.message_summary || agent.current_task);
    }

    getTimestampAgeSeconds(timestamp) {
        const parsed = this.toTimestamp(timestamp);
        if (!parsed) {
            return null;
        }
        return Math.max(0, Math.floor((Date.now() - parsed) / 1000));
    }

    formatRelativeTime(timestamp) {
        const parsed = this.toTimestamp(timestamp);
        if (!parsed) {
            return '';
        }

        const diffSeconds = Math.max(0, Math.floor((Date.now() - parsed) / 1000));
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

    toTimestamp(value) {
        if (!value) {
            return 0;
        }
        if (typeof value === 'number' && Number.isFinite(value)) {
            return value;
        }

        const parsed = new Date(value);
        if (Number.isNaN(parsed.getTime()) || parsed.getFullYear() <= 1971) {
            return 0;
        }
        return parsed.getTime();
    }

    normalizeStatusClass(status) {
        return String(status || 'pending').toLowerCase().replace(/_/g, '-');
    }

    normalizeWhitespace(text) {
        return String(text || '').replace(/\s+/g, ' ').trim();
    }

    truncateText(text, maxLength) {
        const normalized = this.normalizeWhitespace(text);
        if (!normalized) {
            return '';
        }

        const chars = Array.from(normalized);
        if (chars.length <= maxLength) {
            return normalized;
        }

        return `${chars.slice(0, maxLength).join('')}...`;
    }

    escapeHtml(text) {
        return String(text || '')
            .replace(/&/g, '&amp;')
            .replace(/</g, '&lt;')
            .replace(/>/g, '&gt;')
            .replace(/"/g, '&quot;')
            .replace(/'/g, '&#39;');
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

    destroy() {
        this.hideTabPreview();
        if (this.contentContainer) {
            this.contentContainer.removeEventListener('wheel', this.handleContentWheel);
        }
        if (this.container) {
            this.container.remove();
        }
    }
}

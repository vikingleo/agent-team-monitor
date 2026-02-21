// API Configuration
const API_BASE_URL = window.location.origin;
const API_ENDPOINTS = {
    state: `${API_BASE_URL}/api/state`,
    teams: `${API_BASE_URL}/api/teams`,
    processes: `${API_BASE_URL}/api/processes`,
    health: `${API_BASE_URL}/api/health`
};

// State
let isConnected = true;
let updateInterval = null;
let previousState = null; // 存储上一次的状态用于对比

// Initialize
document.addEventListener('DOMContentLoaded', () => {
    console.log('Claude Agent Team Monitor initialized');
    initTabs();
    startAutoRefresh();
    fetchData();
});

// Initialize tabs
function initTabs() {
    const tabButtons = document.querySelectorAll('.tab-button');

    tabButtons.forEach(button => {
        button.addEventListener('click', () => {
            const tabName = button.getAttribute('data-tab');
            switchTab(tabName);
        });
    });
}

// Switch tabs
function switchTab(tabName) {
    // Update buttons
    document.querySelectorAll('.tab-button').forEach(btn => {
        btn.classList.remove('active');
    });
    document.querySelector(`[data-tab="${tabName}"]`).classList.add('active');

    // Update content
    document.querySelectorAll('.tab-content').forEach(content => {
        content.classList.remove('active');
    });
    document.getElementById(`${tabName}-tab`).classList.add('active');
}

// Auto-refresh
function startAutoRefresh() {
    updateInterval = setInterval(fetchData, 1000);
}

function stopAutoRefresh() {
    if (updateInterval) {
        clearInterval(updateInterval);
        updateInterval = null;
    }
}

// Fetch data from API
async function fetchData() {
    try {
        const response = await fetch(API_ENDPOINTS.state);
        if (!response.ok) {
            throw new Error(`HTTP error! status: ${response.status}`);
        }
        const data = await response.json();
        updateUI(data);
        updateConnectionStatus(true);
    } catch (error) {
        console.error('Error fetching data:', error);
        updateConnectionStatus(false);
    }
}

// Update UI with new data (智能更新，只更新变化的部分)
function updateUI(data) {
    updateLastUpdateTime(data.updated_at);

    // 更新计数徽章
    updateBadges(data);

    // 只在数据真正变化时才更新
    if (!previousState || JSON.stringify(previousState.processes) !== JSON.stringify(data.processes)) {
        updateProcesses(data.processes || []);
    }

    if (!previousState || JSON.stringify(previousState.teams) !== JSON.stringify(data.teams)) {
        updateTeams(data.teams || []);
    }

    previousState = data;
}

// Update count badges
function updateBadges(data) {
    document.getElementById('teams-count').textContent = (data.teams || []).length;
    document.getElementById('processes-count').textContent = (data.processes || []).length;
}

// Update last update time
function updateLastUpdateTime(timestamp) {
    const date = new Date(timestamp);
    const timeString = date.toLocaleTimeString('zh-CN');
    document.getElementById('last-update').textContent = `最后更新: ${timeString}`;
}

// Update connection status
function updateConnectionStatus(connected) {
    const statusEl = document.getElementById('connection-status');
    if (connected !== isConnected) {
        isConnected = connected;
        if (connected) {
            statusEl.innerHTML = '<span class="status-dot"></span> 已连接';
            statusEl.className = 'status-indicator connected';
        } else {
            statusEl.innerHTML = '<span class="status-dot"></span> 已断开';
            statusEl.className = 'status-indicator disconnected';
        }
    }
}

// Update processes section
function updateProcesses(processes) {
    const container = document.getElementById('processes-container');

    if (processes.length === 0) {
        container.innerHTML = '<p class="empty-state">未检测到 Claude 进程</p>';
        return;
    }

    const html = `
        <div class="process-list">
            ${processes.map(proc => renderProcess(proc)).join('')}
        </div>
    `;
    container.innerHTML = html;
}

// Render a single process
function renderProcess(process) {
    const uptime = formatUptime(process.started_at);
    return `
        <div class="process-item">
            <div class="process-info">
                <span class="process-pid">进程 ID: ${process.pid}</span>
                <span class="process-uptime">运行时间: ${uptime}</span>
            </div>
        </div>
    `;
}

// Update teams section
function updateTeams(teams) {
    const container = document.getElementById('teams-container');
    const nav = document.getElementById('team-nav');

    if (teams.length === 0) {
        container.innerHTML = '<p class="empty-state">未找到活动团队</p>';
        nav.innerHTML = '';
        nav.style.display = 'none';
        return;
    }

    const html = `
        <div class="teams-grid">
            ${teams.map(team => renderTeam(team)).join('')}
        </div>
    `;
    container.innerHTML = html;

    // Render team nav (only show when more than 1 team)
    if (teams.length > 1) {
        nav.style.display = '';
        nav.innerHTML = teams.map(team => {
            const teamId = `team-${encodeURIComponent(team.name)}`;
            return `<a class="team-nav-item" data-team-id="${teamId}" title="${escapeHtml(team.name)}">${escapeHtml(team.name)}</a>`;
        }).join('');

        // Bind click handlers
        nav.querySelectorAll('.team-nav-item').forEach(item => {
            item.addEventListener('click', () => {
                const targetId = item.getAttribute('data-team-id');
                const target = document.getElementById(targetId);
                if (target) {
                    target.scrollIntoView({ behavior: 'smooth', block: 'start' });
                }
            });
        });

        updateTeamNavActive();
    } else {
        nav.style.display = 'none';
        nav.innerHTML = '';
    }
}

// Highlight the team nav item closest to viewport top
function updateTeamNavActive() {
    const nav = document.getElementById('team-nav');
    if (!nav || nav.style.display === 'none') return;

    const items = nav.querySelectorAll('.team-nav-item');
    if (items.length === 0) return;

    let activeId = null;
    let minDistance = Infinity;

    items.forEach(item => {
        const teamId = item.getAttribute('data-team-id');
        const el = document.getElementById(teamId);
        if (el) {
            const rect = el.getBoundingClientRect();
            const distance = Math.abs(rect.top);
            if (distance < minDistance) {
                minDistance = distance;
                activeId = teamId;
            }
        }
    });

    items.forEach(item => {
        item.classList.toggle('active', item.getAttribute('data-team-id') === activeId);
    });
}

// Listen for scroll to update active team nav
let scrollRAF = null;
window.addEventListener('scroll', () => {
    if (scrollRAF) return;
    scrollRAF = requestAnimationFrame(() => {
        updateTeamNavActive();
        scrollRAF = null;
    });
}, { passive: true });

// Render a single team
function renderTeam(team) {
    const createdDate = new Date(team.created_at).toLocaleString('zh-CN');
    const members = team.members || [];
    const tasks = team.tasks || [];
    const workingCount = members.filter(member => member.status === 'working').length;
    const teamId = `team-${encodeURIComponent(team.name)}`;

    return `
        <div class="team-card" id="${teamId}">
            <div class="team-header">
                <div class="team-header-left">
                    <div class="team-name">${escapeHtml(team.name)}</div>
                    <div class="team-created">创建时间: ${createdDate}</div>
                </div>
                <button class="team-delete-btn" onclick="deleteTeam('${escapeHtml(team.name)}')" title="清理团队"><svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M3 6h18"/><path d="M19 6v14c0 1-1 2-2 2H7c-1 0-2-1-2-2V6"/><path d="M8 6V4c0-1 1-2 2-2h4c1 0 2 1 2 2v2"/></svg> 清理</button>
            </div>

            <div class="team-section office-scene">
                <h3><span class="section-icon"><svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><rect x="2" y="7" width="20" height="14" rx="2" ry="2"/><path d="M16 7V5a2 2 0 0 0-2-2h-4a2 2 0 0 0-2 2v2"/></svg></span> 办公区实况 (${members.length} 位同事, ${workingCount} 位忙碌中)</h3>
                <p class="office-hint">每位成员用"人话"同步当前状态、思路和工具动作。</p>
                ${renderAgentsWithTasks(members, tasks)}
            </div>
        </div>
    `;
}

// Delete a team
async function deleteTeam(teamName) {
    if (!confirm(`确定要清理团队「${teamName}」吗？\n\n这将删除该团队的配置和任务数据。`)) {
        return;
    }

    try {
        const response = await fetch(`${API_ENDPOINTS.teams}/${encodeURIComponent(teamName)}`, {
            method: 'DELETE',
        });
        if (!response.ok) {
            throw new Error(`HTTP error! status: ${response.status}`);
        }
        fetchData();
    } catch (error) {
        console.error('Error deleting team:', error);
        alert(`清理团队失败: ${error.message}`);
    }
}

function groupTasksByOwner(agents, tasks) {
    const agentNames = new Set(agents.map(agent => agent.name));
    const tasksByOwner = {};
    const unassignedTasks = [];

    tasks.forEach(task => {
        let owner = task.owner || '';

        if (!owner && task.subject && agentNames.has(task.subject)) {
            owner = task.subject;
        }

        if (owner) {
            if (!tasksByOwner[owner]) {
                tasksByOwner[owner] = [];
            }
            tasksByOwner[owner].push(task);
        } else {
            unassignedTasks.push(task);
        }
    });

    return { tasksByOwner, unassignedTasks };
}

// Render agents with their tasks
function renderAgentsWithTasks(agents, tasks) {
    if (agents.length === 0) {
        return '<p class="empty-state">无成员</p>';
    }

    const { tasksByOwner, unassignedTasks } = groupTasksByOwner(agents, tasks);

    return `
        <div class="agent-list office-floor">
            ${agents.map(agent => renderAgentWithTasks(agent, tasksByOwner[agent.name] || [])).join('')}
            ${unassignedTasks.length > 0 ? renderUnassignedTasks(unassignedTasks) : ''}
        </div>
    `;
}

function getRoleIcon(agent) {
    const name = (agent.name || '').toLowerCase();
    // Return first letter uppercase as text initial
    const initial = (agent.name || 'A').charAt(0).toUpperCase();
    return initial;
}

function normalizeDialogText(text, maxLength = 90) {
    if (!text) {
        return '';
    }

    const normalized = String(text).replace(/\s+/g, ' ').trim();
    if (normalized.length <= maxLength) {
        return normalized;
    }

    return `${normalized.slice(0, maxLength)}...`;
}

function isValidTimestamp(timestamp) {
    if (!timestamp) {
        return false;
    }

    const parsed = new Date(timestamp);
    if (Number.isNaN(parsed.getTime())) {
        return false;
    }

    return parsed.getFullYear() > 1971;
}

function formatRelativeTime(timestamp) {
    if (!isValidTimestamp(timestamp)) {
        return '';
    }

    const now = Date.now();
    const target = new Date(timestamp).getTime();
    const diffSeconds = Math.max(0, Math.floor((now - target) / 1000));

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

    const diffDays = Math.floor(diffHours / 24);
    return `${diffDays}天前`;
}

function getTimestampAgeSeconds(timestamp) {
    if (!isValidTimestamp(timestamp)) {
        return null;
    }

    const now = Date.now();
    const target = new Date(timestamp).getTime();
    const diffSeconds = Math.floor((now - target) / 1000);
    return Math.max(0, diffSeconds);
}

function isAgentInMotion(agent) {
    const lastActiveAge = getTimestampAgeSeconds(agent.last_active_time);
    const lastMessageAge = getTimestampAgeSeconds(agent.last_message_time);

    const hasRecentSignal =
        (lastActiveAge !== null && lastActiveAge <= 180) ||
        (lastMessageAge !== null && lastMessageAge <= 180);

    const hasActionPayload = Boolean(agent.last_tool_use || agent.last_thinking || agent.message_summary);

    return hasRecentSignal || (agent.status === 'working' && hasActionPayload);
}

function getAgentMotionLabel(agent, moving) {
    if (moving) {
        return '● 正在活动';
    }

    const lastActiveText = formatRelativeTime(agent.last_active_time);
    if (lastActiveText) {
        return `○ ${lastActiveText}`;
    }

    return '○ 待命';
}

function buildAgentDialogues(agent, tasks) {
    if (Array.isArray(agent.office_dialogues) && agent.office_dialogues.length > 0) {
        return agent.office_dialogues
            .map(line => normalizeDialogText(line, 100))
            .filter(Boolean)
            .slice(0, 3);
    }

    const dialogues = [];
    const showCurrentTask = agent.current_task && agent.current_task !== agent.name;
    const activeTask = tasks.find(task => task.status === 'in_progress') || tasks[0];

    if (showCurrentTask) {
        dialogues.push(`我正在推进「${normalizeDialogText(agent.current_task, 60)}」`);
    } else if (activeTask) {
        dialogues.push(`我在处理任务 #${activeTask.id}：${normalizeDialogText(activeTask.subject, 60)}`);
    }

    if (agent.last_tool_use) {
        const toolDetail = agent.last_tool_detail ? `（${normalizeDialogText(agent.last_tool_detail, 45)}）` : '';
        dialogues.push(`我刚使用了 ${agent.last_tool_use}${toolDetail}`);
    }

    if (agent.last_thinking) {
        dialogues.push(`我在想：${normalizeDialogText(agent.last_thinking)}`);
    }

    if (agent.message_summary) {
        dialogues.push(`我刚收到：${normalizeDialogText(agent.message_summary)}`);
    }

    if (dialogues.length === 0) {
        if (agent.status === 'working') {
            dialogues.push('我正专注处理中，稍后同步最新进展。');
        } else if (agent.status === 'completed') {
            dialogues.push('我这边已完成本轮工作，等待下一项安排。');
        } else {
            dialogues.push('我这边空闲待命，随时可以接新任务。');
        }
    }

    const lastActiveText = formatRelativeTime(agent.last_active_time);
    if (lastActiveText) {
        dialogues.push(`我最后一次动作是 ${lastActiveText}`);
    }

    return dialogues.slice(0, 3);
}

// Render a single agent with their tasks
function renderAgentWithTasks(agent, tasks) {
    const statusClass = agent.status.toLowerCase();
    const statusText = formatAgentStatus(agent.status);
    const dialogues = buildAgentDialogues(agent, tasks);
    const roleEmoji = getRoleIcon(agent);
    const moving = isAgentInMotion(agent);
    const motionClass = moving ? 'active-motion' : 'idle-motion';
    const motionLabel = getAgentMotionLabel(agent, moving);

    return `
        <div class="agent-item office-desk ${statusClass} ${motionClass}">
            <div class="agent-header">
                <span class="agent-avatar" aria-hidden="true">${roleEmoji}</span>
                <span class="agent-name">${escapeHtml(agent.name)}</span>
                <span class="agent-type">[${escapeHtml(agent.agent_type)}]</span>
                <span class="agent-status ${statusClass}">${statusText}</span>
                <span class="agent-activity ${moving ? 'active' : 'idle'}">${escapeHtml(motionLabel)}</span>
            </div>
            <div class="agent-dialogues">
                ${dialogues.map((dialogue, index) => `<div class="agent-bubble ${index === 0 ? 'primary' : 'secondary'}">${escapeHtml(dialogue)}</div>`).join('')}
            </div>
            ${agent.cwd ? `<div class="agent-cwd">${escapeHtml(agent.cwd)}</div>` : ''}
            ${agent.todos && agent.todos.length > 0 ? renderAgentTodos(agent.todos) : ''}
        </div>
    `;
}

// Render task list for an agent
function renderAgentTaskList(tasks) {
    return `
        <div class="task-list-compact">
            ${tasks.map(task => {
                const statusClass = task.status.toLowerCase();
                const statusText = formatTaskStatus(task.status);
                return `
                    <div class="task-item-compact">
                        <span class="task-id">${escapeHtml(task.id)}</span>
                        <span class="task-status ${statusClass}">${statusText}</span>
                        <span class="task-subject-compact">${escapeHtml(task.subject)}</span>
                    </div>
                `;
            }).join('')}
        </div>
    `;
}

// Render todo list for an agent
function renderAgentTodos(todos) {
    const todoStatusIcon = (status) => {
        switch (status) {
            case 'in_progress': return '<svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="#34d399" stroke-width="2.5"><circle cx="12" cy="12" r="10"/><path d="M12 6v6l4 2"/></svg>';
            case 'completed': return '<svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="#6b7280" stroke-width="2.5"><path d="M20 6L9 17l-5-5"/></svg>';
            default: return '<svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="#63636e" stroke-width="2"><rect x="3" y="3" width="18" height="18" rx="3"/></svg>';
        }
    };

    return `
        <div class="agent-todos">
            <div class="task-list-title">待办清单</div>
            <div class="todo-list-compact">
                ${todos.map(todo => {
                    const icon = todoStatusIcon(todo.status);
                    const label = todo.status === 'in_progress' && todo.active_form ? todo.active_form : todo.content;
                    const statusClass = todo.status.toLowerCase().replace('_', '-');
                    return `
                        <div class="todo-item-compact ${statusClass}">
                            <span class="todo-icon">${icon}</span>
                            <span class="todo-content">${escapeHtml(label)}</span>
                        </div>
                    `;
                }).join('')}
            </div>
        </div>
    `;
}

// Render unassigned tasks
function renderUnassignedTasks(tasks) {
    return `
        <div class="agent-item unassigned office-broadcast">
            <div class="agent-header">
                <span class="agent-avatar" aria-hidden="true">!</span>
                <span class="agent-name">前台广播</span>
                <span class="agent-type">[${tasks.length} 条待认领任务]</span>
            </div>
            <div class="agent-dialogues">
                <div class="agent-bubble primary">有 ${tasks.length} 项任务暂未分配，欢迎同事主动认领。</div>
            </div>
            <div class="agent-tasks">${renderAgentTaskList(tasks)}</div>
        </div>
    `;
}

// Render agents list
function renderAgents(agents) {
    if (agents.length === 0) {
        return '<p class="empty-state">无成员</p>';
    }

    return `
        <div class="agent-list">
            ${agents.map(agent => renderAgent(agent)).join('')}
        </div>
    `;
}

// Render a single agent
function renderAgent(agent) {
    const statusClass = agent.status.toLowerCase();
    const statusText = formatAgentStatus(agent.status);

    return `
        <div class="agent-item">
            <div class="agent-header">
                <span class="agent-name">${escapeHtml(agent.name)}</span>
                <span class="agent-type">[${escapeHtml(agent.agent_type)}]</span>
                <span class="agent-status ${statusClass}">${statusText}</span>
            </div>
            ${agent.cwd ? `<div class="agent-cwd">${escapeHtml(agent.cwd)}</div>` : ''}
            ${agent.current_task ? `<div class="agent-task">任务: ${escapeHtml(agent.current_task)}</div>` : ''}
        </div>
    `;
}

// Render tasks list
function renderTasks(tasks) {
    if (tasks.length === 0) {
        return '<p class="empty-state">无任务</p>';
    }

    return `
        <div class="task-list">
            ${tasks.map(task => renderTask(task)).join('')}
        </div>
    `;
}

// Render a single task
function renderTask(task) {
    const statusClass = task.status.toLowerCase();
    const statusText = formatTaskStatus(task.status);
    const owner = task.owner || '未分配';

    return `
        <div class="task-item">
            <div class="task-header">
                <span class="task-id">${escapeHtml(task.id)}</span>
                <span class="task-status ${statusClass}">${statusText}</span>
            </div>
            <div class="task-subject">${escapeHtml(task.subject)}</div>
            <div class="task-owner">负责人: ${escapeHtml(owner)}</div>
        </div>
    `;
}

// Format uptime
function formatUptime(startedAt) {
    const start = new Date(startedAt);
    const now = new Date();
    const diff = Math.floor((now - start) / 1000); // seconds

    const hours = Math.floor(diff / 3600);
    const minutes = Math.floor((diff % 3600) / 60);
    const seconds = diff % 60;

    if (hours > 0) {
        return `${hours}小时 ${minutes}分钟`;
    } else if (minutes > 0) {
        return `${minutes}分钟 ${seconds}秒`;
    } else {
        return `${seconds}秒`;
    }
}

// Format agent status
function formatAgentStatus(status) {
    const statusMap = {
        'working': '工作中',
        'idle': '空闲',
        'completed': '已完成'
    };
    return statusMap[status] || status.toUpperCase();
}

// Format task status
function formatTaskStatus(status) {
    const statusMap = {
        'in_progress': '进行中',
        'pending': '待处理',
        'completed': '已完成'
    };
    return statusMap[status] || status.toUpperCase();
}

// Escape HTML to prevent XSS
function escapeHtml(text) {
    const div = document.createElement('div');
    div.textContent = text;
    return div.innerHTML;
}

// Handle visibility change (pause updates when tab is hidden)
document.addEventListener('visibilitychange', () => {
    if (document.hidden) {
        stopAutoRefresh();
    } else {
        startAutoRefresh();
        fetchData();
    }
});

// Handle errors
window.addEventListener('error', (event) => {
    console.error('Global error:', event.error);
});

// Handle unhandled promise rejections
window.addEventListener('unhandledrejection', (event) => {
    console.error('Unhandled promise rejection:', event.reason);
});

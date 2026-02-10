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
            statusEl.textContent = '● 已连接';
            statusEl.className = 'status-indicator connected';
        } else {
            statusEl.textContent = '● 已断开';
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

    if (teams.length === 0) {
        container.innerHTML = '<p class="empty-state">未找到活动团队</p>';
        return;
    }

    const html = `
        <div class="teams-grid">
            ${teams.map(team => renderTeam(team)).join('')}
        </div>
    `;
    container.innerHTML = html;
}

// Render a single team
function renderTeam(team) {
    const createdDate = new Date(team.created_at).toLocaleString('zh-CN');

    return `
        <div class="team-card">
            <div class="team-header">
                <div class="team-name">${escapeHtml(team.name)}</div>
                <div class="team-created">创建时间: ${createdDate}</div>
            </div>

            <div class="team-section">
                <h3>成员 (${team.members?.length || 0})</h3>
                ${renderAgents(team.members || [])}
            </div>

            <div class="team-section">
                <h3>任务 (${team.tasks?.length || 0})</h3>
                ${renderTasks(team.tasks || [])}
            </div>
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
            <div class="agent-info">
                <span class="agent-name">${escapeHtml(agent.name)}</span>
                <span class="agent-type">[${escapeHtml(agent.agent_type)}]</span>
            </div>
            <div>
                <span class="agent-status ${statusClass}">${statusText}</span>
                ${agent.current_task ? `<div class="agent-task">任务: ${escapeHtml(agent.current_task)}</div>` : ''}
                ${agent.cwd ? `<div class="agent-cwd">📁 ${escapeHtml(agent.cwd)}</div>` : ''}
            </div>
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

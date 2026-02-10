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

// Initialize
document.addEventListener('DOMContentLoaded', () => {
    console.log('Claude Agent Team Monitor initialized');
    startAutoRefresh();
    fetchData();
});

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

// Update UI with new data
function updateUI(data) {
    updateLastUpdateTime(data.updated_at);
    updateProcesses(data.processes || []);
    updateTeams(data.teams || []);
}

// Update last update time
function updateLastUpdateTime(timestamp) {
    const date = new Date(timestamp);
    const timeString = date.toLocaleTimeString();
    document.getElementById('last-update').textContent = `Last updated: ${timeString}`;
}

// Update connection status
function updateConnectionStatus(connected) {
    const statusEl = document.getElementById('connection-status');
    if (connected !== isConnected) {
        isConnected = connected;
        if (connected) {
            statusEl.textContent = '● Connected';
            statusEl.className = 'status-indicator connected';
        } else {
            statusEl.textContent = '● Disconnected';
            statusEl.className = 'status-indicator disconnected';
        }
    }
}

// Update processes section
function updateProcesses(processes) {
    const container = document.getElementById('processes-container');

    if (processes.length === 0) {
        container.innerHTML = '<p class="empty-state">No Claude processes detected</p>';
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
                <span class="process-pid">PID: ${process.pid}</span>
                <span class="process-uptime">Uptime: ${uptime}</span>
            </div>
        </div>
    `;
}

// Update teams section
function updateTeams(teams) {
    const container = document.getElementById('teams-container');

    if (teams.length === 0) {
        container.innerHTML = '<p class="empty-state">No active teams found</p>';
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
    const createdDate = new Date(team.created_at).toLocaleString();

    return `
        <div class="team-card">
            <div class="team-header">
                <div class="team-name">${escapeHtml(team.name)}</div>
                <div class="team-created">Created: ${createdDate}</div>
            </div>

            <div class="team-section">
                <h3>Agents (${team.members?.length || 0})</h3>
                ${renderAgents(team.members || [])}
            </div>

            <div class="team-section">
                <h3>Tasks (${team.tasks?.length || 0})</h3>
                ${renderTasks(team.tasks || [])}
            </div>
        </div>
    `;
}

// Render agents list
function renderAgents(agents) {
    if (agents.length === 0) {
        return '<p class="empty-state">No agents</p>';
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
    const statusText = agent.status.toUpperCase();

    return `
        <div class="agent-item">
            <div class="agent-info">
                <span class="agent-name">${escapeHtml(agent.name)}</span>
                <span class="agent-type">[${escapeHtml(agent.agent_type)}]</span>
            </div>
            <div>
                <span class="agent-status ${statusClass}">${statusText}</span>
                ${agent.current_task ? `<div class="agent-task">Task: ${escapeHtml(agent.current_task)}</div>` : ''}
            </div>
        </div>
    `;
}

// Render tasks list
function renderTasks(tasks) {
    if (tasks.length === 0) {
        return '<p class="empty-state">No tasks</p>';
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
    const owner = task.owner || 'unassigned';

    return `
        <div class="task-item">
            <div class="task-header">
                <span class="task-id">[${escapeHtml(task.id)}]</span>
                <span class="task-status ${statusClass}">${statusText}</span>
            </div>
            <div class="task-subject">${escapeHtml(task.subject)}</div>
            <div class="task-owner">Owner: ${escapeHtml(owner)}</div>
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
        return `${hours}h ${minutes}m`;
    } else if (minutes > 0) {
        return `${minutes}m ${seconds}s`;
    } else {
        return `${seconds}s`;
    }
}

// Format task status
function formatTaskStatus(status) {
    const statusMap = {
        'in_progress': 'IN PROGRESS',
        'pending': 'PENDING',
        'completed': 'COMPLETED'
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

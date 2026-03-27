import { getDesktopPreferences, initDesktopUI, isDesktopMode, setDesktopPreferences } from './desktop-ui.js';

// API Configuration
const API_BASE_URL = window.location.origin;
const API_ENDPOINTS = {
    state: `${API_BASE_URL}/api/state`,
    teams: `${API_BASE_URL}/api/teams`,
    processes: `${API_BASE_URL}/api/processes`,
    health: `${API_BASE_URL}/api/health`
};
const DESKTOP_BRIDGE = window.AgentMonitorDesktopBridge || null;
const IS_DESKTOP_MODE = isDesktopMode();

// State
let isConnected = true;
let updateInterval = null;
let previousState = null; // 存储上一次渲染状态用于对比
let latestRawState = null; // 存储后端原始状态
let currentProviderFilter = 'all'; // all | claude | codex | openclaw
let hideIdleAgents = true;
const THEME_STORAGE_KEY = 'atm-dashboard-theme';
const DEFAULT_THEME = 'light';
const DASHBOARD_ACTIVE_WINDOW_MS = 20 * 60 * 1000;
let activeAgentDetailKey = null;
let dashboardScrollRoot = null;
let scrollRAF = null;
// Initialize
document.addEventListener('DOMContentLoaded', () => {
    console.log('Agent Team Monitor initialized');
    if (IS_DESKTOP_MODE) {
        const prefs = getDesktopPreferences();
        hideIdleAgents = prefs.hideIdleAgents !== false;
        currentProviderFilter = prefs.providerFilter || 'all';
    }
    applyDesktopMode();
    initDashboardScrollHandling();
    initThemeSwitcher();
    initTabs();
    initViewFilters();
    initAgentDetailModal();
    startAutoRefresh();
    fetchData();
    initDesktopUI({
        onRefresh: () => {
            fetchData();
        },
        onPreferencesChanged: (preferences) => {
            hideIdleAgents = preferences.hideIdleAgents !== false;
            currentProviderFilter = preferences.providerFilter || 'all';
            const hideIdleToggle = document.getElementById('hide-idle-toggle');
            if (hideIdleToggle) {
                hideIdleToggle.checked = hideIdleAgents;
            }
            document.querySelectorAll('.filter-chip').forEach((chip) => {
                const provider = chip.getAttribute('data-provider') || 'all';
                chip.classList.toggle('active', provider === currentProviderFilter);
            });
            renderFilteredUI();
        }
    });
});

function initThemeSwitcher() {
    const buttons = document.querySelectorAll('[data-theme-choice]');
    if (!buttons.length) {
        return;
    }

    const initialTheme = getStoredTheme();
    applyTheme(initialTheme);

    if (IS_DESKTOP_MODE) {
        return;
    }

    buttons.forEach((button) => {
        button.addEventListener('click', () => {
            const nextTheme = button.getAttribute('data-theme-choice') || DEFAULT_THEME;
            applyTheme(nextTheme);
        });
    });
}

function applyDesktopMode() {
    document.body.setAttribute('data-desktop-mode', IS_DESKTOP_MODE ? '1' : '0');
    document.documentElement.setAttribute('data-desktop-mode', IS_DESKTOP_MODE ? '1' : '0');
}

function getDashboardScrollRoot() {
    if (!IS_DESKTOP_MODE) {
        return null;
    }

    if (dashboardScrollRoot instanceof HTMLElement && dashboardScrollRoot.isConnected) {
        return dashboardScrollRoot;
    }

    dashboardScrollRoot = document.getElementById('app-scroll-root');
    return dashboardScrollRoot instanceof HTMLElement ? dashboardScrollRoot : null;
}

function findScrollableAncestor(start, fallback) {
    let node = start instanceof Element ? start : null;

    while (node && node !== document.body && node !== document.documentElement) {
        if (node === fallback) {
            return fallback;
        }

        if (node instanceof HTMLElement) {
            const style = window.getComputedStyle(node);
            const overflowY = style.overflowY || '';
            const canScroll = /(auto|scroll|overlay)/.test(overflowY) && node.scrollHeight > node.clientHeight + 1;
            if (canScroll) {
                return node;
            }
        }

        node = node.parentElement;
    }

    return fallback || null;
}

function scheduleTeamNavActiveUpdate() {
    if (scrollRAF) {
        return;
    }

    scrollRAF = requestAnimationFrame(() => {
        updateTeamNavActive();
        scrollRAF = null;
    });
}

function initDashboardScrollHandling() {
    if (!IS_DESKTOP_MODE) {
        window.addEventListener('scroll', scheduleTeamNavActiveUpdate, { passive: true });
        return;
    }

    const scrollRoot = getDashboardScrollRoot();
    if (!(scrollRoot instanceof HTMLElement)) {
        window.addEventListener('scroll', scheduleTeamNavActiveUpdate, { passive: true });
        return;
    }

    scrollRoot.setAttribute('tabindex', '-1');

    const focusScrollRoot = () => {
        scrollRoot.focus({ preventScroll: true });
    };

    scrollRoot.addEventListener('pointerdown', () => {
        focusScrollRoot();
    }, { passive: true });

    scrollRoot.addEventListener('scroll', scheduleTeamNavActiveUpdate, { passive: true });
    scrollRoot.addEventListener('wheel', (event) => {
        if (event.defaultPrevented || event.ctrlKey || Math.abs(event.deltaY) < 0.01) {
            return;
        }

        const activeScroller = findScrollableAncestor(event.target, scrollRoot);
        if (!(activeScroller instanceof HTMLElement) || activeScroller !== scrollRoot) {
            return;
        }

        const before = scrollRoot.scrollTop;
        scrollRoot.scrollTop += event.deltaY;
        if (scrollRoot.scrollTop !== before) {
            event.preventDefault();
        }
    }, { passive: false });

    document.addEventListener('keydown', (event) => {
        const activeElement = document.activeElement;
        const isTyping = activeElement instanceof HTMLElement && (
            activeElement.tagName === 'INPUT' ||
            activeElement.tagName === 'TEXTAREA' ||
            activeElement.tagName === 'SELECT' ||
            activeElement.isContentEditable
        );
        if (isTyping || event.defaultPrevented) {
            return;
        }

        let delta = 0;
        switch (event.key) {
            case 'ArrowDown':
                delta = 56;
                break;
            case 'ArrowUp':
                delta = -56;
                break;
            case 'PageDown':
                delta = Math.max(320, Math.floor(scrollRoot.clientHeight * 0.9));
                break;
            case 'PageUp':
                delta = -Math.max(320, Math.floor(scrollRoot.clientHeight * 0.9));
                break;
            case ' ':
                delta = event.shiftKey
                    ? -Math.max(320, Math.floor(scrollRoot.clientHeight * 0.9))
                    : Math.max(320, Math.floor(scrollRoot.clientHeight * 0.9));
                break;
            case 'Home':
                scrollRoot.scrollTo({ top: 0, behavior: 'smooth' });
                focusScrollRoot();
                event.preventDefault();
                return;
            case 'End':
                scrollRoot.scrollTo({ top: scrollRoot.scrollHeight, behavior: 'smooth' });
                focusScrollRoot();
                event.preventDefault();
                return;
            default:
                return;
        }

        const before = scrollRoot.scrollTop;
        scrollRoot.scrollBy({ top: delta, behavior: 'smooth' });
        focusScrollRoot();
        if (delta !== 0 || scrollRoot.scrollTop !== before) {
            event.preventDefault();
        }
    });

    window.setTimeout(() => {
        focusScrollRoot();
        scheduleTeamNavActiveUpdate();
    }, 0);
}

function getStoredTheme() {
    if (IS_DESKTOP_MODE) {
        return getDesktopPreferences().theme || DEFAULT_THEME;
    }

    try {
        const storedTheme = localStorage.getItem(THEME_STORAGE_KEY);
        if (storedTheme === 'light' || storedTheme === 'dark') {
            return storedTheme;
        }
    } catch (error) {
        console.warn('Unable to read theme from localStorage:', error);
    }

    return DEFAULT_THEME;
}

function applyTheme(theme) {
    const nextTheme = theme === 'dark' ? 'dark' : DEFAULT_THEME;
    document.documentElement.setAttribute('data-theme', nextTheme);

    if (!IS_DESKTOP_MODE) {
        try {
            localStorage.setItem(THEME_STORAGE_KEY, nextTheme);
        } catch (error) {
            console.warn('Unable to persist theme to localStorage:', error);
        }
    }

    document.querySelectorAll('[data-theme-choice]').forEach((button) => {
        const isActive = button.getAttribute('data-theme-choice') === nextTheme;
        button.classList.toggle('active', isActive);
        button.setAttribute('aria-pressed', isActive ? 'true' : 'false');
    });
}

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
    if (IS_DESKTOP_MODE && DESKTOP_BRIDGE) {
        updateInterval = DESKTOP_BRIDGE.startDesktopPolling(fetchData);
        return;
    }

    updateInterval = setInterval(fetchData, 1000);
}

function stopAutoRefresh() {
    if (updateInterval) {
        if (typeof updateInterval === 'function') {
            updateInterval();
        } else {
            clearInterval(updateInterval);
        }
        updateInterval = null;
    }
}

// Fetch data from API
async function fetchData() {
    try {
        let data;

        if (IS_DESKTOP_MODE && DESKTOP_BRIDGE) {
            data = await DESKTOP_BRIDGE.fetchDesktopState();
        } else {
            const response = await fetch(`${API_ENDPOINTS.state}?_ts=${Date.now()}`, {
                cache: 'no-store',
                headers: {
                    'Cache-Control': 'no-cache'
                }
            });
            if (!response.ok) {
                throw new Error(`HTTP error! status: ${response.status}`);
            }
            data = await response.json();
        }

        updateUI(data);
        updateConnectionStatus(true);
    } catch (error) {
        console.error('Error fetching data:', error);
        updateConnectionStatus(false);
    }
}

// Update UI with new data (智能更新，只更新变化的部分)
function updateUI(data) {
    latestRawState = data;
    renderFilteredUI();
}

function initViewFilters() {
    const chips = document.querySelectorAll('.filter-chip');
    chips.forEach((chip) => {
        chip.addEventListener('click', () => {
            const nextFilter = chip.getAttribute('data-provider') || 'all';
            currentProviderFilter = nextFilter;
            chips.forEach(node => node.classList.toggle('active', node === chip));
            renderFilteredUI();
            if (IS_DESKTOP_MODE) {
                const desktopProviderFilterSelect = document.getElementById('desktop-provider-filter-select');
                if (desktopProviderFilterSelect) {
                    desktopProviderFilterSelect.value = nextFilter;
                }
                setDesktopPreferences({
                    ...getDesktopPreferences(),
                    providerFilter: nextFilter
                }).catch((error) => {
                    console.error('Failed to persist desktop provider filter:', error);
                });
            }
        });
    });

    const hideIdleToggle = document.getElementById('hide-idle-toggle');
    if (hideIdleToggle) {
        hideIdleToggle.checked = hideIdleAgents;
        hideIdleToggle.addEventListener('change', (event) => {
            hideIdleAgents = Boolean(event.target.checked);
            renderFilteredUI();
            if (IS_DESKTOP_MODE) {
                const desktopHideIdleToggle = document.getElementById('desktop-hide-idle-toggle');
                if (desktopHideIdleToggle) {
                    desktopHideIdleToggle.checked = hideIdleAgents;
                }
                setDesktopPreferences({
                    ...getDesktopPreferences(),
                    hideIdleAgents
                }).catch((error) => {
                    console.error('Failed to persist desktop idle filter:', error);
                });
            }
        });
    }
}

function renderFilteredUI() {
    if (!latestRawState) {
        return;
    }

    updateProviderFilterStats(latestRawState);

    const filtered = buildFilteredState(latestRawState);
    updateLastUpdateTime(filtered.updated_at || latestRawState.updated_at);
    updateBadges(filtered);

    // 只在数据真正变化时才更新
    if (!previousState || JSON.stringify(previousState.processes) !== JSON.stringify(filtered.processes)) {
        updateProcesses(filtered.processes || []);
    }

    if (!previousState || JSON.stringify(previousState.teams) !== JSON.stringify(filtered.teams)) {
        updateTeams(filtered.teams || []);
    }

    previousState = filtered;
}

function buildFilteredState(rawState) {
    const sourceTeams = Array.isArray(rawState.teams) ? rawState.teams : [];
    const sourceProcesses = Array.isArray(rawState.processes) ? rawState.processes : [];

    const teams = sourceTeams
        .filter(team => matchesProvider(currentProviderFilter, detectTeamProvider(team)))
        .map(projectVisibleMembers)
        .filter(team => shouldKeepTeam(team));

    const processes = sourceProcesses.filter(process =>
        matchesProvider(currentProviderFilter, detectProcessProvider(process))
    );

    return {
        updated_at: rawState.updated_at,
        teams,
        processes
    };
}

function projectVisibleMembers(team) {
    const members = Array.isArray(team.members) ? team.members : [];
    const tasks = Array.isArray(team.tasks) ? team.tasks : [];
    const visibleMembers = hideIdleAgents
        ? members.filter(member => shouldShowDashboardAgent(member, tasks))
        : members;
    return {
        ...team,
        members: visibleMembers
    };
}

function updateProviderFilterStats(rawState) {
    const sourceTeams = Array.isArray(rawState.teams) ? rawState.teams : [];
    const counts = {
        claude: { teams: 0, agents: 0 },
        codex: { teams: 0, agents: 0 },
        openclaw: { teams: 0, agents: 0 }
    };

    sourceTeams.forEach((team) => {
        const provider = detectTeamProvider(team);
        if (provider !== 'claude' && provider !== 'codex' && provider !== 'openclaw') {
            return;
        }

        const projected = projectVisibleMembers(team);
        if (!shouldKeepTeam(projected)) {
            return;
        }

        counts[provider].teams += 1;
        counts[provider].agents += Array.isArray(projected.members) ? projected.members.length : 0;
    });

    const claudeCount = document.getElementById('claude-filter-count');
    if (claudeCount) {
        claudeCount.textContent = `(team:${counts.claude.teams},agent:${counts.claude.agents})`;
    }

    const codexCount = document.getElementById('codex-filter-count');
    if (codexCount) {
        codexCount.textContent = `(team:${counts.codex.teams},agent:${counts.codex.agents})`;
    }

    const openClawCount = document.getElementById('openclaw-filter-count');
    if (openClawCount) {
        openClawCount.textContent = `(team:${counts.openclaw.teams},agent:${counts.openclaw.agents})`;
    }
}

function shouldKeepTeam(team) {
    if (!hideIdleAgents) {
        return true;
    }

    const members = Array.isArray(team.members) ? team.members : [];
    if (members.length > 0) {
        return true;
    }

    const tasks = Array.isArray(team.tasks) ? team.tasks : [];
    return tasks.some(task => String(task.status || '').toLowerCase() !== 'completed');
}

function shouldShowDashboardAgent(agent, teamTasks = []) {
    if (!agent) {
        return false;
    }

    const status = String(agent.status || 'idle').toLowerCase();
    if (status === 'working' || status === 'busy') {
        return true;
    }

    if (agentHasVisibleTask(agent, teamTasks)) {
        return true;
    }

    if (hasAgentRecentSignal(agent)) {
        return true;
    }

    const activityTs = getAgentActivityTimestamp(agent);
    if (!activityTs) {
        return false;
    }

    return Date.now() - activityTs <= DASHBOARD_ACTIVE_WINDOW_MS;
}

function agentHasVisibleTask(agent, teamTasks = []) {
    if (String(agent?.current_task || '').trim() !== '') {
        return true;
    }

    const agentName = String(agent?.name || '');
    return (teamTasks || []).some((task) => {
        if (String(task?.status || '').toLowerCase() === 'completed') {
            return false;
        }

        const owner = String(task?.owner || '').trim();
        const subject = String(task?.subject || '').trim();
        return owner === agentName || (!owner && subject === agentName);
    });
}

function hasAgentRecentSignal(agent) {
    if (!agent) {
        return false;
    }

    if (agent.last_tool_use || agent.last_thinking || agent.message_summary || agent.latest_message || agent.latest_response) {
        return true;
    }

    if (Array.isArray(agent.recent_events) && agent.recent_events.some((event) => normalizeMultilineText(event?.text || ''))) {
        return true;
    }

    return Array.isArray(agent.office_dialogues) && agent.office_dialogues.some((line) => normalizeMultilineText(line));
}

function getAgentActivityTimestamp(agent) {
    return toTimestamp(agent?.last_active_time || agent?.last_message_time || agent?.last_activity);
}

function toTimestamp(value) {
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

function matchesProvider(activeFilter, entityProvider) {
    if (activeFilter === 'all') {
        return true;
    }
    return entityProvider === activeFilter;
}

function detectTeamProvider(team) {
    const direct = String((team && team.provider) || '').toLowerCase();
    if (direct === 'claude' || direct === 'codex' || direct === 'openclaw') {
        return direct;
    }

    const members = Array.isArray(team && team.members) ? team.members : [];
    for (const member of members) {
        const provider = String((member && member.provider) || '').toLowerCase();
        if (provider === 'claude' || provider === 'codex' || provider === 'openclaw') {
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

function detectProcessProvider(process) {
    const direct = String((process && process.provider) || '').toLowerCase();
    if (direct === 'claude' || direct === 'codex' || direct === 'openclaw') {
        return direct;
    }

    const cmd = String((process && process.command) || '').toLowerCase();
    if (cmd.includes('openclaw')) {
        return 'openclaw';
    }
    if (cmd.includes('codex')) {
        return 'codex';
    }
    if (cmd.includes('claude')) {
        return 'claude';
    }

    return 'unknown';
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
        container.innerHTML = '<p class="empty-state">当前筛选下未检测到进程</p>';
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
    const provider = detectProcessProvider(process).toUpperCase();
    return `
        <div class="process-item">
            <div class="process-info">
                <span class="process-pid">PID ${process.pid}</span>
                <span class="process-uptime">${provider}</span>
                <span class="process-uptime">${uptime}</span>
            </div>
            ${process.command ? `<div class="process-cmd">${escapeHtml(process.command)}</div>` : ''}
        </div>
    `;
}

// Update teams section
function updateTeams(teams) {
    const container = document.getElementById('teams-container');
    const nav = document.getElementById('team-nav');

    if (teams.length === 0) {
        container.innerHTML = '<p class="empty-state"><svg viewBox="0 0 24 24" width="32" height="32" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round"><path d="M16 21v-2a4 4 0 0 0-4-4H6a4 4 0 0 0-4 4v2"/><circle cx="9" cy="7" r="4"/><path d="M22 21v-2a4 4 0 0 0-3-3.87"/><path d="M16 3.13a4 4 0 0 1 0 7.75"/></svg>当前筛选下未找到活动团队</p>';
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
            const provider = detectTeamProvider(team);
            const providerBadge = provider === 'unknown' ? '' : ` [${escapeHtml(String(provider))}]`;
            return `<a class="team-nav-item" data-team-id="${teamId}" title="${escapeHtml(team.name)}">${escapeHtml(team.name)}${providerBadge}</a>`;
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

    syncAgentDetailModal(buildTeamDetailLookup(teams));
}

function buildTeamDetailLookup(teams) {
    const lookup = new Map();

    (teams || []).forEach((team) => {
        const members = Array.isArray(team.members) ? team.members : [];
        const tasks = Array.isArray(team.tasks) ? team.tasks : [];
        const { tasksByOwner, unassignedTasks } = groupTasksByOwner(members, tasks);

        members.forEach((agent) => {
            lookup.set(buildAgentDetailKey(team, agent), {
                team,
                agent,
                tasks: tasksByOwner[agent.name] || [],
                broadcast: false
            });
        });

        if (unassignedTasks.length > 0) {
            lookup.set(buildBroadcastDetailKey(team), {
                team,
                tasks: unassignedTasks,
                broadcast: true
            });
        }
    });

    return lookup;
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

// Render a single team
function renderTeam(team) {
    const createdDate = new Date(team.created_at).toLocaleString('zh-CN');
    const projectCwd = team.project_cwd || '';
    const members = team.members || [];
    const tasks = team.tasks || [];
    const provider = detectTeamProvider(team);
    const workingCount = members.filter(member => member.status === 'working').length;
    const activeCount = members.filter(member => isAgentInMotion(member)).length;
    const responseCount = members.filter(member => Boolean(agentPrimaryOutput(member))).length;
    const pendingTasks = tasks.filter(task => task.status !== 'completed').length;
    const teamId = `team-${encodeURIComponent(team.name)}`;
    const canDelete = provider !== 'codex';
    const providerBadge = provider !== 'unknown' ? `<span class="agent-type">[${escapeHtml(provider)}]</span>` : '';

    return `
        <div class="team-card" id="${teamId}">
            <div class="team-header">
                <div class="team-header-left">
                    <div class="team-name">${escapeHtml(team.name)} ${providerBadge}</div>
                    <div class="team-created">创建时间: ${createdDate}</div>
                    ${projectCwd ? `<div class="team-cwd">工作目录: ${escapeHtml(projectCwd)}</div>` : ''}
                </div>
                ${canDelete ? `<button class="team-delete-btn" onclick="deleteTeam('${escapeHtml(team.name)}')" title="清理团队"><svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M3 6h18"/><path d="M19 6v14c0 1-1 2-2 2H7c-1 0-2-1-2-2V6"/><path d="M8 6V4c0-1 1-2 2-2h4c1 0 2 1 2 2v2"/></svg> 清理</button>` : ''}
            </div>

            <div class="team-summary-bar">
                <div class="team-summary-item">
                    <span class="team-summary-label">成员</span>
                    <span class="team-summary-value">${members.length}</span>
                </div>
                <div class="team-summary-item">
                    <span class="team-summary-label">活跃中</span>
                    <span class="team-summary-value">${activeCount}</span>
                </div>
                <div class="team-summary-item">
                    <span class="team-summary-label">完整输出</span>
                    <span class="team-summary-value">${responseCount}</span>
                </div>
                <div class="team-summary-item">
                    <span class="team-summary-label">待处理任务</span>
                    <span class="team-summary-value">${pendingTasks}</span>
                </div>
            </div>

            <div class="team-section office-scene">
                <h3><span class="section-icon"><svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><rect x="2" y="7" width="20" height="14" rx="2" ry="2"/><path d="M16 7V5a2 2 0 0 0-2-2h-4a2 2 0 0 0-2 2v2"/></svg></span> 办公区实况 (${members.length} 位同事, ${workingCount} 位忙碌中)</h3>
                <p class="office-hint">常态下展示 4 列概览卡片，点击任意 Agent 可查看最近活动、完整输出、工具调用和待办详情。</p>
                ${renderAgentsWithTasks(team, members, tasks)}
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
        if (IS_DESKTOP_MODE && DESKTOP_BRIDGE) {
            await DESKTOP_BRIDGE.deleteDesktopTeam(teamName);
        } else {
            const response = await fetch(`${API_ENDPOINTS.teams}/${encodeURIComponent(teamName)}`, {
                method: 'DELETE',
            });
            if (!response.ok) {
                throw new Error(`HTTP error! status: ${response.status}`);
            }
        }
        await fetchData();
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
function renderAgentsWithTasks(team, agents, tasks) {
    if (agents.length === 0) {
        if (tasks.length > 0) {
            return `
                <div class="agent-list office-floor">
                    ${renderUnassignedTasks(team, tasks)}
                </div>
            `;
        }
        return '<p class="empty-state">无成员</p>';
    }

    const { tasksByOwner, unassignedTasks } = groupTasksByOwner(agents, tasks);

    return `
        <div class="agent-list office-floor">
            ${agents.map(agent => renderAgentCard(team, agent, tasksByOwner[agent.name] || [])).join('')}
            ${unassignedTasks.length > 0 ? renderUnassignedTasks(team, unassignedTasks) : ''}
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
    const lastSignalText = formatRelativeTime(agent.last_active_time || agent.last_message_time);

    if (moving) {
        if (lastSignalText) {
            return `● 正在活动 · 最后活动 ${lastSignalText}`;
        }
        return '● 正在活动';
    }

    if (lastSignalText) {
        return `○ 最后活动 ${lastSignalText}`;
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

    const lastActiveText = formatRelativeTime(agent.last_active_time || agent.last_message_time);
    if (lastActiveText) {
        dialogues.push(`我最后一次动作是 ${lastActiveText}`);
    }

    return dialogues.slice(0, 3);
}

function renderAgentCard(team, agent, tasks) {
    const statusClass = agent.status.toLowerCase();
    const statusText = formatAgentStatus(agent.status);
    const roleEmoji = getRoleIcon(agent);
    const moving = isAgentInMotion(agent);
    const motionClass = moving ? 'active-motion' : 'idle-motion';
    const motionLabel = getAgentMotionLabel(agent, moving);
    const primarySignal = buildAgentPrimarySignal(agent, tasks);
    const latestText = buildAgentLatestStatus(agent, tasks);
    const metaText = buildAgentMetaSummary(agent, tasks, moving);

    return `
        <button
            type="button"
            class="agent-item agent-compact-card office-desk ${statusClass} ${motionClass}"
            data-agent-detail-key="${escapeHtml(buildAgentCardKey(team, agent))}"
            aria-label="查看 ${escapeHtml(agent.name)} 详情"
        >
            <div class="agent-header">
                <span class="agent-avatar" aria-hidden="true">${roleEmoji}</span>
                <span class="agent-name">${escapeHtml(agent.name)}</span>
                <span class="agent-type">[${escapeHtml(agent.agent_type)}]</span>
                <span class="agent-status ${statusClass}">${statusText}</span>
                <span class="agent-activity ${moving ? 'active' : 'idle'}">${escapeHtml(motionLabel)}</span>
            </div>
            ${primarySignal ? `
                <div class="agent-compact-signal ${escapeHtml(primarySignal.kind)}">
                    <div class="agent-compact-label">${escapeHtml(primarySignal.label)}</div>
                    <div class="agent-compact-value">${escapeHtml(primarySignal.value)}</div>
                </div>
            ` : ''}
            <div class="agent-compact-body">
                <div class="agent-compact-latest">${escapeHtml(latestText)}</div>
                <div class="agent-compact-meta">${escapeHtml(metaText)}</div>
            </div>
        </button>
    `;
}

function buildAgentCardKey(team, agent) {
    return `${team.name}::agent-card::${agent.name}`;
}

function buildAgentDetailKey(team, agent) {
    return `${team.name}::agent::${agent.name}`;
}

function buildBroadcastDetailKey(team) {
    return `${team.name}::broadcast`;
}

function buildAgentPrimarySignal(agent, tasks) {
    if (agent.current_task) {
        return { kind: 'task', label: '当前任务', value: truncateMultiline(agent.current_task, 44) };
    }
    if (agent.last_tool_use) {
        return {
            kind: 'tool',
            label: '调用工具',
            value: truncateMultiline(agent.last_tool_detail ? `${agent.last_tool_use} · ${agent.last_tool_detail}` : agent.last_tool_use, 44)
        };
    }
    if (agent.message_summary || agent.latest_message) {
        return { kind: 'message', label: '最近消息', value: truncateMultiline(agent.message_summary || agent.latest_message, 44) };
    }
    if (agent.last_thinking) {
        return { kind: 'thinking', label: '最新思路', value: truncateMultiline(agent.last_thinking, 44) };
    }
    if (tasks.length > 0) {
        return { kind: 'task', label: '负责任务', value: truncateMultiline(tasks[0].subject || '未命名任务', 44) };
    }
    return null;
}

function buildAgentLatestStatus(agent, tasks) {
    const output = truncateMultiline(agentPrimaryOutput(agent), 80);
    if (output) {
        return output;
    }

    const dialogues = buildAgentDialogues(agent, tasks);
    if (dialogues.length > 0) {
        return truncateMultiline(dialogues[0], 80);
    }

    return '暂无最新输出';
}

function buildAgentMetaSummary(agent, tasks, moving) {
    const parts = [];
    const relative = formatRelativeTime(agent.last_active_time || agent.last_message_time || agent.last_activity);

    if (relative) {
        parts.push(`最后活动 ${relative}`);
    } else {
        parts.push(moving ? '正在活动' : '待命');
    }

    if (tasks.length > 0) {
        parts.push(`${tasks.length} 项任务`);
    }

    if (Array.isArray(agent.todos) && agent.todos.length > 0) {
        parts.push(`${agent.todos.length} 条待办`);
    }

    return parts.join(' · ');
}

// Render a single agent with their tasks
function renderAgentWithTasks(agent, tasks) {
    const primaryOutput = agentPrimaryOutput(agent);
    const timeline = buildAgentTimeline(agent, tasks);
    const latestSections = [
        renderAgentSignals(agent),
        primaryOutput ? renderAgentOutput(primaryOutput) : '',
        tasks.length > 0 ? `<div class="agent-tasks-panel agent-panel-section panel-tasklist"><div class="agent-panel-title">负责任务</div>${renderAgentTaskList(tasks)}</div>` : '',
        agent.todos && agent.todos.length > 0 ? renderAgentTodos(agent.todos) : ''
    ].filter(Boolean).join('');

    return `
        <div class="agent-detail-shell">
            <div class="agent-layout">
                <div class="agent-main-column">
                    ${latestSections || renderAgentLatestFallback('暂无新的输出或工具信号')}
                </div>
                <div class="agent-side-column">
                    ${renderAgentTimeline(timeline)}
                </div>
            </div>
            ${agent.cwd ? `<div class="agent-cwd">${escapeHtml(agent.cwd)}</div>` : ''}
        </div>
    `;
}

function agentPrimaryOutput(agent) {
    return normalizeMultilineText(agent.latest_response || agent.latest_message || '');
}

function buildAgentTimeline(agent, tasks) {
    if (Array.isArray(agent.recent_events) && agent.recent_events.length > 0) {
        return agent.recent_events
            .map((event) => ({
                kind: event.kind || 'message',
                title: event.title || inferEventTitle(event.kind),
                text: normalizeMultilineText(event.text || ''),
                time: event.timestamp,
                relative: formatRelativeTime(event.timestamp)
            }))
            .filter(event => event.text)
            .slice(0, 8);
    }

    const timeline = [];

    if (agent.last_tool_use) {
        timeline.push({
            kind: 'tool',
            title: '工具调用',
            text: agent.last_tool_detail ? `${agent.last_tool_use} · ${agent.last_tool_detail}` : agent.last_tool_use,
            time: agent.last_active_time || agent.last_message_time || agent.last_activity,
            relative: formatRelativeTime(agent.last_active_time || agent.last_message_time || agent.last_activity)
        });
    }

    if (agent.last_thinking) {
        timeline.push({
            kind: 'thinking',
            title: '思路',
            text: normalizeMultilineText(agent.last_thinking),
            time: agent.last_active_time || agent.last_message_time || agent.last_activity,
            relative: formatRelativeTime(agent.last_active_time || agent.last_message_time || agent.last_activity)
        });
    }

    if (agent.latest_message) {
        timeline.push({
            kind: 'message',
            title: '消息',
            text: normalizeMultilineText(agent.latest_message),
            time: agent.last_message_time || agent.last_activity,
            relative: formatRelativeTime(agent.last_message_time || agent.last_activity)
        });
    }

    if (agent.current_task) {
        timeline.push({
            kind: 'task',
            title: '当前任务',
            text: normalizeMultilineText(agent.current_task),
            time: agent.last_active_time || agent.last_activity,
            relative: formatRelativeTime(agent.last_active_time || agent.last_activity)
        });
    }

    if (!timeline.length && tasks.length > 0) {
        timeline.push({
            kind: 'task',
            title: '任务',
            text: normalizeMultilineText(tasks[0].subject || ''),
            time: tasks[0].updated_at || tasks[0].created_at,
            relative: formatRelativeTime(tasks[0].updated_at || tasks[0].created_at)
        });
    }

    return timeline.slice(0, 6);
}

function renderAgentSignals(agent, options = {}) {
    const { truncate = false } = options;
    const chips = [];

    if (agent.current_task) {
        chips.push({
            kind: 'task',
            label: '当前任务',
            value: truncate ? truncateMultiline(agent.current_task, 100) : normalizeMultilineText(agent.current_task)
        });
    }
    if (agent.last_tool_use) {
        chips.push({
            kind: 'tool',
            label: '调用工具',
            value: truncate
                ? truncateMultiline(agent.last_tool_detail ? `${agent.last_tool_use} · ${agent.last_tool_detail}` : agent.last_tool_use, 100)
                : normalizeMultilineText(agent.last_tool_detail ? `${agent.last_tool_use} · ${agent.last_tool_detail}` : agent.last_tool_use)
        });
    }
    if (agent.message_summary) {
        chips.push({
            kind: 'message',
            label: '最近消息',
            value: truncate ? truncateMultiline(agent.message_summary, 100) : normalizeMultilineText(agent.message_summary)
        });
    }
    if (agent.last_thinking) {
        chips.push({
            kind: 'thinking',
            label: '最新思路',
            value: truncate ? truncateMultiline(agent.last_thinking, 100) : normalizeMultilineText(agent.last_thinking)
        });
    }

    if (chips.length === 0) {
        return '';
    }

    return `
        <div class="agent-signal-grid">
            ${chips.map(chip => `
                <div class="agent-signal-card ${escapeHtml(chip.kind)}">
                    <div class="agent-panel-title">${escapeHtml(chip.label)}</div>
                    <div class="agent-signal-value">${escapeHtml(chip.value)}</div>
                </div>
            `).join('')}
        </div>
    `;
}

function renderAgentOutput(text) {
    return `
        <div class="agent-output-panel agent-panel-section panel-output">
            <div class="agent-panel-title">最新完整输出</div>
            <pre class="agent-output-text">${escapeHtml(text)}</pre>
        </div>
    `;
}

function renderAgentLatestFallback(text) {
    return `
        <div class="agent-output-panel agent-panel-section panel-output">
            <div class="agent-panel-title">最新状态</div>
            <div class="agent-empty-text">${escapeHtml(text || '暂无新的输出或工具信号')}</div>
        </div>
    `;
}

function renderAgentTimeline(timeline) {
    return `
        <div class="agent-timeline-panel agent-panel-section panel-activity">
            <div class="agent-panel-title">最近活动</div>
            ${timeline.length > 0 ? `
                <div class="agent-timeline">
                    ${timeline.map(event => `
                        <div class="timeline-item ${escapeHtml(event.kind)}">
                            <div class="timeline-marker">${escapeHtml(eventGlyph(event.kind))}</div>
                            <div class="timeline-content">
                                <div class="timeline-head">
                                    <span class="timeline-title">${escapeHtml(event.title)}</span>
                                    ${event.relative ? `<span class="timeline-time">${escapeHtml(event.relative)}</span>` : ''}
                                </div>
                                <pre class="timeline-text">${escapeHtml(event.text)}</pre>
                            </div>
                        </div>
                    `).join('')}
                </div>
            ` : '<div class="agent-empty-text">暂无最近活动</div>'}
        </div>
    `;
}

function inferEventTitle(kind) {
    switch ((kind || '').toLowerCase()) {
        case 'response': return '输出';
        case 'thinking': return '思路';
        case 'tool': return '工具调用';
        case 'task': return '任务';
        case 'status': return '状态';
        default: return '消息';
    }
}

function eventGlyph(kind) {
    switch ((kind || '').toLowerCase()) {
        case 'response': return '↳';
        case 'thinking': return '⋯';
        case 'tool': return '⌘';
        case 'task': return '•';
        case 'status': return '○';
        default: return '✦';
    }
}

function normalizeMultilineText(text) {
    if (!text) {
        return '';
    }

    return String(text)
        .replace(/\r\n/g, '\n')
        .replace(/\n{3,}/g, '\n\n')
        .trim();
}

function truncateMultiline(text, maxLength = 140) {
    const normalized = normalizeMultilineText(text);
    if (!normalized) {
        return '';
    }

    const chars = Array.from(normalized);
    if (chars.length <= maxLength) {
        return normalized;
    }

    return `${chars.slice(0, maxLength).join('')}...`;
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
        <div class="agent-todos agent-panel-section panel-todo">
            <div class="agent-panel-title">待办清单</div>
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
function renderUnassignedTasks(team, tasks) {
    return `
        <button
            type="button"
            class="agent-item agent-compact-card unassigned office-broadcast"
            data-agent-detail-key="${escapeHtml(buildBroadcastDetailKey(team))}"
            aria-label="查看待认领任务详情"
        >
            <div class="agent-header">
                <span class="agent-avatar" aria-hidden="true">!</span>
                <span class="agent-name">前台广播</span>
                <span class="agent-type">[待认领]</span>
                <span class="agent-status pending">待处理</span>
            </div>
            <div class="agent-compact-signal task">
                <div class="agent-compact-label">待认领任务</div>
                <div class="agent-compact-value">${tasks.length} 项待分配</div>
            </div>
            <div class="agent-compact-body">
                <div class="agent-compact-latest">有任务尚未分配，欢迎团队成员主动认领。</div>
                <div class="agent-compact-meta">${tasks.length} 项任务等待负责人</div>
            </div>
        </button>
    `;
}

function renderBroadcastDetail(tasks) {
    return `
        <div class="agent-detail-shell">
            <div class="agent-output-panel agent-panel-section panel-output">
                <div class="agent-panel-title">广播说明</div>
                <div class="agent-empty-text">有 ${escapeHtml(String(tasks.length))} 项任务尚未分配，欢迎团队成员主动认领。</div>
            </div>
            <div class="agent-tasks-panel agent-panel-section panel-tasklist">
                <div class="agent-panel-title">待认领任务</div>
                ${renderAgentTaskList(tasks)}
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

function initAgentDetailModal() {
    document.addEventListener('click', (event) => {
        const opener = event.target.closest('[data-agent-detail-key]');
        if (opener) {
            openAgentDetail(opener.getAttribute('data-agent-detail-key'));
            return;
        }

        const closer = event.target.closest('[data-close-agent-detail]');
        if (closer) {
            closeAgentDetail();
        }
    });

    document.addEventListener('keydown', (event) => {
        if (event.key === 'Escape') {
            closeAgentDetail();
        }
    });
}

function openAgentDetail(key) {
    if (!key) {
        return;
    }

    activeAgentDetailKey = key;
    syncAgentDetailModal(buildTeamDetailLookup(previousState ? previousState.teams || [] : []));
}

function closeAgentDetail() {
    activeAgentDetailKey = null;

    const modal = document.getElementById('agent-detail-modal');
    const content = document.getElementById('agent-detail-content');

    if (content) {
        content.innerHTML = '';
    }

    if (modal) {
        modal.hidden = true;
        document.body.classList.remove('agent-detail-open');
    }
}

function syncAgentDetailModal(lookup) {
    const modal = document.getElementById('agent-detail-modal');
    const content = document.getElementById('agent-detail-content');

    if (!modal || !content) {
        return;
    }

    if (!activeAgentDetailKey) {
        modal.hidden = true;
        document.body.classList.remove('agent-detail-open');
        return;
    }

    const resolved = resolveActiveAgentDetail(activeAgentDetailKey, lookup);
    if (!resolved) {
        closeAgentDetail();
        return;
    }

    content.innerHTML = resolved.broadcast
        ? renderAgentDetailModalContent(resolved.team, null, resolved.tasks, true)
        : renderAgentDetailModalContent(resolved.team, resolved.agent, resolved.tasks, false);

    modal.hidden = false;
    document.body.classList.add('agent-detail-open');
}

function resolveActiveAgentDetail(activeKey, lookup) {
    if (!lookup || lookup.size === 0) {
        return null;
    }

    for (const [key, value] of lookup.entries()) {
        if (value.broadcast && activeKey === key) {
            return value;
        }

        if (!value.broadcast && (activeKey === key || activeKey === buildAgentCardKey(value.team, value.agent))) {
            return value;
        }
    }

    return null;
}

function renderAgentDetailModalContent(team, agent, tasks, isBroadcast) {
    if (isBroadcast) {
        return `
            <div class="agent-detail-header">
                <div class="agent-detail-title-wrap">
                    <div id="agent-detail-title" class="agent-detail-title">前台广播</div>
                    <div class="agent-detail-subtitle">${escapeHtml(team.name)} · ${escapeHtml(String(tasks.length))} 条待认领任务</div>
                </div>
            </div>
            ${renderBroadcastDetail(tasks)}
        `;
    }

    const statusClass = String(agent.status || 'idle').toLowerCase();
    const statusText = formatAgentStatus(agent.status);
    const moving = isAgentInMotion(agent);
    const motionLabel = getAgentMotionLabel(agent, moving);

    return `
        <div class="agent-detail-header">
            <div class="agent-detail-title-wrap">
                <div id="agent-detail-title" class="agent-detail-title">${escapeHtml(agent.name)}</div>
                <div class="agent-detail-subtitle">${escapeHtml(team.name)} · ${escapeHtml(agent.agent_type || 'agent')}</div>
            </div>
            <div class="agent-detail-badges">
                <span class="agent-status ${escapeHtml(statusClass)}">${escapeHtml(statusText)}</span>
                <span class="agent-activity ${moving ? 'active' : 'idle'}">${escapeHtml(motionLabel)}</span>
            </div>
        </div>
        ${renderAgentWithTasks(agent, tasks)}
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

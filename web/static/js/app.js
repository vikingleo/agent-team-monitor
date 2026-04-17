import { getDesktopPreferences, initDesktopUI, isDesktopMode, setDesktopAdminAccess, setDesktopPreferences } from './desktop-ui.js';

// API Configuration
const API_BASE_URL = window.location.origin;
const API_ENDPOINTS = {
    state: `${API_BASE_URL}/api/state`,
    teams: `${API_BASE_URL}/api/teams`,
    managedTeams: `${API_BASE_URL}/api/managed/teams`,
    agentMessage: `${API_BASE_URL}/api/agents/message`,
    authStatus: `${API_BASE_URL}/api/auth/status`,
    authLogin: `${API_BASE_URL}/api/auth/login`,
    authLogout: `${API_BASE_URL}/api/auth/logout`,
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
const TEAM_OVERVIEW_KEY = '__team__';
const CONTROL_FEED_LIMIT = 48;
const STATE_LABELS = {
    working: '工作中',
    busy: '忙碌中',
    idle: '空闲',
    completed: '已完成',
    pending: '待处理',
    in_progress: '进行中',
    running: '运行中',
    running_detached: '后台运行',
    stopped: '已停止',
    failed: '失败',
    error: '异常',
    paused: '已暂停',
    waiting: '等待中',
    created: '已创建',
    starting: '启动中',
    stopping: '停止中',
    online: '在线',
    offline: '离线',
    detached: '已脱离',
    unknown: '未知'
};
const ROLE_LABELS = {
    lead: '主成员',
    leader: '主成员',
    primary: '主成员',
    owner: '主成员',
    main: '主成员',
    agent: '成员',
    member: '成员',
    worker: '执行成员',
    coder: '编码成员',
    implementer: '实现成员',
    planner: '规划成员',
    reviewer: '评审成员',
    researcher: '研究成员',
    coordinator: '协调成员',
    manager: '管理成员',
    frontend: '前端',
    backend: '后端',
    fullstack: '全栈',
    ui: '界面',
    ux: '体验',
    design: '设计',
    qa: '测试',
    tester: '测试',
    ops: '运维',
    devops: '运维',
    infra: '基础设施',
    infrastructure: '基础设施',
    unknown: '未标注'
};
let activeAgentDetailKey = null;
let dashboardScrollRoot = null;
let scrollRAF = null;
let latestManagedTeams = [];
let selectedTeamName = null;
let selectedAgentKey = null;
let deferredTeamsRender = null;
let controlComposerIsComposing = false;
const controlComposerDrafts = {};
const controlComposerFeedback = {};
const controlComposerSubmitting = {};
const pendingOperatorMessages = [];
let adminAuthState = {
    configured: false,
    authenticated: false,
};

function isModalVisible(id) {
    const modal = document.getElementById(id);
    return Boolean(modal && modal.hidden === false);
}

function syncOverlayState() {
    const shouldLock = Boolean(activeAgentDetailKey) || isModalVisible('auth-modal') || isModalVisible('managed-team-modal');
    document.body.classList.toggle('agent-detail-open', shouldLock);
}
// Initialize
document.addEventListener('DOMContentLoaded', async () => {
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
    initManagedTeamControls();
    initAuthControls();
    initControlWorkspace();
    initAgentDetailModal();
    await refreshAuthStatus();
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
    setDesktopAdminAccess(isAdminAuthenticated());
});

function isAdminAuthenticated() {
    return adminAuthState.configured === true && adminAuthState.authenticated === true;
}

async function refreshAuthStatus() {
    try {
        const response = await fetch(API_ENDPOINTS.authStatus, { cache: 'no-store' });
        if (!response.ok) {
            throw new Error(`HTTP ${response.status}`);
        }
        adminAuthState = await response.json();
    } catch (error) {
        console.error('Failed to load auth status:', error);
        adminAuthState = {
            configured: false,
            authenticated: false,
        };
    }

    applyAdminAccessState();
    return { ...adminAuthState };
}

function applyAdminAccessState() {
    const indicator = document.getElementById('admin-auth-status');
    const button = document.getElementById('admin-auth-button');

    if (indicator) {
        indicator.classList.remove('authenticated', 'unconfigured');
        if (!adminAuthState.configured) {
            indicator.textContent = '管理：未配置';
            indicator.classList.add('unconfigured');
        } else if (adminAuthState.authenticated) {
            indicator.textContent = '管理：已登录';
            indicator.classList.add('authenticated');
        } else {
            indicator.textContent = '管理：未登录';
        }
    }

    if (button) {
        if (!adminAuthState.configured) {
            button.textContent = '未配置登录';
            button.disabled = true;
        } else if (adminAuthState.authenticated) {
            button.textContent = '退出管理';
            button.disabled = false;
        } else {
            button.textContent = '管理员登录';
            button.disabled = false;
        }
    }

    setDesktopAdminAccess(isAdminAuthenticated());

    if (previousState) {
        renderFilteredUI();
    }
}

function initAuthControls() {
    const authButton = document.getElementById('admin-auth-button');
    const authModal = document.getElementById('auth-modal');
    const authForm = document.getElementById('auth-login-form');
    const authFeedback = document.getElementById('auth-feedback');
    const authUsername = document.getElementById('auth-username');
    const authPassword = document.getElementById('auth-password');
    const authSubmitButton = document.getElementById('auth-submit-button');

    if (authButton) {
        authButton.addEventListener('click', async () => {
            if (!adminAuthState.configured) {
                return;
            }

            if (adminAuthState.authenticated) {
                try {
                    const response = await fetch(API_ENDPOINTS.authLogout, { method: 'POST' });
                    if (!response.ok) {
                        throw new Error(`HTTP ${response.status}`);
                    }
                    await refreshAuthStatus();
                } catch (error) {
                    console.error('Failed to logout admin:', error);
                }
                return;
            }

            if (authModal) {
                authModal.hidden = false;
                syncOverlayState();
            }
            if (authFeedback) {
                authFeedback.textContent = '';
            }
            if (authUsername) {
                authUsername.focus();
            }
        });
    }

    document.addEventListener('click', (event) => {
        const closer = event.target.closest('[data-close-auth-modal]');
        if (!closer) {
            return;
        }
        closeAuthModal();
    });

    if (authForm) {
        authForm.addEventListener('submit', async (event) => {
            event.preventDefault();
            if (!authUsername || !authPassword) {
                return;
            }

            if (authSubmitButton) {
                authSubmitButton.disabled = true;
            }
            if (authFeedback) {
                authFeedback.textContent = '登录中...';
            }

            try {
                const response = await fetch(API_ENDPOINTS.authLogin, {
                    method: 'POST',
                    headers: {
                        'Content-Type': 'application/json'
                    },
                    body: JSON.stringify({
                        username: authUsername.value.trim(),
                        password: authPassword.value
                    })
                });
                if (!response.ok) {
                    const message = await response.text();
                    throw new Error(message.trim() || '登录失败');
                }

                closeAuthModal();
                authPassword.value = '';
                await refreshAuthStatus();
            } catch (error) {
                if (authFeedback) {
                    authFeedback.textContent = `登录失败: ${error.message}`;
                }
            } finally {
                if (authSubmitButton) {
                    authSubmitButton.disabled = false;
                }
            }
        });
    }
}

function closeAuthModal() {
    const authModal = document.getElementById('auth-modal');
    if (authModal) {
        authModal.hidden = true;
    }
    syncOverlayState();
}

function openManagedTeamModal() {
    const modal = document.getElementById('managed-team-modal');
    const nameInput = document.getElementById('managed-team-name');
    const feedback = document.getElementById('managed-team-feedback');
    if (!modal) {
        return;
    }

    modal.hidden = false;
    if (feedback) {
        feedback.textContent = '';
    }
    syncOverlayState();

    if (nameInput instanceof HTMLInputElement) {
        window.setTimeout(() => nameInput.focus(), 0);
    }
}

function closeManagedTeamModal() {
    const modal = document.getElementById('managed-team-modal');
    if (modal) {
        modal.hidden = true;
    }
    syncOverlayState();
}

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

function initControlWorkspace() {
    document.addEventListener('click', (event) => {
        const selector = event.target.closest('[data-control-select]');
        if (!selector) {
            return;
        }

        selectedTeamName = selector.getAttribute('data-control-team-name') || null;
        const scope = selector.getAttribute('data-control-scope') || 'team';
        selectedAgentKey = scope === 'agent'
            ? (selector.getAttribute('data-control-agent-key') || TEAM_OVERVIEW_KEY)
            : TEAM_OVERVIEW_KEY;
        rerenderControlWorkspace();
    });

    document.addEventListener('input', (event) => {
        const textarea = event.target.closest('[data-control-composer-input]');
        if (!(textarea instanceof HTMLTextAreaElement)) {
            return;
        }

        const selectionKey = textarea.getAttribute('data-control-selection-key') || '';
        if (!selectionKey) {
            return;
        }

        controlComposerDrafts[selectionKey] = textarea.value;
        controlComposerFeedback[selectionKey] = '';
    });

    document.addEventListener('compositionstart', (event) => {
        const textarea = event.target.closest('[data-control-composer-input]');
        if (!(textarea instanceof HTMLTextAreaElement)) {
            return;
        }

        controlComposerIsComposing = true;
    });

    document.addEventListener('compositionend', (event) => {
        const textarea = event.target.closest('[data-control-composer-input]');
        if (!(textarea instanceof HTMLTextAreaElement)) {
            return;
        }

        const selectionKey = textarea.getAttribute('data-control-selection-key') || '';
        if (selectionKey) {
            controlComposerDrafts[selectionKey] = textarea.value;
        }
        controlComposerIsComposing = false;
        flushDeferredControlPanelRender();
    });

    document.addEventListener('keydown', (event) => {
        const textarea = event.target.closest('[data-control-composer-input]');
        if (!(textarea instanceof HTMLTextAreaElement)) {
            return;
        }

        if (event.key === 'Enter' && !event.shiftKey && !event.isComposing && !controlComposerIsComposing) {
            event.preventDefault();
            textarea.closest('[data-control-composer-form]')?.requestSubmit();
        }
    });

    document.addEventListener('submit', async (event) => {
        const form = event.target.closest('[data-control-composer-form]');
        if (!form) {
            return;
        }

        event.preventDefault();
        await submitControlComposer(form);
    });
}

function rerenderControlWorkspace() {
    if (!previousState) {
        return;
    }

    updateTeams(previousState.teams || []);
}

function flushDeferredControlPanelRender() {
    if (!deferredTeamsRender) {
        return;
    }

    const nextTeams = deferredTeamsRender;
    deferredTeamsRender = null;
    updateTeams(nextTeams);
}

function shouldDeferControlPanelRender() {
    const activeElement = document.activeElement;
    return controlComposerIsComposing &&
        activeElement instanceof HTMLTextAreaElement &&
        activeElement.matches('[data-control-composer-input]');
}

function captureControlComposerState() {
    const activeElement = document.activeElement;
    if (!(activeElement instanceof HTMLTextAreaElement) || !activeElement.matches('[data-control-composer-input]')) {
        return null;
    }

    const selectionKey = activeElement.getAttribute('data-control-selection-key') || '';
    if (selectionKey) {
        controlComposerDrafts[selectionKey] = activeElement.value;
    }

    return {
        selectionKey,
        selectionStart: activeElement.selectionStart,
        selectionEnd: activeElement.selectionEnd,
        scrollTop: activeElement.scrollTop,
    };
}

function selectorEscape(value) {
    if (window.CSS && typeof window.CSS.escape === 'function') {
        return window.CSS.escape(value);
    }

    return String(value || '').replace(/["\\]/g, '\\$&');
}

function restoreControlComposerState(snapshot) {
    if (!snapshot || !snapshot.selectionKey) {
        return;
    }

    window.requestAnimationFrame(() => {
        const textarea = document.querySelector(
            `[data-control-composer-input][data-control-selection-key="${selectorEscape(snapshot.selectionKey)}"]`
        );
        if (!(textarea instanceof HTMLTextAreaElement)) {
            return;
        }

        textarea.focus({ preventScroll: true });
        textarea.selectionStart = snapshot.selectionStart;
        textarea.selectionEnd = snapshot.selectionEnd;
        textarea.scrollTop = snapshot.scrollTop;
    });
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
        let managedTeams = [];

        if (IS_DESKTOP_MODE && DESKTOP_BRIDGE) {
            data = await DESKTOP_BRIDGE.fetchDesktopState();
            const managedResponse = await fetch(`${API_ENDPOINTS.managedTeams}?_ts=${Date.now()}`, {
                cache: 'no-store'
            });
            if (managedResponse.ok) {
                managedTeams = await managedResponse.json();
            }
        } else {
            const [response, managedResponse] = await Promise.all([
                fetch(`${API_ENDPOINTS.state}?_ts=${Date.now()}`, {
                    cache: 'no-store',
                    headers: {
                        'Cache-Control': 'no-cache'
                    }
                }),
                fetch(`${API_ENDPOINTS.managedTeams}?_ts=${Date.now()}`, {
                    cache: 'no-store'
                })
            ]);
            if (!response.ok) {
                throw new Error(`HTTP error! status: ${response.status}`);
            }
            data = await response.json();
            if (managedResponse.ok) {
                managedTeams = await managedResponse.json();
            }
        }

        updateUI(data, managedTeams);
        updateConnectionStatus(true);
    } catch (error) {
        console.error('Error fetching data:', error);
        updateConnectionStatus(false);
    }
}

// Update UI with new data (智能更新，只更新变化的部分)
function updateUI(data, managedTeams = []) {
    latestRawState = data;
    latestManagedTeams = Array.isArray(managedTeams) ? managedTeams : [];
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

    updateManagedTeams(latestManagedTeams);
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

function initManagedTeamControls() {
    const form = document.getElementById('managed-team-form');
    const openButton = document.getElementById('managed-team-open-modal');
    if (!form) {
        return;
    }

    if (openButton) {
        openButton.addEventListener('click', () => {
            if (!isAdminAuthenticated()) {
                setManagedTeamFeedback('管理员登录后才能创建团队');
                return;
            }
            openManagedTeamModal();
        });
    }

    document.addEventListener('click', (event) => {
        const closer = event.target.closest('[data-close-managed-team-modal]');
        if (!closer) {
            return;
        }
        closeManagedTeamModal();
    });

    document.addEventListener('keydown', (event) => {
        if (event.key !== 'Escape') {
            return;
        }
        if (isModalVisible('managed-team-modal')) {
            closeManagedTeamModal();
        }
    });

    form.addEventListener('submit', async (event) => {
        event.preventDefault();
        if (!isAdminAuthenticated()) {
            setManagedTeamFeedback('管理员登录后才能创建团队');
            return;
        }

        const nameInput = document.getElementById('managed-team-name');
        const workspaceInput = document.getElementById('managed-team-workspace');
        const modelInput = document.getElementById('managed-team-model');
        const permissionSelect = document.getElementById('managed-team-permission');
        const autostartToggle = document.getElementById('managed-team-autostart');
        const initialTaskInput = document.getElementById('managed-team-initial-task');
        const submitButton = document.getElementById('managed-team-submit');

        if (!nameInput || !workspaceInput) {
            return;
        }

        submitButton.disabled = true;
        setManagedTeamFeedback('创建中...');

        try {
            const response = await fetch(API_ENDPOINTS.managedTeams, {
                method: 'POST',
                headers: {
                    'Content-Type': 'application/json'
                },
                body: JSON.stringify({
                    name: nameInput.value.trim(),
                    provider: 'claude',
                    workspace: workspaceInput.value.trim(),
                    model: modelInput ? modelInput.value.trim() : '',
                    permission: permissionSelect ? permissionSelect.value : ''
                })
            });
            if (!response.ok) {
                const message = await response.text();
                throw new Error(message.trim() || '创建失败');
            }

            const created = await response.json();

            if (autostartToggle && autostartToggle.checked && created?.id) {
                const startResponse = await fetch(`${API_ENDPOINTS.managedTeams}/${encodeURIComponent(created.id)}/start`, {
                    method: 'POST'
                });
                if (!startResponse.ok) {
                    const message = await startResponse.text();
                    throw new Error(message.trim() || '启动失败');
                }

                const initialTask = initialTaskInput ? initialTaskInput.value.trim() : '';
                if (initialTask) {
                    const messageResponse = await fetch(`${API_ENDPOINTS.managedTeams}/${encodeURIComponent(created.id)}/message`, {
                        method: 'POST',
                        headers: {
                            'Content-Type': 'application/json'
                        },
                        body: JSON.stringify({ text: initialTask })
                    });
                    if (!messageResponse.ok) {
                        const message = await messageResponse.text();
                        throw new Error(message.trim() || '首条任务发送失败');
                    }
                }
            }

            nameInput.value = '';
            workspaceInput.value = '';
            if (modelInput) {
                modelInput.value = '';
            }
            if (permissionSelect) {
                permissionSelect.value = '';
            }
            if (initialTaskInput) {
                initialTaskInput.value = '';
            }
            setManagedTeamFeedback('团队已创建' + (autostartToggle && autostartToggle.checked ? '并启动' : ''));
            closeManagedTeamModal();
            await fetchData();
        } catch (error) {
            setManagedTeamFeedback(`创建失败: ${error.message}`);
        } finally {
            submitButton.disabled = false;
        }
    });

    document.addEventListener('click', async (event) => {
        const actionButton = event.target.closest('[data-managed-action]');
        if (!actionButton) {
            return;
        }
        if (!isAdminAuthenticated()) {
            setManagedTeamFeedback('管理员登录后才能管理自托管团队');
            return;
        }

        const teamID = actionButton.getAttribute('data-managed-team-id');
        const agentID = actionButton.getAttribute('data-managed-agent-id') || '';
        const action = actionButton.getAttribute('data-managed-action');
        const label = actionButton.getAttribute('data-managed-label') || '受管会话';
        const feedbackNode = actionButton.closest('[data-managed-feedback-scope]')?.querySelector('[data-managed-feedback]') || null;
        if (!teamID || !action) {
            return;
        }

        const actionText = formatManagedActionText(action);
        const endpoint = agentID
            ? `${API_ENDPOINTS.managedTeams}/${encodeURIComponent(teamID)}/agents/${encodeURIComponent(agentID)}/${action}`
            : `${API_ENDPOINTS.managedTeams}/${encodeURIComponent(teamID)}/${action}`;
        actionButton.disabled = true;
        setManagedActionFeedback(feedbackNode, `${label} ${actionText}中...`);
        try {
            const response = await fetch(endpoint, { method: 'POST' });
            if (!response.ok) {
                const message = await response.text();
                throw new Error(message.trim() || `${action} 失败`);
            }
            setManagedActionFeedback(feedbackNode, `${label} 已${actionText}`);
            await fetchData();
        } catch (error) {
            setManagedActionFeedback(feedbackNode, `${label} ${actionText}失败: ${error.message}`);
        } finally {
            actionButton.disabled = false;
        }
    });

    document.addEventListener('submit', async (event) => {
        const messageForm = event.target.closest('[data-managed-message-form]');
        if (!messageForm) {
            return;
        }
        event.preventDefault();
        if (!isAdminAuthenticated()) {
            setManagedTeamFeedback('管理员登录后才能下发任务');
            return;
        }

        const teamID = messageForm.getAttribute('data-managed-team-id');
        const agentID = messageForm.getAttribute('data-managed-agent-id');
        const textarea = messageForm.querySelector('textarea[name="managed-team-message"]');
        const submitButton = messageForm.querySelector('button[type="submit"]');
        const text = textarea ? textarea.value.trim() : '';
        if (!teamID || !text) {
            return;
        }

        submitButton.disabled = true;
        try {
            const response = await fetch(`${API_ENDPOINTS.managedTeams}/${encodeURIComponent(teamID)}/message`, {
                method: 'POST',
                headers: {
                    'Content-Type': 'application/json'
                },
                body: JSON.stringify({ text, agent_id: agentID || undefined })
            });
            if (!response.ok) {
                const message = await response.text();
                throw new Error(message.trim() || '发送失败');
            }
            textarea.value = '';
            setManagedTeamFeedback('首条任务已写入受管会话');
            await fetchData();
        } catch (error) {
            setManagedTeamFeedback(`发送失败: ${error.message}`);
        } finally {
            submitButton.disabled = false;
        }
    });
}

function setManagedTeamFeedback(message) {
    const feedback = document.getElementById('managed-team-feedback');
    if (feedback) {
        feedback.textContent = message || '';
    }
}

function setManagedActionFeedback(feedbackNode, message) {
    if (feedbackNode) {
        feedbackNode.textContent = message || '';
    }
    setManagedTeamFeedback(message);
}

async function sendAgentMessageRequest({ teamName = '', agentName = '', transport = '', managedTeamID = '', managedAgentID = '', text = '' }) {
    const normalizedText = String(text || '').trim();
    if (!normalizedText) {
        throw new Error('请输入消息');
    }

    if (transport === 'managed_pty' && managedTeamID) {
        const response = await fetch(`${API_ENDPOINTS.managedTeams}/${encodeURIComponent(managedTeamID)}/message`, {
            method: 'POST',
            headers: {
                'Content-Type': 'application/json'
            },
            body: JSON.stringify({ text: normalizedText, agent_id: managedAgentID || undefined })
        });
        if (!response.ok) {
            const message = (await response.text()) || '发送失败';
            throw new Error(message.trim());
        }

    return {
        confirmation: managedAgentID ? '已写入受管成员会话' : '已写入受管主成员会话'
    };
}

    if (!agentName) {
        throw new Error('当前团队视图不支持直接发送，请先选择一个具体成员');
    }

    if (IS_DESKTOP_MODE && DESKTOP_BRIDGE?.sendDesktopAgentMessage) {
        await DESKTOP_BRIDGE.sendDesktopAgentMessage(teamName, agentName, normalizedText);
        return {
            confirmation: '已写入成员收件箱，等待成员读取'
        };
    }

    const response = await fetch(API_ENDPOINTS.agentMessage, {
        method: 'POST',
        headers: {
            'Content-Type': 'application/json'
        },
        body: JSON.stringify({
            team_name: teamName,
            agent_name: agentName,
            text: normalizedText
        })
    });

    if (!response.ok) {
        const message = (await response.text()) || '发送失败';
        throw new Error(message.trim());
    }

    return {
        confirmation: '已写入成员收件箱，等待成员读取'
    };
}

function updateManagedTeams(teams) {
    const openButton = document.getElementById('managed-team-open-modal');
    const submitButton = document.getElementById('managed-team-submit');
    const nameInput = document.getElementById('managed-team-name');
    const workspaceInput = document.getElementById('managed-team-workspace');
    const modelInput = document.getElementById('managed-team-model');
    const permissionSelect = document.getElementById('managed-team-permission');
    const autostartToggle = document.getElementById('managed-team-autostart');
    const initialTaskInput = document.getElementById('managed-team-initial-task');
    const controlsEnabled = isAdminAuthenticated();

    [openButton, submitButton, nameInput, workspaceInput, modelInput, permissionSelect, autostartToggle, initialTaskInput].forEach((element) => {
        if (element) {
            element.disabled = !controlsEnabled;
        }
    });
}

function renderManagedTeamCard(item) {
    const spec = item.spec || {};
    const run = item.run || null;
    const status = String((run && run.status) || 'stopped');
    const statusLabel = formatManagedRunStatus(status);
    const running = status === 'running' || status === 'running_detached';
    const controllable = Boolean(run && run.controllable);
    const disabled = !isAdminAuthenticated();
    const workspace = spec.workspace || '';
    const model = (spec.agents && spec.agents[0] && spec.agents[0].model) || '';

    return `
        <div class="team-card managed-team-card">
            <div class="team-header">
                <div class="team-header-left">
                    <div class="team-name">${escapeHtml(spec.name || spec.id || '未命名团队')} <span class="agent-type">[受管]</span></div>
                    <div class="team-created">标识：${escapeHtml(spec.id || '')}</div>
                    ${workspace ? `<div class="team-cwd">工作目录: ${escapeHtml(workspace)}</div>` : ''}
                    ${model ? `<div class="team-created">模型: ${escapeHtml(model)}</div>` : ''}
                </div>
                <div class="managed-team-actions">
                    <button class="team-delete-btn" type="button" data-managed-action="start" data-managed-team-id="${escapeHtml(spec.id || '')}" ${running || disabled ? 'disabled' : ''}>启动</button>
                    <button class="team-delete-btn" type="button" data-managed-action="stop" data-managed-team-id="${escapeHtml(spec.id || '')}" ${!running || !controllable || disabled ? 'disabled' : ''}>停止</button>
                </div>
            </div>
            <div class="team-summary-bar">
                <div class="team-summary-item">
                    <span class="team-summary-label">状态</span>
                    <span class="team-summary-value">${escapeHtml(statusLabel)}</span>
                </div>
                <div class="team-summary-item">
                    <span class="team-summary-label">进程</span>
                    <span class="team-summary-value">${escapeHtml(String((run && run.pid) || '-'))}</span>
                </div>
                <div class="team-summary-item">
                    <span class="team-summary-label">可控</span>
                    <span class="team-summary-value">${controllable ? '是' : '否'}</span>
                </div>
            </div>
            <form class="managed-message-form" data-managed-message-form="true" data-managed-team-id="${escapeHtml(spec.id || '')}">
                <textarea name="managed-team-message" class="agent-command-input" rows="3" placeholder="给受管主成员下发首条任务" ${!running || !controllable || disabled ? 'disabled' : ''}></textarea>
                <div class="agent-command-actions">
                    <button type="submit" class="agent-command-submit" ${!running || !controllable || disabled ? 'disabled' : ''}>下发任务</button>
                    <span class="agent-command-feedback">${run && run.last_error ? escapeHtml(run.last_error) : ''}</span>
                </div>
            </form>
        </div>
    `;
}

function isManagedAgent(team, agent) {
    return Boolean(team?.managed && normalizeLookupToken(team?.managed_team_id) && normalizeLookupToken(agent?.agent_id));
}

function isManagedAgentControllable(agent) {
    return String(agent?.command_transport || '') === 'managed_pty';
}

function isManagedAgentRunning(agent) {
    return String(agent?.status || '').toLowerCase() === 'working';
}

function countManagedRunningMembers(members) {
    return (Array.isArray(members) ? members : []).filter(member => isManagedAgentRunning(member)).length;
}

function countManagedControllableMembers(members) {
    return (Array.isArray(members) ? members : []).filter(member => isManagedAgentControllable(member)).length;
}

function describeManagedAgentControlState(agent) {
    if (isManagedAgentControllable(agent)) {
        return '会话在线，可直接停止或发消息';
    }

    if (isManagedAgentRunning(agent)) {
        return '会话在线，但当前控制进程不可用';
    }

    const reason = normalizeMultilineText(agent?.command_reason || '');
    if (reason.includes('尚未启动')) {
        return '会话未启动，可单独启动';
    }
    if (reason && !reason.includes('已停止或已脱离当前控制进程')) {
        return reason;
    }

    return '会话已停止，可单独启动';
}

function managedAgentControlTone(agent) {
    if (isManagedAgentControllable(agent)) {
        return 'controllable';
    }
    if (isManagedAgentRunning(agent)) {
        return 'detached';
    }
    return 'stopped';
}

function formatManagedActionText(action) {
    switch (String(action || '').toLowerCase()) {
        case 'start':
            return '启动';
        case 'stop':
            return '停止';
        case 'message':
            return '发送';
        default:
            return action || '操作';
    }
}

function renderManagedAgentActionButtons(team, agent, controlsEnabled) {
    const teamID = team?.managed_team_id || '';
    const agentID = agent?.agent_id || '';
    const running = isManagedAgentRunning(agent);
    const controllable = isManagedAgentControllable(agent);

    return `
        <div class="managed-team-actions">
            <button class="team-delete-btn start" type="button" data-managed-action="start" data-managed-team-id="${escapeHtml(teamID)}" data-managed-agent-id="${escapeHtml(agentID)}" data-managed-label="${escapeHtml(team?.name || '受管团队')} / ${escapeHtml(agent?.name || agentID || '成员')}" ${!controlsEnabled || !teamID || !agentID || running ? 'disabled' : ''}>启动</button>
            <button class="team-delete-btn stop" type="button" data-managed-action="stop" data-managed-team-id="${escapeHtml(teamID)}" data-managed-agent-id="${escapeHtml(agentID)}" data-managed-label="${escapeHtml(team?.name || '受管团队')} / ${escapeHtml(agent?.name || agentID || '成员')}" ${!controlsEnabled || !teamID || !agentID || !controllable ? 'disabled' : ''}>停止</button>
        </div>
    `;
}

function renderTeamActions(team, provider, canDelete) {
    if (team && team.managed) {
        const members = Array.isArray(team.members) ? team.members : [];
        const runningCount = countManagedRunningMembers(members);
        const controllableCount = countManagedControllableMembers(members);
        const disabled = !isAdminAuthenticated();
        return `
            <div class="managed-team-actions">
                <button class="team-delete-btn start" type="button" data-managed-action="start" data-managed-team-id="${escapeHtml(team.managed_team_id || '')}" data-managed-label="${escapeHtml(team.name || '受管团队')} 全部成员" ${members.length === 0 || runningCount >= members.length || disabled ? 'disabled' : ''}>全部启动</button>
                <button class="team-delete-btn stop" type="button" data-managed-action="stop" data-managed-team-id="${escapeHtml(team.managed_team_id || '')}" data-managed-label="${escapeHtml(team.name || '受管团队')} 全部成员" ${controllableCount === 0 || disabled ? 'disabled' : ''}>全部停止</button>
            </div>
        `;
    }

    if (canDelete) {
        return `<button class="team-delete-btn danger" onclick="deleteTeam('${escapeHtml(team.name)}')" title="清理团队"><svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M3 6h18"/><path d="M19 6v14c0 1-1 2-2 2H7c-1 0-2-1-2-2V6"/><path d="M8 6V4c0-1 1-2 2-2h4c0 1 2 1 2 2v2"/></svg> 清理</button>`;
    }

    return '';
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
        claudeCount.textContent = `(团队:${counts.claude.teams},成员:${counts.claude.agents})`;
    }

    const codexCount = document.getElementById('codex-filter-count');
    if (codexCount) {
        codexCount.textContent = `(团队:${counts.codex.teams},成员:${counts.codex.agents})`;
    }

    const openClawCount = document.getElementById('openclaw-filter-count');
    if (openClawCount) {
        openClawCount.textContent = `(团队:${counts.openclaw.teams},成员:${counts.openclaw.agents})`;
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

function providerDisplayName(provider) {
    switch (String(provider || '').toLowerCase()) {
        case 'claude': return 'Claude';
        case 'codex': return 'Codex';
        case 'openclaw': return 'OpenClaw';
        default: return '团队';
    }
}

function containsChineseText(value) {
    return /[\u4e00-\u9fff]/.test(String(value || ''));
}

function formatMappedLabel(value, dictionary, fallback = '') {
    const normalized = String(value || '').trim();
    if (!normalized) {
        return fallback;
    }

    if (containsChineseText(normalized)) {
        return normalized;
    }

    const key = normalized.toLowerCase();
    if (dictionary[key]) {
        return dictionary[key];
    }

    const tokens = key.split(/[_\-\s/]+/).filter(Boolean);
    if (tokens.length > 1) {
        const translated = tokens.map((token) => dictionary[token] || '');
        if (translated.every(Boolean)) {
            return translated.join(' / ');
        }
    }

    return fallback || normalized;
}

function formatManagedRunStatus(status) {
    return formatMappedLabel(status, STATE_LABELS, '未知');
}

function formatAgentTypeLabel(agentType) {
    return formatMappedLabel(agentType, ROLE_LABELS, '成员');
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
    const detectedProvider = detectProcessProvider(process);
    const provider = detectedProvider === 'unknown' ? '未知' : providerDisplayName(detectedProvider);
    return `
        <div class="process-item">
            <div class="process-info">
                <span class="process-pid">进程 ${process.pid}</span>
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
    const lookup = buildTeamDetailLookup(teams);

    if (nav) {
        nav.innerHTML = '';
        nav.style.display = 'none';
    }

    if (shouldDeferControlPanelRender()) {
        deferredTeamsRender = teams;
        syncAgentDetailModal(lookup);
        return;
    }

    const focusSnapshot = captureControlComposerState();
    ensureControlSelection(teams);

    if (teams.length === 0) {
        selectedTeamName = null;
        selectedAgentKey = null;
        container.innerHTML = '<p class="empty-state"><svg viewBox="0 0 24 24" width="32" height="32" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round"><path d="M16 21v-2a4 4 0 0 0-4-4H6a4 4 0 0 0-4 4v2"/><circle cx="9" cy="7" r="4"/><path d="M22 21v-2a4 4 0 0 0-3-3.87"/><path d="M16 3.13a4 4 0 0 1 0 7.75"/></svg>当前筛选下未找到活动团队</p>';
        syncAgentDetailModal(lookup);
        return;
    }

    const context = resolveControlSelection(teams);
    container.innerHTML = renderControlWorkspace(teams, context);
    syncAgentDetailModal(lookup);
    restoreControlComposerState(focusSnapshot);
}

function buildTeamControlKey(team) {
    return `${team?.name || 'unknown'}::team-overview`;
}

function ensureControlSelection(teams) {
    if (!Array.isArray(teams) || teams.length === 0) {
        selectedTeamName = null;
        selectedAgentKey = null;
        return;
    }

    let team = teams.find(item => item.name === selectedTeamName);
    if (!team) {
        team = teams[0];
        selectedTeamName = team.name;
        selectedAgentKey = null;
    }

    const members = Array.isArray(team.members) ? team.members : [];
    if (selectedAgentKey === TEAM_OVERVIEW_KEY) {
        return;
    }

    if (!selectedAgentKey) {
        selectedAgentKey = members.length > 0 ? buildAgentDetailKey(team, members[0]) : TEAM_OVERVIEW_KEY;
        return;
    }

    const hasSelectedAgent = members.some(agent => buildAgentDetailKey(team, agent) === selectedAgentKey);
    if (!hasSelectedAgent) {
        selectedAgentKey = members.length > 0 ? buildAgentDetailKey(team, members[0]) : TEAM_OVERVIEW_KEY;
    }
}

function resolveControlSelection(teams) {
    const team = (teams || []).find(item => item.name === selectedTeamName) || null;
    if (!team) {
        return null;
    }

    const members = Array.isArray(team.members) ? team.members : [];
    const teamTasks = Array.isArray(team.tasks) ? team.tasks : [];
    const { tasksByOwner, unassignedTasks } = groupTasksByOwner(members, teamTasks);
    const agent = selectedAgentKey && selectedAgentKey !== TEAM_OVERVIEW_KEY
        ? members.find(item => buildAgentDetailKey(team, item) === selectedAgentKey) || null
        : null;

    return {
        team,
        agent,
        members,
        provider: detectTeamProvider(team),
        teamTasks,
        tasksByOwner,
        unassignedTasks,
        tasks: agent ? (tasksByOwner[agent.name] || []) : teamTasks,
        selectionKey: agent ? buildAgentDetailKey(team, agent) : buildTeamControlKey(team),
        teamKey: buildTeamControlKey(team)
    };
}

function renderControlWorkspace(teams, context) {
    return `
        <div class="control-workspace">
            ${renderControlSidebar(teams, context)}
            <section class="control-main">
                ${context ? renderControlMain(context) : '<p class="empty-state">请选择一个团队</p>'}
            </section>
        </div>
    `;
}

function renderControlSidebar(teams, context) {
    const totalAgents = (teams || []).reduce((sum, team) => sum + ((team?.members || []).length), 0);
    const activeTeams = (teams || []).filter(team => (team?.members || []).some(agent => isAgentInMotion(agent))).length;

    return `
        <aside class="control-sidebar">
            <div class="control-sidebar-header">
                <h2>团队总览</h2>
                <p>${teams.length} 支团队 · ${totalAgents} 个可见成员 · ${activeTeams} 支团队仍在流动</p>
            </div>
            <div class="control-sidebar-scroll">
                ${teams.map(team => renderControlSidebarTeam(team, context)).join('')}
            </div>
        </aside>
    `;
}

function renderControlSidebarTeam(team, context) {
    const members = Array.isArray(team.members) ? team.members : [];
    const tasks = Array.isArray(team.tasks) ? team.tasks : [];
    const provider = detectTeamProvider(team);
    const providerLabel = providerDisplayName(provider);
    const activeCount = members.filter(agent => isAgentInMotion(agent)).length;
    const pendingTaskCount = tasks.filter(task => String(task?.status || '').toLowerCase() !== 'completed').length;
    const runningCount = team.managed ? countManagedRunningMembers(members) : members.filter(agent => String(agent?.status || '').toLowerCase() === 'working').length;
    const { tasksByOwner } = groupTasksByOwner(members, tasks);
    const expanded = context?.team?.name === team.name;
    const teamSelected = expanded && !context?.agent;

    return `
        <section class="control-team-rail ${expanded ? 'active' : ''}">
            <button
                type="button"
                class="control-team-card ${teamSelected ? 'selected' : ''}"
                data-control-select="true"
                data-control-scope="team"
                data-control-team-name="${escapeHtml(team.name)}"
            >
                <div class="control-team-card-top">
                    <span class="control-provider-pill">${escapeHtml(providerLabel)}</span>
                    ${team.managed ? '<span class="control-provider-pill managed">受管</span>' : '<span class="control-provider-pill imported">导入</span>'}
                </div>
                <div class="control-team-card-name">${escapeHtml(team.name)}</div>
                <div class="control-team-card-meta">${members.length} 个成员 · ${pendingTaskCount} 项待处理</div>
                <div class="control-team-card-stats">
                    <span>${runningCount} 运行中</span>
                    <span>${activeCount} 活跃</span>
                </div>
            </button>
            ${expanded ? `
                <div class="control-agent-list">
                    <button
                        type="button"
                        class="control-agent-nav control-agent-overview ${teamSelected ? 'selected' : ''}"
                        data-control-select="true"
                        data-control-scope="team"
                        data-control-team-name="${escapeHtml(team.name)}"
                    >
                        <span class="control-agent-marker">全</span>
                        <span class="control-agent-copy">
                            <span class="control-agent-name">全部活动</span>
                            <span class="control-agent-meta">团队广播、任务板和全部成员动态</span>
                        </span>
                    </button>
                    ${members.map(agent => renderControlSidebarAgent(team, agent, tasksByOwner[agent.name] || [])).join('')}
                    ${members.length === 0 ? '<div class="control-agent-empty">当前筛选下没有可见成员</div>' : ''}
                </div>
            ` : ''}
        </section>
    `;
}

function renderControlSidebarAgent(team, agent, tasks) {
    const key = buildAgentDetailKey(team, agent);
    const selected = team.name === selectedTeamName && selectedAgentKey === key;
    const primarySignal = buildAgentPrimarySignal(agent, tasks);
    const meta = primarySignal
        ? truncateMultiline(primarySignal.value, 50)
        : buildAgentMetaSummary(agent, tasks, isAgentInMotion(agent));

    return `
        <button
            type="button"
            class="control-agent-nav ${selected ? 'selected' : ''}"
            data-control-select="true"
            data-control-scope="agent"
            data-control-team-name="${escapeHtml(team.name)}"
            data-control-agent-key="${escapeHtml(key)}"
        >
            <span class="control-agent-marker ${escapeHtml(String(agent?.status || 'idle').toLowerCase())}">${escapeHtml(getRoleIcon(agent))}</span>
            <span class="control-agent-copy">
                <span class="control-agent-name">${escapeHtml(agent.name || '未命名成员')}</span>
                <span class="control-agent-meta">${escapeHtml(meta)}</span>
            </span>
        </button>
    `;
}

function renderControlMain(context) {
    const feedItems = buildConversationFeed(context);
    return `
        <div class="control-main-shell">
            ${renderControlHero(context)}
            <section class="control-feed-panel">
                <div class="control-section-bar">
                    <h3>当前活动流</h3>
                    <div class="control-section-meta">${feedItems.length} 条活动</div>
                </div>
                <div class="control-feed-scroll">
                    ${renderConversationFeed(feedItems)}
                </div>
            </section>
            ${renderControlComposer(context)}
        </div>
    `;
}

function renderControlHero(context) {
    const { team, agent, provider, tasks, members, teamTasks } = context;
    const providerLabel = providerDisplayName(provider);
    const statusClass = agent ? String(agent?.status || 'idle').toLowerCase() : '';
    const canDelete = !team.managed && provider !== 'codex' && isAdminAuthenticated();
    const actionMarkup = agent && isManagedAgent(team, agent)
        ? renderManagedAgentActionButtons(team, agent, isAdminAuthenticated())
        : renderTeamActions(team, provider, canDelete);

    return `
        <section class="control-hero" ${actionMarkup ? 'data-managed-feedback-scope="true"' : ''}>
            <div class="control-hero-top">
                <div class="control-hero-copy">
                    <div class="control-hero-title-row">
                        <div>
                            <h2>${escapeHtml(agent ? agent.name : team.name)}</h2>
                            <p>${escapeHtml(agent ? `${team.name} · ${formatAgentTypeLabel(agent.agent_type)}` : `${providerLabel} · ${team.managed ? '受管团队' : '导入团队'}`)}</p>
                        </div>
                        <div class="control-hero-badges">
                            <span class="control-provider-pill ${team.managed ? 'managed' : 'imported'}">${team.managed ? '受管' : providerLabel}</span>
                            ${agent ? `<span class="agent-status ${escapeHtml(statusClass)}">${escapeHtml(formatAgentStatus(agent.status))}</span>` : `<span class="control-provider-pill subtle">${escapeHtml(`${members.length} 个成员`)}</span>`}
                        </div>
                    </div>
                    <div class="control-hero-path">
                        ${agent?.cwd ? escapeHtml(agent.cwd) : team.project_cwd ? escapeHtml(team.project_cwd) : '未提供工作目录'}
                    </div>
                </div>
                ${actionMarkup ? `
                    <div class="control-hero-actions">
                        ${actionMarkup}
                        <span class="agent-command-feedback control-action-feedback" data-managed-feedback></span>
                    </div>
                ` : ''}
            </div>
            <div class="control-stat-grid">
                ${renderControlStatCards(context)}
            </div>
            <div class="control-detail-grid">
                ${agent ? renderControlAgentDetail(team, agent, tasks) : renderControlTeamDetail(team, teamTasks, members)}
            </div>
        </section>
    `;
}

function renderControlStatCards(context) {
    const { team, agent, provider, tasks, members, teamTasks } = context;
    if (agent) {
        const lastActive = formatRelativeTime(agent.last_active_time || agent.last_message_time || agent.last_activity) || '暂无活动';
        const transport = isManagedAgent(team, agent)
            ? '受管终端'
            : agent.command_transport === 'claude_inbox'
                ? 'Claude 收件箱'
                : (agent.command_reason ? '只读' : '未知');
        const todos = Array.isArray(agent.todos) ? agent.todos.length : 0;
        const statusMeta = isManagedAgent(team, agent)
            ? describeManagedAgentControlState(agent)
            : (agent.command_reason || buildAgentMetaSummary(agent, tasks, isAgentInMotion(agent)));
        const transportMeta = describeAgentTransportState(team, agent);
        return [
            renderControlStatCard('状态', formatAgentStatus(agent.status), statusMeta),
            renderControlStatCard('通道', transport, transportMeta),
            renderControlStatCard('最近活动', lastActive, agent.current_task ? truncateMultiline(agent.current_task, 40) : '暂无当前任务'),
            renderControlStatCard('任务 / 待办', `${tasks.length} / ${todos}`, tasks[0] ? truncateMultiline(tasks[0].subject || '', 40) : '暂无挂起任务')
        ].join('');
    }

    const pendingTasks = teamTasks.filter(task => String(task?.status || '').toLowerCase() !== 'completed');
    const activeMembers = members.filter(member => isAgentInMotion(member)).length;
    return [
        renderControlStatCard('来源', providerDisplayName(provider), team.managed ? '由系统直接托管' : '由外部数据导入'),
        renderControlStatCard('成员', String(members.length), `${activeMembers} 个仍在活动`),
        renderControlStatCard('待处理任务', String(pendingTasks.length), pendingTasks[0] ? truncateMultiline(pendingTasks[0].subject || '', 40) : '当前任务板为空'),
        renderControlStatCard('操控模式', team.managed ? '团队可控' : '按成员控制', team.managed ? '未选成员时将发给团队主成员' : '请选择具体成员后发送')
    ].join('');
}

function renderControlStatCard(label, value, meta) {
    return `
        <div class="control-stat-card">
            <div class="control-stat-main">
                <div class="control-stat-label">${escapeHtml(label)}</div>
                <div class="control-stat-value">${escapeHtml(value)}</div>
            </div>
            <div class="control-stat-meta">${escapeHtml(meta)}</div>
        </div>
    `;
}

function renderControlAgentDetail(team, agent, tasks) {
    const output = agentPrimaryOutput(agent);
    const signals = renderAgentSignals(agent, { truncate: true });

    return `
        <div class="control-output-card">
            <h3>${escapeHtml(agent.name)} 的最新输出</h3>
            ${output
                ? `<pre class="control-output-text">${escapeHtml(output)}</pre>`
                : '<div class="control-empty-inline">暂无最新完整输出，等待下一次响应。</div>'}
            ${agent.cwd ? `<div class="control-path-footnote">${escapeHtml(agent.cwd)}</div>` : ''}
        </div>
        <div class="control-detail-stack">
            <div class="control-mini-panel">
                ${signals || '<div class="control-empty-inline">当前没有新的任务、工具或思路信号。</div>'}
            </div>
            <div class="control-mini-panel">
                ${tasks.length > 0 ? renderAgentTaskList(tasks) : '<div class="control-empty-inline">当前没有分配到该成员的任务。</div>'}
            </div>
            <div class="control-mini-panel">
                ${Array.isArray(agent.todos) && agent.todos.length > 0 ? renderAgentTodos(agent.todos, { showTitle: false }) : '<div class="control-empty-inline">当前没有同步到待办清单。</div>'}
            </div>
        </div>
    `;
}

function renderControlTeamDetail(team, teamTasks, members) {
    const pendingTasks = teamTasks.filter(task => String(task?.status || '').toLowerCase() !== 'completed');
    const overview = buildControlTeamOverview(team, members, pendingTasks);

    return `
        <div class="control-output-card">
            <h3>${escapeHtml(team.name)} 的当前态势</h3>
            <div class="control-overview-text">${escapeHtml(overview)}</div>
            ${team.project_cwd ? `<div class="control-path-footnote">${escapeHtml(team.project_cwd)}</div>` : ''}
        </div>
        <div class="control-detail-stack">
            <div class="control-mini-panel">
                ${members.length > 0 ? `
                    <div class="control-member-pills">
                        ${members.map(member => `
                            <span class="control-member-pill ${escapeHtml(String(member?.status || 'idle').toLowerCase())}">
                                ${escapeHtml(member.name || '成员')}
                            </span>
                        `).join('')}
                    </div>
                ` : '<div class="control-empty-inline">当前筛选下没有可见成员。</div>'}
            </div>
            <div class="control-mini-panel">
                ${pendingTasks.length > 0 ? renderAgentTaskList(pendingTasks.slice(0, 6)) : '<div class="control-empty-inline">任务板已经清空。</div>'}
            </div>
        </div>
    `;
}

function buildControlTeamOverview(team, members, pendingTasks) {
    const workingCount = members.filter(member => String(member?.status || '').toLowerCase() === 'working').length;
    const activeCount = members.filter(member => isAgentInMotion(member)).length;
    if (team.managed) {
        return `${team.name} 当前处于 ${formatManagedRunStatus(team.managed_status || '')} 状态，可见 ${members.length} 个成员，其中 ${workingCount} 个处于工作中，${pendingTasks.length} 项任务尚未完成。`;
    }

    return `${team.name} 当前由 ${members.length} 个成员构成，其中 ${activeCount} 个最近仍有活动信号，任务板上还有 ${pendingTasks.length} 项待处理。`;
}

function isTerminalActivityText(text = '') {
    return /\b(bash|terminal|shell|powershell|pwsh|cmd|zsh|sh)\b/i.test(String(text || ''));
}

function decorateConversationItem(item) {
    const next = { ...item };
    const title = String(item?.title || '');
    const text = String(item?.text || '');
    const signal = `${title} ${text}`;

    if (item?.kind === 'terminal' || item?.kind === 'terminal_output') {
        return next;
    }

    if (item?.kind === 'tool' && isTerminalActivityText(signal)) {
        next.kind = 'terminal';
        next.title = title && title !== '工具调用' ? title : '终端命令';
        return next;
    }

    if (item?.kind === 'tool_result' && isTerminalActivityText(signal)) {
        next.kind = 'terminal_output';
        next.title = '终端输出';
        return next;
    }

    if (title.includes('工具结果') && isTerminalActivityText(signal)) {
        next.kind = 'terminal_output';
        next.title = '终端输出';
        return next;
    }

    return next;
}

function describeAgentTransportState(team, agent) {
    if (!agent) {
        return '';
    }

    if (isManagedAgent(team, agent)) {
        return describeManagedAgentControlState(agent);
    }

    if (agent.command_transport === 'claude_inbox') {
        return '可通过 Claude 收件箱定向发送';
    }

    const reason = normalizeMultilineText(agent.command_reason || '');
    if (reason) {
        return reason;
    }

    return '当前监控仅展示活动流，暂不支持向该类型注入指令';
}

function buildConversationFeed(context) {
    if (!context?.team) {
        return [];
    }

    const items = [];
    const { team, agent, members, tasks, teamTasks, tasksByOwner, unassignedTasks, teamKey } = context;

    if (agent) {
        buildAgentTimeline(agent, tasks).forEach((event, index) => {
            items.push({
                id: `${buildAgentDetailKey(team, agent)}::event::${index}`,
                role: 'agent',
                actor: agent.name,
                kind: event.kind || 'message',
                title: event.title || '消息',
                text: event.text || '',
                timestamp: toTimestamp(event.time) || getAgentActivityTimestamp(agent) || 0,
                relative: event.relative || formatRelativeTime(event.time)
            });
        });

        const output = agentPrimaryOutput(agent);
        if (output && output !== normalizeMultilineText(agent.latest_message || '')) {
            items.push({
                id: `${buildAgentDetailKey(team, agent)}::output`,
                role: 'agent',
                actor: agent.name,
                kind: 'response',
                title: '完整输出',
                text: output,
                timestamp: getAgentActivityTimestamp(agent) || 0,
                relative: formatRelativeTime(agent.last_active_time || agent.last_message_time || agent.last_activity)
            });
        }

        (tasks || [])
            .filter(task => String(task?.status || '').toLowerCase() !== 'completed')
            .slice(0, 3)
            .forEach((task) => {
                items.push({
                    id: `${buildAgentDetailKey(team, agent)}::task::${task.id}`,
                    role: 'system',
                    actor: '任务板',
                    kind: 'task',
                    title: `任务 ${task.id}`,
                    text: task.subject || '',
                    timestamp: toTimestamp(task.updated_at || task.created_at),
                    relative: formatRelativeTime(task.updated_at || task.created_at)
                });
            });

        (Array.isArray(agent.todos) ? agent.todos : [])
            .slice(0, 3)
            .forEach((todo, index) => {
                items.push({
                    id: `${buildAgentDetailKey(team, agent)}::todo::${index}`,
                    role: 'system',
                    actor: '待办',
                    kind: 'task',
                    title: `待办 ${formatTodoStatus(todo.status)}`,
                    text: todo.active_form || todo.content || '',
                    timestamp: getAgentActivityTimestamp(agent) || 0,
                    relative: formatRelativeTime(agent.last_active_time || agent.last_message_time || agent.last_activity)
                });
            });

        addPendingOperatorMessages(items, teamKey, buildAgentDetailKey(team, agent));
    } else {
        members.forEach((member) => {
            buildAgentTimeline(member, tasksByOwner[member.name] || []).forEach((event, index) => {
                items.push({
                    id: `${buildAgentDetailKey(team, member)}::team-feed::${index}`,
                    role: 'agent',
                    actor: member.name,
                    kind: event.kind || 'message',
                    title: event.title || '消息',
                    text: event.text || '',
                    timestamp: toTimestamp(event.time) || getAgentActivityTimestamp(member) || 0,
                    relative: event.relative || formatRelativeTime(event.time)
                });
            });
        });

        unassignedTasks.forEach((task) => {
            items.push({
                id: `${teamKey}::unassigned::${task.id}`,
                role: 'system',
                actor: '前台广播',
                kind: 'task',
                title: `待认领任务 ${task.id}`,
                text: task.subject || '',
                timestamp: toTimestamp(task.updated_at || task.created_at),
                relative: formatRelativeTime(task.updated_at || task.created_at)
            });
        });

        if (items.length === 0 && teamTasks.length > 0) {
            teamTasks.slice(0, 4).forEach((task) => {
                items.push({
                    id: `${teamKey}::task-board::${task.id}`,
                    role: 'system',
                    actor: '任务板',
                    kind: 'task',
                    title: `任务 ${task.id}`,
                    text: task.subject || '',
                    timestamp: toTimestamp(task.updated_at || task.created_at),
                    relative: formatRelativeTime(task.updated_at || task.created_at)
                });
            });
        }

        addPendingOperatorMessages(items, teamKey, '');
    }

    items.sort((a, b) => {
        if (a.timestamp === b.timestamp) {
            return String(a.id).localeCompare(String(b.id));
        }
        return a.timestamp - b.timestamp;
    });

    return items.slice(-CONTROL_FEED_LIMIT).map(decorateConversationItem);
}

function addPendingOperatorMessages(items, teamKey, agentKey) {
    pendingOperatorMessages.forEach((message) => {
        if (message.teamKey !== teamKey) {
            return;
        }

        if (agentKey && message.agentKey !== agentKey) {
            return;
        }

        items.push({
            id: message.id,
            role: 'operator',
            actor: '我',
            kind: 'message',
            title: message.title,
            text: message.text,
            timestamp: toTimestamp(message.timestamp),
            relative: formatRelativeTime(message.timestamp)
        });
    });
}

function controlFeedAvatarLabel(item) {
    if (item?.role === 'operator') {
        return '我';
    }
    if (item?.role === 'system') {
        return '系统';
    }

    const actor = String(item?.actor || '').trim();
    return actor ? actor.slice(0, 2) : '?';
}

function renderConversationFeed(items) {
    if (!items.length) {
        return '<div class="control-feed-empty">当前没有新的活动信号，等待下一次对话或工具回声。</div>';
    }

    return `
        <div class="control-feed">
            ${items.map(item => `
                <article class="control-feed-item ${escapeHtml(item.role)} ${escapeHtml(item.kind || 'message')}">
                    <div class="control-feed-avatar">${escapeHtml(controlFeedAvatarLabel(item))}</div>
                    <div class="control-feed-bubble">
                        <div class="control-feed-head">
                            <span class="control-feed-actor">${escapeHtml(item.actor || '系统')}</span>
                            <span class="control-feed-title">${escapeHtml(item.title || '活动')}</span>
                            ${item.relative ? `<span class="control-feed-time">${escapeHtml(item.relative)}</span>` : ''}
                        </div>
                        <pre class="control-feed-text">${escapeHtml(item.text || '')}</pre>
                    </div>
                </article>
            `).join('')}
        </div>
    `;
}

function buildControlComposerConfig(context) {
    const { team, agent, selectionKey } = context;
    const feedback = controlComposerFeedback[selectionKey] || '';
    const submitting = controlComposerSubmitting[selectionKey] === true;

    if (!adminAuthState.configured) {
        return {
            selectionKey,
            targetLabel: agent ? agent.name : team.name,
            channelLabel: '只读',
            hint: '未配置管理员账号密码，当前仅允许浏览。',
            placeholder: '当前不可发送',
            buttonLabel: '发送',
            enabled: false,
            feedback,
            submitting
        };
    }

    if (!isAdminAuthenticated()) {
        return {
            selectionKey,
            targetLabel: agent ? agent.name : team.name,
            channelLabel: '只读',
            hint: '管理员登录后才能发送指令。',
            placeholder: '当前不可发送',
            buttonLabel: '发送',
            enabled: false,
            feedback,
            submitting
        };
    }

    if (agent && isManagedAgent(team, agent)) {
        const controllable = isManagedAgentControllable(agent);
        return {
            selectionKey,
            targetLabel: agent.name,
            channelLabel: '受管终端',
            hint: controllable ? '当前直连到受管会话，消息会原样写入该成员。' : describeManagedAgentControlState(agent),
            placeholder: controllable ? `输入发给 ${agent.name} 的命令、修复任务或追问` : '当前会话不可写入',
            buttonLabel: '发送给成员',
            enabled: controllable,
            transport: agent.command_transport || '',
            managedTeamID: team.managed_team_id || '',
            managedAgentID: agent.agent_id || '',
            teamName: team.name,
            agentName: agent.name,
            feedback,
            submitting
        };
    }

    if (!agent && team.managed) {
        const lead = Array.isArray(team.members) && team.members.length > 0 ? team.members[0] : null;
        const enabled = Boolean(lead && isManagedAgentControllable(lead) && team.managed_team_id);
        return {
            selectionKey,
            targetLabel: `${team.name} 主成员`,
            channelLabel: '受管团队',
            hint: enabled ? '当前处于团队总览，消息将默认写给该受管团队的主成员。' : '团队主成员当前不可控；可以先在左侧选择具体成员。',
            placeholder: enabled ? `输入发给 ${team.name} 主成员的团队级指令` : '当前团队总览不可写入',
            buttonLabel: '发送给主成员',
            enabled,
            transport: 'managed_pty',
            managedTeamID: team.managed_team_id || '',
            managedAgentID: '',
            teamName: team.name,
            agentName: '',
            feedback,
            submitting
        };
    }

    if (agent && agent.command_transport === 'claude_inbox') {
        return {
            selectionKey,
            targetLabel: agent.name,
            channelLabel: 'Claude 收件箱',
            hint: '消息会以队友消息的形式投递给当前成员。',
            placeholder: `输入发给 ${agent.name} 的命令、消息或追问`,
            buttonLabel: '发送给成员',
            enabled: true,
            transport: agent.command_transport,
            managedTeamID: '',
            managedAgentID: '',
            teamName: team.name,
            agentName: agent.name,
            feedback,
            submitting
        };
    }

    return {
        selectionKey,
        targetLabel: agent ? agent.name : team.name,
        channelLabel: '只读',
        hint: agent
            ? (agent.command_reason || '当前通道暂不支持直接下发指令。')
            : '当前是团队总览。为避免误投，导入团队视图下需要先选择一个具体成员。',
        placeholder: agent ? '当前通道暂不支持发送' : '请选择左侧具体成员',
        buttonLabel: '发送',
        enabled: false,
        transport: agent?.command_transport || '',
        managedTeamID: '',
        managedAgentID: '',
        teamName: team.name,
        agentName: agent?.name || '',
        feedback,
        submitting
    };
}

function renderControlComposer(context) {
    const config = buildControlComposerConfig(context);
    const draft = controlComposerDrafts[config.selectionKey] || '';

    return `
        <form
            class="control-composer"
            data-control-composer-form="true"
            data-control-selection-key="${escapeHtml(config.selectionKey)}"
            data-command-transport="${escapeHtml(config.transport || '')}"
            data-managed-team-id="${escapeHtml(config.managedTeamID || '')}"
            data-managed-agent-id="${escapeHtml(config.managedAgentID || '')}"
            data-team-name="${escapeHtml(config.teamName || '')}"
            data-agent-name="${escapeHtml(config.agentName || '')}"
        >
            <div class="control-composer-top">
                <h3>对 ${escapeHtml(config.targetLabel)} 发送指令</h3>
                <div class="control-composer-meta">
                    <span class="control-provider-pill subtle">${escapeHtml(config.channelLabel)}</span>
                    <span class="control-composer-shortcut">回车发送 · Shift+回车换行</span>
                </div>
            </div>
            <textarea
                class="agent-command-input control-composer-input"
                name="control-composer-text"
                rows="3"
                data-control-composer-input="true"
                data-control-selection-key="${escapeHtml(config.selectionKey)}"
                placeholder="${escapeHtml(config.placeholder)}"
                ${!config.enabled ? 'disabled' : ''}
            >${escapeHtml(draft)}</textarea>
            <div class="control-composer-bottom">
                <div class="control-composer-hint">${escapeHtml(config.hint)}</div>
                <div class="control-composer-actions">
                    <span class="agent-command-feedback control-composer-feedback">${escapeHtml(config.feedback)}</span>
                    <button type="submit" class="agent-command-submit" ${!config.enabled || config.submitting ? 'disabled' : ''}>${escapeHtml(config.submitting ? '发送中...' : config.buttonLabel)}</button>
                </div>
            </div>
        </form>
    `;
}

async function submitControlComposer(form) {
    const textarea = form.querySelector('[data-control-composer-input]');
    const submitButton = form.querySelector('button[type="submit"]');
    const selectionKey = form.getAttribute('data-control-selection-key') || '';
    const text = textarea instanceof HTMLTextAreaElement ? textarea.value.trim() : '';

    if (!selectionKey) {
        return;
    }

    if (!isAdminAuthenticated()) {
        controlComposerFeedback[selectionKey] = '管理员登录后才能发送指令';
        rerenderControlWorkspace();
        return;
    }

    if (!text) {
        controlComposerFeedback[selectionKey] = '请输入消息';
        rerenderControlWorkspace();
        return;
    }

    if (submitButton instanceof HTMLButtonElement) {
        submitButton.disabled = true;
    }
    controlComposerSubmitting[selectionKey] = true;
    controlComposerFeedback[selectionKey] = '发送中...';
    rerenderControlWorkspace();

    try {
        const result = await sendAgentMessageRequest({
            teamName: form.getAttribute('data-team-name') || '',
            agentName: form.getAttribute('data-agent-name') || '',
            transport: form.getAttribute('data-command-transport') || '',
            managedTeamID: form.getAttribute('data-managed-team-id') || '',
            managedAgentID: form.getAttribute('data-managed-agent-id') || '',
            text
        });

        const teams = previousState?.teams || [];
        const team = teams.find(item => item.name === (form.getAttribute('data-team-name') || '')) || null;
        const agent = resolveMessageTargetAgent(team, form.getAttribute('data-agent-name') || '', form.getAttribute('data-managed-agent-id') || '');
        queueOperatorMessage(team, agent, text);

        controlComposerDrafts[selectionKey] = '';
        controlComposerFeedback[selectionKey] = result.confirmation;
        await fetchData();
    } catch (error) {
        controlComposerFeedback[selectionKey] = `发送失败: ${error.message}`;
        rerenderControlWorkspace();
    } finally {
        controlComposerSubmitting[selectionKey] = false;
        if (submitButton instanceof HTMLButtonElement) {
            submitButton.disabled = false;
        }
        rerenderControlWorkspace();
    }
}

function resolveMessageTargetAgent(team, agentName, managedAgentID) {
    if (!team) {
        return null;
    }

    const members = Array.isArray(team.members) ? team.members : [];
    if (managedAgentID) {
        return members.find(agent => String(agent?.agent_id || '') === managedAgentID) || null;
    }

    if (agentName) {
        return members.find(agent => String(agent?.name || '') === agentName) || null;
    }

    return null;
}

function queueOperatorMessage(team, agent, text) {
    if (!team) {
        return;
    }

    pendingOperatorMessages.push({
        id: `operator-${Date.now()}-${Math.random().toString(16).slice(2, 8)}`,
        teamKey: buildTeamControlKey(team),
        agentKey: agent ? buildAgentDetailKey(team, agent) : '',
        title: agent ? `发送给 ${agent.name}` : `发送给 ${team.name} 主成员`,
        text: normalizeMultilineText(text),
        timestamp: new Date().toISOString()
    });

    if (pendingOperatorMessages.length > 80) {
        pendingOperatorMessages.splice(0, pendingOperatorMessages.length - 80);
    }
}

function formatTodoStatus(status) {
    switch (String(status || '').toLowerCase()) {
        case 'in_progress':
            return '进行中';
        case 'completed':
            return '已完成';
        default:
            return '待处理';
    }
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
    const managedRunningCount = team.managed ? countManagedRunningMembers(members) : 0;
    const managedControllableCount = team.managed ? countManagedControllableMembers(members) : 0;
    const teamId = `team-${encodeURIComponent(team.name)}`;
    const canDelete = !team.managed && provider !== 'codex' && isAdminAuthenticated();
    const providerBadge = provider !== 'unknown' ? `<span class="agent-type">[${escapeHtml(providerDisplayName(provider))}]</span>` : '';
    const controlBadge = team.managed
        ? `<span class="agent-type">[受管:${escapeHtml(formatManagedRunStatus(team.managed_status || ''))}]</span>`
        : `<span class="agent-type">[导入]</span>`;
    const headerAction = renderTeamActions(team, provider, canDelete);
    const summaryBar = team.managed ? `
                <div class="team-summary-item">
                    <span class="team-summary-label">成员</span>
                    <span class="team-summary-value">${members.length}</span>
                </div>
                <div class="team-summary-item">
                    <span class="team-summary-label">受管运行</span>
                    <span class="team-summary-value">${managedRunningCount}/${members.length}</span>
                </div>
                <div class="team-summary-item">
                    <span class="team-summary-label">本地可控</span>
                    <span class="team-summary-value">${managedControllableCount}/${members.length}</span>
                </div>
                <div class="team-summary-item">
                    <span class="team-summary-label">待处理任务</span>
                    <span class="team-summary-value">${pendingTasks}</span>
                </div>
            ` : `
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
            `;
    const officeHint = team.managed
        ? '团队卡片顶部负责全部启动 / 停止；成员卡片下方负责单成员启动 / 停止；点击成员卡片后，可在详情里继续单成员控制与发消息。'
        : '常态下展示 4 列概览卡片，点击任意成员可查看最近活动、完整输出、工具调用和待办详情。';

    return `
        <div class="team-card" id="${teamId}">
            <div class="team-header">
                <div class="team-header-left">
                    <div class="team-name">${escapeHtml(team.name)} ${providerBadge} ${controlBadge}</div>
                    <div class="team-created">创建时间: ${createdDate}</div>
                    ${projectCwd ? `<div class="team-cwd">工作目录: ${escapeHtml(projectCwd)}</div>` : ''}
                    ${team.log_path ? `<div class="team-cwd">日志: ${escapeHtml(team.log_path)}</div>` : ''}
                </div>
                ${headerAction}
            </div>

            <div class="team-summary-bar">
                ${summaryBar}
            </div>

            <div class="team-section office-scene">
                <h3><span class="section-icon"><svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><rect x="2" y="7" width="20" height="14" rx="2" ry="2"/><path d="M16 7V5a2 2 0 0 0-2-2h-4a2 2 0 0 0-2 2v2"/></svg></span> 办公区实况（${members.length} 位同事，${workingCount} 位忙碌中）</h3>
                <p class="office-hint">${officeHint}</p>
                ${renderAgentsWithTasks(team, members, tasks)}
            </div>
        </div>
    `;
}

// Delete a team
async function deleteTeam(teamName) {
    if (!isAdminAuthenticated()) {
        alert('管理员登录后才能清理团队');
        return;
    }

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

    const cardMarkup = `
        <button
            type="button"
            class="agent-item agent-compact-card office-desk ${statusClass} ${motionClass}"
            data-agent-detail-key="${escapeHtml(buildAgentCardKey(team, agent))}"
            aria-label="查看 ${escapeHtml(agent.name)} 详情"
        >
            <div class="agent-header">
                <span class="agent-avatar" aria-hidden="true">${roleEmoji}</span>
                <span class="agent-name">${escapeHtml(agent.name)}</span>
                <span class="agent-type">[${escapeHtml(formatAgentTypeLabel(agent.agent_type))}]</span>
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

    const managedControls = renderManagedAgentControls(team, agent);
    if (!managedControls) {
        return cardMarkup;
    }

    return `
        <div class="agent-card-shell">
            ${cardMarkup}
            ${managedControls}
        </div>
    `;
}

function renderManagedAgentControls(team, agent) {
    if (!isManagedAgent(team, agent)) {
        return '';
    }

    const stateTone = managedAgentControlTone(agent);
    const controlsEnabled = isAdminAuthenticated();

    return `
        <div class="managed-agent-controls ${escapeHtml(stateTone)}">
            <div class="managed-agent-state">
                <div class="managed-agent-state-label">受管会话</div>
                <div class="managed-agent-state-value">${escapeHtml(describeManagedAgentControlState(agent))}</div>
            </div>
            ${renderManagedAgentActionButtons(team, agent, controlsEnabled)}
        </div>
    `;
}

function normalizeLookupToken(value) {
    return String(value || '').trim().toLowerCase();
}

function buildAgentIdentityKey(agent) {
    if (!agent) {
        return 'unknown';
    }

    const agentID = normalizeLookupToken(agent.agent_id);
    if (agentID) {
        return `id:${agentID}`;
    }

    const parts = [];
    const cwd = normalizeLookupToken(agent.cwd);
    const joinedAt = normalizeLookupToken(agent.joined_at);
    const name = normalizeLookupToken(agent.name);
    const agentType = normalizeLookupToken(agent.agent_type);

    if (cwd) {
        parts.push(`cwd:${cwd}`);
    }
    if (joinedAt) {
        parts.push(`joined:${joinedAt}`);
    }
    if (name) {
        parts.push(`name:${name}`);
    }
    if (agentType) {
        parts.push(`type:${agentType}`);
    }

    return parts.length > 0 ? parts.join('::') : 'unknown';
}

function buildAgentCardKey(team, agent) {
    return `${team.name}::agent::${buildAgentIdentityKey(agent)}`;
}

function buildAgentDetailKey(team, agent) {
    return `${team.name}::agent::${buildAgentIdentityKey(agent)}`;
}

function buildBroadcastDetailKey(team) {
    return `${team.name}::broadcast`;
}

function buildAgentPrimarySignal(agent, tasks) {
    if (agent.current_task) {
        return { kind: 'task', label: '当前任务', value: truncateMultiline(agent.current_task, 44) };
    }
    if (agent.last_tool_use) {
        const terminal = isTerminalActivityText(`${agent.last_tool_use || ''} ${agent.last_tool_detail || ''}`);
        return {
            kind: terminal ? 'terminal' : 'tool',
            label: terminal ? '终端命令' : '调用工具',
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
        const terminal = isTerminalActivityText(`${agent.last_tool_use || ''} ${agent.last_tool_detail || ''}`);
        timeline.push({
            kind: terminal ? 'terminal' : 'tool',
            title: terminal ? '终端命令' : '工具调用',
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
        const terminal = isTerminalActivityText(`${agent.last_tool_use || ''} ${agent.last_tool_detail || ''}`);
        chips.push({
            kind: terminal ? 'terminal' : 'tool',
            label: terminal ? '终端命令' : '调用工具',
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
        case 'tool_result': return '工具结果';
        case 'terminal': return '终端命令';
        case 'terminal_output': return '终端输出';
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
        case 'tool_result': return '≋';
        case 'terminal': return '⌥';
        case 'terminal_output': return '⧉';
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
function renderAgentTodos(todos, options = {}) {
    const { showTitle = true } = options;
    const todoStatusIcon = (status) => {
        switch (status) {
            case 'in_progress': return '<svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="#34d399" stroke-width="2.5"><circle cx="12" cy="12" r="10"/><path d="M12 6v6l4 2"/></svg>';
            case 'completed': return '<svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="#6b7280" stroke-width="2.5"><path d="M20 6L9 17l-5-5"/></svg>';
            default: return '<svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="#63636e" stroke-width="2"><rect x="3" y="3" width="18" height="18" rx="3"/></svg>';
        }
    };

    return `
        <div class="agent-todos agent-panel-section panel-todo">
            ${showTitle ? '<div class="agent-panel-title">待办清单</div>' : ''}
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
                <span class="agent-type">[${escapeHtml(formatAgentTypeLabel(agent.agent_type))}]</span>
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

    document.addEventListener('submit', async (event) => {
        const form = event.target.closest('[data-agent-message-form]');
        if (!form) {
            return;
        }

        event.preventDefault();
        const teamName = form.getAttribute('data-team-name') || '';
        const agentName = form.getAttribute('data-agent-name') || '';
        const textarea = form.querySelector('textarea[name="agent-message-text"]');
        const submitButton = form.querySelector('button[type="submit"]');
        const feedback = form.querySelector('[data-agent-message-feedback]');
        const text = textarea ? textarea.value.trim() : '';

        if (!isAdminAuthenticated()) {
            if (feedback) {
                feedback.textContent = '管理员登录后才能发送消息';
            }
            return;
        }

        if (!text) {
            if (feedback) {
                feedback.textContent = '请输入消息';
            }
            return;
        }

        if (submitButton) {
            submitButton.disabled = true;
        }
        if (feedback) {
            feedback.textContent = '发送中...';
        }

        try {
            const transport = form.getAttribute('data-command-transport') || '';
            const managedTeamID = form.getAttribute('data-managed-team-id') || '';
            const managedAgentID = form.getAttribute('data-managed-agent-id') || '';
            const result = await sendAgentMessageRequest({
                teamName,
                agentName,
                transport,
                managedTeamID,
                managedAgentID,
                text
            });

            const team = (previousState?.teams || []).find(item => item.name === teamName) || null;
            const agent = resolveMessageTargetAgent(team, agentName, managedAgentID);
            queueOperatorMessage(team, agent, text);

            if (textarea) {
                textarea.value = '';
            }
            if (feedback) {
                feedback.textContent = result.confirmation;
            }
            fetchData();
        } catch (error) {
            if (feedback) {
                feedback.textContent = `发送失败: ${error.message}`;
            }
        } finally {
            if (submitButton) {
                submitButton.disabled = false;
            }
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
    }
    syncOverlayState();
}

function syncAgentDetailModal(lookup) {
    const modal = document.getElementById('agent-detail-modal');
    const content = document.getElementById('agent-detail-content');

    if (!modal || !content) {
        return;
    }

    if (!activeAgentDetailKey) {
        modal.hidden = true;
        syncOverlayState();
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
    syncOverlayState();
}

function resolveActiveAgentDetail(activeKey, lookup) {
    if (!lookup || lookup.size === 0) {
        return null;
    }

    for (const [key, value] of lookup.entries()) {
        if (activeKey === key) {
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
                <div class="agent-detail-subtitle">${escapeHtml(team.name)} · ${escapeHtml(formatAgentTypeLabel(agent.agent_type))}</div>
            </div>
            <div class="agent-detail-badges">
                <span class="agent-status ${escapeHtml(statusClass)}">${escapeHtml(statusText)}</span>
                <span class="agent-activity ${moving ? 'active' : 'idle'}">${escapeHtml(motionLabel)}</span>
            </div>
        </div>
        ${renderAgentCommandComposer(team, agent)}
        ${renderAgentWithTasks(agent, tasks)}
    `;
}

function renderAgentCommandComposer(team, agent) {
    if (!agent) {
        return '';
    }

    if (isManagedAgent(team, agent)) {
        const controlsEnabled = isAdminAuthenticated();
        const controllable = isManagedAgentControllable(agent);
        const stateTone = managedAgentControlTone(agent);
        let controlHint = '当前受管会话的启动、停止与发消息入口已统一到这里。';
        if (!adminAuthState.configured) {
            controlHint = '未配置管理员账号密码，当前仅允许浏览受管会话状态。';
        } else if (!controlsEnabled) {
            controlHint = '管理员登录后才能启动、停止或向受管会话发消息。';
        } else if (controllable) {
            controlHint = '当前直连受管终端，可直接下发任务。';
        } else if (isManagedAgentRunning(agent)) {
            controlHint = '该受管会话仍在运行，但当前控制进程不可用；暂时无法停止或发消息。';
        } else {
            controlHint = '先启动这个受管成员，再向它下发任务。';
        }

        return `
            <form
                class="agent-command-panel managed-agent-command-panel"
                data-agent-message-form="true"
                data-managed-feedback-scope="true"
                data-command-transport="${escapeHtml(agent.command_transport || '')}"
                data-managed-team-id="${escapeHtml(team.managed_team_id || '')}"
                data-managed-agent-id="${escapeHtml(agent.agent_id || '')}"
                data-team-name="${escapeHtml(team.name)}"
                data-agent-name="${escapeHtml(agent.name)}"
            >
                <div class="agent-panel-title">受管会话控制</div>
                <div class="agent-command-hint">${escapeHtml(describeManagedAgentControlState(agent))}</div>
                <div class="managed-agent-controls within-panel ${escapeHtml(stateTone)}">
                    <div class="managed-agent-state">
                        <div class="managed-agent-state-label">当前状态</div>
                        <div class="managed-agent-state-value">${escapeHtml(controlHint)}</div>
                    </div>
                    ${renderManagedAgentActionButtons(team, agent, controlsEnabled)}
                </div>
                <span class="agent-command-feedback" data-managed-feedback></span>
                <div class="agent-command-divider"></div>
                <div class="agent-panel-title">直接发消息</div>
                <div class="agent-command-hint">${escapeHtml(controlHint)}</div>
                <textarea
                    class="agent-command-input"
                    name="agent-message-text"
                    rows="4"
                    placeholder="输入你要发给这个受管成员的命令或任务"
                    ${!controlsEnabled || !controllable ? 'disabled' : ''}
                ></textarea>
                <div class="agent-command-actions">
                    <button type="submit" class="agent-command-submit" ${!controlsEnabled || !controllable ? 'disabled' : ''}>发送</button>
                    <span class="agent-command-feedback" data-agent-message-feedback></span>
                </div>
            </form>
        `;
    }

    if (!adminAuthState.configured) {
        return `
            <div class="agent-command-panel unsupported">
                <div class="agent-panel-title">直接发消息</div>
                <div class="agent-command-hint">未配置管理员账号密码，当前仅允许浏览。</div>
            </div>
        `;
    }

    if (!isAdminAuthenticated()) {
        return `
            <div class="agent-command-panel unsupported">
                <div class="agent-panel-title">直接发消息</div>
                <div class="agent-command-hint">管理员登录后才能发送消息或修改设置。</div>
            </div>
        `;
    }

    if (agent.command_transport === 'claude_inbox') {
        return `
            <form
                class="agent-command-panel"
                data-agent-message-form="true"
                data-command-transport="${escapeHtml(agent.command_transport)}"
                data-team-name="${escapeHtml(team.name)}"
                data-agent-name="${escapeHtml(agent.name)}"
            >
                <div class="agent-panel-title">直接发消息</div>
                <div class="agent-command-hint">当前通过 Claude 收件箱投递，消息会以队友消息的形式送达。</div>
                <textarea
                    class="agent-command-input"
                    name="agent-message-text"
                    rows="4"
                    placeholder="输入你要发给这个成员的命令或消息"
                ></textarea>
                <div class="agent-command-actions">
                    <button type="submit" class="agent-command-submit">发送</button>
                    <span class="agent-command-feedback" data-agent-message-feedback></span>
                </div>
            </form>
        `;
    }

    if (agent.command_reason) {
        return `
            <div class="agent-command-panel unsupported">
                <div class="agent-panel-title">直接发消息</div>
                <div class="agent-command-hint">${escapeHtml(agent.command_reason)}</div>
            </div>
        `;
    }

    return '';
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
    return formatMappedLabel(status, STATE_LABELS, '未知');
}

// Format task status
function formatTaskStatus(status) {
    return formatMappedLabel(status, STATE_LABELS, '未知');
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

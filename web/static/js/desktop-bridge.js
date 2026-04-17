const DESKTOP_REFRESH_INTERVAL_MS = 1000;

function readInitialDesktopPreferences() {
    const initial = window.__ATM_DESKTOP_INITIAL_PREFERENCES__;
    if (!initial || typeof initial !== 'object') {
        return null;
    }

    return { ...initial };
}

function isDesktopModeEnabled() {
    if (typeof window === 'undefined') {
        return false;
    }

    if (window.__ATM_DESKTOP__ === true && typeof window.atmDesktopGetState === 'function') {
        return true;
    }

    try {
        const params = new URLSearchParams(window.location.search);
        return params.get('desktop') === '1' && typeof window.atmDesktopGetState === 'function';
    } catch (_) {
        return false;
    }
}

async function fetchDesktopState() {
    if (!isDesktopModeEnabled()) {
        throw new Error('desktop bridge unavailable');
    }

    return await window.atmDesktopGetState();
}

async function deleteDesktopTeam(teamName) {
    if (!isDesktopModeEnabled()) {
        throw new Error('desktop bridge unavailable');
    }

    return await window.atmDesktopDeleteTeam(teamName);
}

async function sendDesktopAgentMessage(teamName, agentName, text) {
    if (!isDesktopModeEnabled()) {
        throw new Error('desktop bridge unavailable');
    }

    if (typeof window.atmDesktopSendAgentMessage !== 'function') {
        throw new Error('desktop agent message bridge unavailable');
    }

    return await window.atmDesktopSendAgentMessage(teamName, agentName, text);
}

async function getDesktopContext() {
    if (!isDesktopModeEnabled()) {
        throw new Error('desktop bridge unavailable');
    }

    return await window.atmDesktopGetContext();
}

async function quitDesktopApp() {
    if (!isDesktopModeEnabled()) {
        throw new Error('desktop bridge unavailable');
    }

    return await window.atmDesktopQuit();
}

async function navigateDesktopApp(target) {
    if (!isDesktopModeEnabled()) {
        throw new Error('desktop bridge unavailable');
    }

    return await window.atmDesktopNavigate(target);
}

async function getDesktopPreferences() {
    if (!isDesktopModeEnabled()) {
        throw new Error('desktop bridge unavailable');
    }

    if (typeof window.atmDesktopGetPreferences === 'function') {
        return await window.atmDesktopGetPreferences();
    }

    return readInitialDesktopPreferences();
}

async function setDesktopPreferences(preferences) {
    if (!isDesktopModeEnabled()) {
        throw new Error('desktop bridge unavailable');
    }

    if (typeof window.atmDesktopSetPreferences !== 'function') {
        throw new Error('desktop preferences bridge unavailable');
    }

    return await window.atmDesktopSetPreferences(preferences);
}

async function openDesktopExternal(target) {
    if (!isDesktopModeEnabled()) {
        throw new Error('desktop bridge unavailable');
    }

    if (typeof window.atmDesktopOpenExternal !== 'function') {
        throw new Error('desktop external bridge unavailable');
    }

    return await window.atmDesktopOpenExternal(target);
}

async function setDesktopWindowTitle(title) {
    if (!isDesktopModeEnabled()) {
        throw new Error('desktop bridge unavailable');
    }

    if (typeof window.atmDesktopSetWindowTitle !== 'function') {
        throw new Error('desktop window title bridge unavailable');
    }

    return await window.atmDesktopSetWindowTitle(title);
}

async function hideDesktopWindow() {
    if (!isDesktopModeEnabled()) {
        throw new Error('desktop bridge unavailable');
    }

    if (typeof window.atmDesktopHideWindow !== 'function') {
        throw new Error('desktop hide bridge unavailable');
    }

    return await window.atmDesktopHideWindow();
}

async function showDesktopWindow() {
    if (!isDesktopModeEnabled()) {
        throw new Error('desktop bridge unavailable');
    }

    if (typeof window.atmDesktopShowWindow !== 'function') {
        throw new Error('desktop show bridge unavailable');
    }

    return await window.atmDesktopShowWindow();
}

async function openDesktopPreferences() {
    if (!isDesktopModeEnabled()) {
        throw new Error('desktop bridge unavailable');
    }

    if (typeof window.atmDesktopOpenPreferences !== 'function') {
        throw new Error('desktop preferences window bridge unavailable');
    }

    return await window.atmDesktopOpenPreferences();
}

async function openDesktopAbout() {
    if (!isDesktopModeEnabled()) {
        throw new Error('desktop bridge unavailable');
    }

    if (typeof window.atmDesktopOpenAbout !== 'function') {
        throw new Error('desktop about window bridge unavailable');
    }

    return await window.atmDesktopOpenAbout();
}

function startDesktopPolling(fetcher, intervalMs = DESKTOP_REFRESH_INTERVAL_MS) {
    if (typeof fetcher !== 'function') {
        throw new Error('desktop polling requires a fetcher callback');
    }

    let timer = null;

    const run = async () => {
        await fetcher();
    };

    run();
    timer = window.setInterval(run, intervalMs);

    return () => {
        if (timer) {
            window.clearInterval(timer);
            timer = null;
        }
    };
}

window.AgentMonitorDesktopBridge = {
    isDesktopModeEnabled,
    fetchDesktopState,
    deleteDesktopTeam,
    sendDesktopAgentMessage,
    getDesktopContext,
    quitDesktopApp,
    navigateDesktopApp,
    getDesktopPreferences,
    setDesktopPreferences,
    openDesktopExternal,
    setDesktopWindowTitle,
    hideDesktopWindow,
    showDesktopWindow,
    openDesktopPreferences,
    openDesktopAbout,
    startDesktopPolling,
    readInitialDesktopPreferences,
    DESKTOP_REFRESH_INTERVAL_MS,
};

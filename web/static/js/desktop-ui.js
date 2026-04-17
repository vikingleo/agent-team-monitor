const DESKTOP_BRIDGE = window.AgentMonitorDesktopBridge || null;
const IS_DESKTOP_MODE = Boolean(DESKTOP_BRIDGE && DESKTOP_BRIDGE.isDesktopModeEnabled());
const DESKTOP_PREFS_KEY = 'atm-desktop-preferences';
const DEFAULT_PREFERENCES = {
    hideIdleAgents: true,
    startupView: 'dashboard',
    providerFilter: 'all',
    theme: 'light',
    notifyTaskCompletion: true,
    notifyStaleAgents: true,
    closeToTray: true,
    launchOnLogin: false,
    startMinimizedToTray: false,
};

let desktopPreferencesCache = normalizePreferences(
    IS_DESKTOP_MODE
        ? (DESKTOP_BRIDGE?.readInitialDesktopPreferences?.() || DEFAULT_PREFERENCES)
        : readWebPreferences()
);
let desktopAdminAccessEnabled = true;

function normalizeProviderFilter(value) {
    switch (value) {
        case 'claude':
        case 'codex':
        case 'openclaw':
            return value;
        default:
            return 'all';
    }
}

function normalizeTheme(value) {
    return value === 'dark' ? 'dark' : 'light';
}

function normalizeStartupView(value) {
    return value === 'game' ? 'game' : 'dashboard';
}

function normalizePreferences(prefs) {
    return {
        hideIdleAgents: prefs?.hideIdleAgents !== false,
        startupView: normalizeStartupView(prefs?.startupView),
        providerFilter: normalizeProviderFilter(prefs?.providerFilter),
        theme: normalizeTheme(prefs?.theme),
        notifyTaskCompletion: prefs?.notifyTaskCompletion !== false,
        notifyStaleAgents: prefs?.notifyStaleAgents !== false,
        closeToTray: prefs?.closeToTray !== false,
        launchOnLogin: prefs?.launchOnLogin === true,
        startMinimizedToTray: prefs?.startMinimizedToTray === true,
    };
}

function readWebPreferences() {
    try {
        const raw = window.localStorage.getItem(DESKTOP_PREFS_KEY);
        if (!raw) {
            return { ...DEFAULT_PREFERENCES };
        }
        return normalizePreferences(JSON.parse(raw));
    } catch (_) {
        return { ...DEFAULT_PREFERENCES };
    }
}

function writeWebPreferences(prefs) {
    try {
        window.localStorage.setItem(DESKTOP_PREFS_KEY, JSON.stringify(prefs));
    } catch (_) {
    }
}

async function loadDesktopPreferences() {
    if (!IS_DESKTOP_MODE || !DESKTOP_BRIDGE) {
        desktopPreferencesCache = readWebPreferences();
        return { ...desktopPreferencesCache };
    }

    try {
        const remote = await DESKTOP_BRIDGE.getDesktopPreferences();
        desktopPreferencesCache = normalizePreferences(remote || DEFAULT_PREFERENCES);
    } catch (error) {
        console.error('Failed to load desktop preferences:', error);
        desktopPreferencesCache = normalizePreferences(
            DESKTOP_BRIDGE.readInitialDesktopPreferences?.() || DEFAULT_PREFERENCES
        );
    }

    return { ...desktopPreferencesCache };
}

async function saveDesktopPreferences(nextPrefs) {
    if (!desktopAdminAccessEnabled) {
        throw new Error('管理员登录后才能修改桌面设置');
    }

    const normalized = normalizePreferences(nextPrefs);

    if (!IS_DESKTOP_MODE || !DESKTOP_BRIDGE) {
        desktopPreferencesCache = normalized;
        writeWebPreferences(normalized);
        return { ...desktopPreferencesCache };
    }

    try {
        const remote = await DESKTOP_BRIDGE.setDesktopPreferences(normalized);
        desktopPreferencesCache = normalizePreferences(remote || normalized);
    } catch (error) {
        console.error('Failed to persist desktop preferences:', error);
        throw error;
    }

    return { ...desktopPreferencesCache };
}

export function setDesktopAdminAccess(enabled) {
    desktopAdminAccessEnabled = enabled !== false;

    const settingsButton = document.getElementById('desktop-settings-button');
    if (settingsButton) {
        settingsButton.disabled = !desktopAdminAccessEnabled;
        settingsButton.title = desktopAdminAccessEnabled ? '' : '管理员登录后才能修改设置';
    }

    [
        'desktop-hide-idle-toggle',
        'desktop-startup-view-select',
        'desktop-provider-filter-select',
        'desktop-notify-task-completion-toggle',
        'desktop-notify-stale-agents-toggle',
        'desktop-close-to-tray-toggle',
        'desktop-launch-on-login-toggle',
        'desktop-start-minimized-to-tray-toggle',
    ].forEach((id) => {
        const element = document.getElementById(id);
        if (element) {
            element.disabled = !desktopAdminAccessEnabled;
        }
    });
}

function currentDesktopView() {
    const path = window.location.pathname || '/';
    return path.startsWith('/game') ? 'game' : 'dashboard';
}

function syncViewSwitcher(currentView) {
    document.querySelectorAll('.view-switcher-link').forEach((link) => {
        const target = link.getAttribute('data-desktop-target') || link.getAttribute('href') || '/';
        const isActive = currentView === 'game' ? target === '/game/' : target === '/';
        link.classList.toggle('active', isActive);
        if (isActive) {
            link.setAttribute('aria-current', 'page');
        } else {
            link.removeAttribute('aria-current');
        }
    });

    const currentViewLabel = document.getElementById('desktop-current-view');
    if (currentViewLabel) {
        currentViewLabel.textContent = currentView === 'game' ? '办公场景' : '监控面板';
    }
}

function bindSettingsDismiss(settingsButton, settingsPanel) {
    if (!settingsButton || !settingsPanel) {
        return;
    }

    document.addEventListener('click', (event) => {
        if (settingsPanel.hidden) {
            return;
        }

        const target = event.target;
        if (target instanceof Node && (settingsPanel.contains(target) || settingsButton.contains(target))) {
            return;
        }

        settingsPanel.hidden = true;
    });

    document.addEventListener('keydown', (event) => {
        if (event.key === 'Escape') {
            settingsPanel.hidden = true;
        }
    });
}

function applyDesktopPreferencesToForm(preferences) {
    const hideIdleToggle = document.getElementById('desktop-hide-idle-toggle');
    const startupViewSelect = document.getElementById('desktop-startup-view-select');
    const providerFilterSelect = document.getElementById('desktop-provider-filter-select');
    const notifyTaskCompletionToggle = document.getElementById('desktop-notify-task-completion-toggle');
    const notifyStaleAgentsToggle = document.getElementById('desktop-notify-stale-agents-toggle');
    const closeToTrayToggle = document.getElementById('desktop-close-to-tray-toggle');
    const launchOnLoginToggle = document.getElementById('desktop-launch-on-login-toggle');
    const startMinimizedToTrayToggle = document.getElementById('desktop-start-minimized-to-tray-toggle');

    if (hideIdleToggle) {
        hideIdleToggle.checked = preferences.hideIdleAgents !== false;
    }
    if (startupViewSelect) {
        startupViewSelect.value = preferences.startupView || 'dashboard';
    }
    if (providerFilterSelect) {
        providerFilterSelect.value = preferences.providerFilter || 'all';
    }
    if (notifyTaskCompletionToggle) {
        notifyTaskCompletionToggle.checked = preferences.notifyTaskCompletion !== false;
    }
    if (notifyStaleAgentsToggle) {
        notifyStaleAgentsToggle.checked = preferences.notifyStaleAgents !== false;
    }
    if (closeToTrayToggle) {
        closeToTrayToggle.checked = preferences.closeToTray !== false;
    }
    if (launchOnLoginToggle) {
        launchOnLoginToggle.checked = preferences.launchOnLogin === true;
    }
    if (startMinimizedToTrayToggle) {
        startMinimizedToTrayToggle.checked = preferences.startMinimizedToTray === true;
    }
}

function bindExternalLinks() {
    document.addEventListener('click', async (event) => {
        const anchor = event.target instanceof Element ? event.target.closest('a[href]') : null;
        if (!anchor) {
            return;
        }

        const href = anchor.getAttribute('href') || '';
        const target = anchor.getAttribute('target') || '';
        const isExternal = href.startsWith('http://') || href.startsWith('https://');
        if (!isExternal && target !== '_blank') {
            return;
        }

        if (!href.startsWith('http://') && !href.startsWith('https://')) {
            return;
        }

        event.preventDefault();
        try {
            await DESKTOP_BRIDGE.openDesktopExternal(href);
        } catch (error) {
            console.error('Failed to open external link:', error);
        }
    });
}

function bindDesktopShortcuts(onRefresh) {
    document.addEventListener('keydown', async (event) => {
        const activeElement = document.activeElement;
        const isTyping = activeElement instanceof HTMLElement && (
            activeElement.tagName === 'INPUT' ||
            activeElement.tagName === 'TEXTAREA' ||
            activeElement.isContentEditable
        );
        if (isTyping) {
            return;
        }

        const key = String(event.key || '').toLowerCase();
        if (key === 'f5' || ((event.ctrlKey || event.metaKey) && key === 'r')) {
            event.preventDefault();
            if (typeof onRefresh === 'function') {
                onRefresh();
            }
            return;
        }

        if ((event.ctrlKey || event.metaKey) && key === '1') {
            event.preventDefault();
            await DESKTOP_BRIDGE.navigateDesktopApp('/');
            return;
        }

        if ((event.ctrlKey || event.metaKey) && key === '2') {
            event.preventDefault();
            await DESKTOP_BRIDGE.navigateDesktopApp('/game/');
        }
    });
}

function applyThemeChoice(theme) {
    document.documentElement.setAttribute('data-theme', theme);
    document.querySelectorAll('[data-theme-choice]').forEach((button) => {
        const active = button.getAttribute('data-theme-choice') === theme;
        button.classList.toggle('active', active);
        button.setAttribute('aria-pressed', active ? 'true' : 'false');
    });
}

export async function initDesktopUI(options = {}) {
    if (!IS_DESKTOP_MODE || !DESKTOP_BRIDGE) {
        return { ...desktopPreferencesCache };
    }

    const {
        onRefresh = null,
        onPreferencesChanged = null,
    } = options;

    document.body.setAttribute('data-desktop-mode', '1');
    document.documentElement.setAttribute('data-desktop-mode', '1');

    const toolbar = document.getElementById('desktop-toolbar');
    const refreshButton = document.getElementById('desktop-refresh-button');
    const hideButton = document.getElementById('desktop-hide-button');
    const quitButton = document.getElementById('desktop-quit-button');
    const aboutButton = document.getElementById('desktop-about-button');
    const settingsButton = document.getElementById('desktop-settings-button');
    const contextPill = document.getElementById('desktop-context-pill');
    const settingsPanel = document.getElementById('desktop-settings-panel');
    const hideIdleToggle = document.getElementById('desktop-hide-idle-toggle');
    const startupViewSelect = document.getElementById('desktop-startup-view-select');
    const providerFilterSelect = document.getElementById('desktop-provider-filter-select');
    const notifyTaskCompletionToggle = document.getElementById('desktop-notify-task-completion-toggle');
    const notifyStaleAgentsToggle = document.getElementById('desktop-notify-stale-agents-toggle');
    const closeToTrayToggle = document.getElementById('desktop-close-to-tray-toggle');
    const launchOnLoginToggle = document.getElementById('desktop-launch-on-login-toggle');
    const startMinimizedToTrayToggle = document.getElementById('desktop-start-minimized-to-tray-toggle');
    const preferences = await loadDesktopPreferences();
    const activeView = currentDesktopView();

    if (toolbar) {
        toolbar.hidden = false;
    }
    applyDesktopPreferencesToForm(preferences);

    applyThemeChoice(preferences.theme);
    syncViewSwitcher(activeView);
    bindSettingsDismiss(settingsButton, settingsPanel);
    bindExternalLinks();
    bindDesktopShortcuts(onRefresh);

    try {
        await DESKTOP_BRIDGE.setDesktopWindowTitle(
            activeView === 'game' ? 'Agent Team Monitor · 办公场景' : 'Agent Team Monitor · 监控面板'
        );
    } catch (error) {
        console.error('Failed to update desktop window title:', error);
    }

    if (refreshButton && typeof onRefresh === 'function') {
        refreshButton.addEventListener('click', () => {
            onRefresh();
        });
    }

    if (hideButton) {
        hideButton.addEventListener('click', async () => {
            try {
                await DESKTOP_BRIDGE.hideDesktopWindow();
            } catch (error) {
                console.error('Failed to hide desktop window:', error);
            }
        });
    }

    if (aboutButton) {
        aboutButton.addEventListener('click', async () => {
            try {
                await DESKTOP_BRIDGE.openDesktopAbout();
            } catch (error) {
                console.error('Failed to open desktop about window:', error);
            }
        });
    }

    if (settingsButton && settingsPanel) {
        settingsButton.addEventListener('click', (event) => {
            event.stopPropagation();
            DESKTOP_BRIDGE.openDesktopPreferences().catch((error) => {
                console.error('Failed to open native preferences window:', error);
                settingsPanel.hidden = !settingsPanel.hidden;
            });
        });
    }

    if (hideIdleToggle) {
        hideIdleToggle.addEventListener('change', async () => {
            const saved = await saveDesktopPreferences({
                ...desktopPreferencesCache,
                hideIdleAgents: Boolean(hideIdleToggle.checked),
            });
            if (typeof onPreferencesChanged === 'function') {
                onPreferencesChanged({ ...saved });
            }
        });
    }

    if (startupViewSelect) {
        startupViewSelect.addEventListener('change', async () => {
            const saved = await saveDesktopPreferences({
                ...desktopPreferencesCache,
                startupView: startupViewSelect.value === 'game' ? 'game' : 'dashboard',
            });
            if (typeof onPreferencesChanged === 'function') {
                onPreferencesChanged({ ...saved });
            }
        });
    }

    if (providerFilterSelect) {
        providerFilterSelect.addEventListener('change', async () => {
            const saved = await saveDesktopPreferences({
                ...desktopPreferencesCache,
                providerFilter: normalizeProviderFilter(providerFilterSelect.value),
            });
            if (typeof onPreferencesChanged === 'function') {
                onPreferencesChanged({ ...saved });
            }
        });
    }

    if (notifyTaskCompletionToggle) {
        notifyTaskCompletionToggle.addEventListener('change', async () => {
            const saved = await saveDesktopPreferences({
                ...desktopPreferencesCache,
                notifyTaskCompletion: Boolean(notifyTaskCompletionToggle.checked),
            });
            if (typeof onPreferencesChanged === 'function') {
                onPreferencesChanged({ ...saved });
            }
        });
    }

    if (notifyStaleAgentsToggle) {
        notifyStaleAgentsToggle.addEventListener('change', async () => {
            const saved = await saveDesktopPreferences({
                ...desktopPreferencesCache,
                notifyStaleAgents: Boolean(notifyStaleAgentsToggle.checked),
            });
            if (typeof onPreferencesChanged === 'function') {
                onPreferencesChanged({ ...saved });
            }
        });
    }

    if (closeToTrayToggle) {
        closeToTrayToggle.addEventListener('change', async () => {
            const saved = await saveDesktopPreferences({
                ...desktopPreferencesCache,
                closeToTray: Boolean(closeToTrayToggle.checked),
            });
            if (typeof onPreferencesChanged === 'function') {
                onPreferencesChanged({ ...saved });
            }
        });
    }

    if (launchOnLoginToggle) {
        launchOnLoginToggle.addEventListener('change', async () => {
            const saved = await saveDesktopPreferences({
                ...desktopPreferencesCache,
                launchOnLogin: Boolean(launchOnLoginToggle.checked),
            });
            if (typeof onPreferencesChanged === 'function') {
                onPreferencesChanged({ ...saved });
            }
        });
    }

    if (startMinimizedToTrayToggle) {
        startMinimizedToTrayToggle.addEventListener('change', async () => {
            const saved = await saveDesktopPreferences({
                ...desktopPreferencesCache,
                startMinimizedToTray: Boolean(startMinimizedToTrayToggle.checked),
            });
            if (typeof onPreferencesChanged === 'function') {
                onPreferencesChanged({ ...saved });
            }
        });
    }

    document.querySelectorAll('[data-theme-choice]').forEach((button) => {
        button.addEventListener('click', async () => {
            const nextTheme = normalizeTheme(button.getAttribute('data-theme-choice'));
            const saved = await saveDesktopPreferences({
                ...desktopPreferencesCache,
                theme: nextTheme,
            });
            applyThemeChoice(saved.theme);
            if (typeof onPreferencesChanged === 'function') {
                onPreferencesChanged({ ...saved });
            }
        });
    });

    if (quitButton) {
        quitButton.addEventListener('click', async () => {
            try {
                await DESKTOP_BRIDGE.quitDesktopApp();
            } catch (error) {
                console.error('Failed to quit desktop app:', error);
                alert(`退出应用失败: ${error.message}`);
            }
        });
    }

    try {
        const context = await DESKTOP_BRIDGE.getDesktopContext();
        if (contextPill) {
            const provider = String(context?.provider || 'both').toUpperCase();
            contextPill.textContent = `桌面应用 · ${provider}`;
        }
    } catch (error) {
        console.error('Failed to load desktop context:', error);
    }

    document.querySelectorAll('[data-desktop-target]').forEach((link) => {
        link.addEventListener('click', async (event) => {
            event.preventDefault();
            const target = link.getAttribute('data-desktop-target') || '/';
            try {
                await DESKTOP_BRIDGE.navigateDesktopApp(target);
            } catch (error) {
                console.error('Failed to navigate desktop app:', error);
            }
        });
    });

    window.addEventListener('atm-desktop-preferences-updated', (event) => {
        const updated = normalizePreferences(event?.detail || DEFAULT_PREFERENCES);
        desktopPreferencesCache = updated;
        applyThemeChoice(updated.theme);
        applyDesktopPreferencesToForm(updated);
        if (typeof onPreferencesChanged === 'function') {
            onPreferencesChanged({ ...updated });
        }
    });

    if (typeof onPreferencesChanged === 'function') {
        onPreferencesChanged({ ...desktopPreferencesCache });
    }

    return { ...desktopPreferencesCache };
}

export function isDesktopMode() {
    return IS_DESKTOP_MODE;
}

export function getDesktopPreferences() {
    return { ...desktopPreferencesCache };
}

export async function setDesktopPreferences(nextPrefs) {
    return saveDesktopPreferences(nextPrefs);
}

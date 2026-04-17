package main

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"runtime"
	"strings"

	"github.com/liaoweijun/agent-team-monitor/pkg/api"
	"github.com/liaoweijun/agent-team-monitor/pkg/monitor"
)

func desktopBridgeInitJS(prefs desktopPreferences) string {
	payload, err := json.Marshal(prefs)
	if err != nil {
		payload = []byte(`{}`)
	}

	return fmt.Sprintf(`
window.__ATM_DESKTOP__ = true;
window.__ATM_DESKTOP_BRIDGE_VERSION__ = 1;
window.__ATM_DESKTOP_INITIAL_PREFERENCES__ = %s;
window.__ATM_REPORT_DESKTOP_PATHNAME__ = function() {
  if (typeof window.atmDesktopReportPathname === 'function') {
    window.atmDesktopReportPathname(window.location.pathname || '/');
  }
};
window.__ATM_NATIVE_SCROLL_FALLBACK__ = function(commandOrPayload, amount) {
  var payload = commandOrPayload && typeof commandOrPayload === 'object'
    ? commandOrPayload
    : { command: commandOrPayload, amount: amount, x: arguments[2], y: arguments[3] };
  var command = String(payload.command || 'by');
  var delta = Number(payload.amount || 0);
  var pointerX = Number(payload.x);
  var pointerY = Number(payload.y);
  var hasPointer = Number.isFinite(pointerX) && Number.isFinite(pointerY);
  var pathname = window.location.pathname || '';

  function applyScroll(root) {
    if (!root) {
      return false;
    }

    var maxScrollTop = Math.max(0, root.scrollHeight - root.clientHeight);
    var before = root.scrollTop || 0;

    if (command === 'top') {
      root.scrollTop = 0;
    } else if (command === 'bottom') {
      root.scrollTop = maxScrollTop;
    } else {
      var next = before + delta;
      if (next < 0) {
        next = 0;
      }
      if (next > maxScrollTop) {
        next = maxScrollTop;
      }
      root.scrollTop = next;
    }

    return root.scrollTop !== before;
  }

  function canScroll(node) {
    if (!(node instanceof HTMLElement)) {
      return false;
    }

    var overflowY = window.getComputedStyle(node).overflowY || '';
    if (!/(auto|scroll|overlay)/.test(overflowY)) {
      return false;
    }

    return node.scrollHeight > node.clientHeight + 1;
  }

  function findScrollableTarget(start, fallback) {
    var node = start instanceof Element ? start : null;
    while (node && node !== document.body && node !== document.documentElement) {
      if (canScroll(node)) {
        return node;
      }
      if (node === fallback) {
        break;
      }
      node = node.parentElement;
    }

    return canScroll(fallback) ? fallback : null;
  }

  if (pathname.startsWith('/game')) {
    var sidebarContent = document.querySelector('.sidebar-content');
    var sidebarTabs = document.querySelector('.team-tabs');
    if (!sidebarContent && !sidebarTabs) {
      return false;
    }

    var hovered = hasPointer ? document.elementFromPoint(pointerX, pointerY) : null;
    var sidebarTarget = null;

    if (hovered instanceof Element) {
      if (sidebarContent && sidebarContent.contains(hovered)) {
        sidebarTarget = findScrollableTarget(hovered, sidebarContent);
      } else if (sidebarTabs && sidebarTabs.contains(hovered)) {
        sidebarTarget = findScrollableTarget(hovered, sidebarTabs);
      }
    }

    if (!sidebarTarget) {
      sidebarTarget = canScroll(sidebarContent) ? sidebarContent : sidebarTabs;
    }

    return applyScroll(sidebarTarget);
  }

  var root = document.getElementById('app-scroll-root') || document.scrollingElement || document.documentElement;
  return applyScroll(root);
};
window.addEventListener('popstate', window.__ATM_REPORT_DESKTOP_PATHNAME__);
window.addEventListener('hashchange', window.__ATM_REPORT_DESKTOP_PATHNAME__);
window.setTimeout(window.__ATM_REPORT_DESKTOP_PATHNAME__, 0);
`, string(payload))
}

type desktopBridge struct {
	collector   *monitor.Collector
	auth        *api.AuthManager
	view        desktopBridgeView
	provider    string
	preferences *desktopPreferencesController
	tray        *desktopTray
	windows     *desktopNativeWindows
}

func newDesktopBridge(collector *monitor.Collector, auth *api.AuthManager, provider string, preferences *desktopPreferencesController, tray *desktopTray, windows *desktopNativeWindows) *desktopBridge {
	if preferences == nil {
		preferences = newDesktopPreferencesController(newInMemoryDesktopPreferencesStore(), tray, nil)
	}

	return &desktopBridge{
		collector:   collector,
		auth:        auth,
		provider:    provider,
		preferences: preferences,
		tray:        tray,
		windows:     windows,
	}
}

func (b *desktopBridge) bind(w desktopBridgeView) error {
	if w == nil {
		return fmt.Errorf("desktop bridge requires webview")
	}
	b.view = w

	w.Init(desktopBridgeInitJS(b.getPreferences()))

	if err := w.Bind("atmDesktopGetState", b.getState); err != nil {
		return fmt.Errorf("bind atmDesktopGetState: %w", err)
	}
	if err := w.Bind("atmDesktopDeleteTeam", b.deleteTeam); err != nil {
		return fmt.Errorf("bind atmDesktopDeleteTeam: %w", err)
	}
	if err := w.Bind("atmDesktopSendAgentMessage", b.sendAgentMessage); err != nil {
		return fmt.Errorf("bind atmDesktopSendAgentMessage: %w", err)
	}
	if err := w.Bind("atmDesktopGetContext", b.getContext); err != nil {
		return fmt.Errorf("bind atmDesktopGetContext: %w", err)
	}
	if err := w.Bind("atmDesktopQuit", b.quit); err != nil {
		return fmt.Errorf("bind atmDesktopQuit: %w", err)
	}
	if err := w.Bind("atmDesktopNavigate", b.navigate); err != nil {
		return fmt.Errorf("bind atmDesktopNavigate: %w", err)
	}
	if err := w.Bind("atmDesktopGetPreferences", b.getPreferences); err != nil {
		return fmt.Errorf("bind atmDesktopGetPreferences: %w", err)
	}
	if err := w.Bind("atmDesktopSetPreferences", b.setPreferences); err != nil {
		return fmt.Errorf("bind atmDesktopSetPreferences: %w", err)
	}
	if err := w.Bind("atmDesktopOpenExternal", b.openExternal); err != nil {
		return fmt.Errorf("bind atmDesktopOpenExternal: %w", err)
	}
	if err := w.Bind("atmDesktopSetWindowTitle", b.setWindowTitle); err != nil {
		return fmt.Errorf("bind atmDesktopSetWindowTitle: %w", err)
	}
	if err := w.Bind("atmDesktopHideWindow", b.hideWindow); err != nil {
		return fmt.Errorf("bind atmDesktopHideWindow: %w", err)
	}
	if err := w.Bind("atmDesktopShowWindow", b.showWindow); err != nil {
		return fmt.Errorf("bind atmDesktopShowWindow: %w", err)
	}
	if err := w.Bind("atmDesktopOpenPreferences", b.openPreferencesWindow); err != nil {
		return fmt.Errorf("bind atmDesktopOpenPreferences: %w", err)
	}
	if err := w.Bind("atmDesktopOpenAbout", b.openAboutWindow); err != nil {
		return fmt.Errorf("bind atmDesktopOpenAbout: %w", err)
	}
	if err := w.Bind("atmDesktopReportPathname", b.reportPathname); err != nil {
		return fmt.Errorf("bind atmDesktopReportPathname: %w", err)
	}
	return nil
}

func (b *desktopBridge) getState() (json.RawMessage, error) {
	if b == nil || b.collector == nil {
		return nil, fmt.Errorf("desktop bridge collector unavailable")
	}

	state := b.collector.GetState()
	payload, err := json.Marshal(state)
	if err != nil {
		return nil, err
	}

	return json.RawMessage(payload), nil
}

func (b *desktopBridge) requireAdmin() error {
	if b == nil || b.auth == nil {
		return fmt.Errorf("admin login not configured")
	}
	return b.auth.RequireAuthenticated()
}

func (b *desktopBridge) deleteTeam(teamName string) (map[string]interface{}, error) {
	if b == nil || b.collector == nil {
		return nil, fmt.Errorf("desktop bridge collector unavailable")
	}
	if err := b.requireAdmin(); err != nil {
		return nil, err
	}

	if err := b.collector.DeleteTeam(teamName); err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"status":  "ok",
		"message": "Team deleted",
	}, nil
}

func (b *desktopBridge) sendAgentMessage(teamName, agentName, text string) (map[string]interface{}, error) {
	if b == nil || b.collector == nil {
		return nil, fmt.Errorf("desktop bridge collector unavailable")
	}
	if err := b.requireAdmin(); err != nil {
		return nil, err
	}

	if err := b.collector.SendAgentMessage(teamName, agentName, text); err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"status":  "ok",
		"message": "Message queued",
	}, nil
}

func (b *desktopBridge) getContext() map[string]interface{} {
	prefs := defaultDesktopPreferences()
	if b != nil && b.preferences != nil {
		prefs = b.preferences.Get()
	}

	return map[string]interface{}{
		"mode":        "desktop",
		"provider":    b.provider,
		"platform":    runtime.GOOS,
		"startupView": prefs.StartupView,
	}
}

func (b *desktopBridge) quit() error {
	if b == nil || b.view == nil {
		return fmt.Errorf("desktop bridge view unavailable")
	}

	if b.tray != nil {
		b.tray.allowNextCloseToQuit()
	}

	b.view.Dispatch(func() {
		b.view.Terminate()
	})
	return nil
}

func (b *desktopBridge) navigate(target string) error {
	if b == nil || b.view == nil {
		return fmt.Errorf("desktop bridge view unavailable")
	}

	destination := strings.TrimSpace(target)
	if destination == "" {
		destination = "/"
	}

	b.view.Dispatch(func() {
		b.view.Navigate(destination)
	})
	return nil
}

func (b *desktopBridge) reportPathname(pathname string) {
	type pathReporter interface {
		SetPathname(string)
	}

	reporter, ok := b.view.(pathReporter)
	if !ok {
		return
	}

	reporter.SetPathname(pathname)
}

func (b *desktopBridge) getPreferences() desktopPreferences {
	if b == nil || b.preferences == nil {
		return defaultDesktopPreferences()
	}

	return b.preferences.Get()
}

func (b *desktopBridge) setPreferences(input desktopPreferences) (desktopPreferences, error) {
	if b == nil || b.preferences == nil {
		return defaultDesktopPreferences(), fmt.Errorf("desktop preferences unavailable")
	}
	if err := b.requireAdmin(); err != nil {
		return defaultDesktopPreferences(), err
	}

	return b.preferences.Set(input)
}

func (b *desktopBridge) openExternal(target string) error {
	address := strings.TrimSpace(target)
	if address == "" {
		return fmt.Errorf("external URL is required")
	}
	if !strings.HasPrefix(address, "https://") && !strings.HasPrefix(address, "http://") {
		return fmt.Errorf("unsupported external URL %q", target)
	}

	cmd, err := desktopOpenCommand(address)
	if err != nil {
		return err
	}

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("open external URL: %w", err)
	}

	return nil
}

func desktopOpenCommand(address string) (*exec.Cmd, error) {
	switch runtime.GOOS {
	case "darwin":
		return exec.Command("open", address), nil
	case "windows":
		return exec.Command("rundll32", "url.dll,FileProtocolHandler", address), nil
	case "linux":
		return exec.Command("xdg-open", address), nil
	default:
		return nil, fmt.Errorf("open external URL unsupported on %s", runtime.GOOS)
	}
}

func (b *desktopBridge) setWindowTitle(title string) error {
	if b == nil || b.view == nil {
		return fmt.Errorf("desktop bridge view unavailable")
	}

	nextTitle := strings.TrimSpace(title)
	if nextTitle == "" {
		nextTitle = windowTitle
	}

	b.view.Dispatch(func() {
		b.view.SetTitle(nextTitle)
	})
	return nil
}

func (b *desktopBridge) hideWindow() error {
	if b == nil || b.tray == nil {
		return fmt.Errorf("desktop tray unavailable")
	}

	b.tray.hideWindow()
	return nil
}

func (b *desktopBridge) showWindow() error {
	if b == nil || b.tray == nil {
		return fmt.Errorf("desktop tray unavailable")
	}

	b.tray.showWindow()
	return nil
}

func (b *desktopBridge) openPreferencesWindow() error {
	if b == nil || b.windows == nil {
		return fmt.Errorf("desktop native windows unavailable")
	}
	if err := b.requireAdmin(); err != nil {
		return err
	}

	b.windows.showPreferences()
	return nil
}

func (b *desktopBridge) openAboutWindow() error {
	if b == nil || b.windows == nil {
		return fmt.Errorf("desktop native windows unavailable")
	}

	b.windows.showAbout()
	return nil
}

func (b *desktopBridge) refreshView() {
	if b == nil || b.view == nil {
		return
	}

	b.view.Eval(`window.location.reload();`)
}

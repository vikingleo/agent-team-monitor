package main

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"runtime"
	"strings"

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
window.__ATM_NATIVE_SCROLL_FALLBACK__ = function(command, amount) {
  if ((window.location.pathname || '').startsWith('/game')) {
    return false;
  }

  var root = document.getElementById('app-scroll-root') || document.scrollingElement || document.documentElement;
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
    var delta = Number(amount || 0);
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
};
`, string(payload))
}

type desktopBridge struct {
	collector   *monitor.Collector
	view        desktopBridgeView
	provider    string
	preferences *desktopPreferencesController
	tray        *desktopTray
	windows     *desktopNativeWindows
}

func newDesktopBridge(collector *monitor.Collector, provider string, preferences *desktopPreferencesController, tray *desktopTray, windows *desktopNativeWindows) *desktopBridge {
	if preferences == nil {
		preferences = newDesktopPreferencesController(newInMemoryDesktopPreferencesStore(), tray, nil)
	}

	return &desktopBridge{
		collector:   collector,
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

func (b *desktopBridge) deleteTeam(teamName string) (map[string]interface{}, error) {
	if b == nil || b.collector == nil {
		return nil, fmt.Errorf("desktop bridge collector unavailable")
	}

	if err := b.collector.DeleteTeam(teamName); err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"status":  "ok",
		"message": "Team deleted",
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

	cmd := exec.Command("xdg-open", address)
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("open external URL: %w", err)
	}

	return nil
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

package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"unsafe"

	"github.com/liaoweijun/agent-team-monitor/pkg/monitor"
	"github.com/liaoweijun/agent-team-monitor/pkg/types"
)

type stubDesktopBridgeView struct {
	titles      []string
	navigations []string
	evals       []string
}

func (s *stubDesktopBridgeView) Dispatch(fn func()) {
	if fn != nil {
		fn()
	}
}

func (s *stubDesktopBridgeView) Window() unsafe.Pointer { return nil }
func (s *stubDesktopBridgeView) Terminate()             {}
func (s *stubDesktopBridgeView) SetTitle(title string)  { s.titles = append(s.titles, title) }
func (s *stubDesktopBridgeView) Init(string)            {}
func (s *stubDesktopBridgeView) Eval(js string)         { s.evals = append(s.evals, js) }
func (s *stubDesktopBridgeView) Navigate(target string) {
	s.navigations = append(s.navigations, target)
}
func (s *stubDesktopBridgeView) Bind(string, interface{}) error { return nil }

func newTestDesktopPreferencesController() *desktopPreferencesController {
	return newDesktopPreferencesController(newInMemoryDesktopPreferencesStore(), nil, nil)
}

func TestDesktopBridgeGetState_ReturnsJSONPayload(t *testing.T) {
	collector, err := monitor.NewCollector()
	if err != nil {
		t.Fatalf("new collector: %v", err)
	}
	defer func() {
		_ = collector.Stop()
	}()

	bridge := newDesktopBridge(collector, "both", newTestDesktopPreferencesController(), nil, nil)
	raw, err := bridge.getState()
	if err != nil {
		t.Fatalf("getState returned error: %v", err)
	}

	var decoded types.MonitorState
	if err := json.Unmarshal(raw, &decoded); err != nil {
		t.Fatalf("bridge payload should be valid monitor state JSON: %v", err)
	}
}

func TestDesktopBridgeDeleteTeam_RemovesClaudeTeamArtifacts(t *testing.T) {
	tempHome := t.TempDir()
	t.Setenv("HOME", tempHome)

	teamName := "desktop-bridge-team"
	teamDir := filepath.Join(tempHome, ".claude", "teams", teamName)
	taskDir := filepath.Join(tempHome, ".claude", "tasks", teamName)

	if err := os.MkdirAll(teamDir, 0o755); err != nil {
		t.Fatalf("create team dir: %v", err)
	}
	if err := os.MkdirAll(taskDir, 0o755); err != nil {
		t.Fatalf("create task dir: %v", err)
	}

	collector, err := monitor.NewCollector()
	if err != nil {
		t.Fatalf("new collector: %v", err)
	}
	defer func() {
		_ = collector.Stop()
	}()

	bridge := newDesktopBridge(collector, "both", newTestDesktopPreferencesController(), nil, nil)
	result, err := bridge.deleteTeam(teamName)
	if err != nil {
		t.Fatalf("deleteTeam returned error: %v", err)
	}

	if result["status"] != "ok" {
		t.Fatalf("unexpected delete result: %#v", result)
	}

	if _, err := os.Stat(teamDir); !os.IsNotExist(err) {
		t.Fatalf("expected team dir to be removed, stat err = %v", err)
	}
	if _, err := os.Stat(taskDir); !os.IsNotExist(err) {
		t.Fatalf("expected task dir to be removed, stat err = %v", err)
	}
}

func TestDesktopBridgeGetContext_ReturnsDesktopMetadata(t *testing.T) {
	collector, err := monitor.NewCollector()
	if err != nil {
		t.Fatalf("new collector: %v", err)
	}
	defer func() {
		_ = collector.Stop()
	}()

	bridge := newDesktopBridge(collector, "both", newTestDesktopPreferencesController(), nil, nil)
	context := bridge.getContext()

	if context["mode"] != "desktop" {
		t.Fatalf("unexpected mode: %#v", context)
	}
	if context["provider"] != "both" {
		t.Fatalf("unexpected provider: %#v", context)
	}
}

func TestDesktopBridgePreferencesRoundTrip(t *testing.T) {
	bridge := newDesktopBridge(nil, "both", newTestDesktopPreferencesController(), nil, nil)

	prefs := bridge.getPreferences()
	if prefs != defaultDesktopPreferences() {
		t.Fatalf("unexpected default preferences: %#v", prefs)
	}

	saved, err := bridge.setPreferences(desktopPreferences{
		HideIdleAgents:       false,
		StartupView:          "game",
		ProviderFilter:       "claude",
		Theme:                "dark",
		NotifyTaskCompletion: false,
		NotifyStaleAgents:    false,
		CloseToTray:          false,
		LaunchOnLogin:        true,
		StartMinimizedToTray: true,
	})
	if err != nil {
		t.Fatalf("setPreferences returned error: %v", err)
	}

	if saved.StartupView != "game" || saved.ProviderFilter != "claude" || saved.Theme != "dark" || saved.HideIdleAgents || saved.NotifyTaskCompletion || saved.NotifyStaleAgents || saved.CloseToTray || !saved.LaunchOnLogin || !saved.StartMinimizedToTray {
		t.Fatalf("unexpected saved preferences: %#v", saved)
	}
}

func TestDesktopBridgeOpenExternal_RejectsUnsupportedSchemes(t *testing.T) {
	bridge := newDesktopBridge(nil, "both", newTestDesktopPreferencesController(), nil, nil)

	if err := bridge.openExternal("file:///tmp/test"); err == nil {
		t.Fatal("expected openExternal to reject non-http URL")
	}
}

func TestDesktopBridgeNativeWindowActionsRequireNativeWindows(t *testing.T) {
	bridge := newDesktopBridge(nil, "both", newTestDesktopPreferencesController(), nil, nil)

	if err := bridge.openPreferencesWindow(); err == nil {
		t.Fatal("expected openPreferencesWindow to fail without native window support")
	}
	if err := bridge.openAboutWindow(); err == nil {
		t.Fatal("expected openAboutWindow to fail without native window support")
	}
}

func TestDesktopBridgeNavigateUsesViewNavigation(t *testing.T) {
	view := &stubDesktopBridgeView{}
	bridge := newDesktopBridge(nil, "both", newTestDesktopPreferencesController(), nil, nil)
	bridge.view = view

	if err := bridge.navigate("/game/"); err != nil {
		t.Fatalf("navigate returned error: %v", err)
	}

	if len(view.navigations) != 1 || view.navigations[0] != "/game/" {
		t.Fatalf("expected view navigation to /game/, got %#v", view.navigations)
	}
	if len(view.titles) != 0 {
		t.Fatalf("expected navigate to avoid SetTitle side effects, got %#v", view.titles)
	}
}

func TestDesktopBridgeRefreshViewReloadsPage(t *testing.T) {
	view := &stubDesktopBridgeView{}
	bridge := newDesktopBridge(nil, "both", newTestDesktopPreferencesController(), nil, nil)
	bridge.view = view

	bridge.refreshView()

	if len(view.evals) != 1 || view.evals[0] != `window.location.reload();` {
		t.Fatalf("expected refreshView to reload page, got %#v", view.evals)
	}
}

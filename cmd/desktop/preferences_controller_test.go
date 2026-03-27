package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDesktopPreferencesControllerSetAppliesAutostart(t *testing.T) {
	store := newInMemoryDesktopPreferencesStore()
	autostartPath := filepath.Join(t.TempDir(), "autostart", "agent-team-monitor.desktop")
	controller := newDesktopPreferencesController(store, nil, newTestDesktopAutostartManager(autostartPath, "/tmp/agent-team-monitor-desktop"))

	saved, err := controller.Set(desktopPreferences{
		HideIdleAgents:       true,
		StartupView:          "dashboard",
		ProviderFilter:       "all",
		Theme:                "light",
		NotifyTaskCompletion: true,
		NotifyStaleAgents:    true,
		CloseToTray:          true,
		LaunchOnLogin:        true,
		StartMinimizedToTray: true,
	})
	if err != nil {
		t.Fatalf("Set returned error: %v", err)
	}
	if !saved.LaunchOnLogin || !saved.StartMinimizedToTray {
		t.Fatalf("unexpected saved preferences: %#v", saved)
	}

	content, err := os.ReadFile(autostartPath)
	if err != nil {
		t.Fatalf("expected autostart file to be written: %v", err)
	}
	if len(content) == 0 {
		t.Fatal("expected autostart file to be non-empty")
	}
}

func TestDesktopPreferencesControllerDisablesAutostart(t *testing.T) {
	autostartPath := filepath.Join(t.TempDir(), "autostart", "agent-team-monitor.desktop")
	if err := os.MkdirAll(filepath.Dir(autostartPath), 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	if err := os.WriteFile(autostartPath, []byte("existing"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	controller := newDesktopPreferencesController(newInMemoryDesktopPreferencesStore(), nil, newTestDesktopAutostartManager(autostartPath, "/tmp/agent-team-monitor-desktop"))
	_, err := controller.Set(defaultDesktopPreferences())
	if err != nil {
		t.Fatalf("Set returned error: %v", err)
	}
	if _, err := os.Stat(autostartPath); !os.IsNotExist(err) {
		t.Fatalf("expected autostart file removed, stat err = %v", err)
	}
}

func TestBuildDesktopAutostartEntryEscapesExecPath(t *testing.T) {
	entry := buildDesktopAutostartEntry("/opt/Agent Team Monitor/agent-team-monitor-desktop")
	if got, want := entry, "Exec=/opt/Agent\\ Team\\ Monitor/agent-team-monitor-desktop"; !containsLine(got, want) {
		t.Fatalf("expected autostart entry to contain escaped Exec line %q, got:\n%s", want, got)
	}
}

func containsLine(content, line string) bool {
	for _, candidate := range splitLines(content) {
		if candidate == line {
			return true
		}
	}
	return false
}

func splitLines(content string) []string {
	lines := make([]string, 0)
	start := 0
	for i := 0; i < len(content); i++ {
		if content[i] == '\n' {
			lines = append(lines, content[start:i])
			start = i + 1
		}
	}
	if start < len(content) {
		lines = append(lines, content[start:])
	}
	return lines
}

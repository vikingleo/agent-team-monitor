package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDesktopPreferencesDefaults(t *testing.T) {
	prefs := defaultDesktopPreferences()

	if !prefs.HideIdleAgents {
		t.Fatal("expected desktop preferences to hide idle agents by default")
	}
	if prefs.StartupView != "dashboard" {
		t.Fatalf("unexpected startup view: %q", prefs.StartupView)
	}
	if prefs.ProviderFilter != "all" {
		t.Fatalf("unexpected provider filter: %q", prefs.ProviderFilter)
	}
	if prefs.Theme != "light" {
		t.Fatalf("unexpected theme: %q", prefs.Theme)
	}
	if !prefs.NotifyTaskCompletion {
		t.Fatal("expected task completion notifications enabled by default")
	}
	if !prefs.NotifyStaleAgents {
		t.Fatal("expected stale agent notifications enabled by default")
	}
	if !prefs.CloseToTray {
		t.Fatal("expected close-to-tray enabled by default")
	}
	if prefs.LaunchOnLogin {
		t.Fatal("expected launchOnLogin disabled by default")
	}
	if prefs.StartMinimizedToTray {
		t.Fatal("expected startMinimizedToTray disabled by default")
	}
}

func TestDesktopPreferencesStartupRoute(t *testing.T) {
	if route := (desktopPreferences{StartupView: "game"}).startupRoute(); route != "/game/" {
		t.Fatalf("unexpected game route: %q", route)
	}
	if route := (desktopPreferences{StartupView: "dashboard"}).startupRoute(); route != "/" {
		t.Fatalf("unexpected dashboard route: %q", route)
	}
}

func TestDesktopPreferencesStoreSetPersistsNormalizedValues(t *testing.T) {
	configHome := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", configHome)

	store, err := newDesktopPreferencesStore()
	if err != nil {
		t.Fatalf("newDesktopPreferencesStore: %v", err)
	}

	saved, err := store.Set(desktopPreferences{
		HideIdleAgents:       false,
		StartupView:          "GAME",
		ProviderFilter:       "Codex",
		Theme:                "DARK",
		NotifyTaskCompletion: false,
		NotifyStaleAgents:    false,
		CloseToTray:          false,
		LaunchOnLogin:        true,
		StartMinimizedToTray: true,
	})
	if err != nil {
		t.Fatalf("Set returned error: %v", err)
	}

	if saved.HideIdleAgents {
		t.Fatal("expected hideIdleAgents to persist false")
	}
	if saved.StartupView != "game" {
		t.Fatalf("unexpected normalized startup view: %q", saved.StartupView)
	}
	if saved.ProviderFilter != "codex" {
		t.Fatalf("unexpected normalized provider filter: %q", saved.ProviderFilter)
	}
	if saved.Theme != "dark" {
		t.Fatalf("unexpected normalized theme: %q", saved.Theme)
	}
	if saved.NotifyTaskCompletion {
		t.Fatal("expected notifyTaskCompletion to persist false")
	}
	if saved.NotifyStaleAgents {
		t.Fatal("expected notifyStaleAgents to persist false")
	}
	if saved.CloseToTray {
		t.Fatal("expected closeToTray to persist false")
	}
	if !saved.LaunchOnLogin {
		t.Fatal("expected launchOnLogin to persist true")
	}
	if !saved.StartMinimizedToTray {
		t.Fatal("expected startMinimizedToTray to persist true")
	}

	reloaded := loadDesktopPreferences(store.path)
	if reloaded != saved {
		t.Fatalf("expected reloaded preferences to match saved value: %#v != %#v", reloaded, saved)
	}
}

func TestLoadDesktopPreferencesFallsBackOnInvalidJSON(t *testing.T) {
	configHome := t.TempDir()
	path := filepath.Join(configHome, desktopConfigDirName, desktopPreferencesFileName)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	if err := os.WriteFile(path, []byte("{invalid"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	loaded := loadDesktopPreferences(path)
	if loaded != defaultDesktopPreferences() {
		t.Fatalf("expected invalid preferences to fall back to defaults, got %#v", loaded)
	}
}

func TestLoadDesktopPreferencesDefaultsMissingNotificationFields(t *testing.T) {
	configHome := t.TempDir()
	path := filepath.Join(configHome, desktopConfigDirName, desktopPreferencesFileName)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	content := `{"startupView":"game","theme":"dark"}`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	loaded := loadDesktopPreferences(path)
	if !loaded.NotifyTaskCompletion || !loaded.NotifyStaleAgents {
		t.Fatalf("expected missing notification fields to default to true, got %#v", loaded)
	}
	if !loaded.CloseToTray {
		t.Fatalf("expected missing closeToTray field to default to true, got %#v", loaded)
	}
	if loaded.LaunchOnLogin || loaded.StartMinimizedToTray {
		t.Fatalf("expected missing startup behavior fields to default to false, got %#v", loaded)
	}
	if loaded.StartupView != "game" || loaded.Theme != "dark" {
		t.Fatalf("unexpected loaded preferences: %#v", loaded)
	}
}

func TestDesktopPreferencesPathUsesUserConfigDir(t *testing.T) {
	configHome := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", configHome)

	path, err := desktopPreferencesPath()
	if err != nil {
		t.Fatalf("desktopPreferencesPath: %v", err)
	}

	if !strings.HasPrefix(path, configHome) {
		t.Fatalf("expected desktop preferences path under %q, got %q", configHome, path)
	}
	if !strings.HasSuffix(path, filepath.Join(desktopConfigDirName, desktopPreferencesFileName)) {
		t.Fatalf("unexpected desktop preferences path: %q", path)
	}
}

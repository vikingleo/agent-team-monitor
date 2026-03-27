package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

const (
	desktopConfigDirName         = "agent-team-monitor"
	desktopPreferencesFileName   = "desktop-preferences.json"
	defaultDesktopTheme          = "light"
	defaultDesktopStartupView    = "dashboard"
	defaultDesktopProviderFilter = "all"
)

type desktopPreferences struct {
	HideIdleAgents       bool   `json:"hideIdleAgents"`
	StartupView          string `json:"startupView"`
	ProviderFilter       string `json:"providerFilter"`
	Theme                string `json:"theme"`
	NotifyTaskCompletion bool   `json:"notifyTaskCompletion"`
	NotifyStaleAgents    bool   `json:"notifyStaleAgents"`
	CloseToTray          bool   `json:"closeToTray"`
	LaunchOnLogin        bool   `json:"launchOnLogin"`
	StartMinimizedToTray bool   `json:"startMinimizedToTray"`
}

type desktopPreferencesFile struct {
	HideIdleAgents       *bool  `json:"hideIdleAgents"`
	StartupView          string `json:"startupView"`
	ProviderFilter       string `json:"providerFilter"`
	Theme                string `json:"theme"`
	NotifyTaskCompletion *bool  `json:"notifyTaskCompletion"`
	NotifyStaleAgents    *bool  `json:"notifyStaleAgents"`
	CloseToTray          *bool  `json:"closeToTray"`
	LaunchOnLogin        *bool  `json:"launchOnLogin"`
	StartMinimizedToTray *bool  `json:"startMinimizedToTray"`
}

type desktopPreferencesStore struct {
	path  string
	mu    sync.Mutex
	prefs desktopPreferences
}

func defaultDesktopPreferences() desktopPreferences {
	return desktopPreferences{
		HideIdleAgents:       true,
		StartupView:          defaultDesktopStartupView,
		ProviderFilter:       defaultDesktopProviderFilter,
		Theme:                defaultDesktopTheme,
		NotifyTaskCompletion: true,
		NotifyStaleAgents:    true,
		CloseToTray:          true,
		LaunchOnLogin:        false,
		StartMinimizedToTray: false,
	}
}

func boolPtr(value bool) *bool {
	v := value
	return &v
}

func normalizeDesktopPreferences(input desktopPreferences) desktopPreferences {
	return desktopPreferences{
		HideIdleAgents:       input.HideIdleAgents,
		StartupView:          normalizeDesktopStartupView(input.StartupView),
		ProviderFilter:       normalizeDesktopProviderFilter(input.ProviderFilter),
		Theme:                normalizeDesktopTheme(input.Theme),
		NotifyTaskCompletion: input.NotifyTaskCompletion,
		NotifyStaleAgents:    input.NotifyStaleAgents,
		CloseToTray:          input.CloseToTray,
		LaunchOnLogin:        input.LaunchOnLogin,
		StartMinimizedToTray: input.StartMinimizedToTray,
	}
}

func mergeDesktopPreferences(base desktopPreferences, input desktopPreferencesFile) desktopPreferences {
	merged := base
	if input.HideIdleAgents != nil {
		merged.HideIdleAgents = *input.HideIdleAgents
	}
	if input.NotifyTaskCompletion != nil {
		merged.NotifyTaskCompletion = *input.NotifyTaskCompletion
	}
	if input.NotifyStaleAgents != nil {
		merged.NotifyStaleAgents = *input.NotifyStaleAgents
	}
	if input.CloseToTray != nil {
		merged.CloseToTray = *input.CloseToTray
	}
	if input.LaunchOnLogin != nil {
		merged.LaunchOnLogin = *input.LaunchOnLogin
	}
	if input.StartMinimizedToTray != nil {
		merged.StartMinimizedToTray = *input.StartMinimizedToTray
	}
	if strings.TrimSpace(input.StartupView) != "" {
		merged.StartupView = input.StartupView
	}
	if strings.TrimSpace(input.ProviderFilter) != "" {
		merged.ProviderFilter = input.ProviderFilter
	}
	if strings.TrimSpace(input.Theme) != "" {
		merged.Theme = input.Theme
	}

	return normalizeDesktopPreferences(merged)
}

func normalizeDesktopStartupView(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "game":
		return "game"
	default:
		return defaultDesktopStartupView
	}
}

func normalizeDesktopProviderFilter(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "claude", "codex", "openclaw":
		return strings.ToLower(strings.TrimSpace(value))
	default:
		return defaultDesktopProviderFilter
	}
}

func normalizeDesktopTheme(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "dark":
		return "dark"
	default:
		return defaultDesktopTheme
	}
}

func (p desktopPreferences) startupRoute() string {
	if normalizeDesktopStartupView(p.StartupView) == "game" {
		return "/game/"
	}
	return "/"
}

func newDesktopPreferencesStore() (*desktopPreferencesStore, error) {
	path, err := desktopPreferencesPath()
	if err != nil {
		return nil, err
	}

	return &desktopPreferencesStore{
		path:  path,
		prefs: loadDesktopPreferences(path),
	}, nil
}

func newInMemoryDesktopPreferencesStore() *desktopPreferencesStore {
	return &desktopPreferencesStore{
		prefs: defaultDesktopPreferences(),
	}
}

func desktopPreferencesPath() (string, error) {
	configDir, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("resolve user config dir: %w", err)
	}

	return filepath.Join(configDir, desktopConfigDirName, desktopPreferencesFileName), nil
}

func loadDesktopPreferences(path string) desktopPreferences {
	defaults := defaultDesktopPreferences()
	if strings.TrimSpace(path) == "" {
		return defaults
	}

	content, err := os.ReadFile(path)
	if err != nil {
		return defaults
	}

	if len(strings.TrimSpace(string(content))) == 0 {
		return defaults
	}

	var decoded desktopPreferencesFile
	if err := json.Unmarshal(content, &decoded); err != nil {
		return defaults
	}

	return mergeDesktopPreferences(defaults, decoded)
}

func (s *desktopPreferencesStore) Get() desktopPreferences {
	if s == nil {
		return defaultDesktopPreferences()
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	return s.prefs
}

func (s *desktopPreferencesStore) Set(input desktopPreferences) (desktopPreferences, error) {
	if s == nil {
		return defaultDesktopPreferences(), fmt.Errorf("desktop preferences store unavailable")
	}

	normalized := normalizeDesktopPreferences(input)

	s.mu.Lock()
	defer s.mu.Unlock()

	if strings.TrimSpace(s.path) == "" {
		s.prefs = normalized
		return s.prefs, nil
	}

	if err := os.MkdirAll(filepath.Dir(s.path), 0o755); err != nil {
		return s.prefs, fmt.Errorf("create desktop config dir: %w", err)
	}

	payload, err := json.MarshalIndent(normalized, "", "  ")
	if err != nil {
		return s.prefs, fmt.Errorf("marshal desktop preferences: %w", err)
	}

	payload = append(payload, '\n')
	if err := os.WriteFile(s.path, payload, 0o644); err != nil {
		return s.prefs, fmt.Errorf("write desktop preferences: %w", err)
	}

	s.prefs = normalized
	return s.prefs, nil
}

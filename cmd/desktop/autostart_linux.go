//go:build linux

package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const desktopAutostartFileName = "agent-team-monitor.desktop"

type desktopAutostartManager struct {
	path            string
	resolveExecPath func() (string, error)
}

func newDesktopAutostartManager() (*desktopAutostartManager, error) {
	path, err := desktopAutostartPath()
	if err != nil {
		return nil, err
	}

	return &desktopAutostartManager{
		path:            path,
		resolveExecPath: desktopAutostartExecPath,
	}, nil
}

func newTestDesktopAutostartManager(path, execPath string) *desktopAutostartManager {
	return &desktopAutostartManager{
		path: path,
		resolveExecPath: func() (string, error) {
			return execPath, nil
		},
	}
}

func desktopAutostartPath() (string, error) {
	configDir, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("resolve autostart config dir: %w", err)
	}

	return filepath.Join(configDir, "autostart", desktopAutostartFileName), nil
}

func desktopAutostartExecPath() (string, error) {
	executable, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("resolve current executable: %w", err)
	}

	if !filepath.IsAbs(executable) {
		absolute, absErr := filepath.Abs(executable)
		if absErr == nil {
			executable = absolute
		}
	}

	return executable, nil
}

func (m *desktopAutostartManager) Apply(enabled bool) error {
	if m == nil || strings.TrimSpace(m.path) == "" {
		return nil
	}

	if !enabled {
		if err := os.Remove(m.path); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("remove desktop autostart entry: %w", err)
		}
		return nil
	}

	if m.resolveExecPath == nil {
		return fmt.Errorf("resolve desktop autostart executable: unavailable")
	}

	executable, err := m.resolveExecPath()
	if err != nil {
		return err
	}
	executable = strings.TrimSpace(executable)
	if executable == "" {
		return fmt.Errorf("resolve desktop autostart executable: empty path")
	}

	if err := os.MkdirAll(filepath.Dir(m.path), 0o755); err != nil {
		return fmt.Errorf("create autostart directory: %w", err)
	}

	payload := buildDesktopAutostartEntry(executable)
	if err := os.WriteFile(m.path, []byte(payload), 0o644); err != nil {
		return fmt.Errorf("write desktop autostart entry: %w", err)
	}

	return nil
}

func buildDesktopAutostartEntry(execPath string) string {
	return fmt.Sprintf(`[Desktop Entry]
Version=1.0
Type=Application
Name=Agent Team Monitor
Comment=Launch Agent Team Monitor in a native desktop window
Exec=%s
Icon=agent-team-monitor
Terminal=false
Categories=Development;Monitor;
Keywords=agent;monitor;claude;codex;openclaw;
StartupNotify=false
X-GNOME-Autostart-enabled=true
`, escapeDesktopExec(execPath))
}

func escapeDesktopExec(value string) string {
	var builder strings.Builder
	builder.Grow(len(value) + 8)

	for _, r := range value {
		switch r {
		case '\\', ' ', '\t', '\n', '"', '\'', '>', '<', '~', '|', '&', ';', '$', '*', '?', '#', '(', ')', '`':
			builder.WriteByte('\\')
		}
		builder.WriteRune(r)
	}

	return builder.String()
}

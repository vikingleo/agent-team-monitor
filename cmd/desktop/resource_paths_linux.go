//go:build linux

package main

import (
	"os"
	"path/filepath"
	"strings"
)

func desktopInstallPrefixCandidates() []string {
	candidates := []string{
		filepath.Join(userHomeDir(), ".local"),
		"/usr",
	}

	if appDir := strings.TrimSpace(os.Getenv("APPDIR")); appDir != "" {
		candidates = append(candidates, filepath.Join(appDir, "usr"))
	}

	if executable, err := os.Executable(); err == nil {
		resolved := executable
		if eval, evalErr := filepath.EvalSymlinks(executable); evalErr == nil {
			resolved = eval
		}
		execDir := filepath.Dir(resolved)
		candidates = append(candidates, filepath.Clean(filepath.Join(execDir, "..")))
	}

	return uniqueExistingDesktopPaths(candidates)
}

func desktopAppRootCandidates() []string {
	candidates := []string{}

	if appDir := strings.TrimSpace(os.Getenv("APPDIR")); appDir != "" {
		candidates = append(candidates, appDir)
	}

	if executable, err := os.Executable(); err == nil {
		resolved := executable
		if eval, evalErr := filepath.EvalSymlinks(executable); evalErr == nil {
			resolved = eval
		}
		execDir := filepath.Dir(resolved)
		candidates = append(candidates, filepath.Clean(filepath.Join(execDir, "..", "..")))
	}

	if cwd, err := os.Getwd(); err == nil {
		candidates = append(candidates, cwd)
	}

	return uniqueExistingDesktopPaths(candidates)
}

func uniqueExistingDesktopPaths(paths []string) []string {
	seen := make(map[string]struct{}, len(paths))
	unique := make([]string, 0, len(paths))

	for _, candidate := range paths {
		candidate = strings.TrimSpace(candidate)
		if candidate == "" {
			continue
		}
		clean := filepath.Clean(candidate)
		if _, exists := seen[clean]; exists {
			continue
		}
		seen[clean] = struct{}{}
		unique = append(unique, clean)
	}

	return unique
}

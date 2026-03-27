package app

import "strings"

// FormatVersionLabel normalizes the CLI/Desktop version label so callers do not
// accidentally print duplicate "v" prefixes when the build metadata already
// contains a semantic-version tag like "v1.5.0".
func FormatVersionLabel(name, version string) string {
	trimmedName := strings.TrimSpace(name)
	trimmedVersion := strings.TrimSpace(version)

	if trimmedVersion == "" {
		return trimmedName
	}

	if strings.HasPrefix(strings.ToLower(trimmedVersion), "v") {
		return trimmedName + " " + trimmedVersion
	}

	return trimmedName + " v" + trimmedVersion
}

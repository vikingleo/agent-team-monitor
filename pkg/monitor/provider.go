package monitor

import (
	"fmt"
	"strings"
)

// ProviderMode controls which data sources are monitored.
type ProviderMode string

const (
	ProviderClaude   ProviderMode = "claude"
	ProviderCodex    ProviderMode = "codex"
	ProviderOpenClaw ProviderMode = "openclaw"
	ProviderBoth     ProviderMode = "both"
)

// ParseProviderMode parses provider mode from CLI/env inputs.
func ParseProviderMode(raw string) (ProviderMode, error) {
	value := strings.ToLower(strings.TrimSpace(raw))
	if value == "" {
		return ProviderBoth, nil
	}

	switch ProviderMode(value) {
	case ProviderClaude, ProviderCodex, ProviderOpenClaw, ProviderBoth:
		return ProviderMode(value), nil
	default:
		return "", fmt.Errorf("invalid provider %q (expected: claude, codex, openclaw, both)", raw)
	}
}

// normalizeProviderMode applies defaults and falls back to both on invalid values.
func normalizeProviderMode(raw ProviderMode) ProviderMode {
	mode, err := ParseProviderMode(string(raw))
	if err != nil {
		return ProviderBoth
	}
	return mode
}

func (m ProviderMode) IncludesClaude() bool {
	return m == ProviderClaude || m == ProviderBoth
}

func (m ProviderMode) IncludesCodex() bool {
	return m == ProviderCodex || m == ProviderBoth
}

func (m ProviderMode) IncludesOpenClaw() bool {
	return m == ProviderOpenClaw || m == ProviderBoth
}

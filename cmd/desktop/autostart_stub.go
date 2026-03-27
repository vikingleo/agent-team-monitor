//go:build !linux

package main

type desktopAutostartManager struct{}

func newDesktopAutostartManager() (*desktopAutostartManager, error) { return nil, nil }
func (m *desktopAutostartManager) Apply(enabled bool) error         { return nil }

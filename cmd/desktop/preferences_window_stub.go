//go:build !linux

package main

type desktopNativeWindows struct{}

func newDesktopNativeWindows(host desktopUIHost, preferences *desktopPreferencesController, provider, version string, onUpdated func(desktopPreferences)) *desktopNativeWindows {
	return nil
}

func (n *desktopNativeWindows) install()         {}
func (n *desktopNativeWindows) destroy()         {}
func (n *desktopNativeWindows) showPreferences() {}
func (n *desktopNativeWindows) showAbout()       {}

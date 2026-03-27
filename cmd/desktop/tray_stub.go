//go:build !linux

package main

import "time"

type desktopTray struct{}

func newDesktopTray(host desktopUIHost) *desktopTray      { return nil }
func (t *desktopTray) install() error                     { return nil }
func (t *desktopTray) destroy()                           {}
func (t *desktopTray) showWindow()                        {}
func (t *desktopTray) hideWindow()                        {}
func (t *desktopTray) hideWindowSoon(delay time.Duration) {}
func (t *desktopTray) allowNextCloseToQuit()              {}
func (t *desktopTray) clearQuitIntent()                   {}
func (t *desktopTray) setCloseToTrayEnabled(enabled bool) {}

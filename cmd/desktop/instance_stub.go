//go:build !linux

package main

type desktopSingleInstance struct{}

func acquireDesktopSingleInstance() (*desktopSingleInstance, bool, error) {
	return &desktopSingleInstance{}, false, nil
}

func (i *desktopSingleInstance) SetActivateHandler(fn func()) {}

func (i *desktopSingleInstance) Close() error { return nil }

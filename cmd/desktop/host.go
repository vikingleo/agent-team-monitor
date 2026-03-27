package main

import "unsafe"

type desktopUIHost interface {
	Dispatch(func())
	Window() unsafe.Pointer
	Terminate()
	SetTitle(string)
}

type desktopBridgeView interface {
	desktopUIHost
	Init(string)
	Eval(string)
	Navigate(string)
	Bind(string, interface{}) error
}

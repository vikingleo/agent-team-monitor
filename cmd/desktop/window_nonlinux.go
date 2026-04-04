//go:build !linux

package main

import (
	"fmt"
	"os"
	"reflect"
	"strings"
	"sync"
	"sync/atomic"
	"time"
	"unsafe"

	agentapp "github.com/liaoweijun/agent-team-monitor/internal/app"
	webview "github.com/webview/webview_go"
)

type desktopMainWindow struct {
	provider    string
	title       string
	version     string
	preferences *desktopPreferencesController
	tray        *desktopTray
	native      *desktopNativeWindows
	bridge      *desktopBridge
	session     *agentapp.WebSession
	webview     webview.WebView

	mu             sync.Mutex
	quit           sync.Once
	dispatchClosed atomic.Bool
}

func newDesktopMainWindow(session *agentapp.WebSession, preferences *desktopPreferencesController, provider, version string) (*desktopMainWindow, error) {
	if session == nil {
		return nil, fmt.Errorf("desktop web session unavailable")
	}

	if preferences == nil {
		preferences = newDesktopPreferencesController(newInMemoryDesktopPreferencesStore(), nil, nil)
	}

	wv := webview.New(desktopWebViewDebugEnabled())
	if !validDesktopWebView(wv) {
		return nil, fmt.Errorf("create webview: native window unavailable (%s)", desktopDisplayFailureContext())
	}

	w := &desktopMainWindow{
		provider:    provider,
		title:       windowTitle,
		version:     version,
		preferences: preferences,
		session:     session,
		webview:     wv,
	}

	w.SetTitle(windowTitle)
	w.webview.SetSize(1480, 980, webview.HintNone)
	return w, nil
}

func validDesktopWebView(view webview.WebView) bool {
	if view == nil {
		return false
	}

	value := reflect.ValueOf(view)
	if !value.IsValid() {
		return false
	}
	if value.Kind() == reflect.Pointer && value.IsNil() {
		return false
	}

	if value.Kind() == reflect.Pointer {
		elem := value.Elem()
		if elem.IsValid() {
			handle := elem.FieldByName("w")
			if handle.IsValid() {
				return !handle.IsZero()
			}
		}
	}

	return true
}

func desktopDisplayPreflight() error {
	return nil
}

func desktopDisplayFailureContext() string {
	return runtimeDesktopDisplayContext()
}

func runtimeDesktopDisplayContext() string {
	return "native desktop runtime unavailable"
}

func (w *desktopMainWindow) attachBridge(bridge *desktopBridge) error {
	if w == nil || w.webview == nil {
		return fmt.Errorf("desktop webview unavailable")
	}
	if bridge == nil {
		return fmt.Errorf("desktop bridge unavailable")
	}

	if err := bridge.bind(w); err != nil {
		return err
	}
	w.bridge = bridge
	return nil
}

func (w *desktopMainWindow) loadInitialView() {
	if w == nil || w.webview == nil || w.session == nil {
		return
	}

	prefs := defaultDesktopPreferences()
	if w.preferences != nil {
		prefs = w.preferences.Get()
	}

	target := strings.TrimRight(w.session.BaseURL, "/") + prefs.startupRoute()
	w.webview.Navigate(target)
}

func (w *desktopMainWindow) Run() {
	if w == nil || w.webview == nil {
		return
	}

	w.loadInitialView()
	w.webview.Run()
	w.dispatchClosed.Store(true)
}

func (w *desktopMainWindow) Dispatch(fn func()) {
	if w == nil || w.webview == nil || fn == nil || w.dispatchClosed.Load() || !validDesktopWebView(w.webview) {
		return
	}
	w.webview.Dispatch(fn)
}

func (w *desktopMainWindow) Window() unsafe.Pointer {
	if w == nil || w.webview == nil {
		return nil
	}
	return w.webview.Window()
}

func (w *desktopMainWindow) Present() {
	if w == nil || w.webview == nil {
		return
	}

	w.Dispatch(func() {
		w.webview.Eval(`window.focus && window.focus();`)
	})
}

func (w *desktopMainWindow) Terminate() {
	if w == nil {
		return
	}

	w.quit.Do(func() {
		w.dispatchClosed.Store(true)
		if w.webview != nil {
			w.webview.Terminate()
		}
	})
}

func (w *desktopMainWindow) SetTitle(title string) {
	if w == nil || w.webview == nil {
		return
	}

	nextTitle := strings.TrimSpace(title)
	if nextTitle == "" {
		nextTitle = windowTitle
	}

	w.mu.Lock()
	w.title = nextTitle
	w.mu.Unlock()
	w.webview.SetTitle(nextTitle)
}

func (w *desktopMainWindow) Init(js string) {
	if w == nil || w.webview == nil {
		return
	}
	w.webview.Init(js)
}

func (w *desktopMainWindow) Eval(js string) {
	if w == nil || w.webview == nil {
		return
	}
	w.webview.Eval(js)
}

func (w *desktopMainWindow) Navigate(target string) {
	if w == nil || w.webview == nil || w.session == nil {
		return
	}

	next := strings.TrimSpace(target)
	if next == "" {
		next = "/"
	}

	if strings.HasPrefix(next, "http://") || strings.HasPrefix(next, "https://") {
		w.webview.Navigate(next)
		return
	}

	base := strings.TrimRight(w.session.BaseURL, "/")
	if !strings.HasPrefix(next, "/") {
		next = "/" + next
	}
	w.webview.Navigate(base + next)
}

func (w *desktopMainWindow) Bind(name string, fn interface{}) error {
	if w == nil || w.webview == nil {
		return fmt.Errorf("desktop webview unavailable")
	}
	return w.webview.Bind(name, fn)
}

func (w *desktopMainWindow) Destroy() {
	if w == nil || w.webview == nil {
		return
	}
	w.dispatchClosed.Store(true)
	w.webview.Destroy()
}

func (w *desktopMainWindow) reloadCurrentViewSoon(delay time.Duration) {
	if w == nil || w.bridge == nil {
		return
	}
	if delay < 0 {
		delay = 0
	}

	go func() {
		if delay > 0 {
			time.Sleep(delay)
		}
		w.Dispatch(func() {
			w.bridge.refreshView()
		})
	}()
}

func desktopWebViewDebugEnabled() bool {
	value := strings.ToLower(strings.TrimSpace(os.Getenv("ATM_DESKTOP_WEBVIEW_DEBUG")))
	switch value {
	case "1", "true", "yes", "on":
		return true
	default:
		return false
	}
}

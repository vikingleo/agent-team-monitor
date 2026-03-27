//go:build linux

package main

/*
#cgo pkg-config: gtk+-3.0
#include <gtk/gtk.h>
*/
import "C"

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"time"
	"unsafe"

	agentapp "github.com/liaoweijun/agent-team-monitor/internal/app"
	webview "github.com/webview/webview_go"
)

const desktopX11SocketLimit = 512

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

	mu   sync.Mutex
	quit sync.Once
}

func newDesktopMainWindow(session *agentapp.WebSession, preferences *desktopPreferencesController, provider, version string) (*desktopMainWindow, error) {
	if session == nil {
		return nil, fmt.Errorf("desktop web session unavailable")
	}

	if preferences == nil {
		preferences = newDesktopPreferencesController(newInMemoryDesktopPreferencesStore(), nil, nil)
	}

	wv := webview.New(false)
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
	w.applyWindowIcon()
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

func desktopDisplayContext() string {
	parts := make([]string, 0, 3)
	if display := strings.TrimSpace(os.Getenv("DISPLAY")); display != "" {
		parts = append(parts, "DISPLAY="+display)
	}
	if wayland := strings.TrimSpace(os.Getenv("WAYLAND_DISPLAY")); wayland != "" {
		parts = append(parts, "WAYLAND_DISPLAY="+wayland)
	}
	if sessionType := strings.TrimSpace(os.Getenv("XDG_SESSION_TYPE")); sessionType != "" {
		parts = append(parts, "XDG_SESSION_TYPE="+sessionType)
	}
	if len(parts) == 0 {
		return "no desktop display environment detected"
	}
	return strings.Join(parts, ", ")
}

func desktopDisplayPreflight() error {
	if hint := desktopX11LimitHint(); hint != "" {
		return fmt.Errorf("%s (%s)", hint, desktopDisplayContext())
	}
	return nil
}

func desktopDisplayFailureContext() string {
	context := desktopDisplayContext()
	if hint := desktopX11LimitHint(); hint != "" {
		return context + "; " + hint
	}
	return context
}

func desktopX11LimitHint() string {
	if !desktopUsesX11Display() {
		return ""
	}

	socketPath := desktopX11SocketPath(strings.TrimSpace(os.Getenv("DISPLAY")))
	if socketPath == "" {
		return ""
	}

	count, err := countUnixSocketEntries(socketPath)
	if err != nil || count < desktopX11SocketLimit {
		return ""
	}

	return fmt.Sprintf(
		"X11 client limit reached on %s with %d active connections; close some GUI apps or restart the desktop session",
		socketPath,
		count,
	)
}

func desktopUsesX11Display() bool {
	display := strings.TrimSpace(os.Getenv("DISPLAY"))
	if display == "" {
		return false
	}

	sessionType := strings.ToLower(strings.TrimSpace(os.Getenv("XDG_SESSION_TYPE")))
	return sessionType == "" || sessionType == "x11"
}

func desktopX11SocketPath(display string) string {
	display = strings.TrimSpace(display)
	if !strings.HasPrefix(display, ":") {
		return ""
	}

	index := 1
	for index < len(display) && display[index] >= '0' && display[index] <= '9' {
		index++
	}
	if index == 1 {
		return ""
	}

	return "@/tmp/.X11-unix/X" + display[1:index]
}

func countUnixSocketEntries(path string) (int, error) {
	file, err := os.Open("/proc/net/unix")
	if err != nil {
		return 0, err
	}
	defer file.Close()

	return countUnixSocketEntriesInReader(file, path)
}

func countUnixSocketEntriesInReader(reader io.Reader, path string) (int, error) {
	scanner := bufio.NewScanner(reader)
	count := 0

	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) > 7 && fields[7] == path {
			count++
		}
	}

	if err := scanner.Err(); err != nil {
		return 0, err
	}

	return count, nil
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
	w.scheduleWindowIconRefresh()
	w.webview.Run()
}

func (w *desktopMainWindow) Dispatch(fn func()) {
	if w == nil || w.webview == nil || fn == nil {
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
		window := gtkWindowFromHost(w)
		if window == nil {
			return
		}
		C.gtk_widget_show_all(window)
		C.gtk_window_present((*C.GtkWindow)(unsafe.Pointer(window)))
	})
}

func (w *desktopMainWindow) Terminate() {
	if w == nil {
		return
	}

	w.quit.Do(func() {
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
	w.webview.Destroy()
}

func (w *desktopMainWindow) applyWindowIcon() {
	if w == nil || w.webview == nil {
		return
	}

	iconPath := desktopWindowIconPath()
	if iconPath == "" {
		return
	}

	window := gtkWindowFromHost(w)
	if window == nil {
		return
	}

	cPath := C.CString(iconPath)
	defer C.free(unsafe.Pointer(cPath))

	var err *C.GError
	pixbuf := C.gdk_pixbuf_new_from_file(cPath, &err)
	if pixbuf == nil {
		if err != nil {
			C.g_error_free(err)
		}
		return
	}
	defer C.g_object_unref(C.gpointer(pixbuf))

	C.gtk_window_set_icon((*C.GtkWindow)(unsafe.Pointer(window)), (*C.GdkPixbuf)(unsafe.Pointer(pixbuf)))
	C.gtk_window_set_default_icon((*C.GdkPixbuf)(unsafe.Pointer(pixbuf)))
	iconName := C.CString("agent-team-monitor")
	defer C.free(unsafe.Pointer(iconName))
	C.gtk_window_set_icon_name((*C.GtkWindow)(unsafe.Pointer(window)), iconName)
}

func (w *desktopMainWindow) scheduleWindowIconRefresh() {
	if w == nil {
		return
	}

	go func() {
		time.Sleep(400 * time.Millisecond)
		w.Dispatch(func() {
			w.applyWindowIcon()
		})
	}()
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

func gtkWindowFromHost(host desktopUIHost) *C.GtkWidget {
	if host == nil {
		return nil
	}
	window := host.Window()
	if window == nil {
		return nil
	}
	return (*C.GtkWidget)(window)
}

func desktopWindowIconPath() string {
	candidates := []string{}
	for _, prefix := range desktopInstallPrefixCandidates() {
		candidates = append(candidates,
			filepath.Join(prefix, "share", "icons", "hicolor", "512x512", "apps", "agent-team-monitor.png"),
			filepath.Join(prefix, "share", "icons", "hicolor", "256x256", "apps", "agent-team-monitor.png"),
			filepath.Join(prefix, "share", "icons", "hicolor", "128x128", "apps", "agent-team-monitor.png"),
			filepath.Join(prefix, "share", "agent-team-monitor", "assets", "icons", "agent-team-monitor.png"),
		)
	}
	for _, root := range desktopAppRootCandidates() {
		candidates = append(candidates, filepath.Join(root, "assets", "icons", "agent-team-monitor.png"))
	}

	for _, candidate := range candidates {
		if stat, err := os.Stat(candidate); err == nil && !stat.IsDir() {
			return candidate
		}
	}

	return ""
}

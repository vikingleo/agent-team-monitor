//go:build linux

package main

/*
#cgo pkg-config: gtk+-3.0
#include <gtk/gtk.h>

extern gboolean atmWindowScroll(GtkWidget *widget, GdkEventScroll *event, gpointer data);
extern gboolean atmWindowKeyPress(GtkWidget *widget, GdkEventKey *event, gpointer data);

static GtkWidget* atm_window_content(GtkWidget *window) {
	if (window == NULL || !GTK_IS_BIN(window)) {
		return NULL;
	}

	return gtk_bin_get_child(GTK_BIN(window));
}

static gboolean atm_repair_window_focus(GtkWidget *window, GdkEvent *event, gpointer data) {
	GtkWidget *child = atm_window_content(window);
	if (child == NULL) {
		return FALSE;
	}

	gtk_widget_set_can_focus(child, TRUE);
	gtk_widget_grab_focus(child);
	return FALSE;
}

static void atm_install_window_focus_repair(GtkWidget *window) {
	if (window == NULL) {
		return;
	}

	g_signal_connect(window, "focus-in-event", G_CALLBACK(atm_repair_window_focus), NULL);
}

static void atm_focus_window_content(GtkWidget *window) {
	atm_repair_window_focus(window, NULL, NULL);
}

static void atm_install_window_scroll_fallback(GtkWidget *window) {
	GtkWidget *child = atm_window_content(window);
	if (window == NULL || child == NULL) {
		return;
	}

	gtk_widget_add_events(child, GDK_SCROLL_MASK | GDK_SMOOTH_SCROLL_MASK | GDK_KEY_PRESS_MASK);
	gtk_widget_add_events(window, GDK_KEY_PRESS_MASK);
	g_signal_connect(child, "scroll-event", G_CALLBACK(atmWindowScroll), NULL);
	g_signal_connect(child, "key-press-event", G_CALLBACK(atmWindowKeyPress), NULL);
	g_signal_connect(window, "key-press-event", G_CALLBACK(atmWindowKeyPress), NULL);
}

static double atm_scroll_event_delta_y(GdkEventScroll *event) {
	double delta_x = 0;
	double delta_y = 0;
	if (event == NULL) {
		return 0;
	}

	if (gdk_event_get_scroll_deltas((GdkEvent *)event, &delta_x, &delta_y)) {
		return delta_y * 48.0;
	}

	switch (event->direction) {
	case GDK_SCROLL_UP:
		return -48.0;
	case GDK_SCROLL_DOWN:
		return 48.0;
	default:
		return 0;
	}
}

static double atm_scroll_event_x(GdkEventScroll *event) {
	double x = 0;
	double y = 0;
	if (event == NULL) {
		return 0;
	}

	if (gdk_event_get_coords((GdkEvent *)event, &x, &y)) {
		return x;
	}

	return 0;
}

static double atm_scroll_event_y(GdkEventScroll *event) {
	double x = 0;
	double y = 0;
	if (event == NULL) {
		return 0;
	}

	if (gdk_event_get_coords((GdkEvent *)event, &x, &y)) {
		return y;
	}

	return 0;
}

static guint atm_key_event_keyval(GdkEventKey *event) {
	if (event == NULL) {
		return 0;
	}

	return event->keyval;
}

static guint atm_key_event_state(GdkEventKey *event) {
	if (event == NULL) {
		return 0;
	}

	return event->state;
}
*/
import "C"

import (
	"bufio"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"sync/atomic"
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

	mu             sync.Mutex
	currentPath    string
	quit           sync.Once
	dispatchClosed atomic.Bool
}

var (
	desktopWindowMu     sync.Mutex
	activeDesktopWindow *desktopMainWindow
)

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
	w.applyWindowIcon()
	installDesktopWindowFocusRepair(w.Window())
	installDesktopWindowScrollFallback(w.Window())
	registerActiveDesktopWindow(w)
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

	route := prefs.startupRoute()
	w.SetPathname(route)
	target := strings.TrimRight(w.session.BaseURL, "/") + route
	w.webview.Navigate(target)
}

func (w *desktopMainWindow) Run() {
	if w == nil || w.webview == nil {
		return
	}

	w.loadInitialView()
	w.scheduleWindowIconRefresh()
	w.scheduleInitialFocusRepair()
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
		window := gtkWindowFromHost(w)
		if window == nil {
			return
		}
		C.gtk_widget_show_all(window)
		C.gtk_window_present((*C.GtkWindow)(unsafe.Pointer(window)))
		focusDesktopWindowContent(unsafe.Pointer(window))
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

	w.SetPathname(next)

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

func (w *desktopMainWindow) SetPathname(pathname string) {
	if w == nil {
		return
	}

	w.mu.Lock()
	w.currentPath = normalizeDesktopPath(pathname)
	w.mu.Unlock()
}

func (w *desktopMainWindow) usesNativeScrollFallback() bool {
	if w == nil {
		return true
	}

	w.mu.Lock()
	pathname := w.currentPath
	w.mu.Unlock()

	return !strings.HasPrefix(pathname, "/game")
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
	unregisterActiveDesktopWindow(w)
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

func installDesktopWindowFocusRepair(window unsafe.Pointer) {
	if window == nil {
		return
	}

	C.atm_install_window_focus_repair((*C.GtkWidget)(window))
}

func focusDesktopWindowContent(window unsafe.Pointer) {
	if window == nil {
		return
	}

	C.atm_focus_window_content((*C.GtkWidget)(window))
}

func (w *desktopMainWindow) scheduleInitialFocusRepair() {
	if w == nil {
		return
	}

	go func() {
		time.Sleep(250 * time.Millisecond)
		w.Dispatch(func() {
			focusDesktopWindowContent(w.Window())
		})
	}()
}

func registerActiveDesktopWindow(window *desktopMainWindow) {
	desktopWindowMu.Lock()
	activeDesktopWindow = window
	desktopWindowMu.Unlock()
}

func unregisterActiveDesktopWindow(window *desktopMainWindow) {
	desktopWindowMu.Lock()
	if activeDesktopWindow == window {
		activeDesktopWindow = nil
	}
	desktopWindowMu.Unlock()
}

func currentActiveDesktopWindow() *desktopMainWindow {
	desktopWindowMu.Lock()
	defer desktopWindowMu.Unlock()
	return activeDesktopWindow
}

func installDesktopWindowScrollFallback(window unsafe.Pointer) {
	if window == nil {
		return
	}

	C.atm_install_window_scroll_fallback((*C.GtkWidget)(window))
}

func (w *desktopMainWindow) nativeScrollBy(delta int) bool {
	if w == nil || w.webview == nil || w.dispatchClosed.Load() {
		return false
	}

	w.webview.Eval(fmt.Sprintf(`window.__ATM_NATIVE_SCROLL_FALLBACK__ && window.__ATM_NATIVE_SCROLL_FALLBACK__('by', %d);`, delta))
	return true
}

func (w *desktopMainWindow) nativeScrollAt(delta int, x, y float64) bool {
	if w == nil || w.webview == nil || w.dispatchClosed.Load() {
		return false
	}

	w.webview.Eval(fmt.Sprintf(`window.__ATM_NATIVE_SCROLL_FALLBACK__ && window.__ATM_NATIVE_SCROLL_FALLBACK__({ command: 'by', amount: %d, x: %.3f, y: %.3f });`, delta, x, y))
	return true
}

func (w *desktopMainWindow) nativeScrollToEdge(bottom bool) bool {
	if w == nil || w.webview == nil || w.dispatchClosed.Load() {
		return false
	}

	command := "top"
	if bottom {
		command = "bottom"
	}
	w.webview.Eval(fmt.Sprintf(`window.__ATM_NATIVE_SCROLL_FALLBACK__ && window.__ATM_NATIVE_SCROLL_FALLBACK__('%s', 0);`, command))
	return true
}

//export atmWindowScroll
func atmWindowScroll(widget *C.GtkWidget, event *C.GdkEventScroll, data C.gpointer) C.gboolean {
	window := currentActiveDesktopWindow()
	if window == nil {
		return C.FALSE
	}

	if !window.usesNativeScrollFallback() {
		return C.FALSE
	}

	delta := int(C.atm_scroll_event_delta_y(event))
	if delta == 0 {
		return C.FALSE
	}

	x := float64(C.atm_scroll_event_x(event))
	y := float64(C.atm_scroll_event_y(event))
	if window.nativeScrollAt(delta, x, y) {
		return C.TRUE
	}

	if window.nativeScrollBy(delta) {
		return C.TRUE
	}

	return C.FALSE
}

//export atmWindowKeyPress
func atmWindowKeyPress(widget *C.GtkWidget, event *C.GdkEventKey, data C.gpointer) C.gboolean {
	window := currentActiveDesktopWindow()
	if window == nil {
		return C.FALSE
	}

	if !window.usesNativeScrollFallback() {
		return C.FALSE
	}

	state := uint(C.atm_key_event_state(event))
	blockingModifiers := uint(C.GDK_CONTROL_MASK | C.GDK_MOD1_MASK | C.GDK_SUPER_MASK | C.GDK_META_MASK)
	if state&blockingModifiers != 0 {
		return C.FALSE
	}

	key := uint(C.atm_key_event_keyval(event))
	pageDelta := 840

	switch key {
	case uint(C.GDK_KEY_Down), uint(C.GDK_KEY_KP_Down):
		if window.nativeScrollBy(56) {
			return C.TRUE
		}
	case uint(C.GDK_KEY_Up), uint(C.GDK_KEY_KP_Up):
		if window.nativeScrollBy(-56) {
			return C.TRUE
		}
	case uint(C.GDK_KEY_Page_Down), uint(C.GDK_KEY_KP_Page_Down), uint(C.GDK_KEY_space), uint(C.GDK_KEY_KP_Space):
		if state&uint(C.GDK_SHIFT_MASK) != 0 {
			if window.nativeScrollBy(-pageDelta) {
				return C.TRUE
			}
			return C.FALSE
		}
		if window.nativeScrollBy(pageDelta) {
			return C.TRUE
		}
	case uint(C.GDK_KEY_Page_Up), uint(C.GDK_KEY_KP_Page_Up):
		if window.nativeScrollBy(-pageDelta) {
			return C.TRUE
		}
	case uint(C.GDK_KEY_Home), uint(C.GDK_KEY_KP_Home):
		if window.nativeScrollToEdge(false) {
			return C.TRUE
		}
	case uint(C.GDK_KEY_End), uint(C.GDK_KEY_KP_End):
		if window.nativeScrollToEdge(true) {
			return C.TRUE
		}
	}

	return C.FALSE
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

func normalizeDesktopPath(pathname string) string {
	trimmed := strings.TrimSpace(pathname)
	if trimmed == "" {
		return "/"
	}

	if strings.HasPrefix(trimmed, "http://") || strings.HasPrefix(trimmed, "https://") {
		parsed, err := url.Parse(trimmed)
		if err == nil && parsed != nil {
			trimmed = parsed.Path
		}
	}

	if hashIndex := strings.Index(trimmed, "#"); hashIndex >= 0 {
		trimmed = trimmed[:hashIndex]
	}
	if queryIndex := strings.Index(trimmed, "?"); queryIndex >= 0 {
		trimmed = trimmed[:queryIndex]
	}
	if trimmed == "" {
		return "/"
	}
	if !strings.HasPrefix(trimmed, "/") {
		trimmed = "/" + trimmed
	}
	return trimmed
}

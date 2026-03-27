//go:build linux

package main

/*
#cgo pkg-config: gtk+-3.0
#cgo LDFLAGS: -ldl
#include <gtk/gtk.h>
#include <dlfcn.h>
#include <stdlib.h>

extern void atmTrayActivate();
extern void atmTrayMenuShow();
extern void atmTrayMenuHide();
extern void atmTrayMenuPreferences();
extern void atmTrayMenuAbout();
extern void atmTrayMenuQuit();
extern gboolean atmWindowDelete(GtkWidget *widget, GdkEvent *event, gpointer data);

typedef struct _AppIndicator AppIndicator;
typedef AppIndicator* (*atm_app_indicator_new_fn)(const gchar *id, const gchar *icon_name, gint category);
typedef void (*atm_app_indicator_set_menu_fn)(AppIndicator *self, GtkMenu *menu);
typedef void (*atm_app_indicator_set_status_fn)(AppIndicator *self, gint status);
typedef void (*atm_app_indicator_set_title_fn)(AppIndicator *self, const gchar *title);
typedef void (*atm_app_indicator_set_icon_full_fn)(AppIndicator *self, const gchar *icon_name, const gchar *icon_desc);
typedef void (*atm_app_indicator_set_icon_theme_path_fn)(AppIndicator *self, const gchar *icon_theme_path);

static void* atm_indicator_lib = NULL;
static AppIndicator* atm_indicator = NULL;
static GtkWidget* atm_menu = NULL;
static atm_app_indicator_new_fn atm_app_indicator_new_ptr = NULL;
static atm_app_indicator_set_menu_fn atm_app_indicator_set_menu_ptr = NULL;
static atm_app_indicator_set_status_fn atm_app_indicator_set_status_ptr = NULL;
static atm_app_indicator_set_title_fn atm_app_indicator_set_title_ptr = NULL;
static atm_app_indicator_set_icon_full_fn atm_app_indicator_set_icon_full_ptr = NULL;
static atm_app_indicator_set_icon_theme_path_fn atm_app_indicator_set_icon_theme_path_ptr = NULL;

static GtkWidget* atm_new_menu_item(const char *label, GCallback callback) {
	GtkWidget *item = gtk_menu_item_new_with_label(label);
	g_signal_connect(item, "activate", callback, NULL);
	gtk_widget_show(item);
	return item;
}

static gboolean atm_load_indicator_library() {
	if (atm_indicator_lib != NULL) {
		return TRUE;
	}

	atm_indicator_lib = dlopen("libayatana-appindicator3.so.1", RTLD_NOW | RTLD_LOCAL);
	if (atm_indicator_lib == NULL) {
		return FALSE;
	}

	atm_app_indicator_new_ptr = (atm_app_indicator_new_fn)dlsym(atm_indicator_lib, "app_indicator_new");
	atm_app_indicator_set_menu_ptr = (atm_app_indicator_set_menu_fn)dlsym(atm_indicator_lib, "app_indicator_set_menu");
	atm_app_indicator_set_status_ptr = (atm_app_indicator_set_status_fn)dlsym(atm_indicator_lib, "app_indicator_set_status");
	atm_app_indicator_set_title_ptr = (atm_app_indicator_set_title_fn)dlsym(atm_indicator_lib, "app_indicator_set_title");
	atm_app_indicator_set_icon_full_ptr = (atm_app_indicator_set_icon_full_fn)dlsym(atm_indicator_lib, "app_indicator_set_icon_full");
	atm_app_indicator_set_icon_theme_path_ptr = (atm_app_indicator_set_icon_theme_path_fn)dlsym(atm_indicator_lib, "app_indicator_set_icon_theme_path");

	if (atm_app_indicator_new_ptr == NULL || atm_app_indicator_set_menu_ptr == NULL || atm_app_indicator_set_status_ptr == NULL) {
		dlclose(atm_indicator_lib);
		atm_indicator_lib = NULL;
		atm_app_indicator_new_ptr = NULL;
		atm_app_indicator_set_menu_ptr = NULL;
		atm_app_indicator_set_status_ptr = NULL;
		atm_app_indicator_set_title_ptr = NULL;
		atm_app_indicator_set_icon_full_ptr = NULL;
		atm_app_indicator_set_icon_theme_path_ptr = NULL;
		return FALSE;
	}

	return TRUE;
}

static gboolean atm_create_tray_icon(const char *icon_name, const char *icon_theme_path, const char *icon_full_path) {
	if (atm_indicator != NULL) {
		return TRUE;
	}

	if (!atm_load_indicator_library()) {
		return FALSE;
	}

	atm_menu = gtk_menu_new();
	gtk_menu_shell_append(GTK_MENU_SHELL(atm_menu), atm_new_menu_item("显示窗口", G_CALLBACK(atmTrayMenuShow)));
	gtk_menu_shell_append(GTK_MENU_SHELL(atm_menu), atm_new_menu_item("设置", G_CALLBACK(atmTrayMenuPreferences)));
	gtk_menu_shell_append(GTK_MENU_SHELL(atm_menu), atm_new_menu_item("关于", G_CALLBACK(atmTrayMenuAbout)));
	gtk_menu_shell_append(GTK_MENU_SHELL(atm_menu), atm_new_menu_item("隐藏到托盘", G_CALLBACK(atmTrayMenuHide)));
	gtk_menu_shell_append(GTK_MENU_SHELL(atm_menu), atm_new_menu_item("退出应用", G_CALLBACK(atmTrayMenuQuit)));

	// Enum values verified from upstream Ayatana AppIndicator header:
	// category application-status = 0, status active = 1, passive = 0.
	atm_indicator = atm_app_indicator_new_ptr("agent-team-monitor", icon_name, 0);
	if (atm_indicator == NULL) {
		gtk_widget_destroy(atm_menu);
		atm_menu = NULL;
		return FALSE;
	}

	if (atm_app_indicator_set_title_ptr != NULL) {
		atm_app_indicator_set_title_ptr(atm_indicator, "Agent Team Monitor");
	}
	if (atm_app_indicator_set_icon_theme_path_ptr != NULL && icon_theme_path != NULL && icon_theme_path[0] != '\0') {
		atm_app_indicator_set_icon_theme_path_ptr(atm_indicator, icon_theme_path);
	}
	if (atm_app_indicator_set_icon_full_ptr != NULL && icon_full_path != NULL && icon_full_path[0] != '\0') {
		atm_app_indicator_set_icon_full_ptr(atm_indicator, icon_full_path, "Agent Team Monitor");
	}
	atm_app_indicator_set_menu_ptr(atm_indicator, GTK_MENU(atm_menu));
	atm_app_indicator_set_status_ptr(atm_indicator, 1);
	return TRUE;
}

static gboolean atm_can_create_tray_icon() {
	return atm_load_indicator_library();
}

static void atm_destroy_tray_icon() {
	if (atm_menu != NULL) {
		gtk_widget_destroy(atm_menu);
		atm_menu = NULL;
	}
	if (atm_indicator != NULL) {
		if (atm_app_indicator_set_status_ptr != NULL) {
			atm_app_indicator_set_status_ptr(atm_indicator, 0);
		}
		g_object_unref(atm_indicator);
		atm_indicator = NULL;
	}
	if (atm_indicator_lib != NULL) {
		dlclose(atm_indicator_lib);
		atm_indicator_lib = NULL;
		atm_app_indicator_new_ptr = NULL;
		atm_app_indicator_set_menu_ptr = NULL;
		atm_app_indicator_set_status_ptr = NULL;
		atm_app_indicator_set_title_ptr = NULL;
		atm_app_indicator_set_icon_full_ptr = NULL;
		atm_app_indicator_set_icon_theme_path_ptr = NULL;
	}
}

static void atm_window_show(GtkWidget *window) {
	gtk_widget_show_all(window);
	gtk_window_present(GTK_WINDOW(window));
}

static void atm_window_hide(GtkWidget *window) {
	gtk_widget_hide(window);
}

static void atm_connect_delete_handler(GtkWidget *window) {
	g_signal_connect(window, "delete-event", G_CALLBACK(atmWindowDelete), NULL);
}
*/
import "C"

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
	"unsafe"
)

type desktopTray struct {
	host desktopUIHost
}

var (
	trayMu             sync.Mutex
	activeTray         *desktopTray
	quitFromTray       bool
	closeToTrayEnabled = true
)

func newDesktopTray(host desktopUIHost) *desktopTray {
	if host == nil {
		return nil
	}

	return &desktopTray{host: host}
}

func (t *desktopTray) install() error {
	if t == nil || t.host == nil {
		return nil
	}

	if C.atm_can_create_tray_icon() == C.FALSE {
		return fmt.Errorf("load libayatana-appindicator3.so.1")
	}

	trayMu.Lock()
	activeTray = t
	trayMu.Unlock()

	t.host.Dispatch(func() {
		window := (*C.GtkWidget)(t.host.Window())
		iconName := C.CString("agent-team-monitor")
		iconThemePath := C.CString(desktopIconThemePath())
		iconFullPath := C.CString(desktopIconFilePath())
		defer C.free(unsafe.Pointer(iconName))
		defer C.free(unsafe.Pointer(iconThemePath))
		defer C.free(unsafe.Pointer(iconFullPath))

		C.atm_create_tray_icon(iconName, iconThemePath, iconFullPath)
		C.atm_connect_delete_handler(window)
	})

	return nil
}

func desktopIconThemePath() string {
	candidates := []string{}
	for _, prefix := range desktopInstallPrefixCandidates() {
		candidates = append(candidates,
			filepath.Join(prefix, "share", "icons", "hicolor", "512x512", "apps"),
			filepath.Join(prefix, "share", "icons", "hicolor", "256x256", "apps"),
			filepath.Join(prefix, "share", "icons", "hicolor", "128x128", "apps"),
		)
	}

	for _, candidate := range candidates {
		if stat, err := os.Stat(candidate); err == nil && stat.IsDir() {
			return candidate
		}
	}

	return ""
}

func desktopIconFilePath() string {
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

func userHomeDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return home
}

func (t *desktopTray) destroy() {
	if t == nil || t.host == nil {
		return
	}

	trayMu.Lock()
	if activeTray == t {
		activeTray = nil
	}
	trayMu.Unlock()

	t.host.Dispatch(func() {
		t.destroyDirect()
	})
}

func (t *desktopTray) destroyDirect() {
	if t == nil {
		return
	}

	trayMu.Lock()
	if activeTray == t {
		activeTray = nil
	}
	trayMu.Unlock()

	C.atm_destroy_tray_icon()
}

func (t *desktopTray) showWindow() {
	if t == nil || t.host == nil {
		return
	}

	t.host.Dispatch(func() {
		window := t.host.Window()
		C.atm_window_show((*C.GtkWidget)(window))
		focusDesktopWindowContent(window)
	})
}

func (t *desktopTray) hideWindow() {
	if t == nil || t.host == nil {
		return
	}

	t.host.Dispatch(func() {
		C.atm_window_hide((*C.GtkWidget)(t.host.Window()))
	})
}

func (t *desktopTray) hideWindowSoon(delay time.Duration) {
	if t == nil || t.host == nil {
		return
	}
	if delay < 0 {
		delay = 0
	}

	go func() {
		if delay > 0 {
			time.Sleep(delay)
		}
		t.hideWindow()
	}()
}

func (t *desktopTray) allowNextCloseToQuit() {
	trayMu.Lock()
	quitFromTray = true
	trayMu.Unlock()
}

func (t *desktopTray) clearQuitIntent() {
	trayMu.Lock()
	quitFromTray = false
	trayMu.Unlock()
}

func (t *desktopTray) setCloseToTrayEnabled(enabled bool) {
	trayMu.Lock()
	closeToTrayEnabled = enabled
	trayMu.Unlock()
}

//export atmTrayActivate
func atmTrayActivate() {
	trayMu.Lock()
	tray := activeTray
	trayMu.Unlock()

	if tray != nil {
		tray.showWindow()
	}
}

//export atmTrayMenuShow
func atmTrayMenuShow() {
	atmTrayActivate()
}

//export atmTrayMenuHide
func atmTrayMenuHide() {
	trayMu.Lock()
	tray := activeTray
	trayMu.Unlock()

	if tray != nil {
		tray.hideWindow()
	}
}

//export atmTrayMenuPreferences
func atmTrayMenuPreferences() {
	nativeWindowsMu.Lock()
	native := activeNativeWindows
	nativeWindowsMu.Unlock()

	if native != nil {
		native.showPreferences()
	}
}

//export atmTrayMenuAbout
func atmTrayMenuAbout() {
	nativeWindowsMu.Lock()
	native := activeNativeWindows
	nativeWindowsMu.Unlock()

	if native != nil {
		native.showAbout()
	}
}

//export atmTrayMenuQuit
func atmTrayMenuQuit() {
	trayMu.Lock()
	tray := activeTray
	trayMu.Unlock()

	if tray != nil {
		tray.allowNextCloseToQuit()
		tray.host.Terminate()
	}
}

//export atmWindowDelete
func atmWindowDelete(widget *C.GtkWidget, event *C.GdkEvent, data C.gpointer) C.gboolean {
	trayMu.Lock()
	shouldQuit := quitFromTray
	shouldHide := closeToTrayEnabled
	if quitFromTray {
		quitFromTray = false
	}
	tray := activeTray
	trayMu.Unlock()

	if shouldQuit || !shouldHide {
		return C.FALSE
	}

	if tray != nil {
		C.atm_window_hide(widget)
		return C.TRUE
	}

	return C.FALSE
}

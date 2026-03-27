//go:build linux

package main

/*
#cgo pkg-config: gtk+-3.0
#include <gtk/gtk.h>
#include <stdlib.h>

extern void atmPreferencesWindowSaved(char *payload);
extern void atmPreferencesWindowClosed();
extern void atmAboutWindowClosed();

typedef struct {
	GtkWidget *window;
	GtkWidget *hide_idle;
	GtkWidget *startup_view;
	GtkWidget *provider_filter;
	GtkWidget *theme;
	GtkWidget *notify_task_completion;
	GtkWidget *notify_stale_agents;
	GtkWidget *close_to_tray;
	GtkWidget *launch_on_login;
	GtkWidget *start_minimized_to_tray;
} ATMPreferencesWindow;

static void atm_preferences_window_on_destroy(GtkWidget *widget, gpointer user_data) {
	atmPreferencesWindowClosed();
}

static void atm_about_window_on_destroy(GtkWidget *widget, gpointer user_data) {
	atmAboutWindowClosed();
}

static void atm_preferences_window_destroy(ATMPreferencesWindow *prefs) {
	if (prefs == NULL) {
		return;
	}
	if (prefs->window != NULL) {
		gtk_widget_destroy(prefs->window);
	}
	g_free(prefs);
}

static GtkWidget* atm_preferences_row_toggle(const char *label, gboolean active, GtkWidget **toggle_ref) {
	GtkWidget *row = gtk_box_new(GTK_ORIENTATION_HORIZONTAL, 12);
	GtkWidget *text = gtk_label_new(label);
	gtk_widget_set_halign(text, GTK_ALIGN_START);
	gtk_widget_set_hexpand(text, TRUE);
	GtkWidget *toggle = gtk_switch_new();
	gtk_switch_set_active(GTK_SWITCH(toggle), active);
	gtk_box_pack_start(GTK_BOX(row), text, TRUE, TRUE, 0);
	gtk_box_pack_end(GTK_BOX(row), toggle, FALSE, FALSE, 0);
	*toggle_ref = toggle;
	return row;
}

static GtkWidget* atm_preferences_row_select(const char *label, const char **items, int count, const char *active, GtkWidget **combo_ref) {
	GtkWidget *row = gtk_box_new(GTK_ORIENTATION_HORIZONTAL, 12);
	GtkWidget *text = gtk_label_new(label);
	gtk_widget_set_halign(text, GTK_ALIGN_START);
	gtk_widget_set_hexpand(text, TRUE);
	GtkWidget *combo = gtk_combo_box_text_new();
	for (int i = 0; i < count; i++) {
		gtk_combo_box_text_append_text(GTK_COMBO_BOX_TEXT(combo), items[i]);
		if (g_strcmp0(items[i], active) == 0) {
			gtk_combo_box_set_active(GTK_COMBO_BOX(combo), i);
		}
	}
	gtk_box_pack_start(GTK_BOX(row), text, TRUE, TRUE, 0);
	gtk_box_pack_end(GTK_BOX(row), combo, FALSE, FALSE, 0);
	*combo_ref = combo;
	return row;
}

static void atm_preferences_window_save(GtkButton *button, gpointer user_data) {
	ATMPreferencesWindow *prefs = (ATMPreferencesWindow *)user_data;
	const char *startup_view = gtk_combo_box_text_get_active_text(GTK_COMBO_BOX_TEXT(prefs->startup_view));
	const char *provider_filter = gtk_combo_box_text_get_active_text(GTK_COMBO_BOX_TEXT(prefs->provider_filter));
	const char *theme = gtk_combo_box_text_get_active_text(GTK_COMBO_BOX_TEXT(prefs->theme));
	const gboolean hide_idle = gtk_switch_get_active(GTK_SWITCH(prefs->hide_idle));
	const gboolean notify_task_completion = gtk_switch_get_active(GTK_SWITCH(prefs->notify_task_completion));
	const gboolean notify_stale_agents = gtk_switch_get_active(GTK_SWITCH(prefs->notify_stale_agents));
	const gboolean close_to_tray = gtk_switch_get_active(GTK_SWITCH(prefs->close_to_tray));
	const gboolean launch_on_login = gtk_switch_get_active(GTK_SWITCH(prefs->launch_on_login));
	const gboolean start_minimized_to_tray = gtk_switch_get_active(GTK_SWITCH(prefs->start_minimized_to_tray));

	char *payload = g_strdup_printf(
		"{\"hideIdleAgents\":%s,\"startupView\":\"%s\",\"providerFilter\":\"%s\",\"theme\":\"%s\",\"notifyTaskCompletion\":%s,\"notifyStaleAgents\":%s,\"closeToTray\":%s,\"launchOnLogin\":%s,\"startMinimizedToTray\":%s}",
		hide_idle ? "true" : "false",
		startup_view != NULL ? startup_view : "dashboard",
		provider_filter != NULL ? provider_filter : "all",
		theme != NULL ? theme : "light",
		notify_task_completion ? "true" : "false",
		notify_stale_agents ? "true" : "false",
		close_to_tray ? "true" : "false",
		launch_on_login ? "true" : "false",
		start_minimized_to_tray ? "true" : "false"
	);
	atmPreferencesWindowSaved(payload);
	g_free(payload);
}

static ATMPreferencesWindow* atm_preferences_window_new(const char *provider, const char *version, const char *startup_view, const char *provider_filter, const char *theme,
	gboolean hide_idle, gboolean notify_task_completion, gboolean notify_stale_agents, gboolean close_to_tray,
	gboolean launch_on_login, gboolean start_minimized_to_tray) {
	ATMPreferencesWindow *prefs = g_new0(ATMPreferencesWindow, 1);
	prefs->window = gtk_window_new(GTK_WINDOW_TOPLEVEL);
	gtk_window_set_title(GTK_WINDOW(prefs->window), "Agent Team Monitor 设置");
	gtk_window_set_default_size(GTK_WINDOW(prefs->window), 460, 500);
	gtk_window_set_resizable(GTK_WINDOW(prefs->window), FALSE);
	gtk_container_set_border_width(GTK_CONTAINER(prefs->window), 18);
	g_signal_connect(prefs->window, "destroy", G_CALLBACK(atm_preferences_window_on_destroy), NULL);

	GtkWidget *outer = gtk_box_new(GTK_ORIENTATION_VERTICAL, 16);
	gtk_container_add(GTK_CONTAINER(prefs->window), outer);

	GtkWidget *title = gtk_label_new(NULL);
	gtk_label_set_markup(GTK_LABEL(title), "<span weight='bold' size='large'>桌面应用设置</span>");
	gtk_widget_set_halign(title, GTK_ALIGN_START);
	gtk_box_pack_start(GTK_BOX(outer), title, FALSE, FALSE, 0);

	GtkWidget *subtitle = gtk_label_new(NULL);
	char *subtitle_text = g_strdup_printf("当前 Provider: %s    版本: %s", provider, version);
	gtk_label_set_text(GTK_LABEL(subtitle), subtitle_text);
	g_free(subtitle_text);
	gtk_widget_set_halign(subtitle, GTK_ALIGN_START);
	gtk_box_pack_start(GTK_BOX(outer), subtitle, FALSE, FALSE, 0);

	GtkWidget *panel = gtk_box_new(GTK_ORIENTATION_VERTICAL, 12);
	gtk_box_pack_start(GTK_BOX(outer), panel, TRUE, TRUE, 0);

	const char *startup_items[] = {"dashboard", "game"};
	gtk_box_pack_start(GTK_BOX(panel), atm_preferences_row_select("默认启动视图", startup_items, 2, startup_view, &prefs->startup_view), FALSE, FALSE, 0);

	const char *provider_items[] = {"all", "claude", "codex", "openclaw"};
	gtk_box_pack_start(GTK_BOX(panel), atm_preferences_row_select("团队来源筛选", provider_items, 4, provider_filter, &prefs->provider_filter), FALSE, FALSE, 0);

	const char *theme_items[] = {"light", "dark"};
	gtk_box_pack_start(GTK_BOX(panel), atm_preferences_row_select("主题", theme_items, 2, theme, &prefs->theme), FALSE, FALSE, 0);

	gtk_box_pack_start(GTK_BOX(panel), atm_preferences_row_toggle("默认隐藏空闲成员", hide_idle, &prefs->hide_idle), FALSE, FALSE, 0);
	gtk_box_pack_start(GTK_BOX(panel), atm_preferences_row_toggle("任务完成时发送系统通知", notify_task_completion, &prefs->notify_task_completion), FALSE, FALSE, 0);
	gtk_box_pack_start(GTK_BOX(panel), atm_preferences_row_toggle("成员长时间无活动时通知", notify_stale_agents, &prefs->notify_stale_agents), FALSE, FALSE, 0);
	gtk_box_pack_start(GTK_BOX(panel), atm_preferences_row_toggle("关闭窗口时隐藏到托盘", close_to_tray, &prefs->close_to_tray), FALSE, FALSE, 0);
	gtk_box_pack_start(GTK_BOX(panel), atm_preferences_row_toggle("开机自动启动桌面应用", launch_on_login, &prefs->launch_on_login), FALSE, FALSE, 0);
	gtk_box_pack_start(GTK_BOX(panel), atm_preferences_row_toggle("启动时直接驻留托盘", start_minimized_to_tray, &prefs->start_minimized_to_tray), FALSE, FALSE, 0);

	GtkWidget *actions = gtk_box_new(GTK_ORIENTATION_HORIZONTAL, 10);
	gtk_widget_set_halign(actions, GTK_ALIGN_END);
	gtk_box_pack_end(GTK_BOX(outer), actions, FALSE, FALSE, 0);

	GtkWidget *close_button = gtk_button_new_with_label("关闭");
	g_signal_connect_swapped(close_button, "clicked", G_CALLBACK(gtk_widget_destroy), prefs->window);
	gtk_box_pack_end(GTK_BOX(actions), close_button, FALSE, FALSE, 0);

	GtkWidget *save_button = gtk_button_new_with_label("保存设置");
	g_signal_connect(save_button, "clicked", G_CALLBACK(atm_preferences_window_save), prefs);
	gtk_box_pack_end(GTK_BOX(actions), save_button, FALSE, FALSE, 0);

	return prefs;
}

static void atm_preferences_window_show(ATMPreferencesWindow *prefs) {
	if (prefs == NULL || prefs->window == NULL) {
		return;
	}
	gtk_widget_show_all(prefs->window);
	gtk_window_present(GTK_WINDOW(prefs->window));
}

static GtkWidget* atm_about_window_new(const char *version) {
	GtkWidget *dialog = gtk_about_dialog_new();
	gtk_about_dialog_set_program_name(GTK_ABOUT_DIALOG(dialog), "Agent Team Monitor");
	gtk_about_dialog_set_version(GTK_ABOUT_DIALOG(dialog), version);
	gtk_about_dialog_set_comments(GTK_ABOUT_DIALOG(dialog), "Linux 桌面应用，用于监控 Claude / Codex / OpenClaw 智能体团队。");
	gtk_about_dialog_set_website(GTK_ABOUT_DIALOG(dialog), "https://github.com/vikingleo/agent-team-monitor");
	gtk_about_dialog_set_logo_icon_name(GTK_ABOUT_DIALOG(dialog), "agent-team-monitor");
	g_signal_connect(dialog, "destroy", G_CALLBACK(atm_about_window_on_destroy), NULL);
	return dialog;
}

static void atm_about_window_show(GtkWidget *dialog) {
	if (dialog == NULL) {
		return;
	}
	gtk_widget_show(dialog);
	gtk_window_present(GTK_WINDOW(dialog));
}
*/
import "C"

import (
	"encoding/json"
	"sync"
	"unsafe"
)

func boolToGBoolean(value bool) C.gboolean {
	if value {
		return C.TRUE
	}
	return C.FALSE
}

type desktopNativeWindows struct {
	host          desktopUIHost
	preferences   *desktopPreferencesController
	provider      string
	version       string
	onUpdated     func(desktopPreferences)
	mu            sync.Mutex
	preferencesUI *C.ATMPreferencesWindow
	aboutUI       *C.GtkWidget
}

var (
	nativeWindowsMu     sync.Mutex
	activeNativeWindows *desktopNativeWindows
)

func newDesktopNativeWindows(host desktopUIHost, preferences *desktopPreferencesController, provider, version string, onUpdated func(desktopPreferences)) *desktopNativeWindows {
	if host == nil || preferences == nil {
		return nil
	}

	return &desktopNativeWindows{
		host:        host,
		preferences: preferences,
		provider:    provider,
		version:     version,
		onUpdated:   onUpdated,
	}
}

func (n *desktopNativeWindows) install() {
	nativeWindowsMu.Lock()
	activeNativeWindows = n
	nativeWindowsMu.Unlock()
}

func (n *desktopNativeWindows) destroy() {
	if n == nil || n.host == nil {
		return
	}

	nativeWindowsMu.Lock()
	if activeNativeWindows == n {
		activeNativeWindows = nil
	}
	nativeWindowsMu.Unlock()

	n.host.Dispatch(func() {
		n.mu.Lock()
		defer n.mu.Unlock()
		if n.preferencesUI != nil {
			prefs := n.preferencesUI
			n.preferencesUI = nil
			C.atm_preferences_window_destroy(prefs)
		}
		if n.aboutUI != nil {
			about := n.aboutUI
			n.aboutUI = nil
			C.gtk_widget_destroy(about)
		}
	})
}

func (n *desktopNativeWindows) showPreferences() {
	if n == nil || n.host == nil {
		return
	}

	prefs := n.preferences.Get()
	provider := C.CString(n.provider)
	version := C.CString(n.version)
	startupView := C.CString(prefs.StartupView)
	providerFilter := C.CString(prefs.ProviderFilter)
	theme := C.CString(prefs.Theme)
	defer C.free(unsafe.Pointer(provider))
	defer C.free(unsafe.Pointer(version))
	defer C.free(unsafe.Pointer(startupView))
	defer C.free(unsafe.Pointer(providerFilter))
	defer C.free(unsafe.Pointer(theme))

	n.host.Dispatch(func() {
		n.mu.Lock()
		defer n.mu.Unlock()

		if n.preferencesUI != nil {
			C.atm_preferences_window_show(n.preferencesUI)
			return
		}

		n.preferencesUI = C.atm_preferences_window_new(
			provider,
			version,
			startupView,
			providerFilter,
			theme,
			boolToGBoolean(prefs.HideIdleAgents),
			boolToGBoolean(prefs.NotifyTaskCompletion),
			boolToGBoolean(prefs.NotifyStaleAgents),
			boolToGBoolean(prefs.CloseToTray),
			boolToGBoolean(prefs.LaunchOnLogin),
			boolToGBoolean(prefs.StartMinimizedToTray),
		)
		C.atm_preferences_window_show(n.preferencesUI)
	})
}

func (n *desktopNativeWindows) showAbout() {
	if n == nil || n.host == nil {
		return
	}

	version := C.CString(n.version)
	defer C.free(unsafe.Pointer(version))

	n.host.Dispatch(func() {
		n.mu.Lock()
		defer n.mu.Unlock()

		if n.aboutUI == nil {
			n.aboutUI = C.atm_about_window_new(version)
		}
		C.atm_about_window_show(n.aboutUI)
	})
}

//export atmPreferencesWindowSaved
func atmPreferencesWindowSaved(payload *C.char) {
	nativeWindowsMu.Lock()
	native := activeNativeWindows
	nativeWindowsMu.Unlock()

	if native == nil {
		return
	}

	var prefs desktopPreferences
	if err := json.Unmarshal([]byte(C.GoString(payload)), &prefs); err != nil {
		return
	}

	current := native.preferences.Get()
	saved, err := native.preferences.Set(mergeDesktopPreferences(current, desktopPreferencesFile{
		HideIdleAgents:       boolPtr(prefs.HideIdleAgents),
		StartupView:          prefs.StartupView,
		ProviderFilter:       prefs.ProviderFilter,
		Theme:                prefs.Theme,
		NotifyTaskCompletion: boolPtr(prefs.NotifyTaskCompletion),
		NotifyStaleAgents:    boolPtr(prefs.NotifyStaleAgents),
		CloseToTray:          boolPtr(prefs.CloseToTray),
		LaunchOnLogin:        boolPtr(prefs.LaunchOnLogin),
		StartMinimizedToTray: boolPtr(prefs.StartMinimizedToTray),
	}))
	if err != nil {
		return
	}

	native.host.Dispatch(func() {
		native.mu.Lock()
		if native.preferencesUI != nil {
			prefs := native.preferencesUI
			native.preferencesUI = nil
			C.atm_preferences_window_destroy(prefs)
		}
		native.mu.Unlock()

		if native.onUpdated != nil {
			native.onUpdated(saved)
		}
	})
}

//export atmPreferencesWindowClosed
func atmPreferencesWindowClosed() {
	nativeWindowsMu.Lock()
	native := activeNativeWindows
	nativeWindowsMu.Unlock()

	if native == nil {
		return
	}

	native.mu.Lock()
	native.preferencesUI = nil
	native.mu.Unlock()
}

//export atmAboutWindowClosed
func atmAboutWindowClosed() {
	nativeWindowsMu.Lock()
	native := activeNativeWindows
	nativeWindowsMu.Unlock()

	if native == nil {
		return
	}

	native.mu.Lock()
	native.aboutUI = nil
	native.mu.Unlock()
}

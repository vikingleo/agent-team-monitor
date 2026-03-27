package main

import "fmt"

type desktopPreferencesController struct {
	store     *desktopPreferencesStore
	tray      *desktopTray
	autostart *desktopAutostartManager
}

func newDesktopPreferencesController(store *desktopPreferencesStore, tray *desktopTray, autostart *desktopAutostartManager) *desktopPreferencesController {
	if store == nil {
		store = newInMemoryDesktopPreferencesStore()
	}

	return &desktopPreferencesController{
		store:     store,
		tray:      tray,
		autostart: autostart,
	}
}

func (c *desktopPreferencesController) Get() desktopPreferences {
	if c == nil || c.store == nil {
		return defaultDesktopPreferences()
	}

	return c.store.Get()
}

func (c *desktopPreferencesController) Set(input desktopPreferences) (desktopPreferences, error) {
	if c == nil || c.store == nil {
		return defaultDesktopPreferences(), fmt.Errorf("desktop preferences unavailable")
	}

	current := c.store.Get()
	normalized := normalizeDesktopPreferences(input)

	if c.autostart != nil {
		if err := c.autostart.Apply(normalized.LaunchOnLogin); err != nil {
			return current, err
		}
	}

	saved, err := c.store.Set(normalized)
	if err != nil {
		if c.autostart != nil {
			_ = c.autostart.Apply(current.LaunchOnLogin)
		}
		return current, err
	}

	c.applyRuntime(saved)
	return saved, nil
}

func (c *desktopPreferencesController) Reconcile() error {
	if c == nil {
		return nil
	}

	prefs := c.Get()
	if c.autostart != nil {
		if err := c.autostart.Apply(prefs.LaunchOnLogin); err != nil {
			return err
		}
	}

	c.applyRuntime(prefs)
	return nil
}

func (c *desktopPreferencesController) applyRuntime(prefs desktopPreferences) {
	if c == nil || c.tray == nil {
		return
	}

	c.tray.setCloseToTrayEnabled(prefs.CloseToTray)
	c.tray.clearQuitIntent()
}

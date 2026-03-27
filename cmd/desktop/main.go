package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"runtime"
	"syscall"
	"time"

	agentapp "github.com/liaoweijun/agent-team-monitor/internal/app"
)

const (
	defaultProvider = "both"
	windowTitle     = "Agent Team Monitor"
)

var (
	provider   = flag.String("provider", defaultProvider, "Data source provider: claude, codex, openclaw, both")
	version    = flag.Bool("version", false, "Show version information")
	appVersion = "dev"
)

func main() {
	flag.Parse()

	if *version {
		fmt.Println(agentapp.FormatVersionLabel(windowTitle, appVersion))
		return
	}

	if runtime.GOOS != "linux" {
		log.Fatalf("desktop window is currently implemented for Linux")
	}

	instance, alreadyRunning, err := acquireDesktopSingleInstance()
	if err != nil {
		log.Fatalf("prepare desktop single instance: %v", err)
	}
	if alreadyRunning {
		return
	}
	defer func() {
		if err := instance.Close(); err != nil {
			log.Printf("close desktop single instance: %v", err)
		}
	}()

	if err := desktopDisplayPreflight(); err != nil {
		log.Fatalf("desktop display unavailable: %v", err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	preferences, err := newDesktopPreferencesStore()
	if err != nil {
		log.Fatalf("load desktop preferences: %v", err)
	}
	autostart, err := newDesktopAutostartManager()
	if err != nil {
		log.Fatalf("prepare desktop autostart: %v", err)
	}

	session, err := agentapp.StartWeb(*provider, "127.0.0.1:0")
	if err != nil {
		log.Fatalf("start desktop web session: %v", err)
	}
	defer func() {
		if err := session.Stop(); err != nil {
			log.Printf("stop desktop web session: %v", err)
		}
	}()

	mainWindow, err := newDesktopMainWindow(session, nil, *provider, appVersion)
	if err != nil {
		log.Fatalf("create desktop main window: %v", err)
	}

	tray := newDesktopTray(mainWindow)
	preferencesController := newDesktopPreferencesController(preferences, tray, autostart)
	mainWindow.tray = tray
	if tray != nil {
		if err := tray.install(); err != nil {
			log.Printf("install desktop tray: %v", err)
		}
	}
	if err := preferencesController.Reconcile(); err != nil {
		log.Printf("reconcile desktop preferences: %v", err)
	}

	mainWindow.preferences = preferencesController

	nativeWindows := newDesktopNativeWindows(mainWindow, preferencesController, *provider, appVersion, func(saved desktopPreferences) {
		mainWindow.reloadCurrentViewSoon(50 * time.Millisecond)
	})
	mainWindow.native = nativeWindows
	if nativeWindows != nil {
		nativeWindows.install()
	}

	bridge := newDesktopBridge(session.Collector, *provider, preferencesController, tray, nativeWindows)
	if err := mainWindow.attachBridge(bridge); err != nil {
		log.Fatalf("attach desktop bridge: %v", err)
	}

	notifier := newDesktopNotifier(session.Collector, preferences)
	go notifier.Start(ctx)

	if preferencesController.Get().StartMinimizedToTray && tray != nil {
		tray.hideWindowSoon(200 * time.Millisecond)
	}

	instance.SetActivateHandler(func() {
		if tray != nil {
			tray.showWindow()
			return
		}
		mainWindow.Present()
	})

	go func() {
		<-ctx.Done()
		mainWindow.Terminate()
	}()

	mainWindow.Run()
	if nativeWindows != nil {
		nativeWindows.destroyDirect()
	}
	if tray != nil {
		tray.destroyDirect()
	}
	mainWindow.Destroy()
}

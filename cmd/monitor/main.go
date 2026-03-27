package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	agentapp "github.com/liaoweijun/agent-team-monitor/internal/app"
)

var (
	webMode    = flag.Bool("web", false, "Run in web mode (HTTP server)")
	webAddr    = flag.String("addr", ":8080", "Web server address")
	provider   = flag.String("provider", "both", "Data source provider: claude, codex, openclaw, both")
	version    = flag.Bool("version", false, "Show version information")
	appVersion = "dev"
)

const appName = "Agent Team Monitor"

func main() {
	flag.Parse()

	if *version {
		fmt.Println(agentapp.FormatVersionLabel(appName, appVersion))
		os.Exit(0)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if *webMode {
		runWebMode(ctx)
		return
	}

	runTUIMode(ctx)
}

func runTUIMode(ctx context.Context) {
	if err := agentapp.RunTUI(ctx, *provider); err != nil {
		log.Fatalf("Error running TUI: %v", err)
	}
}

func runWebMode(ctx context.Context) {
	session, err := agentapp.StartWeb(*provider, *webAddr)
	if err != nil {
		log.Fatalf("Error starting web server: %v", err)
	}
	defer session.Stop()

	fmt.Printf("Web dashboard available at %s\n", session.BaseURL)
	fmt.Println("Press Ctrl+C to stop")

	<-ctx.Done()
	fmt.Println("\nShutting down web server...")
	if err := session.Stop(); err != nil {
		log.Printf("Error stopping server: %v", err)
	}
}

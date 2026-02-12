package main

import (
	"flag"
	"fmt"
	"io/fs"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/liaoweijun/agent-team-monitor/pkg/api"
	"github.com/liaoweijun/agent-team-monitor/pkg/monitor"
	"github.com/liaoweijun/agent-team-monitor/pkg/ui"
	"github.com/liaoweijun/agent-team-monitor/web"
)

var (
	webMode = flag.Bool("web", false, "Run in web mode (HTTP server)")
	webAddr = flag.String("addr", ":8080", "Web server address")
	version = flag.Bool("version", false, "Show version information")
)

const (
	appVersion = "1.2.0"
	appName    = "Claude Agent Team Monitor"
)

func main() {
	flag.Parse()

	// Show version
	if *version {
		fmt.Printf("%s v%s\n", appName, appVersion)
		os.Exit(0)
	}

	// Create collector
	collector, err := monitor.NewCollector()
	if err != nil {
		log.Fatalf("Failed to create collector: %v", err)
	}

	// Start collecting data
	if err := collector.Start(); err != nil {
		log.Fatalf("Failed to start collector: %v", err)
	}
	defer collector.Stop()

	// Setup signal handling for graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	if *webMode {
		// Run in web mode
		runWebMode(collector, sigChan)
	} else {
		// Run in TUI mode
		runTUIMode(collector, sigChan)
	}
}

func runTUIMode(collector *monitor.Collector, sigChan chan os.Signal) {
	go func() {
		<-sigChan
		fmt.Println("\nShutting down...")
		collector.Stop()
		os.Exit(0)
	}()

	// Run TUI
	if err := ui.Run(collector); err != nil {
		log.Fatalf("Error running TUI: %v", err)
	}
}

func runWebMode(collector *monitor.Collector, sigChan chan os.Signal) {
	// Create embedded static filesystem
	staticFS, err := fs.Sub(web.StaticFiles, "static")
	if err != nil {
		log.Fatalf("Failed to load embedded static files: %v", err)
	}

	// Create and start web server
	server := api.NewServer(collector, *webAddr, staticFS)

	// Handle shutdown
	go func() {
		<-sigChan
		fmt.Println("\nShutting down web server...")
		if err := server.Stop(); err != nil {
			log.Printf("Error stopping server: %v", err)
		}
		collector.Stop()
		os.Exit(0)
	}()

	// Start server
	fmt.Printf("Web dashboard available at http://localhost%s\n", *webAddr)
	fmt.Println("Press Ctrl+C to stop")

	if err := server.Start(); err != nil {
		log.Fatalf("Error starting web server: %v", err)
	}
}

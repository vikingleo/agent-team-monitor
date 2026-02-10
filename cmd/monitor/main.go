package main

import (
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/liaoweijun/agent-team-monitor/pkg/monitor"
	"github.com/liaoweijun/agent-team-monitor/pkg/ui"
)

func main() {
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

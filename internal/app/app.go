package app

import (
	"context"
	"fmt"
	"io/fs"
	"net"
	"strings"
	"sync"

	"github.com/liaoweijun/agent-team-monitor/pkg/api"
	"github.com/liaoweijun/agent-team-monitor/pkg/monitor"
	"github.com/liaoweijun/agent-team-monitor/pkg/ui"
	"github.com/liaoweijun/agent-team-monitor/web"
)

type Mode string

const (
	ModeTUI Mode = "tui"
	ModeWeb Mode = "web"
)

type Config struct {
	Mode     Mode
	Provider string
	WebAddr  string
}

type WebSession struct {
	Collector *monitor.Collector
	Server    *api.Server
	Addr      string
	BaseURL   string

	stopOnce sync.Once
}

func StartCollector(provider string) (*monitor.Collector, error) {
	providerMode, err := monitor.ParseProviderMode(provider)
	if err != nil {
		return nil, fmt.Errorf("invalid provider mode: %w", err)
	}

	collector, err := monitor.NewCollectorWithOptions(monitor.CollectorOptions{
		Provider: providerMode,
	})
	if err != nil {
		return nil, err
	}

	if err := collector.Start(); err != nil {
		return nil, err
	}

	return collector, nil
}

func RunTUI(ctx context.Context, provider string) error {
	collector, err := StartCollector(provider)
	if err != nil {
		return err
	}

	return runTUIWithCollector(ctx, collector, collector.Stop, ui.RunWithContext)
}

type tuiRunner func(context.Context, *monitor.Collector) error

func runTUIWithCollector(ctx context.Context, collector *monitor.Collector, stop func() error, run tuiRunner) error {
	if stop != nil {
		defer func() {
			_ = stop()
		}()
	}

	return run(ctx, collector)
}

func StartWeb(provider, requestedAddr string) (*WebSession, error) {
	collector, err := StartCollector(provider)
	if err != nil {
		return nil, err
	}

	staticFS, err := fs.Sub(web.StaticFiles, "static")
	if err != nil {
		collector.Stop()
		return nil, fmt.Errorf("load embedded static files: %w", err)
	}

	resolvedAddr, err := resolveWebAddr(requestedAddr)
	if err != nil {
		collector.Stop()
		return nil, err
	}

	server := api.NewServer(collector, resolvedAddr, staticFS)
	listener, err := net.Listen("tcp", resolvedAddr)
	if err != nil {
		collector.Stop()
		return nil, fmt.Errorf("listen on %s: %w", resolvedAddr, err)
	}

	actualAddr := listener.Addr().String()
	session := &WebSession{
		Collector: collector,
		Server:    server,
		Addr:      actualAddr,
		BaseURL:   buildLocalhostURL(actualAddr),
	}

	go func() {
		if err := server.StartListener(listener); err != nil {
			if !isServerClosed(err) {
				// Preserve existing CLI-style behavior: surfacing the error is the caller's job.
			}
		}
	}()

	return session, nil
}

func (s *WebSession) Stop() error {
	var stopErr error
	s.stopOnce.Do(func() {
		if s.Server != nil {
			stopErr = s.Server.Stop()
		}
		if s.Collector != nil {
			_ = s.Collector.Stop()
		}
	})
	return stopErr
}

func resolveWebAddr(requested string) (string, error) {
	addr := strings.TrimSpace(requested)
	if addr == "" {
		addr = ":8000"
	}

	if addr == ":0" || addr == "localhost:0" || addr == "127.0.0.1:0" {
		return pickRandomAddr("127.0.0.1")
	}

	if strings.HasPrefix(addr, ":") {
		return addr, nil
	}

	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		return "", fmt.Errorf("invalid web address %q: %w", requested, err)
	}
	if port == "0" {
		return pickRandomAddr(host)
	}

	return net.JoinHostPort(host, port), nil
}

func pickRandomAddr(host string) (string, error) {
	if strings.TrimSpace(host) == "" {
		host = "127.0.0.1"
	}
	return net.JoinHostPort(host, "0"), nil
}

func buildLocalhostURL(addr string) string {
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		if strings.HasPrefix(addr, ":") {
			return "http://localhost" + addr
		}
		return "http://" + addr
	}

	normalizedHost := host
	switch host {
	case "", "0.0.0.0", "::":
		normalizedHost = "localhost"
	}

	return "http://" + net.JoinHostPort(normalizedHost, port)
}

func isServerClosed(err error) bool {
	if err == nil {
		return false
	}
	return strings.Contains(err.Error(), "Server closed")
}

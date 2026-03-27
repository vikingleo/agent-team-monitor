package app

import (
	"context"
	"testing"
	"time"

	"github.com/liaoweijun/agent-team-monitor/pkg/monitor"
)

func TestResolveWebAddr(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    string
		wantErr bool
	}{
		{name: "default", input: "", want: ":8000"},
		{name: "short port", input: ":3000", want: ":3000"},
		{name: "random localhost", input: "127.0.0.1:0", want: "127.0.0.1:0"},
		{name: "random short", input: ":0", want: "127.0.0.1:0"},
		{name: "full host port", input: "localhost:9000", want: "localhost:9000"},
		{name: "invalid", input: "abc", wantErr: true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := resolveWebAddr(tc.input)
			if tc.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("resolveWebAddr error: %v", err)
			}
			if got != tc.want {
				t.Fatalf("expected %q, got %q", tc.want, got)
			}
		})
	}
}

func TestBuildLocalhostURL(t *testing.T) {
	tests := []struct {
		addr string
		want string
	}{
		{addr: ":8000", want: "http://localhost:8000"},
		{addr: "0.0.0.0:8123", want: "http://localhost:8123"},
		{addr: "127.0.0.1:9000", want: "http://127.0.0.1:9000"},
		{addr: "[::]:4567", want: "http://localhost:4567"},
	}

	for _, tc := range tests {
		t.Run(tc.addr, func(t *testing.T) {
			got := buildLocalhostURL(tc.addr)
			if got != tc.want {
				t.Fatalf("expected %q, got %q", tc.want, got)
			}
		})
	}
}

func TestRunTUIWithCollector_ReturnsWhenUIExits(t *testing.T) {
	stopped := 0

	err := runTUIWithCollector(context.Background(), nil, func() error {
		stopped++
		return nil
	}, func(context.Context, *monitor.Collector) error {
		return nil
	})
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if stopped != 1 {
		t.Fatalf("expected stop to be called once, got %d", stopped)
	}
}

func TestRunTUIWithCollector_ReturnsAfterContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	stopped := 0
	done := make(chan error, 1)
	go func() {
		done <- runTUIWithCollector(ctx, nil, func() error {
			stopped++
			return nil
		}, func(ctx context.Context, _ *monitor.Collector) error {
			<-ctx.Done()
			return nil
		})
	}()

	cancel()

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("expected nil error, got %v", err)
		}
	case <-time.After(500 * time.Millisecond):
		t.Fatal("runTUIWithCollector did not return after context cancellation")
	}

	if stopped != 1 {
		t.Fatalf("expected stop to be called once, got %d", stopped)
	}
}

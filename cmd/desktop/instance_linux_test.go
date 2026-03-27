//go:build linux

package main

import (
	"net"
	"path/filepath"
	"testing"
	"time"
)

func TestAcquireDesktopSingleInstanceActivatesExistingInstance(t *testing.T) {
	t.Parallel()

	socketPath := filepath.Join(t.TempDir(), "desktop-instance.sock")

	instance, alreadyRunning, err := acquireDesktopSingleInstanceAtPath(socketPath)
	if err != nil {
		t.Fatalf("acquireDesktopSingleInstanceAtPath returned error: %v", err)
	}
	if alreadyRunning {
		t.Fatalf("first acquire unexpectedly reported existing instance")
	}
	defer func() {
		if err := instance.Close(); err != nil {
			t.Fatalf("instance.Close returned error: %v", err)
		}
	}()

	activated := make(chan struct{}, 1)
	instance.SetActivateHandler(func() {
		select {
		case activated <- struct{}{}:
		default:
		}
	})

	other, alreadyRunning, err := acquireDesktopSingleInstanceAtPath(socketPath)
	if err != nil {
		t.Fatalf("second acquireDesktopSingleInstanceAtPath returned error: %v", err)
	}
	if other != nil {
		t.Fatalf("second acquire returned a listener instead of nil")
	}
	if !alreadyRunning {
		t.Fatalf("second acquire did not detect existing instance")
	}

	select {
	case <-activated:
	case <-time.After(2 * time.Second):
		t.Fatalf("existing instance was not activated")
	}
}

func TestAcquireDesktopSingleInstanceRemovesStaleSocket(t *testing.T) {
	t.Parallel()

	socketPath := filepath.Join(t.TempDir(), "desktop-instance.sock")
	listener, err := net.Listen("unix", socketPath)
	if err != nil {
		t.Fatalf("net.Listen returned error: %v", err)
	}
	if err := listener.Close(); err != nil {
		t.Fatalf("listener.Close returned error: %v", err)
	}

	instance, alreadyRunning, err := acquireDesktopSingleInstanceAtPath(socketPath)
	if err != nil {
		t.Fatalf("acquireDesktopSingleInstanceAtPath returned error: %v", err)
	}
	if alreadyRunning {
		t.Fatalf("stale socket should not be treated as running instance")
	}
	defer func() {
		if err := instance.Close(); err != nil {
			t.Fatalf("instance.Close returned error: %v", err)
		}
	}()
}

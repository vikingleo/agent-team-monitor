package monitor

import (
	"testing"

	"github.com/fsnotify/fsnotify"
)

func TestCollectorStop_IgnoresLateFilesystemCallback(t *testing.T) {
	collector, err := NewCollector()
	if err != nil {
		t.Fatalf("NewCollector error: %v", err)
	}

	if err := collector.Stop(); err != nil {
		t.Fatalf("Stop error: %v", err)
	}

	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("expected no panic after stop, got %v", r)
		}
	}()

	collector.fsMonitor.onChange(fsnotify.Event{Name: "/tmp/demo", Op: fsnotify.Create})
}

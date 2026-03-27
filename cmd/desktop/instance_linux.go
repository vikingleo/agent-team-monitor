//go:build linux

package main

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

const (
	desktopInstanceSocketDirName  = "agent-team-monitor"
	desktopInstanceSocketFileName = "desktop-instance.sock"
	desktopInstanceActivateCmd    = "activate"
)

type desktopSingleInstance struct {
	path     string
	listener net.Listener

	mu         sync.RWMutex
	onActivate func()
}

func acquireDesktopSingleInstance() (*desktopSingleInstance, bool, error) {
	path, err := desktopSingleInstanceSocketPath()
	if err != nil {
		return nil, false, err
	}

	return acquireDesktopSingleInstanceAtPath(path)
}

func acquireDesktopSingleInstanceAtPath(path string) (*desktopSingleInstance, bool, error) {
	if strings.TrimSpace(path) == "" {
		return nil, false, fmt.Errorf("resolve desktop instance socket: empty path")
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return nil, false, fmt.Errorf("create desktop instance directory: %w", err)
	}

	if activated, err := notifyDesktopSingleInstance(path); err == nil && activated {
		return nil, true, nil
	}

	if err := removeStaleDesktopInstanceSocket(path); err != nil {
		return nil, false, err
	}

	listener, err := net.Listen("unix", path)
	if err != nil {
		if activated, notifyErr := notifyDesktopSingleInstance(path); notifyErr == nil && activated {
			return nil, true, nil
		}
		return nil, false, fmt.Errorf("listen desktop instance socket: %w", err)
	}

	if err := os.Chmod(path, 0o600); err != nil {
		_ = listener.Close()
		_ = os.Remove(path)
		return nil, false, fmt.Errorf("protect desktop instance socket: %w", err)
	}

	instance := &desktopSingleInstance{
		path:     path,
		listener: listener,
	}
	go instance.serve()
	return instance, false, nil
}

func desktopSingleInstanceSocketPath() (string, error) {
	baseDir := strings.TrimSpace(os.Getenv("XDG_RUNTIME_DIR"))
	if baseDir == "" {
		cacheDir, err := os.UserCacheDir()
		if err != nil {
			return "", fmt.Errorf("resolve desktop instance directory: %w", err)
		}
		baseDir = cacheDir
	}

	return filepath.Join(baseDir, desktopInstanceSocketDirName, desktopInstanceSocketFileName), nil
}

func notifyDesktopSingleInstance(path string) (bool, error) {
	conn, err := net.DialTimeout("unix", path, 300*time.Millisecond)
	if err != nil {
		return false, err
	}
	defer conn.Close()

	_ = conn.SetWriteDeadline(time.Now().Add(1 * time.Second))
	if _, err := io.WriteString(conn, desktopInstanceActivateCmd+"\n"); err != nil {
		return false, err
	}

	return true, nil
}

func removeStaleDesktopInstanceSocket(path string) error {
	info, err := os.Lstat(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("inspect desktop instance socket: %w", err)
	}
	if info.Mode()&os.ModeSocket == 0 {
		return fmt.Errorf("desktop instance path is not a socket: %s", path)
	}
	if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("remove stale desktop instance socket: %w", err)
	}
	return nil
}

func (i *desktopSingleInstance) SetActivateHandler(fn func()) {
	if i == nil {
		return
	}

	i.mu.Lock()
	i.onActivate = fn
	i.mu.Unlock()
}

func (i *desktopSingleInstance) Close() error {
	if i == nil {
		return nil
	}

	var closeErr error
	if i.listener != nil {
		closeErr = i.listener.Close()
	}
	if strings.TrimSpace(i.path) != "" {
		if err := os.Remove(i.path); err != nil && !errors.Is(err, os.ErrNotExist) && closeErr == nil {
			closeErr = err
		}
	}
	return closeErr
}

func (i *desktopSingleInstance) serve() {
	if i == nil || i.listener == nil {
		return
	}

	for {
		conn, err := i.listener.Accept()
		if err != nil {
			if isClosedNetworkError(err) {
				return
			}
			continue
		}

		go i.handle(conn)
	}
}

func (i *desktopSingleInstance) handle(conn net.Conn) {
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(1 * time.Second))

	command, err := bufio.NewReader(io.LimitReader(conn, 128)).ReadString('\n')
	if err != nil {
		return
	}
	if strings.TrimSpace(command) != desktopInstanceActivateCmd {
		return
	}

	i.mu.RLock()
	onActivate := i.onActivate
	i.mu.RUnlock()

	if onActivate != nil {
		onActivate()
	}
}

func isClosedNetworkError(err error) bool {
	if err == nil {
		return false
	}
	return strings.Contains(strings.ToLower(err.Error()), "use of closed network connection")
}

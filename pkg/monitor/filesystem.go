package monitor

import (
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/fsnotify/fsnotify"
)

// FileSystemMonitor monitors Claude team and task directories
type FileSystemMonitor struct {
	watcher   *fsnotify.Watcher
	claudeDir string
	teamsDir  string
	tasksDir  string
	onChange  func(event fsnotify.Event)
}

// NewFileSystemMonitor creates a new filesystem monitor
func NewFileSystemMonitor(onChange func(event fsnotify.Event)) (*FileSystemMonitor, error) {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}

	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}

	claudeDir := filepath.Join(homeDir, ".claude")
	teamsDir := filepath.Join(claudeDir, "teams")
	tasksDir := filepath.Join(claudeDir, "tasks")

	return &FileSystemMonitor{
		watcher:   watcher,
		claudeDir: claudeDir,
		teamsDir:  teamsDir,
		tasksDir:  tasksDir,
		onChange:  onChange,
	}, nil
}

// Start begins monitoring the filesystem
func (fsm *FileSystemMonitor) Start() error {
	// Watch root directories and existing subdirectories.
	// This allows auto-recovery when users manually remove ~/.claude/teams or ~/.claude/tasks.
	if err := fsm.ensureRootsWatched(); err != nil {
		return err
	}

	go fsm.watch()
	return nil
}

func (fsm *FileSystemMonitor) ensureRootsWatched() error {
	if err := os.MkdirAll(fsm.teamsDir, 0755); err != nil {
		return err
	}
	if err := os.MkdirAll(fsm.tasksDir, 0755); err != nil {
		return err
	}

	if err := fsm.addWatch(fsm.claudeDir); err != nil {
		return err
	}
	if err := fsm.addWatch(fsm.teamsDir); err != nil {
		return err
	}
	if err := fsm.addWatch(fsm.tasksDir); err != nil {
		return err
	}

	fsm.watchSubdirectories(fsm.teamsDir)
	fsm.watchSubdirectories(fsm.tasksDir)
	return nil
}

func (fsm *FileSystemMonitor) addWatch(path string) error {
	if err := fsm.watcher.Add(path); err != nil {
		// fsnotify returns an "already exists" error on duplicate adds.
		if strings.Contains(err.Error(), "already exists") {
			return nil
		}
		return err
	}
	return nil
}

// watchSubdirectories adds watchers for all subdirectories
func (fsm *FileSystemMonitor) watchSubdirectories(dir string) {
	filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if info.IsDir() && path != dir {
			if err := fsm.addWatch(path); err != nil {
				log.Printf("Failed to watch subdirectory %s: %v", path, err)
			}
		}
		return nil
	})
}

// watch processes filesystem events
func (fsm *FileSystemMonitor) watch() {
	for {
		select {
		case event, ok := <-fsm.watcher.Events:
			if !ok {
				return
			}

			// Recover root watches after manual directory cleanup/recreation.
			if event.Op&(fsnotify.Create|fsnotify.Remove|fsnotify.Rename) != 0 {
				if err := fsm.ensureRootsWatched(); err != nil {
					log.Printf("Failed to recover root watchers: %v", err)
				}
			}

			// Handle directory creation to watch new subdirectories
			if event.Op&fsnotify.Create == fsnotify.Create {
				if info, err := os.Stat(event.Name); err == nil && info.IsDir() {
					if err := fsm.addWatch(event.Name); err != nil {
						log.Printf("Failed to watch new directory %s: %v", event.Name, err)
					}
					fsm.watchSubdirectories(event.Name)
				}
			}

			if fsm.onChange != nil {
				fsm.onChange(event)
			}

		case err, ok := <-fsm.watcher.Errors:
			if !ok {
				return
			}
			log.Println("Filesystem watcher error:", err)
		}
	}
}

// Stop stops the filesystem monitor
func (fsm *FileSystemMonitor) Stop() error {
	return fsm.watcher.Close()
}

// GetTeamsDir returns the teams directory path
func (fsm *FileSystemMonitor) GetTeamsDir() string {
	return fsm.teamsDir
}

// GetTasksDir returns the tasks directory path
func (fsm *FileSystemMonitor) GetTasksDir() string {
	return fsm.tasksDir
}

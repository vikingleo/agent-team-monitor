package monitor

import (
	"log"
	"os"
	"path/filepath"

	"github.com/fsnotify/fsnotify"
)

// FileSystemMonitor monitors Claude team and task directories
type FileSystemMonitor struct {
	watcher   *fsnotify.Watcher
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

	teamsDir := filepath.Join(homeDir, ".claude", "teams")
	tasksDir := filepath.Join(homeDir, ".claude", "tasks")

	return &FileSystemMonitor{
		watcher:   watcher,
		teamsDir:  teamsDir,
		tasksDir:  tasksDir,
		onChange:  onChange,
	}, nil
}

// Start begins monitoring the filesystem
func (fsm *FileSystemMonitor) Start() error {
	// Create directories if they don't exist
	os.MkdirAll(fsm.teamsDir, 0755)
	os.MkdirAll(fsm.tasksDir, 0755)

	// Watch teams directory
	if err := fsm.watcher.Add(fsm.teamsDir); err != nil {
		return err
	}

	// Watch tasks directory
	if err := fsm.watcher.Add(fsm.tasksDir); err != nil {
		return err
	}

	// Watch subdirectories in teams
	fsm.watchSubdirectories(fsm.teamsDir)
	fsm.watchSubdirectories(fsm.tasksDir)

	go fsm.watch()
	return nil
}

// watchSubdirectories adds watchers for all subdirectories
func (fsm *FileSystemMonitor) watchSubdirectories(dir string) {
	filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if info.IsDir() && path != dir {
			fsm.watcher.Add(path)
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

			// Handle directory creation to watch new subdirectories
			if event.Op&fsnotify.Create == fsnotify.Create {
				if info, err := os.Stat(event.Name); err == nil && info.IsDir() {
					fsm.watcher.Add(event.Name)
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

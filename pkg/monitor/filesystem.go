package monitor

import (
	"log"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/fsnotify/fsnotify"
)

var projectsSessionIDPattern = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)

// FileSystemMonitorOptions controls filesystem watcher behavior.
type FileSystemMonitorOptions struct {
	Provider ProviderMode
}

// FileSystemMonitor monitors Claude/Codex runtime directories.
type FileSystemMonitor struct {
	watcher          *fsnotify.Watcher
	provider         ProviderMode
	claudeDir        string
	teamsDir         string
	tasksDir         string
	projectsDir      string
	codexDir         string
	codexSessionsDir string
	onChange         func(event fsnotify.Event)
}

// NewFileSystemMonitor creates a new filesystem monitor
func NewFileSystemMonitor(options FileSystemMonitorOptions, onChange func(event fsnotify.Event)) (*FileSystemMonitor, error) {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}

	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}

	provider := normalizeProviderMode(options.Provider)

	fsm := &FileSystemMonitor{
		watcher:  watcher,
		provider: provider,
		onChange: onChange,
	}

	if provider.IncludesClaude() {
		fsm.claudeDir = filepath.Join(homeDir, ".claude")
		fsm.teamsDir = filepath.Join(fsm.claudeDir, "teams")
		fsm.tasksDir = filepath.Join(fsm.claudeDir, "tasks")
		fsm.projectsDir = filepath.Join(fsm.claudeDir, "projects")
	}
	if provider.IncludesCodex() {
		fsm.codexDir = filepath.Join(homeDir, ".codex")
		fsm.codexSessionsDir = filepath.Join(fsm.codexDir, "sessions")
	}

	return fsm, nil
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
	if fsm.provider.IncludesClaude() {
		if err := os.MkdirAll(fsm.teamsDir, 0755); err != nil {
			return err
		}
		if err := os.MkdirAll(fsm.tasksDir, 0755); err != nil {
			return err
		}
		if err := os.MkdirAll(fsm.projectsDir, 0755); err != nil {
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
		if err := fsm.addWatch(fsm.projectsDir); err != nil {
			return err
		}

		fsm.watchSubdirectories(fsm.teamsDir)
		fsm.watchSubdirectories(fsm.tasksDir)
		fsm.watchProjectsSubdirectories()
	}

	if fsm.provider.IncludesCodex() {
		if err := os.MkdirAll(fsm.codexSessionsDir, 0755); err != nil {
			return err
		}
		if err := fsm.addWatch(fsm.codexDir); err != nil {
			return err
		}
		if err := fsm.addWatch(fsm.codexSessionsDir); err != nil {
			return err
		}
		fsm.watchSubdirectories(fsm.codexSessionsDir)
	}

	return nil
}

func (fsm *FileSystemMonitor) addWatch(path string) error {
	if strings.TrimSpace(path) == "" {
		return nil
	}
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
	if strings.TrimSpace(dir) == "" {
		return
	}
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

// watchProjectsSubdirectories adds focused watchers for ~/.claude/projects:
// - projects root
// - each project directory (level 1)
// - each session directory (level 2, UUID-like)
// - each subagents directory (level 3, named "subagents")
func (fsm *FileSystemMonitor) watchProjectsSubdirectories() {
	fsm.watchProjectsSubtree(fsm.projectsDir)
}

func (fsm *FileSystemMonitor) watchProjectsSubtree(root string) {
	filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if !info.IsDir() {
			return nil
		}
		if !fsm.shouldWatchProjectsDir(path) {
			return nil
		}

		if err := fsm.addWatch(path); err != nil {
			log.Printf("Failed to watch project directory %s: %v", path, err)
		}
		return nil
	})
}

func (fsm *FileSystemMonitor) shouldWatchProjectsDir(path string) bool {
	if strings.TrimSpace(fsm.projectsDir) == "" {
		return false
	}

	cleanProjects := filepath.Clean(fsm.projectsDir)
	cleanPath := filepath.Clean(path)

	if cleanPath == cleanProjects {
		return true
	}

	rel, err := filepath.Rel(cleanProjects, cleanPath)
	if err != nil {
		return false
	}
	if rel == "." || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return false
	}

	parts := strings.Split(rel, string(filepath.Separator))
	switch len(parts) {
	case 1:
		// ~/.claude/projects/<project-slug>
		return true
	case 2:
		// ~/.claude/projects/<project-slug>/<session-id>
		return projectsSessionIDPattern.MatchString(parts[1])
	case 3:
		// ~/.claude/projects/<project-slug>/<session-id>/subagents
		return projectsSessionIDPattern.MatchString(parts[1]) && parts[2] == "subagents"
	default:
		return false
	}
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
					if fsm.shouldWatchProjectsDir(event.Name) {
						if err := fsm.addWatch(event.Name); err != nil {
							log.Printf("Failed to watch new project directory %s: %v", event.Name, err)
						}
						fsm.watchProjectsSubtree(event.Name)
					} else {
						if err := fsm.addWatch(event.Name); err != nil {
							log.Printf("Failed to watch new directory %s: %v", event.Name, err)
						}
						fsm.watchSubdirectories(event.Name)
					}
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

// GetCodexSessionsDir returns the codex sessions directory path.
func (fsm *FileSystemMonitor) GetCodexSessionsDir() string {
	return fsm.codexSessionsDir
}

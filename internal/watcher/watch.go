package watcher

import (
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/fsnotify/fsnotify"
)

// Watcher wraps fsnotify and sends change events
type Watcher struct {
	fsw     *fsnotify.Watcher
	dir     string
	Changes chan struct{}
	done    chan struct{}
}

// shouldIgnore returns true for paths we don't want to watch
func shouldIgnore(path string) bool {
	base := filepath.Base(path)

	// Ignore hidden directories and common noise
	ignoreDirs := []string{".git", "node_modules", ".next", "vendor", "__pycache__", ".cache"}
	for _, d := range ignoreDirs {
		if base == d || strings.Contains(path, "/"+d+"/") {
			return true
		}
	}

	// Ignore temp files
	if strings.HasSuffix(base, ".swp") || strings.HasSuffix(base, ".swo") || strings.HasSuffix(base, "~") {
		return true
	}

	return false
}

// New creates a new file watcher for a directory
func New(dir string) (*Watcher, error) {
	fsw, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}

	w := &Watcher{
		fsw:     fsw,
		dir:     dir,
		Changes: make(chan struct{}, 1),
		done:    make(chan struct{}),
	}

	// Add directory and subdirectories
	err = filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // Skip errors
		}
		if info.IsDir() {
			if shouldIgnore(path) {
				return filepath.SkipDir
			}
			fsw.Add(path)
		}
		return nil
	})
	if err != nil {
		fsw.Close()
		return nil, err
	}

	return w, nil
}

// Start begins watching for changes
func (w *Watcher) Start() {
	go func() {
		// Debounce timer
		var debounceTimer *time.Timer

		defer func() {
			if debounceTimer != nil {
				debounceTimer.Stop()
			}
		}()

		for {
			var debounce <-chan time.Time
			if debounceTimer != nil {
				debounce = debounceTimer.C
			}

			select {
			case <-w.done:
				return
			case event, ok := <-w.fsw.Events:
				if !ok {
					return
				}

				// Skip ignored paths
				if shouldIgnore(event.Name) {
					continue
				}

				// If a new directory is created, watch it
				if event.Op&fsnotify.Create != 0 {
					info, err := os.Stat(event.Name)
					if err == nil && info.IsDir() && !shouldIgnore(event.Name) {
						w.fsw.Add(event.Name)
					}
				}

				// Debounce: wait 100ms for more changes before notifying
				if debounceTimer != nil {
					debounceTimer.Stop()
				}
				debounceTimer = time.NewTimer(100 * time.Millisecond)

			case <-debounce:
				// Send change notification (non-blocking)
				select {
				case w.Changes <- struct{}{}:
				default:
				}
				debounceTimer = nil

			case _, ok := <-w.fsw.Errors:
				if !ok {
					return
				}
				// Ignore errors, keep watching
			}
		}
	}()
}

// Close stops the watcher
func (w *Watcher) Close() error {
	close(w.done)
	close(w.Changes)
	return w.fsw.Close()
}

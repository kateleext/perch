package watcher

import "github.com/fsnotify/fsnotify"

// Watcher wraps fsnotify and sends change events
type Watcher struct {
	fsw     *fsnotify.Watcher
	Changes chan string
	Errors  chan error
}

// New creates a new file watcher for a directory
func New(dir string) (*Watcher, error) {
	// TODO: create fsnotify watcher
	// TODO: add directory recursively
	// TODO: filter out .git, node_modules, etc
	return nil, nil
}

// Start begins watching for changes
func (w *Watcher) Start() {
	// TODO: goroutine that forwards fsnotify events
	// TODO: debounce rapid changes
}

// Close stops the watcher
func (w *Watcher) Close() error {
	return w.fsw.Close()
}

// Package watchman provides fsnotify fallback implementation
package watchman

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/poltergeist/poltergeist/pkg/logger"
)

// FSNotifyWatcher provides file watching using fsnotify
type FSNotifyWatcher struct {
	watcher        *fsnotify.Watcher
	logger         logger.Logger
	patterns       []string
	exclusions     []string
	callbacks      map[string]func(FileEvent)
	pendingWatches map[string]*pendingWatch
	settling       time.Duration
	pendingEvents  map[string]pendingFSEvent
	nextGeneration uint64
	mu             sync.RWMutex
	processOnce    sync.Once
	ctx            context.Context
	cancel         context.CancelFunc
}

type pendingFSEvent struct {
	event      fsnotify.Event
	generation uint64
}

type pendingWatch struct {
	callback  func(FileEvent)
	events    []FileEvent
	committed bool
}

// NewFSNotifyWatcher creates a new fsnotify-based watcher
func NewFSNotifyWatcher(log logger.Logger) (*FSNotifyWatcher, error) {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, fmt.Errorf("failed to create fsnotify watcher: %w", err)
	}

	ctx, cancel := context.WithCancel(context.Background())

	return &FSNotifyWatcher{
		watcher:        watcher,
		logger:         log,
		callbacks:      make(map[string]func(FileEvent)),
		pendingWatches: make(map[string]*pendingWatch),
		pendingEvents:  make(map[string]pendingFSEvent),
		settling:       100 * time.Millisecond, // Default settling time
		ctx:            ctx,
		cancel:         cancel,
	}, nil
}

// Close closes the watcher
func (f *FSNotifyWatcher) Close() error {
	f.cancel()
	return f.watcher.Close()
}

// SetPatterns sets the file patterns to watch
func (f *FSNotifyWatcher) SetPatterns(patterns []string) {
	f.mu.Lock()
	f.patterns = patterns
	f.mu.Unlock()
}

// SetExclusions sets the exclusion patterns
func (f *FSNotifyWatcher) SetExclusions(exclusions []string) {
	f.mu.Lock()
	f.exclusions = exclusions
	f.mu.Unlock()
}

// SetSettlingDelay sets the delay for event settling
func (f *FSNotifyWatcher) SetSettlingDelay(delay time.Duration) {
	f.mu.Lock()
	f.settling = delay
	f.mu.Unlock()
}

// Watch starts watching a directory
func (f *FSNotifyWatcher) Watch(root string, callback func(FileEvent)) error {
	f.beginWatch(root, callback)
	f.startProcessing()

	// Add root directory
	if err := f.addDirectory(root); err != nil {
		f.abortWatch(root)
		return fmt.Errorf("failed to watch %s: %w", root, err)
	}
	f.commitWatch(root)

	f.logger.Info(fmt.Sprintf("Started watching %s with fsnotify", root))
	return nil
}

// WatchProject watches an entire project directory recursively
func (f *FSNotifyWatcher) WatchProject(projectPath string, callback func(FileEvent)) error {
	f.beginWatch(projectPath, callback)
	// Windows can emit events while directories are being registered. Consume
	// them before walking so fsnotify's backend cannot block a later Add call.
	f.startProcessing()

	// Walk the directory tree and add all directories
	err := filepath.Walk(projectPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Skip excluded paths
		if f.isExcluded(path) {
			if info.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		// Add directories to watcher
		if info.IsDir() {
			if err := f.watcher.Add(path); err != nil {
				f.logger.Warn(fmt.Sprintf("Failed to watch directory %s: %v", path, err))
			} else {
				f.logger.Debug(fmt.Sprintf("Watching directory: %s", path))
			}
		}

		return nil
	})

	if err != nil {
		f.abortWatch(projectPath)
		return fmt.Errorf("failed to walk project directory: %w", err)
	}
	f.commitWatch(projectPath)

	return nil
}

func (f *FSNotifyWatcher) startProcessing() {
	f.processOnce.Do(func() {
		go f.processEvents()
	})
}

func (f *FSNotifyWatcher) beginWatch(root string, callback func(FileEvent)) {
	f.mu.Lock()
	f.pendingWatches[root] = &pendingWatch{callback: callback}
	f.mu.Unlock()
}

func (f *FSNotifyWatcher) commitWatch(root string) {
	f.mu.Lock()
	pending := f.pendingWatches[root]
	if pending == nil {
		f.mu.Unlock()
		return
	}
	pending.committed = true
	f.mu.Unlock()

	go f.activateWatch(root, pending)
}

func (f *FSNotifyWatcher) activateWatch(root string, pending *pendingWatch) {
	for {
		f.mu.Lock()
		current := f.pendingWatches[root]
		if current != pending || !pending.committed {
			f.mu.Unlock()
			return
		}
		if len(pending.events) == 0 {
			f.callbacks[root] = pending.callback
			delete(f.pendingWatches, root)
			f.mu.Unlock()
			return
		}
		event := pending.events[0]
		pending.events = pending.events[1:]
		f.mu.Unlock()

		pending.callback(event)
	}
}

func (f *FSNotifyWatcher) abortWatch(root string) {
	f.mu.Lock()
	pending := f.pendingWatches[root]
	delete(f.pendingWatches, root)
	var events []FileEvent
	if pending != nil {
		events = append(events, pending.events...)
	}
	f.mu.Unlock()

	for _, event := range events {
		f.dispatchEvent(event)
	}
}

// addDirectory adds a directory to the watcher
func (f *FSNotifyWatcher) addDirectory(dir string) error {
	// Check if directory should be excluded
	if f.isExcluded(dir) {
		return nil
	}

	// Add the directory
	if err := f.watcher.Add(dir); err != nil {
		return err
	}

	// Recursively add subdirectories
	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}

	for _, entry := range entries {
		if entry.IsDir() {
			subdir := filepath.Join(dir, entry.Name())
			if !f.isExcluded(subdir) {
				if err := f.addDirectory(subdir); err != nil {
					f.logger.Warn(fmt.Sprintf("Failed to watch subdirectory %s: %v", subdir, err))
				}
			}
		}
	}

	return nil
}

// processEvents processes fsnotify events
func (f *FSNotifyWatcher) processEvents() {
	for {
		select {
		case <-f.ctx.Done():
			return

		case event, ok := <-f.watcher.Events:
			if !ok {
				return
			}

			// Skip if path is excluded or doesn't match patterns
			if f.isExcluded(event.Name) || !f.matchesPattern(event.Name) {
				continue
			}

			// Handle directory creation - add to watcher
			if event.Op&fsnotify.Create == fsnotify.Create {
				if info, err := os.Stat(event.Name); err == nil && info.IsDir() {
					f.addDirectory(event.Name)
				}
			}

			// Apply settling delay
			f.handleEventWithSettling(event)

		case err, ok := <-f.watcher.Errors:
			if !ok {
				return
			}
			f.logger.Error(fmt.Sprintf("Watcher error: %v", err))
		}
	}
}

// handleEventWithSettling handles an event with settling delay
func (f *FSNotifyWatcher) handleEventWithSettling(event fsnotify.Event) {
	f.mu.Lock()
	if pending, exists := f.pendingEvents[event.Name]; exists {
		event.Op |= pending.event.Op
	}
	f.nextGeneration++
	generation := f.nextGeneration
	f.pendingEvents[event.Name] = pendingFSEvent{
		event:      event,
		generation: generation,
	}
	settlingDelay := f.settling
	f.mu.Unlock()

	// Schedule event processing after settling delay
	time.AfterFunc(settlingDelay, func() {
		f.mu.Lock()
		pending, exists := f.pendingEvents[event.Name]
		if !exists || pending.generation != generation {
			// Event was updated or removed, skip
			f.mu.Unlock()
			return
		}
		delete(f.pendingEvents, event.Name)
		f.mu.Unlock()

		// Convert and dispatch event
		fileEvent := f.convertEvent(pending.event)
		f.dispatchEvent(fileEvent)
	})
}

// convertEvent converts fsnotify event to FileEvent
func (f *FSNotifyWatcher) convertEvent(event fsnotify.Event) FileEvent {
	fileEvent := FileEvent{
		Path: event.Name,
	}

	// Determine event type
	switch {
	case event.Op&fsnotify.Create == fsnotify.Create:
		fileEvent.Type = FileCreated
	case event.Op&fsnotify.Write == fsnotify.Write:
		fileEvent.Type = FileModified
	case event.Op&fsnotify.Remove == fsnotify.Remove:
		fileEvent.Type = FileDeleted
	case event.Op&fsnotify.Rename == fsnotify.Rename:
		fileEvent.Type = FileRenamed
	default:
		fileEvent.Type = FileModified
	}

	// Get file info if it exists
	if info, err := os.Stat(event.Name); err == nil {
		fileEvent.IsDir = info.IsDir()
		fileEvent.Size = info.Size()
		fileEvent.Mode = info.Mode()
		fileEvent.ModTime = info.ModTime()
	} else if fileEvent.Type != FileDeleted {
		// File doesn't exist but event isn't delete - might be rename
		fileEvent.Type = FileDeleted
	}

	return fileEvent
}

// dispatchEvent dispatches an event to the appropriate callback
func (f *FSNotifyWatcher) dispatchEvent(event FileEvent) {
	f.mu.Lock()

	// Find the best matching callback
	var bestMatch string
	var bestCallback func(FileEvent)

	for root, callback := range f.callbacks {
		if strings.HasPrefix(event.Path, root) {
			if len(root) > len(bestMatch) {
				bestMatch = root
				bestCallback = callback
			}
		}
	}

	var bestPending *pendingWatch
	for root, pending := range f.pendingWatches {
		if strings.HasPrefix(event.Path, root) && len(root) > len(bestMatch) {
			bestMatch = root
			bestCallback = nil
			bestPending = pending
		}
	}

	if bestPending != nil {
		bestPending.events = append(bestPending.events, event)
		f.mu.Unlock()
		return
	}
	f.mu.Unlock()

	if bestCallback != nil {
		bestCallback(event)
	}
}

// isExcluded checks if a path should be excluded
func (f *FSNotifyWatcher) isExcluded(path string) bool {
	f.mu.RLock()
	defer f.mu.RUnlock()

	// Check against exclusion patterns
	for _, pattern := range f.exclusions {
		// Simple pattern matching - could be enhanced with glob support
		if strings.Contains(path, pattern) {
			return true
		}
	}

	// Check common exclusions
	base := filepath.Base(path)
	commonExclusions := []string{
		".git", ".svn", ".hg", ".bzr",
		"node_modules", "vendor", ".idea",
		".vscode", "__pycache__", ".pytest_cache",
		"target", "build", "dist", "out",
	}

	for _, exc := range commonExclusions {
		if base == exc {
			return true
		}
	}

	return false
}

// matchesPattern checks if a path matches watch patterns
func (f *FSNotifyWatcher) matchesPattern(path string) bool {
	f.mu.RLock()
	defer f.mu.RUnlock()

	// If no patterns specified, match everything
	if len(f.patterns) == 0 {
		return true
	}

	// Check against patterns
	for _, pattern := range f.patterns {
		// Simple pattern matching
		if matched, _ := filepath.Match(pattern, filepath.Base(path)); matched {
			return true
		}

		// Check if pattern matches any part of the path
		if strings.Contains(pattern, "**") {
			// Handle ** glob pattern
			parts := strings.Split(pattern, "**")
			if len(parts) == 2 {
				prefix := parts[0]
				suffix := parts[1]
				if strings.HasPrefix(path, prefix) {
					if suffix == "" || strings.HasSuffix(path, strings.TrimPrefix(suffix, "/")) {
						return true
					}
				}
			}
		}
	}

	return false
}

// Remove stops watching a path
func (f *FSNotifyWatcher) Remove(path string) error {
	f.mu.Lock()
	delete(f.callbacks, path)
	delete(f.pendingWatches, path)
	f.mu.Unlock()

	watchedPaths := f.watcher.WatchList()
	removed := false
	var firstErr error
	for _, watchedPath := range watchedPaths {
		if !pathWithinRoot(watchedPath, path) {
			continue
		}
		removed = true
		if err := f.watcher.Remove(watchedPath); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	if !removed {
		return f.watcher.Remove(path)
	}
	return firstErr
}

func pathWithinRoot(path, root string) bool {
	relative, err := filepath.Rel(root, path)
	if err != nil {
		return false
	}
	return relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator))
}

// List returns all watched paths
func (f *FSNotifyWatcher) List() []string {
	return f.watcher.WatchList()
}

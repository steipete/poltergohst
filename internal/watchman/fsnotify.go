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
	watcher      *fsnotify.Watcher
	logger       logger.Logger
	patterns     []string
	exclusions   []string
	callbacks    map[string]func(FileEvent)
	settling     time.Duration
	pendingEvents map[string]time.Time
	mu           sync.RWMutex
	ctx          context.Context
	cancel       context.CancelFunc
}

// NewFSNotifyWatcher creates a new fsnotify-based watcher
func NewFSNotifyWatcher(log logger.Logger) (*FSNotifyWatcher, error) {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, fmt.Errorf("failed to create fsnotify watcher: %w", err)
	}
	
	ctx, cancel := context.WithCancel(context.Background())
	
	return &FSNotifyWatcher{
		watcher:       watcher,
		logger:        log,
		callbacks:     make(map[string]func(FileEvent)),
		pendingEvents: make(map[string]time.Time),
		settling:      100 * time.Millisecond, // Default settling time
		ctx:           ctx,
		cancel:        cancel,
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
	// Store callback
	f.mu.Lock()
	f.callbacks[root] = callback
	f.mu.Unlock()
	
	// Add root directory
	if err := f.addDirectory(root); err != nil {
		return fmt.Errorf("failed to watch %s: %w", root, err)
	}
	
	// Start event processing
	go f.processEvents()
	
	f.logger.Info(fmt.Sprintf("Started watching %s with fsnotify", root))
	return nil
}

// WatchProject watches an entire project directory recursively
func (f *FSNotifyWatcher) WatchProject(projectPath string, callback func(FileEvent)) error {
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
		return fmt.Errorf("failed to walk project directory: %w", err)
	}
	
	// Store callback for this project
	f.mu.Lock()
	f.callbacks[projectPath] = callback
	f.mu.Unlock()
	
	// Start event processing if not already started
	go f.processEvents()
	
	return nil
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
	f.pendingEvents[event.Name] = time.Now()
	settlingDelay := f.settling
	f.mu.Unlock()
	
	// Schedule event processing after settling delay
	time.AfterFunc(settlingDelay, func() {
		f.mu.Lock()
		lastEventTime, exists := f.pendingEvents[event.Name]
		if !exists || time.Since(lastEventTime) < settlingDelay {
			// Event was updated or removed, skip
			f.mu.Unlock()
			return
		}
		delete(f.pendingEvents, event.Name)
		f.mu.Unlock()
		
		// Convert and dispatch event
		fileEvent := f.convertEvent(event)
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
	f.mu.RLock()
	defer f.mu.RUnlock()
	
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
	f.mu.Unlock()
	
	return f.watcher.Remove(path)
}

// List returns all watched paths
func (f *FSNotifyWatcher) List() []string {
	return f.watcher.WatchList()
}
// Package config provides configuration management including hot-reload functionality
package config

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
	"github.com/poltergeist/poltergeist/pkg/types"
)

// ReloadManager handles configuration hot-reload functionality
type ReloadManager struct {
	configPath      string
	logger          logger.Logger
	watcher         *fsnotify.Watcher
	callbacks       []ReloadCallback
	lastModTime     time.Time
	debounceTimer   *time.Timer
	debouncePeriod  time.Duration
	mu              sync.RWMutex
	ctx             context.Context
	cancel          context.CancelFunc
	isWatching      bool
}

// ReloadCallback is called when configuration changes
type ReloadCallback func(*types.PoltergeistConfig, error)

// ReloadEvent represents a configuration reload event
type ReloadEvent struct {
	Path      string                     `json:"path"`
	Timestamp time.Time                  `json:"timestamp"`
	Config    *types.PoltergeistConfig   `json:"config,omitempty"`
	Error     error                      `json:"error,omitempty"`
	EventType ReloadEventType            `json:"eventType"`
}

// ReloadEventType represents the type of reload event
type ReloadEventType string

const (
	ReloadEventTypeModified ReloadEventType = "modified"
	ReloadEventTypeCreated  ReloadEventType = "created"
	ReloadEventTypeRemoved  ReloadEventType = "removed"
	ReloadEventTypeError    ReloadEventType = "error"
)

// NewReloadManager creates a new configuration reload manager
func NewReloadManager(configPath string, log logger.Logger) *ReloadManager {
	ctx, cancel := context.WithCancel(context.Background())
	
	return &ReloadManager{
		configPath:     configPath,
		logger:         log,
		debouncePeriod: 500 * time.Millisecond,
		ctx:            ctx,
		cancel:         cancel,
	}
}

// AddCallback adds a reload callback
func (rm *ReloadManager) AddCallback(callback ReloadCallback) {
	rm.mu.Lock()
	defer rm.mu.Unlock()
	rm.callbacks = append(rm.callbacks, callback)
}

// RemoveAllCallbacks removes all reload callbacks
func (rm *ReloadManager) RemoveAllCallbacks() {
	rm.mu.Lock()
	defer rm.mu.Unlock()
	rm.callbacks = nil
}

// StartWatching begins watching the configuration file for changes
func (rm *ReloadManager) StartWatching() error {
	rm.mu.Lock()
	defer rm.mu.Unlock()

	if rm.isWatching {
		return fmt.Errorf("already watching configuration file")
	}

	// Create file watcher
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return fmt.Errorf("failed to create file watcher: %w", err)
	}
	rm.watcher = watcher

	// Add config file to watcher
	configDir := filepath.Dir(rm.configPath)
	if err := rm.watcher.Add(configDir); err != nil {
		rm.watcher.Close()
		return fmt.Errorf("failed to watch config directory: %w", err)
	}

	// Get initial modification time
	if stat, err := os.Stat(rm.configPath); err == nil {
		rm.lastModTime = stat.ModTime()
	}

	rm.isWatching = true

	// Start watching in background
	go rm.watchLoop()

	rm.logger.Debug("Started watching configuration file",
		logger.WithField("path", rm.configPath))

	return nil
}

// StopWatching stops watching the configuration file
func (rm *ReloadManager) StopWatching() error {
	rm.mu.Lock()
	defer rm.mu.Unlock()

	if !rm.isWatching {
		return nil
	}

	rm.cancel()

	if rm.debounceTimer != nil {
		rm.debounceTimer.Stop()
		rm.debounceTimer = nil
	}

	if rm.watcher != nil {
		if err := rm.watcher.Close(); err != nil {
			rm.logger.Warn("Error closing file watcher", logger.WithField("error", err))
		}
		rm.watcher = nil
	}

	rm.isWatching = false

	rm.logger.Debug("Stopped watching configuration file")
	return nil
}

// IsWatching returns whether the manager is currently watching
func (rm *ReloadManager) IsWatching() bool {
	rm.mu.RLock()
	defer rm.mu.RUnlock()
	return rm.isWatching
}

// TriggerReload manually triggers a configuration reload
func (rm *ReloadManager) TriggerReload() {
	rm.logger.Debug("Manually triggering configuration reload")
	rm.handleConfigChange(ReloadEventTypeModified)
}

// ValidateBeforeReload validates configuration before triggering callbacks
func (rm *ReloadManager) ValidateBeforeReload(enable bool) {
	// This could be extended to add validation logic
	rm.logger.Debug("Configuration validation setting updated",
		logger.WithField("enabled", enable))
}

func (rm *ReloadManager) watchLoop() {
	defer func() {
		if r := recover(); r != nil {
			rm.logger.Error("Configuration watcher panic recovered",
				logger.WithField("panic", r))
		}
	}()

	for {
		select {
		case <-rm.ctx.Done():
			return

		case event, ok := <-rm.watcher.Events:
			if !ok {
				return
			}

			// Only process events for our config file
			if !rm.isConfigFileEvent(event.Name) {
				continue
			}

			rm.logger.Debug("Configuration file event received",
				logger.WithField("event", event.String()))

			eventType := rm.mapFsnotifyEvent(event.Op)
			rm.debounceReload(eventType)

		case err, ok := <-rm.watcher.Errors:
			if !ok {
				return
			}

			rm.logger.Error("Configuration file watcher error",
				logger.WithField("error", err))

			// Notify callbacks about the error
			rm.notifyCallbacks(nil, err, ReloadEventTypeError)
		}
	}
}

func (rm *ReloadManager) isConfigFileEvent(eventPath string) bool {
	// Check if event is for our config file or a related file
	configFileName := filepath.Base(rm.configPath)
	eventFileName := filepath.Base(eventPath)

	// Direct match
	if eventFileName == configFileName {
		return true
	}

	// Check for temporary files that editors create
	return strings.HasPrefix(eventFileName, configFileName) ||
		   strings.HasSuffix(eventFileName, ".tmp") &&
		   strings.Contains(eventFileName, configFileName)
}

func (rm *ReloadManager) mapFsnotifyEvent(op fsnotify.Op) ReloadEventType {
	switch {
	case op&fsnotify.Write == fsnotify.Write:
		return ReloadEventTypeModified
	case op&fsnotify.Create == fsnotify.Create:
		return ReloadEventTypeCreated
	case op&fsnotify.Remove == fsnotify.Remove:
		return ReloadEventTypeRemoved
	default:
		return ReloadEventTypeModified
	}
}

func (rm *ReloadManager) debounceReload(eventType ReloadEventType) {
	rm.mu.Lock()
	defer rm.mu.Unlock()

	// Cancel existing timer
	if rm.debounceTimer != nil {
		rm.debounceTimer.Stop()
	}

	// Create new timer
	rm.debounceTimer = time.AfterFunc(rm.debouncePeriod, func() {
		rm.handleConfigChange(eventType)
	})
}

func (rm *ReloadManager) handleConfigChange(eventType ReloadEventType) {
	rm.logger.Debug("Processing configuration change",
		logger.WithField("eventType", eventType))

	// Check if file was removed
	if eventType == ReloadEventTypeRemoved {
		err := fmt.Errorf("configuration file was removed: %s", rm.configPath)
		rm.notifyCallbacks(nil, err, eventType)
		return
	}

	// Check file modification time to avoid duplicate reloads
	stat, err := os.Stat(rm.configPath)
	if err != nil {
		rm.logger.Error("Failed to stat configuration file",
			logger.WithField("error", err))
		rm.notifyCallbacks(nil, err, ReloadEventTypeError)
		return
	}

	rm.mu.Lock()
	if !stat.ModTime().After(rm.lastModTime) {
		rm.mu.Unlock()
		rm.logger.Debug("Configuration file not modified, skipping reload")
		return
	}
	rm.lastModTime = stat.ModTime()
	rm.mu.Unlock()

	// Load new configuration
	manager := NewManager()
	config, err := manager.LoadConfig(rm.configPath)
	if err != nil {
		rm.logger.Error("Failed to reload configuration",
			logger.WithField("error", err))
		rm.notifyCallbacks(nil, err, ReloadEventTypeError)
		return
	}

	rm.logger.Info("Configuration reloaded successfully",
		logger.WithField("targets", len(config.Targets)))

	// Notify callbacks
	rm.notifyCallbacks(config, nil, eventType)
}

func (rm *ReloadManager) notifyCallbacks(config *types.PoltergeistConfig, err error, eventType ReloadEventType) {
	rm.mu.RLock()
	callbacks := make([]ReloadCallback, len(rm.callbacks))
	copy(callbacks, rm.callbacks)
	rm.mu.RUnlock()

	// Create reload event for tracking
	_ = ReloadEvent{
		Path:      rm.configPath,
		Timestamp: time.Now(),
		Config:    config,
		Error:     err,
		EventType: eventType,
	}

	rm.logger.Debug("Notifying reload callbacks",
		logger.WithField("callbackCount", len(callbacks)),
		logger.WithField("eventType", eventType))

	// Notify all callbacks
	for _, callback := range callbacks {
		go func(cb ReloadCallback) {
			defer func() {
				if r := recover(); r != nil {
					rm.logger.Error("Reload callback panic recovered",
						logger.WithField("panic", r))
				}
			}()
			cb(config, err)
		}(callback)
	}
}

// SetDebouncePeriod sets the debounce period for file change events
func (rm *ReloadManager) SetDebouncePeriod(period time.Duration) {
	rm.mu.Lock()
	defer rm.mu.Unlock()
	rm.debouncePeriod = period
}

// GetLastReloadTime returns the timestamp of the last configuration reload
func (rm *ReloadManager) GetLastReloadTime() time.Time {
	rm.mu.RLock()
	defer rm.mu.RUnlock()
	return rm.lastModTime
}

// GetConfigPath returns the path of the watched configuration file
func (rm *ReloadManager) GetConfigPath() string {
	return rm.configPath
}
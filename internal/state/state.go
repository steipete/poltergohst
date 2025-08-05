// Package state provides persistent state management for Poltergeist
package state

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/poltergeist/poltergeist/pkg/logger"
	"github.com/poltergeist/poltergeist/pkg/types"
)

// PoltergeistState represents the persistent state of a target
type PoltergeistState struct {
	TargetName    string              `json:"targetName"`
	BuildStatus   types.BuildStatus   `json:"buildStatus"`
	LastBuildTime time.Time           `json:"lastBuildTime"`
	BuildCount    int                 `json:"buildCount"`
	FailureCount  int                 `json:"failureCount"`
	ProcessID     int                 `json:"processId"`
	Heartbeat     time.Time           `json:"heartbeat"`
	LastError     string              `json:"lastError,omitempty"`
	BuildDuration time.Duration       `json:"buildDuration,omitempty"`
	ChangedFiles  []string            `json:"changedFiles,omitempty"`
	Metadata      map[string]interface{} `json:"metadata,omitempty"`
}

// StateManager handles persistent state files
type StateManager struct {
	stateDir       string
	logger         logger.Logger
	mu             sync.RWMutex
	states         map[string]*PoltergeistState
	heartbeatStop  chan struct{}
	heartbeatTimer *time.Ticker
}

// NewStateManager creates a new state manager
func NewStateManager(projectRoot string, log logger.Logger) *StateManager {
	stateDir := filepath.Join(projectRoot, ".poltergeist", "state")
	
	// Ensure state directory exists
	if err := os.MkdirAll(stateDir, 0755); err != nil {
		log.Error("Failed to create state directory", logger.WithField("error", err))
	}
	
	return &StateManager{
		stateDir: stateDir,
		logger:   log,
		states:   make(map[string]*PoltergeistState),
	}
}

// InitializeState creates or updates state for a target
func (sm *StateManager) InitializeState(target types.Target) (*PoltergeistState, error) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	
	state := &PoltergeistState{
		TargetName:  target.GetName(),
		BuildStatus: types.BuildStatusIdle,
		ProcessID:   os.Getpid(),
		Heartbeat:   time.Now(),
		Metadata:    make(map[string]interface{}),
	}
	
	// Load existing state if available
	existingState, err := sm.loadStateFile(target.GetName())
	if err == nil && existingState != nil {
		// Preserve build statistics
		state.BuildCount = existingState.BuildCount
		state.FailureCount = existingState.FailureCount
		state.LastBuildTime = existingState.LastBuildTime
		state.BuildDuration = existingState.BuildDuration
	}
	
	// Save state
	if err := sm.saveStateFile(state); err != nil {
		return nil, fmt.Errorf("failed to save initial state: %w", err)
	}
	
	sm.states[target.GetName()] = state
	return state, nil
}

// ReadState reads the state for a target
func (sm *StateManager) ReadState(targetName string) (*PoltergeistState, error) {
	sm.mu.RLock()
	
	// Check memory cache first
	if state, ok := sm.states[targetName]; ok {
		sm.mu.RUnlock()
		return state, nil
	}
	sm.mu.RUnlock()
	
	// Load from file
	return sm.loadStateFile(targetName)
}

// UpdateState updates the state for a target
func (sm *StateManager) UpdateState(targetName string, updates map[string]interface{}) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	
	state, ok := sm.states[targetName]
	if !ok {
		// Load from file if not in memory
		var err error
		state, err = sm.loadStateFile(targetName)
		if err != nil {
			return fmt.Errorf("target state not found: %s", targetName)
		}
		sm.states[targetName] = state
	}
	
	// Apply updates
	for key, value := range updates {
		switch key {
		case "buildStatus":
			if status, ok := value.(types.BuildStatus); ok {
				state.BuildStatus = status
			}
		case "lastBuildTime":
			if t, ok := value.(time.Time); ok {
				state.LastBuildTime = t
			}
		case "buildCount":
			if count, ok := value.(int); ok {
				state.BuildCount = count
			}
		case "failureCount":
			if count, ok := value.(int); ok {
				state.FailureCount = count
			}
		case "lastError":
			if err, ok := value.(string); ok {
				state.LastError = err
			}
		case "buildDuration":
			if duration, ok := value.(time.Duration); ok {
				state.BuildDuration = duration
			}
		case "changedFiles":
			if files, ok := value.([]string); ok {
				state.ChangedFiles = files
			}
		default:
			// Store in metadata
			if state.Metadata == nil {
				state.Metadata = make(map[string]interface{})
			}
			state.Metadata[key] = value
		}
	}
	
	state.Heartbeat = time.Now()
	
	// Save to file
	return sm.saveStateFile(state)
}

// UpdateBuildStatus updates the build status for a target
func (sm *StateManager) UpdateBuildStatus(targetName string, status types.BuildStatus) error {
	updates := map[string]interface{}{
		"buildStatus": status,
	}
	
	if status == types.BuildStatusSucceeded || status == types.BuildStatusFailed {
		updates["lastBuildTime"] = time.Now()
		
		// Update counters
		sm.mu.RLock()
		state, ok := sm.states[targetName]
		sm.mu.RUnlock()
		
		if ok {
			if status == types.BuildStatusSucceeded {
				updates["buildCount"] = state.BuildCount + 1
			} else {
				updates["failureCount"] = state.FailureCount + 1
			}
		}
	}
	
	return sm.UpdateState(targetName, updates)
}

// RemoveState removes the state for a target
func (sm *StateManager) RemoveState(targetName string) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	
	delete(sm.states, targetName)
	
	stateFile := sm.getStateFilePath(targetName)
	if err := os.Remove(stateFile); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to remove state file: %w", err)
	}
	
	return nil
}

// IsLocked checks if a target is locked by another process
func (sm *StateManager) IsLocked(targetName string) (bool, error) {
	state, err := sm.loadStateFile(targetName)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, err
	}
	
	// Check if process is still alive
	if state.ProcessID == os.Getpid() {
		return false, nil // Our own process
	}
	
	// Check heartbeat (consider dead if older than 30 seconds)
	if time.Since(state.Heartbeat) > 30*time.Second {
		return false, nil
	}
	
	// Check if process exists
	process, err := os.FindProcess(state.ProcessID)
	if err != nil {
		return false, nil
	}
	
	// Try to signal the process (0 signal doesn't kill, just checks)
	if err := process.Signal(os.Signal(nil)); err != nil {
		return false, nil // Process doesn't exist
	}
	
	return true, nil
}

// DiscoverStates finds all existing state files
func (sm *StateManager) DiscoverStates() (map[string]*PoltergeistState, error) {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	
	states := make(map[string]*PoltergeistState)
	
	files, err := os.ReadDir(sm.stateDir)
	if err != nil {
		if os.IsNotExist(err) {
			return states, nil
		}
		return nil, fmt.Errorf("failed to read state directory: %w", err)
	}
	
	for _, file := range files {
		if filepath.Ext(file.Name()) != ".json" {
			continue
		}
		
		targetName := file.Name()[:len(file.Name())-5] // Remove .json
		state, err := sm.loadStateFile(targetName)
		if err != nil {
			sm.logger.Warn("Failed to load state file", 
				logger.WithField("target", targetName),
				logger.WithField("error", err))
			continue
		}
		
		states[targetName] = state
	}
	
	return states, nil
}

// StartHeartbeat starts the heartbeat updater
func (sm *StateManager) StartHeartbeat(ctx context.Context) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	
	if sm.heartbeatTimer != nil {
		return // Already running
	}
	
	sm.heartbeatStop = make(chan struct{})
	sm.heartbeatTimer = time.NewTicker(10 * time.Second)
	
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case <-sm.heartbeatStop:
				return
			case <-sm.heartbeatTimer.C:
				sm.updateHeartbeats()
			}
		}
	}()
}

// StopHeartbeat stops the heartbeat updater
func (sm *StateManager) StopHeartbeat() {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	
	if sm.heartbeatTimer != nil {
		sm.heartbeatTimer.Stop()
		sm.heartbeatTimer = nil
	}
	
	if sm.heartbeatStop != nil {
		close(sm.heartbeatStop)
		sm.heartbeatStop = nil
	}
}

// Cleanup removes stale state files
func (sm *StateManager) Cleanup() error {
	sm.StopHeartbeat()
	
	sm.mu.Lock()
	defer sm.mu.Unlock()
	
	// Mark all our states as stopped
	for _, state := range sm.states {
		state.BuildStatus = types.BuildStatusIdle
		state.ProcessID = 0
		if err := sm.saveStateFile(state); err != nil {
			sm.logger.Warn("Failed to save final state",
				logger.WithField("target", state.TargetName),
				logger.WithField("error", err))
		}
	}
	
	return nil
}

// Private methods

func (sm *StateManager) getStateFilePath(targetName string) string {
	return filepath.Join(sm.stateDir, targetName+".json")
}

func (sm *StateManager) loadStateFile(targetName string) (*PoltergeistState, error) {
	stateFile := sm.getStateFilePath(targetName)
	
	data, err := os.ReadFile(stateFile)
	if err != nil {
		return nil, err
	}
	
	var state PoltergeistState
	if err := json.Unmarshal(data, &state); err != nil {
		return nil, fmt.Errorf("failed to parse state file: %w", err)
	}
	
	return &state, nil
}

func (sm *StateManager) saveStateFile(state *PoltergeistState) error {
	stateFile := sm.getStateFilePath(state.TargetName)
	
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal state: %w", err)
	}
	
	// Write atomically
	tempFile := stateFile + ".tmp"
	if err := os.WriteFile(tempFile, data, 0644); err != nil {
		return fmt.Errorf("failed to write state file: %w", err)
	}
	
	if err := os.Rename(tempFile, stateFile); err != nil {
		os.Remove(tempFile) // Clean up
		return fmt.Errorf("failed to rename state file: %w", err)
	}
	
	return nil
}

func (sm *StateManager) updateHeartbeats() {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	
	now := time.Now()
	for _, state := range sm.states {
		state.Heartbeat = now
		if err := sm.saveStateFile(state); err != nil {
			sm.logger.Debug("Failed to update heartbeat",
				logger.WithField("target", state.TargetName),
				logger.WithField("error", err))
		}
	}
}
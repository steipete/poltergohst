// Package mocks provides mock implementations of interfaces for testing.
// These follow Go best practices for test doubles.
package mocks

import (
	"context"
	"sync"
	"time"

	"github.com/poltergeist/poltergeist/pkg/interfaces"
	"github.com/poltergeist/poltergeist/pkg/logger"
	"github.com/poltergeist/poltergeist/pkg/state"
	"github.com/poltergeist/poltergeist/pkg/types"
)

// MockStateManager is a mock implementation of StateManager for testing
type MockStateManager struct {
	mu           sync.RWMutex
	states       map[string]*state.PoltergeistState
	initError    error
	updateError  error
	cleanupError error
	heartbeatCh  chan struct{}
}

// NewMockStateManager creates a new mock state manager
func NewMockStateManager() *MockStateManager {
	return &MockStateManager{
		states:      make(map[string]*state.PoltergeistState),
		heartbeatCh: make(chan struct{}, 1),
	}
}

// InitializeState initializes state for a target
func (m *MockStateManager) InitializeState(target types.Target) (*state.PoltergeistState, error) {
	if m.initError != nil {
		return nil, m.initError
	}
	
	m.mu.Lock()
	defer m.mu.Unlock()
	
	state := &state.PoltergeistState{
		TargetName:   target.GetName(),
		LastBuildTime: time.Now(),
		BuildStatus:  types.BuildStatusQueued,
		BuildCount:   0,
		FailureCount: 0,
	}
	
	m.states[target.GetName()] = state
	return state, nil
}

// UpdateState updates the state for a target
func (m *MockStateManager) UpdateState(targetName string, status types.BuildStatus, changedFiles []string) error {
	if m.updateError != nil {
		return m.updateError
	}
	
	m.mu.Lock()
	defer m.mu.Unlock()
	
	if state, ok := m.states[targetName]; ok {
		state.BuildStatus = status
		state.LastBuildTime = time.Now()
		state.BuildCount++
		
		if status == types.BuildStatusSucceeded {
			// Success count tracked via BuildCount - FailureCount
		} else if status == types.BuildStatusFailed {
			state.FailureCount++
		}
		
		state.LastError = ""
		if status == types.BuildStatusFailed {
			state.LastError = "mock error"
		}
	}
	
	return nil
}

// GetState retrieves the state for a target
func (m *MockStateManager) GetState(targetName string) (*state.PoltergeistState, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	state, ok := m.states[targetName]
	if !ok {
		return nil, nil
	}
	
	return state, nil
}

// GetAllStates retrieves states for all targets
func (m *MockStateManager) GetAllStates() ([]*state.PoltergeistState, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	states := make([]*state.PoltergeistState, 0, len(m.states))
	for _, state := range m.states {
		states = append(states, state)
	}
	
	return states, nil
}

// StartHeartbeat starts the heartbeat mechanism
func (m *MockStateManager) StartHeartbeat(ctx context.Context) {
	select {
	case m.heartbeatCh <- struct{}{}:
	default:
	}
}

// StopHeartbeat stops the heartbeat mechanism
func (m *MockStateManager) StopHeartbeat() {
	// No-op for mock
}

// Cleanup performs cleanup operations
func (m *MockStateManager) Cleanup() error {
	return m.cleanupError
}

// SetInitError sets the error to return from InitializeState
func (m *MockStateManager) SetInitError(err error) {
	m.initError = err
}

// SetUpdateError sets the error to return from UpdateState
func (m *MockStateManager) SetUpdateError(err error) {
	m.updateError = err
}

// SetCleanupError sets the error to return from Cleanup
func (m *MockStateManager) SetCleanupError(err error) {
	m.cleanupError = err
}

// MockBuilder is a mock implementation of Builder for testing
type MockBuilder struct {
	mu             sync.RWMutex
	validateError  error
	buildError     error
	cleanError     error
	buildCallCount int
	cleanCallCount int
	lastBuildFiles []string
	target         types.Target
	lastBuildTime  time.Duration
}

// NewMockBuilder creates a new mock builder
func NewMockBuilder() *MockBuilder {
	return &MockBuilder{
		lastBuildTime: 100 * time.Millisecond,
	}
}

// Validate validates the builder configuration
func (m *MockBuilder) Validate() error {
	return m.validateError
}

// Build executes the build
func (m *MockBuilder) Build(ctx context.Context, changedFiles []string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	m.buildCallCount++
	m.lastBuildFiles = changedFiles
	
	return m.buildError
}

// Clean cleans build artifacts
func (m *MockBuilder) Clean() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	m.cleanCallCount++
	return m.cleanError
}

// GetTarget returns the target for this builder
func (m *MockBuilder) GetTarget() types.Target {
	return m.target
}

// GetLastBuildTime returns the last build time
func (m *MockBuilder) GetLastBuildTime() time.Duration {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.lastBuildTime
}

// SetTarget sets the target for this builder
func (m *MockBuilder) SetTarget(target types.Target) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.target = target
}

// GetEstimatedBuildTime returns estimated build time
func (m *MockBuilder) GetEstimatedBuildTime() time.Duration {
	return 5 * time.Second
}

// GetSuccessRate returns the success rate of builds
func (m *MockBuilder) GetSuccessRate() float64 {
	return 0.9 // 90% success rate for mock
}

// SetValidateError sets the error to return from Validate
func (m *MockBuilder) SetValidateError(err error) {
	m.validateError = err
}

// SetBuildError sets the error to return from Build
func (m *MockBuilder) SetBuildError(err error) {
	m.buildError = err
}

// SetCleanError sets the error to return from Clean
func (m *MockBuilder) SetCleanError(err error) {
	m.cleanError = err
}

// GetBuildCallCount returns the number of times Build was called
func (m *MockBuilder) GetBuildCallCount() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.buildCallCount
}

// GetLastBuildFiles returns the files from the last Build call
func (m *MockBuilder) GetLastBuildFiles() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.lastBuildFiles
}

// MockBuilderFactory is a mock implementation of BuilderFactory
type MockBuilderFactory struct {
	builders map[string]interfaces.Builder
}

// NewMockBuilderFactory creates a new mock builder factory
func NewMockBuilderFactory() *MockBuilderFactory {
	return &MockBuilderFactory{
		builders: make(map[string]interfaces.Builder),
	}
}

// CreateBuilder creates a builder for a target
func (f *MockBuilderFactory) CreateBuilder(target types.Target, projectRoot string, log logger.Logger, stateManager interfaces.StateManager) interfaces.Builder {
	if builder, ok := f.builders[target.GetName()]; ok {
		return builder
	}
	
	return NewMockBuilder()
}

// RegisterBuilder registers a builder for a target
func (f *MockBuilderFactory) RegisterBuilder(targetName string, builder interfaces.Builder) {
	f.builders[targetName] = builder
}

// MockWatchmanClient is a mock implementation of WatchmanClient
type MockWatchmanClient struct {
	mu            sync.RWMutex
	connected     bool
	connectError  error
	watchError    error
	subscriptions map[string]func([]interfaces.FileChange)
}

// NewMockWatchmanClient creates a new mock Watchman client
func NewMockWatchmanClient() *MockWatchmanClient {
	return &MockWatchmanClient{
		subscriptions: make(map[string]func([]interfaces.FileChange)),
	}
}

// Connect connects to Watchman
func (m *MockWatchmanClient) Connect(ctx context.Context) error {
	if m.connectError != nil {
		return m.connectError
	}
	
	m.mu.Lock()
	defer m.mu.Unlock()
	m.connected = true
	return nil
}

// Disconnect disconnects from Watchman
func (m *MockWatchmanClient) Disconnect() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.connected = false
	return nil
}

// IsConnected checks if connected to Watchman
func (m *MockWatchmanClient) IsConnected() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.connected
}

// WatchProject starts watching a project
func (m *MockWatchmanClient) WatchProject(path string) error {
	return m.watchError
}

// Subscribe subscribes to file changes
func (m *MockWatchmanClient) Subscribe(
	path string,
	name string,
	config interfaces.SubscriptionConfig,
	callback func([]interfaces.FileChange),
	exclusions []interface{},
) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	m.subscriptions[name] = callback
	return nil
}

// Unsubscribe unsubscribes from file changes
func (m *MockWatchmanClient) Unsubscribe(path string, name string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	delete(m.subscriptions, name)
	return nil
}

// TriggerFileChange simulates a file change event
func (m *MockWatchmanClient) TriggerFileChange(subscriptionName string, files []interfaces.FileChange) {
	m.mu.RLock()
	callback, ok := m.subscriptions[subscriptionName]
	m.mu.RUnlock()
	
	if ok && callback != nil {
		callback(files)
	}
}

// SetConnectError sets the error to return from Connect
func (m *MockWatchmanClient) SetConnectError(err error) {
	m.connectError = err
}

// SetWatchError sets the error to return from WatchProject
func (m *MockWatchmanClient) SetWatchError(err error) {
	m.watchError = err
}
package engine

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/poltergeist/poltergeist/pkg/interfaces"
	"github.com/poltergeist/poltergeist/pkg/logger"
	"github.com/poltergeist/poltergeist/pkg/mocks"
	"github.com/poltergeist/poltergeist/pkg/types"
)

// TestPoltergeistStartWithContext tests the StartWithContext method with various scenarios
func TestPoltergeistStartWithContext(t *testing.T) {
	tests := []struct {
		name           string
		setupMocks     func() interfaces.PoltergeistDependencies
		targetName     string
		expectError    bool
		errorContains  string
		checkRunning   bool
	}{
		{
			name: "successful start with all dependencies",
			setupMocks: func() interfaces.PoltergeistDependencies {
				return createValidDependencies()
			},
			targetName:   "",
			expectError:  false,
			checkRunning: true,
		},
		{
			name: "start with specific target",
			setupMocks: func() interfaces.PoltergeistDependencies {
				return createValidDependencies()
			},
			targetName:   "test-target",
			expectError:  false,
			checkRunning: true,
		},
		{
			name: "fail when already running",
			setupMocks: func() interfaces.PoltergeistDependencies {
				return createValidDependencies()
			},
			targetName:    "",
			expectError:   true,
			errorContains: "already running",
			checkRunning:  false,
		},
		{
			name: "fail with watchman connection error",
			setupMocks: func() interfaces.PoltergeistDependencies {
				deps := createValidDependencies()
				mockWatchman := deps.WatchmanClient.(*mocks.MockWatchmanClient)
				mockWatchman.SetConnectError(errors.New("connection failed"))
				return deps
			},
			targetName:    "",
			expectError:   true,
			errorContains: "failed to connect to watchman",
			checkRunning:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()

			config := createTestConfig()
			deps := tt.setupMocks()
			log := logger.CreateLoggerWithOutput("", "debug", nil)
			
			p := New(config, "/test/project", log, deps, "test.json")

			// Special case: test already running error
			if tt.errorContains == "already running" {
				// Start it first
				err := p.StartWithContext(ctx, "")
				if err != nil {
					t.Fatalf("Initial start failed: %v", err)
				}
				defer p.Stop()
			}

			// Execute
			err := p.StartWithContext(ctx, tt.targetName)

			// Verify
			if tt.expectError {
				if err == nil {
					t.Errorf("Expected error but got none")
				} else if tt.errorContains != "" && !contains(err.Error(), tt.errorContains) {
					t.Errorf("Expected error containing '%s', got: %v", tt.errorContains, err)
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				}
				
				if tt.checkRunning && !p.isRunning {
					t.Error("Expected Poltergeist to be running")
				}
				
				// Clean up
				p.Stop()
			}
		})
	}
}

// TestSafeGroupPanicRecovery tests that SafeGroup properly recovers from panics
func TestSafeGroupPanicRecovery(t *testing.T) {
	tests := []struct {
		name          string
		operations    []func() error
		expectError   bool
		errorContains string
	}{
		{
			name: "successful operations",
			operations: []func() error{
				func() error { return nil },
				func() error { return nil },
				func() error { return nil },
			},
			expectError: false,
		},
		{
			name: "one operation returns error",
			operations: []func() error{
				func() error { return nil },
				func() error { return errors.New("operation failed") },
				func() error { return nil },
			},
			expectError:   true,
			errorContains: "operation failed",
		},
		{
			name: "one operation panics",
			operations: []func() error{
				func() error { return nil },
				func() error { panic("test panic") },
				func() error { return nil },
			},
			expectError:   true,
			errorContains: "goroutine panic",
		},
		{
			name: "multiple operations panic",
			operations: []func() error{
				func() error { panic("panic 1") },
				func() error { panic("panic 2") },
				func() error { return nil },
			},
			expectError:   true,
			errorContains: "goroutine panic",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup
			ctx := context.Background()
			log := logger.CreateLoggerWithOutput("", "debug", nil)
			
			g, ctx := NewSafeGroup(ctx, log)
			g.SetLimit(2) // Limit concurrency

			// Execute
			for _, op := range tt.operations {
				op := op // Capture loop variable
				g.Go(func() error {
					return op()
				})
			}

			err := g.Wait()

			// Verify
			if tt.expectError {
				if err == nil {
					t.Error("Expected error but got none")
				} else if tt.errorContains != "" && !contains(err.Error(), tt.errorContains) {
					t.Errorf("Expected error containing '%s', got: %v", tt.errorContains, err)
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				}
			}
		})
	}
}

// TestDependencyFactory tests the dependency factory
func TestDependencyFactory(t *testing.T) {
	tests := []struct {
		name          string
		config        *types.PoltergeistConfig
		checkQueue    bool
		checkNotifier bool
	}{
		{
			name:          "basic dependencies without prioritization",
			config:        createTestConfig(),
			checkQueue:    false,
			checkNotifier: false,
		},
		{
			name: "dependencies with prioritization enabled",
			config: func() *types.PoltergeistConfig {
				cfg := createTestConfig()
				cfg.BuildScheduling = &types.BuildSchedulingConfig{
					Parallelization: 4,
					Prioritization: types.BuildPrioritization{
						Enabled: true,
					},
				}
				return cfg
			}(),
			checkQueue:    true,
			checkNotifier: false,
		},
		{
			name: "dependencies with notifications enabled",
			config: func() *types.PoltergeistConfig {
				cfg := createTestConfig()
				enabled := true
				cfg.Notifications = &types.NotificationConfig{
					Enabled: &enabled,
				}
				return cfg
			}(),
			checkQueue:    false,
			checkNotifier: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup
			log := logger.CreateLoggerWithOutput("", "debug", nil)
			factory := NewDependencyFactory("/test/project", log, tt.config)

			// Execute
			deps := factory.CreateDefaults()

			// Verify
			if deps.StateManager == nil {
				t.Error("StateManager should not be nil")
			}
			if deps.BuilderFactory == nil {
				t.Error("BuilderFactory should not be nil")
			}
			if deps.WatchmanClient == nil {
				t.Error("WatchmanClient should not be nil")
			}
			if deps.WatchmanConfigManager == nil {
				t.Error("WatchmanConfigManager should not be nil")
			}

			if tt.checkQueue && deps.BuildQueue == nil {
				t.Error("BuildQueue should not be nil when prioritization is enabled")
			}
			if !tt.checkQueue && deps.BuildQueue != nil {
				t.Error("BuildQueue should be nil when prioritization is disabled")
			}
		})
	}
}

// Helper functions

func createTestConfig() *types.PoltergeistConfig {
	return &types.PoltergeistConfig{
		Version:     "1.0",
		ProjectType: "test",
		Targets:     []types.RawTarget{},
	}
}

func createValidDependencies() interfaces.PoltergeistDependencies {
	return interfaces.PoltergeistDependencies{
		StateManager:          mocks.NewMockStateManager(),
		BuilderFactory:        mocks.NewMockBuilderFactory(),
		WatchmanClient:        mocks.NewMockWatchmanClient(),
		WatchmanConfigManager: &mockWatchmanConfigManager{},
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && s[:len(substr)] == substr || 
		   len(s) > len(substr) && contains(s[1:], substr)
}

// mockWatchmanConfigManager is a minimal mock for WatchmanConfigManager
type mockWatchmanConfigManager struct{}

func (m *mockWatchmanConfigManager) EnsureConfigUpToDate(config *types.PoltergeistConfig) error {
	return nil
}

func (m *mockWatchmanConfigManager) SuggestOptimizations() ([]string, error) {
	return nil, nil
}

func (m *mockWatchmanConfigManager) CreateExclusionExpressions(config *types.PoltergeistConfig) []interface{} {
	return nil
}

func (m *mockWatchmanConfigManager) NormalizeWatchPattern(pattern string) string {
	return pattern
}

func (m *mockWatchmanConfigManager) ValidateWatchPattern(pattern string) error {
	return nil
}
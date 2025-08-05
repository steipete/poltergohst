package state_test

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/poltergeist/poltergeist/pkg/state"
	"github.com/poltergeist/poltergeist/pkg/types"
)

// Mock target for testing
type mockTarget struct {
	name string
}

func (m *mockTarget) GetName() string                   { return m.name }
func (m *mockTarget) GetType() types.TargetType         { return types.TargetTypeExecutable }
func (m *mockTarget) IsEnabled() bool                   { return true }
func (m *mockTarget) GetBuildCommand() string           { return "build" }
func (m *mockTarget) GetWatchPaths() []string           { return []string{"*"} }
func (m *mockTarget) GetSettlingDelay() int             { return 100 }
func (m *mockTarget) GetEnvironment() map[string]string { return nil }
func (m *mockTarget) GetMaxRetries() int                { return 3 }
func (m *mockTarget) GetBackoffMultiplier() float64     { return 2.0 }
func (m *mockTarget) GetDebounceInterval() int          { return 100 }
func (m *mockTarget) GetIcon() string                   { return "" }

func TestStateManager_InitializeState(t *testing.T) {
	tmpDir := t.TempDir()
	sm := state.NewStateManager(tmpDir, nil)
	
	target := &mockTarget{name: "test"}
	
	s, err := sm.InitializeState(target)
	if err != nil {
		t.Fatalf("failed to initialize state: %v", err)
	}
	
	if s.TargetName != "test" {
		t.Errorf("expected target name 'test', got %s", s.TargetName)
	}
	
	if s.BuildStatus != types.BuildStatusIdle {
		t.Errorf("expected idle status, got %s", s.BuildStatus)
	}
	
	if s.ProcessID != os.Getpid() {
		t.Errorf("expected current PID, got %d", s.ProcessID)
	}
	
	// Check state file was created
	stateFile := filepath.Join(tmpDir, ".poltergeist", "state", "test.json")
	if _, err := os.Stat(stateFile); os.IsNotExist(err) {
		t.Error("state file was not created")
	}
}

func TestStateManager_ReadState(t *testing.T) {
	tmpDir := t.TempDir()
	sm := state.NewStateManager(tmpDir, nil)
	
	target := &mockTarget{name: "test"}
	
	// Initialize state
	_, err := sm.InitializeState(target)
	if err != nil {
		t.Fatalf("failed to initialize state: %v", err)
	}
	
	// Read state
	s, err := sm.ReadState("test")
	if err != nil {
		t.Fatalf("failed to read state: %v", err)
	}
	
	if s.TargetName != "test" {
		t.Errorf("expected target name 'test', got %s", s.TargetName)
	}
	
	// Try to read non-existent state
	_, err = sm.ReadState("nonexistent")
	if err == nil {
		t.Error("expected error reading non-existent state")
	}
}

func TestStateManager_UpdateState(t *testing.T) {
	tmpDir := t.TempDir()
	sm := state.NewStateManager(tmpDir, nil)
	
	target := &mockTarget{name: "test"}
	
	// Initialize state
	_, err := sm.InitializeState(target)
	if err != nil {
		t.Fatalf("failed to initialize state: %v", err)
	}
	
	// Update state
	updates := map[string]interface{}{
		"buildStatus":   types.BuildStatusBuilding,
		"lastBuildTime": time.Now(),
		"buildCount":    5,
		"lastError":     "test error",
		"customField":   "custom value",
	}
	
	err = sm.UpdateState("test", updates)
	if err != nil {
		t.Fatalf("failed to update state: %v", err)
	}
	
	// Read updated state
	s, err := sm.ReadState("test")
	if err != nil {
		t.Fatalf("failed to read updated state: %v", err)
	}
	
	if s.BuildStatus != types.BuildStatusBuilding {
		t.Errorf("expected building status, got %s", s.BuildStatus)
	}
	
	if s.BuildCount != 5 {
		t.Errorf("expected build count 5, got %d", s.BuildCount)
	}
	
	if s.LastError != "test error" {
		t.Errorf("expected error 'test error', got %s", s.LastError)
	}
	
	if s.Metadata["customField"] != "custom value" {
		t.Error("custom field not stored in metadata")
	}
}

func TestStateManager_UpdateBuildStatus(t *testing.T) {
	tmpDir := t.TempDir()
	sm := state.NewStateManager(tmpDir, nil)
	
	target := &mockTarget{name: "test"}
	
	// Initialize state
	_, err := sm.InitializeState(target)
	if err != nil {
		t.Fatalf("failed to initialize state: %v", err)
	}
	
	// Update build status to succeeded
	err = sm.UpdateBuildStatus("test", types.BuildStatusSucceeded)
	if err != nil {
		t.Fatalf("failed to update build status: %v", err)
	}
	
	s, _ := sm.ReadState("test")
	if s.BuildStatus != types.BuildStatusSucceeded {
		t.Errorf("expected succeeded status, got %s", s.BuildStatus)
	}
	
	if s.BuildCount != 1 {
		t.Errorf("expected build count 1, got %d", s.BuildCount)
	}
	
	// Update to failed
	err = sm.UpdateBuildStatus("test", types.BuildStatusFailed)
	if err != nil {
		t.Fatalf("failed to update build status: %v", err)
	}
	
	s, _ = sm.ReadState("test")
	if s.FailureCount != 1 {
		t.Errorf("expected failure count 1, got %d", s.FailureCount)
	}
}

func TestStateManager_RemoveState(t *testing.T) {
	tmpDir := t.TempDir()
	sm := state.NewStateManager(tmpDir, nil)
	
	target := &mockTarget{name: "test"}
	
	// Initialize state
	_, err := sm.InitializeState(target)
	if err != nil {
		t.Fatalf("failed to initialize state: %v", err)
	}
	
	// Remove state
	err = sm.RemoveState("test")
	if err != nil {
		t.Fatalf("failed to remove state: %v", err)
	}
	
	// Try to read removed state
	_, err = sm.ReadState("test")
	if err == nil {
		t.Error("expected error reading removed state")
	}
	
	// Check state file was removed
	stateFile := filepath.Join(tmpDir, ".poltergeist", "state", "test.json")
	if _, err := os.Stat(stateFile); !os.IsNotExist(err) {
		t.Error("state file was not removed")
	}
}

func TestStateManager_IsLocked(t *testing.T) {
	tmpDir := t.TempDir()
	sm := state.NewStateManager(tmpDir, nil)
	
	target := &mockTarget{name: "test"}
	
	// Initialize state
	_, err := sm.InitializeState(target)
	if err != nil {
		t.Fatalf("failed to initialize state: %v", err)
	}
	
	// Should not be locked by our own process
	locked, err := sm.IsLocked("test")
	if err != nil {
		t.Fatalf("failed to check lock: %v", err)
	}
	
	if locked {
		t.Error("state should not be locked by own process")
	}
	
	// Simulate another process's state (old heartbeat)
	stateFile := filepath.Join(tmpDir, ".poltergeist", "state", "test.json")
	oldState := &state.PoltergeistState{
		TargetName:  "test",
		ProcessID:   99999, // Non-existent PID
		Heartbeat:   time.Now().Add(-time.Hour), // Old heartbeat
	}
	
	data, _ := json.Marshal(oldState)
	os.WriteFile(stateFile, data, 0644)
	
	// Should not be locked (old heartbeat)
	locked, err = sm.IsLocked("test")
	if err != nil {
		t.Fatalf("failed to check lock: %v", err)
	}
	
	if locked {
		t.Error("state with old heartbeat should not be locked")
	}
}

func TestStateManager_DiscoverStates(t *testing.T) {
	tmpDir := t.TempDir()
	sm := state.NewStateManager(tmpDir, nil)
	
	// Initialize multiple states
	targets := []types.Target{
		&mockTarget{name: "target1"},
		&mockTarget{name: "target2"},
		&mockTarget{name: "target3"},
	}
	
	for _, target := range targets {
		_, err := sm.InitializeState(target)
		if err != nil {
			t.Fatalf("failed to initialize state for %s: %v", target.GetName(), err)
		}
	}
	
	// Discover states
	states, err := sm.DiscoverStates()
	if err != nil {
		t.Fatalf("failed to discover states: %v", err)
	}
	
	if len(states) != 3 {
		t.Errorf("expected 3 states, got %d", len(states))
	}
	
	// Check all targets are present
	for _, target := range targets {
		if _, ok := states[target.GetName()]; !ok {
			t.Errorf("state for %s not discovered", target.GetName())
		}
	}
}

func TestStateManager_Heartbeat(t *testing.T) {
	tmpDir := t.TempDir()
	sm := state.NewStateManager(tmpDir, nil)
	
	target := &mockTarget{name: "test"}
	
	// Initialize state
	initialState, err := sm.InitializeState(target)
	if err != nil {
		t.Fatalf("failed to initialize state: %v", err)
	}
	
	initialHeartbeat := initialState.Heartbeat
	
	// Start heartbeat
	ctx, cancel := context.WithCancel(context.Background())
	sm.StartHeartbeat(ctx)
	
	// Wait for heartbeat update
	time.Sleep(11 * time.Second) // Heartbeat interval is 10 seconds
	
	// Read updated state
	updatedState, err := sm.ReadState("test")
	if err != nil {
		t.Fatalf("failed to read state: %v", err)
	}
	
	if !updatedState.Heartbeat.After(initialHeartbeat) {
		t.Error("heartbeat was not updated")
	}
	
	// Stop heartbeat
	cancel()
	sm.StopHeartbeat()
}

func TestStateManager_Cleanup(t *testing.T) {
	tmpDir := t.TempDir()
	sm := state.NewStateManager(tmpDir, nil)
	
	// Initialize states
	targets := []types.Target{
		&mockTarget{name: "target1"},
		&mockTarget{name: "target2"},
	}
	
	for _, target := range targets {
		_, _ = sm.InitializeState(target)
		sm.UpdateBuildStatus(target.GetName(), types.BuildStatusBuilding)
	}
	
	// Cleanup
	err := sm.Cleanup()
	if err != nil {
		t.Fatalf("cleanup failed: %v", err)
	}
	
	// Check states are marked as idle
	for _, target := range targets {
		s, _ := sm.ReadState(target.GetName())
		if s.BuildStatus != types.BuildStatusIdle {
			t.Errorf("expected idle status after cleanup, got %s", s.BuildStatus)
		}
		if s.ProcessID != 0 {
			t.Error("expected ProcessID to be 0 after cleanup")
		}
	}
}

func TestStateManager_Concurrency(t *testing.T) {
	tmpDir := t.TempDir()
	sm := state.NewStateManager(tmpDir, nil)
	
	target := &mockTarget{name: "test"}
	sm.InitializeState(target)
	
	// Concurrent updates
	var wg sync.WaitGroup
	numGoroutines := 10
	
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			
			for j := 0; j < 10; j++ {
				updates := map[string]interface{}{
					"buildCount": id*10 + j,
				}
				sm.UpdateState("test", updates)
			}
		}(i)
	}
	
	wg.Wait()
	
	// Verify state is consistent
	s, err := sm.ReadState("test")
	if err != nil {
		t.Fatalf("failed to read state: %v", err)
	}
	
	if s.TargetName != "test" {
		t.Error("state corrupted during concurrent updates")
	}
}

func TestStateManager_AtomicWrites(t *testing.T) {
	tmpDir := t.TempDir()
	sm := state.NewStateManager(tmpDir, nil)
	
	target := &mockTarget{name: "test"}
	sm.InitializeState(target)
	
	// Simulate concurrent writes
	var wg sync.WaitGroup
	errors := make(chan error, 100)
	
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			
			status := types.BuildStatusBuilding
			if id%2 == 0 {
				status = types.BuildStatusSucceeded
			}
			err := sm.UpdateBuildStatus("test", status)
			if err != nil {
				errors <- err
			}
		}(i)
	}
	
	wg.Wait()
	close(errors)
	
	// Check for errors
	for err := range errors {
		t.Errorf("concurrent update error: %v", err)
	}
	
	// Verify final state is valid
	_, err := sm.ReadState("test")
	if err != nil {
		t.Fatalf("failed to read final state: %v", err)
	}
	
	// Check state file is valid JSON
	stateFile := filepath.Join(tmpDir, ".poltergeist", "state", "test.json")
	data, _ := os.ReadFile(stateFile)
	
	var parsedState state.PoltergeistState
	if err := json.Unmarshal(data, &parsedState); err != nil {
		t.Errorf("state file contains invalid JSON: %v", err)
	}
}

func BenchmarkStateManager_UpdateState(b *testing.B) {
	tmpDir := b.TempDir()
	sm := state.NewStateManager(tmpDir, nil)
	
	target := &mockTarget{name: "bench"}
	sm.InitializeState(target)
	
	updates := map[string]interface{}{
		"buildCount": 1,
		"lastError":  "test",
	}
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		sm.UpdateState("bench", updates)
	}
}

func BenchmarkStateManager_ReadState(b *testing.B) {
	tmpDir := b.TempDir()
	sm := state.NewStateManager(tmpDir, nil)
	
	target := &mockTarget{name: "bench"}
	sm.InitializeState(target)
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		sm.ReadState("bench")
	}
}
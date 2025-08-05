package daemon_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/poltergeist/poltergeist/pkg/daemon"
	"github.com/poltergeist/poltergeist/pkg/types"
)

func TestDaemon_StartStop(t *testing.T) {
	tmpDir := t.TempDir()
	
	// Create a test config file with a valid target
	configPath := filepath.Join(tmpDir, "poltergeist.config.json")
	
	// Create a test target with enabled file watching
	target := map[string]interface{}{
		"name":         "test-target",
		"type":         "executable",
		"buildCommand": "echo 'building'",
		"watchPaths":   []string{"*.go"},
		"outputPath":   "test-output",
		"enabled":      true, // Must be enabled for the daemon to start
	}
	targetJSON, _ := json.Marshal(target)
	
	config := &types.PoltergeistConfig{
		Version:     "1.0.0",
		ProjectType: types.ProjectTypeNode,
		Targets:     []json.RawMessage{targetJSON},
		Watchman: &types.WatchmanConfig{
			UseDefaultExclusions: true, // Use default exclusions
		},
	}
	data, _ := json.Marshal(config)
	os.WriteFile(configPath, data, 0644)
	
	daemonConfig := daemon.Config{
		ProjectRoot: tmpDir,
		ConfigPath:  configPath,
		LogFile:     filepath.Join(tmpDir, "daemon.log"),
		LogLevel:    "info",
	}
	
	d := daemon.NewManager(daemonConfig)
	
	// Start daemon
	err := d.Start()
	if err != nil {
		// Check if it's a watchman error - if so, skip the test
		if strings.Contains(err.Error(), "watchman") {
			t.Skip("Skipping test due to Watchman issues in test environment")
		}
		t.Fatalf("failed to start daemon: %v", err)
	}
	
	// Wait for daemon to start
	time.Sleep(500 * time.Millisecond)
	
	// Check if daemon is running
	if !d.IsRunning() {
		t.Error("expected daemon to be running")
	}
	
	// Check status
	status, err := d.Status()
	if err != nil {
		t.Fatalf("failed to get status: %v", err)
	}
	
	if status == nil {
		t.Error("expected non-nil status")
	}
	
	// Stop daemon
	err = d.Stop()
	if err != nil {
		t.Fatalf("failed to stop daemon: %v", err)
	}
	
	// Check if daemon stopped
	if d.IsRunning() {
		t.Error("expected daemon to be stopped")
	}
}

func TestDaemon_Restart(t *testing.T) {
	tmpDir := t.TempDir()
	
	// Create a test config file with a valid target
	configPath := filepath.Join(tmpDir, "poltergeist.config.json")
	
	// Create a test target
	target := map[string]interface{}{
		"name":         "test-target",
		"type":         "executable",
		"buildCommand": "echo 'building'",
		"watchPaths":   []string{"*.go"},
		"outputPath":   "test-output",
	}
	targetJSON, _ := json.Marshal(target)
	
	config := &types.PoltergeistConfig{
		Version:     "1.0.0",
		ProjectType: types.ProjectTypeNode,
		Targets:     []json.RawMessage{targetJSON},
		Watchman: &types.WatchmanConfig{
			UseDefaultExclusions: true, // Use default exclusions
		},
	}
	data, _ := json.Marshal(config)
	os.WriteFile(configPath, data, 0644)
	
	daemonConfig := daemon.Config{
		ProjectRoot: tmpDir,
		ConfigPath:  configPath,
		LogFile:     filepath.Join(tmpDir, "daemon.log"),
		LogLevel:    "info",
	}
	
	d := daemon.NewManager(daemonConfig)
	
	// Start daemon
	err := d.Start()
	if err != nil {
		// Check if it's a watchman error - if so, skip the test
		if strings.Contains(err.Error(), "watchman") {
			t.Skip("Skipping test due to Watchman issues in test environment")
		}
		t.Fatalf("failed to start daemon: %v", err)
	}
	
	time.Sleep(500 * time.Millisecond)
	
	// Get original status
	originalStatus, _ := d.Status()
	originalPID := 0
	if originalStatus != nil {
		originalPID = originalStatus.PID
	}
	
	// Restart daemon
	err = d.Restart()
	if err != nil {
		t.Fatalf("failed to restart daemon: %v", err)
	}
	
	time.Sleep(500 * time.Millisecond)
	
	// Check if daemon is running with new PID
	newStatus, _ := d.Status()
	if newStatus == nil {
		t.Error("expected daemon to be running after restart")
	} else if newStatus.PID == originalPID && originalPID != 0 {
		t.Error("expected daemon to have new PID after restart")
	}
	
	// Clean up
	d.Stop()
}

func TestDaemon_Status(t *testing.T) {
	tmpDir := t.TempDir()
	
	// Create a test config file with a valid target
	configPath := filepath.Join(tmpDir, "poltergeist.config.json")
	
	// Create a test target
	target := map[string]interface{}{
		"name":         "test-target",
		"type":         "executable",
		"buildCommand": "echo 'building'",
		"watchPaths":   []string{"*.go"},
		"outputPath":   "test-output",
	}
	targetJSON, _ := json.Marshal(target)
	
	config := &types.PoltergeistConfig{
		Version:     "1.0.0",
		ProjectType: types.ProjectTypeNode,
		Targets:     []json.RawMessage{targetJSON},
		Watchman: &types.WatchmanConfig{
			UseDefaultExclusions: true, // Use default exclusions
		},
	}
	data, _ := json.Marshal(config)
	os.WriteFile(configPath, data, 0644)
	
	daemonConfig := daemon.Config{
		ProjectRoot: tmpDir,
		ConfigPath:  configPath,
		LogFile:     filepath.Join(tmpDir, "daemon.log"),
		LogLevel:    "info",
	}
	
	d := daemon.NewManager(daemonConfig)
	
	// Status when not running
	status, err := d.Status()
	if err == nil && status != nil {
		t.Error("expected no status when daemon not running")
	}
	
	// Start daemon
	err = d.Start()
	if err != nil {
		// Check if it's a watchman error - if so, skip the test
		if strings.Contains(err.Error(), "watchman") {
			t.Skip("Skipping test due to Watchman issues in test environment")
		}
		t.Fatalf("failed to start daemon: %v", err)
	}
	
	time.Sleep(500 * time.Millisecond)
	
	// Status when running
	status, err = d.Status()
	if err != nil {
		t.Fatalf("failed to get status: %v", err)
	}
	
	if status == nil {
		t.Fatal("expected non-nil status")
	}
	
	if status.PID == 0 {
		t.Error("expected non-zero PID")
	}
	
	if !status.Running {
		t.Error("expected daemon to be running")
	}
	
	// Clean up
	d.Stop()
}

func TestDaemon_MultipleStart(t *testing.T) {
	tmpDir := t.TempDir()
	
	// Create a test config file with a valid target
	configPath := filepath.Join(tmpDir, "poltergeist.config.json")
	
	// Create a test target
	target := map[string]interface{}{
		"name":         "test-target",
		"type":         "executable",
		"buildCommand": "echo 'building'",
		"watchPaths":   []string{"*.go"},
		"outputPath":   "test-output",
	}
	targetJSON, _ := json.Marshal(target)
	
	config := &types.PoltergeistConfig{
		Version:     "1.0.0",
		ProjectType: types.ProjectTypeNode,
		Targets:     []json.RawMessage{targetJSON},
		Watchman: &types.WatchmanConfig{
			UseDefaultExclusions: true, // Use default exclusions
		},
	}
	data, _ := json.Marshal(config)
	os.WriteFile(configPath, data, 0644)
	
	daemonConfig := daemon.Config{
		ProjectRoot: tmpDir,
		ConfigPath:  configPath,
		LogFile:     filepath.Join(tmpDir, "daemon.log"),
		LogLevel:    "info",
	}
	
	d := daemon.NewManager(daemonConfig)
	
	// Start daemon
	err := d.Start()
	if err != nil {
		// Check if it's a watchman error - if so, skip the test
		if strings.Contains(err.Error(), "watchman") {
			t.Skip("Skipping test due to Watchman issues in test environment")
		}
		t.Fatalf("failed to start daemon: %v", err)
	}
	
	time.Sleep(500 * time.Millisecond)
	
	// Try to start again - should fail
	err = d.Start()
	if err == nil {
		t.Error("expected error when starting daemon twice")
	}
	
	// Clean up
	d.Stop()
}

func TestDaemon_StopNotRunning(t *testing.T) {
	tmpDir := t.TempDir()
	
	daemonConfig := daemon.Config{
		ProjectRoot: tmpDir,
		ConfigPath:  filepath.Join(tmpDir, "poltergeist.config.json"),
		LogFile:     filepath.Join(tmpDir, "daemon.log"),
		LogLevel:    "info",
	}
	
	d := daemon.NewManager(daemonConfig)
	
	// Try to stop when not running
	err := d.Stop()
	if err == nil {
		t.Error("expected error when stopping non-running daemon")
	}
}

func TestDaemon_InvalidConfig(t *testing.T) {
	tmpDir := t.TempDir()
	
	// Create invalid config file
	configPath := filepath.Join(tmpDir, "poltergeist.config.json")
	os.WriteFile(configPath, []byte("invalid json"), 0644)
	
	daemonConfig := daemon.Config{
		ProjectRoot: tmpDir,
		ConfigPath:  configPath,
		LogFile:     filepath.Join(tmpDir, "daemon.log"),
		LogLevel:    "info",
	}
	
	d := daemon.NewManager(daemonConfig)
	
	// Try to start with invalid config
	err := d.Start()
	if err == nil {
		t.Error("expected error when starting with invalid config")
	}
}
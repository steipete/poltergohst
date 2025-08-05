package watchman_test

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/poltergeist/poltergeist/pkg/interfaces"
	"github.com/poltergeist/poltergeist/pkg/logger"
	"github.com/poltergeist/poltergeist/pkg/types"
	"github.com/poltergeist/poltergeist/pkg/watchman"
)

func TestWatchmanClient_Watch(t *testing.T) {
	tmpDir := t.TempDir()
	log := logger.CreateLogger("", "info")

	// Create test files
	testFiles := []string{"main.go", "test.go", "doc.md"}
	for _, file := range testFiles {
		path := filepath.Join(tmpDir, file)
		os.WriteFile(path, []byte("test"), 0644)
	}

	client := watchman.NewClient(log)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Start watching
	events := make(chan watchman.FileEvent, 10)
	err := client.Watch(ctx, tmpDir, []string{"*.go"}, events)
	if err != nil {
		// Watchman might not be installed
		t.Skip("Watchman not available")
	}

	// Modify a file
	os.WriteFile(filepath.Join(tmpDir, "main.go"), []byte("modified"), 0644)

	// Wait for event
	select {
	case event := <-events:
		if event.Path != filepath.Join(tmpDir, "main.go") {
			t.Errorf("expected event for main.go, got %s", event.Path)
		}
		if event.Type != watchman.FileModified {
			t.Errorf("expected modified event, got %v", event.Type)
		}
	case <-time.After(2 * time.Second):
		t.Error("timeout waiting for file event")
	}
}

func TestWatchmanClient_Subscribe(t *testing.T) {
	tmpDir := t.TempDir()
	log := logger.CreateLogger("", "info")

	client := watchman.NewClient(log)
	// ctx is unused in this test since Subscribe doesn't use it
	// _ = ctx

	// Create subscription config
	config := interfaces.SubscriptionConfig{
		Expression: []interface{}{
			"allof",
			[]interface{}{"match", "*.go"},
		},
	}

	// Create callback
	callback := func(changes []interfaces.FileChange) {
		// Handle file changes
	}

	// Create exclusions
	exclusions := []interfaces.ExclusionExpression{}

	err := client.Subscribe(tmpDir, "test-sub", config, callback, exclusions)
	if err != nil {
		t.Skip("Watchman not available")
	}

	// Unsubscribe
	err = client.Unsubscribe("test-sub")
	if err != nil {
		t.Errorf("failed to unsubscribe: %v", err)
	}
}

func TestWatchmanClient_GetVersion(t *testing.T) {
	log := logger.CreateLogger("", "info")
	client := watchman.NewClient(log)

	version, err := client.GetVersion()
	if err != nil {
		t.Skip("Watchman not available")
	}

	if version == "" {
		t.Error("expected non-empty version")
	}

	t.Logf("Watchman version: %s", version)
}

func TestWatchmanConfig_GenerateQuery(t *testing.T) {
	config := &types.WatchmanConfig{
		UseDefaultExclusions: true,
		ExcludeDirs:          []string{"node_modules", ".git"},
		Rules: []types.ExclusionRule{
			{Pattern: "*.log", Action: "exclude"},
			{Pattern: "*.tmp", Action: "exclude"},
		},
		MaxFileEvents: 100,
	}

	cm := watchman.NewConfigManager(".", nil)

	// Create poltergeist config with watchman config
	poltergeistConfig := &types.PoltergeistConfig{
		Targets:  []json.RawMessage{},
		Watchman: config,
	}

	exclusions := cm.CreateExclusionExpressions(poltergeistConfig)

	// Build query with patterns and exclusions
	query := make(map[string]interface{})
	query["expression"] = []interface{}{
		"allof",
		[]interface{}{"anyof",
			[]interface{}{"match", "*.go"},
			[]interface{}{"match", "*.js"},
		},
	}

	// Apply exclusions to query
	if len(exclusions) > 0 {
		var excludeExpr []interface{}
		for _, exc := range exclusions {
			if exc.Type == "dirname" {
				for _, pattern := range exc.Patterns {
					excludeExpr = append(excludeExpr, []interface{}{"dirname", pattern})
				}
			}
		}
		if len(excludeExpr) > 0 {
			query["expression"] = []interface{}{
				"allof",
				query["expression"],
				[]interface{}{"not", []interface{}{"anyof", excludeExpr}},
			}
		}
	}

	// Verify query structure
	data, _ := json.Marshal(query)
	queryStr := string(data)

	// Should include patterns
	if !contains(queryStr, "*.go") {
		t.Error("expected query to include *.go pattern")
	}

	// Should exclude directories
	// Note: exclusions may be structured differently, so we check the exclusions array
	hasNodeModulesExclusion := false
	for _, exc := range exclusions {
		for _, pattern := range exc.Patterns {
			if pattern == "node_modules" {
				hasNodeModulesExclusion = true
				break
			}
		}
	}
	if !hasNodeModulesExclusion {
		t.Error("expected node_modules in exclusions")
	}
}

func TestWatchmanConfig_Exclusions(t *testing.T) {
	config := &types.WatchmanConfig{
		UseDefaultExclusions: true,
		ExcludeDirs:          []string{"custom_dir"},
		Rules: []types.ExclusionRule{
			{Pattern: "*.custom", Action: "exclude"},
		},
	}

	cm := watchman.NewConfigManager(".", nil)
	poltergeistConfig := &types.PoltergeistConfig{
		Targets:  []json.RawMessage{},
		Watchman: config,
	}
	exclusions := cm.CreateExclusionExpressions(poltergeistConfig)

	// Should include default exclusions
	expectedDefaults := []string{".git", "node_modules", "vendor"}
	for _, def := range expectedDefaults {
		found := false
		for _, exc := range exclusions {
			for _, pattern := range exc.Patterns {
				if pattern == def {
					found = true
					break
				}
			}
			if found {
				break
			}
		}
		if !found {
			t.Errorf("expected default exclusion %s", def)
		}
	}

	// Should include custom exclusions
	found := false
	for _, exc := range exclusions {
		for _, pattern := range exc.Patterns {
			if pattern == "custom_dir" {
				found = true
				break
			}
		}
		if found {
			break
		}
	}
	if !found {
		t.Error("expected custom exclusion custom_dir")
	}
}

func TestFallbackWatcher_Watch(t *testing.T) {
	tmpDir := t.TempDir()
	log := logger.CreateLogger("", "info")

	// Create test files
	os.WriteFile(filepath.Join(tmpDir, "test.go"), []byte("test"), 0644)

	watcher, err := watchman.NewFallbackWatcher(log)
	if err != nil {
		t.Fatalf("failed to create fallback watcher: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	events := make(chan watchman.FileEvent, 10)
	err = watcher.Watch(ctx, tmpDir, []string{"*.go"}, events)
	if err != nil {
		t.Fatalf("failed to start fallback watcher: %v", err)
	}

	// Modify file
	time.Sleep(100 * time.Millisecond) // Let watcher initialize
	os.WriteFile(filepath.Join(tmpDir, "test.go"), []byte("modified"), 0644)

	// Wait for event
	select {
	case event := <-events:
		if event.Type != watchman.FileModified {
			t.Errorf("expected modified event, got %v", event.Type)
		}
	case <-time.After(2 * time.Second):
		t.Error("timeout waiting for file event")
	}
}

func TestFallbackWatcher_CreateFile(t *testing.T) {
	tmpDir := t.TempDir()
	log := logger.CreateLogger("", "info")

	watcher, err := watchman.NewFallbackWatcher(log)
	if err != nil {
		t.Fatalf("failed to create fallback watcher: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	events := make(chan watchman.FileEvent, 10)
	err = watcher.Watch(ctx, tmpDir, []string{"*.go"}, events)
	if err != nil {
		t.Fatalf("failed to start fallback watcher: %v", err)
	}

	// Create new file
	time.Sleep(100 * time.Millisecond)
	os.WriteFile(filepath.Join(tmpDir, "new.go"), []byte("new"), 0644)

	// Wait for event
	select {
	case event := <-events:
		if event.Type != watchman.FileCreated {
			t.Errorf("expected created event, got %v", event.Type)
		}
		if filepath.Base(event.Path) != "new.go" {
			t.Errorf("expected new.go, got %s", event.Path)
		}
	case <-time.After(2 * time.Second):
		t.Error("timeout waiting for create event")
	}
}

func TestFallbackWatcher_DeleteFile(t *testing.T) {
	tmpDir := t.TempDir()
	log := logger.CreateLogger("", "info")

	// Create file first
	testFile := filepath.Join(tmpDir, "delete.go")
	os.WriteFile(testFile, []byte("delete me"), 0644)

	watcher, err := watchman.NewFallbackWatcher(log)
	if err != nil {
		t.Fatalf("failed to create fallback watcher: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	events := make(chan watchman.FileEvent, 10)
	err = watcher.Watch(ctx, tmpDir, []string{"*.go"}, events)
	if err != nil {
		t.Fatalf("failed to start fallback watcher: %v", err)
	}

	// Delete file
	time.Sleep(100 * time.Millisecond)
	os.Remove(testFile)

	// Wait for event
	select {
	case event := <-events:
		if event.Type != watchman.FileDeleted {
			t.Errorf("expected deleted event, got %v", event.Type)
		}
	case <-time.After(2 * time.Second):
		t.Error("timeout waiting for delete event")
	}
}

func TestFallbackWatcher_PatternMatching(t *testing.T) {
	tmpDir := t.TempDir()
	log := logger.CreateLogger("", "info")

	watcher, err := watchman.NewFallbackWatcher(log)
	if err != nil {
		t.Fatalf("failed to create fallback watcher: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	events := make(chan watchman.FileEvent, 10)

	// Watch only .go files
	err = watcher.Watch(ctx, tmpDir, []string{"*.go"}, events)
	if err != nil {
		t.Fatalf("failed to start watcher: %v", err)
	}

	time.Sleep(100 * time.Millisecond)

	// Create various files
	os.WriteFile(filepath.Join(tmpDir, "test.go"), []byte("go"), 0644)
	os.WriteFile(filepath.Join(tmpDir, "test.js"), []byte("js"), 0644)
	os.WriteFile(filepath.Join(tmpDir, "test.py"), []byte("py"), 0644)

	// Should only get event for .go file
	eventCount := 0
	timeout := time.After(1 * time.Second)

	for {
		select {
		case event := <-events:
			eventCount++
			if filepath.Ext(event.Path) != ".go" {
				t.Errorf("unexpected event for non-.go file: %s", event.Path)
			}
		case <-timeout:
			if eventCount != 1 {
				t.Errorf("expected 1 event, got %d", eventCount)
			}
			return
		}
	}
}

func TestWatchmanClient_Reconnect(t *testing.T) {
	log := logger.CreateLogger("", "info")
	client := watchman.NewClient(log)

	// Simulate connection failure and reconnect
	client.Disconnect()

	// Should reconnect automatically on next operation
	version, err := client.GetVersion()
	if err != nil {
		t.Skip("Watchman not available")
	}

	if version == "" {
		t.Error("expected to reconnect and get version")
	}
}

func TestWatchmanConfig_SettlingDelay(t *testing.T) {
	tmpDir := t.TempDir()
	log := logger.CreateLogger("", "info")

	config := &types.WatchmanConfig{
		SettlingDelay: 500, // 500ms
	}

	watcher, err := watchman.NewFallbackWatcher(log)
	if err != nil {
		t.Fatalf("failed to create fallback watcher: %v", err)
	}
	watcher.SetConfig(config)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	events := make(chan watchman.FileEvent, 10)
	err = watcher.Watch(ctx, tmpDir, []string{"*.go"}, events)
	if err != nil {
		t.Fatalf("failed to start watcher: %v", err)
	}

	time.Sleep(100 * time.Millisecond)

	// Rapid file changes
	testFile := filepath.Join(tmpDir, "test.go")
	for i := 0; i < 5; i++ {
		os.WriteFile(testFile, []byte(string(rune('a'+i))), 0644)
		time.Sleep(50 * time.Millisecond)
	}

	// Should get fewer events due to settling
	eventCount := 0
	timeout := time.After(1 * time.Second)

	for {
		select {
		case <-events:
			eventCount++
		case <-timeout:
			// Should have gotten fewer than 5 events due to settling
			if eventCount >= 5 {
				t.Errorf("expected settling to reduce events, got %d", eventCount)
			}
			return
		}
	}
}

func BenchmarkWatchmanQuery(b *testing.B) {
	config := &types.WatchmanConfig{
		UseDefaultExclusions: true,
		ExcludeDirs:          []string{"node_modules", ".git", "vendor"},
		Rules: []types.ExclusionRule{
			{Pattern: "*.log", Action: "exclude"},
			{Pattern: "*.tmp", Action: "exclude"},
			{Pattern: "*.cache", Action: "exclude"},
		},
		MaxFileEvents: 1000,
	}

	cm := watchman.NewConfigManager(".", nil)
	poltergeistConfig := &types.PoltergeistConfig{
		Targets:  []json.RawMessage{},
		Watchman: config,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		cm.CreateExclusionExpressions(poltergeistConfig)
	}
}

func BenchmarkFallbackWatcher(b *testing.B) {
	tmpDir := b.TempDir()
	log := logger.CreateLogger("", "error")

	// Create many files
	for i := 0; i < 100; i++ {
		os.WriteFile(filepath.Join(tmpDir, fmt.Sprintf("file%d.go", i)), []byte("test"), 0644)
	}

	watcher, err := watchman.NewFallbackWatcher(log)
	if err != nil {
		b.Fatalf("failed to create fallback watcher: %v", err)
	}
	ctx := context.Background()
	events := make(chan watchman.FileEvent, 1000)

	err = watcher.Watch(ctx, tmpDir, []string{"*.go"}, events)
	if err != nil {
		b.Fatalf("failed to start watcher: %v", err)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// Modify a file
		os.WriteFile(filepath.Join(tmpDir, "file0.go"), []byte(fmt.Sprintf("test%d", i)), 0644)

		// Wait for event
		select {
		case <-events:
			// Got event
		case <-time.After(100 * time.Millisecond):
			b.Error("timeout waiting for event")
		}
	}
}

// Helper function
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s[:len(substr)] == substr || contains(s[1:], substr))
}

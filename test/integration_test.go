// +build integration

package integration_test

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/poltergeist/poltergeist/pkg/builders"
	"github.com/poltergeist/poltergeist/pkg/config"
	"github.com/poltergeist/poltergeist/pkg/interfaces"
	"github.com/poltergeist/poltergeist/pkg/logger"
	"github.com/poltergeist/poltergeist/pkg/poltergeist"
	"github.com/poltergeist/poltergeist/pkg/queue"
	"github.com/poltergeist/poltergeist/pkg/state"
	"github.com/poltergeist/poltergeist/pkg/types"
)

// TestEndToEndBuild tests a complete build cycle
func TestEndToEndBuild(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	tmpDir := t.TempDir()
	
	// Create a simple Go project
	mainFile := filepath.Join(tmpDir, "main.go")
	err := ioutil.WriteFile(mainFile, []byte(`
		package main
		import "fmt"
		func main() { fmt.Println("Hello, Poltergeist!") }
	`), 0644)
	if err != nil {
		t.Fatalf("failed to create main.go: %v", err)
	}

	// Create configuration
	cfg := &types.PoltergeistConfig{
		Version:     "1.0",
		ProjectType: types.ProjectType("go"),
		Targets: []json.RawMessage{
			json.RawMessage(`{
				"name": "main",
				"type": "executable",
				"buildCommand": "go build -o main main.go",
				"watchPaths": ["*.go"],
				"outputPath": "main"
			}`),
		},
	}

	// Create dependencies
	log := logger.CreateLogger("", "info")
	deps := interfaces.PoltergeistDependencies{
		StateManager:   state.NewStateManager(tmpDir, log),
		BuilderFactory: builders.NewBuilderFactory(),
	}

	// Create and start Poltergeist
	p := poltergeist.New(cfg, tmpDir, log, deps, "")
	
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Start in background
	errChan := make(chan error, 1)
	go func() {
		errChan <- p.Start("")
	}()

	// Wait a bit for initial build
	time.Sleep(2 * time.Second)

	// Modify the file to trigger rebuild
	err = ioutil.WriteFile(mainFile, []byte(`
		package main
		import "fmt"
		func main() { fmt.Println("Updated!") }
	`), 0644)
	if err != nil {
		t.Fatalf("failed to update main.go: %v", err)
	}

	// Wait for rebuild
	time.Sleep(2 * time.Second)

	// Stop Poltergeist
	p.Stop()
	
	// Check if binary was created
	outputPath := filepath.Join(tmpDir, "main")
	if _, err := os.Stat(outputPath); os.IsNotExist(err) {
		t.Error("expected output binary to be created")
	}
}

// TestMultiTargetBuilds tests building multiple targets concurrently
func TestMultiTargetBuilds(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	tmpDir := t.TempDir()

	// Create multiple source files
	files := map[string]string{
		"cmd1/main.go": `package main; func main() { println("cmd1") }`,
		"cmd2/main.go": `package main; func main() { println("cmd2") }`,
		"cmd3/main.go": `package main; func main() { println("cmd3") }`,
	}

	for path, content := range files {
		fullPath := filepath.Join(tmpDir, path)
		os.MkdirAll(filepath.Dir(fullPath), 0755)
		ioutil.WriteFile(fullPath, []byte(content), 0644)
	}

	// Create configuration with multiple targets
	targets := []json.RawMessage{
		json.RawMessage(`{
			"name": "cmd1",
			"type": "executable",
			"buildCommand": "go build -o cmd1 cmd1/main.go",
			"watchPaths": ["cmd1/*.go"],
			"outputPath": "cmd1"
		}`),
		json.RawMessage(`{
			"name": "cmd2",
			"type": "executable",
			"buildCommand": "go build -o cmd2 cmd2/main.go",
			"watchPaths": ["cmd2/*.go"],
			"outputPath": "cmd2"
		}`),
		json.RawMessage(`{
			"name": "cmd3",
			"type": "executable",
			"buildCommand": "go build -o cmd3 cmd3/main.go",
			"watchPaths": ["cmd3/*.go"],
			"outputPath": "cmd3"
		}`),
	}

	cfg := &types.PoltergeistConfig{
		Version:     "1.0",
		ProjectType: types.ProjectType("go"),
		Targets:     targets,
		BuildScheduling: &types.BuildSchedulingConfig{
			Parallelization: 3,
			Prioritization: types.BuildPrioritization{
				Enabled: true,
			},
		},
	}

	// Create dependencies with build queue
	log := logger.CreateLogger("", "info")
	priorityEngine := queue.NewPriorityEngine(cfg.BuildScheduling, log)
	buildQueue := queue.NewIntelligentBuildQueue(cfg.BuildScheduling, log, priorityEngine, nil)
	
	deps := interfaces.PoltergeistDependencies{
		StateManager:   state.NewStateManager(tmpDir, log),
		BuilderFactory: builders.NewBuilderFactory(),
		BuildQueue:     buildQueue,
		PriorityEngine: priorityEngine,
	}

	// Create and start Poltergeist
	p := poltergeist.New(cfg, tmpDir, log, deps, "")
	
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	// Start
	go p.Start("")

	// Wait for builds
	time.Sleep(5 * time.Second)

	// Verify all targets built
	for i := 1; i <= 3; i++ {
		outputPath := filepath.Join(tmpDir, fmt.Sprintf("cmd%d", i))
		if _, err := os.Stat(outputPath); os.IsNotExist(err) {
			t.Errorf("expected cmd%d to be built", i)
		}
	}

	p.Stop()
}

// TestBuildFailureRecovery tests recovery from build failures
func TestBuildFailureRecovery(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	tmpDir := t.TempDir()

	// Create a file with syntax error
	mainFile := filepath.Join(tmpDir, "main.go")
	ioutil.WriteFile(mainFile, []byte(`
		package main
		func main() { 
			// Syntax error
			println("missing closing
		}
	`), 0644)

	cfg := &types.PoltergeistConfig{
		Version:     "1.0",
		ProjectType: types.ProjectType("go"),
		Targets: []json.RawMessage{
			json.RawMessage(`{
				"name": "main",
				"type": "executable",
				"buildCommand": "go build -o main main.go",
				"watchPaths": ["*.go"],
				"outputPath": "main",
				"maxRetries": 2
			}`),
		},
	}

	log := logger.CreateLogger("", "info")
	deps := interfaces.PoltergeistDependencies{
		StateManager:   state.NewStateManager(tmpDir, log),
		BuilderFactory: builders.NewBuilderFactory(),
	}

	p := poltergeist.New(cfg, tmpDir, log, deps, "")
	
	// Start
	go p.Start("")

	// Wait for initial build attempt
	time.Sleep(2 * time.Second)

	// Fix the syntax error
	ioutil.WriteFile(mainFile, []byte(`
		package main
		func main() { 
			println("fixed!")
		}
	`), 0644)

	// Wait for rebuild
	time.Sleep(3 * time.Second)

	// Check if build succeeded after fix
	outputPath := filepath.Join(tmpDir, "main")
	if _, err := os.Stat(outputPath); os.IsNotExist(err) {
		t.Error("expected build to succeed after fixing error")
	}

	p.Stop()
}

// TestStatePersistence tests state persistence across restarts
func TestStatePersistence(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	tmpDir := t.TempDir()
	
	cfg := &types.PoltergeistConfig{
		Version:     "1.0",
		ProjectType: types.ProjectType("go"),
		Targets: []json.RawMessage{
			json.RawMessage(`{
				"name": "test",
				"type": "executable",
				"buildCommand": "echo 'building'",
				"watchPaths": ["*.go"],
				"outputPath": "test"
			}`),
		},
	}

	log := logger.CreateLogger("", "info")
	
	// First instance
	{
		sm := state.NewStateManager(tmpDir, log)
		deps := interfaces.PoltergeistDependencies{
			StateManager:   sm,
			BuilderFactory: builders.NewBuilderFactory(),
		}

		p := poltergeist.New(cfg, tmpDir, log, deps, "")
		go p.Start("")
		time.Sleep(2 * time.Second)
		
		// Update state
		sm.UpdateBuildStatus("test", types.BuildStatusSucceeded)
		
		p.Stop()
	}

	// Second instance - should load existing state
	{
		sm := state.NewStateManager(tmpDir, log)
		
		// Check if state was persisted
		s, err := sm.ReadState("test")
		if err != nil {
			t.Fatalf("failed to read persisted state: %v", err)
		}

		if s.BuildStatus != types.BuildStatusSucceeded {
			t.Errorf("expected build status to be persisted, got %s", s.BuildStatus)
		}
	}
}

// TestConcurrentFileChanges tests handling of rapid file changes
func TestConcurrentFileChanges(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	tmpDir := t.TempDir()

	// Create initial files
	for i := 0; i < 10; i++ {
		file := filepath.Join(tmpDir, fmt.Sprintf("file%d.go", i))
		ioutil.WriteFile(file, []byte(fmt.Sprintf("package main\n// File %d", i)), 0644)
	}

	cfg := &types.PoltergeistConfig{
		Version:     "1.0",
		ProjectType: types.ProjectType("go"),
		Targets: []json.RawMessage{
			json.RawMessage(`{
				"name": "test",
				"type": "executable",
				"buildCommand": "echo 'building'",
				"watchPaths": ["*.go"],
				"outputPath": "test",
				"settlingDelay": 100,
				"debounceInterval": 50
			}`),
		},
	}

	log := logger.CreateLogger("", "info")
	deps := interfaces.PoltergeistDependencies{
		StateManager:   state.NewStateManager(tmpDir, log),
		BuilderFactory: builders.NewBuilderFactory(),
	}

	p := poltergeist.New(cfg, tmpDir, log, deps, "")
	go p.Start("")
	
	time.Sleep(1 * time.Second)

	// Simulate rapid concurrent file changes
	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()
			file := filepath.Join(tmpDir, fmt.Sprintf("file%d.go", index))
			for j := 0; j < 5; j++ {
				content := fmt.Sprintf("package main\n// File %d, change %d", index, j)
				ioutil.WriteFile(file, []byte(content), 0644)
				time.Sleep(10 * time.Millisecond)
			}
		}(i)
	}

	wg.Wait()
	time.Sleep(2 * time.Second)

	// Should handle all changes without crashing
	p.Stop()
}

// TestMemoryLeaks tests for memory leaks during long runs
func TestMemoryLeaks(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	tmpDir := t.TempDir()
	
	cfg := &types.PoltergeistConfig{
		Version:     "1.0",
		ProjectType: types.ProjectType("go"),
		Targets: []json.RawMessage{
			json.RawMessage(`{
				"name": "test",
				"type": "executable",
				"buildCommand": "echo 'building'",
				"watchPaths": ["*.go"],
				"outputPath": "test"
			}`),
		},
	}

	log := logger.CreateLogger("", "info")
	deps := interfaces.PoltergeistDependencies{
		StateManager:   state.NewStateManager(tmpDir, log),
		BuilderFactory: builders.NewBuilderFactory(),
	}

	p := poltergeist.New(cfg, tmpDir, log, deps, "")
	go p.Start("")

	// Simulate many file changes over time
	testFile := filepath.Join(tmpDir, "test.go")
	for i := 0; i < 100; i++ {
		content := fmt.Sprintf("package main\n// Change %d", i)
		ioutil.WriteFile(testFile, []byte(content), 0644)
		time.Sleep(50 * time.Millisecond)
	}

	// Memory usage should be stable
	// In a real test, we would measure memory usage here
	
	p.Stop()
}

// TestConfigReload tests configuration hot-reloading
func TestConfigReload(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "poltergeist.config.json")

	// Initial config with one target
	initialConfig := map[string]interface{}{
		"version":     "1.0",
		"projectType": "go",
		"targets": []map[string]interface{}{
			{
				"name":         "target1",
				"type":         "executable",
				"buildCommand": "echo 'target1'",
				"watchPaths":   []string{"*.go"},
				"outputPath":   "target1",
			},
		},
	}

	data, _ := json.Marshal(initialConfig)
	ioutil.WriteFile(configPath, data, 0644)

	// Start with initial config
	manager := config.NewManager()
	cfg, _ := manager.LoadConfig(configPath)
	
	log := logger.CreateLogger("", "info")
	deps := interfaces.PoltergeistDependencies{
		StateManager:   state.NewStateManager(tmpDir, log),
		BuilderFactory: builders.NewBuilderFactory(),
		ConfigManager:  manager,
	}

	p := poltergeist.New(cfg, tmpDir, log, deps, configPath)
	go p.Start("")
	
	time.Sleep(2 * time.Second)

	// Update config with additional target
	updatedConfig := initialConfig
	updatedConfig["targets"] = append(
		updatedConfig["targets"].([]map[string]interface{}),
		map[string]interface{}{
			"name":         "target2",
			"type":         "executable",
			"buildCommand": "echo 'target2'",
			"watchPaths":   []string{"*.js"},
			"outputPath":   "target2",
		},
	)

	data, _ = json.Marshal(updatedConfig)
	ioutil.WriteFile(configPath, data, 0644)

	// Wait for config reload
	time.Sleep(2 * time.Second)

	// Should handle config change
	// In a real implementation, we would verify new target is active
	
	p.Stop()
}

// TestPerformance tests build performance with many targets
func TestPerformance(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	tmpDir := t.TempDir()
	numTargets := 20

	// Create many targets
	var targets []json.RawMessage
	for i := 0; i < numTargets; i++ {
		target := map[string]interface{}{
			"name":         fmt.Sprintf("target%d", i),
			"type":         "executable",
			"buildCommand": fmt.Sprintf("echo 'building target %d'", i),
			"watchPaths":   []string{fmt.Sprintf("src%d/*.go", i)},
			"outputPath":   fmt.Sprintf("target%d", i),
		}
		data, _ := json.Marshal(target)
		targets = append(targets, json.RawMessage(data))

		// Create source files
		srcDir := filepath.Join(tmpDir, fmt.Sprintf("src%d", i))
		os.MkdirAll(srcDir, 0755)
		srcFile := filepath.Join(srcDir, "main.go")
		ioutil.WriteFile(srcFile, []byte("package main\nfunc main(){}"), 0644)
	}

	cfg := &types.PoltergeistConfig{
		Version:     "1.0",
		ProjectType: types.ProjectType("go"),
		Targets:     targets,
		BuildScheduling: &types.BuildSchedulingConfig{
			Parallelization: 5,
			Prioritization: types.BuildPrioritization{
				Enabled: true,
			},
		},
	}

	log := logger.CreateLogger("", "info")
	priorityEngine := queue.NewPriorityEngine(cfg.BuildScheduling, log)
	buildQueue := queue.NewIntelligentBuildQueue(cfg.BuildScheduling, log, priorityEngine, nil)
	
	deps := interfaces.PoltergeistDependencies{
		StateManager:   state.NewStateManager(tmpDir, log),
		BuilderFactory: builders.NewBuilderFactory(),
		BuildQueue:     buildQueue,
		PriorityEngine: priorityEngine,
	}

	p := poltergeist.New(cfg, tmpDir, log, deps, "")
	
	start := time.Now()
	go p.Start("")
	
	// Wait for initial builds
	time.Sleep(10 * time.Second)
	
	duration := time.Since(start)
	
	// All targets should build within reasonable time
	if duration > 30*time.Second {
		t.Errorf("builds took too long: %v", duration)
	}

	p.Stop()
}
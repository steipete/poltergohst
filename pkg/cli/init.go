package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/poltergeist/poltergeist/pkg/types"
	"github.com/spf13/cobra"
)

func newInitCmd() *cobra.Command {
	var projectType string
	var force bool
	
	cmd := &cobra.Command{
		Use:   "init",
		Short: "Initialize a new Poltergeist configuration",
		Long: `Initialize a new Poltergeist configuration file in the current directory.
This command will detect your project type and create a suitable configuration.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runInit(projectType, force)
		},
	}
	
	cmd.Flags().StringVarP(&projectType, "type", "t", "", "project type (swift, node, rust, python, cmake, mixed)")
	cmd.Flags().BoolVarP(&force, "force", "f", false, "overwrite existing configuration")
	
	return cmd
}

func runInit(projectType string, force bool) error {
	configPath := getConfigPath()
	
	// Check if config already exists
	if _, err := os.Stat(configPath); err == nil && !force {
		return fmt.Errorf("configuration already exists. Use --force to overwrite")
	}
	
	// Detect project type if not specified
	if projectType == "" {
		detected := detectProjectType()
		if detected != "" {
			projectType = detected
			printInfo(fmt.Sprintf("Detected project type: %s", projectType))
		} else {
			projectType = "mixed"
			printInfo("Could not detect project type, using 'mixed'")
		}
	}
	
	// Create configuration
	cfg := createDefaultConfig(projectType)
	
	// Write configuration
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}
	
	if err := os.WriteFile(configPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write config: %w", err)
	}
	
	printSuccess(fmt.Sprintf("Created configuration at %s", configPath))
	printInfo("Edit the configuration to customize your targets and build commands")
	
	return nil
}

func detectProjectType() string {
	// Check for various project files
	checks := map[string]string{
		"Package.swift":    "swift",
		"package.json":     "node",
		"Cargo.toml":       "rust",
		"pyproject.toml":   "python",
		"requirements.txt": "python",
		"CMakeLists.txt":   "cmake",
		"Makefile":         "mixed",
	}
	
	for file, projectType := range checks {
		if _, err := os.Stat(filepath.Join(projectRoot, file)); err == nil {
			return projectType
		}
	}
	
	return ""
}

func createDefaultConfig(projectType string) *types.PoltergeistConfig {
	cfg := &types.PoltergeistConfig{
		Version:     "1.0",
		ProjectType: types.ProjectType(projectType),
		Targets:     []json.RawMessage{},
	}
	
	// Add default targets based on project type
	switch projectType {
	case "swift":
		cfg.Targets = createSwiftTargets()
	case "node":
		cfg.Targets = createNodeTargets()
	case "rust":
		cfg.Targets = createRustTargets()
	case "python":
		cfg.Targets = createPythonTargets()
	case "cmake":
		cfg.Targets = createCMakeTargets()
	default:
		cfg.Targets = createMixedTargets()
	}
	
	// Add default watchman config
	cfg.Watchman = &types.WatchmanConfig{
		UseDefaultExclusions: true,
		ExcludeDirs: []string{
			"node_modules",
			".git",
			"build",
			"dist",
			"target",
			".next",
			".nuxt",
			".cache",
		},
		MaxFileEvents:    1000,
		RecrawlThreshold: 10000,
		SettlingDelay:    1000,
	}
	
	// Add default performance config
	cfg.Performance = &types.PerformanceConfig{
		Profile:      types.PerformanceProfileBalanced,
		AutoOptimize: true,
		Metrics: types.PerformanceMetrics{
			Enabled:        true,
			ReportInterval: 300,
		},
	}
	
	// Add default build scheduling
	cfg.BuildScheduling = &types.BuildSchedulingConfig{
		Parallelization: 2,
		Prioritization: types.BuildPrioritization{
			Enabled:                true,
			FocusDetectionWindow:   300000,
			PriorityDecayTime:      1800000,
			BuildTimeoutMultiplier: 2.0,
		},
	}
	
	// Add notifications
	enabled := true
	cfg.Notifications = &types.NotificationConfig{
		Enabled: &enabled,
	}
	
	return cfg
}

func createSwiftTargets() []json.RawMessage {
	targets := []interface{}{
		map[string]interface{}{
			"name":         "MyApp",
			"type":         "app-bundle",
			"buildCommand": "swift build",
			"watchPaths":   []string{"Sources/**/*.swift", "Package.swift"},
			"platform":     "macos",
			"bundleId":     "com.example.myapp",
		},
		map[string]interface{}{
			"name":         "Tests",
			"type":         "test",
			"testCommand":  "swift test",
			"watchPaths":   []string{"Tests/**/*.swift", "Sources/**/*.swift"},
		},
	}
	
	return marshalTargets(targets)
}

func createNodeTargets() []json.RawMessage {
	targets := []interface{}{
		map[string]interface{}{
			"name":         "build",
			"type":         "executable",
			"buildCommand": "npm run build",
			"watchPaths":   []string{"src/**/*", "package.json"},
			"outputPath":   "dist/index.js",
		},
		map[string]interface{}{
			"name":        "test",
			"type":        "test",
			"testCommand": "npm test",
			"watchPaths":  []string{"src/**/*", "test/**/*"},
		},
	}
	
	return marshalTargets(targets)
}

func createRustTargets() []json.RawMessage {
	targets := []interface{}{
		map[string]interface{}{
			"name":         "debug",
			"type":         "executable",
			"buildCommand": "cargo build",
			"watchPaths":   []string{"src/**/*.rs", "Cargo.toml"},
			"outputPath":   "target/debug/myapp",
		},
		map[string]interface{}{
			"name":         "release",
			"type":         "executable",
			"buildCommand": "cargo build --release",
			"watchPaths":   []string{"src/**/*.rs", "Cargo.toml"},
			"outputPath":   "target/release/myapp",
			"enabled":      false,
		},
		map[string]interface{}{
			"name":        "test",
			"type":        "test",
			"testCommand": "cargo test",
			"watchPaths":  []string{"src/**/*.rs", "tests/**/*.rs"},
		},
	}
	
	return marshalTargets(targets)
}

func createPythonTargets() []json.RawMessage {
	targets := []interface{}{
		map[string]interface{}{
			"name":        "test",
			"type":        "test",
			"testCommand": "pytest",
			"watchPaths":  []string{"**/*.py", "requirements.txt"},
		},
		map[string]interface{}{
			"name":         "lint",
			"type":         "custom",
			"buildCommand": "pylint src/",
			"watchPaths":   []string{"src/**/*.py"},
		},
	}
	
	return marshalTargets(targets)
}

func createCMakeTargets() []json.RawMessage {
	targets := []interface{}{
		map[string]interface{}{
			"name":       "main",
			"type":       "cmake-executable",
			"targetName": "main",
			"buildType":  "Debug",
			"watchPaths": []string{"src/**/*", "include/**/*", "CMakeLists.txt"},
		},
	}
	
	return marshalTargets(targets)
}

func createMixedTargets() []json.RawMessage {
	targets := []interface{}{
		map[string]interface{}{
			"name":         "build",
			"type":         "custom",
			"buildCommand": "make",
			"watchPaths":   []string{"src/**/*", "Makefile"},
		},
		map[string]interface{}{
			"name":         "test",
			"type":         "test",
			"testCommand":  "make test",
			"watchPaths":   []string{"src/**/*", "test/**/*"},
		},
	}
	
	return marshalTargets(targets)
}

func marshalTargets(targets []interface{}) []json.RawMessage {
	result := make([]json.RawMessage, len(targets))
	for i, target := range targets {
		data, _ := json.Marshal(target)
		result[i] = json.RawMessage(data)
	}
	return result
}
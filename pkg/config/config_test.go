package config_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/poltergeist/poltergeist/pkg/config"
	"github.com/poltergeist/poltergeist/pkg/types"
	"gopkg.in/yaml.v3"
)

func TestLoadConfig_JSON(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "poltergeist.config.json")
	
	// Create test config
	testConfig := map[string]interface{}{
		"version":     "1.0",
		"projectType": "mixed",
		"targets": []map[string]interface{}{
			{
				"name":         "test",
				"type":         "executable",
				"buildCommand": "go build",
				"watchPaths":   []string{"*.go"},
				"outputPath":   "test",
			},
		},
	}
	
	data, _ := json.Marshal(testConfig)
	os.WriteFile(configPath, data, 0644)
	
	// Load config
	manager := config.NewManager()
	cfg, err := manager.LoadConfig(configPath)
	if err != nil {
		t.Fatalf("failed to load config: %v", err)
	}
	
	if cfg.Version != "1.0" {
		t.Errorf("expected version 1.0, got %s", cfg.Version)
	}
	
	if cfg.ProjectType != types.ProjectTypeMixed {
		t.Errorf("expected project type mixed, got %s", cfg.ProjectType)
	}
	
	if len(cfg.Targets) != 1 {
		t.Errorf("expected 1 target, got %d", len(cfg.Targets))
	}
}

func TestLoadConfig_YAML(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "poltergeist.config.yaml")
	
	// Create test config
	testConfig := map[string]interface{}{
		"version":     "1.0",
		"projectType": "python",
		"targets": []map[string]interface{}{
			{
				"name":        "test",
				"type":        "test",
				"testCommand": "pytest",
				"watchPaths":  []string{"**/*.py"},
			},
		},
	}
	
	data, _ := yaml.Marshal(testConfig)
	os.WriteFile(configPath, data, 0644)
	
	// Load config
	manager := config.NewManager()
	cfg, err := manager.LoadConfig(configPath)
	if err != nil {
		t.Fatalf("failed to load YAML config: %v", err)
	}
	
	if cfg.ProjectType != types.ProjectType("python") {
		t.Errorf("expected project type python, got %s", cfg.ProjectType)
	}
}

func TestValidateConfig(t *testing.T) {
	manager := config.NewManager()
	
	tests := []struct {
		name    string
		config  *types.PoltergeistConfig
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid config",
			config: &types.PoltergeistConfig{
				Version:     "1.0",
				ProjectType: types.ProjectTypeMixed,
				Targets: []json.RawMessage{
					json.RawMessage(`{
						"name": "test",
						"type": "executable",
						"buildCommand": "go build",
						"watchPaths": ["*.go"],
						"outputPath": "test"
					}`),
				},
			},
			wantErr: false,
		},
		{
			name: "invalid version",
			config: &types.PoltergeistConfig{
				Version:     "2.0",
				ProjectType: types.ProjectTypeMixed,
				Targets:     []json.RawMessage{},
			},
			wantErr: true,
			errMsg:  "unsupported config version",
		},
		{
			name: "invalid project type",
			config: &types.PoltergeistConfig{
				Version:     "1.0",
				ProjectType: types.ProjectType("invalid"),
				Targets:     []json.RawMessage{},
			},
			wantErr: true,
			errMsg:  "invalid project type",
		},
		{
			name: "no targets",
			config: &types.PoltergeistConfig{
				Version:     "1.0",
				ProjectType: types.ProjectTypeMixed,
				Targets:     []json.RawMessage{},
			},
			wantErr: true,
			errMsg:  "no targets defined",
		},
		{
			name: "duplicate target names",
			config: &types.PoltergeistConfig{
				Version:     "1.0",
				ProjectType: types.ProjectTypeMixed,
				Targets: []json.RawMessage{
					json.RawMessage(`{"name": "test", "type": "executable", "buildCommand": "build", "watchPaths": ["*"], "outputPath": "out"}`),
					json.RawMessage(`{"name": "test", "type": "library", "buildCommand": "build", "watchPaths": ["*"], "outputPath": "out", "libraryType": "static"}`),
				},
			},
			wantErr: true,
			errMsg:  "duplicate target name",
		},
		{
			name: "target missing name",
			config: &types.PoltergeistConfig{
				Version:     "1.0",
				ProjectType: types.ProjectTypeMixed,
				Targets: []json.RawMessage{
					json.RawMessage(`{"type": "executable", "buildCommand": "build", "watchPaths": ["*"], "outputPath": "out"}`),
				},
			},
			wantErr: true,
			errMsg:  "missing name",
		},
		{
			name: "target missing build command",
			config: &types.PoltergeistConfig{
				Version:     "1.0",
				ProjectType: types.ProjectTypeMixed,
				Targets: []json.RawMessage{
					json.RawMessage(`{"name": "test", "type": "executable", "watchPaths": ["*"], "outputPath": "out"}`),
				},
			},
			wantErr: true,
			errMsg:  "missing build command",
		},
		{
			name: "target missing watch paths",
			config: &types.PoltergeistConfig{
				Version:     "1.0",
				ProjectType: types.ProjectTypeMixed,
				Targets: []json.RawMessage{
					json.RawMessage(`{"name": "test", "type": "executable", "buildCommand": "build", "watchPaths": [], "outputPath": "out"}`),
				},
			},
			wantErr: true,
			errMsg:  "no watch paths defined",
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := manager.ValidateConfig(tt.config)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateConfig() error = %v, wantErr %v", err, tt.wantErr)
			}
			
			if err != nil && tt.errMsg != "" {
				if !contains(err.Error(), tt.errMsg) {
					t.Errorf("expected error containing '%s', got '%s'", tt.errMsg, err.Error())
				}
			}
		})
	}
}

func TestGetDefaultConfig(t *testing.T) {
	manager := config.NewManager()
	
	projectTypes := []types.ProjectType{
		types.ProjectTypeSwift,
		types.ProjectTypeNode,
		types.ProjectTypeRust,
		types.ProjectTypePython,
		types.ProjectTypeCMake,
		types.ProjectTypeMixed,
	}
	
	for _, pt := range projectTypes {
		cfg := manager.GetDefaultConfig(pt)
		
		if cfg.Version != "1.0" {
			t.Errorf("expected version 1.0 for %s, got %s", pt, cfg.Version)
		}
		
		if cfg.ProjectType != pt {
			t.Errorf("expected project type %s, got %s", pt, cfg.ProjectType)
		}
		
		if cfg.Watchman == nil {
			t.Errorf("expected watchman config for %s", pt)
		}
		
		if cfg.Performance == nil {
			t.Errorf("expected performance config for %s", pt)
		}
		
		if cfg.BuildScheduling == nil {
			t.Errorf("expected build scheduling config for %s", pt)
		}
	}
}

func TestLoadConfig_InvalidFile(t *testing.T) {
	manager := config.NewManager()
	
	// Non-existent file
	_, err := manager.LoadConfig("/non/existent/file.json")
	if err == nil {
		t.Error("expected error for non-existent file")
	}
	
	// Invalid JSON
	tmpDir := t.TempDir()
	invalidPath := filepath.Join(tmpDir, "invalid.json")
	os.WriteFile(invalidPath, []byte("not json"), 0644)
	
	_, err = manager.LoadConfig(invalidPath)
	if err == nil {
		t.Error("expected error for invalid JSON")
	}
}

func TestLoadConfig_ComplexConfig(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "complex.json")
	
	// Create complex config with all features
	complexConfig := `{
		"version": "1.0",
		"projectType": "mixed",
		"targets": [
			{
				"name": "backend",
				"type": "executable",
				"buildCommand": "go build",
				"watchPaths": ["cmd/**/*.go", "pkg/**/*.go"],
				"outputPath": "bin/server",
				"environment": {
					"CGO_ENABLED": "0"
				},
				"settlingDelay": 500,
				"maxRetries": 5
			},
			{
				"name": "frontend",
				"type": "custom",
				"buildCommand": "npm run build",
				"watchPaths": ["src/**/*", "package.json"],
				"config": {
					"outputDir": "dist"
				}
			},
			{
				"name": "docker",
				"type": "docker",
				"buildCommand": "docker build",
				"watchPaths": ["Dockerfile"],
				"imageName": "myapp",
				"tags": ["latest", "dev"],
				"enabled": false
			}
		],
		"watchman": {
			"useDefaultExclusions": true,
			"excludeDirs": ["node_modules", "vendor"],
			"maxFileEvents": 500,
			"recrawlThreshold": 5000,
			"settlingDelay": 200
		},
		"performance": {
			"profile": "balanced",
			"autoOptimize": true,
			"metrics": {
				"enabled": true,
				"reportInterval": 60
			}
		},
		"buildScheduling": {
			"parallelization": 4,
			"prioritization": {
				"enabled": true,
				"focusDetectionWindow": 60000,
				"priorityDecayTime": 300000,
				"buildTimeoutMultiplier": 3.0
			}
		},
		"notifications": {
			"enabled": true,
			"successSound": "default",
			"failureSound": "alert"
		},
		"logging": {
			"file": "poltergeist.log",
			"level": "debug"
		}
	}`
	
	os.WriteFile(configPath, []byte(complexConfig), 0644)
	
	// Load and validate
	manager := config.NewManager()
	cfg, err := manager.LoadConfig(configPath)
	if err != nil {
		t.Fatalf("failed to load complex config: %v", err)
	}
	
	// Verify all sections loaded
	if len(cfg.Targets) != 3 {
		t.Errorf("expected 3 targets, got %d", len(cfg.Targets))
	}
	
	if cfg.Watchman == nil || cfg.Watchman.SettlingDelay != 200 {
		t.Error("watchman config not loaded correctly")
	}
	
	if cfg.Performance == nil || cfg.Performance.Profile != types.PerformanceProfileBalanced {
		t.Error("performance config not loaded correctly")
	}
	
	if cfg.BuildScheduling == nil || cfg.BuildScheduling.Parallelization != 4 {
		t.Error("build scheduling config not loaded correctly")
	}
	
	if cfg.Notifications == nil || cfg.Notifications.Enabled == nil || !*cfg.Notifications.Enabled {
		t.Error("notifications config not loaded correctly")
	}
	
	if cfg.Logging == nil || cfg.Logging.Level != types.LogLevelDebug {
		t.Error("logging config not loaded correctly")
	}
}

func TestDefaultExclusions(t *testing.T) {
	manager := config.NewManager()
	cfg := manager.GetDefaultConfig(types.ProjectTypeMixed)
	
	expectedExclusions := []string{
		"node_modules",
		".git",
		"build",
		"dist",
		"target",
	}
	
	for _, exclusion := range expectedExclusions {
		found := false
		for _, dir := range cfg.Watchman.ExcludeDirs {
			if dir == exclusion {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected default exclusion '%s' not found", exclusion)
		}
	}
}

// Helper function
func contains(s, substr string) bool {
	return len(s) >= len(substr) && s[:len(substr)] == substr || 
		   len(s) > len(substr) && contains(s[1:], substr)
}
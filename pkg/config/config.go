// Package config handles configuration loading and management
package config

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/poltergeist/poltergeist/pkg/types"
	"gopkg.in/yaml.v3"
)

// Manager handles configuration operations
type Manager struct {
	configPath string
}

// NewManager creates a new configuration manager
func NewManager() *Manager {
	return &Manager{}
}

// LoadConfig loads configuration from a file
func (m *Manager) LoadConfig(path string) (*types.PoltergeistConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var cfg types.PoltergeistConfig

	// Try JSON first
	if err := json.Unmarshal(data, &cfg); err == nil {
		return m.validateConfig(&cfg)
	}

	// Try YAML - need special handling for json.RawMessage fields
	var yamlData map[string]interface{}
	if err := yaml.Unmarshal(data, &yamlData); err == nil {
		// Convert YAML data to JSON, then unmarshal
		jsonData, err := json.Marshal(yamlData)
		if err == nil {
			if err := json.Unmarshal(jsonData, &cfg); err == nil {
				return m.validateConfig(&cfg)
			}
		}
	}

	return nil, fmt.Errorf("failed to parse config as JSON or YAML")
}

// ValidateConfig validates a configuration
func (m *Manager) ValidateConfig(config *types.PoltergeistConfig) error {
	// Check version
	if config.Version != "1.0" {
		return fmt.Errorf("unsupported config version: %s", config.Version)
	}

	// Check project type
	validProjectTypes := map[types.ProjectType]bool{
		types.ProjectTypeSwift:  true,
		types.ProjectTypeNode:   true,
		types.ProjectTypeRust:   true,
		types.ProjectTypePython: true,
		types.ProjectTypeCMake:  true,
		types.ProjectTypeMixed:  true,
	}

	if !validProjectTypes[config.ProjectType] {
		return fmt.Errorf("invalid project type: %s", config.ProjectType)
	}

	// Validate targets
	if len(config.Targets) == 0 {
		return fmt.Errorf("no targets defined")
	}

	targetNames := make(map[string]bool)
	for i, rawTarget := range config.Targets {
		target, err := types.ParseTarget(rawTarget)
		if err != nil {
			return fmt.Errorf("target %d: %w", i, err)
		}

		// Check for duplicate names
		if targetNames[target.GetName()] {
			return fmt.Errorf("duplicate target name: %s", target.GetName())
		}
		targetNames[target.GetName()] = true

		// Validate target
		if err := m.validateTarget(target); err != nil {
			return fmt.Errorf("target '%s': %w", target.GetName(), err)
		}
	}

	return nil
}

// WatchConfig watches configuration file for changes
func (m *Manager) WatchConfig(path string, callback func(*types.PoltergeistConfig)) error {
	// TODO: Implement file watching for config
	return fmt.Errorf("config watching not implemented")
}

// GetDefaultConfig returns a default configuration for a project type
func (m *Manager) GetDefaultConfig(projectType types.ProjectType) *types.PoltergeistConfig {
	enabled := true

	return &types.PoltergeistConfig{
		Version:     "1.0",
		ProjectType: projectType,
		Targets:     []json.RawMessage{},
		Watchman: &types.WatchmanConfig{
			UseDefaultExclusions: true,
			ExcludeDirs:          getDefaultExclusions(),
			MaxFileEvents:        1000,
			RecrawlThreshold:     10000,
			SettlingDelay:        1000,
		},
		Performance: &types.PerformanceConfig{
			Profile:      types.PerformanceProfileBalanced,
			AutoOptimize: true,
			Metrics: types.PerformanceMetrics{
				Enabled:        true,
				ReportInterval: 300,
			},
		},
		BuildScheduling: &types.BuildSchedulingConfig{
			Parallelization: 2,
			Prioritization: types.BuildPrioritization{
				Enabled:                true,
				FocusDetectionWindow:   300000,
				PriorityDecayTime:      1800000,
				BuildTimeoutMultiplier: 2.0,
			},
		},
		Notifications: &types.NotificationConfig{
			Enabled: &enabled,
		},
	}
}

// Private methods

func (m *Manager) validateConfig(cfg *types.PoltergeistConfig) (*types.PoltergeistConfig, error) {
	if err := m.ValidateConfig(cfg); err != nil {
		return nil, err
	}
	return cfg, nil
}

func (m *Manager) validateTarget(target types.Target) error {
	// Check name
	if target.GetName() == "" {
		return fmt.Errorf("missing name")
	}

	// Check build command
	if target.GetBuildCommand() == "" {
		// Some target types have alternative commands
		switch target.GetType() {
		case types.TargetTypeTest:
			// Test targets may use testCommand instead
		default:
			return fmt.Errorf("missing build command")
		}
	}

	// Check watch paths
	if len(target.GetWatchPaths()) == 0 {
		return fmt.Errorf("no watch paths defined")
	}

	return nil
}

func getDefaultExclusions() []string {
	return []string{
		"node_modules",
		".git",
		"build",
		"dist",
		"target",
		".next",
		".nuxt",
		".cache",
		"coverage",
		".vscode",
		".idea",
		"*.log",
		"tmp",
		"temp",
		"vendor",
		".terraform",
		"__pycache__",
		".pytest_cache",
		".mypy_cache",
		".tox",
		"*.egg-info",
	}
}

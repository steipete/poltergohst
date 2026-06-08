package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/poltergeist/poltergeist/pkg/types"
)

func TestRunInit_NewConfiguration(t *testing.T) {
	// Setup temp directory
	tempDir := t.TempDir()
	originalProjectRoot := projectRoot
	projectRoot = tempDir
	defer func() { projectRoot = originalProjectRoot }()

	configPath := filepath.Join(tempDir, "poltergeist.config.json")

	// Test initializing new configuration
	err := runInit("swift", false)
	if err != nil {
		t.Fatalf("runInit failed: %v", err)
	}

	// Verify config file was created
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		t.Fatal("Configuration file was not created")
	}

	// Verify config content
	data, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("Failed to read config file: %v", err)
	}

	var config types.PoltergeistConfig
	err = json.Unmarshal(data, &config)
	if err != nil {
		t.Fatalf("Failed to parse config: %v", err)
	}

	// Verify basic config structure
	if config.Version != "1.0" {
		t.Errorf("Expected version 1.0, got %s", config.Version)
	}

	if config.ProjectType != types.ProjectTypeSwift {
		t.Errorf("Expected project type swift, got %s", config.ProjectType)
	}

	if len(config.Targets) == 0 {
		t.Error("Expected targets to be created")
	}

	if config.Watchman == nil {
		t.Error("Expected watchman config to be created")
	}

	if config.Performance == nil {
		t.Error("Expected performance config to be created")
	}

	if config.BuildScheduling == nil {
		t.Error("Expected build scheduling config to be created")
	}
}

func TestRunInit_ExistingConfiguration(t *testing.T) {
	// Setup temp directory
	tempDir := t.TempDir()
	originalProjectRoot := projectRoot
	projectRoot = tempDir
	defer func() { projectRoot = originalProjectRoot }()

	configPath := filepath.Join(tempDir, "poltergeist.config.json")

	// Create existing config
	existingConfig := `{"version": "1.0", "projectType": "node"}`
	err := os.WriteFile(configPath, []byte(existingConfig), 0644)
	if err != nil {
		t.Fatalf("Failed to create existing config: %v", err)
	}

	// Test without force flag - should fail
	err = runInit("swift", false)
	if err == nil {
		t.Error("Expected error when config already exists without force flag")
	}

	// Test with force flag - should succeed
	err = runInit("swift", true)
	if err != nil {
		t.Fatalf("runInit with force failed: %v", err)
	}

	// Verify config was overwritten
	data, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("Failed to read config file: %v", err)
	}

	var config types.PoltergeistConfig
	err = json.Unmarshal(data, &config)
	if err != nil {
		t.Fatalf("Failed to parse config: %v", err)
	}

	if config.ProjectType != types.ProjectTypeSwift {
		t.Errorf("Expected project type swift after overwrite, got %s", config.ProjectType)
	}
}

func TestDetectProjectType_Swift(t *testing.T) {
	tempDir := t.TempDir()
	originalProjectRoot := projectRoot
	projectRoot = tempDir
	defer func() { projectRoot = originalProjectRoot }()

	// Create Package.swift file
	err := os.WriteFile(filepath.Join(tempDir, "Package.swift"), []byte("// swift package"), 0644)
	if err != nil {
		t.Fatalf("Failed to create Package.swift: %v", err)
	}

	detected := detectProjectType()
	if detected != "swift" {
		t.Errorf("Expected swift, got %s", detected)
	}
}

func TestDetectProjectType_Node(t *testing.T) {
	tempDir := t.TempDir()
	originalProjectRoot := projectRoot
	projectRoot = tempDir
	defer func() { projectRoot = originalProjectRoot }()

	// Create package.json file
	packageJSON := `{"name": "test-project", "version": "1.0.0"}`
	err := os.WriteFile(filepath.Join(tempDir, "package.json"), []byte(packageJSON), 0644)
	if err != nil {
		t.Fatalf("Failed to create package.json: %v", err)
	}

	detected := detectProjectType()
	if detected != "node" {
		t.Errorf("Expected node, got %s", detected)
	}
}

func TestDetectProjectType_Rust(t *testing.T) {
	tempDir := t.TempDir()
	originalProjectRoot := projectRoot
	projectRoot = tempDir
	defer func() { projectRoot = originalProjectRoot }()

	// Create Cargo.toml file
	cargoToml := `[package]
name = "test-project"
version = "0.1.0"`
	err := os.WriteFile(filepath.Join(tempDir, "Cargo.toml"), []byte(cargoToml), 0644)
	if err != nil {
		t.Fatalf("Failed to create Cargo.toml: %v", err)
	}

	detected := detectProjectType()
	if detected != "rust" {
		t.Errorf("Expected rust, got %s", detected)
	}
}

func TestDetectProjectType_Python(t *testing.T) {
	tempDir := t.TempDir()
	originalProjectRoot := projectRoot
	projectRoot = tempDir
	defer func() { projectRoot = originalProjectRoot }()

	// Test with pyproject.toml
	pyprojectToml := `[project]
name = "test-project"
version = "0.1.0"`
	err := os.WriteFile(filepath.Join(tempDir, "pyproject.toml"), []byte(pyprojectToml), 0644)
	if err != nil {
		t.Fatalf("Failed to create pyproject.toml: %v", err)
	}

	detected := detectProjectType()
	if detected != "python" {
		t.Errorf("Expected python, got %s", detected)
	}

	// Clean up and test with requirements.txt
	os.Remove(filepath.Join(tempDir, "pyproject.toml"))

	requirements := `requests==2.28.0
numpy==1.21.0`
	err = os.WriteFile(filepath.Join(tempDir, "requirements.txt"), []byte(requirements), 0644)
	if err != nil {
		t.Fatalf("Failed to create requirements.txt: %v", err)
	}

	detected = detectProjectType()
	if detected != "python" {
		t.Errorf("Expected python, got %s", detected)
	}
}

func TestDetectProjectType_CMake(t *testing.T) {
	tempDir := t.TempDir()
	originalProjectRoot := projectRoot
	projectRoot = tempDir
	defer func() { projectRoot = originalProjectRoot }()

	// Create CMakeLists.txt file
	cmakeLists := `cmake_minimum_required(VERSION 3.10)
project(TestProject)`
	err := os.WriteFile(filepath.Join(tempDir, "CMakeLists.txt"), []byte(cmakeLists), 0644)
	if err != nil {
		t.Fatalf("Failed to create CMakeLists.txt: %v", err)
	}

	detected := detectProjectType()
	if detected != "cmake" {
		t.Errorf("Expected cmake, got %s", detected)
	}
}

func TestDetectProjectType_Mixed(t *testing.T) {
	tempDir := t.TempDir()
	originalProjectRoot := projectRoot
	projectRoot = tempDir
	defer func() { projectRoot = originalProjectRoot }()

	// Create Makefile
	makefile := `all:
	echo "Building project"`
	err := os.WriteFile(filepath.Join(tempDir, "Makefile"), []byte(makefile), 0644)
	if err != nil {
		t.Fatalf("Failed to create Makefile: %v", err)
	}

	detected := detectProjectType()
	if detected != "mixed" {
		t.Errorf("Expected mixed, got %s", detected)
	}
}

func TestDetectProjectType_Unknown(t *testing.T) {
	tempDir := t.TempDir()
	originalProjectRoot := projectRoot
	projectRoot = tempDir
	defer func() { projectRoot = originalProjectRoot }()

	// Empty directory
	detected := detectProjectType()
	if detected != "" {
		t.Errorf("Expected empty string for unknown project, got %s", detected)
	}
}

func TestDetectProjectType_Priority(t *testing.T) {
	tempDir := t.TempDir()
	originalProjectRoot := projectRoot
	projectRoot = tempDir
	defer func() { projectRoot = originalProjectRoot }()

	// Create multiple project files - Swift should have priority
	files := map[string]string{
		"Package.swift": "// swift package",
		"package.json":  `{"name": "test"}`,
		"Cargo.toml":    "[package]\nname = \"test\"",
		"Makefile":      "all:\n\techo test",
	}

	for filename, content := range files {
		err := os.WriteFile(filepath.Join(tempDir, filename), []byte(content), 0644)
		if err != nil {
			t.Fatalf("Failed to create %s: %v", filename, err)
		}
	}

	detected := detectProjectType()
	if detected != "swift" {
		t.Errorf("Expected swift (first in check order), got %s", detected)
	}
}

func TestCreateDefaultConfig_Swift(t *testing.T) {
	config := createDefaultConfig("swift")

	if config.ProjectType != types.ProjectTypeSwift {
		t.Errorf("Expected swift project type, got %s", config.ProjectType)
	}

	if len(config.Targets) == 0 {
		t.Error("Expected targets to be created")
	}

	// Verify Swift-specific targets were created
	var targets []map[string]interface{}
	for _, rawTarget := range config.Targets {
		var target map[string]interface{}
		json.Unmarshal(rawTarget, &target)
		targets = append(targets, target)
	}

	// Should have MyApp and Tests targets
	foundApp := false
	foundTests := false
	for _, target := range targets {
		if name, ok := target["name"].(string); ok {
			if name == "MyApp" {
				foundApp = true
				if targetType := target["type"].(string); targetType != "app-bundle" {
					t.Errorf("Expected app-bundle type for MyApp, got %s", targetType)
				}
			}
			if name == "Tests" {
				foundTests = true
				if targetType := target["type"].(string); targetType != "test" {
					t.Errorf("Expected test type for Tests, got %s", targetType)
				}
			}
		}
	}

	if !foundApp {
		t.Error("MyApp target not found in Swift config")
	}
	if !foundTests {
		t.Error("Tests target not found in Swift config")
	}
}

func TestCreateDefaultConfig_Node(t *testing.T) {
	config := createDefaultConfig("node")

	if config.ProjectType != types.ProjectTypeNode {
		t.Errorf("Expected node project type, got %s", config.ProjectType)
	}

	// Verify Node-specific targets
	var targets []map[string]interface{}
	for _, rawTarget := range config.Targets {
		var target map[string]interface{}
		json.Unmarshal(rawTarget, &target)
		targets = append(targets, target)
	}

	foundBuild := false
	foundTest := false
	for _, target := range targets {
		if name, ok := target["name"].(string); ok {
			if name == "build" {
				foundBuild = true
				if buildCommand := target["buildCommand"].(string); buildCommand != "npm run build" {
					t.Errorf("Expected 'npm run build' command, got %s", buildCommand)
				}
			}
			if name == "test" {
				foundTest = true
				if testCommand := target["testCommand"].(string); testCommand != "npm test" {
					t.Errorf("Expected 'npm test' command, got %s", testCommand)
				}
			}
		}
	}

	if !foundBuild {
		t.Error("build target not found in Node config")
	}
	if !foundTest {
		t.Error("test target not found in Node config")
	}
}

func TestCreateDefaultConfig_Rust(t *testing.T) {
	config := createDefaultConfig("rust")

	if config.ProjectType != types.ProjectTypeRust {
		t.Errorf("Expected rust project type, got %s", config.ProjectType)
	}

	// Verify Rust-specific targets
	var targets []map[string]interface{}
	for _, rawTarget := range config.Targets {
		var target map[string]interface{}
		json.Unmarshal(rawTarget, &target)
		targets = append(targets, target)
	}

	foundDebug := false
	foundRelease := false
	foundTest := false
	for _, target := range targets {
		if name, ok := target["name"].(string); ok {
			switch name {
			case "debug":
				foundDebug = true
				if buildCommand := target["buildCommand"].(string); buildCommand != "cargo build" {
					t.Errorf("Expected 'cargo build' command for debug, got %s", buildCommand)
				}
			case "release":
				foundRelease = true
				if buildCommand := target["buildCommand"].(string); buildCommand != "cargo build --release" {
					t.Errorf("Expected 'cargo build --release' command for release, got %s", buildCommand)
				}
				// Release should be disabled by default
				if enabled, ok := target["enabled"].(bool); ok && enabled {
					t.Error("Release target should be disabled by default")
				}
			case "test":
				foundTest = true
				if testCommand := target["testCommand"].(string); testCommand != "cargo test" {
					t.Errorf("Expected 'cargo test' command, got %s", testCommand)
				}
			}
		}
	}

	if !foundDebug {
		t.Error("debug target not found in Rust config")
	}
	if !foundRelease {
		t.Error("release target not found in Rust config")
	}
	if !foundTest {
		t.Error("test target not found in Rust config")
	}
}

func TestCreateDefaultConfig_Python(t *testing.T) {
	config := createDefaultConfig("python")

	if config.ProjectType != types.ProjectTypePython {
		t.Errorf("Expected python project type, got %s", config.ProjectType)
	}

	// Verify Python-specific targets
	var targets []map[string]interface{}
	for _, rawTarget := range config.Targets {
		var target map[string]interface{}
		json.Unmarshal(rawTarget, &target)
		targets = append(targets, target)
	}

	foundTest := false
	foundLint := false
	for _, target := range targets {
		if name, ok := target["name"].(string); ok {
			if name == "test" {
				foundTest = true
				if testCommand := target["testCommand"].(string); testCommand != "pytest" {
					t.Errorf("Expected 'pytest' command, got %s", testCommand)
				}
			}
			if name == "lint" {
				foundLint = true
				if buildCommand := target["buildCommand"].(string); buildCommand != "pylint src/" {
					t.Errorf("Expected 'pylint src/' command, got %s", buildCommand)
				}
			}
		}
	}

	if !foundTest {
		t.Error("test target not found in Python config")
	}
	if !foundLint {
		t.Error("lint target not found in Python config")
	}
}

func TestCreateDefaultConfig_CMake(t *testing.T) {
	config := createDefaultConfig("cmake")

	if config.ProjectType != types.ProjectTypeCMake {
		t.Errorf("Expected cmake project type, got %s", config.ProjectType)
	}

	// Verify CMake-specific targets
	var targets []map[string]interface{}
	for _, rawTarget := range config.Targets {
		var target map[string]interface{}
		json.Unmarshal(rawTarget, &target)
		targets = append(targets, target)
	}

	if len(targets) == 0 {
		t.Fatal("No targets found in CMake config")
	}

	target := targets[0]
	if name := target["name"].(string); name != "main" {
		t.Errorf("Expected 'main' target name, got %s", name)
	}

	if targetType := target["type"].(string); targetType != "cmake-executable" {
		t.Errorf("Expected 'cmake-executable' type, got %s", targetType)
	}

	if targetName := target["targetName"].(string); targetName != "main" {
		t.Errorf("Expected 'main' targetName, got %s", targetName)
	}

	if buildType := target["buildType"].(string); buildType != "Debug" {
		t.Errorf("Expected 'Debug' buildType, got %s", buildType)
	}
}

func TestCreateDefaultConfig_Mixed(t *testing.T) {
	config := createDefaultConfig("mixed")

	if config.ProjectType != types.ProjectTypeMixed {
		t.Errorf("Expected mixed project type, got %s", config.ProjectType)
	}

	// Verify Mixed-specific targets
	var targets []map[string]interface{}
	for _, rawTarget := range config.Targets {
		var target map[string]interface{}
		json.Unmarshal(rawTarget, &target)
		targets = append(targets, target)
	}

	foundBuild := false
	foundTest := false
	for _, target := range targets {
		if name, ok := target["name"].(string); ok {
			if name == "build" {
				foundBuild = true
				if buildCommand := target["buildCommand"].(string); buildCommand != "make" {
					t.Errorf("Expected 'make' command, got %s", buildCommand)
				}
			}
			if name == "test" {
				foundTest = true
				if testCommand := target["testCommand"].(string); testCommand != "make test" {
					t.Errorf("Expected 'make test' command, got %s", testCommand)
				}
			}
		}
	}

	if !foundBuild {
		t.Error("build target not found in Mixed config")
	}
	if !foundTest {
		t.Error("test target not found in Mixed config")
	}
}

func TestCreateDefaultConfig_CommonStructure(t *testing.T) {
	config := createDefaultConfig("swift")

	// Test common config structure that should be present regardless of project type
	if config.Version != "1.0" {
		t.Errorf("Expected version 1.0, got %s", config.Version)
	}

	// Watchman config
	if config.Watchman == nil {
		t.Fatal("Watchman config should not be nil")
	}

	if !config.Watchman.UseDefaultExclusions {
		t.Error("UseDefaultExclusions should be true")
	}

	expectedExcludes := []string{"node_modules", ".git", "build", "dist", "target", ".next", ".nuxt", ".cache"}
	for _, expected := range expectedExcludes {
		found := false
		for _, actual := range config.Watchman.ExcludeDirs {
			if actual == expected {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Expected exclude dir '%s' not found", expected)
		}
	}

	if config.Watchman.MaxFileEvents != 1000 {
		t.Errorf("Expected MaxFileEvents 1000, got %d", config.Watchman.MaxFileEvents)
	}

	// Performance config
	if config.Performance == nil {
		t.Fatal("Performance config should not be nil")
	}

	if config.Performance.Profile != types.PerformanceProfileBalanced {
		t.Errorf("Expected balanced performance profile, got %s", config.Performance.Profile)
	}

	if !config.Performance.AutoOptimize {
		t.Error("AutoOptimize should be true")
	}

	if !config.Performance.Metrics.Enabled {
		t.Error("Performance metrics should be enabled")
	}

	// Build scheduling config
	if config.BuildScheduling == nil {
		t.Fatal("Build scheduling config should not be nil")
	}

	if config.BuildScheduling.Parallelization != 2 {
		t.Errorf("Expected parallelization 2, got %d", config.BuildScheduling.Parallelization)
	}

	if !config.BuildScheduling.Prioritization.Enabled {
		t.Error("Prioritization should be enabled")
	}

	// Notifications config
	if config.Notifications == nil {
		t.Fatal("Notifications config should not be nil")
	}

	if config.Notifications.Enabled == nil || !*config.Notifications.Enabled {
		t.Error("Notifications should be enabled")
	}
}

func TestMarshalTargets(t *testing.T) {
	targets := []interface{}{
		map[string]interface{}{
			"name": "test-target",
			"type": "executable",
		},
		map[string]interface{}{
			"name":    "another-target",
			"type":    "test",
			"enabled": false,
		},
	}

	result := marshalTargets(targets)

	if len(result) != 2 {
		t.Errorf("Expected 2 marshaled targets, got %d", len(result))
	}

	// Verify first target
	var target1 map[string]interface{}
	err := json.Unmarshal(result[0], &target1)
	if err != nil {
		t.Fatalf("Failed to unmarshal first target: %v", err)
	}

	if target1["name"] != "test-target" {
		t.Errorf("Expected name 'test-target', got %s", target1["name"])
	}

	if target1["type"] != "executable" {
		t.Errorf("Expected type 'executable', got %s", target1["type"])
	}

	// Verify second target
	var target2 map[string]interface{}
	err = json.Unmarshal(result[1], &target2)
	if err != nil {
		t.Fatalf("Failed to unmarshal second target: %v", err)
	}

	if target2["name"] != "another-target" {
		t.Errorf("Expected name 'another-target', got %s", target2["name"])
	}

	if target2["enabled"] != false {
		t.Errorf("Expected enabled false, got %v", target2["enabled"])
	}
}

// Test edge cases and error conditions
func TestRunInit_InvalidProjectType(t *testing.T) {
	tempDir := t.TempDir()
	originalProjectRoot := projectRoot
	projectRoot = tempDir
	defer func() { projectRoot = originalProjectRoot }()

	// Test with invalid project type - should default to mixed
	err := runInit("invalid-type", false)
	if err != nil {
		t.Fatalf("runInit failed with invalid type: %v", err)
	}

	// Verify config was created with mixed type
	configPath := filepath.Join(tempDir, "poltergeist.config.json")
	data, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("Failed to read config file: %v", err)
	}

	var config types.PoltergeistConfig
	err = json.Unmarshal(data, &config)
	if err != nil {
		t.Fatalf("Failed to parse config: %v", err)
	}

	// Should have created mixed targets since invalid type is treated as mixed
	if len(config.Targets) == 0 {
		t.Error("Expected targets to be created even with invalid type")
	}
}

func TestRunInit_ReadOnlyDirectory(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("Skipping read-only test when running as root")
	}

	tempDir := t.TempDir()
	originalProjectRoot := projectRoot
	projectRoot = tempDir
	defer func() { projectRoot = originalProjectRoot }()

	// Make directory read-only
	err := os.Chmod(tempDir, 0444)
	if err != nil {
		t.Fatalf("Failed to make directory read-only: %v", err)
	}
	defer os.Chmod(tempDir, 0755) // Restore permissions for cleanup

	// Should fail to write config
	err = runInit("swift", false)
	if err == nil {
		t.Error("Expected error when trying to write to read-only directory")
	}
}

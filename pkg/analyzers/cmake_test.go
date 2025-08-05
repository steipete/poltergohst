package analyzers_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/poltergeist/poltergeist/pkg/analyzers"
	"github.com/poltergeist/poltergeist/pkg/types"
)

func TestCMakeAnalyzer_AnalyzeProject_BasicProject(t *testing.T) {
	tempDir := t.TempDir()
	analyzer := analyzers.NewCMakeAnalyzer(tempDir)

	// Create basic CMakeLists.txt
	cmakeContent := `cmake_minimum_required(VERSION 3.10)
project(TestProject VERSION 1.0.0)

add_executable(myapp main.cpp)
add_library(mylib STATIC lib.cpp)`

	err := os.WriteFile(filepath.Join(tempDir, "CMakeLists.txt"), []byte(cmakeContent), 0644)
	if err != nil {
		t.Fatalf("Failed to create CMakeLists.txt: %v", err)
	}

	project, err := analyzer.AnalyzeProject(nil)
	if err != nil {
		t.Fatalf("Failed to analyze project: %v", err)
	}

	if project.Name != "TestProject" {
		t.Errorf("Expected project name 'TestProject', got '%s'", project.Name)
	}

	if project.Version != "1.0.0" {
		t.Errorf("Expected project version '1.0.0', got '%s'", project.Version)
	}

	if len(project.Targets) != 2 {
		t.Errorf("Expected 2 targets, got %d", len(project.Targets))
	}

	// Check targets
	foundExecutable := false
	foundLibrary := false

	for _, target := range project.Targets {
		switch target.Name {
		case "myapp":
			foundExecutable = true
			if target.Type != "EXECUTABLE" {
				t.Errorf("Expected myapp to be EXECUTABLE, got %s", target.Type)
			}
		case "mylib":
			foundLibrary = true
			if target.Type != "STATIC_LIBRARY" {
				t.Errorf("Expected mylib to be STATIC_LIBRARY, got %s", target.Type)
			}
		}
	}

	if !foundExecutable {
		t.Error("Expected to find executable target 'myapp'")
	}
	if !foundLibrary {
		t.Error("Expected to find library target 'mylib'")
	}
}

func TestCMakeAnalyzer_AnalyzeProject_WithTests(t *testing.T) {
	tempDir := t.TempDir()
	analyzer := analyzers.NewCMakeAnalyzer(tempDir)

	// Create CMakeLists.txt with tests
	cmakeContent := `cmake_minimum_required(VERSION 3.10)
project(TestProject)

add_executable(myapp main.cpp)

enable_testing()
add_test(NAME unit_tests COMMAND ./test_runner)
add_test(NAME integration_tests COMMAND ./integration_runner)`

	err := os.WriteFile(filepath.Join(tempDir, "CMakeLists.txt"), []byte(cmakeContent), 0644)
	if err != nil {
		t.Fatalf("Failed to create CMakeLists.txt: %v", err)
	}

	options := &analyzers.AnalysisOptions{
		IncludeTests: true,
	}

	project, err := analyzer.AnalyzeProject(options)
	if err != nil {
		t.Fatalf("Failed to analyze project with tests: %v", err)
	}

	// Should find executable + 2 tests
	if len(project.Targets) != 3 {
		t.Errorf("Expected 3 targets (1 executable + 2 tests), got %d", len(project.Targets))
	}

	testCount := 0
	for _, target := range project.Targets {
		if target.Type == "TEST" {
			testCount++
		}
	}

	if testCount != 2 {
		t.Errorf("Expected 2 test targets, got %d", testCount)
	}
}

func TestCMakeAnalyzer_AnalyzeProject_NoTests(t *testing.T) {
	tempDir := t.TempDir()
	analyzer := analyzers.NewCMakeAnalyzer(tempDir)

	// Create CMakeLists.txt with tests
	cmakeContent := `cmake_minimum_required(VERSION 3.10)
project(TestProject)

add_executable(myapp main.cpp)

enable_testing()
add_test(NAME unit_tests COMMAND ./test_runner)`

	err := os.WriteFile(filepath.Join(tempDir, "CMakeLists.txt"), []byte(cmakeContent), 0644)
	if err != nil {
		t.Fatalf("Failed to create CMakeLists.txt: %v", err)
	}

	options := &analyzers.AnalysisOptions{
		IncludeTests: false,
	}

	project, err := analyzer.AnalyzeProject(options)
	if err != nil {
		t.Fatalf("Failed to analyze project without tests: %v", err)
	}

	// Should only find executable (no tests)
	if len(project.Targets) != 1 {
		t.Errorf("Expected 1 target (executable only), got %d", len(project.Targets))
	}

	if project.Targets[0].Type != "EXECUTABLE" {
		t.Errorf("Expected EXECUTABLE target, got %s", project.Targets[0].Type)
	}
}

func TestCMakeAnalyzer_AnalyzeProject_RecursiveSearch(t *testing.T) {
	tempDir := t.TempDir()
	analyzer := analyzers.NewCMakeAnalyzer(tempDir)

	// Create subdirectory structure
	subDir := filepath.Join(tempDir, "subproject")
	err := os.MkdirAll(subDir, 0755)
	if err != nil {
		t.Fatalf("Failed to create subdirectory: %v", err)
	}

	// Main CMakeLists.txt
	mainCMake := `cmake_minimum_required(VERSION 3.10)
project(MainProject)
add_subdirectory(subproject)
add_executable(main main.cpp)`

	err = os.WriteFile(filepath.Join(tempDir, "CMakeLists.txt"), []byte(mainCMake), 0644)
	if err != nil {
		t.Fatalf("Failed to create main CMakeLists.txt: %v", err)
	}

	// Sub CMakeLists.txt
	subCMake := `add_library(sublib STATIC sublib.cpp)`

	err = os.WriteFile(filepath.Join(subDir, "CMakeLists.txt"), []byte(subCMake), 0644)
	if err != nil {
		t.Fatalf("Failed to create sub CMakeLists.txt: %v", err)
	}

	options := &analyzers.AnalysisOptions{
		RecursiveSearch: true,
	}

	project, err := analyzer.AnalyzeProject(options)
	if err != nil {
		t.Fatalf("Failed to analyze project recursively: %v", err)
	}

	// Should find both main and sub targets
	if len(project.Targets) != 2 {
		t.Errorf("Expected 2 targets (main + sub), got %d", len(project.Targets))
	}

	foundMain := false
	foundSub := false

	for _, target := range project.Targets {
		switch target.Name {
		case "main":
			foundMain = true
		case "sublib":
			foundSub = true
		}
	}

	if !foundMain {
		t.Error("Expected to find main target")
	}
	if !foundSub {
		t.Error("Expected to find sub target")
	}
}

func TestCMakeAnalyzer_AnalyzeProject_NonRecursive(t *testing.T) {
	tempDir := t.TempDir()
	analyzer := analyzers.NewCMakeAnalyzer(tempDir)

	// Create subdirectory structure (same as above)
	subDir := filepath.Join(tempDir, "subproject")
	err := os.MkdirAll(subDir, 0755)
	if err != nil {
		t.Fatalf("Failed to create subdirectory: %v", err)
	}

	// Main CMakeLists.txt
	mainCMake := `cmake_minimum_required(VERSION 3.10)
project(MainProject)
add_executable(main main.cpp)`

	err = os.WriteFile(filepath.Join(tempDir, "CMakeLists.txt"), []byte(mainCMake), 0644)
	if err != nil {
		t.Fatalf("Failed to create main CMakeLists.txt: %v", err)
	}

	// Sub CMakeLists.txt
	subCMake := `add_library(sublib STATIC sublib.cpp)`

	err = os.WriteFile(filepath.Join(subDir, "CMakeLists.txt"), []byte(subCMake), 0644)
	if err != nil {
		t.Fatalf("Failed to create sub CMakeLists.txt: %v", err)
	}

	options := &analyzers.AnalysisOptions{
		RecursiveSearch: false,
	}

	project, err := analyzer.AnalyzeProject(options)
	if err != nil {
		t.Fatalf("Failed to analyze project non-recursively: %v", err)
	}

	// Should only find main target
	if len(project.Targets) != 1 {
		t.Errorf("Expected 1 target (main only), got %d", len(project.Targets))
	}

	if project.Targets[0].Name != "main" {
		t.Errorf("Expected main target, got %s", project.Targets[0].Name)
	}
}

func TestCMakeAnalyzer_AnalyzeProject_NoCMakeFiles(t *testing.T) {
	tempDir := t.TempDir()
	analyzer := analyzers.NewCMakeAnalyzer(tempDir)

	// Empty directory - no CMakeLists.txt
	_, err := analyzer.AnalyzeProject(nil)
	if err == nil {
		t.Error("Expected error when no CMakeLists.txt found")
	}

	if !strings.Contains(err.Error(), "no CMakeLists.txt files found") {
		t.Errorf("Expected 'no CMakeLists.txt files found' error, got: %v", err)
	}
}

func TestCMakeAnalyzer_FindTargets(t *testing.T) {
	tempDir := t.TempDir()
	analyzer := analyzers.NewCMakeAnalyzer(tempDir)

	// Create CMakeLists.txt
	cmakeContent := `cmake_minimum_required(VERSION 3.10)
project(TestProject)

add_executable(app1 main1.cpp)
add_executable(app2 main2.cpp)
add_library(lib1 SHARED lib1.cpp)`

	err := os.WriteFile(filepath.Join(tempDir, "CMakeLists.txt"), []byte(cmakeContent), 0644)
	if err != nil {
		t.Fatalf("Failed to create CMakeLists.txt: %v", err)
	}

	targets, err := analyzer.FindTargets(nil)
	if err != nil {
		t.Fatalf("Failed to find targets: %v", err)
	}

	if len(targets) != 3 {
		t.Errorf("Expected 3 targets, got %d", len(targets))
	}

	targetNames := make(map[string]bool)
	for _, target := range targets {
		targetNames[target.Name] = true
	}

	expectedNames := []string{"app1", "app2", "lib1"}
	for _, name := range expectedNames {
		if !targetNames[name] {
			t.Errorf("Expected to find target '%s'", name)
		}
	}
}

func TestCMakeAnalyzer_ValidateTarget(t *testing.T) {
	tempDir := t.TempDir()
	analyzer := analyzers.NewCMakeAnalyzer(tempDir)

	// Create CMakeLists.txt
	cmakeContent := `cmake_minimum_required(VERSION 3.10)
project(TestProject)
add_executable(test_target main.cpp)`

	err := os.WriteFile(filepath.Join(tempDir, "CMakeLists.txt"), []byte(cmakeContent), 0644)
	if err != nil {
		t.Fatalf("Failed to create CMakeLists.txt: %v", err)
	}

	tests := []struct {
		name          string
		target        *types.CMakeExecutableTarget
		expectError   bool
		expectedError string
	}{
		{
			name: "valid target",
			target: &types.CMakeExecutableTarget{
				BaseTarget: types.BaseTarget{
					Name: "valid-target",
				},
				TargetName: "test_target",
				Generator:  "Unix Makefiles",
			},
			expectError: false,
		},
		{
			name: "missing target name",
			target: &types.CMakeExecutableTarget{
				BaseTarget: types.BaseTarget{
					Name: "invalid-target",
				},
				TargetName: "",
			},
			expectError:   true,
			expectedError: "target name is required",
		},
		{
			name: "invalid generator",
			target: &types.CMakeExecutableTarget{
				BaseTarget: types.BaseTarget{
					Name: "invalid-generator",
				},
				TargetName: "test_target",
				Generator:  "Invalid Generator",
			},
			expectError:   true,
			expectedError: "invalid generator",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := analyzer.ValidateTarget(tt.target)

			if tt.expectError {
				if err == nil {
					t.Error("Expected validation error, got nil")
				} else if !strings.Contains(err.Error(), tt.expectedError) {
					t.Errorf("Expected error containing '%s', got: %v", tt.expectedError, err)
				}
			} else {
				if err != nil {
					t.Errorf("Expected no validation error, got: %v", err)
				}
			}
		})
	}
}

func TestCMakeAnalyzer_ValidateTarget_NoCMakeFile(t *testing.T) {
	tempDir := t.TempDir()
	analyzer := analyzers.NewCMakeAnalyzer(tempDir)

	// No CMakeLists.txt in directory
	target := &types.CMakeExecutableTarget{
		BaseTarget: types.BaseTarget{
			Name: "test-target",
		},
		TargetName: "test_target",
	}

	err := analyzer.ValidateTarget(target)
	if err == nil {
		t.Error("Expected validation error when CMakeLists.txt not found")
	}

	if !strings.Contains(err.Error(), "CMakeLists.txt not found") {
		t.Errorf("Expected 'CMakeLists.txt not found' error, got: %v", err)
	}
}

func TestCMakeAnalyzer_GetRecommendedConfig(t *testing.T) {
	tempDir := t.TempDir()
	analyzer := analyzers.NewCMakeAnalyzer(tempDir)

	// Create CMakeLists.txt with various target types
	cmakeContent := `cmake_minimum_required(VERSION 3.10)
project(TestProject)

add_executable(myapp main.cpp)
add_library(mylib STATIC lib.cpp)
add_library(mysharedlib SHARED shared.cpp)`

	err := os.WriteFile(filepath.Join(tempDir, "CMakeLists.txt"), []byte(cmakeContent), 0644)
	if err != nil {
		t.Fatalf("Failed to create CMakeLists.txt: %v", err)
	}

	config, err := analyzer.GetRecommendedConfig()
	if err != nil {
		t.Fatalf("Failed to get recommended config: %v", err)
	}

	// Check basic config properties
	if config.Version != "1.0" {
		t.Errorf("Expected version '1.0', got '%s'", config.Version)
	}

	if config.ProjectType != types.ProjectTypeCMake {
		t.Errorf("Expected project type CMake, got %s", config.ProjectType)
	}

	if len(config.Targets) != 3 {
		t.Errorf("Expected 3 targets in config, got %d", len(config.Targets))
	}

	// Parse and validate targets
	targetTypes := make(map[string]string)
	for _, rawTarget := range config.Targets {
		var target map[string]interface{}
		err := json.Unmarshal(rawTarget, &target)
		if err != nil {
			t.Fatalf("Failed to unmarshal target: %v", err)
		}

		name := target["name"].(string)
		targetType := target["type"].(string)
		targetTypes[name] = targetType
	}

	expectedTypes := map[string]string{
		"myapp":       "cmake-executable",
		"mylib":       "cmake-library",
		"mysharedlib": "cmake-library",
	}

	for name, expectedType := range expectedTypes {
		if actualType, ok := targetTypes[name]; !ok {
			t.Errorf("Expected target '%s' not found in config", name)
		} else if actualType != expectedType {
			t.Errorf("Expected target '%s' to have type '%s', got '%s'", name, expectedType, actualType)
		}
	}
}

func TestCMakeAnalyzer_GetBuildCommands(t *testing.T) {
	tempDir := t.TempDir()
	analyzer := analyzers.NewCMakeAnalyzer(tempDir)

	target := analyzers.CMakeTarget{
		Name: "test_target",
		Type: "EXECUTABLE",
	}

	commands := analyzer.GetBuildCommands(target, types.CMakeBuildTypeDebug)

	expectedCommands := []string{
		"cmake -B build -DCMAKE_BUILD_TYPE=Debug",
		"cmake --build build --target test_target",
	}

	if len(commands) != len(expectedCommands) {
		t.Errorf("Expected %d commands, got %d", len(expectedCommands), len(commands))
	}

	for i, expected := range expectedCommands {
		if i >= len(commands) {
			t.Errorf("Missing command at index %d: %s", i, expected)
			continue
		}

		if commands[i] != expected {
			t.Errorf("Command at index %d: expected '%s', got '%s'", i, expected, commands[i])
		}
	}
}

func TestCMakeAnalyzer_GetBuildCommands_Release(t *testing.T) {
	tempDir := t.TempDir()
	analyzer := analyzers.NewCMakeAnalyzer(tempDir)

	target := analyzers.CMakeTarget{
		Name: "release_target",
		Type: "EXECUTABLE",
	}

	commands := analyzer.GetBuildCommands(target, types.CMakeBuildTypeRelease)

	// First command should specify Release build type
	if !strings.Contains(commands[0], "-DCMAKE_BUILD_TYPE=Release") {
		t.Errorf("Expected Release build type in command: %s", commands[0])
	}
}

func TestCMakeAnalyzer_ComplexProject(t *testing.T) {
	tempDir := t.TempDir()
	analyzer := analyzers.NewCMakeAnalyzer(tempDir)

	// Create complex project structure
	dirs := []string{
		"src",
		"tests",
		"lib",
		"external",
	}

	for _, dir := range dirs {
		err := os.MkdirAll(filepath.Join(tempDir, dir), 0755)
		if err != nil {
			t.Fatalf("Failed to create directory %s: %v", dir, err)
		}
	}

	// Main CMakeLists.txt
	mainCMake := `cmake_minimum_required(VERSION 3.15)
project(ComplexProject VERSION 2.1.0 LANGUAGES CXX)

set(CMAKE_CXX_STANDARD 17)

# Main executable
add_executable(myapp
    src/main.cpp
    src/utils.cpp
)

# Static library
add_library(core STATIC
    lib/core.cpp
    lib/helpers.cpp
)

# Shared library
add_library(plugin SHARED
    lib/plugin.cpp
)

# Link libraries
target_link_libraries(myapp core)

# Tests (if enabled)
option(BUILD_TESTS "Build tests" ON)
if(BUILD_TESTS)
    enable_testing()
    add_subdirectory(tests)
endif()`

	err := os.WriteFile(filepath.Join(tempDir, "CMakeLists.txt"), []byte(mainCMake), 0644)
	if err != nil {
		t.Fatalf("Failed to create main CMakeLists.txt: %v", err)
	}

	// Tests CMakeLists.txt
	testsCMake := `add_executable(unit_tests
    test_main.cpp
    test_core.cpp
)

target_link_libraries(unit_tests core)

add_test(NAME UnitTests COMMAND unit_tests)`

	err = os.WriteFile(filepath.Join(tempDir, "tests", "CMakeLists.txt"), []byte(testsCMake), 0644)
	if err != nil {
		t.Fatalf("Failed to create tests CMakeLists.txt: %v", err)
	}

	options := &analyzers.AnalysisOptions{
		IncludeTests:    true,
		RecursiveSearch: true,
		BuildDir:        "cmake-build",
		Generator:       "Ninja",
	}

	project, err := analyzer.AnalyzeProject(options)
	if err != nil {
		t.Fatalf("Failed to analyze complex project: %v", err)
	}

	// Verify project info
	if project.Name != "ComplexProject" {
		t.Errorf("Expected project name 'ComplexProject', got '%s'", project.Name)
	}

	if project.Version != "2.1.0" {
		t.Errorf("Expected project version '2.1.0', got '%s'", project.Version)
	}

	if project.BuildDir != "cmake-build" {
		t.Errorf("Expected build dir 'cmake-build', got '%s'", project.BuildDir)
	}

	if project.Generator != "Ninja" {
		t.Errorf("Expected generator 'Ninja', got '%s'", project.Generator)
	}

	// Should find: myapp, core, plugin, unit_tests, UnitTests
	expectedTargetCount := 5
	if len(project.Targets) != expectedTargetCount {
		t.Errorf("Expected %d targets, got %d", expectedTargetCount, len(project.Targets))
	}

	// Verify target types
	targetsByType := make(map[string]int)
	for _, target := range project.Targets {
		targetsByType[target.Type]++
	}

	expected := map[string]int{
		"EXECUTABLE":     2, // myapp, unit_tests
		"STATIC_LIBRARY": 1, // core
		"SHARED_LIBRARY": 1, // plugin
		"TEST":           1, // UnitTests
	}

	for targetType, expectedCount := range expected {
		if actualCount := targetsByType[targetType]; actualCount != expectedCount {
			t.Errorf("Expected %d targets of type %s, got %d", expectedCount, targetType, actualCount)
		}
	}
}

func TestCMakeAnalyzer_EdgeCases(t *testing.T) {
	tempDir := t.TempDir()
	analyzer := analyzers.NewCMakeAnalyzer(tempDir)

	tests := []struct {
		name          string
		cmakeContent  string
		expectError   bool
		expectedCount int
	}{
		{
			name: "empty cmake file",
			cmakeContent: `# Empty file
			# Just comments`,
			expectError:   false,
			expectedCount: 0,
		},
		{
			name: "project with no targets",
			cmakeContent: `cmake_minimum_required(VERSION 3.10)
project(EmptyProject)
# No targets defined`,
			expectError:   false,
			expectedCount: 0,
		},
		{
			name: "malformed target definitions",
			cmakeContent: `cmake_minimum_required(VERSION 3.10)
project(MalformedProject)
add_executable(
# Incomplete target definition`,
			expectError:   false,
			expectedCount: 0, // Should skip malformed targets
		},
		{
			name: "targets with complex names",
			cmakeContent: `cmake_minimum_required(VERSION 3.10)
project(ComplexNames)
add_executable(my-app-v2.1 main.cpp)
add_library(lib_core_utils STATIC utils.cpp)`,
			expectError:   false,
			expectedCount: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Clean up and create new CMakeLists.txt
			cmakeFile := filepath.Join(tempDir, "CMakeLists.txt")
			os.Remove(cmakeFile)

			err := os.WriteFile(cmakeFile, []byte(tt.cmakeContent), 0644)
			if err != nil {
				t.Fatalf("Failed to create CMakeLists.txt: %v", err)
			}

			project, err := analyzer.AnalyzeProject(nil)

			if tt.expectError {
				if err == nil {
					t.Error("Expected error, got nil")
				}
			} else {
				if err != nil {
					t.Errorf("Expected no error, got: %v", err)
				}

				if len(project.Targets) != tt.expectedCount {
					t.Errorf("Expected %d targets, got %d", tt.expectedCount, len(project.Targets))
				}
			}
		})
	}
}

func TestCMakeAnalyzer_DefaultAnalysisOptions(t *testing.T) {
	options := analyzers.DefaultAnalysisOptions()

	if !options.IncludeTests {
		t.Error("Expected IncludeTests to be true by default")
	}

	if !options.AnalyzeDeps {
		t.Error("Expected AnalyzeDeps to be true by default")
	}

	if options.BuildDir != "build" {
		t.Errorf("Expected default BuildDir 'build', got '%s'", options.BuildDir)
	}

	if options.Generator != "" {
		t.Errorf("Expected empty default Generator, got '%s'", options.Generator)
	}

	if !options.RecursiveSearch {
		t.Error("Expected RecursiveSearch to be true by default")
	}
}

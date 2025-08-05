package validation_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/poltergeist/poltergeist/pkg/types"
	"github.com/poltergeist/poltergeist/pkg/validation"
)

func TestTargetValidator_ValidateBasicFields(t *testing.T) {
	tempDir := t.TempDir()
	validator := validation.NewTargetValidator(tempDir)
	
	// Create output directory for valid test
	os.MkdirAll(filepath.Join(tempDir, "output"), 0755)

	tests := []struct {
		name          string
		target        types.Target
		expectInvalid bool  // Whether result.Valid should be false
		expectIssue   bool  // Whether any error/warning is expected
		errorLevel    validation.ValidationLevel
	}{
		{
			name: "valid executable target",
			target: &types.ExecutableTarget{
				BaseTarget: types.BaseTarget{
					Name:         "valid-target",
					Type:         types.TargetTypeExecutable,
					BuildCommand: "go build",
					WatchPaths:   []string{"*.go"},
				},
				OutputPath: "output/binary",
			},
			expectInvalid: false,
			expectIssue:   false,
		},
		{
			name: "missing name",
			target: &types.ExecutableTarget{
				BaseTarget: types.BaseTarget{
					Name:         "",
					Type:         types.TargetTypeExecutable,
					BuildCommand: "go build",
					WatchPaths:   []string{"*.go"},
				},
			},
			expectInvalid: true,
			expectIssue:   true,
			errorLevel:    validation.ValidationLevelError,
		},
		{
			name: "name with spaces",
			target: &types.ExecutableTarget{
				BaseTarget: types.BaseTarget{
					Name:         "invalid name",
					Type:         types.TargetTypeExecutable,
					BuildCommand: "go build",
					WatchPaths:   []string{"*.go"},
				},
			},
			expectInvalid: true,
			expectIssue:   true,
			errorLevel:    validation.ValidationLevelError,
		},
		{
			name: "missing build command",
			target: &types.ExecutableTarget{
				BaseTarget: types.BaseTarget{
					Name:         "test-target",
					Type:         types.TargetTypeExecutable,
					BuildCommand: "",
					WatchPaths:   []string{"*.go"},
				},
			},
			expectInvalid: true,
			expectIssue:   true,
			errorLevel:    validation.ValidationLevelError,
		},
		{
			name: "missing watch paths",
			target: &types.ExecutableTarget{
				BaseTarget: types.BaseTarget{
					Name:         "test-target",
					Type:         types.TargetTypeExecutable,
					BuildCommand: "go build",
					WatchPaths:   []string{},
				},
			},
			expectInvalid: false,  // Warnings don't make result invalid
			expectIssue:   true,   // But we still expect a warning
			errorLevel:    validation.ValidationLevelWarning,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := validator.Validate(tt.target)

			// Check if validation result matches expectation
			if tt.expectInvalid {
				if result.Valid {
					t.Error("Expected validation to fail, but it passed")
				}
			} else {
				if !result.Valid {
					t.Errorf("Expected validation to pass, but it failed: %v", result.Errors)
				}
			}

			// Check if we got the expected error/warning
			if tt.expectIssue {
				found := false
				for _, err := range result.Errors {
					if err.Level == tt.errorLevel {
						found = true
						break
					}
				}

				if !found {
					t.Errorf("Expected issue with level %s, but not found", tt.errorLevel)
				}
			} else {
				if len(result.Errors) > 0 {
					t.Errorf("Expected no issues, but got: %v", result.Errors)
				}
			}
		})
	}
}

func TestTargetValidator_ValidateTypeSpecific(t *testing.T) {
	tempDir := t.TempDir()
	validator := validation.NewTargetValidator(tempDir)

	tests := []struct {
		name           string
		target         types.Target
		expectWarning  bool
		expectError    bool
		expectedField  string
	}{
		{
			name: "executable without output path",
			target: &types.ExecutableTarget{
				BaseTarget: types.BaseTarget{
					Name:         "test-exe",
					Type:         types.TargetTypeExecutable,
					BuildCommand: "go build",
					WatchPaths:   []string{"*.go"},
				},
				OutputPath: "",
			},
			expectWarning: true,
			expectedField: "outputPath",
		},
		{
			name: "app bundle without bundle ID",
			target: &types.AppBundleTarget{
				BaseTarget: types.BaseTarget{
					Name:         "test-app",
					Type:         types.TargetTypeAppBundle,
					BuildCommand: "xcodebuild",
					WatchPaths:   []string{"*.swift"},
				},
				BundleID: "",
			},
			expectError:   true,
			expectedField: "bundleId",
		},
		{
			name: "test target without test command",
			target: &types.TestTarget{
				BaseTarget: types.BaseTarget{
					Name:         "test-tests",
					Type:         types.TargetTypeTest,
					BuildCommand: "go test",
					WatchPaths:   []string{"*_test.go"},
				},
				TestCommand: "",
			},
			expectError:   true,
			expectedField: "testCommand",
		},
		{
			name: "docker target without image name",
			target: &types.DockerTarget{
				BaseTarget: types.BaseTarget{
					Name:         "test-docker",
					Type:         types.TargetTypeDocker,
					BuildCommand: "docker build",
					WatchPaths:   []string{"Dockerfile"},
				},
				ImageName: "",
			},
			expectError:   true,
			expectedField: "imageName",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := validator.Validate(tt.target)

			if tt.expectError || tt.expectWarning {
				if result.Valid && tt.expectError {
					t.Error("Expected validation to fail with error, but it passed")
				}

				found := false
				for _, err := range result.Errors {
					if err.Field == tt.expectedField {
						found = true
						if tt.expectError && err.Level != validation.ValidationLevelError {
							t.Errorf("Expected error level, got %s", err.Level)
						}
						if tt.expectWarning && err.Level != validation.ValidationLevelWarning {
							t.Errorf("Expected warning level, got %s", err.Level)
						}
						break
					}
				}

				if !found {
					t.Errorf("Expected validation error for field %s, but not found", tt.expectedField)
				}
			}
		})
	}
}

func TestTargetValidator_ValidatePaths(t *testing.T) {
	tempDir := t.TempDir()
	validator := validation.NewTargetValidator(tempDir)

	// Create test directory structure
	testDir := filepath.Join(tempDir, "output")
	err := os.MkdirAll(testDir, 0755)
	if err != nil {
		t.Fatalf("Failed to create test directory: %v", err)
	}

	tests := []struct {
		name          string
		target        types.Target
		expectWarning bool
		expectedField string
	}{
		{
			name: "valid relative output path",
			target: &types.ExecutableTarget{
				BaseTarget: types.BaseTarget{
					Name:         "test-exe",
					Type:         types.TargetTypeExecutable,
					BuildCommand: "go build",
					WatchPaths:   []string{"*.go"},
				},
				OutputPath: "output/binary",
			},
			expectWarning: false,
		},
		{
			name: "absolute output path",
			target: &types.ExecutableTarget{
				BaseTarget: types.BaseTarget{
					Name:         "test-exe",
					Type:         types.TargetTypeExecutable,
					BuildCommand: "go build",
					WatchPaths:   []string{"*.go"},
				},
				OutputPath: "/absolute/path/binary",
			},
			expectWarning: true,
			expectedField: "outputPath",
		},
		{
			name: "nonexistent output directory",
			target: &types.ExecutableTarget{
				BaseTarget: types.BaseTarget{
					Name:         "test-exe",
					Type:         types.TargetTypeExecutable,
					BuildCommand: "go build",
					WatchPaths:   []string{"*.go"},
				},
				OutputPath: "nonexistent/binary",
			},
			expectWarning: true,
			expectedField: "outputPath",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := validator.Validate(tt.target)

			if tt.expectWarning {
				found := false
				for _, err := range result.Errors {
					if err.Field == tt.expectedField && err.Level == validation.ValidationLevelWarning {
						found = true
						break
					}
				}

				if !found {
					t.Errorf("Expected warning for field %s, but not found", tt.expectedField)
				}
			}
		})
	}
}

func TestTargetValidator_ValidateWatchPaths(t *testing.T) {
	tempDir := t.TempDir()
	validator := validation.NewTargetValidator(tempDir)

	// Create test files
	testFile := filepath.Join(tempDir, "test.go")
	err := os.WriteFile(testFile, []byte("package main"), 0644)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	tests := []struct {
		name          string
		watchPaths    []string
		expectError   bool
		expectWarning bool
	}{
		{
			name:       "valid pattern paths",
			watchPaths: []string{"*.go", "**/*.js"},
		},
		{
			name:       "valid existing file path",
			watchPaths: []string{"test.go"},
		},
		{
			name:        "empty watch path",
			watchPaths:  []string{""},
			expectError: true,
		},
		{
			name:          "absolute watch path",
			watchPaths:    []string{"/absolute/path/*.go"},
			expectWarning: true,
		},
		{
			name:          "nonexistent file path",
			watchPaths:    []string{"nonexistent.go"},
			expectWarning: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			target := &types.ExecutableTarget{
				BaseTarget: types.BaseTarget{
					Name:         "test-target",
					Type:         types.TargetTypeExecutable,
					BuildCommand: "go build",
					WatchPaths:   tt.watchPaths,
				},
			}

			result := validator.Validate(target)

			if tt.expectError {
				if result.Valid {
					t.Error("Expected validation to fail, but it passed")
				}

				found := false
				for _, err := range result.Errors {
					if err.Level == validation.ValidationLevelError && err.Field == "watchPaths" {
						found = true
						break
					}
				}

				if !found {
					t.Error("Expected error for watchPaths field")
				}
			}

			if tt.expectWarning {
				found := false
				for _, err := range result.Errors {
					if err.Level == validation.ValidationLevelWarning && err.Field == "watchPaths" {
						found = true
						break
					}
				}

				if !found {
					t.Error("Expected warning for watchPaths field")
				}
			}
		})
	}
}

func TestTargetValidator_ValidateMultiple(t *testing.T) {
	tempDir := t.TempDir()
	validator := validation.NewTargetValidator(tempDir)

	targets := []types.Target{
		&types.ExecutableTarget{
			BaseTarget: types.BaseTarget{
				Name:         "target1",
				Type:         types.TargetTypeExecutable,
				BuildCommand: "go build",
				WatchPaths:   []string{"*.go"},
			},
		},
		&types.ExecutableTarget{
			BaseTarget: types.BaseTarget{
				Name:         "target2",
				Type:         types.TargetTypeExecutable,
				BuildCommand: "go build",
				WatchPaths:   []string{"*.go"},
			},
		},
		&types.ExecutableTarget{
			BaseTarget: types.BaseTarget{
				Name:         "target1", // Duplicate name
				Type:         types.TargetTypeExecutable,
				BuildCommand: "go build",
				WatchPaths:   []string{"*.go"},
			},
		},
	}

	result := validator.ValidateMultiple(targets)

	if result.Valid {
		t.Error("Expected validation to fail due to duplicate names")
	}

	found := false
	for _, err := range result.Errors {
		if err.Field == "name" && err.Level == validation.ValidationLevelError {
			found = true
			break
		}
	}

	if !found {
		t.Error("Expected error for duplicate target names")
	}
}

func TestTargetValidator_ValidateConfiguration(t *testing.T) {
	tempDir := t.TempDir()
	validator := validation.NewTargetValidator(tempDir)

	tests := []struct {
		name        string
		config      *types.PoltergeistConfig
		expectError bool
	}{
		{
			name: "valid configuration",
			config: &types.PoltergeistConfig{
				Version:     "1.0",
				ProjectType: types.ProjectTypeNode,
				Targets: []json.RawMessage{
					createTargetJSON(t, map[string]interface{}{
						"name":         "valid-target",
						"type":         "executable",
						"buildCommand": "npm run build",
						"watchPaths":   []string{"src/**/*.js"},
					}),
				},
			},
			expectError: false,
		},
		{
			name: "configuration with no targets",
			config: &types.PoltergeistConfig{
				Version:     "1.0",
				ProjectType: types.ProjectTypeNode,
				Targets:     []json.RawMessage{},
			},
			expectError: true,
		},
		{
			name: "configuration with invalid target",
			config: &types.PoltergeistConfig{
				Version:     "1.0",
				ProjectType: types.ProjectTypeNode,
				Targets: []json.RawMessage{
					[]byte(`{"invalid": "json"`), // Invalid JSON
				},
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := validator.ValidateConfiguration(tt.config)

			if tt.expectError {
				if result.Valid {
					t.Error("Expected configuration validation to fail")
				}
			} else {
				if !result.Valid {
					t.Errorf("Expected configuration validation to pass, got errors: %v", result.Errors)
				}
			}
		})
	}
}

func TestValidationResult_AddError(t *testing.T) {
	result := &validation.ValidationResult{Valid: true}

	// Add warning - should not affect validity
	result.AddError("target1", "field1", "warning message", validation.ValidationLevelWarning)
	if !result.Valid {
		t.Error("Warning should not make result invalid")
	}

	// Add error - should make result invalid
	result.AddError("target1", "field2", "error message", validation.ValidationLevelError)
	if result.Valid {
		t.Error("Error should make result invalid")
	}

	if len(result.Errors) != 2 {
		t.Errorf("Expected 2 errors, got %d", len(result.Errors))
	}

	// Verify error details
	warningErr := result.Errors[0]
	if warningErr.Target != "target1" || warningErr.Field != "field1" || warningErr.Level != validation.ValidationLevelWarning {
		t.Error("Warning error details incorrect")
	}

	errorErr := result.Errors[1]
	if errorErr.Target != "target1" || errorErr.Field != "field2" || errorErr.Level != validation.ValidationLevelError {
		t.Error("Error details incorrect")
	}
}

func TestValidationError_Error(t *testing.T) {
	err := validation.ValidationError{
		Target:  "test-target",
		Field:   "testField",
		Message: "test message",
		Level:   validation.ValidationLevelError,
	}

	expected := "[error] test-target.testField: test message"
	if err.Error() != expected {
		t.Errorf("Expected error string '%s', got '%s'", expected, err.Error())
	}
}

func TestTargetValidator_EdgeCases(t *testing.T) {
	tempDir := t.TempDir()
	validator := validation.NewTargetValidator(tempDir)

	tests := []struct {
		name   string
		target types.Target
	}{
		{
			name: "library target with all fields",
			target: &types.LibraryTarget{
				BaseTarget: types.BaseTarget{
					Name:         "test-lib",
					Type:         types.TargetTypeLibrary,
					BuildCommand: "cargo build",
					WatchPaths:   []string{"src/**/*.rs"},
				},
				OutputPath:  "target/release/libtest.a",
				LibraryType: types.LibraryTypeStatic,
			},
		},
		{
			name: "framework target",
			target: &types.FrameworkTarget{
				BaseTarget: types.BaseTarget{
					Name:         "test-framework",
					Type:         types.TargetTypeFramework,
					BuildCommand: "xcodebuild",
					WatchPaths:   []string{"**/*.swift"},
				},
				OutputPath: "build/TestFramework.framework",
				Platform:   types.PlatformMacOS,
			},
		},
		{
			name: "custom target",
			target: &types.CustomTarget{
				BaseTarget: types.BaseTarget{
					Name:         "custom-target",
					Type:         types.TargetTypeCustom,
					BuildCommand: "make custom",
					WatchPaths:   []string{"**/*"},
				},
				Config: map[string]interface{}{
					"customSetting": "value",
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := validator.Validate(tt.target)

			// These should all be valid
			if !result.Valid {
				t.Errorf("Expected validation to pass, got errors: %v", result.Errors)
			}
		})
	}
}

func TestTargetValidator_CMakeTargets(t *testing.T) {
	tempDir := t.TempDir()
	validator := validation.NewTargetValidator(tempDir)

	tests := []struct {
		name   string
		target types.Target
	}{
		{
			name: "cmake executable target",
			target: &types.CMakeExecutableTarget{
				BaseTarget: types.BaseTarget{
					Name:         "cmake-exe",
					Type:         types.TargetTypeCMakeExecutable,
					BuildCommand: "cmake --build build --target exe",
					WatchPaths:   []string{"src/**/*.cpp", "CMakeLists.txt"},
				},
				Generator:  "Unix Makefiles",
				BuildType:  types.CMakeBuildTypeDebug,
				TargetName: "exe",
			},
		},
		{
			name: "cmake library target",
			target: &types.CMakeLibraryTarget{
				BaseTarget: types.BaseTarget{
					Name:         "cmake-lib",
					Type:         types.TargetTypeCMakeLibrary,
					BuildCommand: "cmake --build build --target lib",
					WatchPaths:   []string{"src/**/*.cpp", "CMakeLists.txt"},
				},
				Generator:   "Unix Makefiles",
				BuildType:   types.CMakeBuildTypeRelease,
				TargetName:  "lib",
				LibraryType: types.LibraryTypeStatic,
			},
		},
		{
			name: "cmake custom target",
			target: &types.CMakeCustomTarget{
				BaseTarget: types.BaseTarget{
					Name:         "cmake-custom",
					Type:         types.TargetTypeCMakeCustom,
					BuildCommand: "cmake --build build --target custom",
					WatchPaths:   []string{"**/*.cmake"},
				},
				Generator:  "Ninja",
				BuildType:  types.CMakeBuildTypeDebug,
				TargetName: "custom",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := validator.Validate(tt.target)

			// These should all be valid
			if !result.Valid {
				t.Errorf("Expected validation to pass, got errors: %v", result.Errors)
			}
		})
	}
}

func TestTargetValidator_ComplexValidation(t *testing.T) {
	tempDir := t.TempDir()
	validator := validation.NewTargetValidator(tempDir)

	// Create complex directory structure
	srcDir := filepath.Join(tempDir, "src")
	testDir := filepath.Join(tempDir, "test")
	buildDir := filepath.Join(tempDir, "build")

	dirs := []string{srcDir, testDir, buildDir}
	for _, dir := range dirs {
		err := os.MkdirAll(dir, 0755)
		if err != nil {
			t.Fatalf("Failed to create directory %s: %v", dir, err)
		}
	}

	// Create test files
	files := map[string]string{
		"src/main.go":    "package main",
		"test/main_test.go": "package main",
		"Makefile":       "all:\n\techo build",
	}

	for path, content := range files {
		fullPath := filepath.Join(tempDir, path)
		err := os.WriteFile(fullPath, []byte(content), 0644)
		if err != nil {
			t.Fatalf("Failed to create file %s: %v", fullPath, err)
		}
	}

	// Complex target with multiple watch paths and validation scenarios
	target := &types.ExecutableTarget{
		BaseTarget: types.BaseTarget{
			Name: "complex-target",
			Type: types.TargetTypeExecutable,
			BuildCommand: "make && go build -o build/app ./src",
			WatchPaths: []string{
				"src/**/*.go",    // Pattern path (valid)
				"test/*.go",      // Pattern path (valid)
				"Makefile",       // Existing file (valid)
				"nonexistent.go", // Nonexistent file (warning)
				"/abs/path.go",   // Absolute path (warning)
			},
		},
		OutputPath: "build/app", // Valid relative path to existing dir
	}

	result := validator.Validate(target)

	// Should be valid overall (warnings don't invalidate)
	if !result.Valid {
		t.Errorf("Expected complex target to be valid, got errors: %v", result.Errors)
	}

	// Should have warnings for nonexistent file and absolute path
	warningCount := 0
	for _, err := range result.Errors {
		if err.Level == validation.ValidationLevelWarning {
			warningCount++
		}
	}

	if warningCount != 2 {
		t.Errorf("Expected 2 warnings, got %d", warningCount)
	}
}

// Helper functions

func createTargetJSON(t *testing.T, target map[string]interface{}) json.RawMessage {
	data, err := json.Marshal(target)
	if err != nil {
		t.Fatalf("Failed to marshal target: %v", err)
	}
	return json.RawMessage(data)
}
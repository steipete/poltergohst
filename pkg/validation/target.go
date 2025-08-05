// Package validation provides target validation functionality
package validation

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/poltergeist/poltergeist/pkg/types"
)

// TargetValidator validates build targets
type TargetValidator struct {
	projectRoot string
}

// NewTargetValidator creates a new target validator
func NewTargetValidator(projectRoot string) *TargetValidator {
	return &TargetValidator{
		projectRoot: projectRoot,
	}
}

// ValidationError represents a validation error
type ValidationError struct {
	Target  string
	Field   string
	Message string
	Level   ValidationLevel
}

// ValidationLevel represents error severity
type ValidationLevel string

const (
	ValidationLevelError   ValidationLevel = "error"
	ValidationLevelWarning ValidationLevel = "warning"
	ValidationLevelInfo    ValidationLevel = "info"
)

func (e *ValidationError) Error() string {
	return fmt.Sprintf("[%s] %s.%s: %s", e.Level, e.Target, e.Field, e.Message)
}

// ValidationResult contains validation results
type ValidationResult struct {
	Valid  bool
	Errors []ValidationError
}

// AddError adds an error to the validation result
func (r *ValidationResult) AddError(target, field, message string, level ValidationLevel) {
	r.Errors = append(r.Errors, ValidationError{
		Target:  target,
		Field:   field,
		Message: message,
		Level:   level,
	})
	if level == ValidationLevelError {
		r.Valid = false
	}
}

// Validate validates a target
func (v *TargetValidator) Validate(target types.Target) *ValidationResult {
	result := &ValidationResult{Valid: true}

	// Basic validation
	v.validateBasicFields(target, result)
	
	// Type-specific validation
	v.validateTypeSpecific(target, result)
	
	// Path validation
	v.validatePaths(target, result)
	
	// Watch paths validation
	v.validateWatchPaths(target, result)

	return result
}

// ValidateMultiple validates multiple targets
func (v *TargetValidator) ValidateMultiple(targets []types.Target) *ValidationResult {
	result := &ValidationResult{Valid: true}
	
	names := make(map[string]bool)
	
	for _, target := range targets {
		// Check for duplicate names
		if names[target.GetName()] {
			result.AddError(target.GetName(), "name", "duplicate target name", ValidationLevelError)
		}
		names[target.GetName()] = true
		
		// Validate individual target
		targetResult := v.Validate(target)
		result.Errors = append(result.Errors, targetResult.Errors...)
		if !targetResult.Valid {
			result.Valid = false
		}
	}
	
	return result
}

func (v *TargetValidator) validateBasicFields(target types.Target, result *ValidationResult) {
	name := target.GetName()
	
	if name == "" {
		result.AddError("", "name", "target name is required", ValidationLevelError)
		return
	}
	
	if strings.Contains(name, " ") {
		result.AddError(name, "name", "target name cannot contain spaces", ValidationLevelError)
	}
	
	if target.GetBuildCommand() == "" {
		result.AddError(name, "buildCommand", "build command is required", ValidationLevelError)
	}
	
	if len(target.GetWatchPaths()) == 0 {
		result.AddError(name, "watchPaths", "at least one watch path is required", ValidationLevelWarning)
	}
}

func (v *TargetValidator) validateTypeSpecific(target types.Target, result *ValidationResult) {
	name := target.GetName()
	
	switch target.GetType() {
	case types.TargetTypeExecutable:
		// Executable-specific validation
		if executable, ok := target.(*types.ExecutableTarget); ok {
			if executable.OutputPath == "" {
				result.AddError(name, "outputPath", "output path is required for executable targets", ValidationLevelWarning)
			}
		}
		
	case types.TargetTypeAppBundle:
		// App bundle-specific validation
		if appBundle, ok := target.(*types.AppBundleTarget); ok {
			if appBundle.BundleID == "" {
				result.AddError(name, "bundleId", "bundle ID is required for app bundle targets", ValidationLevelError)
			}
		}
		
	case types.TargetTypeTest:
		// Test-specific validation
		if test, ok := target.(*types.TestTarget); ok {
			if test.TestCommand == "" {
				result.AddError(name, "testCommand", "test command is required for test targets", ValidationLevelError)
			}
		}
		
	case types.TargetTypeDocker:
		// Docker-specific validation
		if docker, ok := target.(*types.DockerTarget); ok {
			if docker.ImageName == "" {
				result.AddError(name, "imageName", "image name is required for Docker targets", ValidationLevelError)
			}
		}
	}
}

func (v *TargetValidator) validatePaths(target types.Target, result *ValidationResult) {
	name := target.GetName()
	
	// Validate output paths based on target type
	switch t := target.(type) {
	case *types.ExecutableTarget:
		if t.OutputPath != "" {
			v.validateOutputPath(name, t.OutputPath, result)
		}
	case *types.LibraryTarget:
		if t.OutputPath != "" {
			v.validateOutputPath(name, t.OutputPath, result)
		}
	case *types.FrameworkTarget:
		if t.OutputPath != "" {
			v.validateOutputPath(name, t.OutputPath, result)
		}
	}
}

func (v *TargetValidator) validateOutputPath(targetName, outputPath string, result *ValidationResult) {
	if filepath.IsAbs(outputPath) {
		result.AddError(targetName, "outputPath", "output path should be relative to project root", ValidationLevelWarning)
		return
	}
	
	fullPath := filepath.Join(v.projectRoot, outputPath)
	dir := filepath.Dir(fullPath)
	
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		result.AddError(targetName, "outputPath", fmt.Sprintf("output directory does not exist: %s", dir), ValidationLevelWarning)
	}
}

func (v *TargetValidator) validateWatchPaths(target types.Target, result *ValidationResult) {
	name := target.GetName()
	watchPaths := target.GetWatchPaths()
	
	// Check if watch paths are empty
	if len(watchPaths) == 0 {
		result.AddError(name, "watchPaths", "no watch paths specified", ValidationLevelWarning)
		return
	}
	
	for _, path := range watchPaths {
		if path == "" {
			result.AddError(name, "watchPaths", "empty watch path", ValidationLevelError)
			continue
		}
		
		// Check for absolute paths
		if filepath.IsAbs(path) {
			result.AddError(name, "watchPaths", fmt.Sprintf("watch path should be relative: %s", path), ValidationLevelWarning)
			continue
		}
		
		// Check if path exists (for non-pattern paths)
		if !strings.Contains(path, "*") && !strings.Contains(path, "?") {
			fullPath := filepath.Join(v.projectRoot, path)
			if _, err := os.Stat(fullPath); os.IsNotExist(err) {
				result.AddError(name, "watchPaths", fmt.Sprintf("watch path does not exist: %s", path), ValidationLevelWarning)
			}
		}
	}
}

// ValidateConfiguration validates an entire configuration
func (v *TargetValidator) ValidateConfiguration(config *types.PoltergeistConfig) *ValidationResult {
	result := &ValidationResult{Valid: true}
	
	if len(config.Targets) == 0 {
		result.AddError("config", "targets", "no targets defined", ValidationLevelError)
		return result
	}
	
	targets := make([]types.Target, 0, len(config.Targets))
	for _, rawTarget := range config.Targets {
		target, err := types.ParseTarget(rawTarget)
		if err != nil {
			result.AddError("config", "targets", fmt.Sprintf("failed to parse target: %v", err), ValidationLevelError)
			continue
		}
		targets = append(targets, target)
	}
	
	// Validate all targets
	targetsResult := v.ValidateMultiple(targets)
	result.Errors = append(result.Errors, targetsResult.Errors...)
	if !targetsResult.Valid {
		result.Valid = false
	}
	
	return result
}
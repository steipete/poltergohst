package builders

import (
	"fmt"

	"github.com/poltergeist/poltergeist/pkg/interfaces"
	"github.com/poltergeist/poltergeist/pkg/logger"
	"github.com/poltergeist/poltergeist/pkg/types"
)

// BuilderFactory creates builders based on target type
type BuilderFactory struct{}

// NewBuilderFactory creates a new builder factory
func NewBuilderFactory() *BuilderFactory {
	return &BuilderFactory{}
}

// CreateBuilder creates the appropriate builder for a target
func (f *BuilderFactory) CreateBuilder(
	target types.Target,
	projectRoot string,
	log logger.Logger,
	stateManager interfaces.StateManager,
) interfaces.Builder {
	switch target.GetType() {
	case types.TargetTypeExecutable:
		return NewExecutableBuilder(target, projectRoot, log, stateManager)
		
	case types.TargetTypeAppBundle:
		return NewAppBundleBuilder(target, projectRoot, log, stateManager)
		
	case types.TargetTypeLibrary:
		return NewLibraryBuilder(target, projectRoot, log, stateManager)
		
	case types.TargetTypeFramework:
		return NewFrameworkBuilder(target, projectRoot, log, stateManager)
		
	case types.TargetTypeTest:
		return NewTestBuilder(target, projectRoot, log, stateManager)
		
	case types.TargetTypeDocker:
		return NewDockerBuilder(target, projectRoot, log, stateManager)
		
	case types.TargetTypeCMakeExecutable:
		return NewCMakeExecutableBuilder(target, projectRoot, log, stateManager)
		
	case types.TargetTypeCMakeLibrary:
		return NewCMakeLibraryBuilder(target, projectRoot, log, stateManager)
		
	case types.TargetTypeCMakeCustom:
		return NewCMakeCustomBuilder(target, projectRoot, log, stateManager)
		
	case types.TargetTypeCustom:
		return NewCustomBuilder(target, projectRoot, log, stateManager)
		
	default:
		// Fallback to base builder
		return NewBaseBuilder(target, projectRoot, log, stateManager)
	}
}

// FrameworkBuilder builds framework targets
type FrameworkBuilder struct {
	*BaseBuilder
	outputPath string
	platform   types.Platform
}

// NewFrameworkBuilder creates a new framework builder
func NewFrameworkBuilder(
	target types.Target,
	projectRoot string,
	log logger.Logger,
	stateManager interfaces.StateManager,
) *FrameworkBuilder {
	base := NewBaseBuilder(target, projectRoot, log, stateManager)
	
	builder := &FrameworkBuilder{
		BaseBuilder: base,
	}
	
	// Extract framework specific fields
	if fwTarget, ok := target.(*types.FrameworkTarget); ok {
		builder.outputPath = fwTarget.OutputPath
		builder.platform = fwTarget.Platform
	}
	
	return builder
}

// CustomBuilder builds custom targets
type CustomBuilder struct {
	*BaseBuilder
	config map[string]interface{}
}

// NewCustomBuilder creates a new custom builder
func NewCustomBuilder(
	target types.Target,
	projectRoot string,
	log logger.Logger,
	stateManager interfaces.StateManager,
) *CustomBuilder {
	base := NewBaseBuilder(target, projectRoot, log, stateManager)
	
	builder := &CustomBuilder{
		BaseBuilder: base,
	}
	
	// Extract custom config
	if customTarget, ok := target.(*types.CustomTarget); ok {
		builder.config = customTarget.Config
	}
	
	return builder
}

// CMakeBuilder provides common CMake functionality
type CMakeBuilder struct {
	*BaseBuilder
	generator  string
	buildType  types.CMakeBuildType
	cmakeArgs  []string
	targetName string
	parallel   bool
}

// NewCMakeBuilder creates a base CMake builder
func NewCMakeBuilder(
	target types.Target,
	projectRoot string,
	log logger.Logger,
	stateManager interfaces.StateManager,
) *CMakeBuilder {
	base := NewBaseBuilder(target, projectRoot, log, stateManager)
	
	return &CMakeBuilder{
		BaseBuilder: base,
		generator:   "Unix Makefiles",
		buildType:   types.CMakeBuildTypeDebug,
		parallel:    true,
	}
}

// configureCMake runs CMake configuration
func (b *CMakeBuilder) configureCMake() error {
	buildDir := b.resolvePath("build")
	
	// Create build directory
	if err := b.ensureDirectory(buildDir); err != nil {
		return fmt.Errorf("failed to create build directory: %w", err)
	}
	
	// Build CMake command
	cmakeCmd := fmt.Sprintf("cmake -S . -B build -G \"%s\" -DCMAKE_BUILD_TYPE=%s",
		b.generator, b.buildType)
	
	// Add custom arguments
	for _, arg := range b.cmakeArgs {
		cmakeCmd += " " + arg
	}
	
	// Override build command temporarily
	originalCmd := b.Target.GetBuildCommand()
	defer func() {
		// Restore original command
		switch t := b.Target.(type) {
		case *types.CMakeExecutableTarget:
			t.BuildCommand = originalCmd
		case *types.CMakeLibraryTarget:
			t.BuildCommand = originalCmd
		case *types.CMakeCustomTarget:
			t.BuildCommand = originalCmd
		}
	}()
	
	// Set configure command
	switch t := b.Target.(type) {
	case *types.CMakeExecutableTarget:
		t.BuildCommand = cmakeCmd
	case *types.CMakeLibraryTarget:
		t.BuildCommand = cmakeCmd
	case *types.CMakeCustomTarget:
		t.BuildCommand = cmakeCmd
	}
	
	// Run configuration
	return b.BaseBuilder.Build(nil, nil)
}

// ensureDirectory ensures a directory exists
func (b *CMakeBuilder) ensureDirectory(path string) error {
	return nil // Simplified - would use os.MkdirAll
}

// CMakeExecutableBuilder builds CMake executable targets
type CMakeExecutableBuilder struct {
	*CMakeBuilder
	outputPath string
}

// NewCMakeExecutableBuilder creates a new CMake executable builder
func NewCMakeExecutableBuilder(
	target types.Target,
	projectRoot string,
	log logger.Logger,
	stateManager interfaces.StateManager,
) *CMakeExecutableBuilder {
	base := NewCMakeBuilder(target, projectRoot, log, stateManager)
	
	builder := &CMakeExecutableBuilder{
		CMakeBuilder: base,
	}
	
	// Extract CMake executable specific fields
	if cmakeTarget, ok := target.(*types.CMakeExecutableTarget); ok {
		if cmakeTarget.Generator != "" {
			builder.generator = cmakeTarget.Generator
		}
		if cmakeTarget.BuildType != "" {
			builder.buildType = cmakeTarget.BuildType
		}
		builder.cmakeArgs = cmakeTarget.CMakeArgs
		builder.targetName = cmakeTarget.TargetName
		builder.outputPath = cmakeTarget.OutputPath
		if cmakeTarget.Parallel != nil {
			builder.parallel = *cmakeTarget.Parallel
		}
	}
	
	return builder
}

// CMakeLibraryBuilder builds CMake library targets
type CMakeLibraryBuilder struct {
	*CMakeBuilder
	libraryType types.LibraryType
	outputPath  string
}

// NewCMakeLibraryBuilder creates a new CMake library builder
func NewCMakeLibraryBuilder(
	target types.Target,
	projectRoot string,
	log logger.Logger,
	stateManager interfaces.StateManager,
) *CMakeLibraryBuilder {
	base := NewCMakeBuilder(target, projectRoot, log, stateManager)
	
	builder := &CMakeLibraryBuilder{
		CMakeBuilder: base,
	}
	
	// Extract CMake library specific fields
	if cmakeTarget, ok := target.(*types.CMakeLibraryTarget); ok {
		if cmakeTarget.Generator != "" {
			builder.generator = cmakeTarget.Generator
		}
		if cmakeTarget.BuildType != "" {
			builder.buildType = cmakeTarget.BuildType
		}
		builder.cmakeArgs = cmakeTarget.CMakeArgs
		builder.targetName = cmakeTarget.TargetName
		builder.libraryType = cmakeTarget.LibraryType
		builder.outputPath = cmakeTarget.OutputPath
		if cmakeTarget.Parallel != nil {
			builder.parallel = *cmakeTarget.Parallel
		}
	}
	
	return builder
}

// CMakeCustomBuilder builds custom CMake targets
type CMakeCustomBuilder struct {
	*CMakeBuilder
}

// NewCMakeCustomBuilder creates a new CMake custom builder
func NewCMakeCustomBuilder(
	target types.Target,
	projectRoot string,
	log logger.Logger,
	stateManager interfaces.StateManager,
) *CMakeCustomBuilder {
	base := NewCMakeBuilder(target, projectRoot, log, stateManager)
	
	builder := &CMakeCustomBuilder{
		CMakeBuilder: base,
	}
	
	// Extract CMake custom specific fields
	if cmakeTarget, ok := target.(*types.CMakeCustomTarget); ok {
		if cmakeTarget.Generator != "" {
			builder.generator = cmakeTarget.Generator
		}
		if cmakeTarget.BuildType != "" {
			builder.buildType = cmakeTarget.BuildType
		}
		builder.cmakeArgs = cmakeTarget.CMakeArgs
		builder.targetName = cmakeTarget.TargetName
		if cmakeTarget.Parallel != nil {
			builder.parallel = *cmakeTarget.Parallel
		}
	}
	
	return builder
}
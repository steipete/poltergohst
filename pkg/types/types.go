// Package types provides core types and configurations for Poltergeist
package types

import (
	"encoding/json"
	"fmt"
	"time"
)

// TargetType represents supported build target types
type TargetType string

const (
	TargetTypeExecutable      TargetType = "executable"
	TargetTypeAppBundle       TargetType = "app-bundle"
	TargetTypeLibrary         TargetType = "library"
	TargetTypeFramework       TargetType = "framework"
	TargetTypeTest            TargetType = "test"
	TargetTypeDocker          TargetType = "docker"
	TargetTypeCustom          TargetType = "custom"
	TargetTypeCMakeExecutable TargetType = "cmake-executable"
	TargetTypeCMakeLibrary    TargetType = "cmake-library"
	TargetTypeCMakeCustom     TargetType = "cmake-custom"
)

// Platform represents supported Apple platforms
type Platform string

const (
	PlatformMacOS    Platform = "macos"
	PlatformIOS      Platform = "ios"
	PlatformTVOS     Platform = "tvos"
	PlatformWatchOS  Platform = "watchos"
	PlatformVisionOS Platform = "visionos"
)

// LibraryType represents library linkage types
type LibraryType string

const (
	LibraryTypeStatic  LibraryType = "static"
	LibraryTypeDynamic LibraryType = "dynamic"
	LibraryTypeShared  LibraryType = "shared"
)

// CMakeBuildType represents CMake build configurations
type CMakeBuildType string

const (
	CMakeBuildTypeDebug          CMakeBuildType = "Debug"
	CMakeBuildTypeRelease        CMakeBuildType = "Release"
	CMakeBuildTypeRelWithDebInfo CMakeBuildType = "RelWithDebInfo"
	CMakeBuildTypeMinSizeRel     CMakeBuildType = "MinSizeRel"
)

// ProjectType represents different project ecosystems
type ProjectType string

const (
	ProjectTypeSwift  ProjectType = "swift"
	ProjectTypeNode   ProjectType = "node"
	ProjectTypeRust   ProjectType = "rust"
	ProjectTypePython ProjectType = "python"
	ProjectTypeCMake  ProjectType = "cmake"
	ProjectTypeMixed  ProjectType = "mixed"
)

// PerformanceProfile represents performance optimization profiles
type PerformanceProfile string

const (
	PerformanceProfileConservative PerformanceProfile = "conservative"
	PerformanceProfileBalanced     PerformanceProfile = "balanced"
	PerformanceProfileAggressive   PerformanceProfile = "aggressive"
)

// LogLevel represents logging verbosity levels
type LogLevel string

const (
	LogLevelDebug LogLevel = "debug"
	LogLevelInfo  LogLevel = "info"
	LogLevelWarn  LogLevel = "warn"
	LogLevelError LogLevel = "error"
)

// BuildStatus represents the current state of a build
type BuildStatus string

const (
	BuildStatusIdle      BuildStatus = "idle"
	BuildStatusQueued    BuildStatus = "queued"
	BuildStatusBuilding  BuildStatus = "building"
	BuildStatusSucceeded BuildStatus = "succeeded"
	BuildStatusFailed    BuildStatus = "failed"
	BuildStatusCancelled BuildStatus = "cancelled"
)

// ChangeType represents the classification of file changes
type ChangeType string

const (
	ChangeTypeDirect    ChangeType = "direct"
	ChangeTypeShared    ChangeType = "shared"
	ChangeTypeGenerated ChangeType = "generated"
)

// BaseTarget represents common fields for all target types
type BaseTarget struct {
	Name              string            `json:"name" yaml:"name"`
	Type              TargetType        `json:"type" yaml:"type"`
	Enabled           *bool             `json:"enabled,omitempty" yaml:"enabled,omitempty"`
	BuildCommand      string            `json:"buildCommand,omitempty" yaml:"buildCommand,omitempty"`
	WatchPaths        []string          `json:"watchPaths" yaml:"watchPaths"`
	SettlingDelay     *int              `json:"settlingDelay,omitempty" yaml:"settlingDelay,omitempty"`
	Environment       map[string]string `json:"environment,omitempty" yaml:"environment,omitempty"`
	MaxRetries        *int              `json:"maxRetries,omitempty" yaml:"maxRetries,omitempty"`
	BackoffMultiplier *float64          `json:"backoffMultiplier,omitempty" yaml:"backoffMultiplier,omitempty"`
	DebounceInterval  *int              `json:"debounceInterval,omitempty" yaml:"debounceInterval,omitempty"`
	Icon              string            `json:"icon,omitempty" yaml:"icon,omitempty"`
}

// ExecutableTarget represents a CLI tool or binary target
type ExecutableTarget struct {
	BaseTarget
	OutputPath string `json:"outputPath" yaml:"outputPath"`
}

// AppBundleTarget represents macOS/iOS application bundles
type AppBundleTarget struct {
	BaseTarget
	Platform      Platform `json:"platform,omitempty" yaml:"platform,omitempty"`
	BundleID      string   `json:"bundleId" yaml:"bundleId"`
	AutoRelaunch  *bool    `json:"autoRelaunch,omitempty" yaml:"autoRelaunch,omitempty"`
	LaunchCommand string   `json:"launchCommand,omitempty" yaml:"launchCommand,omitempty"`
}

// LibraryTarget represents static or dynamic libraries
type LibraryTarget struct {
	BaseTarget
	OutputPath  string      `json:"outputPath" yaml:"outputPath"`
	LibraryType LibraryType `json:"libraryType" yaml:"libraryType"`
}

// FrameworkTarget represents Apple frameworks
type FrameworkTarget struct {
	BaseTarget
	OutputPath string   `json:"outputPath" yaml:"outputPath"`
	Platform   Platform `json:"platform,omitempty" yaml:"platform,omitempty"`
}

// TestTarget represents test suites
type TestTarget struct {
	BaseTarget
	TestCommand  string `json:"testCommand" yaml:"testCommand"`
	CoverageFile string `json:"coverageFile,omitempty" yaml:"coverageFile,omitempty"`
}

// DockerTarget represents container images
type DockerTarget struct {
	BaseTarget
	ImageName  string   `json:"imageName" yaml:"imageName"`
	Dockerfile string   `json:"dockerfile,omitempty" yaml:"dockerfile,omitempty"`
	Context    string   `json:"context,omitempty" yaml:"context,omitempty"`
	Tags       []string `json:"tags,omitempty" yaml:"tags,omitempty"`
}

// CustomTarget represents user-defined targets
type CustomTarget struct {
	BaseTarget
	Config map[string]interface{} `json:"config,omitempty" yaml:"config,omitempty"`
}

// CMakeExecutableTarget represents CMake executable targets
type CMakeExecutableTarget struct {
	BaseTarget
	Generator  string         `json:"generator,omitempty" yaml:"generator,omitempty"`
	BuildType  CMakeBuildType `json:"buildType,omitempty" yaml:"buildType,omitempty"`
	CMakeArgs  []string       `json:"cmakeArgs,omitempty" yaml:"cmakeArgs,omitempty"`
	TargetName string         `json:"targetName" yaml:"targetName"`
	OutputPath string         `json:"outputPath,omitempty" yaml:"outputPath,omitempty"`
	Parallel   *bool          `json:"parallel,omitempty" yaml:"parallel,omitempty"`
}

// CMakeLibraryTarget represents CMake library targets
type CMakeLibraryTarget struct {
	BaseTarget
	Generator   string         `json:"generator,omitempty" yaml:"generator,omitempty"`
	BuildType   CMakeBuildType `json:"buildType,omitempty" yaml:"buildType,omitempty"`
	CMakeArgs   []string       `json:"cmakeArgs,omitempty" yaml:"cmakeArgs,omitempty"`
	TargetName  string         `json:"targetName" yaml:"targetName"`
	LibraryType LibraryType    `json:"libraryType" yaml:"libraryType"`
	OutputPath  string         `json:"outputPath,omitempty" yaml:"outputPath,omitempty"`
	Parallel    *bool          `json:"parallel,omitempty" yaml:"parallel,omitempty"`
}

// CMakeCustomTarget represents custom CMake targets
type CMakeCustomTarget struct {
	BaseTarget
	Generator  string         `json:"generator,omitempty" yaml:"generator,omitempty"`
	BuildType  CMakeBuildType `json:"buildType,omitempty" yaml:"buildType,omitempty"`
	CMakeArgs  []string       `json:"cmakeArgs,omitempty" yaml:"cmakeArgs,omitempty"`
	TargetName string         `json:"targetName" yaml:"targetName"`
	Parallel   *bool          `json:"parallel,omitempty" yaml:"parallel,omitempty"`
}

// Target represents any build target (interface)
type Target interface {
	GetName() string
	GetType() TargetType
	IsEnabled() bool
	GetBuildCommand() string
	GetWatchPaths() []string
	GetSettlingDelay() int
	GetEnvironment() map[string]string
	GetMaxRetries() int
	GetBackoffMultiplier() float64
	GetDebounceInterval() int
	GetIcon() string
}

// ExclusionRule represents a file exclusion pattern
type ExclusionRule struct {
	Pattern string `json:"pattern" yaml:"pattern"`
	Action  string `json:"action" yaml:"action"`
	Reason  string `json:"reason" yaml:"reason"`
	Enabled *bool  `json:"enabled,omitempty" yaml:"enabled,omitempty"`
}

// PerformanceMetrics represents performance monitoring configuration
type PerformanceMetrics struct {
	Enabled        bool `json:"enabled" yaml:"enabled"`
	ReportInterval int  `json:"reportInterval" yaml:"reportInterval"`
}

// PerformanceConfig represents performance optimization settings
type PerformanceConfig struct {
	Profile      PerformanceProfile `json:"profile" yaml:"profile"`
	AutoOptimize bool               `json:"autoOptimize" yaml:"autoOptimize"`
	Metrics      PerformanceMetrics `json:"metrics" yaml:"metrics"`
}

// WatchmanConfig represents file watching configuration
type WatchmanConfig struct {
	UseDefaultExclusions bool            `json:"useDefaultExclusions" yaml:"useDefaultExclusions"`
	ExcludeDirs          []string        `json:"excludeDirs" yaml:"excludeDirs"`
	ProjectType          ProjectType     `json:"projectType,omitempty" yaml:"projectType,omitempty"`
	MaxFileEvents        int             `json:"maxFileEvents" yaml:"maxFileEvents"`
	RecrawlThreshold     int             `json:"recrawlThreshold" yaml:"recrawlThreshold"`
	SettlingDelay        int             `json:"settlingDelay" yaml:"settlingDelay"`
	Rules                []ExclusionRule `json:"rules,omitempty" yaml:"rules,omitempty"`
}

// BuildPrioritization represents build priority configuration
type BuildPrioritization struct {
	Enabled                bool    `json:"enabled" yaml:"enabled"`
	FocusDetectionWindow   int     `json:"focusDetectionWindow" yaml:"focusDetectionWindow"`
	PriorityDecayTime      int     `json:"priorityDecayTime" yaml:"priorityDecayTime"`
	BuildTimeoutMultiplier float64 `json:"buildTimeoutMultiplier" yaml:"buildTimeoutMultiplier"`
}

// BuildSchedulingConfig represents build scheduling configuration
type BuildSchedulingConfig struct {
	Parallelization int                 `json:"parallelization" yaml:"parallelization"`
	Prioritization  BuildPrioritization `json:"prioritization" yaml:"prioritization"`
}

// NotificationConfig represents notification preferences
type NotificationConfig struct {
	Enabled      *bool  `json:"enabled,omitempty" yaml:"enabled,omitempty"`
	SuccessSound string `json:"successSound,omitempty" yaml:"successSound,omitempty"`
	FailureSound string `json:"failureSound,omitempty" yaml:"failureSound,omitempty"`
}

// LoggingConfig represents logging configuration
type LoggingConfig struct {
	File  string   `json:"file" yaml:"file"`
	Level LogLevel `json:"level" yaml:"level"`
}

// PoltergeistConfig represents the main configuration
type PoltergeistConfig struct {
	Version         string                 `json:"version" yaml:"version"`
	ProjectType     ProjectType            `json:"projectType" yaml:"projectType"`
	Targets         []json.RawMessage      `json:"targets" yaml:"targets"`
	Watchman        *WatchmanConfig        `json:"watchman,omitempty" yaml:"watchman,omitempty"`
	Performance     *PerformanceConfig     `json:"performance,omitempty" yaml:"performance,omitempty"`
	BuildScheduling *BuildSchedulingConfig `json:"buildScheduling,omitempty" yaml:"buildScheduling,omitempty"`
	Notifications   *NotificationConfig    `json:"notifications,omitempty" yaml:"notifications,omitempty"`
	Logging         *LoggingConfig         `json:"logging,omitempty" yaml:"logging,omitempty"`
}

// ChangeEvent represents a file change event
type ChangeEvent struct {
	File            string     `json:"file"`
	Timestamp       time.Time  `json:"timestamp"`
	AffectedTargets []string   `json:"affectedTargets"`
	ChangeType      ChangeType `json:"changeType"`
	ImpactWeight    float64    `json:"impactWeight"`
}

// TargetPriority represents target priority scoring
type TargetPriority struct {
	Target                string        `json:"target"`
	Score                 float64       `json:"score"`
	LastDirectChange      time.Time     `json:"lastDirectChange"`
	DirectChangeFrequency float64       `json:"directChangeFrequency"`
	FocusMultiplier       float64       `json:"focusMultiplier"`
	AvgBuildTime          time.Duration `json:"avgBuildTime"`
	SuccessRate           float64       `json:"successRate"`
	RecentChanges         []ChangeEvent `json:"recentChanges"`
}

// BuildRequest represents a queued build request
type BuildRequest struct {
	Target          Target    `json:"target"`
	Priority        float64   `json:"priority"`
	Timestamp       time.Time `json:"timestamp"`
	TriggeringFiles []string  `json:"triggeringFiles"`
	ID              string    `json:"id"`
}

// ParseTarget unmarshals a target from JSON based on its type
func ParseTarget(data []byte) (Target, error) {
	var base struct {
		Type TargetType `json:"type"`
	}

	if err := json.Unmarshal(data, &base); err != nil {
		return nil, fmt.Errorf("failed to parse target type: %w", err)
	}

	switch base.Type {
	case TargetTypeExecutable:
		var t ExecutableTarget
		if err := json.Unmarshal(data, &t); err != nil {
			return nil, err
		}
		return &t, nil

	case TargetTypeAppBundle:
		var t AppBundleTarget
		if err := json.Unmarshal(data, &t); err != nil {
			return nil, err
		}
		return &t, nil

	case TargetTypeLibrary:
		var t LibraryTarget
		if err := json.Unmarshal(data, &t); err != nil {
			return nil, err
		}
		return &t, nil

	case TargetTypeFramework:
		var t FrameworkTarget
		if err := json.Unmarshal(data, &t); err != nil {
			return nil, err
		}
		return &t, nil

	case TargetTypeTest:
		var t TestTarget
		if err := json.Unmarshal(data, &t); err != nil {
			return nil, err
		}
		return &t, nil

	case TargetTypeDocker:
		var t DockerTarget
		if err := json.Unmarshal(data, &t); err != nil {
			return nil, err
		}
		return &t, nil

	case TargetTypeCMakeExecutable:
		var t CMakeExecutableTarget
		if err := json.Unmarshal(data, &t); err != nil {
			return nil, err
		}
		return &t, nil

	case TargetTypeCMakeLibrary:
		var t CMakeLibraryTarget
		if err := json.Unmarshal(data, &t); err != nil {
			return nil, err
		}
		return &t, nil

	case TargetTypeCMakeCustom:
		var t CMakeCustomTarget
		if err := json.Unmarshal(data, &t); err != nil {
			return nil, err
		}
		return &t, nil

	case TargetTypeCustom:
		var t CustomTarget
		if err := json.Unmarshal(data, &t); err != nil {
			return nil, err
		}
		return &t, nil

	default:
		return nil, fmt.Errorf("unknown target type: %s", base.Type)
	}
}

// Target interface implementations for each type

func (t *BaseTarget) GetName() string         { return t.Name }
func (t *BaseTarget) GetType() TargetType     { return t.Type }
func (t *BaseTarget) IsEnabled() bool         { return t.Enabled == nil || *t.Enabled }
func (t *BaseTarget) GetBuildCommand() string { return t.BuildCommand }
func (t *BaseTarget) GetWatchPaths() []string { return t.WatchPaths }
func (t *BaseTarget) GetSettlingDelay() int {
	if t.SettlingDelay != nil {
		return *t.SettlingDelay
	}
	return 1000
}
func (t *BaseTarget) GetEnvironment() map[string]string { return t.Environment }
func (t *BaseTarget) GetMaxRetries() int {
	if t.MaxRetries != nil {
		return *t.MaxRetries
	}
	return 3
}
func (t *BaseTarget) GetBackoffMultiplier() float64 {
	if t.BackoffMultiplier != nil {
		return *t.BackoffMultiplier
	}
	return 2.0
}
func (t *BaseTarget) GetDebounceInterval() int {
	if t.DebounceInterval != nil {
		return *t.DebounceInterval
	}
	return 100
}
func (t *BaseTarget) GetIcon() string { return t.Icon }

// Embed BaseTarget methods in specific target types
func (t *ExecutableTarget) GetName() string                   { return t.BaseTarget.GetName() }
func (t *ExecutableTarget) GetType() TargetType               { return t.BaseTarget.GetType() }
func (t *ExecutableTarget) IsEnabled() bool                   { return t.BaseTarget.IsEnabled() }
func (t *ExecutableTarget) GetBuildCommand() string           { return t.BaseTarget.GetBuildCommand() }
func (t *ExecutableTarget) GetWatchPaths() []string           { return t.BaseTarget.GetWatchPaths() }
func (t *ExecutableTarget) GetSettlingDelay() int             { return t.BaseTarget.GetSettlingDelay() }
func (t *ExecutableTarget) GetEnvironment() map[string]string { return t.BaseTarget.GetEnvironment() }
func (t *ExecutableTarget) GetMaxRetries() int                { return t.BaseTarget.GetMaxRetries() }
func (t *ExecutableTarget) GetBackoffMultiplier() float64     { return t.BaseTarget.GetBackoffMultiplier() }
func (t *ExecutableTarget) GetDebounceInterval() int          { return t.BaseTarget.GetDebounceInterval() }
func (t *ExecutableTarget) GetIcon() string                   { return t.BaseTarget.GetIcon() }

// Repeat for other target types...
// (Implementation is identical, just delegating to BaseTarget)

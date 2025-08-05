// Package interfaces provides abstractions for dependency injection and testability
package interfaces

import (
	"context"
	"time"

	"github.com/poltergeist/poltergeist/pkg/logger"
	"github.com/poltergeist/poltergeist/pkg/state"
	"github.com/poltergeist/poltergeist/pkg/types"
)

// WatchmanClient abstracts file watching operations
type WatchmanClient interface {
	Connect(ctx context.Context) error
	Disconnect() error
	WatchProject(projectPath string) error
	Subscribe(
		root string,
		name string,
		config SubscriptionConfig,
		callback FileChangeCallback,
		exclusions []ExclusionExpression,
	) error
	Unsubscribe(subscriptionName string) error
	IsConnected() bool
}

// SubscriptionConfig represents watchman subscription configuration
type SubscriptionConfig struct {
	Expression []interface{}
	Fields     []string
}

// FileChangeCallback is called when files change
type FileChangeCallback func(files []FileChange)

// FileChange represents a changed file
type FileChange struct {
	Name   string
	Exists bool
	Type   string
}

// ExclusionExpression represents a watchman exclusion pattern
type ExclusionExpression struct {
	Type    string
	Patterns []string
}

// StateManager handles persistent state for targets
type StateManager interface {
	InitializeState(target types.Target) (*state.PoltergeistState, error)
	ReadState(targetName string) (*state.PoltergeistState, error)
	UpdateState(targetName string, updates map[string]interface{}) error
	UpdateBuildStatus(targetName string, status types.BuildStatus) error
	RemoveState(targetName string) error
	IsLocked(targetName string) (bool, error)
	DiscoverStates() (map[string]*state.PoltergeistState, error)
	StartHeartbeat(ctx context.Context)
	StopHeartbeat()
	Cleanup() error
}

// Builder represents a target builder
type Builder interface {
	Validate() error
	Build(ctx context.Context, changedFiles []string) error
	Clean() error
	GetTarget() types.Target
	GetLastBuildTime() time.Duration
	GetSuccessRate() float64
}

// BuilderFactory creates builders for targets
type BuilderFactory interface {
	CreateBuilder(
		target types.Target,
		projectRoot string,
		logger logger.Logger,
		stateManager StateManager,
	) Builder
}

// BuildNotifier handles build notifications
type BuildNotifier interface {
	NotifyBuildStart(target string)
	NotifyBuildSuccess(target string, duration time.Duration)
	NotifyBuildFailure(target string, err error)
	NotifyQueueStatus(active int, queued int)
}

// WatchmanConfigManager manages watchman configuration
type WatchmanConfigManager interface {
	EnsureConfigUpToDate(config *types.PoltergeistConfig) error
	SuggestOptimizations() ([]string, error)
	CreateExclusionExpressions(config *types.PoltergeistConfig) []ExclusionExpression
	NormalizeWatchPattern(pattern string) string
	ValidateWatchPattern(pattern string) error
}

// BuildQueue manages build requests
type BuildQueue interface {
	Enqueue(request *types.BuildRequest) error
	Dequeue() (*types.BuildRequest, error)
	Peek() (*types.BuildRequest, error)
	Size() int
	Clear()
	RegisterTarget(target types.Target, builder Builder)
	OnFileChanged(files []string, targets []types.Target)
	Start(ctx context.Context)
	Stop()
}

// PriorityEngine calculates build priorities
type PriorityEngine interface {
	CalculatePriority(target types.Target, triggeringFiles []string) float64
	UpdateTargetMetrics(target string, buildTime time.Duration, success bool)
	GetTargetPriority(target string) *types.TargetPriority
	RecordFileChange(file string, targets []string)
}

// ProcessManager handles process lifecycle
type ProcessManager interface {
	RegisterShutdownHandler(handler func())
	Start(ctx context.Context)
	Stop()
	IsRunning() bool
}

// ConfigManager handles configuration loading and validation
type ConfigManager interface {
	LoadConfig(path string) (*types.PoltergeistConfig, error)
	ValidateConfig(config *types.PoltergeistConfig) error
	WatchConfig(path string, callback func(*types.PoltergeistConfig)) error
	GetDefaultConfig(projectType types.ProjectType) *types.PoltergeistConfig
}

// FileSystemUtils provides file system operations
type FileSystemUtils interface {
	Exists(path string) bool
	IsDirectory(path string) bool
	CreateDirectory(path string) error
	RemoveDirectory(path string) error
	CopyFile(src, dst string) error
	MoveFile(src, dst string) error
	ReadFile(path string) ([]byte, error)
	WriteFile(path string, data []byte) error
	WalkDirectory(path string, callback func(path string, isDir bool) error) error
}

// DaemonManager manages background daemon processes
type DaemonManager interface {
	Start(config *types.PoltergeistConfig) error
	Stop() error
	Restart() error
	Status() (DaemonStatus, error)
	IsRunning() bool
}

// DaemonStatus represents daemon process status
type DaemonStatus struct {
	Running   bool
	PID       int
	StartTime time.Time
	Targets   []string
	Builds    int
	Errors    int
}

// PoltergeistDependencies contains all injectable dependencies
type PoltergeistDependencies struct {
	WatchmanClient        WatchmanClient
	StateManager          StateManager
	BuilderFactory        BuilderFactory
	Notifier              BuildNotifier
	WatchmanConfigManager WatchmanConfigManager
	ConfigManager         ConfigManager
	FileSystemUtils       FileSystemUtils
	ProcessManager        ProcessManager
	BuildQueue            BuildQueue
	PriorityEngine        PriorityEngine
}
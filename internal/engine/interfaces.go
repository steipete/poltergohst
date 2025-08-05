// Package engine provides core interfaces for the build orchestration system.
// Following Go best practices: "Accept interfaces, return structs" and 
// "Don't design with interfaces, discover them."
package engine

import (
	"context"
	"time"

	"github.com/poltergeist/poltergeist/pkg/logger"
	"github.com/poltergeist/poltergeist/pkg/types"
)

// Builder represents a target builder interface.
// This interface has multiple implementations: XcodeBuilder, CMakeBuilder, CustomBuilder, etc.
// KEEP: Multiple implementations justify the abstraction.
type Builder interface {
	Validate() error
	Build(ctx context.Context, changedFiles []string) error
	Clean() error
	GetTarget() types.Target
	GetLastBuildTime() time.Duration
	GetSuccessRate() float64
}

// BuilderFactory creates builders for targets.
// KEEP: Factory pattern with multiple builder types.
type BuilderFactory interface {
	CreateBuilder(
		target types.Target,
		projectRoot string,
		logger logger.Logger,
	) Builder
}

// Logger represents logging capabilities.
// KEEP: Allows for different logging implementations (structured, file, remote).
type Logger interface {
	Debug(msg string, fields ...logger.Field)
	Info(msg string, fields ...logger.Field)
	Warn(msg string, fields ...logger.Field)
	Error(msg string, fields ...logger.Field)
	InfoContext(ctx context.Context, msg string, fields ...logger.Field)
	ErrorContext(ctx context.Context, msg string, fields ...logger.Field)
}

// The following interfaces were REMOVED as they had single implementations:
// - WatchmanClient: Use concrete watchman.Client type
// - StateManager: Use concrete state.Manager type
// - WatchmanConfigManager: Use concrete watchman.ConfigManager type
// - BuildNotifier: Use concrete notifier.Notifier type
// - ProcessManager: Use concrete process.Manager type
// - FileSystemUtils: Use standard library functions
// - PriorityEngine: Use concrete engine.PriorityEngine type
// - BuildQueue: Use concrete engine.BuildQueue type

// These can be added back when a second implementation is needed,
// following the Go principle of discovering interfaces through use.
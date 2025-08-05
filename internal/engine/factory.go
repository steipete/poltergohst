package engine

import (
	"github.com/poltergeist/poltergeist/pkg/builders"
	"github.com/poltergeist/poltergeist/pkg/interfaces"
	"github.com/poltergeist/poltergeist/pkg/logger"
	"github.com/poltergeist/poltergeist/pkg/queue"
	"github.com/poltergeist/poltergeist/pkg/state"
	"github.com/poltergeist/poltergeist/pkg/types"
	"github.com/poltergeist/poltergeist/pkg/watchman"
)

// DependencyFactory creates default implementations of dependencies.
// This follows the dependency injection pattern and removes hidden
// concrete fallbacks from constructors.
type DependencyFactory struct {
	projectRoot string
	logger      logger.Logger
	config      *types.PoltergeistConfig
}

// NewDependencyFactory creates a new dependency factory
func NewDependencyFactory(projectRoot string, logger logger.Logger, config *types.PoltergeistConfig) *DependencyFactory {
	return &DependencyFactory{
		projectRoot: projectRoot,
		logger:      logger,
		config:      config,
	}
}

// CreateDefaults creates all default dependencies for Poltergeist.
// This centralizes dependency creation and makes it explicit and testable.
func (f *DependencyFactory) CreateDefaults() interfaces.PoltergeistDependencies {
	deps := interfaces.PoltergeistDependencies{
		StateManager:          f.createStateManager(),
		BuilderFactory:        f.createBuilderFactory(),
		WatchmanClient:        f.createWatchmanClient(),
		WatchmanConfigManager: f.createWatchmanConfigManager(),
	}
	
	// Create priority engine and build queue if prioritization is enabled
	if f.config.BuildScheduling != nil && 
	   f.config.BuildScheduling.Prioritization.Enabled {
		deps.PriorityEngine = f.createPriorityEngine()
		deps.BuildQueue = f.createBuildQueue(deps.PriorityEngine)
	}
	
	// Create notifier if notifications are enabled
	if f.config.Notifications != nil && 
	   f.config.Notifications.Enabled != nil && 
	   *f.config.Notifications.Enabled {
		deps.Notifier = f.createNotifier()
	}
	
	return deps
}

// CreateWithOverrides creates dependencies with specific overrides.
// This is useful for testing or custom configurations.
func (f *DependencyFactory) CreateWithOverrides(overrides interfaces.PoltergeistDependencies) interfaces.PoltergeistDependencies {
	deps := f.CreateDefaults()
	
	// Apply overrides (non-nil values replace defaults)
	if overrides.StateManager != nil {
		deps.StateManager = overrides.StateManager
	}
	if overrides.BuilderFactory != nil {
		deps.BuilderFactory = overrides.BuilderFactory
	}
	if overrides.ProcessManager != nil {
		deps.ProcessManager = overrides.ProcessManager
	}
	if overrides.WatchmanClient != nil {
		deps.WatchmanClient = overrides.WatchmanClient
	}
	if overrides.WatchmanConfigManager != nil {
		deps.WatchmanConfigManager = overrides.WatchmanConfigManager
	}
	if overrides.Notifier != nil {
		deps.Notifier = overrides.Notifier
	}
	if overrides.BuildQueue != nil {
		deps.BuildQueue = overrides.BuildQueue
	}
	if overrides.PriorityEngine != nil {
		deps.PriorityEngine = overrides.PriorityEngine
	}
	
	return deps
}

// Individual factory methods for each dependency

func (f *DependencyFactory) createStateManager() interfaces.StateManager {
	return state.NewStateManager(f.projectRoot, f.logger)
}

func (f *DependencyFactory) createBuilderFactory() interfaces.BuilderFactory {
	return builders.NewBuilderFactory()
}

func (f *DependencyFactory) createWatchmanClient() interfaces.WatchmanClient {
	return watchman.NewClient(f.logger)
}

func (f *DependencyFactory) createWatchmanConfigManager() interfaces.WatchmanConfigManager {
	return watchman.NewConfigManager(f.projectRoot, f.logger)
}

func (f *DependencyFactory) createPriorityEngine() interfaces.PriorityEngine {
	if f.config.BuildScheduling == nil {
		return nil
	}
	return queue.NewPriorityEngine(f.config.BuildScheduling, f.logger)
}

func (f *DependencyFactory) createBuildQueue(priorityEngine interfaces.PriorityEngine) interfaces.BuildQueue {
	if f.config.BuildScheduling == nil {
		return nil
	}
	return queue.NewIntelligentBuildQueue(
		f.config.BuildScheduling,
		f.logger,
		priorityEngine,
		f.createNotifier(), // Create notifier if needed
	)
}

func (f *DependencyFactory) createNotifier() interfaces.BuildNotifier {
	// TODO: Implement notifier creation based on config
	// For now, return nil as notifier implementation is not provided
	return nil
}
// Package poltergeist provides the core build orchestration engine
package engine

import (
	"context"
	"fmt"
	"path/filepath"
	"sync"
	"time"

	"github.com/poltergeist/poltergeist/pkg/interfaces"
	"github.com/poltergeist/poltergeist/pkg/logger"
	"github.com/poltergeist/poltergeist/pkg/types"
)

// TargetState tracks the state of a single target
type TargetState struct {
	Target       types.Target
	Builder      interfaces.Builder
	Watching     bool
	LastBuild    types.BuildStatus
	PendingFiles map[string]bool
	BuildTimer   *time.Timer
	mu           sync.Mutex
}

// ConfigChanges represents configuration changes
type ConfigChanges struct {
	TargetsAdded    []types.Target
	TargetsRemoved  []string
	TargetsModified []struct {
		Name      string
		OldTarget types.Target
		NewTarget types.Target
	}
	WatchmanChanged         bool
	NotificationsChanged    bool
	BuildSchedulingChanged  bool
}

// Poltergeist is the main build orchestration engine
type Poltergeist struct {
	config                *types.PoltergeistConfig
	projectRoot           string
	configPath            string
	logger                logger.Logger
	stateManager          interfaces.StateManager
	processManager        interfaces.ProcessManager
	watchman              interfaces.WatchmanClient
	notifier              interfaces.BuildNotifier
	builderFactory        interfaces.BuilderFactory
	watchmanConfigManager interfaces.WatchmanConfigManager
	buildQueue            interfaces.BuildQueue
	priorityEngine        interfaces.PriorityEngine
	
	targetStates          map[string]*TargetState
	buildSchedulingConfig *types.BuildSchedulingConfig
	
	isRunning bool
	ctx       context.Context
	cancel    context.CancelFunc
	wg        sync.WaitGroup
	mu        sync.RWMutex
}

// New creates a new Poltergeist instance
func New(
	config *types.PoltergeistConfig,
	projectRoot string,
	log logger.Logger,
	deps interfaces.PoltergeistDependencies,
	configPath string,
) *Poltergeist {
	ctx, cancel := context.WithCancel(context.Background())
	
	// Ensure project root is absolute
	absProjectRoot, err := filepath.Abs(projectRoot)
	if err != nil {
		log.Error(fmt.Sprintf("Failed to get absolute path for project root: %v", err))
		absProjectRoot = projectRoot // Fall back to provided path
	} else {
		projectRoot = absProjectRoot
	}
	
	// Initialize build scheduling config with defaults
	buildSchedulingConfig := &types.BuildSchedulingConfig{
		Parallelization: 2,
		Prioritization: types.BuildPrioritization{
			Enabled:                true,
			FocusDetectionWindow:   300000,  // 5 minutes
			PriorityDecayTime:      1800000, // 30 minutes
			BuildTimeoutMultiplier: 2.0,
		},
	}
	
	if config.BuildScheduling != nil {
		buildSchedulingConfig = config.BuildScheduling
	}
	
	// Validate required dependencies
	if deps.StateManager == nil {
		panic("StateManager dependency is required")
	}
	if deps.BuilderFactory == nil {
		panic("BuilderFactory dependency is required")
	}
	if deps.WatchmanClient == nil {
		panic("WatchmanClient dependency is required")
	}
	if deps.WatchmanConfigManager == nil {
		panic("WatchmanConfigManager dependency is required")
	}
	
	p := &Poltergeist{
		config:                config,
		projectRoot:           projectRoot,
		configPath:            configPath,
		logger:                log,
		stateManager:          deps.StateManager,
		builderFactory:        deps.BuilderFactory,
		notifier:              deps.Notifier,
		watchman:              deps.WatchmanClient,
		watchmanConfigManager: deps.WatchmanConfigManager,
		processManager:        deps.ProcessManager,
		buildQueue:            deps.BuildQueue,
		priorityEngine:        deps.PriorityEngine,
		targetStates:          make(map[string]*TargetState),
		buildSchedulingConfig: buildSchedulingConfig,
		ctx:                   ctx,
		cancel:                cancel,
	}
	
	return p
}

// StartWithContext begins watching and building targets with the given context.
// This follows Go best practices by accepting context from the caller.
func (p *Poltergeist) StartWithContext(ctx context.Context, targetName string) error {
	p.mu.Lock()
	if p.isRunning {
		p.mu.Unlock()
		return fmt.Errorf("Poltergeist is already running")
	}
	p.isRunning = true
	
	// Replace internal context with the provided one
	p.ctx, p.cancel = context.WithCancel(ctx)
	p.mu.Unlock()
	
	return p.start(targetName)
}

// Start begins watching and building targets (deprecated - use StartWithContext)
func (p *Poltergeist) Start(targetName string) error {
	return p.StartWithContext(context.Background(), targetName)
}

// start is the internal implementation
func (p *Poltergeist) start(targetName string) error {
	
	p.logger.Info("Starting Poltergeist...")
	
	// Start heartbeat
	p.stateManager.StartHeartbeat(p.ctx)
	
	// Setup Watchman configuration
	if err := p.setupWatchmanConfig(); err != nil {
		return fmt.Errorf("failed to setup watchman config: %w", err)
	}
	
	// Initialize notifier if enabled
	if p.config.Notifications != nil && p.config.Notifications.Enabled != nil && *p.config.Notifications.Enabled {
		// Notifier should be initialized
	}
	
	// Initialize build queue
	if p.buildQueue != nil {
		p.buildQueue.Start(p.ctx)
	}
	
	// Determine targets to watch
	targetsToWatch := p.getTargetsToWatch(targetName)
	if len(targetsToWatch) == 0 {
		return fmt.Errorf("no targets to watch")
	}
	
	p.logger.Info(fmt.Sprintf("Building %d enabled target(s)", len(targetsToWatch)))
	
	// Initialize target states
	for _, target := range targetsToWatch {
		builder := p.builderFactory.CreateBuilder(
			target,
			p.projectRoot,
			p.logger,
			p.stateManager,
		)
		
		if err := builder.Validate(); err != nil {
			return fmt.Errorf("target validation failed for %s: %w", target.GetName(), err)
		}
		
		targetState := &TargetState{
			Target:       target,
			Builder:      builder,
			Watching:     false,
			PendingFiles: make(map[string]bool),
		}
		
		p.mu.Lock()
		p.targetStates[target.GetName()] = targetState
		p.mu.Unlock()
		
		// Register with build queue
		if p.buildQueue != nil {
			p.buildQueue.RegisterTarget(target, builder)
		}
		
		// Initialize state file
		if _, err := p.stateManager.InitializeState(target); err != nil {
			p.logger.Warn(fmt.Sprintf("Failed to initialize state for %s", target.GetName()),
				logger.WithField("error", err))
		}
	}
	
	// Connect to Watchman
	if err := p.watchman.Connect(p.ctx); err != nil {
		return fmt.Errorf("failed to connect to watchman: %w", err)
	}
	
	// Watch the project
	if err := p.watchman.WatchProject(p.projectRoot); err != nil {
		return fmt.Errorf("failed to watch project: %w", err)
	}
	
	// Subscribe to file changes
	if err := p.subscribeToChanges(); err != nil {
		return fmt.Errorf("failed to subscribe to changes: %w", err)
	}
	
	// Perform initial builds
	if err := p.performInitialBuilds(); err != nil {
		p.logger.Warn("Initial builds encountered errors", logger.WithField("error", err))
	}
	
	p.logger.Info("Poltergeist is now watching for changes...")
	
	// Register shutdown handlers
	if p.processManager != nil {
		p.processManager.RegisterShutdownHandler(func() {
			p.Stop()
			p.Cleanup()
		})
		p.processManager.Start(p.ctx)
	}
	
	return nil
}

// StopWithContext stops the Poltergeist engine with the given context for timeout control.
// This follows Go best practices for graceful shutdown.
func (p *Poltergeist) StopWithContext(ctx context.Context) {
	p.mu.Lock()
	if !p.isRunning {
		p.mu.Unlock()
		return
	}
	p.isRunning = false
	p.mu.Unlock()
	
	p.logger.Info("Stopping Poltergeist...")
	
	// Cancel internal context to signal shutdown
	p.cancel()
	
	// Create a channel to signal completion
	done := make(chan struct{})
	
	go func() {
		// Stop build queue
		if p.buildQueue != nil {
			p.buildQueue.Stop()
		}
		
		// Stop heartbeat
		p.stateManager.StopHeartbeat()
		
		// Disconnect from Watchman
		if p.watchman != nil && p.watchman.IsConnected() {
			if err := p.watchman.Disconnect(); err != nil {
				p.logger.Warn("Failed to disconnect from watchman", logger.WithField("error", err))
			}
		}
		
		// Wait for all goroutines
		p.wg.Wait()
		
		close(done)
	}()
	
	// Wait for shutdown or context timeout
	select {
	case <-done:
		p.logger.Info("Poltergeist stopped gracefully")
	case <-ctx.Done():
		p.logger.Warn("Poltergeist shutdown timed out", logger.WithField("error", ctx.Err()))
	}
}

// Stop stops the Poltergeist engine (deprecated - use StopWithContext)
func (p *Poltergeist) Stop() {
	// Use a 30-second timeout for backward compatibility
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	p.StopWithContext(ctx)
}

// Cleanup performs cleanup operations
func (p *Poltergeist) Cleanup() error {
	return p.stateManager.Cleanup()
}

// Private methods

func (p *Poltergeist) getTargetsToWatch(targetName string) []types.Target {
	var targets []types.Target
	
	if targetName != "" {
		// Find specific target
		for _, rawTarget := range p.config.Targets {
			target, err := types.ParseTarget(rawTarget)
			if err != nil {
				p.logger.Warn("Failed to parse target", logger.WithField("error", err))
				continue
			}
			
			if target.GetName() == targetName {
				if target.IsEnabled() {
					targets = append(targets, target)
				}
				break
			}
		}
	} else {
		// Get all enabled targets
		for _, rawTarget := range p.config.Targets {
			target, err := types.ParseTarget(rawTarget)
			if err != nil {
				p.logger.Warn("Failed to parse target", logger.WithField("error", err))
				continue
			}
			
			if target.IsEnabled() {
				targets = append(targets, target)
			}
		}
	}
	
	return targets
}

func (p *Poltergeist) setupWatchmanConfig() error {
	p.logger.Info("Setting up Watchman configuration...")
	
	if err := p.watchmanConfigManager.EnsureConfigUpToDate(p.config); err != nil {
		return err
	}
	
	// Suggest optimizations
	suggestions, err := p.watchmanConfigManager.SuggestOptimizations()
	if err == nil && len(suggestions) > 0 {
		p.logger.Info("Optimization suggestions:")
		for _, s := range suggestions {
			p.logger.Info(fmt.Sprintf("  • %s", s))
		}
	}
	
	return nil
}

func (p *Poltergeist) subscribeToChanges() error {
	// Group targets by watch paths
	pathToTargets := make(map[string][]string)
	
	p.mu.RLock()
	for name, state := range p.targetStates {
		for _, pattern := range state.Target.GetWatchPaths() {
			pathToTargets[pattern] = append(pathToTargets[pattern], name)
		}
	}
	p.mu.RUnlock()
	
	// Create subscriptions
	exclusions := p.watchmanConfigManager.CreateExclusionExpressions(p.config)
	
	for pattern, targetNames := range pathToTargets {
		// Normalize and validate pattern
		normalizedPattern := p.watchmanConfigManager.NormalizeWatchPattern(pattern)
		if err := p.watchmanConfigManager.ValidateWatchPattern(normalizedPattern); err != nil {
			return fmt.Errorf("invalid watch pattern %s: %w", pattern, err)
		}
		
		subscriptionName := fmt.Sprintf("poltergeist_%s", normalizedPattern)
		
		// Create subscription
		err := p.watchman.Subscribe(
			p.projectRoot,
			subscriptionName,
			interfaces.SubscriptionConfig{
				Expression: []interface{}{"match", normalizedPattern, "wholename"},
				Fields:     []string{"name", "exists", "type"},
			},
			func(files []interfaces.FileChange) {
				p.handleFileChanges(files, targetNames)
			},
			exclusions,
		)
		
		if err != nil {
			return fmt.Errorf("failed to subscribe to %s: %w", pattern, err)
		}
		
		p.logger.Info(fmt.Sprintf("Watching %d target(s): %s", len(targetNames), normalizedPattern))
	}
	
	// Subscribe to config file changes
	if p.configPath != "" {
		configName := filepath.Base(p.configPath)
		err := p.watchman.Subscribe(
			p.projectRoot,
			"poltergeist_config",
			interfaces.SubscriptionConfig{
				Expression: []interface{}{"match", configName, "wholename"},
				Fields:     []string{"name", "exists", "type"},
			},
			p.handleConfigChange,
			nil,
		)
		
		if err != nil {
			p.logger.Warn("Failed to watch config file", logger.WithField("error", err))
		} else {
			p.logger.Info("Watching configuration file for changes")
		}
	}
	
	return nil
}

func (p *Poltergeist) handleFileChanges(files []interfaces.FileChange, targetNames []string) {
	// Pre-allocate with expected capacity for better performance
	changedFiles := make([]string, 0, len(files))
	for _, f := range files {
		if f.Exists {
			changedFiles = append(changedFiles, f.Name)
		}
	}
	
	if len(changedFiles) == 0 {
		return
	}
	
	p.logger.Debug(fmt.Sprintf("Files changed: %v", changedFiles))
	
	// Use intelligent build queue if available
	if p.buildQueue != nil && p.buildSchedulingConfig.Prioritization.Enabled {
		// Pre-allocate with known capacity
		targets := make([]types.Target, 0, len(targetNames))
		p.mu.RLock()
		for _, name := range targetNames {
			if state, ok := p.targetStates[name]; ok {
				targets = append(targets, state.Target)
			}
		}
		p.mu.RUnlock()
		
		p.buildQueue.OnFileChanged(changedFiles, targets)
		return
	}
	
	// Fallback to immediate builds
	for _, targetName := range targetNames {
		p.mu.RLock()
		state, ok := p.targetStates[targetName]
		p.mu.RUnlock()
		
		if !ok {
			continue
		}
		
		state.mu.Lock()
		for _, file := range changedFiles {
			state.PendingFiles[file] = true
		}
		
		// Clear existing timer
		if state.BuildTimer != nil {
			state.BuildTimer.Stop()
		}
		
		// Set new timer with settling delay
		delay := time.Duration(state.Target.GetSettlingDelay()) * time.Millisecond
		state.BuildTimer = time.AfterFunc(delay, func() {
			p.buildTarget(targetName)
		})
		state.mu.Unlock()
	}
}

func (p *Poltergeist) handleConfigChange(files []interfaces.FileChange) {
	if len(files) == 0 {
		return
	}
	
	p.logger.Info("Configuration file changed, reloading...")
	
	// TODO: Implement config reload logic
}

func (p *Poltergeist) performInitialBuilds() error {
	// Use intelligent build queue if available
	if p.buildQueue != nil && p.buildSchedulingConfig.Prioritization.Enabled {
		// Pre-allocate with known capacity
		p.mu.RLock()
		targets := make([]types.Target, 0, len(p.targetStates))
		for _, state := range p.targetStates {
			targets = append(targets, state.Target)
		}
		p.mu.RUnlock()
		
		p.buildQueue.OnFileChanged([]string{"initial build"}, targets)
		return nil
	}
	
	// Use SafeGroup for concurrent builds with proper error handling and panic recovery
	g, ctx := NewSafeGroup(p.ctx, p.logger)
	
	// Set reasonable concurrency limit to prevent resource exhaustion
	parallelism := p.buildSchedulingConfig.Parallelization
	if parallelism <= 0 {
		parallelism = 2 // Default safe parallelism
	}
	g.SetLimit(parallelism)
	
	p.mu.RLock()
	for name := range p.targetStates {
		name := name // Capture loop variable (Go best practice)
		g.Go(func() error {
			// Check context cancellation before building
			select {
			case <-ctx.Done():
				return ctx.Err()
			default:
			}
			
			if err := p.buildTarget(name); err != nil {
				return fmt.Errorf("%s: %w", name, err)
			}
			return nil
		})
	}
	p.mu.RUnlock()
	
	// Wait returns the first error encountered, cancelling all other operations
	if err := g.Wait(); err != nil {
		return fmt.Errorf("initial builds failed: %w", err)
	}
	
	return nil
}

func (p *Poltergeist) buildTarget(targetName string) error {
	p.mu.RLock()
	state, ok := p.targetStates[targetName]
	p.mu.RUnlock()
	
	if !ok {
		return fmt.Errorf("target not found: %s", targetName)
	}
	
	state.mu.Lock()
	changedFiles := make([]string, 0, len(state.PendingFiles))
	for file := range state.PendingFiles {
		changedFiles = append(changedFiles, file)
	}
	state.PendingFiles = make(map[string]bool)
	state.mu.Unlock()
	
	// Update build status
	if err := p.stateManager.UpdateBuildStatus(targetName, types.BuildStatusBuilding); err != nil {
		p.logger.Warn("Failed to update build status", logger.WithField("error", err))
	}
	
	// Notify build start
	if p.notifier != nil {
		p.notifier.NotifyBuildStart(targetName)
	}
	
	// Perform build
	startTime := time.Now()
	err := state.Builder.Build(p.ctx, changedFiles)
	duration := time.Since(startTime)
	
	// Update build status and notify
	if err != nil {
		p.stateManager.UpdateBuildStatus(targetName, types.BuildStatusFailed)
		if p.notifier != nil {
			p.notifier.NotifyBuildFailure(targetName, err)
		}
		return err
	}
	
	p.stateManager.UpdateBuildStatus(targetName, types.BuildStatusSucceeded)
	if p.notifier != nil {
		p.notifier.NotifyBuildSuccess(targetName, duration)
	}
	
	// Update priority engine metrics
	if p.priorityEngine != nil {
		p.priorityEngine.UpdateTargetMetrics(targetName, duration, true)
	}
	
	return nil
}
// Package queue provides intelligent build queue management
package engine

import (
	"context"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/poltergeist/poltergeist/pkg/interfaces"
	"github.com/poltergeist/poltergeist/pkg/logger"
	"github.com/poltergeist/poltergeist/pkg/types"
)

// IntelligentBuildQueue manages prioritized build requests
type IntelligentBuildQueue struct {
	config         *types.BuildSchedulingConfig
	logger         logger.Logger
	priorityEngine interfaces.PriorityEngine
	notifier       interfaces.BuildNotifier
	
	queue          []*types.BuildRequest
	targets        map[string]types.Target
	builders       map[string]interfaces.Builder
	activeBuilds   map[string]*types.BuildRequest
	
	mu             sync.RWMutex
	ctx            context.Context
	cancel         context.CancelFunc
	wg             sync.WaitGroup
}

// NewIntelligentBuildQueue creates a new intelligent build queue
func NewIntelligentBuildQueue(
	config *types.BuildSchedulingConfig,
	log logger.Logger,
	priorityEngine interfaces.PriorityEngine,
	notifier interfaces.BuildNotifier,
) *IntelligentBuildQueue {
	ctx, cancel := context.WithCancel(context.Background())
	
	return &IntelligentBuildQueue{
		config:         config,
		logger:         log,
		priorityEngine: priorityEngine,
		notifier:       notifier,
		targets:        make(map[string]types.Target),
		builders:       make(map[string]interfaces.Builder),
		activeBuilds:   make(map[string]*types.BuildRequest),
		ctx:            ctx,
		cancel:         cancel,
	}
}

// RegisterTarget registers a target with its builder
func (q *IntelligentBuildQueue) RegisterTarget(target types.Target, builder interfaces.Builder) {
	q.mu.Lock()
	defer q.mu.Unlock()
	
	q.targets[target.GetName()] = target
	q.builders[target.GetName()] = builder
}

// OnFileChanged handles file change events
func (q *IntelligentBuildQueue) OnFileChanged(files []string, targets []types.Target) {
	q.mu.Lock()
	defer q.mu.Unlock()
	
	// Create build requests for affected targets
	for _, target := range targets {
		// Check if already queued or building
		if q.isTargetPending(target.GetName()) {
			continue
		}
		
		// Calculate priority
		priority := 50.0
		if q.priorityEngine != nil {
			priority = q.priorityEngine.CalculatePriority(target, files)
		}
		
		// Create build request
		request := &types.BuildRequest{
			Target:          target,
			Priority:        float64(priority),
			Timestamp:       time.Now(),
			TriggeringFiles: files,
			ID:              uuid.New().String(),
		}
		
		// Add to queue
		q.queue = append(q.queue, request)
		if q.logger != nil {
			q.logger.Debug("Queued build request",
				logger.WithField("target", target.GetName()),
				logger.WithField("priority", priority))
		}
	}
	
	// Sort queue by priority
	q.sortQueue()
	
	// Update notifier
	if q.notifier != nil {
		q.notifier.NotifyQueueStatus(len(q.activeBuilds), len(q.queue))
	}
}

// Start starts the build queue processor
func (q *IntelligentBuildQueue) Start(ctx context.Context) {
	q.wg.Add(1)
	go q.processQueue()
}

// Stop stops the build queue
func (q *IntelligentBuildQueue) Stop() {
	q.cancel()
	q.wg.Wait()
}

// Enqueue adds a build request to the queue
func (q *IntelligentBuildQueue) Enqueue(request *types.BuildRequest) error {
	q.mu.Lock()
	defer q.mu.Unlock()
	
	q.queue = append(q.queue, request)
	q.sortQueue()
	
	return nil
}

// Dequeue removes and returns the highest priority request
func (q *IntelligentBuildQueue) Dequeue() (*types.BuildRequest, error) {
	q.mu.Lock()
	defer q.mu.Unlock()
	
	if len(q.queue) == 0 {
		return nil, nil
	}
	
	request := q.queue[0]
	q.queue = q.queue[1:]
	
	return request, nil
}

// Peek returns the highest priority request without removing it
func (q *IntelligentBuildQueue) Peek() (*types.BuildRequest, error) {
	q.mu.RLock()
	defer q.mu.RUnlock()
	
	if len(q.queue) == 0 {
		return nil, nil
	}
	
	return q.queue[0], nil
}

// Size returns the queue size
func (q *IntelligentBuildQueue) Size() int {
	q.mu.RLock()
	defer q.mu.RUnlock()
	return len(q.queue)
}

// Clear clears the queue
func (q *IntelligentBuildQueue) Clear() {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.queue = nil
}

// Private methods

func (q *IntelligentBuildQueue) processQueue() {
	defer q.wg.Done()
	
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()
	
	for {
		select {
		case <-q.ctx.Done():
			return
		case <-ticker.C:
			q.processNextBuild()
		}
	}
}

func (q *IntelligentBuildQueue) processNextBuild() {
	q.mu.Lock()
	
	// Check if we can start more builds
	if len(q.activeBuilds) >= q.config.Parallelization {
		q.mu.Unlock()
		return
	}
	
	// Get next build request
	if len(q.queue) == 0 {
		q.mu.Unlock()
		return
	}
	
	request := q.queue[0]
	q.queue = q.queue[1:]
	
	// Mark as active
	q.activeBuilds[request.Target.GetName()] = request
	
	// Get builder
	builder := q.builders[request.Target.GetName()]
	
	q.mu.Unlock()
	
	// Start build in goroutine
	q.wg.Add(1)
	go func() {
		defer q.wg.Done()
		q.executeBuild(request, builder)
	}()
}

func (q *IntelligentBuildQueue) executeBuild(request *types.BuildRequest, builder interfaces.Builder) {
	startTime := time.Now()
	
	// Notify build start
	if q.notifier != nil {
		q.notifier.NotifyBuildStart(request.Target.GetName())
	}
	
	// Execute build
	err := builder.Build(q.ctx, request.TriggeringFiles)
	duration := time.Since(startTime)
	
	// Update metrics
	if q.priorityEngine != nil {
		q.priorityEngine.UpdateTargetMetrics(request.Target.GetName(), duration, err == nil)
	}
	
	// Notify completion
	if q.notifier != nil {
		if err != nil {
			q.notifier.NotifyBuildFailure(request.Target.GetName(), err)
		} else {
			q.notifier.NotifyBuildSuccess(request.Target.GetName(), duration)
		}
	}
	
	// Remove from active builds
	q.mu.Lock()
	delete(q.activeBuilds, request.Target.GetName())
	q.mu.Unlock()
	
	// Update queue status
	if q.notifier != nil {
		q.mu.RLock()
		active := len(q.activeBuilds)
		queued := len(q.queue)
		q.mu.RUnlock()
		q.notifier.NotifyQueueStatus(active, queued)
	}
}

func (q *IntelligentBuildQueue) isTargetPending(targetName string) bool {
	// Check if in active builds
	if _, ok := q.activeBuilds[targetName]; ok {
		return true
	}
	
	// Check if in queue
	for _, req := range q.queue {
		if req.Target.GetName() == targetName {
			return true
		}
	}
	
	return false
}

func (q *IntelligentBuildQueue) sortQueue() {
	// Simple priority sort - higher priority first
	for i := 0; i < len(q.queue)-1; i++ {
		for j := i + 1; j < len(q.queue); j++ {
			if q.queue[j].Priority > q.queue[i].Priority {
				q.queue[i], q.queue[j] = q.queue[j], q.queue[i]
			}
		}
	}
}

// PriorityEngine manages build priorities (merged from pkg/queue/priority.go)

// PriorityEngine calculates build priorities
type PriorityEngine struct {
	config        *types.BuildSchedulingConfig
	logger        logger.Logger
	targetMetrics map[string]*targetMetrics
	fileChanges   map[string][]fileChangeRecord
	mu            sync.RWMutex
}

type targetMetrics struct {
	lastBuildTime     time.Duration
	totalBuilds       int
	successfulBuilds  int
	lastDirectChange  time.Time
	changeFrequency   float64
	recentChanges     []types.ChangeEvent
}

type fileChangeRecord struct {
	timestamp time.Time
	targets   []string
}

// NewPriorityEngine creates a new priority engine
func NewPriorityEngine(config *types.BuildSchedulingConfig, log logger.Logger) *PriorityEngine {
	return &PriorityEngine{
		config:        config,
		logger:        log,
		targetMetrics: make(map[string]*targetMetrics),
		fileChanges:   make(map[string][]fileChangeRecord),
	}
}

// CalculatePriority calculates priority for a build request
func (e *PriorityEngine) CalculatePriority(target types.Target, triggeringFiles []string) float64 {
	e.mu.RLock()
	defer e.mu.RUnlock()
	
	basePriority := 50.0
	
	// Get target metrics
	metrics, exists := e.targetMetrics[target.GetName()]
	if !exists {
		// New target gets moderate priority
		return basePriority
	}
	
	// Factor 1: Recent direct changes (higher priority)
	if time.Since(metrics.lastDirectChange) < time.Duration(e.config.Prioritization.FocusDetectionWindow)*time.Millisecond {
		basePriority += 30.0
	}
	
	// Factor 2: Change frequency (more frequent = higher priority)
	basePriority += metrics.changeFrequency * 10.0
	
	// Factor 3: Success rate (lower success = lower priority)
	if metrics.totalBuilds > 0 {
		successRate := float64(metrics.successfulBuilds) / float64(metrics.totalBuilds)
		// Only apply success rate if we have build history
		basePriority *= (0.5 + successRate*0.5) // Scale between 0.5x and 1x
	}
	
	// Factor 4: Build time (faster builds get slight priority)
	if metrics.lastBuildTime < 5*time.Second {
		basePriority += 10.0
	} else if metrics.lastBuildTime > 30*time.Second {
		basePriority -= 10.0
	}
	
	// Factor 5: Time decay (older requests lose priority)
	for _, change := range metrics.recentChanges {
		age := time.Since(change.Timestamp)
		decayTime := time.Duration(e.config.Prioritization.PriorityDecayTime) * time.Millisecond
		if age < decayTime {
			decayFactor := 1.0 - (float64(age) / float64(decayTime))
			basePriority += decayFactor * 5.0
		}
	}
	
	// Clamp to reasonable range
	if basePriority < 0 {
		basePriority = 0
	} else if basePriority > 100 {
		basePriority = 100
	}
	
	return basePriority
}

// UpdateTargetMetrics updates metrics after a build
func (e *PriorityEngine) UpdateTargetMetrics(target string, buildTime time.Duration, success bool) {
	e.mu.Lock()
	defer e.mu.Unlock()
	
	metrics, exists := e.targetMetrics[target]
	if !exists {
		metrics = &targetMetrics{
			recentChanges: make([]types.ChangeEvent, 0),
		}
		e.targetMetrics[target] = metrics
	}
	
	metrics.lastBuildTime = buildTime
	metrics.totalBuilds++
	if success {
		metrics.successfulBuilds++
	}
	
	// Update change frequency
	e.updateChangeFrequency(metrics)
}

// GetTargetPriority returns priority information for a target
func (e *PriorityEngine) GetTargetPriority(target string) *types.TargetPriority {
	e.mu.RLock()
	defer e.mu.RUnlock()
	
	metrics, exists := e.targetMetrics[target]
	if !exists {
		return nil
	}
	
	successRate := float64(metrics.successfulBuilds) / float64(max(metrics.totalBuilds, 1))
	
	// Calculate priority score based on metrics
	score := 50.0 // Base priority
	
	// Factor 1: Recent direct changes (higher priority)
	if e.config != nil && e.config.Prioritization.Enabled {
		if time.Since(metrics.lastDirectChange) < time.Duration(e.config.Prioritization.FocusDetectionWindow)*time.Millisecond {
			score += 30.0
		}
		
		// Factor 2: Change frequency (more frequent = higher priority)
		score += metrics.changeFrequency * 10.0
		
		// Factor 3: Build time (longer builds = slightly higher priority)
		buildTimeSeconds := metrics.lastBuildTime.Seconds()
		score += math.Min(buildTimeSeconds/10.0, 10.0)
		
		// Factor 4: Success rate (lower success = higher priority for fixing)
		score += (1.0 - successRate) * 20.0
	}
	
	return &types.TargetPriority{
		Target:                target,
		Score:                 score,
		LastDirectChange:      metrics.lastDirectChange,
		DirectChangeFrequency: metrics.changeFrequency,
		FocusMultiplier:       1.0,
		AvgBuildTime:          metrics.lastBuildTime,
		SuccessRate:           successRate,
		RecentChanges:         metrics.recentChanges,
	}
}

// RecordFileChange records a file change event
func (e *PriorityEngine) RecordFileChange(file string, targets []string) {
	e.mu.Lock()
	defer e.mu.Unlock()
	
	// Record file change
	record := fileChangeRecord{
		timestamp: time.Now(),
		targets:   targets,
	}
	
	e.fileChanges[file] = append(e.fileChanges[file], record)
	
	// Update target metrics
	for _, target := range targets {
		metrics, exists := e.targetMetrics[target]
		if !exists {
			metrics = &targetMetrics{
				recentChanges: make([]types.ChangeEvent, 0),
			}
			e.targetMetrics[target] = metrics
		}
		
		metrics.lastDirectChange = time.Now()
		
		// Add to recent changes
		event := types.ChangeEvent{
			File:            file,
			Timestamp:       time.Now(),
			AffectedTargets: targets,
			ChangeType:      types.ChangeTypeDirect,
			ImpactWeight:    1.0,
		}
		
		metrics.recentChanges = append(metrics.recentChanges, event)
		
		// Keep only recent changes
		if len(metrics.recentChanges) > 100 {
			metrics.recentChanges = metrics.recentChanges[1:]
		}
		
		// Update change frequency
		e.updateChangeFrequency(metrics)
	}
}

// Private methods

func (e *PriorityEngine) updateChangeFrequency(metrics *targetMetrics) {
	// Calculate change frequency based on recent changes
	if len(metrics.recentChanges) < 2 {
		metrics.changeFrequency = 0
		return
	}
	
	// Calculate average time between changes
	totalDuration := time.Duration(0)
	for i := 1; i < len(metrics.recentChanges); i++ {
		duration := metrics.recentChanges[i].Timestamp.Sub(metrics.recentChanges[i-1].Timestamp)
		totalDuration += duration
	}
	
	avgDuration := totalDuration / time.Duration(len(metrics.recentChanges)-1)
	
	// Convert to frequency (changes per minute)
	if avgDuration > 0 {
		metrics.changeFrequency = float64(time.Minute) / float64(avgDuration)
	} else {
		metrics.changeFrequency = 0
	}
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
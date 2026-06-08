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

	queue        []*types.BuildRequest
	targets      map[string]types.Target
	builders     map[string]interfaces.Builder
	activeBuilds map[string]*types.BuildRequest

	mu     sync.RWMutex
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
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

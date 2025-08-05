package queue_test

import (
	"context"
	"fmt"
	"sort"
	"sync"
	"testing"
	"time"

	"github.com/poltergeist/poltergeist/pkg/queue"
	"github.com/poltergeist/poltergeist/pkg/types"
)

// Test intelligent build ordering with priority engine
func TestIntelligentBuildOrdering_PriorityBased(t *testing.T) {
	config := &types.BuildSchedulingConfig{
		Parallelization: 1,
		Prioritization: types.BuildPrioritization{
			Enabled: true,
		},
	}

	// Create priority engine that returns different priorities
	priorityEngine := &dynamicPriorityEngine{
		priorities: map[string]float64{
			"critical": 100.0,
			"high":     80.0,
			"medium":   50.0,
			"low":      20.0,
		},
	}

	q := queue.NewIntelligentBuildQueue(config, nil, priorityEngine, nil)

	// Create targets with different priorities
	targets := []types.Target{
		&mockTarget{name: "low"},
		&mockTarget{name: "high"},
		&mockTarget{name: "critical"},
		&mockTarget{name: "medium"},
	}

	// Register targets
	for _, target := range targets {
		q.RegisterTarget(target, &mockBuilder{target: target})
	}

	// Trigger file changes
	q.OnFileChanged([]string{"test.go"}, targets)

	// Verify ordering - should be critical, high, medium, low
	expectedOrder := []string{"critical", "high", "medium", "low"}
	actualOrder := make([]string, 0, len(targets))

	for i := 0; i < len(targets); i++ {
		req, err := q.Dequeue()
		if err != nil {
			t.Fatalf("Failed to dequeue: %v", err)
		}
		if req == nil {
			t.Fatal("Got nil request")
		}
		actualOrder = append(actualOrder, req.Target.GetName())
	}

	for i, expected := range expectedOrder {
		if actualOrder[i] != expected {
			t.Errorf("Order mismatch at position %d: expected %s, got %s", i, expected, actualOrder[i])
		}
	}
}

func TestIntelligentBuildOrdering_TimeBasedPriority(t *testing.T) {
	config := &types.BuildSchedulingConfig{
		Parallelization: 2,
		Prioritization: types.BuildPrioritization{
			Enabled:              true,
			PriorityDecayTime:    1000, // 1 second
			FocusDetectionWindow: 5000, // 5 seconds
		},
	}

	priorityEngine := &timeBasedPriorityEngine{}
	q := queue.NewIntelligentBuildQueue(config, nil, priorityEngine, nil)

	// Create targets
	target1 := &mockTarget{name: "target1"}
	target2 := &mockTarget{name: "target2"}

	q.RegisterTarget(target1, &mockBuilder{target: target1})
	q.RegisterTarget(target2, &mockBuilder{target: target2})

	// First change to target1
	q.OnFileChanged([]string{"file1.go"}, []types.Target{target1})
	time.Sleep(100 * time.Millisecond)

	// Second change to target2 (should have higher priority due to recency)
	q.OnFileChanged([]string{"file2.go"}, []types.Target{target2})

	// target2 should be dequeued first due to more recent change
	req1, _ := q.Dequeue()
	if req1.Target.GetName() != "target2" {
		t.Errorf("Expected target2 first, got %s", req1.Target.GetName())
	}

	req2, _ := q.Dequeue()
	if req2.Target.GetName() != "target1" {
		t.Errorf("Expected target1 second, got %s", req2.Target.GetName())
	}
}

func TestIntelligentBuildOrdering_FileChangeFrequency(t *testing.T) {
	config := &types.BuildSchedulingConfig{
		Parallelization: 1,
		Prioritization: types.BuildPrioritization{
			Enabled: true,
		},
	}

	priorityEngine := &frequencyBasedPriorityEngine{
		changeCount: make(map[string]int),
	}
	q := queue.NewIntelligentBuildQueue(config, nil, priorityEngine, nil)

	target1 := &mockTarget{name: "frequent"}
	target2 := &mockTarget{name: "rare"}

	q.RegisterTarget(target1, &mockBuilder{target: target1})
	q.RegisterTarget(target2, &mockBuilder{target: target2})

	// Trigger multiple changes for target1
	for i := 0; i < 5; i++ {
		q.OnFileChanged([]string{"file.go"}, []types.Target{target1})
		q.Clear() // Clear queue to avoid accumulation
		time.Sleep(10 * time.Millisecond)
	}

	// Single change for target2
	q.OnFileChanged([]string{"file.go"}, []types.Target{target2})

	// Now trigger changes for both - frequent target should have higher priority
	q.OnFileChanged([]string{"file.go"}, []types.Target{target1, target2})

	req, _ := q.Dequeue()
	if req.Target.GetName() != "frequent" {
		t.Errorf("Expected frequent target first, got %s", req.Target.GetName())
	}
}

func TestIntelligentBuildOrdering_BuildSuccessRate(t *testing.T) {
	config := &types.BuildSchedulingConfig{
		Parallelization: 1,
		Prioritization: types.BuildPrioritization{
			Enabled: true,
		},
	}

	priorityEngine := &successRateBasedPriorityEngine{
		successRates: map[string]float64{
			"reliable": 0.95,
			"flaky":    0.30,
		},
	}
	q := queue.NewIntelligentBuildQueue(config, nil, priorityEngine, nil)

	target1 := &mockTarget{name: "reliable"}
	target2 := &mockTarget{name: "flaky"}

	q.RegisterTarget(target1, &mockBuilder{target: target1})
	q.RegisterTarget(target2, &mockBuilder{target: target2})

	// Trigger changes for both targets
	q.OnFileChanged([]string{"file.go"}, []types.Target{target1, target2})

	// Reliable target should be prioritized
	req, _ := q.Dequeue()
	if req.Target.GetName() != "reliable" {
		t.Errorf("Expected reliable target first, got %s", req.Target.GetName())
	}
}

func TestIntelligentBuildOrdering_BuildTimeConsideration(t *testing.T) {
	config := &types.BuildSchedulingConfig{
		Parallelization: 2,
		Prioritization: types.BuildPrioritization{
			Enabled:                true,
			BuildTimeoutMultiplier: 2.0,
		},
	}

	priorityEngine := &buildTimeBasedPriorityEngine{
		buildTimes: map[string]time.Duration{
			"fast": 1 * time.Second,
			"slow": 30 * time.Second,
		},
	}
	q := queue.NewIntelligentBuildQueue(config, nil, priorityEngine, nil)

	target1 := &mockTarget{name: "fast"}
	target2 := &mockTarget{name: "slow"}

	q.RegisterTarget(target1, &mockBuilder{target: target1})
	q.RegisterTarget(target2, &mockBuilder{target: target2})

	// Trigger changes
	q.OnFileChanged([]string{"file.go"}, []types.Target{target1, target2})

	// Fast target should be prioritized for quicker feedback
	req, _ := q.Dequeue()
	if req.Target.GetName() != "fast" {
		t.Errorf("Expected fast target first, got %s", req.Target.GetName())
	}
}

func TestIntelligentBuildOrdering_ParallelExecution(t *testing.T) {
	config := &types.BuildSchedulingConfig{
		Parallelization: 3,
		Prioritization: types.BuildPrioritization{
			Enabled: true,
		},
	}

	notifier := &concurrentNotifier{}
	q := queue.NewIntelligentBuildQueue(config, nil, &mockPriorityEngine{}, notifier)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	q.Start(ctx)
	defer q.Stop()

	var wg sync.WaitGroup
	buildStartTimes := make(map[string]time.Time)
	mu := sync.Mutex{}

	// Create targets with different execution times
	targets := []string{"target1", "target2", "target3"}
	for _, name := range targets {
		target := &mockTarget{name: name}
		builder := &mockBuilder{
			target: target,
			buildFunc: func(ctx context.Context, files []string) error {
				mu.Lock()
				buildStartTimes[name] = time.Now()
				mu.Unlock()
				time.Sleep(200 * time.Millisecond) // Simulate build time
				wg.Done()
				return nil
			},
		}
		q.RegisterTarget(target, builder)
	}

	// Trigger builds
	wg.Add(3)
	for _, name := range targets {
		target := &mockTarget{name: name}
		q.OnFileChanged([]string{"test.go"}, []types.Target{target})
	}

	// Wait for all builds to complete
	done := make(chan bool)
	go func() {
		wg.Wait()
		done <- true
	}()

	select {
	case <-done:
		// Success - check that builds ran in parallel
		mu.Lock()
		times := make([]time.Time, 0, len(buildStartTimes))
		for _, startTime := range buildStartTimes {
			times = append(times, startTime)
		}
		mu.Unlock()

		sort.Slice(times, func(i, j int) bool {
			return times[i].Before(times[j])
		})

		// All builds should start within a reasonable time window (parallel execution)
		maxStartGap := 100 * time.Millisecond
		for i := 1; i < len(times); i++ {
			gap := times[i].Sub(times[i-1])
			if gap > maxStartGap {
				t.Errorf("Build start gap too large: %v (expected < %v)", gap, maxStartGap)
			}
		}

	case <-time.After(2 * time.Second):
		t.Fatal("Parallel builds did not complete in time")
	}
}

func TestIntelligentBuildOrdering_DynamicPriorityUpdates(t *testing.T) {
	config := &types.BuildSchedulingConfig{
		Parallelization: 1,
		Prioritization: types.BuildPrioritization{
			Enabled: true,
		},
	}

	priorityEngine := &dynamicPriorityEngine{
		priorities: map[string]float64{
			"target1": 50.0,
			"target2": 50.0,
		},
		mu: sync.RWMutex{},
	}

	q := queue.NewIntelligentBuildQueue(config, nil, priorityEngine, nil)

	target1 := &mockTarget{name: "target1"}
	target2 := &mockTarget{name: "target2"}

	q.RegisterTarget(target1, &mockBuilder{target: target1})
	q.RegisterTarget(target2, &mockBuilder{target: target2})

	// Initial equal priority
	q.OnFileChanged([]string{"file.go"}, []types.Target{target1, target2})

	// Update priority dynamically
	priorityEngine.setPriority("target2", 100.0)

	// Clear and re-queue
	q.Clear()
	q.OnFileChanged([]string{"file.go"}, []types.Target{target1, target2})

	// target2 should now have higher priority
	req, _ := q.Dequeue()
	if req.Target.GetName() != "target2" {
		t.Errorf("Expected target2 after priority update, got %s", req.Target.GetName())
	}
}

func TestIntelligentBuildOrdering_QueueSaturation(t *testing.T) {
	config := &types.BuildSchedulingConfig{
		Parallelization: 2,
		Prioritization: types.BuildPrioritization{
			Enabled: true,
		},
	}

	q := queue.NewIntelligentBuildQueue(config, nil, &mockPriorityEngine{}, nil)

	// Create many targets
	numTargets := 100
	targets := make([]types.Target, numTargets)
	for i := 0; i < numTargets; i++ {
		target := &mockTarget{name: fmt.Sprintf("target%d", i)}
		targets[i] = target
		q.RegisterTarget(target, &mockBuilder{target: target})
	}

	// Queue all targets
	q.OnFileChanged([]string{"file.go"}, targets)

	// Verify queue size
	if q.Size() != numTargets {
		t.Errorf("Expected queue size %d, got %d", numTargets, q.Size())
	}

	// Dequeue all and verify order is maintained
	prevPriority := 1000.0
	for i := 0; i < numTargets; i++ {
		req, err := q.Dequeue()
		if err != nil {
			t.Fatalf("Failed to dequeue item %d: %v", i, err)
		}
		if req.Priority > prevPriority {
			t.Errorf("Priority ordering violated at item %d: current=%f, previous=%f",
				i, req.Priority, prevPriority)
		}
		prevPriority = req.Priority
	}
}

// Benchmark intelligent ordering performance
func BenchmarkIntelligentOrdering_PriorityCalculation(b *testing.B) {
	config := &types.BuildSchedulingConfig{
		Parallelization: 4,
		Prioritization: types.BuildPrioritization{
			Enabled: true,
		},
	}

	priorityEngine := &benchmarkPriorityEngine{}
	q := queue.NewIntelligentBuildQueue(config, nil, priorityEngine, nil)

	target := &mockTarget{name: "benchmark"}
	q.RegisterTarget(target, &mockBuilder{target: target})

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		q.OnFileChanged([]string{"file.go"}, []types.Target{target})
		q.Clear()
	}
}

func BenchmarkIntelligentOrdering_LargeQueue(b *testing.B) {
	config := &types.BuildSchedulingConfig{
		Parallelization: 4,
		Prioritization: types.BuildPrioritization{
			Enabled: true,
		},
	}

	q := queue.NewIntelligentBuildQueue(config, nil, &mockPriorityEngine{}, nil)

	// Create 1000 targets
	targets := make([]types.Target, 1000)
	for i := 0; i < 1000; i++ {
		target := &mockTarget{name: fmt.Sprintf("target%d", i)}
		targets[i] = target
		q.RegisterTarget(target, &mockBuilder{target: target})
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		q.OnFileChanged([]string{"file.go"}, targets)

		// Dequeue all
		for j := 0; j < 1000; j++ {
			q.Dequeue()
		}
	}
}

// Mock implementations for testing

type dynamicPriorityEngine struct {
	priorities map[string]float64
	mu         sync.RWMutex
}

func (e *dynamicPriorityEngine) CalculatePriority(target types.Target, files []string) float64 {
	e.mu.RLock()
	defer e.mu.RUnlock()
	if priority, ok := e.priorities[target.GetName()]; ok {
		return priority
	}
	return 50.0
}

func (e *dynamicPriorityEngine) setPriority(targetName string, priority float64) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.priorities[targetName] = priority
}

func (e *dynamicPriorityEngine) UpdateTargetMetrics(target string, buildTime time.Duration, success bool) {
}
func (e *dynamicPriorityEngine) GetTargetPriority(target string) *types.TargetPriority { return nil }
func (e *dynamicPriorityEngine) RecordFileChange(file string, targets []string)        {}

type timeBasedPriorityEngine struct {
	lastChanges map[string]time.Time
	mu          sync.RWMutex
}

func (e *timeBasedPriorityEngine) CalculatePriority(target types.Target, files []string) float64 {
	e.mu.Lock()
	defer e.mu.Unlock()

	if e.lastChanges == nil {
		e.lastChanges = make(map[string]time.Time)
	}

	now := time.Now()
	e.lastChanges[target.GetName()] = now

	// More recent changes get higher priority
	return float64(now.Unix())
}

func (e *timeBasedPriorityEngine) UpdateTargetMetrics(target string, buildTime time.Duration, success bool) {
}
func (e *timeBasedPriorityEngine) GetTargetPriority(target string) *types.TargetPriority { return nil }
func (e *timeBasedPriorityEngine) RecordFileChange(file string, targets []string)        {}

type frequencyBasedPriorityEngine struct {
	changeCount map[string]int
	mu          sync.RWMutex
}

func (e *frequencyBasedPriorityEngine) CalculatePriority(target types.Target, files []string) float64 {
	e.mu.Lock()
	defer e.mu.Unlock()

	count := e.changeCount[target.GetName()]
	e.changeCount[target.GetName()] = count + 1

	// Higher frequency = higher priority
	return float64(count * 10)
}

func (e *frequencyBasedPriorityEngine) UpdateTargetMetrics(target string, buildTime time.Duration, success bool) {
}
func (e *frequencyBasedPriorityEngine) GetTargetPriority(target string) *types.TargetPriority {
	return nil
}
func (e *frequencyBasedPriorityEngine) RecordFileChange(file string, targets []string) {}

type successRateBasedPriorityEngine struct {
	successRates map[string]float64
}

func (e *successRateBasedPriorityEngine) CalculatePriority(target types.Target, files []string) float64 {
	if rate, ok := e.successRates[target.GetName()]; ok {
		return rate * 100 // Scale to 0-100
	}
	return 50.0
}

func (e *successRateBasedPriorityEngine) UpdateTargetMetrics(target string, buildTime time.Duration, success bool) {
}
func (e *successRateBasedPriorityEngine) GetTargetPriority(target string) *types.TargetPriority {
	return nil
}
func (e *successRateBasedPriorityEngine) RecordFileChange(file string, targets []string) {}

type buildTimeBasedPriorityEngine struct {
	buildTimes map[string]time.Duration
}

func (e *buildTimeBasedPriorityEngine) CalculatePriority(target types.Target, files []string) float64 {
	if buildTime, ok := e.buildTimes[target.GetName()]; ok {
		// Faster builds get higher priority (inverse relationship)
		return 100.0 - float64(buildTime.Seconds())
	}
	return 50.0
}

func (e *buildTimeBasedPriorityEngine) UpdateTargetMetrics(target string, buildTime time.Duration, success bool) {
}
func (e *buildTimeBasedPriorityEngine) GetTargetPriority(target string) *types.TargetPriority {
	return nil
}
func (e *buildTimeBasedPriorityEngine) RecordFileChange(file string, targets []string) {}

type benchmarkPriorityEngine struct{}

func (e *benchmarkPriorityEngine) CalculatePriority(target types.Target, files []string) float64 {
	// Simulate complex priority calculation
	priority := 0.0
	for _, file := range files {
		priority += float64(len(file)) * 1.5
	}
	priority += float64(len(target.GetName())) * 2.0
	return priority
}

func (e *benchmarkPriorityEngine) UpdateTargetMetrics(target string, buildTime time.Duration, success bool) {
}
func (e *benchmarkPriorityEngine) GetTargetPriority(target string) *types.TargetPriority { return nil }
func (e *benchmarkPriorityEngine) RecordFileChange(file string, targets []string)        {}

type concurrentNotifier struct {
	mu           sync.Mutex
	buildStarts  []string
	buildSuccess []string
	buildFailure []string
}

func (n *concurrentNotifier) NotifyBuildStart(target string) {
	n.mu.Lock()
	defer n.mu.Unlock()
	n.buildStarts = append(n.buildStarts, target)
}

func (n *concurrentNotifier) NotifyBuildSuccess(target string, duration time.Duration) {
	n.mu.Lock()
	defer n.mu.Unlock()
	n.buildSuccess = append(n.buildSuccess, target)
}

func (n *concurrentNotifier) NotifyBuildFailure(target string, err error) {
	n.mu.Lock()
	defer n.mu.Unlock()
	n.buildFailure = append(n.buildFailure, target)
}

func (n *concurrentNotifier) NotifyQueueStatus(active int, queued int) {}

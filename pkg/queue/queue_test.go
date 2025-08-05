package queue_test

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/poltergeist/poltergeist/pkg/queue"
	"github.com/poltergeist/poltergeist/pkg/types"
)

// Mock implementations

type mockBuilder struct {
	buildFunc func(ctx context.Context, files []string) error
	target    types.Target
}

func (m *mockBuilder) Validate() error { return nil }
func (m *mockBuilder) Build(ctx context.Context, files []string) error {
	if m.buildFunc != nil {
		return m.buildFunc(ctx, files)
	}
	return nil
}
func (m *mockBuilder) Clean() error                      { return nil }
func (m *mockBuilder) GetTarget() types.Target           { return m.target }
func (m *mockBuilder) GetLastBuildTime() time.Duration   { return time.Second }
func (m *mockBuilder) GetSuccessRate() float64           { return 1.0 }

type mockTarget struct {
	name string
}

func (m *mockTarget) GetName() string                 { return m.name }
func (m *mockTarget) GetType() types.TargetType       { return types.TargetTypeExecutable }
func (m *mockTarget) IsEnabled() bool                 { return true }
func (m *mockTarget) GetBuildCommand() string         { return "build" }
func (m *mockTarget) GetWatchPaths() []string         { return []string{"*"} }
func (m *mockTarget) GetSettlingDelay() int           { return 100 }
func (m *mockTarget) GetEnvironment() map[string]string { return nil }
func (m *mockTarget) GetMaxRetries() int              { return 3 }
func (m *mockTarget) GetBackoffMultiplier() float64   { return 2.0 }
func (m *mockTarget) GetDebounceInterval() int        { return 100 }
func (m *mockTarget) GetIcon() string                 { return "" }

type mockPriorityEngine struct{}

func (m *mockPriorityEngine) CalculatePriority(target types.Target, files []string) float64 {
	return 50.0
}
func (m *mockPriorityEngine) UpdateTargetMetrics(target string, buildTime time.Duration, success bool) {}
func (m *mockPriorityEngine) GetTargetPriority(target string) *types.TargetPriority { return nil }
func (m *mockPriorityEngine) RecordFileChange(file string, targets []string) {}

type mockNotifier struct {
	mu           sync.Mutex
	buildStarts  []string
	buildSuccess []string
	buildFailure []string
}

func (m *mockNotifier) NotifyBuildStart(target string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.buildStarts = append(m.buildStarts, target)
}
func (m *mockNotifier) NotifyBuildSuccess(target string, duration time.Duration) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.buildSuccess = append(m.buildSuccess, target)
}
func (m *mockNotifier) NotifyBuildFailure(target string, err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.buildFailure = append(m.buildFailure, target)
}
func (m *mockNotifier) NotifyQueueStatus(active int, queued int) {}

// Tests

func TestBuildQueue_Enqueue(t *testing.T) {
	config := &types.BuildSchedulingConfig{
		Parallelization: 2,
		Prioritization: types.BuildPrioritization{
			Enabled: true,
		},
	}
	
	q := queue.NewIntelligentBuildQueue(config, nil, &mockPriorityEngine{}, nil)
	
	request := &types.BuildRequest{
		Target:          &mockTarget{name: "test"},
		Priority:        50,
		Timestamp:       time.Now(),
		TriggeringFiles: []string{"test.go"},
		ID:              uuid.New().String(),
	}
	
	err := q.Enqueue(request)
	if err != nil {
		t.Fatalf("failed to enqueue: %v", err)
	}
	
	if q.Size() != 1 {
		t.Errorf("expected queue size 1, got %d", q.Size())
	}
}

func TestBuildQueue_Dequeue(t *testing.T) {
	config := &types.BuildSchedulingConfig{
		Parallelization: 2,
	}
	
	q := queue.NewIntelligentBuildQueue(config, nil, nil, nil)
	
	// Add multiple requests with different priorities
	requests := []*types.BuildRequest{
		{
			Target:   &mockTarget{name: "low"},
			Priority: 10,
			ID:       uuid.New().String(),
		},
		{
			Target:   &mockTarget{name: "high"},
			Priority: 90,
			ID:       uuid.New().String(),
		},
		{
			Target:   &mockTarget{name: "medium"},
			Priority: 50,
			ID:       uuid.New().String(),
		},
	}
	
	for _, req := range requests {
		q.Enqueue(req)
	}
	
	// Should dequeue in priority order
	req1, _ := q.Dequeue()
	if req1.Priority != 90 {
		t.Errorf("expected priority 90, got %f", req1.Priority)
	}
	
	req2, _ := q.Dequeue()
	if req2.Priority != 50 {
		t.Errorf("expected priority 50, got %f", req2.Priority)
	}
	
	req3, _ := q.Dequeue()
	if req3.Priority != 10 {
		t.Errorf("expected priority 10, got %f", req3.Priority)
	}
}

func TestBuildQueue_Parallelization(t *testing.T) {
	config := &types.BuildSchedulingConfig{
		Parallelization: 2,
		Prioritization: types.BuildPrioritization{
			Enabled: false,
		},
	}
	
	notifier := &mockNotifier{}
	q := queue.NewIntelligentBuildQueue(config, nil, nil, notifier)
	
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	
	q.Start(ctx)
	defer q.Stop()
	
	// Register targets with builders
	var wg sync.WaitGroup
	buildCount := 0
	mu := sync.Mutex{}
	
	for i := 0; i < 4; i++ {
		target := &mockTarget{name: string(rune('A' + i))}
		builder := &mockBuilder{
			target: target,
			buildFunc: func(ctx context.Context, files []string) error {
				mu.Lock()
				buildCount++
				mu.Unlock()
				time.Sleep(100 * time.Millisecond)
				wg.Done()
				return nil
			},
		}
		q.RegisterTarget(target, builder)
	}
	
	// Trigger builds
	targets := []types.Target{
		&mockTarget{name: "A"},
		&mockTarget{name: "B"},
		&mockTarget{name: "C"},
		&mockTarget{name: "D"},
	}
	
	wg.Add(4)
	q.OnFileChanged([]string{"test.go"}, targets)
	
	// Wait for builds to complete
	done := make(chan bool)
	go func() {
		wg.Wait()
		done <- true
	}()
	
	select {
	case <-done:
		// Success
	case <-time.After(2 * time.Second):
		t.Fatal("builds did not complete in time")
	}
	
	mu.Lock()
	if buildCount != 4 {
		t.Errorf("expected 4 builds, got %d", buildCount)
	}
	mu.Unlock()
}

func TestBuildQueue_Clear(t *testing.T) {
	config := &types.BuildSchedulingConfig{
		Parallelization: 1,
	}
	
	q := queue.NewIntelligentBuildQueue(config, nil, nil, nil)
	
	// Add requests
	for i := 0; i < 5; i++ {
		q.Enqueue(&types.BuildRequest{
			Target:   &mockTarget{name: "test"},
			Priority: float64(i * 10),
			ID:       uuid.New().String(),
		})
	}
	
	if q.Size() != 5 {
		t.Errorf("expected size 5, got %d", q.Size())
	}
	
	q.Clear()
	
	if q.Size() != 0 {
		t.Errorf("expected size 0 after clear, got %d", q.Size())
	}
}

func TestBuildQueue_OnFileChanged(t *testing.T) {
	config := &types.BuildSchedulingConfig{
		Parallelization: 2,
		Prioritization: types.BuildPrioritization{
			Enabled: true,
		},
	}
	
	engine := &mockPriorityEngine{}
	q := queue.NewIntelligentBuildQueue(config, nil, engine, nil)
	
	// Register targets
	targets := []types.Target{
		&mockTarget{name: "target1"},
		&mockTarget{name: "target2"},
	}
	
	for _, target := range targets {
		q.RegisterTarget(target, &mockBuilder{target: target})
	}
	
	// Trigger file changes
	q.OnFileChanged([]string{"file1.go", "file2.go"}, targets)
	
	if q.Size() != 2 {
		t.Errorf("expected 2 queued builds, got %d", q.Size())
	}
}

func TestBuildQueue_NoDuplicates(t *testing.T) {
	config := &types.BuildSchedulingConfig{
		Parallelization: 1,
	}
	
	q := queue.NewIntelligentBuildQueue(config, nil, nil, nil)
	
	target := &mockTarget{name: "test"}
	q.RegisterTarget(target, &mockBuilder{target: target})
	
	// Trigger multiple file changes for same target
	targets := []types.Target{target}
	
	q.OnFileChanged([]string{"file1.go"}, targets)
	q.OnFileChanged([]string{"file2.go"}, targets)
	q.OnFileChanged([]string{"file3.go"}, targets)
	
	// Should only queue once since target is already pending
	if q.Size() != 1 {
		t.Errorf("expected 1 queued build (no duplicates), got %d", q.Size())
	}
}

func TestBuildQueue_BuildFailure(t *testing.T) {
	config := &types.BuildSchedulingConfig{
		Parallelization: 1,
	}
	
	notifier := &mockNotifier{}
	q := queue.NewIntelligentBuildQueue(config, nil, nil, notifier)
	
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	
	q.Start(ctx)
	defer q.Stop()
	
	// Register target with failing builder
	target := &mockTarget{name: "failing"}
	builder := &mockBuilder{
		target: target,
		buildFunc: func(ctx context.Context, files []string) error {
			return context.DeadlineExceeded
		},
	}
	q.RegisterTarget(target, builder)
	
	// Trigger build
	q.OnFileChanged([]string{"test.go"}, []types.Target{target})
	
	// Wait for build to fail
	time.Sleep(500 * time.Millisecond)
	
	// Check notifications
	if len(notifier.buildFailure) != 1 {
		t.Errorf("expected 1 build failure notification, got %d", len(notifier.buildFailure))
	}
}

func BenchmarkBuildQueue_Enqueue(b *testing.B) {
	config := &types.BuildSchedulingConfig{
		Parallelization: 4,
	}
	
	q := queue.NewIntelligentBuildQueue(config, nil, nil, nil)
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		q.Enqueue(&types.BuildRequest{
			Target:   &mockTarget{name: "bench"},
			Priority: float64(i % 100),
			ID:       uuid.New().String(),
		})
	}
}

func BenchmarkBuildQueue_Dequeue(b *testing.B) {
	config := &types.BuildSchedulingConfig{
		Parallelization: 4,
	}
	
	q := queue.NewIntelligentBuildQueue(config, nil, nil, nil)
	
	// Pre-fill queue
	for i := 0; i < b.N; i++ {
		q.Enqueue(&types.BuildRequest{
			Target:   &mockTarget{name: "bench"},
			Priority: float64(i % 100),
			ID:       uuid.New().String(),
		})
	}
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		q.Dequeue()
	}
}
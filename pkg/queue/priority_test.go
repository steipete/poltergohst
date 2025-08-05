package queue_test

import (
	"testing"
	"time"

	"github.com/poltergeist/poltergeist/pkg/queue"
	"github.com/poltergeist/poltergeist/pkg/types"
)

func TestPriorityEngine_CalculatePriority(t *testing.T) {
	config := &types.BuildSchedulingConfig{
		Prioritization: types.BuildPrioritization{
			Enabled:                true,
			FocusDetectionWindow:   300000, // 5 minutes
			PriorityDecayTime:      1800000, // 30 minutes
			BuildTimeoutMultiplier: 2.0,
		},
	}
	
	engine := queue.NewPriorityEngine(config, nil)
	
	// Test new target
	target := &mockTarget{name: "new-target"}
	priority := engine.CalculatePriority(target, []string{"file.go"})
	
	if priority != 50.0 {
		t.Errorf("expected base priority 50 for new target, got %f", priority)
	}
	
	// Record some file changes
	engine.RecordFileChange("file1.go", []string{"target1"})
	engine.RecordFileChange("file2.go", []string{"target1"})
	
	// Update metrics
	engine.UpdateTargetMetrics("target1", 5*time.Second, true)
	
	// Calculate priority with history
	target1 := &mockTarget{name: "target1"}
	priority = engine.CalculatePriority(target1, []string{"file3.go"})
	
	// Should be adjusted based on metrics
	if priority == 50.0 {
		t.Error("expected priority to be adjusted from base 50")
	}
}

func TestPriorityEngine_UpdateTargetMetrics(t *testing.T) {
	config := &types.BuildSchedulingConfig{
		Prioritization: types.BuildPrioritization{
			Enabled: true,
		},
	}
	
	engine := queue.NewPriorityEngine(config, nil)
	
	// Update metrics multiple times
	engine.UpdateTargetMetrics("target1", 2*time.Second, true)
	engine.UpdateTargetMetrics("target1", 3*time.Second, true)
	engine.UpdateTargetMetrics("target1", 4*time.Second, false)
	
	// Get priority info
	priority := engine.GetTargetPriority("target1")
	
	if priority == nil {
		t.Fatal("expected priority info, got nil")
	}
	
	if priority.Target != "target1" {
		t.Errorf("expected target name 'target1', got %s", priority.Target)
	}
	
	// Success rate should be 2/3
	if priority.SuccessRate != 2.0/3.0 {
		t.Errorf("expected success rate 0.666, got %f", priority.SuccessRate)
	}
	
	// Last build time should be 4 seconds
	if priority.AvgBuildTime != 4*time.Second {
		t.Errorf("expected last build time 4s, got %s", priority.AvgBuildTime)
	}
}

func TestPriorityEngine_RecordFileChange(t *testing.T) {
	config := &types.BuildSchedulingConfig{
		Prioritization: types.BuildPrioritization{
			Enabled: true,
		},
	}
	
	engine := queue.NewPriorityEngine(config, nil)
	
	// Record multiple file changes
	engine.RecordFileChange("file1.go", []string{"target1", "target2"})
	engine.RecordFileChange("file2.go", []string{"target1"})
	engine.RecordFileChange("file3.go", []string{"target2", "target3"})
	
	// Get priority for target1 (most changes)
	priority1 := engine.GetTargetPriority("target1")
	if priority1 == nil {
		t.Fatal("expected priority info for target1")
	}
	
	// Check recent changes
	if len(priority1.RecentChanges) != 2 {
		t.Errorf("expected 2 recent changes for target1, got %d", len(priority1.RecentChanges))
	}
	
	// Get priority for target3 (least changes)
	priority3 := engine.GetTargetPriority("target3")
	if priority3 == nil {
		t.Fatal("expected priority info for target3")
	}
	
	if len(priority3.RecentChanges) != 1 {
		t.Errorf("expected 1 recent change for target3, got %d", len(priority3.RecentChanges))
	}
}

func TestPriorityEngine_FocusDetection(t *testing.T) {
	config := &types.BuildSchedulingConfig{
		Prioritization: types.BuildPrioritization{
			Enabled:              true,
			FocusDetectionWindow: 1000, // 1 second for testing
		},
	}
	
	engine := queue.NewPriorityEngine(config, nil)
	
	// Record recent change
	engine.RecordFileChange("file.go", []string{"focused-target"})
	target := &mockTarget{name: "focused-target"}
	
	// Priority should be boosted (recent change)
	priority := engine.CalculatePriority(target, []string{"file.go"})
	if priority <= 50.0 {
		t.Errorf("expected boosted priority for recent change, got %f", priority)
	}
	
	// Wait for focus window to expire
	time.Sleep(1100 * time.Millisecond)
	
	// Priority should be back to normal (50 base + 10 for fast build time)
	priority = engine.CalculatePriority(target, []string{"file.go"})
	if priority != 60.0 {
		t.Errorf("expected normal priority (60.0 = 50 base + 10 fast build) after focus window, got %f", priority)
	}
}

func TestPriorityEngine_SuccessRateImpact(t *testing.T) {
	config := &types.BuildSchedulingConfig{
		Prioritization: types.BuildPrioritization{
			Enabled: true,
		},
	}
	
	engine := queue.NewPriorityEngine(config, nil)
	
	// Create two targets with different success rates
	engine.UpdateTargetMetrics("successful", time.Second, true)
	engine.UpdateTargetMetrics("successful", time.Second, true)
	engine.UpdateTargetMetrics("successful", time.Second, true)
	
	engine.UpdateTargetMetrics("failing", time.Second, false)
	engine.UpdateTargetMetrics("failing", time.Second, false)
	engine.UpdateTargetMetrics("failing", time.Second, true)
	
	successTarget := &mockTarget{name: "successful"}
	failTarget := &mockTarget{name: "failing"}
	
	successPriority := engine.CalculatePriority(successTarget, []string{"file.go"})
	failPriority := engine.CalculatePriority(failTarget, []string{"file.go"})
	
	// Successful target should have higher priority
	if successPriority <= failPriority {
		t.Errorf("expected successful target to have higher priority: success=%f, fail=%f", 
			successPriority, failPriority)
	}
}

func TestPriorityEngine_BuildTimeImpact(t *testing.T) {
	config := &types.BuildSchedulingConfig{
		Prioritization: types.BuildPrioritization{
			Enabled: true,
		},
	}
	
	engine := queue.NewPriorityEngine(config, nil)
	
	// Create targets with different build times
	engine.UpdateTargetMetrics("fast", 2*time.Second, true)
	engine.UpdateTargetMetrics("slow", 45*time.Second, true)
	
	fastTarget := &mockTarget{name: "fast"}
	slowTarget := &mockTarget{name: "slow"}
	
	// Record same recent changes for both
	engine.RecordFileChange("file.go", []string{"fast", "slow"})
	
	fastPriority := engine.CalculatePriority(fastTarget, []string{"file.go"})
	slowPriority := engine.CalculatePriority(slowTarget, []string{"file.go"})
	
	// Fast builds should have slightly higher priority
	if fastPriority <= slowPriority {
		t.Errorf("expected fast target to have higher priority: fast=%f, slow=%f",
			fastPriority, slowPriority)
	}
}

func TestPriorityEngine_ChangeFrequency(t *testing.T) {
	config := &types.BuildSchedulingConfig{
		Prioritization: types.BuildPrioritization{
			Enabled: true,
		},
	}
	
	engine := queue.NewPriorityEngine(config, nil)
	
	// Simulate frequent changes to one target
	for i := 0; i < 10; i++ {
		engine.RecordFileChange(string(rune('a'+i))+".go", []string{"frequent"})
		time.Sleep(10 * time.Millisecond)
	}
	
	// Simulate infrequent changes to another
	engine.RecordFileChange("rare.go", []string{"infrequent"})
	
	frequentTarget := &mockTarget{name: "frequent"}
	infrequentTarget := &mockTarget{name: "infrequent"}
	
	frequentPriority := engine.CalculatePriority(frequentTarget, []string{"new.go"})
	infrequentPriority := engine.CalculatePriority(infrequentTarget, []string{"new.go"})
	
	// Frequently changed target should have higher priority
	if frequentPriority <= infrequentPriority {
		t.Errorf("expected frequent target to have higher priority: frequent=%f, infrequent=%f",
			frequentPriority, infrequentPriority)
	}
}

func TestPriorityEngine_PriorityDecay(t *testing.T) {
	config := &types.BuildSchedulingConfig{
		Prioritization: types.BuildPrioritization{
			Enabled:           true,
			PriorityDecayTime: 100, // 100ms for testing
		},
	}
	
	engine := queue.NewPriorityEngine(config, nil)
	
	// Record a change
	engine.RecordFileChange("file.go", []string{"decaying"})
	target := &mockTarget{name: "decaying"}
	
	// Get initial priority
	initialPriority := engine.CalculatePriority(target, []string{"file.go"})
	
	// Wait for partial decay
	time.Sleep(50 * time.Millisecond)
	midPriority := engine.CalculatePriority(target, []string{"file.go"})
	
	// Wait for full decay
	time.Sleep(60 * time.Millisecond)
	finalPriority := engine.CalculatePriority(target, []string{"file.go"})
	
	// Priority should decrease over time
	if midPriority >= initialPriority {
		t.Error("expected priority to decay over time")
	}
	
	if finalPriority >= midPriority {
		t.Error("expected continued priority decay")
	}
}

func TestPriorityEngine_MaxRecentChanges(t *testing.T) {
	config := &types.BuildSchedulingConfig{
		Prioritization: types.BuildPrioritization{
			Enabled: true,
		},
	}
	
	engine := queue.NewPriorityEngine(config, nil)
	
	// Record more than 100 changes
	for i := 0; i < 150; i++ {
		engine.RecordFileChange(string(rune(i))+".go", []string{"target"})
	}
	
	priority := engine.GetTargetPriority("target")
	if priority == nil {
		t.Fatal("expected priority info")
	}
	
	// Should keep only recent 100 changes
	if len(priority.RecentChanges) > 100 {
		t.Errorf("expected max 100 recent changes, got %d", len(priority.RecentChanges))
	}
}

func TestPriorityEngine_PriorityClamping(t *testing.T) {
	config := &types.BuildSchedulingConfig{
		Prioritization: types.BuildPrioritization{
			Enabled: true,
		},
	}
	
	engine := queue.NewPriorityEngine(config, nil)
	
	// Create a target with many positive factors
	for i := 0; i < 50; i++ {
		engine.RecordFileChange(string(rune(i))+".go", []string{"high"})
		engine.UpdateTargetMetrics("high", time.Second, true)
	}
	
	highTarget := &mockTarget{name: "high"}
	priority := engine.CalculatePriority(highTarget, []string{"new.go"})
	
	// Should be clamped to 100
	if priority > 100 {
		t.Errorf("expected priority clamped to 100, got %f", priority)
	}
	
	// Create a target with many negative factors
	for i := 0; i < 50; i++ {
		engine.UpdateTargetMetrics("low", time.Hour, false)
	}
	
	lowTarget := &mockTarget{name: "low"}
	priority = engine.CalculatePriority(lowTarget, []string{"new.go"})
	
	// Should be clamped to 0
	if priority < 0 {
		t.Errorf("expected priority clamped to 0, got %f", priority)
	}
}

func BenchmarkPriorityEngine_CalculatePriority(b *testing.B) {
	config := &types.BuildSchedulingConfig{
		Prioritization: types.BuildPrioritization{
			Enabled: true,
		},
	}
	
	engine := queue.NewPriorityEngine(config, nil)
	
	// Setup some history
	for i := 0; i < 100; i++ {
		engine.RecordFileChange("file.go", []string{"target"})
		engine.UpdateTargetMetrics("target", time.Second, true)
	}
	
	target := &mockTarget{name: "target"}
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		engine.CalculatePriority(target, []string{"file.go"})
	}
}

func BenchmarkPriorityEngine_RecordFileChange(b *testing.B) {
	config := &types.BuildSchedulingConfig{
		Prioritization: types.BuildPrioritization{
			Enabled: true,
		},
	}
	
	engine := queue.NewPriorityEngine(config, nil)
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		engine.RecordFileChange("file.go", []string{"target1", "target2", "target3"})
	}
}
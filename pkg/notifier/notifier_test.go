package notifier_test

import (
	"fmt"
	"testing"
	"time"

	"github.com/poltergeist/poltergeist/pkg/logger"
	"github.com/poltergeist/poltergeist/pkg/notifier"
)

func TestNotifier_BuildSuccess(t *testing.T) {
	log := logger.CreateLogger("", "info")
	
	config := notifier.Config{
		Enabled:      true,
		SuccessSound: "default",
		FailureSound: "alert",
	}
	
	n := notifier.New(config, log)
	
	// This would normally show a system notification
	// In tests, we just verify it doesn't crash
	n.NotifyBuildSuccess("Test Target", 5*time.Second)
}

func TestNotifier_BuildFailure(t *testing.T) {
	log := logger.CreateLogger("", "info")
	
	config := notifier.Config{
		Enabled:      true,
		SuccessSound: "default",
		FailureSound: "alert",
	}
	
	n := notifier.New(config, log)
	
	buildErr := fmt.Errorf("syntax error at line 42")
	n.NotifyBuildFailure("Test Target", buildErr)
}

func TestNotifier_BuildStart(t *testing.T) {
	log := logger.CreateLogger("", "info")
	
	config := notifier.Config{
		Enabled: true,
	}
	
	n := notifier.New(config, log)
	
	// Test build start notification
	n.NotifyBuildStart("Test Target")
}

func TestNotifier_QueueStatus(t *testing.T) {
	log := logger.CreateLogger("", "info")
	
	config := notifier.Config{
		Enabled: true,
	}
	
	n := notifier.New(config, log)
	
	// Test queue status notification
	n.NotifyQueueStatus(2, 5)
}

func TestNotifier_Disabled(t *testing.T) {
	log := logger.CreateLogger("", "info")
	
	config := notifier.Config{
		Enabled: false,
	}
	
	n := notifier.New(config, log)
	
	// Should not send notification when disabled
	// These methods don't return errors, they just don't do anything when disabled
	n.NotifyBuildSuccess("Test", 1*time.Second)
	n.NotifyBuildFailure("Test", fmt.Errorf("test error"))
	n.NotifyBuildStart("Test")
	n.NotifyQueueStatus(1, 2)
}

func TestNotifier_CustomSound(t *testing.T) {
	log := logger.CreateLogger("", "info")
	
	config := notifier.Config{
		Enabled:      true,
		SuccessSound: "Glass",
		FailureSound: "Basso",
	}
	
	n := notifier.New(config, log)
	
	// Test custom sounds
	n.NotifyBuildSuccess("Test", 1*time.Second)
	n.NotifyBuildFailure("Test", fmt.Errorf("custom failure"))
}

func TestNotifier_MultipleTargets(t *testing.T) {
	log := logger.CreateLogger("", "info")
	
	config := notifier.Config{
		Enabled: true,
	}
	
	n := notifier.New(config, log)
	
	targets := []string{"backend", "frontend", "tests"}
	
	for _, target := range targets {
		n.NotifyBuildStart(target)
		n.NotifyBuildSuccess(target, 1*time.Second)
	}
}

func TestNotifier_BuildProgress(t *testing.T) {
	log := logger.CreateLogger("", "info")
	
	config := notifier.Config{
		Enabled: true,
	}
	
	n := notifier.New(config, log)
	
	// Simulate build progress notifications
	n.NotifyBuildStart("backend")
	time.Sleep(100 * time.Millisecond)
	
	n.NotifyQueueStatus(1, 2)
	time.Sleep(100 * time.Millisecond)
	
	n.NotifyBuildSuccess("backend", 2*time.Second)
}

func TestNotifier_ConcurrentNotifications(t *testing.T) {
	log := logger.CreateLogger("", "info")
	
	config := notifier.Config{
		Enabled: true,
	}
	
	n := notifier.New(config, log)
	
	// Send multiple notifications concurrently
	done := make(chan bool, 5)
	
	for i := 0; i < 5; i++ {
		go func(idx int) {
			n.NotifyBuildSuccess(
				fmt.Sprintf("Target-%d", idx),
				1*time.Second,
			)
			done <- true
		}(i)
	}
	
	// Wait for all notifications
	for i := 0; i < 5; i++ {
		<-done
	}
}

func TestNotifier_ErrorFormats(t *testing.T) {
	log := logger.CreateLogger("", "info")
	
	config := notifier.Config{
		Enabled: true,
	}
	
	n := notifier.New(config, log)
	
	// Test various error formats
	errors := []error{
		fmt.Errorf("simple error"),
		fmt.Errorf("multi-line\nerror\nmessage"),
		fmt.Errorf("error with special chars: %s %d %%", "test", 42),
		nil, // Should handle nil gracefully
	}
	
	for _, err := range errors {
		n.NotifyBuildFailure("test", err)
	}
}

func BenchmarkNotifier_Success(b *testing.B) {
	log := logger.CreateLogger("", "error")
	
	config := notifier.Config{
		Enabled: false, // Disable actual notifications for benchmark
	}
	
	n := notifier.New(config, log)
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		n.NotifyBuildSuccess("Benchmark", 1*time.Second)
	}
}

func BenchmarkNotifier_Failure(b *testing.B) {
	log := logger.CreateLogger("", "error")
	
	config := notifier.Config{
		Enabled: false,
	}
	
	n := notifier.New(config, log)
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		n.NotifyBuildFailure("Benchmark", fmt.Errorf("test error"))
	}
}

func BenchmarkNotifier_QueueStatus(b *testing.B) {
	log := logger.CreateLogger("", "error")
	
	config := notifier.Config{
		Enabled: false,
	}
	
	n := notifier.New(config, log)
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		n.NotifyQueueStatus(2, 5)
	}
}
package process_test

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/poltergeist/poltergeist/pkg/logger"
	"github.com/poltergeist/poltergeist/pkg/process"
)

func TestManagerStopWithBackgroundContext(t *testing.T) {
	manager := process.NewManager(logger.CreateLogger("", "error"))
	manager.Start(context.Background())

	stopped := make(chan struct{})
	go func() {
		manager.Stop()
		close(stopped)
	}()

	select {
	case <-stopped:
	case <-time.After(time.Second):
		t.Fatal("process manager did not stop")
	}

	if manager.IsRunning() {
		t.Error("expected process manager to be stopped")
	}
}

func TestManagerStopSuppressesConcurrentContextShutdown(t *testing.T) {
	for range 50 {
		manager := process.NewManager(logger.CreateLogger("", "error"))
		ctx, cancel := context.WithCancel(context.Background())
		var shutdownCalls atomic.Int32
		manager.RegisterShutdownHandler(func() {
			shutdownCalls.Add(1)
		})
		manager.Start(ctx)

		stopped := make(chan struct{})
		go func() {
			manager.Stop()
			close(stopped)
		}()

		deadline := time.Now().Add(time.Second)
		for manager.IsRunning() && time.Now().Before(deadline) {
			time.Sleep(time.Millisecond)
		}
		if manager.IsRunning() {
			cancel()
			t.Fatal("process manager did not begin stopping")
		}

		cancel()
		select {
		case <-stopped:
		case <-time.After(time.Second):
			t.Fatal("process manager did not stop")
		}

		if got := shutdownCalls.Load(); got != 0 {
			t.Fatalf("explicit stop ran shutdown handler %d times", got)
		}
	}
}

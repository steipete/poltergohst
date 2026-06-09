package watchman

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/poltergeist/poltergeist/pkg/logger"
)

func TestRemoveCancelsPendingWatchActivation(t *testing.T) {
	root := t.TempDir()
	subdir := filepath.Join(root, "subdir")
	if err := os.Mkdir(subdir, 0o755); err != nil {
		t.Fatal(err)
	}

	watcher, err := NewFSNotifyWatcher(logger.CreateLogger("", "error"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = watcher.Close()
	})
	if err := watcher.watcher.Add(root); err != nil {
		t.Fatal(err)
	}
	if err := watcher.watcher.Add(subdir); err != nil {
		t.Fatal(err)
	}

	callbackStarted := make(chan struct{})
	releaseCallback := make(chan struct{})
	watcher.beginWatch(root, func(FileEvent) {
		close(callbackStarted)
		<-releaseCallback
	})

	watcher.mu.Lock()
	pending := watcher.pendingWatches[root]
	pending.committed = true
	pending.events = append(pending.events, FileEvent{Path: filepath.Join(root, "file.go")})
	watcher.mu.Unlock()

	activationDone := make(chan struct{})
	go func() {
		watcher.activateWatch(root, pending)
		close(activationDone)
	}()

	select {
	case <-callbackStarted:
	case <-time.After(time.Second):
		t.Fatal("pending callback did not start")
	}

	if err := watcher.Remove(root); err != nil {
		t.Fatalf("remove failed: %v", err)
	}
	close(releaseCallback)

	select {
	case <-activationDone:
	case <-time.After(time.Second):
		t.Fatal("watch activation did not stop after removal")
	}

	watcher.mu.RLock()
	_, callbackExists := watcher.callbacks[root]
	_, pendingExists := watcher.pendingWatches[root]
	watcher.mu.RUnlock()
	if callbackExists || pendingExists {
		t.Fatal("removed watch was reactivated")
	}
	for _, watchedPath := range watcher.watcher.WatchList() {
		if pathWithinRoot(watchedPath, root) {
			t.Fatalf("recursive watch remains active: %s", watchedPath)
		}
	}
}

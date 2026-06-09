package watchman

import (
	"bufio"
	"context"
	"encoding/json"
	"net"
	"path/filepath"
	"testing"
	"time"

	"github.com/poltergeist/poltergeist/pkg/interfaces"
	"github.com/poltergeist/poltergeist/pkg/logger"
	"github.com/poltergeist/poltergeist/pkg/types"
)

func TestUnsubscribeAfterReceiverStarts(t *testing.T) {
	clientConn, serverConn := net.Pipe()
	t.Cleanup(func() {
		_ = clientConn.Close()
		_ = serverConn.Close()
	})

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	client := &UnifiedClient{
		logger: logger.CreateLogger("", "error"),
		watchmanConn: &WatchmanConnection{
			conn:   clientConn,
			reader: bufio.NewReader(clientConn),
			writer: bufio.NewWriter(clientConn),
		},
		useWatchman: true,
		subscriptions: map[string]*subscription{
			"test": {name: "test", root: "/repo"},
		},
		ctx:             ctx,
		cancel:          cancel,
		eventChan:       make(chan FileEvent, 1),
		receiverStarted: true,
	}

	serverDone := make(chan error, 1)
	go func() {
		reader := bufio.NewReader(serverConn)
		line, err := reader.ReadBytes('\n')
		if err != nil {
			serverDone <- err
			return
		}

		var command WatchmanCommand
		if err := json.Unmarshal(line, &command); err != nil {
			serverDone <- err
			return
		}

		_, err = serverConn.Write([]byte(
			"{\"unilateral\":true,\"subscription\":\"test\",\"root\":\"/repo\",\"files\":[{\"name\":\"changed.go\",\"exists\":true,\"type\":\"f\"}]}\n" +
				"{\"unsubscribe\":\"test\",\"deleted\":true}\n",
		))
		serverDone <- err
	}()

	go client.receiveWatchmanEvents()
	if err := client.Unsubscribe("test"); err != nil {
		t.Fatalf("unsubscribe failed: %v", err)
	}

	select {
	case err := <-serverDone:
		if err != nil {
			t.Fatalf("fake Watchman server failed: %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for fake Watchman server")
	}

	client.mu.RLock()
	_, exists := client.subscriptions["test"]
	client.mu.RUnlock()
	if exists {
		t.Fatal("subscription was not removed")
	}

	select {
	case event := <-client.eventChan:
		if event.Path != filepath.Join("/repo", "changed.go") {
			t.Fatalf("unexpected event path: %s", event.Path)
		}
	case <-time.After(time.Second):
		t.Fatal("interleaved subscription event was not delivered")
	}
}

func TestWatchmanCommandsAfterReceiverStarts(t *testing.T) {
	clientConn, serverConn := net.Pipe()
	t.Cleanup(func() {
		_ = clientConn.Close()
		_ = serverConn.Close()
	})

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	client := &UnifiedClient{
		logger: logger.CreateLogger("", "error"),
		watchmanConn: &WatchmanConnection{
			conn:   clientConn,
			reader: bufio.NewReader(clientConn),
			writer: bufio.NewWriter(clientConn),
		},
		useWatchman:   true,
		subscriptions: make(map[string]*subscription),
		config:        &types.WatchmanConfig{},
		ctx:           ctx,
		cancel:        cancel,
		eventChan:     make(chan FileEvent, 1),
	}

	serverDone := make(chan error, 1)
	go func() {
		reader := bufio.NewReader(serverConn)
		responses := []string{
			"{\"watch\":\"/repo\"}\n",
			"{\"clock\":\"c:1:1\"}\n",
			"{\"unilateral\":true,\"subscription\":\"second\",\"root\":\"/repo\",\"files\":[{\"name\":\"new.go\",\"exists\":true,\"type\":\"f\"}]}\n" +
				"{\"subscribe\":\"second\"}\n",
			"{\"version\":\"2026.06.09\"}\n",
			"{\"unsubscribe\":\"second\",\"deleted\":true}\n",
		}
		for _, response := range responses {
			line, err := reader.ReadBytes('\n')
			if err != nil {
				serverDone <- err
				return
			}
			var command WatchmanCommand
			if err := json.Unmarshal(line, &command); err != nil {
				serverDone <- err
				return
			}
			if _, err := serverConn.Write([]byte(response)); err != nil {
				serverDone <- err
				return
			}
		}
		serverDone <- nil
	}()

	client.StartEventReceiver()
	if err := client.WatchProject("/repo"); err != nil {
		t.Fatalf("watch project failed: %v", err)
	}
	if err := client.Subscribe(
		"/repo",
		"second",
		interfaces.SubscriptionConfig{},
		nil,
		nil,
	); err != nil {
		t.Fatalf("subscribe failed: %v", err)
	}
	version, err := client.GetVersion()
	if err != nil {
		t.Fatalf("version failed: %v", err)
	}
	if version != "2026.06.09" {
		t.Fatalf("unexpected version: %s", version)
	}
	if err := client.Unsubscribe("second"); err != nil {
		t.Fatalf("unsubscribe failed: %v", err)
	}

	select {
	case event := <-client.eventChan:
		if event.Path != filepath.Join("/repo", "new.go") {
			t.Fatalf("unexpected event path: %s", event.Path)
		}
	case <-time.After(time.Second):
		t.Fatal("interleaved subscription event was not delivered")
	}

	select {
	case err := <-serverDone:
		if err != nil {
			t.Fatalf("fake Watchman server failed: %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for fake Watchman server")
	}
}

func TestWatchmanCommandsBeforeReceiverRouteEvents(t *testing.T) {
	clientConn, serverConn := net.Pipe()
	t.Cleanup(func() {
		_ = clientConn.Close()
		_ = serverConn.Close()
	})

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	client := &UnifiedClient{
		logger: logger.CreateLogger("", "error"),
		watchmanConn: &WatchmanConnection{
			conn:   clientConn,
			reader: bufio.NewReader(clientConn),
			writer: bufio.NewWriter(clientConn),
		},
		useWatchman: true,
		subscriptions: map[string]*subscription{
			"existing": {name: "existing", root: "/repo"},
		},
		config:    &types.WatchmanConfig{},
		ctx:       ctx,
		cancel:    cancel,
		eventChan: make(chan FileEvent, 1),
	}

	serverDone := make(chan error, 1)
	go func() {
		reader := bufio.NewReader(serverConn)
		line, err := reader.ReadBytes('\n')
		if err != nil {
			serverDone <- err
			return
		}
		var command WatchmanCommand
		if err := json.Unmarshal(line, &command); err != nil {
			serverDone <- err
			return
		}
		_, err = serverConn.Write([]byte(
			"{\"unilateral\":true,\"subscription\":\"existing\",\"root\":\"/repo\",\"files\":[{\"name\":\"early.go\",\"exists\":true,\"type\":\"f\"}]}\n" +
				"{\"version\":\"2026.06.09\"}\n",
		))
		serverDone <- err
	}()

	version, err := client.GetVersion()
	if err != nil {
		t.Fatalf("version failed: %v", err)
	}
	if version != "2026.06.09" {
		t.Fatalf("unexpected version: %s", version)
	}

	select {
	case event := <-client.eventChan:
		if event.Path != filepath.Join("/repo", "early.go") {
			t.Fatalf("unexpected event path: %s", event.Path)
		}
	case <-time.After(time.Second):
		t.Fatal("unilateral event was not delivered")
	}

	select {
	case err := <-serverDone:
		if err != nil {
			t.Fatalf("fake Watchman server failed: %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for fake Watchman server")
	}
}

func TestPendingFSNotifyEventsAreCoalescedAndBounded(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	client := &UnifiedClient{
		logger:        logger.CreateLogger("", "error"),
		config:        &types.WatchmanConfig{MaxFileEvents: 2},
		ctx:           ctx,
		cancel:        cancel,
		eventChan:     make(chan FileEvent, 1),
		subscriptions: make(map[string]*subscription),
	}

	client.handleFSNotifyEvent(FileEvent{Path: "a.go", Type: FileCreated})
	client.handleFSNotifyEvent(FileEvent{Path: "b.go", Type: FileCreated})
	client.handleFSNotifyEvent(FileEvent{Path: "a.go", Type: FileModified})
	client.handleFSNotifyEvent(FileEvent{Path: "c.go", Type: FileCreated})

	if len(client.pendingFSEvents) != 2 {
		t.Fatalf("expected 2 pending events, got %d", len(client.pendingFSEvents))
	}
	if client.pendingFSEvents[0].Path != "a.go" || client.pendingFSEvents[0].Type != FileModified {
		t.Fatalf("unexpected coalesced event: %+v", client.pendingFSEvents[0])
	}
	if client.pendingFSEvents[1].Path != "c.go" {
		t.Fatalf("unexpected newest event: %+v", client.pendingFSEvents[1])
	}
}

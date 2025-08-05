// Example demonstrating Go-Watchman bindings
package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/poltergeist/poltergeist/pkg/logger"
	"github.com/poltergeist/poltergeist/pkg/watchman"
)

func main() {
	// Create logger
	log := logger.CreateLogger("", "info")

	// Determine which implementation to use
	useWatchman := checkWatchmanAvailable()

	if useWatchman {
		log.Info("Using Watchman for file watching")
		runWatchmanExample(log)
	} else {
		log.Info("Watchman not available, using fsnotify fallback")
		runFSNotifyExample(log)
	}
}

// checkWatchmanAvailable checks if Watchman is installed and running
func checkWatchmanAvailable() bool {
	conn, err := watchman.Connect()
	if err != nil {
		return false
	}
	defer conn.Close()

	// Try to get version
	_, err = conn.Version()
	return err == nil
}

// runWatchmanExample demonstrates using the Watchman protocol
func runWatchmanExample(log logger.Logger) {
	// Connect to Watchman
	conn, err := watchman.Connect()
	if err != nil {
		log.Error(fmt.Sprintf("Failed to connect to Watchman: %v", err))
		return
	}
	defer conn.Close()

	// Get Watchman version
	version, err := conn.Version()
	if err != nil {
		log.Error(fmt.Sprintf("Failed to get Watchman version: %v", err))
	} else {
		log.Info(fmt.Sprintf("Connected to Watchman version: %s", version))
	}

	// Watch current directory
	cwd, _ := os.Getwd()
	projectPath := cwd

	// Watch the project
	resp, err := conn.WatchProject(projectPath)
	if err != nil {
		log.Error(fmt.Sprintf("Failed to watch project: %v", err))
		return
	}

	log.Info(fmt.Sprintf("Watching project: %s", resp.Watch))
	if resp.RelativeRoot != "" {
		log.Info(fmt.Sprintf("Relative root: %s", resp.RelativeRoot))
	}

	// Get initial clock
	clock, err := conn.Clock(resp.Watch)
	if err != nil {
		log.Error(fmt.Sprintf("Failed to get clock: %v", err))
	}

	// Create subscription for Go files
	subscriptionName := "go-files"
	query := watchman.SubscriptionQuery{
		Expression: watchman.AllOfExpression(
			watchman.AnyOfExpression(
				watchman.MatchExpression("*.go", false),
				watchman.MatchExpression("go.mod", false),
				watchman.MatchExpression("go.sum", false),
			),
			watchman.NotExpression(
				watchman.MatchExpression("*_test.go", false),
			),
		),
		Fields: []string{"name", "size", "mtime_ms", "exists", "type", "new"},
		Since:  clock,
		Empty:  true,
	}

	// Subscribe to changes
	_, err = conn.Subscribe(resp.Watch, subscriptionName, query)
	if err != nil {
		log.Error(fmt.Sprintf("Failed to subscribe: %v", err))
		return
	}

	log.Info(fmt.Sprintf("Created subscription: %s", subscriptionName))

	// Process events in a goroutine
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			default:
				// Receive events
				event, err := conn.Receive()
				if err != nil {
					log.Error(fmt.Sprintf("Error receiving event: %v", err))
					continue
				}

				// Check if it's a subscription notification
				if event.Subscription == subscriptionName {
					log.Info(fmt.Sprintf("Files changed in %s:", event.Root))
					for _, file := range event.Files {
						fileEvent := watchman.ConvertWatchmanFile(event.Root, file)
						logFileEvent(log, fileEvent)
					}
				}
			}
		}
	}()

	// Example: Perform a one-time query
	performQuery(conn, resp.Watch, log)

	// Example: Create a trigger
	createTriggerExample(conn, resp.Watch, log)

	// Wait for interrupt
	waitForInterrupt(log)

	// Clean up
	conn.Unsubscribe(resp.Watch, subscriptionName)
}

// runFSNotifyExample demonstrates using the fsnotify fallback
func runFSNotifyExample(log logger.Logger) {
	// Create fsnotify watcher
	watcher, err := watchman.NewFSNotifyWatcher(log)
	if err != nil {
		log.Error(fmt.Sprintf("Failed to create watcher: %v", err))
		return
	}
	defer watcher.Close()

	// Configure watcher
	watcher.SetPatterns([]string{
		"*.go",
		"go.mod",
		"go.sum",
	})

	watcher.SetExclusions([]string{
		"vendor",
		".git",
		"_test.go",
	})

	watcher.SetSettlingDelay(200 * time.Millisecond)

	// Watch current directory
	cwd, _ := os.Getwd()

	// Define callback for file events
	callback := func(event watchman.FileEvent) {
		logFileEvent(log, event)
	}

	// Start watching
	err = watcher.WatchProject(cwd, callback)
	if err != nil {
		log.Error(fmt.Sprintf("Failed to watch project: %v", err))
		return
	}

	log.Info(fmt.Sprintf("Watching project with fsnotify: %s", cwd))

	// Wait for interrupt
	waitForInterrupt(log)
}

// performQuery demonstrates performing a one-time query
func performQuery(conn *watchman.WatchmanConnection, root string, log logger.Logger) {
	// Query for all Go files
	query := watchman.Query{
		Expression: watchman.MatchExpression("*.go", false),
		Fields:     []string{"name", "size", "mtime_ms"},
	}

	resp, err := conn.Query(root, query)
	if err != nil {
		log.Error(fmt.Sprintf("Query failed: %v", err))
		return
	}

	log.Info(fmt.Sprintf("Found %d Go files:", len(resp.Files)))
	for _, file := range resp.Files {
		log.Debug(fmt.Sprintf("  %s (size: %d bytes)", file.Name, file.Size))
	}
}

// createTriggerExample demonstrates creating a trigger
func createTriggerExample(conn *watchman.WatchmanConnection, root string, log logger.Logger) {
	// Create a trigger that runs when Go files change
	triggerName := "go-build-trigger"

	query := watchman.Query{
		Expression: watchman.MatchExpression("*.go", false),
	}

	command := []string{"echo", "Go files changed, would rebuild..."}

	err := conn.Trigger(root, triggerName, query, command)
	if err != nil {
		log.Error(fmt.Sprintf("Failed to create trigger: %v", err))
		return
	}

	log.Info(fmt.Sprintf("Created trigger: %s", triggerName))

	// List triggers
	_, err = conn.TriggerList(root)
	if err != nil {
		log.Error(fmt.Sprintf("Failed to list triggers: %v", err))
	} else {
		log.Debug("Active triggers listed successfully")
	}

	// Clean up trigger on exit
	defer func() {
		if err := conn.TriggerDel(root, triggerName); err != nil {
			log.Error(fmt.Sprintf("Failed to delete trigger: %v", err))
		}
	}()
}

// logFileEvent logs a file event
func logFileEvent(log logger.Logger, event watchman.FileEvent) {
	eventType := ""
	switch event.Type {
	case watchman.FileCreated:
		eventType = "CREATED"
	case watchman.FileModified:
		eventType = "MODIFIED"
	case watchman.FileDeleted:
		eventType = "DELETED"
	case watchman.FileRenamed:
		eventType = "RENAMED"
	}

	relPath, _ := filepath.Rel(".", event.Path)

	if event.IsDir {
		log.Info(fmt.Sprintf("[%s] Directory: %s", eventType, relPath))
	} else {
		log.Info(fmt.Sprintf("[%s] File: %s (size: %d bytes)", eventType, relPath, event.Size))
	}
}

// waitForInterrupt waits for SIGINT or SIGTERM
func waitForInterrupt(log logger.Logger) {
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	log.Info("Watching for file changes... Press Ctrl+C to stop")
	<-sigChan
	log.Info("Shutting down...")
}

// Example usage instructions:
//
// 1. Build and run:
//    go build -o watchman-example examples/watchman_example.go
//    ./watchman-example
//
// 2. In another terminal, make changes:
//    touch test.go
//    echo "package main" > test.go
//    rm test.go
//
// 3. Observe the file change events being logged
//
// The example will automatically use Watchman if available,
// otherwise it falls back to fsnotify.

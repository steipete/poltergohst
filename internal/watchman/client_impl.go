// Package watchman provides the complete Watchman client implementation
package watchman

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/poltergeist/poltergeist/pkg/interfaces"
	"github.com/poltergeist/poltergeist/pkg/logger"
	"github.com/poltergeist/poltergeist/pkg/types"
)

// UnifiedClient provides file watching with Watchman or fsnotify fallback
type UnifiedClient struct {
	logger          logger.Logger
	watchmanConn    *WatchmanConnection
	fsnotifyWatcher *FSNotifyWatcher
	useWatchman     bool
	subscriptions   map[string]*subscription
	projectRoot     string
	config          *types.WatchmanConfig
	mu              sync.RWMutex
	ctx             context.Context
	cancel          context.CancelFunc
	eventChan       chan FileEvent
	settlingDelay   time.Duration
}

type subscription struct {
	name       string
	root       string
	expression []interface{}
	callback   interfaces.FileChangeCallback
	query      SubscriptionQuery
}

// NewUnifiedClient creates a new unified Watchman/fsnotify client
func NewUnifiedClient(log logger.Logger, config *types.WatchmanConfig) *UnifiedClient {
	ctx, cancel := context.WithCancel(context.Background())

	client := &UnifiedClient{
		logger:        log,
		subscriptions: make(map[string]*subscription),
		config:        config,
		ctx:           ctx,
		cancel:        cancel,
		eventChan:     make(chan FileEvent, 1000),
		settlingDelay: time.Duration(config.SettlingDelay) * time.Millisecond,
	}

	// Try to connect to Watchman
	if conn, err := Connect(); err == nil {
		// Verify connection with version check
		if version, err := conn.Version(); err == nil {
			client.watchmanConn = conn
			client.useWatchman = true
			log.Info(fmt.Sprintf("Connected to Watchman version %s", version))
		} else {
			conn.Close()
			log.Info("Watchman connection failed, using fsnotify fallback")
		}
	} else {
		log.Info(fmt.Sprintf("Watchman not available (%v), using fsnotify fallback", err))
	}

	// Initialize fsnotify if Watchman not available
	if !client.useWatchman {
		if watcher, err := NewFSNotifyWatcher(log); err == nil {
			client.fsnotifyWatcher = watcher

			// Configure fsnotify with config settings
			if config.ExcludeDirs != nil {
				client.fsnotifyWatcher.SetExclusions(config.ExcludeDirs)
			}
			if config.SettlingDelay > 0 {
				client.fsnotifyWatcher.SetSettlingDelay(time.Duration(config.SettlingDelay) * time.Millisecond)
			}
		} else {
			log.Error(fmt.Sprintf("Failed to create fsnotify watcher: %v", err))
		}
	}

	// Start event processor
	go client.processEvents()

	// Start receiving Watchman events if connected
	if client.useWatchman && client.watchmanConn != nil {
		go client.receiveWatchmanEvents()
	}

	return client
}

// Connect establishes connection to the file watcher
func (c *UnifiedClient) Connect(ctx context.Context) error {
	if c.useWatchman && c.watchmanConn != nil {
		// Already connected
		return nil
	}

	if !c.useWatchman && c.fsnotifyWatcher != nil {
		// Using fsnotify, already initialized
		return nil
	}

	return fmt.Errorf("no file watcher available")
}

// Disconnect closes the connection
func (c *UnifiedClient) Disconnect() error {
	c.cancel()

	if c.watchmanConn != nil {
		return c.watchmanConn.Close()
	}

	if c.fsnotifyWatcher != nil {
		return c.fsnotifyWatcher.Close()
	}

	return nil
}

// WatchProject sets up watching for a project
func (c *UnifiedClient) WatchProject(projectPath string) error {
	c.mu.Lock()
	c.projectRoot = projectPath
	c.mu.Unlock()

	if c.useWatchman {
		resp, err := c.watchmanConn.WatchProject(projectPath)
		if err != nil {
			return fmt.Errorf("failed to watch project: %w", err)
		}

		c.mu.Lock()
		if resp.RelativeRoot != "" {
			c.projectRoot = filepath.Join(resp.Watch, resp.RelativeRoot)
		} else {
			c.projectRoot = resp.Watch
		}
		c.mu.Unlock()

		c.logger.Info(fmt.Sprintf("Watching project with Watchman: %s", c.projectRoot))
	} else if c.fsnotifyWatcher != nil {
		// Set up fsnotify watching
		err := c.fsnotifyWatcher.WatchProject(projectPath, func(event FileEvent) {
			c.eventChan <- event
		})
		if err != nil {
			return fmt.Errorf("failed to watch project with fsnotify: %w", err)
		}

		c.logger.Info(fmt.Sprintf("Watching project with fsnotify: %s", projectPath))
	} else {
		return fmt.Errorf("no file watcher available")
	}

	return nil
}

// Subscribe creates a subscription for file changes
func (c *UnifiedClient) Subscribe(
	root string,
	name string,
	config interfaces.SubscriptionConfig,
	callback interfaces.FileChangeCallback,
	exclusions []interfaces.ExclusionExpression,
) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Create subscription record
	sub := &subscription{
		name:       name,
		root:       root,
		expression: config.Expression,
		callback:   callback,
	}

	if c.useWatchman {
		// Build Watchman query
		var expressions []Expression
		var finalExpr Expression

		// Use provided expression if available
		if len(config.Expression) > 0 {
			// Convert []interface{} to Expression
			// This is a pass-through since the expression is already built
			finalExpr = config.Expression
		} else {
			// Build expression from patterns
			// Add pattern expressions
			for _, pattern := range []string{} {
				if strings.Contains(pattern, "**") {
					// Handle recursive patterns
					expressions = append(expressions, MatchExpression(pattern, true))
				} else {
					expressions = append(expressions, MatchExpression(pattern, false))
				}
			}

			// Add exclusions
			var exclusionExprs []Expression
			for _, exc := range exclusions {
				if exc.Type == "dir" {
					for _, pattern := range exc.Patterns {
						exclusionExprs = append(exclusionExprs,
							MatchExpression(fmt.Sprintf("**/%s/**", pattern), true))
					}
				} else {
					for _, pattern := range exc.Patterns {
						exclusionExprs = append(exclusionExprs,
							MatchExpression(pattern, false))
					}
				}
			}

			// Add default exclusions from config
			if c.config.UseDefaultExclusions {
				for _, dir := range getDefaultExclusions() {
					exclusionExprs = append(exclusionExprs,
						MatchExpression(fmt.Sprintf("**/%s/**", dir), true))
				}
			}

			// Build final expression
			if len(expressions) > 0 && len(exclusionExprs) > 0 {
				finalExpr = AllOfExpression(
					AnyOfExpression(expressions...),
					NotExpression(AnyOfExpression(exclusionExprs...)),
				)
			} else if len(expressions) > 0 {
				finalExpr = AnyOfExpression(expressions...)
			} else {
				finalExpr = MatchExpression("**", true) // Watch everything
			}
		}

		// Get initial clock
		clock, err := c.watchmanConn.Clock(root)
		if err != nil {
			c.logger.Warn(fmt.Sprintf("Failed to get clock: %v", err))
			clock = ""
		}

		// Create Watchman subscription
		sub.query = SubscriptionQuery{
			Expression: finalExpr,
			Fields:     []string{"name", "size", "mtime_ms", "exists", "type", "new"},
			Since:      clock,
			Empty:      true,
		}

		if _, err := c.watchmanConn.Subscribe(root, name, sub.query); err != nil {
			return fmt.Errorf("failed to create Watchman subscription: %w", err)
		}

	} else if c.fsnotifyWatcher != nil {
		// Configure fsnotify patterns from expression
		// Extract patterns from expression if possible
		patterns := extractPatternsFromExpression(config.Expression)
		c.fsnotifyWatcher.SetPatterns(patterns)

		// Note: fsnotify is already set up in WatchProject
		// The callback is registered there
	}

	c.subscriptions[name] = sub
	c.logger.Debug(fmt.Sprintf("Created subscription: %s", name))

	return nil
}

// Unsubscribe removes a subscription
func (c *UnifiedClient) Unsubscribe(subscriptionName string) error {
	c.mu.Lock()
	sub, exists := c.subscriptions[subscriptionName]
	if !exists {
		c.mu.Unlock()
		return fmt.Errorf("subscription %s not found", subscriptionName)
	}
	delete(c.subscriptions, subscriptionName)
	c.mu.Unlock()

	if c.useWatchman && c.watchmanConn != nil {
		err := c.watchmanConn.Unsubscribe(sub.root, subscriptionName)
		return err
	}

	return nil
}

// IsConnected checks if connected to file watcher
func (c *UnifiedClient) IsConnected() bool {
	if c.useWatchman {
		return c.watchmanConn != nil
	}
	return c.fsnotifyWatcher != nil
}

// GetVersion returns the Watchman version or "fsnotify" for fallback
func (c *UnifiedClient) GetVersion() (string, error) {
	if c.useWatchman && c.watchmanConn != nil {
		return c.watchmanConn.Version()
	}
	return "fsnotify", nil
}

// receiveWatchmanEvents receives events from Watchman
func (c *UnifiedClient) receiveWatchmanEvents() {
	c.logger.Debug("Starting Watchman event receiver")

	for {
		select {
		case <-c.ctx.Done():
			c.logger.Debug("Watchman event receiver shutting down (context cancelled)")
			return
		default:
			if c.watchmanConn == nil {
				c.logger.Debug("Watchman connection lost, stopping event receiver")
				return
			}

			// Set read timeout to avoid blocking forever
			resp, err := c.watchmanConn.Receive()
			if err != nil {
				if strings.Contains(err.Error(), "EOF") || strings.Contains(err.Error(), "closed") {
					c.logger.Debug("Watchman connection closed, stopping event receiver")
					return
				}
				// Log the error but continue trying to receive events
				c.logger.Debug(fmt.Sprintf("Error receiving Watchman event (will retry): %v", err))

				// Small delay to avoid tight loop on persistent errors
				time.Sleep(100 * time.Millisecond)
				continue
			}

			// Process subscription notification
			if resp.Subscription != "" {
				c.logger.Debug(fmt.Sprintf("Received Watchman subscription event: %s with %d files",
					resp.Subscription, len(resp.Files)))
				c.handleWatchmanResponse(resp)
			} else if resp.Log != "" {
				// Log Watchman's log messages
				c.logger.Debug(fmt.Sprintf("Watchman log: %s", resp.Log))
			}
		}
	}
}

// handleWatchmanResponse processes a Watchman response
func (c *UnifiedClient) handleWatchmanResponse(resp *WatchmanResponse) {
	c.mu.RLock()
	sub, exists := c.subscriptions[resp.Subscription]
	c.mu.RUnlock()

	if !exists {
		c.logger.Debug(fmt.Sprintf("Received event for unknown subscription: %s", resp.Subscription))
		return
	}

	c.logger.Debug(fmt.Sprintf("Processing %d file changes for subscription %s", len(resp.Files), resp.Subscription))

	// Convert Watchman files to events
	for _, file := range resp.Files {
		event := ConvertWatchmanFile(resp.Root, file)
		c.logger.Debug(fmt.Sprintf("File event: %s (%v)", event.Path, event.Type))
		c.eventChan <- event
	}

	// Log that we're queuing events
	if len(resp.Files) > 0 {
		c.logger.Debug(fmt.Sprintf("Queued %d events for processing (subscription: %s)",
			len(resp.Files), sub.name))
	}
}

// processEvents processes file events with settling
func (c *UnifiedClient) processEvents() {
	pendingEvents := make(map[string]*FileEvent)
	timers := make(map[string]*time.Timer)

	c.logger.Debug(fmt.Sprintf("Event processor started with settling delay: %v", c.settlingDelay))

	for {
		select {
		case <-c.ctx.Done():
			c.logger.Debug("Event processor shutting down")
			return

		case event := <-c.eventChan:
			c.logger.Debug(fmt.Sprintf("Processing event for: %s (type: %v)", event.Path, event.Type))

			// Cancel existing timer if present
			if timer, exists := timers[event.Path]; exists {
				timer.Stop()
				delete(timers, event.Path)
				c.logger.Debug(fmt.Sprintf("Cancelled existing timer for: %s", event.Path))
			}

			// Store pending event
			pendingEvents[event.Path] = &event

			// Schedule dispatch after settling delay
			eventPath := event.Path // Capture for closure
			timer := time.AfterFunc(c.settlingDelay, func() {
				c.mu.Lock()
				delete(timers, eventPath)
				if pendingEvent, exists := pendingEvents[eventPath]; exists {
					delete(pendingEvents, eventPath)
					c.mu.Unlock()
					c.logger.Debug(fmt.Sprintf("Settling delay expired, dispatching event for: %s", eventPath))
					c.dispatchEvent(*pendingEvent)
				} else {
					c.mu.Unlock()
				}
			})
			timers[event.Path] = timer
			c.logger.Debug(fmt.Sprintf("Scheduled dispatch for %s after %v", event.Path, c.settlingDelay))
		}
	}
}

// dispatchEvent dispatches an event to subscriptions
func (c *UnifiedClient) dispatchEvent(event FileEvent) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	c.logger.Debug(fmt.Sprintf("Dispatching event for file: %s", event.Path))

	// Find matching subscriptions
	matchCount := 0
	for _, sub := range c.subscriptions {
		if c.eventMatchesSubscription(event, sub) {
			matchCount++
			// Convert to interface type
			change := interfaces.FileChange{
				Name:   event.Path,
				Exists: event.Type != FileDeleted,
				Type:   getFileType(event),
			}

			// Call callback
			if sub.callback != nil {
				c.logger.Debug(fmt.Sprintf("Invoking callback for subscription: %s", sub.name))
				sub.callback([]interfaces.FileChange{change})
			} else {
				c.logger.Warn(fmt.Sprintf("No callback registered for subscription: %s", sub.name))
			}
		}
	}

	if matchCount == 0 {
		c.logger.Debug(fmt.Sprintf("No matching subscriptions for event: %s", event.Path))
	} else {
		c.logger.Debug(fmt.Sprintf("Event matched %d subscriptions", matchCount))
	}
}

// eventMatchesSubscription checks if an event matches a subscription
func (c *UnifiedClient) eventMatchesSubscription(event FileEvent, sub *subscription) bool {
	// Check if path is under subscription root
	if !strings.HasPrefix(event.Path, sub.root) {
		return false
	}

	// If subscription has expression, assume it matches (already filtered by Watchman)
	if len(sub.expression) > 0 && c.useWatchman {
		return true
	}

	// For fsnotify, check against patterns extracted from expression
	patterns := extractPatternsFromExpression(sub.expression)
	for _, pattern := range patterns {
		if matched, _ := filepath.Match(pattern, filepath.Base(event.Path)); matched {
			return true
		}

		// Handle ** patterns
		if strings.Contains(pattern, "**") {
			parts := strings.Split(pattern, "**")
			if len(parts) == 2 {
				prefix := parts[0]
				suffix := parts[1]
				relPath, _ := filepath.Rel(sub.root, event.Path)
				if strings.HasPrefix(relPath, prefix) {
					if suffix == "" || strings.HasSuffix(relPath, strings.TrimPrefix(suffix, "/")) {
						return true
					}
				}
			}
		}
	}

	return false
}

// getFileType returns the file type string
func getFileType(event FileEvent) string {
	if event.IsDir {
		return "d"
	}
	return "f"
}

// extractPatternsFromExpression extracts glob patterns from a Watchman expression
func extractPatternsFromExpression(expr []interface{}) []string {
	if len(expr) == 0 {
		return []string{"**"}
	}

	var patterns []string
	extractPatterns(expr, &patterns)

	if len(patterns) == 0 {
		patterns = []string{"**"}
	}

	return patterns
}

// extractPatterns recursively extracts patterns from expression
func extractPatterns(expr interface{}, patterns *[]string) {
	switch v := expr.(type) {
	case []interface{}:
		if len(v) > 0 {
			if cmd, ok := v[0].(string); ok {
				switch cmd {
				case "match":
					if len(v) > 1 {
						if pattern, ok := v[1].(string); ok {
							*patterns = append(*patterns, pattern)
						}
					}
				case "anyof", "allof":
					for i := 1; i < len(v); i++ {
						extractPatterns(v[i], patterns)
					}
				case "not":
					// Skip negations for pattern extraction
				}
			}
		}
	}
}

// getDefaultExclusions returns default directory exclusions
func getDefaultExclusions() []string {
	return []string{
		".git",
		".svn",
		".hg",
		".bzr",
		"node_modules",
		"vendor",
		".idea",
		".vscode",
		"__pycache__",
		".pytest_cache",
		"target",
		"build",
		"dist",
		"out",
		".poltergeist",
	}
}

// Watch is a high-level method to watch paths with a callback
func (c *UnifiedClient) Watch(ctx context.Context, root string, patterns []string, events chan FileEvent) error {
	// Set project root
	if err := c.WatchProject(root); err != nil {
		return err
	}

	// Build expression for patterns
	var expressions []Expression
	for _, pattern := range patterns {
		if strings.Contains(pattern, "**") {
			expressions = append(expressions, MatchExpression(pattern, true))
		} else {
			expressions = append(expressions, MatchExpression(pattern, false))
		}
	}

	var finalExpr []interface{}
	if len(expressions) > 0 {
		// AnyOfExpression returns []interface{} already
		finalExpr = AnyOfExpression(expressions...).([]interface{})
	} else {
		// MatchExpression returns []interface{} already
		finalExpr = MatchExpression("**", true).([]interface{})
	}

	// Create subscription
	config := interfaces.SubscriptionConfig{
		Expression: finalExpr,
		Fields:     []string{"name", "size", "mtime_ms", "exists", "type"},
	}

	callback := func(changes []interfaces.FileChange) {
		for _, change := range changes {
			event := FileEvent{
				Path:  change.Name,
				IsDir: change.Type == "d",
			}

			if change.Exists {
				event.Type = FileModified
			} else {
				event.Type = FileDeleted
			}

			// Check if it's a new file
			if change.Type == "f" && change.Exists {
				// Could check for "new" field from Watchman to determine if created
				event.Type = FileCreated
			}

			events <- event
		}
	}

	subscriptionName := fmt.Sprintf("watch-%d", time.Now().Unix())
	return c.Subscribe(root, subscriptionName, config, callback, nil)
}

// List returns all watched paths
func (c *UnifiedClient) List() []string {
	if c.fsnotifyWatcher != nil {
		return c.fsnotifyWatcher.List()
	}

	c.mu.RLock()
	defer c.mu.RUnlock()

	paths := make([]string, 0, len(c.subscriptions))
	for _, sub := range c.subscriptions {
		paths = append(paths, sub.root)
	}
	return paths
}

// Package watchman provides file watching capabilities
package watchman

import (
	"context"
	"time"

	"github.com/poltergeist/poltergeist/pkg/interfaces"
	"github.com/poltergeist/poltergeist/pkg/logger"
	"github.com/poltergeist/poltergeist/pkg/types"
)

// Client implements the WatchmanClient interface
type Client struct {
	impl *UnifiedClient
}

// NewClient creates a new watchman client
func NewClient(log logger.Logger) *Client {
	// Create default config if not provided
	config := &types.WatchmanConfig{
		UseDefaultExclusions: true,
		SettlingDelay:       1000,
		MaxFileEvents:       1000,
	}
	
	return &Client{
		impl: NewUnifiedClient(log, config),
	}
}

// NewClientWithConfig creates a new watchman client with configuration
func NewClientWithConfig(log logger.Logger, config *types.WatchmanConfig) *Client {
	return &Client{
		impl: NewUnifiedClient(log, config),
	}
}

// Connect establishes connection to watchman
func (c *Client) Connect(ctx context.Context) error {
	return c.impl.Connect(ctx)
}

// Disconnect closes the watchman connection
func (c *Client) Disconnect() error {
	return c.impl.Disconnect()
}

// WatchProject sets up watching for a project
func (c *Client) WatchProject(projectPath string) error {
	return c.impl.WatchProject(projectPath)
}

// Subscribe creates a subscription for file changes
func (c *Client) Subscribe(
	root string,
	name string,
	config interfaces.SubscriptionConfig,
	callback interfaces.FileChangeCallback,
	exclusions []interfaces.ExclusionExpression,
) error {
	return c.impl.Subscribe(root, name, config, callback, exclusions)
}

// Unsubscribe removes a subscription
func (c *Client) Unsubscribe(subscriptionName string) error {
	return c.impl.Unsubscribe(subscriptionName)
}

// IsConnected checks if connected to watchman
func (c *Client) IsConnected() bool {
	return c.impl.IsConnected()
}

// GetVersion returns the watchman version
func (c *Client) GetVersion() (string, error) {
	return c.impl.GetVersion()
}

// Watch is a simplified method to watch paths
func (c *Client) Watch(ctx context.Context, root string, patterns []string, events chan FileEvent) error {
	return c.impl.Watch(ctx, root, patterns, events)
}

// Subscription represents a file watch subscription
type Subscription struct {
	Name       string
	Root       string
	Expression []interface{}
}

// FallbackWatcher provides fsnotify-based file watching when Watchman is unavailable
type FallbackWatcher struct {
	impl *FSNotifyWatcher
}

// NewFallbackWatcher creates a new fallback watcher
func NewFallbackWatcher(log logger.Logger) (*FallbackWatcher, error) {
	impl, err := NewFSNotifyWatcher(log)
	if err != nil {
		return nil, err
	}
	
	return &FallbackWatcher{impl: impl}, nil
}

// Watch starts watching a directory
func (f *FallbackWatcher) Watch(ctx context.Context, root string, patterns []string, events chan FileEvent) error {
	f.impl.SetPatterns(patterns)
	
	callback := func(event FileEvent) {
		select {
		case events <- event:
		case <-ctx.Done():
			return
		}
	}
	
	return f.impl.WatchProject(root, callback)
}

// Close closes the watcher
func (f *FallbackWatcher) Close() error {
	return f.impl.Close()
}

// SetConfig sets the watcher configuration
func (f *FallbackWatcher) SetConfig(config *types.WatchmanConfig) {
	if config == nil {
		return
	}
	
	if config.SettlingDelay > 0 {
		f.impl.SetSettlingDelay(time.Duration(config.SettlingDelay) * time.Millisecond)
	}
	
	if len(config.ExcludeDirs) > 0 {
		f.impl.SetExclusions(config.ExcludeDirs)
	}
}
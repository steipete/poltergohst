package cli

import (
	"context"
	"time"
)

// Config holds all CLI configuration, making it testable and eliminating globals.
// This follows Go best practices for dependency injection and testability.
type Config struct {
	ConfigFile  string
	ProjectRoot string
	Verbosity   string
	Version     string
}

// NewConfig creates a new CLI configuration with defaults
func NewConfig() *Config {
	return &Config{
		ProjectRoot: ".",
		Verbosity:   "info",
	}
}

// RuntimeConfig holds runtime configuration for commands
type RuntimeConfig struct {
	Config      *Config
	Context     context.Context
	StartTime   time.Time
	RequestID   string
}

// NewRuntimeConfig creates a runtime configuration with context
func NewRuntimeConfig(cfg *Config, ctx context.Context) *RuntimeConfig {
	if ctx == nil {
		ctx = context.Background()
	}
	
	return &RuntimeConfig{
		Config:    cfg,
		Context:   ctx,
		StartTime: time.Now(),
		RequestID: generateRequestID(),
	}
}

// WithTimeout creates a new context with timeout
func (rc *RuntimeConfig) WithTimeout(timeout time.Duration) (context.Context, context.CancelFunc) {
	return context.WithTimeout(rc.Context, timeout)
}

// generateRequestID creates a unique request ID for tracing
func generateRequestID() string {
	return "req_" + time.Now().Format("20060102_150405")
}
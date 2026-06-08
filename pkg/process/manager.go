// Package process provides process management utilities
package process

import (
	"context"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/poltergeist/poltergeist/pkg/logger"
)

// Manager handles process lifecycle and signals
type Manager struct {
	logger           logger.Logger
	shutdownHandlers []func()
	heartbeatFunc    func()
	heartbeatStop    chan struct{}
	// Removed ctx and cancel - contexts should be passed as parameters
	wg      sync.WaitGroup
	mu      sync.Mutex
	running bool
}

// NewManager creates a new process manager
func NewManager(log logger.Logger) *Manager {
	return &Manager{
		logger:           log,
		shutdownHandlers: make([]func(), 0),
		running:          false,
	}
}

// RegisterShutdownHandler adds a shutdown handler
func (m *Manager) RegisterShutdownHandler(handler func()) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.shutdownHandlers = append(m.shutdownHandlers, handler)
}

// Start starts the process manager with the given context.
// The context controls the lifetime of the manager.
func (m *Manager) Start(ctx context.Context) {
	m.mu.Lock()
	if m.running {
		m.mu.Unlock()
		return
	}
	m.running = true
	m.mu.Unlock()

	// Handle OS signals
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM, syscall.SIGHUP)

	m.wg.Add(1)
	go func() {
		defer m.wg.Done()

		select {
		case <-ctx.Done():
			m.handleShutdown()
		case sig := <-sigChan:
			m.logger.Info("Received signal", logger.WithField("signal", sig))
			m.handleShutdown()
		}
	}()

	// Start heartbeat if configured
	if m.heartbeatFunc != nil {
		m.startHeartbeat(ctx)
	}
}

// Stop stops the process manager
func (m *Manager) Stop() {
	m.mu.Lock()
	if !m.running {
		m.mu.Unlock()
		return
	}
	m.running = false
	m.mu.Unlock()

	// Stop heartbeat
	if m.heartbeatStop != nil {
		close(m.heartbeatStop)
	}

	// Wait for goroutines
	m.wg.Wait()
}

// IsRunning checks if the process manager is running
func (m *Manager) IsRunning() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.running
}

// SetHeartbeat sets the heartbeat function
func (m *Manager) SetHeartbeat(fn func()) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.heartbeatFunc = fn
}

// Private methods

func (m *Manager) handleShutdown() {
	m.logger.Info("Initiating graceful shutdown...")

	// Call shutdown handlers in reverse order
	m.mu.Lock()
	handlers := make([]func(), len(m.shutdownHandlers))
	copy(handlers, m.shutdownHandlers)
	m.running = false
	m.mu.Unlock()

	for i := len(handlers) - 1; i >= 0; i-- {
		handlers[i]()
	}
}

func (m *Manager) startHeartbeat(ctx context.Context) {
	m.heartbeatStop = make(chan struct{})
	interval := 10 * time.Second

	m.wg.Add(1)
	go func() {
		defer m.wg.Done()

		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		for {
			select {
			case <-m.heartbeatStop:
				return
			case <-ctx.Done():
				return
			case <-ticker.C:
				if m.heartbeatFunc != nil {
					m.heartbeatFunc()
				}
			}
		}
	}()
}

// ProcessInfo represents information about a running process
type ProcessInfo struct {
	PID       int
	StartTime time.Time
	IsRunning bool
	Command   string
}

// GetProcessInfo returns information about a process
func GetProcessInfo(pid int) (*ProcessInfo, error) {
	proc, err := os.FindProcess(pid)
	if err != nil {
		return nil, err
	}

	// Check if process is running
	err = proc.Signal(syscall.Signal(0))
	isRunning := err == nil

	return &ProcessInfo{
		PID:       pid,
		IsRunning: isRunning,
		StartTime: time.Now(), // Simplified - would need platform-specific code
	}, nil
}

// KillProcess terminates a process
func KillProcess(pid int) error {
	proc, err := os.FindProcess(pid)
	if err != nil {
		return err
	}

	// Try graceful shutdown first
	if err := proc.Signal(syscall.SIGTERM); err != nil {
		// Force kill if graceful fails
		return proc.Kill()
	}

	// Wait a bit for graceful shutdown
	time.Sleep(2 * time.Second)

	// Check if still running
	if err := proc.Signal(syscall.Signal(0)); err == nil {
		// Still running, force kill
		return proc.Kill()
	}

	return nil
}

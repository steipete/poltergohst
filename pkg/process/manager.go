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
	stop    chan struct{}
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
	stop := make(chan struct{})
	m.stop = stop
	var heartbeatStop chan struct{}
	m.wg.Add(1)
	if m.heartbeatFunc != nil {
		heartbeatStop = make(chan struct{})
		m.heartbeatStop = heartbeatStop
		m.wg.Add(1)
	}
	m.mu.Unlock()

	// Handle OS signals
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM, syscall.SIGHUP)

	go func() {
		defer m.wg.Done()
		defer signal.Stop(sigChan)

		select {
		case <-ctx.Done():
			if m.beginShutdown() {
				m.handleShutdown()
			}
		case sig := <-sigChan:
			if m.beginShutdown() {
				m.logger.Info("Received signal", logger.WithField("signal", sig))
				m.handleShutdown()
			}
		case <-stop:
		}
	}()

	// Start heartbeat if configured
	if heartbeatStop != nil {
		go m.runHeartbeat(ctx, heartbeatStop)
	}
}

// Stop stops the process manager
func (m *Manager) Stop() {
	if !m.beginShutdown() {
		return
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
	m.mu.Unlock()

	for i := len(handlers) - 1; i >= 0; i-- {
		handlers[i]()
	}
}

func (m *Manager) beginShutdown() bool {
	m.mu.Lock()
	if !m.running {
		m.mu.Unlock()
		return false
	}

	m.running = false
	stop := m.stop
	m.stop = nil
	heartbeatStop := m.heartbeatStop
	m.heartbeatStop = nil
	m.mu.Unlock()

	close(stop)
	if heartbeatStop != nil {
		close(heartbeatStop)
	}

	return true
}

func (m *Manager) runHeartbeat(ctx context.Context, stop <-chan struct{}) {
	defer m.wg.Done()

	interval := 10 * time.Second
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-stop:
			return
		case <-ctx.Done():
			return
		case <-ticker.C:
			m.mu.Lock()
			heartbeat := m.heartbeatFunc
			m.mu.Unlock()
			if heartbeat != nil {
				heartbeat()
			}
		}
	}
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

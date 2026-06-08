// Package daemon provides background daemon functionality
package daemon

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/poltergeist/poltergeist/pkg/interfaces"
	"github.com/poltergeist/poltergeist/pkg/logger"
	"github.com/poltergeist/poltergeist/pkg/poltergeist"
	"github.com/poltergeist/poltergeist/pkg/process"
	"github.com/poltergeist/poltergeist/pkg/types"
)

// Manager manages the Poltergeist daemon
type Manager struct {
	projectRoot    string
	configPath     string
	pidFile        string
	logFile        string
	stateDir       string
	logger         logger.Logger
	processManager *process.Manager
	poltergeist    *poltergeist.Poltergeist
	mu             sync.RWMutex
}

// Config represents daemon configuration
type Config struct {
	ProjectRoot string
	ConfigPath  string
	LogFile     string
	LogLevel    string
}

// NewManager creates a new daemon manager
func NewManager(config Config) *Manager {
	stateDir := filepath.Join(config.ProjectRoot, ".poltergeist")
	pidFile := filepath.Join(stateDir, "daemon.pid")

	// Create logger
	log := logger.CreateLogger(config.LogFile, config.LogLevel)

	return &Manager{
		projectRoot:    config.ProjectRoot,
		configPath:     config.ConfigPath,
		pidFile:        pidFile,
		logFile:        config.LogFile,
		stateDir:       stateDir,
		logger:         log,
		processManager: process.NewManager(log),
	}
}

// StartWithContext starts the daemon with the given context
func (m *Manager) StartWithContext(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Check if already running
	if m.isRunning() {
		return ErrDaemonAlreadyRunning
	}

	// Ensure state directory exists
	if err := os.MkdirAll(m.stateDir, 0755); err != nil {
		return fmt.Errorf("failed to create state directory: %w", err)
	}

	// Write PID file
	if err := m.writePIDFile(); err != nil {
		return fmt.Errorf("failed to write PID file: %w", err)
	}

	// Load configuration
	cfg, err := m.loadConfig()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// Create dependency factory and build dependencies
	factory := poltergeist.NewDependencyFactory(m.projectRoot, m.logger, cfg)
	deps := factory.CreateDefaults()

	// Create Poltergeist instance with properly injected dependencies
	m.poltergeist = poltergeist.New(cfg, m.projectRoot, m.logger, deps, m.configPath)

	// Register shutdown handler
	m.processManager.RegisterShutdownHandler(func() {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		m.StopWithContext(shutdownCtx)
	})

	// Start process manager with context
	m.processManager.Start(ctx)

	// Start Poltergeist with context
	if err := m.poltergeist.StartWithContext(ctx, ""); err != nil {
		m.removePIDFile()
		return fmt.Errorf("failed to start Poltergeist: %w", err)
	}

	m.logger.Info("Daemon started successfully")

	// Run in background with context
	go m.runWithContext(ctx)

	return nil
}

// Start starts the daemon (deprecated - use StartWithContext)
func (m *Manager) Start() error {
	return m.StartWithContext(context.Background())
}

// StopWithContext stops the daemon with the given context for timeout control
func (m *Manager) StopWithContext(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if !m.isRunning() {
		return ErrDaemonNotRunning
	}

	m.logger.Info("Stopping daemon...")

	// Stop Poltergeist with context
	if m.poltergeist != nil {
		m.poltergeist.StopWithContext(ctx)
		m.poltergeist.Cleanup()
	}

	// Stop process manager
	m.processManager.Stop()

	// Remove PID file
	m.removePIDFile()

	m.logger.Info("Daemon stopped")

	return nil
}

// Stop stops the daemon (deprecated - use StopWithContext)
func (m *Manager) Stop() error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	return m.StopWithContext(ctx)
}

// Restart restarts the daemon
func (m *Manager) Restart() error {
	if err := m.Stop(); err != nil {
		// Ignore error if not running
		if !errors.Is(err, ErrDaemonNotRunning) {
			return err
		}
	}

	// Wait a moment
	time.Sleep(2 * time.Second)

	return m.Start()
}

// Status returns the daemon status
func (m *Manager) Status() (*Status, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if !m.isRunning() {
		return nil, nil
	}

	status := &Status{
		Running: true,
	}

	// Daemon is running, get additional info
	pid, err := m.readPIDFile()
	if err == nil {
		status.PID = pid

		// Get process info
		if info, err := process.GetProcessInfo(pid); err == nil {
			status.StartTime = info.StartTime
		}
	}

	// Get target information
	if m.poltergeist != nil {
		// TODO: Get targets from Poltergeist
		status.Targets = []string{}
	}

	return status, nil
}

// IsRunning checks if the daemon is running
func (m *Manager) IsRunning() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.isRunning()
}

// Private methods

func (m *Manager) runWithContext(ctx context.Context) {
	// Keep daemon running until context is cancelled
	<-ctx.Done()
	m.logger.Info("Daemon context cancelled", logger.WithField("reason", ctx.Err()))
}

// Deprecated: Use runWithContext instead
func (m *Manager) run() {
	// Keep daemon running
	select {}
}

func (m *Manager) isRunning() bool {
	pid, err := m.readPIDFile()
	if err != nil {
		return false
	}

	// Check if process exists
	proc, err := os.FindProcess(pid)
	if err != nil {
		return false
	}

	// Check if process is alive
	err = proc.Signal(os.Signal(nil))
	return err == nil
}

func (m *Manager) writePIDFile() error {
	pid := os.Getpid()
	return os.WriteFile(m.pidFile, []byte(fmt.Sprintf("%d", pid)), 0644)
}

func (m *Manager) readPIDFile() (int, error) {
	data, err := os.ReadFile(m.pidFile)
	if err != nil {
		return 0, err
	}

	var pid int
	if _, err := fmt.Sscanf(string(data), "%d", &pid); err != nil {
		return 0, err
	}

	return pid, nil
}

func (m *Manager) removePIDFile() {
	os.Remove(m.pidFile)
}

func (m *Manager) loadConfig() (*types.PoltergeistConfig, error) {
	data, err := os.ReadFile(m.configPath)
	if err != nil {
		return nil, err
	}

	var cfg types.PoltergeistConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}

	return &cfg, nil
}

// Status represents daemon status
type Status struct {
	Running   bool
	PID       int
	StartTime time.Time
	Targets   []string
	Builds    int
	Errors    int
}

// Worker represents a daemon worker process
type Worker struct {
	id         string
	targetName string
	builder    interfaces.Builder
	logger     logger.Logger
	ctx        context.Context
	cancel     context.CancelFunc
	wg         sync.WaitGroup
}

// NewWorker creates a new worker
func NewWorker(id string, targetName string, builder interfaces.Builder, log logger.Logger) *Worker {
	ctx, cancel := context.WithCancel(context.Background())

	return &Worker{
		id:         id,
		targetName: targetName,
		builder:    builder,
		logger:     log.WithTarget(targetName),
		ctx:        ctx,
		cancel:     cancel,
	}
}

// Start starts the worker
func (w *Worker) Start() {
	w.wg.Add(1)
	go w.run()
}

// Stop stops the worker
func (w *Worker) Stop() {
	w.cancel()
	w.wg.Wait()
}

// run is the worker's main loop
func (w *Worker) run() {
	defer w.wg.Done()

	w.logger.Info("Worker started", logger.WithField("id", w.id))

	for {
		select {
		case <-w.ctx.Done():
			w.logger.Info("Worker stopped", logger.WithField("id", w.id))
			return
		default:
			// TODO: Get build requests from queue
			time.Sleep(100 * time.Millisecond)
		}
	}
}

// Package builders provides build target implementations
package builders

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/poltergeist/poltergeist/pkg/interfaces"
	"github.com/poltergeist/poltergeist/pkg/logger"
	"github.com/poltergeist/poltergeist/pkg/types"
)

// BaseBuilder provides common functionality for all builders
type BaseBuilder struct {
	Target       types.Target
	ProjectRoot  string
	Logger       logger.Logger
	StateManager interfaces.StateManager
	
	lastBuildTime time.Duration
	totalBuilds   int
	successBuilds int
	mu            sync.RWMutex
}

// NewBaseBuilder creates a new base builder
func NewBaseBuilder(
	target types.Target,
	projectRoot string,
	log logger.Logger,
	stateManager interfaces.StateManager,
) *BaseBuilder {
	var targetLogger logger.Logger
	if log != nil {
		targetLogger = log.WithTarget(target.GetName())
	}
	
	return &BaseBuilder{
		Target:       target,
		ProjectRoot:  projectRoot,
		Logger:       targetLogger,
		StateManager: stateManager,
	}
}

// Validate validates the builder configuration
func (b *BaseBuilder) Validate() error {
	// Check if project root exists
	if _, err := os.Stat(b.ProjectRoot); os.IsNotExist(err) {
		return fmt.Errorf("project root does not exist: %s", b.ProjectRoot)
	}
	
	// Validate watch paths
	if len(b.Target.GetWatchPaths()) == 0 {
		return fmt.Errorf("no watch paths defined for target %s", b.Target.GetName())
	}
	
	// Validate build command
	if b.Target.GetBuildCommand() == "" {
		return fmt.Errorf("no build command defined for target %s", b.Target.GetName())
	}
	
	return nil
}

// Build executes the build command
func (b *BaseBuilder) Build(ctx context.Context, changedFiles []string) error {
	startTime := time.Now()
	defer func() {
		b.mu.Lock()
		b.lastBuildTime = time.Since(startTime)
		b.totalBuilds++
		b.mu.Unlock()
	}()
	
	// Prepare log file
	logFile, err := b.prepareLogFile()
	if err != nil {
		if b.Logger != nil {
			b.Logger.Warn(fmt.Sprintf("Failed to create log file: %v", err))
		}
	}
	defer func() {
		if logFile != nil {
			logFile.Close()
		}
	}()
	
	// Log build start
	timestamp := time.Now().Format("2006-01-02 15:04:05")
	b.logToFile(logFile, fmt.Sprintf("\n=== Build Started at %s ===\n", timestamp))
	
	if b.Logger != nil {
		b.Logger.Info(fmt.Sprintf("Building with %d changed files", len(changedFiles)))
		if len(changedFiles) > 0 {
			b.logToFile(logFile, fmt.Sprintf("Changed files: %v\n", changedFiles))
		}
	}
	
	// Execute build command
	buildCmd := b.Target.GetBuildCommand()
	cmd := b.createCommand(ctx, buildCmd)
	b.logToFile(logFile, fmt.Sprintf("Executing: %s\n", buildCmd))
	
	// Set environment variables
	if env := b.Target.GetEnvironment(); env != nil {
		cmd.Env = os.Environ()
		for k, v := range env {
			cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", k, v))
		}
	}
	
	// Set working directory
	cmd.Dir = b.ProjectRoot
	
	// Capture output with tee to log file
	var outputBuffer bytes.Buffer
	var multiWriter io.Writer = &outputBuffer
	if logFile != nil {
		multiWriter = io.MultiWriter(&outputBuffer, logFile)
	}
	
	// Create pipes for stdout and stderr
	cmd.Stdout = multiWriter
	cmd.Stderr = multiWriter
	
	// Run the command
	err = cmd.Run()
	output := outputBuffer.Bytes()
	
	// Log build result
	duration := time.Since(startTime)
	if err != nil {
		if b.Logger != nil {
			b.Logger.Error("Build failed", 
				logger.WithField("error", err),
				logger.WithField("output", string(output)))
		}
		b.logToFile(logFile, fmt.Sprintf("\n=== Build FAILED after %s ===\n", duration))
		b.logToFile(logFile, fmt.Sprintf("Error: %v\n", err))
		return fmt.Errorf("build failed: %w\n%s", err, output)
	}
	
	b.mu.Lock()
	b.successBuilds++
	b.mu.Unlock()
	
	if b.Logger != nil {
		b.Logger.Success(fmt.Sprintf("Build completed in %s", duration))
		
		if len(output) > 0 {
			b.Logger.Debug("Build output", logger.WithField("output", string(output)))
		}
	}
	
	b.logToFile(logFile, fmt.Sprintf("\n=== Build SUCCEEDED after %s ===\n", duration))
	
	return nil
}

// Clean performs cleanup operations
func (b *BaseBuilder) Clean() error {
	// Default implementation - can be overridden
	return nil
}

// GetTarget returns the target
func (b *BaseBuilder) GetTarget() types.Target {
	return b.Target
}

// GetLastBuildTime returns the last build duration
func (b *BaseBuilder) GetLastBuildTime() time.Duration {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.lastBuildTime
}

// GetSuccessRate returns the build success rate
func (b *BaseBuilder) GetSuccessRate() float64 {
	b.mu.RLock()
	defer b.mu.RUnlock()
	
	if b.totalBuilds == 0 {
		return 1.0
	}
	
	return float64(b.successBuilds) / float64(b.totalBuilds)
}

// createCommand creates an exec.Cmd from a command string
func (b *BaseBuilder) createCommand(ctx context.Context, command string) *exec.Cmd {
	// Parse command with shell
	var cmd *exec.Cmd
	if strings.Contains(command, "&&") || strings.Contains(command, "||") || 
	   strings.Contains(command, "|") || strings.Contains(command, ";") {
		// Complex command - use shell
		cmd = exec.CommandContext(ctx, "sh", "-c", command)
	} else {
		// Simple command - parse directly
		parts := strings.Fields(command)
		if len(parts) > 0 {
			cmd = exec.CommandContext(ctx, parts[0], parts[1:]...)
		} else {
			cmd = exec.CommandContext(ctx, "sh", "-c", command)
		}
	}
	
	return cmd
}

// resolvePath resolves a path relative to project root
func (b *BaseBuilder) resolvePath(path string) string {
	if filepath.IsAbs(path) {
		return path
	}
	return filepath.Join(b.ProjectRoot, path)
}

// fileExists checks if a file exists
func (b *BaseBuilder) fileExists(path string) bool {
	_, err := os.Stat(b.resolvePath(path))
	return err == nil
}

// prepareLogFile creates or opens the log file for this target
func (b *BaseBuilder) prepareLogFile() (*os.File, error) {
	// Create logs directory
	logDir := filepath.Join(b.ProjectRoot, ".poltergeist", "logs")
	if err := os.MkdirAll(logDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create log directory: %w", err)
	}
	
	// Open log file in append mode
	logPath := filepath.Join(logDir, fmt.Sprintf("%s.log", b.Target.GetName()))
	logFile, err := os.OpenFile(logPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return nil, fmt.Errorf("failed to open log file: %w", err)
	}
	
	return logFile, nil
}

// logToFile writes a message to the log file if available
func (b *BaseBuilder) logToFile(logFile *os.File, message string) {
	if logFile != nil {
		logFile.WriteString(message)
	}
}

// ExecutableBuilder builds executable targets
type ExecutableBuilder struct {
	*BaseBuilder
	outputPath string
}

// NewExecutableBuilder creates a new executable builder
func NewExecutableBuilder(
	target types.Target,
	projectRoot string,
	log logger.Logger,
	stateManager interfaces.StateManager,
) *ExecutableBuilder {
	base := NewBaseBuilder(target, projectRoot, log, stateManager)
	
	// Extract output path
	outputPath := ""
	if execTarget, ok := target.(*types.ExecutableTarget); ok {
		outputPath = execTarget.OutputPath
	}
	
	return &ExecutableBuilder{
		BaseBuilder: base,
		outputPath:  outputPath,
	}
}

// Validate validates the executable builder
func (b *ExecutableBuilder) Validate() error {
	if err := b.BaseBuilder.Validate(); err != nil {
		return err
	}
	
	if b.outputPath == "" {
		return fmt.Errorf("output path not specified for executable target %s", b.Target.GetName())
	}
	
	return nil
}

// Build builds the executable
func (b *ExecutableBuilder) Build(ctx context.Context, changedFiles []string) error {
	// Remove old executable if it exists
	outputPath := b.resolvePath(b.outputPath)
	if b.fileExists(outputPath) {
		if err := os.Remove(outputPath); err != nil {
			if b.Logger != nil {
				b.Logger.Warn("Failed to remove old executable", logger.WithField("error", err))
			}
		}
	}
	
	// Run the build
	if err := b.BaseBuilder.Build(ctx, changedFiles); err != nil {
		return err
	}
	
	// Verify output was created
	if !b.fileExists(outputPath) {
		return fmt.Errorf("build succeeded but output not found: %s", outputPath)
	}
	
	// Make executable
	if err := os.Chmod(outputPath, 0755); err != nil {
		if b.Logger != nil {
			b.Logger.Warn("Failed to make output executable", logger.WithField("error", err))
		}
	}
	
	return nil
}

// AppBundleBuilder builds app bundle targets
type AppBundleBuilder struct {
	*BaseBuilder
	bundleID      string
	platform      types.Platform
	autoRelaunch  bool
	launchCommand string
}

// NewAppBundleBuilder creates a new app bundle builder
func NewAppBundleBuilder(
	target types.Target,
	projectRoot string,
	log logger.Logger,
	stateManager interfaces.StateManager,
) *AppBundleBuilder {
	base := NewBaseBuilder(target, projectRoot, log, stateManager)
	
	builder := &AppBundleBuilder{
		BaseBuilder: base,
	}
	
	// Extract app bundle specific fields
	if appTarget, ok := target.(*types.AppBundleTarget); ok {
		builder.bundleID = appTarget.BundleID
		builder.platform = appTarget.Platform
		if appTarget.AutoRelaunch != nil {
			builder.autoRelaunch = *appTarget.AutoRelaunch
		}
		builder.launchCommand = appTarget.LaunchCommand
	}
	
	return builder
}

// Build builds the app bundle
func (b *AppBundleBuilder) Build(ctx context.Context, changedFiles []string) error {
	// Kill running app if auto-relaunch is enabled
	if b.autoRelaunch {
		b.killRunningApp()
	}
	
	// Run the build
	if err := b.BaseBuilder.Build(ctx, changedFiles); err != nil {
		return err
	}
	
	// Relaunch app if enabled
	if b.autoRelaunch && b.launchCommand != "" {
		if err := b.launchApp(ctx); err != nil {
			if b.Logger != nil {
				b.Logger.Warn("Failed to relaunch app", logger.WithField("error", err))
			}
		}
	}
	
	return nil
}

func (b *AppBundleBuilder) killRunningApp() {
	if b.bundleID == "" {
		return
	}
	
	// Use pkill or killall to terminate app by bundle ID
	cmd := exec.Command("pkill", "-f", b.bundleID)
	if err := cmd.Run(); err != nil {
		// Try alternative methods
		cmd = exec.Command("killall", "-9", b.bundleID)
		cmd.Run() // Ignore error - app might not be running
	}
}

func (b *AppBundleBuilder) launchApp(ctx context.Context) error {
	if b.launchCommand == "" {
		return nil
	}
	
	cmd := b.createCommand(ctx, b.launchCommand)
	
	// Run in background
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to launch app: %w", err)
	}
	
	// Detach from process
	go cmd.Wait()
	
	if b.Logger != nil {
		b.Logger.Info("App relaunched successfully")
	}
	return nil
}

// LibraryBuilder builds library targets
type LibraryBuilder struct {
	*BaseBuilder
	outputPath  string
	libraryType types.LibraryType
}

// NewLibraryBuilder creates a new library builder
func NewLibraryBuilder(
	target types.Target,
	projectRoot string,
	log logger.Logger,
	stateManager interfaces.StateManager,
) *LibraryBuilder {
	base := NewBaseBuilder(target, projectRoot, log, stateManager)
	
	builder := &LibraryBuilder{
		BaseBuilder: base,
	}
	
	// Extract library specific fields
	if libTarget, ok := target.(*types.LibraryTarget); ok {
		builder.outputPath = libTarget.OutputPath
		builder.libraryType = libTarget.LibraryType
	}
	
	return builder
}

// DockerBuilder builds Docker images
type DockerBuilder struct {
	*BaseBuilder
	imageName  string
	dockerfile string
	context    string
	tags       []string
}

// NewDockerBuilder creates a new Docker builder
func NewDockerBuilder(
	target types.Target,
	projectRoot string,
	log logger.Logger,
	stateManager interfaces.StateManager,
) *DockerBuilder {
	base := NewBaseBuilder(target, projectRoot, log, stateManager)
	
	builder := &DockerBuilder{
		BaseBuilder: base,
		context:     ".",
		dockerfile:  "Dockerfile",
	}
	
	// Extract Docker specific fields
	if dockerTarget, ok := target.(*types.DockerTarget); ok {
		builder.imageName = dockerTarget.ImageName
		if dockerTarget.Dockerfile != "" {
			builder.dockerfile = dockerTarget.Dockerfile
		}
		if dockerTarget.Context != "" {
			builder.context = dockerTarget.Context
		}
		builder.tags = dockerTarget.Tags
	}
	
	return builder
}

// Build builds the Docker image
func (b *DockerBuilder) Build(ctx context.Context, changedFiles []string) error {
	// Build the Docker image with proper tags
	buildCmd := fmt.Sprintf("docker build -f %s -t %s", b.dockerfile, b.imageName)
	
	// Add additional tags
	for _, tag := range b.tags {
		buildCmd += fmt.Sprintf(" -t %s:%s", b.imageName, tag)
	}
	
	// Add context
	buildCmd += fmt.Sprintf(" %s", b.context)
	
	// Override the build command
	originalCmd := b.Target.GetBuildCommand()
	b.Target.(*types.DockerTarget).BuildCommand = buildCmd
	
	// Run the build
	err := b.BaseBuilder.Build(ctx, changedFiles)
	
	// Restore original command
	b.Target.(*types.DockerTarget).BuildCommand = originalCmd
	
	return err
}

// TestBuilder builds and runs test targets
type TestBuilder struct {
	*BaseBuilder
	testCommand  string
	coverageFile string
}

// NewTestBuilder creates a new test builder
func NewTestBuilder(
	target types.Target,
	projectRoot string,
	log logger.Logger,
	stateManager interfaces.StateManager,
) *TestBuilder {
	base := NewBaseBuilder(target, projectRoot, log, stateManager)
	
	builder := &TestBuilder{
		BaseBuilder: base,
	}
	
	// Extract test specific fields
	if testTarget, ok := target.(*types.TestTarget); ok {
		builder.testCommand = testTarget.TestCommand
		builder.coverageFile = testTarget.CoverageFile
	}
	
	return builder
}

// Build runs the tests
func (b *TestBuilder) Build(ctx context.Context, changedFiles []string) error {
	// Use test command instead of build command
	originalCmd := b.Target.GetBuildCommand()
	if b.testCommand != "" {
		b.Target.(*types.TestTarget).BuildCommand = b.testCommand
	}
	
	// Run the tests
	err := b.BaseBuilder.Build(ctx, changedFiles)
	
	// Restore original command
	b.Target.(*types.TestTarget).BuildCommand = originalCmd
	
	// Check coverage file if specified
	if err == nil && b.coverageFile != "" {
		if b.fileExists(b.coverageFile) {
			if b.Logger != nil {
				b.Logger.Info("Coverage report generated", 
					logger.WithField("file", b.coverageFile))
			}
		}
	}
	
	return err
}
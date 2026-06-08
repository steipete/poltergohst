// Package cli provides the polter command for smart binary execution
package cli

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/fatih/color"
	"github.com/poltergeist/poltergeist/pkg/logger"
	"github.com/poltergeist/poltergeist/pkg/state"
	"github.com/poltergeist/poltergeist/pkg/types"
	"github.com/spf13/cobra"
)

var (
	polterTimeout  int
	polterForce    bool
	polterNoWait   bool
	polterVerbose  bool
	polterShowLogs bool
	polterLogLines int
)

// newPolterCmd creates the polter command
func newPolterCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "polter [target] [args...]",
		Short: "Smart wrapper for running executables managed by Poltergeist",
		Long: `Smart wrapper that ensures you never run stale or failed builds by:
  - Checking build status before execution
  - Waiting for in-progress builds to complete  
  - Failing fast on build errors with clear messages
  - Executing fresh binaries only when builds succeed`,
		Args:                  cobra.ArbitraryArgs,
		DisableFlagsInUseLine: true,
		RunE:                  runPolter,
	}

	cmd.Flags().IntVarP(&polterTimeout, "timeout", "t", 300000, "Build wait timeout in milliseconds")
	cmd.Flags().BoolVarP(&polterForce, "force", "f", false, "Run even if build failed")
	cmd.Flags().BoolVarP(&polterNoWait, "no-wait", "n", false, "Don't wait for builds, fail if building")
	cmd.Flags().BoolVar(&polterVerbose, "verbose", false, "Show detailed status information")
	cmd.Flags().BoolVar(&polterShowLogs, "logs", true, "Show build logs during progress")
	cmd.Flags().IntVar(&polterLogLines, "log-lines", 5, "Number of log lines to show")

	return cmd
}

func runPolter(cmd *cobra.Command, args []string) error {
	// Set up colors
	errorStyle := color.New(color.FgRed)
	warningStyle := color.New(color.FgYellow)
	successStyle := color.New(color.FgGreen)
	infoStyle := color.New(color.FgCyan)

	var targetName string
	var targetArgs []string

	if len(args) > 0 {
		targetName = args[0]
		if len(args) > 1 {
			targetArgs = args[1:]
		}
	}

	// Load configuration
	configPath := getConfigPath()
	cfg, err := loadConfig(configPath)
	if err != nil {
		errorStyle.Println("❌ Failed to load configuration:", err)
		return fmt.Errorf("failed to load configuration: %w", err)
	}

	// If no target specified, find first executable target
	if targetName == "" {
		for _, rawTarget := range cfg.Targets {
			target, err := types.ParseTarget(rawTarget)
			if err != nil {
				continue
			}
			if target.GetType() == types.TargetTypeExecutable && target.IsEnabled() {
				targetName = target.GetName()
				infoStyle.Printf("🎯 Using default target: %s\n", targetName)
				break
			}
		}

		if targetName == "" {
			errorStyle.Println("❌ No executable targets configured")
			warningStyle.Println("💡 Configure an executable target in poltergeist.config.json")
			return fmt.Errorf("no executable targets configured")
		}
	}

	// Find target in configuration
	var target types.Target
	for _, rawTarget := range cfg.Targets {
		t, err := types.ParseTarget(rawTarget)
		if err != nil {
			continue
		}
		if t.GetName() == targetName {
			target = t
			break
		}
	}

	if target == nil {
		// Try stale execution as fallback
		if polterVerbose {
			warningStyle.Printf("⚠️  Target '%s' not found in config - attempting stale execution\n", targetName)
		}
		exitCode := executeStaleWithWarning(targetName, projectRoot, targetArgs, errorStyle, warningStyle, successStyle, infoStyle)
		os.Exit(exitCode)
	}

	// Validate target type
	if target.GetType() != types.TargetTypeExecutable {
		errorStyle.Printf("❌ Target '%s' is not executable (type: %s)\n", targetName, target.GetType())
		warningStyle.Println("💡 polter only works with executable targets")
		return fmt.Errorf("target is not executable")
	}

	if polterVerbose {
		infoStyle.Printf("📍 Project root: %s\n", projectRoot)
		infoStyle.Printf("🎯 Target: %s\n", target.GetName())
	}

	// Check build status
	status := getBuildStatus(projectRoot, target)

	if polterVerbose {
		infoStyle.Printf("📊 Build status: %s\n", status)
	}

	// Handle different build states
	switch status {
	case "building":
		if polterNoWait {
			errorStyle.Println("❌ Build in progress and --no-wait specified")
			return fmt.Errorf("build in progress")
		}

		result := waitForBuildCompletion(projectRoot, target, time.Duration(polterTimeout)*time.Millisecond, successStyle, errorStyle)
		
		if result == "timeout" {
			errorStyle.Printf("❌ Build timeout after %dms\n", polterTimeout)
			warningStyle.Println("💡 Solutions:")
			fmt.Printf("   • Increase timeout: polter %s --timeout %d\n", targetName, polterTimeout*2)
			fmt.Println("   • Check build logs: poltergeist logs")
			fmt.Println("   • Verify Poltergeist is running: poltergeist status")
			return fmt.Errorf("build timeout")
		}

		if result == "failed" && !polterForce {
			errorStyle.Println("❌ Build failed")
			warningStyle.Println("💡 Options:")
			fmt.Println("   • Check build logs: poltergeist logs")
			fmt.Printf("   • Force execution anyway: polter %s --force\n", targetName)
			fmt.Println("   • Fix build errors and try again")
			return fmt.Errorf("build failed")
		}

		if result == "failed" && polterForce {
			warningStyle.Println("⚠️  Running despite build failure (--force specified)")
		}

	case "failed":
		if !polterForce {
			errorStyle.Println("❌ Last build failed")
			warningStyle.Println("🔧 Run `poltergeist logs` for details or use --force to run anyway")
			return fmt.Errorf("last build failed")
		}
		warningStyle.Println("⚠️  Running despite build failure (--force specified)")

	case "success":
		if polterVerbose {
			successStyle.Println("✅ Build successful")
		}

	case "unknown":
		warningStyle.Println("⚠️  Build status unknown, proceeding...")
	}

	// Execute the target
	exitCode := executeTarget(target, projectRoot, targetArgs, errorStyle, successStyle)
	if exitCode != 0 {
		return fmt.Errorf("execution failed with exit code %d", exitCode)
	}
	return nil
}

func getBuildStatus(projectRoot string, target types.Target) string {
	log := logger.CreateLogger("", verbosity)
	sm := state.NewStateManager(projectRoot, log)
	state, err := sm.ReadState(target.GetName())
	if err != nil || state == nil {
		return "unknown"
	}

	switch state.BuildStatus {
	case types.BuildStatusBuilding:
		return "building"
	case types.BuildStatusFailed:
		return "failed"
	case types.BuildStatusSucceeded:
		return "success"
	default:
		return "unknown"
	}
}

func waitForBuildCompletion(projectRoot string, target types.Target, timeout time.Duration, successStyle, errorStyle *color.Color) string {
	startTime := time.Now()
	fmt.Print("Build in progress")
	
	for time.Since(startTime) < timeout {
		status := getBuildStatus(projectRoot, target)
		
		elapsed := time.Since(startTime)
		fmt.Printf("\rBuild in progress... %.1fs", elapsed.Seconds())

		switch status {
		case "success":
			fmt.Println()
			successStyle.Println("✅ Build completed successfully")
			return "success"
		case "failed":
			fmt.Println()
			errorStyle.Println("❌ Build failed")
			return "failed"
		case "building":
			// Continue waiting
		default:
			// Build process died or status changed
			fmt.Println()
			return status
		}

		time.Sleep(250 * time.Millisecond)
	}

	fmt.Println()
	return "timeout"
}

func executeTarget(target types.Target, projectRoot string, args []string, errorStyle, successStyle *color.Color) int {
	// Get output path based on target type
	var binaryPath string
	
	// Check if target is an ExecutableTarget type
	if execTarget, ok := target.(*types.ExecutableTarget); ok && execTarget.OutputPath != "" {
		binaryPath = filepath.Join(projectRoot, execTarget.OutputPath)
	} else {
		errorStyle.Printf("❌ Target '%s' does not have an output path\n", target.GetName())
		return 1
	}

	if _, err := os.Stat(binaryPath); os.IsNotExist(err) {
		errorStyle.Printf("❌ Binary not found: %s\n", binaryPath)
		fmt.Println("🔧 Try running: poltergeist watch")
		return 1
	}

	successStyle.Printf("✅ Running fresh binary: %s\n", target.GetName())

	// Execute the binary
	cmd := exec.Command(binaryPath, args...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Dir = projectRoot

	if err := cmd.Run(); err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return exitErr.ExitCode()
		}
		errorStyle.Printf("❌ Failed to execute %s: %v\n", target.GetName(), err)
		return 1
	}

	return 0
}

func executeStaleWithWarning(targetName string, projectRoot string, args []string, 
	errorStyle, warningStyle, successStyle, infoStyle *color.Color) int {
	// Try common binary locations
	possiblePaths := []string{
		filepath.Join(projectRoot, targetName),
		filepath.Join(projectRoot, "build", targetName),
		filepath.Join(projectRoot, "dist", targetName),
		filepath.Join(projectRoot, targetName+".exe"),
		filepath.Join(projectRoot, "build", targetName+".exe"),
		filepath.Join(projectRoot, "dist", targetName+".exe"),
	}

	var binaryPath string
	for _, path := range possiblePaths {
		if _, err := os.Stat(path); err == nil {
			binaryPath = path
			break
		}
	}

	if binaryPath == "" {
		errorStyle.Printf("❌ Binary not found for target '%s'\n", targetName)
		warningStyle.Println("Tried the following locations:")
		for _, path := range possiblePaths {
			fmt.Printf("   %s\n", path)
		}
		warningStyle.Println("🔧 Try running a manual build first")
		return 1
	}

	// Show warning banner
	warningStyle.Println("⚠️  POLTERGEIST NOT RUNNING - EXECUTING POTENTIALLY STALE BINARY")
	warningStyle.Println("   The binary may be outdated. For fresh builds, start Poltergeist:")
	warningStyle.Println("   poltergeist watch")
	fmt.Println()

	if polterVerbose {
		infoStyle.Printf("📍 Project root: %s\n", projectRoot)
		infoStyle.Printf("🎯 Binary path: %s\n", binaryPath)
		warningStyle.Println("⚠️  Status: Executing without build verification")
	}

	successStyle.Printf("✅ Running binary: %s (potentially stale)\n", targetName)

	// Execute the binary
	cmd := exec.Command(binaryPath, args...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Dir = projectRoot

	if err := cmd.Run(); err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return exitErr.ExitCode()
		}
		errorStyle.Printf("❌ Failed to execute %s: %v\n", targetName, err)
		
		// Provide helpful suggestions
		if strings.Contains(err.Error(), "permission denied") {
			warningStyle.Println("💡 Permission denied:")
			fmt.Printf("   • Run: chmod +x %s\n", binaryPath)
			fmt.Println("   • Check file permissions")
		} else if strings.Contains(err.Error(), "no such file") {
			warningStyle.Println("💡 Tips:")
			fmt.Println("   • Check if the binary exists and is executable")
			fmt.Println("   • Try running: poltergeist watch")
			fmt.Println("   • Verify the output path in your configuration")
		}
		
		return 1
	}

	return 0
}
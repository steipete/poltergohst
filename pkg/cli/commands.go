package cli

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/fatih/color"
	"github.com/poltergeist/poltergeist/pkg/state"
	"github.com/poltergeist/poltergeist/pkg/types"
	"github.com/spf13/cobra"
)

func newStatusCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Show status of all targets",
		Long:  `Display the current build status of all targets, including last build time and results.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runStatus()
		},
	}
}

func newListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List all configured targets",
		Long:  `List all targets defined in the configuration file.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runList()
		},
	}
}

func newBuildCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "build [target]",
		Short: "Build a specific target once",
		Long:  `Build a target immediately without watching for changes.`,
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runBuild(args[0])
		},
	}
}

func newCleanCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "clean",
		Short: "Clean build artifacts and state",
		Long:  `Remove all build artifacts and state files.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runClean()
		},
	}
}

func newDaemonCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "daemon",
		Short: "Manage the Poltergeist daemon",
		Long:  `Control the Poltergeist background daemon process.`,
	}

	cmd.AddCommand(
		&cobra.Command{
			Use:   "start",
			Short: "Start the daemon",
			RunE: func(cmd *cobra.Command, args []string) error {
				return runDaemonStart()
			},
		},
		&cobra.Command{
			Use:   "stop",
			Short: "Stop the daemon",
			RunE: func(cmd *cobra.Command, args []string) error {
				return runDaemonStop()
			},
		},
		&cobra.Command{
			Use:   "restart",
			Short: "Restart the daemon",
			RunE: func(cmd *cobra.Command, args []string) error {
				return runDaemonRestart()
			},
		},
		&cobra.Command{
			Use:   "status",
			Short: "Show daemon status",
			RunE: func(cmd *cobra.Command, args []string) error {
				return runDaemonStatus()
			},
		},
	)

	return cmd
}

func newLogsCmd() *cobra.Command {
	var follow bool
	var lines int

	cmd := &cobra.Command{
		Use:   "logs [target]",
		Short: "Show build logs",
		Long:  `Display build logs for all targets or a specific target.`,
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			targetName := ""
			if len(args) > 0 {
				targetName = args[0]
			}
			return runLogs(targetName, follow, lines)
		},
	}

	cmd.Flags().BoolVarP(&follow, "follow", "f", false, "follow log output")
	cmd.Flags().IntVarP(&lines, "lines", "n", 50, "number of lines to show")

	return cmd
}

func newValidateCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "validate",
		Short: "Validate the configuration file",
		Long:  `Check that the configuration file is valid and all targets are properly configured.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runValidate()
		},
	}
}

func newVersionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print the version number of Poltergeist",
		Long:  `Print the version number of Poltergeist`,
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Printf("👻 Poltergeist v%s\n", version)
		},
	}
}

// Implementation functions

func runStatus() error {
	// Load config to get targets
	cfg, err := loadConfig(getConfigPath())
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// Create state manager
	sm := state.NewStateManager(projectRoot, nil)

	// Discover all states
	states, err := sm.DiscoverStates()
	if err != nil {
		return fmt.Errorf("failed to discover states: %w", err)
	}

	// Print status table
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "TARGET\tSTATUS\tLAST BUILD\tBUILDS\tFAILURES")
	fmt.Fprintln(w, "------\t------\t----------\t------\t--------")

	for _, rawTarget := range cfg.Targets {
		target, err := types.ParseTarget(rawTarget)
		if err != nil {
			continue
		}

		status := "idle"
		lastBuild := "-"
		builds := 0
		failures := 0

		if state, ok := states[target.GetName()]; ok {
			status = string(state.BuildStatus)
			if !state.LastBuildTime.IsZero() {
				lastBuild = state.LastBuildTime.Format("15:04:05")
			}
			builds = state.BuildCount
			failures = state.FailureCount
		}

		// Color status
		statusColor := color.WhiteString(status)
		switch status {
		case "succeeded":
			statusColor = color.GreenString(status)
		case "failed":
			statusColor = color.RedString(status)
		case "building":
			statusColor = color.YellowString(status)
		}

		fmt.Fprintf(w, "%s\t%s\t%s\t%d\t%d\n",
			target.GetName(),
			statusColor,
			lastBuild,
			builds,
			failures,
		)
	}

	w.Flush()
	return nil
}

func runList() error {
	cfg, err := loadConfig(getConfigPath())
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	printInfo(fmt.Sprintf("Project type: %s", cfg.ProjectType))
	fmt.Println()

	// Print targets table
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "NAME\tTYPE\tENABLED\tWATCH PATHS")
	fmt.Fprintln(w, "----\t----\t-------\t-----------")

	for _, rawTarget := range cfg.Targets {
		target, err := types.ParseTarget(rawTarget)
		if err != nil {
			continue
		}

		enabled := "✓"
		if !target.IsEnabled() {
			enabled = "✗"
		}

		watchPaths := ""
		if len(target.GetWatchPaths()) > 0 {
			watchPaths = target.GetWatchPaths()[0]
			if len(target.GetWatchPaths()) > 1 {
				watchPaths += fmt.Sprintf(" (+%d more)", len(target.GetWatchPaths())-1)
			}
		}

		fmt.Fprintf(w, "%s\t%s\t%s\t%s\n",
			target.GetName(),
			target.GetType(),
			enabled,
			watchPaths,
		)
	}

	w.Flush()
	return nil
}

func runBuild(targetName string) error {
	printInfo(fmt.Sprintf("Building target: %s", targetName))

	// Load configuration
	cfg, err := loadConfig(getConfigPath())
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// Find target
	var target types.Target
	found := false
	for _, rawTarget := range cfg.Targets {
		t, err := types.ParseTarget(rawTarget)
		if err != nil {
			continue
		}
		if t.GetName() == targetName {
			target = t
			found = true
			break
		}
	}

	if !found {
		return fmt.Errorf("target not found: %s", targetName)
	}

	// Create state manager
	sm := state.NewStateManager(projectRoot, nil)

	// Execute build command
	buildCmd := target.GetBuildCommand()
	if buildCmd == "" {
		// For test targets, check if we have a test command in the raw target
		if target.GetType() == "test" {
			// Re-parse the raw target to get test command
			for _, rawTarget := range cfg.Targets {
				var targetMap map[string]interface{}
				if err := json.Unmarshal(rawTarget, &targetMap); err != nil {
					continue
				}
				if name, ok := targetMap["name"].(string); ok && name == targetName {
					if testCmd, ok := targetMap["testCommand"].(string); ok && testCmd != "" {
						buildCmd = testCmd
						break
					}
				}
			}
			if buildCmd == "" {
				return fmt.Errorf("no build or test command defined for target %s", targetName)
			}
		} else {
			return fmt.Errorf("no build command defined for target %s", targetName)
		}
	}

	printInfo(fmt.Sprintf("Running: %s", buildCmd))

	// Execute the build
	cmd := exec.Command("sh", "-c", buildCmd)
	cmd.Dir = projectRoot
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	startTime := time.Now()
	err = cmd.Run()
	duration := time.Since(startTime)

	// Update state
	if err != nil {
		sm.UpdateBuildStatus(targetName, types.BuildStatusFailed)
		printError(fmt.Sprintf("Build failed for %s (%.2fs): %v", targetName, duration.Seconds(), err))
		return err
	}

	sm.UpdateBuildStatus(targetName, types.BuildStatusSucceeded)
	printSuccess(fmt.Sprintf("Build succeeded for %s (%.2fs)", targetName, duration.Seconds()))
	return nil
}

func runClean() error {
	// Remove state directory
	stateDir := filepath.Join(projectRoot, ".poltergeist")
	if err := os.RemoveAll(stateDir); err != nil {
		return fmt.Errorf("failed to remove state directory: %w", err)
	}

	printSuccess("Cleaned build artifacts and state")
	return nil
}

func runDaemonStart() error {
	printInfo("Starting daemon...")
	// TODO: Implement daemon start
	return fmt.Errorf("daemon mode not implemented yet")
}

func runDaemonStop() error {
	printInfo("Stopping daemon...")
	// TODO: Implement daemon stop
	return fmt.Errorf("daemon mode not implemented yet")
}

func runDaemonRestart() error {
	printInfo("Restarting daemon...")
	// TODO: Implement daemon restart
	return fmt.Errorf("daemon mode not implemented yet")
}

func runDaemonStatus() error {
	// TODO: Implement daemon status
	printWarning("Daemon is not running")
	return nil
}

func runLogs(targetName string, follow bool, lines int) error {
	// Determine log directory
	logDir := filepath.Join(projectRoot, ".poltergeist", "logs")

	// Check if log directory exists
	if _, err := os.Stat(logDir); os.IsNotExist(err) {
		printWarning("No logs found. Run 'poltergeist watch' to start logging.")
		return nil
	}

	// Get log files to display
	var logFiles []string
	if targetName != "" {
		// Show logs for specific target
		targetLogFile := filepath.Join(logDir, fmt.Sprintf("%s.log", targetName))
		if _, err := os.Stat(targetLogFile); os.IsNotExist(err) {
			return fmt.Errorf("no logs found for target: %s", targetName)
		}
		logFiles = []string{targetLogFile}
		printInfo(fmt.Sprintf("Showing logs for target: %s", targetName))
	} else {
		// Show all logs
		entries, err := os.ReadDir(logDir)
		if err != nil {
			return fmt.Errorf("failed to read log directory: %w", err)
		}

		for _, entry := range entries {
			if !entry.IsDir() && filepath.Ext(entry.Name()) == ".log" {
				logFiles = append(logFiles, filepath.Join(logDir, entry.Name()))
			}
		}

		if len(logFiles) == 0 {
			printWarning("No log files found")
			return nil
		}
		printInfo("Showing all logs")
	}

	// Display logs
	for _, logFile := range logFiles {
		if err := displayLogFile(logFile, lines, follow); err != nil {
			printError(fmt.Sprintf("Failed to display %s: %v", filepath.Base(logFile), err))
		}
	}

	return nil
}

func displayLogFile(logFile string, lines int, follow bool) error {
	if follow {
		// Use tail -f for following logs
		cmd := exec.Command("tail", "-f", "-n", fmt.Sprintf("%d", lines), logFile)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr

		// Handle interrupt gracefully
		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, os.Interrupt)
		go func() {
			<-sigChan
			if cmd.Process != nil {
				cmd.Process.Kill()
			}
		}()

		return cmd.Run()
	} else {
		// Read last N lines
		content, err := readLastNLines(logFile, lines)
		if err != nil {
			return err
		}

		// Print header if multiple files
		targetName := strings.TrimSuffix(filepath.Base(logFile), ".log")
		fmt.Printf("\n=== %s ===\n", targetName)
		fmt.Print(content)
	}

	return nil
}

func readLastNLines(filename string, n int) (string, error) {
	file, err := os.Open(filename)
	if err != nil {
		return "", err
	}
	defer file.Close()

	// Read all lines
	var allLines []string
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		allLines = append(allLines, scanner.Text())
	}

	if err := scanner.Err(); err != nil {
		return "", err
	}

	// Get last N lines
	start := 0
	if len(allLines) > n {
		start = len(allLines) - n
	}

	lastLines := allLines[start:]
	return strings.Join(lastLines, "\n") + "\n", nil
}

func runValidate() error {
	configPath := getConfigPath()

	// Try to load and validate config
	cfg, err := loadConfig(configPath)
	if err != nil {
		printError(fmt.Sprintf("Configuration is invalid: %v", err))
		return err
	}

	// Validate targets
	errors := []string{}
	warnings := []string{}

	for i, rawTarget := range cfg.Targets {
		target, err := types.ParseTarget(rawTarget)
		if err != nil {
			errors = append(errors, fmt.Sprintf("Target %d: %v", i, err))
			continue
		}

		// Check for common issues
		if target.GetName() == "" {
			errors = append(errors, fmt.Sprintf("Target %d: missing name", i))
		}

		if len(target.GetWatchPaths()) == 0 {
			warnings = append(warnings, fmt.Sprintf("Target '%s': no watch paths defined", target.GetName()))
		}

		// Check for build command (test targets can use testCommand instead)
		if target.GetType() == "test" {
			// Check raw target for test command
			hasTestCommand := false
			var targetMap map[string]interface{}
			if err := json.Unmarshal(rawTarget, &targetMap); err == nil {
				if testCmd, ok := targetMap["testCommand"].(string); ok && testCmd != "" {
					hasTestCommand = true
				}
			}
			if !hasTestCommand && target.GetBuildCommand() == "" {
				errors = append(errors, fmt.Sprintf("Target '%s': no test or build command defined", target.GetName()))
			}
		} else if target.GetBuildCommand() == "" {
			errors = append(errors, fmt.Sprintf("Target '%s': no build command defined", target.GetName()))
		}
	}

	// Print results
	if len(errors) > 0 {
		printError("Configuration has errors:")
		for _, err := range errors {
			fmt.Printf("  ✗ %s\n", err)
		}
	}

	if len(warnings) > 0 {
		printWarning("Configuration warnings:")
		for _, warn := range warnings {
			fmt.Printf("  ⚠ %s\n", warn)
		}
	}

	if len(errors) == 0 {
		printSuccess("Configuration is valid")
		return nil
	}

	return fmt.Errorf("configuration has %d error(s)", len(errors))
}

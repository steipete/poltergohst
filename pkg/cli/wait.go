package cli

import (
	"context"
	"fmt"
	"time"

	"github.com/poltergeist/poltergeist/pkg/state"
	"github.com/poltergeist/poltergeist/pkg/types"
	"github.com/spf13/cobra"
)

func newWaitCmd() *cobra.Command {
	var timeout int
	var targets []string
	var status string
	var pollInterval int

	cmd := &cobra.Command{
		Use:   "wait",
		Short: "Wait for targets to reach a specific state",
		Long: `Wait for one or more targets to reach a specific build status.
This command is useful in CI/CD pipelines to wait for builds to complete.`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			targetName := ""
			if len(args) > 0 {
				targetName = args[0]
			}

			return runWait(targetName, targets, status, timeout, pollInterval)
		},
	}

	cmd.Flags().IntVarP(&timeout, "timeout", "t", 300, "timeout in seconds")
	cmd.Flags().StringSliceVar(&targets, "targets", nil, "specific targets to wait for (comma-separated)")
	cmd.Flags().StringVarP(&status, "status", "s", "succeeded", "status to wait for (succeeded, failed, idle)")
	cmd.Flags().IntVar(&pollInterval, "poll-interval", 2, "polling interval in seconds")

	return cmd
}

// WaitResult represents the result of waiting for a target
type WaitResult struct {
	Target   string
	Status   types.BuildStatus
	Duration time.Duration
	Success  bool
	TimedOut bool
	Error    error
}

// runWait waits for targets to reach the specified status
func runWait(targetName string, targets []string, status string, timeoutSec int, pollIntervalSec int) error {
	// Validate status
	targetStatus := types.BuildStatus(status)
	validStatuses := []types.BuildStatus{
		types.BuildStatusIdle,
		types.BuildStatusQueued,
		types.BuildStatusBuilding,
		types.BuildStatusSucceeded,
		types.BuildStatusFailed,
		types.BuildStatusCancelled,
	}

	valid := false
	for _, validStatus := range validStatuses {
		if targetStatus == validStatus {
			valid = true
			break
		}
	}

	if !valid {
		return fmt.Errorf("invalid status '%s'. Valid statuses: idle, queued, building, succeeded, failed, cancelled", status)
	}

	// Determine which targets to wait for
	var targetNames []string
	if targetName != "" {
		targetNames = []string{targetName}
	} else if len(targets) > 0 {
		targetNames = targets
	} else {
		// Load config to get all targets
		cfg, err := loadConfig(getConfigPath())
		if err != nil {
			return fmt.Errorf("failed to load config: %w", err)
		}

		for _, rawTarget := range cfg.Targets {
			target, err := types.ParseTarget(rawTarget)
			if err != nil {
				continue
			}
			targetNames = append(targetNames, target.GetName())
		}

		if len(targetNames) == 0 {
			return fmt.Errorf("no targets found to wait for")
		}
	}

	printInfo(fmt.Sprintf("Waiting for %d target(s) to reach status '%s'", len(targetNames), status))
	if timeoutSec > 0 {
		printInfo(fmt.Sprintf("Timeout: %d seconds", timeoutSec))
	}

	// Create state manager
	sm := state.NewStateManager(projectRoot, nil)

	// Setup context with timeout
	ctx := context.Background()
	if timeoutSec > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, time.Duration(timeoutSec)*time.Second)
		defer cancel()
	}

	// Wait for targets
	results, err := waitForTargets(ctx, sm, targetNames, targetStatus, time.Duration(pollIntervalSec)*time.Second)
	if err != nil {
		return err
	}

	// Display results
	return displayWaitResults(results, targetStatus)
}

// waitForTargets waits for the specified targets to reach the target status
func waitForTargets(ctx context.Context, sm *state.StateManager, targetNames []string, targetStatus types.BuildStatus, pollInterval time.Duration) ([]WaitResult, error) {
	startTime := time.Now()
	results := make([]WaitResult, len(targetNames))

	// Initialize results
	for i, name := range targetNames {
		results[i] = WaitResult{
			Target: name,
		}
	}

	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()

	completed := make(map[string]bool)

	for {
		select {
		case <-ctx.Done():
			// Timeout or cancellation
			for i := range results {
				if !completed[results[i].Target] {
					results[i].TimedOut = true
					results[i].Duration = time.Since(startTime)
				}
			}
			return results, nil

		case <-ticker.C:
			allCompleted := true

			for i, targetName := range targetNames {
				if completed[targetName] {
					continue
				}

				// Check current status
				currentState, err := sm.ReadState(targetName)
				if err != nil {
					results[i].Error = err
					results[i].Duration = time.Since(startTime)
					completed[targetName] = true
					continue
				}

				results[i].Status = currentState.BuildStatus

				// Check if target reached desired status
				if currentState.BuildStatus == targetStatus {
					results[i].Success = true
					results[i].Duration = time.Since(startTime)
					completed[targetName] = true
					printSuccess(fmt.Sprintf("Target '%s' reached status '%s'", targetName, targetStatus))
				} else {
					allCompleted = false

					// Print periodic status updates
					if int(time.Since(startTime).Seconds())%10 == 0 {
						printInfo(fmt.Sprintf("Target '%s' status: %s (waiting for %s)", targetName, currentState.BuildStatus, targetStatus))
					}
				}
			}

			if allCompleted {
				return results, nil
			}
		}
	}
}

// displayWaitResults displays the final results of waiting
func displayWaitResults(results []WaitResult, targetStatus types.BuildStatus) error {
	fmt.Println()
	printInfo("Wait Results:")
	fmt.Println()

	successCount := 0
	timeoutCount := 0
	errorCount := 0

	for _, result := range results {
		status := "UNKNOWN"
		switch {
		case result.Error != nil:
			status = fmt.Sprintf("ERROR: %v", result.Error)
			errorCount++
		case result.TimedOut:
			status = fmt.Sprintf("TIMEOUT (last status: %s)", result.Status)
			timeoutCount++
		case result.Success:
			status = "SUCCESS"
			successCount++
		default:
			status = fmt.Sprintf("INCOMPLETE (status: %s)", result.Status)
		}

		fmt.Printf("  %-20s %-30s %v\n", result.Target, status, result.Duration.Round(time.Second))
	}

	fmt.Println()
	printInfo(fmt.Sprintf("Summary: %d succeeded, %d timed out, %d errors", successCount, timeoutCount, errorCount))

	// Return error if not all targets succeeded
	if successCount != len(results) {
		return fmt.Errorf("not all targets reached the desired status")
	}

	return nil
}

// waitForSpecificTarget waits for a single target with more detailed monitoring
func waitForSpecificTarget(ctx context.Context, sm *state.StateManager, targetName string, targetStatus types.BuildStatus, pollInterval time.Duration) (*WaitResult, error) {
	startTime := time.Now()
	result := &WaitResult{
		Target: targetName,
	}

	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()

	var lastStatus types.BuildStatus
	statusChangeTime := startTime

	for {
		select {
		case <-ctx.Done():
			result.TimedOut = true
			result.Duration = time.Since(startTime)
			return result, nil

		case <-ticker.C:
			currentState, err := sm.ReadState(targetName)
			if err != nil {
				result.Error = err
				result.Duration = time.Since(startTime)
				return result, nil
			}

			result.Status = currentState.BuildStatus

			// Detect status changes
			if currentState.BuildStatus != lastStatus {
				if lastStatus != "" {
					printInfo(fmt.Sprintf("Target '%s' status changed: %s -> %s", targetName, lastStatus, currentState.BuildStatus))
				}
				lastStatus = currentState.BuildStatus
				statusChangeTime = time.Now()
			}

			// Check if target reached desired status
			if currentState.BuildStatus == targetStatus {
				result.Success = true
				result.Duration = time.Since(startTime)
				return result, nil
			}

			// Check for stuck builds (no status change for too long)
			if currentState.BuildStatus == types.BuildStatusBuilding {
				stuckThreshold := 5 * time.Minute // Configurable threshold
				if time.Since(statusChangeTime) > stuckThreshold {
					printWarning(fmt.Sprintf("Target '%s' has been building for %v (possibly stuck)", targetName, time.Since(statusChangeTime).Round(time.Second)))
				}
			}
		}
	}
}

// waitForAnyTarget waits for any of the targets to reach the status (first completion wins)
func waitForAnyTarget(ctx context.Context, sm *state.StateManager, targetNames []string, targetStatus types.BuildStatus, pollInterval time.Duration) (*WaitResult, error) {
	startTime := time.Now()
	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return &WaitResult{
				TimedOut: true,
				Duration: time.Since(startTime),
			}, nil

		case <-ticker.C:
			for _, targetName := range targetNames {
				currentState, err := sm.ReadState(targetName)
				if err != nil {
					continue // Skip targets with errors, check others
				}

				if currentState.BuildStatus == targetStatus {
					return &WaitResult{
						Target:   targetName,
						Status:   currentState.BuildStatus,
						Success:  true,
						Duration: time.Since(startTime),
					}, nil
				}
			}
		}
	}
}

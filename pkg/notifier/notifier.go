// Package notifier provides build notification functionality
package notifier

import (
	"fmt"
	"runtime"
	"time"

	"github.com/gen2brain/beeep"
	"github.com/poltergeist/poltergeist/pkg/logger"
)

// BuildNotifier handles build notifications
type BuildNotifier struct {
	enabled      bool
	successSound string
	failureSound string
	logger       logger.Logger
}

// Config represents notification configuration
type Config struct {
	Enabled      bool
	SuccessSound string
	FailureSound string
}

// New creates a new build notifier
func New(config Config, log logger.Logger) *BuildNotifier {
	return &BuildNotifier{
		enabled:      config.Enabled,
		successSound: config.SuccessSound,
		failureSound: config.FailureSound,
		logger:       log,
	}
}

// NotifyBuildStart notifies that a build has started
func (n *BuildNotifier) NotifyBuildStart(target string) {
	if !n.enabled {
		return
	}
	
	title := "👻 Poltergeist"
	message := fmt.Sprintf("Building %s...", target)
	
	n.sendNotification(title, message, "")
}

// NotifyBuildSuccess notifies that a build succeeded
func (n *BuildNotifier) NotifyBuildSuccess(target string, duration time.Duration) {
	if !n.enabled {
		return
	}
	
	title := "✅ Build Succeeded"
	message := fmt.Sprintf("%s built in %s", target, formatDuration(duration))
	
	n.sendNotification(title, message, n.successSound)
}

// NotifyBuildFailure notifies that a build failed
func (n *BuildNotifier) NotifyBuildFailure(target string, err error) {
	if !n.enabled {
		return
	}
	
	title := "❌ Build Failed"
	message := fmt.Sprintf("%s: %v", target, err)
	
	n.sendNotification(title, message, n.failureSound)
}

// NotifyQueueStatus notifies about queue status
func (n *BuildNotifier) NotifyQueueStatus(active int, queued int) {
	// Only notify if there's a significant queue
	if queued > 5 {
		title := "⏳ Build Queue"
		message := fmt.Sprintf("%d active, %d queued", active, queued)
		n.sendNotification(title, message, "")
	}
}

// Private methods

func (n *BuildNotifier) sendNotification(title, message, soundName string) {
	// Platform-specific notification
	switch runtime.GOOS {
	case "darwin":
		n.sendMacNotification(title, message, soundName)
	case "linux":
		n.sendLinuxNotification(title, message)
	case "windows":
		n.sendWindowsNotification(title, message)
	default:
		// Fallback to console
		n.logger.Info(fmt.Sprintf("%s: %s", title, message))
	}
}

func (n *BuildNotifier) sendMacNotification(title, message, soundName string) {
	// Use beeep for cross-platform notifications
	if err := beeep.Notify(title, message, ""); err != nil {
		n.logger.Debug("Failed to send notification", logger.WithField("error", err))
	}
	
	// Play sound if specified
	if soundName != "" {
		if err := beeep.Beep(beeep.DefaultFreq, beeep.DefaultDuration); err != nil {
			n.logger.Debug("Failed to play sound", logger.WithField("error", err))
		}
	}
}

func (n *BuildNotifier) sendLinuxNotification(title, message string) {
	// Use notify-send on Linux
	if err := beeep.Notify(title, message, ""); err != nil {
		n.logger.Debug("Failed to send notification", logger.WithField("error", err))
	}
}

func (n *BuildNotifier) sendWindowsNotification(title, message string) {
	// Use Windows toast notifications
	if err := beeep.Notify(title, message, ""); err != nil {
		n.logger.Debug("Failed to send notification", logger.WithField("error", err))
	}
}

func formatDuration(d time.Duration) string {
	if d < time.Second {
		return fmt.Sprintf("%dms", d.Milliseconds())
	}
	if d < time.Minute {
		return fmt.Sprintf("%.1fs", d.Seconds())
	}
	return fmt.Sprintf("%dm%ds", int(d.Minutes()), int(d.Seconds())%60)
}
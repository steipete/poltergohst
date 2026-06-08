package daemon

import "errors"

// Sentinel errors for daemon operations following Go best practices.
// These enable reliable error checking with errors.Is()
var (
	// ErrDaemonNotRunning indicates the daemon is not currently running
	ErrDaemonNotRunning = errors.New("daemon is not running")

	// ErrDaemonAlreadyRunning indicates the daemon is already running
	ErrDaemonAlreadyRunning = errors.New("daemon is already running")

	// ErrDaemonStartFailed indicates the daemon failed to start
	ErrDaemonStartFailed = errors.New("daemon failed to start")

	// ErrDaemonStopFailed indicates the daemon failed to stop
	ErrDaemonStopFailed = errors.New("daemon failed to stop")
)

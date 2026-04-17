package app

import "github.com/icex/termdesk/internal/logging"

// dbg writes a debug-level message to the log file (if active).
// This is a convenience wrapper used throughout the app package.
func dbg(format string, args ...any) {
	logging.Debug(format, args...)
}

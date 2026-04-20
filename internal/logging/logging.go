// Package logging provides structured leveled logging for termdesk.
// Log output goes to ~/.local/share/termdesk/termdesk.log (not terminal).
// Activation: --log-level CLI flag, log_level in config.toml, or TERMDESK_DEBUG=1.
package logging

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime/debug"
	"strings"
	"sync"
	"time"
)

// Level represents a log severity.
type Level int

const (
	LevelOff   Level = iota // logging disabled
	LevelError              // errors only
	LevelWarn               // errors + warnings
	LevelInfo               // errors + warnings + info
	LevelDebug              // everything
)

var (
	mu       sync.Mutex
	logFile  *os.File
	logLevel Level = LevelOff
	initDone bool
)

// Init opens the log file and sets the active level.
// Safe to call multiple times — only the first call opens the file.
// A level of LevelOff disables logging entirely.
func Init(level Level) {
	mu.Lock()
	defer mu.Unlock()

	logLevel = level
	if level == LevelOff {
		return
	}
	if initDone {
		return
	}
	initDone = true

	home, err := os.UserHomeDir()
	if err != nil {
		return
	}
	logPath := filepath.Join(home, ".local", "share", "termdesk", "termdesk.log")
	os.MkdirAll(filepath.Dir(logPath), 0o755)
	f, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
	if err != nil {
		return
	}
	logFile = f

	// Write header
	bi, _ := debug.ReadBuildInfo()
	ver := "unknown"
	if bi != nil {
		ver = bi.GoVersion
	}
	exe, _ := os.Executable()
	fmt.Fprintf(f, "=== termdesk log %s level=%s ===\n", time.Now().Format(time.RFC3339), LevelName(level))
	fmt.Fprintf(f, "exe: %s  go: %s\n\n", exe, ver)
}

// SetLevel changes the log level at runtime.
func SetLevel(level Level) {
	mu.Lock()
	logLevel = level
	mu.Unlock()
}

// GetLevel returns the current log level.
func GetLevel() Level {
	mu.Lock()
	defer mu.Unlock()
	return logLevel
}

func log(level Level, prefix, format string, args ...any) {
	mu.Lock()
	defer mu.Unlock()
	if logLevel < level || logFile == nil {
		return
	}
	ts := time.Now().Format("15:04:05.000")
	msg := fmt.Sprintf(format, args...)
	fmt.Fprintf(logFile, "%s  %s  %s\n", ts, prefix, msg)
}

// Debug logs a debug message (most verbose).
func Debug(format string, args ...any) {
	log(LevelDebug, "DBG", format, args...)
}

// Info logs an informational message.
func Info(format string, args ...any) {
	log(LevelInfo, "INF", format, args...)
}

// Warn logs a warning message.
func Warn(format string, args ...any) {
	log(LevelWarn, "WRN", format, args...)
}

// Error logs an error message.
func Error(format string, args ...any) {
	log(LevelError, "ERR", format, args...)
}

// ParseLevel converts a string to a Level. Returns LevelOff for unknown values.
func ParseLevel(s string) Level {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "debug":
		return LevelDebug
	case "info":
		return LevelInfo
	case "warn", "warning":
		return LevelWarn
	case "error", "err":
		return LevelError
	case "off", "none", "":
		return LevelOff
	default:
		return LevelOff
	}
}

// LevelName returns the string name for a level.
func LevelName(l Level) string {
	switch l {
	case LevelDebug:
		return "debug"
	case LevelInfo:
		return "info"
	case LevelWarn:
		return "warn"
	case LevelError:
		return "error"
	default:
		return "off"
	}
}

// Close flushes and closes the log file.
func Close() {
	mu.Lock()
	defer mu.Unlock()
	if logFile != nil {
		logFile.Close()
		logFile = nil
	}
}

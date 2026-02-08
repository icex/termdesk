package app

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime/debug"
	"sync"
	"time"
)

var (
	debugOnce   sync.Once
	debugFile   *os.File
	debugActive bool
)

// initDebug opens the debug log file if we're running in dev mode.
// Dev mode is detected by checking if the binary was built with `go run`
// (which uses a temp directory) vs a proper `go build` binary.
func initDebug() {
	debugOnce.Do(func() {
		// Check if running via `go run` (binary in temp dir) or TERMDESK_DEBUG=1
		exe, _ := os.Executable()
		inTmp := len(exe) > 0 && (filepath.HasPrefix(exe, os.TempDir()) || filepath.HasPrefix(exe, "/tmp"))
		if !inTmp && os.Getenv("TERMDESK_DEBUG") != "1" {
			return
		}

		home, err := os.UserHomeDir()
		if err != nil {
			return
		}
		logPath := filepath.Join(home, ".local", "share", "termdesk", "debug.log")
		os.MkdirAll(filepath.Dir(logPath), 0o755)
		f, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
		if err != nil {
			return
		}
		debugFile = f
		debugActive = true

		// Write header
		bi, _ := debug.ReadBuildInfo()
		ver := "unknown"
		if bi != nil {
			ver = bi.GoVersion
		}
		fmt.Fprintf(f, "=== termdesk debug log %s ===\n", time.Now().Format(time.RFC3339))
		fmt.Fprintf(f, "exe: %s  go: %s\n\n", exe, ver)
	})
}

// dbg writes a formatted debug message to the log file (if active).
func dbg(format string, args ...any) {
	initDebug()
	if !debugActive {
		return
	}
	ts := time.Now().Format("15:04:05.000")
	fmt.Fprintf(debugFile, "%s  %s\n", ts, fmt.Sprintf(format, args...))
}

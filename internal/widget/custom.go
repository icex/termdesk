package widget

import (
	"context"
	"os/exec"
	"strings"
	"time"
)

const (
	shellWidgetTimeout    = 2 * time.Second
	shellWidgetMaxOutput  = 20 // max display characters
	shellWidgetDefaultInt = 2  // default refresh interval in seconds
)

// ShellWidget executes a shell command periodically and displays its output.
type ShellWidget struct {
	WidgetName string
	Label      string // display name for settings
	Icon       string
	Command    string
	Interval   int    // refresh interval in seconds (0 = use default)
	OnClick    string // command to launch on click (e.g. "lazygit")

	lastOutput string
	lastRun    time.Time
}

func (w *ShellWidget) Name() string { return w.WidgetName }

func (w *ShellWidget) Render() string {
	if w.lastOutput == "" {
		return ""
	}
	if w.Icon != "" {
		return w.Icon + " " + w.lastOutput
	}
	return w.lastOutput
}

func (w *ShellWidget) ColorLevel() string { return "" }

// NeedsRefresh returns true if the refresh interval has elapsed.
func (w *ShellWidget) NeedsRefresh() bool {
	interval := w.Interval
	if interval <= 0 {
		interval = shellWidgetDefaultInt
	}
	return w.lastRun.IsZero() || time.Since(w.lastRun) >= time.Duration(interval)*time.Second
}

// SetOutput updates the widget's cached output and marks the refresh time.
func (w *ShellWidget) SetOutput(output string) {
	w.lastOutput = output
	w.lastRun = time.Now()
}

// MarkRun records that a refresh attempt was made (even if pending async).
func (w *ShellWidget) MarkRun() {
	w.lastRun = time.Now()
}

// RunCommand executes the shell command and returns the result.
// Safe to call from any goroutine.
func RunCommand(command string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), shellWidgetTimeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, "sh", "-c", command)
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}

	result := strings.TrimSpace(string(out))
	runes := []rune(result)
	if len(runes) > shellWidgetMaxOutput {
		runes = runes[:shellWidgetMaxOutput]
	}
	return string(runes), nil
}

// Refresh re-runs the command synchronously if the interval has elapsed.
// Used in tests. For production use, prefer NeedsRefresh + async RunCommand.
func (w *ShellWidget) Refresh() {
	if !w.NeedsRefresh() {
		return
	}
	w.lastRun = time.Now()

	output, err := RunCommand(w.Command)
	if err != nil {
		return // keep last output on error
	}
	w.lastOutput = output
}

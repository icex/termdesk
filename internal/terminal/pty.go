package terminal

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"syscall"

	"github.com/creack/pty"
)

// PtySession wraps a pseudo-terminal.
type PtySession struct {
	file       *os.File
	cmd        *exec.Cmd
	cellPixelW uint16 // cell width in pixels (for TIOCGWINSZ pixel dimensions)
	cellPixelH uint16 // cell height in pixels (for TIOCGWINSZ pixel dimensions)
}

// NewPtySession starts a command in a new PTY with the given size.
// cellPixelW/cellPixelH set the pixel dimensions in TIOCGWINSZ so child apps
// can detect terminal pixel size (needed for Kitty graphics protocol).
// extraEnv contains additional environment variables (e.g. "KEY=value").
func NewPtySession(command string, args []string, rows, cols, cellPixelW, cellPixelH uint16, workDir string, extraEnv ...string) (*PtySession, error) {
	cmd := exec.Command(command, args...)
	// Build env with proper deduplication. glibc's getenv() returns the FIRST
	// match for duplicate keys, so simply appending overrides after os.Environ()
	// doesn't work — the original value wins. We must remove originals first.
	overrides := append([]string{"TERM=xterm-256color", "COLORTERM=truecolor"}, extraEnv...)
	cmd.Env = buildChildEnv(overrides...)
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Setsid:  true,
		Setctty: true,
		Ctty:    0, // make stdin the controlling terminal — required for sudo
	}

	// Set working directory if provided
	if workDir != "" {
		cmd.Dir = workDir
	}

	ws := &pty.Winsize{
		Rows: rows,
		Cols: cols,
		X:    cols * cellPixelW,
		Y:    rows * cellPixelH,
	}
	f, err := pty.StartWithSize(cmd, ws)
	if err != nil {
		return nil, err
	}

	return &PtySession{file: f, cmd: cmd, cellPixelW: cellPixelW, cellPixelH: cellPixelH}, nil
}

// Read reads from the PTY output.
func (p *PtySession) Read(buf []byte) (int, error) {
	return p.file.Read(buf)
}

// Write writes to the PTY input.
func (p *PtySession) Write(data []byte) (int, error) {
	return p.file.Write(data)
}

// Resize changes the PTY window size, including pixel dimensions.
func (p *PtySession) Resize(rows, cols uint16) error {
	return pty.Setsize(p.file, &pty.Winsize{
		Rows: rows,
		Cols: cols,
		X:    cols * p.cellPixelW,
		Y:    rows * p.cellPixelH,
	})
}

// Close terminates the PTY session.
func (p *PtySession) Close() error {
	_ = p.file.Close()
	if p.cmd.Process != nil {
		_ = p.cmd.Process.Kill()
		_, _ = p.cmd.Process.Wait()
	}
	return nil
}

// Signal sends a signal to the PTY process (e.g. SIGUSR1 for state dump).
func (p *PtySession) Signal(sig os.Signal) error {
	if p.cmd != nil && p.cmd.Process != nil {
		return p.cmd.Process.Signal(sig)
	}
	return fmt.Errorf("no process")
}

// Pid returns the PTY child process ID, or 0 if unavailable.
func (p *PtySession) Pid() int {
	if p.cmd != nil && p.cmd.Process != nil {
		return p.cmd.Process.Pid
	}
	return 0
}

// Fd returns the PTY file descriptor.
func (p *PtySession) Fd() uintptr {
	return p.file.Fd()
}

// buildChildEnv creates a child process environment from os.Environ() with
// overrides properly replacing (not duplicating) existing keys. This is
// necessary because glibc's getenv() returns the first match for a key,
// so appending overrides to os.Environ() doesn't actually override.
// Within overrides, later values for the same key win (last-wins semantics).
func buildChildEnv(overrides ...string) []string {
	// Deduplicate overrides: for each key, keep only the LAST value.
	// This ensures "TERM=xterm-kitty" after "TERM=xterm-256color" wins.
	lastIdx := make(map[string]int, len(overrides))
	for i, kv := range overrides {
		if j := strings.IndexByte(kv, '='); j >= 0 {
			lastIdx[kv[:j]] = i
		}
	}
	deduped := make([]string, 0, len(lastIdx))
	for i, kv := range overrides {
		if j := strings.IndexByte(kv, '='); j >= 0 {
			if lastIdx[kv[:j]] == i {
				deduped = append(deduped, kv)
			}
		} else {
			deduped = append(deduped, kv) // no '=', keep as-is
		}
	}

	// Remove override keys from parent env
	overrideKeys := make(map[string]bool, len(deduped))
	for _, kv := range deduped {
		if j := strings.IndexByte(kv, '='); j >= 0 {
			overrideKeys[kv[:j]] = true
		}
	}
	parent := os.Environ()
	result := make([]string, 0, len(parent)+len(deduped))
	for _, kv := range parent {
		if j := strings.IndexByte(kv, '='); j >= 0 {
			if overrideKeys[kv[:j]] {
				continue // skip — will be replaced by override
			}
		}
		result = append(result, kv)
	}
	return append(result, deduped...)
}

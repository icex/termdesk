package terminal

import (
	"os"
	"os/exec"

	"github.com/creack/pty"
)

// PtySession wraps a pseudo-terminal.
type PtySession struct {
	file *os.File
	cmd  *exec.Cmd
}

// NewPtySession starts a command in a new PTY with the given size.
func NewPtySession(command string, args []string, rows, cols uint16) (*PtySession, error) {
	cmd := exec.Command(command, args...)
	cmd.Env = os.Environ()
	cmd.Env = append(cmd.Env, "TERM=xterm-256color")

	ws := &pty.Winsize{Rows: rows, Cols: cols}
	f, err := pty.StartWithSize(cmd, ws)
	if err != nil {
		return nil, err
	}

	return &PtySession{file: f, cmd: cmd}, nil
}

// Read reads from the PTY output.
func (p *PtySession) Read(buf []byte) (int, error) {
	return p.file.Read(buf)
}

// Write writes to the PTY input.
func (p *PtySession) Write(data []byte) (int, error) {
	return p.file.Write(data)
}

// Resize changes the PTY window size.
func (p *PtySession) Resize(rows, cols uint16) error {
	return pty.Setsize(p.file, &pty.Winsize{Rows: rows, Cols: cols})
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

// Fd returns the PTY file descriptor.
func (p *PtySession) Fd() uintptr {
	return p.file.Fd()
}

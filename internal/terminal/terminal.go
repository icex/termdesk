package terminal

import (
	"io"
	"os"
	"sync"

	uv "github.com/charmbracelet/ultraviolet"
	"github.com/charmbracelet/x/vt"
)

// Terminal combines a PTY session with a VT emulator.
type Terminal struct {
	pty    *PtySession
	emu    *vt.SafeEmulator
	closed bool
	mu     sync.Mutex
}

// New creates a terminal running the given command.
func New(command string, args []string, cols, rows int) (*Terminal, error) {
	p, err := NewPtySession(command, args, uint16(rows), uint16(cols))
	if err != nil {
		return nil, err
	}

	emu := vt.NewSafeEmulator(cols, rows)

	t := &Terminal{
		pty: p,
		emu: emu,
	}

	return t, nil
}

// NewShell creates a terminal running the user's default shell.
func NewShell(cols, rows int) (*Terminal, error) {
	shell := os.Getenv("SHELL")
	if shell == "" {
		shell = "/bin/sh"
	}
	return New(shell, nil, cols, rows)
}

// ReadPtyLoop reads from the PTY and writes to the emulator.
// It returns when the PTY is closed or an error occurs.
// Call this from a goroutine.
func (t *Terminal) ReadPtyLoop() error {
	buf := make([]byte, 4096)
	for {
		t.mu.Lock()
		closed := t.closed
		t.mu.Unlock()
		if closed {
			return nil
		}

		n, err := t.pty.Read(buf)
		if n > 0 {
			t.emu.Write(buf[:n])
		}
		if err != nil {
			if err == io.EOF {
				return nil
			}
			return err
		}
	}
}

// WriteInput sends raw bytes to the PTY (keyboard input).
func (t *Terminal) WriteInput(data []byte) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if !t.closed {
		t.pty.Write(data)
	}
}

// SendKey converts a key event to bytes and sends to the PTY.
func (t *Terminal) SendKey(code rune, mod uv.KeyMod, text string) {
	data := encodeKey(code, mod, text)
	if len(data) > 0 {
		t.WriteInput(data)
	}
}

// Render returns the terminal screen as an ANSI-encoded string.
func (t *Terminal) Render() string {
	return t.emu.Render()
}

// Resize updates the terminal and PTY dimensions.
func (t *Terminal) Resize(cols, rows int) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if !t.closed {
		t.emu.Resize(cols, rows)
		t.pty.Resize(uint16(rows), uint16(cols))
	}
}

// Close terminates the terminal session.
func (t *Terminal) Close() error {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.closed {
		return nil
	}
	t.closed = true
	return t.pty.Close()
}

// IsClosed returns whether the terminal has been closed.
func (t *Terminal) IsClosed() bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.closed
}

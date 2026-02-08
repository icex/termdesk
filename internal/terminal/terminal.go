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
	pty     *PtySession
	emu     *vt.SafeEmulator
	closed  bool
	mu      sync.Mutex
	writeCh chan []byte // buffered write channel for raw PTY input
}

// New creates a terminal running the given command.
func New(command string, args []string, cols, rows int) (*Terminal, error) {
	p, err := NewPtySession(command, args, uint16(rows), uint16(cols))
	if err != nil {
		return nil, err
	}

	emu := vt.NewSafeEmulator(cols, rows)

	t := &Terminal{
		pty:     p,
		emu:     emu,
		writeCh: make(chan []byte, 256),
	}

	// Spawn writer goroutine — drains writeCh and writes to PTY.
	// This keeps WriteInput non-blocking.
	go t.writeLoop()

	// Spawn input forwarder — reads encoded input from the emulator's pipe
	// (filled by SendKey/SendMouse) and writes to PTY.
	go t.inputForwardLoop()

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

// ReadOnce reads one chunk from the PTY and feeds it to the emulator.
// Returns (bytesRead, error). Use this for event-driven reading.
func (t *Terminal) ReadOnce() (int, error) {
	t.mu.Lock()
	closed := t.closed
	t.mu.Unlock()
	if closed {
		return 0, io.EOF
	}

	buf := make([]byte, 32768)
	n, err := t.pty.Read(buf)
	if n > 0 {
		t.emu.Write(buf[:n])
	}
	return n, err
}

// WriteInput sends raw bytes to the PTY (keyboard input).
// Non-blocking — writes are buffered and processed by the writer goroutine.
func (t *Terminal) WriteInput(data []byte) {
	t.mu.Lock()
	closed := t.closed
	t.mu.Unlock()
	if closed {
		return
	}
	// Copy data to avoid races — caller may reuse the slice.
	buf := make([]byte, len(data))
	copy(buf, data)
	select {
	case t.writeCh <- buf:
	default:
		// Channel full — drop input to avoid blocking the UI.
	}
}

// writeLoop drains the write channel and sends to the PTY.
func (t *Terminal) writeLoop() {
	for data := range t.writeCh {
		t.mu.Lock()
		closed := t.closed
		t.mu.Unlock()
		if closed {
			return
		}
		t.pty.Write(data)
	}
}

// inputForwardLoop reads encoded input from the emulator's internal pipe
// (populated by SendKey/SendMouse/SendText) and writes it to the PTY.
// The emulator handles mode-dependent encoding:
// - Application Cursor Keys mode (DECCKM) for arrow keys in nvim
// - Mouse mode tracking (only forwards mouse events when app has enabled mouse)
// - Application Keypad mode for numeric keypad
func (t *Terminal) inputForwardLoop() {
	buf := make([]byte, 4096)
	for {
		n, err := t.emu.Read(buf)
		if n > 0 {
			t.mu.Lock()
			closed := t.closed
			t.mu.Unlock()
			if closed {
				return
			}
			t.pty.Write(buf[:n])
		}
		if err != nil {
			return
		}
	}
}

// SendKey sends a key event through the emulator's input pipeline.
// The emulator handles mode-dependent encoding (Application Cursor Keys, etc.)
// which is critical for apps like nvim.
func (t *Terminal) SendKey(code rune, mod uv.KeyMod, text string) {
	t.mu.Lock()
	closed := t.closed
	t.mu.Unlock()
	if closed {
		return
	}
	t.emu.SendKey(uv.KeyPressEvent(uv.Key{Code: code, Mod: mod, Text: text}))
}

// SendMouse sends a mouse click event through the emulator's input pipeline.
// The emulator only forwards mouse events when the terminal app has enabled
// mouse mode — this prevents SGR sequences from appearing as "weird text".
// col, row: 0-indexed coordinates relative to terminal content area.
func (t *Terminal) SendMouse(button uv.MouseButton, col, row int, release bool) {
	t.mu.Lock()
	closed := t.closed
	t.mu.Unlock()
	if closed {
		return
	}
	if release {
		t.emu.SendMouse(uv.MouseReleaseEvent(uv.Mouse{
			X:      col,
			Y:      row,
			Button: button,
		}))
	} else {
		t.emu.SendMouse(uv.MouseClickEvent(uv.Mouse{
			X:      col,
			Y:      row,
			Button: button,
		}))
	}
}

// SendMouseMotion sends a mouse motion event through the emulator's input pipeline.
// col, row: 0-indexed coordinates.
func (t *Terminal) SendMouseMotion(button uv.MouseButton, col, row int) {
	t.mu.Lock()
	closed := t.closed
	t.mu.Unlock()
	if closed {
		return
	}
	t.emu.SendMouse(uv.MouseMotionEvent(uv.Mouse{
		X:      col,
		Y:      row,
		Button: button,
	}))
}

// SendMouseWheel sends a mouse wheel event through the emulator's input pipeline.
func (t *Terminal) SendMouseWheel(button uv.MouseButton, col, row int) {
	t.mu.Lock()
	closed := t.closed
	t.mu.Unlock()
	if closed {
		return
	}
	t.emu.SendMouse(uv.MouseWheelEvent(uv.Mouse{
		X:      col,
		Y:      row,
		Button: button,
	}))
}

// CursorPosition returns the cursor's X, Y position in the terminal grid.
func (t *Terminal) CursorPosition() (int, int) {
	pos := t.emu.CursorPosition()
	return pos.X, pos.Y
}

// Render returns the terminal screen as an ANSI-encoded string.
func (t *Terminal) Render() string {
	return t.emu.Render()
}

// CellAt returns the VT emulator cell at the given position.
func (t *Terminal) CellAt(x, y int) *uv.Cell {
	return t.emu.CellAt(x, y)
}

// Width returns the emulator's column count.
func (t *Terminal) Width() int {
	return t.emu.Width()
}

// Height returns the emulator's row count.
func (t *Terminal) Height() int {
	return t.emu.Height()
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
	close(t.writeCh)
	return t.pty.Close()
}

// IsClosed returns whether the terminal has been closed.
func (t *Terminal) IsClosed() bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.closed
}

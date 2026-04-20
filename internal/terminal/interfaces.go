package terminal

import (
	"image/color"
	"io"
	"os"

	uv "github.com/charmbracelet/ultraviolet"
	"github.com/charmbracelet/x/vt"
)

// Pty defines the minimal PTY operations used by Terminal.
type Pty interface {
	Read(buf []byte) (int, error)
	Write(data []byte) (int, error)
	Resize(rows, cols uint16) error
	Close() error
	Signal(sig os.Signal) error
	Pid() int
	Fd() uintptr
}

// Emulator defines the minimal VT emulator operations used by Terminal.
type Emulator interface {
	Write(data []byte) (int, error)
	Read(buf []byte) (int, error)
	SendKey(ev uv.KeyEvent)
	SendMouse(ev uv.MouseEvent)
	InputPipe() io.Writer
	CursorPosition() uv.Position
	Render() string
	CellAt(x, y int) *uv.Cell
	Width() int
	Height() int
	Draw(screen uv.Screen, bounds uv.Rectangle)
	Resize(cols, rows int)
	SetCallbacks(cb vt.Callbacks)
	SetDefaultForegroundColor(c color.Color)
	SetDefaultBackgroundColor(c color.Color)
	BackgroundColor() color.Color
	Close() error
}

package terminal

import (
	"strings"
	"sync"

	uv "github.com/charmbracelet/ultraviolet"
	"github.com/charmbracelet/x/vt"
)

// scrollEmulator transfers real VT scroll events into our O(1) history ring.
// The extra mutex keeps Write/Resize and draining native history one operation.
type scrollEmulator struct {
	*vt.SafeEmulator
	writeMu  sync.Mutex
	onScroll func(uv.Line, int)
}

func (e *scrollEmulator) Write(data []byte) (int, error) {
	e.writeMu.Lock()
	defer e.writeMu.Unlock()
	n, err := e.SafeEmulator.Write(data)
	sb := e.SafeEmulator.Scrollback()
	width := e.SafeEmulator.Width()
	for _, line := range sb.Lines() {
		e.onScroll(line, width)
	}
	clear(sb.Lines())
	e.SafeEmulator.ClearScrollback()
	return n, err
}
func (e *scrollEmulator) Resize(w, h int) {
	e.writeMu.Lock()
	defer e.writeMu.Unlock()
	e.SafeEmulator.Resize(w, h)
}
func (t *Terminal) appendScrolledLine(line uv.Line, width int) {
	cells := make([]ScreenCell, width)
	for i := range cells {
		cells[i] = ScreenCell{Content: " ", Width: 1}
		if i < len(line) {
			c := line[i]
			cells[i] = ScreenCell{Content: c.Content, Width: int8(c.Width), Fg: c.Style.Fg, Bg: c.Style.Bg, Attrs: c.Style.Attrs}
		}
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.closed {
		return
	}
	t.scrollRing[t.scrollHead] = cells
	t.scrollHead = (t.scrollHead + 1) % t.scrollCap
	if t.scrollLen < t.scrollCap {
		t.scrollLen++
	}
	t.scrollWidth = width
}

// RestoreVisible replaces the visible snapshot without creating scroll events.
func (e *scrollEmulator) RestoreVisible(data []byte) {
	e.writeMu.Lock()
	defer e.writeMu.Unlock()
	e.SafeEmulator.Write([]byte("\x1b[2J\x1b[H"))
	e.SafeEmulator.Write(data)
	e.SafeEmulator.ClearScrollback()
}

// TextLines reads cells while the adapter excludes every screen writer. Unlike
// SafeEmulator.CellAt, it needs neither per-cell locks nor cloned styled cells.
func (e *scrollEmulator) TextLines() []string {
	e.writeMu.Lock()
	defer e.writeMu.Unlock()
	raw := e.SafeEmulator.Emulator
	w, h := raw.Width(), raw.Height()
	lines := make([]string, h)
	for y := 0; y < h; y++ {
		var line strings.Builder
		line.Grow(w)
		for x := 0; x < w; x++ {
			c := raw.CellAt(x, y)
			if c == nil {
				line.WriteByte(' ')
			} else if c.Width != 0 {
				if c.Content == "" {
					line.WriteByte(' ')
				} else {
					line.WriteString(c.Content)
				}
			}
		}
		lines[y] = strings.TrimRight(line.String(), " ")
	}
	return lines
}
func (e *scrollEmulator) Close() error {
	e.writeMu.Lock()
	defer e.writeMu.Unlock()
	return e.SafeEmulator.Close()
}

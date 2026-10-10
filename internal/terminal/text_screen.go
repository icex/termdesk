package terminal

import (
	"strings"

	uv "github.com/charmbracelet/ultraviolet"
	"github.com/charmbracelet/x/ansi"
)

// textScreen takes a plain-text snapshot under one emulator read lock. It avoids
// allocating a cloned uv.Cell (and acquiring a lock) for every visible cell.
type textScreen struct {
	w, h  int
	cells [][]string
}

func newTextScreen(w, h int) *textScreen {
	s := &textScreen{w: w, h: h, cells: make([][]string, h)}
	for y := range s.cells {
		s.cells[y] = make([]string, w)
	}
	return s
}
func (s *textScreen) Bounds() uv.Rectangle        { return uv.Rect(0, 0, s.w, s.h) }
func (s *textScreen) WidthMethod() uv.WidthMethod { return ansi.GraphemeWidth }
func (s *textScreen) CellAt(x, y int) *uv.Cell    { return nil }
func (s *textScreen) SetCell(x, y int, c *uv.Cell) {
	if y < 0 || y >= s.h || x < 0 || x >= s.w {
		return
	}
	if c == nil {
		s.cells[y][x] = " "
		return
	}
	if c.Width == 0 {
		s.cells[y][x] = ""
		return
	}
	content := c.Content
	if content == "" {
		content = " "
	}
	s.cells[y][x] = content
}
func (s *textScreen) Lines() []string {
	lines := make([]string, s.h)
	for y, row := range s.cells {
		lines[y] = strings.TrimRight(strings.Join(row, ""), " ")
	}
	return lines
}

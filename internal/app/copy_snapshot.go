package app

import "github.com/icex/termdesk/internal/terminal"

// CopySnapshot holds a frozen view of scrollback and screen content for copy mode.
// Scrollback lines are stored by offset (0 = most recent).
type CopySnapshot struct {
	WindowID   string
	Scrollback [][]terminal.ScreenCell
	Screen     [][]terminal.ScreenCell
	Width      int
	Height     int
}

func captureCopySnapshot(windowID string, term *terminal.Terminal) *CopySnapshot {
	if term == nil {
		return nil
	}
	sbLen := term.ScrollbackLen()
	scrollback := make([][]terminal.ScreenCell, sbLen)
	for offset := 0; offset < sbLen; offset++ {
		scrollback[offset] = term.ScrollbackLine(offset)
	}
	screen, w, h := term.SnapshotScreen()
	if screen == nil {
		w = term.Width()
		h = term.Height()
		screen = make([][]terminal.ScreenCell, h)
		for y := 0; y < h; y++ {
			row := make([]terminal.ScreenCell, w)
			for x := 0; x < w; x++ {
				if cell := term.CellAt(x, y); cell != nil {
					row[x] = terminal.ScreenCell{
						Content: cell.Content,
						Attrs:   cell.Style.Attrs,
						Fg:      cell.Style.Fg,
						Bg:      cell.Style.Bg,
						Width:   int8(cell.Width),
					}
				}
			}
			screen[y] = row
		}
	}

	return &CopySnapshot{
		WindowID:   windowID,
		Scrollback: scrollback,
		Screen:     screen,
		Width:      w,
		Height:     h,
	}
}

func (s *CopySnapshot) ScrollbackLen() int {
	if s == nil {
		return 0
	}
	return len(s.Scrollback)
}

func (s *CopySnapshot) ScrollbackLine(offset int) []terminal.ScreenCell {
	if s == nil || offset < 0 || offset >= len(s.Scrollback) {
		return nil
	}
	return s.Scrollback[offset]
}

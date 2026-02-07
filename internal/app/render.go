package app

import (
	"strings"
	"unicode/utf8"

	"github.com/icex/termdesk/internal/config"
	"github.com/icex/termdesk/internal/window"
	"github.com/icex/termdesk/pkg/geometry"
)

// runeLen returns the number of runes (display columns) in a string.
func runeLen(s string) int {
	return utf8.RuneCountInString(s)
}

// Cell represents a single terminal cell with character and style.
type Cell struct {
	Char rune
	Fg   string // foreground color hex
	Bg   string // background color hex
}

// Buffer is a 2D grid of cells representing the terminal screen.
type Buffer struct {
	Width  int
	Height int
	Cells  [][]Cell
}

// NewBuffer creates a buffer filled with spaces and the desktop background.
func NewBuffer(width, height int, bgColor string) *Buffer {
	cells := make([][]Cell, height)
	for y := range cells {
		cells[y] = make([]Cell, width)
		for x := range cells[y] {
			cells[y][x] = Cell{Char: ' ', Bg: bgColor}
		}
	}
	return &Buffer{Width: width, Height: height, Cells: cells}
}

// Set sets a cell at the given position if it's within bounds.
func (b *Buffer) Set(x, y int, char rune, fg, bg string) {
	if x >= 0 && x < b.Width && y >= 0 && y < b.Height {
		b.Cells[y][x] = Cell{Char: char, Fg: fg, Bg: bg}
	}
}

// SetString writes a string starting at (x, y), clipping at buffer edges.
func (b *Buffer) SetString(x, y int, s string, fg, bg string) {
	col := 0
	for _, ch := range s {
		b.Set(x+col, y, ch, fg, bg)
		col++
	}
}

// FillRect fills a rectangular area with a character and colors.
func (b *Buffer) FillRect(r geometry.Rect, char rune, fg, bg string) {
	for y := r.Y; y < r.Bottom(); y++ {
		for x := r.X; x < r.Right(); x++ {
			b.Set(x, y, char, fg, bg)
		}
	}
}

// RenderWindow draws a single window (border + title bar + content) into the buffer.
func RenderWindow(buf *Buffer, w *window.Window, theme config.Theme) {
	if !w.Visible || w.Minimized {
		return
	}

	r := w.Rect
	if r.Width < 3 || r.Height < 3 {
		return
	}

	// Pick colors based on focus state
	borderFg := theme.InactiveBorderFg
	borderBg := theme.InactiveBorderBg
	titleFg := theme.InactiveTitleFg
	titleBg := theme.InactiveTitleBg
	if w.Focused {
		borderFg = theme.ActiveBorderFg
		borderBg = theme.ActiveBorderBg
		titleFg = theme.ActiveTitleFg
		titleBg = theme.ActiveTitleBg
	}

	// Fill content area with spaces (clear background)
	contentRect := w.ContentRect()
	buf.FillRect(contentRect, ' ', borderFg, borderBg)

	// Draw top border with title
	buf.Set(r.X, r.Y, theme.BorderTopLeft, borderFg, titleBg)
	for x := r.X + 1; x < r.Right()-1; x++ {
		buf.Set(x, r.Y, theme.BorderHorizontal, borderFg, titleBg)
	}
	buf.Set(r.Right()-1, r.Y, theme.BorderTopRight, borderFg, titleBg)

	// Draw title text (left-aligned in title bar)
	closeW := runeLen(theme.CloseButton)
	maxW := runeLen(theme.MaxButton)
	buttonsW := closeW
	if w.Resizable {
		buttonsW += maxW
	}
	titleSpace := r.Width - 2 - buttonsW // space between corner and buttons
	title := w.Title
	titleRunes := []rune(title)
	if len(titleRunes) > titleSpace {
		if titleSpace > 3 {
			title = string(titleRunes[:titleSpace-3]) + "..."
		} else if titleSpace > 0 {
			title = string(titleRunes[:titleSpace])
		} else {
			title = ""
		}
	}
	titleX := r.X + 1
	buf.SetString(titleX, r.Y, " "+title+" ", titleFg, titleBg)

	// Draw title bar buttons (right side)
	closeX := r.Right() - 1 - closeW
	if w.Resizable {
		btnStr := theme.MaxButton
		if w.IsMaximized() {
			btnStr = theme.RestoreButton
		}
		btnW := runeLen(btnStr)
		buf.SetString(closeX-btnW, r.Y, btnStr, titleFg, titleBg)
	}
	buf.SetString(closeX, r.Y, theme.CloseButton, titleFg, titleBg)

	// Draw bottom border
	buf.Set(r.X, r.Bottom()-1, theme.BorderBottomLeft, borderFg, borderBg)
	for x := r.X + 1; x < r.Right()-1; x++ {
		buf.Set(x, r.Bottom()-1, theme.BorderHorizontal, borderFg, borderBg)
	}
	buf.Set(r.Right()-1, r.Bottom()-1, theme.BorderBottomRight, borderFg, borderBg)

	// Draw side borders
	for y := r.Y + 1; y < r.Bottom()-1; y++ {
		buf.Set(r.X, y, theme.BorderVertical, borderFg, borderBg)
		buf.Set(r.Right()-1, y, theme.BorderVertical, borderFg, borderBg)
	}
}

// RenderFrame composites all windows using the painter's algorithm.
// Windows are drawn back-to-front in z-order.
func RenderFrame(wm *window.Manager, theme config.Theme) *Buffer {
	wa := wm.WorkArea()
	// Use full bounds for the buffer
	bounds := geometry.Rect{X: 0, Y: 0, Width: wa.Width, Height: wa.Height + wa.Y}
	if bounds.Width <= 0 || bounds.Height <= 0 {
		return NewBuffer(1, 1, theme.DesktopBg)
	}

	buf := NewBuffer(bounds.Width, bounds.Height, theme.DesktopBg)

	// Draw windows back-to-front (painter's algorithm)
	for _, w := range wm.Windows() {
		RenderWindow(buf, w, theme)
	}

	return buf
}

// BufferToString converts the cell buffer to a plain string (without ANSI colors).
// Used for initial rendering; color support will be added with Lipgloss integration.
func BufferToString(buf *Buffer) string {
	var sb strings.Builder
	sb.Grow(buf.Width * buf.Height * 2)
	for y := 0; y < buf.Height; y++ {
		for x := 0; x < buf.Width; x++ {
			sb.WriteRune(buf.Cells[y][x].Char)
		}
		if y < buf.Height-1 {
			sb.WriteByte('\n')
		}
	}
	return sb.String()
}

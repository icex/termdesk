package app

import (
	"fmt"
	"image/color"
	"sort"
	"strconv"
	"strings"
	"sync"
	"unicode/utf8"

	"github.com/charmbracelet/lipgloss"
	uv "github.com/charmbracelet/ultraviolet"
	"github.com/charmbracelet/x/vt"
	tea "charm.land/bubbletea/v2"
	"github.com/icex/termdesk/internal/config"
	"github.com/icex/termdesk/internal/dock"
	"github.com/icex/termdesk/internal/launcher"
	"github.com/icex/termdesk/internal/menubar"
	"github.com/icex/termdesk/internal/terminal"
	"github.com/icex/termdesk/internal/window"
	"github.com/icex/termdesk/pkg/geometry"

	"github.com/mattn/go-runewidth"
)

// builderPool reuses strings.Builder allocations in BufferToString.
var builderPool = sync.Pool{
	New: func() any { return &strings.Builder{} },
}

// bufferPool reuses Buffer allocations across frames.
var bufferPool = sync.Pool{}

// runeLen returns the number of runes (display columns) in a string.
func runeLen(s string) int {
	return utf8.RuneCountInString(s)
}

// stampANSI parses an ANSI-styled string and writes its cells into the buffer
// at position (x, y). Uses a vt emulator as an ANSI→cells parser.
func stampANSI(buf *Buffer, x, y int, s string, width, height int) {
	emu := vt.NewEmulator(width, height)
	// VT emulator needs \r\n for proper line breaks; lipgloss outputs bare \n
	s = strings.ReplaceAll(s, "\n", "\r\n")
	emu.Write([]byte(s))
	for row := 0; row < height; row++ {
		for col := 0; col < width; col++ {
			bx, by := x+col, y+row
			if bx < 0 || bx >= buf.Width || by < 0 || by >= buf.Height {
				continue
			}
			cell := emu.CellAt(col, row)
			ch := ' '
			if cell != nil && cell.Content != "" {
				ch = []rune(cell.Content)[0]
			}
			var fg, bg color.Color
			var attrs uint8
			if cell != nil {
				fg = cell.Style.Fg
				bg = cell.Style.Bg
				attrs = cell.Style.Attrs
			}
			buf.Cells[by][bx] = Cell{Char: ch, Fg: fg, Bg: bg, Attrs: attrs}
		}
	}
	emu.Close()
}

// Cell represents a single terminal cell with character and style.
type Cell struct {
	Char  rune
	Fg    color.Color // nil = default
	Bg    color.Color // nil = default
	Attrs uint8       // text attributes (bold, italic, etc.)
	Width int8        // display width: 1 = normal, 2 = wide, 0 = continuation
}

// Text attribute constants matching ultraviolet.
const (
	AttrBold          = 1 << iota
	AttrFaint
	AttrItalic
	AttrBlink
	AttrRapidBlink
	AttrReverse
	AttrConceal
	AttrStrikethrough
)

// Buffer is a 2D grid of cells representing the terminal screen.
type Buffer struct {
	Width  int
	Height int
	Cells  [][]Cell
}

// hexToColor converts a "#RRGGBB" hex string to color.Color.
// Uses manual hex parsing instead of fmt.Sscanf for performance.
func hexToColor(hex string) color.Color {
	if len(hex) != 7 || hex[0] != '#' {
		return nil
	}
	r := hexByte(hex[1], hex[2])
	g := hexByte(hex[3], hex[4])
	b := hexByte(hex[5], hex[6])
	return color.RGBA{R: r, G: g, B: b, A: 255}
}

func hexByte(hi, lo byte) uint8 {
	return hexNibble(hi)<<4 | hexNibble(lo)
}

func hexNibble(b byte) uint8 {
	switch {
	case b >= '0' && b <= '9':
		return b - '0'
	case b >= 'a' && b <= 'f':
		return b - 'a' + 10
	case b >= 'A' && b <= 'F':
		return b - 'A' + 10
	}
	return 0
}

// NewBuffer creates a buffer filled with spaces and the desktop background.
// All cells get explicit Fg and Bg colors to prevent terminal default bleed-through
// (e.g. Termux's blue background showing through after ANSI resets).
func NewBuffer(width, height int, bgColor string) *Buffer {
	fg := color.RGBA{R: 192, G: 192, B: 192, A: 255} // light gray default fg
	bg := hexToColor(bgColor)
	if bg == nil {
		bg = color.RGBA{R: 0, G: 0, B: 0, A: 255} // fallback black
	}
	cells := make([][]Cell, height)
	for y := range cells {
		cells[y] = make([]Cell, width)
		for x := range cells[y] {
			cells[y][x] = Cell{Char: ' ', Fg: fg, Bg: bg, Width: 1}
		}
	}
	return &Buffer{Width: width, Height: height, Cells: cells}
}

// Set sets a cell at the given position if it's within bounds.
// fg and bg are hex color strings like "#RRGGBB", or "" for default.
func (b *Buffer) Set(x, y int, char rune, fg, bg string) {
	if x >= 0 && x < b.Width && y >= 0 && y < b.Height {
		b.Cells[y][x] = Cell{Char: char, Fg: hexToColor(fg), Bg: hexToColor(bg), Width: 1}
	}
}

// SetCell sets a cell at the given position with color.Color values directly.
func (b *Buffer) SetCell(x, y int, char rune, fg, bg color.Color, attrs uint8) {
	if x >= 0 && x < b.Width && y >= 0 && y < b.Height {
		b.Cells[y][x] = Cell{Char: char, Fg: fg, Bg: bg, Attrs: attrs, Width: 1}
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

// SetStringC writes a string starting at (x, y) using pre-parsed color.Color values.
func (b *Buffer) SetStringC(x, y int, s string, fg, bg color.Color) {
	col := 0
	for _, ch := range s {
		b.SetCell(x+col, y, ch, fg, bg, 0) // SetCell sets Width=1
		col++
	}
}

// FillRectC fills a rectangular area using pre-parsed color.Color values.
func (b *Buffer) FillRectC(r geometry.Rect, char rune, fg, bg color.Color) {
	for y := r.Y; y < r.Bottom(); y++ {
		for x := r.X; x < r.Right(); x++ {
			b.SetCell(x, y, char, fg, bg, 0) // SetCell sets Width=1
		}
	}
}

// Draw implements tea.Layer — transfers cells directly to the screen,
// bypassing ANSI serialization and re-parsing entirely.
func (b *Buffer) Draw(s tea.Screen, r tea.Rectangle) {
	for y := r.Min.Y; y < r.Max.Y && y-r.Min.Y < b.Height; y++ {
		row := y - r.Min.Y
		for x := r.Min.X; x < r.Max.X && x-r.Min.X < b.Width; x++ {
			col := x - r.Min.X
			cell := b.Cells[row][col]
			w := int(cell.Width)
			if w < 0 {
				w = 1
			}
			// Width=0 is valid (continuation cell from wide chars); pass through
			s.SetCell(x, y, &uv.Cell{
				Content: string(cell.Char),
				Width:   w,
				Style:   uv.Style{Fg: cell.Fg, Bg: cell.Bg, Attrs: cell.Attrs},
			})
		}
	}
}

// NewThemedBuffer creates a buffer with the theme's desktop background and optional pattern.
func NewThemedBuffer(width, height int, theme config.Theme) *Buffer {
	c := theme.C()
	fg := c.DefaultFg
	bg := c.DesktopBg
	if bg == nil {
		bg = color.RGBA{A: 255}
	}

	patChar := theme.DesktopPatternChar
	patFg := c.DesktopPatternFg
	if patFg == nil {
		patFg = fg
	}

	cells := make([][]Cell, height)
	for y := range cells {
		cells[y] = make([]Cell, width)
		for x := range cells[y] {
			ch := ' '
			cellFg := fg
			if patChar != 0 {
				if patChar == '░' || patChar == '▒' || patChar == '▓' {
					// Block patterns fill every cell
					ch = patChar
					cellFg = patFg
				} else if (x+y)%4 == 0 {
					// Sparse diamond pattern for dots/symbols
					ch = patChar
					cellFg = patFg
				}
			}
			cells[y][x] = Cell{Char: ch, Fg: cellFg, Bg: bg, Width: 1}
		}
	}
	return &Buffer{Width: width, Height: height, Cells: cells}
}

// AcquireThemedBuffer gets a buffer from the pool or creates a new one.
// The buffer is filled with the theme's background pattern, ready for rendering.
func AcquireThemedBuffer(width, height int, theme config.Theme) *Buffer {
	if v := bufferPool.Get(); v != nil {
		buf := v.(*Buffer)
		if buf.Width == width && buf.Height == height {
			fillThemed(buf, theme)
			return buf
		}
	}
	return NewThemedBuffer(width, height, theme)
}

// ReleaseBuffer returns a buffer to the pool for reuse.
func ReleaseBuffer(buf *Buffer) {
	bufferPool.Put(buf)
}

// fillThemed resets all cells in an existing buffer to the theme's background pattern.
func fillThemed(buf *Buffer, theme config.Theme) {
	c := theme.C()
	fg := c.DefaultFg
	bg := c.DesktopBg
	if bg == nil {
		bg = color.RGBA{A: 255}
	}
	patChar := theme.DesktopPatternChar
	patFg := c.DesktopPatternFg
	if patFg == nil {
		patFg = fg
	}
	for y := range buf.Cells {
		for x := range buf.Cells[y] {
			ch := ' '
			cellFg := fg
			if patChar != 0 {
				if patChar == '░' || patChar == '▒' || patChar == '▓' {
					ch = patChar
					cellFg = patFg
				} else if (x+y)%4 == 0 {
					ch = patChar
					cellFg = patFg
				}
			}
			buf.Cells[y][x] = Cell{Char: ch, Fg: cellFg, Bg: bg, Width: 1}
		}
	}
}

// RenderWindow draws a single window (border + title bar + content) into the buffer.
// showCursor controls whether the terminal cursor is rendered (false = blink-off phase).
// scrollOffset > 0 shows scrollback lines in Copy mode.
func RenderWindow(buf *Buffer, w *window.Window, theme config.Theme, term *terminal.Terminal, showCursor bool, scrollOffset int) {
	if !w.Visible || w.Minimized {
		return
	}

	r := w.Rect
	if r.Width < 3 || r.Height < 3 {
		return
	}

	c := theme.C()

	// Pick pre-parsed colors based on focus state
	borderFg := c.InactiveBorderFg
	borderBg := c.InactiveBorderBg
	titleFg := c.InactiveTitleFg
	titleBg := c.InactiveTitleBg
	if w.Focused {
		borderFg = c.ActiveBorderFg
		borderBg = c.ActiveBorderBg
		titleFg = c.ActiveTitleFg
		titleBg = c.ActiveTitleBg
	} else if w.HasNotification {
		borderFg = c.NotificationFg
		borderBg = c.NotificationBg
	}

	// Fill content area with spaces (clear background)
	contentRect := w.ContentRect()
	contentBg := c.ContentBg
	if contentBg == nil {
		contentBg = borderBg
	}
	buf.FillRectC(contentRect, ' ', borderFg, contentBg)

	tbh := w.TitleBarHeight
	if tbh < 1 {
		tbh = 1
	}

	// Draw title bar rows
	for row := 0; row < tbh; row++ {
		ty := r.Y + row
		if row == 0 {
			buf.SetCell(r.X, ty, theme.BorderTopLeft, borderFg, titleBg, 0)
			for x := r.X + 1; x < r.Right()-1; x++ {
				buf.SetCell(x, ty, theme.BorderHorizontal, borderFg, titleBg, 0)
			}
			buf.SetCell(r.Right()-1, ty, theme.BorderTopRight, borderFg, titleBg, 0)
		} else {
			buf.SetCell(r.X, ty, theme.BorderVertical, borderFg, titleBg, 0)
			for x := r.X + 1; x < r.Right()-1; x++ {
				buf.SetCell(x, ty, ' ', titleFg, titleBg, 0)
			}
			buf.SetCell(r.Right()-1, ty, theme.BorderVertical, borderFg, titleBg, 0)
		}
	}

	// Place title text and buttons on the center row of the title bar
	titleRow := r.Y + tbh/2

	closeW := runeLen(theme.CloseButton)
	minW := runeLen(theme.MinButton)
	maxW := runeLen(theme.MaxButton)
	snapLW := runeLen(theme.SnapLeftButton)
	snapRW := runeLen(theme.SnapRightButton)
	buttonsW := closeW + minW
	if w.Resizable {
		buttonsW += maxW + snapLW + snapRW
	}
	titleSpace := r.Width - 2 - buttonsW
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
	buf.SetStringC(titleX, titleRow, " "+title+" ", titleFg, titleBg)

	// Draw title bar buttons (right side)
	closeX := r.Right() - 1 - closeW
	minX := closeX - minW
	if w.Resizable {
		maxStr := theme.MaxButton
		if w.IsMaximized() {
			maxStr = theme.RestoreButton
		}
		maxX := minX - maxW
		snapRX := maxX - snapRW
		snapLX := snapRX - snapLW
		buf.SetStringC(snapLX, titleRow, theme.SnapLeftButton, titleFg, titleBg)
		buf.SetStringC(snapRX, titleRow, theme.SnapRightButton, titleFg, titleBg)
		buf.SetStringC(maxX, titleRow, maxStr, titleFg, titleBg)
	}
	buf.SetStringC(minX, titleRow, theme.MinButton, titleFg, titleBg)
	buf.SetStringC(closeX, titleRow, theme.CloseButton, titleFg, titleBg)

	// Draw bottom border
	buf.SetCell(r.X, r.Bottom()-1, theme.BorderBottomLeft, borderFg, borderBg, 0)
	for x := r.X + 1; x < r.Right()-1; x++ {
		buf.SetCell(x, r.Bottom()-1, theme.BorderHorizontal, borderFg, borderBg, 0)
	}
	buf.SetCell(r.Right()-1, r.Bottom()-1, theme.BorderBottomRight, borderFg, borderBg, 0)

	// Draw side borders
	for y := r.Y + 1; y < r.Bottom()-1; y++ {
		buf.SetCell(r.X, y, theme.BorderVertical, borderFg, borderBg, 0)
		buf.SetCell(r.Right()-1, y, theme.BorderVertical, borderFg, borderBg, 0)
	}

	// Draw terminal content if present
	if term != nil {
		defaultFg := c.DefaultFg
		renderTerminalContent(buf, contentRect, term, defaultFg, contentBg, scrollOffset)

		// Show scroll indicator in bottom border when scrolled back
		if scrollOffset > 0 {
			maxScroll := term.ScrollbackLen()
			indicator := fmt.Sprintf(" [↑ %d/%d] ", scrollOffset, maxScroll)
			indicatorRunes := []rune(indicator)
			borderY := w.Rect.Bottom() - 1
			startX := w.Rect.X + (w.Rect.Width-len(indicatorRunes))/2
			for i, ch := range indicatorRunes {
				x := startX + i
				if x > w.Rect.X && x < w.Rect.Right()-1 {
					buf.SetCell(x, borderY, ch, c.ActiveBorderFg, c.ActiveBorderBg, 0)
				}
			}
		}

		// Show cursor for focused terminal windows
		if w.Focused && showCursor && !term.IsCursorHidden() && scrollOffset == 0 {
			cx, cy := term.CursorPosition()
			sx := contentRect.X + cx
			sy := contentRect.Y + cy
			if sx >= contentRect.X && sx < contentRect.Right() &&
				sy >= contentRect.Y && sy < contentRect.Bottom() &&
				sx >= 0 && sx < buf.Width && sy >= 0 && sy < buf.Height {
				cell := &buf.Cells[sy][sx]
				cell.Fg, cell.Bg = cell.Bg, cell.Fg
			}
		}
	}

	// Desaturate unfocused windows
	if !w.Focused && theme.UnfocusedFade > 0 {
		dimBg := c.DesktopBg
		for y := contentRect.Y; y < contentRect.Bottom(); y++ {
			for x := contentRect.X; x < contentRect.Right(); x++ {
				if x >= 0 && x < buf.Width && y >= 0 && y < buf.Height {
					cell := &buf.Cells[y][x]
					cell.Fg = desaturateColor(cell.Fg, theme.UnfocusedFade)
					cell.Bg = blendColor(desaturateColor(cell.Bg, theme.UnfocusedFade), dimBg, theme.UnfocusedFade*0.3)
				}
			}
		}
	}
}

// renderTerminalContent copies the VT emulator screen into the buffer using per-cell access.
// defaultFg/defaultBg are used when the VT cell has nil colors (terminal default).
// scrollOffset > 0 means we're viewing scrollback: top portion shows scrollback lines,
// bottom portion shows the emulator's visible screen (minus scrollOffset rows from top).
func renderTerminalContent(buf *Buffer, area geometry.Rect, term *terminal.Terminal, defaultFg, defaultBg color.Color, scrollOffset int) {
	if term == nil {
		return
	}
	termW := term.Width()
	termH := term.Height()

	if scrollOffset <= 0 {
		// Live view — render directly from emulator
		for dy := 0; dy < area.Height && dy < termH; dy++ {
			for dx := 0; dx < area.Width && dx < termW; dx++ {
				cell := term.CellAt(dx, dy)
				if cell == nil {
					continue
				}
				ch := ' '
				if cell.Content != "" {
					runes := []rune(cell.Content)
					ch = runes[0]
				}
				bg := cell.Style.Bg
				if bg == nil {
					bg = defaultBg
				}
				fg := cell.Style.Fg
				if fg == nil {
					fg = defaultFg
				}
				bx, by := area.X+dx, area.Y+dy
				if bx >= 0 && bx < buf.Width && by >= 0 && by < buf.Height {
					w := int8(cell.Width)
					if w < 0 {
						w = 1
					}
					buf.Cells[by][bx] = Cell{Char: ch, Fg: fg, Bg: bg, Attrs: cell.Style.Attrs, Width: w}
				}
			}
		}
		return
	}

	// Scrollback view: compose scrollback lines + visible screen
	// scrollOffset=5, screen height=24 → show 5 scrollback lines at top + top 19 emulator rows
	scrollLines := scrollOffset
	if scrollLines > area.Height {
		scrollLines = area.Height
	}
	emuLines := area.Height - scrollLines

	// Render scrollback lines at the top
	for dy := 0; dy < scrollLines; dy++ {
		// scrollback offset: most recent scrollback line = offset 0
		// We want the topmost display row to show the oldest of the visible scrollback range
		sbIdx := scrollOffset - 1 - dy
		line := term.ScrollbackLine(sbIdx)
		for dx := 0; dx < area.Width; dx++ {
			if line != nil && dx < len(line) {
				sc := line[dx]
				ch := ' '
				if sc.Content != "" {
					runes := []rune(sc.Content)
					ch = runes[0]
				}
				fg := sc.Fg
				if fg == nil {
					fg = defaultFg
				}
				bg := sc.Bg
				if bg == nil {
					bg = defaultBg
				}
				buf.SetCell(area.X+dx, area.Y+dy, ch, fg, bg, sc.Attrs)
			} else {
				buf.SetCell(area.X+dx, area.Y+dy, ' ', defaultFg, defaultBg, 0)
			}
		}
	}

	// Render emulator rows below the scrollback
	for dy := 0; dy < emuLines && dy < termH; dy++ {
		for dx := 0; dx < area.Width && dx < termW; dx++ {
			cell := term.CellAt(dx, dy)
			if cell == nil {
				continue
			}
			ch := ' '
			if cell.Content != "" {
				runes := []rune(cell.Content)
				ch = runes[0]
			}
			bg := cell.Style.Bg
			if bg == nil {
				bg = defaultBg
			}
			fg := cell.Style.Fg
			if fg == nil {
				fg = defaultFg
			}
			bx, by := area.X+dx, area.Y+scrollLines+dy
			if bx >= 0 && bx < buf.Width && by >= 0 && by < buf.Height {
				w := int8(cell.Width)
				if w < 0 {
					w = 1
				}
				buf.Cells[by][bx] = Cell{Char: ch, Fg: fg, Bg: bg, Attrs: cell.Style.Attrs, Width: w}
			}
		}
	}
}

// stripANSI removes ANSI escape sequences from a string, returning runes.
func stripANSI(s string) []rune {
	var result []rune
	i := 0
	runes := []rune(s)
	for i < len(runes) {
		if runes[i] == '\x1b' && i+1 < len(runes) {
			i++ // skip ESC
			switch {
			case runes[i] == '[':
				// CSI sequence: ESC [ ... final byte (0x40-0x7E)
				i++
				for i < len(runes) && runes[i] < 0x40 || runes[i] > 0x7E {
					if runes[i] >= 0x40 && runes[i] <= 0x7E {
						break
					}
					i++
				}
				if i < len(runes) {
					i++ // skip final byte
				}
			case runes[i] == ']':
				// OSC sequence: ESC ] ... ST or BEL
				i++
				for i < len(runes) {
					if runes[i] == '\x07' { // BEL
						i++
						break
					}
					if runes[i] == '\x1b' && i+1 < len(runes) && runes[i+1] == '\\' { // ST
						i += 2
						break
					}
					i++
				}
			default:
				// Other ESC sequences (e.g., ESC ( B for charset):
				// skip intermediate bytes (0x20-0x2F) then final byte (0x30-0x7E)
				for i < len(runes) && runes[i] >= 0x20 && runes[i] <= 0x2F {
					i++
				}
				if i < len(runes) {
					i++ // skip final byte
				}
			}
		} else {
			result = append(result, runes[i])
			i++
		}
	}
	return result
}

// RenderFrame composites all windows using the painter's algorithm.
// Windows are drawn back-to-front in z-order.
// animRects provides animated rect overrides for windows currently animating.
// SelectionInfo holds the current copy-mode selection state for rendering.
type SelectionInfo struct {
	Active   bool
	Start    geometry.Point // X=col, Y=absLine
	End      geometry.Point // X=col, Y=absLine
	CopyMode bool          // true when in copy mode (for scrollbar)
}

func RenderFrame(wm *window.Manager, theme config.Theme, terminals map[string]*terminal.Terminal, animRects map[string]geometry.Rect, showCursor bool, scrollOffset int, sel SelectionInfo) *Buffer {
	wa := wm.WorkArea()
	// Use full terminal bounds for the buffer (includes reserved rows for menu/dock)
	fullWidth := wa.Width
	fullHeight := wa.Height + wa.Y + wm.ReservedBottom()
	if fullWidth <= 0 || fullHeight <= 0 {
		return AcquireThemedBuffer(1, 1, theme)
	}

	buf := AcquireThemedBuffer(fullWidth, fullHeight, theme)

	// Draw windows back-to-front (painter's algorithm)
	for _, w := range wm.Windows() {
		var term *terminal.Terminal
		if terminals != nil {
			term = terminals[w.ID]
		}
		// Only apply scrollOffset to the focused window
		winScroll := 0
		if w.Focused {
			winScroll = scrollOffset
		}
		// Use animated rect if available
		if animRect, ok := animRects[w.ID]; ok {
			origRect := w.Rect
			// Clamp animated rect to valid range (spring can overshoot)
			if animRect.Width < 1 {
				animRect.Width = 1
			}
			if animRect.Height < 1 {
				animRect.Height = 1
			}
			w.Rect = animRect
			// Skip terminal content during close/minimize animation (window is shrinking)
			if animRect.Width <= 3 || animRect.Height <= 3 {
				RenderWindow(buf, w, theme, nil, showCursor, 0)
			} else {
				RenderWindow(buf, w, theme, term, showCursor, winScroll)
			}
			w.Rect = origRect // restore for state consistency
		} else {
			RenderWindow(buf, w, theme, term, showCursor, winScroll)
		}
	}

	// Render selection highlighting and scrollbar for the focused window in copy mode
	if fw := wm.FocusedWindow(); fw != nil && sel.CopyMode {
		cr := fw.ContentRect()
		if term := terminals[fw.ID]; term != nil {
			sbLen := term.ScrollbackLen()
			if sel.Active {
				renderSelection(buf, cr, sel.Start, sel.End, scrollOffset, sbLen, cr.Height)
			}
			renderScrollbar(buf, fw, theme, term, scrollOffset)
		}
	}

	return buf
}

// renderSelection highlights selected cells by inverting fg/bg.
func renderSelection(buf *Buffer, contentRect geometry.Rect, start, end geometry.Point, scrollOffset, scrollbackLen, contentH int) {
	// Normalize: ensure start <= end
	sLine, sCol := start.Y, start.X
	eLine, eCol := end.Y, end.X
	if sLine > eLine || (sLine == eLine && sCol > eCol) {
		sLine, eLine = eLine, sLine
		sCol, eCol = eCol, sCol
	}

	for dy := 0; dy < contentH; dy++ {
		absLine := mouseToAbsLine(dy, scrollOffset, scrollbackLen, contentH)
		if absLine < sLine || absLine > eLine {
			continue
		}
		colStart := 0
		colEnd := contentRect.Width
		if absLine == sLine {
			colStart = sCol
		}
		if absLine == eLine {
			colEnd = eCol + 1
		}
		if colStart < 0 {
			colStart = 0
		}
		if colEnd > contentRect.Width {
			colEnd = contentRect.Width
		}
		for dx := colStart; dx < colEnd; dx++ {
			bx := contentRect.X + dx
			by := contentRect.Y + dy
			if bx >= 0 && bx < buf.Width && by >= 0 && by < buf.Height {
				cell := &buf.Cells[by][bx]
				cell.Fg, cell.Bg = cell.Bg, cell.Fg
			}
		}
	}
}

// renderScrollbar draws a scrollbar on the right border of the window in copy mode.
func renderScrollbar(buf *Buffer, w *window.Window, theme config.Theme, term *terminal.Terminal, scrollOffset int) {
	c := theme.C()
	r := w.Rect
	trackX := r.Right() - 1
	trackTop := r.Y + 1
	trackBot := r.Bottom() - 2 // above bottom border
	trackH := trackBot - trackTop + 1
	if trackH < 3 {
		return
	}

	sbLen := term.ScrollbackLen()
	termH := term.Height()
	totalLines := sbLen + termH
	contentH := w.ContentRect().Height
	if totalLines <= contentH {
		return // no scrollbar needed
	}

	// Thumb size and position
	thumbH := trackH * contentH / totalLines
	if thumbH < 1 {
		thumbH = 1
	}
	// scrollOffset=0 means viewing bottom (most recent), scrollOffset=sbLen means top
	viewTop := totalLines - contentH - scrollOffset
	thumbPos := trackH * viewTop / totalLines
	if thumbPos+thumbH > trackH {
		thumbPos = trackH - thumbH
	}
	if thumbPos < 0 {
		thumbPos = 0
	}

	trackFg := c.InactiveBorderFg
	trackBg := c.InactiveBorderBg
	thumbFg := c.ActiveBorderFg

	for dy := 0; dy < trackH; dy++ {
		y := trackTop + dy
		if y < 0 || y >= buf.Height || trackX < 0 || trackX >= buf.Width {
			continue
		}
		if dy >= thumbPos && dy < thumbPos+thumbH {
			buf.SetCell(trackX, y, '▓', thumbFg, trackBg, 0)
		} else {
			buf.SetCell(trackX, y, '░', trackFg, trackBg, 0)
		}
	}
}

// RenderMenuBar draws the menu bar at the top of the buffer.
func RenderMenuBar(buf *Buffer, mb *menubar.MenuBar, theme config.Theme, mode InputMode, prefixPending bool) {
	if mb == nil || buf.Height < 1 {
		return
	}

	c := theme.C()

	// Fill menu bar row with menu bar background
	mbFg := c.MenuBarFg
	mbBg := c.MenuBarBg
	if mbFg == nil {
		mbFg = c.ActiveTitleFg
	}
	if mbBg == nil {
		mbBg = c.ActiveTitleBg
	}
	for x := 0; x < buf.Width; x++ {
		buf.SetCell(x, 0, ' ', mbFg, mbBg, 0)
	}

	// Compute mode badge (placed at far right, fixed width)
	var modeLabel string
	var modeFC, modeBC color.Color
	badgeFg := color.RGBA{R: 30, G: 30, B: 46, A: 255} // #1E1E2E
	if prefixPending {
		modeLabel = " \uf11c PREFIX   "
		modeFC = badgeFg
		modeBC = color.RGBA{R: 224, G: 108, B: 117, A: 255} // #E06C75 red
	} else {
		modeLabel = modeBadge(mode)
		modeFC = badgeFg
		switch mode {
		case ModeTerminal:
			modeBC = color.RGBA{R: 152, G: 195, B: 121, A: 255} // #98C379 green
		case ModeCopy:
			modeBC = color.RGBA{R: 97, G: 175, B: 239, A: 255} // #61AFEF blue
		default:
			modeBC = color.RGBA{R: 229, G: 192, B: 123, A: 255} // #E5C07B yellow
		}
	}
	modeLabelLen := runewidth.StringWidth(modeLabel)

	// Render bar text with reduced width (leave room for mode badge)
	effectiveWidth := buf.Width - modeLabelLen
	if effectiveWidth < 1 {
		effectiveWidth = 1
	}
	barText := mb.Render(effectiveWidth)
	col := 0
	for _, ch := range barText {
		if col < buf.Width {
			buf.SetCell(col, 0, ch, mbFg, mbBg, 0)
		}
		col++
	}

	// Highlight open menu label with accent background
	// Include the leading space for symmetric padding
	if mb.IsOpen() {
		positions := mb.MenuXPositions()
		pos := positions[mb.OpenIndex]
		labelW := len([]rune(mb.Menus[mb.OpenIndex].Label)) + 2
		hlStart := pos
		if pos > 0 {
			hlStart = pos - 1
		}
		for x := hlStart; x < hlStart+labelW && x < buf.Width; x++ {
			buf.Cells[0][x].Bg = c.DockAccentBg
			buf.Cells[0][x].Attrs = AttrBold
		}
	}

	// Colorize right-side CPU/MEM indicators
	for _, zone := range mb.RightZones(effectiveWidth) {
		var zoneColor color.Color
		switch zone.Type {
		case "cpu":
			zoneColor = levelColorC(menubar.CPUColorLevel(mb.CPUPct))
		case "mem":
			_, totalGB := menubar.ReadMemoryInfo()
			zoneColor = levelColorC(menubar.MemColorLevel(mb.MemGB, totalGB))
		case "bat":
			zoneColor = levelColorC(menubar.BatColorLevel(mb.BatPct))
		}
		if zoneColor != nil {
			for x := zone.Start; x < zone.End && x < buf.Width; x++ {
				if x >= 0 && x < buf.Width {
					buf.Cells[0][x].Fg = zoneColor
				}
			}
		}
	}

	// Mode badge at the far right
	modeX := buf.Width - modeLabelLen
	if modeX < 0 {
		modeX = 0
	}
	col = 0
	for _, ch := range modeLabel {
		buf.SetCell(modeX+col, 0, ch, modeFC, modeBC, 0)
		col += runewidth.RuneWidth(ch)
	}

	// Render dropdown if open
	if mb.IsOpen() {
		positions := mb.MenuXPositions()
		dropX := positions[mb.OpenIndex]
		// Use lipgloss-styled dropdown
		borderFg := theme.ActiveBorderFg
		ddBg := theme.ActiveBorderBg
		ddFg := theme.ActiveTitleFg
		hoverFg := "#1E1E2E"
		hoverBg := "#61AFEF"
		shortcutFg := "#888888"
		styled := mb.RenderDropdownStyled(borderFg, ddBg, hoverFg, hoverBg, ddFg, shortcutFg)
		if styled != "" {
			w := lipgloss.Width(styled)
			h := lipgloss.Height(styled)
			stampANSI(buf, dropX, 1, styled, w, h)
		}
	}
}

// levelColor maps a color level name to a hex color (used by overlays).
func levelColor(level string) string {
	switch level {
	case "red":
		return "#E06C75"
	case "yellow":
		return "#E5C07B"
	case "green":
		return "#98C379"
	default:
		return ""
	}
}

// Pre-parsed level colors (avoid hexToColor in hot path).
var (
	levelRed    = color.RGBA{R: 224, G: 108, B: 117, A: 255}
	levelYellow = color.RGBA{R: 229, G: 192, B: 123, A: 255}
	levelGreen  = color.RGBA{R: 152, G: 195, B: 121, A: 255}
)

// levelColorC maps a color level name to a pre-parsed color.Color.
func levelColorC(level string) color.Color {
	switch level {
	case "red":
		return levelRed
	case "yellow":
		return levelYellow
	case "green":
		return levelGreen
	default:
		return nil
	}
}

// modeIcon returns a Nerd Font icon for the input mode.
func modeIcon(mode InputMode) string {
	switch mode {
	case ModeTerminal:
		return "\uf120" //  terminal
	case ModeCopy:
		return "\uf0c5" //  copy
	default:
		return "\uf009" //  grid (window management)
	}
}

// modeBadge returns a fixed-width mode badge string.
// All modes produce the same display width so switching modes
// doesn't shift the clock/CPU/MEM indicators.
func modeBadge(mode InputMode) string {
	icon := modeIcon(mode)
	name := mode.String()
	// Pad name to 8 chars ("TERMINAL" is the longest)
	for len(name) < 8 {
		name += " "
	}
	return " " + icon + " " + name + " "
}

// RenderDock draws the dock at the bottom of the buffer with per-cell accent coloring.
func RenderDock(buf *Buffer, d *dock.Dock, theme config.Theme, animations []Animation) {
	if d == nil || buf.Height < 2 {
		return
	}

	c := theme.C()
	y := buf.Height - 1

	// Fill dock row with base dock colors
	for x := 0; x < buf.Width; x++ {
		buf.SetCell(x, y, ' ', c.DockFg, c.DockBg, 0)
	}

	// Build pulse set from active animations
	pulseItems := make(map[int]bool)
	for _, a := range animations {
		if a.Type == AnimDockPulse && !a.Done {
			pulseItems[a.DockIndex] = true
		}
	}

	// Temporarily activate hover on pulsing items for accent highlighting
	origHover := d.HoverIndex
	if len(pulseItems) > 0 && d.HoverIndex < 0 {
		for idx := range pulseItems {
			d.HoverIndex = idx
			break
		}
	}

	// Render dock cells with per-cell styling
	cells := d.RenderCells(buf.Width)
	d.HoverIndex = origHover // restore original hover

	for x, cell := range cells {
		if cell.Char == 0 {
			continue
		}
		fg := c.DockFg
		bg := c.DockBg
		if cell.IconColor != "" {
			fg = hexToColor(cell.IconColor) // per-icon color still from hex
		}
		if cell.Minimized {
			fg = c.NotificationFg
			if fg == nil {
				fg = levelYellow
			}
		}
		if cell.Accent {
			bg = c.DockAccentBg
		}
		buf.SetCell(x, y, cell.Char, fg, bg, 0)
	}
}

// RenderLauncher draws the launcher overlay centered on the buffer.
func RenderLauncher(buf *Buffer, l *launcher.Launcher, theme config.Theme) {
	if l == nil || !l.Visible {
		return
	}

	lines := l.Render(buf.Width, buf.Height)
	if len(lines) == 0 {
		return
	}

	// Center vertically
	boxH := len(lines)
	startY := (buf.Height - boxH) / 3 // slightly above center looks better
	if startY < 1 {
		startY = 1
	}

	// Center horizontally (first line determines width)
	boxW := utf8.RuneCountInString(lines[0])
	startX := (buf.Width - boxW) / 2
	if startX < 0 {
		startX = 0
	}

	for dy, line := range lines {
		col := 0
		for _, ch := range line {
			buf.Set(startX+col, startY+dy, ch, theme.ActiveTitleFg, theme.ActiveBorderBg)
			col++
		}
	}
}

// RenderConfirmDialog draws a confirmation dialog centered on the buffer.
func RenderConfirmDialog(buf *Buffer, dialog *ConfirmDialog, theme config.Theme) {
	if dialog == nil {
		return
	}

	fgColor := lipgloss.Color(theme.ActiveTitleFg)
	bgColor := lipgloss.Color(theme.ActiveTitleBg)
	borderColor := lipgloss.Color(theme.ActiveBorderFg)
	contentBg := lipgloss.Color(theme.ActiveBorderBg)

	innerW := runeLen(dialog.Title) + 4
	if innerW < 26 {
		innerW = 26
	}

	// Title
	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(fgColor).
		Background(bgColor).
		Width(innerW).
		Align(lipgloss.Center)
	titleStr := titleStyle.Render(dialog.Title)

	// Separator
	sepStyle := lipgloss.NewStyle().
		Foreground(borderColor).
		Background(contentBg).
		Width(innerW)
	sepStr := sepStyle.Render(strings.Repeat("─", innerW))

	// Buttons — fixed width so Yes and No are equal size
	btnW := 8
	activeBtnYes := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#1E1E2E")).
		Background(lipgloss.Color("#98C379")).
		Width(btnW).
		Align(lipgloss.Center)
	activeBtnNo := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#1E1E2E")).
		Background(lipgloss.Color("#E06C75")).
		Width(btnW).
		Align(lipgloss.Center)
	dimBtn := lipgloss.NewStyle().
		Bold(true).
		Foreground(fgColor).
		Background(contentBg).
		Width(btnW).
		Align(lipgloss.Center)

	var yesStr, noStr string
	if dialog.Selected == 0 {
		yesStr = activeBtnYes.Render("Yes")
		noStr = dimBtn.Render("No")
	} else {
		yesStr = dimBtn.Render("Yes")
		noStr = activeBtnNo.Render("No")
	}
	btnRow := lipgloss.JoinHorizontal(lipgloss.Top, yesStr, "  ", noStr)
	btnRowStyle := lipgloss.NewStyle().
		Background(contentBg).
		Width(innerW).
		Align(lipgloss.Center)
	btnStr := btnRowStyle.Render(btnRow)

	// Compose
	inner := strings.Join([]string{titleStr, sepStr, btnStr}, "\n")

	boxStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(borderColor).
		BorderBackground(contentBg)
	rendered := boxStyle.Render(inner)

	w := lipgloss.Width(rendered)
	h := lipgloss.Height(rendered)
	startX := (buf.Width - w) / 2
	startY := (buf.Height - h) / 2
	if startX < 0 {
		startX = 0
	}
	if startY < 0 {
		startY = 0
	}
	stampANSI(buf, startX, startY, rendered, w, h)
}

// RenderRenameDialog draws a text input dialog for renaming a window.
func RenderRenameDialog(buf *Buffer, dialog *RenameDialog, theme config.Theme) {
	if dialog == nil {
		return
	}

	title := "Rename Window"
	inputW := 30
	boxW := inputW + 4
	if boxW < runeLen(title)+4 {
		boxW = runeLen(title) + 4
	}
	boxH := 6

	startX := (buf.Width - boxW) / 2
	startY := (buf.Height - boxH) / 2
	if startX < 0 {
		startX = 0
	}
	if startY < 0 {
		startY = 0
	}

	fg := theme.ActiveTitleFg
	bg := theme.ActiveBorderBg
	titleBg := theme.ActiveTitleBg

	// Top border
	buf.Set(startX, startY, theme.BorderTopLeft, fg, titleBg)
	for x := 1; x < boxW-1; x++ {
		buf.Set(startX+x, startY, theme.BorderHorizontal, fg, titleBg)
	}
	buf.Set(startX+boxW-1, startY, theme.BorderTopRight, fg, titleBg)

	// Title row
	buf.Set(startX, startY+1, theme.BorderVertical, fg, titleBg)
	for x := 1; x < boxW-1; x++ {
		buf.Set(startX+x, startY+1, ' ', fg, titleBg)
	}
	titleX := startX + (boxW-runeLen(title))/2
	buf.SetString(titleX, startY+1, title, fg, titleBg)
	buf.Set(startX+boxW-1, startY+1, theme.BorderVertical, fg, titleBg)

	// Separator
	buf.Set(startX, startY+2, theme.BorderVertical, fg, bg)
	for x := 1; x < boxW-1; x++ {
		buf.Set(startX+x, startY+2, theme.BorderHorizontal, fg, bg)
	}
	buf.Set(startX+boxW-1, startY+2, theme.BorderVertical, fg, bg)

	// Input row
	inputFieldW := boxW - 4
	buf.Set(startX, startY+3, theme.BorderVertical, fg, bg)
	for x := 1; x < boxW-1; x++ {
		buf.Set(startX+x, startY+3, ' ', fg, bg)
	}
	inputX := startX + 2
	// Draw text with cursor
	text := dialog.Text
	// Visible window of text
	visStart := 0
	if dialog.Cursor > inputFieldW-1 {
		visStart = dialog.Cursor - inputFieldW + 1
	}
	for i := 0; i < inputFieldW; i++ {
		idx := visStart + i
		ch := ' '
		if idx < len(text) {
			ch = text[idx]
		}
		cellFg := fg
		cellBg := bg
		if idx == dialog.Cursor {
			cellFg = bg
			cellBg = fg // invert for cursor
		}
		buf.Set(inputX+i, startY+3, ch, cellFg, cellBg)
	}
	buf.Set(startX+boxW-1, startY+3, theme.BorderVertical, fg, bg)

	// Hint row
	hint := "Enter=OK  Esc=Cancel"
	buf.Set(startX, startY+4, theme.BorderVertical, fg, bg)
	for x := 1; x < boxW-1; x++ {
		buf.Set(startX+x, startY+4, ' ', fg, bg)
	}
	hintX := startX + (boxW-runeLen(hint))/2
	buf.SetString(hintX, startY+4, hint, fg, bg)
	buf.Set(startX+boxW-1, startY+4, theme.BorderVertical, fg, bg)

	// Bottom border
	buf.Set(startX, startY+5, theme.BorderBottomLeft, fg, bg)
	for x := 1; x < boxW-1; x++ {
		buf.Set(startX+x, startY+5, theme.BorderHorizontal, fg, bg)
	}
	buf.Set(startX+boxW-1, startY+5, theme.BorderBottomRight, fg, bg)
}

// RenderModal draws a scrollable modal overlay centered on the buffer.
// Supports tabbed modals (when modal.Tabs is non-nil).
func RenderModal(buf *Buffer, modal *ModalOverlay, theme config.Theme) {
	if modal == nil {
		return
	}

	// Resolve which lines to display (tabs or plain)
	lines := modal.Lines
	hasTabBar := modal.Tabs != nil && len(modal.Tabs) > 0
	if hasTabBar {
		if modal.ActiveTab >= len(modal.Tabs) {
			modal.ActiveTab = 0
		}
		lines = modal.Tabs[modal.ActiveTab].Lines
	}

	// Calculate stable dimensions across all tabs
	maxLineW := runeLen(modal.Title)
	if hasTabBar {
		for _, tab := range modal.Tabs {
			for _, line := range tab.Lines {
				if w := runeLen(line); w > maxLineW {
					maxLineW = w
				}
			}
		}
	} else {
		for _, line := range lines {
			if w := runeLen(line); w > maxLineW {
				maxLineW = w
			}
		}
	}
	innerW := maxLineW + 2 // padding
	if innerW > buf.Width-6 {
		innerW = buf.Width - 6
	}

	maxTabLines := len(lines)
	if hasTabBar {
		for _, tab := range modal.Tabs {
			if len(tab.Lines) > maxTabLines {
				maxTabLines = len(tab.Lines)
			}
		}
	}

	visibleLines := buf.Height - 10
	if visibleLines < 3 {
		visibleLines = 3
	}
	if visibleLines > maxTabLines {
		visibleLines = maxTabLines
	}

	// Clamp scroll
	maxScroll := len(lines) - visibleLines
	if maxScroll < 0 {
		maxScroll = 0
	}
	if modal.ScrollY > maxScroll {
		modal.ScrollY = maxScroll
	}

	// Theme colors for lipgloss
	fgColor := lipgloss.Color(theme.ActiveTitleFg)
	bgColor := lipgloss.Color(theme.ActiveTitleBg)
	borderColor := lipgloss.Color(theme.ActiveBorderFg)

	// Title — centered, bold
	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(fgColor).
		Background(bgColor).
		Width(innerW).
		Align(lipgloss.Center)
	titleStr := titleStyle.Render(modal.Title)

	// Separator
	sepStyle := lipgloss.NewStyle().
		Foreground(borderColor).
		Background(bgColor).
		Width(innerW)
	sepStr := sepStyle.Render(strings.Repeat("─", innerW))

	// Tab bar (if tabbed)
	tabBarStr := ""
	if hasTabBar {
		activeTabStyle := lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#1E1E2E")).
			Background(lipgloss.Color("#61AFEF")).
			Padding(0, 1)
		inactiveTabStyle := lipgloss.NewStyle().
			Foreground(fgColor).
			Background(bgColor).
			Padding(0, 1)

		var tabs []string
		for i, tab := range modal.Tabs {
			if i == modal.ActiveTab {
				tabs = append(tabs, activeTabStyle.Render(tab.Title))
			} else {
				tabs = append(tabs, inactiveTabStyle.Render(tab.Title))
			}
		}
		row := lipgloss.JoinHorizontal(lipgloss.Top, tabs...)
		// Pad to full width
		tabRowStyle := lipgloss.NewStyle().
			Background(bgColor).
			Width(innerW)
		tabBarStr = tabRowStyle.Render(row)
	}

	// Content lines
	contentStyle := lipgloss.NewStyle().
		Foreground(fgColor).
		Background(bgColor).
		Width(innerW)

	var contentLines []string
	for i := 0; i < visibleLines; i++ {
		lineIdx := modal.ScrollY + i
		if lineIdx < len(lines) {
			line := lines[lineIdx]
			lineRunes := []rune(line)
			if len(lineRunes) > innerW {
				line = string(lineRunes[:innerW])
			}
			contentLines = append(contentLines, contentStyle.Render(line))
		} else {
			contentLines = append(contentLines, contentStyle.Render(""))
		}
	}
	contentStr := strings.Join(contentLines, "\n")

	// Footer with navigation hints
	footerText := ""
	if hasTabBar {
		footerText = " Tab \u2190\u2192 navigate \u2502 \u2191\u2193 scroll \u2502 Esc close "
	} else {
		footerText = " \u2191\u2193 scroll \u2502 Esc close "
	}
	footerStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#888888")).
		Background(bgColor).
		Width(innerW).
		Align(lipgloss.Center)
	footerStr := footerStyle.Render(footerText)

	// Compose inner content
	spacer := lipgloss.NewStyle().Background(bgColor).Width(innerW).Render("")
	parts := []string{titleStr, sepStr}
	if tabBarStr != "" {
		parts = append(parts, tabBarStr, spacer)
	}
	parts = append(parts, contentStr, footerStr)
	inner := strings.Join(parts, "\n")

	// Wrap in rounded border
	boxStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(borderColor).
		BorderBackground(bgColor)
	rendered := boxStyle.Render(inner)

	// Stamp into buffer centered
	w := lipgloss.Width(rendered)
	h := lipgloss.Height(rendered)
	startX := (buf.Width - w) / 2
	startY := (buf.Height - h) / 2
	if startX < 0 {
		startX = 0
	}
	if startY < 0 {
		startY = 0
	}
	stampANSI(buf, startX, startY, rendered, w, h)
}

// renderExposeMiniWindow draws a single mini window preview at the given position.
// If focused, the title is rendered using the big half-block font. Otherwise bold plain text.
func renderExposeMiniWindow(buf *Buffer, theme config.Theme, w *window.Window, x, y, mw, mh int, focused bool, winNum int) {
	borderFg := theme.InactiveBorderFg
	borderBg := theme.InactiveBorderBg
	titleFg := theme.InactiveTitleFg
	if focused {
		borderFg = theme.ActiveBorderFg
		borderBg = theme.ActiveBorderBg
		titleFg = theme.ActiveTitleFg
	}

	// Draw mini border — top
	buf.Set(x, y, theme.BorderTopLeft, borderFg, borderBg)
	for dx := 1; dx < mw-1; dx++ {
		buf.Set(x+dx, y, theme.BorderHorizontal, borderFg, borderBg)
	}
	buf.Set(x+mw-1, y, theme.BorderTopRight, borderFg, borderBg)

	// Fill content area with content bg
	contentBg := theme.ContentBg
	if contentBg == "" {
		contentBg = borderBg
	}
	for dy := 1; dy < mh-1; dy++ {
		buf.Set(x, y+dy, theme.BorderVertical, borderFg, borderBg)
		for dx := 1; dx < mw-1; dx++ {
			buf.Set(x+dx, y+dy, ' ', borderFg, contentBg)
		}
		buf.Set(x+mw-1, y+dy, theme.BorderVertical, borderFg, borderBg)
	}

	// Bottom border
	if mh > 1 {
		buf.Set(x, y+mh-1, theme.BorderBottomLeft, borderFg, borderBg)
		for dx := 1; dx < mw-1; dx++ {
			buf.Set(x+dx, y+mh-1, theme.BorderHorizontal, borderFg, borderBg)
		}
		buf.Set(x+mw-1, y+mh-1, theme.BorderBottomRight, borderFg, borderBg)
	}

	if mh <= 2 {
		return
	}

	innerW := mw - 2
	innerH := mh - 2
	fgColor := hexToColor(titleFg)
	bgColor := hexToColor(contentBg)

	title := w.Title
	titleRunes := []rune(title)

	if focused {
		// Focused window: show title centered (no number)
		if len(titleRunes) > innerW {
			if innerW > 3 {
				title = string(titleRunes[:innerW-3]) + "..."
			} else if innerW > 0 {
				title = string(titleRunes[:innerW])
			} else {
				title = ""
			}
		}
		if title != "" {
			titleX := x + 1 + (innerW-runeLen(title))/2
			titleY := y + 1 + innerH/2
			col := 0
			for _, ch := range title {
				bx := titleX + col
				if bx >= 0 && bx < buf.Width && titleY >= 0 && titleY < buf.Height {
					buf.Cells[titleY][bx] = Cell{Char: ch, Fg: fgColor, Bg: bgColor, Attrs: AttrBold, Width: 1}
				}
				col++
			}
		}
	} else {
		// Unfocused thumbnail: show "N: Title" centered
		label := fmt.Sprintf("%d: %s", winNum, title)
		labelRunes := []rune(label)
		if len(labelRunes) > innerW {
			if innerW > 3 {
				label = string(labelRunes[:innerW-3]) + "..."
			} else if innerW > 0 {
				label = string(labelRunes[:innerW])
			} else {
				label = ""
			}
		}
		if label != "" {
			lx := x + 1 + (innerW-runeLen(label))/2
			ly := y + 1 + innerH/2
			col := 0
			for _, ch := range label {
				bx := lx + col
				if bx >= 0 && bx < buf.Width && ly >= 0 && ly < buf.Height {
					buf.Cells[ly][bx] = Cell{Char: ch, Fg: fgColor, Bg: bgColor, Width: 1}
				}
				col++
			}
		}
	}
}

// RenderExpose renders the exposé overview with the focused window centered and larger.
// Unfocused windows are shown as small thumbnails in the background.
func RenderExpose(wm *window.Manager, theme config.Theme) *Buffer {
	wa := wm.WorkArea()
	fullWidth := wa.Width
	fullHeight := wa.Height + wa.Y + wm.ReservedBottom()
	if fullWidth <= 0 || fullHeight <= 0 {
		return AcquireThemedBuffer(1, 1, theme)
	}

	buf := AcquireThemedBuffer(fullWidth, fullHeight, theme)

	windows := wm.Windows()
	var visible []*window.Window
	var focusedWin *window.Window
	for _, w := range windows {
		if w.Visible && !w.Minimized {
			visible = append(visible, w)
			if w.Focused {
				focusedWin = w
			}
		}
	}
	n := len(visible)
	if n == 0 {
		msg := "No open windows"
		x := (fullWidth - runeLen(msg)) / 2
		y := fullHeight / 2
		buf.SetString(x, y, msg, theme.ActiveTitleFg, theme.DesktopBg)
		return buf
	}
	sort.Slice(visible, func(i, j int) bool { return visible[i].ID < visible[j].ID })

	// Build window number map (1-based display number for each visible window)
	winNumMap := make(map[string]int, n)
	for i, w := range visible {
		winNumMap[w.ID] = i + 1 // 1-based
	}

	// Calculate background thumbnail positions (grid along the bottom/sides)
	bgCount := n - 1
	if focusedWin == nil {
		// No focused window — fall back to grid layout for all
		focusedWin = visible[0]
		bgCount = n - 1
	}

	// -- Draw unfocused windows as small thumbnails along the bottom strip --
	if bgCount > 0 {
		thumbW := 16
		thumbH := 6
		// Scale down if too many windows
		totalThumbW := bgCount * (thumbW + 1)
		if totalThumbW > wa.Width-2 {
			thumbW = (wa.Width - 2 - bgCount) / bgCount
			if thumbW < 8 {
				thumbW = 8
			}
		}

		startX := wa.X + (wa.Width-bgCount*(thumbW+1)+1)/2
		thumbY := wa.Y + wa.Height - thumbH - 1

		idx := 0
		for _, w := range visible {
			if w.ID == focusedWin.ID {
				continue
			}
			x := startX + idx*(thumbW+1)
			renderExposeMiniWindow(buf, theme, w, x, thumbY, thumbW, thumbH, false, winNumMap[w.ID])
			idx++
		}
	}

	// -- Draw focused window centered and larger --
	// Size: up to 70% of work area, but capped by original window size
	focW := focusedWin.Rect.Width * 7 / 10
	focH := focusedWin.Rect.Height * 7 / 10
	maxW := wa.Width * 7 / 10
	maxH := wa.Height * 7 / 10
	if focW > maxW {
		focW = maxW
	}
	if focH > maxH {
		focH = maxH
	}
	if focW < 16 {
		focW = 16
	}
	if focH < 8 {
		focH = 8
	}

	focX := wa.X + (wa.Width-focW)/2
	focY := wa.Y + (wa.Height-focH)/3 // slightly above center
	renderExposeMiniWindow(buf, theme, focusedWin, focX, focY, focW, focH, true, winNumMap[focusedWin.ID])

	return buf
}

// RenderExposeTransition draws windows at their animated positions during expose enter/exit.
func RenderExposeTransition(wm *window.Manager, theme config.Theme, animations []Animation) *Buffer {
	wa := wm.WorkArea()
	fullWidth := wa.Width
	fullHeight := wa.Height + wa.Y + wm.ReservedBottom()
	if fullWidth <= 0 || fullHeight <= 0 {
		return AcquireThemedBuffer(1, 1, theme)
	}

	buf := AcquireThemedBuffer(fullWidth, fullHeight, theme)

	// Build animation lookup
	animMap := make(map[string]*Animation)
	for i := range animations {
		a := &animations[i]
		if (a.Type == AnimExpose || a.Type == AnimExposeExit) && !a.Done {
			animMap[a.WindowID] = a
		}
	}

	// Build window number map (sorted by ID for consistent numbering)
	var transVisible []*window.Window
	for _, w := range wm.Windows() {
		if w.Visible && !w.Minimized {
			transVisible = append(transVisible, w)
		}
	}
	sort.Slice(transVisible, func(i, j int) bool { return transVisible[i].ID < transVisible[j].ID })
	winNumMap := make(map[string]int, len(transVisible))
	for i, w := range transVisible {
		winNumMap[w.ID] = i + 1
	}

	// Compute expose target rects as fallback for windows without animations.
	// Without this, non-animated windows render at w.Rect (desktop position).
	var focusedWin *window.Window
	for _, w := range transVisible {
		if w.Focused {
			focusedWin = w
			break
		}
	}
	if focusedWin == nil && len(transVisible) > 0 {
		focusedWin = transVisible[0]
	}
	exposeFallback := make(map[string]geometry.Rect)
	if focusedWin != nil {
		exposeFallback[focusedWin.ID] = exposeTargetRect(focusedWin, wa, true)
		bgTargets := exposeBgTargets(transVisible, focusedWin.ID, wa)
		for id, r := range bgTargets {
			exposeFallback[id] = r
		}
	}

	// Draw unfocused windows first (behind), then focused on top
	for _, w := range wm.Windows() {
		if !w.Visible || w.Minimized || w.Focused {
			continue
		}
		renderExposeTransitionWindow(buf, theme, w, animMap, exposeFallback, winNumMap[w.ID])
	}
	for _, w := range wm.Windows() {
		if !w.Visible || w.Minimized || !w.Focused {
			continue
		}
		renderExposeTransitionWindow(buf, theme, w, animMap, exposeFallback, winNumMap[w.ID])
	}

	return buf
}

func renderExposeTransitionWindow(buf *Buffer, theme config.Theme, w *window.Window, animMap map[string]*Animation, exposeFallback map[string]geometry.Rect, winNum int) {
	r := w.Rect
	if a, ok := animMap[w.ID]; ok {
		r = a.currentRect()
	} else if fb, ok := exposeFallback[w.ID]; ok {
		r = fb
	}
	if r.Width < 3 || r.Height < 3 {
		return
	}
	renderExposeMiniWindow(buf, theme, w, r.X, r.Y, r.Width, r.Height, w.Focused, winNum)
}

// writeColorFg writes an ANSI foreground escape sequence to the builder.
func writeColorFg(sb *strings.Builder, c color.Color) {
	if c == nil {
		return
	}
	r, g, b, _ := c.RGBA()
	sb.WriteString("\x1b[38;2;")
	sb.WriteString(strconv.FormatUint(uint64(r>>8), 10))
	sb.WriteByte(';')
	sb.WriteString(strconv.FormatUint(uint64(g>>8), 10))
	sb.WriteByte(';')
	sb.WriteString(strconv.FormatUint(uint64(b>>8), 10))
	sb.WriteByte('m')
}

// writeColorBg writes an ANSI background escape sequence to the builder.
func writeColorBg(sb *strings.Builder, c color.Color) {
	if c == nil {
		return
	}
	r, g, b, _ := c.RGBA()
	sb.WriteString("\x1b[48;2;")
	sb.WriteString(strconv.FormatUint(uint64(r>>8), 10))
	sb.WriteByte(';')
	sb.WriteString(strconv.FormatUint(uint64(g>>8), 10))
	sb.WriteByte(';')
	sb.WriteString(strconv.FormatUint(uint64(b>>8), 10))
	sb.WriteByte('m')
}

// appendSGRColorFg appends ";38;2;R;G;B" to a combined SGR sequence.
// Used within a single \x1b[...m sequence to avoid separate resets.
func appendSGRColorFg(sb *strings.Builder, c color.Color) {
	if c == nil {
		return
	}
	r, g, b, _ := c.RGBA()
	sb.WriteString(";38;2;")
	sb.WriteString(strconv.FormatUint(uint64(r>>8), 10))
	sb.WriteByte(';')
	sb.WriteString(strconv.FormatUint(uint64(g>>8), 10))
	sb.WriteByte(';')
	sb.WriteString(strconv.FormatUint(uint64(b>>8), 10))
}

// appendSGRColorBg appends ";48;2;R;G;B" to a combined SGR sequence.
func appendSGRColorBg(sb *strings.Builder, c color.Color) {
	if c == nil {
		return
	}
	r, g, b, _ := c.RGBA()
	sb.WriteString(";48;2;")
	sb.WriteString(strconv.FormatUint(uint64(r>>8), 10))
	sb.WriteByte(';')
	sb.WriteString(strconv.FormatUint(uint64(g>>8), 10))
	sb.WriteByte(';')
	sb.WriteString(strconv.FormatUint(uint64(b>>8), 10))
}

// attrsToANSI returns ANSI SGR sequences for text attributes.
func attrsToANSI(attrs uint8) string {
	if attrs == 0 {
		return ""
	}
	var parts []string
	if attrs&AttrBold != 0 {
		parts = append(parts, "1")
	}
	if attrs&AttrFaint != 0 {
		parts = append(parts, "2")
	}
	if attrs&AttrItalic != 0 {
		parts = append(parts, "3")
	}
	if attrs&AttrBlink != 0 {
		parts = append(parts, "5")
	}
	if attrs&AttrReverse != 0 {
		parts = append(parts, "7")
	}
	if attrs&AttrConceal != 0 {
		parts = append(parts, "8")
	}
	if attrs&AttrStrikethrough != 0 {
		parts = append(parts, "9")
	}
	if len(parts) == 0 {
		return ""
	}
	return "\x1b[" + strings.Join(parts, ";") + "m"
}

// desaturateColor converts a color toward grayscale.
// t=0 returns the original, t=1 returns fully grayscale.
func desaturateColor(c color.Color, t float64) color.Color {
	if c == nil {
		return nil
	}
	r, g, b, _ := c.RGBA()
	rr := float64(r >> 8)
	gg := float64(g >> 8)
	bb := float64(b >> 8)
	// Perceived luminance (ITU-R BT.709)
	lum := 0.2126*rr + 0.7152*gg + 0.0722*bb
	return color.RGBA{
		R: uint8(rr*(1-t) + lum*t),
		G: uint8(gg*(1-t) + lum*t),
		B: uint8(bb*(1-t) + lum*t),
		A: 255,
	}
}

// blendColor linearly interpolates between two colors.
// t=0 returns c1, t=1 returns c2.
func blendColor(c1, c2 color.Color, t float64) color.Color {
	if c1 == nil {
		return c2
	}
	if c2 == nil {
		return c1
	}
	r1, g1, b1, _ := c1.RGBA()
	r2, g2, b2, _ := c2.RGBA()
	blend := func(a, b uint32) uint8 {
		return uint8((float64(a>>8)*(1-t) + float64(b>>8)*t))
	}
	return color.RGBA{
		R: blend(r1, r2),
		G: blend(g1, g2),
		B: blend(b1, b2),
		A: 255,
	}
}

// colorsEqual compares two color.Color values for equality.
func colorsEqual(a, b color.Color) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	r1, g1, b1, a1 := a.RGBA()
	r2, g2, b2, a2 := b.RGBA()
	return r1 == r2 && g1 == g2 && b1 == b2 && a1 == a2
}

// BufferToString converts the cell buffer to an ANSI-colored string.
// Uses targeted SGR sequences instead of full resets to prevent terminal
// default colors from bleeding through (fixes Termux blue background issue).
func BufferToString(buf *Buffer) string {
	sb := builderPool.Get().(*strings.Builder)
	sb.Reset()
	sb.Grow(buf.Width * buf.Height * 50) // ~180KB for 120x30: each cell needs ~45 bytes for full ANSI color

	var prevFg, prevBg color.Color
	var prevAttrs uint8

	for y := 0; y < buf.Height; y++ {
		for x := 0; x < buf.Width; x++ {
			cell := buf.Cells[y][x]

			fgChanged := !colorsEqual(cell.Fg, prevFg)
			bgChanged := !colorsEqual(cell.Bg, prevBg)
			attrsChanged := cell.Attrs != prevAttrs

			if fgChanged || bgChanged || attrsChanged {
				if attrsChanged {
					// Attrs changed — must reset, then re-emit attrs + both colors
					// in a combined SGR to avoid momentary terminal default flash.
					sb.WriteString("\x1b[0")
					if cell.Attrs&AttrBold != 0 {
						sb.WriteString(";1")
					}
					if cell.Attrs&AttrFaint != 0 {
						sb.WriteString(";2")
					}
					if cell.Attrs&AttrItalic != 0 {
						sb.WriteString(";3")
					}
					if cell.Attrs&AttrBlink != 0 {
						sb.WriteString(";5")
					}
					if cell.Attrs&AttrReverse != 0 {
						sb.WriteString(";7")
					}
					if cell.Attrs&AttrConceal != 0 {
						sb.WriteString(";8")
					}
					if cell.Attrs&AttrStrikethrough != 0 {
						sb.WriteString(";9")
					}
					appendSGRColorFg(sb, cell.Fg)
					appendSGRColorBg(sb, cell.Bg)
					sb.WriteByte('m')
				} else {
					// Only colors changed — use targeted sequences (no reset)
					if fgChanged {
						writeColorFg(sb, cell.Fg)
					}
					if bgChanged {
						writeColorBg(sb, cell.Bg)
					}
				}
				prevFg = cell.Fg
				prevBg = cell.Bg
				prevAttrs = cell.Attrs
			}

			sb.WriteRune(cell.Char)
		}
		if y < buf.Height-1 {
			// No reset at end of line — colors carry over to avoid
			// terminal default bleeding through on line transitions.
			sb.WriteByte('\n')
		}
	}
	sb.WriteString("\x1b[0m") // final reset
	result := sb.String()
	builderPool.Put(sb)
	return result
}

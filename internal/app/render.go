package app

import (
	"fmt"
	"image/color"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/icex/termdesk/internal/config"
	"github.com/icex/termdesk/internal/dock"
	"github.com/icex/termdesk/internal/launcher"
	"github.com/icex/termdesk/internal/menubar"
	"github.com/icex/termdesk/internal/terminal"
	"github.com/icex/termdesk/internal/window"
	"github.com/icex/termdesk/pkg/geometry"

	"github.com/mattn/go-runewidth"
)

// runeLen returns the number of runes (display columns) in a string.
func runeLen(s string) int {
	return utf8.RuneCountInString(s)
}

// Cell represents a single terminal cell with character and style.
type Cell struct {
	Char  rune
	Fg    color.Color // nil = default
	Bg    color.Color // nil = default
	Attrs uint8       // text attributes (bold, italic, etc.)
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
func hexToColor(hex string) color.Color {
	if hex == "" {
		return nil
	}
	if len(hex) == 7 && hex[0] == '#' {
		var r, g, b uint8
		fmt.Sscanf(hex[1:], "%02x%02x%02x", &r, &g, &b)
		return color.RGBA{R: r, G: g, B: b, A: 255}
	}
	return nil
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
			cells[y][x] = Cell{Char: ' ', Fg: fg, Bg: bg}
		}
	}
	return &Buffer{Width: width, Height: height, Cells: cells}
}

// Set sets a cell at the given position if it's within bounds.
// fg and bg are hex color strings like "#RRGGBB", or "" for default.
func (b *Buffer) Set(x, y int, char rune, fg, bg string) {
	if x >= 0 && x < b.Width && y >= 0 && y < b.Height {
		b.Cells[y][x] = Cell{Char: char, Fg: hexToColor(fg), Bg: hexToColor(bg)}
	}
}

// SetCell sets a cell at the given position with color.Color values directly.
func (b *Buffer) SetCell(x, y int, char rune, fg, bg color.Color, attrs uint8) {
	if x >= 0 && x < b.Width && y >= 0 && y < b.Height {
		b.Cells[y][x] = Cell{Char: char, Fg: fg, Bg: bg, Attrs: attrs}
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
func RenderWindow(buf *Buffer, w *window.Window, theme config.Theme, term *terminal.Terminal) {
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
	} else if w.HasNotification {
		borderFg = theme.NotificationFg
		borderBg = theme.NotificationBg
	}

	// Fill content area with spaces (clear background)
	contentRect := w.ContentRect()
	contentBg := theme.ContentBg
	if contentBg == "" {
		contentBg = borderBg
	}
	buf.FillRect(contentRect, ' ', borderFg, contentBg)

	tbh := w.TitleBarHeight
	if tbh < 1 {
		tbh = 1
	}

	// Draw title bar rows
	for row := 0; row < tbh; row++ {
		ty := r.Y + row
		if row == 0 {
			// First row: top border corners
			buf.Set(r.X, ty, theme.BorderTopLeft, borderFg, titleBg)
			for x := r.X + 1; x < r.Right()-1; x++ {
				buf.Set(x, ty, theme.BorderHorizontal, borderFg, titleBg)
			}
			buf.Set(r.Right()-1, ty, theme.BorderTopRight, borderFg, titleBg)
		} else {
			// Extra title bar rows: side borders + fill
			buf.Set(r.X, ty, theme.BorderVertical, borderFg, titleBg)
			for x := r.X + 1; x < r.Right()-1; x++ {
				buf.Set(x, ty, ' ', titleFg, titleBg)
			}
			buf.Set(r.Right()-1, ty, theme.BorderVertical, borderFg, titleBg)
		}
	}

	// Place title text and buttons on the center row of the title bar
	titleRow := r.Y + tbh/2

	// Draw title text (left-aligned in title bar)
	closeW := runeLen(theme.CloseButton)
	minW := runeLen(theme.MinButton)
	maxW := runeLen(theme.MaxButton)
	snapLW := runeLen(theme.SnapLeftButton)
	snapRW := runeLen(theme.SnapRightButton)
	buttonsW := closeW + minW
	if w.Resizable {
		buttonsW += maxW + snapLW + snapRW
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
	buf.SetString(titleX, titleRow, " "+title+" ", titleFg, titleBg)

	// Draw title bar buttons (right side): [◧][◨][□][_][×]
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
		buf.SetString(snapLX, titleRow, theme.SnapLeftButton, titleFg, titleBg)
		buf.SetString(snapRX, titleRow, theme.SnapRightButton, titleFg, titleBg)
		buf.SetString(maxX, titleRow, maxStr, titleFg, titleBg)
	}
	buf.SetString(minX, titleRow, theme.MinButton, titleFg, titleBg)
	buf.SetString(closeX, titleRow, theme.CloseButton, titleFg, titleBg)

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

	// Draw terminal content if present
	if term != nil {
		cbg := theme.ContentBg
		if cbg == "" {
			cbg = borderBg
		}
		// Default FG: light gray text for cells with no explicit foreground
		defaultFg := hexToColor("#C0C0C0")
		renderTerminalContent(buf, contentRect, term, defaultFg, hexToColor(cbg))
	}

	// Desaturate unfocused windows — go monochrome + dim slightly
	if !w.Focused && theme.UnfocusedFade > 0 {
		dimBg := hexToColor(theme.DesktopBg)
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
func renderTerminalContent(buf *Buffer, area geometry.Rect, term *terminal.Terminal, defaultFg, defaultBg color.Color) {
	if term == nil {
		return
	}
	termW := term.Width()
	termH := term.Height()
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
			buf.SetCell(area.X+dx, area.Y+dy, ch, fg, bg, cell.Style.Attrs)
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
func RenderFrame(wm *window.Manager, theme config.Theme, terminals map[string]*terminal.Terminal, animRects map[string]geometry.Rect) *Buffer {
	wa := wm.WorkArea()
	// Use full terminal bounds for the buffer (includes reserved rows for menu/dock)
	fullWidth := wa.Width
	fullHeight := wa.Height + wa.Y + wm.ReservedBottom()
	if fullWidth <= 0 || fullHeight <= 0 {
		return NewBuffer(1, 1, theme.DesktopBg)
	}

	buf := NewBuffer(fullWidth, fullHeight, theme.DesktopBg)

	// Draw windows back-to-front (painter's algorithm)
	for _, w := range wm.Windows() {
		var term *terminal.Terminal
		if terminals != nil {
			term = terminals[w.ID]
		}
		// Use animated rect if available
		if animRect, ok := animRects[w.ID]; ok {
			origRect := w.Rect
			w.Rect = animRect
			// Skip terminal content during close animation (window is shrinking)
			if animRect.Width <= 3 || animRect.Height <= 3 {
				RenderWindow(buf, w, theme, nil)
			} else {
				RenderWindow(buf, w, theme, term)
			}
			w.Rect = origRect // restore for state consistency
		} else {
			RenderWindow(buf, w, theme, term)
		}
	}

	return buf
}

// RenderMenuBar draws the menu bar at the top of the buffer.
func RenderMenuBar(buf *Buffer, mb *menubar.MenuBar, theme config.Theme, mode InputMode) {
	if mb == nil || buf.Height < 1 {
		return
	}

	// Fill menu bar row with menu bar background
	mbFg := theme.MenuBarFg
	mbBg := theme.MenuBarBg
	if mbFg == "" {
		mbFg = theme.ActiveTitleFg
	}
	if mbBg == "" {
		mbBg = theme.ActiveTitleBg
	}
	for x := 0; x < buf.Width; x++ {
		buf.Set(x, 0, ' ', mbFg, mbBg)
	}

	// Compute mode badge (placed at far right, fixed width)
	modeLabel := modeBadge(mode)
	modeLabelLen := runewidth.StringWidth(modeLabel)
	var modeFg, modeBg string
	switch mode {
	case ModeTerminal:
		modeFg = "#1E1E2E"
		modeBg = "#98C379" // green
	case ModeCopy:
		modeFg = "#1E1E2E"
		modeBg = "#61AFEF" // blue
	default: // ModeNormal
		modeFg = "#1E1E2E"
		modeBg = "#E5C07B" // yellow
	}

	// Render bar text with reduced width (leave room for mode badge)
	effectiveWidth := buf.Width - modeLabelLen
	if effectiveWidth < 1 {
		effectiveWidth = 1
	}
	barText := mb.Render(effectiveWidth)
	col := 0
	for _, ch := range barText {
		if col < buf.Width {
			buf.Set(col, 0, ch, mbFg, mbBg)
		}
		col++
	}

	// Colorize right-side CPU/MEM indicators
	for _, zone := range mb.RightZones(effectiveWidth) {
		var colorHex string
		switch zone.Type {
		case "cpu":
			colorHex = levelColor(menubar.CPUColorLevel(mb.CPUPct))
		case "mem":
			_, totalGB := menubar.ReadMemoryInfo()
			colorHex = levelColor(menubar.MemColorLevel(mb.MemGB, totalGB))
		}
		if colorHex != "" {
			for x := zone.Start; x < zone.End && x < buf.Width; x++ {
				if x >= 0 && x < buf.Width {
					buf.Cells[0][x].Fg = hexToColor(colorHex)
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
		buf.Set(modeX+col, 0, ch, modeFg, modeBg)
		col += runewidth.RuneWidth(ch)
	}

	// Render dropdown if open
	if mb.IsOpen() {
		positions := mb.MenuXPositions()
		dropX := positions[mb.OpenIndex]
		lines := mb.RenderDropdown()
		for dy, line := range lines {
			dcol := 0
			for _, ch := range line {
				buf.Set(dropX+dcol, 1+dy, ch, theme.ActiveTitleFg, theme.ActiveBorderBg)
				dcol++
			}
		}
	}
}

// levelColor maps a color level name to a hex color.
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

	y := buf.Height - 1

	// Fill dock row with base dock colors
	for x := 0; x < buf.Width; x++ {
		buf.Set(x, y, ' ', theme.DockFg, theme.DockBg)
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
		fg := theme.DockFg
		bg := theme.DockBg
		if cell.Minimized {
			fg = theme.NotificationFg // distinct color for minimized items
			if fg == "" {
				fg = "#E5C07B" // yellow fallback
			}
		}
		if cell.Accent {
			bg = theme.DockAccentBg
		}
		buf.Set(x, y, cell.Char, fg, bg)
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

	title := dialog.Title
	boxW := runeLen(title) + 6
	if boxW < 28 {
		boxW = 28
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

	// Title row with title bar background
	buf.Set(startX, startY+1, theme.BorderVertical, fg, titleBg)
	for x := 1; x < boxW-1; x++ {
		buf.Set(startX+x, startY+1, ' ', fg, titleBg)
	}
	titleX := startX + (boxW-runeLen(title))/2
	buf.SetString(titleX, startY+1, title, fg, titleBg)
	buf.Set(startX+boxW-1, startY+1, theme.BorderVertical, fg, titleBg)

	// Separator between title and content
	buf.Set(startX, startY+2, theme.BorderVertical, fg, bg)
	for x := 1; x < boxW-1; x++ {
		buf.Set(startX+x, startY+2, theme.BorderHorizontal, fg, bg)
	}
	buf.Set(startX+boxW-1, startY+2, theme.BorderVertical, fg, bg)

	// Hint row
	hint := "Press Y or N"
	buf.Set(startX, startY+3, theme.BorderVertical, fg, bg)
	for x := 1; x < boxW-1; x++ {
		buf.Set(startX+x, startY+3, ' ', fg, bg)
	}
	hintX := startX + (boxW-runeLen(hint))/2
	buf.SetString(hintX, startY+3, hint, fg, bg)
	buf.Set(startX+boxW-1, startY+3, theme.BorderVertical, fg, bg)

	// Buttons row with styled buttons — highlight selected
	selYesFg := "#1E1E2E"
	selYesBg := "#98C379" // green (selected)
	selNoFg := "#1E1E2E"
	selNoBg := "#E06C75" // red (selected)
	dimFg := fg
	dimBg := bg // unselected = blends into dialog bg
	buf.Set(startX, startY+4, theme.BorderVertical, fg, bg)
	for x := 1; x < boxW-1; x++ {
		buf.Set(startX+x, startY+4, ' ', fg, bg)
	}
	// Position buttons centered with gap
	yesLabel := " Yes "
	noLabel := "  No "
	gap := boxW - 2 - len(yesLabel) - len(noLabel)
	if gap < 4 {
		gap = 4
	}
	yesX := startX + 1 + (gap / 3)
	noX := startX + boxW - 1 - len(noLabel) - (gap / 3)
	// Draw Yes button
	yf, yb := dimFg, dimBg
	if dialog.Selected == 0 {
		yf, yb = selYesFg, selYesBg
	}
	for i, ch := range yesLabel {
		buf.Set(yesX+i, startY+4, ch, yf, yb)
	}
	// Draw No button
	nf, nb := dimFg, dimBg
	if dialog.Selected == 1 {
		nf, nb = selNoFg, selNoBg
	}
	for i, ch := range noLabel {
		buf.Set(noX+i, startY+4, ch, nf, nb)
	}
	buf.Set(startX+boxW-1, startY+4, theme.BorderVertical, fg, bg)

	// Bottom border
	buf.Set(startX, startY+5, theme.BorderBottomLeft, fg, bg)
	for x := 1; x < boxW-1; x++ {
		buf.Set(startX+x, startY+5, theme.BorderHorizontal, fg, bg)
	}
	buf.Set(startX+boxW-1, startY+5, theme.BorderBottomRight, fg, bg)
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
func RenderModal(buf *Buffer, modal *ModalOverlay, theme config.Theme) {
	if modal == nil {
		return
	}

	// Calculate box dimensions
	maxLineW := runeLen(modal.Title)
	for _, line := range modal.Lines {
		if w := runeLen(line); w > maxLineW {
			maxLineW = w
		}
	}
	boxW := maxLineW + 4 // 2 border + 2 padding
	if boxW > buf.Width-4 {
		boxW = buf.Width - 4
	}

	visibleLines := buf.Height - 8
	if visibleLines < 3 {
		visibleLines = 3
	}
	if visibleLines > len(modal.Lines) {
		visibleLines = len(modal.Lines)
	}

	boxH := visibleLines + 4 // top border + title + empty + lines + bottom border
	startX := (buf.Width - boxW) / 2
	startY := (buf.Height - boxH) / 2
	if startX < 0 {
		startX = 0
	}
	if startY < 0 {
		startY = 0
	}

	fg := theme.ActiveTitleFg
	bg := theme.ActiveTitleBg
	innerW := boxW - 2

	// Clamp scroll
	maxScroll := len(modal.Lines) - visibleLines
	if maxScroll < 0 {
		maxScroll = 0
	}
	if modal.ScrollY > maxScroll {
		modal.ScrollY = maxScroll
	}

	// Top border
	buf.Set(startX, startY, theme.BorderTopLeft, fg, bg)
	for x := 1; x < boxW-1; x++ {
		buf.Set(startX+x, startY, theme.BorderHorizontal, fg, bg)
	}
	buf.Set(startX+boxW-1, startY, theme.BorderTopRight, fg, bg)

	// Title row
	buf.Set(startX, startY+1, theme.BorderVertical, fg, bg)
	for x := 1; x < boxW-1; x++ {
		buf.Set(startX+x, startY+1, ' ', fg, bg)
	}
	titleX := startX + (boxW-runeLen(modal.Title))/2
	buf.SetString(titleX, startY+1, modal.Title, fg, bg)
	buf.Set(startX+boxW-1, startY+1, theme.BorderVertical, fg, bg)

	// Separator
	buf.Set(startX, startY+2, theme.BorderVertical, fg, bg)
	for x := 1; x < boxW-1; x++ {
		buf.Set(startX+x, startY+2, theme.BorderHorizontal, fg, bg)
	}
	buf.Set(startX+boxW-1, startY+2, theme.BorderVertical, fg, bg)

	// Content lines
	for i := 0; i < visibleLines; i++ {
		lineIdx := modal.ScrollY + i
		y := startY + 3 + i
		buf.Set(startX, y, theme.BorderVertical, fg, bg)
		for x := 1; x < boxW-1; x++ {
			buf.Set(startX+x, y, ' ', fg, bg)
		}
		buf.Set(startX+boxW-1, y, theme.BorderVertical, fg, bg)
		if lineIdx < len(modal.Lines) {
			line := modal.Lines[lineIdx]
			lineRunes := []rune(line)
			if len(lineRunes) > innerW {
				line = string(lineRunes[:innerW])
			}
			buf.SetString(startX+2, y, line, fg, bg)
		}
	}

	// Bottom border
	botY := startY + 3 + visibleLines
	buf.Set(startX, botY, theme.BorderBottomLeft, fg, bg)
	for x := 1; x < boxW-1; x++ {
		buf.Set(startX+x, botY, theme.BorderHorizontal, fg, bg)
	}
	buf.Set(startX+boxW-1, botY, theme.BorderBottomRight, fg, bg)
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
					buf.Cells[titleY][bx] = Cell{Char: ch, Fg: fgColor, Bg: bgColor, Attrs: AttrBold}
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
					buf.Cells[ly][bx] = Cell{Char: ch, Fg: fgColor, Bg: bgColor}
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
		return NewBuffer(1, 1, theme.DesktopBg)
	}

	buf := NewBuffer(fullWidth, fullHeight, theme.DesktopBg)

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
		return NewBuffer(1, 1, theme.DesktopBg)
	}

	buf := NewBuffer(fullWidth, fullHeight, theme.DesktopBg)

	// Build animation lookup
	animMap := make(map[string]*Animation)
	for i := range animations {
		a := &animations[i]
		if (a.Type == AnimExpose || a.Type == AnimExposeExit) && !a.Done {
			animMap[a.WindowID] = a
		}
	}

	// Build window number map
	winNumMap := make(map[string]int)
	visIdx := 1
	for _, w := range wm.Windows() {
		if w.Visible && !w.Minimized {
			winNumMap[w.ID] = visIdx
			visIdx++
		}
	}

	// Draw unfocused windows first (behind), then focused on top
	for _, w := range wm.Windows() {
		if !w.Visible || w.Minimized || w.Focused {
			continue
		}
		renderExposeTransitionWindow(buf, theme, w, animMap, winNumMap[w.ID])
	}
	for _, w := range wm.Windows() {
		if !w.Visible || w.Minimized || !w.Focused {
			continue
		}
		renderExposeTransitionWindow(buf, theme, w, animMap, winNumMap[w.ID])
	}

	return buf
}

func renderExposeTransitionWindow(buf *Buffer, theme config.Theme, w *window.Window, animMap map[string]*Animation, winNum int) {
	r := w.Rect
	if a, ok := animMap[w.ID]; ok {
		t := easeOutCubic(a.Progress)
		r = interpolateRect(a.StartRect, a.EndRect, t)
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
	var sb strings.Builder
	sb.Grow(buf.Width * buf.Height * 10) // generous for ANSI sequences

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
					appendSGRColorFg(&sb, cell.Fg)
					appendSGRColorBg(&sb, cell.Bg)
					sb.WriteByte('m')
				} else {
					// Only colors changed — use targeted sequences (no reset)
					if fgChanged {
						writeColorFg(&sb, cell.Fg)
					}
					if bgChanged {
						writeColorBg(&sb, cell.Bg)
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
	return sb.String()
}

package app

import (
	"fmt"
	"image/color"
	"strings"
	"unicode/utf8"

	"github.com/mattn/go-runewidth"

	"github.com/icex/termdesk/internal/config"
	"github.com/icex/termdesk/internal/terminal"
	"github.com/icex/termdesk/internal/window"
	"github.com/icex/termdesk/pkg/geometry"
)

// RenderWindow draws a single window (border + title bar + content) into the buffer.
// showCursor controls whether the terminal cursor is rendered (false = blink-off phase).
// scrollOffset > 0 shows scrollback lines in Copy mode.
func RenderWindow(buf *Buffer, w *window.Window, theme config.Theme, term *terminal.Terminal, showCursor bool, scrollOffset int, hoverZone window.HitZone) {
	renderWindowChrome(buf, w, theme, hoverZone)
	renderWindowTerminalContent(buf, w, theme, term, showCursor, scrollOffset)
	if w.Exited {
		renderExitedOverlay(buf, w, theme)
	}
}

// renderWindowChrome draws window border, title bar, and content background.
func renderWindowChrome(buf *Buffer, w *window.Window, theme config.Theme, hoverZone window.HitZone) {
	if !w.Visible || w.Minimized {
		return
	}

	r := w.Rect
	if r.Width < 3 || r.Height < 3 {
		return
	}

	c := theme.C()

	topLeft := theme.BorderTopLeft
	topRight := theme.BorderTopRight
	bottomLeft := theme.BorderBottomLeft
	bottomRight := theme.BorderBottomRight

	// Pick pre-parsed colors based on focus state
	borderFg := c.InactiveBorderFg
	borderBg := c.InactiveBorderBg
	titleFg := c.InactiveTitleFg
	titleBg := c.InactiveTitleBg
	if w.Stuck || w.Exited {
		// Stuck or exited terminal — red border and title to signal unresponsive/dead process
		stuckRed := color.RGBA{R: 220, G: 50, B: 50, A: 255}
		borderFg = stuckRed
		titleFg = stuckRed
		titleBg = c.InactiveTitleBg
	} else if w.Focused {
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

	maximized := w.IsMaximized()

	// Draw title bar rows
	for row := 0; row < tbh; row++ {
		ty := r.Y + row
		if maximized {
			// Maximized: no corner chars, fill entire width with title bar background
			for x := r.X; x < r.Right(); x++ {
				if row == 0 {
					buf.SetCell(x, ty, theme.BorderHorizontal, borderFg, titleBg, 0)
				} else {
					buf.SetCell(x, ty, ' ', titleFg, titleBg, 0)
				}
			}
		} else if row == 0 {
			buf.SetCell(r.X, ty, topLeft, borderFg, titleBg, 0)
			for x := r.X + 1; x < r.Right()-1; x++ {
				buf.SetCell(x, ty, theme.BorderHorizontal, borderFg, titleBg, 0)
			}
			buf.SetCell(r.Right()-1, ty, topRight, borderFg, titleBg, 0)
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
	borderInset := 1
	if maximized {
		borderInset = 0
	}
	titleSpace := r.Width - 2*borderInset - buttonsW
	// Build title with optional icon prefix
	iconPrefix := ""
	var iconFg color.Color
	if w.Icon != "" {
		iconPrefix = w.Icon + "  "
		if w.IconColor != "" {
			iconFg = hexToColor(w.IconColor)
			if theme.IsLight() {
				iconFg = darkenColor(iconFg, 0.65)
			}
		}
	}
	title := w.Title
	// Prepend bell icon for unfocused windows with active bell
	bellPrefix := ""
	if w.HasBell && !w.Focused {
		bellPrefix = "\U000F0027 " // 󰀧 bell icon
	}
	fullTitle := iconPrefix + bellPrefix + title
	fullRunes := []rune(fullTitle)
	if len(fullRunes) > titleSpace {
		if titleSpace > 3 {
			fullTitle = string(fullRunes[:titleSpace-3]) + "..."
		} else if titleSpace > 0 {
			fullTitle = string(fullRunes[:titleSpace])
		} else {
			fullTitle = ""
		}
	}
	titleX := r.X + borderInset
	curX := titleX
	// Leading space
	buf.SetCell(curX, titleRow, ' ', titleFg, titleBg, 0)
	curX++
	if iconFg != nil && w.Icon != "" && len([]rune(fullTitle)) >= len([]rune(iconPrefix)) {
		// Render icon in its own color
		buf.SetStringCA(curX, titleRow, w.Icon+"  ", iconFg, titleBg, AttrBold)
		curX += utf8.RuneCountInString(w.Icon) + 2
		// Render bell icon in yellow if present
		if bellPrefix != "" {
			buf.SetStringCA(curX, titleRow, bellPrefix, levelYellow, titleBg, AttrBold)
			curX += utf8.RuneCountInString(bellPrefix)
		}
		// Render remaining title text
		remaining := string([]rune(fullTitle)[len([]rune(iconPrefix))+len([]rune(bellPrefix)):])
		buf.SetStringCA(curX, titleRow, remaining+" ", titleFg, titleBg, AttrBold)
	} else if bellPrefix != "" {
		// No icon but has bell — render bell in yellow, then title
		buf.SetStringCA(curX, titleRow, bellPrefix, levelYellow, titleBg, AttrBold)
		curX += utf8.RuneCountInString(bellPrefix)
		remaining := string([]rune(fullTitle)[len([]rune(bellPrefix)):])
		buf.SetStringCA(curX, titleRow, remaining+" ", titleFg, titleBg, AttrBold)
	} else {
		buf.SetStringCA(curX, titleRow, fullTitle+" ", titleFg, titleBg, AttrBold)
	}

	// Compute button hover colors — if accent blends into title bar, invert title colors instead
	btnHoverFg, btnHoverBg := c.AccentFg, c.AccentColor
	if colorsEqual(btnHoverBg, titleBg) {
		btnHoverFg, btnHoverBg = titleBg, titleFg
	}

	// Draw title bar buttons (right side) — highlight on hover
	closeX := r.Right() - borderInset - closeW
	minX := closeX - minW
	if w.Resizable {
		maxStr := theme.MaxButton
		if w.IsMaximized() {
			maxStr = theme.RestoreButton
		}
		maxX := minX - maxW
		snapRX := maxX - snapRW
		snapLX := snapRX - snapLW
		snapLFg, snapLBg := titleFg, titleBg
		if hoverZone == window.HitSnapLeftButton {
			snapLFg, snapLBg = btnHoverFg, btnHoverBg
		}
		snapRFg, snapRBg := titleFg, titleBg
		if hoverZone == window.HitSnapRightButton {
			snapRFg, snapRBg = btnHoverFg, btnHoverBg
		}
		maxFg, maxBg := titleFg, titleBg
		if w.Focused && c.MaxButtonFg != nil {
			maxFg = c.MaxButtonFg
		}
		if hoverZone == window.HitMaxButton {
			maxFg, maxBg = btnHoverFg, btnHoverBg
		}
		buf.SetStringC(snapLX, titleRow, theme.SnapLeftButton, snapLFg, snapLBg)
		buf.SetStringC(snapRX, titleRow, theme.SnapRightButton, snapRFg, snapRBg)
		buf.SetStringC(maxX, titleRow, maxStr, maxFg, maxBg)
	}
	minFg, minBg := titleFg, titleBg
	if w.Focused && c.MinButtonFg != nil {
		minFg = c.MinButtonFg
	}
	if hoverZone == window.HitMinButton {
		minFg, minBg = btnHoverFg, btnHoverBg
	}
	closeFg, closeBg := titleFg, titleBg
	if w.Focused && c.CloseButtonFg != nil {
		closeFg = c.CloseButtonFg
	}
	if hoverZone == window.HitCloseButton {
		closeFg, closeBg = c.ButtonFg, c.ButtonNoBg
	}
	buf.SetStringC(minX, titleRow, theme.MinButton, minFg, minBg)
	buf.SetStringC(closeX, titleRow, theme.CloseButton, closeFg, closeBg)

	// Draw bottom border and side borders (skip when maximized)
	if !maximized {
		buf.SetCell(r.X, r.Bottom()-1, bottomLeft, borderFg, borderBg, 0)
		for x := r.X + 1; x < r.Right()-1; x++ {
			buf.SetCell(x, r.Bottom()-1, theme.BorderHorizontal, borderFg, borderBg, 0)
		}
		buf.SetCell(r.Right()-1, r.Bottom()-1, bottomRight, borderFg, borderBg, 0)

		for y := r.Y + 1; y < r.Bottom()-1; y++ {
			buf.SetCell(r.X, y, theme.BorderVertical, borderFg, borderBg, 0)
			buf.SetCell(r.Right()-1, y, theme.BorderVertical, borderFg, borderBg, 0)
		}
	}
}

// renderWindowTerminalContent draws terminal content, cursor, and unfocused fade.
func renderWindowTerminalContent(buf *Buffer, w *window.Window, theme config.Theme, term *terminal.Terminal, showCursor bool, scrollOffset int) {
	renderWindowTerminalContentWithSnapshot(buf, w, theme, term, showCursor, scrollOffset, nil)
}

func renderWindowTerminalContentWithSnapshot(buf *Buffer, w *window.Window, theme config.Theme, term *terminal.Terminal, showCursor bool, scrollOffset int, copySnap *CopySnapshot) {
	if !w.Visible || w.Minimized || term == nil {
		return
	}
	c := theme.C()
	contentRect := w.ContentRect()
	borderBg := c.InactiveBorderBg
	if w.Focused {
		borderBg = c.ActiveBorderBg
	} else if w.HasNotification {
		borderBg = c.NotificationBg
	}
	contentBg := c.ContentBg
	if contentBg == nil {
		contentBg = borderBg
	}

	defaultFg := c.DefaultFg
	renderTerminalContentWithSnapshot(buf, contentRect, term, defaultFg, contentBg, scrollOffset, copySnap)

	// Show scroll indicator when scrolled back
	if scrollOffset > 0 {
		maxScroll := term.ScrollbackLen()
		if copySnap != nil {
			maxScroll = copySnap.ScrollbackLen()
		}
		indicator := fmt.Sprintf(" [↑ %d/%d] ", scrollOffset, maxScroll)
		indicatorRunes := []rune(indicator)
		borderY := w.Rect.Bottom() - 1
		startX := w.Rect.X + (w.Rect.Width-len(indicatorRunes))/2
		indFg, indBg := c.ActiveBorderFg, c.ActiveBorderBg
		minX := w.Rect.X
		maxX := w.Rect.Right() - 1
		if w.IsMaximized() {
			// No bottom border — overlay on last content row
			minX = contentRect.X - 1
			maxX = contentRect.Right()
		}
		for i, ch := range indicatorRunes {
			x := startX + i
			if x > minX && x < maxX {
				buf.SetCell(x, borderY, ch, indFg, indBg, 0)
			}
		}
	}

	// Buffer-based cursor: make cursor position visually distinct as a fallback.
	// The real cursor is positioned natively via tea.Cursor on the View (handles
	// shape, blink, and terminal-native rendering). This fallback ensures the
	// cursor cell is always visible even when apps like powerlevel10k set
	// specific colors that might make a simple fg/bg swap invisible.
	if w.Focused && !term.IsCursorHidden() && scrollOffset == 0 {
		cx, cy := term.CursorPosition()
		sx := contentRect.X + cx
		sy := contentRect.Y + cy
		if sx >= contentRect.X && sx < contentRect.Right() &&
			sy >= contentRect.Y && sy < contentRect.Bottom() &&
			sx >= 0 && sx < buf.Width && sy >= 0 && sy < buf.Height {
			cell := &buf.Cells[sy][sx]
			if cell.Width == 0 {
				cell.Char = ' '
				cell.Width = 1
			}
			// Use accent color for guaranteed cursor visibility.
			// A simple fg/bg swap can produce invisible cursors when
			// terminal apps (p10k, starship) use colors close to the
			// terminal background or when cells have nil/default colors.
			cell.Fg = c.AccentFg
			cell.Bg = c.AccentColor
		}
	}

	// Desaturate + darken unfocused windows
	if !w.Focused && theme.UnfocusedFade > 0 {
		dimBg := c.DesktopBg
		for y := contentRect.Y; y < contentRect.Bottom(); y++ {
			for x := contentRect.X; x < contentRect.Right(); x++ {
				if x >= 0 && x < buf.Width && y >= 0 && y < buf.Height {
					cell := &buf.Cells[y][x]
					cell.Fg = desaturateColor(cell.Fg, theme.UnfocusedFade)
					dBg := desaturateColor(cell.Bg, theme.UnfocusedFade)
					dBg = darkenColor(dBg, 1.0-theme.UnfocusedFade*0.15)
					cell.Bg = blendColor(dBg, dimBg, theme.UnfocusedFade*0.3)
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
	renderTerminalContentWithSnapshot(buf, area, term, defaultFg, defaultBg, scrollOffset, nil)
}

func renderTerminalContentWithSnapshot(buf *Buffer, area geometry.Rect, term *terminal.Terminal, defaultFg, defaultBg color.Color, scrollOffset int, copySnap *CopySnapshot) {
	if term == nil {
		return
	}
	snap, termW, termH := term.SnapshotScreen()
	if copySnap != nil {
		snap = copySnap.Screen
		termW = copySnap.Width
		termH = copySnap.Height
	} else if snap == nil {
		termW = term.Width()
		termH = term.Height()
	}

	if scrollOffset <= 0 {
		// Live view — render directly from emulator.
		// Nil-bg cells use defaultBg (theme ContentBg), which matches the
		// emulator's default set by SetDefaultColors(). Apps that want a
		// specific background set it via SGR — those cells already have
		// explicit Bg colors. Using the theme bg for nil cells is correct:
		// it's what the terminal "default background" means, same as any
		// real terminal emulator.
		emuBg := defaultBg

		// Render the FULL content area. For cells beyond the terminal grid,
		// fill with emuBg to match the terminal's background.
		for dy := 0; dy < area.Height; dy++ {
			dx := 0
			for dx < area.Width {
				bx, by := area.X+dx, area.Y+dy
				// Bounds check
				if bx < 0 || bx >= buf.Width || by < 0 || by >= buf.Height {
					dx++
					continue
				}
				// Beyond terminal grid — fill with emulator background
				if dy >= termH || dx >= termW {
					buf.Cells[by][bx] = Cell{Char: ' ', Fg: defaultFg, Bg: emuBg, Width: 1}
					dx++
					continue
				}

				var cell terminal.ScreenCell
				if snap != nil && dy < len(snap) && dx < len(snap[dy]) {
					cell = snap[dy][dx]
				} else if snap == nil {
					if c := term.CellAt(dx, dy); c != nil {
						cell = terminal.ScreenCell{
							Content: c.Content,
							Fg:      c.Style.Fg,
							Bg:      c.Style.Bg,
							Attrs:   c.Style.Attrs,
							Width:   int8(c.Width),
						}
					}
				}

				// Get character width - wide characters (CJK) have Width > 1
				w := cell.Width
				if w == 0 {
					// Uninitialized cell: fill with emulator bg to prevent bleed-through
					buf.Cells[by][bx] = Cell{Char: ' ', Fg: defaultFg, Bg: emuBg, Width: 1}
					dx++
					continue
				}
				if w < 0 {
					w = 1
				}

				ch := firstRune(cell.Content)
				bg := cell.Bg
				if bg == nil {
					bg = emuBg
				}
				fg := cell.Fg
				if fg == nil {
					fg = defaultFg
				}

				// Clip wide characters that would extend past the content area.
				// Without this, BufferToString skips the border cell (Width>1
				// causes x to jump past it), making text overflow into the border.
				clippedContent := ""
				if int(w) > 1 && dx+int(w) > area.Width {
					ch = ' '
					w = 1
				} else if len(cell.Content) > utf8.RuneLen(ch) {
					clippedContent = stripVS16(cell.Content, int(w))
				}

				// Write cell within content area bounds.
				if bx >= area.X && bx < area.X+area.Width && by >= area.Y && by < area.Y+area.Height {
					c := Cell{Char: ch, Fg: fg, Bg: bg, Attrs: cell.Attrs, Width: w, Content: clippedContent}
					buf.Cells[by][bx] = c
				}

				// Advance by character width (1 for normal, 2 for wide characters)
				// This skips the continuation cell that the VT emulator places for wide chars
				dx += int(w)
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

	// Use theme default for nil cells in scrollback view too.
	fillBg := defaultBg

	// Render scrollback lines at the top
	for dy := 0; dy < scrollLines; dy++ {
		// scrollback offset: most recent scrollback line = offset 0
		// We want the topmost display row to show the oldest of the visible scrollback range
		sbIdx := scrollOffset - 1 - dy
		line := term.ScrollbackLine(sbIdx)
		if copySnap != nil {
			line = copySnap.ScrollbackLine(sbIdx)
		}
		dx := 0
		for dx < area.Width {
			if line != nil && dx < len(line) {
				sc := line[dx]

				// Get character width
				w := int(sc.Width)
				// Skip continuation cells
				if w == 0 {
					dx++
					continue
				}
				if w < 0 {
					w = 1
				}

				ch := firstRune(sc.Content)
				bg := sc.Bg
				if bg == nil {
					bg = fillBg
				}
				fg := sc.Fg
				if fg == nil {
					fg = defaultFg
				}

				// Clip wide characters at content area edge
				clippedContent := ""
				if w > 1 && dx+w > area.Width {
					ch = ' '
					w = 1
				} else if len(sc.Content) > utf8.RuneLen(ch) {
					clippedContent = stripVS16(sc.Content, w)
				}

				bx, by := area.X+dx, area.Y+dy
				if bx >= 0 && bx < buf.Width && by >= 0 && by < buf.Height {
					c := Cell{Char: ch, Fg: fg, Bg: bg, Attrs: sc.Attrs, Width: int8(w), Content: clippedContent}
					buf.Cells[by][bx] = c
				}

				// Advance by character width
				dx += w
			} else {
				buf.SetCell(area.X+dx, area.Y+dy, ' ', defaultFg, fillBg, 0)
				dx++
			}
		}
	}

	// Render emulator rows below the scrollback
	maxRows := emuLines
	if termH < maxRows {
		maxRows = termH
	}

	for dy := 0; dy < maxRows; dy++ {
		dx := 0
		for dx < area.Width {
			bx, by := area.X+dx, area.Y+scrollLines+dy
			// Bounds check
			if bx < 0 || bx >= buf.Width || by < 0 || by >= buf.Height {
				dx++
				continue
			}
			// Beyond terminal grid — fill with detected app background
			if dx >= termW {
				buf.Cells[by][bx] = Cell{Char: ' ', Fg: defaultFg, Bg: fillBg, Width: 1}
				dx++
				continue
			}

			var cell terminal.ScreenCell
			if snap != nil && dy < len(snap) && dx < len(snap[dy]) {
				cell = snap[dy][dx]
			} else if snap == nil {
				if c := term.CellAt(dx, dy); c != nil {
					cell = terminal.ScreenCell{
						Content: c.Content,
						Fg:      c.Style.Fg,
						Bg:      c.Style.Bg,
						Attrs:   c.Style.Attrs,
						Width:   int8(c.Width),
					}
				}
			}

			// Get character width - wide characters (CJK) have Width > 1
			w := cell.Width
			if w == 0 {
				// Uninitialized cell: fill with detected app bg
				buf.Cells[by][bx] = Cell{Char: ' ', Fg: defaultFg, Bg: fillBg, Width: 1}
				dx++
				continue
			}
			if w < 0 {
				w = 1
			}

			ch := firstRune(cell.Content)
			bg := cell.Bg
			if bg == nil {
				bg = fillBg
			}
			fg := cell.Fg
			if fg == nil {
				fg = defaultFg
			}

			// Clip wide characters at content area edge
			clippedContent := ""
			if int(w) > 1 && dx+int(w) > area.Width {
				ch = ' '
				w = 1
			} else if len(cell.Content) > utf8.RuneLen(ch) {
				clippedContent = cell.Content
			}

			// Double-check bounds before writing
			if bx >= area.X && bx < area.X+area.Width && by >= area.Y && by < area.Y+area.Height {
				c := Cell{Char: ch, Fg: fg, Bg: bg, Attrs: cell.Attrs, Width: w, Content: clippedContent}
				buf.Cells[by][bx] = c
			}

			// Advance by character width (1 for normal, 2 for wide characters)
			dx += int(w)
		}
	}
}

// renderSearchHighlights highlights all search matches in the visible viewport.
func renderSearchHighlights(buf *Buffer, contentRect geometry.Rect, term *terminal.Terminal, snap *CopySnapshot, scrollOffset, scrollbackLen int, query string, theme config.Theme) {
	if query == "" {
		return
	}
	q := strings.ToLower(query)
	qLen := utf8.RuneCountInString(q)
	contentH := contentRect.Height
	c := theme.C()
	hlFg := c.ContentBg
	hlBg := c.AccentColor

	for dy := 0; dy < contentH; dy++ {
		absLine := mouseToAbsLine(dy, scrollOffset, scrollbackLen, contentH)
		line := getVisibleLineText(absLine, scrollbackLen, term, snap)
		lower := strings.ToLower(line)
		runes := []rune(lower)
		qRunes := []rune(q)
		if len(runes) < len(qRunes) {
			continue
		}
		// Build rune-index → display-column mapping
		colMap := make([]int, len(runes)+1)
		col := 0
		for idx, r := range runes {
			colMap[idx] = col
			w := runewidth.RuneWidth(r)
			if w < 1 {
				w = 1
			}
			col += w
		}
		colMap[len(runes)] = col
		// Find all matches on this line
		for i := 0; i <= len(runes)-len(qRunes); i++ {
			match := true
			for j := 0; j < len(qRunes); j++ {
				if runes[i+j] != qRunes[j] {
					match = false
					break
				}
			}
			if match {
				for k := 0; k < qLen; k++ {
					bx := contentRect.X + colMap[i+k]
					by := contentRect.Y + dy
					if bx >= 0 && bx < buf.Width && by >= 0 && by < buf.Height {
						cell := &buf.Cells[by][bx]
						cell.Fg = hlFg
						cell.Bg = hlBg
					}
				}
			}
		}
	}
}

// getVisibleLineText returns text for a line by absolute line number.
func getVisibleLineText(absLine, scrollbackLen int, term *terminal.Terminal, snap *CopySnapshot) string {
	if snap != nil {
		sbLen := snap.ScrollbackLen()
		if absLine < sbLen {
			offset := sbLen - 1 - absLine
			return cellsToString(snap.ScrollbackLine(offset))
		}
		screenRow := absLine - sbLen
		if screenRow >= 0 && screenRow < len(snap.Screen) {
			return cellsToString(snap.Screen[screenRow])
		}
		return ""
	}
	sbLen := term.ScrollbackLen()
	if absLine < sbLen {
		offset := sbLen - 1 - absLine
		return cellsToString(term.ScrollbackLine(offset))
	}
	screenRow := absLine - sbLen
	// For live terminal, use CellAt
	w := term.Width()
	h := term.Height()
	if screenRow >= 0 && screenRow < h {
		var sb strings.Builder
		for x := 0; x < w; x++ {
			c := term.CellAt(x, screenRow)
			if c != nil && c.Content != "" {
				sb.WriteString(c.Content)
			} else {
				sb.WriteByte(' ')
			}
		}
		return sb.String()
	}
	return ""
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
	renderScrollbarWithSnapshot(buf, w, theme, term, scrollOffset, nil)
}

func renderScrollbarWithSnapshot(buf *Buffer, w *window.Window, theme config.Theme, term *terminal.Terminal, scrollOffset int, copySnap *CopySnapshot) {
	r := w.Rect
	trackX := r.Right() - 1
	trackTop := r.Y + 1
	trackBot := r.Bottom() - 2 // above bottom border
	renderScrollbarCore(buf, trackX, trackTop, trackBot, w.ContentRect().Height, theme, term, scrollOffset, copySnap)
}

// renderPaneScrollbar draws a scrollbar on the right edge of a pane's content rect.
// Used for copy mode in split panes where there is no window-level right border.
func renderPaneScrollbar(buf *Buffer, contentRect geometry.Rect, theme config.Theme, term *terminal.Terminal, scrollOffset int, copySnap *CopySnapshot) {
	trackX := contentRect.Right() - 1
	trackTop := contentRect.Y
	trackBot := contentRect.Bottom() - 1
	renderScrollbarCore(buf, trackX, trackTop, trackBot, contentRect.Height, theme, term, scrollOffset, copySnap)
}

// renderScrollbarCore draws a scrollbar track with thumb at the given position.
func renderScrollbarCore(buf *Buffer, trackX, trackTop, trackBot, contentH int, theme config.Theme, term *terminal.Terminal, scrollOffset int, copySnap *CopySnapshot) {
	trackH := trackBot - trackTop + 1
	if trackH < 3 {
		return
	}

	sbLen := term.ScrollbackLen()
	termH := term.Height()
	if copySnap != nil {
		sbLen = copySnap.ScrollbackLen()
		termH = copySnap.Height
	}
	totalLines := sbLen + termH
	if totalLines <= contentH {
		return // no scrollbar needed
	}

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

	c := theme.C()
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

// renderCopySearchBar draws the copy-mode search prompt inside the focused window.
func renderCopySearchBar(buf *Buffer, w *window.Window, theme config.Theme, query string, dir int, matchIdx, matchCount int) {
	if w == nil || !w.Visible || w.Minimized {
		return
	}
	renderCopySearchBarInRect(buf, w.ContentRect(), theme, query, dir, matchIdx, matchCount)
}

// renderCopySearchBarInRect draws the copy-mode search prompt inside a content rect.
// Works for both regular windows and split panes.
func renderCopySearchBarInRect(buf *Buffer, cr geometry.Rect, theme config.Theme, query string, dir int, matchIdx, matchCount int) {
	if cr.Width <= 0 || cr.Height <= 0 {
		return
	}

	prefix := "/"
	if dir < 0 {
		prefix = "?"
	}
	// Build match count suffix
	countStr := ""
	if query != "" {
		if matchCount > 0 {
			countStr = fmt.Sprintf(" [%d/%d]", matchIdx+1, matchCount)
		} else {
			countStr = " [0/0]"
		}
	}
	text := " " + prefix + query + "\u2588" + countStr + " "
	textW := utf8.RuneCountInString(text)
	barW := textW + 2
	if barW < 20 {
		barW = 20
	}
	if barW > cr.Width {
		barW = cr.Width
	}
	barX := cr.X + (cr.Width-barW)/2
	barY := cr.Y

	c := theme.C()
	for dx := 0; dx < barW; dx++ {
		bx := barX + dx
		if bx >= 0 && bx < buf.Width && barY >= 0 && barY < buf.Height {
			buf.Cells[barY][bx] = Cell{Char: ' ', Fg: c.AccentFg, Bg: c.AccentColor, Width: 1}
		}
	}

	textX := barX + (barW-textW)/2
	col := 0
	for _, ch := range text {
		bx := textX + col
		if bx >= 0 && bx < buf.Width && barY >= 0 && barY < buf.Height {
			buf.Cells[barY][bx] = Cell{Char: ch, Fg: c.AccentFg, Bg: c.AccentColor, Width: 1}
		}
		col++
	}
}

// renderCopyCursor draws the copy mode cursor as a highlighted cell in the viewport.
func renderCopyCursor(buf *Buffer, contentRect geometry.Rect, cursorX, cursorY, scrollOffset, scrollbackLen, contentH int, theme config.Theme) {
	row := absLineToContentRow(cursorY, scrollOffset, scrollbackLen, contentH)
	if row < 0 || row >= contentH {
		return
	}
	sx := contentRect.X + cursorX
	sy := contentRect.Y + row
	if sx < contentRect.X || sx >= contentRect.Right() || sy < contentRect.Y || sy >= contentRect.Bottom() {
		return
	}
	if sx >= 0 && sx < buf.Width && sy >= 0 && sy < buf.Height {
		c := theme.C()
		cell := &buf.Cells[sy][sx]
		if cell.Width == 0 {
			cell.Char = ' '
			cell.Width = 1
		}
		cell.Fg = c.AccentFg
		cell.Bg = c.AccentColor
	}
}

// renderWindowHints draws contextual keyboard shortcut hints in the bottom
// border of the focused window, adapting to the current input mode.
func renderWindowHints(buf *Buffer, w *window.Window, theme config.Theme, mode InputMode, kb config.KeyBindings, selActive bool, searchQuery string) {
	if w == nil || !w.Visible || w.Minimized {
		return
	}
	r := w.Rect
	if r.Width < 20 {
		return
	}
	if w.IsMaximized() {
		return // no bottom border when maximized
	}

	var hints string
	switch mode {
	case ModeCopy:
		if searchQuery != "" {
			hints = " n/N:next/prev  v:select  y:yank  q:quit "
		} else if selActive {
			hints = " y:yank  o:swap  w/b:word  $:eol  q:quit "
		} else {
			hints = " /:search  v:select  Y:line  hjkl  q:quit "
		}
	case ModeTerminal:
		pfx := strings.ToUpper(kb.Prefix)
		if w.IsSplit() {
			hints = " " + pfx + ":prefix  o:pane  %:split  x:close  arrows:focus "
		} else {
			hints = " " + pfx + ":prefix  " + pfx + " " + kb.Help + ":help  " + kb.QuickNextWindow + "/" + kb.QuickPrevWindow + ":switch "
		}
	default: // ModeNormal
		hints = " " + kb.EnterTerminal + ":terminal  " + kb.NewTerminal + ":new  " + kb.CloseWindow + ":close  " + kb.Launcher + ":launcher  " + kb.NextWindow + ":next  " + kb.Help + ":help "
	}

	hintRunes := []rune(hints)
	hintW := len(hintRunes)
	// Clamp to available border width (leave 1 cell margin on each side)
	maxW := r.Width - 2
	if hintW > maxW {
		hintRunes = hintRunes[:maxW]
		hintW = maxW
	}

	c := theme.C()
	borderFg := c.SubtleFg
	borderBg := c.InactiveBorderBg
	if w.Focused {
		borderBg = c.ActiveBorderBg
	}
	borderY := r.Bottom() - 1
	startX := r.X + 1
	for i, ch := range hintRunes {
		x := startX + i
		if x >= 0 && x < buf.Width && borderY >= 0 && borderY < buf.Height {
			buf.SetCell(x, borderY, ch, borderFg, borderBg, 0)
		}
	}
}

// renderExitedOverlay draws a centered "Press r to restart, q to close" message
// over the content area of an exited window.
func renderExitedOverlay(buf *Buffer, w *window.Window, theme config.Theme) {
	cr := w.ContentRect()
	if cr.Width < 10 || cr.Height < 3 {
		return
	}

	msg := " Press r to restart, q to close "
	msgRunes := []rune(msg)
	msgW := len(msgRunes)
	if msgW > cr.Width-2 {
		msg = " r=restart q=close "
		msgRunes = []rune(msg)
		msgW = len(msgRunes)
	}

	// Center the overlay message in the content area
	overlayX := cr.X + (cr.Width-msgW)/2
	overlayY := cr.Y + cr.Height/2

	if overlayY < 0 || overlayY >= buf.Height {
		return
	}

	// Draw message with high-contrast styling
	overlayBg := color.RGBA{R: 180, G: 30, B: 30, A: 255}
	overlayFg := color.RGBA{R: 220, G: 220, B: 220, A: 255}
	for i, ch := range msgRunes {
		x := overlayX + i
		if x >= 0 && x < buf.Width {
			buf.SetCell(x, overlayY, ch, overlayFg, overlayBg, 0)
		}
	}
}


package app

import (
	"fmt"
	"unicode/utf8"

	"github.com/icex/termdesk/internal/config"
	"github.com/icex/termdesk/internal/terminal"
	"github.com/icex/termdesk/pkg/geometry"
)

// renderQuakeTerminal draws the quake dropdown terminal directly into the buffer.
// The quake terminal has no side borders, no top border, no titlebar — only a
// bottom border line. It renders from (0,0) to (width, animH).
// scrollOffset shifts the content for copy mode scrollback.
func renderQuakeTerminal(buf *Buffer, term *terminal.Terminal, theme config.Theme, animH int, width int, scrollOffset int, copySnap *CopySnapshot) {
	if term == nil || animH <= 0 || width <= 0 {
		return
	}

	c := theme.C()
	contentBg := c.ContentBg
	contentFg := c.ActiveTitleFg
	borderFg := c.AccentColor

	// Content area: rows 0 through animH-2 (animH-1 is the bottom border)
	contentH := animH - 1
	if contentH < 0 {
		contentH = 0
	}

	if contentH > 0 {
		area := geometry.Rect{X: 0, Y: 0, Width: width, Height: contentH}
		renderTerminalContentWithSnapshot(buf, area, term, contentFg, contentBg, scrollOffset, copySnap)
	}

	// Bottom border — simple horizontal line
	borderY := animH - 1
	if borderY >= 0 && borderY < buf.Height {
		for x := 0; x < width && x < buf.Width; x++ {
			buf.SetCell(x, borderY, '─', borderFg, contentBg, 0)
		}
	}

	// Scroll indicator in border when scrolled back
	if scrollOffset > 0 && borderY >= 0 && borderY < buf.Height {
		maxScroll := term.ScrollbackLen()
		if copySnap != nil {
			maxScroll = copySnap.ScrollbackLen()
		}
		indicator := fmt.Sprintf(" [↑ %d/%d] ", scrollOffset, maxScroll)
		indicatorRunes := []rune(indicator)
		startX := (width - len(indicatorRunes)) / 2
		indFg, indBg := c.ActiveBorderFg, c.ActiveBorderBg
		for i, ch := range indicatorRunes {
			x := startX + i
			if x >= 0 && x < buf.Width {
				buf.SetCell(x, borderY, ch, indFg, indBg, 0)
			}
		}
	}
}

// renderQuakeCopySearchBar draws the copy-mode search bar at the top of the quake area.
func renderQuakeCopySearchBar(buf *Buffer, theme config.Theme, width int, query string, dir int, matchIdx, matchCount int) {
	if width <= 0 {
		return
	}
	prefix := "/"
	if dir < 0 {
		prefix = "?"
	}
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
	if barW > width {
		barW = width
	}
	barX := (width - barW) / 2
	barY := 0

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

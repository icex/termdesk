package app

import (
	"fmt"

	"github.com/icex/termdesk/internal/config"
	"github.com/icex/termdesk/internal/terminal"
	"github.com/icex/termdesk/internal/window"
)

// renderSplitWindow draws a split window: full border chrome (like regular windows),
// pane terminals, separator lines between panes, and a pane count badge.
func renderSplitWindow(buf *Buffer, w *window.Window, theme config.Theme,
	terminals map[string]*terminal.Terminal, showCursor bool, scrollOffset int,
	sel SelectionInfo, hoverZone window.HitZone) {

	// Use the standard window chrome (title bar + side/bottom borders)
	renderWindowChrome(buf, w, theme, hoverZone)

	// Overlay pane count badge in the title bar (before the buttons)
	renderPaneBadge(buf, w, theme)

	c := theme.C()
	contentBg := c.ContentBg
	if contentBg == nil {
		contentBg = c.InactiveBorderBg
	}
	defaultFg := c.DefaultFg

	cr := w.SplitContentRect()
	panes := w.SplitRoot.Layout(cr)

	for _, p := range panes {
		term := terminals[p.TermID]
		if term == nil {
			// Fill empty pane with background
			buf.FillRectC(p.Rect, ' ', defaultFg, contentBg)
			continue
		}

		// Determine if this pane owns copy mode
		winCopyMode := sel.CopyMode && sel.CopyWindowID == p.TermID
		paneScroll := 0
		var copySnap *CopySnapshot
		if winCopyMode {
			paneScroll = scrollOffset
			copySnap = sel.CopySnap
		}

		renderTerminalContentWithSnapshot(buf, p.Rect, term, defaultFg, contentBg, paneScroll, copySnap)

		// Show scroll indicator when in copy mode and scrolled back
		if winCopyMode && paneScroll > 0 {
			maxScroll := term.ScrollbackLen()
			if copySnap != nil {
				maxScroll = copySnap.ScrollbackLen()
			}
			indicator := fmt.Sprintf(" [↑ %d/%d] ", paneScroll, maxScroll)
			indicatorRunes := []rune(indicator)
			indicatorW := len(indicatorRunes)
			indY := p.Rect.Bottom() - 1
			indX := p.Rect.X + (p.Rect.Width-indicatorW)/2
			indFg, indBg := c.ActiveBorderFg, c.ActiveBorderBg
			for i, ch := range indicatorRunes {
				x := indX + i
				if x >= p.Rect.X && x < p.Rect.Right() &&
					x >= 0 && x < buf.Width && indY >= 0 && indY < buf.Height {
					buf.SetCell(x, indY, ch, indFg, indBg, 0)
				}
			}
		}

		// Cursor for focused pane only
		isFocused := w.Focused && p.TermID == w.FocusedPane
		if isFocused && showCursor && !term.IsCursorHidden() && paneScroll == 0 {
			cx, cy := term.CursorPosition()
			sx := p.Rect.X + cx
			sy := p.Rect.Y + cy
			if sx >= p.Rect.X && sx < p.Rect.Right() &&
				sy >= p.Rect.Y && sy < p.Rect.Bottom() &&
				sx >= 0 && sx < buf.Width && sy >= 0 && sy < buf.Height {
				cell := &buf.Cells[sy][sx]
				if cell.Width == 0 {
					cell.Char = ' '
					cell.Width = 1
				}
				cell.Fg = c.AccentFg
				cell.Bg = c.AccentColor
			}
		}

		// Dim unfocused panes slightly
		if w.Focused && p.TermID != w.FocusedPane {
			dimAmount := 0.15
			dimBg := c.DesktopBg
			for y := p.Rect.Y; y < p.Rect.Bottom(); y++ {
				for x := p.Rect.X; x < p.Rect.Right(); x++ {
					if x >= 0 && x < buf.Width && y >= 0 && y < buf.Height {
						cell := &buf.Cells[y][x]
						cell.Fg = desaturateColor(cell.Fg, dimAmount)
						dBg := desaturateColor(cell.Bg, dimAmount)
						dBg = darkenColor(dBg, 1.0-dimAmount*0.15)
						cell.Bg = blendColor(dBg, dimBg, dimAmount*0.3)
					}
				}
			}
		}

		// Desaturate + darken unfocused windows entirely
		if !w.Focused && theme.UnfocusedFade > 0 {
			dimBg := c.DesktopBg
			for y := p.Rect.Y; y < p.Rect.Bottom(); y++ {
				for x := p.Rect.X; x < p.Rect.Right(); x++ {
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

	// Draw separators
	seps := w.SplitRoot.Separators(cr)
	sepFg := c.InactiveBorderFg
	sepBg := c.InactiveBorderBg
	if w.Focused {
		sepFg = c.ActiveBorderFg
		sepBg = c.ActiveBorderBg
	}
	for _, sep := range seps {
		if sep.Dir == window.SplitHorizontal {
			// Vertical line (│)
			for y := sep.Rect.Y; y < sep.Rect.Bottom(); y++ {
				if sep.Rect.X >= 0 && sep.Rect.X < buf.Width && y >= 0 && y < buf.Height {
					buf.SetCell(sep.Rect.X, y, theme.BorderVertical, sepFg, sepBg, 0)
				}
			}
		} else {
			// Horizontal line (─)
			for x := sep.Rect.X; x < sep.Rect.Right(); x++ {
				if x >= 0 && x < buf.Width && sep.Rect.Y >= 0 && sep.Rect.Y < buf.Height {
					buf.SetCell(x, sep.Rect.Y, theme.BorderHorizontal, sepFg, sepBg, 0)
				}
			}
		}
	}
}

// renderPaneBadge overlays a " [N] │" pane count badge in the title bar,
// placed just before the window buttons.
func renderPaneBadge(buf *Buffer, w *window.Window, theme config.Theme) {
	if !w.Visible || w.Minimized || w.SplitRoot == nil {
		return
	}

	r := w.Rect
	c := theme.C()
	tbh := w.TitleBarHeight
	if tbh < 1 {
		tbh = 1
	}
	titleRow := r.Y + tbh/2

	titleBg := c.InactiveTitleBg
	if w.Focused {
		titleBg = c.ActiveTitleBg
	}

	closeW := runeLen(theme.CloseButton)
	minW := runeLen(theme.MinButton)
	maxW := runeLen(theme.MaxButton)
	snapLW := runeLen(theme.SnapLeftButton)
	snapRW := runeLen(theme.SnapRightButton)

	borderInset := 1
	if w.IsMaximized() {
		borderInset = 0
	}

	// Find the leftmost button position
	leftmostBtnX := r.Right() - borderInset - closeW - minW
	if w.Resizable {
		leftmostBtnX -= maxW + snapLW + snapRW
	}

	paneCount := w.SplitRoot.PaneCount()
	paneBadge := fmt.Sprintf(" [%d] %s", paneCount, string(theme.BorderVertical))
	paneBadgeW := len([]rune(paneBadge))
	badgeX := leftmostBtnX - paneBadgeW

	buf.SetStringC(badgeX, titleRow, paneBadge, c.SubtleFg, titleBg)
}

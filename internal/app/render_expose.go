package app

import (
	"fmt"
	"sort"
	"strings"
	"unicode/utf8"

	"github.com/icex/termdesk/internal/config"
	"github.com/icex/termdesk/internal/window"
	"github.com/icex/termdesk/pkg/geometry"
)


// renderExposeSearchBar draws the search/filter bar near the top of the exposé view.
func renderExposeSearchBar(buf *Buffer, theme config.Theme, filter string, wa geometry.Rect) {
	c := theme.C()
	text := " \u2315 " + filter + "\u2588 " // ⌕ filter█
	textW := utf8.RuneCountInString(text)
	barW := textW + 2 // 1 padding each side
	if barW < 20 {
		barW = 20
	}
	barX := wa.X + (wa.Width-barW)/2
	barY := wa.Y + 1

	// Draw bar background
	for dx := 0; dx < barW; dx++ {
		bx := barX + dx
		if bx >= 0 && bx < buf.Width && barY >= 0 && barY < buf.Height {
			buf.Cells[barY][bx] = Cell{Char: ' ', Fg: c.AccentFg, Bg: c.AccentColor, Width: 1}
		}
	}
	// Draw text centered within bar
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
// filter is the current search string (empty = show all).
func RenderExpose(wm *window.Manager, theme config.Theme, filter string) *Buffer {
	wa := wm.WorkArea()
	fullWidth := wa.Width
	fullHeight := wa.Height + wa.Y + wm.ReservedBottom()
	if fullWidth <= 0 || fullHeight <= 0 {
		return AcquireThemedBuffer(1, 1, theme)
	}

	buf := AcquireThemedBuffer(fullWidth, fullHeight, theme)

	windows := wm.Windows()
	var allVisible []*window.Window
	var focusedWin *window.Window
	for _, w := range windows {
		if w.Visible && !w.Minimized {
			allVisible = append(allVisible, w)
			if w.Focused {
				focusedWin = w
			}
		}
	}
	if len(allVisible) == 0 {
		msg := "No open windows"
		x := (fullWidth - runeLen(msg)) / 2
		y := fullHeight / 2
		buf.SetString(x, y, msg, theme.ActiveTitleFg, theme.DesktopBg)
		return buf
	}
	sort.Slice(allVisible, func(i, j int) bool { return allVisible[i].ID < allVisible[j].ID })

	// Filter by title if search is active
	visible := allVisible
	if filter != "" {
		q := strings.ToLower(filter)
		var filtered []*window.Window
		for _, w := range allVisible {
			if strings.Contains(strings.ToLower(w.Title), q) {
				filtered = append(filtered, w)
			}
		}
		if len(filtered) > 0 {
			visible = filtered
		}
	}
	n := len(visible)

	// Build window number map (1-based display number for each visible window)
	winNumMap := make(map[string]int, n)
	for i, w := range visible {
		winNumMap[w.ID] = i + 1 // 1-based
	}

	// Calculate background thumbnail positions (grid along the bottom/sides)
	bgCount := n - 1
	if focusedWin == nil {
		focusedWin = visible[0]
		bgCount = n - 1
	}
	// If focused is not in filtered set, use first
	inSet := false
	for _, w := range visible {
		if w.ID == focusedWin.ID {
			inSet = true
			break
		}
	}
	if !inSet {
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

	// -- Draw focused window centered at 50% of its actual size --
	// Use pre-maximize rect if available so maximized windows keep their real size
	focRect := focusedWin.Rect
	if focusedWin.PreMaxRect != nil {
		focRect = *focusedWin.PreMaxRect
	}
	// Scale to 50% for a compact expose view
	focW := focRect.Width / 2
	focH := focRect.Height / 2
	maxW := wa.Width - 4
	maxH := wa.Height - 8 // leave room for bottom strip
	if focW > maxW {
		focH = focH * maxW / focW
		focW = maxW
	}
	if focH > maxH {
		focW = focW * maxH / focH
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

	// -- Draw search bar when filter is active --
	if filter != "" {
		renderExposeSearchBar(buf, theme, filter, wa)
	}

	return buf
}

// RenderExposeTransition draws windows at their animated positions during expose enter/exit.
func RenderExposeTransition(wm *window.Manager, theme config.Theme, animations []Animation, filter string) *Buffer {
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

	// Draw search bar when filter is active
	if filter != "" {
		renderExposeSearchBar(buf, theme, filter, wa)
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

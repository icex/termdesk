package app

import (
	"sort"
	"strings"

	"github.com/icex/termdesk/internal/window"
	"github.com/icex/termdesk/pkg/geometry"
)

// sortVisibleByID sorts a window slice by ID for consistent expose strip ordering.
func sortVisibleByID(visible []*window.Window) {
	sort.Slice(visible, func(i, j int) bool { return visible[i].ID < visible[j].ID })
}

// exposeFilteredWindows returns visible, non-minimized windows filtered by
// the current exposeFilter (case-insensitive title substring match).
// Returns all visible windows if filter is empty or matches nothing.
func (m *Model) exposeFilteredWindows() []*window.Window {
	var visible []*window.Window
	for _, w := range m.wm.Windows() {
		if w.Visible && !w.Minimized {
			visible = append(visible, w)
		}
	}
	sortVisibleByID(visible)
	if m.exposeFilter == "" {
		return visible
	}
	q := strings.ToLower(m.exposeFilter)
	var filtered []*window.Window
	for _, w := range visible {
		if strings.Contains(strings.ToLower(w.Title), q) {
			filtered = append(filtered, w)
		}
	}
	if len(filtered) == 0 {
		return visible // don't empty the screen
	}
	return filtered
}

// relayoutExpose recomputes expose positions based on current filter.
// If the focused window is not in the filtered set, focuses the first match.
func (m *Model) relayoutExpose() {
	filtered := m.exposeFilteredWindows()
	if len(filtered) == 0 {
		return
	}
	wa := m.wm.WorkArea()

	// Check if currently focused window is in filtered set
	focusedInSet := false
	var focusedWin *window.Window
	for _, w := range filtered {
		if w.Focused {
			focusedInSet = true
			focusedWin = w
			break
		}
	}
	if !focusedInSet {
		m.wm.FocusWindow(filtered[0].ID)
		focusedWin = filtered[0]
	}

	// Animate focused to center
	focTarget := exposeTargetRect(focusedWin, wa, true)
	m.startExposeAnimation(focusedWin.ID, AnimExpose, focTarget, focTarget)

	// Animate others to strip
	bgTargets := exposeBgTargets(filtered, focusedWin.ID, wa)
	for id, target := range bgTargets {
		m.startExposeAnimation(id, AnimExpose, target, target)
	}
}

// enterExpose transitions into expose mode with animations.
func (m *Model) enterExpose() {
	m.exposeMode = true
	m.exposeFilter = ""
	wa := m.wm.WorkArea()
	var visible []*window.Window
	var focusedWin *window.Window
	for _, w := range m.wm.Windows() {
		if w.Visible && !w.Minimized {
			visible = append(visible, w)
			if w.Focused {
				focusedWin = w
			}
		}
	}
	if len(visible) == 0 {
		return
	}
	sortVisibleByID(visible)
	if focusedWin == nil {
		focusedWin = visible[0]
	}

	// Animate focused window to center
	focTarget := exposeTargetRect(focusedWin, wa, true)
	m.startExposeAnimation(focusedWin.ID, AnimExpose, focusedWin.Rect, focTarget)

	// Animate unfocused windows to bottom strip
	bgTargets := exposeBgTargets(visible, focusedWin.ID, wa)
	for id, target := range bgTargets {
		if w := m.wm.WindowByID(id); w != nil {
			m.startExposeAnimation(id, AnimExpose, w.Rect, target)
		}
	}
}

// exitExpose transitions out of expose mode with animations.
func (m *Model) exitExpose() {
	m.exposeMode = false
	m.exposeFilter = ""
	wa := m.wm.WorkArea()
	var visible []*window.Window
	var focusedWin *window.Window
	for _, w := range m.wm.Windows() {
		if w.Visible && !w.Minimized {
			visible = append(visible, w)
			if w.Focused {
				focusedWin = w
			}
		}
	}
	if len(visible) == 0 {
		return
	}
	sortVisibleByID(visible)
	if focusedWin == nil {
		focusedWin = visible[0]
	}

	// Animate focused window from center back to its real position
	focFrom := exposeTargetRect(focusedWin, wa, true)
	m.startExposeAnimation(focusedWin.ID, AnimExposeExit, focFrom, focusedWin.Rect)

	// Animate unfocused windows from bottom strip back to their positions
	bgTargets := exposeBgTargets(visible, focusedWin.ID, wa)
	for id, from := range bgTargets {
		if w := m.wm.WindowByID(id); w != nil {
			m.startExposeAnimation(id, AnimExposeExit, from, w.Rect)
		}
	}
}

// cycleExposeWindow animates switching focus in expose mode.
// direction: +1 = forward (right), -1 = backward (left).
// Uses ID-sorted visible list for consistent strip ordering so that
// forward cycling visually moves windows in one direction and backward
// cycling moves them in the opposite direction.
func (m *Model) cycleExposeWindow(direction int) {
	wa := m.wm.WorkArea()
	visible := m.exposeFilteredWindows()
	if len(visible) < 2 {
		return
	}

	// Find old focused window in sorted list
	oldFocusedIdx := -1
	var oldFocused *window.Window
	for i, w := range visible {
		if w.Focused {
			oldFocusedIdx = i
			oldFocused = w
			break
		}
	}
	if oldFocused == nil {
		return
	}

	// Cycle through sorted list (not z-order)
	newIdx := (oldFocusedIdx + direction + len(visible)) % len(visible)
	newFocused := visible[newIdx]
	m.wm.FocusWindow(newFocused.ID)

	// Compute old focused window's center rect (where it currently is)
	oldCenterRect := exposeTargetRect(oldFocused, wa, true)

	// Where old focused goes in new strip (with newFocused as center)
	bgTargets := exposeBgTargets(visible, newFocused.ID, wa)
	newBgRect := bgTargets[oldFocused.ID]

	// Where new focused was in old strip (with oldFocused as center)
	oldBgTargets := exposeBgTargets(visible, oldFocused.ID, wa)
	newOldBgRect := oldBgTargets[newFocused.ID]

	// Animate: old focused → shrink to its new bg position
	m.startExposeAnimation(oldFocused.ID, AnimExpose, oldCenterRect, newBgRect)

	// Animate: new focused → grow from its old bg position to center
	newCenterRect := exposeTargetRect(newFocused, wa, true)
	m.startExposeAnimation(newFocused.ID, AnimExpose, newOldBgRect, newCenterRect)

	// Animate all other windows repositioning in the bg strip
	for _, w := range visible {
		if w.ID == oldFocused.ID || w.ID == newFocused.ID {
			continue
		}
		oldPos := oldBgTargets[w.ID]
		newPos := bgTargets[w.ID]
		if oldPos != newPos {
			m.startExposeAnimation(w.ID, AnimExpose, oldPos, newPos)
		}
	}
}

// exposeTargetRect calculates the expose display rect for a window.
// If focused=true, returns a large centered rect; otherwise a small thumbnail.
func exposeTargetRect(w *window.Window, wa geometry.Rect, focused bool) geometry.Rect {
	if focused {
		// Use pre-maximize rect if available so maximized windows keep their real size
		r := w.Rect
		if w.PreMaxRect != nil {
			r = *w.PreMaxRect
		}
		// Scale to 50% of actual size for a compact expose view
		focW := r.Width / 2
		focH := r.Height / 2
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
		return geometry.Rect{
			X:      wa.X + (wa.Width-focW)/2,
			Y:      wa.Y + (wa.Height-focH)/3,
			Width:  focW,
			Height: focH,
		}
	}
	// Small thumbnail — used for bg targets calculation
	return geometry.Rect{X: wa.X, Y: wa.Y, Width: 16, Height: 6}
}

// exposeBgTargets returns a map of window ID → target rect for unfocused windows
// arranged along the bottom of the work area.
func exposeBgTargets(visible []*window.Window, focusedID string, wa geometry.Rect) map[string]geometry.Rect {
	targets := make(map[string]geometry.Rect)
	bgCount := 0
	for _, w := range visible {
		if w.ID != focusedID {
			bgCount++
		}
	}
	if bgCount == 0 {
		return targets
	}

	thumbW := 16
	thumbH := 6
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
		if w.ID == focusedID {
			continue
		}
		x := startX + idx*(thumbW+1)
		targets[w.ID] = geometry.Rect{X: x, Y: thumbY, Width: thumbW, Height: thumbH}
		idx++
	}
	return targets
}

// selectExposeWindow finds which mini-window was clicked in expose mode and focuses it.
// Layout matches RenderExpose: focused window centered, others in bottom strip.
func (m *Model) selectExposeWindow(mouseX, mouseY int) {
	wa := m.wm.WorkArea()
	visible := m.exposeFilteredWindows()
	if len(visible) == 0 {
		return
	}
	var focusedWin *window.Window
	for _, w := range visible {
		if w.Focused {
			focusedWin = w
			break
		}
	}
	if focusedWin == nil {
		focusedWin = visible[0]
	}

	// Check focused center window first (must match exposeTargetRect logic)
	focTarget := exposeTargetRect(focusedWin, wa, true)
	focW := focTarget.Width
	focH := focTarget.Height
	focX := focTarget.X
	focY := focTarget.Y
	if mouseX >= focX && mouseX < focX+focW && mouseY >= focY && mouseY < focY+focH {
		// Already focused — just exit expose
		return
	}

	// Check background thumbnails (bottom strip)
	bgCount := len(visible) - 1
	if bgCount <= 0 {
		return
	}
	thumbW := 16
	thumbH := 6
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
		if mouseX >= x && mouseX < x+thumbW && mouseY >= thumbY && mouseY < thumbY+thumbH {
			m.wm.FocusWindow(w.ID)
			return
		}
		idx++
	}
}

// selectExposeByIndex selects the Nth visible window (0-based) in expose mode.
func (m *Model) selectExposeByIndex(idx int) {
	visible := m.exposeFilteredWindows()
	if idx >= 0 && idx < len(visible) {
		m.wm.FocusWindow(visible[idx].ID)
	}
}

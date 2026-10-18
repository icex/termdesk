package window

import (
	"sort"

	"github.com/icex/termdesk/pkg/geometry"
)

// MoveStep is the number of cells to nudge a window per keypress.
const MoveStep = 4

// ResizeStepW is the number of cells to grow/shrink window width per keypress.
const ResizeStepW = 4

// ResizeStepH is the number of cells to grow/shrink window height per keypress.
const ResizeStepH = 2

// SnapLeft positions the window to fill the left half of the work area.
func SnapLeft(w *Window, workArea geometry.Rect) { snapTo(w, SnapLeftHalf, workArea) }

// SnapRight positions the window to fill the right half of the work area.
func SnapRight(w *Window, workArea geometry.Rect) { snapTo(w, SnapRightHalf, workArea) }

// SnapTop positions the window to fill the top half of the work area.
func SnapTop(w *Window, workArea geometry.Rect) { snapTo(w, SnapTopHalf, workArea) }

// SnapBottom positions the window to fill the bottom half of the work area.
func SnapBottom(w *Window, workArea geometry.Rect) { snapTo(w, SnapBottomHalf, workArea) }

func snapTo(w *Window, edge SnapEdge, workArea geometry.Rect) {
	unpin(w)
	w.Snap = edge
	w.Rect = SnapRect(edge, workArea)
}

// SnapRect returns the rect a window snapped to edge occupies in workArea.
// SnapNone returns workArea unchanged.
func SnapRect(edge SnapEdge, workArea geometry.Rect) geometry.Rect {
	halfW := workArea.Width / 2
	halfH := workArea.Height / 2
	r := workArea
	switch edge {
	case SnapLeftHalf:
		r.Width = halfW
	case SnapRightHalf:
		r.X += halfW
		r.Width -= halfW
	case SnapTopHalf:
		r.Height = halfH
	case SnapBottomHalf:
		r.Y += halfH
		r.Height -= halfH
	}
	return r
}

// unpin drops maximize and snap state before the window is placed freely.
func unpin(w *Window) {
	w.PreMaxRect = nil
	w.AutoMaximized = false
	w.Snap = SnapNone
}

// Maximize expands the window to fill the entire work area.
// Saves the current rect for later restore.
func Maximize(w *Window, workArea geometry.Rect) {
	if w.IsMaximized() {
		return // already maximized
	}
	prev := w.Rect
	w.PreMaxRect = &prev
	w.Rect = workArea
}

// Restore returns the window to its pre-maximize size, or back to its snap
// position (recomputed for workArea) if it was snapped before maximizing.
func Restore(w *Window, workArea geometry.Rect) {
	if w.PreMaxRect != nil {
		w.Rect = *w.PreMaxRect
		w.PreMaxRect = nil
	}
	w.AutoMaximized = false
	if w.Snap != SnapNone {
		w.Rect = SnapRect(w.Snap, workArea)
	}
}

// ToggleMaximize toggles between maximized and restored state. A window that
// is only auto-maximized (screen too small) becomes explicitly maximized
// instead, so it stays full-size when the screen grows again.
func ToggleMaximize(w *Window, workArea geometry.Rect) {
	switch {
	case w.AutoMaximized:
		w.AutoMaximized = false
	case w.IsMaximized():
		Restore(w, workArea)
	default:
		Maximize(w, workArea)
	}
}

// Fit places the window for workArea according to its layout state:
// maximized windows fill it, snapped windows take their half, and free
// windows are shrunk/moved to stay inside it.
func Fit(w *Window, workArea geometry.Rect) {
	switch {
	case w.IsMaximized():
		w.Rect = workArea
	case w.Snap != SnapNone:
		w.Rect = SnapRect(w.Snap, workArea)
	default:
		r := &w.Rect
		r.Width = min(r.Width, workArea.Width)
		r.Height = min(r.Height, workArea.Height)
		r.X = max(min(r.X, workArea.X+workArea.Width-r.Width), workArea.X)
		r.Y = max(min(r.Y, workArea.Y+workArea.Height-r.Height), workArea.Y)
	}
}

// TileAll arranges all visible windows in a grid layout.
func TileAll(windows []*Window, workArea geometry.Rect) {
	var visible []*Window
	for _, w := range windows {
		if w.Visible && !w.Minimized && w.Resizable {
			visible = append(visible, w)
		}
	}
	n := len(visible)
	if n == 0 {
		return
	}
	// Sort by ID for deterministic positioning across detach/attach cycles.
	sort.Slice(visible, func(i, j int) bool { return visible[i].ID < visible[j].ID })

	// Calculate grid dimensions
	cols := 1
	rows := 1
	for cols*rows < n {
		if cols <= rows {
			cols++
		} else {
			rows++
		}
	}

	cellW := workArea.Width / cols
	cellH := workArea.Height / rows

	for i, w := range visible {
		col := i % cols
		row := i / cols
		unpin(w)
		w.Rect = geometry.Rect{
			X:      workArea.X + col*cellW,
			Y:      workArea.Y + row*cellH,
			Width:  cellW,
			Height: cellH,
		}
		// Last column gets remaining width
		if col == cols-1 {
			w.Rect.Width = workArea.Width - col*cellW
		}
		// Last row gets remaining height
		if row == rows-1 {
			w.Rect.Height = workArea.Height - row*cellH
		}
	}
}

// MoveWindow nudges the window by (dx, dy) and clamps within the work area.
func MoveWindow(w *Window, dx, dy int, workArea geometry.Rect) {
	unpin(w)
	w.Rect = w.Rect.Move(dx, dy).Clamp(workArea)
}

// ResizeWindow adjusts the window size by (dw, dh), enforcing minimums and work area bounds.
func ResizeWindow(w *Window, dw, dh int, workArea geometry.Rect) {
	unpin(w)
	newW := w.Rect.Width + dw
	newH := w.Rect.Height + dh
	if newW < MinWindowWidth {
		newW = MinWindowWidth
	}
	if newH < MinWindowHeight {
		newH = MinWindowHeight
	}
	if newW > workArea.Width {
		newW = workArea.Width
	}
	if newH > workArea.Height {
		newH = workArea.Height
	}
	w.Rect.Width = newW
	w.Rect.Height = newH
	w.Rect = w.Rect.Clamp(workArea)
}

// CenterWindow positions the window at ~60%x70% of the work area, centered.
func CenterWindow(w *Window, workArea geometry.Rect) {
	unpin(w)
	cw := workArea.Width * 60 / 100
	ch := workArea.Height * 70 / 100
	if cw < MinWindowWidth {
		cw = MinWindowWidth
	}
	if ch < MinWindowHeight {
		ch = MinWindowHeight
	}
	if cw > workArea.Width {
		cw = workArea.Width
	}
	if ch > workArea.Height {
		ch = workArea.Height
	}
	w.Rect = geometry.Rect{
		X:      workArea.X + (workArea.Width-cw)/2,
		Y:      workArea.Y + (workArea.Height-ch)/2,
		Width:  cw,
		Height: ch,
	}
}

// TileColumns arranges all visible, non-minimized, resizable windows in side-by-side columns.
func TileColumns(windows []*Window, workArea geometry.Rect) {
	var visible []*Window
	for _, w := range windows {
		if w.Visible && !w.Minimized && w.Resizable {
			visible = append(visible, w)
		}
	}
	n := len(visible)
	if n == 0 {
		return
	}
	// Sort by ID for deterministic positioning across detach/attach cycles.
	sort.Slice(visible, func(i, j int) bool { return visible[i].ID < visible[j].ID })
	colW := workArea.Width / n
	for i, w := range visible {
		unpin(w)
		w.Rect = geometry.Rect{
			X:      workArea.X + i*colW,
			Y:      workArea.Y,
			Width:  colW,
			Height: workArea.Height,
		}
		if i == n-1 {
			w.Rect.Width = workArea.Width - i*colW
		}
	}
}

// TileRows arranges all visible, non-minimized, resizable windows stacked in horizontal rows.
func TileRows(windows []*Window, workArea geometry.Rect) {
	var visible []*Window
	for _, w := range windows {
		if w.Visible && !w.Minimized && w.Resizable {
			visible = append(visible, w)
		}
	}
	n := len(visible)
	if n == 0 {
		return
	}
	// Sort by ID for deterministic positioning across detach/attach cycles.
	sort.Slice(visible, func(i, j int) bool { return visible[i].ID < visible[j].ID })
	rowH := workArea.Height / n
	for i, w := range visible {
		unpin(w)
		w.Rect = geometry.Rect{
			X:      workArea.X,
			Y:      workArea.Y + i*rowH,
			Width:  workArea.Width,
			Height: rowH,
		}
		if i == n-1 {
			w.Rect.Height = workArea.Height - i*rowH
		}
	}
}

// MaximizeAll maximizes all visible, non-minimized, resizable windows to fill the work area.
// Fixed-size (non-resizable) windows are left untouched.
func MaximizeAll(windows []*Window, workArea geometry.Rect) {
	for _, w := range windows {
		if w.Visible && !w.Minimized && w.Resizable {
			if !w.IsMaximized() {
				prev := w.Rect
				w.PreMaxRect = &prev
			}
			w.AutoMaximized = false
			w.Rect = workArea
		}
	}
}

// Cascade arranges windows in an overlapping diagonal layout.
func Cascade(windows []*Window, workArea geometry.Rect) {
	var visible []*Window
	for _, w := range windows {
		if w.Visible && !w.Minimized && w.Resizable {
			visible = append(visible, w)
		}
	}
	n := len(visible)
	if n == 0 {
		return
	}
	// Sort by ID for deterministic positioning across detach/attach cycles.
	sort.Slice(visible, func(i, j int) bool { return visible[i].ID < visible[j].ID })

	cw := workArea.Width * 60 / 100
	ch := workArea.Height * 70 / 100
	if cw < MinWindowWidth {
		cw = MinWindowWidth
	}
	if ch < MinWindowHeight {
		ch = MinWindowHeight
	}

	const offsetX = 3
	const offsetY = 2

	maxX := workArea.X + workArea.Width - cw
	maxY := workArea.Y + workArea.Height - ch

	for i, w := range visible {
		unpin(w)
		x := workArea.X + i*offsetX
		y := workArea.Y + i*offsetY
		if maxX > workArea.X {
			x = workArea.X + (i*offsetX)%(maxX-workArea.X+1)
		} else {
			x = workArea.X
		}
		if maxY > workArea.Y {
			y = workArea.Y + (i*offsetY)%(maxY-workArea.Y+1)
		} else {
			y = workArea.Y
		}
		w.Rect = geometry.Rect{
			X:      x,
			Y:      y,
			Width:  cw,
			Height: ch,
		}
	}
}

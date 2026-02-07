package window

import "github.com/icex/termdesk/pkg/geometry"

// SnapLeft positions the window to fill the left half of the work area.
func SnapLeft(w *Window, workArea geometry.Rect) {
	w.PreMaxRect = nil
	w.Rect = geometry.Rect{
		X:      workArea.X,
		Y:      workArea.Y,
		Width:  workArea.Width / 2,
		Height: workArea.Height,
	}
}

// SnapRight positions the window to fill the right half of the work area.
func SnapRight(w *Window, workArea geometry.Rect) {
	w.PreMaxRect = nil
	halfW := workArea.Width / 2
	w.Rect = geometry.Rect{
		X:      workArea.X + halfW,
		Y:      workArea.Y,
		Width:  workArea.Width - halfW,
		Height: workArea.Height,
	}
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

// Restore returns the window to its pre-maximize size.
func Restore(w *Window) {
	if w.PreMaxRect != nil {
		w.Rect = *w.PreMaxRect
		w.PreMaxRect = nil
	}
}

// ToggleMaximize toggles between maximized and restored state.
func ToggleMaximize(w *Window, workArea geometry.Rect) {
	if w.IsMaximized() {
		Restore(w)
	} else {
		Maximize(w, workArea)
	}
}

// TileAll arranges all visible windows in a grid layout.
func TileAll(windows []*Window, workArea geometry.Rect) {
	var visible []*Window
	for _, w := range windows {
		if w.Visible && !w.Minimized {
			visible = append(visible, w)
		}
	}
	n := len(visible)
	if n == 0 {
		return
	}

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
		w.PreMaxRect = nil
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

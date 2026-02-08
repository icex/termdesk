package window

import (
	"unicode/utf8"

	"github.com/icex/termdesk/pkg/geometry"
)

// DragMode describes what kind of drag operation is happening.
type DragMode int

const (
	DragNone DragMode = iota
	DragMove
	DragResizeN
	DragResizeS
	DragResizeE
	DragResizeW
	DragResizeNE
	DragResizeNW
	DragResizeSE
	DragResizeSW
)

// MinWindowWidth is the minimum allowed window width.
const MinWindowWidth = 12

// MinWindowHeight is the minimum allowed window height.
const MinWindowHeight = 4

// DragState tracks an active drag operation.
type DragState struct {
	Active     bool
	WindowID   string
	Mode       DragMode
	StartMouse geometry.Point
	StartRect  geometry.Rect
}

// HitZone describes what part of a window was clicked.
type HitZone int

const (
	HitNone HitZone = iota
	HitTitleBar
	HitCloseButton
	HitMinButton
	HitMaxButton
	HitSnapRightButton
	HitSnapLeftButton
	HitContent
	HitBorderN
	HitBorderS
	HitBorderE
	HitBorderW
	HitBorderNE
	HitBorderNW
	HitBorderSE
	HitBorderSW
)

// HitTest determines which part of the window was clicked.
// closeButtonWidth and maxButtonWidth are the rune widths of the button strings.
func HitTest(w *Window, p geometry.Point, closeButtonWidth, maxButtonWidth int) HitZone {
	r := w.Rect
	if !r.Contains(p) {
		return HitNone
	}

	localX := p.X - r.X
	localY := p.Y - r.Y
	lastCol := r.Width - 1
	lastRow := r.Height - 1
	tbh := w.titleBarRows()

	// Title bar area (may be multiple rows)
	if localY < tbh {
		// Buttons are on the center row of the title bar
		btnRow := tbh / 2
		if localY == btnRow || tbh == 1 {
			// Buttons right-to-left: [snapL][snapR][max][_][×]
			closeStart := lastCol - closeButtonWidth
			if localX > closeStart && localX <= lastCol {
				return HitCloseButton
			}
			minStart := closeStart - 3 // [_] is 3 chars
			if localX > minStart && localX <= closeStart {
				return HitMinButton
			}
			if w.Resizable {
				maxStart := minStart - maxButtonWidth
				if localX > maxStart && localX <= minStart {
					return HitMaxButton
				}
				snapRStart := maxStart - 3
				if localX > snapRStart && localX <= maxStart {
					return HitSnapRightButton
				}
				snapLStart := snapRStart - 3
				if localX > snapLStart && localX <= snapRStart {
					return HitSnapLeftButton
				}
			}
		}
		// Top-left / top-right corners on first row = resize zones
		if localY == 0 {
			if localX == 0 {
				return HitBorderNW
			}
			if localX == lastCol {
				return HitBorderNE
			}
		}
		return HitTitleBar
	}

	// Bottom border
	if localY == lastRow {
		if localX == 0 {
			return HitBorderSW
		}
		if localX == lastCol {
			return HitBorderSE
		}
		return HitBorderS
	}

	// Left border
	if localX == 0 {
		return HitBorderW
	}

	// Right border
	if localX == lastCol {
		return HitBorderE
	}

	return HitContent
}

// HitTestWithTheme is a convenience that computes button widths from theme strings.
func HitTestWithTheme(w *Window, p geometry.Point, closeButton, maxButton string) HitZone {
	return HitTest(w, p, utf8.RuneCountInString(closeButton), utf8.RuneCountInString(maxButton))
}

// DragModeForZone returns the appropriate drag mode for a hit zone.
func DragModeForZone(zone HitZone) DragMode {
	switch zone {
	case HitTitleBar:
		return DragMove
	case HitBorderN:
		return DragResizeN
	case HitBorderS:
		return DragResizeS
	case HitBorderE:
		return DragResizeE
	case HitBorderW:
		return DragResizeW
	case HitBorderNE:
		return DragResizeNE
	case HitBorderNW:
		return DragResizeNW
	case HitBorderSE:
		return DragResizeSE
	case HitBorderSW:
		return DragResizeSW
	default:
		return DragNone
	}
}

// ApplyDrag calculates the new window rect given a drag operation.
func ApplyDrag(state DragState, currentMouse geometry.Point, bounds geometry.Rect) geometry.Rect {
	dx := currentMouse.X - state.StartMouse.X
	dy := currentMouse.Y - state.StartMouse.Y
	r := state.StartRect

	switch state.Mode {
	case DragMove:
		r = r.Move(dx, dy)

	case DragResizeE:
		newW := r.Width + dx
		if newW < MinWindowWidth {
			newW = MinWindowWidth
		}
		r.Width = newW

	case DragResizeS:
		newH := r.Height + dy
		if newH < MinWindowHeight {
			newH = MinWindowHeight
		}
		r.Height = newH

	case DragResizeSE:
		newW := r.Width + dx
		newH := r.Height + dy
		if newW < MinWindowWidth {
			newW = MinWindowWidth
		}
		if newH < MinWindowHeight {
			newH = MinWindowHeight
		}
		r.Width = newW
		r.Height = newH

	case DragResizeW:
		newX := r.X + dx
		newW := r.Width - dx
		if newW < MinWindowWidth {
			newW = MinWindowWidth
			newX = r.X + r.Width - MinWindowWidth
		}
		r.X = newX
		r.Width = newW

	case DragResizeN:
		newY := r.Y + dy
		newH := r.Height - dy
		if newH < MinWindowHeight {
			newH = MinWindowHeight
			newY = r.Y + r.Height - MinWindowHeight
		}
		r.Y = newY
		r.Height = newH

	case DragResizeNW:
		newX := r.X + dx
		newW := r.Width - dx
		newY := r.Y + dy
		newH := r.Height - dy
		if newW < MinWindowWidth {
			newW = MinWindowWidth
			newX = r.X + r.Width - MinWindowWidth
		}
		if newH < MinWindowHeight {
			newH = MinWindowHeight
			newY = r.Y + r.Height - MinWindowHeight
		}
		r.X = newX
		r.Width = newW
		r.Y = newY
		r.Height = newH

	case DragResizeNE:
		newW := r.Width + dx
		newY := r.Y + dy
		newH := r.Height - dy
		if newW < MinWindowWidth {
			newW = MinWindowWidth
		}
		if newH < MinWindowHeight {
			newH = MinWindowHeight
			newY = r.Y + r.Height - MinWindowHeight
		}
		r.Width = newW
		r.Y = newY
		r.Height = newH

	case DragResizeSW:
		newX := r.X + dx
		newW := r.Width - dx
		newH := r.Height + dy
		if newW < MinWindowWidth {
			newW = MinWindowWidth
			newX = r.X + r.Width - MinWindowWidth
		}
		if newH < MinWindowHeight {
			newH = MinWindowHeight
		}
		r.X = newX
		r.Width = newW
		r.Height = newH
	}

	return r.Clamp(bounds)
}

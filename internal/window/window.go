package window

import (
	"github.com/icex/termdesk/pkg/geometry"

	tea "charm.land/bubbletea/v2"
)

// Window represents a single window in the desktop environment.
type Window struct {
	ID        string
	Title     string
	Rect      geometry.Rect
	ZIndex    int
	Focused   bool
	Visible   bool
	Minimized bool
	Resizable bool
	Draggable bool
	Modal     bool
	Content tea.Model

	// PreMaxRect stores the rect before maximization for restore.
	PreMaxRect      *geometry.Rect
	HasNotification bool     // true when window has unread output/bell
	Exited          bool     // true when the PTY process has exited (hold-open mode)
	Command         string   // command that launched this window
	Args            []string // command arguments
	WorkDir         string   // working directory for this terminal
	TitleBarHeight  int      // rows for the title bar (default 1)
	TitleLocked     bool     // true when user has renamed this window (VT title won't override)
	Icon            string   // Nerd Font icon for title bar (e.g. "\uf120")
	IconColor       string   // hex color for the icon (e.g. "#61AFEF")
	HasBell         bool     // true when unfocused/minimized window received a BEL
	HasActivity     bool     // true when unfocused window received PTY output
	Stuck           bool     // true when terminal hasn't produced output within timeout

	// Split pane state
	SplitRoot   *SplitNode // nil = single terminal, non-nil = split tree
	FocusedPane string     // terminal ID of focused pane (empty when unsplit)
}

// NewWindow creates a window with sensible defaults.
func NewWindow(id, title string, rect geometry.Rect, content tea.Model) *Window {
	return &Window{
		ID:        id,
		Title:     title,
		Rect:      rect,
		Visible:   true,
		Resizable: true,
		Draggable: true,
		Content:   content,
	}
}

// IsMaximized reports whether the window is currently maximized.
func (w *Window) IsMaximized() bool {
	return w.PreMaxRect != nil
}

// titleBarRows returns the effective title bar height (minimum 1).
func (w *Window) titleBarRows() int {
	if w.TitleBarHeight < 1 {
		return 1
	}
	return w.TitleBarHeight
}

// TitleBarRect returns the rectangle of the title bar.
func (w *Window) TitleBarRect() geometry.Rect {
	return geometry.Rect{
		X:      w.Rect.X,
		Y:      w.Rect.Y,
		Width:  w.Rect.Width,
		Height: w.titleBarRows(),
	}
}

// ContentRect returns the inner content area (inside borders and title bar).
func (w *Window) ContentRect() geometry.Rect {
	tbh := w.titleBarRows()
	// Maximized: no side borders, no bottom border — content fills all space below title bar.
	if w.IsMaximized() {
		if w.Rect.Width < 1 || w.Rect.Height < tbh+1 {
			return geometry.Rect{}
		}
		return geometry.Rect{
			X:      w.Rect.X,
			Y:      w.Rect.Y + tbh,
			Width:  w.Rect.Width,
			Height: w.Rect.Height - tbh,
		}
	}
	// Normal: title bar rows at top | content | 1 row bottom border
	// Left border (1) + content + right border (1)
	minH := tbh + 2 // title bar + at least 1 content row + bottom border
	if w.Rect.Width < 3 || w.Rect.Height < minH {
		return geometry.Rect{}
	}
	return geometry.Rect{
		X:      w.Rect.X + 1,
		Y:      w.Rect.Y + tbh,
		Width:  w.Rect.Width - 2,
		Height: w.Rect.Height - tbh - 1,
	}
}

// IsSplit reports whether this window has a split pane layout.
func (w *Window) IsSplit() bool {
	return w.SplitRoot != nil
}

// SplitContentRect returns the content area for split windows.
// Uses the same insets as ContentRect (side + bottom borders).
func (w *Window) SplitContentRect() geometry.Rect {
	return w.ContentRect()
}

// CloseButtonPos returns the position of the close button [X] in the title bar.
func (w *Window) CloseButtonPos() geometry.Point {
	return geometry.Point{
		X: w.Rect.X + w.Rect.Width - 4,
		Y: w.Rect.Y,
	}
}

// MaxButtonPos returns the position of the maximize button [□] in the title bar.
// Button layout right-to-left: [◧][◨][□][_][×]
func (w *Window) MaxButtonPos() geometry.Point {
	return geometry.Point{
		X: w.Rect.X + w.Rect.Width - 10, // close(3) + min(3) + max starts here
		Y: w.Rect.Y,
	}
}

// SnapRightButtonPos returns the position of the snap-right button in the title bar.
func (w *Window) SnapRightButtonPos() geometry.Point {
	return geometry.Point{
		X: w.Rect.X + w.Rect.Width - 13,
		Y: w.Rect.Y,
	}
}

// SnapLeftButtonPos returns the position of the snap-left button in the title bar.
func (w *Window) SnapLeftButtonPos() geometry.Point {
	return geometry.Point{
		X: w.Rect.X + w.Rect.Width - 16,
		Y: w.Rect.Y,
	}
}

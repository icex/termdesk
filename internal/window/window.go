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
	Content   tea.Model

	// PreMaxRect stores the rect before maximization for restore.
	PreMaxRect      *geometry.Rect
	HasNotification bool   // true when window has unread output/bell
	Command         string // command that launched this window
	TitleBarHeight  int    // rows for the title bar (default 1)
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
	// Layout: title bar rows at top | content | 1 row bottom border
	// Left border (1) + content + right border (1)
	tbh := w.titleBarRows()
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

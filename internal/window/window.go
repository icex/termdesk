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
	PreMaxRect *geometry.Rect
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

// TitleBarRect returns the rectangle of the title bar (top row of window).
func (w *Window) TitleBarRect() geometry.Rect {
	return geometry.Rect{
		X:      w.Rect.X,
		Y:      w.Rect.Y,
		Width:  w.Rect.Width,
		Height: 1,
	}
}

// ContentRect returns the inner content area (inside borders and title bar).
func (w *Window) ContentRect() geometry.Rect {
	// Border: 1 cell on each side, title bar: 1 row at top
	// Layout: border-top+title | content | border-bottom
	// Left border + content + right border
	if w.Rect.Width < 3 || w.Rect.Height < 3 {
		return geometry.Rect{}
	}
	return geometry.Rect{
		X:      w.Rect.X + 1,
		Y:      w.Rect.Y + 1,
		Width:  w.Rect.Width - 2,
		Height: w.Rect.Height - 2,
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
func (w *Window) MaxButtonPos() geometry.Point {
	return geometry.Point{
		X: w.Rect.X + w.Rect.Width - 7,
		Y: w.Rect.Y,
	}
}

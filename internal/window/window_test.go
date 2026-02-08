package window

import (
	"testing"

	"github.com/icex/termdesk/pkg/geometry"
)

func TestNewWindow(t *testing.T) {
	r := geometry.Rect{X: 10, Y: 5, Width: 40, Height: 20}
	w := NewWindow("test-1", "Test Window", r, nil)

	if w.ID != "test-1" {
		t.Errorf("ID = %q, want %q", w.ID, "test-1")
	}
	if w.Title != "Test Window" {
		t.Errorf("Title = %q, want %q", w.Title, "Test Window")
	}
	if w.Rect != r {
		t.Errorf("Rect = %v, want %v", w.Rect, r)
	}
	if !w.Visible {
		t.Error("expected Visible to be true by default")
	}
	if !w.Resizable {
		t.Error("expected Resizable to be true by default")
	}
	if !w.Draggable {
		t.Error("expected Draggable to be true by default")
	}
	if w.Focused {
		t.Error("expected Focused to be false by default")
	}
	if w.Modal {
		t.Error("expected Modal to be false by default")
	}
	if w.Minimized {
		t.Error("expected Minimized to be false by default")
	}
}

func TestIsMaximized(t *testing.T) {
	w := NewWindow("w1", "Win", geometry.Rect{0, 0, 40, 20}, nil)
	if w.IsMaximized() {
		t.Error("expected not maximized initially")
	}

	r := geometry.Rect{5, 5, 30, 15}
	w.PreMaxRect = &r
	if !w.IsMaximized() {
		t.Error("expected maximized after setting PreMaxRect")
	}
}

func TestTitleBarRect(t *testing.T) {
	w := NewWindow("w1", "Win", geometry.Rect{X: 5, Y: 3, Width: 30, Height: 15}, nil)
	got := w.TitleBarRect()
	want := geometry.Rect{X: 5, Y: 3, Width: 30, Height: 1}
	if got != want {
		t.Errorf("TitleBarRect() = %v, want %v", got, want)
	}
}

func TestContentRect(t *testing.T) {
	w := NewWindow("w1", "Win", geometry.Rect{X: 5, Y: 3, Width: 30, Height: 15}, nil)
	got := w.ContentRect()
	want := geometry.Rect{X: 6, Y: 4, Width: 28, Height: 13}
	if got != want {
		t.Errorf("ContentRect() = %v, want %v", got, want)
	}
}

func TestContentRectTooSmall(t *testing.T) {
	w := NewWindow("w1", "Win", geometry.Rect{X: 0, Y: 0, Width: 2, Height: 2}, nil)
	got := w.ContentRect()
	if !got.IsEmpty() {
		t.Errorf("expected empty ContentRect for tiny window, got %v", got)
	}
}

func TestCloseButtonPos(t *testing.T) {
	w := NewWindow("w1", "Win", geometry.Rect{X: 0, Y: 0, Width: 40, Height: 20}, nil)
	got := w.CloseButtonPos()
	want := geometry.Point{X: 36, Y: 0}
	if got != want {
		t.Errorf("CloseButtonPos() = %v, want %v", got, want)
	}
}

func TestMaxButtonPos(t *testing.T) {
	w := NewWindow("w1", "Win", geometry.Rect{X: 0, Y: 0, Width: 40, Height: 20}, nil)
	got := w.MaxButtonPos()
	want := geometry.Point{X: 30, Y: 0}
	if got != want {
		t.Errorf("MaxButtonPos() = %v, want %v", got, want)
	}
}

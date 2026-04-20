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
	w := NewWindow("w1", "Win", geometry.Rect{X: 0, Y: 0, Width: 40, Height: 20}, nil)
	if w.IsMaximized() {
		t.Error("expected not maximized initially")
	}

	r := geometry.Rect{X: 5, Y: 5, Width: 30, Height: 15}
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

func TestSnapRightButtonPos(t *testing.T) {
	w := NewWindow("w1", "Win", geometry.Rect{X: 0, Y: 0, Width: 40, Height: 20}, nil)
	got := w.SnapRightButtonPos()
	want := geometry.Point{X: 27, Y: 0}
	if got != want {
		t.Errorf("SnapRightButtonPos() = %v, want %v", got, want)
	}
}

func TestSnapLeftButtonPos(t *testing.T) {
	w := NewWindow("w1", "Win", geometry.Rect{X: 0, Y: 0, Width: 40, Height: 20}, nil)
	got := w.SnapLeftButtonPos()
	want := geometry.Point{X: 24, Y: 0}
	if got != want {
		t.Errorf("SnapLeftButtonPos() = %v, want %v", got, want)
	}
}

func TestSnapButtonPosWithOffset(t *testing.T) {
	w := NewWindow("w1", "Win", geometry.Rect{X: 10, Y: 5, Width: 50, Height: 25}, nil)
	// SnapRight: X + Width - 13 = 10 + 50 - 13 = 47
	sr := w.SnapRightButtonPos()
	if sr.X != 47 || sr.Y != 5 {
		t.Errorf("SnapRightButtonPos() with offset = %v, want {47, 5}", sr)
	}
	// SnapLeft: X + Width - 16 = 10 + 50 - 16 = 44
	sl := w.SnapLeftButtonPos()
	if sl.X != 44 || sl.Y != 5 {
		t.Errorf("SnapLeftButtonPos() with offset = %v, want {44, 5}", sl)
	}
}

func TestTitleBarRowsCustomHeight(t *testing.T) {
	w := NewWindow("w1", "Win", geometry.Rect{X: 0, Y: 0, Width: 40, Height: 20}, nil)
	// Default (0) should return 1
	if w.titleBarRows() != 1 {
		t.Errorf("titleBarRows() default = %d, want 1", w.titleBarRows())
	}
	// Custom height of 3
	w.TitleBarHeight = 3
	if w.titleBarRows() != 3 {
		t.Errorf("titleBarRows() custom = %d, want 3", w.titleBarRows())
	}
	// Negative should clamp to 1
	w.TitleBarHeight = -1
	if w.titleBarRows() != 1 {
		t.Errorf("titleBarRows() negative = %d, want 1", w.titleBarRows())
	}
}

func TestTitleBarRectCustomHeight(t *testing.T) {
	w := NewWindow("w1", "Win", geometry.Rect{X: 5, Y: 3, Width: 30, Height: 15}, nil)
	w.TitleBarHeight = 2
	got := w.TitleBarRect()
	want := geometry.Rect{X: 5, Y: 3, Width: 30, Height: 2}
	if got != want {
		t.Errorf("TitleBarRect() with custom height = %v, want %v", got, want)
	}
}

func TestContentRectMaximized(t *testing.T) {
	w := NewWindow("w1", "Win", geometry.Rect{X: 0, Y: 1, Width: 120, Height: 38}, nil)
	prev := w.Rect
	w.PreMaxRect = &prev // mark as maximized
	got := w.ContentRect()
	// Maximized: no borders, content starts at Y+1 (title bar), full width
	want := geometry.Rect{X: 0, Y: 2, Width: 120, Height: 37}
	if got != want {
		t.Errorf("ContentRect() maximized = %v, want %v", got, want)
	}
}

func TestContentRectMaximizedTooSmall(t *testing.T) {
	// Maximized window that is too small (height < tbh+1)
	w := NewWindow("w1", "Win", geometry.Rect{X: 0, Y: 0, Width: 120, Height: 1}, nil)
	prev := geometry.Rect{X: 5, Y: 5, Width: 40, Height: 20}
	w.PreMaxRect = &prev // mark as maximized
	got := w.ContentRect()
	if !got.IsEmpty() {
		t.Errorf("expected empty ContentRect for too-small maximized window, got %v", got)
	}
}

func TestContentRectMaximizedZeroWidth(t *testing.T) {
	w := NewWindow("w1", "Win", geometry.Rect{X: 0, Y: 0, Width: 0, Height: 20}, nil)
	prev := geometry.Rect{X: 5, Y: 5, Width: 40, Height: 20}
	w.PreMaxRect = &prev
	got := w.ContentRect()
	if !got.IsEmpty() {
		t.Errorf("expected empty ContentRect for zero-width maximized window, got %v", got)
	}
}

func TestContentRectMultiRowTitleBar(t *testing.T) {
	w := NewWindow("w1", "Win", geometry.Rect{X: 5, Y: 3, Width: 30, Height: 15}, nil)
	w.TitleBarHeight = 3
	got := w.ContentRect()
	// Normal: content starts at Y+3 (3-row title bar), sides have 1px border,
	// bottom has 1px border. minH = 3 + 2 = 5. Height=15 >= 5.
	// Content: X+1, Y+3, Width-2, Height-3-1
	want := geometry.Rect{X: 6, Y: 6, Width: 28, Height: 11}
	if got != want {
		t.Errorf("ContentRect() multi-row title bar = %v, want %v", got, want)
	}
}

func TestContentRectMultiRowTitleBarTooSmall(t *testing.T) {
	w := NewWindow("w1", "Win", geometry.Rect{X: 0, Y: 0, Width: 30, Height: 3}, nil)
	w.TitleBarHeight = 3
	// minH = 3 + 2 = 5, but Height=3 < 5, so should be empty
	got := w.ContentRect()
	if !got.IsEmpty() {
		t.Errorf("expected empty ContentRect for too-short multi-row title bar, got %v", got)
	}
}

func TestIsSplit(t *testing.T) {
	w := NewWindow("w1", "Win", geometry.Rect{X: 0, Y: 0, Width: 40, Height: 20}, nil)
	if w.IsSplit() {
		t.Error("expected IsSplit() = false for unsplit window")
	}

	w.SplitRoot = &SplitNode{
		Dir: SplitHorizontal, Ratio: 0.5,
		Children: [2]*SplitNode{{TermID: "a"}, {TermID: "b"}},
	}
	if !w.IsSplit() {
		t.Error("expected IsSplit() = true for split window")
	}
}

func TestSplitContentRect(t *testing.T) {
	w := NewWindow("w1", "Win", geometry.Rect{X: 5, Y: 3, Width: 30, Height: 15}, nil)
	w.SplitRoot = &SplitNode{
		Dir: SplitHorizontal, Ratio: 0.5,
		Children: [2]*SplitNode{{TermID: "a"}, {TermID: "b"}},
	}

	// SplitContentRect should match ContentRect
	got := w.SplitContentRect()
	want := w.ContentRect()
	if got != want {
		t.Errorf("SplitContentRect() = %v, want %v (same as ContentRect)", got, want)
	}
}

func TestSplitContentRectMaximized(t *testing.T) {
	w := NewWindow("w1", "Win", geometry.Rect{X: 0, Y: 1, Width: 120, Height: 38}, nil)
	prev := w.Rect
	w.PreMaxRect = &prev
	w.SplitRoot = &SplitNode{
		Dir: SplitHorizontal, Ratio: 0.5,
		Children: [2]*SplitNode{{TermID: "a"}, {TermID: "b"}},
	}

	got := w.SplitContentRect()
	want := w.ContentRect()
	if got != want {
		t.Errorf("SplitContentRect() maximized = %v, want %v", got, want)
	}
}

package window

import (
	"testing"

	"github.com/icex/termdesk/pkg/geometry"
)

var testWorkArea = geometry.Rect{X: 0, Y: 1, Width: 120, Height: 38}

func TestSnapLeft(t *testing.T) {
	w := NewWindow("w1", "Test", geometry.Rect{X: 20, Y: 10, Width: 40, Height: 20}, nil)
	SnapLeft(w, testWorkArea)
	if w.Rect.X != 0 || w.Rect.Y != 1 || w.Rect.Width != 60 || w.Rect.Height != 38 {
		t.Errorf("SnapLeft: %v, want {0 1 60 38}", w.Rect)
	}
	if w.IsMaximized() {
		t.Error("snap should clear maximize state")
	}
}

func TestSnapRight(t *testing.T) {
	w := NewWindow("w1", "Test", geometry.Rect{X: 20, Y: 10, Width: 40, Height: 20}, nil)
	SnapRight(w, testWorkArea)
	if w.Rect.X != 60 || w.Rect.Y != 1 || w.Rect.Width != 60 || w.Rect.Height != 38 {
		t.Errorf("SnapRight: %v, want {60 1 60 38}", w.Rect)
	}
}

func TestMaximize(t *testing.T) {
	original := geometry.Rect{X: 20, Y: 10, Width: 40, Height: 20}
	w := NewWindow("w1", "Test", original, nil)

	Maximize(w, testWorkArea)
	if w.Rect != testWorkArea {
		t.Errorf("Maximize: %v, want %v", w.Rect, testWorkArea)
	}
	if !w.IsMaximized() {
		t.Error("expected maximized state")
	}
	if *w.PreMaxRect != original {
		t.Errorf("PreMaxRect = %v, want %v", *w.PreMaxRect, original)
	}
}

func TestMaximizeAlreadyMaximized(t *testing.T) {
	w := NewWindow("w1", "Test", geometry.Rect{X: 20, Y: 10, Width: 40, Height: 20}, nil)
	Maximize(w, testWorkArea)
	savedRect := *w.PreMaxRect

	// Calling again should be no-op
	Maximize(w, testWorkArea)
	if *w.PreMaxRect != savedRect {
		t.Error("double maximize should not change PreMaxRect")
	}
}

func TestRestore(t *testing.T) {
	original := geometry.Rect{X: 20, Y: 10, Width: 40, Height: 20}
	w := NewWindow("w1", "Test", original, nil)
	Maximize(w, testWorkArea)
	Restore(w)

	if w.Rect != original {
		t.Errorf("Restore: %v, want %v", w.Rect, original)
	}
	if w.IsMaximized() {
		t.Error("should not be maximized after restore")
	}
}

func TestRestoreWhenNotMaximized(t *testing.T) {
	original := geometry.Rect{X: 20, Y: 10, Width: 40, Height: 20}
	w := NewWindow("w1", "Test", original, nil)
	Restore(w) // should be no-op
	if w.Rect != original {
		t.Error("restore on non-maximized should not change rect")
	}
}

func TestToggleMaximize(t *testing.T) {
	original := geometry.Rect{X: 20, Y: 10, Width: 40, Height: 20}
	w := NewWindow("w1", "Test", original, nil)

	ToggleMaximize(w, testWorkArea)
	if w.Rect != testWorkArea {
		t.Error("first toggle should maximize")
	}

	ToggleMaximize(w, testWorkArea)
	if w.Rect != original {
		t.Error("second toggle should restore")
	}
}

func TestTileAllSingle(t *testing.T) {
	w := NewWindow("w1", "Test", geometry.Rect{X: 20, Y: 10, Width: 40, Height: 20}, nil)
	TileAll([]*Window{w}, testWorkArea)
	if w.Rect != testWorkArea {
		t.Errorf("single window tile: %v, want full work area %v", w.Rect, testWorkArea)
	}
}

func TestTileAllTwo(t *testing.T) {
	w1 := NewWindow("w1", "A", geometry.Rect{}, nil)
	w2 := NewWindow("w2", "B", geometry.Rect{}, nil)
	TileAll([]*Window{w1, w2}, testWorkArea)

	// Should split into 2 columns
	if w1.Rect.Width != 60 || w2.Rect.Width != 60 {
		t.Errorf("two tile widths: %d, %d, want 60, 60", w1.Rect.Width, w2.Rect.Width)
	}
	if w1.Rect.X != 0 || w2.Rect.X != 60 {
		t.Errorf("two tile positions: %d, %d", w1.Rect.X, w2.Rect.X)
	}
}

func TestTileAllFour(t *testing.T) {
	windows := make([]*Window, 4)
	for i := range windows {
		windows[i] = NewWindow("w", "W", geometry.Rect{}, nil)
	}
	TileAll(windows, testWorkArea)

	// Should be 2x2 grid
	if windows[0].Rect.X != 0 || windows[0].Rect.Y != 1 {
		t.Errorf("w0 position: %v", windows[0].Rect)
	}
	if windows[1].Rect.X != 60 {
		t.Errorf("w1.X = %d, want 60", windows[1].Rect.X)
	}
	if windows[2].Rect.Y != 20 {
		t.Errorf("w2.Y = %d, want 20", windows[2].Rect.Y)
	}
}

func TestTileAllSkipsMinimized(t *testing.T) {
	w1 := NewWindow("w1", "A", geometry.Rect{}, nil)
	w2 := NewWindow("w2", "B", geometry.Rect{}, nil)
	w2.Minimized = true
	w3 := NewWindow("w3", "C", geometry.Rect{}, nil)

	TileAll([]*Window{w1, w2, w3}, testWorkArea)
	// w2 is minimized, so only w1 and w3 should be tiled
	if w1.Rect.Width != 60 {
		t.Errorf("w1 width = %d, want 60 (half for 2 visible)", w1.Rect.Width)
	}
}

func TestTileAllEmpty(t *testing.T) {
	// Should not panic
	TileAll(nil, testWorkArea)
	TileAll([]*Window{}, testWorkArea)
}

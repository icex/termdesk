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

func TestSnapTop(t *testing.T) {
	w := NewWindow("w1", "Test", geometry.Rect{X: 20, Y: 10, Width: 40, Height: 20}, nil)
	SnapTop(w, testWorkArea)
	want := geometry.Rect{X: 0, Y: 1, Width: 120, Height: 19}
	if w.Rect != want {
		t.Errorf("SnapTop: %v, want %v", w.Rect, want)
	}
	if w.IsMaximized() {
		t.Error("snap should clear maximize state")
	}
}

func TestSnapBottom(t *testing.T) {
	w := NewWindow("w1", "Test", geometry.Rect{X: 20, Y: 10, Width: 40, Height: 20}, nil)
	SnapBottom(w, testWorkArea)
	// halfH = 38/2 = 19, Y = 1+19 = 20, Height = 38-19 = 19
	want := geometry.Rect{X: 0, Y: 20, Width: 120, Height: 19}
	if w.Rect != want {
		t.Errorf("SnapBottom: %v, want %v", w.Rect, want)
	}
}

func TestCenterWindow(t *testing.T) {
	w := NewWindow("w1", "Test", geometry.Rect{X: 0, Y: 0, Width: 40, Height: 10}, nil)
	CenterWindow(w, testWorkArea)
	// 60% of 120 = 72, 70% of 38 = 26
	if w.Rect.Width != 72 || w.Rect.Height != 26 {
		t.Errorf("CenterWindow size: %dx%d, want 72x26", w.Rect.Width, w.Rect.Height)
	}
	// X = 0 + (120-72)/2 = 24, Y = 1 + (38-26)/2 = 7
	if w.Rect.X != 24 || w.Rect.Y != 7 {
		t.Errorf("CenterWindow pos: (%d,%d), want (24,7)", w.Rect.X, w.Rect.Y)
	}
}

func TestCenterWindowClearsMaximize(t *testing.T) {
	w := NewWindow("w1", "Test", geometry.Rect{X: 0, Y: 0, Width: 40, Height: 10}, nil)
	Maximize(w, testWorkArea)
	CenterWindow(w, testWorkArea)
	if w.IsMaximized() {
		t.Error("center should clear maximize state")
	}
}

func TestTileColumns(t *testing.T) {
	w1 := NewWindow("w1", "A", geometry.Rect{}, nil)
	w2 := NewWindow("w2", "B", geometry.Rect{}, nil)
	w3 := NewWindow("w3", "C", geometry.Rect{}, nil)
	TileColumns([]*Window{w1, w2, w3}, testWorkArea)

	if w1.Rect.X != 0 || w1.Rect.Width != 40 {
		t.Errorf("w1: X=%d Width=%d, want X=0 Width=40", w1.Rect.X, w1.Rect.Width)
	}
	if w2.Rect.X != 40 || w2.Rect.Width != 40 {
		t.Errorf("w2: X=%d Width=%d, want X=40 Width=40", w2.Rect.X, w2.Rect.Width)
	}
	// Last column gets remaining: 120 - 80 = 40
	if w3.Rect.X != 80 || w3.Rect.Width != 40 {
		t.Errorf("w3: X=%d Width=%d, want X=80 Width=40", w3.Rect.X, w3.Rect.Width)
	}
	// All full height
	for _, w := range []*Window{w1, w2, w3} {
		if w.Rect.Height != 38 || w.Rect.Y != 1 {
			t.Errorf("column: Y=%d Height=%d, want Y=1 Height=38", w.Rect.Y, w.Rect.Height)
		}
	}
}

func TestTileColumnsEmpty(t *testing.T) {
	TileColumns(nil, testWorkArea)
	TileColumns([]*Window{}, testWorkArea)
}

func TestTileRows(t *testing.T) {
	w1 := NewWindow("w1", "A", geometry.Rect{}, nil)
	w2 := NewWindow("w2", "B", geometry.Rect{}, nil)
	TileRows([]*Window{w1, w2}, testWorkArea)

	// 38/2 = 19
	if w1.Rect.Y != 1 || w1.Rect.Height != 19 {
		t.Errorf("w1: Y=%d Height=%d, want Y=1 Height=19", w1.Rect.Y, w1.Rect.Height)
	}
	if w2.Rect.Y != 20 || w2.Rect.Height != 19 {
		t.Errorf("w2: Y=%d Height=%d, want Y=20 Height=19", w2.Rect.Y, w2.Rect.Height)
	}
	// All full width
	if w1.Rect.Width != 120 || w2.Rect.Width != 120 {
		t.Errorf("widths: %d, %d, want 120", w1.Rect.Width, w2.Rect.Width)
	}
}

func TestTileRowsEmpty(t *testing.T) {
	TileRows(nil, testWorkArea)
	TileRows([]*Window{}, testWorkArea)
}

func TestCascade(t *testing.T) {
	windows := make([]*Window, 3)
	for i := range windows {
		windows[i] = NewWindow("w", "W", geometry.Rect{}, nil)
	}
	Cascade(windows, testWorkArea)

	// 60% of 120 = 72, 70% of 38 = 26
	for i, w := range windows {
		if w.Rect.Width != 72 || w.Rect.Height != 26 {
			t.Errorf("w%d size: %dx%d, want 72x26", i, w.Rect.Width, w.Rect.Height)
		}
	}
	// Windows should be offset diagonally
	if windows[0].Rect.X >= windows[1].Rect.X {
		t.Error("w0 should be left of w1")
	}
	if windows[0].Rect.Y >= windows[1].Rect.Y {
		t.Error("w0 should be above w1")
	}
}

func TestCascadeEmpty(t *testing.T) {
	Cascade(nil, testWorkArea)
	Cascade([]*Window{}, testWorkArea)
}

func TestCascadeSkipsMinimized(t *testing.T) {
	w1 := NewWindow("w1", "A", geometry.Rect{}, nil)
	w2 := NewWindow("w2", "B", geometry.Rect{}, nil)
	w2.Minimized = true
	Cascade([]*Window{w1, w2}, testWorkArea)
	// w2 minimized → only w1 cascaded
	if w1.Rect.Width != 72 {
		t.Errorf("w1 width = %d, want 72", w1.Rect.Width)
	}
}

func TestMoveWindow(t *testing.T) {
	w := NewWindow("w1", "Test", geometry.Rect{X: 20, Y: 10, Width: 40, Height: 20}, nil)
	MoveWindow(w, 4, 0, testWorkArea)
	if w.Rect.X != 24 {
		t.Errorf("MoveWindow right: X=%d, want 24", w.Rect.X)
	}
	if w.Rect.Y != 10 {
		t.Errorf("MoveWindow right: Y=%d, want 10", w.Rect.Y)
	}
}

func TestMoveWindowClamped(t *testing.T) {
	w := NewWindow("w1", "Test", geometry.Rect{X: 78, Y: 1, Width: 40, Height: 20}, nil)
	MoveWindow(w, 4, 0, testWorkArea)
	if w.Rect.Right() > testWorkArea.Right() {
		t.Errorf("move should clamp: Right()=%d > work area right=%d", w.Rect.Right(), testWorkArea.Right())
	}
}

func TestMoveWindowUp(t *testing.T) {
	w := NewWindow("w1", "Test", geometry.Rect{X: 20, Y: 10, Width: 40, Height: 20}, nil)
	MoveWindow(w, 0, -4, testWorkArea)
	if w.Rect.Y != 6 {
		t.Errorf("MoveWindow up: Y=%d, want 6", w.Rect.Y)
	}
}

func TestMoveWindowClearsMaximize(t *testing.T) {
	w := NewWindow("w1", "Test", geometry.Rect{X: 0, Y: 0, Width: 40, Height: 20}, nil)
	Maximize(w, testWorkArea)
	MoveWindow(w, 4, 0, testWorkArea)
	if w.IsMaximized() {
		t.Error("move should clear maximize state")
	}
}

func TestResizeWindow(t *testing.T) {
	w := NewWindow("w1", "Test", geometry.Rect{X: 20, Y: 10, Width: 50, Height: 20}, nil)
	ResizeWindow(w, 4, 0, testWorkArea)
	if w.Rect.Width != 54 {
		t.Errorf("ResizeWindow grow: Width=%d, want 54", w.Rect.Width)
	}
}

func TestResizeWindowShrink(t *testing.T) {
	w := NewWindow("w1", "Test", geometry.Rect{X: 20, Y: 10, Width: 50, Height: 20}, nil)
	ResizeWindow(w, -4, -2, testWorkArea)
	if w.Rect.Width != 46 {
		t.Errorf("ResizeWindow shrink: Width=%d, want 46", w.Rect.Width)
	}
	if w.Rect.Height != 18 {
		t.Errorf("ResizeWindow shrink: Height=%d, want 18", w.Rect.Height)
	}
}

func TestResizeWindowMinimum(t *testing.T) {
	w := NewWindow("w1", "Test", geometry.Rect{X: 20, Y: 10, Width: 42, Height: 12}, nil)
	ResizeWindow(w, -20, -20, testWorkArea)
	if w.Rect.Width < MinWindowWidth {
		t.Errorf("width %d below minimum %d", w.Rect.Width, MinWindowWidth)
	}
	if w.Rect.Height < MinWindowHeight {
		t.Errorf("height %d below minimum %d", w.Rect.Height, MinWindowHeight)
	}
}

func TestMaximizeAll(t *testing.T) {
	w1 := NewWindow("w1", "A", geometry.Rect{X: 5, Y: 5, Width: 40, Height: 20}, nil)
	w2 := NewWindow("w2", "B", geometry.Rect{X: 10, Y: 10, Width: 30, Height: 15}, nil)
	w3 := NewWindow("w3", "C", geometry.Rect{X: 0, Y: 0, Width: 20, Height: 10}, nil)
	w3.Minimized = true // minimized windows should be skipped
	MaximizeAll([]*Window{w1, w2, w3}, testWorkArea)
	if w1.Rect != testWorkArea {
		t.Errorf("w1 not maximized: %v", w1.Rect)
	}
	if w2.Rect != testWorkArea {
		t.Errorf("w2 not maximized: %v", w2.Rect)
	}
	if w3.Rect.Width == testWorkArea.Width {
		t.Error("minimized window should not be maximized")
	}
	if w1.PreMaxRect == nil {
		t.Error("w1 should have PreMaxRect saved")
	}
	if w2.PreMaxRect == nil {
		t.Error("w2 should have PreMaxRect saved")
	}
}

func TestMaximizeAllPreservesExistingMaxState(t *testing.T) {
	w := NewWindow("w1", "A", geometry.Rect{X: 5, Y: 5, Width: 40, Height: 20}, nil)
	// Maximize first
	Maximize(w, testWorkArea)
	origPre := *w.PreMaxRect
	// MaximizeAll should not overwrite existing PreMaxRect
	MaximizeAll([]*Window{w}, testWorkArea)
	if *w.PreMaxRect != origPre {
		t.Errorf("PreMaxRect should be preserved, got %v want %v", *w.PreMaxRect, origPre)
	}
}

func TestMaximizeAllSkipsNonResizable(t *testing.T) {
	w1 := NewWindow("w1", "A", geometry.Rect{X: 5, Y: 5, Width: 40, Height: 20}, nil)
	w2 := NewWindow("w2", "B", geometry.Rect{X: 10, Y: 10, Width: 30, Height: 15}, nil)
	w2.Resizable = false // fixed-size window
	origW2 := w2.Rect

	MaximizeAll([]*Window{w1, w2}, testWorkArea)

	if w1.Rect != testWorkArea {
		t.Errorf("resizable w1 should be maximized: %v", w1.Rect)
	}
	if w2.Rect != origW2 {
		t.Errorf("non-resizable w2 should be unchanged: got %v, want %v", w2.Rect, origW2)
	}
	if w2.PreMaxRect != nil {
		t.Error("non-resizable w2 should not have PreMaxRect set")
	}
}

func TestTileAllSkipsNonResizable(t *testing.T) {
	w1 := NewWindow("w1", "A", geometry.Rect{X: 5, Y: 5, Width: 40, Height: 20}, nil)
	w2 := NewWindow("w2", "B", geometry.Rect{X: 10, Y: 10, Width: 30, Height: 15}, nil)
	w2.Resizable = false
	origW2 := w2.Rect

	TileAll([]*Window{w1, w2}, testWorkArea)

	// w1 should fill the entire work area (only resizable window)
	if w1.Rect != testWorkArea {
		t.Errorf("resizable w1 should fill work area: %v", w1.Rect)
	}
	if w2.Rect != origW2 {
		t.Errorf("non-resizable w2 should be unchanged: got %v, want %v", w2.Rect, origW2)
	}
}

func TestTileColumnsSkipsNonResizable(t *testing.T) {
	w1 := NewWindow("w1", "A", geometry.Rect{}, nil)
	w2 := NewWindow("w2", "B", geometry.Rect{X: 10, Y: 10, Width: 30, Height: 15}, nil)
	w2.Resizable = false
	origW2 := w2.Rect

	TileColumns([]*Window{w1, w2}, testWorkArea)

	if w1.Rect.Width != testWorkArea.Width {
		t.Errorf("w1 should get full width: %d", w1.Rect.Width)
	}
	if w2.Rect != origW2 {
		t.Errorf("non-resizable w2 should be unchanged: got %v, want %v", w2.Rect, origW2)
	}
}

func TestTileRowsSkipsNonResizable(t *testing.T) {
	w1 := NewWindow("w1", "A", geometry.Rect{}, nil)
	w2 := NewWindow("w2", "B", geometry.Rect{X: 10, Y: 10, Width: 30, Height: 15}, nil)
	w2.Resizable = false
	origW2 := w2.Rect

	TileRows([]*Window{w1, w2}, testWorkArea)

	if w1.Rect.Height != testWorkArea.Height {
		t.Errorf("w1 should get full height: %d", w1.Rect.Height)
	}
	if w2.Rect != origW2 {
		t.Errorf("non-resizable w2 should be unchanged: got %v, want %v", w2.Rect, origW2)
	}
}

func TestCascadeSkipsNonResizable(t *testing.T) {
	w1 := NewWindow("w1", "A", geometry.Rect{}, nil)
	w2 := NewWindow("w2", "B", geometry.Rect{X: 10, Y: 10, Width: 30, Height: 15}, nil)
	w2.Resizable = false
	origW2 := w2.Rect

	Cascade([]*Window{w1, w2}, testWorkArea)

	if w1.Rect.Width != 72 {
		t.Errorf("w1 should be cascaded: width=%d, want 72", w1.Rect.Width)
	}
	if w2.Rect != origW2 {
		t.Errorf("non-resizable w2 should be unchanged: got %v, want %v", w2.Rect, origW2)
	}
}

func TestResizeWindowMaxBoundsClamping(t *testing.T) {
	// Work area is 120x38 starting at Y=1
	w := NewWindow("w1", "Test", geometry.Rect{X: 0, Y: 1, Width: 100, Height: 30}, nil)
	// Try to grow beyond work area width
	ResizeWindow(w, 50, 0, testWorkArea)
	if w.Rect.Width > testWorkArea.Width {
		t.Errorf("width %d exceeds work area width %d", w.Rect.Width, testWorkArea.Width)
	}
	if w.Rect.Width != testWorkArea.Width {
		t.Errorf("width should be clamped to work area: got %d, want %d", w.Rect.Width, testWorkArea.Width)
	}
}

func TestResizeWindowMaxHeightClamping(t *testing.T) {
	w := NewWindow("w1", "Test", geometry.Rect{X: 0, Y: 1, Width: 50, Height: 30}, nil)
	// Try to grow beyond work area height
	ResizeWindow(w, 0, 50, testWorkArea)
	if w.Rect.Height > testWorkArea.Height {
		t.Errorf("height %d exceeds work area height %d", w.Rect.Height, testWorkArea.Height)
	}
	if w.Rect.Height != testWorkArea.Height {
		t.Errorf("height should be clamped to work area: got %d, want %d", w.Rect.Height, testWorkArea.Height)
	}
}

func TestCenterWindowTinyWorkArea(t *testing.T) {
	// Work area smaller than MinWindowWidth x MinWindowHeight
	tinyWA := geometry.Rect{X: 0, Y: 0, Width: 30, Height: 8}
	w := NewWindow("w1", "Test", geometry.Rect{X: 0, Y: 0, Width: 40, Height: 10}, nil)
	CenterWindow(w, tinyWA)

	// 60% of 30 = 18, which is < MinWindowWidth(40), so cw = 40
	// But 40 > 30 (work area width), so cw gets clamped to 30
	if w.Rect.Width > tinyWA.Width {
		t.Errorf("width %d exceeds tiny work area width %d", w.Rect.Width, tinyWA.Width)
	}
	if w.Rect.Width != tinyWA.Width {
		t.Errorf("width should be clamped to work area: got %d, want %d", w.Rect.Width, tinyWA.Width)
	}

	// 70% of 8 = 5, which is < MinWindowHeight(10), so ch = 10
	// But 10 > 8 (work area height), so ch gets clamped to 8
	if w.Rect.Height > tinyWA.Height {
		t.Errorf("height %d exceeds tiny work area height %d", w.Rect.Height, tinyWA.Height)
	}
	if w.Rect.Height != tinyWA.Height {
		t.Errorf("height should be clamped to work area: got %d, want %d", w.Rect.Height, tinyWA.Height)
	}
}

func TestCascadeTinyWorkArea(t *testing.T) {
	// Work area where 60% width < MinWindowWidth and 70% height < MinWindowHeight
	tinyWA := geometry.Rect{X: 0, Y: 0, Width: 50, Height: 12}
	w1 := NewWindow("w1", "A", geometry.Rect{}, nil)
	w2 := NewWindow("w2", "B", geometry.Rect{}, nil)
	Cascade([]*Window{w1, w2}, tinyWA)

	// 60% of 50 = 30, < MinWindowWidth(40), so cw = 40
	// 70% of 12 = 8, < MinWindowHeight(10), so ch = 10
	if w1.Rect.Width != 40 {
		t.Errorf("cascade tiny width = %d, want MinWindowWidth(%d)", w1.Rect.Width, MinWindowWidth)
	}
	if w1.Rect.Height != 10 {
		t.Errorf("cascade tiny height = %d, want MinWindowHeight(%d)", w1.Rect.Height, MinWindowHeight)
	}

	// maxX = 0 + 50 - 40 = 10, maxY = 0 + 12 - 10 = 2
	// w1: x = 0 + (0*3)%(10+1) = 0, y = 0 + (0*2)%(2+1) = 0
	// w2: x = 0 + (1*3)%(10+1) = 3, y = 0 + (1*2)%(2+1) = 2
	if w2.Rect.X != 3 || w2.Rect.Y != 2 {
		t.Errorf("cascade tiny w2 position: (%d, %d), want (3, 2)", w2.Rect.X, w2.Rect.Y)
	}
}

func TestCascadeVeryTinyWorkArea(t *testing.T) {
	// Work area exactly equal to MinWindowWidth/MinWindowHeight:
	// maxX = workArea.X + workArea.Width - cw = 0 + 40 - 40 = 0
	// maxY = workArea.Y + workArea.Height - ch = 0 + 10 - 10 = 0
	// This triggers the maxX <= workArea.X and maxY <= workArea.Y branches
	veryTinyWA := geometry.Rect{X: 0, Y: 0, Width: 40, Height: 10}
	w1 := NewWindow("w1", "A", geometry.Rect{}, nil)
	w2 := NewWindow("w2", "B", geometry.Rect{}, nil)
	Cascade([]*Window{w1, w2}, veryTinyWA)

	// All windows should be placed at workArea origin
	if w1.Rect.X != 0 || w1.Rect.Y != 0 {
		t.Errorf("very tiny cascade w1: (%d, %d), want (0, 0)", w1.Rect.X, w1.Rect.Y)
	}
	if w2.Rect.X != 0 || w2.Rect.Y != 0 {
		t.Errorf("very tiny cascade w2: (%d, %d), want (0, 0)", w2.Rect.X, w2.Rect.Y)
	}
}

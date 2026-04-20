package window

import (
	"testing"

	"github.com/icex/termdesk/pkg/geometry"
)

func TestHitTest(t *testing.T) {
	w := NewWindow("w1", "Test", geometry.Rect{X: 5, Y: 3, Width: 30, Height: 15}, nil)
	w.Resizable = true

	tests := []struct {
		name string
		p    geometry.Point
		want HitZone
	}{
		{"outside", geometry.Point{X: 0, Y: 0}, HitNone},
		{"title bar", geometry.Point{X: 10, Y: 3}, HitTitleBar},
		{"close button", geometry.Point{X: 33, Y: 3}, HitCloseButton},
		{"min button", geometry.Point{X: 30, Y: 3}, HitMinButton},
		{"max button", geometry.Point{X: 27, Y: 3}, HitMaxButton},
		{"content", geometry.Point{X: 15, Y: 10}, HitContent},
		{"left border", geometry.Point{X: 5, Y: 10}, HitBorderW},
		{"right border", geometry.Point{X: 34, Y: 10}, HitBorderE},
		{"bottom border", geometry.Point{X: 15, Y: 17}, HitBorderS},
		{"bottom-left corner", geometry.Point{X: 5, Y: 17}, HitBorderSW},
		{"bottom-right corner", geometry.Point{X: 34, Y: 17}, HitBorderSE},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := HitTest(w, tt.p, 3, 3)
			if got != tt.want {
				t.Errorf("HitTest(%v) = %v, want %v", tt.p, got, tt.want)
			}
		})
	}
}

func TestHitTestNonResizable(t *testing.T) {
	w := NewWindow("w1", "Test", geometry.Rect{X: 0, Y: 0, Width: 30, Height: 10}, nil)
	w.Resizable = false

	// Area where max button would be should return title bar for non-resizable
	// (non-resizable windows have no max/snap buttons, only min and close)
	got := HitTest(w, geometry.Point{X: 20, Y: 0}, 3, 3)
	if got != HitTitleBar {
		t.Errorf("expected HitTitleBar for non-resizable window, got %v", got)
	}
}

func TestHitTestWithTheme(t *testing.T) {
	w := NewWindow("w1", "Test", geometry.Rect{X: 0, Y: 0, Width: 30, Height: 10}, nil)
	got := HitTestWithTheme(w, geometry.Point{X: 15, Y: 5}, "[X]", "[□]")
	if got != HitContent {
		t.Errorf("expected HitContent, got %v", got)
	}
}

func TestDragModeForZone(t *testing.T) {
	tests := []struct {
		zone HitZone
		want DragMode
	}{
		{HitTitleBar, DragMove},
		{HitBorderN, DragResizeN},
		{HitBorderS, DragResizeS},
		{HitBorderE, DragResizeE},
		{HitBorderW, DragResizeW},
		{HitBorderNE, DragResizeNE},
		{HitBorderNW, DragResizeNW},
		{HitBorderSE, DragResizeSE},
		{HitBorderSW, DragResizeSW},
		{HitContent, DragNone},
		{HitCloseButton, DragNone},
		{HitNone, DragNone},
	}
	for _, tt := range tests {
		got := DragModeForZone(tt.zone)
		if got != tt.want {
			t.Errorf("DragModeForZone(%v) = %v, want %v", tt.zone, got, tt.want)
		}
	}
}

func TestApplyDragMove(t *testing.T) {
	bounds := geometry.Rect{X: 0, Y: 0, Width: 100, Height: 50}
	state := DragState{
		Active:     true,
		Mode:       DragMove,
		StartMouse: geometry.Point{X: 15, Y: 10},
		StartRect:  geometry.Rect{X: 10, Y: 5, Width: 30, Height: 15},
	}

	got := ApplyDrag(state, geometry.Point{X: 25, Y: 15}, bounds)
	want := geometry.Rect{X: 20, Y: 10, Width: 30, Height: 15}
	if got != want {
		t.Errorf("move drag = %v, want %v", got, want)
	}
}

func TestApplyDragMoveClamp(t *testing.T) {
	bounds := geometry.Rect{X: 0, Y: 0, Width: 50, Height: 30}
	state := DragState{
		Mode:       DragMove,
		StartMouse: geometry.Point{X: 15, Y: 10},
		StartRect:  geometry.Rect{X: 10, Y: 5, Width: 30, Height: 15},
	}

	// Drag far right, should clamp
	got := ApplyDrag(state, geometry.Point{X: 100, Y: 10}, bounds)
	if got.Right() > bounds.Right() {
		t.Errorf("move should clamp: right=%d > bounds=%d", got.Right(), bounds.Right())
	}
}

func TestApplyDragResizeE(t *testing.T) {
	bounds := geometry.Rect{X: 0, Y: 0, Width: 100, Height: 50}
	state := DragState{
		Mode:       DragResizeE,
		StartMouse: geometry.Point{X: 40, Y: 10},
		StartRect:  geometry.Rect{X: 10, Y: 5, Width: 30, Height: 15},
	}

	got := ApplyDrag(state, geometry.Point{X: 50, Y: 10}, bounds)
	if got.Width != 40 {
		t.Errorf("resize E: width = %d, want 40", got.Width)
	}
}

func TestApplyDragResizeS(t *testing.T) {
	bounds := geometry.Rect{X: 0, Y: 0, Width: 100, Height: 50}
	state := DragState{
		Mode:       DragResizeS,
		StartMouse: geometry.Point{X: 20, Y: 20},
		StartRect:  geometry.Rect{X: 10, Y: 5, Width: 30, Height: 15},
	}

	got := ApplyDrag(state, geometry.Point{X: 20, Y: 25}, bounds)
	if got.Height != 20 {
		t.Errorf("resize S: height = %d, want 20", got.Height)
	}
}

func TestApplyDragResizeSE(t *testing.T) {
	bounds := geometry.Rect{X: 0, Y: 0, Width: 100, Height: 50}
	state := DragState{
		Mode:       DragResizeSE,
		StartMouse: geometry.Point{X: 40, Y: 20},
		StartRect:  geometry.Rect{X: 10, Y: 5, Width: 30, Height: 15},
	}

	got := ApplyDrag(state, geometry.Point{X: 50, Y: 25}, bounds)
	if got.Width != 40 || got.Height != 20 {
		t.Errorf("resize SE: %dx%d, want 40x20", got.Width, got.Height)
	}
}

func TestApplyDragResizeW(t *testing.T) {
	bounds := geometry.Rect{X: 0, Y: 0, Width: 200, Height: 50}
	state := DragState{
		Mode:       DragResizeW,
		StartMouse: geometry.Point{X: 10, Y: 10},
		StartRect:  geometry.Rect{X: 10, Y: 5, Width: 60, Height: 15},
	}

	got := ApplyDrag(state, geometry.Point{X: 5, Y: 10}, bounds)
	if got.X != 5 || got.Width != 65 {
		t.Errorf("resize W: x=%d w=%d, want x=5 w=65", got.X, got.Width)
	}
}

func TestApplyDragResizeN(t *testing.T) {
	bounds := geometry.Rect{X: 0, Y: 0, Width: 100, Height: 50}
	state := DragState{
		Mode:       DragResizeN,
		StartMouse: geometry.Point{X: 20, Y: 5},
		StartRect:  geometry.Rect{X: 10, Y: 5, Width: 30, Height: 15},
	}

	got := ApplyDrag(state, geometry.Point{X: 20, Y: 2}, bounds)
	if got.Y != 2 || got.Height != 18 {
		t.Errorf("resize N: y=%d h=%d, want y=2 h=18", got.Y, got.Height)
	}
}

func TestApplyDragResizeNW(t *testing.T) {
	bounds := geometry.Rect{X: 0, Y: 0, Width: 200, Height: 50}
	state := DragState{
		Mode:       DragResizeNW,
		StartMouse: geometry.Point{X: 10, Y: 5},
		StartRect:  geometry.Rect{X: 10, Y: 5, Width: 60, Height: 15},
	}

	got := ApplyDrag(state, geometry.Point{X: 5, Y: 2}, bounds)
	if got.X != 5 || got.Y != 2 || got.Width != 65 || got.Height != 18 {
		t.Errorf("resize NW: %v, want {5 2 65 18}", got)
	}
}

func TestApplyDragResizeNE(t *testing.T) {
	bounds := geometry.Rect{X: 0, Y: 0, Width: 100, Height: 50}
	state := DragState{
		Mode:       DragResizeNE,
		StartMouse: geometry.Point{X: 40, Y: 5},
		StartRect:  geometry.Rect{X: 10, Y: 5, Width: 30, Height: 15},
	}

	got := ApplyDrag(state, geometry.Point{X: 50, Y: 2}, bounds)
	if got.Width != 40 || got.Y != 2 || got.Height != 18 {
		t.Errorf("resize NE: %v, want w=40 y=2 h=18", got)
	}
}

func TestApplyDragResizeSW(t *testing.T) {
	bounds := geometry.Rect{X: 0, Y: 0, Width: 200, Height: 50}
	state := DragState{
		Mode:       DragResizeSW,
		StartMouse: geometry.Point{X: 10, Y: 20},
		StartRect:  geometry.Rect{X: 10, Y: 5, Width: 60, Height: 15},
	}

	got := ApplyDrag(state, geometry.Point{X: 5, Y: 25}, bounds)
	if got.X != 5 || got.Width != 65 || got.Height != 20 {
		t.Errorf("resize SW: %v, want x=5 w=65 h=20", got)
	}
}

func TestApplyDragMinSize(t *testing.T) {
	bounds := geometry.Rect{X: 0, Y: 0, Width: 100, Height: 50}

	// Resize E to very small
	state := DragState{
		Mode:       DragResizeE,
		StartMouse: geometry.Point{X: 40, Y: 10},
		StartRect:  geometry.Rect{X: 10, Y: 5, Width: 30, Height: 15},
	}
	got := ApplyDrag(state, geometry.Point{X: 5, Y: 10}, bounds)
	if got.Width < MinWindowWidth {
		t.Errorf("width %d below minimum %d", got.Width, MinWindowWidth)
	}

	// Resize S to very small
	state.Mode = DragResizeS
	state.StartMouse = geometry.Point{X: 20, Y: 20}
	got = ApplyDrag(state, geometry.Point{X: 20, Y: 0}, bounds)
	if got.Height < MinWindowHeight {
		t.Errorf("height %d below minimum %d", got.Height, MinWindowHeight)
	}
}

func TestApplyDragNone(t *testing.T) {
	bounds := geometry.Rect{X: 0, Y: 0, Width: 100, Height: 50}
	state := DragState{
		Mode:      DragNone,
		StartRect: geometry.Rect{X: 10, Y: 5, Width: 30, Height: 15},
	}
	got := ApplyDrag(state, geometry.Point{X: 50, Y: 50}, bounds)
	if got != state.StartRect.Clamp(bounds) {
		t.Error("DragNone should not change rect")
	}
}

func TestApplyDragResizeWMinSize(t *testing.T) {
	bounds := geometry.Rect{X: 0, Y: 0, Width: 200, Height: 50}
	state := DragState{
		Mode:       DragResizeW,
		StartMouse: geometry.Point{X: 10, Y: 10},
		StartRect:  geometry.Rect{X: 10, Y: 5, Width: 45, Height: 15},
	}
	// Drag right by 20, making width = 45 - 20 = 25 < MinWindowWidth (40)
	got := ApplyDrag(state, geometry.Point{X: 30, Y: 10}, bounds)
	if got.Width < MinWindowWidth {
		t.Errorf("resize W width %d below minimum %d", got.Width, MinWindowWidth)
	}
	// X should be clamped: startX + startW - MinWindowWidth = 10 + 45 - 40 = 15
	if got.X != 15 {
		t.Errorf("resize W X = %d, want 15", got.X)
	}
}

func TestApplyDragResizeNMinSize(t *testing.T) {
	bounds := geometry.Rect{X: 0, Y: 0, Width: 100, Height: 50}
	state := DragState{
		Mode:       DragResizeN,
		StartMouse: geometry.Point{X: 20, Y: 5},
		StartRect:  geometry.Rect{X: 10, Y: 5, Width: 40, Height: 12},
	}
	// Drag down by 10, making height = 12 - 10 = 2 < MinWindowHeight (10)
	got := ApplyDrag(state, geometry.Point{X: 20, Y: 15}, bounds)
	if got.Height < MinWindowHeight {
		t.Errorf("resize N height %d below minimum %d", got.Height, MinWindowHeight)
	}
	// Y should be clamped: startY + startH - MinWindowHeight = 5 + 12 - 10 = 7
	if got.Y != 7 {
		t.Errorf("resize N Y = %d, want 7", got.Y)
	}
}

func TestApplyDragResizeNWMinSize(t *testing.T) {
	bounds := geometry.Rect{X: 0, Y: 0, Width: 200, Height: 50}
	state := DragState{
		Mode:       DragResizeNW,
		StartMouse: geometry.Point{X: 10, Y: 5},
		StartRect:  geometry.Rect{X: 10, Y: 5, Width: 45, Height: 12},
	}
	// Drag right and down to shrink below minimums
	got := ApplyDrag(state, geometry.Point{X: 30, Y: 15}, bounds)
	if got.Width < MinWindowWidth {
		t.Errorf("resize NW width %d below minimum %d", got.Width, MinWindowWidth)
	}
	if got.Height < MinWindowHeight {
		t.Errorf("resize NW height %d below minimum %d", got.Height, MinWindowHeight)
	}
	// X clamped: 10 + 45 - 40 = 15
	if got.X != 15 {
		t.Errorf("resize NW X = %d, want 15", got.X)
	}
	// Y clamped: 5 + 12 - 10 = 7
	if got.Y != 7 {
		t.Errorf("resize NW Y = %d, want 7", got.Y)
	}
}

func TestApplyDragResizeNEMinSize(t *testing.T) {
	bounds := geometry.Rect{X: 0, Y: 0, Width: 100, Height: 50}
	state := DragState{
		Mode:       DragResizeNE,
		StartMouse: geometry.Point{X: 50, Y: 5},
		StartRect:  geometry.Rect{X: 10, Y: 5, Width: 45, Height: 12},
	}
	// Drag left by 20 (shrink width) and down by 10 (shrink height)
	got := ApplyDrag(state, geometry.Point{X: 30, Y: 15}, bounds)
	if got.Width < MinWindowWidth {
		t.Errorf("resize NE width %d below minimum %d", got.Width, MinWindowWidth)
	}
	if got.Height < MinWindowHeight {
		t.Errorf("resize NE height %d below minimum %d", got.Height, MinWindowHeight)
	}
	// Y clamped: 5 + 12 - 10 = 7
	if got.Y != 7 {
		t.Errorf("resize NE Y = %d, want 7", got.Y)
	}
}

func TestApplyDragResizeSWMinSize(t *testing.T) {
	bounds := geometry.Rect{X: 0, Y: 0, Width: 200, Height: 50}
	state := DragState{
		Mode:       DragResizeSW,
		StartMouse: geometry.Point{X: 10, Y: 20},
		StartRect:  geometry.Rect{X: 10, Y: 5, Width: 45, Height: 12},
	}
	// Drag right by 20 (shrink width) and up by 10 (shrink height)
	got := ApplyDrag(state, geometry.Point{X: 30, Y: 10}, bounds)
	if got.Width < MinWindowWidth {
		t.Errorf("resize SW width %d below minimum %d", got.Width, MinWindowWidth)
	}
	if got.Height < MinWindowHeight {
		t.Errorf("resize SW height %d below minimum %d", got.Height, MinWindowHeight)
	}
	// X clamped: 10 + 45 - 40 = 15
	if got.X != 15 {
		t.Errorf("resize SW X = %d, want 15", got.X)
	}
}

func TestApplyDragResizeSEMinSize(t *testing.T) {
	bounds := geometry.Rect{X: 0, Y: 0, Width: 100, Height: 50}
	state := DragState{
		Mode:       DragResizeSE,
		StartMouse: geometry.Point{X: 40, Y: 20},
		StartRect:  geometry.Rect{X: 10, Y: 5, Width: 45, Height: 12},
	}
	// Drag left and up to shrink both below minimum
	got := ApplyDrag(state, geometry.Point{X: 10, Y: 10}, bounds)
	if got.Width < MinWindowWidth {
		t.Errorf("resize SE width %d below minimum %d", got.Width, MinWindowWidth)
	}
	if got.Height < MinWindowHeight {
		t.Errorf("resize SE height %d below minimum %d", got.Height, MinWindowHeight)
	}
}

func TestHitTestMaximizedContent(t *testing.T) {
	w := NewWindow("w1", "Test", geometry.Rect{X: 0, Y: 1, Width: 120, Height: 38}, nil)
	prev := geometry.Rect{X: 10, Y: 5, Width: 40, Height: 20}
	w.PreMaxRect = &prev // maximized
	w.Resizable = true

	// Click in the content area (below title bar)
	got := HitTest(w, geometry.Point{X: 60, Y: 20}, 3, 3)
	if got != HitContent {
		t.Errorf("maximized content click = %v, want HitContent", got)
	}

	// Click on bottom row -- maximized has no bottom border
	got = HitTest(w, geometry.Point{X: 60, Y: 38}, 3, 3)
	if got != HitContent {
		t.Errorf("maximized bottom row = %v, want HitContent", got)
	}

	// Click on left edge -- maximized has no side borders
	got = HitTest(w, geometry.Point{X: 0, Y: 20}, 3, 3)
	if got != HitContent {
		t.Errorf("maximized left edge = %v, want HitContent", got)
	}

	// Click on right edge -- maximized has no side borders
	got = HitTest(w, geometry.Point{X: 119, Y: 20}, 3, 3)
	if got != HitContent {
		t.Errorf("maximized right edge = %v, want HitContent", got)
	}
}

func TestHitTestMaximizedTitleBarButtons(t *testing.T) {
	w := NewWindow("w1", "Test", geometry.Rect{X: 0, Y: 0, Width: 120, Height: 38}, nil)
	prev := geometry.Rect{X: 10, Y: 5, Width: 40, Height: 20}
	w.PreMaxRect = &prev // maximized
	w.Resizable = true

	// Title bar area
	got := HitTest(w, geometry.Point{X: 10, Y: 0}, 3, 3)
	if got != HitTitleBar {
		t.Errorf("maximized title bar = %v, want HitTitleBar", got)
	}

	// Close button: right edge is lastCol=119, closeStart = 119-3 = 116
	got = HitTest(w, geometry.Point{X: 118, Y: 0}, 3, 3)
	if got != HitCloseButton {
		t.Errorf("maximized close button = %v, want HitCloseButton", got)
	}
}

func TestHitTestMaximizedNoCornersResize(t *testing.T) {
	w := NewWindow("w1", "Test", geometry.Rect{X: 0, Y: 0, Width: 120, Height: 38}, nil)
	prev := geometry.Rect{X: 10, Y: 5, Width: 40, Height: 20}
	w.PreMaxRect = &prev // maximized
	w.Resizable = true

	// Top-left corner on maximized window should NOT be resize
	got := HitTest(w, geometry.Point{X: 0, Y: 0}, 3, 3)
	if got == HitBorderNW {
		t.Error("maximized window top-left should not be HitBorderNW")
	}

	// Top-right corner on maximized window should NOT be resize
	got = HitTest(w, geometry.Point{X: 119, Y: 0}, 3, 3)
	if got == HitBorderNE {
		t.Error("maximized window top-right should not be HitBorderNE")
	}
}

func TestHitTestSnapButtons(t *testing.T) {
	w := NewWindow("w1", "Test", geometry.Rect{X: 0, Y: 0, Width: 60, Height: 20}, nil)
	w.Resizable = true

	// Button layout right-to-left (each 3 chars): [snapL][snapR][max][_][x]
	// lastCol = 59
	// closeStart = 59 - 3 = 56,  close range: (56, 59]
	// minStart = 56 - 3 = 53,    min range: (53, 56]
	// maxStart = 53 - 3 = 50,    max range: (50, 53]
	// snapRStart = 50 - 3 = 47,  snapR range: (47, 50]
	// snapLStart = 47 - 3 = 44,  snapL range: (44, 47]

	got := HitTest(w, geometry.Point{X: 49, Y: 0}, 3, 3)
	if got != HitSnapRightButton {
		t.Errorf("snap right button = %v, want HitSnapRightButton", got)
	}

	got = HitTest(w, geometry.Point{X: 46, Y: 0}, 3, 3)
	if got != HitSnapLeftButton {
		t.Errorf("snap left button = %v, want HitSnapLeftButton", got)
	}
}

func TestHitTestCornerNW(t *testing.T) {
	w := NewWindow("w1", "Test", geometry.Rect{X: 5, Y: 3, Width: 30, Height: 15}, nil)
	w.Resizable = true

	got := HitTest(w, geometry.Point{X: 5, Y: 3}, 3, 3)
	if got != HitBorderNW {
		t.Errorf("NW corner = %v, want HitBorderNW", got)
	}
}

func TestHitTestCornerNE(t *testing.T) {
	w := NewWindow("w1", "Test", geometry.Rect{X: 5, Y: 3, Width: 30, Height: 15}, nil)
	w.Resizable = true
	w.TitleBarHeight = 3 // multi-row: btnRow = 1, so row 0 skips button checks

	// NE corner: localX == lastCol (29), localY == 0
	// With multi-row title bar, buttons are on row 1, so row 0 reaches corner checks.
	got := HitTest(w, geometry.Point{X: 34, Y: 3}, 3, 3)
	if got != HitBorderNE {
		t.Errorf("NE corner = %v, want HitBorderNE", got)
	}
}

func TestHitTestMultiRowTitleBar(t *testing.T) {
	w := NewWindow("w1", "Test", geometry.Rect{X: 0, Y: 0, Width: 60, Height: 20}, nil)
	w.Resizable = true
	w.TitleBarHeight = 3

	// Click on row 1 of a 3-row title bar (not the button row which is row 1 = tbh/2 = 1)
	// btnRow = 3/2 = 1, so row 1 IS the button row
	// Row 0 is first row - check for NW corner
	got := HitTest(w, geometry.Point{X: 0, Y: 0}, 3, 3)
	if got != HitBorderNW {
		t.Errorf("multi-row title NW corner = %v, want HitBorderNW", got)
	}

	// Row 2 of title bar, not the button row, should be HitTitleBar
	got = HitTest(w, geometry.Point{X: 10, Y: 2}, 3, 3)
	if got != HitTitleBar {
		t.Errorf("multi-row title bar row 2 = %v, want HitTitleBar", got)
	}
}

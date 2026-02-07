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
		{"outside", geometry.Point{0, 0}, HitNone},
		{"title bar", geometry.Point{10, 3}, HitTitleBar},
		{"close button", geometry.Point{33, 3}, HitCloseButton},
		{"max button", geometry.Point{30, 3}, HitMaxButton},
		{"content", geometry.Point{15, 10}, HitContent},
		{"left border", geometry.Point{5, 10}, HitBorderW},
		{"right border", geometry.Point{34, 10}, HitBorderE},
		{"bottom border", geometry.Point{15, 17}, HitBorderS},
		{"bottom-left corner", geometry.Point{5, 17}, HitBorderSW},
		{"bottom-right corner", geometry.Point{34, 17}, HitBorderSE},
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
	got := HitTest(w, geometry.Point{24, 0}, 3, 3)
	if got != HitTitleBar {
		t.Errorf("expected HitTitleBar for non-resizable window, got %v", got)
	}
}

func TestHitTestWithTheme(t *testing.T) {
	w := NewWindow("w1", "Test", geometry.Rect{X: 0, Y: 0, Width: 30, Height: 10}, nil)
	got := HitTestWithTheme(w, geometry.Point{15, 5}, "[X]", "[□]")
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
		StartMouse: geometry.Point{15, 10},
		StartRect:  geometry.Rect{X: 10, Y: 5, Width: 30, Height: 15},
	}

	got := ApplyDrag(state, geometry.Point{25, 15}, bounds)
	want := geometry.Rect{X: 20, Y: 10, Width: 30, Height: 15}
	if got != want {
		t.Errorf("move drag = %v, want %v", got, want)
	}
}

func TestApplyDragMoveClamp(t *testing.T) {
	bounds := geometry.Rect{X: 0, Y: 0, Width: 50, Height: 30}
	state := DragState{
		Mode:       DragMove,
		StartMouse: geometry.Point{15, 10},
		StartRect:  geometry.Rect{X: 10, Y: 5, Width: 30, Height: 15},
	}

	// Drag far right, should clamp
	got := ApplyDrag(state, geometry.Point{100, 10}, bounds)
	if got.Right() > bounds.Right() {
		t.Errorf("move should clamp: right=%d > bounds=%d", got.Right(), bounds.Right())
	}
}

func TestApplyDragResizeE(t *testing.T) {
	bounds := geometry.Rect{X: 0, Y: 0, Width: 100, Height: 50}
	state := DragState{
		Mode:       DragResizeE,
		StartMouse: geometry.Point{40, 10},
		StartRect:  geometry.Rect{X: 10, Y: 5, Width: 30, Height: 15},
	}

	got := ApplyDrag(state, geometry.Point{50, 10}, bounds)
	if got.Width != 40 {
		t.Errorf("resize E: width = %d, want 40", got.Width)
	}
}

func TestApplyDragResizeS(t *testing.T) {
	bounds := geometry.Rect{X: 0, Y: 0, Width: 100, Height: 50}
	state := DragState{
		Mode:       DragResizeS,
		StartMouse: geometry.Point{20, 20},
		StartRect:  geometry.Rect{X: 10, Y: 5, Width: 30, Height: 15},
	}

	got := ApplyDrag(state, geometry.Point{20, 25}, bounds)
	if got.Height != 20 {
		t.Errorf("resize S: height = %d, want 20", got.Height)
	}
}

func TestApplyDragResizeSE(t *testing.T) {
	bounds := geometry.Rect{X: 0, Y: 0, Width: 100, Height: 50}
	state := DragState{
		Mode:       DragResizeSE,
		StartMouse: geometry.Point{40, 20},
		StartRect:  geometry.Rect{X: 10, Y: 5, Width: 30, Height: 15},
	}

	got := ApplyDrag(state, geometry.Point{50, 25}, bounds)
	if got.Width != 40 || got.Height != 20 {
		t.Errorf("resize SE: %dx%d, want 40x20", got.Width, got.Height)
	}
}

func TestApplyDragResizeW(t *testing.T) {
	bounds := geometry.Rect{X: 0, Y: 0, Width: 100, Height: 50}
	state := DragState{
		Mode:       DragResizeW,
		StartMouse: geometry.Point{10, 10},
		StartRect:  geometry.Rect{X: 10, Y: 5, Width: 30, Height: 15},
	}

	got := ApplyDrag(state, geometry.Point{5, 10}, bounds)
	if got.X != 5 || got.Width != 35 {
		t.Errorf("resize W: x=%d w=%d, want x=5 w=35", got.X, got.Width)
	}
}

func TestApplyDragResizeN(t *testing.T) {
	bounds := geometry.Rect{X: 0, Y: 0, Width: 100, Height: 50}
	state := DragState{
		Mode:       DragResizeN,
		StartMouse: geometry.Point{20, 5},
		StartRect:  geometry.Rect{X: 10, Y: 5, Width: 30, Height: 15},
	}

	got := ApplyDrag(state, geometry.Point{20, 2}, bounds)
	if got.Y != 2 || got.Height != 18 {
		t.Errorf("resize N: y=%d h=%d, want y=2 h=18", got.Y, got.Height)
	}
}

func TestApplyDragResizeNW(t *testing.T) {
	bounds := geometry.Rect{X: 0, Y: 0, Width: 100, Height: 50}
	state := DragState{
		Mode:       DragResizeNW,
		StartMouse: geometry.Point{10, 5},
		StartRect:  geometry.Rect{X: 10, Y: 5, Width: 30, Height: 15},
	}

	got := ApplyDrag(state, geometry.Point{5, 2}, bounds)
	if got.X != 5 || got.Y != 2 || got.Width != 35 || got.Height != 18 {
		t.Errorf("resize NW: %v, want {5 2 35 18}", got)
	}
}

func TestApplyDragResizeNE(t *testing.T) {
	bounds := geometry.Rect{X: 0, Y: 0, Width: 100, Height: 50}
	state := DragState{
		Mode:       DragResizeNE,
		StartMouse: geometry.Point{40, 5},
		StartRect:  geometry.Rect{X: 10, Y: 5, Width: 30, Height: 15},
	}

	got := ApplyDrag(state, geometry.Point{50, 2}, bounds)
	if got.Width != 40 || got.Y != 2 || got.Height != 18 {
		t.Errorf("resize NE: %v, want w=40 y=2 h=18", got)
	}
}

func TestApplyDragResizeSW(t *testing.T) {
	bounds := geometry.Rect{X: 0, Y: 0, Width: 100, Height: 50}
	state := DragState{
		Mode:       DragResizeSW,
		StartMouse: geometry.Point{10, 20},
		StartRect:  geometry.Rect{X: 10, Y: 5, Width: 30, Height: 15},
	}

	got := ApplyDrag(state, geometry.Point{5, 25}, bounds)
	if got.X != 5 || got.Width != 35 || got.Height != 20 {
		t.Errorf("resize SW: %v, want x=5 w=35 h=20", got)
	}
}

func TestApplyDragMinSize(t *testing.T) {
	bounds := geometry.Rect{X: 0, Y: 0, Width: 100, Height: 50}

	// Resize E to very small
	state := DragState{
		Mode:       DragResizeE,
		StartMouse: geometry.Point{40, 10},
		StartRect:  geometry.Rect{X: 10, Y: 5, Width: 30, Height: 15},
	}
	got := ApplyDrag(state, geometry.Point{5, 10}, bounds)
	if got.Width < MinWindowWidth {
		t.Errorf("width %d below minimum %d", got.Width, MinWindowWidth)
	}

	// Resize S to very small
	state.Mode = DragResizeS
	state.StartMouse = geometry.Point{20, 20}
	got = ApplyDrag(state, geometry.Point{20, 0}, bounds)
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
	got := ApplyDrag(state, geometry.Point{50, 50}, bounds)
	if got != state.StartRect.Clamp(bounds) {
		t.Error("DragNone should not change rect")
	}
}

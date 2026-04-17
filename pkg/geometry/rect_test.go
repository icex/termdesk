package geometry

import "testing"

func TestRectContains(t *testing.T) {
	r := Rect{X: 5, Y: 5, Width: 10, Height: 10}
	if !r.Contains(Point{10, 10}) {
		t.Error("expected center point to be contained")
	}
	if r.Contains(Point{15, 15}) {
		t.Error("expected exclusive edge to not be contained")
	}
	if r.Contains(Point{4, 5}) {
		t.Error("expected point left of rect to not be contained")
	}
}

func TestRectEdges(t *testing.T) {
	r := Rect{X: 5, Y: 10, Width: 20, Height: 15}
	if r.Right() != 25 {
		t.Errorf("Right() = %d, want 25", r.Right())
	}
	if r.Bottom() != 25 {
		t.Errorf("Bottom() = %d, want 25", r.Bottom())
	}
}

func TestRectIsEmpty(t *testing.T) {
	tests := []struct {
		name string
		r    Rect
		want bool
	}{
		{"normal", Rect{0, 0, 10, 10}, false},
		{"zero width", Rect{0, 0, 0, 10}, true},
		{"zero height", Rect{0, 0, 10, 0}, true},
		{"negative width", Rect{0, 0, -1, 10}, true},
		{"negative height", Rect{0, 0, 10, -1}, true},
		{"zero rect", Rect{}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.r.IsEmpty(); got != tt.want {
				t.Errorf("IsEmpty() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestRectIntersect(t *testing.T) {
	tests := []struct {
		name     string
		r, other Rect
		want     Rect
	}{
		{
			"overlapping",
			Rect{0, 0, 10, 10}, Rect{5, 5, 10, 10},
			Rect{5, 5, 5, 5},
		},
		{
			"contained",
			Rect{0, 0, 20, 20}, Rect{5, 5, 5, 5},
			Rect{5, 5, 5, 5},
		},
		{
			"no overlap",
			Rect{0, 0, 5, 5}, Rect{10, 10, 5, 5},
			Rect{},
		},
		{
			"adjacent",
			Rect{0, 0, 5, 5}, Rect{5, 0, 5, 5},
			Rect{},
		},
		{
			"same rect",
			Rect{3, 3, 7, 7}, Rect{3, 3, 7, 7},
			Rect{3, 3, 7, 7},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.r.Intersect(tt.other)
			if got != tt.want {
				t.Errorf("Intersect() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestRectUnion(t *testing.T) {
	tests := []struct {
		name     string
		r, other Rect
		want     Rect
	}{
		{
			"two rects",
			Rect{0, 0, 5, 5}, Rect{10, 10, 5, 5},
			Rect{0, 0, 15, 15},
		},
		{
			"overlapping",
			Rect{0, 0, 10, 10}, Rect{5, 5, 10, 10},
			Rect{0, 0, 15, 15},
		},
		{
			"with empty",
			Rect{5, 5, 10, 10}, Rect{},
			Rect{5, 5, 10, 10},
		},
		{
			"empty with rect",
			Rect{}, Rect{5, 5, 10, 10},
			Rect{5, 5, 10, 10},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.r.Union(tt.other)
			if got != tt.want {
				t.Errorf("Union() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestRectMove(t *testing.T) {
	r := Rect{X: 5, Y: 10, Width: 20, Height: 15}
	got := r.Move(3, -2)
	want := Rect{X: 8, Y: 8, Width: 20, Height: 15}
	if got != want {
		t.Errorf("Move() = %v, want %v", got, want)
	}
}

func TestRectResize(t *testing.T) {
	tests := []struct {
		name   string
		r      Rect
		dw, dh int
		want   Rect
	}{
		{"grow", Rect{0, 0, 10, 10}, 5, 3, Rect{0, 0, 15, 13}},
		{"shrink", Rect{0, 0, 10, 10}, -3, -5, Rect{0, 0, 7, 5}},
		{"clamp negative width", Rect{0, 0, 5, 10}, -10, 0, Rect{0, 0, 0, 10}},
		{"clamp negative height", Rect{0, 0, 10, 3}, 0, -5, Rect{0, 0, 10, 0}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.r.Resize(tt.dw, tt.dh)
			if got != tt.want {
				t.Errorf("Resize() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestRectClamp(t *testing.T) {
	bounds := Rect{X: 0, Y: 0, Width: 100, Height: 50}
	tests := []struct {
		name string
		r    Rect
		want Rect
	}{
		{
			"already inside",
			Rect{10, 10, 20, 15},
			Rect{10, 10, 20, 15},
		},
		{
			"overflow right",
			Rect{90, 10, 20, 15},
			Rect{80, 10, 20, 15},
		},
		{
			"overflow bottom",
			Rect{10, 45, 20, 15},
			Rect{10, 35, 20, 15},
		},
		{
			"negative x",
			Rect{-5, 10, 20, 15},
			Rect{0, 10, 20, 15},
		},
		{
			"negative y",
			Rect{10, -5, 20, 15},
			Rect{10, 0, 20, 15},
		},
		{
			"too wide for bounds",
			Rect{10, 10, 200, 15},
			Rect{0, 10, 100, 15},
		},
		{
			"too tall for bounds",
			Rect{10, 10, 20, 100},
			Rect{10, 0, 20, 50},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.r.Clamp(bounds)
			if got != tt.want {
				t.Errorf("Clamp() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestRectOverlaps(t *testing.T) {
	tests := []struct {
		name     string
		r, other Rect
		want     bool
	}{
		{"overlapping", Rect{0, 0, 10, 10}, Rect{5, 5, 10, 10}, true},
		{"no overlap", Rect{0, 0, 5, 5}, Rect{10, 10, 5, 5}, false},
		{"adjacent", Rect{0, 0, 5, 5}, Rect{5, 0, 5, 5}, false},
		{"contained", Rect{0, 0, 20, 20}, Rect{5, 5, 5, 5}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.r.Overlaps(tt.other)
			if got != tt.want {
				t.Errorf("Overlaps() = %v, want %v", got, tt.want)
			}
		})
	}
}

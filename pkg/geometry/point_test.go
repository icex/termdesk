package geometry

import "testing"

func TestPointAdd(t *testing.T) {
	tests := []struct {
		name     string
		p, other Point
		want     Point
	}{
		{"positive", Point{1, 2}, Point{3, 4}, Point{4, 6}},
		{"negative", Point{5, 5}, Point{-3, -2}, Point{2, 3}},
		{"zero", Point{1, 2}, Point{0, 0}, Point{1, 2}},
		{"origin", Point{0, 0}, Point{0, 0}, Point{0, 0}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.p.Add(tt.other)
			if got != tt.want {
				t.Errorf("Add() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestPointSub(t *testing.T) {
	tests := []struct {
		name     string
		p, other Point
		want     Point
	}{
		{"positive", Point{5, 7}, Point{3, 4}, Point{2, 3}},
		{"negative result", Point{1, 1}, Point{3, 5}, Point{-2, -4}},
		{"zero", Point{3, 3}, Point{3, 3}, Point{0, 0}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.p.Sub(tt.other)
			if got != tt.want {
				t.Errorf("Sub() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestPointIn(t *testing.T) {
	r := Rect{X: 10, Y: 10, Width: 20, Height: 15}
	tests := []struct {
		name string
		p    Point
		want bool
	}{
		{"inside", Point{15, 15}, true},
		{"top-left corner", Point{10, 10}, true},
		{"bottom-right exclusive", Point{30, 25}, false},
		{"just inside right", Point{29, 24}, true},
		{"left of rect", Point{9, 15}, false},
		{"above rect", Point{15, 9}, false},
		{"below rect", Point{15, 25}, false},
		{"right of rect", Point{30, 15}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.p.In(r)
			if got != tt.want {
				t.Errorf("In() = %v, want %v", got, tt.want)
			}
		})
	}
}

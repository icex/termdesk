package geometry

// Rect represents a rectangle defined by its top-left corner and dimensions.
type Rect struct {
	X      int
	Y      int
	Width  int
	Height int
}

// Contains reports whether the point is inside the rectangle.
func (r Rect) Contains(p Point) bool {
	return p.In(r)
}

// Right returns the x coordinate of the right edge (exclusive).
func (r Rect) Right() int {
	return r.X + r.Width
}

// Bottom returns the y coordinate of the bottom edge (exclusive).
func (r Rect) Bottom() int {
	return r.Y + r.Height
}

// IsEmpty reports whether the rectangle has zero or negative area.
func (r Rect) IsEmpty() bool {
	return r.Width <= 0 || r.Height <= 0
}

// Intersect returns the largest rectangle contained by both r and other.
// If they don't overlap, an empty Rect is returned.
func (r Rect) Intersect(other Rect) Rect {
	x0 := max(r.X, other.X)
	y0 := max(r.Y, other.Y)
	x1 := min(r.Right(), other.Right())
	y1 := min(r.Bottom(), other.Bottom())
	if x0 >= x1 || y0 >= y1 {
		return Rect{}
	}
	return Rect{X: x0, Y: y0, Width: x1 - x0, Height: y1 - y0}
}

// Union returns the smallest rectangle that contains both r and other.
func (r Rect) Union(other Rect) Rect {
	if r.IsEmpty() {
		return other
	}
	if other.IsEmpty() {
		return r
	}
	x0 := min(r.X, other.X)
	y0 := min(r.Y, other.Y)
	x1 := max(r.Right(), other.Right())
	y1 := max(r.Bottom(), other.Bottom())
	return Rect{X: x0, Y: y0, Width: x1 - x0, Height: y1 - y0}
}

// Move returns a new Rect shifted by dx, dy.
func (r Rect) Move(dx, dy int) Rect {
	return Rect{X: r.X + dx, Y: r.Y + dy, Width: r.Width, Height: r.Height}
}

// Resize returns a new Rect with dimensions adjusted by dw, dh.
// The resulting dimensions are clamped to be non-negative.
func (r Rect) Resize(dw, dh int) Rect {
	w := r.Width + dw
	h := r.Height + dh
	if w < 0 {
		w = 0
	}
	if h < 0 {
		h = 0
	}
	return Rect{X: r.X, Y: r.Y, Width: w, Height: h}
}

// Clamp returns a new Rect moved to fit entirely within bounds.
// If r is larger than bounds in any dimension, it is resized to fit.
func (r Rect) Clamp(bounds Rect) Rect {
	result := r

	// Clamp size to bounds
	if result.Width > bounds.Width {
		result.Width = bounds.Width
	}
	if result.Height > bounds.Height {
		result.Height = bounds.Height
	}

	// Clamp position
	if result.X < bounds.X {
		result.X = bounds.X
	}
	if result.Y < bounds.Y {
		result.Y = bounds.Y
	}
	if result.Right() > bounds.Right() {
		result.X = bounds.Right() - result.Width
	}
	if result.Bottom() > bounds.Bottom() {
		result.Y = bounds.Bottom() - result.Height
	}

	return result
}

// Overlaps reports whether r and other share any area.
func (r Rect) Overlaps(other Rect) bool {
	return !r.Intersect(other).IsEmpty()
}

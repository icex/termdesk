package geometry

// Point represents a 2D position in terminal coordinates.
type Point struct {
	X int
	Y int
}

// Add returns a new Point offset by other.
func (p Point) Add(other Point) Point {
	return Point{X: p.X + other.X, Y: p.Y + other.Y}
}

// Sub returns the vector from other to p.
func (p Point) Sub(other Point) Point {
	return Point{X: p.X - other.X, Y: p.Y - other.Y}
}

// In reports whether the point is inside the given rectangle.
func (p Point) In(r Rect) bool {
	return p.X >= r.X && p.X < r.X+r.Width &&
		p.Y >= r.Y && p.Y < r.Y+r.Height
}

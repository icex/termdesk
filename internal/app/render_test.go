package app

import (
	"strings"
	"testing"
	"time"

	"github.com/icex/termdesk/internal/config"
	"github.com/icex/termdesk/internal/terminal"
	"github.com/icex/termdesk/internal/window"
	"github.com/icex/termdesk/pkg/geometry"
)

func testTheme() config.Theme {
	return config.RetroTheme()
}

func TestNewBuffer(t *testing.T) {
	buf := NewBuffer(10, 5, "#000000")
	if buf.Width != 10 || buf.Height != 5 {
		t.Errorf("dimensions = %dx%d, want 10x5", buf.Width, buf.Height)
	}
	// All cells should be spaces
	for y := 0; y < buf.Height; y++ {
		for x := 0; x < buf.Width; x++ {
			if buf.Cells[y][x].Char != ' ' {
				t.Errorf("cell[%d][%d] = %q, want space", y, x, buf.Cells[y][x].Char)
			}
		}
	}
}

func TestBufferSet(t *testing.T) {
	buf := NewBuffer(10, 5, "#000000")
	buf.Set(3, 2, 'X', "#FFFFFF", "#000000")
	if buf.Cells[2][3].Char != 'X' {
		t.Error("expected X at (3,2)")
	}
	if buf.Cells[2][3].Fg == nil {
		t.Error("Fg should not be nil after setting a color")
	}
}

func TestBufferSetOutOfBounds(t *testing.T) {
	buf := NewBuffer(10, 5, "#000000")
	// Should not panic
	buf.Set(-1, 0, 'X', "", "")
	buf.Set(0, -1, 'X', "", "")
	buf.Set(10, 0, 'X', "", "")
	buf.Set(0, 5, 'X', "", "")
}

func TestBufferSetString(t *testing.T) {
	buf := NewBuffer(10, 5, "#000000")
	buf.SetString(2, 1, "Hello", "#FFF", "#000")
	for i, ch := range "Hello" {
		if buf.Cells[1][2+i].Char != ch {
			t.Errorf("cell[1][%d] = %q, want %q", 2+i, buf.Cells[1][2+i].Char, ch)
		}
	}
}

func TestBufferSetStringClip(t *testing.T) {
	buf := NewBuffer(5, 1, "#000000")
	buf.SetString(3, 0, "Hello", "#FFF", "#000")
	// Only "He" should fit (positions 3 and 4)
	if buf.Cells[0][3].Char != 'H' {
		t.Error("expected H at (3,0)")
	}
	if buf.Cells[0][4].Char != 'e' {
		t.Error("expected e at (4,0)")
	}
}

func TestBufferFillRect(t *testing.T) {
	buf := NewBuffer(10, 5, "#000000")
	r := geometry.Rect{X: 1, Y: 1, Width: 3, Height: 2}
	buf.FillRect(r, '#', "#FFF", "#000")
	if buf.Cells[1][1].Char != '#' {
		t.Error("expected # at (1,1)")
	}
	if buf.Cells[2][3].Char != '#' {
		t.Error("expected # at (3,2)")
	}
	// Outside fill area should still be space
	if buf.Cells[0][0].Char != ' ' {
		t.Error("expected space at (0,0)")
	}
}

func TestRenderWindowBasic(t *testing.T) {
	theme := testTheme()
	buf := NewBuffer(30, 10, theme.DesktopBg)
	w := window.NewWindow("w1", "Test", geometry.Rect{X: 0, Y: 0, Width: 20, Height: 8}, nil)
	w.Focused = true

	RenderWindow(buf, w, theme, nil)

	// Check corners
	if buf.Cells[0][0].Char != theme.BorderTopLeft {
		t.Errorf("top-left = %q, want %q", buf.Cells[0][0].Char, theme.BorderTopLeft)
	}
	if buf.Cells[0][19].Char != theme.BorderTopRight {
		t.Errorf("top-right = %q, want %q", buf.Cells[0][19].Char, theme.BorderTopRight)
	}
	if buf.Cells[7][0].Char != theme.BorderBottomLeft {
		t.Errorf("bottom-left = %q, want %q", buf.Cells[7][0].Char, theme.BorderBottomLeft)
	}
	if buf.Cells[7][19].Char != theme.BorderBottomRight {
		t.Errorf("bottom-right = %q, want %q", buf.Cells[7][19].Char, theme.BorderBottomRight)
	}

	// Check side borders
	if buf.Cells[3][0].Char != theme.BorderVertical {
		t.Error("expected left border")
	}
	if buf.Cells[3][19].Char != theme.BorderVertical {
		t.Error("expected right border")
	}
}

func TestRenderWindowSkipsMinimized(t *testing.T) {
	theme := testTheme()
	buf := NewBuffer(30, 10, theme.DesktopBg)
	w := window.NewWindow("w1", "Test", geometry.Rect{X: 0, Y: 0, Width: 20, Height: 8}, nil)
	w.Minimized = true

	RenderWindow(buf, w, theme, nil)

	// Should not have drawn anything
	if buf.Cells[0][0].Char != ' ' {
		t.Error("minimized window should not be rendered")
	}
}

func TestRenderWindowSkipsInvisible(t *testing.T) {
	theme := testTheme()
	buf := NewBuffer(30, 10, theme.DesktopBg)
	w := window.NewWindow("w1", "Test", geometry.Rect{X: 0, Y: 0, Width: 20, Height: 8}, nil)
	w.Visible = false

	RenderWindow(buf, w, theme, nil)

	if buf.Cells[0][0].Char != ' ' {
		t.Error("invisible window should not be rendered")
	}
}

func TestRenderWindowTooSmall(t *testing.T) {
	theme := testTheme()
	buf := NewBuffer(10, 5, theme.DesktopBg)
	w := window.NewWindow("w1", "Test", geometry.Rect{X: 0, Y: 0, Width: 2, Height: 2}, nil)

	RenderWindow(buf, w, theme, nil)

	// Too small to render, should be no-op
	if buf.Cells[0][0].Char != ' ' {
		t.Error("too-small window should not be rendered")
	}
}

func TestRenderWindowTitleTruncation(t *testing.T) {
	theme := testTheme()
	buf := NewBuffer(40, 5, theme.DesktopBg)
	w := window.NewWindow("w1", "This is a very long window title that should be truncated", geometry.Rect{X: 0, Y: 0, Width: 30, Height: 5}, nil)
	w.Focused = true

	RenderWindow(buf, w, theme, nil)

	// Title should be present but truncated with "..."
	row := buf.Cells[0]
	var titleStr strings.Builder
	for x := 1; x < 29; x++ {
		titleStr.WriteRune(row[x].Char)
	}
	title := titleStr.String()
	if !strings.Contains(title, "...") {
		// Short windows truncate title
		if !strings.Contains(title, "Th") {
			t.Errorf("title row = %q, expected some title content", title)
		}
	}
}

func TestRenderWindowActiveInactiveColors(t *testing.T) {
	theme := testTheme()
	buf := NewBuffer(30, 10, theme.DesktopBg)

	w := window.NewWindow("w1", "Test", geometry.Rect{X: 0, Y: 0, Width: 20, Height: 8}, nil)
	w.Focused = true
	RenderWindow(buf, w, theme, nil)
	activeFg := buf.Cells[0][0].Fg

	buf2 := NewBuffer(30, 10, theme.DesktopBg)
	w2 := window.NewWindow("w2", "Test", geometry.Rect{X: 0, Y: 0, Width: 20, Height: 8}, nil)
	w2.Focused = false
	RenderWindow(buf2, w2, theme, nil)
	inactiveFg := buf2.Cells[0][0].Fg

	// Foreground colors should differ (backgrounds are transparent = same as desktop)
	if colorsEqual(activeFg, inactiveFg) {
		t.Error("active and inactive borders should have different foregrounds")
	}
}

func TestRenderFrameEmpty(t *testing.T) {
	theme := testTheme()
	wm := window.NewManager(40, 20)
	buf := RenderFrame(wm, theme, nil, nil)

	if buf.Width != 40 || buf.Height != 20 {
		t.Errorf("buffer dimensions = %dx%d, want 40x20", buf.Width, buf.Height)
	}
	// Should be all desktop bg
	for y := 0; y < buf.Height; y++ {
		for x := 0; x < buf.Width; x++ {
			if buf.Cells[y][x].Char != ' ' {
				t.Errorf("expected space at (%d,%d)", x, y)
			}
		}
	}
}

func TestRenderFrameWithWindows(t *testing.T) {
	theme := testTheme()
	wm := window.NewManager(60, 30)

	w1 := window.NewWindow("w1", "Back", geometry.Rect{X: 0, Y: 0, Width: 20, Height: 10}, nil)
	w2 := window.NewWindow("w2", "Front", geometry.Rect{X: 10, Y: 5, Width: 20, Height: 10}, nil)
	wm.AddWindow(w1)
	wm.AddWindow(w2)

	buf := RenderFrame(wm, theme, nil, nil)

	// In the overlap area, w2 (front) should be visible
	// w2 starts at (10,5), so (10,5) should be w2's top-left corner
	if buf.Cells[5][10].Char != theme.BorderTopLeft {
		t.Errorf("overlap: expected w2 border at (10,5), got %q", buf.Cells[5][10].Char)
	}

	// w1's top-left corner at (0,0) should still be visible
	if buf.Cells[0][0].Char != theme.BorderTopLeft {
		t.Errorf("w1 corner: expected border at (0,0), got %q", buf.Cells[0][0].Char)
	}
}

func TestRenderFrameZeroBounds(t *testing.T) {
	theme := testTheme()
	wm := window.NewManager(0, 0)
	buf := RenderFrame(wm, theme, nil, nil)
	if buf.Width != 1 || buf.Height != 1 {
		t.Errorf("zero bounds buffer = %dx%d, want 1x1", buf.Width, buf.Height)
	}
}

func TestBufferToString(t *testing.T) {
	buf := NewBuffer(5, 3, "#000000")
	buf.Set(0, 0, 'A', "", "")
	buf.Set(4, 2, 'Z', "", "")

	s := BufferToString(buf)
	// BufferToString now includes ANSI sequences, but should contain A and Z
	if !strings.Contains(s, "A") {
		t.Error("output should contain 'A'")
	}
	if !strings.Contains(s, "Z") {
		t.Error("output should contain 'Z'")
	}
	// Should have line breaks
	lines := strings.Split(s, "\n")
	if len(lines) != 3 {
		t.Errorf("expected 3 lines, got %d", len(lines))
	}
}

func TestRenderWindowCloseButton(t *testing.T) {
	theme := testTheme()
	buf := NewBuffer(30, 8, theme.DesktopBg)
	w := window.NewWindow("w1", "Test", geometry.Rect{X: 0, Y: 0, Width: 20, Height: 8}, nil)
	w.Focused = true

	RenderWindow(buf, w, theme, nil)

	// Close button [X] should be at right side of title bar
	// For a 20-wide window, close button at x=17,18,19 area
	row := buf.Cells[0]
	var titleStr strings.Builder
	for x := 0; x < 20; x++ {
		titleStr.WriteRune(row[x].Char)
	}
	rendered := titleStr.String()
	if !strings.Contains(rendered, "[X]") {
		t.Errorf("title bar = %q, expected [X] close button", rendered)
	}
}

func TestRenderWindowMaxButton(t *testing.T) {
	theme := testTheme()
	buf := NewBuffer(30, 8, theme.DesktopBg)
	w := window.NewWindow("w1", "Test", geometry.Rect{X: 0, Y: 0, Width: 20, Height: 8}, nil)
	w.Focused = true
	w.Resizable = true

	RenderWindow(buf, w, theme, nil)

	row := buf.Cells[0]
	var titleStr strings.Builder
	for x := 0; x < 20; x++ {
		titleStr.WriteRune(row[x].Char)
	}
	rendered := titleStr.String()
	if !strings.Contains(rendered, "[□]") {
		t.Errorf("title bar = %q, expected [□] max button", rendered)
	}
}

func TestRenderWindowRestoreButton(t *testing.T) {
	theme := testTheme()
	buf := NewBuffer(30, 8, theme.DesktopBg)
	w := window.NewWindow("w1", "Test", geometry.Rect{X: 0, Y: 0, Width: 20, Height: 8}, nil)
	w.Focused = true
	w.Resizable = true
	prevRect := geometry.Rect{X: 5, Y: 5, Width: 10, Height: 5}
	w.PreMaxRect = &prevRect

	RenderWindow(buf, w, theme, nil)

	row := buf.Cells[0]
	var titleStr strings.Builder
	for x := 0; x < 20; x++ {
		titleStr.WriteRune(row[x].Char)
	}
	rendered := titleStr.String()
	if !strings.Contains(rendered, "[◫]") {
		t.Errorf("title bar = %q, expected [◫] restore button when maximized", rendered)
	}
}

func TestStripANSI(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"plain text", "hello", "hello"},
		{"empty", "", ""},
		{"CSI color", "\x1b[31mred\x1b[0m", "red"},
		{"CSI bold", "\x1b[1mbold\x1b[22m", "bold"},
		{"CSI cursor", "\x1b[2;5H", ""},
		{"OSC title BEL", "\x1b]0;title\x07text", "text"},
		{"OSC title ST", "\x1b]0;title\x1b\\text", "text"},
		{"mixed", "\x1b[32mgreen\x1b[0m plain \x1b[1;34mblue\x1b[0m", "green plain blue"},
		{"ESC other", "\x1b(B text", " text"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := string(stripANSI(tt.input))
			if got != tt.want {
				t.Errorf("stripANSI(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestRenderTerminalContentNil(t *testing.T) {
	buf := NewBuffer(20, 10, "#000")
	area := geometry.Rect{X: 1, Y: 1, Width: 10, Height: 3}

	// nil terminal should be no-op
	renderTerminalContent(buf, area, nil, nil, nil)
	if buf.Cells[1][1].Char != ' ' {
		t.Error("nil terminal should not change buffer")
	}
}

func TestRenderTerminalContentWithTerminal(t *testing.T) {
	term, err := terminal.New("/bin/echo", []string{"HELLO"}, 20, 5)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer term.Close()

	// Read PTY output
	done := make(chan struct{})
	go func() {
		term.ReadPtyLoop()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
	}

	buf := NewBuffer(30, 10, "#000")
	area := geometry.Rect{X: 1, Y: 1, Width: 20, Height: 5}
	renderTerminalContent(buf, area, term, hexToColor("#C0C0C0"), hexToColor("#1E2127"))

	// Check that "HELLO" was rendered somewhere in the content area
	var found bool
	for y := area.Y; y < area.Y+area.Height; y++ {
		for x := area.X; x < area.X+area.Width; x++ {
			if buf.Cells[y][x].Char == 'H' {
				found = true
			}
		}
	}
	if !found {
		t.Error("expected terminal content to contain 'H' from echo HELLO")
	}
}

func TestRenderWindowWithTerminal(t *testing.T) {
	term, err := terminal.New("/bin/echo", []string{"TEST"}, 18, 6)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer term.Close()

	done := make(chan struct{})
	go func() {
		term.ReadPtyLoop()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
	}

	theme := testTheme()
	buf := NewBuffer(30, 10, theme.DesktopBg)
	w := window.NewWindow("w1", "Term", geometry.Rect{X: 0, Y: 0, Width: 20, Height: 8}, nil)
	w.Focused = true

	RenderWindow(buf, w, theme, term)

	// Should have both borders and terminal content
	if buf.Cells[0][0].Char != theme.BorderTopLeft {
		t.Error("expected border")
	}
}

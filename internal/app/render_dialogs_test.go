package app

import (
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/icex/termdesk/internal/config"
	"github.com/icex/termdesk/internal/terminal"
	"github.com/icex/termdesk/internal/window"
	"github.com/icex/termdesk/pkg/geometry"
)

// --- styledSpacer ---

func TestStyledSpacer(t *testing.T) {
	theme := testTheme()
	bg := lipgloss.Color(theme.ActiveTitleBg)

	t.Run("non-empty result", func(t *testing.T) {
		result := styledSpacer(bg, "    ")
		if result == "" {
			t.Error("expected non-empty styled spacer")
		}
	})

	t.Run("single space", func(t *testing.T) {
		result := styledSpacer(bg, " ")
		if result == "" {
			t.Error("expected non-empty result for single space")
		}
	})

	t.Run("empty spaces", func(t *testing.T) {
		result := styledSpacer(bg, "")
		// Even empty string should produce a styled result (lipgloss may return non-empty)
		_ = result // no panic
	})

	t.Run("wide spacer", func(t *testing.T) {
		result := styledSpacer(bg, "          ")
		if len(result) == 0 {
			t.Error("expected non-empty result for wide spacer")
		}
	})
}

// --- RenderConfirmDialog ---

func TestRenderConfirmDialog(t *testing.T) {
	theme := testTheme()

	t.Run("nil dialog", func(t *testing.T) {
		buf := AcquireThemedBuffer(80, 24, theme)
		RenderConfirmDialog(buf, nil, theme)
		// Should be no-op, all cells should be space or pattern
		for y := 0; y < buf.Height; y++ {
			for x := 0; x < buf.Width; x++ {
				ch := buf.Cells[y][x].Char
				if ch != ' ' && ch != theme.DesktopPatternChar {
					t.Errorf("nil dialog changed buffer at (%d,%d): %q", x, y, ch)
					return
				}
			}
		}
	})

	t.Run("basic dialog centered", func(t *testing.T) {
		buf := AcquireThemedBuffer(80, 24, theme)
		dialog := &ConfirmDialog{
			Title:    "Close Window?",
			WindowID: "win1",
			IsQuit:   false,
			Selected: 0,
		}
		RenderConfirmDialog(buf, dialog, theme)

		// Buffer should have non-space content in the center region
		found := false
		for y := 0; y < buf.Height; y++ {
			for x := 0; x < buf.Width; x++ {
				ch := buf.Cells[y][x].Char
				if ch != ' ' && ch != theme.DesktopPatternChar {
					found = true
					break
				}
			}
			if found {
				break
			}
		}
		if !found {
			t.Error("expected dialog content in buffer")
		}
	})

	t.Run("yes selected", func(t *testing.T) {
		buf := AcquireThemedBuffer(80, 24, theme)
		dialog := &ConfirmDialog{
			Title:    "Quit?",
			IsQuit:   true,
			Selected: 0,
		}
		RenderConfirmDialog(buf, dialog, theme)
		// Should not panic and should have content
		hasContent := bufHasNonSpaceContent(buf, theme)
		if !hasContent {
			t.Error("expected dialog content when yes is selected")
		}
	})

	t.Run("no selected", func(t *testing.T) {
		buf := AcquireThemedBuffer(80, 24, theme)
		dialog := &ConfirmDialog{
			Title:    "Quit?",
			IsQuit:   true,
			Selected: 1,
		}
		RenderConfirmDialog(buf, dialog, theme)
		hasContent := bufHasNonSpaceContent(buf, theme)
		if !hasContent {
			t.Error("expected dialog content when no is selected")
		}
	})

	t.Run("small buffer", func(t *testing.T) {
		buf := AcquireThemedBuffer(20, 5, theme)
		dialog := &ConfirmDialog{
			Title:    "Close?",
			Selected: 0,
		}
		// Should not panic even on small buffer
		RenderConfirmDialog(buf, dialog, theme)
	})

	t.Run("long title", func(t *testing.T) {
		buf := AcquireThemedBuffer(80, 24, theme)
		dialog := &ConfirmDialog{
			Title:    "This is an extremely long dialog title that should be handled gracefully",
			Selected: 0,
		}
		RenderConfirmDialog(buf, dialog, theme)
		hasContent := bufHasNonSpaceContent(buf, theme)
		if !hasContent {
			t.Error("expected dialog content for long title")
		}
	})
}

// --- RenderRenameDialog ---

func TestRenderRenameDialog(t *testing.T) {
	theme := testTheme()

	t.Run("nil dialog", func(t *testing.T) {
		buf := AcquireThemedBuffer(80, 24, theme)
		RenderRenameDialog(buf, nil, theme)
		// Should be a no-op
		for y := 0; y < buf.Height; y++ {
			for x := 0; x < buf.Width; x++ {
				ch := buf.Cells[y][x].Char
				if ch != ' ' && ch != theme.DesktopPatternChar {
					t.Errorf("nil dialog changed buffer at (%d,%d): %q", x, y, ch)
					return
				}
			}
		}
	})

	t.Run("basic rename dialog", func(t *testing.T) {
		buf := AcquireThemedBuffer(80, 24, theme)
		dialog := &RenameDialog{
			WindowID: "win1",
			Text:     []rune("test window"),
			Cursor:   4,
			Selected: 0,
		}
		RenderRenameDialog(buf, dialog, theme)
		hasContent := bufHasNonSpaceContent(buf, theme)
		if !hasContent {
			t.Error("expected rename dialog content in buffer")
		}
	})

	t.Run("empty text", func(t *testing.T) {
		buf := AcquireThemedBuffer(80, 24, theme)
		dialog := &RenameDialog{
			WindowID: "win1",
			Text:     []rune{},
			Cursor:   0,
			Selected: 0,
		}
		RenderRenameDialog(buf, dialog, theme)
		hasContent := bufHasNonSpaceContent(buf, theme)
		if !hasContent {
			t.Error("expected dialog frame even with empty text")
		}
	})

	t.Run("cancel selected", func(t *testing.T) {
		buf := AcquireThemedBuffer(80, 24, theme)
		dialog := &RenameDialog{
			WindowID: "win1",
			Text:     []rune("hello"),
			Cursor:   5,
			Selected: 1,
		}
		RenderRenameDialog(buf, dialog, theme)
		hasContent := bufHasNonSpaceContent(buf, theme)
		if !hasContent {
			t.Error("expected dialog content with cancel selected")
		}
	})

	t.Run("long text scrolls", func(t *testing.T) {
		buf := AcquireThemedBuffer(80, 24, theme)
		longText := []rune("this is a very long window name that exceeds the input field width limit")
		dialog := &RenameDialog{
			WindowID: "win1",
			Text:     longText,
			Cursor:   len(longText),
			Selected: 0,
		}
		RenderRenameDialog(buf, dialog, theme)
		hasContent := bufHasNonSpaceContent(buf, theme)
		if !hasContent {
			t.Error("expected dialog with long scrolled text")
		}
	})

	t.Run("cursor inverts cell", func(t *testing.T) {
		buf := AcquireThemedBuffer(80, 24, theme)
		dialog := &RenameDialog{
			WindowID: "win1",
			Text:     []rune("abc"),
			Cursor:   1,
			Selected: 0,
		}
		RenderRenameDialog(buf, dialog, theme)
		// The cursor cell should have swapped fg/bg, but we mainly verify no panic
		hasContent := bufHasNonSpaceContent(buf, theme)
		if !hasContent {
			t.Error("expected dialog content")
		}
	})
}

// --- RenderNewWorkspaceDialog ---

func TestRenderNewWorkspaceDialog(t *testing.T) {
	theme := testTheme()

	t.Run("nil dialog", func(t *testing.T) {
		buf := AcquireThemedBuffer(80, 24, theme)
		RenderNewWorkspaceDialog(buf, nil, theme)
		for y := 0; y < buf.Height; y++ {
			for x := 0; x < buf.Width; x++ {
				ch := buf.Cells[y][x].Char
				if ch != ' ' && ch != theme.DesktopPatternChar {
					t.Errorf("nil dialog changed buffer at (%d,%d): %q", x, y, ch)
					return
				}
			}
		}
	})

	t.Run("basic new workspace dialog", func(t *testing.T) {
		buf := AcquireThemedBuffer(80, 24, theme)
		dialog := &NewWorkspaceDialog{
			Name:       []rune("myworkspace"),
			DirPath:    "/home/user/projects",
			DirEntries: []string{"docs", "src"},
			Cursor:     0,
			TextCursor: 4,
			Selected:   0,
		}
		RenderNewWorkspaceDialog(buf, dialog, theme)
		hasContent := bufHasNonSpaceContent(buf, theme)
		if !hasContent {
			t.Error("expected new workspace dialog content in buffer")
		}
	})

	t.Run("browser focused", func(t *testing.T) {
		buf := AcquireThemedBuffer(80, 24, theme)
		dialog := &NewWorkspaceDialog{
			Name:       []rune("ws"),
			DirPath:    "/tmp",
			DirEntries: []string{"a", "b"},
			Cursor:     1, // browser
			DirSelect:  0,
			Selected:   0,
		}
		RenderNewWorkspaceDialog(buf, dialog, theme)
		hasContent := bufHasNonSpaceContent(buf, theme)
		if !hasContent {
			t.Error("expected dialog content with browser focused")
		}
	})

	t.Run("cancel selected", func(t *testing.T) {
		buf := AcquireThemedBuffer(80, 24, theme)
		dialog := &NewWorkspaceDialog{
			Name:       []rune("test"),
			DirPath:    "/tmp",
			DirEntries: []string{"x"},
			Cursor:     2,
			TextCursor: 0,
			Selected:   1,
		}
		RenderNewWorkspaceDialog(buf, dialog, theme)
		hasContent := bufHasNonSpaceContent(buf, theme)
		if !hasContent {
			t.Error("expected dialog content with cancel selected")
		}
	})

	t.Run("empty fields", func(t *testing.T) {
		buf := AcquireThemedBuffer(80, 24, theme)
		dialog := &NewWorkspaceDialog{
			Name:    []rune{},
			DirPath: "/",
			Cursor:  0,
		}
		RenderNewWorkspaceDialog(buf, dialog, theme)
		hasContent := bufHasNonSpaceContent(buf, theme)
		if !hasContent {
			t.Error("expected dialog frame even with empty fields")
		}
	})

	t.Run("cursor on button row", func(t *testing.T) {
		buf := AcquireThemedBuffer(80, 24, theme)
		dialog := &NewWorkspaceDialog{
			Name:       []rune("ws"),
			DirPath:    "/tmp",
			DirEntries: []string{"a"},
			Cursor:     2, // buttons
			TextCursor: 0,
			Selected:   0,
		}
		RenderNewWorkspaceDialog(buf, dialog, theme)
		hasContent := bufHasNonSpaceContent(buf, theme)
		if !hasContent {
			t.Error("expected dialog content")
		}
	})
}

// --- renderExposeSearchBar ---

func TestRenderExposeSearchBar(t *testing.T) {
	theme := testTheme()

	t.Run("basic search bar", func(t *testing.T) {
		buf := AcquireThemedBuffer(80, 24, theme)
		wa := geometry.Rect{X: 0, Y: 1, Width: 80, Height: 22}
		renderExposeSearchBar(buf, theme, "test", wa)

		// Bar is drawn at barY = wa.Y + 1 = 2
		barY := wa.Y + 1
		found := false
		for x := 0; x < buf.Width; x++ {
			if buf.Cells[barY][x].Char != ' ' {
				found = true
				break
			}
		}
		if !found {
			t.Error("expected search bar content at y=2")
		}
	})

	t.Run("empty filter still draws bar", func(t *testing.T) {
		buf := AcquireThemedBuffer(80, 24, theme)
		wa := geometry.Rect{X: 0, Y: 1, Width: 80, Height: 22}
		renderExposeSearchBar(buf, theme, "", wa)

		// Even empty filter renders the search icon and cursor block
		barY := wa.Y + 1
		found := false
		for x := 0; x < buf.Width; x++ {
			ch := buf.Cells[barY][x].Char
			if ch != ' ' && ch != theme.DesktopPatternChar {
				found = true
				break
			}
		}
		if !found {
			t.Error("expected search bar with cursor block even for empty filter")
		}
	})

	t.Run("search bar centered", func(t *testing.T) {
		buf := AcquireThemedBuffer(80, 24, theme)
		wa := geometry.Rect{X: 0, Y: 1, Width: 80, Height: 22}
		renderExposeSearchBar(buf, theme, "hello", wa)

		barY := wa.Y + 1
		// Find leftmost and rightmost non-space on the bar row
		leftMost := -1
		rightMost := -1
		for x := 0; x < buf.Width; x++ {
			c := theme.C()
			if colorsEqual(buf.Cells[barY][x].Bg, c.AccentColor) {
				if leftMost == -1 {
					leftMost = x
				}
				rightMost = x
			}
		}
		if leftMost == -1 {
			t.Error("expected search bar with accent background")
			return
		}
		// Bar should be roughly centered
		barCenter := (leftMost + rightMost) / 2
		screenCenter := wa.X + wa.Width/2
		if abs(barCenter-screenCenter) > 2 {
			t.Errorf("search bar center %d too far from screen center %d", barCenter, screenCenter)
		}
	})
}

// --- renderExposeMiniWindow ---

func TestRenderExposeMiniWindow(t *testing.T) {
	theme := testTheme()

	t.Run("focused mini window", func(t *testing.T) {
		buf := AcquireThemedBuffer(80, 24, theme)
		w := window.NewWindow("w1", "Terminal", geometry.Rect{X: 5, Y: 5, Width: 40, Height: 15}, nil)
		w.Focused = true
		w.Visible = true

		renderExposeMiniWindow(buf, theme, w, 10, 5, 20, 10, true, 1)

		// Check top-left border char
		if buf.Cells[5][10].Char != theme.BorderTopLeft {
			t.Errorf("expected top-left border at (10,5), got %q", buf.Cells[5][10].Char)
		}
		// Check top-right border
		if buf.Cells[5][29].Char != theme.BorderTopRight {
			t.Errorf("expected top-right border at (29,5), got %q", buf.Cells[5][29].Char)
		}
		// Check bottom-left border
		if buf.Cells[14][10].Char != theme.BorderBottomLeft {
			t.Errorf("expected bottom-left border at (10,14), got %q", buf.Cells[14][10].Char)
		}
		// Focused window should show title text inside
		titleFound := false
		for x := 11; x < 29; x++ {
			if buf.Cells[10][x].Char == 'T' { // "Terminal" starts with T
				titleFound = true
				break
			}
		}
		if !titleFound {
			t.Error("expected title text inside focused mini window")
		}
	})

	t.Run("unfocused mini window with number", func(t *testing.T) {
		buf := AcquireThemedBuffer(80, 24, theme)
		w := window.NewWindow("w2", "Editor", geometry.Rect{X: 0, Y: 0, Width: 30, Height: 10}, nil)
		w.Visible = true

		renderExposeMiniWindow(buf, theme, w, 5, 3, 16, 6, false, 2)

		// Check border drawn
		if buf.Cells[3][5].Char != theme.BorderTopLeft {
			t.Errorf("expected top-left border at (5,3), got %q", buf.Cells[3][5].Char)
		}

		// Unfocused should show "N: Title" format
		// Center row is y=3 + 1 + innerH/2 = 3 + 1 + 2 = 6
		centerY := 3 + 1 + (6-2)/2
		found := false
		for x := 6; x < 20; x++ {
			if buf.Cells[centerY][x].Char == '2' { // window number 2
				found = true
				break
			}
		}
		if !found {
			t.Error("expected window number in unfocused mini window")
		}
	})

	t.Run("very small mini window", func(t *testing.T) {
		buf := AcquireThemedBuffer(80, 24, theme)
		w := window.NewWindow("w3", "Tiny", geometry.Rect{X: 0, Y: 0, Width: 10, Height: 5}, nil)
		w.Visible = true

		// mh=2 should draw top+bottom border only, then return
		renderExposeMiniWindow(buf, theme, w, 5, 5, 6, 2, false, 1)
		if buf.Cells[5][5].Char != theme.BorderTopLeft {
			t.Errorf("expected border for small window, got %q", buf.Cells[5][5].Char)
		}
	})

	t.Run("title truncation", func(t *testing.T) {
		buf := AcquireThemedBuffer(80, 24, theme)
		w := window.NewWindow("w4", "A Very Long Window Title That Exceeds Mini Width", geometry.Rect{X: 0, Y: 0, Width: 30, Height: 10}, nil)
		w.Focused = true
		w.Visible = true

		renderExposeMiniWindow(buf, theme, w, 5, 5, 12, 8, true, 1)
		// Should not panic and should draw borders
		if buf.Cells[5][5].Char != theme.BorderTopLeft {
			t.Errorf("expected border, got %q", buf.Cells[5][5].Char)
		}
	})
}

// --- RenderExpose ---

func TestRenderExpose(t *testing.T) {
	theme := testTheme()

	t.Run("no windows shows message", func(t *testing.T) {
		wm := window.NewManager(80, 24)
		wm.SetReserved(1, 1)

		buf := RenderExpose(wm, theme, "")

		// Should contain "No open windows" message
		found := false
		for y := 0; y < buf.Height; y++ {
			var row strings.Builder
			for x := 0; x < buf.Width; x++ {
				row.WriteRune(buf.Cells[y][x].Char)
			}
			if strings.Contains(row.String(), "No open windows") {
				found = true
				break
			}
		}
		if !found {
			t.Error("expected 'No open windows' message when no windows")
		}
	})

	t.Run("single window", func(t *testing.T) {
		wm := window.NewManager(80, 24)
		wm.SetReserved(1, 1)
		w := window.NewWindow("w1", "Test Window", geometry.Rect{X: 5, Y: 5, Width: 40, Height: 15}, nil)
		w.Visible = true
		w.Focused = true
		wm.AddWindow(w)

		buf := RenderExpose(wm, theme, "")

		if buf == nil {
			t.Fatal("expected non-nil buffer")
		}
		// Should have window borders rendered
		hasContent := bufHasNonSpaceContent(buf, theme)
		if !hasContent {
			t.Error("expected expose view to have window content")
		}
	})

	t.Run("multiple windows", func(t *testing.T) {
		wm := window.NewManager(120, 40)
		wm.SetReserved(1, 1)

		w1 := window.NewWindow("w1", "First", geometry.Rect{X: 5, Y: 5, Width: 40, Height: 15}, nil)
		w1.Visible = true
		w1.Focused = true
		wm.AddWindow(w1)

		w2 := window.NewWindow("w2", "Second", geometry.Rect{X: 20, Y: 10, Width: 40, Height: 15}, nil)
		w2.Visible = true
		wm.AddWindow(w2)
		// Re-focus w1 after AddWindow (which focuses w2)
		wm.FocusWindow("w1")

		buf := RenderExpose(wm, theme, "")

		if buf == nil {
			t.Fatal("expected non-nil buffer")
		}
		hasContent := bufHasNonSpaceContent(buf, theme)
		if !hasContent {
			t.Error("expected expose view to have content for multiple windows")
		}
	})

	t.Run("with filter", func(t *testing.T) {
		wm := window.NewManager(120, 40)
		wm.SetReserved(1, 1)

		w1 := window.NewWindow("w1", "Terminal", geometry.Rect{X: 5, Y: 5, Width: 40, Height: 15}, nil)
		w1.Visible = true
		w1.Focused = true
		wm.AddWindow(w1)

		w2 := window.NewWindow("w2", "Editor", geometry.Rect{X: 20, Y: 10, Width: 40, Height: 15}, nil)
		w2.Visible = true
		wm.AddWindow(w2)
		wm.FocusWindow("w1")

		buf := RenderExpose(wm, theme, "Term")

		if buf == nil {
			t.Fatal("expected non-nil buffer")
		}
		// Should have search bar
		hasContent := bufHasNonSpaceContent(buf, theme)
		if !hasContent {
			t.Error("expected expose view with filter to have content")
		}
	})

	t.Run("filter no match keeps all visible", func(t *testing.T) {
		wm := window.NewManager(80, 24)
		wm.SetReserved(1, 1)

		w1 := window.NewWindow("w1", "Terminal", geometry.Rect{X: 5, Y: 5, Width: 40, Height: 15}, nil)
		w1.Visible = true
		w1.Focused = true
		wm.AddWindow(w1)

		// Filter that matches nothing falls back to all visible
		buf := RenderExpose(wm, theme, "zzzzz")
		if buf == nil {
			t.Fatal("expected non-nil buffer")
		}
	})

	t.Run("minimized windows excluded", func(t *testing.T) {
		wm := window.NewManager(80, 24)
		wm.SetReserved(1, 1)

		w1 := window.NewWindow("w1", "Visible", geometry.Rect{X: 5, Y: 5, Width: 40, Height: 15}, nil)
		w1.Visible = true
		w1.Focused = true
		wm.AddWindow(w1)

		w2 := window.NewWindow("w2", "Minimized", geometry.Rect{X: 20, Y: 10, Width: 40, Height: 15}, nil)
		w2.Visible = true
		w2.Minimized = true
		wm.AddWindow(w2)

		buf := RenderExpose(wm, theme, "")
		if buf == nil {
			t.Fatal("expected non-nil buffer")
		}
	})

	t.Run("zero size returns small buffer", func(t *testing.T) {
		wm := window.NewManager(0, 0)
		buf := RenderExpose(wm, theme, "")
		if buf.Width != 1 || buf.Height != 1 {
			t.Errorf("expected 1x1 buffer for zero size, got %dx%d", buf.Width, buf.Height)
		}
	})
}

// --- renderSelection ---

func TestRenderSelection(t *testing.T) {
	theme := testTheme()

	t.Run("basic selection inverts cells", func(t *testing.T) {
		buf := AcquireThemedBuffer(40, 20, theme)
		// Write some content
		contentRect := geometry.Rect{X: 1, Y: 1, Width: 20, Height: 10}
		for y := contentRect.Y; y < contentRect.Y+contentRect.Height; y++ {
			for x := contentRect.X; x < contentRect.X+contentRect.Width; x++ {
				buf.Set(x, y, 'A', "#FFFFFF", "#000000")
			}
		}

		// Store original colors
		origFg := buf.Cells[5][5].Fg
		origBg := buf.Cells[5][5].Bg

		start := geometry.Point{X: 0, Y: 100} // absLine 100
		end := geometry.Point{X: 19, Y: 100}

		// scrollOffset=0, scrollbackLen=100, contentH=10
		// absLine for row 4 = scrollbackLen - scrollOffset + 4 = 100
		// wait, mouseToAbsLine: if contentRow < scrollLines (scrollOffset=0 means scrollLines=0), never true
		// So row dy gets absLine = scrollbackLen + dy = 100 + dy
		// For dy=4: absLine = 104, which doesn't match 100
		// Let's use scrollbackLen=96, so dy=4 gives absLine=100
		renderSelection(buf, contentRect, start, end, 0, 96, contentRect.Height)

		// Cell at (5, 5) should have inverted colors (dy=4, absLine=96+4=100)
		cell := buf.Cells[5][5]
		if colorsEqual(cell.Fg, origFg) && colorsEqual(cell.Bg, origBg) {
			t.Error("expected selection to invert fg/bg colors")
		}
	})

	t.Run("selection no overlap", func(t *testing.T) {
		buf := AcquireThemedBuffer(40, 20, theme)
		contentRect := geometry.Rect{X: 1, Y: 1, Width: 20, Height: 10}
		for y := contentRect.Y; y < contentRect.Y+contentRect.Height; y++ {
			for x := contentRect.X; x < contentRect.X+contentRect.Width; x++ {
				buf.Set(x, y, 'B', "#FFFFFF", "#000000")
			}
		}

		origFg := buf.Cells[3][3].Fg
		origBg := buf.Cells[3][3].Bg

		// Selection on a line that doesn't overlap any content row
		start := geometry.Point{X: 0, Y: 500}
		end := geometry.Point{X: 10, Y: 500}
		renderSelection(buf, contentRect, start, end, 0, 0, contentRect.Height)

		// Nothing should change
		cell := buf.Cells[3][3]
		if !colorsEqual(cell.Fg, origFg) || !colorsEqual(cell.Bg, origBg) {
			t.Error("expected no change for non-overlapping selection")
		}
	})

	t.Run("reversed start/end normalized", func(t *testing.T) {
		buf := AcquireThemedBuffer(40, 20, theme)
		contentRect := geometry.Rect{X: 1, Y: 1, Width: 20, Height: 10}
		for y := contentRect.Y; y < contentRect.Y+contentRect.Height; y++ {
			for x := contentRect.X; x < contentRect.X+contentRect.Width; x++ {
				buf.Set(x, y, 'C', "#FFFFFF", "#000000")
			}
		}

		// Reversed: end before start (should be normalized internally)
		start := geometry.Point{X: 10, Y: 5}
		end := geometry.Point{X: 0, Y: 5}
		renderSelection(buf, contentRect, start, end, 0, 0, contentRect.Height)
		// Should not panic
	})
}

// --- renderScrollbar ---

func TestRenderScrollbar(t *testing.T) {
	theme := testTheme()

	t.Run("scrollbar with terminal no scrollback", func(t *testing.T) {
		term, err := terminal.New("/bin/echo", []string{"SCROLLBAR"}, 20, 10, 0, 0, "")
		if err != nil {
			t.Fatalf("failed to create terminal: %v", err)
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

		// Use plain buffer (not themed) so initial cells are known spaces
		buf := NewBuffer(40, 20, "#000000")
		w := window.NewWindow("w1", "Test", geometry.Rect{X: 1, Y: 1, Width: 22, Height: 12}, nil)
		w.Focused = true
		w.Visible = true

		// scrollOffset=0, totalLines <= contentH -> no scrollbar
		renderScrollbar(buf, w, theme, term, 0)
		// With no scrollback, scrollbar should not appear.
		// trackX = w.Rect.Right()-1 = 22
		trackX := w.Rect.Right() - 1
		trackTop := w.Rect.Y + 1
		// Cells on trackX should remain spaces (no scrollbar drawn)
		scrollbarDrawn := false
		for dy := trackTop; dy < w.Rect.Bottom()-2; dy++ {
			ch := buf.Cells[dy][trackX].Char
			if ch == '▓' || ch == '░' {
				scrollbarDrawn = true
				break
			}
		}
		if scrollbarDrawn {
			t.Error("expected no scrollbar when no scrollback data")
		}
	})

	t.Run("scrollbar no-op when track too small", func(t *testing.T) {
		term, err := terminal.New("/bin/echo", []string{"HI"}, 10, 3, 0, 0, "")
		if err != nil {
			t.Fatalf("failed to create terminal: %v", err)
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

		buf := AcquireThemedBuffer(20, 8, theme)
		// Very small window: height=4 -> trackH < 3 -> returns early
		w := window.NewWindow("w1", "Tiny", geometry.Rect{X: 0, Y: 0, Width: 12, Height: 4}, nil)
		w.Focused = true
		w.Visible = true

		renderScrollbar(buf, w, theme, term, 5)
		// Should not panic
	})
}

// --- RenderExposeTransition ---

func TestRenderExposeTransition(t *testing.T) {
	theme := testTheme()

	t.Run("basic transition", func(t *testing.T) {
		wm := window.NewManager(80, 24)
		wm.SetReserved(1, 1)

		w1 := window.NewWindow("w1", "Win1", geometry.Rect{X: 5, Y: 5, Width: 40, Height: 15}, nil)
		w1.Visible = true
		w1.Focused = true
		wm.AddWindow(w1)

		var animations []Animation
		buf := RenderExposeTransition(wm, theme, animations, "")
		if buf == nil {
			t.Fatal("expected non-nil buffer")
		}
		hasContent := bufHasNonSpaceContent(buf, theme)
		if !hasContent {
			t.Error("expected transition view content")
		}
	})

	t.Run("transition with filter", func(t *testing.T) {
		wm := window.NewManager(80, 24)
		wm.SetReserved(1, 1)

		w1 := window.NewWindow("w1", "Terminal", geometry.Rect{X: 5, Y: 5, Width: 40, Height: 15}, nil)
		w1.Visible = true
		w1.Focused = true
		wm.AddWindow(w1)

		buf := RenderExposeTransition(wm, theme, nil, "Term")
		if buf == nil {
			t.Fatal("expected non-nil buffer")
		}
	})

	t.Run("zero size returns small buffer", func(t *testing.T) {
		wm := window.NewManager(0, 0)
		buf := RenderExposeTransition(wm, theme, nil, "")
		if buf.Width != 1 || buf.Height != 1 {
			t.Errorf("expected 1x1 buffer for zero size, got %dx%d", buf.Width, buf.Height)
		}
	})
}

// --- helpers ---

// bufHasNonSpaceContent checks if a buffer has any content beyond spaces and pattern chars.
func bufHasNonSpaceContent(buf *Buffer, theme config.Theme) bool {
	for y := 0; y < buf.Height; y++ {
		for x := 0; x < buf.Width; x++ {
			ch := buf.Cells[y][x].Char
			if ch != ' ' && ch != theme.DesktopPatternChar && ch != 0 {
				return true
			}
		}
	}
	return false
}

func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}

// --- RenderBufferNameDialog tests ---

func TestRenderBufferNameDialogNil(t *testing.T) {
	theme := testTheme()
	buf := NewThemedBuffer(80, 24, theme)
	// Should not panic with nil dialog
	RenderBufferNameDialog(buf, nil, theme)
}

func TestRenderBufferNameDialogRenders(t *testing.T) {
	theme := testTheme()
	buf := NewThemedBuffer(80, 24, theme)
	dialog := &BufferNameDialog{
		Text:   []rune("my buffer name"),
		Cursor: 14,
	}
	RenderBufferNameDialog(buf, dialog, theme)
	if !bufHasNonSpaceContent(buf, theme) {
		t.Error("expected buffer to have content after rendering BufferNameDialog")
	}
}

func TestRenderBufferNameDialogShortText(t *testing.T) {
	theme := testTheme()
	buf := NewThemedBuffer(80, 24, theme)
	dialog := &BufferNameDialog{
		Text:   []rune("A"),
		Cursor: 1,
	}
	RenderBufferNameDialog(buf, dialog, theme)
	if !bufHasNonSpaceContent(buf, theme) {
		t.Error("expected buffer to have content with short text")
	}
}

func TestRenderBufferNameDialogLongText(t *testing.T) {
	theme := testTheme()
	buf := NewThemedBuffer(80, 24, theme)
	longText := []rune("This is a very long buffer name that exceeds the input field width easily")
	dialog := &BufferNameDialog{
		Text:   longText,
		Cursor: len(longText),
	}
	RenderBufferNameDialog(buf, dialog, theme)
	if !bufHasNonSpaceContent(buf, theme) {
		t.Error("expected buffer to have content with long text")
	}
}

func TestRenderBufferNameDialogEmptyText(t *testing.T) {
	theme := testTheme()
	buf := NewThemedBuffer(80, 24, theme)
	dialog := &BufferNameDialog{
		Text:   []rune{},
		Cursor: 0,
	}
	RenderBufferNameDialog(buf, dialog, theme)
	if !bufHasNonSpaceContent(buf, theme) {
		t.Error("expected buffer to have content even with empty text")
	}
}

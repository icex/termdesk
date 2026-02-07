package clipboard

import (
	"strings"
	"testing"
)

func TestNew(t *testing.T) {
	c := New()
	if c.Visible {
		t.Error("should not be visible initially")
	}
	if c.Len() != 0 {
		t.Error("should be empty initially")
	}
}

func TestCopyPaste(t *testing.T) {
	c := New()
	c.Copy("hello")
	if c.Paste() != "hello" {
		t.Errorf("paste = %q, want 'hello'", c.Paste())
	}

	c.Copy("world")
	if c.Paste() != "world" {
		t.Errorf("paste = %q, want 'world'", c.Paste())
	}
}

func TestCopyEmpty(t *testing.T) {
	c := New()
	c.Copy("")
	if c.Len() != 0 {
		t.Error("empty string should not be stored")
	}
}

func TestPasteEmpty(t *testing.T) {
	c := New()
	if c.Paste() != "" {
		t.Errorf("paste on empty = %q, want empty", c.Paste())
	}
}

func TestHistoryCapacity(t *testing.T) {
	c := New()
	for i := 0; i < 10; i++ {
		c.Copy(string(rune('a' + i)))
	}
	if c.Len() != historyCapacity {
		t.Errorf("len = %d, want %d", c.Len(), historyCapacity)
	}
	// Most recent should be the last added
	if c.Paste() != "j" {
		t.Errorf("paste = %q, want 'j'", c.Paste())
	}
}

func TestShowHideHistory(t *testing.T) {
	c := New()
	c.ShowHistory()
	if !c.Visible {
		t.Error("should be visible after ShowHistory")
	}
	if c.SelectedIdx != 0 {
		t.Error("selection should reset to 0")
	}

	c.HideHistory()
	if c.Visible {
		t.Error("should not be visible after HideHistory")
	}
}

func TestToggleHistory(t *testing.T) {
	c := New()
	c.ToggleHistory()
	if !c.Visible {
		t.Error("first toggle should show")
	}
	c.ToggleHistory()
	if c.Visible {
		t.Error("second toggle should hide")
	}
}

func TestMoveSelection(t *testing.T) {
	c := New()
	c.Copy("a")
	c.Copy("b")
	c.Copy("c")
	c.ShowHistory()

	c.MoveSelection(1)
	if c.SelectedIdx != 1 {
		t.Errorf("selection = %d, want 1", c.SelectedIdx)
	}

	c.MoveSelection(1)
	if c.SelectedIdx != 2 {
		t.Errorf("selection = %d, want 2", c.SelectedIdx)
	}

	// Wrap forward
	c.MoveSelection(1)
	if c.SelectedIdx != 0 {
		t.Errorf("selection = %d, want 0 (wrapped)", c.SelectedIdx)
	}

	// Wrap backward
	c.MoveSelection(-1)
	if c.SelectedIdx != 2 {
		t.Errorf("selection = %d, want 2 (wrapped back)", c.SelectedIdx)
	}
}

func TestMoveSelectionEmpty(t *testing.T) {
	c := New()
	c.ShowHistory()
	c.MoveSelection(1) // should not panic
	if c.SelectedIdx != 0 {
		t.Error("selection should stay 0 on empty")
	}
}

func TestPasteSelected(t *testing.T) {
	c := New()
	c.Copy("first")
	c.Copy("second")
	c.Copy("third")
	c.ShowHistory()

	// Index 0 = newest = "third"
	if c.PasteSelected() != "third" {
		t.Errorf("selected = %q, want 'third'", c.PasteSelected())
	}

	c.MoveSelection(1)
	if c.PasteSelected() != "second" {
		t.Errorf("selected = %q, want 'second'", c.PasteSelected())
	}

	c.MoveSelection(1)
	if c.PasteSelected() != "first" {
		t.Errorf("selected = %q, want 'first'", c.PasteSelected())
	}
}

func TestPasteSelectedEmpty(t *testing.T) {
	c := New()
	c.ShowHistory()
	if c.PasteSelected() != "" {
		t.Error("selected on empty should be empty string")
	}
}

func TestRender(t *testing.T) {
	c := New()
	c.Copy("hello")
	c.Copy("world")
	c.ShowHistory()

	lines := c.Render(80, 24)
	if len(lines) == 0 {
		t.Fatal("expected lines")
	}

	allText := strings.Join(lines, "\n")
	if !strings.Contains(allText, "┌") {
		t.Error("expected top border")
	}
	if !strings.Contains(allText, "└") {
		t.Error("expected bottom border")
	}
	if !strings.Contains(allText, "Clipboard History") {
		t.Error("expected title")
	}
	if !strings.Contains(allText, ">") {
		t.Error("expected selection indicator")
	}
	if !strings.Contains(allText, "world") {
		t.Error("expected newest entry 'world'")
	}
	if !strings.Contains(allText, "hello") {
		t.Error("expected older entry 'hello'")
	}
}

func TestRenderEmpty(t *testing.T) {
	c := New()
	c.ShowHistory()

	lines := c.Render(80, 24)
	allText := strings.Join(lines, "\n")
	if !strings.Contains(allText, "No clipboard entries") {
		t.Error("expected 'No clipboard entries' message")
	}
}

func TestRenderNewlinePreview(t *testing.T) {
	c := New()
	c.Copy("line1\nline2\nline3")
	c.ShowHistory()

	lines := c.Render(80, 24)
	allText := strings.Join(lines, "\n")
	if !strings.Contains(allText, "↵") {
		t.Error("expected newline replacement with ↵")
	}
}

func TestRenderNarrow(t *testing.T) {
	c := New()
	c.Copy("some text")
	c.ShowHistory()
	lines := c.Render(25, 10)
	if len(lines) == 0 {
		t.Error("expected lines even when narrow")
	}
}

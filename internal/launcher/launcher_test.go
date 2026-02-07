package launcher

import (
	"strings"
	"testing"
)

func TestNew(t *testing.T) {
	l := New()
	if len(l.Registry) == 0 {
		t.Error("expected default apps")
	}
	if l.Visible {
		t.Error("should not be visible initially")
	}
}

func TestShowHide(t *testing.T) {
	l := New()

	l.Show()
	if !l.Visible {
		t.Error("should be visible after Show")
	}
	if l.Query != "" {
		t.Error("query should be empty after Show")
	}

	l.Hide()
	if l.Visible {
		t.Error("should not be visible after Hide")
	}
}

func TestToggle(t *testing.T) {
	l := New()

	l.Toggle()
	if !l.Visible {
		t.Error("first toggle should show")
	}

	l.Toggle()
	if l.Visible {
		t.Error("second toggle should hide")
	}
}

func TestTypeChar(t *testing.T) {
	l := New()
	l.Show()

	l.TypeChar('t')
	l.TypeChar('e')
	if l.Query != "te" {
		t.Errorf("query = %q, want 'te'", l.Query)
	}
}

func TestBackspace(t *testing.T) {
	l := New()
	l.Show()

	l.TypeChar('a')
	l.TypeChar('b')
	l.Backspace()
	if l.Query != "a" {
		t.Errorf("query = %q, want 'a'", l.Query)
	}

	l.Backspace()
	if l.Query != "" {
		t.Errorf("query = %q, want empty", l.Query)
	}

	// Backspace on empty should be no-op
	l.Backspace()
	if l.Query != "" {
		t.Error("backspace on empty should be no-op")
	}
}

func TestSetQuery(t *testing.T) {
	l := New()
	l.Show()

	l.SetQuery("term")
	if l.Query != "term" {
		t.Errorf("query = %q, want 'term'", l.Query)
	}
	if l.SelectedIdx != 0 {
		t.Error("selection should reset on query change")
	}
}

func TestFuzzyFilter(t *testing.T) {
	l := New()
	l.Show()

	l.SetQuery("term")
	if len(l.Results) == 0 {
		t.Error("expected results for 'term'")
	}
	// Terminal should be first (prefix match)
	if l.Results[0].Name != "Terminal" {
		t.Errorf("first result = %q, want 'Terminal'", l.Results[0].Name)
	}
}

func TestFuzzyFilterNoResults(t *testing.T) {
	l := New()
	l.Show()

	l.SetQuery("zzzzzzzzz")
	if len(l.Results) != 0 {
		t.Errorf("expected 0 results, got %d", len(l.Results))
	}
}

func TestMoveSelection(t *testing.T) {
	l := New()
	l.Show()
	l.SetQuery("") // show all

	l.MoveSelection(1)
	if l.SelectedIdx != 1 {
		t.Errorf("selection = %d, want 1", l.SelectedIdx)
	}

	// Wrap around
	for i := 0; i < len(l.Results)+1; i++ {
		l.MoveSelection(1)
	}
	// Should have wrapped
	if l.SelectedIdx < 0 || l.SelectedIdx >= len(l.Results) {
		t.Errorf("selection out of bounds: %d", l.SelectedIdx)
	}

	// Move up from 0
	l.SelectedIdx = 0
	l.MoveSelection(-1)
	if l.SelectedIdx != len(l.Results)-1 {
		t.Errorf("selection = %d, want last", l.SelectedIdx)
	}
}

func TestMoveSelectionEmpty(t *testing.T) {
	l := New()
	l.Show()
	l.SetQuery("zzzzz")
	l.MoveSelection(1) // should not panic
}

func TestSelectedEntry(t *testing.T) {
	l := New()
	l.Show()

	entry := l.SelectedEntry()
	if entry == nil {
		t.Error("expected selected entry")
	}
}

func TestSelectedEntryEmpty(t *testing.T) {
	l := New()
	l.Show()
	l.SetQuery("zzzzz")

	entry := l.SelectedEntry()
	if entry != nil {
		t.Error("expected nil entry when no results")
	}
}

func TestRender(t *testing.T) {
	l := New()
	l.Show()

	lines := l.Render(80, 24)
	if len(lines) == 0 {
		t.Fatal("expected lines")
	}

	// Should have borders
	allText := strings.Join(lines, "\n")
	if !strings.Contains(allText, "┌") {
		t.Error("expected top border")
	}
	if !strings.Contains(allText, "└") {
		t.Error("expected bottom border")
	}
	if !strings.Contains(allText, ">") {
		t.Error("expected prompt or selection indicator")
	}
}

func TestRenderNoResults(t *testing.T) {
	l := New()
	l.Show()
	l.SetQuery("zzzzz")

	lines := l.Render(80, 24)
	allText := strings.Join(lines, "\n")
	if !strings.Contains(allText, "No results") {
		t.Error("expected 'No results' message")
	}
}

func TestRenderNarrow(t *testing.T) {
	l := New()
	l.Show()
	// Very narrow width
	lines := l.Render(25, 10)
	if len(lines) == 0 {
		t.Error("expected lines even when narrow")
	}
}

func TestDefaultApps(t *testing.T) {
	apps := defaultApps()
	if len(apps) == 0 {
		t.Error("expected default apps")
	}
}

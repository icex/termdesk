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

func TestGetHistory(t *testing.T) {
	c := New()
	got := c.GetHistory()
	if len(got) != 0 {
		t.Errorf("GetHistory on empty: len = %d, want 0", len(got))
	}

	c.Copy("alpha")
	c.Copy("beta")
	c.Copy("gamma")
	got = c.GetHistory()
	if len(got) != 3 {
		t.Fatalf("GetHistory: len = %d, want 3", len(got))
	}
	if got[0] != "gamma" {
		t.Errorf("GetHistory[0] = %q, want 'gamma'", got[0])
	}
	if got[2] != "alpha" {
		t.Errorf("GetHistory[2] = %q, want 'alpha'", got[2])
	}
}

func TestRestoreHistory(t *testing.T) {
	c := New()
	c.Copy("old")
	c.RestoreHistory([]string{"first", "second", "third"})
	if c.Len() != 3 {
		t.Fatalf("Len after restore = %d, want 3", c.Len())
	}
	if c.SelectedIdx != 0 {
		t.Errorf("SelectedIdx after restore = %d, want 0", c.SelectedIdx)
	}
}

func TestRestoreHistoryEmpty(t *testing.T) {
	c := New()
	c.Copy("something")
	c.RestoreHistory([]string{})
	if c.Len() != 0 {
		t.Errorf("Len after empty restore = %d, want 0", c.Len())
	}
	if c.Paste() != "" {
		t.Errorf("Paste after empty restore = %q, want empty", c.Paste())
	}
}

func TestDeleteItem(t *testing.T) {
	c := New()
	c.Copy("a")
	c.Copy("b")
	c.Copy("c")
	c.DeleteItem(1) // delete middle "b"
	if c.Len() != 2 {
		t.Fatalf("Len after delete = %d, want 2", c.Len())
	}
	got := c.GetHistory()
	if got[0] != "c" {
		t.Errorf("after delete [0] = %q, want 'c'", got[0])
	}
	if got[1] != "a" {
		t.Errorf("after delete [1] = %q, want 'a'", got[1])
	}
}

func TestDeleteItemOutOfBounds(t *testing.T) {
	c := New()
	c.Copy("a")
	c.Copy("b")
	c.DeleteItem(-1)
	if c.Len() != 2 {
		t.Errorf("Len after negative delete = %d, want 2", c.Len())
	}
	c.DeleteItem(5)
	if c.Len() != 2 {
		t.Errorf("Len after over-index delete = %d, want 2", c.Len())
	}
}

func TestDeleteItemOnlyEntry(t *testing.T) {
	c := New()
	c.Copy("only")
	c.DeleteItem(0)
	if c.Len() != 0 {
		t.Errorf("Len after deleting only item = %d, want 0", c.Len())
	}
	if c.Paste() != "" {
		t.Errorf("Paste after deleting only item = %q, want empty", c.Paste())
	}
}

func TestSetSelectedNameAndRender(t *testing.T) {
	c := New()
	c.Copy("first")
	c.Copy("second")
	c.ShowHistory()
	c.SetSelectedName(0, "build-log")

	entries := c.HistoryEntries()
	if entries[0].Name != "build-log" {
		t.Fatalf("entry[0] name = %q, want build-log", entries[0].Name)
	}

	lines := c.Render(80, 24)
	all := strings.Join(lines, "\n")
	if !strings.Contains(all, "[build-log]") {
		t.Fatalf("render should include named prefix, got: %q", all)
	}
}

func TestAppend(t *testing.T) {
	c := New()
	c.Copy("hello")
	c.Append(" world")
	if c.Paste() != "hello world" {
		t.Errorf("Paste after append = %q, want 'hello world'", c.Paste())
	}
	if c.Len() != 1 {
		t.Errorf("Len after append = %d, want 1 (same entry)", c.Len())
	}
}

func TestAppendEmpty(t *testing.T) {
	c := New()
	c.Append("text")
	if c.Paste() != "text" {
		t.Errorf("Paste after append on empty = %q, want 'text'", c.Paste())
	}
}

func TestAppendEmptyString(t *testing.T) {
	c := New()
	c.Copy("data")
	c.Append("")
	if c.Paste() != "data" {
		t.Errorf("Paste after empty append = %q, want 'data'", c.Paste())
	}
}

func TestSetSelectedNameTrimmedAndClear(t *testing.T) {
	c := New()
	c.Copy("value")
	c.ShowHistory()

	c.SetSelectedName(0, "  temp  ")
	if got := c.SelectedName(); got != "temp" {
		t.Fatalf("SelectedName = %q, want temp", got)
	}

	c.SetSelectedName(0, "")
	if got := c.SelectedName(); got != "" {
		t.Fatalf("SelectedName after clear = %q, want empty", got)
	}
}

func TestAutoNameSelected(t *testing.T) {
	c := New()
	c.Copy("alpha")
	c.Copy("beta")
	c.Copy("gamma")
	c.ShowHistory()

	// AutoName the selected (index 0 = newest = "gamma")
	c.AutoNameSelected(0)
	entries := c.HistoryEntries()
	if entries[0].Name != "buf-1" {
		t.Errorf("AutoNameSelected first call: got %q, want buf-1", entries[0].Name)
	}

	// AutoName another entry — should increment
	c.AutoNameSelected(1)
	entries = c.HistoryEntries()
	if entries[1].Name != "buf-2" {
		t.Errorf("AutoNameSelected second call: got %q, want buf-2", entries[1].Name)
	}

	// Third call should be buf-3
	c.AutoNameSelected(2)
	entries = c.HistoryEntries()
	if entries[2].Name != "buf-3" {
		t.Errorf("AutoNameSelected third call: got %q, want buf-3", entries[2].Name)
	}
}

func TestItoa(t *testing.T) {
	tests := []struct {
		input int
		want  string
	}{
		{0, "0"},
		{1, "1"},
		{9, "9"},
		{10, "10"},
		{42, "42"},
		{123, "123"},
		{999999, "999999"},
	}
	for _, tc := range tests {
		got := itoa(tc.input)
		if got != tc.want {
			t.Errorf("itoa(%d) = %q, want %q", tc.input, got, tc.want)
		}
	}
}

func TestSelectedNameEmpty(t *testing.T) {
	c := New()
	// Empty clipboard — SelectedName should return ""
	if got := c.SelectedName(); got != "" {
		t.Errorf("SelectedName on empty clipboard = %q, want empty", got)
	}
}

func TestSelectedNameOutOfBounds(t *testing.T) {
	c := New()
	c.Copy("only")
	c.ShowHistory()
	// Force SelectedIdx out of bounds
	c.SelectedIdx = 5
	if got := c.SelectedName(); got != "" {
		t.Errorf("SelectedName with OOB index = %q, want empty", got)
	}
	c.SelectedIdx = -1
	if got := c.SelectedName(); got != "" {
		t.Errorf("SelectedName with negative index = %q, want empty", got)
	}
}

func TestSetSelectedNameOutOfBounds(t *testing.T) {
	c := New()
	c.Copy("value")
	// Should not panic on OOB index
	c.SetSelectedName(5, "name")
	c.SetSelectedName(-1, "name")
	// Entry should remain unnamed
	entries := c.HistoryEntries()
	if entries[0].Name != "" {
		t.Errorf("entry name should be empty after OOB SetSelectedName, got %q", entries[0].Name)
	}
}

func TestRenderNamedEntries(t *testing.T) {
	c := New()
	c.Copy("alpha")
	c.Copy("beta")
	c.ShowHistory()
	c.AutoNameSelected(0)

	lines := c.Render(80, 24)
	all := strings.Join(lines, "\n")
	if !strings.Contains(all, "[buf-") {
		t.Error("render should include auto-generated name prefix")
	}
}

package launcher

import (
	"testing"

	"github.com/icex/termdesk/internal/apps/registry"
)

func testRegistry() []registry.RegistryEntry {
	return []registry.RegistryEntry{
		{Name: "Terminal", Icon: "\uf120", Command: "$SHELL"},
		{Name: "nvim", Icon: "\ue62b", Command: "nvim"},
		{Name: "Files", Icon: "\uf07b", Command: "mc"},
	}
}

func disableExecIndex(l *Launcher) {
	l.ExecIndex = nil
	l.execLoaded = true
}

func TestNew(t *testing.T) {
	l := New(testRegistry())
	if len(l.Registry) == 0 {
		t.Error("expected default apps")
	}
	if l.Visible {
		t.Error("should not be visible initially")
	}
}

func TestShowHide(t *testing.T) {
	l := New(testRegistry())

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
	l := New(testRegistry())

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
	l := New(testRegistry())
	l.Show()

	l.TypeChar('t')
	l.TypeChar('e')
	if l.Query != "te" {
		t.Errorf("query = %q, want 'te'", l.Query)
	}
}

func TestBackspace(t *testing.T) {
	l := New(testRegistry())
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
	l := New(testRegistry())
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
	l := New(testRegistry())
	disableExecIndex(l)
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
	l := New(testRegistry())
	disableExecIndex(l)
	l.Show()

	l.SetQuery("zzzzzzzzz")
	if len(l.Results) != 0 {
		t.Errorf("expected 0 results, got %d", len(l.Results))
	}
}

func TestMoveSelection(t *testing.T) {
	l := New(testRegistry())
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
	l := New(testRegistry())
	l.Show()
	l.SetQuery("zzzzz")
	l.MoveSelection(1) // should not panic
}

func TestSelectedEntry(t *testing.T) {
	l := New(testRegistry())
	l.Show()

	entry := l.SelectedEntry()
	if entry == nil {
		t.Error("expected selected entry")
	}
}

func TestSelectedEntryEmpty(t *testing.T) {
	l := New(testRegistry())
	l.Show()
	l.SetQuery("zzzzz")

	entry := l.SelectedEntry()
	if entry != nil {
		t.Error("expected nil entry when no results")
	}
}




// --- RecordLaunch tests ---

func TestRecordLaunchBasic(t *testing.T) {
	l := New(testRegistry())
	l.RecordLaunch("nvim")
	if len(l.RecentApps) != 1 {
		t.Fatalf("expected 1 recent, got %d", len(l.RecentApps))
	}
	if l.RecentApps[0] != "nvim" {
		t.Errorf("recent[0] = %q, want 'nvim'", l.RecentApps[0])
	}
}

func TestRecordLaunchDeduplicate(t *testing.T) {
	l := New(testRegistry())
	l.RecordLaunch("nvim")
	l.RecordLaunch("mc")
	l.RecordLaunch("nvim") // should move nvim to front, not duplicate

	if len(l.RecentApps) != 2 {
		t.Fatalf("expected 2 recents, got %d", len(l.RecentApps))
	}
	if l.RecentApps[0] != "nvim" {
		t.Errorf("recent[0] = %q, want 'nvim'", l.RecentApps[0])
	}
	if l.RecentApps[1] != "mc" {
		t.Errorf("recent[1] = %q, want 'mc'", l.RecentApps[1])
	}
}

func TestRecordLaunchCapAtFive(t *testing.T) {
	l := New(testRegistry())
	for i := 0; i < 7; i++ {
		l.RecordLaunch(string(rune('a' + i)))
	}
	if len(l.RecentApps) != 5 {
		t.Fatalf("expected 5 recents (capped), got %d", len(l.RecentApps))
	}
	// Most recent should be at front
	if l.RecentApps[0] != "g" {
		t.Errorf("recent[0] = %q, want 'g'", l.RecentApps[0])
	}
	// Oldest kept should be 'c' (a and b evicted)
	if l.RecentApps[4] != "c" {
		t.Errorf("recent[4] = %q, want 'c'", l.RecentApps[4])
	}
}

// --- ToggleFavorite tests ---

func TestToggleFavoriteAdd(t *testing.T) {
	l := New(testRegistry())
	l.Show()
	// Select first entry (Terminal)
	l.SelectedIdx = 0

	result := l.ToggleFavorite()
	if !result {
		t.Error("expected true when favoriting")
	}
	if !l.Favorites["$SHELL"] {
		t.Error("expected $SHELL to be favorited")
	}
}

func TestToggleFavoriteRemove(t *testing.T) {
	l := New(testRegistry())
	l.Show()
	l.SelectedIdx = 0

	l.ToggleFavorite()           // add
	result := l.ToggleFavorite() // remove
	if result {
		t.Error("expected false when unfavoriting")
	}
	if l.Favorites["$SHELL"] {
		t.Error("expected $SHELL to no longer be favorited")
	}
}

func TestToggleFavoriteNoSelection(t *testing.T) {
	l := New(testRegistry())
	l.Show()
	l.SetQuery("zzzzz") // no results, so no selection

	result := l.ToggleFavorite()
	if result {
		t.Error("expected false when no entry selected")
	}
}

func TestRecordQueryHistoryAndDedup(t *testing.T) {
	l := New(testRegistry())
	l.RecordQuery("nvim")
	l.RecordQuery("htop")
	l.RecordQuery("nvim")

	if len(l.QueryHistory) != 2 {
		t.Fatalf("history len = %d, want 2", len(l.QueryHistory))
	}
	if l.QueryHistory[0] != "nvim" {
		t.Fatalf("history[0] = %q, want nvim", l.QueryHistory[0])
	}
	if l.QueryHistory[1] != "htop" {
		t.Fatalf("history[1] = %q, want htop", l.QueryHistory[1])
	}
}

func TestPrevNextQuery(t *testing.T) {
	l := New(testRegistry())
	l.RecordQuery("one")
	l.RecordQuery("two")
	l.RecordQuery("three")

	l.PrevQuery()
	if l.Query != "three" {
		t.Fatalf("PrevQuery #1 = %q, want three", l.Query)
	}
	l.PrevQuery()
	if l.Query != "two" {
		t.Fatalf("PrevQuery #2 = %q, want two", l.Query)
	}
	l.NextQuery()
	if l.Query != "three" {
		t.Fatalf("NextQuery = %q, want three", l.Query)
	}
	l.NextQuery()
	if l.Query != "" {
		t.Fatalf("NextQuery to live prompt = %q, want empty", l.Query)
	}
}

func TestCompleteFromSelected(t *testing.T) {
	l := New(testRegistry())
	l.Show()
	l.SetQuery("te")
	l.SelectedIdx = 0

	if !l.CompleteFromSelected() {
		t.Fatal("CompleteFromSelected should return true")
	}
	if l.Query != "Terminal" {
		t.Fatalf("query = %q, want Terminal", l.Query)
	}
}

func TestExecResultsAndSuggestions(t *testing.T) {
	l := New(testRegistry())
	l.ExecIndex = []string{"lazygit", "ls", "lua"}
	l.execLoaded = true

	l.SetQuery("la")
	if len(l.Results) == 0 {
		t.Fatal("expected exec match results")
	}
	if len(l.Suggestions) == 0 {
		t.Fatal("expected suggestions")
	}
}

// --- IsFavorite tests ---

func TestIsFavorite(t *testing.T) {
	l := New(testRegistry())
	if l.IsFavorite("nvim") {
		t.Error("should not be favorite initially")
	}
	l.Favorites["nvim"] = true
	if !l.IsFavorite("nvim") {
		t.Error("should be favorite after setting")
	}
	if l.IsFavorite("mc") {
		t.Error("mc should not be favorite")
	}
}

// --- sortByPriority tests ---

func TestSortByPriorityFavoritesFirst(t *testing.T) {
	l := New(testRegistry())
	l.Show()
	// Favorite the last entry (Files/mc)
	l.Favorites["mc"] = true
	l.updateResults()

	if l.Results[0].Command != "mc" {
		t.Errorf("expected favorite 'mc' first, got %q", l.Results[0].Command)
	}
}

func TestSortByPriorityRecentsFirst(t *testing.T) {
	l := New(testRegistry())
	l.Show()
	// Record nvim as recent
	l.RecordLaunch("nvim")
	l.updateResults()

	if l.Results[0].Command != "nvim" {
		t.Errorf("expected recent 'nvim' first, got %q", l.Results[0].Command)
	}
}

func TestSortByPriorityRecentsOrdering(t *testing.T) {
	l := New(testRegistry())
	l.Show()
	// Record mc first, then nvim (nvim is more recent)
	l.RecordLaunch("mc")
	l.RecordLaunch("nvim")
	l.updateResults()

	// nvim should be before mc (lower recent index = more recent)
	nvimIdx := -1
	mcIdx := -1
	for i, r := range l.Results {
		if r.Command == "nvim" {
			nvimIdx = i
		}
		if r.Command == "mc" {
			mcIdx = i
		}
	}
	if nvimIdx > mcIdx {
		t.Errorf("expected nvim (idx=%d) before mc (idx=%d)", nvimIdx, mcIdx)
	}
}

func TestSortByPriorityFavoritesBeforeRecents(t *testing.T) {
	l := New(testRegistry())
	l.Show()
	// Make mc a favorite, make nvim recent but not favorite
	l.Favorites["mc"] = true
	l.RecordLaunch("nvim")
	l.updateResults()

	// mc (favorite) should be before nvim (recent)
	mcIdx := -1
	nvimIdx := -1
	for i, r := range l.Results {
		if r.Command == "mc" {
			mcIdx = i
		}
		if r.Command == "nvim" {
			nvimIdx = i
		}
	}
	if mcIdx > nvimIdx {
		t.Errorf("expected favorite mc (idx=%d) before recent nvim (idx=%d)", mcIdx, nvimIdx)
	}
}






func TestFuzzyFilterSubstringMatch(t *testing.T) {
	// Test substring (non-prefix) matches
	entries := []registry.RegistryEntry{
		{Name: "MyTerminal", Icon: "T", Command: "myterm"},
		{Name: "Terminal", Icon: "T", Command: "term"},
	}
	l := New(entries)
	disableExecIndex(l)
	l.Show()
	l.SetQuery("terminal")
	if len(l.Results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(l.Results))
	}
	// "Terminal" has prefix match (score=2), "MyTerminal" has substring (score=1)
	if l.Results[0].Name != "Terminal" {
		t.Errorf("expected prefix match 'Terminal' first, got %q", l.Results[0].Name)
	}
	if l.Results[1].Name != "MyTerminal" {
		t.Errorf("expected substring match 'MyTerminal' second, got %q", l.Results[1].Name)
	}
}

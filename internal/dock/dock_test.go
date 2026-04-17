package dock

import (
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/icex/termdesk/internal/apps/registry"
)

func testRegistry() []registry.RegistryEntry {
	return []registry.RegistryEntry{
		{Name: "Terminal", Icon: "\uf120", IconColor: "#98C379", Command: "$SHELL"},
		{Name: "nvim", Icon: "\ue62b", IconColor: "#61AFEF", Command: "nvim"},
		{Name: "Files", Icon: "\uf07b", IconColor: "#E5C07B", Command: "yazi"},
		{Name: "System Monitor", Icon: "\uf200", IconColor: "#E06C75", Command: "htop"},
	}
}

func TestNew(t *testing.T) {
	d := New(testRegistry(), 120)
	if d.Width != 120 {
		t.Errorf("width = %d, want 120", d.Width)
	}
	if d.ItemCount() == 0 {
		t.Error("expected default items")
	}
	if d.HoverIndex != -1 {
		t.Errorf("hover = %d, want -1", d.HoverIndex)
	}
}

func TestSetWidth(t *testing.T) {
	d := New(testRegistry(), 80)
	d.SetWidth(120)
	if d.Width != 120 {
		t.Errorf("width = %d, want 120", d.Width)
	}
}

func TestItemCount(t *testing.T) {
	d := New(testRegistry(), 120)
	// 4 registry entries + launcher + expose = 6
	if d.ItemCount() != 6 {
		t.Errorf("ItemCount = %d, want 6", d.ItemCount())
	}
}

func TestSetHover(t *testing.T) {
	d := New(testRegistry(), 120)
	d.SetHover(0)
	if d.HoverIndex != 0 {
		t.Errorf("hover = %d, want 0", d.HoverIndex)
	}
	d.SetHover(-1)
	if d.HoverIndex != -1 {
		t.Errorf("hover = %d, want -1", d.HoverIndex)
	}
	// Out of bounds should not change
	d.SetHover(99)
	if d.HoverIndex != -1 {
		t.Errorf("hover = %d, want -1 (unchanged)", d.HoverIndex)
	}
}

func TestRender(t *testing.T) {
	d := New(testRegistry(), 120)
	rendered := d.Render(120)

	if len(rendered) == 0 {
		t.Error("expected non-empty render")
	}
	if !strings.Contains(rendered, "Terminal") {
		t.Error("expected 'Terminal' in render")
	}
	if !strings.Contains(rendered, "nvim") {
		t.Error("expected 'nvim' in render")
	}
	if !strings.Contains(rendered, "│") {
		t.Error("expected separator in render")
	}
}

func TestRenderHover(t *testing.T) {
	d := New(testRegistry(), 120)
	d.SetHover(1) // hover Terminal (index 1, after launcher)
	rendered := d.Render(120)

	// Hovered item should still be present (accent bg applied at render time, no brackets)
	if !strings.Contains(rendered, "Terminal") {
		t.Error("expected hovered item label in render")
	}
}

func TestRenderNarrow(t *testing.T) {
	d := New(testRegistry(), 20)
	rendered := d.Render(20)
	// Should not panic even if too narrow
	if len(rendered) == 0 {
		t.Error("expected non-empty render even when narrow")
	}
}

func TestItemAtX(t *testing.T) {
	d := New(testRegistry(), 120)

	// Find the first item's position
	positions := d.itemPositions()
	if len(positions) == 0 {
		t.Fatal("no positions")
	}

	idx := d.ItemAtX(positions[0] + 1)
	if idx != 0 {
		t.Errorf("ItemAtX at first item = %d, want 0", idx)
	}

	// Way outside
	idx = d.ItemAtX(0)
	if idx != -1 {
		t.Errorf("ItemAtX(0) = %d, want -1", idx)
	}
}

func TestItemWidth(t *testing.T) {
	d := New(testRegistry(), 120)
	w := d.itemWidth(0)
	if w <= 0 {
		t.Errorf("itemWidth(0) = %d, want > 0", w)
	}
	w = d.itemWidth(-1)
	if w != 0 {
		t.Errorf("itemWidth(-1) = %d, want 0", w)
	}
}

func TestSpecialItems(t *testing.T) {
	d := New(testRegistry(), 120)
	// First item should be launcher
	if d.Items[0].Special != "launcher" {
		t.Errorf("first item special = %q, want launcher", d.Items[0].Special)
	}
	// Last item should be expose
	last := d.Items[len(d.Items)-1]
	if last.Special != "expose" {
		t.Errorf("last item special = %q, want expose", last.Special)
	}
}

func TestRenderCells(t *testing.T) {
	d := New(testRegistry(), 120)
	cells := d.RenderCells(120)
	if len(cells) != 120 {
		t.Errorf("RenderCells len = %d, want 120", len(cells))
	}
	// No cells should be accented without hover
	for _, c := range cells {
		if c.Accent {
			t.Error("expected no accent cells without hover")
			break
		}
	}
	// With hover, some cells should be accented
	d.SetHover(1)
	cells = d.RenderCells(120)
	hasAccent := false
	for _, c := range cells {
		if c.Accent {
			hasAccent = true
			break
		}
	}
	if !hasAccent {
		t.Error("expected accent cells with hover")
	}
}

func TestIconColors(t *testing.T) {
	d := New(testRegistry(), 120)
	// Every default item should have a color
	for i, item := range d.Items {
		if item.IconColor == "" {
			t.Errorf("item %d (%s) has no IconColor", i, item.Label)
		}
	}
}

func TestRenderCellsIconColor(t *testing.T) {
	d := New(testRegistry(), 120)
	cells := d.RenderCells(120)

	// At least some cells should have icon colors
	hasColor := false
	for _, c := range cells {
		if c.IconColor != "" {
			hasColor = true
			break
		}
	}
	if !hasColor {
		t.Error("expected some cells with IconColor set")
	}
}

func TestRenderCellsHoverIconColor(t *testing.T) {
	d := New(testRegistry(), 120)
	d.SetHover(1) // hover Terminal item
	cells := d.RenderCells(120)

	// Hovered item should have both Accent and IconColor
	hasAccentWithColor := false
	for _, c := range cells {
		if c.Accent && c.IconColor != "" {
			hasAccentWithColor = true
			break
		}
	}
	if !hasAccentWithColor {
		t.Error("expected hovered icon cells with both Accent and IconColor")
	}
}

func TestRenderCellsSeparator(t *testing.T) {
	d := New(testRegistry(), 120)
	cells := d.RenderCells(120)

	// Should have separator cells (│ between items in non-icons-only mode)
	hasSeparator := false
	for _, c := range cells {
		if c.Separator {
			hasSeparator = true
			break
		}
	}
	if !hasSeparator {
		t.Error("expected separator cells between dock items")
	}
}

func TestRenderCellsSeparatorIconsOnly(t *testing.T) {
	d := New(testRegistry(), 120)
	d.IconsOnly = true
	cells := d.RenderCells(120)

	// In icons-only mode, separator is just space — no non-space chars between spans
	// so Separator should not be set (separator detection checks ch != ' ')
	hasSeparator := false
	for _, c := range cells {
		if c.Separator {
			hasSeparator = true
			break
		}
	}
	if hasSeparator {
		t.Error("icons-only mode should not have separator cells (separator is space)")
	}
}

func TestIconsOnlyMode(t *testing.T) {
	d := New(testRegistry(), 120)
	d.IconsOnly = true
	rendered := d.Render(120)

	// Should not contain labels (except as part of icon chars)
	if strings.Contains(rendered, "Terminal") {
		t.Error("icons-only mode should not show 'Terminal' label")
	}
	if strings.Contains(rendered, "nvim") {
		t.Error("icons-only mode should not show 'nvim' label")
	}
}

func TestRuneCount(t *testing.T) {
	if utf8.RuneCountInString("hello") != 5 {
		t.Error("RuneCountInString(hello) != 5")
	}
	if utf8.RuneCountInString("") != 0 {
		t.Error("RuneCountInString('') != 0")
	}
	if utf8.RuneCountInString("日本語") != 3 {
		t.Error("RuneCountInString(日本語) != 3")
	}
}

func TestActiveCellForFocusedWindow(t *testing.T) {
	d := New(testRegistry(), 120)
	// Add a running window item with a window ID
	d.Items = append(d.Items, DockItem{
		Icon:     "\uf120",
		Label:    "Shell",
		Special:  "running",
		WindowID: "win-abc",
	})
	// No focused window — no active cells
	cells := d.RenderCells(120)
	for _, c := range cells {
		if c.Active {
			t.Error("expected no active cells without FocusedWindowID")
			break
		}
	}

	// Set focused window — matching item should get Active
	d.FocusedWindowID = "win-abc"
	cells = d.RenderCells(120)
	hasActive := false
	for _, c := range cells {
		if c.Active {
			hasActive = true
			break
		}
	}
	if !hasActive {
		t.Error("expected active cells for focused window")
	}
}

func TestActiveCellOnlyForMatchingWindow(t *testing.T) {
	d := New(testRegistry(), 120)
	d.Items = append(d.Items,
		DockItem{Icon: "\uf120", Label: "Win1", Special: "running", WindowID: "win-1"},
		DockItem{Icon: "\uf120", Label: "Win2", Special: "running", WindowID: "win-2"},
	)
	d.FocusedWindowID = "win-1"
	cells := d.RenderCells(120)

	// Count active vs running cells — Active should only appear on win-1's span
	// Find the span for each item
	positions := d.itemPositions()
	win1Idx := len(d.Items) - 2 // second to last
	win2Idx := len(d.Items) - 1 // last

	win1Start := positions[win1Idx]
	win1End := win1Start + d.itemWidth(win1Idx)
	win2Start := positions[win2Idx]
	win2End := win2Start + d.itemWidth(win2Idx)

	for x := win1Start; x < win1End && x < len(cells); x++ {
		if cells[x].Char != ' ' && !cells[x].Active {
			t.Errorf("cell at %d in win-1 span should be active", x)
			break
		}
	}
	for x := win2Start; x < win2End && x < len(cells); x++ {
		if cells[x].Active {
			t.Errorf("cell at %d in win-2 span should not be active", x)
			break
		}
	}
}

func TestActiveAndHoverCoexist(t *testing.T) {
	d := New(testRegistry(), 120)
	d.Items = append(d.Items, DockItem{
		Icon: "\uf120", Label: "Shell", Special: "running", WindowID: "win-1",
	})
	d.FocusedWindowID = "win-1"
	hoverIdx := len(d.Items) - 1 // hover the active item
	cells := d.RenderCellsWithHover(120, hoverIdx)

	hasActiveAndAccent := false
	for _, c := range cells {
		if c.Active && c.Accent {
			hasActiveAndAccent = true
			break
		}
	}
	if !hasActiveAndAccent {
		t.Error("expected cells with both Active and Accent when hovering active window")
	}
}

func TestMinimizedCellFlag(t *testing.T) {
	d := New(testRegistry(), 120)
	d.Items = append(d.Items, DockItem{
		Icon: "\uf120", Label: "Min", Special: "minimized", WindowID: "win-min",
	})
	cells := d.RenderCells(120)
	hasMinimized := false
	for _, c := range cells {
		if c.Minimized {
			hasMinimized = true
			break
		}
	}
	if !hasMinimized {
		t.Error("expected minimized cells for minimized window item")
	}
}

func TestRunningCellFlag(t *testing.T) {
	d := New(testRegistry(), 120)
	d.Items = append(d.Items, DockItem{
		Icon: "\uf120", Label: "Run", Special: "running", WindowID: "win-run",
	})
	cells := d.RenderCells(120)
	hasRunning := false
	for _, c := range cells {
		if c.Running {
			hasRunning = true
			break
		}
	}
	if !hasRunning {
		t.Error("expected running cells for running window item")
	}
}

func TestNoRunningDotInRender(t *testing.T) {
	d := New(testRegistry(), 120)
	rendered := d.Render(120)
	// The middle dot (·) should not appear in the dock render
	if strings.Contains(rendered, "\u00b7") {
		t.Error("dock should not contain running indicator dot (·)")
	}
}

func TestTooltipTextWithFullTitle(t *testing.T) {
	item := DockItem{Label: "Short", FullTitle: "A much longer full title"}
	if got := item.TooltipText(); got != "A much longer full title" {
		t.Errorf("TooltipText with FullTitle = %q, want 'A much longer full title'", got)
	}
}

func TestTooltipTextWithoutFullTitle(t *testing.T) {
	item := DockItem{Label: "Terminal"}
	if got := item.TooltipText(); got != "Terminal" {
		t.Errorf("TooltipText without FullTitle = %q, want 'Terminal'", got)
	}
}

func TestTooltipTextExposeItem(t *testing.T) {
	// The expose item in New() has both Label and FullTitle
	d := New(testRegistry(), 120)
	last := d.Items[len(d.Items)-1]
	if last.FullTitle == "" {
		t.Fatal("expose item should have FullTitle set")
	}
	if got := last.TooltipText(); got != last.FullTitle {
		t.Errorf("TooltipText for expose = %q, want %q", got, last.FullTitle)
	}
}

func TestItemCenterX(t *testing.T) {
	d := New(testRegistry(), 120)
	positions := d.itemPositions()

	// Test valid indices
	for i := range d.Items {
		cx := d.ItemCenterX(i)
		expected := positions[i] + d.itemWidth(i)/2
		if cx != expected {
			t.Errorf("ItemCenterX(%d) = %d, want %d", i, cx, expected)
		}
	}
}

func TestItemCenterXInvalidIndex(t *testing.T) {
	d := New(testRegistry(), 120)
	// Invalid index returns Width/2
	cx := d.ItemCenterX(-1)
	if cx != 120/2 {
		t.Errorf("ItemCenterX(-1) = %d, want %d", cx, 120/2)
	}
	cx = d.ItemCenterX(99)
	if cx != 120/2 {
		t.Errorf("ItemCenterX(99) = %d, want %d", cx, 120/2)
	}
}

func TestItemWidthWithBell(t *testing.T) {
	d := New(testRegistry(), 120)
	// Get width of a normal item
	normalW := d.itemWidth(1)
	// Set bell on that item
	d.Items[1].HasBell = true
	bellW := d.itemWidth(1)
	// Bell adds 2 extra characters for the bell prefix
	if bellW != normalW+2 {
		t.Errorf("itemWidth with bell = %d, want %d (normal %d + 2)", bellW, normalW+2, normalW)
	}
}

func TestItemWidthWithActivity(t *testing.T) {
	d := New(testRegistry(), 120)
	normalW := d.itemWidth(1)
	d.Items[1].HasActivity = true
	actW := d.itemWidth(1)
	if actW != normalW+2 {
		t.Errorf("itemWidth with activity = %d, want %d (normal %d + 2)", actW, normalW+2, normalW)
	}
}

func TestItemWidthTitleOnly(t *testing.T) {
	d := New(testRegistry(), 120)
	// Add a title-only item (no icon)
	d.Items = append(d.Items, DockItem{Label: "Hello", Special: "minimized"})
	idx := len(d.Items) - 1
	// " Hello " = 5 + 2
	w := d.itemWidth(idx)
	expected := utf8.RuneCountInString("Hello") + 2
	if w != expected {
		t.Errorf("itemWidth for title-only = %d, want %d", w, expected)
	}
}

func TestItemWidthTitleOnlyWithBell(t *testing.T) {
	d := New(testRegistry(), 120)
	d.Items = append(d.Items, DockItem{Label: "Hello", Special: "minimized", HasBell: true})
	idx := len(d.Items) - 1
	w := d.itemWidth(idx)
	// " 󰀧 Hello " = 5 + 2(bell) + 2(padding) = 9
	expected := utf8.RuneCountInString("Hello") + 2 + 2
	if w != expected {
		t.Errorf("itemWidth for title-only with bell = %d, want %d", w, expected)
	}
}

func TestRenderItemsWithBell(t *testing.T) {
	d := New(testRegistry(), 120)
	d.Items[1].HasBell = true
	rendered := d.renderItems()
	// Bell icon U+F0027 should appear in the rendered output
	if !strings.Contains(rendered, "\U000F0027") {
		t.Error("renderItems should include bell icon for item with HasBell")
	}
}

func TestRenderItemsWithActivity(t *testing.T) {
	d := New(testRegistry(), 120)
	d.Items[1].HasActivity = true
	rendered := d.renderItems()
	// Activity spinner should appear — one of the spinner chars
	hasSpinner := false
	for _, ch := range activitySpinner {
		if strings.ContainsRune(rendered, ch) {
			hasSpinner = true
			break
		}
	}
	if !hasSpinner {
		t.Error("renderItems should include activity spinner for item with HasActivity")
	}
}

func TestRenderCellsWithBellFlag(t *testing.T) {
	d := New(testRegistry(), 120)
	d.Items[1].HasBell = true
	cells := d.RenderCells(120)
	hasBell := false
	for _, c := range cells {
		if c.HasBell {
			hasBell = true
			break
		}
	}
	if !hasBell {
		t.Error("expected cells with HasBell flag for item with bell")
	}
}

func TestRenderCellsWithActivityFlag(t *testing.T) {
	d := New(testRegistry(), 120)
	d.Items[1].HasActivity = true
	cells := d.RenderCells(120)
	hasActivity := false
	for _, c := range cells {
		if c.HasActivity {
			hasActivity = true
			break
		}
	}
	if !hasActivity {
		t.Error("expected cells with HasActivity flag for item with activity")
	}
}

func TestRenderItemsTitleOnly(t *testing.T) {
	d := New(testRegistry(), 120)
	d.Items = append(d.Items, DockItem{Label: "MyWin", Special: "minimized"})
	rendered := d.renderItems()
	if !strings.Contains(rendered, "MyWin") {
		t.Error("renderItems should include title-only item label")
	}
}

func TestItemWidthIconsOnly(t *testing.T) {
	d := New(testRegistry(), 120)
	d.IconsOnly = true
	w := d.itemWidth(1) // Item with icon
	iconW := utf8.RuneCountInString(d.Items[1].Icon)
	expected := iconW + 3 // " icon  " (rune count + 3 padding)
	if w != expected {
		t.Errorf("itemWidth icons-only = %d, want %d", w, expected)
	}
}

func TestIconsOnlyHoverCoversFullBlock(t *testing.T) {
	d := New(testRegistry(), 80)
	d.IconsOnly = true

	// Hover first item
	cells := d.RenderCellsWithHover(80, 0)

	// Find first accent cell
	firstAccent := -1
	lastAccent := -1
	for x, c := range cells {
		if c.Accent {
			if firstAccent == -1 {
				firstAccent = x
			}
			lastAccent = x
		}
	}
	if firstAccent == -1 {
		t.Fatal("no accent cells found for hovered item")
	}
	accentWidth := lastAccent - firstAccent + 1
	expectedWidth := utf8.RuneCountInString(d.Items[0].Icon) + 3 // " icon  " = rune count + 3
	if accentWidth != expectedWidth {
		t.Errorf("hover accent width = %d, want %d (first=%d last=%d)", accentWidth, expectedWidth, firstAccent, lastAccent)
	}
	// Verify: space, icon (continuation), space
	if cells[firstAccent].Char != ' ' {
		t.Errorf("first accent cell should be space, got %q", cells[firstAccent].Char)
	}
	if cells[lastAccent].Char != ' ' {
		t.Errorf("last accent cell should be space, got %q", cells[lastAccent].Char)
	}
}

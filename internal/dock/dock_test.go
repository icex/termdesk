package dock

import (
	"strings"
	"testing"
	"unicode/utf8"
)

func TestNew(t *testing.T) {
	d := New(120)
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
	d := New(80)
	d.SetWidth(120)
	if d.Width != 120 {
		t.Errorf("width = %d, want 120", d.Width)
	}
}

func TestItemCount(t *testing.T) {
	d := New(120)
	if d.ItemCount() != 6 {
		t.Errorf("ItemCount = %d, want 6", d.ItemCount())
	}
}

func TestSetHover(t *testing.T) {
	d := New(120)
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
	d := New(120)
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
	d := New(120)
	d.SetHover(1) // hover Terminal (index 1, after launcher)
	rendered := d.Render(120)

	if !strings.Contains(rendered, "[") {
		t.Error("expected brackets around hovered item")
	}
}

func TestRenderNarrow(t *testing.T) {
	d := New(20)
	rendered := d.Render(20)
	// Should not panic even if too narrow
	if len(rendered) == 0 {
		t.Error("expected non-empty render even when narrow")
	}
}

func TestItemAtX(t *testing.T) {
	d := New(120)

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
	d := New(120)
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
	d := New(120)
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
	d := New(120)
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
	d := New(120)
	// Every default item should have a color
	for i, item := range d.Items {
		if item.IconColor == "" {
			t.Errorf("item %d (%s) has no IconColor", i, item.Label)
		}
	}
}

func TestRenderCellsIconColor(t *testing.T) {
	d := New(120)
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
	d := New(120)
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

func TestIconsOnlyMode(t *testing.T) {
	d := New(120)
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

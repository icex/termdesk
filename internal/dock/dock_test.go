package dock

import (
	"strings"
	"testing"
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
	if d.ItemCount() != 4 {
		t.Errorf("ItemCount = %d, want 4", d.ItemCount())
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
	d.SetHover(0)
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

func TestRuneCount(t *testing.T) {
	if runeCount("hello") != 5 {
		t.Error("runeCount(hello) != 5")
	}
	if runeCount("") != 0 {
		t.Error("runeCount('') != 0")
	}
	if runeCount("日本語") != 3 {
		t.Error("runeCount(日本語) != 3")
	}
}

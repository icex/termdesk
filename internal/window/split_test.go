package window

import (
	"testing"

	"github.com/icex/termdesk/pkg/geometry"
)

func TestSplitNodeIsLeaf(t *testing.T) {
	leaf := &SplitNode{TermID: "a"}
	if !leaf.IsLeaf() {
		t.Fatal("expected leaf")
	}
	internal := &SplitNode{
		Dir:      SplitHorizontal,
		Ratio:    0.5,
		Children: [2]*SplitNode{{TermID: "a"}, {TermID: "b"}},
	}
	if internal.IsLeaf() {
		t.Fatal("expected non-leaf")
	}
}

func TestPaneCount(t *testing.T) {
	// Single leaf
	leaf := &SplitNode{TermID: "a"}
	if leaf.PaneCount() != 1 {
		t.Fatalf("expected 1, got %d", leaf.PaneCount())
	}
	// Two panes
	root := &SplitNode{
		Dir: SplitHorizontal, Ratio: 0.5,
		Children: [2]*SplitNode{{TermID: "a"}, {TermID: "b"}},
	}
	if root.PaneCount() != 2 {
		t.Fatalf("expected 2, got %d", root.PaneCount())
	}
	// Three panes (nested)
	root = &SplitNode{
		Dir: SplitHorizontal, Ratio: 0.5,
		Children: [2]*SplitNode{
			{TermID: "a"},
			{Dir: SplitVertical, Ratio: 0.5, Children: [2]*SplitNode{
				{TermID: "b"}, {TermID: "c"},
			}},
		},
	}
	if root.PaneCount() != 3 {
		t.Fatalf("expected 3, got %d", root.PaneCount())
	}
}

func TestAllTermIDs(t *testing.T) {
	root := &SplitNode{
		Dir: SplitHorizontal, Ratio: 0.5,
		Children: [2]*SplitNode{
			{TermID: "a"},
			{Dir: SplitVertical, Ratio: 0.5, Children: [2]*SplitNode{
				{TermID: "b"}, {TermID: "c"},
			}},
		},
	}
	ids := root.AllTermIDs()
	if len(ids) != 3 || ids[0] != "a" || ids[1] != "b" || ids[2] != "c" {
		t.Fatalf("unexpected IDs: %v", ids)
	}
}

func TestFindLeaf(t *testing.T) {
	root := &SplitNode{
		Dir: SplitHorizontal, Ratio: 0.5,
		Children: [2]*SplitNode{{TermID: "a"}, {TermID: "b"}},
	}
	if root.FindLeaf("a") == nil {
		t.Fatal("expected to find 'a'")
	}
	if root.FindLeaf("b") == nil {
		t.Fatal("expected to find 'b'")
	}
	if root.FindLeaf("c") != nil {
		t.Fatal("should not find 'c'")
	}
}

func TestFindParent(t *testing.T) {
	child0 := &SplitNode{TermID: "a"}
	child1 := &SplitNode{TermID: "b"}
	root := &SplitNode{
		Dir: SplitHorizontal, Ratio: 0.5,
		Children: [2]*SplitNode{child0, child1},
	}
	parent := root.FindParent("a")
	if parent != root {
		t.Fatal("parent of 'a' should be root")
	}
	parent = root.FindParent("b")
	if parent != root {
		t.Fatal("parent of 'b' should be root")
	}
	if root.FindParent("missing") != nil {
		t.Fatal("should return nil for missing")
	}
}

func TestLayoutHorizontal(t *testing.T) {
	root := &SplitNode{
		Dir: SplitHorizontal, Ratio: 0.5,
		Children: [2]*SplitNode{{TermID: "a"}, {TermID: "b"}},
	}
	area := geometry.Rect{X: 0, Y: 0, Width: 81, Height: 24}
	panes := root.Layout(area)
	if len(panes) != 2 {
		t.Fatalf("expected 2 panes, got %d", len(panes))
	}
	// Left pane: 40 cols (floor(80*0.5)), separator at 40, right pane: 40 cols
	left := panes[0]
	right := panes[1]
	if left.TermID != "a" || right.TermID != "b" {
		t.Fatal("wrong term IDs")
	}
	if left.Rect.X != 0 || left.Rect.Width+1+right.Rect.Width != 81 {
		t.Fatalf("widths don't add up: left=%d sep=1 right=%d total=%d",
			left.Rect.Width, right.Rect.Width, left.Rect.Width+1+right.Rect.Width)
	}
	if left.Rect.Height != 24 || right.Rect.Height != 24 {
		t.Fatal("heights should equal area height")
	}
}

func TestLayoutVertical(t *testing.T) {
	root := &SplitNode{
		Dir: SplitVertical, Ratio: 0.5,
		Children: [2]*SplitNode{{TermID: "a"}, {TermID: "b"}},
	}
	area := geometry.Rect{X: 0, Y: 0, Width: 80, Height: 25}
	panes := root.Layout(area)
	if len(panes) != 2 {
		t.Fatalf("expected 2 panes, got %d", len(panes))
	}
	top := panes[0]
	bot := panes[1]
	if top.Rect.Y != 0 {
		t.Fatalf("top pane should start at Y=0, got %d", top.Rect.Y)
	}
	if top.Rect.Height+1+bot.Rect.Height != 25 {
		t.Fatalf("heights don't add up: top=%d sep=1 bot=%d",
			top.Rect.Height, bot.Rect.Height)
	}
}

func TestSeparators(t *testing.T) {
	root := &SplitNode{
		Dir: SplitHorizontal, Ratio: 0.5,
		Children: [2]*SplitNode{{TermID: "a"}, {TermID: "b"}},
	}
	area := geometry.Rect{X: 0, Y: 0, Width: 81, Height: 24}
	seps := root.Separators(area)
	if len(seps) != 1 {
		t.Fatalf("expected 1 separator, got %d", len(seps))
	}
	sep := seps[0]
	if sep.Dir != SplitHorizontal {
		t.Fatal("separator dir should be horizontal")
	}
	if sep.Rect.Width != 1 || sep.Rect.Height != 24 {
		t.Fatalf("bad separator dimensions: %dx%d", sep.Rect.Width, sep.Rect.Height)
	}
}

func TestSeparatorsNested(t *testing.T) {
	root := &SplitNode{
		Dir: SplitHorizontal, Ratio: 0.5,
		Children: [2]*SplitNode{
			{TermID: "a"},
			{Dir: SplitVertical, Ratio: 0.5, Children: [2]*SplitNode{
				{TermID: "b"}, {TermID: "c"},
			}},
		},
	}
	area := geometry.Rect{X: 0, Y: 0, Width: 81, Height: 25}
	seps := root.Separators(area)
	if len(seps) != 2 {
		t.Fatalf("expected 2 separators, got %d", len(seps))
	}
}

func TestRemoveLeaf(t *testing.T) {
	root := &SplitNode{
		Dir: SplitHorizontal, Ratio: 0.5,
		Children: [2]*SplitNode{{TermID: "a"}, {TermID: "b"}},
	}
	// Remove 'a' -> should return leaf 'b'
	newRoot := root.RemoveLeaf("a")
	if newRoot == nil || !newRoot.IsLeaf() || newRoot.TermID != "b" {
		t.Fatal("expected leaf 'b' after removing 'a'")
	}

	// Remove from 3-pane tree
	root = &SplitNode{
		Dir: SplitHorizontal, Ratio: 0.5,
		Children: [2]*SplitNode{
			{TermID: "a"},
			{Dir: SplitVertical, Ratio: 0.5, Children: [2]*SplitNode{
				{TermID: "b"}, {TermID: "c"},
			}},
		},
	}
	newRoot = root.RemoveLeaf("b")
	if newRoot.PaneCount() != 2 {
		t.Fatalf("expected 2 panes after removing 'b', got %d", newRoot.PaneCount())
	}
	ids := newRoot.AllTermIDs()
	if ids[0] != "a" || ids[1] != "c" {
		t.Fatalf("unexpected IDs after remove: %v", ids)
	}

	// Remove non-existent
	root2 := &SplitNode{
		Dir: SplitHorizontal, Ratio: 0.5,
		Children: [2]*SplitNode{{TermID: "x"}, {TermID: "y"}},
	}
	newRoot = root2.RemoveLeaf("z")
	if newRoot != root2 {
		t.Fatal("removing non-existent should return same root")
	}
}

func TestReplaceLeaf(t *testing.T) {
	root := &SplitNode{
		Dir: SplitHorizontal, Ratio: 0.5,
		Children: [2]*SplitNode{{TermID: "a"}, {TermID: "b"}},
	}
	newNode := &SplitNode{
		Dir: SplitVertical, Ratio: 0.5,
		Children: [2]*SplitNode{{TermID: "a"}, {TermID: "c"}},
	}
	ok := root.ReplaceLeaf("a", newNode)
	if !ok {
		t.Fatal("replace should succeed")
	}
	if root.PaneCount() != 3 {
		t.Fatalf("expected 3 panes, got %d", root.PaneCount())
	}
	ids := root.AllTermIDs()
	if ids[0] != "a" || ids[1] != "c" || ids[2] != "b" {
		t.Fatalf("unexpected IDs: %v", ids)
	}
}

func TestReplaceLeafNotFound(t *testing.T) {
	root := &SplitNode{
		Dir: SplitHorizontal, Ratio: 0.5,
		Children: [2]*SplitNode{{TermID: "a"}, {TermID: "b"}},
	}
	newNode := &SplitNode{TermID: "c"}
	ok := root.ReplaceLeaf("nonexistent", newNode)
	if ok {
		t.Fatal("replace should fail for nonexistent leaf")
	}
}

func TestReplaceLeafOnLeaf(t *testing.T) {
	// Calling ReplaceLeaf on a leaf node should return false (can't replace children of a leaf)
	leaf := &SplitNode{TermID: "a"}
	newNode := &SplitNode{TermID: "b"}
	ok := leaf.ReplaceLeaf("a", newNode)
	if ok {
		t.Fatal("ReplaceLeaf on a leaf node should return false")
	}
}

func TestReplaceLeafNested(t *testing.T) {
	// Tree: root(H) -> [a, inner(V) -> [b, c]]
	// Replace "b" inside the nested subtree.
	root := &SplitNode{
		Dir: SplitHorizontal, Ratio: 0.5,
		Children: [2]*SplitNode{
			{TermID: "a"},
			{Dir: SplitVertical, Ratio: 0.5, Children: [2]*SplitNode{
				{TermID: "b"}, {TermID: "c"},
			}},
		},
	}
	newNode := &SplitNode{
		Dir: SplitHorizontal, Ratio: 0.5,
		Children: [2]*SplitNode{{TermID: "b"}, {TermID: "d"}},
	}
	ok := root.ReplaceLeaf("b", newNode)
	if !ok {
		t.Fatal("replace should succeed for nested leaf")
	}
	if root.PaneCount() != 4 {
		t.Fatalf("expected 4 panes, got %d", root.PaneCount())
	}
	ids := root.AllTermIDs()
	expected := []string{"a", "b", "d", "c"}
	for i, want := range expected {
		if ids[i] != want {
			t.Fatalf("ids[%d] = %q, want %q", i, ids[i], want)
		}
	}
}

func TestReplaceLeafSecondChild(t *testing.T) {
	// Replace the second child directly
	root := &SplitNode{
		Dir: SplitHorizontal, Ratio: 0.5,
		Children: [2]*SplitNode{{TermID: "a"}, {TermID: "b"}},
	}
	newNode := &SplitNode{
		Dir: SplitVertical, Ratio: 0.5,
		Children: [2]*SplitNode{{TermID: "b"}, {TermID: "c"}},
	}
	ok := root.ReplaceLeaf("b", newNode)
	if !ok {
		t.Fatal("replace should succeed")
	}
	if root.PaneCount() != 3 {
		t.Fatalf("expected 3 panes, got %d", root.PaneCount())
	}
	ids := root.AllTermIDs()
	if ids[0] != "a" || ids[1] != "b" || ids[2] != "c" {
		t.Fatalf("unexpected IDs: %v", ids)
	}
}

func TestLayoutZeroArea(t *testing.T) {
	root := &SplitNode{
		Dir: SplitHorizontal, Ratio: 0.5,
		Children: [2]*SplitNode{{TermID: "a"}, {TermID: "b"}},
	}
	panes := root.Layout(geometry.Rect{X: 0, Y: 0, Width: 0, Height: 0})
	if len(panes) != 0 {
		t.Fatalf("expected 0 panes for zero area, got %d", len(panes))
	}
}

func TestSeparatorsZeroArea(t *testing.T) {
	root := &SplitNode{
		Dir: SplitHorizontal, Ratio: 0.5,
		Children: [2]*SplitNode{{TermID: "a"}, {TermID: "b"}},
	}
	seps := root.Separators(geometry.Rect{X: 0, Y: 0, Width: 0, Height: 0})
	if len(seps) != 0 {
		t.Fatalf("expected 0 separators for zero area, got %d", len(seps))
	}
}

func TestSeparatorsLeaf(t *testing.T) {
	leaf := &SplitNode{TermID: "a"}
	seps := leaf.Separators(geometry.Rect{X: 0, Y: 0, Width: 80, Height: 24})
	if len(seps) != 0 {
		t.Fatalf("expected 0 separators for leaf, got %d", len(seps))
	}
}

func TestSeparatorsVertical(t *testing.T) {
	root := &SplitNode{
		Dir: SplitVertical, Ratio: 0.5,
		Children: [2]*SplitNode{{TermID: "a"}, {TermID: "b"}},
	}
	area := geometry.Rect{X: 0, Y: 0, Width: 80, Height: 25}
	seps := root.Separators(area)
	if len(seps) != 1 {
		t.Fatalf("expected 1 separator, got %d", len(seps))
	}
	if seps[0].Dir != SplitVertical {
		t.Fatal("separator dir should be vertical")
	}
	if seps[0].Rect.Width != 80 || seps[0].Rect.Height != 1 {
		t.Fatalf("bad separator dimensions: %dx%d", seps[0].Rect.Width, seps[0].Rect.Height)
	}
}

func TestFindParentNested(t *testing.T) {
	inner := &SplitNode{
		Dir: SplitVertical, Ratio: 0.5,
		Children: [2]*SplitNode{{TermID: "b"}, {TermID: "c"}},
	}
	root := &SplitNode{
		Dir: SplitHorizontal, Ratio: 0.5,
		Children: [2]*SplitNode{{TermID: "a"}, inner},
	}
	parent := root.FindParent("b")
	if parent != inner {
		t.Fatal("parent of 'b' should be the inner node")
	}
	parent = root.FindParent("c")
	if parent != inner {
		t.Fatal("parent of 'c' should be the inner node")
	}
}

func TestFindLeafNotInLeaf(t *testing.T) {
	leaf := &SplitNode{TermID: "a"}
	if leaf.FindLeaf("b") != nil {
		t.Fatal("should not find 'b' in leaf 'a'")
	}
}

func TestRemoveLeafSingleLeaf(t *testing.T) {
	leaf := &SplitNode{TermID: "a"}
	result := leaf.RemoveLeaf("a")
	if result != nil {
		t.Fatal("removing the only leaf should return nil")
	}
}

func TestRemoveLeafNonexistentFromLeaf(t *testing.T) {
	leaf := &SplitNode{TermID: "a"}
	result := leaf.RemoveLeaf("nonexistent")
	if result != leaf {
		t.Fatal("removing nonexistent from leaf should return same leaf")
	}
}

func TestPaneRectForTerm(t *testing.T) {
	root := &SplitNode{
		Dir: SplitHorizontal, Ratio: 0.5,
		Children: [2]*SplitNode{{TermID: "a"}, {TermID: "b"}},
	}
	area := geometry.Rect{X: 0, Y: 0, Width: 81, Height: 24}
	r := root.PaneRectForTerm("a", area)
	if r.Width <= 0 {
		t.Fatal("expected non-empty rect for 'a'")
	}
	r = root.PaneRectForTerm("missing", area)
	if r.Width != 0 {
		t.Fatal("expected empty rect for missing term")
	}
}

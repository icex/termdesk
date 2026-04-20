package window

import (
	"testing"
)

func TestEncodeSplitTree(t *testing.T) {
	// Simple 2-pane horizontal split
	root := &SplitNode{
		Dir:   SplitHorizontal,
		Ratio: 0.50,
		Children: [2]*SplitNode{
			{TermID: "abc123"},
			{TermID: "def456"},
		},
	}
	encoded := EncodeSplitTree(root)
	if encoded != "S:H:0.50 L:abc123 L:def456" {
		t.Fatalf("unexpected encoding: %q", encoded)
	}

	decoded := DecodeSplitTree(encoded)
	if decoded == nil || decoded.IsLeaf() {
		t.Fatal("decoded should be internal node")
	}
	if decoded.Dir != SplitHorizontal {
		t.Fatalf("expected H, got V")
	}
	if decoded.Children[0].TermID != "abc123" {
		t.Fatalf("child0 termID: %q", decoded.Children[0].TermID)
	}
	if decoded.Children[1].TermID != "def456" {
		t.Fatalf("child1 termID: %q", decoded.Children[1].TermID)
	}
}

func TestEncodeSplitTreeNested(t *testing.T) {
	root := &SplitNode{
		Dir:   SplitHorizontal,
		Ratio: 0.50,
		Children: [2]*SplitNode{
			{TermID: "a"},
			{
				Dir:   SplitVertical,
				Ratio: 0.60,
				Children: [2]*SplitNode{
					{TermID: "b"},
					{TermID: "c"},
				},
			},
		},
	}
	encoded := EncodeSplitTree(root)
	expected := "S:H:0.50 L:a S:V:0.60 L:b L:c"
	if encoded != expected {
		t.Fatalf("expected %q, got %q", expected, encoded)
	}

	decoded := DecodeSplitTree(encoded)
	if decoded == nil || decoded.IsLeaf() {
		t.Fatal("decoded should be internal")
	}
	if decoded.Children[0].TermID != "a" {
		t.Fatalf("child0: %q", decoded.Children[0].TermID)
	}
	inner := decoded.Children[1]
	if inner.IsLeaf() || inner.Dir != SplitVertical {
		t.Fatal("child1 should be V split")
	}
	if inner.Children[0].TermID != "b" || inner.Children[1].TermID != "c" {
		t.Fatal("inner children wrong")
	}
}

func TestDecodeSplitTreeEmpty(t *testing.T) {
	if DecodeSplitTree("") != nil {
		t.Fatal("empty should return nil")
	}
}

func TestEncodeSplitTreeNil(t *testing.T) {
	if EncodeSplitTree(nil) != "" {
		t.Fatal("nil should return empty string")
	}
}

func TestRoundTrip(t *testing.T) {
	// Deep tree: ((a | b) / (c | d))
	root := &SplitNode{
		Dir:   SplitVertical,
		Ratio: 0.50,
		Children: [2]*SplitNode{
			{
				Dir:   SplitHorizontal,
				Ratio: 0.40,
				Children: [2]*SplitNode{
					{TermID: "a"},
					{TermID: "b"},
				},
			},
			{
				Dir:   SplitHorizontal,
				Ratio: 0.60,
				Children: [2]*SplitNode{
					{TermID: "c"},
					{TermID: "d"},
				},
			},
		},
	}
	encoded := EncodeSplitTree(root)
	decoded := DecodeSplitTree(encoded)
	reencoded := EncodeSplitTree(decoded)
	if encoded != reencoded {
		t.Fatalf("round-trip mismatch:\n  %q\n  %q", encoded, reencoded)
	}
	ids := decoded.AllTermIDs()
	if len(ids) != 4 || ids[0] != "a" || ids[1] != "b" || ids[2] != "c" || ids[3] != "d" {
		t.Fatalf("unexpected IDs: %v", ids)
	}
}

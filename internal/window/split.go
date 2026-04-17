package window

import (
	"fmt"
	"strings"

	"github.com/icex/termdesk/pkg/geometry"
)

// MinPaneSize is the minimum width or height for a pane in cells.
const MinPaneSize = 5

// SplitDir indicates the axis of a split.
type SplitDir int

const (
	SplitHorizontal SplitDir = iota // left | right (vertical divider line)
	SplitVertical                   // top / bottom (horizontal divider line)
)

// SplitNode is a binary tree node for recursive split layouts.
// Leaf nodes have TermID set and nil Children.
// Internal nodes have two children and a ratio.
type SplitNode struct {
	Dir      SplitDir       // meaningful only for internal nodes
	Ratio    float64        // 0.0..1.0 — fraction of space for Children[0]
	Children [2]*SplitNode  // [0]=left/top, [1]=right/bottom; nil for leaves
	TermID   string         // terminal ID for leaf nodes (empty for internal)
}

// PaneRect maps a leaf terminal ID to its screen rectangle.
type PaneRect struct {
	TermID string
	Rect   geometry.Rect
}

// Separator describes a 1-cell divider line between two panes.
type Separator struct {
	Rect geometry.Rect
	Dir  SplitDir    // direction of the split that created this separator
	Node *SplitNode  // the internal node owning this separator
}

// IsLeaf returns true if this node is a leaf (has a terminal, no children).
func (n *SplitNode) IsLeaf() bool {
	return n.Children[0] == nil
}

// PaneCount returns the number of leaf nodes (terminal panes).
func (n *SplitNode) PaneCount() int {
	if n.IsLeaf() {
		return 1
	}
	return n.Children[0].PaneCount() + n.Children[1].PaneCount()
}

// AllTermIDs collects all leaf terminal IDs in tree order (left/top first).
func (n *SplitNode) AllTermIDs() []string {
	if n.IsLeaf() {
		return []string{n.TermID}
	}
	ids := n.Children[0].AllTermIDs()
	ids = append(ids, n.Children[1].AllTermIDs()...)
	return ids
}

// FindLeaf searches the tree for a leaf with the given terminal ID.
func (n *SplitNode) FindLeaf(termID string) *SplitNode {
	if n.IsLeaf() {
		if n.TermID == termID {
			return n
		}
		return nil
	}
	if found := n.Children[0].FindLeaf(termID); found != nil {
		return found
	}
	return n.Children[1].FindLeaf(termID)
}

// FindParent returns the parent node of the leaf with the given terminal ID,
// or nil if the leaf is the root or not found.
func (n *SplitNode) FindParent(termID string) *SplitNode {
	if n.IsLeaf() {
		return nil
	}
	for i := 0; i < 2; i++ {
		child := n.Children[i]
		if child.IsLeaf() && child.TermID == termID {
			return n
		}
		if found := child.FindParent(termID); found != nil {
			return found
		}
	}
	return nil
}

// Layout computes the screen rectangle for each leaf pane within the given area.
// Separators consume 1 cell between children.
func (n *SplitNode) Layout(area geometry.Rect) []PaneRect {
	if area.Width <= 0 || area.Height <= 0 {
		return nil
	}
	if n.IsLeaf() {
		return []PaneRect{{TermID: n.TermID, Rect: area}}
	}

	var first, second geometry.Rect
	if n.Dir == SplitHorizontal {
		// left | separator(1) | right
		leftW := int(float64(area.Width-1) * n.Ratio)
		if leftW < 1 {
			leftW = 1
		}
		rightW := area.Width - leftW - 1
		if rightW < 1 {
			rightW = 1
			leftW = area.Width - 1 - rightW
		}
		first = geometry.Rect{X: area.X, Y: area.Y, Width: leftW, Height: area.Height}
		second = geometry.Rect{X: area.X + leftW + 1, Y: area.Y, Width: rightW, Height: area.Height}
	} else {
		// top / separator(1) / bottom
		topH := int(float64(area.Height-1) * n.Ratio)
		if topH < 1 {
			topH = 1
		}
		botH := area.Height - topH - 1
		if botH < 1 {
			botH = 1
			topH = area.Height - 1 - botH
		}
		first = geometry.Rect{X: area.X, Y: area.Y, Width: area.Width, Height: topH}
		second = geometry.Rect{X: area.X, Y: area.Y + topH + 1, Width: area.Width, Height: botH}
	}

	result := n.Children[0].Layout(first)
	result = append(result, n.Children[1].Layout(second)...)
	return result
}

// Separators computes the screen positions of all separator lines within the given area.
func (n *SplitNode) Separators(area geometry.Rect) []Separator {
	if area.Width <= 0 || area.Height <= 0 || n.IsLeaf() {
		return nil
	}

	var seps []Separator
	var firstArea, secondArea geometry.Rect

	if n.Dir == SplitHorizontal {
		leftW := int(float64(area.Width-1) * n.Ratio)
		if leftW < 1 {
			leftW = 1
		}
		rightW := area.Width - leftW - 1
		if rightW < 1 {
			rightW = 1
			leftW = area.Width - 1 - rightW
		}
		sepX := area.X + leftW
		seps = append(seps, Separator{
			Rect: geometry.Rect{X: sepX, Y: area.Y, Width: 1, Height: area.Height},
			Dir:  SplitHorizontal,
			Node: n,
		})
		firstArea = geometry.Rect{X: area.X, Y: area.Y, Width: leftW, Height: area.Height}
		secondArea = geometry.Rect{X: sepX + 1, Y: area.Y, Width: rightW, Height: area.Height}
	} else {
		topH := int(float64(area.Height-1) * n.Ratio)
		if topH < 1 {
			topH = 1
		}
		botH := area.Height - topH - 1
		if botH < 1 {
			botH = 1
			topH = area.Height - 1 - botH
		}
		sepY := area.Y + topH
		seps = append(seps, Separator{
			Rect: geometry.Rect{X: area.X, Y: sepY, Width: area.Width, Height: 1},
			Dir:  SplitVertical,
			Node: n,
		})
		firstArea = geometry.Rect{X: area.X, Y: area.Y, Width: area.Width, Height: topH}
		secondArea = geometry.Rect{X: area.X, Y: sepY + 1, Width: area.Width, Height: botH}
	}

	seps = append(seps, n.Children[0].Separators(firstArea)...)
	seps = append(seps, n.Children[1].Separators(secondArea)...)
	return seps
}

// RemoveLeaf removes the leaf with the given terminal ID from the tree.
// Returns the new root (nil if tree becomes empty, or the surviving sibling
// if the parent had only two children).
func (n *SplitNode) RemoveLeaf(termID string) *SplitNode {
	if n.IsLeaf() {
		if n.TermID == termID {
			return nil
		}
		return n
	}
	// Check if either direct child is the target leaf
	for i := 0; i < 2; i++ {
		child := n.Children[i]
		if child.IsLeaf() && child.TermID == termID {
			// Return the other child (it replaces this node)
			return n.Children[1-i]
		}
	}
	// Recurse into children
	for i := 0; i < 2; i++ {
		if !n.Children[i].IsLeaf() {
			newChild := n.Children[i].RemoveLeaf(termID)
			if newChild != n.Children[i] {
				n.Children[i] = newChild
				if newChild == nil {
					return n.Children[1-i]
				}
				return n
			}
		}
	}
	return n
}

// ReplaceLeaf finds the leaf with oldTermID and replaces it with newNode.
// Returns true if the replacement was made.
func (n *SplitNode) ReplaceLeaf(oldTermID string, newNode *SplitNode) bool {
	if n.IsLeaf() {
		return false
	}
	for i := 0; i < 2; i++ {
		child := n.Children[i]
		if child.IsLeaf() && child.TermID == oldTermID {
			n.Children[i] = newNode
			return true
		}
		if child.ReplaceLeaf(oldTermID, newNode) {
			return true
		}
	}
	return false
}

// PaneRectForTerm returns the rectangle for a specific terminal ID, or empty rect if not found.
func (n *SplitNode) PaneRectForTerm(termID string, area geometry.Rect) geometry.Rect {
	for _, pr := range n.Layout(area) {
		if pr.TermID == termID {
			return pr.Rect
		}
	}
	return geometry.Rect{}
}

// EncodeSplitTree serializes a split tree to a compact string for workspace persistence.
// Format: "L:termID" for leaves, "S:dir:ratio child0 child1" for internal nodes.
// Pre-order traversal with space-separated tokens.
func EncodeSplitTree(n *SplitNode) string {
	if n == nil {
		return ""
	}
	if n.IsLeaf() {
		return "L:" + n.TermID
	}
	dir := "H"
	if n.Dir == SplitVertical {
		dir = "V"
	}
	return fmt.Sprintf("S:%s:%.2f %s %s", dir, n.Ratio,
		EncodeSplitTree(n.Children[0]), EncodeSplitTree(n.Children[1]))
}

// DecodeSplitTree deserializes a split tree from a string produced by EncodeSplitTree.
func DecodeSplitTree(s string) *SplitNode {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}
	tokens := strings.Fields(s)
	node, _ := decodeSplitNode(tokens, 0)
	return node
}

func decodeSplitNode(tokens []string, idx int) (*SplitNode, int) {
	if idx >= len(tokens) {
		return nil, idx
	}
	token := tokens[idx]
	if strings.HasPrefix(token, "L:") {
		return &SplitNode{TermID: token[2:]}, idx + 1
	}
	if strings.HasPrefix(token, "S:") {
		parts := strings.SplitN(token, ":", 3)
		if len(parts) < 3 {
			return nil, idx + 1
		}
		dir := SplitHorizontal
		if parts[1] == "V" {
			dir = SplitVertical
		}
		ratio := 0.5
		fmt.Sscanf(parts[2], "%f", &ratio)
		child0, nextIdx := decodeSplitNode(tokens, idx+1)
		child1, nextIdx2 := decodeSplitNode(tokens, nextIdx)
		return &SplitNode{
			Dir:      dir,
			Ratio:    ratio,
			Children: [2]*SplitNode{child0, child1},
		}, nextIdx2
	}
	return nil, idx + 1
}

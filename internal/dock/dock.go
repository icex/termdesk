package dock

import (
	"strings"
	"unicode/utf8"
)

// DockItem represents a launchable app in the dock.
type DockItem struct {
	Icon      string // Nerd Font icon
	IconColor string // hex color for the icon (e.g. "#98C379"), "" = default
	Label     string
	Command   string
	Args      []string
	Special   string // "launcher", "expose", "minimized", or "" for normal items
	WindowID  string // window ID for minimized items
}

// DockCell represents a single cell in the dock render output with styling info.
type DockCell struct {
	Char      rune
	Accent    bool   // true = use accent color (hovered item)
	Special   bool   // true = use a distinct style (launcher/expose buttons)
	Minimized bool   // true = minimized window item
	Indicator bool   // true = running indicator dot
	Pulse     bool   // true = dock launch pulse animation active
	IconColor string // hex color for icon characters, "" = default
}

// Dock represents the bottom dock bar.
type Dock struct {
	Items           []DockItem
	Width           int
	HoverIndex      int  // -1 = none
	IconsOnly       bool // true = show only icons, no labels
	RunningCommands map[string]bool // set of commands that have running windows
}

// New creates a dock with default items.
func New(width int) *Dock {
	return &Dock{
		Items: []DockItem{
			{Icon: "\uf135", IconColor: "#56B6C2", Label: "Launch", Special: "launcher"},   // cyan
			{Icon: "\uf120", IconColor: "#98C379", Label: "Terminal", Command: "$SHELL"},    // green
			{Icon: "\ue62b", IconColor: "#61AFEF", Label: "nvim", Command: "nvim"},          // blue
			{Icon: "\uf07b", IconColor: "#E5C07B", Label: "Files", Command: "mc"},           // yellow
			{Icon: "\uf1ec", IconColor: "#D19A66", Label: "Calc", Command: "python3"},       // orange
			{Icon: "\uf26c", IconColor: "#C678DD", Label: "Expose\u0301", Special: "expose"}, // purple
		},
		Width:           width,
		HoverIndex:      -1,
		RunningCommands: make(map[string]bool),
	}
}

// SetWidth updates the dock width.
func (d *Dock) SetWidth(w int) {
	d.Width = w
}

// ItemCount returns the number of dock items.
func (d *Dock) ItemCount() int {
	return len(d.Items)
}

// ItemAtX returns the dock item index at the given X position, or -1.
func (d *Dock) ItemAtX(x int) int {
	positions := d.itemPositions()
	for i, pos := range positions {
		itemW := d.itemWidth(i)
		if x >= pos && x < pos+itemW {
			return i
		}
	}
	return -1
}

// SetHover sets the hover index.
func (d *Dock) SetHover(idx int) {
	if idx >= -1 && idx < len(d.Items) {
		d.HoverIndex = idx
	}
}

// IsRunning returns whether a dock item has a running window.
func (d *Dock) IsRunning(idx int) bool {
	if idx < 0 || idx >= len(d.Items) {
		return false
	}
	cmd := d.Items[idx].Command
	return cmd != "" && d.RunningCommands[cmd]
}

// Render returns the dock bar as a single string.
func (d *Dock) Render(width int) string {
	var sb strings.Builder

	// Center the dock items
	content := d.renderItems()
	contentLen := utf8.RuneCountInString(content)
	padding := (width - contentLen) / 2
	if padding < 0 {
		padding = 0
	}

	sb.WriteString(strings.Repeat(" ", padding))
	sb.WriteString(content)

	// Fill remaining width
	remaining := width - padding - contentLen
	if remaining > 0 {
		sb.WriteString(strings.Repeat(" ", remaining))
	}

	return sb.String()
}

// RenderCells returns dock content as structured cells for per-cell styling.
func (d *Dock) RenderCells(width int) []DockCell {
	cells := make([]DockCell, width)

	// Build content and track which cells belong to which item
	type itemSpan struct {
		start, end int
		index      int
	}

	content := d.renderItems()
	contentLen := utf8.RuneCountInString(content)
	padding := (width - contentLen) / 2
	if padding < 0 {
		padding = 0
	}

	// Build span map
	var spans []itemSpan
	x := padding
	sep := d.separatorWidth()
	for i := range d.Items {
		iw := d.itemWidth(i)
		spans = append(spans, itemSpan{start: x, end: x + iw, index: i})
		x += iw + sep
	}

	// Compute icon width for each item (to colorize icon chars)
	iconWidths := make([]int, len(d.Items))
	for i, item := range d.Items {
		iconWidths[i] = utf8.RuneCountInString(item.Icon)
	}

	// Fill cells
	col := 0
	for _, ch := range d.Render(width) {
		if col >= width {
			break
		}
		cell := DockCell{Char: ch}
		for _, span := range spans {
			if col >= span.start && col < span.end {
				if span.index == d.HoverIndex {
					cell.Accent = true
				}
				item := d.Items[span.index]
				if item.Special == "minimized" {
					cell.Minimized = true
				} else if item.Special != "" {
					cell.Special = true
				}
				// Determine if this cell is part of the icon
				localX := col - span.start
				bracketOffset := 0
				if span.index == d.HoverIndex {
					bracketOffset = 1 // skip leading '['
				}
				if localX >= bracketOffset && localX < bracketOffset+iconWidths[span.index] {
					cell.IconColor = item.IconColor
				}
				break
			}
		}
		cells[col] = cell
		col++
	}

	return cells
}

func (d *Dock) renderItems() string {
	var parts []string
	for i, item := range d.Items {
		var s string
		if d.IconsOnly {
			s = item.Icon
		} else {
			s = item.Icon + " " + item.Label
		}
		// Running indicator dot (macOS-like)
		if d.IsRunning(i) {
			s += "\u00b7" // middle dot ·
		}
		if i == d.HoverIndex {
			s = "[" + s + "]"
		}
		parts = append(parts, s)
	}
	sep := " " + string('\u2502') + " " // " │ "
	if d.IconsOnly {
		sep = " " // thin separator for icons-only
	}
	return strings.Join(parts, sep)
}

func (d *Dock) separatorWidth() int {
	if d.IconsOnly {
		return 1 // just a space
	}
	return 3 // " │ "
}

func (d *Dock) itemPositions() []int {
	positions := make([]int, len(d.Items))
	content := d.renderItems()
	contentLen := utf8.RuneCountInString(content)
	padding := (d.Width - contentLen) / 2
	if padding < 0 {
		padding = 0
	}

	sep := d.separatorWidth()
	x := padding
	for i := range d.Items {
		positions[i] = x
		w := d.itemWidth(i)
		x += w + sep
	}
	return positions
}

func (d *Dock) itemWidth(idx int) int {
	if idx < 0 || idx >= len(d.Items) {
		return 0
	}
	item := d.Items[idx]
	var w int
	if d.IconsOnly {
		w = utf8.RuneCountInString(item.Icon)
	} else {
		w = utf8.RuneCountInString(item.Icon) + 1 + utf8.RuneCountInString(item.Label)
	}
	if d.IsRunning(idx) {
		w++ // dot
	}
	if idx == d.HoverIndex {
		w += 2 // brackets
	}
	return w
}

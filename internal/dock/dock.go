package dock

import (
	"strings"
)

// DockItem represents a launchable app in the dock.
type DockItem struct {
	Icon    string // Nerd Font icon
	Label   string
	Command string
	Args    []string
}

// Dock represents the bottom dock bar.
type Dock struct {
	Items      []DockItem
	Width      int
	HoverIndex int // -1 = none
}

// New creates a dock with default items.
func New(width int) *Dock {
	return &Dock{
		Items: []DockItem{
			{Icon: "", Label: "Terminal", Command: "$SHELL"},
			{Icon: "", Label: "nvim", Command: "nvim"},
			{Icon: "", Label: "Files", Command: "spf"},
			{Icon: "", Label: "Calc", Command: "calc"},
		},
		Width:      width,
		HoverIndex: -1,
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

// Render returns the dock bar as a single string.
func (d *Dock) Render(width int) string {
	var sb strings.Builder

	// Center the dock items
	content := d.renderItems()
	contentLen := runeCount(content)
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

func (d *Dock) renderItems() string {
	var parts []string
	for i, item := range d.Items {
		s := item.Icon + " " + item.Label
		if i == d.HoverIndex {
			s = "[" + s + "]"
		}
		parts = append(parts, s)
	}
	return strings.Join(parts, " │ ")
}

func (d *Dock) itemPositions() []int {
	positions := make([]int, len(d.Items))
	content := d.renderItems()
	contentLen := runeCount(content)
	padding := (d.Width - contentLen) / 2
	if padding < 0 {
		padding = 0
	}

	x := padding
	for i, item := range d.Items {
		positions[i] = x
		w := d.itemWidth(i)
		x += w + 3 // " │ " separator
		_ = item
	}
	return positions
}

func (d *Dock) itemWidth(idx int) int {
	if idx < 0 || idx >= len(d.Items) {
		return 0
	}
	item := d.Items[idx]
	w := runeCount(item.Icon) + 1 + len(item.Label) // "icon label"
	if idx == d.HoverIndex {
		w += 2 // brackets
	}
	return w
}

func runeCount(s string) int {
	n := 0
	for range s {
		n++
	}
	return n
}

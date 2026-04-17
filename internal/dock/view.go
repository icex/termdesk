package dock

import (
	"strings"
	"time"
	"unicode/utf8"
)

// activitySpinner defines the spinner characters for the dock activity indicator.
// Quarter-circle rotation — widely supported in all Unicode fonts (Geometric Shapes block).
var activitySpinner = []rune{'◐', '◓', '◑', '◒'}

// activitySpinnerFrame returns the current spinner frame based on wall clock time.
// Changes every 120ms for a smooth ~8fps animation without needing a dedicated tick.
func activitySpinnerFrame() rune {
	frame := int(time.Now().UnixMilli()/120) % len(activitySpinner)
	return activitySpinner[frame]
}

// Render returns the dock bar as a single string.
func (d *Dock) Render(width int) string {
	var sb strings.Builder

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
	return d.RenderCellsWithHover(width, d.HoverIndex)
}

// RenderCellsWithHover returns dock cells using the given hover index (avoids mutating d.HoverIndex).
func (d *Dock) RenderCellsWithHover(width int, hoverIdx int) []DockCell {
	cells := make([]DockCell, width)

	// Build content once and reuse (avoids calling renderItems twice)
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
	for i := range d.Items {
		iw := d.itemWidth(i)
		spans = append(spans, itemSpan{start: x, end: x + iw, index: i})
		if i < len(d.Items)-1 {
			x += iw + d.separatorWidthAt(i+1)
		} else {
			x += iw
		}
	}

	// Compute icon rune count for each item (to colorize icon cells)
	iconWidths := make([]int, len(d.Items))
	for i, item := range d.Items {
		iconWidths[i] = utf8.RuneCountInString(item.Icon)
	}

	// Compute extended hover span (includes adjacent separator whitespace)
	hoverStart, hoverEnd := -1, -1
	if hoverIdx >= 0 && hoverIdx < len(spans) {
		hoverStart = spans[hoverIdx].start
		hoverEnd = spans[hoverIdx].end
		// In icons-only mode, the item is already padded — no extension needed
		if !d.IconsOnly {
			// Extend into left separator, but not across the divider
			if hoverIdx > 0 && !d.isDividerBoundary(hoverIdx) {
				sepW := d.separatorWidthAt(hoverIdx)
				hoverStart -= sepW / 2
			}
			// Extend into right separator
			if hoverIdx < len(spans)-1 {
				sepW := d.separatorWidthAt(hoverIdx + 1)
				hoverEnd += sepW / 2
			}
		}
	}

	// Fill cells directly from pre-computed content (avoids duplicate renderItems via Render)
	col := 0
	// Left padding
	for col < padding && col < width {
		cells[col] = DockCell{Char: ' '}
		col++
	}
	// Content characters
	for _, ch := range content {
		if col >= width {
			break
		}
		cell := DockCell{Char: ch}
		inSpan := false
		for _, span := range spans {
			if col >= span.start && col < span.end {
				inSpan = true
				if span.index == hoverIdx {
					cell.Accent = true
				}
				item := d.Items[span.index]
				if item.Stuck {
					cell.Stuck = true
				}
				if item.HasBell {
					cell.HasBell = true
				}
				if item.HasActivity {
					cell.HasActivity = true
				}
				if item.Special == "minimized" {
					cell.Minimized = true
				} else if item.Special == "running" {
					cell.Running = true
				} else if item.Special != "" {
					cell.Special = true
				}
				if item.WindowID != "" && item.WindowID == d.FocusedWindowID {
					cell.Active = true
				}
				localX := col - span.start
				iconOff := 0
				if item.Icon != "" {
					iconOff = 1 // skip leading " " padding
				}
				if localX >= iconOff && localX < iconOff+iconWidths[span.index] {
					cell.IconColor = item.IconColor
				}
				break
			}
		}
		// Extend hover accent into separator whitespace around hovered item
		if !inSpan && hoverIdx >= 0 && col >= hoverStart && col < hoverEnd {
			cell.Accent = true
		}
		if !inSpan && ch != ' ' {
			cell.Separator = true
		}
		cells[col] = cell
		col++
	}
	// Right padding
	for col < width {
		cells[col] = DockCell{Char: ' '}
		col++
	}

	return cells
}

func (d *Dock) renderItems() string {
	var sb strings.Builder
	spinner := activitySpinnerFrame()

	normalSep := string('\u2502') // "│"
	if d.IconsOnly {
		normalSep = "" // padding is part of the item in icons-only mode
	}
	dividerSep := string('\u2502') // "│" between shortcuts and windows

	for i, item := range d.Items {
		if i > 0 {
			// Use divider separator at the boundary between shortcuts and window items
			if d.isDividerBoundary(i) {
				sb.WriteString(dividerSep)
			} else {
				sb.WriteString(normalSep)
			}
		}
		label := item.Label
		if item.HasBell {
			label = "\U000F0027 " + label // 󰀧 bell icon prefix
		} else if item.HasActivity {
			label = string(spinner) + " " + label
		}
		if item.Icon == "" {
			sb.WriteString(" " + label + " ")
		} else if d.IconsOnly {
			sb.WriteString(" " + item.Icon + "  ")
		} else {
			sb.WriteString(" " + item.Icon + "  " + label + " ")
		}
	}
	return sb.String()
}

// isDividerBoundary returns true if a divider should appear before item at index i
// (i.e., between the last shortcut/expose and the first window list entry).
func (d *Dock) isDividerBoundary(i int) bool {
	if i <= 0 || i >= len(d.Items) {
		return false
	}
	cur := d.Items[i]
	prev := d.Items[i-1]
	isWindow := cur.Special == "minimized" || cur.Special == "running"
	prevIsShortcut := prev.Special != "minimized" && prev.Special != "running"
	return isWindow && prevIsShortcut
}

func (d *Dock) separatorWidth() int {
	if d.IconsOnly {
		return 0 // padding is part of the item in icons-only mode
	}
	return 1 // "│"
}

// separatorWidthAt returns the separator width before item at index i.
func (d *Dock) separatorWidthAt(i int) int {
	if d.isDividerBoundary(i) {
		return 1 // "│" divider
	}
	return d.separatorWidth()
}

func (d *Dock) itemPositions() []int {
	positions := make([]int, len(d.Items))
	// Compute total content width from item widths + separators (avoids renderItems call)
	totalW := 0
	for i := range d.Items {
		totalW += d.itemWidth(i)
		if i > 0 {
			totalW += d.separatorWidthAt(i)
		}
	}
	padding := (d.Width - totalW) / 2
	if padding < 0 {
		padding = 0
	}

	x := padding
	for i := range d.Items {
		positions[i] = x
		w := d.itemWidth(i)
		if i < len(d.Items)-1 {
			x += w + d.separatorWidthAt(i+1)
		} else {
			x += w
		}
	}
	return positions
}

func (d *Dock) itemWidth(idx int) int {
	if idx < 0 || idx >= len(d.Items) {
		return 0
	}
	item := d.Items[idx]
	// Bell prefix: "󰀧 " (2 chars), Activity prefix: "⠋ " (2 chars)
	extra := 0
	if item.HasBell {
		extra = 2
	} else if item.HasActivity {
		extra = 2
	}
	if item.Icon == "" {
		// " label " = label + 2
		return utf8.RuneCountInString(item.Label) + extra + 2
	}
	if d.IconsOnly {
		// " icon  " = runeCount + 3
		return utf8.RuneCountInString(item.Icon) + 3
	}
	// " icon  label " = runeCount + 2 (gap) + label + 2 (padding)
	return utf8.RuneCountInString(item.Icon) + utf8.RuneCountInString(item.Label) + 4 + extra
}

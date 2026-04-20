package dock

// SetWidth updates the dock width.
func (d *Dock) SetWidth(w int) {
	d.Width = w
}

// ItemCount returns the number of dock items.
func (d *Dock) ItemCount() int {
	return len(d.Items)
}

// ItemAtX returns the dock item index at the given X position, or -1.
// The hit area for each item extends into the adjacent separator whitespace,
// so clicking between icons still selects the nearest item.
func (d *Dock) ItemAtX(x int) int {
	positions := d.itemPositions()
	n := len(positions)
	for i, pos := range positions {
		itemW := d.itemWidth(i)
		hitStart := pos
		hitEnd := pos + itemW

		// In icons-only mode, the item is already padded — no extension needed
		if !d.IconsOnly {
			// Extend hit area into left separator (half), but not across the divider
			if i > 0 && !d.isDividerBoundary(i) {
				sepW := d.separatorWidthAt(i)
				hitStart -= sepW / 2
			}
			// Extend hit area into right separator (half)
			if i < n-1 {
				sepW := d.separatorWidthAt(i + 1)
				hitEnd += sepW / 2
			}
		}

		if x >= hitStart && x < hitEnd {
			return i
		}
	}
	return -1
}

// ItemCenterX returns the horizontal center of the dock item at the given index.
func (d *Dock) ItemCenterX(idx int) int {
	positions := d.itemPositions()
	if idx < 0 || idx >= len(positions) {
		return d.Width / 2
	}
	return positions[idx] + d.itemWidth(idx)/2
}

// SetHover sets the hover index.
func (d *Dock) SetHover(idx int) {
	if idx >= -1 && idx < len(d.Items) {
		d.HoverIndex = idx
	}
}

package clipboard

import (
	"strings"

	"github.com/icex/termdesk/pkg/ringbuf"
)

const historyCapacity = 5

// Clipboard manages a ring buffer of copied text with a history picker UI.
type Clipboard struct {
	history     *ringbuf.RingBuf[string]
	Visible     bool // whether the history picker is shown
	SelectedIdx int
}

// New creates a new clipboard manager.
func New() *Clipboard {
	return &Clipboard{
		history: ringbuf.New[string](historyCapacity),
	}
}

// Copy adds text to the clipboard history.
func (c *Clipboard) Copy(text string) {
	if text == "" {
		return
	}
	c.history.Push(text)
}

// Paste returns the most recent clipboard entry, or empty string.
func (c *Clipboard) Paste() string {
	v, ok := c.history.Latest()
	if !ok {
		return ""
	}
	return v
}

// PasteSelected returns the currently selected entry in the history picker.
func (c *Clipboard) PasteSelected() string {
	items := c.historyNewestFirst()
	if c.SelectedIdx >= 0 && c.SelectedIdx < len(items) {
		return items[c.SelectedIdx]
	}
	return ""
}

// History returns all clipboard entries from newest to oldest.
func (c *Clipboard) historyNewestFirst() []string {
	items := c.history.Items()
	// Reverse: newest first
	for i, j := 0, len(items)-1; i < j; i, j = i+1, j-1 {
		items[i], items[j] = items[j], items[i]
	}
	return items
}

// Len returns the number of items in the clipboard history.
func (c *Clipboard) Len() int {
	return c.history.Len()
}

// ShowHistory opens the history picker overlay.
func (c *Clipboard) ShowHistory() {
	c.Visible = true
	c.SelectedIdx = 0
}

// HideHistory closes the history picker.
func (c *Clipboard) HideHistory() {
	c.Visible = false
}

// ToggleHistory toggles the history picker.
func (c *Clipboard) ToggleHistory() {
	if c.Visible {
		c.HideHistory()
	} else {
		c.ShowHistory()
	}
}

// MoveSelection moves the selection in the history picker.
func (c *Clipboard) MoveSelection(delta int) {
	count := c.history.Len()
	if count == 0 {
		return
	}
	c.SelectedIdx += delta
	if c.SelectedIdx < 0 {
		c.SelectedIdx = count - 1
	}
	if c.SelectedIdx >= count {
		c.SelectedIdx = 0
	}
}

// Render returns the clipboard history picker as lines for overlay rendering.
func (c *Clipboard) Render(width, height int) []string {
	boxW := min(50, width-4)
	if boxW < 20 {
		boxW = 20
	}
	innerW := boxW - 2

	var lines []string

	// Top border
	lines = append(lines, "┌"+strings.Repeat("─", innerW)+"┐")

	// Title
	title := " Clipboard History "
	if len([]rune(title)) > innerW {
		title = title[:innerW]
	}
	padR := innerW - len([]rune(title))
	lines = append(lines, "│"+title+strings.Repeat(" ", padR)+"│")

	// Separator
	lines = append(lines, "├"+strings.Repeat("─", innerW)+"┤")

	items := c.historyNewestFirst()
	if len(items) == 0 {
		empty := " No clipboard entries"
		for len([]rune(empty)) < innerW {
			empty += " "
		}
		lines = append(lines, "│"+empty+"│")
	}

	for i, item := range items {
		prefix := "  "
		if i == c.SelectedIdx {
			prefix = "> "
		}

		// Truncate long entries and show single line preview
		preview := strings.ReplaceAll(item, "\n", "↵")
		label := prefix + preview
		runes := []rune(label)
		if len(runes) > innerW {
			label = string(runes[:innerW-1]) + "…"
		}
		for len([]rune(label)) < innerW {
			label += " "
		}
		lines = append(lines, "│"+label+"│")
	}

	// Bottom border
	lines = append(lines, "└"+strings.Repeat("─", innerW)+"┘")

	return lines
}

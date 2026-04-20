package clipboard

import (
	"strings"

	"github.com/icex/termdesk/pkg/ringbuf"
)

const historyCapacity = 5

// Entry represents a clipboard buffer entry, optionally named.
type Entry struct {
	Text string
	Name string
}

// Clipboard manages a ring buffer of copied text with a history picker UI.
type Clipboard struct {
	history     *ringbuf.RingBuf[Entry]
	Visible     bool // whether the history picker is shown
	SelectedIdx int
	nextNameID  int
}

// New creates a new clipboard manager.
func New() *Clipboard {
	return &Clipboard{
		history: ringbuf.New[Entry](historyCapacity),
	}
}

// Copy adds text to the clipboard history.
func (c *Clipboard) Copy(text string) {
	if text == "" {
		return
	}
	c.history.Push(Entry{Text: text})
}

// Append adds text to the most recent clipboard entry, or creates a new one.
func (c *Clipboard) Append(text string) {
	if text == "" {
		return
	}
	v, ok := c.history.Latest()
	if !ok {
		c.history.Push(Entry{Text: text})
		return
	}
	v.Text += text
	c.history.ReplaceLast(v)
}

// Paste returns the most recent clipboard entry, or empty string.
func (c *Clipboard) Paste() string {
	v, ok := c.history.Latest()
	if !ok {
		return ""
	}
	return v.Text
}

// PasteSelected returns the currently selected entry in the history picker.
func (c *Clipboard) PasteSelected() string {
	items := c.HistoryEntries()
	if c.SelectedIdx >= 0 && c.SelectedIdx < len(items) {
		return items[c.SelectedIdx].Text
	}
	return ""
}

// HistoryItems returns all clipboard entries from newest to oldest.
func (c *Clipboard) HistoryItems() []string {
	entries := c.HistoryEntries()
	items := make([]string, 0, len(entries))
	for _, e := range entries {
		items = append(items, e.Text)
	}
	return items
}

// HistoryEntries returns all clipboard entries from newest to oldest.
func (c *Clipboard) HistoryEntries() []Entry {
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

	entries := c.HistoryEntries()
	items := make([]string, 0, len(entries))
	for _, e := range entries {
		items = append(items, e.Text)
	}
	if len(items) == 0 {
		empty := " No clipboard entries"
		for len([]rune(empty)) < innerW {
			empty += " "
		}
		lines = append(lines, "│"+empty+"│")
	}

	for i, item := range items {
		name := ""
		if i < len(entries) && entries[i].Name != "" {
			name = "[" + entries[i].Name + "] "
		}
		prefix := "  "
		if i == c.SelectedIdx {
			prefix = "> "
		}

		// Truncate long entries and show single line preview
		preview := strings.ReplaceAll(item, "\n", "↵")
		preview = name + preview

		// Reserve space for " d" indicator at the end
		maxPreviewWidth := innerW - 2 // reserve 2 chars for " d"
		label := prefix + preview
		runes := []rune(label)
		if len(runes) > maxPreviewWidth {
			label = string(runes[:maxPreviewWidth-1]) + "…"
		}

		// Pad and add delete indicator
		for len([]rune(label)) < maxPreviewWidth {
			label += " "
		}
		label += " d" // add delete key indicator

		lines = append(lines, "│"+label+"│")
	}

	// Help line
	if len(items) > 0 {
		help := " ↑↓:select Enter:paste v:view n:name N:clear d:delete Esc:close"
		helpRunes := []rune(help)
		if len(helpRunes) > innerW {
			help = string(helpRunes[:innerW])
		}
		for len([]rune(help)) < innerW {
			help += " "
		}
		lines = append(lines, "├"+strings.Repeat("─", innerW)+"┤")
		lines = append(lines, "│"+help+"│")
	}

	// Bottom border
	lines = append(lines, "└"+strings.Repeat("─", innerW)+"┘")

	return lines
}

// GetHistory returns the clipboard history as a slice (newest first).
func (c *Clipboard) GetHistory() []string {
	return c.HistoryItems()
}

// RestoreHistory replaces clipboard history with provided items.
func (c *Clipboard) RestoreHistory(items []string) {
	c.history = ringbuf.New[Entry](historyCapacity)
	for i := len(items) - 1; i >= 0; i-- {
		c.history.Push(Entry{Text: items[i]})
	}
	c.SelectedIdx = 0
}

// DeleteItem removes an item from the clipboard history by index (0 = newest).
func (c *Clipboard) DeleteItem(idx int) {
	items := c.HistoryEntries()
	if idx < 0 || idx >= len(items) {
		return
	}

	// Remove the item at index
	newItems := append(items[:idx], items[idx+1:]...)

	// Rebuild the ring buffer
	c.history = ringbuf.New[Entry](historyCapacity)
	for i := len(newItems) - 1; i >= 0; i-- {
		c.history.Push(newItems[i])
	}
}

// SetSelectedName sets a custom name for an entry by browser index (0 = newest).
func (c *Clipboard) SetSelectedName(idx int, name string) {
	items := c.HistoryEntries()
	if idx < 0 || idx >= len(items) {
		return
	}
	items[idx].Name = strings.TrimSpace(name)
	c.rebuildFromEntries(items)
}

// AutoNameSelected sets a generated name for a selected entry.
func (c *Clipboard) AutoNameSelected(idx int) {
	c.nextNameID++
	c.SetSelectedName(idx, "buf-"+itoa(c.nextNameID))
}

// SelectedName returns the name of selected entry, if any.
func (c *Clipboard) SelectedName() string {
	items := c.HistoryEntries()
	if c.SelectedIdx < 0 || c.SelectedIdx >= len(items) {
		return ""
	}
	return items[c.SelectedIdx].Name
}

func (c *Clipboard) rebuildFromEntries(items []Entry) {
	c.history = ringbuf.New[Entry](historyCapacity)
	for i := len(items) - 1; i >= 0; i-- {
		c.history.Push(items[i])
	}
}

func itoa(v int) string {
	if v == 0 {
		return "0"
	}
	var b [20]byte
	i := len(b)
	for v > 0 {
		i--
		b[i] = byte('0' + (v % 10))
		v /= 10
	}
	return string(b[i:])
}

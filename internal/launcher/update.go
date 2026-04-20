package launcher

import "strings"

// Show makes the launcher visible and resets state.
func (l *Launcher) Show() {
	l.Visible = true
	l.Query = ""
	l.historyIdx = -1
	l.SelectedIdx = 0
	l.updateResults()
}

// Hide hides the launcher.
func (l *Launcher) Hide() {
	l.Visible = false
	l.Query = ""
	l.historyIdx = -1
}

// Toggle toggles the launcher visibility.
func (l *Launcher) Toggle() {
	if l.Visible {
		l.Hide()
	} else {
		l.Show()
	}
}

// SetQuery updates the search query and filters results.
func (l *Launcher) SetQuery(q string) {
	l.Query = q
	l.historyIdx = -1
	l.SelectedIdx = 0
	l.updateResults()
}

// TypeChar appends a character to the query.
func (l *Launcher) TypeChar(ch rune) {
	l.Query += string(ch)
	l.historyIdx = -1
	l.SelectedIdx = 0
	l.updateResults()
}

// Backspace removes the last character from the query.
func (l *Launcher) Backspace() {
	if len(l.Query) > 0 {
		runes := []rune(l.Query)
		l.Query = string(runes[:len(runes)-1])
		l.historyIdx = -1
		l.SelectedIdx = 0
		l.updateResults()
	}
}

// MoveSelection moves the selection up or down.
func (l *Launcher) MoveSelection(delta int) {
	if len(l.Results) == 0 {
		return
	}
	l.SelectedIdx += delta
	if l.SelectedIdx < 0 {
		l.SelectedIdx = len(l.Results) - 1
	}
	if l.SelectedIdx >= len(l.Results) {
		l.SelectedIdx = 0
	}
}

// SelectedEntry returns the currently selected entry, or nil.
func (l *Launcher) SelectedEntry() *AppEntry {
	if l.SelectedIdx >= 0 && l.SelectedIdx < len(l.Results) {
		entry := l.Results[l.SelectedIdx]
		return &entry
	}
	return nil
}

// RecordLaunch records a command as recently launched. Keeps at most 5.
func (l *Launcher) RecordLaunch(command string) {
	// Remove existing occurrence
	filtered := make([]string, 0, len(l.RecentApps))
	for _, c := range l.RecentApps {
		if c != command {
			filtered = append(filtered, c)
		}
	}
	// Prepend
	l.RecentApps = append([]string{command}, filtered...)
	if len(l.RecentApps) > 5 {
		l.RecentApps = l.RecentApps[:5]
	}
}

// ToggleFavorite toggles the favorite status of the currently selected entry.
// Returns true if the entry was favorited.
func (l *Launcher) ToggleFavorite() bool {
	entry := l.SelectedEntry()
	if entry == nil {
		return false
	}
	if l.Favorites[entry.Command] {
		delete(l.Favorites, entry.Command)
		return false
	}
	l.Favorites[entry.Command] = true
	l.updateResults()
	return true
}

// IsFavorite returns true if the command is favorited.
func (l *Launcher) IsFavorite(command string) bool {
	return l.Favorites[command]
}

// RecordQuery stores a non-empty launcher query in history (newest first).
func (l *Launcher) RecordQuery(query string) {
	query = strings.TrimSpace(query)
	if query == "" {
		return
	}
	var filtered []string
	for _, q := range l.QueryHistory {
		if q != query {
			filtered = append(filtered, q)
		}
	}
	l.QueryHistory = append([]string{query}, filtered...)
	if len(l.QueryHistory) > 20 {
		l.QueryHistory = l.QueryHistory[:20]
	}
	l.historyIdx = -1
}

// PrevQuery moves backward in prompt history and loads that query.
func (l *Launcher) PrevQuery() {
	if len(l.QueryHistory) == 0 {
		return
	}
	if l.historyIdx < len(l.QueryHistory)-1 {
		l.historyIdx++
	}
	l.Query = l.QueryHistory[l.historyIdx]
	l.SelectedIdx = 0
	l.updateResults()
}

// NextQuery moves forward in prompt history and loads that query.
func (l *Launcher) NextQuery() {
	if len(l.QueryHistory) == 0 {
		return
	}
	if l.historyIdx > 0 {
		l.historyIdx--
		l.Query = l.QueryHistory[l.historyIdx]
	} else {
		l.historyIdx = -1
		l.Query = ""
	}
	l.SelectedIdx = 0
	l.updateResults()
}

// CompleteFromSelected sets the query to the selected app name.
func (l *Launcher) CompleteFromSelected() bool {
	entry := l.SelectedEntry()
	if entry == nil {
		return false
	}
	if entry.Source == "exec" {
		l.Query = entry.Command
	} else {
		l.Query = entry.Name
	}
	l.historyIdx = -1
	l.SelectedIdx = 0
	l.updateResults()
	return true
}

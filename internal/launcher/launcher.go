package launcher

import (
	"os"
	"os/exec"
	"sort"
	"strings"
)

// AppEntry represents a launchable application.
type AppEntry struct {
	Name    string
	Icon    string
	Command string
	Args    []string
}

// Launcher is the command palette / app launcher overlay.
type Launcher struct {
	Visible     bool
	Query       string
	Registry    []AppEntry
	Results     []AppEntry
	SelectedIdx int
	Width       int
	Height      int
}

// New creates a launcher with default apps and scans $PATH.
func New() *Launcher {
	l := &Launcher{
		Registry: defaultApps(),
	}
	l.scanPath()
	l.updateResults()
	return l
}

// Show makes the launcher visible and resets state.
func (l *Launcher) Show() {
	l.Visible = true
	l.Query = ""
	l.SelectedIdx = 0
	l.updateResults()
}

// Hide hides the launcher.
func (l *Launcher) Hide() {
	l.Visible = false
	l.Query = ""
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
	l.SelectedIdx = 0
	l.updateResults()
}

// TypeChar appends a character to the query.
func (l *Launcher) TypeChar(ch rune) {
	l.Query += string(ch)
	l.SelectedIdx = 0
	l.updateResults()
}

// Backspace removes the last character from the query.
func (l *Launcher) Backspace() {
	if len(l.Query) > 0 {
		runes := []rune(l.Query)
		l.Query = string(runes[:len(runes)-1])
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

// Render returns the launcher overlay as lines.
func (l *Launcher) Render(width, height int) []string {
	boxW := min(60, width-4)
	if boxW < 20 {
		boxW = 20
	}
	innerW := boxW - 2

	var lines []string

	// Top border
	lines = append(lines, "┌"+strings.Repeat("─", innerW)+"┐")

	// Search bar
	prompt := " > " + l.Query + "█"
	if len([]rune(prompt)) > innerW {
		prompt = prompt[:innerW]
	}
	for len([]rune(prompt)) < innerW {
		prompt += " "
	}
	lines = append(lines, "│"+prompt+"│")

	// Separator
	lines = append(lines, "├"+strings.Repeat("─", innerW)+"┤")

	// Results (show up to 8)
	maxResults := min(8, len(l.Results))
	if maxResults == 0 {
		empty := " No results"
		for len([]rune(empty)) < innerW {
			empty += " "
		}
		lines = append(lines, "│"+empty+"│")
	}
	for i := 0; i < maxResults; i++ {
		entry := l.Results[i]
		prefix := "  "
		if i == l.SelectedIdx {
			prefix = "> "
		}
		label := prefix + entry.Icon + " " + entry.Name
		if len([]rune(label)) > innerW {
			label = string([]rune(label)[:innerW])
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

func (l *Launcher) updateResults() {
	if l.Query == "" {
		l.Results = make([]AppEntry, len(l.Registry))
		copy(l.Results, l.Registry)
		return
	}

	q := strings.ToLower(l.Query)
	type scored struct {
		entry AppEntry
		score int
	}
	var matches []scored
	for _, e := range l.Registry {
		name := strings.ToLower(e.Name)
		if strings.Contains(name, q) {
			s := 0
			if strings.HasPrefix(name, q) {
				s = 2 // prefix bonus
			} else {
				s = 1
			}
			matches = append(matches, scored{e, s})
		}
	}

	sort.Slice(matches, func(i, j int) bool {
		return matches[i].score > matches[j].score
	})

	l.Results = make([]AppEntry, len(matches))
	for i, m := range matches {
		l.Results[i] = m.entry
	}
}

func (l *Launcher) scanPath() {
	// Scan $PATH for known tools
	externalApps := []struct {
		name string
		icon string
	}{
		{"nvim", ""},
		{"spf", ""},
		{"claude", "󱜙"},
		{"tetrigo", ""},
		{"mc", ""},
		{"htop", ""},
		{"btop", ""},
	}

	for _, app := range externalApps {
		if _, err := exec.LookPath(app.name); err == nil {
			// Check if already in registry
			found := false
			for _, e := range l.Registry {
				if e.Command == app.name {
					found = true
					break
				}
			}
			if !found {
				l.Registry = append(l.Registry, AppEntry{
					Name:    app.name,
					Icon:    app.icon,
					Command: app.name,
				})
			}
		}
	}
}

func defaultApps() []AppEntry {
	shell := os.Getenv("SHELL")
	if shell == "" {
		shell = "/bin/sh"
	}
	return []AppEntry{
		{Name: "Terminal", Icon: "", Command: shell},
		{Name: "nvim", Icon: "", Command: "nvim"},
		{Name: "Files (spf)", Icon: "", Command: "spf"},
		{Name: "Calculator", Icon: "", Command: "calc"},
	}
}

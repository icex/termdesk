package config

import (
	"os"
	"path/filepath"
	"strings"
)

// ProjectConfig represents a .termdesk.toml project configuration.
type ProjectConfig struct {
	ProjectDir string          // absolute path where .termdesk.toml was found
	AutoStart  []AutoStartItem // commands to auto-start
	DockItems  []DockEntry     // custom dock shortcuts
}

// AutoStartItem represents a single auto-start command.
type AutoStartItem struct {
	Command   string   // command to run
	Args      []string // arguments
	Directory string   // working directory (relative to project root)
	Title     string   // window title
}

// DockEntry represents a custom dock shortcut from project config.
type DockEntry struct {
	Name      string   // display label
	Icon      string   // Nerd Font icon (optional)
	IconColor string   // hex color (optional)
	Command   string   // command to run
	Args      []string // arguments
	Position  int      // 0 = append at end, N = insert at position N among app items
}

// FindProjectConfig searches for .termdesk.toml starting from startDir
// and walking up to the user's home directory.
// Returns nil if not found.
func FindProjectConfig(startDir string) (*ProjectConfig, error) {
	if startDir == "" {
		var err error
		startDir, err = os.Getwd()
		if err != nil {
			return nil, err
		}
	}

	// Make absolute
	startDir, err := filepath.Abs(startDir)
	if err != nil {
		return nil, err
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}

	dir := startDir
	for {
		configPath := filepath.Join(dir, ".termdesk.toml")
		if _, err := os.Stat(configPath); err == nil {
			// Found it!
			data, err := os.ReadFile(configPath)
			if err != nil {
				return nil, err
			}

			config := &ProjectConfig{
				ProjectDir: dir,
			}
			parseProjectConfig(config, string(data))
			return config, nil
		}

		// Move up one directory
		parent := filepath.Dir(dir)
		if parent == dir || dir == home {
			// Reached root or home, stop
			break
		}
		dir = parent
	}

	return nil, nil // not found
}

// parseProjectConfig parses .termdesk.toml content into ProjectConfig.
func parseProjectConfig(config *ProjectConfig, content string) {
	lines := strings.Split(content, "\n")
	var currentItem *AutoStartItem
	var currentDock *DockEntry
	section := "" // "autostart" or "dock"

	flush := func() {
		if currentItem != nil {
			config.AutoStart = append(config.AutoStart, *currentItem)
			currentItem = nil
		}
		if currentDock != nil {
			config.DockItems = append(config.DockItems, *currentDock)
			currentDock = nil
		}
	}

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		if line == "[[autostart]]" {
			flush()
			section = "autostart"
			currentItem = &AutoStartItem{
				Directory: ".", // default to project root
			}
			continue
		}

		if line == "[[dock]]" {
			flush()
			section = "dock"
			currentDock = &DockEntry{}
			continue
		}

		key, val, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		key = strings.TrimSpace(key)
		val = strings.TrimSpace(val)

		switch section {
		case "autostart":
			if currentItem == nil {
				continue
			}
			switch key {
			case "command":
				currentItem.Command = extractQuotedValue(val)
			case "args":
				currentItem.Args = parseStringArray(extractArrayValue(val))
			case "directory":
				currentItem.Directory = extractQuotedValue(val)
			case "title":
				currentItem.Title = extractQuotedValue(val)
			}
		case "dock":
			if currentDock == nil {
				continue
			}
			switch key {
			case "name":
				currentDock.Name = extractQuotedValue(val)
			case "icon":
				currentDock.Icon = extractQuotedValue(val)
			case "icon_color":
				currentDock.IconColor = extractQuotedValue(val)
			case "command":
				currentDock.Command = extractQuotedValue(val)
			case "args":
				currentDock.Args = parseStringArray(extractArrayValue(val))
			case "position":
				currentDock.Position = parseIntSafe(extractQuotedValue(val))
			}
		}
	}

	flush()
}

// extractQuotedValue extracts the first quoted string from a TOML value.
// Handles escaped quotes within the string. If the value is not quoted,
// returns it as-is (for numbers, bools, etc.).
func extractQuotedValue(s string) string {
	s = strings.TrimSpace(s)
	if !strings.HasPrefix(s, "\"") {
		return s
	}
	s = s[1:] // strip leading quote
	for i := 0; i < len(s); i++ {
		if s[i] == '\\' {
			i++ // skip escaped char
			continue
		}
		if s[i] == '"' {
			return s[:i]
		}
	}
	return s // no closing quote, return as-is
}

// extractArrayValue extracts the first bracketed array from a TOML value.
func extractArrayValue(s string) string {
	s = strings.TrimSpace(s)
	if !strings.HasPrefix(s, "[") {
		return s
	}
	depth := 0
	for i := 0; i < len(s); i++ {
		if s[i] == '[' {
			depth++
		}
		if s[i] == ']' {
			depth--
			if depth == 0 {
				return s[:i+1]
			}
		}
	}
	return s
}

func parseStringArray(s string) []string {
	s = strings.TrimPrefix(s, "[")
	s = strings.TrimSuffix(s, "]")
	if s == "" {
		return nil
	}
	var result []string
	var current strings.Builder
	inQuote := false
	escaped := false
	for _, ch := range s {
		if escaped {
			current.WriteRune(ch)
			escaped = false
			continue
		}
		if ch == '\\' && inQuote {
			escaped = true
			continue
		}
		if ch == '"' {
			inQuote = !inQuote
			continue
		}
		if ch == ',' && !inQuote {
			result = append(result, strings.TrimSpace(current.String()))
			current.Reset()
			continue
		}
		current.WriteRune(ch)
	}
	if current.Len() > 0 || len(result) > 0 {
		result = append(result, strings.TrimSpace(current.String()))
	}
	return result
}

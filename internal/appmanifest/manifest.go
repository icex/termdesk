package appmanifest

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// AppManifest describes an app's metadata and window preferences.
type AppManifest struct {
	App    AppInfo
	Window WindowInfo
}

// AppInfo holds display metadata for a termdesk app.
type AppInfo struct {
	Name      string
	Icon      string
	IconColor string
	Category  string // e.g. "games", "" = default
}

// WindowInfo holds window creation preferences.
type WindowInfo struct {
	Width     int   // 0 = default
	Height    int   // 0 = default
	Resizable *bool // nil = default (true)
	Maximized bool
}

// IsResizable returns whether the window should be resizable (default true).
func (w WindowInfo) IsResizable() bool {
	if w.Resizable == nil {
		return true
	}
	return *w.Resizable
}

// IsFixedSize returns true if both width and height are set.
func (w WindowInfo) IsFixedSize() bool {
	return w.Width > 0 && w.Height > 0
}

// LoadManifest reads a manifest TOML file next to the given binary path.
// For a binary at /usr/bin/termdesk-calc, it looks for /usr/bin/termdesk-calc.toml.
// Returns nil if not found.
func LoadManifest(binaryPath string) *AppManifest {
	tomlPath := binaryPath + ".toml"
	data, err := os.ReadFile(tomlPath)
	if err != nil {
		return nil
	}
	m := &AppManifest{}
	parseManifest(m, string(data))
	return m
}

// LoadManifestsInDir scans a directory for all *.toml app manifests.
// Returns manifests keyed by command name (e.g. "termdesk-calc").
func LoadManifestsInDir(dir string) map[string]*AppManifest {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	result := make(map[string]*AppManifest)
	for _, e := range entries {
		name := e.Name()
		if !strings.HasSuffix(name, ".toml") {
			continue
		}
		// Derive command name from filename: termdesk-calc.toml → termdesk-calc
		cmdName := strings.TrimSuffix(name, ".toml")
		data, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			continue
		}
		m := &AppManifest{}
		parseManifest(m, string(data))
		if m.App.Name == "" {
			m.App.Name = cmdName
		}
		result[cmdName] = m
	}
	return result
}

func parseManifest(m *AppManifest, data string) {
	section := ""
	for _, line := range strings.Split(data, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			section = strings.TrimSpace(line[1 : len(line)-1])
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}
		key := strings.TrimSpace(parts[0])
		val := strings.TrimSpace(parts[1])
		val = strings.Trim(val, "\"")

		switch section {
		case "app":
			switch key {
			case "name":
				m.App.Name = val
			case "icon":
				m.App.Icon = unescapeIcon(val)
			case "icon_color":
				m.App.IconColor = val
			case "category":
				m.App.Category = val
			}
		case "window":
			switch key {
			case "width":
				if v, err := strconv.Atoi(val); err == nil {
					m.Window.Width = v
				}
			case "height":
				if v, err := strconv.Atoi(val); err == nil {
					m.Window.Height = v
				}
			case "resizable":
				b := val == "true"
				m.Window.Resizable = &b
			case "maximized":
				m.Window.Maximized = val == "true"
			}
		}
	}
}

// unescapeIcon converts \uXXXX sequences in a TOML string to actual runes.
func unescapeIcon(s string) string {
	if !strings.Contains(s, "\\u") {
		return s
	}
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		if i+5 < len(s) && s[i] == '\\' && s[i+1] == 'u' {
			hex := s[i+2 : i+6]
			if v, err := strconv.ParseInt(hex, 16, 32); err == nil {
				b.WriteRune(rune(v))
				i += 5
				continue
			}
		}
		b.WriteByte(s[i])
	}
	return b.String()
}

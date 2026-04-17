package registry

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/icex/termdesk/internal/appmanifest"
)

// RegistryEntry is a unified app entry used by dock, launcher, and menu bar.
type RegistryEntry struct {
	Name      string
	Icon      string
	IconColor string
	Command   string
	Args      []string
	Window    appmanifest.WindowInfo
	Category  string // e.g. "games", "" = default
}

// BuildRegistry creates the unified app list from built-in defaults,
// PATH scanning, and discovered manifests next to exePath.
func BuildRegistry(exePath string) []RegistryEntry {
	manifests := make(map[string]*appmanifest.AppManifest)
	if exePath != "" {
		dir := filepath.Dir(exePath)
		manifests = appmanifest.LoadManifestsInDir(dir)
	}

	var entries []RegistryEntry

	// 1. Terminal (always first)
	shell := os.Getenv("SHELL")
	if shell == "" {
		shell = "/bin/sh"
	}
	entries = append(entries, RegistryEntry{
		Name:      "Terminal",
		Icon:      "\uf120",
		IconColor: "#98C379",
		Command:   "$SHELL",
	})

	// 2. File manager: prefer yazi, fallback to mc
	for _, fm := range []string{"yazi", "mc"} {
		if _, err := exec.LookPath(fm); err == nil {
			entries = append(entries, RegistryEntry{
				Name:      "Files",
				Icon:      "\uf07b",
				IconColor: "#E5C07B",
				Command:   fm,
			})
			break
		}
	}

	// 3. Known apps from PATH with default icons
	knownApps := []struct {
		cmd       string
		name      string
		icon      string
		iconColor string
		args      []string
		window    appmanifest.WindowInfo // default window prefs
	}{
		{"carbonyl", "Browser", "\uf0ac", "#4285F4", []string{"https://google.com"}, appmanifest.WindowInfo{Maximized: true}},
		{"nvim", "nvim", "\ue62b", "#61AFEF", nil, appmanifest.WindowInfo{Maximized: true}},
		{"htop", "System Monitor", "\uf200", "#E06C75", nil, appmanifest.WindowInfo{Maximized: false}},
		{"btop", "btop", "\uf200", "#E06C75", nil, appmanifest.WindowInfo{Maximized: false}},
		{"spf", "Superfile", "\uf07b", "#98C379", nil, appmanifest.WindowInfo{Maximized: false}},
		{"ranger", "ranger", "\uf07b", "#E5C07B", nil, appmanifest.WindowInfo{Maximized: false}},
		{"lf", "lf", "\uf07b", "#E5C07B", nil, appmanifest.WindowInfo{Maximized: false}},
		{"claude", "claude", "\U000f0619", "#89B4FA", nil, appmanifest.WindowInfo{}},
		{"tetrigo", "tetrigo", "\uf11b", "#F38BA8", nil, appmanifest.WindowInfo{}},
	}

	for _, app := range knownApps {
		if _, err := exec.LookPath(app.cmd); err != nil {
			continue
		}
		entries = append(entries, RegistryEntry{
			Name:      app.name,
			Icon:      app.icon,
			IconColor: app.iconColor,
			Command:   app.cmd,
			Args:      app.args,
			Window:    app.window,
		})
	}

	// 4. Discovered manifest apps (termdesk-*)
	for cmdName, m := range manifests {
		// Resolve to full path next to exePath
		fullPath := filepath.Join(filepath.Dir(exePath), cmdName)
		if _, err := os.Stat(fullPath); err != nil {
			// Also try PATH
			if p, err := exec.LookPath(cmdName); err == nil {
				fullPath = p
			} else {
				continue // binary not found, skip
			}
		}
		entries = append(entries, RegistryEntry{
			Name:      m.App.Name,
			Icon:      m.App.Icon,
			IconColor: m.App.IconColor,
			Command:   cmdName,
			Args:      nil,
			Window:    m.Window,
			Category:  m.App.Category,
		})
	}

	return entries
}

// FindBinary resolves a command name to a full path.
// Checks exePath's directory first, then PATH.
func FindBinary(exePath, name string) string {
	if exePath != "" {
		candidate := filepath.Join(filepath.Dir(exePath), name)
		if _, err := os.Stat(candidate); err == nil {
			return candidate
		}
	}
	if p, err := exec.LookPath(name); err == nil {
		return p
	}
	return name
}

// ManifestForCommand loads the manifest for a given command, if one exists.
func ManifestForCommand(exePath, command string) *appmanifest.AppManifest {
	// Try next to exePath first
	if exePath != "" {
		binPath := filepath.Join(filepath.Dir(exePath), command)
		if m := appmanifest.LoadManifest(binPath); m != nil {
			return m
		}
	}
	// Try next to the command's own binary
	if p, err := exec.LookPath(command); err == nil {
		if m := appmanifest.LoadManifest(p); m != nil {
			return m
		}
	}
	// Try stripping path to get base name
	base := filepath.Base(command)
	if base != command && exePath != "" {
		binPath := filepath.Join(filepath.Dir(exePath), base)
		if m := appmanifest.LoadManifest(binPath); m != nil {
			return m
		}
	}
	return nil
}

// DockEntries returns registry entries suitable for the dock.
// Excludes manifest apps (termdesk-*) and uncommon PATH discoveries.
func DockEntries(entries []RegistryEntry) []RegistryEntry {
	var result []RegistryEntry
	for _, e := range entries {
		// Skip termdesk-* bundled apps (games, accessories)
		if strings.HasPrefix(e.Command, "termdesk-") {
			continue
		}
		// Skip less common PATH discoveries that shouldn't clutter the dock
		switch e.Command {
		case "spf", "claude", "tetrigo", "btop", "ranger", "lf":
			continue
		}
		result = append(result, e)
	}
	return result
}

// StripTermdeskPrefix returns a display name from a termdesk-* command.
func StripTermdeskPrefix(cmd string) string {
	base := filepath.Base(cmd)
	if strings.HasPrefix(base, "termdesk-") {
		return base[len("termdesk-"):]
	}
	return base
}

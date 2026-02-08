package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// KeyBindings holds configurable key mappings.
type KeyBindings struct {
	Prefix        string // prefix key for Terminal mode (default "ctrl+g")
	Quit          string // quit app (default "q")
	NewTerminal   string // open new terminal (default "n")
	CloseWindow   string // close focused window (default "w")
	EnterTerminal string // enter terminal mode (default "i")
	Minimize      string // minimize to dock (default "m")
	Rename        string // rename window (default "r")
	DockFocus     string // focus dock (default "d")
	Launcher      string // open launcher (default "space")
	SnapLeft      string // snap window left (default "h")
	SnapRight     string // snap window right (default "l")
	Maximize      string // maximize window (default "k")
	Restore       string // restore window (default "j")
	TileAll       string // tile all windows (default "t")
	Expose        string // enter expose mode (default "x")
	NextWindow    string // next window (default "tab")
	PrevWindow    string // previous window (default "shift+tab")
	Help          string // show help (default "f1")
	ToggleExpose  string // toggle expose (default "f9")
	MenuBar       string // open menu bar (default "f10")
	MenuFile      string // open File menu (default "f")
	MenuApps      string // open Apps menu (default "a")
	MenuView      string // open View menu (default "v")
}

// DefaultKeyBindings returns the default key bindings.
func DefaultKeyBindings() KeyBindings {
	return KeyBindings{
		Prefix:        "ctrl+g",
		Quit:          "q",
		NewTerminal:   "n",
		CloseWindow:   "w",
		EnterTerminal: "i",
		Minimize:      "m",
		Rename:        "r",
		DockFocus:     "d",
		Launcher:      "space",
		SnapLeft:      "h",
		SnapRight:     "l",
		Maximize:      "k",
		Restore:       "j",
		TileAll:       "t",
		Expose:        "x",
		NextWindow:    "tab",
		PrevWindow:    "shift+tab",
		Help:          "f1",
		ToggleExpose:  "f9",
		MenuBar:       "f10",
		MenuFile:      "f",
		MenuApps:      "a",
		MenuView:      "v",
	}
}

// UserConfig holds persistent user settings.
type UserConfig struct {
	Theme      string `toml:"theme"`
	IconsOnly  bool   `toml:"icons_only"`
	Animations bool   `toml:"animations"`
	Keys       KeyBindings
}

// DefaultUserConfig returns the default user configuration.
func DefaultUserConfig() UserConfig {
	return UserConfig{
		Theme:      "modern",
		IconsOnly:  false,
		Animations: true,
		Keys:       DefaultKeyBindings(),
	}
}

// configDir returns the path to ~/.config/termdesk/.
func configDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".config", "termdesk")
}

// configPath returns the path to the config file.
func configPath() string {
	dir := configDir()
	if dir == "" {
		return ""
	}
	return filepath.Join(dir, "config.toml")
}

// LoadUserConfig reads the user config from ~/.config/termdesk/config.toml.
// Returns default config if the file doesn't exist.
func LoadUserConfig() UserConfig {
	cfg := DefaultUserConfig()
	path := configPath()
	if path == "" {
		return cfg
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return cfg
	}

	parseConfig(&cfg, string(data))

	return cfg
}

// parseConfig parses TOML content with section support into cfg.
func parseConfig(cfg *UserConfig, data string) {
	section := ""
	for _, line := range strings.Split(data, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		// Section header
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
		case "":
			switch key {
			case "theme":
				cfg.Theme = val
			case "icons_only":
				cfg.IconsOnly = val == "true"
			case "animations":
				cfg.Animations = val == "true"
			}
		case "keybindings":
			parseKeybinding(&cfg.Keys, key, val)
		}
	}
}

// parseKeybinding sets a single keybinding field by name.
func parseKeybinding(kb *KeyBindings, key, val string) {
	if val == "" {
		return
	}
	switch key {
	case "prefix":
		kb.Prefix = val
	case "quit":
		kb.Quit = val
	case "new_terminal":
		kb.NewTerminal = val
	case "close_window":
		kb.CloseWindow = val
	case "enter_terminal":
		kb.EnterTerminal = val
	case "minimize":
		kb.Minimize = val
	case "rename":
		kb.Rename = val
	case "dock_focus":
		kb.DockFocus = val
	case "launcher":
		kb.Launcher = val
	case "snap_left":
		kb.SnapLeft = val
	case "snap_right":
		kb.SnapRight = val
	case "maximize":
		kb.Maximize = val
	case "restore":
		kb.Restore = val
	case "tile_all":
		kb.TileAll = val
	case "expose":
		kb.Expose = val
	case "next_window":
		kb.NextWindow = val
	case "prev_window":
		kb.PrevWindow = val
	case "help":
		kb.Help = val
	case "toggle_expose":
		kb.ToggleExpose = val
	case "menu_bar":
		kb.MenuBar = val
	case "menu_file":
		kb.MenuFile = val
	case "menu_apps":
		kb.MenuApps = val
	case "menu_view":
		kb.MenuView = val
	}
}

// SaveUserConfig writes the user config to ~/.config/termdesk/config.toml.
func SaveUserConfig(cfg UserConfig) error {
	dir := configDir()
	if dir == "" {
		return nil
	}

	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}

	var sb strings.Builder
	sb.WriteString("# Termdesk configuration\n\n")
	sb.WriteString("theme = \"")
	sb.WriteString(cfg.Theme)
	sb.WriteString("\"\n")
	if cfg.IconsOnly {
		sb.WriteString("icons_only = true\n")
	} else {
		sb.WriteString("icons_only = false\n")
	}
	if cfg.Animations {
		sb.WriteString("animations = true\n")
	} else {
		sb.WriteString("animations = false\n")
	}

	// Write keybindings section (only non-default values)
	defaults := DefaultKeyBindings()
	kb := cfg.Keys
	var kbLines []string
	if kb.Prefix != defaults.Prefix {
		kbLines = append(kbLines, fmt.Sprintf("prefix = %q", kb.Prefix))
	}
	if kb.Quit != defaults.Quit {
		kbLines = append(kbLines, fmt.Sprintf("quit = %q", kb.Quit))
	}
	if kb.NewTerminal != defaults.NewTerminal {
		kbLines = append(kbLines, fmt.Sprintf("new_terminal = %q", kb.NewTerminal))
	}
	if kb.CloseWindow != defaults.CloseWindow {
		kbLines = append(kbLines, fmt.Sprintf("close_window = %q", kb.CloseWindow))
	}
	if kb.EnterTerminal != defaults.EnterTerminal {
		kbLines = append(kbLines, fmt.Sprintf("enter_terminal = %q", kb.EnterTerminal))
	}
	if kb.Minimize != defaults.Minimize {
		kbLines = append(kbLines, fmt.Sprintf("minimize = %q", kb.Minimize))
	}
	if kb.Rename != defaults.Rename {
		kbLines = append(kbLines, fmt.Sprintf("rename = %q", kb.Rename))
	}
	if kb.DockFocus != defaults.DockFocus {
		kbLines = append(kbLines, fmt.Sprintf("dock_focus = %q", kb.DockFocus))
	}
	if kb.Launcher != defaults.Launcher {
		kbLines = append(kbLines, fmt.Sprintf("launcher = %q", kb.Launcher))
	}
	if kb.SnapLeft != defaults.SnapLeft {
		kbLines = append(kbLines, fmt.Sprintf("snap_left = %q", kb.SnapLeft))
	}
	if kb.SnapRight != defaults.SnapRight {
		kbLines = append(kbLines, fmt.Sprintf("snap_right = %q", kb.SnapRight))
	}
	if kb.Maximize != defaults.Maximize {
		kbLines = append(kbLines, fmt.Sprintf("maximize = %q", kb.Maximize))
	}
	if kb.Restore != defaults.Restore {
		kbLines = append(kbLines, fmt.Sprintf("restore = %q", kb.Restore))
	}
	if kb.TileAll != defaults.TileAll {
		kbLines = append(kbLines, fmt.Sprintf("tile_all = %q", kb.TileAll))
	}
	if kb.Expose != defaults.Expose {
		kbLines = append(kbLines, fmt.Sprintf("expose = %q", kb.Expose))
	}
	if kb.NextWindow != defaults.NextWindow {
		kbLines = append(kbLines, fmt.Sprintf("next_window = %q", kb.NextWindow))
	}
	if kb.PrevWindow != defaults.PrevWindow {
		kbLines = append(kbLines, fmt.Sprintf("prev_window = %q", kb.PrevWindow))
	}
	if kb.Help != defaults.Help {
		kbLines = append(kbLines, fmt.Sprintf("help = %q", kb.Help))
	}
	if kb.ToggleExpose != defaults.ToggleExpose {
		kbLines = append(kbLines, fmt.Sprintf("toggle_expose = %q", kb.ToggleExpose))
	}
	if kb.MenuBar != defaults.MenuBar {
		kbLines = append(kbLines, fmt.Sprintf("menu_bar = %q", kb.MenuBar))
	}
	if kb.MenuFile != defaults.MenuFile {
		kbLines = append(kbLines, fmt.Sprintf("menu_file = %q", kb.MenuFile))
	}
	if kb.MenuApps != defaults.MenuApps {
		kbLines = append(kbLines, fmt.Sprintf("menu_apps = %q", kb.MenuApps))
	}
	if kb.MenuView != defaults.MenuView {
		kbLines = append(kbLines, fmt.Sprintf("menu_view = %q", kb.MenuView))
	}
	if len(kbLines) > 0 {
		sb.WriteString("\n[keybindings]\n")
		for _, l := range kbLines {
			sb.WriteString(l + "\n")
		}
	}

	return os.WriteFile(configPath(), []byte(sb.String()), 0o644)
}

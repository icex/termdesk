package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// WorkspaceHistoryEntry tracks when a workspace was last accessed.
type WorkspaceHistoryEntry struct {
	Path       string    `toml:"path"`        // absolute path to workspace file
	LastAccess time.Time `toml:"last_access"` // when it was last loaded
}

// KeyBindings holds configurable key mappings.
type KeyBindings struct {
	Prefix             string // prefix key for Terminal mode (default "ctrl+a")
	Quit               string // quit app (default "q")
	NewTerminal        string // open new terminal (default "n")
	CloseWindow        string // close focused window (default "w")
	EnterTerminal      string // enter terminal mode (default "i")
	Minimize           string // minimize to dock (default "m")
	Rename             string // rename window (default "r")
	DockFocus          string // focus dock (default "d")
	Launcher           string // open launcher (default "space")
	SnapLeft           string // snap window left (default "h")
	SnapRight          string // snap window right (default "l")
	Maximize           string // maximize window (default "k")
	Restore            string // restore window (default "j")
	TileAll            string // tile all windows (default "t")
	Expose             string // enter expose mode (default "x")
	NextWindow         string // next window (default "tab")
	PrevWindow         string // previous window (default "shift+tab")
	Help               string // show help (default "f1")
	ToggleExpose       string // toggle expose (default "f9")
	MenuBar            string // open menu bar (default "f10")
	MenuFile           string // open File menu (default "f")
	MenuApps           string // open Apps menu (default "a")
	MenuView           string // open View menu (default "v")
	MoveLeft           string // move window left (default "shift+left")
	MoveRight          string // move window right (default "shift+right")
	MoveUp             string // move window up (default "shift+up")
	MoveDown           string // move window down (default "shift+down")
	GrowWidth          string // grow window width (default "alt+right")
	ShrinkWidth        string // shrink window width (default "alt+left")
	GrowHeight         string // grow window height (default "alt+down")
	ShrinkHeight       string // shrink window height (default "alt+up")
	SnapTop            string // snap window to top half (default "u")
	SnapBottom         string // snap window to bottom half (default "p")
	Center             string // center window (default "o")
	TileColumns        string // tile windows in columns (default "|")
	TileRows           string // tile windows in rows (default "_")
	Cascade            string // cascade windows (default "\\")
	ClipboardHistory   string // toggle clipboard history (default "y")
	MenuEdit           string // open Edit menu (default "e")
	NotificationCenter string // toggle notification center (default "b")
	Settings           string // open settings panel (default ",")
	QuickNextWindow    string // global quick next window (default "alt+]")
	QuickPrevWindow    string // global quick prev window (default "alt+[")
	TileMaximized      string // maximize all windows (default "")
	ShowDesktop        string // minimize all windows (default "")
	ProjectPicker      string // toggle recent projects picker (default "ctrl+shift+p")
	SaveWorkspace      string // manually save workspace (default "ctrl+s")
	LoadWorkspace      string // browse and load workspaces (default "ctrl+o")
	NewWorkspace       string // create new workspace (default "ctrl+shift+n")
	ShowKeys           string // toggle show-keys overlay (default "f2")
	ToggleTiling       string // toggle tiling mode (default "f6")
	SwapLeft           string // swap window left (default "alt+h")
	SwapRight          string // swap window right (default "alt+l")
	SwapUp             string // swap window up (default "alt+k")
	SwapDown           string // swap window down (default "alt+j")
	SyncPanes          string // toggle pane synchronize (default "s")
	QuakeTerminal      string // toggle quake dropdown terminal (default "ctrl+`")
}

// DefaultKeyBindings returns the default key bindings.
func DefaultKeyBindings() KeyBindings {
	return KeyBindings{
		Prefix:             "ctrl+a",
		Quit:               "q",
		NewTerminal:        "n",
		CloseWindow:        "w",
		EnterTerminal:      "i",
		Minimize:           "m",
		Rename:             "r",
		DockFocus:          ".",
		Launcher:           "space",
		SnapLeft:           "h",
		SnapRight:          "l",
		Maximize:           "k",
		Restore:            "j",
		TileAll:            "t",
		Expose:             "x",
		NextWindow:         "tab",
		PrevWindow:         "shift+tab",
		Help:               "f1",
		ToggleExpose:       "f9",
		MenuBar:            "f10",
		MenuFile:           "f",
		MenuApps:           "a",
		MenuView:           "v",
		MoveLeft:           "shift+left",
		MoveRight:          "shift+right",
		MoveUp:             "shift+up",
		MoveDown:           "shift+down",
		GrowWidth:          "alt+right",
		ShrinkWidth:        "alt+left",
		GrowHeight:         "alt+down",
		ShrinkHeight:       "alt+up",
		SnapTop:            "u",
		SnapBottom:         "p",
		Center:             "o",
		TileColumns:        "|",
		TileRows:           "_",
		Cascade:            "\\",
		ClipboardHistory:   "y",
		MenuEdit:           "e",
		NotificationCenter: "b",
		Settings:           ",",
		QuickNextWindow:    "alt+]",
		QuickPrevWindow:    "alt+[",
		TileMaximized:      "g",
		ShowDesktop:        "s",
		ProjectPicker:      "ctrl+shift+p",
		SaveWorkspace:      "ctrl+s",
		LoadWorkspace:      "ctrl+o",
		NewWorkspace:       "ctrl+shift+n",
		ShowKeys:           "f2",
		ToggleTiling:       "f6",
		SwapLeft:           "alt+h",
		SwapRight:          "alt+l",
		SwapUp:             "alt+k",
		SwapDown:           "alt+j",
		SyncPanes:          "S",
		QuakeTerminal:      "ctrl+`",
	}
}

// UserConfig holds persistent user settings.
type UserConfig struct {
	Theme                 string                  `toml:"theme"`
	IconsOnly             bool                    `toml:"icons_only"`
	Animations            bool                    `toml:"animations"`
	AnimationSpeed        string                  `toml:"animation_speed"` // "slow", "normal", "fast"
	AnimationStyle        string                  `toml:"animation_style"` // "smooth", "snappy", "bouncy"
	ShowDeskClock         bool                    `toml:"show_desk_clock"`
	MinimizeToDock        bool                    `toml:"minimize_to_dock"`
	HideDockWhenMaximized bool                    `toml:"hide_dock_when_maximized"`
	TourCompleted         bool                    `toml:"tour_completed"`
	DefaultTerminalMode   bool                    `toml:"default_terminal_mode"`   // start in Terminal mode on window focus
	HideDockApps          bool                    `toml:"hide_dock_apps"`          // hide common app shortcuts in dock
	FocusFollowsMouse     bool                    `toml:"focus_follows_mouse"`   // auto-focus window under mouse cursor
	ShowResizeIndicator   bool                    `toml:"show_resize_indicator"` // show resize dimensions overlay
	ShowKeys              bool                    `toml:"show_keys"`
	TilingMode            bool                    `toml:"tiling_mode"`
	TilingLayout          string                  `toml:"tiling_layout"`           // "columns", "rows", "all"
	WorkspaceAutoSave     bool                    `toml:"workspace_auto_save"`     // enable auto-save
	WorkspaceAutoSaveMin  int                     `toml:"workspace_auto_save_min"` // auto-save interval in minutes
	RecentApps            []string                `toml:"recent_apps"`
	Favorites             []string                `toml:"favorites"`
	RecentWorkspaces      []WorkspaceHistoryEntry `toml:"recent_workspaces"` // workspace load history
	LogLevel              string                  `toml:"log_level"`         // "off", "error", "warn", "info", "debug"
	QuakeHeightPercent    int                     `toml:"quake_height_percent"` // quake terminal height as % of screen (default 40)
	EnabledWidgets        []string                `toml:"enabled_widgets"`   // ordered list of enabled widget names (nil = all defaults)
	CustomWidgets         []CustomWidgetDef       `toml:"custom_widgets"`    // user-defined shell widgets
	WallpaperMode         string                  `toml:"wallpaper_mode"`          // "theme", "color", "pattern", "program"
	WallpaperColor        string                  `toml:"wallpaper_color"`         // hex "#RRGGBB" for solid color mode
	WallpaperPattern      string                  `toml:"wallpaper_pattern"`       // pattern character(s) for pattern mode
	WallpaperPatternFg    string                  `toml:"wallpaper_pattern_fg"`    // pattern foreground hex
	WallpaperPatternBg    string                  `toml:"wallpaper_pattern_bg"`    // pattern background hex
	WallpaperProgram      string                  `toml:"wallpaper_program"`       // command for program mode (e.g. "cmatrix -s")
	Keys                  KeyBindings
}

// CustomWidgetDef defines a user-created shell-based widget.
type CustomWidgetDef struct {
	Name     string // unique name for the widget
	Label    string // display name in settings
	Icon     string // Nerd Font icon prefix
	Command  string // shell command to execute
	Interval int    // refresh interval in seconds (0 = use default 2s)
	OnClick  string // command to launch on click (e.g. "lazygit")
}

// DefaultUserConfig returns the default user configuration.
func DefaultUserConfig() UserConfig {
	return UserConfig{
		Theme:                "modern",
		IconsOnly:            false,
		Animations:           true,
		AnimationSpeed:       "fast",
		AnimationStyle:       "smooth",
		MinimizeToDock:       true,
		TilingLayout:         "columns",
		WorkspaceAutoSave:    true,
		WorkspaceAutoSaveMin:  1, // 1 minute default
		QuakeHeightPercent:   40,
		Keys:                 DefaultKeyBindings(),
	}
}

const (
	envConfigDir  = "TERMDESK_CONFIG_DIR"
	envConfigPath = "TERMDESK_CONFIG_PATH"
)

// configDir returns the path to ~/.config/termdesk/ or an override.
func configDir() string {
	if override := strings.TrimSpace(os.Getenv(envConfigPath)); override != "" {
		return filepath.Dir(override)
	}
	if override := strings.TrimSpace(os.Getenv(envConfigDir)); override != "" {
		return override
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".config", "termdesk")
}

// ConfigDir returns the path to the config directory, honoring overrides.
func ConfigDir() string {
	return configDir()
}

// configPath returns the path to the config file, honoring overrides.
func configPath() string {
	if override := strings.TrimSpace(os.Getenv(envConfigPath)); override != "" {
		return override
	}
	dir := configDir()
	if dir == "" {
		return ""
	}
	return filepath.Join(dir, "config.toml")
}

// ConfigPath returns the path to the config file, honoring overrides.
func ConfigPath() string {
	return configPath()
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
	var currentWorkspace *WorkspaceHistoryEntry
	var currentCustomWidget *CustomWidgetDef

	for _, line := range strings.Split(data, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		// Section header
		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			sectionName := strings.TrimSpace(line[1 : len(line)-1])

			// Handle array tables [[recent_workspaces]] and [[custom_widgets]]
			if strings.HasPrefix(sectionName, "[") && strings.HasSuffix(sectionName, "]") {
				innerSection := strings.Trim(sectionName, "[]")
				if innerSection == "recent_workspaces" {
					if currentCustomWidget != nil {
						cfg.CustomWidgets = append(cfg.CustomWidgets, *currentCustomWidget)
						currentCustomWidget = nil
					}
					if currentWorkspace != nil {
						cfg.RecentWorkspaces = append(cfg.RecentWorkspaces, *currentWorkspace)
					}
					currentWorkspace = &WorkspaceHistoryEntry{}
				} else if innerSection == "custom_widgets" {
					if currentWorkspace != nil {
						cfg.RecentWorkspaces = append(cfg.RecentWorkspaces, *currentWorkspace)
						currentWorkspace = nil
					}
					if currentCustomWidget != nil {
						cfg.CustomWidgets = append(cfg.CustomWidgets, *currentCustomWidget)
					}
					currentCustomWidget = &CustomWidgetDef{}
				}
				continue
			}

			section = sectionName
			continue
		}

		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}
		key := strings.TrimSpace(parts[0])
		val := strings.TrimSpace(parts[1])
		if len(val) >= 2 && val[0] == '"' && val[len(val)-1] == '"' {
			val = val[1 : len(val)-1]
		}
		val = strings.ReplaceAll(val, "\\\"", "\"")
		val = strings.ReplaceAll(val, "\\\\", "\\")

		// Handle custom widget entry fields
		if currentCustomWidget != nil {
			switch key {
			case "name":
				currentCustomWidget.Name = val
			case "label":
				currentCustomWidget.Label = val
			case "icon":
				currentCustomWidget.Icon = val
			case "command":
				currentCustomWidget.Command = val
			case "interval":
				if n := parseIntSafe(val); n > 0 {
					currentCustomWidget.Interval = n
				}
			case "onClick", "on_click":
				currentCustomWidget.OnClick = val
			}
			continue
		}

		// Handle workspace entry fields
		if currentWorkspace != nil {
			switch key {
			case "path":
				currentWorkspace.Path = val
			case "last_access":
				if t, err := time.Parse(time.RFC3339, val); err == nil {
					currentWorkspace.LastAccess = t
				}
			}
			continue
		}

		switch section {
		case "":
			switch key {
			case "theme":
				cfg.Theme = val
			case "icons_only":
				cfg.IconsOnly = val == "true"
			case "animations":
				cfg.Animations = val == "true"
			case "animation_speed":
				cfg.AnimationSpeed = val
			case "animation_style":
				cfg.AnimationStyle = val
			case "show_desk_clock":
				cfg.ShowDeskClock = val == "true"
			case "minimize_to_dock", "show_launched_in_dock":
				cfg.MinimizeToDock = val == "true"
			case "hide_dock_when_maximized":
				cfg.HideDockWhenMaximized = val == "true"
			case "tour_completed":
				cfg.TourCompleted = val == "true"
			case "show_keys":
				cfg.ShowKeys = val == "true"
			case "default_terminal_mode":
				cfg.DefaultTerminalMode = val == "true"
			case "hide_dock_apps":
				cfg.HideDockApps = val == "true"
			case "focus_follows_mouse":
				cfg.FocusFollowsMouse = val == "true"
			case "show_resize_indicator":
				cfg.ShowResizeIndicator = val == "true"
			case "tiling_mode":
				cfg.TilingMode = val == "true"
			case "tiling_layout":
				cfg.TilingLayout = normalizeTilingLayout(val)
			case "workspace_auto_save":
				cfg.WorkspaceAutoSave = val == "true"
			case "workspace_auto_save_min":
				if minutes := parseIntSafe(val); minutes > 0 {
					cfg.WorkspaceAutoSaveMin = minutes
				}
			case "quake_height_percent":
				if pct := parseIntSafe(val); pct >= 10 && pct <= 90 {
					cfg.QuakeHeightPercent = pct
				}
			case "recent_apps":
				cfg.RecentApps = splitList(val)
			case "favorites":
				cfg.Favorites = splitList(val)
			case "enabled_widgets":
				cfg.EnabledWidgets = splitList(val)
			case "log_level":
				cfg.LogLevel = val
			case "wallpaper_mode":
				cfg.WallpaperMode = val
			case "wallpaper_color":
				cfg.WallpaperColor = val
			case "wallpaper_pattern":
				cfg.WallpaperPattern = val
			case "wallpaper_pattern_fg":
				cfg.WallpaperPatternFg = val
			case "wallpaper_pattern_bg":
				cfg.WallpaperPatternBg = val
			case "wallpaper_program":
				cfg.WallpaperProgram = val
			}
		case "keybindings":
			parseKeybinding(&cfg.Keys, key, val)
		}
	}

	// Append last pending entries
	if currentCustomWidget != nil {
		cfg.CustomWidgets = append(cfg.CustomWidgets, *currentCustomWidget)
	}
	if currentWorkspace != nil {
		cfg.RecentWorkspaces = append(cfg.RecentWorkspaces, *currentWorkspace)
	}
}

// parseKeybinding sets a single keybinding field by name.
func parseKeybinding(kb *KeyBindings, key, val string) {
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
	case "move_left":
		kb.MoveLeft = val
	case "move_right":
		kb.MoveRight = val
	case "move_up":
		kb.MoveUp = val
	case "move_down":
		kb.MoveDown = val
	case "grow_width":
		kb.GrowWidth = val
	case "shrink_width":
		kb.ShrinkWidth = val
	case "grow_height":
		kb.GrowHeight = val
	case "shrink_height":
		kb.ShrinkHeight = val
	case "snap_top":
		kb.SnapTop = val
	case "snap_bottom":
		kb.SnapBottom = val
	case "center":
		kb.Center = val
	case "tile_columns":
		kb.TileColumns = val
	case "tile_rows":
		kb.TileRows = val
	case "cascade":
		kb.Cascade = val
	case "clipboard_history":
		kb.ClipboardHistory = val
	case "menu_edit":
		kb.MenuEdit = val
	case "notification_center":
		kb.NotificationCenter = val
	case "settings":
		kb.Settings = val
	case "quick_next_window":
		kb.QuickNextWindow = val
	case "quick_prev_window":
		kb.QuickPrevWindow = val
	case "tile_maximized":
		kb.TileMaximized = val
	case "show_desktop":
		kb.ShowDesktop = val
	case "project_picker":
		kb.ProjectPicker = val
	case "save_workspace":
		kb.SaveWorkspace = val
	case "load_workspace":
		kb.LoadWorkspace = val
	case "new_workspace":
		kb.NewWorkspace = val
	case "show_keys":
		kb.ShowKeys = val
	case "toggle_tiling":
		kb.ToggleTiling = val
	case "swap_left":
		kb.SwapLeft = val
	case "swap_right":
		kb.SwapRight = val
	case "swap_up":
		kb.SwapUp = val
	case "swap_down":
		kb.SwapDown = val
	case "sync_panes":
		kb.SyncPanes = val
	case "quake_terminal":
		kb.QuakeTerminal = val
	}
}

// splitList splits a comma-separated string into a trimmed string slice.
func splitList(s string) []string {
	if s == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	var result []string
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			result = append(result, p)
		}
	}
	return result
}

// parseIntSafe parses an integer from a string, returning 0 on error.
func parseIntSafe(s string) int {
	var n int
	fmt.Sscanf(s, "%d", &n)
	return n
}

func normalizeTilingLayout(layout string) string {
	switch strings.ToLower(strings.TrimSpace(layout)) {
	case "rows":
		return "rows"
	case "all", "tile_all", "grid":
		return "all"
	default:
		return "columns"
	}
}

// SaveUserConfig writes the user config to ~/.config/termdesk/config.toml.
func SaveUserConfig(cfg UserConfig) error {
	dir := configDir()
	if dir == "" {
		return fmt.Errorf("cannot determine config directory")
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
	if cfg.AnimationSpeed != "" {
		sb.WriteString("animation_speed = \"")
		sb.WriteString(cfg.AnimationSpeed)
		sb.WriteString("\"\n")
	}
	if cfg.AnimationStyle != "" {
		sb.WriteString("animation_style = \"")
		sb.WriteString(cfg.AnimationStyle)
		sb.WriteString("\"\n")
	}
	if cfg.ShowDeskClock {
		sb.WriteString("show_desk_clock = true\n")
	} else {
		sb.WriteString("show_desk_clock = false\n")
	}
	if cfg.MinimizeToDock {
		sb.WriteString("minimize_to_dock = true\n")
	} else {
		sb.WriteString("minimize_to_dock = false\n")
	}
	if cfg.HideDockWhenMaximized {
		sb.WriteString("hide_dock_when_maximized = true\n")
	} else {
		sb.WriteString("hide_dock_when_maximized = false\n")
	}
	if cfg.TourCompleted {
		sb.WriteString("tour_completed = true\n")
	} else {
		sb.WriteString("tour_completed = false\n")
	}
	if cfg.DefaultTerminalMode {
		sb.WriteString("default_terminal_mode = true\n")
	}
	if cfg.HideDockApps {
		sb.WriteString("hide_dock_apps = true\n")
	}
	if cfg.FocusFollowsMouse {
		sb.WriteString("focus_follows_mouse = true\n")
	}
	if cfg.ShowResizeIndicator {
		sb.WriteString("show_resize_indicator = true\n")
	}
	if cfg.ShowKeys {
		sb.WriteString("show_keys = true\n")
	} else {
		sb.WriteString("show_keys = false\n")
	}
	if cfg.TilingMode {
		sb.WriteString("tiling_mode = true\n")
	} else {
		sb.WriteString("tiling_mode = false\n")
	}
	sb.WriteString("tiling_layout = \"")
	sb.WriteString(normalizeTilingLayout(cfg.TilingLayout))
	sb.WriteString("\"\n")
	if cfg.WorkspaceAutoSave {
		sb.WriteString("workspace_auto_save = true\n")
	} else {
		sb.WriteString("workspace_auto_save = false\n")
	}
	if cfg.WorkspaceAutoSaveMin > 0 {
		sb.WriteString(fmt.Sprintf("workspace_auto_save_min = %d\n", cfg.WorkspaceAutoSaveMin))
	}
	if cfg.QuakeHeightPercent > 0 && cfg.QuakeHeightPercent != 40 {
		sb.WriteString(fmt.Sprintf("quake_height_percent = %d\n", cfg.QuakeHeightPercent))
	}
	if len(cfg.RecentApps) > 0 {
		sb.WriteString("recent_apps = \"" + strings.Join(cfg.RecentApps, ",") + "\"\n")
	}
	if len(cfg.Favorites) > 0 {
		sb.WriteString("favorites = \"" + strings.Join(cfg.Favorites, ",") + "\"\n")
	}
	if len(cfg.EnabledWidgets) > 0 {
		sb.WriteString("enabled_widgets = \"" + strings.Join(cfg.EnabledWidgets, ",") + "\"\n")
	}
	if cfg.LogLevel != "" && cfg.LogLevel != "off" {
		sb.WriteString(fmt.Sprintf("log_level = %q\n", cfg.LogLevel))
	}
	if cfg.WallpaperMode != "" && cfg.WallpaperMode != "theme" {
		sb.WriteString(fmt.Sprintf("wallpaper_mode = %q\n", cfg.WallpaperMode))
	}
	if cfg.WallpaperColor != "" {
		sb.WriteString(fmt.Sprintf("wallpaper_color = %q\n", cfg.WallpaperColor))
	}
	if cfg.WallpaperPattern != "" {
		escaped := strings.ReplaceAll(cfg.WallpaperPattern, "\\", "\\\\")
		escaped = strings.ReplaceAll(escaped, "\"", "\\\"")
		sb.WriteString("wallpaper_pattern = \"" + escaped + "\"\n")
	}
	if cfg.WallpaperPatternFg != "" {
		sb.WriteString(fmt.Sprintf("wallpaper_pattern_fg = %q\n", cfg.WallpaperPatternFg))
	}
	if cfg.WallpaperPatternBg != "" {
		sb.WriteString(fmt.Sprintf("wallpaper_pattern_bg = %q\n", cfg.WallpaperPatternBg))
	}
	if cfg.WallpaperProgram != "" {
		sb.WriteString(fmt.Sprintf("wallpaper_program = %q\n", cfg.WallpaperProgram))
	}

	// Write custom widgets
	if len(cfg.CustomWidgets) > 0 {
		sb.WriteString("\n")
		for _, cw := range cfg.CustomWidgets {
			sb.WriteString("[[custom_widgets]]\n")
			sb.WriteString(fmt.Sprintf("name = %q\n", cw.Name))
			if cw.Label != "" {
				sb.WriteString(fmt.Sprintf("label = %q\n", cw.Label))
			}
			if cw.Icon != "" {
				sb.WriteString("icon = \"" + cw.Icon + "\"\n")
			}
			sb.WriteString(fmt.Sprintf("command = %q\n", cw.Command))
			if cw.Interval > 0 {
				sb.WriteString(fmt.Sprintf("interval = %d\n", cw.Interval))
			}
			if cw.OnClick != "" {
				sb.WriteString(fmt.Sprintf("onClick = %q\n", cw.OnClick))
			}
		}
	}

	// Write workspace history
	if len(cfg.RecentWorkspaces) > 0 {
		sb.WriteString("\n")
		for _, ws := range cfg.RecentWorkspaces {
			sb.WriteString("[[recent_workspaces]]\n")
			sb.WriteString(fmt.Sprintf("path = %q\n", ws.Path))
			sb.WriteString(fmt.Sprintf("last_access = %q\n", ws.LastAccess.Format(time.RFC3339)))
		}
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
	if kb.MoveLeft != defaults.MoveLeft {
		kbLines = append(kbLines, fmt.Sprintf("move_left = %q", kb.MoveLeft))
	}
	if kb.MoveRight != defaults.MoveRight {
		kbLines = append(kbLines, fmt.Sprintf("move_right = %q", kb.MoveRight))
	}
	if kb.MoveUp != defaults.MoveUp {
		kbLines = append(kbLines, fmt.Sprintf("move_up = %q", kb.MoveUp))
	}
	if kb.MoveDown != defaults.MoveDown {
		kbLines = append(kbLines, fmt.Sprintf("move_down = %q", kb.MoveDown))
	}
	if kb.GrowWidth != defaults.GrowWidth {
		kbLines = append(kbLines, fmt.Sprintf("grow_width = %q", kb.GrowWidth))
	}
	if kb.ShrinkWidth != defaults.ShrinkWidth {
		kbLines = append(kbLines, fmt.Sprintf("shrink_width = %q", kb.ShrinkWidth))
	}
	if kb.GrowHeight != defaults.GrowHeight {
		kbLines = append(kbLines, fmt.Sprintf("grow_height = %q", kb.GrowHeight))
	}
	if kb.ShrinkHeight != defaults.ShrinkHeight {
		kbLines = append(kbLines, fmt.Sprintf("shrink_height = %q", kb.ShrinkHeight))
	}
	if kb.SnapTop != defaults.SnapTop {
		kbLines = append(kbLines, fmt.Sprintf("snap_top = %q", kb.SnapTop))
	}
	if kb.SnapBottom != defaults.SnapBottom {
		kbLines = append(kbLines, fmt.Sprintf("snap_bottom = %q", kb.SnapBottom))
	}
	if kb.Center != defaults.Center {
		kbLines = append(kbLines, fmt.Sprintf("center = %q", kb.Center))
	}
	if kb.TileColumns != defaults.TileColumns {
		kbLines = append(kbLines, fmt.Sprintf("tile_columns = %q", kb.TileColumns))
	}
	if kb.TileRows != defaults.TileRows {
		kbLines = append(kbLines, fmt.Sprintf("tile_rows = %q", kb.TileRows))
	}
	if kb.Cascade != defaults.Cascade {
		kbLines = append(kbLines, fmt.Sprintf("cascade = %q", kb.Cascade))
	}
	if kb.ClipboardHistory != defaults.ClipboardHistory {
		kbLines = append(kbLines, fmt.Sprintf("clipboard_history = %q", kb.ClipboardHistory))
	}
	if kb.MenuEdit != defaults.MenuEdit {
		kbLines = append(kbLines, fmt.Sprintf("menu_edit = %q", kb.MenuEdit))
	}
	if kb.NotificationCenter != defaults.NotificationCenter {
		kbLines = append(kbLines, fmt.Sprintf("notification_center = %q", kb.NotificationCenter))
	}
	if kb.Settings != defaults.Settings {
		kbLines = append(kbLines, fmt.Sprintf("settings = %q", kb.Settings))
	}
	if kb.QuickNextWindow != defaults.QuickNextWindow {
		kbLines = append(kbLines, fmt.Sprintf("quick_next_window = %q", kb.QuickNextWindow))
	}
	if kb.QuickPrevWindow != defaults.QuickPrevWindow {
		kbLines = append(kbLines, fmt.Sprintf("quick_prev_window = %q", kb.QuickPrevWindow))
	}
	if kb.TileMaximized != defaults.TileMaximized {
		kbLines = append(kbLines, fmt.Sprintf("tile_maximized = %q", kb.TileMaximized))
	}
	if kb.ShowDesktop != defaults.ShowDesktop {
		kbLines = append(kbLines, fmt.Sprintf("show_desktop = %q", kb.ShowDesktop))
	}
	if kb.ProjectPicker != defaults.ProjectPicker {
		kbLines = append(kbLines, fmt.Sprintf("project_picker = %q", kb.ProjectPicker))
	}
	if kb.SaveWorkspace != defaults.SaveWorkspace {
		kbLines = append(kbLines, fmt.Sprintf("save_workspace = %q", kb.SaveWorkspace))
	}
	if kb.LoadWorkspace != defaults.LoadWorkspace {
		kbLines = append(kbLines, fmt.Sprintf("load_workspace = %q", kb.LoadWorkspace))
	}
	if kb.NewWorkspace != defaults.NewWorkspace {
		kbLines = append(kbLines, fmt.Sprintf("new_workspace = %q", kb.NewWorkspace))
	}
	if kb.ShowKeys != defaults.ShowKeys {
		kbLines = append(kbLines, fmt.Sprintf("show_keys = %q", kb.ShowKeys))
	}
	if kb.ToggleTiling != defaults.ToggleTiling {
		kbLines = append(kbLines, fmt.Sprintf("toggle_tiling = %q", kb.ToggleTiling))
	}
	if kb.SwapLeft != defaults.SwapLeft {
		kbLines = append(kbLines, fmt.Sprintf("swap_left = %q", kb.SwapLeft))
	}
	if kb.SwapRight != defaults.SwapRight {
		kbLines = append(kbLines, fmt.Sprintf("swap_right = %q", kb.SwapRight))
	}
	if kb.SwapUp != defaults.SwapUp {
		kbLines = append(kbLines, fmt.Sprintf("swap_up = %q", kb.SwapUp))
	}
	if kb.SwapDown != defaults.SwapDown {
		kbLines = append(kbLines, fmt.Sprintf("swap_down = %q", kb.SwapDown))
	}
	if kb.SyncPanes != defaults.SyncPanes {
		kbLines = append(kbLines, fmt.Sprintf("sync_panes = %q", kb.SyncPanes))
	}
	if kb.QuakeTerminal != defaults.QuakeTerminal {
		kbLines = append(kbLines, fmt.Sprintf("quake_terminal = %q", kb.QuakeTerminal))
	}
	if len(kbLines) > 0 {
		sb.WriteString("\n[keybindings]\n")
		for _, l := range kbLines {
			sb.WriteString(l + "\n")
		}
	}

	path := configPath()
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, []byte(sb.String()), 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

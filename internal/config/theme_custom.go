package config

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"unicode/utf8"
)

// customThemesDir returns the path to ~/.config/termdesk/themes (honoring
// TERMDESK_CONFIG_DIR / TERMDESK_CONFIG_PATH overrides via configDir()).
// Returns "" if no config directory is resolvable.
func customThemesDir() string {
	dir := configDir()
	if dir == "" {
		return ""
	}
	return filepath.Join(dir, "themes")
}

// CustomThemesDir is the exported accessor — used by settings/docs so users
// can be pointed at the right directory.
func CustomThemesDir() string {
	return customThemesDir()
}

// LoadCustomTheme reads ~/.config/termdesk/themes/<name>.toml and returns
// the parsed theme. The second return value is false if the file does not
// exist or fails to open. Parse errors for individual fields are silently
// ignored — unknown keys, malformed values, and missing fields all fall
// through with their zero value, matching parseConfig's behavior.
func LoadCustomTheme(name string) (Theme, bool) {
	dir := customThemesDir()
	if dir == "" || name == "" {
		return Theme{}, false
	}
	if strings.ContainsAny(name, "/\\") || name == "." || name == ".." {
		return Theme{}, false
	}
	data, err := os.ReadFile(filepath.Join(dir, name+".toml"))
	if err != nil {
		return Theme{}, false
	}
	t := parseThemeToml(string(data))
	t.Name = name
	t.ParseColors()
	return t, true
}

// ListCustomThemes returns the names of .toml files in the custom themes
// directory (without the extension). Returns nil if the directory doesn't
// exist. Used by ThemeNames() to surface custom themes in the settings UI.
func ListCustomThemes() []string {
	dir := customThemesDir()
	if dir == "" {
		return nil
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	var names []string
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		n := e.Name()
		if !strings.HasSuffix(n, ".toml") {
			continue
		}
		names = append(names, strings.TrimSuffix(n, ".toml"))
	}
	return names
}

// parseThemeToml parses a minimal flat TOML file into a Theme. Keys mirror
// the Theme struct field names (snake_case also accepted). Unknown keys are
// ignored so users get forward-compatibility when the struct grows.
func parseThemeToml(data string) Theme {
	var t Theme
	for _, line := range strings.Split(data, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if strings.HasPrefix(line, "[") {
			continue // sections are ignored; schema is flat
		}
		eq := strings.IndexByte(line, '=')
		if eq < 0 {
			continue
		}
		key := strings.TrimSpace(line[:eq])
		val := strings.TrimSpace(line[eq+1:])
		if c := strings.Index(val, " #"); c >= 0 {
			val = strings.TrimSpace(val[:c])
		}
		val = unquoteTOML(val)
		setThemeField(&t, key, val)
	}
	return t
}

// unquoteTOML strips surrounding quotes and decodes the two escape sequences
// our writer emits (`\"` and `\\`). Matches parseConfig()'s handling.
func unquoteTOML(v string) string {
	if len(v) >= 2 && v[0] == '"' && v[len(v)-1] == '"' {
		v = v[1 : len(v)-1]
	}
	v = strings.ReplaceAll(v, "\\\"", "\"")
	v = strings.ReplaceAll(v, "\\\\", "\\")
	return v
}

// firstRune returns the first rune of s, or 0 if s is empty. Used for the
// single-rune border and pattern fields.
func firstRune(s string) rune {
	r, _ := utf8.DecodeRuneInString(s)
	if r == utf8.RuneError {
		return 0
	}
	return r
}

func setThemeField(t *Theme, key, val string) {
	switch strings.ToLower(key) {
	case "name":
		t.Name = val
	case "bordertopleft", "border_top_left":
		t.BorderTopLeft = firstRune(val)
	case "bordertopright", "border_top_right":
		t.BorderTopRight = firstRune(val)
	case "borderbottomleft", "border_bottom_left":
		t.BorderBottomLeft = firstRune(val)
	case "borderbottomright", "border_bottom_right":
		t.BorderBottomRight = firstRune(val)
	case "borderhorizontal", "border_horizontal":
		t.BorderHorizontal = firstRune(val)
	case "bordervertical", "border_vertical":
		t.BorderVertical = firstRune(val)
	case "closebutton", "close_button":
		t.CloseButton = val
	case "minbutton", "min_button":
		t.MinButton = val
	case "maxbutton", "max_button":
		t.MaxButton = val
	case "restorebutton", "restore_button":
		t.RestoreButton = val
	case "snapleftbutton", "snap_left_button":
		t.SnapLeftButton = val
	case "snaprightbutton", "snap_right_button":
		t.SnapRightButton = val
	case "closebuttonfg", "close_button_fg":
		t.CloseButtonFg = val
	case "minbuttonfg", "min_button_fg":
		t.MinButtonFg = val
	case "maxbuttonfg", "max_button_fg":
		t.MaxButtonFg = val
	case "activeborderfg", "active_border_fg":
		t.ActiveBorderFg = val
	case "activeborderbg", "active_border_bg":
		t.ActiveBorderBg = val
	case "inactiveborderfg", "inactive_border_fg":
		t.InactiveBorderFg = val
	case "inactiveborderbg", "inactive_border_bg":
		t.InactiveBorderBg = val
	case "activetitlefg", "active_title_fg":
		t.ActiveTitleFg = val
	case "activetitlebg", "active_title_bg":
		t.ActiveTitleBg = val
	case "inactivetitlefg", "inactive_title_fg":
		t.InactiveTitleFg = val
	case "inactivetitlebg", "inactive_title_bg":
		t.InactiveTitleBg = val
	case "contentbg", "content_bg":
		t.ContentBg = val
	case "notificationfg", "notification_fg":
		t.NotificationFg = val
	case "notificationbg", "notification_bg":
		t.NotificationBg = val
	case "menubarfg", "menu_bar_fg":
		t.MenuBarFg = val
	case "menubarbg", "menu_bar_bg":
		t.MenuBarBg = val
	case "dockfg", "dock_fg":
		t.DockFg = val
	case "dockbg", "dock_bg":
		t.DockBg = val
	case "dockaccentbg", "dock_accent_bg":
		t.DockAccentBg = val
	case "desktopbg", "desktop_bg":
		t.DesktopBg = val
	case "accentcolor", "accent_color":
		t.AccentColor = val
	case "accentfg", "accent_fg":
		t.AccentFg = val
	case "subtlefg", "subtle_fg":
		t.SubtleFg = val
	case "buttonyesbg", "button_yes_bg":
		t.ButtonYesBg = val
	case "buttonnobg", "button_no_bg":
		t.ButtonNoBg = val
	case "buttonfg", "button_fg":
		t.ButtonFg = val
	case "desktoppatternchar", "desktop_pattern_char":
		t.DesktopPatternChar = firstRune(val)
	case "desktoppatternfg", "desktop_pattern_fg":
		t.DesktopPatternFg = val
	case "defaultfg", "default_fg":
		t.DefaultFg = val
	case "titlebarheight", "title_bar_height":
		if n, err := strconv.Atoi(val); err == nil {
			t.TitleBarHeight = n
		}
	case "unfocusedfade", "unfocused_fade":
		if f, err := strconv.ParseFloat(val, 64); err == nil {
			t.UnfocusedFade = f
		}
	default:
		// ANSI palette entries: ansi0..ansi15 / ansi_0..ansi_15
		k := strings.ToLower(key)
		k = strings.TrimPrefix(k, "ansi_")
		k = strings.TrimPrefix(k, "ansi")
		if n, err := strconv.Atoi(k); err == nil && n >= 0 && n < 16 {
			t.ANSIPalette[n] = val
		}
	}
}

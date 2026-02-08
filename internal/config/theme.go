package config

import (
	"image/color"
)

// Theme defines the visual appearance of the desktop environment.
type Theme struct {
	Name string

	// Window borders
	BorderTopLeft     rune
	BorderTopRight    rune
	BorderBottomLeft  rune
	BorderBottomRight rune
	BorderHorizontal  rune
	BorderVertical    rune

	// Title bar buttons
	CloseButton     string
	MinButton       string
	MaxButton       string
	RestoreButton   string
	SnapLeftButton  string
	SnapRightButton string

	// Colors (hex strings "#RRGGBB")
	ActiveBorderFg   string
	ActiveBorderBg   string
	InactiveBorderFg string
	InactiveBorderBg string
	ActiveTitleFg    string
	ActiveTitleBg    string
	InactiveTitleFg  string
	InactiveTitleBg  string
	ContentBg        string // content area bg (slightly different from border bg)
	NotificationFg   string // attention/bell border color
	NotificationBg   string
	MenuBarFg        string
	MenuBarBg        string
	DockFg           string
	DockBg           string
	DockAccentBg     string // hover/active dock item background
	DesktopBg        string

	// Desktop background pattern
	DesktopPatternChar rune        // pattern character (0 = none)
	DesktopPatternFg   string      // pattern foreground color hex

	// Title bar height in rows (1 = compact, 3 = spacious; default 1)
	TitleBarHeight int

	// Unfocused window content fading (0.0 = no fade, 0.5 = 50% toward bg)
	UnfocusedFade float64

	// Pre-parsed color.Color values (populated by ParseColors).
	colors parsedColors
}

// parsedColors holds pre-parsed color values to avoid repeated hex parsing.
type parsedColors struct {
	parsed           bool
	ActiveBorderFg   color.Color
	ActiveBorderBg   color.Color
	InactiveBorderFg color.Color
	InactiveBorderBg color.Color
	ActiveTitleFg    color.Color
	ActiveTitleBg    color.Color
	InactiveTitleFg  color.Color
	InactiveTitleBg  color.Color
	ContentBg        color.Color
	NotificationFg   color.Color
	NotificationBg   color.Color
	MenuBarFg        color.Color
	MenuBarBg        color.Color
	DockFg           color.Color
	DockBg           color.Color
	DockAccentBg     color.Color
	DesktopBg        color.Color
	DesktopPatternFg color.Color
	DefaultFg        color.Color // light gray #C0C0C0
}

// hexToColor converts a "#RRGGBB" hex string to color.Color.
func hexToColor(hex string) color.Color {
	if len(hex) != 7 || hex[0] != '#' {
		return nil
	}
	r := hexByte(hex[1], hex[2])
	g := hexByte(hex[3], hex[4])
	b := hexByte(hex[5], hex[6])
	return color.RGBA{R: r, G: g, B: b, A: 255}
}

func hexByte(hi, lo byte) uint8 {
	return hexNibble(hi)<<4 | hexNibble(lo)
}

func hexNibble(b byte) uint8 {
	switch {
	case b >= '0' && b <= '9':
		return b - '0'
	case b >= 'a' && b <= 'f':
		return b - 'a' + 10
	case b >= 'A' && b <= 'F':
		return b - 'A' + 10
	}
	return 0
}

// ParseColors pre-parses all hex color strings into color.Color values.
// Call this once when the theme is loaded or changed.
func (t *Theme) ParseColors() {
	t.colors = parsedColors{
		parsed:           true,
		ActiveBorderFg:   hexToColor(t.ActiveBorderFg),
		ActiveBorderBg:   hexToColor(t.ActiveBorderBg),
		InactiveBorderFg: hexToColor(t.InactiveBorderFg),
		InactiveBorderBg: hexToColor(t.InactiveBorderBg),
		ActiveTitleFg:    hexToColor(t.ActiveTitleFg),
		ActiveTitleBg:    hexToColor(t.ActiveTitleBg),
		InactiveTitleFg:  hexToColor(t.InactiveTitleFg),
		InactiveTitleBg:  hexToColor(t.InactiveTitleBg),
		ContentBg:        hexToColor(t.ContentBg),
		NotificationFg:   hexToColor(t.NotificationFg),
		NotificationBg:   hexToColor(t.NotificationBg),
		MenuBarFg:        hexToColor(t.MenuBarFg),
		MenuBarBg:        hexToColor(t.MenuBarBg),
		DockFg:           hexToColor(t.DockFg),
		DockBg:           hexToColor(t.DockBg),
		DockAccentBg:     hexToColor(t.DockAccentBg),
		DesktopBg:        hexToColor(t.DesktopBg),
		DesktopPatternFg: hexToColor(t.DesktopPatternFg),
		DefaultFg:        color.RGBA{R: 192, G: 192, B: 192, A: 255},
	}
}

// C returns the pre-parsed colors. Panics if ParseColors hasn't been called.
func (t *Theme) C() *parsedColors {
	if !t.colors.parsed {
		t.ParseColors()
	}
	return &t.colors
}

// RetroTheme returns a Windows 1.0 / System 1 inspired theme.
func RetroTheme() Theme {
	t := Theme{
		Name:              "retro",
		BorderTopLeft:     '┌',
		BorderTopRight:    '┐',
		BorderBottomLeft:  '└',
		BorderBottomRight: '┘',
		BorderHorizontal:  '─',
		BorderVertical:    '│',
		CloseButton:       "[X]",
		MinButton:         "[_]",
		MaxButton:         "[□]",
		RestoreButton:     "[◫]",
		SnapLeftButton:    "[◧]",
		SnapRightButton:   "[◨]",
		ActiveBorderFg:    "#FFFFFF",
		ActiveBorderBg:    "#008080",
		InactiveBorderFg:  "#80A0A0",
		InactiveBorderBg:  "#008080",
		ActiveTitleFg:     "#FFFFFF",
		ActiveTitleBg:     "#008080",
		InactiveTitleFg:   "#607070",
		InactiveTitleBg:   "#008080",
		ContentBg:         "#000000",
		NotificationFg:    "#FFFF00",
		NotificationBg:    "#008080",
		MenuBarFg:         "#C0C0C0",
		MenuBarBg:         "#003838",
		DockFg:            "#FFFFFF",
		DockBg:            "#000040",
		DockAccentBg:      "#0000C0",
		DesktopBg:         "#008080",
		DesktopPatternChar: '░',
		DesktopPatternFg:   "#006060",
		TitleBarHeight:    1,
		UnfocusedFade:     0.4,
	}
	t.ParseColors()
	return t
}

// ModernTheme returns an OneDark-inspired theme matching the dotfiles setup.
func ModernTheme() Theme {
	t := Theme{
		Name:              "modern",
		BorderTopLeft:     '╭',
		BorderTopRight:    '╮',
		BorderBottomLeft:  '╰',
		BorderBottomRight: '╯',
		BorderHorizontal:  '─',
		BorderVertical:    '│',
		CloseButton:       "[×]",
		MinButton:         "[_]",
		MaxButton:         "[□]",
		RestoreButton:     "[◫]",
		SnapLeftButton:    "[◧]",
		SnapRightButton:   "[◨]",
		ActiveBorderFg:    "#61AFEF",
		ActiveBorderBg:    "#282C34",
		InactiveBorderFg:  "#5C6370",
		InactiveBorderBg:  "#282C34",
		ActiveTitleFg:     "#ABB2BF",
		ActiveTitleBg:     "#282C34",
		InactiveTitleFg:   "#5C6370",
		InactiveTitleBg:   "#282C34",
		ContentBg:         "#1E2127",
		NotificationFg:    "#E5C07B",
		NotificationBg:    "#282C34",
		MenuBarFg:         "#ABB2BF",
		MenuBarBg:         "#1A1D23",
		DockFg:            "#ABB2BF",
		DockBg:            "#21252B",
		DockAccentBg:      "#3E4451",
		DesktopBg:         "#282C34",
		DesktopPatternChar: '·',
		DesktopPatternFg:   "#2E3340",
		TitleBarHeight:    1,
		UnfocusedFade:     0.35,
	}
	t.ParseColors()
	return t
}

// TokyoNightTheme returns a Tokyo Night inspired dark theme.
func TokyoNightTheme() Theme {
	t := Theme{
		Name:              "tokyonight",
		BorderTopLeft:     '╭',
		BorderTopRight:    '╮',
		BorderBottomLeft:  '╰',
		BorderBottomRight: '╯',
		BorderHorizontal:  '─',
		BorderVertical:    '│',
		CloseButton:       "[×]",
		MinButton:         "[_]",
		MaxButton:         "[□]",
		RestoreButton:     "[◫]",
		SnapLeftButton:    "[◧]",
		SnapRightButton:   "[◨]",
		ActiveBorderFg:    "#7AA2F7",
		ActiveBorderBg:    "#1A1B26",
		InactiveBorderFg:  "#565F89",
		InactiveBorderBg:  "#1A1B26",
		ActiveTitleFg:     "#C0CAF5",
		ActiveTitleBg:     "#1A1B26",
		InactiveTitleFg:   "#565F89",
		InactiveTitleBg:   "#1A1B26",
		ContentBg:         "#16161E",
		NotificationFg:    "#E0AF68",
		NotificationBg:    "#1A1B26",
		MenuBarFg:         "#C0CAF5",
		MenuBarBg:         "#111218",
		DockFg:            "#C0CAF5",
		DockBg:            "#16161E",
		DockAccentBg:      "#24283B",
		DesktopBg:         "#1A1B26",
		DesktopPatternChar: '✦',
		DesktopPatternFg:   "#1E1F2B",
		TitleBarHeight:    1,
		UnfocusedFade:     0.4,
	}
	t.ParseColors()
	return t
}

// CatppuccinTheme returns a Catppuccin Mocha inspired theme.
func CatppuccinTheme() Theme {
	t := Theme{
		Name:              "catppuccin",
		BorderTopLeft:     '╭',
		BorderTopRight:    '╮',
		BorderBottomLeft:  '╰',
		BorderBottomRight: '╯',
		BorderHorizontal:  '─',
		BorderVertical:    '│',
		CloseButton:       "[×]",
		MinButton:         "[_]",
		MaxButton:         "[□]",
		RestoreButton:     "[◫]",
		SnapLeftButton:    "[◧]",
		SnapRightButton:   "[◨]",
		ActiveBorderFg:    "#89B4FA",
		ActiveBorderBg:    "#1E1E2E",
		InactiveBorderFg:  "#585B70",
		InactiveBorderBg:  "#1E1E2E",
		ActiveTitleFg:     "#CDD6F4",
		ActiveTitleBg:     "#1E1E2E",
		InactiveTitleFg:   "#585B70",
		InactiveTitleBg:   "#1E1E2E",
		ContentBg:         "#181825",
		NotificationFg:    "#F9E2AF",
		NotificationBg:    "#1E1E2E",
		MenuBarFg:         "#CDD6F4",
		MenuBarBg:         "#14141F",
		DockFg:            "#CDD6F4",
		DockBg:            "#181825",
		DockAccentBg:      "#313244",
		DesktopBg:         "#1E1E2E",
		DesktopPatternChar: '◦',
		DesktopPatternFg:   "#232336",
		TitleBarHeight:    1,
		UnfocusedFade:     0.35,
	}
	t.ParseColors()
	return t
}

// GetTheme returns a theme by name. Falls back to modern.
func GetTheme(name string) Theme {
	switch name {
	case "retro":
		return RetroTheme()
	case "tokyonight":
		return TokyoNightTheme()
	case "catppuccin":
		return CatppuccinTheme()
	default:
		return ModernTheme()
	}
}

// TitleBarRows returns the title bar height, defaulting to 1 if not set.
func (t Theme) TitleBarRows() int {
	if t.TitleBarHeight < 1 {
		return 1
	}
	return t.TitleBarHeight
}

// ThemeNames returns the names of all available themes.
func ThemeNames() []string {
	return []string{"retro", "modern", "tokyonight", "catppuccin"}
}

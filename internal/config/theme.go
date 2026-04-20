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

	// Per-button foreground colors (optional, hex "#RRGGBB"; empty = use title fg)
	CloseButtonFg string // close button fg (e.g. red for macOS traffic light)
	MinButtonFg   string // minimize button fg (e.g. yellow)
	MaxButtonFg   string // maximize button fg (e.g. green)

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

	// Accent & interaction colors
	AccentColor  string // primary accent (menu hover bg, active tab bg, menubar highlight)
	AccentFg     string // text on accent backgrounds
	SubtleFg     string // dim text (shortcuts, footers, separators)
	ButtonYesBg  string // confirm/positive button background
	ButtonNoBg   string // cancel/destructive button background
	ButtonFg     string // button text foreground (on colored buttons)

	// Desktop background pattern
	DesktopPatternChar rune        // pattern character (0 = none)
	DesktopPatternFg   string      // pattern foreground color hex

	// Default terminal foreground color (optional, hex "#RRGGBB")
	DefaultFg string

	// ANSI 16-color palette override for terminal content.
	// Colors 0-7 are standard, 8-15 are bright variants.
	// Empty strings use auto-detected dark/light defaults.
	ANSIPalette [16]string

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
	AccentColor      color.Color
	AccentFg         color.Color
	SubtleFg         color.Color
	ButtonYesBg      color.Color
	ButtonNoBg       color.Color
	ButtonFg         color.Color
	CloseButtonFg    color.Color // optional per-button fg
	MinButtonFg      color.Color
	MaxButtonFg      color.Color
	DefaultFg        color.Color // light gray #C0C0C0
	ANSIPalette      [16]color.Color // ANSI 16-color palette for terminal content
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
	defFg := t.DefaultFg
	if defFg == "" {
		defFg = "#C0C0C0" // default light gray
	}

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
		AccentColor:      hexToColor(t.AccentColor),
		AccentFg:         hexToColor(t.AccentFg),
		SubtleFg:         hexToColor(t.SubtleFg),
		ButtonYesBg:      hexToColor(t.ButtonYesBg),
		ButtonNoBg:       hexToColor(t.ButtonNoBg),
		ButtonFg:         hexToColor(t.ButtonFg),
		CloseButtonFg:    hexToColor(t.CloseButtonFg),
		MinButtonFg:      hexToColor(t.MinButtonFg),
		MaxButtonFg:      hexToColor(t.MaxButtonFg),
		DefaultFg:        hexToColor(defFg),
	}

	// Parse ANSI palette: use theme-specified colors, fill gaps with defaults.
	defaults := darkANSIPalette
	if isLightBackground(t.ContentBg) {
		defaults = lightANSIPalette
	}
	for i := 0; i < 16; i++ {
		hex := t.ANSIPalette[i]
		if hex == "" {
			hex = defaults[i]
		}
		t.colors.ANSIPalette[i] = hexToColor(hex)
	}
}

// IsLight returns true if this theme has a light background (based on MenuBarBg luminance).
func (t *Theme) IsLight() bool {
	bg := t.MenuBarBg
	if bg == "" {
		bg = t.ContentBg
	}
	return isLightBackground(bg)
}

// isLightBackground returns true if the hex color is perceived as light.
func isLightBackground(hex string) bool {
	c := hexToColor(hex)
	if c == nil {
		return false
	}
	r, g, b, _ := c.RGBA()
	// Relative luminance (simplified): 0.299*R + 0.587*G + 0.114*B
	lum := (299*uint32(r>>8) + 587*uint32(g>>8) + 114*uint32(b>>8)) / 1000
	return lum > 128
}

// Default ANSI 16-color palettes optimized for contrast.
// Dark palette: mid-tone colors balanced for BOTH foreground and background use.
// Bright enough to read as text on dark backgrounds, dark enough to serve as
// panel/segment backgrounds with white text (MC, p10k, etc.).
// Inspired by GNOME Terminal Tango + Nord palettes.
var darkANSIPalette = [16]string{
	"#3B4252", // 0: black (dark gray)
	"#BF616A", // 1: red (muted, works as bg)
	"#6A9F49", // 2: green (medium, works as bg)
	"#D4A14B", // 3: yellow (gold, works as bg)
	"#5B8EC6", // 4: blue (medium, readable as fg AND bg)
	"#A06FAD", // 5: magenta (medium)
	"#4E9F9B", // 6: cyan (medium)
	"#ABB2BF", // 7: white (light gray)
	"#545862", // 8: bright black
	"#D47476", // 9: bright red
	"#8FC46A", // 10: bright green
	"#E5C07B", // 11: bright yellow
	"#7CAED4", // 12: bright blue
	"#C688CE", // 13: bright magenta
	"#6BC1CD", // 14: bright cyan
	"#C8CCD4", // 15: bright white
}

// Light palette: darker colors for light backgrounds (One Light style).
var lightANSIPalette = [16]string{
	"#383A42", // 0: black
	"#E45649", // 1: red
	"#50A14F", // 2: green
	"#986801", // 3: yellow (dark, visible on white)
	"#4078F2", // 4: blue
	"#A626A4", // 5: magenta
	"#0184BC", // 6: cyan
	"#A0A1A7", // 7: white (medium gray)
	"#696C77", // 8: bright black
	"#E45649", // 9: bright red
	"#50A14F", // 10: bright green
	"#C18401", // 11: bright yellow
	"#4078F2", // 12: bright blue
	"#A626A4", // 13: bright magenta
	"#0184BC", // 14: bright cyan
	"#202227", // 15: bright white (dark, visible on white)
}


// C returns the pre-parsed colors. Panics if ParseColors hasn't been called.
func (t *Theme) C() *parsedColors {
	if !t.colors.parsed {
		t.ParseColors()
	}
	return &t.colors
}

// GetTheme returns a theme by name. Falls back to modern.
func GetTheme(name string) Theme {
	switch name {
	case "retro":
		return RetroTheme()
	case "sleek":
		return SleekTheme()
	case "tokyonight":
		return TokyoNightTheme()
	case "catppuccin":
		return CatppuccinTheme()
	case "redmond":
		return RedmondTheme()
	case "platinum":
		return PlatinumTheme()
	case "ubuntu":
		return UbuntuTheme()
	case "aqua":
		return AquaTheme()
	case "springboard":
		return SpringboardTheme()
	case "nord":
		return NordTheme()
	case "dracula":
		return DraculaTheme()
	case "solarized":
		return SolarizedTheme()
	case "sequoia":
		return SequoiaTheme()
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
	return []string{
		"modern", "sleek", "retro", "tokyonight", "catppuccin",
		"redmond", "platinum", "ubuntu", "aqua",
		"springboard", "nord", "dracula", "solarized", "sequoia",
	}
}

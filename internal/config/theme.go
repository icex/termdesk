package config

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

	// Title bar height in rows (1 = compact, 3 = spacious; default 1)
	TitleBarHeight int

	// Unfocused window content fading (0.0 = no fade, 0.5 = 50% toward bg)
	UnfocusedFade float64
}

// RetroTheme returns a Windows 1.0 / System 1 inspired theme.
func RetroTheme() Theme {
	return Theme{
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
		ActiveBorderBg:    "#008080", // = DesktopBg (transparent)
		InactiveBorderFg:  "#80A0A0",
		InactiveBorderBg:  "#008080", // = DesktopBg (transparent)
		ActiveTitleFg:     "#FFFFFF",
		ActiveTitleBg:     "#008080", // = DesktopBg (transparent)
		InactiveTitleFg:   "#607070",
		InactiveTitleBg:   "#008080", // = DesktopBg (transparent)
		ContentBg:         "#000000",
		NotificationFg:    "#FFFF00",
		NotificationBg:    "#008080", // = DesktopBg (transparent)
		MenuBarFg:         "#C0C0C0",
		MenuBarBg:         "#003838",
		DockFg:            "#FFFFFF",
		DockBg:            "#000040",
		DockAccentBg:      "#0000C0",
		DesktopBg:         "#008080",
		TitleBarHeight:    1,
		UnfocusedFade:     0.4,
	}
}

// ModernTheme returns an OneDark-inspired theme matching the dotfiles setup.
func ModernTheme() Theme {
	return Theme{
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
		ActiveBorderBg:    "#282C34", // = DesktopBg (transparent)
		InactiveBorderFg:  "#5C6370",
		InactiveBorderBg:  "#282C34", // = DesktopBg (transparent)
		ActiveTitleFg:     "#ABB2BF",
		ActiveTitleBg:     "#282C34", // = DesktopBg (transparent)
		InactiveTitleFg:   "#5C6370",
		InactiveTitleBg:   "#282C34", // = DesktopBg (transparent)
		ContentBg:         "#1E2127",
		NotificationFg:    "#E5C07B",
		NotificationBg:    "#282C34", // = DesktopBg (transparent)
		MenuBarFg:         "#ABB2BF",
		MenuBarBg:         "#1A1D23",
		DockFg:            "#ABB2BF",
		DockBg:            "#21252B",
		DockAccentBg:      "#3E4451",
		DesktopBg:         "#282C34",
		TitleBarHeight:    1,
		UnfocusedFade:     0.35,
	}
}

// TokyoNightTheme returns a Tokyo Night inspired dark theme.
func TokyoNightTheme() Theme {
	return Theme{
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
		ActiveBorderBg:    "#1A1B26", // = DesktopBg (transparent)
		InactiveBorderFg:  "#565F89",
		InactiveBorderBg:  "#1A1B26", // = DesktopBg (transparent)
		ActiveTitleFg:     "#C0CAF5",
		ActiveTitleBg:     "#1A1B26", // = DesktopBg (transparent)
		InactiveTitleFg:   "#565F89",
		InactiveTitleBg:   "#1A1B26", // = DesktopBg (transparent)
		ContentBg:         "#16161E",
		NotificationFg:    "#E0AF68",
		NotificationBg:    "#1A1B26", // = DesktopBg (transparent)
		MenuBarFg:         "#C0CAF5",
		MenuBarBg:         "#111218",
		DockFg:            "#C0CAF5",
		DockBg:            "#16161E",
		DockAccentBg:      "#24283B",
		DesktopBg:         "#1A1B26",
		TitleBarHeight:    1,
		UnfocusedFade:     0.4,
	}
}

// CatppuccinTheme returns a Catppuccin Mocha inspired theme.
func CatppuccinTheme() Theme {
	return Theme{
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
		ActiveBorderBg:    "#1E1E2E", // = DesktopBg (transparent)
		InactiveBorderFg:  "#585B70",
		InactiveBorderBg:  "#1E1E2E", // = DesktopBg (transparent)
		ActiveTitleFg:     "#CDD6F4",
		ActiveTitleBg:     "#1E1E2E", // = DesktopBg (transparent)
		InactiveTitleFg:   "#585B70",
		InactiveTitleBg:   "#1E1E2E", // = DesktopBg (transparent)
		ContentBg:         "#181825",
		NotificationFg:    "#F9E2AF",
		NotificationBg:    "#1E1E2E", // = DesktopBg (transparent)
		MenuBarFg:         "#CDD6F4",
		MenuBarBg:         "#14141F",
		DockFg:            "#CDD6F4",
		DockBg:            "#181825",
		DockAccentBg:      "#313244",
		DesktopBg:         "#1E1E2E",
		TitleBarHeight:    1,
		UnfocusedFade:     0.35,
	}
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

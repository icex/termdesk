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

	// Title bar
	CloseButton   string
	MaxButton     string
	RestoreButton string

	// Colors (ANSI 256 color codes or hex strings)
	ActiveBorderFg   string
	ActiveBorderBg   string
	InactiveBorderFg string
	InactiveBorderBg string
	ActiveTitleFg    string
	ActiveTitleBg    string
	InactiveTitleFg  string
	InactiveTitleBg  string
	MenuBarFg        string
	MenuBarBg        string
	DockFg           string
	DockBg           string
	DesktopBg        string
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
		MaxButton:         "[□]",
		RestoreButton:     "[◫]",
		ActiveBorderFg:    "#FFFFFF",
		ActiveBorderBg:    "#000080",
		InactiveBorderFg:  "#808080",
		InactiveBorderBg:  "#000000",
		ActiveTitleFg:     "#FFFFFF",
		ActiveTitleBg:     "#000080",
		InactiveTitleFg:   "#C0C0C0",
		InactiveTitleBg:   "#808080",
		MenuBarFg:         "#000000",
		MenuBarBg:         "#C0C0C0",
		DockFg:            "#FFFFFF",
		DockBg:            "#000040",
		DesktopBg:         "#008080",
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
		MaxButton:         "[□]",
		RestoreButton:     "[◫]",
		ActiveBorderFg:    "#61AFEF",
		ActiveBorderBg:    "#282C34",
		InactiveBorderFg:  "#5C6370",
		InactiveBorderBg:  "#282C34",
		ActiveTitleFg:     "#ABB2BF",
		ActiveTitleBg:     "#3E4451",
		InactiveTitleFg:   "#5C6370",
		InactiveTitleBg:   "#282C34",
		MenuBarFg:         "#ABB2BF",
		MenuBarBg:         "#21252B",
		DockFg:            "#ABB2BF",
		DockBg:            "#21252B",
		DesktopBg:         "#282C34",
	}
}

// GetTheme returns a theme by name. Falls back to retro.
func GetTheme(name string) Theme {
	switch name {
	case "modern":
		return ModernTheme()
	default:
		return RetroTheme()
	}
}

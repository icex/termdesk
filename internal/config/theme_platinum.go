package config

// PlatinumTheme returns a classic Mac System 7 inspired theme (non-copyrightable name).
func PlatinumTheme() Theme {
	t := Theme{
		Name:              "platinum",
		BorderTopLeft:     '┌',
		BorderTopRight:    '┐',
		BorderBottomLeft:  '└',
		BorderBottomRight: '┘',
		BorderHorizontal:  '─',
		BorderVertical:    '│',
		CloseButton:       " ■ ",  // System 7 close box
		MinButton:         " ─ ",  // collapse
		MaxButton:         " □ ",  // zoom box
		RestoreButton:     " ◫ ",
		SnapLeftButton:    " ◧ ",
		SnapRightButton:   " ◨ ",
		ActiveBorderFg:    "#000000",
		ActiveBorderBg:    "#DDDDDD",
		InactiveBorderFg:  "#666666",
		InactiveBorderBg:  "#CCCCCC",
		ActiveTitleFg:     "#000000",
		ActiveTitleBg:     "#CCCCCC",
		InactiveTitleFg:   "#888888",
		InactiveTitleBg:   "#CCCCCC",
		ContentBg:         "#FFFFFF",
		NotificationFg:    "#FF6600",
		NotificationBg:    "#DDDDDD",
		MenuBarFg:         "#000000",
		MenuBarBg:         "#B8B8B8",
		DockFg:            "#000000",
		DockBg:            "#B8B8B8",
		DockAccentBg:      "#000000",
		DesktopBg:         "#5577AA",
		AccentColor:       "#000000",
		AccentFg:          "#FFFFFF",
		SubtleFg:          "#666666",
		ButtonYesBg:       "#CCCCCC",
		ButtonNoBg:        "#666666",
		ButtonFg:          "#000000",
		DesktopPatternChar: '⠿',
		DesktopPatternFg:   "#446699",
		DefaultFg:          "#000000",
		TitleBarHeight:    1,
		UnfocusedFade:     0.25,
	}
	t.ParseColors()
	return t
}

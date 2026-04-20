package config

// UbuntuTheme returns an Ubuntu-inspired theme, modernized with official brand colors.
func UbuntuTheme() Theme {
	t := Theme{
		Name:              "ubuntu",
		BorderTopLeft:     '╭',
		BorderTopRight:    '╮',
		BorderBottomLeft:  '╰',
		BorderBottomRight: '╯',
		BorderHorizontal:  '─',
		BorderVertical:    '│',
		CloseButton:       " × ",
		MinButton:         " ─ ",
		MaxButton:         " □ ",
		RestoreButton:     " ◫ ",
		SnapLeftButton:    " ◧ ",
		SnapRightButton:   " ◨ ",
		ActiveBorderFg:    "#77216f", // Ubuntu Purple
		ActiveBorderBg:    "#2d2d2d", // Darker Grey
		InactiveBorderFg:  "#5e2750",
		InactiveBorderBg:  "#2d2d2d",
		ActiveTitleFg:     "#ffffff",
		ActiveTitleBg:     "#2d2d2d",
		InactiveTitleFg:   "#aea79f",
		InactiveTitleBg:   "#2d2d2d",
		ContentBg:         "#101010", // Near black for terminal content
		NotificationFg:    "#da9191", // Light grey
		NotificationBg:    "#2c001e", // Aubergine
		MenuBarFg:         "#ffffff",
		MenuBarBg:         "#3d0022", // Bold aubergine bar
		DockFg:            "#ffffff",
		DockBg:            "#3d0022",
		DockAccentBg:      "#5e2750",
		DesktopBg:         "#15000f", // Almost black aubergine background
		AccentColor:       "#77216f", // Purple
		AccentFg:          "#ffffff",
		SubtleFg:          "#aea79f",
		ButtonYesBg:       "#77216f",
		ButtonNoBg:        "#77216f",
		ButtonFg:          "#ffffff",
		DesktopPatternChar: '╳',      // Kept original pattern
		DesktopPatternFg:  "#250019", // Subtle dark pattern
		TitleBarHeight:    1,
		UnfocusedFade:     0.4,
	}
	t.ParseColors()
	return t
}

package config

// SpringboardTheme returns an iOS-inspired ultra-clean light theme.
func SpringboardTheme() Theme {
	t := Theme{
		Name:              "springboard",
		BorderTopLeft:     '╭',
		BorderTopRight:    '╮',
		BorderBottomLeft:  '╰',
		BorderBottomRight: '╯',
		BorderHorizontal:  '─',
		BorderVertical:    '│',
		CloseButton:       " ● ",  // iOS traffic light
		MinButton:         " ● ",
		MaxButton:         " ● ",
		RestoreButton:     " ● ",
		SnapLeftButton:    " ◧ ",
		SnapRightButton:   " ◨ ",
		CloseButtonFg:     "#FF3B30", // iOS red
		MinButtonFg:       "#FF9500", // iOS orange
		MaxButtonFg:       "#34C759", // iOS green
		ActiveBorderFg:    "#007AFF",
		ActiveBorderBg:    "#F2F2F7",
		InactiveBorderFg:  "#C7C7CC",
		InactiveBorderBg:  "#E5E5EA",
		ActiveTitleFg:     "#000000",
		ActiveTitleBg:     "#F2F2F7",
		InactiveTitleFg:   "#8E8E93",
		InactiveTitleBg:   "#E5E5EA",
		ContentBg:         "#FFFFFF",
		NotificationFg:    "#FF3B30",
		NotificationBg:    "#F2F2F7",
		MenuBarFg:         "#000000",
		MenuBarBg:         "#D8D8DE",
		DockFg:            "#1C1C1E",
		DockBg:            "#D8D8DE",
		DockAccentBg:      "#007AFF",
		DesktopBg:         "#6E5DCC",
		AccentColor:       "#007AFF",
		AccentFg:          "#FFFFFF",
		SubtleFg:          "#8E8E93",
		ButtonYesBg:       "#34C759",
		ButtonNoBg:        "#FF3B30",
		ButtonFg:          "#FFFFFF",
		DesktopPatternChar: 0, // ultra-clean, no pattern
		DesktopPatternFg:   "#5B4DB8",
		DefaultFg:          "#000000",
		TitleBarHeight:    1,
		UnfocusedFade:     0.2,
	}
	t.ParseColors()
	return t
}

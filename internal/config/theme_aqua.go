package config

// AquaTheme returns a modern macOS-inspired theme with blue accents and brushed aluminum feel.
func AquaTheme() Theme {
	t := Theme{
		Name:              "aqua",
		BorderTopLeft:     '╭',
		BorderTopRight:    '╮',
		BorderBottomLeft:  '╰',
		BorderBottomRight: '╯',
		BorderHorizontal:  '─',
		BorderVertical:    '│',
		CloseButton:       " ● ",  // macOS traffic light style
		MinButton:         " ● ",
		MaxButton:         " ● ",
		RestoreButton:     " ● ",
		SnapLeftButton:    " ◧ ",
		SnapRightButton:   " ◨ ",
		CloseButtonFg:     "#FF5F57", // macOS red traffic light
		MinButtonFg:       "#FEBC2E", // macOS yellow traffic light
		MaxButtonFg:       "#28C840", // macOS green traffic light
		ActiveBorderFg:    "#0066CC",
		ActiveBorderBg:    "#E8E8E8",
		InactiveBorderFg:  "#999999",
		InactiveBorderBg:  "#D4D4D4",
		ActiveTitleFg:     "#1A1A1A",
		ActiveTitleBg:     "#E0E0E0",
		InactiveTitleFg:   "#808080",
		InactiveTitleBg:   "#D4D4D4",
		ContentBg:         "#FFFFFF",
		NotificationFg:    "#FF3B30",
		NotificationBg:    "#E8E8E8",
		MenuBarFg:         "#1A1A1A",
		MenuBarBg:         "#C8C8D0",
		DockFg:            "#333333",
		DockBg:            "#C8C8D0",
		DockAccentBg:      "#0066CC",
		DesktopBg:         "#1E3A5F",
		AccentColor:       "#0066CC",
		AccentFg:          "#FFFFFF",
		SubtleFg:          "#666666",
		ButtonYesBg:       "#34C759",
		ButtonNoBg:        "#FF3B30",
		ButtonFg:          "#FFFFFF",
		DesktopPatternChar: '◇',
		DesktopPatternFg:   "#17304F",
		DefaultFg:          "#000000",
		TitleBarHeight:    1,
		UnfocusedFade:     0.25,
	}
	t.ParseColors()
	return t
}

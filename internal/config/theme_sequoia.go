package config

// SequoiaTheme returns a modern macOS dark mode inspired theme.
func SequoiaTheme() Theme {
	t := Theme{
		Name:              "sequoia",
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
		ActiveBorderFg:    "#6E6E73", // macOS warm border
		ActiveBorderBg:    "#3B3B3D", // macOS warm title bar grey
		InactiveBorderFg:  "#48484A",
		InactiveBorderBg:  "#323234",
		ActiveTitleFg:     "#F5F5F7", // Apple primary text
		ActiveTitleBg:     "#3B3B3D", // Warm elevated surface
		InactiveTitleFg:   "#6E6E73",
		InactiveTitleBg:   "#323234",
		ContentBg:         "#1E1E1E", // Dark content, clear contrast with title bar
		NotificationFg:    "#FF9F0A", // macOS system orange
		NotificationBg:    "#3B3B3D",
		MenuBarFg:         "#E5E5EA",
		MenuBarBg:         "#0A0A0B", // Deep contrasting bar
		DockFg:            "#E5E5EA",
		DockBg:            "#0A0A0B", // Deep contrasting dock
		DockAccentBg:      "#48484A",
		DesktopBg:         "#1B1033", // macOS Sequoia sunset deep purple
		AccentColor:       "#007AFF", // macOS system blue
		AccentFg:          "#FFFFFF",
		SubtleFg:          "#6E6E73", // Apple secondary label
		ButtonYesBg:       "#007AFF", // macOS blue confirmation
		ButtonNoBg:        "#48484A", // macOS neutral grey cancel
		ButtonFg:          "#FFFFFF",
		DesktopPatternChar: '·',      // Subtle warm dot
		DesktopPatternFg:  "#261845", // Faint purple glow on desktop
		TitleBarHeight:    1,
		UnfocusedFade:     0.25,
	}
	t.ParseColors()
	return t
}

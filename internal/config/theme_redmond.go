package config

// RedmondTheme returns a Windows 3.1 inspired theme with classic blue desktop.
func RedmondTheme() Theme {
	t := Theme{
		Name:              "redmond",
		BorderTopLeft:     '┌',
		BorderTopRight:    '┐',
		BorderBottomLeft:  '└',
		BorderBottomRight: '┘',
		BorderHorizontal:  '─',
		BorderVertical:    '│',
		CloseButton:       " × ",  // Win 95 close
		MinButton:         " ▁ ",  // Win 95 minimize (underbar)
		MaxButton:         " □ ",  // Win 95 maximize
		RestoreButton:     " ◫ ",
		SnapLeftButton:    " ◧ ",
		SnapRightButton:   " ◨ ",
		ActiveBorderFg:    "#000000",
		ActiveBorderBg:    "#C0C0C0",
		InactiveBorderFg:  "#808080",
		InactiveBorderBg:  "#C0C0C0",
		ActiveTitleFg:     "#FFFFFF",
		ActiveTitleBg:     "#000080",
		InactiveTitleFg:   "#C0C0C0",
		InactiveTitleBg:   "#808080",
		ContentBg:         "#FFFFFF",
		NotificationFg:    "#FF0000",
		NotificationBg:    "#C0C0C0",
		MenuBarFg:         "#000000",
		MenuBarBg:         "#C0C0C0",
		DockFg:            "#000000",
		DockBg:            "#C0C0C0",
		DockAccentBg:      "#000080",
		DesktopBg:         "#008080",
		AccentColor:       "#000080",
		AccentFg:          "#FFFFFF",
		SubtleFg:          "#555555",
		ButtonYesBg:       "#C0C0C0",
		ButtonNoBg:        "#808080",
		ButtonFg:          "#000000",
		DesktopPatternChar: '▚',
		DesktopPatternFg:   "#006868",
		DefaultFg:          "#000000",
		TitleBarHeight:    1,
		UnfocusedFade:     0.3,
	}
	t.ParseColors()
	return t
}

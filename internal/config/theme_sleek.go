package config

// SleekTheme returns a compact, edge-to-edge modern theme.
func SleekTheme() Theme {
	t := Theme{
		Name:               "sleek",
		BorderTopLeft:      '┌',
		BorderTopRight:     '┐',
		BorderBottomLeft:   '└',
		BorderBottomRight:  '┘',
		BorderHorizontal:   '─',
		BorderVertical:     '│',
		CloseButton:       " × ",
		MinButton:         " - ",
		MaxButton:         " □ ",
		RestoreButton:     " ◫ ",
		SnapLeftButton:    " ◧ ",
		SnapRightButton:   " ◨ ",
		ActiveBorderFg:     "#3F4752",
		ActiveBorderBg:     "#15181D",
		InactiveBorderFg:   "#2E343C",
		InactiveBorderBg:   "#15181D",
		ActiveTitleFg:      "#b2b6bc",
		ActiveTitleBg:      "#1C2128",
		InactiveTitleFg:    "#7E8795",
		InactiveTitleBg:    "#1A1F26",
		ContentBg:          "#15181D",
		NotificationFg:     "#bfa25e",
		NotificationBg:     "#1C2128",
		MenuBarFg:          "#C8CED8",
		MenuBarBg:          "#06080B",
		DockFg:             "#C8CED8",
		DockBg:             "#06080B",
		DockAccentBg:       "#222834",
		DesktopBg:          "#0F1115",
		AccentColor:        "#4CAF7A",
		AccentFg:           "#0B0E12",
		SubtleFg:           "#6B7380",
		ButtonYesBg:        "#4CAF7A",
		ButtonNoBg:         "#E06C75",
		ButtonFg:           "#0B0E12",
		DesktopPatternChar: '󰽀',
		DesktopPatternFg:   "#191a1c",
		TitleBarHeight:     1,
		UnfocusedFade:      0.5,
	}
	t.ParseColors()
	return t
}

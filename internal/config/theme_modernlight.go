package config

// ModernLightTheme returns a clean, contemporary light theme inspired by
// modern design systems (GitHub Light, Linear, Vercel) — indigo accent,
// soft neutrals, chrome-style window controls.
func ModernLightTheme() Theme {
	t := Theme{
		Name:               "modernlight",
		BorderTopLeft:      '╭',
		BorderTopRight:     '╮',
		BorderBottomLeft:   '╰',
		BorderBottomRight:  '╯',
		BorderHorizontal:   '─',
		BorderVertical:     '│',
		CloseButton:        "  ", // nf-cod-chrome_close
		MinButton:          "  ", // nf-cod-chrome_minimize
		MaxButton:          "  ", // nf-cod-chrome_maximize
		RestoreButton:      "  ", // nf-cod-chrome_restore
		SnapLeftButton:     " ◧ ",
		SnapRightButton:    " ◨ ",
		CloseButtonFg:      "#E5484D", // muted red
		MinButtonFg:        "#E1A11A", // amber
		MaxButtonFg:        "#30A46C", // emerald
		ActiveBorderFg:     "#C7CBD1", // hairline neutral — indigo is reserved for accent bits
		ActiveBorderBg:     "#FFFFFF",
		InactiveBorderFg:   "#E6E8EB",
		InactiveBorderBg:   "#FFFFFF",
		ActiveTitleFg:      "#1F2328",
		ActiveTitleBg:      "#FFFFFF",
		InactiveTitleFg:    "#8C959F",
		InactiveTitleBg:    "#FFFFFF",
		ContentBg:          "#FFFFFF",
		NotificationFg:     "#BF8700",
		NotificationBg:     "#FFFFFF",
		MenuBarFg:          "#1F2328",
		MenuBarBg:          "#F6F8FA",
		DockFg:             "#1F2328",
		DockBg:             "#F6F8FA",
		DockAccentBg:       "#E7E9EE",
		DesktopBg:          "#EDEFF2",
		AccentColor:        "#6366F1",
		AccentFg:           "#FFFFFF",
		SubtleFg:           "#656D76",
		ButtonYesBg:        "#30A46C",
		ButtonNoBg:         "#E5484D",
		ButtonFg:           "#FFFFFF",
		DesktopPatternChar: '·',
		DesktopPatternFg:   "#D8DBE0",
		DefaultFg:          "#1F2328",
		TitleBarHeight:     1,
		UnfocusedFade:      0.15,
	}
	t.ParseColors()
	return t
}

package config

// ModernTheme returns an OneDark-inspired theme matching the dotfiles setup.
func ModernTheme() Theme {
	t := Theme{
		Name:              "modern",
		BorderTopLeft:     '╭',
		BorderTopRight:    '╮',
		BorderBottomLeft:  '╰',
		BorderBottomRight: '╯',
		BorderHorizontal:  '─',
		BorderVertical:    '│',
		CloseButton:       " \ueab8 ", // nf-cod-chrome_close
		MinButton:         " \ueaba ", // nf-cod-chrome_minimize
		MaxButton:         " \ueab9 ", // nf-cod-chrome_maximize
		RestoreButton:     " \ueabb ", // nf-cod-chrome_restore
		SnapLeftButton:    " ◧ ",
		SnapRightButton:   " ◨ ",
		CloseButtonFg:     "#E06C75",  // OneDark red
		MinButtonFg:       "#E5C07B",  // OneDark yellow
		MaxButtonFg:       "#98C379",  // OneDark green
		ActiveBorderFg:    "#61AFEF",
		ActiveBorderBg:    "#282C34",
		InactiveBorderFg:  "#5C6370",
		InactiveBorderBg:  "#282C34",
		ActiveTitleFg:     "#ABB2BF",
		ActiveTitleBg:     "#282C34",
		InactiveTitleFg:   "#5C6370",
		InactiveTitleBg:   "#282C34",
		ContentBg:         "#1E2127",
		NotificationFg:    "#E5C07B",
		NotificationBg:    "#282C34",
		MenuBarFg:         "#ABB2BF",
		MenuBarBg:         "#0E1014",
		DockFg:            "#ABB2BF",
		DockBg:            "#0E1014",
		DockAccentBg:      "#3E4451",
		DesktopBg:         "#282C34",
		AccentColor:       "#61AFEF",
		AccentFg:          "#1E1E2E",
		SubtleFg:          "#5C6370",
		ButtonYesBg:       "#98C379",
		ButtonNoBg:        "#E06C75",
		ButtonFg:          "#303740",
		DesktopPatternChar: '░',
		DesktopPatternFg:   "#303844",
		TitleBarHeight:     1,
		UnfocusedFade:      0.35,
	}
	t.ParseColors()
	return t
}

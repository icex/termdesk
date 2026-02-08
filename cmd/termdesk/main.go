package main

import (
	"fmt"
	"os"

	"github.com/icex/termdesk/internal/app"
	"github.com/icex/termdesk/internal/config"

	tea "charm.land/bubbletea/v2"
)

func main() {
	// Set terminal default background to match desktop theme.
	// This prevents the terminal's native background from bleeding through
	// when Bubble Tea's renderer uses erase sequences (Termux fix).
	userCfg := config.LoadUserConfig()
	theme := config.GetTheme(userCfg.Theme)
	if bg := theme.DesktopBg; len(bg) == 7 && bg[0] == '#' {
		fmt.Fprintf(os.Stdout, "\x1b]11;rgb:%s/%s/%s\x07", bg[1:3], bg[3:5], bg[5:7])
		defer fmt.Fprintf(os.Stdout, "\x1b]111\x07") // restore original on exit
	}

	m := app.New()
	p := tea.NewProgram(m)
	m.SetProgram(p)
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "termdesk: %v\n", err)
		os.Exit(1)
	}
}

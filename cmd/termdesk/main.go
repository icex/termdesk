package main

import (
	"fmt"
	"os"

	"github.com/icex/termdesk/internal/app"

	tea "charm.land/bubbletea/v2"
)

func main() {
	m := app.New()
	p := tea.NewProgram(m)
	m.SetProgram(p)
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "termdesk: %v\n", err)
		os.Exit(1)
	}
}

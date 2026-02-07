package main

import (
	"fmt"
	"os"

	"github.com/icex/termdesk/internal/app"

	tea "charm.land/bubbletea/v2"
)

func main() {
	p := tea.NewProgram(app.New())
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "termdesk: %v\n", err)
		os.Exit(1)
	}
}

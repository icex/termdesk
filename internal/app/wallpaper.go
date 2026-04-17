package app

import (
	"image/color"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/icex/termdesk/internal/logging"
	"github.com/icex/termdesk/internal/terminal"
)

var (
	lastWallpaperLaunch     time.Time
	wallpaperLaunchFailures int
)

// buildWallpaperConfig creates a WallpaperConfig from the model's wallpaper settings.
// Returns nil for "theme" mode (use default theme background).
func (m *Model) buildWallpaperConfig() *WallpaperConfig {
	mode := m.wallpaperMode
	if mode == "" || mode == "theme" {
		return nil
	}
	wp := &WallpaperConfig{Mode: mode}
	switch mode {
	case "color":
		wp.Color = hexToColor(m.wallpaperColor)
	case "pattern":
		if m.wallpaperPattern != "" {
			r, _ := utf8.DecodeRuneInString(m.wallpaperPattern)
			if r != utf8.RuneError {
				wp.PatChar = r
			}
		}
		wp.PatFg = hexToColor(m.wallpaperPatternFg)
		wp.PatBg = hexToColor(m.wallpaperPatternBg)
	case "program":
		// Program mode uses solid bg; terminal output overlaid in RenderFrame
		wp.Color = hexToColor(m.theme.DesktopBg)
	}
	return wp
}

// wallpaperTermID is the sentinel ID for the wallpaper program terminal.
const wallpaperTermID = "__wallpaper__"

// wallpaperDesktopBg returns the background color to use for the desktop.
// For "color" mode, returns the user-chosen color; otherwise returns the theme default.
func (m *Model) wallpaperDesktopBg() color.Color {
	if m.wallpaperMode == "color" && m.wallpaperColor != "" {
		if c := hexToColor(m.wallpaperColor); c != nil {
			return c
		}
	}
	bg := m.theme.C().DesktopBg
	if bg == nil {
		bg = color.RGBA{A: 255}
	}
	return bg
}

// launchWallpaperTerminal starts a headless terminal running the wallpaper program.
func (m *Model) launchWallpaperTerminal() {
	if m.wallpaperProgram == "" {
		return
	}
	if wallpaperLaunchFailures > 3 && time.Since(lastWallpaperLaunch) < 10*time.Second {
		logging.Error("wallpaper program failing repeatedly, backing off")
		return
	}
	lastWallpaperLaunch = time.Now()
	m.closeWallpaperTerminal()

	wa := m.wm.WorkArea()
	cols := wa.Width
	rows := wa.Height
	if cols < 1 {
		cols = 80
	}
	if rows < 1 {
		rows = 24
	}

	// Parse command — support "command args..." format
	parts := strings.Fields(m.wallpaperProgram)
	if len(parts) == 0 {
		return
	}
	cmd := parts[0]
	var args []string
	if len(parts) > 1 {
		args = parts[1:]
	}

	term, err := terminal.New(cmd, args, cols, rows, m.cellPixelW, m.cellPixelH, "")
	if err != nil {
		wallpaperLaunchFailures++
		logging.Error("wallpaper program %q failed to launch: %v", cmd, err)
		return
	}
	wallpaperLaunchFailures = 0
	c := m.theme.C()
	term.SetDefaultColors(c.DefaultFg, c.DesktopBg)
	m.wallpaperTerminal = term
	m.terminals[wallpaperTermID] = term
	m.termCreatedAt[wallpaperTermID] = time.Now()
	m.spawnPTYReader(wallpaperTermID, term)
}

// closeWallpaperTerminal stops the wallpaper program terminal.
func (m *Model) closeWallpaperTerminal() {
	if m.wallpaperTerminal != nil {
		m.wallpaperTerminal.Close()
	}
	m.wallpaperTerminal = nil
	delete(m.terminals, wallpaperTermID)
	delete(m.termCreatedAt, wallpaperTermID)
	delete(m.termHasOutput, wallpaperTermID)
}

// resizeWallpaperTerminal resizes the wallpaper terminal to match the work area.
func (m *Model) resizeWallpaperTerminal() {
	if m.wallpaperTerminal == nil {
		return
	}
	wa := m.wm.WorkArea()
	cols := wa.Width
	rows := wa.Height
	if cols < 1 || rows < 1 {
		return
	}
	m.wallpaperTerminal.Resize(cols, rows)
}

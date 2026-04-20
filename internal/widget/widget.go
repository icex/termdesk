package widget

import (
	"strings"

	"github.com/mattn/go-runewidth"
)

// Widget is a topbar status indicator with rendering and optional color coding.
type Widget interface {
	Name() string       // unique id: "cpu", "mem", "battery", "clock", "user"
	Render() string     // formatted output string (may include Nerd Font icons)
	ColorLevel() string // "green", "yellow", "red", or "" for default
}

// Zone describes a clickable region in the widget bar.
type Zone struct {
	Start int    // X start position (absolute)
	End   int    // X end position (exclusive)
	Type  string // widget name for click handling
}

// Bar holds an ordered list of widgets for the right side of the menubar.
type Bar struct {
	Widgets []Widget
}

// NewDefaultBar creates a bar with the standard built-in widgets.
// Backward-compatible wrapper around NewBar with default enabled list.
func NewDefaultBar(username string) *Bar {
	return NewBar(DefaultRegistry(), DefaultEnabledWidgets(), username, username, nil)
}

// NewBar creates a bar with widgets from the enabled list, in order.
// For built-in widget names, concrete instances are created.
// For custom widget names, instances are looked up in customWidgets.
// Unknown names are silently skipped.
func NewBar(reg *Registry, enabled []string, username, displayName string,
	customWidgets map[string]*ShellWidget) *Bar {
	var widgets []Widget
	for _, name := range enabled {
		if !reg.Has(name) {
			continue
		}
		w := createBuiltinWidget(name, username, displayName)
		if w != nil {
			widgets = append(widgets, w)
			continue
		}
		if customWidgets != nil {
			if cw, ok := customWidgets[name]; ok {
				widgets = append(widgets, cw)
			}
		}
	}
	return &Bar{Widgets: widgets}
}

// createBuiltinWidget creates a built-in widget by name, or returns nil for unknown names.
func createBuiltinWidget(name, username, displayName string) Widget {
	switch name {
	case "cpu":
		return &CPUWidget{}
	case "mem":
		return &MemoryWidget{}
	case "battery":
		return &BatteryWidget{}
	case "notification":
		return &NotificationWidget{}
	case "workspace":
		return &WorkspaceWidget{DisplayName: "Default"}
	case "clock":
		return &ClockWidget{}
	case "hostname":
		hn := ""
		if idx := strings.Index(displayName, "@"); idx >= 0 {
			hn = displayName[idx+1:]
		}
		return &HostnameWidget{Hostname: hn, IsSSH: isSSHSession()}
	case "user":
		return &UserWidget{Username: username}
	case "disk":
		return &DiskWidget{}
	case "load":
		return &LoadWidget{}
	case "git":
		return &GitBranchWidget{}
	case "docker":
		return &DockerWidget{}
	case "weather":
		return &WeatherWidget{}
	case "mail":
		return &MailWidget{}
	default:
		return nil
	}
}

// Render returns the composed widget bar string with separators.
func (b *Bar) Render() string {
	var parts []string
	for _, w := range b.Widgets {
		s := w.Render()
		if s != "" {
			parts = append(parts, s)
		}
	}
	if len(parts) == 0 {
		return ""
	}
	return " " + strings.Join(parts, " \u2502 ") + " "
}

// DisplayWidth returns the visual terminal width of the widget bar string.
func (b *Bar) DisplayWidth() int {
	return DisplayWidth(b.Render())
}

// RenderZones returns clickable zones with absolute X positions.
// Each zone includes 1 space of padding on each side of the widget text,
// with only the │ separator character between zones.
func (b *Bar) RenderZones(totalWidth int) []Zone {
	rightW := b.DisplayWidth()
	startX := totalWidth - rightW

	var zones []Zone
	x := startX // start of rendered string
	first := true
	for _, w := range b.Widgets {
		s := w.Render()
		if s == "" {
			continue
		}
		if !first {
			x += 1 // skip "│" separator
		}
		first = false
		// Zone covers " widget " (space + content + space)
		zoneW := DisplayWidth(s) + 2
		zones = append(zones, Zone{Start: x, End: x + zoneW, Type: w.Name()})
		x += zoneW
	}
	return zones
}

// DisplayWidth returns the visual terminal width of a string
// using go-runewidth for accurate character width measurement.
func DisplayWidth(s string) int {
	return runewidth.StringWidth(s)
}

package dock

import (
	"unicode"
	"unicode/utf8"

	"github.com/icex/termdesk/internal/apps/registry"
)

// DockItem represents a launchable app in the dock.
type DockItem struct {
	Icon            string // Nerd Font icon
	IconColor       string // hex color for the icon (e.g. "#98C379"), "" = default
	Label           string // displayed label (may be truncated)
	FullTitle       string // full untruncated title (for tooltip), empty = use Label
	Command         string
	Args            []string
	Special         string // "launcher", "expose", "minimized", "running", or "" for normal items
	WindowID        string // window ID for minimized/running items
	Stuck           bool   // true = stuck/unresponsive terminal
	HasBell         bool   // true = bell rang while unfocused/minimized
	HasActivity     bool   // true = unfocused window received PTY output
}

// TooltipText returns the best text for a tooltip — full title if set, else label.
func (d DockItem) TooltipText() string {
	if d.FullTitle != "" {
		return d.FullTitle
	}
	return d.Label
}

// HasStatusSlot reports whether the item always reserves 2 cells for the
// bell/activity prefix. Window entries do, so their width stays constant as
// those flags toggle on focus changes.
func (d DockItem) HasStatusSlot() bool {
	return d.Special == "minimized" || d.Special == "running"
}

// HasOwnIndicator reports whether the window title already starts with a
// status glyph — Claude Code's "✳", Codex/CLI braille spinners, Nerd Font
// icons — so the dock shouldn't stack its generic activity spinner on top.
func (d DockItem) HasOwnIndicator() bool {
	r, _ := utf8.DecodeRuneInString(d.TooltipText())
	return r > unicode.MaxASCII && (unicode.Is(unicode.So, r) || unicode.Is(unicode.Co, r))
}

// DockCell represents a single cell in the dock render output with styling info.
type DockCell struct {
	Char      rune
	Accent    bool   // true = use accent color (hovered item)
	Special   bool   // true = use a distinct style (launcher/expose buttons)
	Minimized bool   // true = minimized window item
	Running   bool   // true = running window item (shown via "Show Running Apps")
	Active    bool   // true = focused/active window item
	Indicator bool   // true = running indicator dot
	Pulse     bool   // true = dock launch pulse animation active
	Separator bool   // true = separator character between items
	Stuck       bool   // true = stuck/unresponsive terminal
	HasBell     bool   // true = bell icon (yellow flash)
	HasActivity bool   // true = unfocused window has new PTY output
	IconColor   string // hex color for icon characters, "" = default
	Padding     bool   // true = outside the item span (left/right filler), renderer may drop it for pill mode
}

// Dock represents the bottom dock bar.
type Dock struct {
	Items           []DockItem
	BaseItems       []DockItem // original items from New(), preserved for rebuild
	Width           int
	HoverIndex      int    // -1 = none
	FocusedWindowID string // window ID of the currently focused window
	IconsOnly       bool   // true = show only icons, no labels
	MinimizeToDock  bool   // true = minimized windows appear in dock
	Pill            bool   // true = render as centered rounded pill with transparent sides
}

// New creates a dock from registry entries, with Launcher and Exposé bookends.
func New(entries []registry.RegistryEntry, width int) *Dock {
	items := []DockItem{
		{Icon: "\uf135", IconColor: "#56B6C2", Label: "Launch", Special: "launcher"},
	}
	for _, e := range entries {
		items = append(items, DockItem{
			Icon:      e.Icon,
			IconColor: e.IconColor,
			Label:     e.Name,
			Command:   e.Command,
		})
	}
	items = append(items, DockItem{
		Icon: "\uf26c", IconColor: "#C678DD", Label: "Expose\u0301", FullTitle: "Expose windows", Special: "expose",
	})
	// Preserve original items so updateDockRunning can rebuild after hideDockApps toggle
	base := make([]DockItem, len(items))
	copy(base, items)
	return &Dock{
		Items:      items,
		BaseItems:  base,
		Width:      width,
		HoverIndex: -1,
	}
}

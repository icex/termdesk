package launcher

import "github.com/icex/termdesk/internal/apps/registry"

// AppEntry represents a launchable application.
type AppEntry struct {
	Name    string
	Icon    string
	Command string
	Args    []string
	Source  string
}

// Launcher is the command palette / app launcher overlay.
type Launcher struct {
	Visible      bool
	Query        string
	Registry     []AppEntry
	Results      []AppEntry
	SelectedIdx  int
	Width        int
	Height       int
	RecentApps   []string        // last 5 launched app commands
	Favorites    map[string]bool // pinned app commands
	QueryHistory []string        // prompt history (newest first)
	historyIdx   int             // -1 = not browsing history
	ExecIndex    []string        // cached PATH executables
	execLoaded   bool
	execLoading  bool
	Suggestions  []string
}

// New creates a launcher from the unified app registry.
func New(entries []registry.RegistryEntry) *Launcher {
	var reg []AppEntry
	for _, e := range entries {
		reg = append(reg, AppEntry{
			Name:    e.Name,
			Icon:    e.Icon,
			Command: e.Command,
			Args:    e.Args,
			Source:  "registry",
		})
	}
	l := &Launcher{
		Registry:  reg,
		Favorites: make(map[string]bool),
	}
	l.updateResults()
	return l
}


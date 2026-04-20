package launcher

import (
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"

	tea "charm.land/bubbletea/v2"
)

// ExecIndexReadyMsg is sent when the async PATH scan completes.
type ExecIndexReadyMsg []string

// NeedsExecScan returns true if the exec index hasn't been loaded or started.
func (l *Launcher) NeedsExecScan() bool {
	return !l.execLoaded && !l.execLoading
}

// MarkExecLoading marks the exec index as currently loading.
func (l *Launcher) MarkExecLoading() {
	l.execLoading = true
}

// SetExecIndex sets the exec index and marks it as loaded.
func (l *Launcher) SetExecIndex(execs []string) {
	l.ExecIndex = execs
	l.execLoaded = true
	l.execLoading = false
}

// ScanExecIndex returns a tea.Cmd that scans PATH in the background.
func ScanExecIndex() tea.Cmd {
	return func() tea.Msg {
		return ExecIndexReadyMsg(buildExecIndex())
	}
}

// RefreshResults re-runs the result filter with the current query.
func (l *Launcher) RefreshResults() {
	l.updateResults()
}

func (l *Launcher) ensureExecIndex() {
	if l.execLoaded || l.execLoading {
		return
	}
	// Synchronous fallback (only if async wasn't triggered).
	l.execLoaded = true
	l.ExecIndex = buildExecIndex()
}

func buildExecIndex() []string {
	pathEnv := os.Getenv("PATH")
	if pathEnv == "" {
		return nil
	}
	paths := filepath.SplitList(pathEnv)
	seen := make(map[string]struct{})
	var execs []string
	for _, dir := range paths {
		entries, err := os.ReadDir(dir)
		if err != nil {
			continue
		}
		for _, ent := range entries {
			name := ent.Name()
			if name == "" {
				continue
			}
			if _, ok := seen[name]; ok {
				continue
			}
			info, err := ent.Info()
			if err != nil {
				continue
			}
			mode := info.Mode()
			if mode.IsDir() {
				continue
			}
			if runtime.GOOS != "windows" && mode.Perm()&0o111 == 0 {
				continue
			}
			seen[name] = struct{}{}
			execs = append(execs, name)
		}
	}
	sort.Strings(execs)
	return execs
}

func execSuggestions(execs []string, token string, limit int) []string {
	if token == "" || len(execs) == 0 || limit <= 0 {
		return nil
	}
	needle := strings.ToLower(token)
	var prefix []string
	var contains []string
	for _, e := range execs {
		name := strings.ToLower(e)
		if strings.HasPrefix(name, needle) {
			prefix = append(prefix, e)
		} else if strings.Contains(name, needle) {
			contains = append(contains, e)
		}
	}
	result := append(prefix, contains...)
	if len(result) > limit {
		result = result[:limit]
	}
	return result
}

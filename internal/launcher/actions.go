package launcher

import (
	"sort"
	"strings"
)

func (l *Launcher) updateResults() {
	l.Suggestions = nil
	if l.Query == "" {
		l.Results = make([]AppEntry, len(l.Registry))
		copy(l.Results, l.Registry)
		l.sortByPriority()
		return
	}

	l.ensureExecIndex()
	fields := strings.Fields(l.Query)
	cmdToken := ""
	if len(fields) > 0 {
		cmdToken = fields[0]
	}

	q := strings.ToLower(l.Query)
	if cmdToken != "" {
		q = strings.ToLower(cmdToken)
	}
	cmdLower := strings.ToLower(cmdToken)
	type scored struct {
		entry AppEntry
		score int
	}
	var matches []scored
	seenCmd := make(map[string]struct{})
	for _, e := range l.Registry {
		name := strings.ToLower(e.Name)
		cmd := strings.ToLower(e.Command)
		if strings.Contains(name, q) || strings.Contains(cmd, q) {
			s := 0
			if strings.HasPrefix(name, q) || strings.HasPrefix(cmd, q) {
				s = 2 // prefix bonus
			} else {
				s = 1
			}
			matches = append(matches, scored{e, s})
		}
		seenCmd[e.Command] = struct{}{}
	}

	if cmdToken != "" {
		for _, exe := range l.ExecIndex {
			if _, ok := seenCmd[exe]; ok {
				continue
			}
			name := strings.ToLower(exe)
			if strings.Contains(name, cmdLower) {
				s := 0
				if strings.HasPrefix(name, cmdLower) {
					s = 2
				} else {
					s = 1
				}
				matches = append(matches, scored{AppEntry{Name: exe, Icon: "\uf120", Command: exe, Source: "exec"}, s})
			}
		}
		l.Suggestions = execSuggestions(l.ExecIndex, cmdToken, 5)
	}

	sort.Slice(matches, func(i, j int) bool {
		return matches[i].score > matches[j].score
	})

	l.Results = make([]AppEntry, len(matches))
	for i, m := range matches {
		l.Results[i] = m.entry
	}
	l.sortByPriority()
}

// sortByPriority sorts results: favorites first, then recents, then rest.
func (l *Launcher) sortByPriority() {
	recentIdx := make(map[string]int)
	for i, cmd := range l.RecentApps {
		recentIdx[cmd] = i
	}

	sort.SliceStable(l.Results, func(i, j int) bool {
		ai := l.Results[i]
		aj := l.Results[j]
		fi := l.Favorites[ai.Command]
		fj := l.Favorites[aj.Command]
		if fi != fj {
			return fi
		}
		ri, riOk := recentIdx[ai.Command]
		rj, rjOk := recentIdx[aj.Command]
		if riOk != rjOk {
			return riOk
		}
		if riOk && rjOk {
			return ri < rj
		}
		return false
	})
}

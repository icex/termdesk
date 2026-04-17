package widget

import (
	"os/exec"
	"strings"
	"time"
)

const gitRefreshInterval = 10 * time.Second

// GitBranchWidget displays the current git branch.
type GitBranchWidget struct {
	Branch   string
	Dirty    bool
	lastRead time.Time
}

func (w *GitBranchWidget) Name() string { return "git" }

func (w *GitBranchWidget) Render() string {
	if w.Branch == "" {
		return ""
	}
	dirty := ""
	if w.Dirty {
		dirty = "*"
	}
	return "\xee\x9c\xa5 " + w.Branch + dirty //  nf-dev-git_branch
}

func (w *GitBranchWidget) ColorLevel() string {
	if w.Branch == "" {
		return ""
	}
	if w.Dirty {
		return "yellow"
	}
	return "green"
}

func (w *GitBranchWidget) NeedsRefresh() bool {
	return w.lastRead.IsZero() || time.Since(w.lastRead) >= gitRefreshInterval
}

func (w *GitBranchWidget) MarkRefreshed() {
	w.lastRead = time.Now()
}

// ReadGitBranch returns the current branch name and dirty state.
// Returns empty branch if not in a git repo.
func ReadGitBranch() (branch string, dirty bool) {
	out, err := exec.Command("git", "rev-parse", "--abbrev-ref", "HEAD").Output()
	if err != nil {
		return "", false
	}
	branch = strings.TrimSpace(string(out))
	if branch == "" {
		return "", false
	}
	// Check dirty state.
	err = exec.Command("git", "diff", "--quiet", "HEAD").Run()
	dirty = err != nil
	return branch, dirty
}

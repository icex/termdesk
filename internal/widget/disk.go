package widget

import (
	"fmt"
	"time"
)

const diskRefreshInterval = 30 * time.Second

// DiskWidget displays disk usage percentage.
type DiskWidget struct {
	UsedPct int
	lastRead time.Time
}

func (w *DiskWidget) Name() string { return "disk" }

func (w *DiskWidget) Render() string {
	if w.lastRead.IsZero() {
		return ""
	}
	return fmt.Sprintf("\xf3\xb0\x8b\x8a %d%%", w.UsedPct) // 󰋊 nf-md-harddisk
}

func (w *DiskWidget) ColorLevel() string {
	if w.UsedPct >= 80 {
		return "red"
	}
	if w.UsedPct >= 60 {
		return "yellow"
	}
	return "green"
}

func (w *DiskWidget) NeedsRefresh() bool {
	return w.lastRead.IsZero() || time.Since(w.lastRead) >= diskRefreshInterval
}

func (w *DiskWidget) MarkRefreshed() {
	w.lastRead = time.Now()
}

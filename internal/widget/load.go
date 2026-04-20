package widget

import (
	"fmt"
	"time"
)

const loadRefreshInterval = 5 * time.Second

// LoadWidget displays the 1-minute load average.
type LoadWidget struct {
	LoadAvg float64
	NumCPU  int
	lastRead time.Time
}

func (w *LoadWidget) Name() string { return "load" }

func (w *LoadWidget) Render() string {
	if w.lastRead.IsZero() {
		return ""
	}
	return fmt.Sprintf("\xf3\xb0\x8a\x9a %.2f", w.LoadAvg) // 󰊚 nf-md-gauge
}

func (w *LoadWidget) ColorLevel() string {
	if w.NumCPU <= 0 {
		return "green"
	}
	if w.LoadAvg >= float64(w.NumCPU)*2 {
		return "red"
	}
	if w.LoadAvg >= float64(w.NumCPU) {
		return "yellow"
	}
	return "green"
}

func (w *LoadWidget) NeedsRefresh() bool {
	return w.lastRead.IsZero() || time.Since(w.lastRead) >= loadRefreshInterval
}

func (w *LoadWidget) MarkRefreshed() {
	w.lastRead = time.Now()
}

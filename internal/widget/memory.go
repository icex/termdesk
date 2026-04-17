package widget

import (
	"fmt"
)

// MemoryWidget displays used memory in GB.
type MemoryWidget struct {
	UsedGB  float64
	TotalGB float64
}

func (w *MemoryWidget) Name() string { return "mem" }

func (w *MemoryWidget) Render() string {
	icon := "\uf85a" // 󰡚 nf-md-memory
	return fmt.Sprintf("%s %.1fG", icon, w.UsedGB)
}

func (w *MemoryWidget) ColorLevel() string {
	if w.TotalGB <= 0 {
		return "green"
	}
	pct := w.UsedGB / w.TotalGB * 100
	if pct >= 80 {
		return "red"
	}
	if pct >= 60 {
		return "yellow"
	}
	return "green"
}


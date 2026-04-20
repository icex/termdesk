package widget

import (
	"fmt"
)

// CPUWidget displays CPU usage percentage with a sparkline chart.
type CPUWidget struct {
	Pct     float64
	History []float64 // rolling samples for sparkline (max 20)
}

func (w *CPUWidget) Name() string { return "cpu" }

func (w *CPUWidget) Render() string {
	return formatCPU(w.Pct) + formatCPUChart(w.History)
}

func (w *CPUWidget) ColorLevel() string {
	if w.Pct >= 80 {
		return "red"
	}
	if w.Pct >= 50 {
		return "yellow"
	}
	return "green"
}

// Update sets the current CPU percentage and appends to history.
func (w *CPUWidget) Update(pct float64) {
	w.Pct = pct
	w.History = append(w.History, pct)
	if len(w.History) > 20 {
		w.History = w.History[1:]
	}
}

func formatCPU(pct float64) string {
	icon := "\uf108" // 󰈸 nf-md-desktop-classic
	if pct >= 50 {
		icon = "\uf0e7" //  nf-fa-bolt
	}
	return fmt.Sprintf("%s %2.0f%%", icon, pct)
}

const cpuChartWidth = 10

func formatCPUChart(history []float64) string {
	blocks := []rune{'▁', '▂', '▃', '▄', '▅', '▆', '▇', '█'}
	result := make([]rune, cpuChartWidth)
	for i := range result {
		result[i] = '▁'
	}
	start := cpuChartWidth - len(history)
	if start < 0 {
		start = 0
	}
	for i, pct := range history {
		pos := start + i
		if pos >= cpuChartWidth {
			break
		}
		idx := int(pct / 100.0 * 7)
		if idx > 7 {
			idx = 7
		}
		if idx < 0 {
			idx = 0
		}
		result[pos] = blocks[idx]
	}
	return string(result)
}


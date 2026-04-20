//go:build linux

package widget

import (
	"os"
	"strconv"
	"strings"
)

// ReadCPUPercent reads CPU usage from /proc/stat.
func ReadCPUPercent() float64 {
	data, err := os.ReadFile("/proc/stat")
	if err != nil {
		return 0
	}
	return parseCPUPercent(string(data))
}

func parseCPUPercent(data string) float64 {
	lines := strings.Split(data, "\n")
	if len(lines) == 0 {
		return 0
	}
	fields := strings.Fields(lines[0])
	if len(fields) < 5 || fields[0] != "cpu" {
		return 0
	}
	var total, idle float64
	for i := 1; i < len(fields); i++ {
		v, _ := strconv.ParseFloat(fields[i], 64)
		total += v
		if i == 4 {
			idle = v
		}
	}
	if total == 0 {
		return 0
	}
	return (total - idle) / total * 100
}

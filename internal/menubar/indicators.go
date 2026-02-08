package menubar

import (
	"os"
	"strconv"
	"strings"
)

// ReadCPUPercent reads CPU usage from /proc/stat.
// Returns approximate overall CPU percentage.
func ReadCPUPercent() float64 {
	data, err := os.ReadFile("/proc/stat")
	if err != nil {
		return 0
	}
	lines := strings.Split(string(data), "\n")
	if len(lines) == 0 {
		return 0
	}
	// First line: cpu user nice system idle iowait irq softirq steal guest guest_nice
	fields := strings.Fields(lines[0])
	if len(fields) < 5 || fields[0] != "cpu" {
		return 0
	}
	var total, idle float64
	for i := 1; i < len(fields); i++ {
		v, _ := strconv.ParseFloat(fields[i], 64)
		total += v
		if i == 4 { // idle
			idle = v
		}
	}
	if total == 0 {
		return 0
	}
	return (total - idle) / total * 100
}

// ReadMemoryGB reads used memory in GB from /proc/meminfo.
func ReadMemoryGB() float64 {
	used, _ := ReadMemoryInfo()
	return used
}

// ReadMemoryInfo reads used and total memory in GB from /proc/meminfo.
func ReadMemoryInfo() (usedGB, totalGB float64) {
	data, err := os.ReadFile("/proc/meminfo")
	if err != nil {
		return 0, 0
	}
	var memTotal, memAvailable float64
	for _, line := range strings.Split(string(data), "\n") {
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		v, _ := strconv.ParseFloat(fields[1], 64)
		switch fields[0] {
		case "MemTotal:":
			memTotal = v
		case "MemAvailable:":
			memAvailable = v
		}
	}
	usedKB := memTotal - memAvailable
	return usedKB / 1024 / 1024, memTotal / 1024 / 1024
}

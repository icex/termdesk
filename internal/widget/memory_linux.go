//go:build linux

package widget

import (
	"os"
	"strconv"
	"strings"
)

// ReadMemoryInfo reads used and total memory in GB from /proc/meminfo.
func ReadMemoryInfo() (usedGB, totalGB float64) {
	data, err := os.ReadFile("/proc/meminfo")
	if err != nil {
		return 0, 0
	}
	return parseMemoryInfo(string(data))
}

func parseMemoryInfo(data string) (usedGB, totalGB float64) {
	var memTotal, memAvailable float64
	for _, line := range strings.Split(data, "\n") {
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

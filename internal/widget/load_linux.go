//go:build linux

package widget

import (
	"os"
	"runtime"
	"strconv"
	"strings"
)

// ReadLoadAvg reads the 1-minute load average and CPU count on Linux.
func ReadLoadAvg() (avg float64, numCPU int) {
	numCPU = runtime.NumCPU()
	data, err := os.ReadFile("/proc/loadavg")
	if err != nil {
		return 0, numCPU
	}
	fields := strings.Fields(string(data))
	if len(fields) < 1 {
		return 0, numCPU
	}
	avg, _ = strconv.ParseFloat(fields[0], 64)
	return avg, numCPU
}

//go:build darwin

package widget

import (
	"os/exec"
	"runtime"
	"strconv"
	"strings"
)

// ReadLoadAvg reads the 1-minute load average and CPU count on macOS.
func ReadLoadAvg() (avg float64, numCPU int) {
	numCPU = runtime.NumCPU()
	out, err := exec.Command("sysctl", "-n", "vm.loadavg").Output()
	if err != nil {
		return 0, numCPU
	}
	// Output format: "{ 1.23 4.56 7.89 }"
	s := strings.TrimSpace(string(out))
	s = strings.Trim(s, "{ }")
	fields := strings.Fields(s)
	if len(fields) < 1 {
		return 0, numCPU
	}
	avg, _ = strconv.ParseFloat(fields[0], 64)
	return avg, numCPU
}

//go:build darwin

package widget

import (
	"os/exec"
	"strconv"
	"strings"
)

// ReadCPUPercent reads CPU usage via top on macOS.
func ReadCPUPercent() float64 {
	out, err := exec.Command("top", "-l", "2", "-n", "0", "-F").Output()
	if err != nil {
		return 0
	}
	// Parse the last "CPU usage" line (first sample is cumulative since boot).
	var idle float64
	for _, line := range strings.Split(string(out), "\n") {
		if strings.Contains(line, "CPU usage") {
			for _, field := range strings.Fields(line) {
				if strings.HasSuffix(field, "idle") {
					// Previous field is the value like "85.0%"
					break
				}
			}
			// Parse: "CPU usage: 12.5% user, 5.0% sys, 82.5% idle"
			parts := strings.Split(line, ",")
			for _, part := range parts {
				part = strings.TrimSpace(part)
				if strings.HasSuffix(part, "idle") {
					val := strings.TrimSuffix(strings.Fields(part)[0], "%")
					idle, _ = strconv.ParseFloat(val, 64)
				}
			}
		}
	}
	return 100 - idle
}

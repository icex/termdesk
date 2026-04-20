//go:build darwin

package widget

import (
	"os/exec"
	"strconv"
	"strings"
)

// ReadDiskInfo reads disk usage percentage on macOS.
// Uses /System/Volumes/Data (APFS data volume) for accurate usage,
// falling back to / if unavailable.
func ReadDiskInfo() int {
	// Try APFS data volume first (/ only shows the system snapshot on macOS).
	pct := readDFPercent("/System/Volumes/Data")
	if pct > 0 {
		return pct
	}
	return readDFPercent("/")
}

func readDFPercent(path string) int {
	out, err := exec.Command("df", "-h", path).Output()
	if err != nil {
		return 0
	}
	lines := strings.Split(string(out), "\n")
	if len(lines) < 2 {
		return 0
	}
	fields := strings.Fields(lines[1])
	if len(fields) < 5 {
		return 0
	}
	cap := strings.TrimSuffix(fields[4], "%")
	pct, err := strconv.Atoi(cap)
	if err != nil {
		return 0
	}
	return pct
}

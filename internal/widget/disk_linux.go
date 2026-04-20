//go:build linux

package widget

import (
	"os/exec"
	"strconv"
	"strings"
)

// ReadDiskInfo reads disk usage percentage on Linux.
func ReadDiskInfo() int {
	out, err := exec.Command("df", "-h", "/").Output()
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

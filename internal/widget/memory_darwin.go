//go:build darwin

package widget

import (
	"os/exec"
	"strconv"
	"strings"
)

// ReadMemoryInfo reads used and total memory in GB on macOS.
func ReadMemoryInfo() (usedGB, totalGB float64) {
	// Total physical memory via sysctl.
	totalOut, err := exec.Command("sysctl", "-n", "hw.memsize").Output()
	if err != nil {
		return 0, 0
	}
	totalBytes, err := strconv.ParseFloat(strings.TrimSpace(string(totalOut)), 64)
	if err != nil {
		return 0, 0
	}
	totalGB = totalBytes / 1024 / 1024 / 1024

	// Page statistics via vm_stat.
	vmOut, err := exec.Command("vm_stat").Output()
	if err != nil {
		return 0, totalGB
	}

	// Parse page size from first line: "Mach Virtual Memory Statistics: (page size of 16384 bytes)"
	lines := strings.Split(string(vmOut), "\n")
	pageSize := 16384.0
	if len(lines) > 0 {
		if idx := strings.Index(lines[0], "page size of "); idx != -1 {
			rest := lines[0][idx+len("page size of "):]
			if spIdx := strings.Index(rest, " "); spIdx != -1 {
				if ps, err := strconv.ParseFloat(rest[:spIdx], 64); err == nil {
					pageSize = ps
				}
			}
		}
	}

	// Parse page counts.
	pages := map[string]float64{}
	for _, line := range lines[1:] {
		parts := strings.SplitN(line, ":", 2)
		if len(parts) != 2 {
			continue
		}
		key := strings.TrimSpace(parts[0])
		val := strings.TrimSpace(parts[1])
		val = strings.TrimSuffix(val, ".")
		if v, err := strconv.ParseFloat(val, 64); err == nil {
			pages[key] = v
		}
	}

	// Used = active + wired + compressor (similar to Activity Monitor "Memory Used").
	usedPages := pages["Pages active"] + pages["Pages wired down"] + pages["Pages occupied by compressor"]
	usedGB = usedPages * pageSize / 1024 / 1024 / 1024
	return usedGB, totalGB
}

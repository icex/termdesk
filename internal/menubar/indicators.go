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

// BatteryInfo holds battery status.
type BatteryInfo struct {
	Percent  float64
	Charging bool
	Present  bool
}

// ReadBattery reads battery status from /sys/class/power_supply/.
// Discovers batteries dynamically — works on Linux (BAT0, BAT1, ...),
// Android/Termux (battery), and any other naming convention.
func ReadBattery() BatteryInfo {
	entries, err := os.ReadDir("/sys/class/power_supply")
	if err != nil {
		return BatteryInfo{}
	}
	for _, entry := range entries {
		base := "/sys/class/power_supply/" + entry.Name()
		// Check if this is a battery (type file contains "Battery")
		typeData, err := os.ReadFile(base + "/type")
		if err != nil {
			continue
		}
		if strings.TrimSpace(string(typeData)) != "Battery" {
			continue
		}
		capData, err := os.ReadFile(base + "/capacity")
		if err != nil {
			continue
		}
		pct, err := strconv.ParseFloat(strings.TrimSpace(string(capData)), 64)
		if err != nil {
			continue
		}
		charging := false
		if statusData, err := os.ReadFile(base + "/status"); err == nil {
			s := strings.TrimSpace(string(statusData))
			charging = s == "Charging" || s == "Full"
		}
		return BatteryInfo{Percent: pct, Charging: charging, Present: true}
	}
	return BatteryInfo{}
}

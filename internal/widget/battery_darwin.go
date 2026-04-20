//go:build darwin

package widget

import (
	"os/exec"
	"regexp"
	"strconv"
	"strings"
)

var batteryPctRe = regexp.MustCompile(`(\d+)%`)

// ReadBattery reads battery status via pmset on macOS.
func ReadBattery() BatteryInfo {
	out, err := exec.Command("pmset", "-g", "batt").Output()
	if err != nil {
		return BatteryInfo{}
	}
	output := string(out)

	// Find battery percentage (e.g. "85%").
	match := batteryPctRe.FindStringSubmatch(output)
	if match == nil {
		return BatteryInfo{}
	}
	pct, err := strconv.ParseFloat(match[1], 64)
	if err != nil {
		return BatteryInfo{}
	}

	// pmset detail line contains "charging", "charged", or "discharging".
	// "discharging" contains "charging" as a substring, so check it explicitly.
	charging := !strings.Contains(output, "discharging") &&
		(strings.Contains(output, "charging") || strings.Contains(output, "charged"))

	return BatteryInfo{Percent: pct, Charging: charging, Present: true}
}

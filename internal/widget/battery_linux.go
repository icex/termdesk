//go:build linux

package widget

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// ReadBattery reads battery status from /sys/class/power_supply/.
func ReadBattery() BatteryInfo {
	return readBatteryFromDir("/sys/class/power_supply")
}

func readBatteryFromDir(dir string) BatteryInfo {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return BatteryInfo{}
	}
	for _, entry := range entries {
		base := filepath.Join(dir, entry.Name())
		typeData, err := os.ReadFile(filepath.Join(base, "type"))
		if err != nil {
			continue
		}
		if strings.TrimSpace(string(typeData)) != "Battery" {
			continue
		}
		capData, err := os.ReadFile(filepath.Join(base, "capacity"))
		if err != nil {
			continue
		}
		pct, err := strconv.ParseFloat(strings.TrimSpace(string(capData)), 64)
		if err != nil {
			continue
		}
		charging := false
		if statusData, err := os.ReadFile(filepath.Join(base, "status")); err == nil {
			s := strings.TrimSpace(string(statusData))
			charging = s == "Charging" || s == "Full"
		}
		return BatteryInfo{Percent: pct, Charging: charging, Present: true}
	}
	return BatteryInfo{}
}

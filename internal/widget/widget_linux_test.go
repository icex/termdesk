//go:build linux

package widget

import (
	"math"
	"os"
	"path/filepath"
	"testing"
)

func TestReadBatteryFromDir(t *testing.T) {
	dir := t.TempDir()
	bat := filepath.Join(dir, "BAT0")
	if err := os.MkdirAll(bat, 0o700); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(bat, "type"), []byte("Battery"), 0o600); err != nil {
		t.Fatalf("write type: %v", err)
	}
	if err := os.WriteFile(filepath.Join(bat, "capacity"), []byte("55"), 0o600); err != nil {
		t.Fatalf("write capacity: %v", err)
	}
	if err := os.WriteFile(filepath.Join(bat, "status"), []byte("Charging"), 0o600); err != nil {
		t.Fatalf("write status: %v", err)
	}

	info := readBatteryFromDir(dir)
	if !info.Present || info.Percent != 55 || !info.Charging {
		t.Fatalf("info=%+v", info)
	}
}

func TestReadBatteryFromDirEmpty(t *testing.T) {
	dir := t.TempDir()
	info := readBatteryFromDir(dir)
	if info.Present {
		t.Fatalf("expected not present for empty dir, got %+v", info)
	}
}

func TestReadBatteryFromDirMissing(t *testing.T) {
	info := readBatteryFromDir("/nonexistent/path/that/does/not/exist")
	if info.Present {
		t.Fatalf("expected not present for missing dir, got %+v", info)
	}
}

func TestReadBatteryFromDirNonBattery(t *testing.T) {
	dir := t.TempDir()
	ac := filepath.Join(dir, "AC0")
	if err := os.MkdirAll(ac, 0o700); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(ac, "type"), []byte("Mains"), 0o600); err != nil {
		t.Fatalf("write type: %v", err)
	}

	info := readBatteryFromDir(dir)
	if info.Present {
		t.Fatalf("expected not present for non-battery type, got %+v", info)
	}
}

func TestReadBatteryFromDirBadCapacity(t *testing.T) {
	dir := t.TempDir()
	bat := filepath.Join(dir, "BAT0")
	if err := os.MkdirAll(bat, 0o700); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(bat, "type"), []byte("Battery"), 0o600); err != nil {
		t.Fatalf("write type: %v", err)
	}
	if err := os.WriteFile(filepath.Join(bat, "capacity"), []byte("not-a-number"), 0o600); err != nil {
		t.Fatalf("write capacity: %v", err)
	}

	info := readBatteryFromDir(dir)
	if info.Present {
		t.Fatalf("expected not present for bad capacity, got %+v", info)
	}
}

func TestReadBatteryFromDirNoCapacity(t *testing.T) {
	dir := t.TempDir()
	bat := filepath.Join(dir, "BAT0")
	if err := os.MkdirAll(bat, 0o700); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(bat, "type"), []byte("Battery"), 0o600); err != nil {
		t.Fatalf("write type: %v", err)
	}
	// No capacity file written.

	info := readBatteryFromDir(dir)
	if info.Present {
		t.Fatalf("expected not present for missing capacity, got %+v", info)
	}
}

func TestReadBatteryFromDirNoTypeFile(t *testing.T) {
	dir := t.TempDir()
	bat := filepath.Join(dir, "BAT0")
	if err := os.MkdirAll(bat, 0o700); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	// No type file written, only capacity.
	if err := os.WriteFile(filepath.Join(bat, "capacity"), []byte("80"), 0o600); err != nil {
		t.Fatalf("write capacity: %v", err)
	}

	info := readBatteryFromDir(dir)
	if info.Present {
		t.Fatalf("expected not present for missing type file, got %+v", info)
	}
}

func TestReadBatteryFromDirFullStatus(t *testing.T) {
	dir := t.TempDir()
	bat := filepath.Join(dir, "BAT0")
	if err := os.MkdirAll(bat, 0o700); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(bat, "type"), []byte("Battery"), 0o600); err != nil {
		t.Fatalf("write type: %v", err)
	}
	if err := os.WriteFile(filepath.Join(bat, "capacity"), []byte("100"), 0o600); err != nil {
		t.Fatalf("write capacity: %v", err)
	}
	if err := os.WriteFile(filepath.Join(bat, "status"), []byte("Full"), 0o600); err != nil {
		t.Fatalf("write status: %v", err)
	}

	info := readBatteryFromDir(dir)
	if !info.Present || info.Percent != 100 || !info.Charging {
		t.Fatalf("info=%+v", info)
	}
}

func TestReadBatteryFromDirDischargingStatus(t *testing.T) {
	dir := t.TempDir()
	bat := filepath.Join(dir, "BAT0")
	if err := os.MkdirAll(bat, 0o700); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(bat, "type"), []byte("Battery"), 0o600); err != nil {
		t.Fatalf("write type: %v", err)
	}
	if err := os.WriteFile(filepath.Join(bat, "capacity"), []byte("42"), 0o600); err != nil {
		t.Fatalf("write capacity: %v", err)
	}
	if err := os.WriteFile(filepath.Join(bat, "status"), []byte("Discharging"), 0o600); err != nil {
		t.Fatalf("write status: %v", err)
	}

	info := readBatteryFromDir(dir)
	if !info.Present || info.Percent != 42 || info.Charging {
		t.Fatalf("info=%+v", info)
	}
}

func TestParseCPUPercent(t *testing.T) {
	data := "cpu  100 50 50 300 0 0 0 0 0 0\n"
	got := parseCPUPercent(data)
	if math.Abs(got-40.0) > 0.001 {
		t.Fatalf("cpu pct=%v", got)
	}
}

func TestParseCPUPercentEmpty(t *testing.T) {
	got := parseCPUPercent("")
	if got != 0 {
		t.Fatalf("expected 0 for empty input, got %v", got)
	}
}

func TestParseCPUPercentBadFormat(t *testing.T) {
	got := parseCPUPercent("notcpu 100 200 300 400\n")
	if got != 0 {
		t.Fatalf("expected 0 for bad format, got %v", got)
	}
}

func TestParseCPUPercentTooFewFields(t *testing.T) {
	got := parseCPUPercent("cpu 100\n")
	if got != 0 {
		t.Fatalf("expected 0 for too few fields, got %v", got)
	}
}

func TestParseMemoryInfo(t *testing.T) {
	data := "MemTotal:       2048000 kB\nMemAvailable:   1024000 kB\n"
	used, total := parseMemoryInfo(data)
	if math.Abs(total-1.953125) > 1e-6 {
		t.Fatalf("total=%v", total)
	}
	if math.Abs(used-0.9765625) > 1e-6 {
		t.Fatalf("used=%v", used)
	}
}

func TestParseMemoryInfoEmpty(t *testing.T) {
	used, total := parseMemoryInfo("")
	if used != 0 || total != 0 {
		t.Fatalf("expected 0,0 for empty input, got %v,%v", used, total)
	}
}

func TestParseMemoryInfoPartial(t *testing.T) {
	// Only MemTotal, no MemAvailable.
	data := "MemTotal:       2048000 kB\n"
	used, total := parseMemoryInfo(data)
	if math.Abs(total-1.953125) > 1e-6 {
		t.Fatalf("total=%v", total)
	}
	// used = (memTotal - 0) / 1024 / 1024
	if math.Abs(used-1.953125) > 1e-6 {
		t.Fatalf("used=%v", used)
	}
}

package widget

import (
	"fmt"
)

// BatteryWidget displays battery percentage and charging status.
type BatteryWidget struct {
	Pct      float64
	Charging bool
	Present  bool
}

func (w *BatteryWidget) Name() string { return "battery" }

func (w *BatteryWidget) Render() string {
	if !w.Present {
		return ""
	}
	icon := "\uf244" //  nf-fa-battery_empty
	if w.Pct >= 80 {
		icon = "\uf240" //  nf-fa-battery_full
	} else if w.Pct >= 60 {
		icon = "\uf241" //  nf-fa-battery_three_quarters
	} else if w.Pct >= 40 {
		icon = "\uf242" //  nf-fa-battery_half
	} else if w.Pct >= 20 {
		icon = "\uf243" //  nf-fa-battery_quarter
	}
	charge := ""
	if w.Charging {
		charge = " \uf1e6" //  nf-fa-plug
	}
	return fmt.Sprintf("%s %.0f%%%s", icon, w.Pct, charge)
}

func (w *BatteryWidget) ColorLevel() string {
	if !w.Present {
		return ""
	}
	if w.Pct >= 25 {
		return "green"
	}
	if w.Pct >= 15 {
		return "yellow"
	}
	return "red"
}

// BatteryInfo holds battery status read from the system.
type BatteryInfo struct {
	Percent  float64
	Charging bool
	Present  bool
}


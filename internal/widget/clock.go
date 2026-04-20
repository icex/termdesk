package widget

import "time"

// ClockWidget displays the current time.
type ClockWidget struct{}

func (w *ClockWidget) Name() string       { return "clock" }
func (w *ClockWidget) Render() string     { return time.Now().Format("03:04 PM") }
func (w *ClockWidget) ColorLevel() string { return "" }

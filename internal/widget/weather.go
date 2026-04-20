package widget

import (
	"context"
	"os/exec"
	"strings"
	"time"
)

const weatherRefreshInterval = 10 * time.Minute

// WeatherWidget displays the current weather from wttr.in.
type WeatherWidget struct {
	Text     string
	lastRead time.Time
}

func (w *WeatherWidget) Name() string { return "weather" }

func (w *WeatherWidget) Render() string {
	if w.Text == "" {
		return ""
	}
	return w.Text
}

func (w *WeatherWidget) ColorLevel() string { return "" }

func (w *WeatherWidget) NeedsRefresh() bool {
	return w.lastRead.IsZero() || time.Since(w.lastRead) >= weatherRefreshInterval
}

func (w *WeatherWidget) MarkRefreshed() {
	w.lastRead = time.Now()
}

// ReadWeather fetches current weather from wttr.in.
// Returns empty string on failure.
func ReadWeather() string {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	out, err := exec.CommandContext(ctx, "curl", "-s", "wttr.in/?format=%c+%t").Output()
	if err != nil {
		return ""
	}
	s := strings.TrimSpace(string(out))
	// Collapse consecutive spaces (wttr.in emoji icons leave extra whitespace)
	for strings.Contains(s, "  ") {
		s = strings.ReplaceAll(s, "  ", " ")
	}
	runes := []rune(s)
	if len(runes) > 20 {
		runes = runes[:20]
	}
	return string(runes)
}

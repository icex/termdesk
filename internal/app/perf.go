package app

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// perfTracker collects rendering and UI performance statistics.
// Enabled via TERMDESK_PERF=1 environment variable.
// Stats are shown as an overlay and logged to ~/.local/share/termdesk/perf.log.
type perfTracker struct {
	enabled bool

	// Per-frame timing (set during each View/Update cycle)
	lastViewTime    time.Duration // last View() duration
	lastUpdateTime  time.Duration // last Update() duration
	lastFrameTime   time.Duration // last RenderFrame() duration
	lastANSITime    time.Duration // last BufferToString() duration
	lastANSIBytes   int           // last BufferToString() output size
	lastWindowCount int           // windows rendered in last frame
	lastCacheHits   int           // window cache hits in last frame
	lastCacheMisses int           // window cache misses in last frame

	// Rolling averages (updated per frame)
	frameCount  int64
	totalView   time.Duration
	totalUpdate time.Duration
	totalFrame  time.Duration
	totalANSI   time.Duration

	// FPS tracking
	fpsFrames   int
	fpsLastTick time.Time
	fps         float64

	// Log file
	mu      sync.Mutex
	logFile *os.File
	logTick time.Time // last periodic log write
}

func newPerfTracker() *perfTracker {
	v := os.Getenv("TERMDESK_PERF")
	enabled := v == "1" || v == "true"
	pt := &perfTracker{
		enabled:     enabled,
		fpsLastTick: time.Now(),
		logTick:     time.Now(),
	}
	if enabled {
		pt.openLog()
	}
	return pt
}

func (pt *perfTracker) openLog() {
	home, err := os.UserHomeDir()
	if err != nil {
		return
	}
	dir := filepath.Join(home, ".local", "share", "termdesk")
	os.MkdirAll(dir, 0o755)
	f, err := os.OpenFile(filepath.Join(dir, "perf.log"),
		os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
	if err != nil {
		return
	}
	pt.logFile = f
	fmt.Fprintf(f, "=== termdesk perf log %s ===\n", time.Now().Format(time.RFC3339))
	fmt.Fprintf(f, "%-12s %-8s %-8s %-8s %-8s %-6s %-6s %-5s %-8s %-8s\n",
		"time", "fps", "view", "update", "frame", "ansi", "bytes", "wins", "hits", "misses")
}

// recordView records timing for a View() call.
func (pt *perfTracker) recordView(d time.Duration) {
	pt.lastViewTime = d
	pt.totalView += d
	pt.frameCount++

	// FPS calculation
	pt.fpsFrames++
	elapsed := time.Since(pt.fpsLastTick)
	if elapsed >= time.Second {
		pt.fps = float64(pt.fpsFrames) / elapsed.Seconds()
		pt.fpsFrames = 0
		pt.fpsLastTick = time.Now()
	}

	// Periodic log write (every 2 seconds)
	if pt.logFile != nil && time.Since(pt.logTick) >= 2*time.Second {
		pt.logTick = time.Now()
		pt.writeLog()
	}
}

// recordUpdate records timing for an Update() call.
func (pt *perfTracker) recordUpdate(d time.Duration) {
	pt.lastUpdateTime = d
	pt.totalUpdate += d
}

// recordFrame records timing for a RenderFrame() call.
func (pt *perfTracker) recordFrame(d time.Duration, windowCount, cacheHits, cacheMisses int) {
	pt.lastFrameTime = d
	pt.totalFrame += d
	pt.lastWindowCount = windowCount
	pt.lastCacheHits = cacheHits
	pt.lastCacheMisses = cacheMisses
}

// recordANSI records timing for BufferToString().
func (pt *perfTracker) recordANSI(d time.Duration, bytes int) {
	pt.lastANSITime = d
	pt.totalANSI += d
	pt.lastANSIBytes = bytes
}

func (pt *perfTracker) writeLog() {
	pt.mu.Lock()
	defer pt.mu.Unlock()
	if pt.logFile == nil {
		return
	}
	avgView := time.Duration(0)
	avgUpdate := time.Duration(0)
	avgFrame := time.Duration(0)
	avgANSI := time.Duration(0)
	if pt.frameCount > 0 {
		avgView = pt.totalView / time.Duration(pt.frameCount)
		avgUpdate = pt.totalUpdate / time.Duration(pt.frameCount)
		avgFrame = pt.totalFrame / time.Duration(pt.frameCount)
		avgANSI = pt.totalANSI / time.Duration(pt.frameCount)
	}
	fmt.Fprintf(pt.logFile, "%-12s %7.1f %7s %7s %7s %5s %5dK %4d   %4d     %4d   (avg: v=%s u=%s f=%s a=%s n=%d)\n",
		time.Now().Format("15:04:05.000"),
		pt.fps,
		fmtDur(pt.lastViewTime),
		fmtDur(pt.lastUpdateTime),
		fmtDur(pt.lastFrameTime),
		fmtDur(pt.lastANSITime),
		pt.lastANSIBytes/1024,
		pt.lastWindowCount,
		pt.lastCacheHits,
		pt.lastCacheMisses,
		fmtDur(avgView),
		fmtDur(avgUpdate),
		fmtDur(avgFrame),
		fmtDur(avgANSI),
		pt.frameCount,
	)
}

func (pt *perfTracker) close() {
	pt.mu.Lock()
	defer pt.mu.Unlock()
	if pt.logFile != nil {
		pt.logFile.Close()
		pt.logFile = nil
	}
}

// fmtDur formats a duration concisely: "1.2ms", "456µs", "12ms".
func fmtDur(d time.Duration) string {
	us := d.Microseconds()
	if us < 1000 {
		return fmt.Sprintf("%dµs", us)
	}
	ms := float64(us) / 1000.0
	if ms < 10 {
		return fmt.Sprintf("%.1fms", ms)
	}
	return fmt.Sprintf("%.0fms", ms)
}

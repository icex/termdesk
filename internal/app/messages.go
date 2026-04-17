package app

import (
	"time"

	tea "charm.land/bubbletea/v2"
)

// WindowResizedMsg is sent when the terminal window is resized.
type WindowResizedMsg struct {
	Width  int
	Height int
}

// PtyOutputMsg signals that a PTY has produced output.
type PtyOutputMsg struct {
	WindowID string
}

// PtyClosedMsg signals that a PTY session has ended.
type PtyClosedMsg struct {
	WindowID string
	Err      error
}

// WorkspaceAutoSaveMsg triggers workspace state save.
type WorkspaceAutoSaveMsg struct {
	Time time.Time
}

// WorkspaceRestoreMsg triggers workspace restoration.
type WorkspaceRestoreMsg struct{}

// AutoStartMsg triggers project auto-start after workspace restore completes.
// Sent as a separate message so it runs in its own Update cycle.
type AutoStartMsg struct{}

// CleanupMsg triggers periodic cleanup of old clipboard and notification data.
type CleanupMsg struct {
	Time time.Time
}

// ResizeRedrawMsg triggers a delayed full redraw after terminal resize,
// giving terminal apps time to respond to SIGWINCH before re-rendering.
type ResizeRedrawMsg struct{}

// ResizeSettleTickMsg is a self-sustaining ticker that fires rapidly after
// terminal resize, keeping View() calls flowing so terminal content updates
// as child apps respond to SIGWINCH. Same pattern as AnimationTickMsg.
type ResizeSettleTickMsg struct {
	Time time.Time
}

// KittyFlushMsg delivers accumulated Kitty graphics APC data to write
// directly to /dev/tty, bypassing BubbleTea's rendering pipeline.
type KittyFlushMsg struct {
	Data []byte
}

// ImageFlushMsg delivers accumulated sixel/iTerm2 image data to write
// directly to the host terminal via tea.Raw(), bypassing BT's rendering.
type ImageFlushMsg struct {
	Data []byte
}

// BellMsg signals that a terminal rang the bell (BEL character).
type BellMsg struct {
	WindowID string
}

// CustomWidgetResultMsg delivers async shell widget command results.
type CustomWidgetResultMsg struct {
	Results map[string]string // widget name → output (absent = command failed)
}

// BuiltinWidgetDataMsg delivers async results from slow built-in widgets
// (disk, load, git, docker, weather, mail) that have per-widget refresh intervals.
type BuiltinWidgetDataMsg struct {
	DiskPct     int
	LoadAvg     float64
	NumCPU      int
	GitBranch   string
	GitDirty    bool
	DockerCount int
	DockerAvail bool
	WeatherText string
	MailCount   int
	Refreshed   map[string]bool // which widgets were actually refreshed
}

// ImageRefreshMsg triggers a deferred sixel/iTerm2 image refresh.
// Sent when a refresh was throttled (< 50ms since last) so the final
// position update happens after the throttle window expires.
type ImageRefreshMsg struct{}

// ImageClearScreenMsg is sent when image placements are cleared (e.g. `clear`
// command). Forces a full screen repaint so BT overwrites stale sixel pixels
// that were injected via tea.Raw() and are invisible to the diff renderer.
type ImageClearScreenMsg struct{}

// WorkspaceDiscoveryMsg delivers workspace discovery results asynchronously.
type WorkspaceDiscoveryMsg struct {
	Workspaces   []string // workspace file paths
	WindowCounts []int    // window count per workspace
}

const resizeSettleDuration = 1200 * time.Millisecond // keep ticking for 1.2s after resize

func tickResizeSettle() tea.Cmd {
	return tea.Tick(50*time.Millisecond, func(t time.Time) tea.Msg {
		return ResizeSettleTickMsg{Time: t}
	})
}

func tickWorkspaceAutoSave() tea.Cmd {
	return tea.Tick(60*time.Second, func(t time.Time) tea.Msg {
		return WorkspaceAutoSaveMsg{Time: t}
	})
}

func tickCleanup() tea.Cmd {
	return tea.Tick(5*time.Minute, func(t time.Time) tea.Msg {
		return CleanupMsg{Time: t}
	})
}

package app

import (
	"math"
	"time"

	"github.com/charmbracelet/harmonica"
	"github.com/icex/termdesk/pkg/geometry"

	tea "charm.land/bubbletea/v2"
)

// AnimationType describes what kind of animation is playing.
type AnimationType int

const (
	AnimNone AnimationType = iota
	AnimOpen              // window appearing
	AnimClose             // window disappearing
	AnimMaximize          // transition to maximized
	AnimRestore           // transition from maximized
	AnimSnap              // snap to half-screen
	AnimTile              // tile layout transition
	AnimMinimize          // minimize to dock
	AnimRestore2          // restore from minimized
	AnimDockPulse         // dock item launch pulse
	AnimExpose            // transition into exposé
	AnimExposeExit        // transition out of exposé
)

// Animation represents an in-progress visual animation.
// Window/expose animations use spring physics (harmonica).
// Dock pulse uses time-based progress (not a rect animation).
type Animation struct {
	Type      AnimationType
	WindowID  string        // which window is animating (or "" for dock)
	DockIndex int           // dock item index for dock pulse
	StartRect geometry.Rect // original rect (kept for minimize recovery)
	EndRect   geometry.Rect // target position/size

	// Spring state per dimension (used for window animations)
	X, Y, W, H     float64
	VX, VY, VW, VH float64
	Spring          harmonica.Spring

	// Time-based fields (used only for dock pulse)
	StartTime time.Time
	Duration  time.Duration
	Progress  float64 // 0.0 to 1.0 (dock pulse only)

	Done bool
}

// AnimationTickMsg triggers animation frame updates.
type AnimationTickMsg struct {
	Time time.Time
}

const (
	animFrameRate   = 16 * time.Millisecond  // ~60fps
	dockPulseDur    = 400 * time.Millisecond // dock pulse
	cursorBlinkRate = 500 * time.Millisecond // cursor blink interval
)

// Spring presets for different animation feels.
var (
	springSnappy = harmonica.NewSpring(harmonica.FPS(60), 12.0, 1.0) // critically damped, fast
	springBouncy = harmonica.NewSpring(harmonica.FPS(60), 9.0, 0.6)  // slight bounce, quick
	springSmooth = harmonica.NewSpring(harmonica.FPS(60), 10.0, 0.9) // fast settle
	springExpose = harmonica.NewSpring(harmonica.FPS(60), 11.0, 1.0) // snappy expose
)

// springForType returns the appropriate spring for an animation type.
func springForType(typ AnimationType) harmonica.Spring {
	switch typ {
	case AnimOpen:
		return springBouncy
	case AnimExpose, AnimExposeExit:
		return springExpose
	case AnimTile:
		return springSmooth
	default:
		return springSnappy
	}
}

// CursorBlinkMsg triggers cursor blink state toggle.
type CursorBlinkMsg struct{}

// tickCursorBlink returns a Cmd that sends a CursorBlinkMsg after one blink interval.
func tickCursorBlink() tea.Cmd {
	return tea.Tick(cursorBlinkRate, func(t time.Time) tea.Msg {
		return CursorBlinkMsg{}
	})
}

// tickAnimation returns a Cmd that sends an AnimationTickMsg after one frame.
func tickAnimation() tea.Cmd {
	return tea.Tick(animFrameRate, func(t time.Time) tea.Msg {
		return AnimationTickMsg{Time: t}
	})
}

// settled returns true when the spring animation is close enough to the target.
func settled(a *Animation) bool {
	const posThresh = 0.5
	const velThresh = 0.1
	return math.Abs(a.X-float64(a.EndRect.X)) < posThresh &&
		math.Abs(a.Y-float64(a.EndRect.Y)) < posThresh &&
		math.Abs(a.W-float64(a.EndRect.Width)) < posThresh &&
		math.Abs(a.H-float64(a.EndRect.Height)) < posThresh &&
		math.Abs(a.VX) < velThresh && math.Abs(a.VY) < velThresh &&
		math.Abs(a.VW) < velThresh && math.Abs(a.VH) < velThresh
}

// currentRect returns the animation's current interpolated rect from spring state.
func (a *Animation) currentRect() geometry.Rect {
	return geometry.Rect{
		X:      int(math.Round(a.X)),
		Y:      int(math.Round(a.Y)),
		Width:  max(1, int(math.Round(a.W))),
		Height: max(1, int(math.Round(a.H))),
	}
}

// updateAnimations advances all active animations and returns whether any are still running.
func (m *Model) updateAnimations(now time.Time) bool {
	hasActive := false
	for i := range m.animations {
		a := &m.animations[i]
		if a.Done {
			continue
		}

		// Dock pulse: time-based progress (not a rect animation)
		if a.Type == AnimDockPulse {
			elapsed := now.Sub(a.StartTime)
			raw := float64(elapsed) / float64(a.Duration)
			if raw >= 1.0 {
				a.Progress = 1.0
				a.Done = true
			} else {
				// Smooth deceleration for pulse
				t := raw - 1
				a.Progress = t*t*t + 1
				hasActive = true
			}
			continue
		}

		// Spring physics update for all other animations
		a.X, a.VX = a.Spring.Update(a.X, a.VX, float64(a.EndRect.X))
		a.Y, a.VY = a.Spring.Update(a.Y, a.VY, float64(a.EndRect.Y))
		a.W, a.VW = a.Spring.Update(a.W, a.VW, float64(a.EndRect.Width))
		a.H, a.VH = a.Spring.Update(a.H, a.VH, float64(a.EndRect.Height))

		if settled(a) {
			a.Done = true
			m.finalizeAnimation(a)
		} else {
			hasActive = true
		}
	}

	// Remove completed animations
	active := m.animations[:0]
	for _, a := range m.animations {
		if !a.Done {
			active = append(active, a)
		}
	}
	m.animations = active
	return hasActive
}

// finalizeAnimation sets the window to its final state when animation completes.
func (m *Model) finalizeAnimation(a *Animation) {
	if a.WindowID == "" {
		return // dock animation, no window to update
	}
	w := m.wm.WindowByID(a.WindowID)
	if w == nil {
		return
	}
	switch a.Type {
	case AnimClose:
		m.closeTerminal(a.WindowID)
		m.wm.RemoveWindow(a.WindowID)
	case AnimMinimize:
		w.Minimized = true
		w.Rect = a.StartRect // restore to pre-minimize rect for later unminimize
	case AnimRestore2:
		w.Rect = a.EndRect
		w.Minimized = false
		m.resizeTerminalForWindow(w)
	case AnimExpose:
		// Window stays at its real rect; exposé rendering handles display
	case AnimExposeExit:
		// Window returns to its actual rect (already stored in w.Rect)
		m.resizeTerminalForWindow(w)
	default:
		w.Rect = a.EndRect
		m.resizeTerminalForWindow(w)
	}
}

// animatedRect returns the current interpolated rect for a window,
// or the window's actual rect if not animating.
func (m *Model) animatedRect(windowID string) (geometry.Rect, bool) {
	for _, a := range m.animations {
		if a.WindowID == windowID && !a.Done {
			return a.currentRect(), true
		}
	}
	return geometry.Rect{}, false
}

// isAnimatingClose returns true if the window is in a close animation.
func (m *Model) isAnimatingClose(windowID string) bool {
	for _, a := range m.animations {
		if a.WindowID == windowID && a.Type == AnimClose && !a.Done {
			return true
		}
	}
	return false
}

// dockPulseProgress returns the pulse progress for a dock item, or -1 if not pulsing.
func (m *Model) dockPulseProgress(dockIdx int) float64 {
	for _, a := range m.animations {
		if a.Type == AnimDockPulse && a.DockIndex == dockIdx && !a.Done {
			return a.Progress
		}
	}
	return -1
}

// startWindowAnimation creates a new spring-based animation for a window transition.
func (m *Model) startWindowAnimation(windowID string, typ AnimationType, from, to geometry.Rect) {
	m.animations = append(m.animations, Animation{
		Type:      typ,
		WindowID:  windowID,
		StartRect: from,
		EndRect:   to,
		X:         float64(from.X),
		Y:         float64(from.Y),
		W:         float64(from.Width),
		H:         float64(from.Height),
		Spring:    springForType(typ),
	})
}

// startExposeAnimation creates a spring-based animation with the expose spring preset.
func (m *Model) startExposeAnimation(windowID string, typ AnimationType, from, to geometry.Rect) {
	m.animations = append(m.animations, Animation{
		Type:      typ,
		WindowID:  windowID,
		StartRect: from,
		EndRect:   to,
		X:         float64(from.X),
		Y:         float64(from.Y),
		W:         float64(from.Width),
		H:         float64(from.Height),
		Spring:    springExpose,
	})
}

// startDockPulse starts a dock item pulse animation (time-based, not spring).
func (m *Model) startDockPulse(dockIdx int) {
	m.animations = append(m.animations, Animation{
		Type:      AnimDockPulse,
		DockIndex: dockIdx,
		StartTime: time.Now(),
		Duration:  dockPulseDur,
	})
}

// hasActiveAnimations returns whether any animations are running.
func (m *Model) hasActiveAnimations() bool {
	return len(m.animations) > 0
}

// hasExposeAnimations returns whether any expose enter/exit animations are running.
func (m *Model) hasExposeAnimations() bool {
	for _, a := range m.animations {
		if (a.Type == AnimExpose || a.Type == AnimExposeExit) && !a.Done {
			return true
		}
	}
	return false
}

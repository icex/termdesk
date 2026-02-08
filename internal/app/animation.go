package app

import (
	"math"
	"time"

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
type Animation struct {
	Type      AnimationType
	WindowID  string         // which window is animating (or "" for dock)
	DockIndex int            // dock item index for dock pulse
	StartRect geometry.Rect  // starting position/size
	EndRect   geometry.Rect  // target position/size
	StartTime time.Time
	Duration  time.Duration
	Progress  float64        // 0.0 to 1.0
	Done      bool
}

// AnimationTickMsg triggers animation frame updates.
type AnimationTickMsg struct {
	Time time.Time
}

const (
	animFrameRate  = 16 * time.Millisecond  // ~60fps
	animDuration   = 200 * time.Millisecond // window animations
	exposeDuration = 350 * time.Millisecond // exposé transitions (smoother)
	dockPulseDur   = 400 * time.Millisecond // dock pulse
)

// tickAnimation returns a Cmd that sends an AnimationTickMsg after one frame.
func tickAnimation() tea.Cmd {
	return tea.Tick(animFrameRate, func(t time.Time) tea.Msg {
		return AnimationTickMsg{Time: t}
	})
}

// easeOutCubic provides a smooth deceleration curve.
func easeOutCubic(t float64) float64 {
	t -= 1
	return t*t*t + 1
}

// easeInOutCubic provides a smooth acceleration + deceleration curve.
func easeInOutCubic(t float64) float64 {
	if t < 0.5 {
		return 4 * t * t * t
	}
	return 1 - math.Pow(-2*t+2, 3)/2
}

// easeOutBack provides an overshoot-then-settle curve (bouncy feel).
func easeOutBack(t float64) float64 {
	c1 := 1.70158
	c3 := c1 + 1
	return 1 + c3*math.Pow(t-1, 3) + c1*math.Pow(t-1, 2)
}

// interpolateRect linearly interpolates between two rects.
func interpolateRect(from, to geometry.Rect, t float64) geometry.Rect {
	lerp := func(a, b int) int {
		return a + int(float64(b-a)*t)
	}
	return geometry.Rect{
		X:      lerp(from.X, to.X),
		Y:      lerp(from.Y, to.Y),
		Width:  lerp(from.Width, to.Width),
		Height: lerp(from.Height, to.Height),
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
		elapsed := now.Sub(a.StartTime)
		raw := float64(elapsed) / float64(a.Duration)
		if raw >= 1.0 {
			a.Progress = 1.0
			a.Done = true
			m.finalizeAnimation(a)
		} else {
			switch a.Type {
			case AnimExpose, AnimExposeExit:
				a.Progress = easeInOutCubic(raw)
			default:
				a.Progress = easeOutCubic(raw)
			}
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
			t := a.Progress
			if a.Type == AnimOpen {
				t = easeOutBack(a.Progress) // bouncy open
			}
			return interpolateRect(a.StartRect, a.EndRect, t), true
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

// startWindowAnimation creates a new animation for a window transition.
func (m *Model) startWindowAnimation(windowID string, typ AnimationType, from, to geometry.Rect) {
	m.animations = append(m.animations, Animation{
		Type:      typ,
		WindowID:  windowID,
		StartRect: from,
		EndRect:   to,
		StartTime: time.Now(),
		Duration:  animDuration,
	})
}

// startExposeAnimation creates an animation with the longer exposé duration.
func (m *Model) startExposeAnimation(windowID string, typ AnimationType, from, to geometry.Rect) {
	m.animations = append(m.animations, Animation{
		Type:      typ,
		WindowID:  windowID,
		StartRect: from,
		EndRect:   to,
		StartTime: time.Now(),
		Duration:  exposeDuration,
	})
}

// startDockPulse starts a dock item pulse animation.
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

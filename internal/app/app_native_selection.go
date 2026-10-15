package app

import (
	tea "charm.land/bubbletea/v2"
	"github.com/icex/termdesk/internal/terminal"
	"github.com/icex/termdesk/pkg/geometry"
)

// Codex copies native mouse selections to its desktop clipboard. A persistent
// local session retains that clipboard even after an SSH client attaches. Keep
// forwarding mouse events to Codex, but also copy the explicitly selected cells
// through OSC 52, which follows the current attachment. Do not apply this to
// editors: a Vim visual selection is not necessarily a clipboard copy.
type nativeSelection struct {
	target   *terminal.Terminal
	rect     geometry.Rect
	start    geometry.Point
	snapshot *CopySnapshot
	dragged  bool
}

func (m *Model) beginNativeSelection(term *terminal.Terminal, rect geometry.Rect, mouse tea.Mouse) {
	if mouse.Button != tea.MouseLeft || !term.HasMouseMode() || term.ForegroundCommand() != "codex" {
		return
	}
	screen, width, height := term.SnapshotScreen()
	if screen == nil {
		return
	}
	m.nativeSelection = &nativeSelection{
		target: term, rect: rect,
		start:    geometry.Point{X: mouse.X - rect.X, Y: mouse.Y - rect.Y},
		snapshot: &CopySnapshot{Screen: screen, Width: width, Height: height},
	}
}

func (m *Model) trackNativeSelection(term *terminal.Terminal, mouse tea.Mouse) {
	if sel := m.nativeSelection; sel != nil && sel.target == term && mouse.Button == tea.MouseLeft {
		sel.dragged = true
	}
}

func (m *Model) finishNativeSelection(mouse tea.Mouse) tea.Cmd {
	sel := m.nativeSelection
	m.nativeSelection = nil
	if sel == nil || !sel.dragged || mouse.Button != tea.MouseLeft ||
		m.inputMode != ModeTerminal || m.hasActiveOverlay() || sel.target != m.pasteTarget() ||
		sel.target.IsClosed() || !sel.rect.Contains(geometry.Point{X: mouse.X, Y: mouse.Y}) ||
		sel.snapshot.Width != sel.target.Width() || sel.snapshot.Height != sel.target.Height() {
		return nil
	}
	end := geometry.Point{X: mouse.X - sel.rect.X, Y: mouse.Y - sel.rect.Y}
	start, end, ok := nativeSelectionRange(sel.start, end, sel.snapshot.Width)
	if !ok {
		return nil
	}
	text := extractSelTextWithSnapshot(sel.target, sel.snapshot, start, end)
	if text == "" {
		return nil
	}
	m.clipboard.Copy(text)
	return osc52SetCmd(text)
}

// Codex treats mouse endpoints as positions between cells; our copy extractor
// uses inclusive cell coordinates. Normalize either drag direction, then make
// the final endpoint inclusive, including a drag ending at the next row's start.
func nativeSelectionRange(start, end geometry.Point, width int) (geometry.Point, geometry.Point, bool) {
	if width <= 0 {
		return start, end, false
	}
	a, b := start.Y*width+start.X, end.Y*width+end.X
	if a == b {
		return start, end, false
	}
	if a > b {
		a, b = b, a
	}
	b--
	return geometry.Point{X: a % width, Y: a / width}, geometry.Point{X: b % width, Y: b / width}, true
}

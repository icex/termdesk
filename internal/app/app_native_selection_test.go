package app

import (
	"fmt"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/icex/termdesk/internal/terminal"
	"github.com/icex/termdesk/pkg/geometry"
)

func nativeSelectionModel(t *testing.T) Model {
	t.Helper()
	m := clipboardModel(t)
	_, term := m.focusedTerminal()
	screen, width, height := term.SnapshotScreen()
	for x, ch := range "selected Codex text" {
		screen[0][x] = terminal.ScreenCell{Content: string(ch), Width: 1}
	}
	m.nativeSelection = &nativeSelection{
		target: term, rect: geometry.Rect{Width: width, Height: height},
		start: geometry.Point{}, dragged: true,
		snapshot: &CopySnapshot{Screen: screen, Width: width, Height: height},
	}
	return m
}

func TestNativeDragCopiesSelectedTextWithoutEnteringCopyMode(t *testing.T) {
	for _, backwards := range []bool{false, true} {
		m := nativeSelectionModel(t)
		end := len("selected Codex text")
		if backwards {
			m.nativeSelection.start.X = end
			end = 0
		}
		cmd := m.finishNativeSelection(tea.Mouse{X: end, Y: 0, Button: tea.MouseLeft})
		if cmd == nil || fmt.Sprint(cmd()) != "selected Codex text" {
			t.Fatal("drag did not forward the selected cells to the terminal clipboard")
		}
		if m.clipboard.Paste() != "selected Codex text" || m.inputMode != ModeTerminal || m.copySnapshot != nil || m.nativeSelection != nil {
			t.Fatal("native drag did not preserve application selection ownership and clipboard history")
		}
	}
}

func TestNativeDragDoesNotCopyClicksOrAfterFocusOrSizeChanges(t *testing.T) {
	for _, scenario := range []string{"click", "focus", "resize", "overlay", "outside"} {
		t.Run(scenario, func(t *testing.T) {
			m := nativeSelectionModel(t)
			m.clipboard.Copy("keep previous copy")
			mouse := tea.Mouse{X: 8, Y: 0, Button: tea.MouseLeft}
			switch scenario {
			case "click":
				m.nativeSelection.dragged = false
			case "focus":
				m.wm.RemoveWindow(m.wm.FocusedWindow().ID)
			case "resize":
				m.nativeSelection.snapshot.Width--
			case "overlay":
				m.clipboard.ShowHistory()
			case "outside":
				mouse.X = m.nativeSelection.rect.Width
			}
			if m.finishNativeSelection(mouse) != nil || m.clipboard.Paste() != "keep previous copy" || m.nativeSelection != nil {
				t.Fatal("cancelled drag changed clipboard contents or left stale selection state")
			}
		})
	}
}

func TestOtherApplicationsDoNotAcquireNativeCopyBridge(t *testing.T) {
	m := clipboardModel(t)
	_, term := m.focusedTerminal()
	m.beginNativeSelection(term, geometry.Rect{Width: term.Width(), Height: term.Height()}, tea.Mouse{Button: tea.MouseLeft})
	if m.nativeSelection != nil {
		t.Fatal("clipboard bridge was enabled for an unrelated application")
	}
}

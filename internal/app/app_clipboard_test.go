package app

import (
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/icex/termdesk/internal/terminal"
	"github.com/icex/termdesk/internal/window"
	"github.com/icex/termdesk/pkg/geometry"
)

func clipboardModel(t *testing.T) Model {
	t.Helper()
	m := setupReadyModel()
	w := window.NewWindow("clipboard-test", "Clipboard", geometry.Rect{X: 0, Y: 1, Width: 50, Height: 12}, nil)
	m.wm.AddWindow(w)
	m.wm.FocusWindow(w.ID)
	term, err := terminal.New("/bin/cat", nil, 48, 10, 0, 0, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { term.Close() })
	m.terminals[w.ID] = term
	m.inputMode = ModeTerminal
	return m
}

func TestClipboardPasteRequiresUserRequest(t *testing.T) {
	m := clipboardModel(t)
	m.clipboard.Copy("saved")
	updated, _ := m.Update(tea.ClipboardMsg{Content: "unsolicited", Selection: 'c'})
	m = updated.(Model)
	if m.clipboard.Paste() != "saved" {
		t.Fatal("unsolicited clipboard reply changed saved text")
	}
	if m.requestClipboardPaste() == nil || m.pasteRequest == nil {
		t.Fatal("explicit paste did not request the host clipboard")
	}
	updated, _ = m.Update(tea.ClipboardMsg{Content: "new host clipboard", Selection: 'c'})
	m = updated.(Model)
	if m.pasteRequest != nil || m.clipboard.Paste() != "new host clipboard" {
		t.Fatal("host clipboard response did not complete paste or refresh history")
	}
}

func TestClipboardPasteTimeoutAndLateReply(t *testing.T) {
	m := clipboardModel(t)
	m.clipboard.Copy("fallback")
	m.requestClipboardPaste()
	generation := m.pasteRequest.generation
	updated, _ := m.Update(clipboardPasteTimeoutMsg(generation + 1))
	m = updated.(Model)
	if m.pasteRequest == nil {
		t.Fatal("stale timer cancelled a current paste")
	}
	updated, _ = m.Update(clipboardPasteTimeoutMsg(generation))
	m = updated.(Model)
	if m.pasteRequest != nil {
		t.Fatal("timed out paste remained pending")
	}
	updated, _ = m.Update(tea.ClipboardMsg{Content: "late", Selection: 'c'})
	m = updated.(Model)
	if m.clipboard.Paste() != "fallback" {
		t.Fatal("late response replaced clipboard after timeout")
	}
}

func TestNativePasteCancelsClipboardRead(t *testing.T) {
	m := clipboardModel(t)
	m.requestClipboardPaste()
	updated, _ := m.Update(tea.PasteMsg{Content: "native terminal paste"})
	m = updated.(Model)
	if m.pasteRequest != nil || m.clipboard.Paste() != "native terminal paste" {
		t.Fatal("native paste did not cancel pending read and refresh fallback")
	}
}

func TestPasteShortcutsAndLiteralControlV(t *testing.T) {
	for _, key := range []tea.Key{
		{Code: tea.KeyInsert, Mod: tea.ModShift},
		{Code: 'v', Mod: tea.ModCtrl | tea.ModShift},
	} {
		m := clipboardModel(t)
		updated, cmd := m.handleKeyPress(tea.KeyPressMsg(key))
		if updated.(Model).pasteRequest == nil || cmd == nil {
			t.Fatalf("%s did not request text paste", key.String())
		}
	}
	m := clipboardModel(t)
	updated, _ := m.handleKeyPress(tea.KeyPressMsg{Code: 'v', Mod: tea.ModCtrl})
	if updated.(Model).pasteRequest != nil {
		t.Fatal("Ctrl+V was stolen from the child app")
	}
}

func TestEmptyClipboardReplyDoesNotPasteStaleFallback(t *testing.T) {
	m := clipboardModel(t)
	m.clipboard.Copy("stale saved text")
	m.requestClipboardPaste()
	generation := m.pasteGeneration
	updated, _ := m.Update(tea.ClipboardMsg{Selection: 'p', Content: "primary"})
	m = updated.(Model)
	if m.pasteRequest == nil {
		t.Fatal("primary-selection reply satisfied a system clipboard request")
	}
	updated, _ = m.Update(tea.ClipboardMsg{Selection: 'c'})
	m = updated.(Model)
	if m.pasteRequest != nil {
		t.Fatal("empty but valid reply left a stale fallback armed")
	}
	updated, _ = m.Update(clipboardPasteTimeoutMsg(generation))
	m = updated.(Model)
	if len(m.notifications.History()) != 1 {
		t.Fatal("empty clipboard was not reported exactly once")
	}
}

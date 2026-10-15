package app

import (
	"encoding/base64"
	"sync"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/icex/termdesk/internal/notification"
	"github.com/icex/termdesk/internal/terminal"
)

// clipboardForwarder preserves copy order without blocking an emulator on the
// UI. A goroutine per copy can deliver older text after newer text. Likewise,
// returning separate tea.Cmds lets Bubble Tea execute clipboard writes out of
// order. One worker sends both the history event and BT's clipboard message
// directly, in order. Pending history is bounded to the clipboard's five slots.
type clipboardForwarder struct {
	mu      sync.Mutex
	pending []tea.Msg
	running bool
}

func (f *clipboardForwarder) copy(p *tea.Program, msg TerminalClipboardMsg) {
	msg.forwarded = true
	f.enqueue(p, msg)
}

func (f *clipboardForwarder) enqueue(p *tea.Program, msg tea.Msg) {
	f.mu.Lock()
	if len(f.pending) == 5 {
		copy(f.pending, f.pending[1:])
		f.pending = f.pending[:4]
	}
	f.pending = append(f.pending, msg)
	start := !f.running
	f.running = true
	f.mu.Unlock()
	if start {
		go f.forward(p)
	}
}

func (f *clipboardForwarder) forward(p *tea.Program) {
	for {
		f.mu.Lock()
		if len(f.pending) == 0 {
			f.pending = nil
			f.running = false
			f.mu.Unlock()
			return
		}
		msg := f.pending[0]
		f.pending[0] = nil // release text once sent
		f.pending = f.pending[1:]
		f.mu.Unlock()
		safeSend(p, msg)
		if copied, ok := msg.(TerminalClipboardMsg); ok {
			safeSend(p, tea.SetClipboard(copied.Text)())
		}
	}
}

type terminalClipboardReadMsg struct {
	target    *terminal.Terminal
	selection string
}

type pasteRequest struct {
	paste      bool
	readers    []terminalClipboardReadMsg
	generation uint64
	target     *terminal.Terminal
	fallback   string
}

type clipboardPasteTimeoutMsg uint64

func (m *Model) pasteTarget() *terminal.Terminal {
	if m.quakeVisible && m.quakeTerminal != nil {
		return m.quakeTerminal
	}
	_, term := m.focusedTerminal()
	return term
}

// requestClipboardPaste reads the connecting terminal's clipboard, never the
// SSH server's desktop clipboard. Some terminals disable OSC 52 reads; retain
// the internal clipboard as a bounded fallback for those terminals.
func (m *Model) requestClipboardPaste() tea.Cmd {
	if m.inputMode == ModeCopy {
		m.exitCopyMode()
	}
	m.inputMode = ModeTerminal
	target := m.pasteTarget()
	if target == nil {
		return nil
	}
	if m.pasteRequest != nil {
		m.pasteRequest.paste = true
		m.pasteRequest.target = target
		return nil
	}
	m.pasteGeneration++
	m.pasteRequest = &pasteRequest{paste: true, generation: m.pasteGeneration, target: target, fallback: m.clipboard.Paste()}
	return clipboardReadCmd(m.pasteGeneration)
}

func clipboardReadCmd(generation uint64) tea.Cmd {
	return tea.Batch(tea.ReadClipboard, tea.Tick(time.Second, func(time.Time) tea.Msg {
		return clipboardPasteTimeoutMsg(generation)
	}))
}

func (m *Model) requestChildClipboardRead(reader terminalClipboardReadMsg) tea.Cmd {
	if reader.target == nil || reader.target.IsClosed() {
		return nil
	}
	if m.pasteRequest != nil {
		if len(m.pasteRequest.readers) < 8 {
			m.pasteRequest.readers = append(m.pasteRequest.readers, reader)
		} else {
			replyChildClipboard(reader, m.clipboard.Paste())
		}
		return nil
	}
	m.pasteGeneration++
	m.pasteRequest = &pasteRequest{generation: m.pasteGeneration, fallback: m.clipboard.Paste(), readers: []terminalClipboardReadMsg{reader}}
	return clipboardReadCmd(m.pasteGeneration)
}

func replyChildClipboard(reader terminalClipboardReadMsg, text string) {
	if !reader.target.IsClosed() {
		reader.target.QueueResponse([]byte("\x1b]52;" + reader.selection + ";" + base64.StdEncoding.EncodeToString([]byte(text)) + "\x07"))
	}
}

func (m *Model) finishClipboardPaste(req *pasteRequest, text string) {
	for _, reader := range req.readers {
		replyChildClipboard(reader, text)
	}
	if !req.paste {
		return
	}
	// A delayed reply must never go to a newly focused pane/workspace, or to
	// a pane now hidden by a dialog. The user can retry after moving focus.
	if m.pasteTarget() != req.target || m.hasActiveOverlay() || m.menuBar.IsOpen() || req.target.IsClosed() {
		return
	}
	if text == "" {
		m.notifications.Push("Clipboard", "No text available. Use your terminal's Paste command to send local clipboard text.", notification.Info)
		return
	}
	m.pasteText(text)
}

func (m *Model) pasteText(text string) {
	// Pasted data is never interpreted as application shortcuts. In particular,
	// pasting over a modal must not accidentally type into the shell behind it.
	if m.hasActiveOverlay() || m.menuBar.IsOpen() {
		return
	}
	if m.inputMode == ModeCopy {
		m.exitCopyMode()
		m.inputMode = ModeTerminal
	}
	if term := m.pasteTarget(); term != nil && text != "" {
		term.Paste(text)
	}
}

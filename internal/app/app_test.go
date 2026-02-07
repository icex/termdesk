package app

import (
	"testing"

	tea "charm.land/bubbletea/v2"
)

func TestNew(t *testing.T) {
	m := New()
	if m.width != 0 || m.height != 0 {
		t.Error("expected zero dimensions on new model")
	}
	if m.ready {
		t.Error("expected model to not be ready initially")
	}
}

func TestInit(t *testing.T) {
	m := New()
	cmd := m.Init()
	if cmd != nil {
		t.Error("expected nil cmd from Init")
	}
}

func TestUpdateWindowSize(t *testing.T) {
	m := New()
	msg := tea.WindowSizeMsg{Width: 120, Height: 40}
	updated, _ := m.Update(msg)
	model := updated.(Model)
	if model.width != 120 || model.height != 40 {
		t.Errorf("expected 120x40, got %dx%d", model.width, model.height)
	}
	if !model.ready {
		t.Error("expected model to be ready after window size msg")
	}
}

func TestUpdateQuitQ(t *testing.T) {
	m := New()
	m.ready = true

	msg := tea.KeyPressMsg(tea.Key{Code: 'q'})
	_, cmd := m.Update(msg)
	if cmd == nil {
		t.Error("expected quit command on q")
	}
}

func TestViewSetsAltScreen(t *testing.T) {
	m := New()
	m.ready = true
	m.width = 80
	m.height = 24
	v := m.View()
	if !v.AltScreen {
		t.Error("expected AltScreen to be true")
	}
}

func TestViewSetsMouseMode(t *testing.T) {
	m := New()
	m.ready = true
	v := m.View()
	if v.MouseMode != tea.MouseModeAllMotion {
		t.Errorf("expected MouseModeAllMotion, got %v", v.MouseMode)
	}
}

func TestViewBeforeReady(t *testing.T) {
	m := New()
	v := m.View()
	if v.Content == nil {
		t.Error("expected content to be set before ready")
	}
}

func TestViewAfterReady(t *testing.T) {
	m := New()
	m.ready = true
	m.width = 80
	m.height = 24
	v := m.View()
	// View should not be nil after ready
	if v.AltScreen != true {
		t.Error("expected alt screen mode")
	}
}

func TestUpdateUnknownMsgPassthrough(t *testing.T) {
	m := New()
	// Some random message type
	type customMsg struct{}
	updated, cmd := m.Update(customMsg{})
	model := updated.(Model)
	if model.ready {
		t.Error("unknown message should not change ready state")
	}
	if cmd != nil {
		t.Error("unknown message should not produce a command")
	}
}

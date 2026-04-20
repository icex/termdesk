package terminal

import (
	"testing"

	uv "github.com/charmbracelet/ultraviolet"
)

func TestEncodeKeyPrintable(t *testing.T) {
	got := encodeKey('a', 0, "a")
	if string(got) != "a" {
		t.Errorf("printable 'a': got %q", got)
	}
}

func TestEncodeKeyText(t *testing.T) {
	got := encodeKey('A', uv.ModShift, "A")
	if string(got) != "A" {
		t.Errorf("shift+a text: got %q", got)
	}
}

func TestEncodeKeyCtrlC(t *testing.T) {
	got := encodeKey('c', uv.ModCtrl, "")
	if len(got) != 1 || got[0] != 0x03 {
		t.Errorf("ctrl+c: got %v, want [0x03]", got)
	}
}

func TestEncodeKeyCtrlD(t *testing.T) {
	got := encodeKey('d', uv.ModCtrl, "")
	if len(got) != 1 || got[0] != 0x04 {
		t.Errorf("ctrl+d: got %v, want [0x04]", got)
	}
}

func TestEncodeKeyCtrlZ(t *testing.T) {
	got := encodeKey('z', uv.ModCtrl, "")
	if len(got) != 1 || got[0] != 0x1A {
		t.Errorf("ctrl+z: got %v, want [0x1A]", got)
	}
}

func TestEncodeKeyCtrlBracket(t *testing.T) {
	got := encodeKey('[', uv.ModCtrl, "")
	if len(got) != 1 || got[0] != 0x1B {
		t.Errorf("ctrl+[: got %v, want [0x1B]", got)
	}
}

func TestEncodeKeyEnter(t *testing.T) {
	got := encodeKey(uv.KeyEnter, 0, "")
	if string(got) != "\r" {
		t.Errorf("enter: got %q", got)
	}
}

func TestEncodeKeyTab(t *testing.T) {
	got := encodeKey(uv.KeyTab, 0, "")
	if string(got) != "\t" {
		t.Errorf("tab: got %q", got)
	}
}

func TestEncodeKeyShiftTab(t *testing.T) {
	got := encodeKey(uv.KeyTab, uv.ModShift, "")
	if string(got) != "\x1b[Z" {
		t.Errorf("shift+tab: got %q", got)
	}
}

func TestEncodeKeyBackspace(t *testing.T) {
	got := encodeKey(uv.KeyBackspace, 0, "")
	if len(got) != 1 || got[0] != 0x7F {
		t.Errorf("backspace: got %v", got)
	}
}

func TestEncodeKeyEscape(t *testing.T) {
	got := encodeKey(uv.KeyEscape, 0, "")
	if len(got) != 1 || got[0] != 0x1B {
		t.Errorf("escape: got %v", got)
	}
}

func TestEncodeKeyArrowUp(t *testing.T) {
	got := encodeKey(uv.KeyUp, 0, "")
	if string(got) != "\x1b[A" {
		t.Errorf("up: got %q", got)
	}
}

func TestEncodeKeyArrowDown(t *testing.T) {
	got := encodeKey(uv.KeyDown, 0, "")
	if string(got) != "\x1b[B" {
		t.Errorf("down: got %q", got)
	}
}

func TestEncodeKeyArrowRight(t *testing.T) {
	got := encodeKey(uv.KeyRight, 0, "")
	if string(got) != "\x1b[C" {
		t.Errorf("right: got %q", got)
	}
}

func TestEncodeKeyArrowLeft(t *testing.T) {
	got := encodeKey(uv.KeyLeft, 0, "")
	if string(got) != "\x1b[D" {
		t.Errorf("left: got %q", got)
	}
}

func TestEncodeKeyArrowWithMod(t *testing.T) {
	// Ctrl+Up = ESC[1;5A
	got := encodeKey(uv.KeyUp, uv.ModCtrl, "")
	if string(got) != "\x1b[1;5A" {
		t.Errorf("ctrl+up: got %q, want \\x1b[1;5A", got)
	}
}

func TestEncodeKeyAltChar(t *testing.T) {
	got := encodeKey('x', uv.ModAlt, "")
	if string(got) != "\x1bx" {
		t.Errorf("alt+x: got %q", got)
	}
}

func TestEncodeKeyAltCharWithText(t *testing.T) {
	got := encodeKey('x', uv.ModAlt, "x")
	if string(got) != "\x1bx" {
		t.Errorf("alt+x (text): got %q", got)
	}
}

func TestEncodeKeyHome(t *testing.T) {
	got := encodeKey(uv.KeyHome, 0, "")
	if string(got) != "\x1b[H" {
		t.Errorf("home: got %q", got)
	}
}

func TestEncodeKeyEnd(t *testing.T) {
	got := encodeKey(uv.KeyEnd, 0, "")
	if string(got) != "\x1b[F" {
		t.Errorf("end: got %q", got)
	}
}

func TestEncodeKeyDelete(t *testing.T) {
	got := encodeKey(uv.KeyDelete, 0, "")
	if string(got) != "\x1b[3~" {
		t.Errorf("delete: got %q", got)
	}
}

func TestEncodeKeyFunctionKeys(t *testing.T) {
	tests := []struct {
		code rune
		want string
	}{
		{uv.KeyF1, "\x1bOP"},
		{uv.KeyF2, "\x1bOQ"},
		{uv.KeyF3, "\x1bOR"},
		{uv.KeyF4, "\x1bOS"},
		{uv.KeyF5, "\x1b[15~"},
		{uv.KeyF12, "\x1b[24~"},
	}
	for _, tt := range tests {
		got := encodeKey(tt.code, 0, "")
		if string(got) != tt.want {
			t.Errorf("F key %d: got %q, want %q", tt.code, got, tt.want)
		}
	}
}

func TestEncodeKeySpace(t *testing.T) {
	got := encodeKey(uv.KeySpace, 0, "")
	if string(got) != " " {
		t.Errorf("space: got %q", got)
	}
}

func TestEncodeKeyUnknown(t *testing.T) {
	got := encodeKey(0, 0, "")
	if got != nil {
		t.Errorf("unknown key: got %v, want nil", got)
	}
}

func TestEncodeKeyUTF8(t *testing.T) {
	got := encodeKey('日', 0, "日")
	if string(got) != "日" {
		t.Errorf("utf8: got %q", got)
	}
}

func TestModParam(t *testing.T) {
	tests := []struct {
		mod  uv.KeyMod
		want int
	}{
		{0, 1},
		{uv.ModShift, 2},
		{uv.ModAlt, 3},
		{uv.ModShift | uv.ModAlt, 4},
		{uv.ModCtrl, 5},
		{uv.ModShift | uv.ModCtrl, 6},
	}
	for _, tt := range tests {
		got := modParam(tt.mod)
		if got != tt.want {
			t.Errorf("modParam(%v) = %d, want %d", tt.mod, got, tt.want)
		}
	}
}

func TestEncodeKeyInsert(t *testing.T) {
	got := encodeKey(uv.KeyInsert, 0, "")
	if string(got) != "\x1b[2~" {
		t.Errorf("insert: got %q", got)
	}
}

func TestEncodeKeyPgUpDown(t *testing.T) {
	up := encodeKey(uv.KeyPgUp, 0, "")
	if string(up) != "\x1b[5~" {
		t.Errorf("pgup: got %q", up)
	}
	down := encodeKey(uv.KeyPgDown, 0, "")
	if string(down) != "\x1b[6~" {
		t.Errorf("pgdown: got %q", down)
	}
}

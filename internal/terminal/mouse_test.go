package terminal

import "testing"

func TestEncodeMouse(t *testing.T) {
	got := string(encodeMouse(MouseLeft, 10, 5, false))
	want := "\x1b[<0;10;5M"
	if got != want {
		t.Fatalf("encodeMouse=%q want %q", got, want)
	}

	got = string(encodeMouse(MouseRight, 1, 1, true))
	want = "\x1b[<2;1;1m"
	if got != want {
		t.Fatalf("encodeMouse release=%q want %q", got, want)
	}
}

func TestEncodeMouseMotion(t *testing.T) {
	got := string(encodeMouseMotion(MouseLeft, 3, 4))
	want := "\x1b[<32;3;4M"
	if got != want {
		t.Fatalf("encodeMouseMotion=%q want %q", got, want)
	}
}

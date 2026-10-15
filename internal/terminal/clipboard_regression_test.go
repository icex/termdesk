package terminal

import (
	"encoding/base64"
	"reflect"
	"testing"
)

func TestOSC52EveryReadBoundary(t *testing.T) {
	text := "Claude → Vim\n日本語 clipboard"
	for _, end := range []string{"\x07", "\x1b\\"} {
		seq := "before\x1b]52;c;" + base64.StdEncoding.EncodeToString([]byte(text)) + end + "after"
		for split := 1; split < len(seq); split++ {
			var copied []string
			term := &Terminal{onClipboard: func(s string) { copied = append(copied, s) }}
			out := append(term.extractOSC52([]byte(seq[:split])), term.extractOSC52([]byte(seq[split:]))...)
			if string(out) != "beforeafter" || !reflect.DeepEqual(copied, []string{text}) {
				t.Fatalf("terminator %q split %d: output %q, copies %q", end, split, out, copied)
			}
		}
	}
}

func TestOSC52BytewiseAndConsecutiveCopies(t *testing.T) {
	var copied []string
	term := &Terminal{onClipboard: func(s string) { copied = append(copied, s) }}
	seq := "\x1b]52;c;Zmlyc3Q=\x07\x1b]52;c;c2Vjb25k\x1b\\visible"
	var out []byte
	for _, b := range []byte(seq) {
		out = append(out, term.extractOSC52([]byte{b})...)
	}
	if string(out) != "visible" || !reflect.DeepEqual(copied, []string{"first", "second"}) {
		t.Fatalf("output %q, copies %q", out, copied)
	}
}

func TestOSC52PartialPrefixDoesNotEatOtherControls(t *testing.T) {
	for _, seq := range []string{"\x1b[31mred\x1b[0m", "\x1b]520;other\x07", "\x1b]8;;url\x1b\\link"} {
		term := &Terminal{}
		var out []byte
		for _, b := range []byte(seq) {
			out = append(out, term.extractOSC52([]byte{b})...)
		}
		if string(out) != seq {
			t.Fatalf("output %q, want %q", out, seq)
		}
	}
}

func TestOSC52ReadQueryAtEveryBoundary(t *testing.T) {
	for _, seq := range []string{"\x1b]52;c;?\x07", "\x1b]52;;?\x1b\\"} {
		for split := 1; split < len(seq); split++ {
			var selections []string
			term := &Terminal{onClipboardRead: func(s string) { selections = append(selections, s) }}
			term.extractOSC52([]byte(seq[:split]))
			term.extractOSC52([]byte(seq[split:]))
			if !reflect.DeepEqual(selections, []string{"c"}) {
				t.Fatalf("split %d: selections %q", split, selections)
			}
		}
	}
}

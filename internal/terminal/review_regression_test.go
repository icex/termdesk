package terminal

import (
	"bytes"
	"testing"
	"time"
)

func regressionTerminal(t *testing.T) *Terminal {
	t.Helper()
	tr, err := New("/bin/cat", nil, 80, 24, 0, 0, "")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { tr.Close() })
	return tr
}
func TestCloseDrainsQueuedOutput(t *testing.T) {
	tr := regressionTerminal(t)
	entered, release := make(chan struct{}), make(chan struct{})
	tr.SetOnOutput(func() { close(entered); <-release })
	tr.emuCh <- []byte("one")
	<-entered
	tr.emuCh <- []byte("two")
	done := make(chan struct{})
	go func() { tr.Close(); close(done) }()
	deadline := time.After(time.Second)
	for !tr.IsClosed() {
		select {
		case <-deadline:
			t.Fatal("Close did not begin")
		default:
			time.Sleep(time.Millisecond)
		}
	}
	close(release)
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("Close hung with queued output")
	}
}
func TestTypingAndScreenRedrawDoNotCreateScrollback(t *testing.T) {
	tr := regressionTerminal(t)
	for _, data := range []string{"abc", "\x1b[Hrewritten", "\x1b[2J\x1b[H"} {
		tr.safeEmuWrite([]byte(data))
		if data != "\x1b[2J\x1b[H" && tr.ScrollbackLen() != 0 {
			t.Fatal("typing created scrollback")
		}
	}
	// Real linefeeds at the bottom must still record history.
	tr.safeEmuWrite([]byte("\x1b[24;1Hbottom\r\n"))
	if tr.ScrollbackLen() == 0 {
		t.Fatal("real scroll was lost")
	}
}
func TestKittyAPCSizeLimitAndRecovery(t *testing.T) {
	for _, terminator := range [][]byte{[]byte("\x1b\\"), []byte("\x07")} {
		tr := &Terminal{}
		tr.extractKittyAPCs(append([]byte("\x1b_G"), bytes.Repeat([]byte("A"), maxKittyImageSize+1)...))
		if len(tr.kittyAPCBuf) > maxKittyImageSize {
			t.Fatal("unterminated APC grew beyond cap")
		}
		tr.extractKittyAPCs(bytes.Repeat([]byte("B"), 65536))
		_, text := tr.extractKittyAPCs(append(terminator, []byte("after")...))
		if string(text) != "after" {
			t.Fatalf("text after rejected APC lost: %q", text)
		}
	}
	tr := &Terminal{}
	tr.extractKittyAPCs(append([]byte("\x1b_G"), bytes.Repeat([]byte("A"), maxKittyImageSize+1)...))
	tr.extractKittyAPCs([]byte("\x1b"))
	_, text := tr.extractKittyAPCs([]byte("\\after"))
	if string(text) != "after" {
		t.Fatal("split ST not recovered")
	}
	tr = &Terminal{}
	segs, _ := tr.extractKittyAPCs(append(append([]byte("\x1b_G"), bytes.Repeat([]byte("A"), maxKittyImageSize+1)...), []byte("\x1b\\")...))
	for _, seg := range segs {
		if seg.Cmd != nil {
			t.Fatal("oversized complete APC parsed")
		}
	}
}
func TestKittyChunkLimit(t *testing.T) {
	kp := newTestKP()
	cmd := &KittyCommand{Action: KittyActionTransmit, Medium: KittyMediumDirect, More: true, Data: bytes.Repeat([]byte("a"), maxKittyImageSize)}
	kp.ForwardCommand(cmd, nil, "w", 0, 0, 80, 24, 1, 1, 0, 0, 0, false, nil)
	cmd.Data = []byte("one byte too much")
	kp.ForwardCommand(cmd, nil, "w", 0, 0, 80, 24, 1, 1, 0, 0, 0, false, nil)
	if kp.pendingDirect["w"] != nil {
		t.Fatal("oversized chunked image retained")
	}
}
func TestKittySeparateTransmitAndPlaceUseSameMappedID(t *testing.T) {
	kp := newTestKP()
	for _, win := range []string{"first", "second"} {
		raw := []byte("a=t,i=42,f=100;AAAA")
		cmd, _ := ParseKittyCommand(raw)
		kp.ForwardCommand(cmd, raw, win, 0, 0, 80, 24, 1, 1, 0, 0, 0, false, nil)
		mapped := kp.imageIDMap[win][42]
		if mapped == 0 {
			t.Fatal("transmitted ID not mapped")
		}
		transmit := kp.FlushPending()
		if !bytes.Contains(transmit, []byte("a=t,i=")) {
			t.Fatal("transmission lost")
		}
		kp.ForwardCommand(&KittyCommand{Action: KittyActionPlace, ImageID: 42}, nil, win, 0, 0, 80, 24, 1, 1, 0, 0, 0, false, nil)
		if kp.placements[win][mapped] == nil {
			t.Fatal("placement used another image ID")
		}
	}
	if kp.imageIDMap["first"][42] == kp.imageIDMap["second"][42] {
		t.Fatal("cross-window collision")
	}
}
func TestAllFilesystemGraphicsQueriesAreRejected(t *testing.T) {
	kp := newTestKP()
	for _, medium := range []KittyGraphicsMedium{KittyMediumFile, KittyMediumTempFile, KittyMediumSharedMemory} {
		var reply []byte
		kp.ForwardCommand(&KittyCommand{Action: KittyActionQuery, Medium: medium}, nil, "w", 0, 0, 0, 0, 0, 0, 0, 0, 0, false, func(b []byte) { reply = b })
		if !bytes.Contains(reply, []byte("ENOTSUPPORTED")) {
			t.Fatalf("medium %c accepted", medium)
		}
	}
}

func TestKittyDeclaredDimensionsAreBounded(t *testing.T) {
	kp := newTestKP()
	var reply []byte
	kp.ForwardCommand(&KittyCommand{Action: KittyActionTransmitPlace, Medium: KittyMediumDirect, Rows: 1 << 30, Data: []byte("a")}, nil, "w", 0, 0, 80, 24, 1, 1, 0, 0, 0, false, func(b []byte) { reply = b })
	if !bytes.Contains(reply, []byte("EINVAL")) || len(kp.pendingDirect) != 0 || len(kp.FlushPending()) != 0 {
		t.Fatal("unbounded dimensions accepted")
	}
}

func TestNonReadingChildDoesNotBlockInputForwarder(t *testing.T) {
	tr, err := New("/bin/sh", []string{"-c", "stty raw -echo; sleep 10"}, 80, 24, 0, 0, "")
	if err != nil {
		t.Fatal(err)
	}
	defer tr.Close()
	tr.emuCh <- bytes.Repeat([]byte("\x1b[5n"), 10000)
	done := make(chan struct{})
	go func() { tr.SendKey('x', 0, "x"); close(done) }()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("non-reading child blocked UI input")
	}
}

func TestBracketedPasteCannotBeTerminatedByNestedSequence(t *testing.T) {
	term := &Terminal{writeCh: make(chan []byte, 1), bracketedPaste: true}
	term.Paste("\x1b[20\x1b[201~1~; rm -rf ~\n")
	got := <-term.writeCh
	if !bytes.HasPrefix(got, []byte("\x1b[200~")) || bytes.Count(got, []byte("\x1b[201~")) != 1 || !bytes.HasSuffix(got, []byte("\x1b[201~")) {
		t.Fatalf("paste terminated early: %q", got)
	}
}

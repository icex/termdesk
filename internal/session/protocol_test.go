package session

import (
	"bytes"
	"encoding/binary"
	"io"
	"testing"
)

func TestWriteReadMsg(t *testing.T) {
	tests := []struct {
		name    string
		typ     byte
		payload []byte
	}{
		{"input", MsgInput, []byte("hello world")},
		{"output", MsgOutput, []byte("\x1b[31mred\x1b[0m")},
		{"resize", MsgResize, EncodeResize(24, 80)},
		{"detach", MsgDetach, nil},
		{"redraw", MsgRedraw, []byte("screen content")},
		{"empty output", MsgOutput, nil},
		{"large payload", MsgInput, bytes.Repeat([]byte("x"), 4096)},
		{"single byte", MsgInput, []byte{0x42}},
		{"binary payload", MsgOutput, []byte{0x00, 0xff, 0x80, 0x01}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			if err := WriteMsg(&buf, tt.typ, tt.payload); err != nil {
				t.Fatalf("WriteMsg: %v", err)
			}

			typ, payload, err := ReadMsg(&buf)
			if err != nil {
				t.Fatalf("ReadMsg: %v", err)
			}
			if typ != tt.typ {
				t.Errorf("type = %c, want %c", typ, tt.typ)
			}
			if !bytes.Equal(payload, tt.payload) {
				t.Errorf("payload mismatch: got %d bytes, want %d", len(payload), len(tt.payload))
			}
		})
	}
}

func TestEncodeDecodeResize(t *testing.T) {
	rows, cols := uint16(50), uint16(120)
	data := EncodeResize(rows, cols)
	if len(data) != 4 {
		t.Fatalf("EncodeResize returned %d bytes, want 4", len(data))
	}
	gotRows, gotCols := DecodeResize(data)
	if gotRows != rows || gotCols != cols {
		t.Errorf("DecodeResize = (%d, %d), want (%d, %d)", gotRows, gotCols, rows, cols)
	}
}

func TestEncodeDecodeResizeVariousSizes(t *testing.T) {
	tests := []struct {
		name string
		rows uint16
		cols uint16
	}{
		{"standard 24x80", 24, 80},
		{"large 200x300", 200, 300},
		{"zero", 0, 0},
		{"max uint16", 65535, 65535},
		{"1x1", 1, 1},
		{"tall narrow", 500, 10},
		{"short wide", 5, 1000},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data := EncodeResize(tt.rows, tt.cols)
			gotRows, gotCols := DecodeResize(data)
			if gotRows != tt.rows || gotCols != tt.cols {
				t.Errorf("got (%d, %d), want (%d, %d)", gotRows, gotCols, tt.rows, tt.cols)
			}
		})
	}
}

func TestDecodeResizeShort(t *testing.T) {
	rows, cols := DecodeResize([]byte{1, 2})
	if rows != 0 || cols != 0 {
		t.Errorf("short payload should return (0, 0), got (%d, %d)", rows, cols)
	}
}

func TestDecodeResizeEmpty(t *testing.T) {
	rows, cols := DecodeResize(nil)
	if rows != 0 || cols != 0 {
		t.Errorf("nil payload should return (0, 0), got (%d, %d)", rows, cols)
	}
}

func TestDecodeResizeThreeBytes(t *testing.T) {
	rows, cols := DecodeResize([]byte{0, 1, 2})
	if rows != 0 || cols != 0 {
		t.Errorf("3-byte payload should return (0, 0), got (%d, %d)", rows, cols)
	}
}

func TestDecodeResizeLongerPayload(t *testing.T) {
	// If we have more than 4 bytes, only the first 4 should matter.
	data := EncodeResize(24, 80)
	data = append(data, 0xff, 0xff) // extra garbage
	rows, cols := DecodeResize(data)
	if rows != 24 || cols != 80 {
		t.Errorf("got (%d, %d), want (24, 80)", rows, cols)
	}
}

func TestReadMsgTooLarge(t *testing.T) {
	var buf bytes.Buffer
	// Write a header claiming MaxPayload+1 bytes
	header := [5]byte{MsgInput}
	header[1] = 0x00
	header[2] = 0x10
	header[3] = 0x00
	header[4] = 0x01 // 1048577 = MaxPayload+1
	buf.Write(header[:])

	_, _, err := ReadMsg(&buf)
	if err == nil {
		t.Error("expected error for too-large message")
	}
}

func TestReadMsgTruncatedPayload(t *testing.T) {
	var buf bytes.Buffer
	// Write a header claiming 10 bytes of payload, but only provide 3.
	header := [5]byte{MsgInput, 0, 0, 0, 10}
	buf.Write(header[:])
	buf.Write([]byte("abc")) // only 3 of the promised 10 bytes

	_, _, err := ReadMsg(&buf)
	if err == nil {
		t.Error("expected error for truncated payload")
	}
}

func TestReadMsgEmptyReader(t *testing.T) {
	var buf bytes.Buffer
	_, _, err := ReadMsg(&buf)
	if err == nil {
		t.Error("expected error reading from empty reader")
	}
}

func TestReadMsgPartialHeader(t *testing.T) {
	// Only 3 bytes of a 5-byte header.
	var buf bytes.Buffer
	buf.Write([]byte{MsgInput, 0, 0})
	_, _, err := ReadMsg(&buf)
	if err == nil {
		t.Error("expected error for partial header")
	}
}

func TestMultipleMessages(t *testing.T) {
	var buf bytes.Buffer
	WriteMsg(&buf, MsgInput, []byte("hello"))
	WriteMsg(&buf, MsgResize, EncodeResize(24, 80))
	WriteMsg(&buf, MsgDetach, nil)

	typ1, p1, _ := ReadMsg(&buf)
	typ2, p2, _ := ReadMsg(&buf)
	typ3, _, _ := ReadMsg(&buf)

	if typ1 != MsgInput || string(p1) != "hello" {
		t.Errorf("msg1: type=%c payload=%q", typ1, p1)
	}
	if typ2 != MsgResize || len(p2) != 4 {
		t.Errorf("msg2: type=%c payload len=%d", typ2, len(p2))
	}
	if typ3 != MsgDetach {
		t.Errorf("msg3: type=%c, want D", typ3)
	}
}

func TestWriteMsgHeaderFormat(t *testing.T) {
	// Verify the exact wire format of a message.
	var buf bytes.Buffer
	payload := []byte("test")
	WriteMsg(&buf, MsgOutput, payload)

	raw := buf.Bytes()
	// Expected: 1 byte type + 4 bytes length + payload
	if len(raw) != 5+len(payload) {
		t.Fatalf("raw length = %d, want %d", len(raw), 5+len(payload))
	}
	if raw[0] != MsgOutput {
		t.Errorf("type byte = %c, want %c", raw[0], MsgOutput)
	}
	length := binary.BigEndian.Uint32(raw[1:5])
	if length != uint32(len(payload)) {
		t.Errorf("length field = %d, want %d", length, len(payload))
	}
	if !bytes.Equal(raw[5:], payload) {
		t.Errorf("payload mismatch")
	}
}

func TestWriteMsgNilPayload(t *testing.T) {
	var buf bytes.Buffer
	if err := WriteMsg(&buf, MsgDetach, nil); err != nil {
		t.Fatalf("WriteMsg with nil payload: %v", err)
	}

	raw := buf.Bytes()
	if len(raw) != 5 {
		t.Fatalf("expected 5 bytes for nil-payload message, got %d", len(raw))
	}
	length := binary.BigEndian.Uint32(raw[1:5])
	if length != 0 {
		t.Errorf("expected length=0 for nil payload, got %d", length)
	}
}

func TestWriteMsgToFailingWriter(t *testing.T) {
	err := WriteMsg(&failWriter{}, MsgInput, []byte("test"))
	if err == nil {
		t.Error("expected error writing to failing writer")
	}
}

func TestReadMsgFromEOF(t *testing.T) {
	// Reading from an io.Reader that returns EOF immediately.
	r := bytes.NewReader(nil)
	_, _, err := ReadMsg(r)
	if err == nil {
		t.Error("expected error reading from EOF reader")
	}
}

func TestMsgConstants(t *testing.T) {
	// Verify that message type constants have expected values.
	if MsgInput != 'I' {
		t.Errorf("MsgInput = %c, want 'I'", MsgInput)
	}
	if MsgOutput != 'O' {
		t.Errorf("MsgOutput = %c, want 'O'", MsgOutput)
	}
	if MsgResize != 'R' {
		t.Errorf("MsgResize = %c, want 'R'", MsgResize)
	}
	if MsgDetach != 'D' {
		t.Errorf("MsgDetach = %c, want 'D'", MsgDetach)
	}
	if MsgRedraw != 'S' {
		t.Errorf("MsgRedraw = %c, want 'S'", MsgRedraw)
	}
}

func TestMaxPayloadConstant(t *testing.T) {
	if MaxPayload != 1<<20 {
		t.Errorf("MaxPayload = %d, want %d", MaxPayload, 1<<20)
	}
}

func TestReadMsgExactMaxPayload(t *testing.T) {
	// Write a message with exactly MaxPayload size (should succeed).
	var buf bytes.Buffer
	payload := bytes.Repeat([]byte("A"), MaxPayload)
	if err := WriteMsg(&buf, MsgInput, payload); err != nil {
		t.Fatalf("WriteMsg: %v", err)
	}

	typ, got, err := ReadMsg(&buf)
	if err != nil {
		t.Fatalf("ReadMsg: %v", err)
	}
	if typ != MsgInput {
		t.Errorf("type = %c, want 'I'", typ)
	}
	if len(got) != MaxPayload {
		t.Errorf("payload length = %d, want %d", len(got), MaxPayload)
	}
}

// failWriter is an io.Writer that always returns an error.
type failWriter struct{}

func (fw *failWriter) Write(p []byte) (int, error) {
	return 0, io.ErrClosedPipe
}

func TestWriteReadMsgRoundTripPipe(t *testing.T) {
	// Test round-trip through an io.Pipe instead of bytes.Buffer.
	pr, pw := io.Pipe()

	done := make(chan struct{})
	var gotTyp byte
	var gotPayload []byte
	var gotErr error

	go func() {
		defer close(done)
		gotTyp, gotPayload, gotErr = ReadMsg(pr)
	}()

	payload := []byte("pipe test data")
	if err := WriteMsg(pw, MsgOutput, payload); err != nil {
		t.Fatalf("WriteMsg to pipe: %v", err)
	}
	pw.Close()

	<-done
	if gotErr != nil {
		t.Fatalf("ReadMsg from pipe: %v", gotErr)
	}
	if gotTyp != MsgOutput {
		t.Errorf("type = %c, want 'O'", gotTyp)
	}
	if !bytes.Equal(gotPayload, payload) {
		t.Errorf("payload mismatch")
	}
}

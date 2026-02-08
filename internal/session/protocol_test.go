package session

import (
	"bytes"
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
		{"empty output", MsgOutput, nil},
		{"large payload", MsgInput, bytes.Repeat([]byte("x"), 4096)},
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

func TestDecodeResizeShort(t *testing.T) {
	rows, cols := DecodeResize([]byte{1, 2})
	if rows != 0 || cols != 0 {
		t.Errorf("short payload should return (0, 0), got (%d, %d)", rows, cols)
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

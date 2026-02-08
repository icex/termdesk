package session

import (
	"encoding/binary"
	"fmt"
	"io"
)

// Message types for the TLV wire protocol between client and server.
const (
	MsgInput  byte = 'I' // client → server: raw terminal input
	MsgOutput byte = 'O' // server → client: raw PTY output
	MsgResize byte = 'R' // client → server: terminal resize (4 bytes)
	MsgDetach byte = 'D' // client → server: clean disconnect
	MsgRedraw byte = 'S' // server → client: full screen state on connect
)

// MaxPayload limits message size to 1MB.
const MaxPayload = 1 << 20

// WriteMsg writes a TLV message: type(1) + length(4 BE) + payload.
func WriteMsg(w io.Writer, typ byte, payload []byte) error {
	var header [5]byte
	header[0] = typ
	binary.BigEndian.PutUint32(header[1:], uint32(len(payload)))
	if _, err := w.Write(header[:]); err != nil {
		return err
	}
	if len(payload) > 0 {
		_, err := w.Write(payload)
		return err
	}
	return nil
}

// ReadMsg reads a TLV message. Returns type and payload.
func ReadMsg(r io.Reader) (byte, []byte, error) {
	var header [5]byte
	if _, err := io.ReadFull(r, header[:]); err != nil {
		return 0, nil, err
	}
	typ := header[0]
	length := binary.BigEndian.Uint32(header[1:])
	if length > MaxPayload {
		return 0, nil, fmt.Errorf("message too large: %d bytes", length)
	}
	if length == 0 {
		return typ, nil, nil
	}
	payload := make([]byte, length)
	if _, err := io.ReadFull(r, payload); err != nil {
		return 0, nil, err
	}
	return typ, payload, nil
}

// EncodeResize encodes terminal rows and cols into a 4-byte payload.
func EncodeResize(rows, cols uint16) []byte {
	buf := make([]byte, 4)
	binary.BigEndian.PutUint16(buf[0:2], rows)
	binary.BigEndian.PutUint16(buf[2:4], cols)
	return buf
}

// DecodeResize decodes a 4-byte payload into terminal rows and cols.
func DecodeResize(data []byte) (rows, cols uint16) {
	if len(data) < 4 {
		return 0, 0
	}
	rows = binary.BigEndian.Uint16(data[0:2])
	cols = binary.BigEndian.Uint16(data[2:4])
	return
}

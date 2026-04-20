package terminal

import "fmt"

// SGR mouse encoding for forwarding mouse events to terminal applications.
// Uses the SGR extended mouse protocol (\x1b[<btn;col;row;M/m) which supports
// coordinates beyond 223 (unlike X10 mode).

// MouseButton constants for SGR encoding.
const (
	MouseLeft       = 0
	MouseMiddle     = 1
	MouseRight      = 2
	MouseRelease    = 3
	MouseWheelUp    = 64
	MouseWheelDown  = 65
	MouseMotionFlag = 32 // OR with button for motion events
)

// encodeMouse encodes a mouse event in SGR format.
// button: mouse button (0=left, 1=middle, 2=right, 64=wheel up, 65=wheel down)
// col, row: 1-indexed terminal coordinates
// release: true for button release, false for press
func encodeMouse(button, col, row int, release bool) []byte {
	suffix := byte('M') // press
	if release {
		suffix = byte('m') // release
	}
	// SGR format: \x1b[<button;col;row;M/m
	s := fmt.Sprintf("\x1b[<%d;%d;%d%c", button, col, row, suffix)
	return []byte(s)
}

// encodeMouseMotion encodes a mouse motion event in SGR format.
func encodeMouseMotion(button, col, row int) []byte {
	return encodeMouse(button|MouseMotionFlag, col, row, false)
}

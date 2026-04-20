package terminal

import uv "github.com/charmbracelet/ultraviolet"

// encodeKey converts a key event into bytes to send to a PTY.
func encodeKey(code rune, mod uv.KeyMod, text string) []byte {
	// Handle Ctrl+key combinations
	if mod&uv.ModCtrl != 0 {
		if code >= 'a' && code <= 'z' {
			return []byte{byte(code - 'a' + 1)}
		}
		if code >= 'A' && code <= 'Z' {
			return []byte{byte(code - 'A' + 1)}
		}
		switch code {
		case '@':
			return []byte{0x00} // Ctrl+@
		case '[':
			return []byte{0x1B} // Ctrl+[ = Escape
		case '\\':
			return []byte{0x1C}
		case ']':
			return []byte{0x1D}
		case '^':
			return []byte{0x1E}
		case '_':
			return []byte{0x1F}
		}
	}

	// Handle Alt+key — send ESC prefix
	if mod&uv.ModAlt != 0 && mod&uv.ModCtrl == 0 {
		if text != "" {
			return append([]byte{0x1B}, []byte(text)...)
		}
		if code > 0 && code < 128 {
			return []byte{0x1B, byte(code)}
		}
	}

	// Special keys
	switch code {
	case uv.KeyEnter:
		return []byte{'\r'}
	case uv.KeyTab:
		if mod&uv.ModShift != 0 {
			return []byte("\x1b[Z") // Shift+Tab (backtab)
		}
		return []byte{'\t'}
	case uv.KeyBackspace:
		return []byte{0x7F}
	case uv.KeyEscape:
		return []byte{0x1B}
	case uv.KeySpace:
		return []byte{' '}
	case uv.KeyUp:
		return arrowKey('A', mod)
	case uv.KeyDown:
		return arrowKey('B', mod)
	case uv.KeyRight:
		return arrowKey('C', mod)
	case uv.KeyLeft:
		return arrowKey('D', mod)
	case uv.KeyHome:
		return []byte("\x1b[H")
	case uv.KeyEnd:
		return []byte("\x1b[F")
	case uv.KeyInsert:
		return []byte("\x1b[2~")
	case uv.KeyDelete:
		return []byte("\x1b[3~")
	case uv.KeyPgUp:
		return []byte("\x1b[5~")
	case uv.KeyPgDown:
		return []byte("\x1b[6~")
	case uv.KeyF1:
		return []byte("\x1bOP")
	case uv.KeyF2:
		return []byte("\x1bOQ")
	case uv.KeyF3:
		return []byte("\x1bOR")
	case uv.KeyF4:
		return []byte("\x1bOS")
	case uv.KeyF5:
		return []byte("\x1b[15~")
	case uv.KeyF6:
		return []byte("\x1b[17~")
	case uv.KeyF7:
		return []byte("\x1b[18~")
	case uv.KeyF8:
		return []byte("\x1b[19~")
	case uv.KeyF9:
		return []byte("\x1b[20~")
	case uv.KeyF10:
		return []byte("\x1b[21~")
	case uv.KeyF11:
		return []byte("\x1b[23~")
	case uv.KeyF12:
		return []byte("\x1b[24~")
	}

	// Printable text
	if text != "" {
		return []byte(text)
	}

	// Single printable rune
	if code >= 32 && code < 127 {
		return []byte{byte(code)}
	}

	// Multi-byte UTF-8 rune
	if code >= 128 {
		return []byte(string(code))
	}

	return nil
}

// arrowKey encodes an arrow key with optional modifiers.
func arrowKey(dir byte, mod uv.KeyMod) []byte {
	m := modParam(mod)
	if m > 1 {
		return []byte{0x1B, '[', '1', ';', byte('0' + m), dir}
	}
	return []byte{0x1B, '[', dir}
}

// modParam returns the xterm modifier parameter for CSI sequences.
func modParam(mod uv.KeyMod) int {
	m := 1
	if mod&uv.ModShift != 0 {
		m += 1
	}
	if mod&uv.ModAlt != 0 {
		m += 2
	}
	if mod&uv.ModCtrl != 0 {
		m += 4
	}
	return m
}

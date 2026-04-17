package terminal

import (
	"bytes"
	"fmt"
	uv "github.com/charmbracelet/ultraviolet"
	"github.com/charmbracelet/x/ansi"
	"github.com/charmbracelet/x/vt"
	"github.com/icex/termdesk/internal/logging"
	"image/color"
	"io"
	"os"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
)

// ScreenCell captures a single cell's content and style for scrollback storage.
type ScreenCell struct {
	Content string
	Fg      color.Color
	Bg      color.Color
	Attrs   uint8
	Width   int8 // character width: 1 for normal, 2 for wide (CJK), 0 for spacer
}

const defaultScrollbackCap = 10000

// Terminal combines a PTY session with a VT emulator.
type Terminal struct {
	pty          Pty
	emu          Emulator
	closed       bool
	cursorHidden bool // tracks whether the child app hid the cursor (DECTCEM)
	mu           sync.Mutex
	closeOnce    sync.Once
	writeCh      chan []byte // buffered write channel for raw PTY input
	emuCh        chan []byte // async emulator write channel
	emuOverflow  []byte      // overflow buffer when emuCh is full (protected by mu)
	done         chan struct{}
	inputDone    chan struct{} // closed when inputForwardLoop exits
	emuWriteDone chan struct{} // closed when emuWriteLoop exits

	// Scrollback ring buffer — lines that scrolled off the top of the screen.
	// Uses circular indexing for O(1) append without copying.
	scrollRing  [][]ScreenCell // fixed-capacity ring buffer
	scrollHead  int            // index of next write slot
	scrollLen   int            // number of stored lines (≤ scrollCap)
	scrollCap   int            // max scrollback lines
	scrollWidth int            // width at which scrollback was captured

	mouseMode int    // active mouse mode number (0 = none)
	altScreen bool   // tracks whether the child app switched to alt screen
	title     string // last OSC 0/2 window title from child app
	// Sync output (mode 2026) — when active, the child app is mid-frame.
	// Rendering is suppressed until the sync block ends so we don't render
	// partially-cleared screens and flicker while the app redraws.
	syncOutput     bool      // true while ESC[?2026h is active
	syncOutputTime time.Time // when sync started; 100ms timeout prevents freeze

	// App state persistence — used by apps that support SIGUSR1/OSC 667 protocol.
	appState   string // last received app state (base64-encoded JSON)
	osc667Buf  []byte // buffer for incomplete OSC 667 sequences (emuWriteLoop only)
	dirty      bool
	dirtyGen   uint64    // incremented on each write batch; renderer compares to detect changes
	dirtyUntil time.Time // force dirty=true for this duration after resize/alt-screen
	onOutput   func()    // called after emuWriteLoop processes data (for render notification)

	// Cell pixel dimensions — for CSI 14t/16t/18t query responses and PTY pixel size.
	cellPixelW int // cell width in pixels
	cellPixelH int // cell height in pixels

	// Kitty graphics passthrough — intercepts APC sequences from child apps.
	onKittyGraphics func(cmd *KittyCommand, rawData []byte) *PlacementResult
	kittyAPCBuf     []byte // buffer for incomplete APC sequences spanning buffers
	onScreenClear   func() // called when child app clears the screen (e.g. `clear` command)
	onBell          func() // called when child app sends BEL (\x07)
	bellPending     bool   // set by emu Bell callback; fired after emu.Write returns

	// Image passthrough (sixel/iTerm2) — intercepts DCS/OSC sequences from child apps.
	onImageGraphics func(rawData []byte, format ImageFormat, estimatedCellRows int) int
	sixelDCSBuf     []byte // buffer for incomplete sixel DCS sequences
	iterm2OSCBuf    []byte // buffer for incomplete iTerm2 OSC sequences
}

type emuScreen struct {
	w     int
	h     int
	cells [][]ScreenCell
}

func newEmuScreen(w, h int) *emuScreen {
	cells := make([][]ScreenCell, h)
	for y := 0; y < h; y++ {
		cells[y] = make([]ScreenCell, w)
	}
	return &emuScreen{w: w, h: h, cells: cells}
}

func (s *emuScreen) Bounds() uv.Rectangle {
	return uv.Rect(0, 0, s.w, s.h)
}

func (s *emuScreen) CellAt(x, y int) *uv.Cell {
	if y < 0 || y >= s.h || x < 0 || x >= s.w {
		return nil
	}
	cell := s.cells[y][x]
	return &uv.Cell{
		Content: cell.Content,
		Width:   int(cell.Width),
		Style:   uv.Style{Fg: cell.Fg, Bg: cell.Bg, Attrs: cell.Attrs},
	}
}

func (s *emuScreen) SetCell(x, y int, c *uv.Cell) {
	if y < 0 || y >= s.h || x < 0 || x >= s.w {
		return
	}
	if c == nil {
		s.cells[y][x] = ScreenCell{Content: " ", Width: 1}
		return
	}
	w := int8(c.Width)
	if w < 0 {
		w = 1
	}
	fg := c.Style.Fg
	bg := c.Style.Bg
	// The VT emulator initializes unwritten cells with color.Black (Gray16{0})
	// as Fg/Bg. These are non-nil but represent "default terminal color", not
	// an explicit color set by the app. Normalize to nil so the renderer can
	// substitute the theme's ContentBg/DefaultFg colors instead.
	// Explicitly-set colors (e.g. \e[40m) use ansi.BasicColor which does NOT
	// match color.Black, so they are preserved correctly.
	if fg == color.Black {
		fg = nil
	}
	if bg == color.Black {
		bg = nil
	}
	s.cells[y][x] = ScreenCell{
		Content: c.Content,
		Fg:      fg,
		Bg:      bg,
		Attrs:   c.Style.Attrs,
		Width:   w,
	}
}

func (s *emuScreen) WidthMethod() uv.WidthMethod {
	return ansi.GraphemeWidth
}

// New creates a terminal running the given command.
// cellPixelW/cellPixelH are the cell pixel dimensions for TIOCGWINSZ and CSI query responses.
// extraEnv contains additional environment variables (e.g. "KEY=value").
func New(command string, args []string, cols, rows, cellPixelW, cellPixelH int, workDir string, extraEnv ...string) (*Terminal, error) {
	p, err := NewPtySession(command, args, uint16(rows), uint16(cols), uint16(cellPixelW), uint16(cellPixelH), workDir, extraEnv...)
	if err != nil {
		return nil, err
	}

	emu := vt.NewSafeEmulator(cols, rows)
	t := newTerminal(p, emu, cols, rows)
	t.cellPixelW = cellPixelW
	t.cellPixelH = cellPixelH
	return t, nil
}

// NewWithDeps creates a terminal with provided PTY and emulator (primarily for tests).
func NewWithDeps(pty Pty, emu Emulator, cols, rows int) *Terminal {
	return newTerminal(pty, emu, cols, rows)
}

func newTerminal(pty Pty, emu Emulator, cols, rows int) *Terminal {
	t := &Terminal{
		pty:          pty,
		emu:          emu,
		writeCh:      make(chan []byte, 256),
		emuCh:        make(chan []byte, 512),
		done:         make(chan struct{}),
		inputDone:    make(chan struct{}),
		emuWriteDone: make(chan struct{}),
		scrollCap:    defaultScrollbackCap,
		scrollRing:   make([][]ScreenCell, defaultScrollbackCap),
		scrollWidth:  cols,
	}

	// Track cursor visibility, mouse mode, and alt screen via emulator callbacks.
	if emu != nil {
		emu.SetCallbacks(vt.Callbacks{
			Bell: func() {
				// Only set a flag here — do NOT call onBell directly.
				// This callback runs inside emu.Write() while the SafeEmulator
				// lock is held. Calling p.Send() (via onBell) would block if
				// BT's message channel is full, and BT's View() needs the emu
				// lock to render → deadlock. The flag is checked in emuWriteLoop
				// after emu.Write() returns and the lock is released.
				t.mu.Lock()
				t.bellPending = true
				t.mu.Unlock()
			},
			CursorVisibility: func(visible bool) {
				t.mu.Lock()
				t.cursorHidden = !visible
				t.dirty = true
				t.mu.Unlock()
			},
			AltScreen: func(on bool) {
				t.mu.Lock()
				t.altScreen = on
				t.dirty = true
				// Grace period on alt-screen transitions: the app (nvim, etc.)
				// needs time to repaint the new screen. Without this, the first
				// few renders may cache a partially-drawn alt screen (showing
				// theme background instead of the app's colors).
				t.dirtyUntil = time.Now().Add(500 * time.Millisecond)
				t.mu.Unlock()
			},
			Title: func(title string) {
				t.mu.Lock()
				t.title = title
				t.dirty = true
				t.mu.Unlock()
			},
			EnableMode: func(mode ansi.Mode) {
				switch mode {
				case ansi.X10MouseMode, ansi.NormalMouseMode,
					ansi.HighlightMouseMode, ansi.ButtonEventMouseMode,
					ansi.AnyEventMouseMode:
					t.mu.Lock()
					t.mouseMode = mode.Mode()
					t.mu.Unlock()
				case ansi.ModeSynchronizedOutput:
					t.mu.Lock()
					t.syncOutput = true
					t.syncOutputTime = time.Now()
					t.mu.Unlock()
				}
			},
			DisableMode: func(mode ansi.Mode) {
				switch mode {
				case ansi.X10MouseMode, ansi.NormalMouseMode,
					ansi.HighlightMouseMode, ansi.ButtonEventMouseMode,
					ansi.AnyEventMouseMode:
					t.mu.Lock()
					if t.mouseMode == mode.Mode() {
						t.mouseMode = 0
					}
					t.mu.Unlock()
				case ansi.ModeSynchronizedOutput:
					t.mu.Lock()
					t.syncOutput = false
					t.mu.Unlock()
				}
			},
		})
	}

	// Spawn writer goroutine — drains writeCh and writes to PTY.
	// This keeps WriteInput non-blocking.
	go t.writeLoop()

	// Spawn async emulator writer — decouples PTY reads from emulator processing.
	// This eliminates SafeEmulator write-lock contention with CellAt reads during rendering.
	go t.emuWriteLoop()

	// Spawn input forwarder — reads encoded input from the emulator's pipe
	// (filled by SendKey/SendMouse) and writes to PTY.
	go t.inputForwardLoop()

	return t
}

// NewShell creates a terminal running the user's default shell.
func NewShell(cols, rows, cellPixelW, cellPixelH int, workDir string, extraEnv ...string) (*Terminal, error) {
	shell := os.Getenv("SHELL")
	if shell == "" {
		shell = "/bin/sh"
	}
	return New(shell, nil, cols, rows, cellPixelW, cellPixelH, workDir, extraEnv...)
}

// SetCellPixelSize updates the cell pixel dimensions used for CSI query responses.
func (t *Terminal) SetCellPixelSize(w, h int) {
	t.mu.Lock()
	t.cellPixelW = w
	t.cellPixelH = h
	t.mu.Unlock()
}

// SetDefaultColors sets the emulator's default foreground and background colors.
// This ensures that OSC 10/11 queries report the actual visual colors (matching the
// theme's ContentBg/DefaultFg) and that cells using "default background" (SGR 49)
// render correctly. Pass nil to keep the emulator's built-in default for that color.
func (t *Terminal) SetDefaultColors(fg, bg color.Color) {
	if fg != nil {
		t.emu.SetDefaultForegroundColor(fg)
	}
	if bg != nil {
		t.emu.SetDefaultBackgroundColor(bg)
	}
}

// BackgroundColor returns the emulator's current background color.
// If the child app set the bg via OSC 11, that color is returned.
// Otherwise returns the default bg set by SetDefaultColors.
func (t *Terminal) BackgroundColor() color.Color {
	return t.emu.BackgroundColor()
}

// ReadPtyLoop reads from the PTY and writes to the emulator synchronously.
// It returns when the PTY is closed or an error occurs.
// Call this from a goroutine. Used primarily in tests.
func (t *Terminal) ReadPtyLoop() error {
	buf := make([]byte, 4096)
	for {
		t.mu.Lock()
		closed := t.closed
		t.mu.Unlock()
		if closed {
			return nil
		}

		n, err := t.pty.Read(buf)
		if n > 0 {
			t.emu.Write(buf[:n])
		}
		if err != nil {
			if err == io.EOF {
				return nil
			}
			return err
		}
	}
}

// ReadOnce reads one chunk from the PTY and feeds it to the emulator asynchronously.
// The caller provides a reusable buffer to avoid per-call allocation.
// Returns (bytesRead, error). Use this for event-driven reading.
//
// When emuCh is full, data is appended to an overflow buffer instead of being
// dropped. The overflow is flushed as a single coalesced chunk on the next
// successful channel send — this prevents terminal state corruption from
// partial escape sequences while keeping the PTY reader non-blocking.
func (t *Terminal) ReadOnce(buf []byte) (int, error) {
	t.mu.Lock()
	if t.closed {
		t.mu.Unlock()
		return 0, io.EOF
	}
	t.mu.Unlock()

	n, err := t.pty.Read(buf)
	if n > 0 {
		data := make([]byte, n)
		copy(data, buf[:n])
		t.mu.Lock()
		if !t.closed {
			// Prepend any overflow data from previous reads.
			if len(t.emuOverflow) > 0 {
				data = append(t.emuOverflow, data...)
				t.emuOverflow = nil
			}
			select {
			case t.emuCh <- data:
			default:
				// Channel full — stash in overflow buffer instead of dropping.
				// Capped at 2MB to bound memory; beyond that we must drop to
				// prevent unbounded growth if the emulator is truly stuck.
				const maxOverflow = 2 << 20
				if len(t.emuOverflow)+len(data) <= maxOverflow {
					t.emuOverflow = append(t.emuOverflow, data...)
				}
				// else: truly stuck, drop to keep PTY reads flowing
			}
		}
		t.mu.Unlock()
	}
	return n, err
}

// OSC 667 markers for app state protocol.
var (
	osc667Prefix  = []byte("\x1b]667;state-response;")
	osc667TermST  = []byte("\x1b\\") // ST (String Terminator)
	osc667TermBEL = byte('\x07')     // BEL terminator
)

// stripOSC667 scans data for OSC 667 state-response sequences, extracts the
// payload into t.appState, and returns data with those sequences removed.
// Buffers incomplete sequences across calls (large payloads may span multiple
// PTY read chunks). Only called from emuWriteLoop (single goroutine).
// extractOSCTitles pre-processes raw PTY data to extract OSC 0/2 title
// sequences ourselves, working around a bug in the charmbracelet/x/ansi parser
// where byte 0x9C inside an OSC payload is treated as the C1 String Terminator
// instead of a UTF-8 continuation byte. This breaks titles containing characters
// like ✳ (U+2733, encoded as \xe2\x9c\xb3) — the \x9c byte terminates the
// OSC prematurely, truncating the title to just \xe2.
//
// We scan for OSC 0/2 sequences terminated by BEL (\x07) or 7-bit ST (\x1b\\),
// extract the title as valid UTF-8, set it on the Terminal, and strip the
// sequence entirely from the emulator's input.
func (t *Terminal) extractOSCTitles(data []byte) []byte {
	// Quick check: skip if no ESC ] followed by '0' or '2' present.
	// This avoids byte-by-byte processing for data that only has
	// non-title OSC sequences (like OSC 1337 for iTerm2 images).
	if !bytes.Contains(data, []byte{0x1b, ']', '0', ';'}) &&
		!bytes.Contains(data, []byte{0x1b, ']', '2', ';'}) {
		return data
	}

	var result []byte
	i := 0
	for i < len(data) {
		// Look for OSC start: ESC ]
		if i+1 < len(data) && data[i] == 0x1b && data[i+1] == ']' {
			// Find the OSC command number and semicolon
			j := i + 2
			cmd := 0
			hasCmd := false
			for j < len(data) && data[j] >= '0' && data[j] <= '9' {
				cmd = cmd*10 + int(data[j]-'0')
				hasCmd = true
				j++
			}
			// Only intercept OSC 0 (title+icon) and OSC 2 (title)
			if hasCmd && (cmd == 0 || cmd == 2) && j < len(data) && data[j] == ';' {
				j++ // skip ';'
				// Find the terminator: BEL (\x07) or 7-bit ST (\x1b\\)
				titleStart := j
				terminated := false
				for j < len(data) {
					if data[j] == 0x07 {
						title := string(data[titleStart:j])
						t.mu.Lock()
						t.title = title
						t.dirty = true
						t.mu.Unlock()
						j++ // skip BEL
						terminated = true
						break
					}
					if data[j] == 0x1b && j+1 < len(data) && data[j+1] == '\\' {
						title := string(data[titleStart:j])
						t.mu.Lock()
						t.title = title
						t.dirty = true
						t.mu.Unlock()
						j += 2 // skip ESC backslash
						terminated = true
						break
					}
					j++
				}
				if terminated {
					i = j
					continue
				}
				// Not terminated in this chunk — pass through as-is
			}
		}
		result = append(result, data[i])
		i++
	}
	return result
}

func (t *Terminal) stripOSC667(data []byte) []byte {
	// Prepend any buffered data from a previous incomplete sequence.
	if len(t.osc667Buf) > 0 {
		data = append(t.osc667Buf, data...)
		t.osc667Buf = nil
	}

	idx := bytes.Index(data, osc667Prefix)
	if idx < 0 {
		return data
	}

	// Find the end of the OSC sequence (BEL or ST)
	payloadStart := idx + len(osc667Prefix)
	endIdx := -1
	for i := payloadStart; i < len(data); i++ {
		if data[i] == osc667TermBEL {
			endIdx = i
			break
		}
		if i+1 < len(data) && data[i] == '\x1b' && data[i+1] == '\\' {
			endIdx = i
			break
		}
	}
	if endIdx < 0 {
		// Incomplete sequence — buffer from prefix onward, return data before it.
		// Safety: discard if buffer grows too large (64KB).
		if len(data)-idx < 64*1024 {
			t.osc667Buf = make([]byte, len(data)-idx)
			copy(t.osc667Buf, data[idx:])
		}
		if idx > 0 {
			return data[:idx]
		}
		return nil
	}

	// Extract payload
	payload := string(data[payloadStart:endIdx])
	t.mu.Lock()
	t.appState = payload
	t.mu.Unlock()

	// Determine how many bytes to strip (including terminator)
	stripEnd := endIdx + 1
	if endIdx+1 < len(data) && data[endIdx] == '\x1b' {
		stripEnd = endIdx + 2 // ST is 2 bytes
	}

	// Build result without the OSC 667 sequence
	result := make([]byte, 0, len(data)-(stripEnd-idx))
	result = append(result, data[:idx]...)
	result = append(result, data[stripEnd:]...)

	// Recursively strip in case there are multiple sequences
	return t.stripOSC667(result)
}

// interceptCSIWindowOps scans PTY output for CSI window operation queries
// (XTWINOPS) and responds with appropriate pixel/cell dimensions.
// Called from safeEmuWrite before data reaches the emulator.
// Only called from emuWriteLoop (single goroutine).
func (t *Terminal) interceptCSIWindowOps(data []byte) []byte {
	t.mu.Lock()
	cpw := t.cellPixelW
	cph := t.cellPixelH
	t.mu.Unlock()
	if cpw == 0 || cph == 0 {
		return data // no pixel info available
	}

	// Scan for CSI Ps t patterns: \x1b [ <digits> t
	i := 0
	for i < len(data)-3 { // minimum: \x1b [ N t = 4 bytes
		if data[i] != '\x1b' || data[i+1] != '[' {
			i++
			continue
		}
		// Parse digits after CSI
		j := i + 2
		for j < len(data) && data[j] >= '0' && data[j] <= '9' {
			j++
		}
		if j >= len(data) || j == i+2 || data[j] != 't' {
			i++
			continue
		}
		// Parse the parameter number
		param, err := strconv.Atoi(string(data[i+2 : j]))
		if err != nil {
			i++
			continue
		}
		seqEnd := j + 1 // past the 't'

		var response string
		switch param {
		case 14: // Report window size in pixels → CSI 4 ; height ; width t
			w := t.emu.Width()
			h := t.emu.Height()
			response = fmt.Sprintf("\x1b[4;%d;%dt", h*cph, w*cpw)
		case 16: // Report cell size in pixels → CSI 6 ; height ; width t
			response = fmt.Sprintf("\x1b[6;%d;%dt", cph, cpw)
		case 18: // Report text area size in chars → CSI 8 ; rows ; cols t
			response = fmt.Sprintf("\x1b[8;%d;%dt", t.emu.Height(), t.emu.Width())
		}

		if response != "" {
			// Send response back to child via PTY write channel
			resp := []byte(response)
			select {
			case t.writeCh <- resp:
			default:
			}
			// Remove the query from data
			data = append(data[:i], data[seqEnd:]...)
			continue // don't advance i — next seq might be at same position
		}
		i++
	}
	return data
}

// kittyAPCSegment describes a segment of data followed by an APC command.
// The emulator should process DataBefore, then the callback fires for Cmd.
type kittyAPCSegment struct {
	DataBefore []byte        // terminal data to feed emulator before this APC
	Cmd        *KittyCommand // parsed APC command (nil if parse failed)
	RawData    []byte        // raw APC payload for forwarding
}

// extractKittyAPCs extracts Kitty graphics APC sequences (ESC_G...ST) from
// data. Returns segments (data-before + APC command) and trailing non-APC data.
// Handles sequences that span buffer boundaries via kittyAPCBuf.
// The caller is responsible for feeding DataBefore to the emulator BEFORE
// dispatching each Cmd, ensuring cursor position is accurate.
func (t *Terminal) extractKittyAPCs(data []byte) (segments []kittyAPCSegment, trailing []byte) {
	// If we have a partial APC buffered from a previous call, prepend it.
	if len(t.kittyAPCBuf) > 0 {
		combined := make([]byte, len(t.kittyAPCBuf)+len(data))
		copy(combined, t.kittyAPCBuf)
		copy(combined[len(t.kittyAPCBuf):], data)
		data = combined
		t.kittyAPCBuf = t.kittyAPCBuf[:0]
	}

	// Fast path: no ESC in data means no APC sequences.
	if !bytes.ContainsRune(data, '\x1b') {
		return nil, data
	}

	i := 0
	lastCopy := 0

	for i < len(data) {
		if data[i] != '\x1b' || i+2 >= len(data) || data[i+1] != '_' || data[i+2] != 'G' {
			i++
			continue
		}

		// Found ESC _ G — search for terminator
		apcStart := i + 3
		found := false
		for j := apcStart; j < len(data); j++ {
			terminated := false
			end := j
			if data[j] == '\x1b' && j+1 < len(data) && data[j+1] == '\\' {
				terminated = true
				end = j + 2
			} else if data[j] == '\x07' {
				terminated = true
				end = j + 1
			}
			if terminated {
				payload := data[apcStart:j]
				cmd, _ := ParseKittyCommand(payload)
				seg := kittyAPCSegment{
					DataBefore: data[lastCopy:i],
					Cmd:        cmd,
					RawData:    payload,
				}
				segments = append(segments, seg)
				i = end
				lastCopy = end
				found = true
				break
			}
		}
		if !found {
			// Incomplete APC — buffer from ESC_G onwards
			t.kittyAPCBuf = append(t.kittyAPCBuf[:0], data[i:]...)
			trailing = data[lastCopy:i]
			return
		}
	}

	trailing = data[lastCopy:]
	return
}

// SetOnKittyGraphics sets a callback that fires when a Kitty graphics APC
// sequence is detected in the PTY output. The callback receives the parsed
// command and the raw APC payload (between ESC_G and ST). Returns a
// PlacementResult so the caller can inject cursor movement into the emulator
// (matching what a real terminal would do with C=0).
func (t *Terminal) SetOnKittyGraphics(fn func(cmd *KittyCommand, rawData []byte) *PlacementResult) {
	t.mu.Lock()
	t.onKittyGraphics = fn
	t.mu.Unlock()
}

// SetOnScreenClear sets a callback that fires when the child app clears the
// screen (CSI 2J or CSI 3J). Used to clear Kitty graphics placements.
func (t *Terminal) SetOnScreenClear(fn func()) {
	t.mu.Lock()
	t.onScreenClear = fn
	t.mu.Unlock()
}

// SetOnImageGraphics sets a callback that fires when a sixel or iTerm2 inline
// image sequence is detected in the PTY output. The callback receives the raw
// data, image format, and estimated cell rows. Returns the actual cell rows
// used for cursor injection.
func (t *Terminal) SetOnImageGraphics(fn func(rawData []byte, format ImageFormat, estimatedCellRows int) int) {
	t.mu.Lock()
	t.onImageGraphics = fn
	t.mu.Unlock()
}

// IsAltScreen returns whether the child app has switched to the alternate screen.
func (t *Terminal) IsAltScreen() bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.altScreen
}

// SetOnOutput sets a callback that fires after the emulator processes new data.
// Used to notify the app for re-rendering AFTER the emulator is up-to-date,
// rather than when PTY data was read (which may be before processing).
func (t *Terminal) SetOnOutput(fn func()) {
	t.mu.Lock()
	t.onOutput = fn
	t.mu.Unlock()
}

// SetOnBell sets a callback that fires when the child app rings the bell (BEL).
// Used to show bell indicators on unfocused/minimized windows.
func (t *Terminal) SetOnBell(fn func()) {
	t.mu.Lock()
	t.onBell = fn
	t.mu.Unlock()
}

// emuWriteLoop drains the async emulator channel and writes to the emulator.
// It batches pending data and processes line-by-line to capture scrollback,
// similar to how tmux processes all available PTY data per event loop iteration.
// After processing, it calls onOutput to notify the app for rendering.
func (t *Terminal) emuWriteLoop() {
	defer close(t.emuWriteDone)
	for data := range t.emuCh {
		// Batch: drain any additional pending chunks to reduce overhead.
		for {
			select {
			case more := <-t.emuCh:
				data = append(data, more...)
			default:
				goto process
			}
		}
	process:
		t.safeEmuWrite(data)
		// Mark dirty AFTER emulator write completes. Previously this was set
		// BEFORE emu.Write() in feedEmuData, creating a race: BT's View() could
		// call ConsumeDirty() mid-write, get dirty=true, render stale content,
		// and consume the flag — so the real update never triggered a re-render.
		// Symptom: after burst output (e.g. ps -ef), the shell prompt wasn't
		// displayed until the user pressed Enter again.
		t.mu.Lock()
		t.dirty = true
		t.dirtyGen++
		// Large bursts: force re-snapshot for 200ms so intermediate View()
		// calls don't cache a partially-drawn screen. ConsumeDirty returns
		// true without consuming during this window, so every frame
		// re-snapshots the emulator.
		if len(data) > 4096 {
			t.dirtyUntil = time.Now().Add(200 * time.Millisecond)
		}
		// Check if the child app is mid-frame (sync output mode 2026 active).
		// If so, suppress render notification — the frame isn't complete yet.
		// 100ms timeout prevents permanent freeze if sync never ends.
		syncing := t.syncOutput && time.Since(t.syncOutputTime) < 100*time.Millisecond
		t.mu.Unlock()
		// Fire deferred bell callback AFTER emu.Write() returns — the emu lock
		// is no longer held, so p.Send() in onBell can safely block without
		// causing a deadlock with BT's View() → emu.CellAt().
		t.mu.Lock()
		bell := t.bellPending
		bellFn := t.onBell
		t.bellPending = false
		t.mu.Unlock()
		if bell && bellFn != nil {
			bellFn()
		}
		// Notify after emulator is updated — View() will snapshot fresh content.
		// Skip while sync output is active: the app is mid-frame (between
		// ESC[?2026h and ESC[?2026l). Rendering now would show a
		// partially-cleared screen and cause flickering on rapid redraws.
		if !syncing {
			t.mu.Lock()
			fn := t.onOutput
			t.mu.Unlock()
			if fn != nil {
				fn()
			}
		}
	}
}

// safeEmuWrite processes a batch of emulator data with panic recovery.
// It processes line-by-line (split at \n), checking row 0 after each line
// to capture scrolled-off lines. Each \n causes at most 1 scroll, so
// comparing row 0 is sufficient — no full-screen snapshot needed.
//
// Kitty APC sequences are extracted and interleaved: data before each APC
// is fed to the emulator first, ensuring CursorPosition() is accurate when
// the APC callback fires.
func (t *Terminal) safeEmuWrite(data []byte) {
	defer func() {
		if r := recover(); r != nil {
			logging.Error("safeEmuWrite panic (terminal may be corrupted): %v", r)
		}
	}()

	data = t.extractOSCTitles(data)
	data = t.stripOSC667(data)
	if len(data) == 0 {
		return
	}
	data = t.interceptCSIWindowOps(data)
	if len(data) == 0 {
		return
	}

	// Extract Kitty APC sequences. If there are APCs, we interleave: feed
	// data before each APC to the emulator, then fire the APC callback.
	// This ensures CursorPosition() reflects data processed BEFORE the APC,
	// fixing image positioning accuracy.
	t.mu.Lock()
	apcFn := t.onKittyGraphics
	t.mu.Unlock()

	if apcFn != nil {
		segments, trailing := t.extractKittyAPCs(data)
		if len(segments) > 0 {
			for _, seg := range segments {
				if len(seg.DataBefore) > 0 {
					t.feedEmuData(seg.DataBefore)
				}
				if seg.Cmd != nil {
					result := apcFn(seg.Cmd, seg.RawData)
					// When a Kitty image is placed with C=0 (default), the real
					// terminal advances the cursor past the image. Since we strip
					// the APC, the emulator cursor stays put. Inject newlines to
					// match real terminal behavior — otherwise the next prompt
					// appears under the image instead of below it.
					if result != nil && result.Rows > 0 && result.CursorMove != 1 {
						posBefore := t.emu.CursorPosition()
						t.mu.Lock()
						scrollBefore := t.scrollLen
						t.mu.Unlock()

						var nl []byte
						for i := 0; i < result.Rows; i++ {
							nl = append(nl, '\n')
						}
						// Reset cursor X to column 0, matching real terminal behavior
						// after image placement. Without this, the prompt appears
						// centered under the image instead of at the left margin.
						nl = append(nl, '\r')
						t.feedEmuData(nl)

						posAfter := t.emu.CursorPosition()
						t.mu.Lock()
						scrollAfter := t.scrollLen
						t.mu.Unlock()
						kittyPassDbg("cursor injection: %d newlines, cursor (%d,%d)->(%d,%d), scrollback %d->%d",
							result.Rows, posBefore.X, posBefore.Y, posAfter.X, posAfter.Y, scrollBefore, scrollAfter)
					}
				}
			}
			if len(trailing) > 0 {
				t.feedEmuData(trailing)
			}
			return
		}
		// No APCs found — trailing is the full data, fall through.
		data = trailing
	}

	// Extract image sequences (sixel DCS or iTerm2 OSC). Same interleaving
	// pattern as Kitty APCs: feed data before each image to the emulator first,
	// then fire the image callback, then advance the emulator cursor.
	//
	// Cursor advancement uses newlines (\n) — they scroll at the bottom
	// margin like a real terminal would after sixel rendering. CUD (CSI B)
	// stops at the bottom margin WITHOUT scrolling, which prevents text
	// from moving up to make room for the image.
	t.mu.Lock()
	imgFn := t.onImageGraphics
	t.mu.Unlock()

	if imgFn != nil {
		// Try sixel DCS extraction
		sixelSegs, sixelTrailing := t.extractSixelDCS(data)
		if len(sixelSegs) > 0 {
			for _, seg := range sixelSegs {
				if len(seg.DataBefore) > 0 {
					t.feedEmuData(seg.DataBefore)
				}
				cpH := t.cellPixelH
				if cpH <= 0 {
					cpH = 16 // reasonable default cell height in pixels
				}
				cellRows := (seg.PixelRows + cpH - 1) / cpH

				imagePassDbg("safeEmuWrite: sixel seg pixelRows=%d cpH=%d cellRows=%d rawLen=%d",
					seg.PixelRows, cpH, cellRows, len(seg.RawDCS))
				imgFn(seg.RawDCS, ImageFormatSixel, 0)
				if cellRows > 0 {
					advance := make([]byte, cellRows+1)
					for i := 0; i < cellRows; i++ {
						advance[i] = '\n'
					}
					advance[cellRows] = '\r'
					t.feedEmuData(advance)
				}
			}
			if len(sixelTrailing) > 0 {
				t.feedEmuData(sixelTrailing)
			}
			return
		}

		// Try iTerm2 OSC extraction (File=, MultipartFile=, FilePart=, FileEnd)
		iterm2Segs, iterm2Trailing := t.extractIterm2OSC(sixelTrailing)
		if len(iterm2Segs) > 0 {
			for _, seg := range iterm2Segs {
				if len(seg.DataBefore) > 0 {
					t.feedEmuData(seg.DataBefore)
				}
				if seg.IsMultipart {
					// FilePart/FileEnd: forward raw without cursor injection.
					imagePassDbg("safeEmuWrite: iterm2 multipart chunk len=%d", len(seg.RawOSC))
					imgFn(seg.RawOSC, ImageFormatIterm2, 0)
				} else {
					// File= or MultipartFile=: constrain width AND height
					// to fit within the guest terminal so the host terminal
					// doesn't scroll when rendering the image (which would
					// corrupt the entire viewport in a composited WM).
					// Use placement tracking (cellRows > 0) so that
					// RefreshAllImages positions the image inside the
					// correct window via CUP. Without this, the image
					// renders wherever BT's host cursor happens to be.
					cols := t.emu.Width()
					rows := t.emu.Height()
					constrained := constrainIterm2Width(seg.RawOSC, cols, t.cellPixelW)
					cellRows := estimateIterm2CellRows(seg.RawOSC, cols, t.cellPixelW, t.cellPixelH)
					if cellRows <= 0 {
						cellRows = 1
					}

					// Clamp cellRows to available space from cursor to
					// bottom of terminal. Without this, large images
					// (e.g. yazi previews) cause excessive scrolling in
					// both the guest emulator and the host terminal.
					cy := t.emu.CursorPosition().Y
					availRows := rows - cy
					if availRows < 1 {
						availRows = 1
					}
					heightClamped := false
					if cellRows > availRows {
						imagePassDbg("safeEmuWrite: iterm2 clamping cellRows %d → %d (curY=%d rows=%d)",
							cellRows, availRows, cy, rows)
						cellRows = availRows
						heightClamped = true
					}

					// Add explicit height param only when the image was
					// clamped — avoids scaling small images UP to fill
					// the available space (which causes distortion).
					if heightClamped {
						constrained = constrainIterm2Height(constrained, cellRows)
					}

					imagePassDbg("safeEmuWrite: iterm2 image len=%d cols=%d cellRows=%d curY=%d",
						len(seg.RawOSC), cols, cellRows, cy)
					imgFn(constrained, ImageFormatIterm2, cellRows)
					if cellRows > 0 {
						advance := make([]byte, cellRows+1)
						for i := 0; i < cellRows; i++ {
							advance[i] = '\n'
						}
						advance[cellRows] = '\r'
						t.feedEmuData(advance)
					}
				}
			}
			if len(iterm2Trailing) > 0 {
				t.feedEmuData(iterm2Trailing)
			}
			return
		}

		data = iterm2Trailing
	} else {
		imagePassDbg("safeEmuWrite: imgFn is nil, image extraction skipped (dataLen=%d)", len(data))
	}

	if len(data) == 0 {
		return
	}

	t.feedEmuData(data)
}

// feedEmuData writes data to the emulator with screen-clear detection and
// scrollback capture. Extracted from safeEmuWrite so it can be called for
// each segment between APC sequences.
func (t *Terminal) feedEmuData(data []byte) {
	// Detect screen clear sequences (CSI 2J or CSI 3J) and notify for
	// Kitty graphics cleanup. The `clear` command and Ctrl+L send these.
	if bytes.Contains(data, []byte("\x1b[2J")) || bytes.Contains(data, []byte("\x1b[3J")) {
		t.mu.Lock()
		fn := t.onScreenClear
		t.mu.Unlock()
		if fn != nil {
			fn()
		}
	}

	t.mu.Lock()
	alt := t.altScreen
	t.mu.Unlock()

	// Skip scrollback capture during alt screen (nvim, htop, etc.)
	if alt {
		t.emu.Write(data)
		return
	}

	h := t.emu.Height()
	if h <= 0 {
		t.emu.Write(data)
		return
	}

	// Process line-by-line: each \n can cause at most 1 scroll.
	// Compare row 0 before/after each line to detect it.
	// For data without \n (typing), this is a single fast iteration.
	//
	// Re-check altScreen after each line: the AltScreen callback fires
	// DURING emu.Write() inside writeLineAndCapture. If the data batch
	// contains an alt-screen-enter sequence (\x1b[?1049h), the flag
	// changes mid-batch. Without this re-check, subsequent lines would
	// be processed with scrollback capture on the alt screen, corrupting
	// the scrollback buffer and causing stale history to bleed into the
	// rendered view.
	start := 0
	for i, b := range data {
		if b == '\n' {
			t.writeLineAndCapture(data[start : i+1])
			start = i + 1
			// Re-check: if we just entered alt screen mid-batch,
			// write remaining data directly without scrollback capture.
			t.mu.Lock()
			nowAlt := t.altScreen
			t.mu.Unlock()
			if nowAlt {
				if start < len(data) {
					t.emu.Write(data[start:])
				}
				return
			}
		}
	}
	if start < len(data) {
		t.writeLineAndCapture(data[start:])
	}
}

// writeLineAndCapture writes one line of data to the emulator, detecting
// scrolls via two methods: row 0 content comparison AND cursor position.
// The cursor-based check catches blank→blank scrolls (e.g., blank rows
// left by Kitty graphics cursor injection where the emulator has no image
// data — all blank rows look identical to rowEqual).
func (t *Terminal) writeLineAndCapture(data []byte) {
	row0Before := t.snapshotRow(0)
	h := t.emu.Height()
	posBefore := t.emu.CursorPosition()

	t.emu.Write(data)

	row0After := t.snapshotRow(0)
	scrolled := !rowEqual(row0Before, row0After)

	// Cursor-based scroll detection: if cursor was at the bottom row and
	// data ended with a newline, a scroll happened even if row 0 content
	// didn't change (blank→blank). feedEmuData splits at '\n', so each
	// call here has at most one '\n' at the end.
	if !scrolled && h > 0 && posBefore.Y >= h-1 &&
		len(data) > 0 && data[len(data)-1] == '\n' {
		scrolled = true
	}

	if scrolled {
		// Read emu width OUTSIDE t.mu to prevent ABBA deadlock: VT emulator
		// callbacks (Bell, CursorVisibility, etc.) acquire t.mu while the emu
		// lock is held. If we held t.mu and then called t.emu.Width() (which
		// acquires the emu lock), a concurrent emu.Write() firing a callback
		// would deadlock — e.g. during RestoreBuffer from a different goroutine.
		emuW := t.emu.Width()
		t.mu.Lock()
		if t.scrollWidth == 0 {
			t.scrollWidth = emuW
		}
		t.scrollRing[t.scrollHead] = row0Before
		t.scrollHead = (t.scrollHead + 1) % t.scrollCap
		if t.scrollLen < t.scrollCap {
			t.scrollLen++
		}
		t.mu.Unlock()
	}
}

// snapshotRow captures a single row from the emulator as ScreenCells.
// Always returns a row (including blank lines) — nil only on error.
func (t *Terminal) snapshotRow(row int) []ScreenCell {
	w := t.emu.Width()
	if w <= 0 {
		return nil
	}
	cells := make([]ScreenCell, w)
	for x := 0; x < w; x++ {
		cell := t.emu.CellAt(x, row)
		if cell == nil {
			cells[x] = ScreenCell{Content: " ", Width: 1}
			continue
		}
		cellWidth := int8(cell.Width)
		if cellWidth < 1 {
			cellWidth = 1
		}
		cells[x] = ScreenCell{
			Content: cell.Content,
			Fg:      cell.Style.Fg,
			Bg:      cell.Style.Bg,
			Attrs:   cell.Style.Attrs,
			Width:   cellWidth,
		}
	}
	return cells
}

// rowEqual compares two ScreenCell rows for equality.
func rowEqual(a, b []ScreenCell) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i].Content != b[i].Content {
			return false
		}
	}
	return true
}

// HasMouseMode returns true if the child app has enabled any mouse reporting.
func (t *Terminal) HasMouseMode() bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.mouseMode != 0
}

// HasMouseMotionMode returns true if the child app tracks mouse motion
// (ButtonEvent mode 1002 or AnyEvent mode 1003).
func (t *Terminal) HasMouseMotionMode() bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.mouseMode == ansi.ButtonEventMouseMode.Mode() || t.mouseMode == ansi.AnyEventMouseMode.Mode()
}

// ScrollbackLen returns the number of lines in the scrollback buffer.
func (t *Terminal) ScrollbackLen() int {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.scrollLen
}

// ScrollbackLine returns a scrollback line by offset from the bottom.
// offset 0 = most recent scrollback line (just above visible screen).
func (t *Terminal) ScrollbackLine(offset int) []ScreenCell {
	t.mu.Lock()
	defer t.mu.Unlock()
	if offset < 0 || offset >= t.scrollLen {
		return nil
	}
	// Ring buffer: head points to next write slot, so most recent is at head-1.
	// offset 0 = head-1, offset 1 = head-2, etc.
	idx := (t.scrollHead - 1 - offset + t.scrollCap) % t.scrollCap
	src := t.scrollRing[idx]
	if src == nil {
		return nil
	}
	// Return a copy to avoid races.
	line := make([]ScreenCell, len(src))
	copy(line, src)
	return line
}

// WritePTYDirect writes bytes directly to the PTY master, bypassing the async
// write channel. This is used for time-critical responses that must arrive
// before other data (e.g. Kitty graphics query responses must arrive before
// the DA1 sentinel response that the emulator generates). Thread-safe because
// os.File.Write() acquires an internal lock.
func (t *Terminal) WritePTYDirect(data []byte) {
	t.mu.Lock()
	if t.closed {
		t.mu.Unlock()
		return
	}
	p := t.pty
	t.mu.Unlock()
	if p != nil {
		p.Write(data)
	}
}

// WriteInput sends raw bytes to the PTY (keyboard input).
// Non-blocking — writes are buffered and processed by the writer goroutine.
func (t *Terminal) WriteInput(data []byte) {
	t.mu.Lock()
	if t.closed {
		t.mu.Unlock()
		return
	}
	// Copy data to avoid races — caller may reuse the slice.
	buf := make([]byte, len(data))
	copy(buf, data)
	select {
	case t.writeCh <- buf:
	default:
		// Channel full — drop input to avoid blocking the UI.
	}
	t.mu.Unlock()
}

// writeLoop drains the write channel and sends to the PTY.
func (t *Terminal) writeLoop() {
	for data := range t.writeCh {
		t.mu.Lock()
		closed := t.closed
		t.mu.Unlock()
		if closed {
			return
		}
		t.pty.Write(data)
	}
}

// inputForwardLoop reads encoded input from the emulator's internal pipe
// (populated by SendKey/SendMouse/SendText) and writes it to the PTY.
// The emulator handles mode-dependent encoding:
// - Application Cursor Keys mode (DECCKM) for arrow keys in nvim
// - Mouse mode tracking (only forwards mouse events when app has enabled mouse)
// - Application Keypad mode for numeric keypad
func (t *Terminal) inputForwardLoop() {
	defer close(t.inputDone)
	buf := make([]byte, 4096)
	for {
		n, err := t.emu.Read(buf)
		if n > 0 {
			t.mu.Lock()
			closed := t.closed
			t.mu.Unlock()
			if closed {
				return
			}
			t.pty.Write(buf[:n])
		}
		if err != nil {
			return
		}
	}
}

// SendKey sends a key event through the emulator's input pipeline.
// The emulator handles mode-dependent encoding (Application Cursor Keys, etc.)
// which is critical for apps like nvim.
func (t *Terminal) SendKey(code rune, mod uv.KeyMod, text string) {
	t.mu.Lock()
	closed := t.closed
	t.mu.Unlock()
	if closed {
		return
	}

	// Workaround: the vt emulator's SendKey only outputs printable characters
	// when Mod==0. Characters typed with Shift or CapsLock (e.g. "A", "!", "@")
	// have non-zero Mod and are silently dropped. For these, write the text
	// directly to the PTY, bypassing the emulator's key encoding.
	if text != "" && mod != 0 && mod&(uv.ModCtrl|uv.ModAlt) == 0 {
		t.WriteInput([]byte(text))
		return
	}

	t.emu.SendKey(uv.KeyPressEvent(uv.Key{Code: code, Mod: mod, Text: text}))
}

// SendMouse sends a mouse click event through the emulator's input pipeline.
// The emulator only forwards mouse events when the terminal app has enabled
// mouse mode — this prevents SGR sequences from appearing as "weird text".
// col, row: 0-indexed coordinates relative to terminal content area.
func (t *Terminal) SendMouse(button uv.MouseButton, col, row int, release bool) {
	t.mu.Lock()
	closed := t.closed
	t.mu.Unlock()
	if closed {
		return
	}
	if release {
		t.emu.SendMouse(uv.MouseReleaseEvent(uv.Mouse{
			X:      col,
			Y:      row,
			Button: button,
		}))
	} else {
		t.emu.SendMouse(uv.MouseClickEvent(uv.Mouse{
			X:      col,
			Y:      row,
			Button: button,
		}))
	}
}

// SendMouseMotion sends a mouse motion event through the emulator's input pipeline.
// col, row: 0-indexed coordinates.
func (t *Terminal) SendMouseMotion(button uv.MouseButton, col, row int) {
	t.mu.Lock()
	closed := t.closed
	t.mu.Unlock()
	if closed {
		return
	}
	t.emu.SendMouse(uv.MouseMotionEvent(uv.Mouse{
		X:      col,
		Y:      row,
		Button: button,
	}))
}

// SendMouseWheel sends a mouse wheel event through the emulator's input pipeline.
func (t *Terminal) SendMouseWheel(button uv.MouseButton, col, row int) {
	t.mu.Lock()
	closed := t.closed
	t.mu.Unlock()
	if closed {
		return
	}
	t.emu.SendMouse(uv.MouseWheelEvent(uv.Mouse{
		X:      col,
		Y:      row,
		Button: button,
	}))
}

// IsCursorHidden returns whether the terminal app has hidden the cursor (DECTCEM).
func (t *Terminal) IsCursorHidden() bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.cursorHidden
}

// Title returns the last OSC 0/2 window title set by the child app, or "".
func (t *Terminal) Title() string {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.title
}

// CursorPosition returns the cursor's X, Y position in the terminal grid.
func (t *Terminal) CursorPosition() (int, int) {
	pos := t.emu.CursorPosition()
	return pos.X, pos.Y
}

// Render returns the terminal screen as an ANSI-encoded string.
func (t *Terminal) Render() string {
	return t.emu.Render()
}

// CellAt returns the VT emulator cell at the given position.
func (t *Terminal) CellAt(x, y int) *uv.Cell {
	return t.emu.CellAt(x, y)
}

// SnapshotScreen returns a consistent snapshot of the emulator screen.
// This reduces tearing by drawing under a single emulator lock.
func (t *Terminal) SnapshotScreen() ([][]ScreenCell, int, int) {
	w := t.emu.Width()
	h := t.emu.Height()
	if w <= 0 || h <= 0 {
		return nil, w, h
	}
	screen := newEmuScreen(w, h)
	t.emu.Draw(screen, uv.Rect(0, 0, w, h))
	return screen.cells, w, h
}

// Width returns the emulator's column count.
func (t *Terminal) Width() int {
	return t.emu.Width()
}

// Height returns the emulator's row count.
func (t *Terminal) Height() int {
	return t.emu.Height()
}

// Resize updates the terminal and PTY dimensions.
func (t *Terminal) Resize(cols, rows int) {
	t.mu.Lock()
	if t.closed {
		t.mu.Unlock()
		return
	}
	if cols > 0 && t.scrollWidth != 0 && cols != t.scrollWidth {
		// Preserve scrollback on width change. Existing lines may be wider/narrower
		// than the new width, but rendering clips to content area and cellsToString
		// handles variable-length slices. Much better than discarding all history.
		t.scrollWidth = cols
	}
	t.dirty = true
	// Keep returning dirty for 800ms after resize so intermediate View()
	// calls don't cache stale content before the terminal app responds to SIGWINCH.
	t.dirtyUntil = time.Now().Add(800 * time.Millisecond)
	p := t.pty
	t.mu.Unlock()

	// Resize emulator and PTY WITHOUT holding t.mu to prevent ABBA deadlock.
	t.emu.Resize(cols, rows)
	if p != nil {
		p.Resize(uint16(rows), uint16(cols))
	}
}

// Close terminates the terminal session.
func (t *Terminal) Close() error {
	var pty Pty
	var emu Emulator

	t.mu.Lock()
	if t.closed {
		t.mu.Unlock()
		return nil
	}
	t.closed = true
	pty = t.pty
	emu = t.emu
	// Release scrollback memory eagerly — don't wait for GC.
	// A 200-col × 10k-line scrollback can hold ~100MB+ of ScreenCell data
	// (strings, color.Color interfaces). Nil the ring so it's GC-eligible
	// immediately, not whenever the Terminal struct itself becomes unreachable.
	t.scrollRing = nil
	t.scrollLen = 0
	t.scrollHead = 0
	t.emuOverflow = nil
	// Close channels under lock to avoid racing sends.
	t.closeOnce.Do(func() {
		close(t.done)
		close(t.writeCh)
		close(t.emuCh)
	})
	t.mu.Unlock()

	// Close the emulator's input pipe writer to unblock inputForwardLoop's
	// emu.Read() with an error — without touching the emulator's internal
	// closed field (avoids the Read/Close data race in the VT library).
	if emu != nil {
		if pw, ok := emu.InputPipe().(io.Closer); ok {
			_ = pw.Close()
		}
	}

	// Wait for inputForwardLoop to exit before calling emu.Close().
	// Now there is no concurrent Read, so Close is safe.
	<-t.inputDone

	// Wait for emuWriteLoop to finish draining buffered writes before
	// closing the emulator — prevents concurrent emu.Write()/emu.Close().
	<-t.emuWriteDone

	if emu != nil {
		_ = emu.Close()
	}
	if pty != nil {
		_ = pty.Close()
	}
	return nil
}

// IsClosed returns whether the terminal has been closed.
func (t *Terminal) IsClosed() bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.closed
}

// ConsumeDirty reports whether the terminal screen changed since last render.
// It resets the dirty flag so subsequent calls return false until new data arrives.
// During the post-resize/alt-screen grace period, it returns true WITHOUT
// consuming, so every View() call re-snapshots the emulator with progressively
// fresher content.
func (t *Terminal) ConsumeDirty() bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	// Suppress dirty during sync output (mode 2026) — the child app is
	// mid-frame. Rendering now would snapshot a partially-cleared screen.
	// 100ms timeout prevents permanent freeze if sync never ends.
	if t.syncOutput && time.Since(t.syncOutputTime) < 100*time.Millisecond {
		return false
	}
	// During grace period (resize or alt-screen transition), always report
	// dirty without consuming. This prevents intermediate View() calls from
	// caching stale content before the terminal app has responded.
	if !t.dirtyUntil.IsZero() && time.Now().Before(t.dirtyUntil) {
		return true
	}
	t.dirtyUntil = time.Time{} // clear expired grace period
	if t.dirty {
		t.dirty = false
		return true
	}
	return false
}

// DirtyGen returns the current dirty generation counter. Each write batch
// increments this counter. The renderer can compare against a stored value
// to detect changes without consuming a boolean flag.
func (t *Terminal) DirtyGen() uint64 {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.dirtyGen
}

// MarkDirty forces the terminal's dirty flag so the next render re-snapshots.
func (t *Terminal) MarkDirty() {
	t.mu.Lock()
	t.dirty = true
	t.mu.Unlock()
}

// CaptureBufferText returns the current visible screen as plain text.
// This captures only visible content without colors/attributes.
// Returns multi-line string with newlines between rows.
// Maximum 10KB returned (truncates if larger).
func (t *Terminal) CaptureBufferText() string {
	t.mu.Lock()
	if t.closed {
		t.mu.Unlock()
		return ""
	}
	t.mu.Unlock()

	// Access emulator WITHOUT holding t.mu to prevent ABBA deadlock:
	// SafeEmulator callbacks acquire t.mu while holding the emulator lock.
	width := t.emu.Width()
	height := t.emu.Height()

	var sb strings.Builder
	maxSize := 10 * 1024 // 10KB limit

	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			cell := t.emu.CellAt(x, y)
			if cell == nil || cell.Content == "" {
				sb.WriteRune(' ')
			} else {
				sb.WriteString(cell.Content)
			}

			// Check size limit
			if sb.Len() > maxSize {
				sb.WriteString("... [truncated]")
				return sb.String()
			}
		}
		if y < height-1 {
			sb.WriteByte('\n')
		}
	}

	return sb.String()
}

// CaptureFullBuffer returns scrollback + visible screen as plain text.
// Captures up to maxLines total lines (scrollback oldest→newest, then visible).
// Trailing blank lines are stripped. Each line is right-trimmed of spaces.
func (t *Terminal) CaptureFullBuffer(maxLines int) string {
	// Phase 1: Copy scrollback data under t.mu.
	t.mu.Lock()
	if t.closed {
		t.mu.Unlock()
		return ""
	}
	scrollLen := t.scrollLen
	scrollCap := t.scrollCap
	scrollHead := t.scrollHead

	// Copy scrollback rows under lock (oldest first).
	scrollCopy := make([][]ScreenCell, scrollLen)
	for i := 0; i < scrollLen; i++ {
		idx := (scrollHead - scrollLen + i + scrollCap) % scrollCap
		src := t.scrollRing[idx]
		if src != nil {
			row := make([]ScreenCell, len(src))
			copy(row, src)
			scrollCopy[i] = row
		}
	}
	t.mu.Unlock()

	// Phase 2: Access emulator WITHOUT holding t.mu to prevent ABBA deadlock.
	width := t.emu.Width()
	height := t.emu.Height()

	totalLines := scrollLen + height
	if totalLines > maxLines {
		totalLines = maxLines
	}

	// Scrollback budget: how many scrollback lines to include.
	scrollBudget := totalLines - height
	if scrollBudget < 0 {
		scrollBudget = 0
	}
	if scrollBudget > scrollLen {
		scrollBudget = scrollLen
	}

	lines := make([]string, 0, totalLines)
	var lb strings.Builder
	lb.Grow(width)

	// Scrollback lines (oldest first, trimmed to budget).
	offset := scrollLen - scrollBudget
	for i := offset; i < scrollLen; i++ {
		lb.Reset()
		row := scrollCopy[i]
		if row != nil {
			for _, cell := range row {
				if cell.Content == "" {
					lb.WriteByte(' ')
				} else {
					lb.WriteString(cell.Content)
				}
			}
		}
		lines = append(lines, strings.TrimRight(lb.String(), " "))
	}

	// Visible screen lines (emulator access without t.mu).
	for y := 0; y < height; y++ {
		lb.Reset()
		for x := 0; x < width; x++ {
			cell := t.emu.CellAt(x, y)
			if cell == nil || cell.Content == "" {
				lb.WriteByte(' ')
			} else {
				lb.WriteString(cell.Content)
			}
		}
		lines = append(lines, strings.TrimRight(lb.String(), " "))
	}

	// Strip trailing blank lines.
	for len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}

	return strings.Join(lines, "\n")
}

// RequestAppState sends SIGUSR1 to the PTY process to request app state.
// Apps that support the protocol will respond with OSC 667.
func (t *Terminal) RequestAppState() error {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.closed {
		return nil
	}
	return t.pty.Signal(syscall.SIGUSR1)
}

// AppState returns the last captured app state (base64-encoded JSON).
func (t *Terminal) AppState() string {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.appState
}

// Pid returns the PTY child process ID, or 0 if unavailable.
func (t *Terminal) Pid() int {
	return t.pty.Pid()
}

// foregroundPgid returns the foreground process group ID of the PTY session
// by reading /proc/PID/stat (Linux). Returns 0 on error.
func (t *Terminal) foregroundPgid() int {
	pid := t.Pid()
	if pid <= 0 {
		return 0
	}
	data, err := os.ReadFile(fmt.Sprintf("/proc/%d/stat", pid))
	if err != nil {
		return 0
	}
	// /proc/PID/stat format: PID (comm) state ppid pgrp session tty_nr tpgid ...
	// Find last ')' to skip comm (may contain spaces/parens).
	s := string(data)
	idx := strings.LastIndex(s, ")")
	if idx < 0 {
		return 0
	}
	fields := strings.Fields(s[idx+1:])
	// fields: [0]=state [1]=ppid [2]=pgrp [3]=session [4]=tty_nr [5]=tpgid
	if len(fields) < 6 {
		return 0
	}
	pgid, err := strconv.Atoi(fields[5])
	if err != nil {
		return 0
	}
	return pgid
}

// ForegroundCommand returns the command name of the PTY's foreground process
// (e.g. "nvim" when the user launched nvim from a shell). Linux only.
func (t *Terminal) ForegroundCommand() string {
	pgid := t.foregroundPgid()
	if pgid <= 0 {
		return ""
	}
	comm, err := os.ReadFile(fmt.Sprintf("/proc/%d/comm", pgid))
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(comm))
}

// GetCWD returns the current working directory of the PTY's foreground process.
// Tries the foreground process first (more accurate when a subprocess like nvim
// has cd'd), falls back to the main PTY process. Linux only (/proc).
func (t *Terminal) GetCWD() string {
	// Try foreground process first.
	if pgid := t.foregroundPgid(); pgid > 0 {
		if cwd, err := os.Readlink(fmt.Sprintf("/proc/%d/cwd", pgid)); err == nil {
			return cwd
		}
	}
	// Fall back to main process.
	pid := t.Pid()
	if pid <= 0 {
		return ""
	}
	cwd, err := os.Readlink(fmt.Sprintf("/proc/%d/cwd", pid))
	if err != nil {
		return ""
	}
	return cwd
}

// RestoreBuffer restores saved content to the terminal, populating both the
// scrollback ring buffer and the visible screen. Lines beyond the emulator
// height go into scrollback (oldest first). Only the last `height` lines
// are written to the emulator for display.
func (t *Terminal) RestoreBuffer(content string) {
	t.mu.Lock()
	if t.closed || content == "" {
		t.mu.Unlock()
		return
	}
	t.dirty = true
	t.mu.Unlock()

	// Access emulator dimensions WITHOUT holding t.mu to prevent ABBA deadlock.
	lines := strings.Split(content, "\n")
	height := t.emu.Height()
	width := t.emu.Width()

	if len(lines) > height {
		// Excess lines go into scrollback ring buffer directly.
		scrollLines := lines[:len(lines)-height]
		t.mu.Lock()
		for _, line := range scrollLines {
			row := make([]ScreenCell, width)
			runes := []rune(line)
			for i := 0; i < width; i++ {
				if i < len(runes) {
					row[i] = ScreenCell{Content: string(runes[i]), Width: 1}
				} else {
					row[i] = ScreenCell{Content: " ", Width: 1}
				}
			}
			t.scrollRing[t.scrollHead] = row
			t.scrollHead = (t.scrollHead + 1) % t.scrollCap
			if t.scrollLen < t.scrollCap {
				t.scrollLen++
			}
		}
		t.mu.Unlock()

		// Write visible portion to emulator WITHOUT holding t.mu.
		visible := strings.Join(lines[len(lines)-height:], "\r\n")
		t.emu.Write([]byte(visible))
	} else {
		// Fits on screen — write all to emulator WITHOUT holding t.mu.
		visible := strings.ReplaceAll(content, "\n", "\r\n")
		t.emu.Write([]byte(visible))
	}
}

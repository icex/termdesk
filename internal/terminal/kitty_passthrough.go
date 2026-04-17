package terminal

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/charmbracelet/x/term"
)

// kittyPassDbg is a temporary debug logger for Kitty graphics passthrough.
func kittyPassDbg(format string, args ...any) {
	f, err := os.OpenFile("/tmp/termdesk-kitty.log", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return
	}
	defer f.Close()
	fmt.Fprintf(f, "[%s] ", time.Now().Format("15:04:05.000"))
	fmt.Fprintf(f, format+"\n", args...)
}

// KittyPassthrough manages forwarding Kitty graphics protocol sequences from
// child PTY applications to the real host terminal. It remaps image IDs to
// prevent collisions between windows, tracks placements for visibility updates,
// and accumulates output for batch flushing.
type KittyPassthrough struct {
	mu            sync.Mutex
	enabled       bool
	placements    map[string]map[uint32]*KittyPlacement // windowID → hostID → placement
	imageIDMap    map[string]map[uint32]uint32           // windowID → guestID → hostID
	nextHostID    uint32
	pendingOutput []byte
	pendingDirect map[string]*pendingDirectTransmit // windowID → accumulating chunks
	cellPixelW    int
	cellPixelH    int
}

// KittyPlacement tracks a placed image for visibility management.
type KittyPlacement struct {
	GuestImageID uint32
	HostImageID  uint32
	PlacementID  uint32
	WindowID     string
	GuestX       int // cursor X in guest terminal at placement time
	AbsoluteLine int // scrollbackLen + cursorY at placement time
	HostX        int // actual screen X
	HostY        int // actual screen Y
	Cols         int // display columns
	Rows         int // original image rows
	DisplayRows  int // capped rows for display
	Hidden       bool
	PixelWidth   int // actual image pixel width (for correct source clipping)
	PixelHeight  int // actual image pixel height (for correct source clipping)

	// Source clipping (preserved for re-placement)
	SourceX      int
	SourceY      int
	SourceWidth  int
	SourceHeight int
	XOffset      int
	YOffset      int
	ZIndex       int32
	Virtual      bool
	IsAltScreen  bool // placed during alt screen

	// Current clipping
	ClipTop         int
	MaxShowable     int
	MaxShowableCols int

	// Deferred placement: don't transition Hidden→visible until the
	// emulator scrollback reaches this value. Cursor injection newlines
	// (in safeEmuWrite) cause scrolling AFTER the callback sends
	// KittyFlushMsg, creating a race — refresh sees stale scrollback
	// and places the image at a pre-scroll position (off-screen flash).
	// Set to 0 for immediate show (no cursor injection expected).
	ReadyScrollback int
}

// pendingDirectTransmit accumulates chunked direct transmissions.
type pendingDirectTransmit struct {
	Data         []byte
	Format       KittyGraphicsFormat
	Compression  KittyGraphicsCompression
	Width        int
	Height       int
	ImageID      uint32
	Columns      int
	Rows         int
	SourceX      int
	SourceY      int
	SourceWidth  int
	SourceHeight int
	XOffset      int
	YOffset      int
	ZIndex       int32
	Virtual      bool
	CursorMove   int
	AndPlace     bool // true if first chunk was a=T (transmit+place)
	// Position from first chunk
	WindowX       int
	WindowY       int
	WindowWidth   int
	WindowHeight  int
	ContentOffX   int
	ContentOffY   int
	CursorX       int
	CursorY       int
	ScrollbackLen int
	IsAltScreen   bool
}

// KittyWindowInfo provides window position/state for placement refresh.
type KittyWindowInfo struct {
	WindowX       int
	WindowY       int
	ContentOffX   int
	ContentOffY   int
	ContentWidth  int // content area width (window width minus borders)
	ContentHeight int // content area height (window height minus titlebar and border)
	Width         int
	Height        int
	Visible       bool
	ScrollbackLen int
	ScrollOffset  int
	IsManipulated bool // being dragged/resized
	ZOrder        int
	IsAltScreen   bool
}

// PlacementResult returned to caller for cursor space reservation.
type PlacementResult struct {
	Rows       int
	Cols       int
	CursorMove int
}

// NewKittyPassthrough creates a new Kitty graphics passthrough.
// It detects whether the host terminal supports Kitty graphics.
func NewKittyPassthrough(cellPixelW, cellPixelH int) *KittyPassthrough {
	enabled := detectKittyGraphicsSupport()
	kittyPassDbg("NewKittyPassthrough: enabled=%v cellPx=%dx%d TERM_PROGRAM=%s TERM=%s",
		enabled, cellPixelW, cellPixelH, os.Getenv("TERM_PROGRAM"), os.Getenv("TERM"))
	return &KittyPassthrough{
		enabled:       enabled,
		placements:    make(map[string]map[uint32]*KittyPlacement),
		imageIDMap:    make(map[string]map[uint32]uint32),
		nextHostID:    1,
		pendingDirect: make(map[string]*pendingDirectTransmit),
		cellPixelW:    cellPixelW,
		cellPixelH:    cellPixelH,
	}
}

// detectKittyGraphicsSupport checks if the host terminal supports Kitty graphics.
// First checks TERMDESK_GRAPHICS (propagated through session system), then
// tries env-based detection, then falls back to an active /dev/tty query.
func detectKittyGraphicsSupport() bool {
	// Session system propagation: BuildAppEnv() detected graphics from the
	// host terminal's env before stripping and set this flag.
	if os.Getenv("TERMDESK_GRAPHICS") == "kitty" {
		return true
	}
	// Direct env checks (for non-session or direct --app usage)
	if os.Getenv("KITTY_WINDOW_ID") != "" {
		return true
	}
	if os.Getenv("WEZTERM_PANE") != "" {
		return true
	}
	tp := os.Getenv("TERM_PROGRAM")
	switch tp {
	case "WezTerm", "kitty", "ghostty":
		return true
	}
	// Konsole also supports Kitty graphics
	if os.Getenv("KONSOLE_DBUS_SESSION") != "" || os.Getenv("KONSOLE_VERSION") != "" {
		return true
	}
	// Do NOT call detectKittyByTTYQuery() here — this runs inside the --app
	// child behind a PTY proxy. Opening /dev/tty accesses the slave PTY (not
	// the real terminal), and term.MakeRaw corrupts the PTY state for BubbleTea.
	// Graphics detection is handled by the session launcher:
	//   - createAndAttach() calls DetectKittyGraphicsTTY() on the real terminal
	//   - BuildAppEnv() propagates TERMDESK_GRAPHICS=kitty to the child
	return false
}

// DetectKittyGraphicsTTY sends a Kitty graphics query directly to /dev/tty.
// Exported for use by the session launcher — must be called BEFORE Setsid
// detaches the controlling terminal. Over SSH, env-based detection fails
// (TERM_PROGRAM etc. are not forwarded), but a TTY query reaches the user's
// real terminal through the SSH channel.
func DetectKittyGraphicsTTY() bool {
	return detectKittyByTTYQuery()
}

// detectKittyByTTYQuery sends a Kitty graphics query to /dev/tty and checks
// for an OK response. Uses raw mode with a short timeout.
func detectKittyByTTYQuery() bool {
	tty, err := os.OpenFile("/dev/tty", os.O_RDWR, 0)
	if err != nil {
		return false
	}
	defer tty.Close()

	fd := tty.Fd()
	oldState, err := term.MakeRaw(fd)
	if err != nil {
		return false
	}
	defer term.Restore(fd, oldState)

	// Send Kitty graphics query with a unique image ID
	_, err = tty.Write([]byte("\x1b_Ga=q,i=31415926,s=1,v=1,t=d,f=24;AAAA\x1b\\"))
	if err != nil {
		return false
	}

	// Read response in a goroutine with timeout.
	// In raw mode (VMIN=1), Read blocks until data arrives.
	// tty.Close() in the defer will unblock a stuck Read.
	done := make(chan bool, 1)
	go func() {
		buf := make([]byte, 512)
		total := 0
		for total < len(buf) {
			n, err := tty.Read(buf[total:])
			if n > 0 {
				total += n
				if bytes.Contains(buf[:total], []byte("OK")) {
					done <- true
					return
				}
			}
			if err != nil {
				break
			}
		}
		done <- false
	}()

	select {
	case ok := <-done:
		return ok
	case <-time.After(500 * time.Millisecond):
		return false
	}
}

// HostTermProgram returns the host terminal's TERM_PROGRAM env var, or "".
// Used to propagate the value to child PTY processes.
func HostTermProgram() string {
	return os.Getenv("TERM_PROGRAM")
}

// HasKittyTerminfo checks if the xterm-kitty terminfo is installed on the system.
// When available, child processes can use TERM=xterm-kitty for full Kitty
// protocol detection by graphics-aware tools.
func HasKittyTerminfo() bool {
	home := os.Getenv("HOME")
	paths := []string{
		home + "/.terminfo/x/xterm-kitty",
		"/usr/share/terminfo/x/xterm-kitty",
		"/usr/lib/terminfo/x/xterm-kitty",
		"/etc/terminfo/x/xterm-kitty",
	}
	for _, p := range paths {
		if _, err := os.Stat(p); err == nil {
			return true
		}
	}
	return false
}

// IsEnabled returns whether Kitty graphics passthrough is active.
func (kp *KittyPassthrough) IsEnabled() bool {
	kp.mu.Lock()
	defer kp.mu.Unlock()
	return kp.enabled
}

// FlushPending returns and clears accumulated APC output bytes.
func (kp *KittyPassthrough) FlushPending() []byte {
	kp.mu.Lock()
	defer kp.mu.Unlock()
	if len(kp.pendingOutput) == 0 {
		return nil
	}
	out := kp.pendingOutput
	kp.pendingOutput = nil
	return out
}

func (kp *KittyPassthrough) allocateHostID() uint32 {
	id := kp.nextHostID
	kp.nextHostID++
	if kp.nextHostID == 0 {
		kp.nextHostID = 1
	}
	return id
}

func (kp *KittyPassthrough) getOrAllocateHostID(windowID string, guestID uint32) uint32 {
	if kp.imageIDMap[windowID] == nil {
		kp.imageIDMap[windowID] = make(map[uint32]uint32)
	}
	if hostID, ok := kp.imageIDMap[windowID][guestID]; ok {
		return hostID
	}
	hostID := kp.allocateHostID()
	kp.imageIDMap[windowID][guestID] = hostID
	return hostID
}

// readKittyFileMedium reads image data from a file-based medium (temp file,
// regular file, or shared memory) and returns the raw bytes. Temp files and
// shared memory segments are deleted after reading (the terminal is expected
// to consume them).
func readKittyFileMedium(medium KittyGraphicsMedium, filePath string) ([]byte, error) {
	path := filePath
	if medium == KittyMediumSharedMemory {
		path = "/dev/shm/" + filePath
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	// Delete temp files and shared memory after reading — the protocol
	// specifies that the terminal consumes them.
	if medium == KittyMediumTempFile || medium == KittyMediumSharedMemory {
		os.Remove(path)
	}
	return data, nil
}

// ForwardCommand processes a Kitty graphics command from a child terminal.
// ptyInput is called to send responses back to the child PTY.
func (kp *KittyPassthrough) ForwardCommand(
	cmd *KittyCommand,
	rawData []byte,
	windowID string,
	windowX, windowY, windowWidth, windowHeight int,
	contentOffX, contentOffY int,
	cursorX, cursorY int,
	scrollbackLen int,
	isAltScreen bool,
	ptyInput func([]byte),
) *PlacementResult {
	kp.mu.Lock()
	defer kp.mu.Unlock()

	if !kp.enabled {
		return nil
	}

	// Filter out echoed responses to prevent feedback loops
	if cmd.Action == KittyActionTransmit && isKittyResponse(cmd.Data) {
		return nil
	}

	kittyPassDbg("ForwardCommand: action=%c imageID=%d medium=%c format=%d width=%d height=%d rows=%d cols=%d compression=%c more=%v filePath=%q dataLen=%d",
		byte(cmd.Action), cmd.ImageID, byte(cmd.Medium), cmd.Format,
		cmd.Width, cmd.Height, cmd.Rows, cmd.Columns, byte(cmd.Compression), cmd.More, cmd.FilePath, len(cmd.Data))

	// Convert file-based mediums to direct transmission by reading the file.
	// kitten icat prefers temp file or shared memory (faster than inline),
	// but the host terminal receives our commands via /dev/tty so we must
	// re-encode as direct inline base64 data.
	// Treat unset (zero) medium as direct — parser default is 'd' but
	// programmatically created commands may omit it.
	if cmd.Medium == 0 {
		cmd.Medium = KittyMediumDirect
	}
	fileConverted := false
	if (cmd.Action == KittyActionTransmit || cmd.Action == KittyActionTransmitPlace) &&
		cmd.Medium != KittyMediumDirect && cmd.FilePath == "" {
		kittyPassDbg("ForwardCommand: file medium %c but filePath is empty — file path decode failed", byte(cmd.Medium))
		return nil
	}
	if (cmd.Action == KittyActionTransmit || cmd.Action == KittyActionTransmitPlace) &&
		cmd.Medium != KittyMediumDirect && cmd.FilePath != "" {
		fileData, err := readKittyFileMedium(cmd.Medium, cmd.FilePath)
		if err != nil {
			kittyPassDbg("ForwardCommand: failed to read file medium %c %q: %v", byte(cmd.Medium), cmd.FilePath, err)
			return nil
		}
		kittyPassDbg("ForwardCommand: converted file medium %c (%s) to direct (%d bytes), format=%d", byte(cmd.Medium), cmd.FilePath, len(fileData), cmd.Format)
		cmd.Data = fileData
		cmd.Medium = KittyMediumDirect
		fileConverted = true
	}

	switch cmd.Action {
	case KittyActionQuery:
		// Reject shared memory queries — we can't reliably read POSIX shm
		// (macOS has no /dev/shm, Linux has race conditions with async processing).
		// kitten icat will fall back to temp file or direct transmission.
		if cmd.Medium == KittyMediumSharedMemory {
			resp := BuildKittyResponse(false, cmd.ImageID, "ENOTSUPPORTED")
			kittyPassDbg("ForwardCommand: rejecting shm query imageID=%d", cmd.ImageID)
			if ptyInput != nil {
				ptyInput(resp)
			}
			return nil
		}
		resp := BuildKittyResponse(true, cmd.ImageID, "")
		kittyPassDbg("ForwardCommand: query imageID=%d, sending response (%d bytes): %q", cmd.ImageID, len(resp), resp)
		if ptyInput != nil {
			ptyInput(resp)
		}
		return nil

	case KittyActionTransmit:
		hasPending := kp.pendingDirect[windowID] != nil
		if !cmd.More && !hasPending && !fileConverted {
			// Simple transmit-only (no place, no file conversion) — forward raw directly.
			// rawData is valid and contains the original APC payload.
			kp.pendingOutput = append(kp.pendingOutput, "\x1b_G"...)
			kp.pendingOutput = append(kp.pendingOutput, rawData...)
			kp.pendingOutput = append(kp.pendingOutput, "\x1b\\"...)
			return nil
		}
		// Chunked transmit, file-converted data, or pending — accumulate and re-encode.
		// For continuation chunks of a=T (transmit+place), the parser defaults
		// to a=t. The pending struct preserves the AndPlace flag from the first chunk.
		andPlace := false
		if hasPending {
			andPlace = kp.pendingDirect[windowID].AndPlace
		}
		return kp.forwardDirectTransmit(cmd, windowID, andPlace,
			windowX, windowY, windowWidth, windowHeight,
			contentOffX, contentOffY, cursorX, cursorY, scrollbackLen, isAltScreen)

	case KittyActionTransmitPlace:
		return kp.forwardDirectTransmit(cmd, windowID, true,
			windowX, windowY, windowWidth, windowHeight,
			contentOffX, contentOffY, cursorX, cursorY, scrollbackLen, isAltScreen)

	case KittyActionPlace:
		kp.forwardPlace(cmd, windowID,
			windowX, windowY, windowWidth, windowHeight,
			contentOffX, contentOffY, cursorX, cursorY, scrollbackLen)
		return nil

	case KittyActionDelete:
		kp.forwardDelete(cmd, windowID)
		return nil
	}

	return nil
}

func (kp *KittyPassthrough) forwardDirectTransmit(
	cmd *KittyCommand, windowID string, andPlace bool,
	windowX, windowY, windowWidth, windowHeight int,
	contentOffX, contentOffY, cursorX, cursorY, scrollbackLen int,
	isAltScreen bool,
) *PlacementResult {
	pending := kp.pendingDirect[windowID]
	if pending == nil {
		pending = &pendingDirectTransmit{
			Format: cmd.Format, Compression: cmd.Compression,
			Width: cmd.Width, Height: cmd.Height, ImageID: cmd.ImageID,
			Columns: cmd.Columns, Rows: cmd.Rows,
			SourceX: cmd.SourceX, SourceY: cmd.SourceY,
			SourceWidth: cmd.SourceWidth, SourceHeight: cmd.SourceHeight,
			XOffset: cmd.XOffset, YOffset: cmd.YOffset,
			ZIndex: cmd.ZIndex, Virtual: cmd.Virtual, CursorMove: cmd.CursorMove,
			AndPlace: andPlace,
			WindowX: windowX, WindowY: windowY,
			WindowWidth: windowWidth, WindowHeight: windowHeight,
			ContentOffX: contentOffX, ContentOffY: contentOffY,
			CursorX: cursorX, CursorY: cursorY,
			ScrollbackLen: scrollbackLen, IsAltScreen: isAltScreen,
		}
		kp.pendingDirect[windowID] = pending
	}

	pending.Data = append(pending.Data, cmd.Data...)

	if cmd.More {
		return nil
	}

	// Final chunk — process complete image
	defer delete(kp.pendingDirect, windowID)

	if len(pending.Data) == 0 {
		return nil
	}

	// Clean up old placements for this window before creating new ones.
	// kitten icat doesn't always send explicit delete commands between
	// images. Without cleanup, old placements accumulate and may appear
	// at stale positions or interfere with the new image.
	if andPlace {
		kp.deleteAllWindowPlacements(windowID)
	}

	hostID := kp.allocateHostID()
	if kp.imageIDMap[windowID] == nil {
		kp.imageIDMap[windowID] = make(map[uint32]uint32)
	}
	kp.imageIDMap[windowID][pending.ImageID] = hostID

	encoded := base64.StdEncoding.EncodeToString(pending.Data)

	// Calculate image dimensions in cells. Use fallback cell pixel sizes
	// when the host terminal didn't report them (e.g., over SSH).
	imgRows := pending.Rows
	imgCols := pending.Columns
	cpW := kp.cellPixelW
	cpH := kp.cellPixelH
	if cpW <= 0 {
		cpW = 10 // reasonable default cell width
	}
	if cpH <= 0 {
		cpH = 20 // reasonable default cell height
	}
	if imgRows == 0 && pending.Height > 0 {
		imgRows = (pending.Height + cpH - 1) / cpH
	}
	if imgCols == 0 && pending.Width > 0 {
		imgCols = (pending.Width + cpW - 1) / cpW
	}

	// Cap to content area remaining from cursor position.
	// Without this, images placed near the bottom/right of a window overflow
	// the window borders, covering dock/menubar/other windows.
	// Use actual content offsets instead of hardcoded -2 to handle varying
	// title bar heights across themes.
	contentWidth := windowWidth - contentOffX - 1  // left offset + right border
	contentHeight := windowHeight - contentOffY - 1 // top offset + bottom border
	displayCols := imgCols
	displayRows := imgRows
	remainingCols := contentWidth - pending.CursorX
	remainingRows := contentHeight - pending.CursorY
	kittyPassDbg("forwardDirectTransmit: cursorY=%d contentH=%d remainingRows=%d imgRows=%d imgCols=%d remainingCols=%d",
		pending.CursorY, contentHeight, remainingRows, imgRows, imgCols, remainingCols)
	if remainingCols > 0 {
		if displayCols > remainingCols {
			displayCols = remainingCols
		}
		// When image width is unknown (imgCols=0), cap to available width
		// to prevent the image from overflowing the right window border.
		if displayCols <= 0 {
			displayCols = remainingCols
		}
	}
	if displayRows > remainingRows && remainingRows > 0 {
		displayRows = remainingRows
	}

	// Transmit in chunks (4096 byte limit per chunk).
	// Only include transmission params (a, i, f, s, v, o) — NOT placement
	// params (c, r, x, y, w, h, z). Placement is done separately via placeOne.
	const chunkSize = 4096
	for i := 0; i < len(encoded); i += chunkSize {
		end := i + chunkSize
		if end > len(encoded) {
			end = len(encoded)
		}
		chunk := encoded[i:end]
		more := end < len(encoded)

		var buf bytes.Buffer
		buf.WriteString("\x1b_G")
		if i == 0 {
			fmt.Fprintf(&buf, "a=t,i=%d,f=%d,q=2",
				hostID, pending.Format)
			if pending.Width > 0 {
				fmt.Fprintf(&buf, ",s=%d", pending.Width)
			}
			if pending.Height > 0 {
				fmt.Fprintf(&buf, ",v=%d", pending.Height)
			}
			if pending.Compression == KittyCompressionZlib {
				buf.WriteString(",o=z")
			}
		} else {
			fmt.Fprintf(&buf, "i=%d,q=2", hostID)
		}
		if more {
			buf.WriteString(",m=1")
		}
		buf.WriteByte(';')
		buf.WriteString(chunk)
		buf.WriteString("\x1b\\")
		kp.pendingOutput = append(kp.pendingOutput, buf.Bytes()...)
	}

	// Track placement — but DON'T place immediately. Cursor injection
	// (imgRows newlines) happens AFTER this function returns, which may
	// cause the emulator to scroll. If we placed now, the image would
	// appear at the pre-scroll position for one frame, leaking outside
	// the window boundary. Instead, mark Hidden and let the next
	// refreshKittyPlacements (triggered by PtyOutputMsg after cursor
	// injection completes) compute the correct post-scroll position.
	if andPlace {
		if kp.placements[windowID] == nil {
			kp.placements[windowID] = make(map[uint32]*KittyPlacement)
		}
		// Calculate expected scrollback after cursor injection.
		// Cursor injection feeds imgRows newlines; each one that hits the
		// bottom row causes a scroll. If cursor is at row Y with content
		// height H, the number of scrolls is max(0, Y + imgRows - H + 1).
		expectedScrolls := pending.CursorY + imgRows - contentHeight + 1
		if expectedScrolls < 0 {
			expectedScrolls = 0
		}
		readyScrollback := pending.ScrollbackLen + expectedScrolls
		// If CursorMove=1 (C=1), no cursor injection happens — show immediately.
		if pending.CursorMove == 1 {
			readyScrollback = 0
		}

		p := &KittyPlacement{
			GuestImageID: pending.ImageID,
			HostImageID:  hostID,
			WindowID:     windowID,
			GuestX:       pending.CursorX,
			AbsoluteLine: pending.ScrollbackLen + pending.CursorY,
			HostX:        0, HostY: 0, // will be set by refresh
			Cols: imgCols, Rows: imgRows, DisplayRows: displayRows,
			MaxShowable: displayRows, MaxShowableCols: displayCols,
			PixelWidth: pending.Width, PixelHeight: pending.Height,
			SourceX: pending.SourceX, SourceY: pending.SourceY,
			SourceWidth: pending.SourceWidth, SourceHeight: pending.SourceHeight,
			XOffset: pending.XOffset, YOffset: pending.YOffset,
			ZIndex: pending.ZIndex, Virtual: pending.Virtual,
			IsAltScreen: pending.IsAltScreen,
			Hidden:          true, // deferred — refresh will show it
			ReadyScrollback: readyScrollback,
		}
		kp.placements[windowID][hostID] = p
		kittyPassDbg("forwardDirectTransmit: deferred place hostID=%d imgPx=%dx%d cells=%dx%d absLine=%d scrollback=%d cursorY=%d",
			hostID, pending.Width, pending.Height, imgCols, imgRows,
			p.AbsoluteLine, pending.ScrollbackLen, pending.CursorY)
		// Return full imgRows for cursor injection. The real terminal
		// advances the cursor by the entire image height (scrolling if
		// needed). displayRows caps the visual placement but the emulator
		// must match the real terminal's cursor behavior.
		return &PlacementResult{Rows: imgRows, Cols: displayCols, CursorMove: pending.CursorMove}
	}
	return nil
}

func (kp *KittyPassthrough) forwardPlace(
	cmd *KittyCommand, windowID string,
	windowX, windowY, windowWidth, windowHeight int,
	contentOffX, contentOffY, cursorX, cursorY, scrollbackLen int,
) {
	hostID := kp.getOrAllocateHostID(windowID, cmd.ImageID)
	hostX := windowX + contentOffX + cursorX
	hostY := windowY + contentOffY + cursorY

	contentWidth := windowWidth - contentOffX - 1
	contentHeight := windowHeight - contentOffY - 1
	imgRows, imgCols := cmd.Rows, cmd.Columns
	displayCols := imgCols
	displayRows := imgRows
	remainingCols := contentWidth - cursorX
	remainingRows := contentHeight - cursorY
	if displayCols > remainingCols && remainingCols > 0 {
		displayCols = remainingCols
	}
	if displayRows > remainingRows && remainingRows > 0 {
		displayRows = remainingRows
	}

	var buf bytes.Buffer
	fmt.Fprintf(&buf, "\x1b[%d;%dH", hostY+1, hostX+1)
	buf.WriteString("\x1b_G")
	fmt.Fprintf(&buf, "a=p,i=%d", hostID)
	if cmd.PlacementID > 0 {
		fmt.Fprintf(&buf, ",p=%d", cmd.PlacementID)
	}
	if displayCols > 0 {
		fmt.Fprintf(&buf, ",c=%d", displayCols)
	}
	if displayRows > 0 {
		fmt.Fprintf(&buf, ",r=%d", displayRows)
	}
	if cmd.SourceX > 0 {
		fmt.Fprintf(&buf, ",x=%d", cmd.SourceX)
	}
	if cmd.SourceY > 0 {
		fmt.Fprintf(&buf, ",y=%d", cmd.SourceY)
	}
	if cmd.SourceWidth > 0 {
		fmt.Fprintf(&buf, ",w=%d", cmd.SourceWidth)
	}
	if cmd.SourceHeight > 0 {
		fmt.Fprintf(&buf, ",h=%d", cmd.SourceHeight)
	}
	if cmd.ZIndex != 0 {
		fmt.Fprintf(&buf, ",z=%d", cmd.ZIndex)
	}
	buf.WriteString(",C=1,q=2\x1b\\")
	kp.pendingOutput = append(kp.pendingOutput, buf.Bytes()...)

	if kp.placements[windowID] == nil {
		kp.placements[windowID] = make(map[uint32]*KittyPlacement)
	}
	kp.placements[windowID][hostID] = &KittyPlacement{
		GuestImageID: cmd.ImageID, HostImageID: hostID,
		PlacementID: cmd.PlacementID, WindowID: windowID,
		GuestX: cursorX, AbsoluteLine: scrollbackLen + cursorY,
		HostX: hostX, HostY: hostY,
		Cols: displayCols, Rows: imgRows, DisplayRows: displayRows,
		SourceX: cmd.SourceX, SourceY: cmd.SourceY,
		SourceWidth: cmd.SourceWidth, SourceHeight: cmd.SourceHeight,
		XOffset: cmd.XOffset, YOffset: cmd.YOffset,
		ZIndex: cmd.ZIndex, Virtual: cmd.Virtual,
	}
}

func (kp *KittyPassthrough) forwardDelete(cmd *KittyCommand, windowID string) {
	switch cmd.Delete {
	case KittyDeleteAll, 0:
		kp.deleteAllWindowPlacements(windowID)
	case KittyDeleteByID:
		if windowMap := kp.imageIDMap[windowID]; windowMap != nil {
			if hostID, ok := windowMap[cmd.ImageID]; ok {
				kp.deleteImageAndPlacements(&KittyPlacement{HostImageID: hostID})
				if p := kp.placements[windowID]; p != nil {
					delete(p, hostID)
				}
				delete(windowMap, cmd.ImageID)
			}
		}
	case KittyDeleteByIDAndPlacement:
		if windowMap := kp.imageIDMap[windowID]; windowMap != nil {
			if hostID, ok := windowMap[cmd.ImageID]; ok {
				var buf bytes.Buffer
				buf.WriteString("\x1b_G")
				fmt.Fprintf(&buf, "a=d,d=I,i=%d", hostID)
				if cmd.PlacementID > 0 {
					fmt.Fprintf(&buf, ",p=%d", cmd.PlacementID)
				}
				buf.WriteString(",q=2\x1b\\")
				kp.pendingOutput = append(kp.pendingOutput, buf.Bytes()...)
			}
		}
	}
}

func (kp *KittyPassthrough) deleteAllWindowPlacements(windowID string) {
	for _, p := range kp.placements[windowID] {
		kp.deleteImageAndPlacements(p)
	}
	delete(kp.placements, windowID)
}

// removePlacement removes a single image's default placement from the host
// terminal but keeps image data in memory. Uses d=p with p=0 to target the
// default placement. Used for hide operations (scroll off-screen, alt-screen).
func (kp *KittyPassthrough) removePlacement(p *KittyPlacement) {
	var buf bytes.Buffer
	buf.WriteString("\x1b_G")
	fmt.Fprintf(&buf, "a=d,d=p,i=%d,p=0,q=2", p.HostImageID)
	buf.WriteString("\x1b\\")
	kp.pendingOutput = append(kp.pendingOutput, buf.Bytes()...)
}

// removeAllVisiblePlacements sends a single d=a command to delete ALL visible
// placements on screen while keeping image data in host memory. Much faster
// than per-image removePlacement when hiding everything (drag, overlays).
func (kp *KittyPassthrough) removeAllVisiblePlacements() {
	kp.pendingOutput = append(kp.pendingOutput, []byte("\x1b_Ga=d,d=a,q=2\x1b\\")...)
}

// deleteImageAndPlacements deletes both the image data AND all placements
// from the host terminal (d=i). Used for full cleanup (window close).
func (kp *KittyPassthrough) deleteImageAndPlacements(p *KittyPlacement) {
	var buf bytes.Buffer
	buf.WriteString("\x1b_G")
	fmt.Fprintf(&buf, "a=d,d=i,i=%d,q=2", p.HostImageID)
	buf.WriteString("\x1b\\")
	kp.pendingOutput = append(kp.pendingOutput, buf.Bytes()...)
}

func (kp *KittyPassthrough) placeOne(p *KittyPlacement) {
	cellH := kp.cellPixelH
	if cellH <= 0 {
		cellH = 20
	}
	cellW := kp.cellPixelW
	if cellW <= 0 {
		cellW = 10
	}

	kittyPassDbg("placeOne: hostID=%d at (%d,%d) rows=%d displayRows=%d maxShowable=%d clipTop=%d pixH=%d",
		p.HostImageID, p.HostX, p.HostY, p.Rows, p.DisplayRows, p.MaxShowable, p.ClipTop, p.PixelHeight)

	var buf bytes.Buffer
	// Position cursor at image location. No save/restore — BubbleTea does
	// a full screen rewrite each render cycle which repositions the cursor.
	fmt.Fprintf(&buf, "\x1b[%d;%dH", p.HostY+1, p.HostX+1)
	buf.WriteString("\x1b_G")
	fmt.Fprintf(&buf, "a=p,i=%d", p.HostImageID)
	if p.PlacementID > 0 {
		fmt.Fprintf(&buf, ",p=%d", p.PlacementID)
	}

	visibleRows := p.MaxShowable
	if visibleRows <= 0 {
		visibleRows = p.DisplayRows
	}
	if visibleRows <= 0 {
		visibleRows = p.Rows
	}
	if visibleRows <= 0 {
		visibleRows = 1
	}
	visibleCols := p.MaxShowableCols
	if visibleCols <= 0 {
		visibleCols = p.Cols
	}
	if visibleCols > 0 {
		fmt.Fprintf(&buf, ",c=%d", visibleCols)
	}
	if visibleRows > 0 {
		fmt.Fprintf(&buf, ",r=%d", visibleRows)
	}

	// Source rect clipping — only specify y=/h=/w= when actual clipping is
	// needed (image partially scrolled off top/bottom, or column-clipped).
	// When the full image is visible, omitting these lets the terminal scale
	// the full image to fit c/r cells, avoiding rounding-error distortion.
	needsVerticalClip := p.ClipTop > 0 || visibleRows < p.Rows
	needsHorizontalClip := visibleCols > 0 && visibleCols < p.Cols

	if p.SourceX > 0 {
		fmt.Fprintf(&buf, ",x=%d", p.SourceX)
	}

	if needsVerticalClip {
		// Compute pixels-per-row from actual image dimensions when available.
		pixelsPerRow := cellH
		if p.PixelHeight > 0 && p.Rows > 0 {
			pixelsPerRow = p.PixelHeight / p.Rows
		}
		sourceY := p.SourceY
		if p.ClipTop > 0 {
			sourceY += p.ClipTop * pixelsPerRow
		}
		sourceHeight := visibleRows * pixelsPerRow
		// Clamp so we don't exceed actual image pixel height.
		if p.PixelHeight > 0 && sourceY+sourceHeight > p.PixelHeight {
			sourceHeight = p.PixelHeight - sourceY
			if sourceHeight <= 0 {
				sourceHeight = 1
			}
		}
		if sourceY > 0 {
			fmt.Fprintf(&buf, ",y=%d", sourceY)
		}
		if sourceHeight > 0 {
			fmt.Fprintf(&buf, ",h=%d", sourceHeight)
		}
	}

	if needsHorizontalClip {
		pixelsPerCol := cellW
		if p.PixelWidth > 0 && p.Cols > 0 {
			pixelsPerCol = p.PixelWidth / p.Cols
		}
		sourceWidth := visibleCols * pixelsPerCol
		if sourceWidth > 0 {
			fmt.Fprintf(&buf, ",w=%d", sourceWidth)
		}
	} else if p.SourceWidth > 0 {
		// Preserve original source width if the app specified one.
		fmt.Fprintf(&buf, ",w=%d", p.SourceWidth)
	}

	// In passthrough mode, our renderer fills every cell with background color.
	// Images at z=0 (default, behind text) would be hidden behind the fill.
	// Force a positive z-index so images render above the text layer.
	zIdx := p.ZIndex
	if zIdx <= 0 {
		zIdx = 10
	}
	fmt.Fprintf(&buf, ",z=%d", zIdx)
	// C=1: do not move cursor after placement. Our frame renderer handles
	// all cursor positioning via tea.Raw — letting the terminal move the
	// cursor would corrupt the next frame's positioning.
	buf.WriteString(",C=1,q=2\x1b\\")
	kp.pendingOutput = append(kp.pendingOutput, buf.Bytes()...)
}

// RefreshAllPlacements updates image visibility and positions based on current window state.
func (kp *KittyPassthrough) RefreshAllPlacements(getWindows func() map[string]*KittyWindowInfo) {
	kp.mu.Lock()
	defer kp.mu.Unlock()

	if !kp.enabled || len(kp.placements) == 0 {
		return
	}

	allWindows := getWindows()

	// Hide all images if any window is being manipulated (animation).
	for windowID := range kp.placements {
		if info := allWindows[windowID]; info != nil && info.IsManipulated {
			hasVisible := false
			for _, placements := range kp.placements {
				for _, p := range placements {
					if !p.Hidden {
						p.Hidden = true
						hasVisible = true
					}
				}
			}
			if hasVisible {
				kp.removeAllVisiblePlacements()
			}
			return
		}
	}

	for windowID, placements := range kp.placements {
		info := allWindows[windowID]
		if info == nil {
			// Window gone — delete all its placements
			for _, p := range placements {
				if !p.Hidden {
					kp.deleteImageAndPlacements(p)
				}
			}
			delete(kp.placements, windowID)
			continue
		}

		viewportTop := info.ScrollbackLen - info.ScrollOffset
		viewportHeight := info.ContentHeight
		viewportWidth := info.ContentWidth

		var toDelete []uint32
		for hostID, p := range placements {
			// Alt screen mismatch
			if info.IsAltScreen != p.IsAltScreen {
				if !p.Hidden {
					kp.removePlacement(p)
					p.Hidden = true
				}
				if !info.IsAltScreen && p.IsAltScreen {
					toDelete = append(toDelete, hostID)
				}
				continue
			}

			relativeY := p.AbsoluteLine - viewportTop
			fullBottom := relativeY + p.Rows
			fullRight := p.GuestX + p.Cols

			visible := info.Visible &&
				relativeY < viewportHeight && fullBottom > 0 &&
				p.GuestX < viewportWidth && fullRight > 0

			clipTop := 0
			clipBottom := 0
			if visible {
				if relativeY < 0 {
					clipTop = -relativeY
				}
				if fullBottom > viewportHeight {
					clipBottom = fullBottom - viewportHeight
				}
			}

			maxRows := p.Rows - clipTop - clipBottom
			if maxRows <= 0 {
				maxRows = 1
			}
			if maxRows > viewportHeight {
				maxRows = viewportHeight
			}

			// Cap columns to viewport width (image may be wider than tiled window)
			maxCols := p.Cols
			if fullRight > viewportWidth {
				maxCols = viewportWidth - p.GuestX
			}
			if maxCols <= 0 {
				visible = false // completely clipped horizontally
			}
			if maxCols > viewportWidth {
				maxCols = viewportWidth
			}

			actualY := relativeY
			if clipTop > 0 {
				actualY = 0
			}
			newHostX := info.WindowX + info.ContentOffX + p.GuestX
			newHostY := info.WindowY + info.ContentOffY + actualY

			// Hide when position would be negative or window is off-screen
			if visible && (newHostX < 0 || newHostY < 0) {
				visible = false
			}
			if visible && (info.WindowX < 0 || info.WindowY < 0) {
				visible = false
			}

			posChanged := newHostX != p.HostX || newHostY != p.HostY || clipTop != p.ClipTop || maxRows != p.MaxShowable || maxCols != p.MaxShowableCols
			if posChanged || !visible != p.Hidden {
				kittyPassDbg("refresh: hostID=%d absLine=%d vpTop=%d relY=%d actualY=%d visible=%v hidden=%v clipTop=%d maxRows=%d oldPos=(%d,%d) newPos=(%d,%d) scrollback=%d scrollOff=%d vpH=%d posChanged=%v",
					hostID, p.AbsoluteLine, viewportTop, relativeY, actualY, visible, p.Hidden,
					clipTop, maxRows, p.HostX, p.HostY, newHostX, newHostY,
					info.ScrollbackLen, info.ScrollOffset, viewportHeight, posChanged)
			}

			if !visible {
				if !p.Hidden {
					kittyPassDbg("refresh: HIDE hostID=%d (fullBottom=%d vpH=%d)", hostID, fullBottom, viewportHeight)
					kp.removePlacement(p)
					p.Hidden = true
				}
			} else if p.Hidden {
				// Don't show until cursor injection scrolling is complete.
				// ReadyScrollback is the expected scrollbackLen after all
				// cursor injection newlines have been processed.
				if p.ReadyScrollback > 0 && info.ScrollbackLen < p.ReadyScrollback {
					kittyPassDbg("refresh: DEFER hostID=%d scrollback=%d < ready=%d",
						hostID, info.ScrollbackLen, p.ReadyScrollback)
					continue
				}
				// Was hidden, now visible — place it
				kittyPassDbg("refresh: SHOW hostID=%d at (%d,%d) scrollback=%d ready=%d",
					hostID, newHostX, newHostY, info.ScrollbackLen, p.ReadyScrollback)
				p.HostX = newHostX
				p.HostY = newHostY
				p.ClipTop = clipTop
				p.MaxShowable = maxRows
				p.MaxShowableCols = maxCols
				kp.placeOne(p)
				p.Hidden = false
			} else if newHostX != p.HostX || newHostY != p.HostY ||
				clipTop != p.ClipTop || maxRows != p.MaxShowable ||
				maxCols != p.MaxShowableCols {
				// Position or clipping changed — re-place atomically (no pre-delete).
				kittyPassDbg("refresh: MOVE hostID=%d from (%d,%d) to (%d,%d) clip %d→%d rows %d→%d",
					hostID, p.HostX, p.HostY, newHostX, newHostY, p.ClipTop, clipTop, p.MaxShowable, maxRows)
				p.HostX = newHostX
				p.HostY = newHostY
				p.ClipTop = clipTop
				p.MaxShowable = maxRows
				p.MaxShowableCols = maxCols
				kp.placeOne(p)
			}
			// else: position unchanged, do nothing
		}

		for _, id := range toDelete {
			delete(placements, id)
		}
	}
}

// HideAllPlacements temporarily hides all visible placements. Used when
// overlays (modals, dialogs, launcher) are active or during drag — images
// at z=10 render above the text layer and would cover UI overlays.
// Uses a single d=a command to remove all visible placements from screen
// while keeping image data in host memory for re-placement.
func (kp *KittyPassthrough) HideAllPlacements() {
	kp.mu.Lock()
	defer kp.mu.Unlock()
	hasVisible := false
	for _, placements := range kp.placements {
		for _, p := range placements {
			if !p.Hidden {
				p.Hidden = true
				hasVisible = true
			}
		}
	}
	if hasVisible {
		kp.removeAllVisiblePlacements()
	}
}

// ClearWindow removes all placements for a window (e.g., on close).
func (kp *KittyPassthrough) ClearWindow(windowID string) {
	kp.mu.Lock()
	defer kp.mu.Unlock()
	kp.deleteAllWindowPlacements(windowID)
	delete(kp.imageIDMap, windowID)
}

// HasPlacements returns true if any placements exist.
func (kp *KittyPassthrough) HasPlacements() bool {
	kp.mu.Lock()
	defer kp.mu.Unlock()
	for _, p := range kp.placements {
		if len(p) > 0 {
			return true
		}
	}
	return false
}

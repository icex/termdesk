package terminal

import (
	"bytes"
	"encoding/base64"
	"encoding/binary"
	"fmt"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/charmbracelet/x/term"
)

// imagePassDbg is a debug logger for sixel/iTerm2 image passthrough.
func imagePassDbg(format string, args ...any) {
	f, err := os.OpenFile("/tmp/termdesk-image.log", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return
	}
	defer f.Close()
	fmt.Fprintf(f, "[%s] ", time.Now().Format("15:04:05.000"))
	fmt.Fprintf(f, format+"\n", args...)
}

// ImageFormat identifies which image protocol a sequence uses.
type ImageFormat int

const (
	ImageFormatSixel  ImageFormat = iota
	ImageFormatIterm2
)

const maxImageSize = 4 * 1024 * 1024 // 4MB per image (base64 images can be large)

const maxImagePlacementsPerWindow = 10

// ImagePlacement tracks a placed image for re-rendering after frame redraws.
// Unlike Kitty graphics (which persist in a separate host terminal layer),
// sixel/iTerm2 images are inline pixel data that BT's frame renderer overwrites.
// We store the raw data and re-send after each frame (tmux-style).
type ImagePlacement struct {
	WindowID     string
	GuestX       int         // cursor X in guest terminal at placement time
	AbsoluteLine int         // scrollbackLen + cursorY at placement time
	RawData      []byte      // complete DCS/OSC sequence
	CellRows     int         // height in cell rows
	Format       ImageFormat
	IsAltScreen  bool
}

// ImageWindowInfo provides window state for image position recalculation.
type ImageWindowInfo struct {
	WindowX       int
	WindowY       int
	ContentOffX   int
	ContentOffY   int
	ContentWidth  int
	ContentHeight int
	Visible       bool
	ScrollbackLen int
	ScrollOffset  int
	IsAltScreen   bool
}

// ImagePassthrough manages forwarding sixel and iTerm2 inline image sequences
// from child PTY applications to the host terminal.
//
// Images are stored as placements and re-rendered after each BT frame redraw
// (tmux-style). This ensures images survive frame redraws, scroll with text,
// and stay hidden behind overlays. Refresh is event-driven (only on specific
// message types) rather than continuous — see Update wrapper in app_update.go.
type ImagePassthrough struct {
	mu            sync.Mutex
	sixelEnabled  bool
	iterm2Enabled bool
	pendingOutput []byte
	cellPixelW    int
	cellPixelH    int
	placements    map[string][]*ImagePlacement // windowID → placements
}

// detectIterm2 checks environment variables to determine iTerm2 support.
// Checks both TERMDESK_* vars (session child) and host terminal vars (direct run).
func detectIterm2() bool {
	// Session child process: BuildAppEnv sets these
	if os.Getenv("TERMDESK_ITERM2") == "1" {
		return true
	}
	if os.Getenv("TERMDESK_GRAPHICS") == "iterm2" {
		return true
	}
	// Direct run: check host terminal env vars
	switch os.Getenv("TERM_PROGRAM") {
	case "iTerm.app", "WezTerm":
		return true
	}
	if os.Getenv("LC_TERMINAL") == "iTerm2" {
		return true
	}
	if os.Getenv("ITERM_SESSION_ID") != "" {
		return true
	}
	if os.Getenv("WEZTERM_PANE") != "" {
		return true
	}
	return false
}

// detectSixel checks environment variables to determine sixel support.
// Checks both TERMDESK_* vars (session child) and host terminal vars (direct run).
func detectSixel() bool {
	// Session child process: BuildAppEnv sets these
	if os.Getenv("TERMDESK_SIXEL") == "1" {
		return true
	}
	if os.Getenv("TERMDESK_GRAPHICS") == "sixel" {
		return true
	}
	// Direct run: check host terminal env vars
	switch os.Getenv("TERM_PROGRAM") {
	case "WezTerm", "ghostty", "foot", "contour", "mlterm", "iTerm.app":
		return true
	}
	if os.Getenv("XTERM_VERSION") != "" {
		return true
	}
	return false
}

// NewImagePassthrough creates a new image passthrough based on detected protocol.
// iTerm2 can be enabled alongside Kitty since they use different sequence types
// (APC vs OSC). Sixel can be enabled alongside Kitty (DCS vs APC).
func NewImagePassthrough(cellPixelW, cellPixelH int) *ImagePassthrough {
	iterm2 := detectIterm2()
	sixel := detectSixel()
	imagePassDbg("NewImagePassthrough: iterm2=%v sixel=%v cellPx=%dx%d TERMDESK_GRAPHICS=%q TERM_PROGRAM=%q",
		iterm2, sixel, cellPixelW, cellPixelH, os.Getenv("TERMDESK_GRAPHICS"), os.Getenv("TERM_PROGRAM"))
	return &ImagePassthrough{
		sixelEnabled:  sixel,
		iterm2Enabled: iterm2,
		cellPixelW:    cellPixelW,
		cellPixelH:    cellPixelH,
		placements:    make(map[string][]*ImagePlacement),
	}
}

// SixelEnabled returns whether sixel passthrough is active.
func (ip *ImagePassthrough) SixelEnabled() bool {
	ip.mu.Lock()
	defer ip.mu.Unlock()
	return ip.sixelEnabled
}

// Iterm2Enabled returns whether iTerm2 inline image passthrough is active.
func (ip *ImagePassthrough) Iterm2Enabled() bool {
	ip.mu.Lock()
	defer ip.mu.Unlock()
	return ip.iterm2Enabled
}

// IsEnabled returns whether any image protocol passthrough is active.
func (ip *ImagePassthrough) IsEnabled() bool {
	ip.mu.Lock()
	defer ip.mu.Unlock()
	return ip.sixelEnabled || ip.iterm2Enabled
}

// FlushPending returns and clears accumulated output bytes.
func (ip *ImagePassthrough) FlushPending() []byte {
	ip.mu.Lock()
	defer ip.mu.Unlock()
	if len(ip.pendingOutput) == 0 {
		return nil
	}
	out := ip.pendingOutput
	ip.pendingOutput = nil
	return out
}

// ForwardImage stores an image placement for re-rendering after each frame.
// The raw data is NOT sent immediately — RefreshAllImages handles rendering
// after BT's frame render (tmux-style). This ensures images survive frame
// redraws, scroll with text, and hide behind overlays.
// Returns the number of cell rows the image occupies (for cursor advancement).
func (ip *ImagePassthrough) ForwardImage(
	rawData []byte,
	format ImageFormat,
	windowID string,
	guestX, guestY int,
	scrollbackLen int,
	isAltScreen bool,
	cellRows int,
) int {
	ip.mu.Lock()
	defer ip.mu.Unlock()

	if format == ImageFormatSixel && !ip.sixelEnabled {
		return 0
	}
	if format == ImageFormatIterm2 && !ip.iterm2Enabled {
		return 0
	}
	if len(rawData) > maxImageSize {
		imagePassDbg("ForwardImage: REJECTED size=%d > max=%d", len(rawData), maxImageSize)
		return 0
	}
	if cellRows <= 0 {
		return 0
	}

	// Store a copy of the raw data (the caller's buffer may be reused).
	dataCopy := make([]byte, len(rawData))
	copy(dataCopy, rawData)

	placement := &ImagePlacement{
		WindowID:     windowID,
		GuestX:       guestX,
		AbsoluteLine: scrollbackLen + guestY,
		RawData:      dataCopy,
		CellRows:     cellRows,
		Format:       format,
		IsAltScreen:  isAltScreen,
	}

	imagePassDbg("ForwardImage: STORE window=%s format=%d guestPos=(%d,%d) absLine=%d cellRows=%d rawLen=%d",
		windowID, format, guestX, guestY, placement.AbsoluteLine, cellRows, len(dataCopy))

	// For iTerm2 on alt screen, replace ALL existing placements for this
	// window. Apps like yazi replace the preview image when the user selects
	// a different file — keeping old placements causes ghosting.
	wps := ip.placements[windowID]
	if format == ImageFormatIterm2 && isAltScreen {
		wps = nil
	}

	// Evict oldest placement if at capacity.
	if len(wps) >= maxImagePlacementsPerWindow {
		ip.placements[windowID] = append(wps[1:], placement)
	} else {
		ip.placements[windowID] = append(wps, placement)
	}

	return cellRows
}

// RefreshAllImages re-renders all stored image placements at their current
// host terminal positions. Called after each dirty frame (via deferred
// ImageRefreshMsg) or on specific events. Removes placements that have
// scrolled completely off screen. Images that extend below the content
// area are truncated (sixel) or clipped (iTerm2) rather than skipped.
func (ip *ImagePassthrough) RefreshAllImages(hostTermHeight int, getWindows func() map[string]*ImageWindowInfo) {
	ip.mu.Lock()
	defer ip.mu.Unlock()

	if len(ip.placements) == 0 {
		return
	}

	allWindows := getWindows()
	rendered := 0

	for windowID, placements := range ip.placements {
		info := allWindows[windowID]
		if info == nil {
			// Window gone — remove all its placements
			delete(ip.placements, windowID)
			continue
		}

		viewportTop := info.ScrollbackLen - info.ScrollOffset
		viewportHeight := info.ContentHeight
		viewportWidth := info.ContentWidth

		var kept []*ImagePlacement
		for _, p := range placements {
			// Alt screen mismatch — hide but keep (might match later)
			if info.IsAltScreen != p.IsAltScreen {
				kept = append(kept, p)
				continue
			}

			relativeY := p.AbsoluteLine - viewportTop
			fullBottom := relativeY + p.CellRows

			// Scrolled way off top — GC permanently
			if fullBottom <= -viewportHeight {
				imagePassDbg("RefreshAllImages: GC window=%s absLine=%d (scrolled off)", windowID, p.AbsoluteLine)
				continue
			}

			kept = append(kept, p)

			// Check visibility: within viewport bounds
			visible := info.Visible &&
				relativeY < viewportHeight && fullBottom > 0 &&
				p.GuestX >= 0 && p.GuestX < viewportWidth

			if !visible {
				imagePassDbg("RefreshAllImages: SKIP window=%s absLine=%d relY=%d visible=%v (vis=%v relY<vpH=%v fullBot>0=%v gX=%d vpW=%d)",
					windowID, p.AbsoluteLine, relativeY, visible,
					info.Visible, relativeY < viewportHeight, fullBottom > 0,
					p.GuestX, viewportWidth)
				continue
			}

			// Compute host coordinates
			hostX := info.WindowX + info.ContentOffX + p.GuestX
			hostY := info.WindowY + info.ContentOffY + relativeY

			// Clamp: if image extends above content area, skip
			// (truncating leading sixel bands is unreliable)
			contentTop := info.WindowY + info.ContentOffY
			if hostY < contentTop {
				imagePassDbg("RefreshAllImages: CLIP-TOP window=%s hostY=%d < contentTop=%d", windowID, hostY, contentTop)
				continue
			}

			// Clamp content bottom against host terminal height.
			contentBottom := info.WindowY + info.ContentOffY + viewportHeight - 1
			if hostTermHeight > 0 && contentBottom >= hostTermHeight {
				contentBottom = hostTermHeight - 1
			}

			renderData := p.RawData
			if p.Format == ImageFormatSixel {
				// Sixel rendering moves the cursor below the last band.
				// If that position is past the terminal bottom, the
				// terminal scrolls and corrupts the viewport. We compute
				// the maximum safe pixel height from hostY to the bottom,
				// subtract one full cell row for the post-sixel cursor,
				// and truncate bands to fit. truncateSixelBands is a
				// no-op when the image already fits.
				cpH := ip.cellPixelH
				if cpH <= 0 {
					cpH = 16
				}

				// Maximum rows the image can occupy: leave 1 row for
				// post-sixel cursor, and never extend past contentBottom.
				safeBottom := contentBottom
				if hostTermHeight > 0 && safeBottom >= hostTermHeight-1 {
					safeBottom = hostTermHeight - 2
				}
				availableRows := safeBottom - hostY + 1
				if availableRows <= 0 {
					imagePassDbg("RefreshAllImages: SKIP-BOTTOM window=%s hostY=%d safeBottom=%d hostH=%d",
						windowID, hostY, safeBottom, hostTermHeight)
					continue
				}

				// Convert available rows to sixel bands, minus 1 band
				// as extra safety margin for sub-cell-row alignment.
				maxBands := (availableRows * cpH) / 6
				if maxBands > 1 {
					maxBands-- // safety margin
				}
				if maxBands <= 0 {
					imagePassDbg("RefreshAllImages: SKIP-BANDS window=%s maxBands=0", windowID)
					continue
				}
				truncated := truncateSixelBands(renderData, maxBands)
				if truncated == nil {
					imagePassDbg("RefreshAllImages: TRUNCATE-FAIL window=%s bands=%d", windowID, maxBands)
					continue
				}
				renderData = truncated
				imagePassDbg("RefreshAllImages: RENDER window=%s hostY=%d avail=%d maxBands=%d cpH=%d safeBot=%d hostH=%d dataLen=%d",
					windowID, hostY, availableRows, maxBands, cpH, safeBottom, hostTermHeight, len(renderData))
			} else if p.Format == ImageFormatIterm2 {
				// iTerm2: constrain width and height to window content
				// area so the host terminal doesn't render beyond the
				// window boundaries (which would scroll and corrupt
				// the composited viewport).
				safeBottom := contentBottom
				if hostTermHeight > 0 && safeBottom >= hostTermHeight-1 {
					safeBottom = hostTermHeight - 2 // leave 1 row to prevent scroll
				}
				availableRows := safeBottom - hostY + 1
				if availableRows <= 0 {
					continue
				}
				// Also clamp against absolute host terminal bottom.
				// iTerm2 advances the cursor after rendering, so we
				// need room for the cursor below the image or the
				// terminal scrolls.
				if hostTermHeight > 0 {
					maxFromHost := hostTermHeight - hostY - 2
					if maxFromHost < availableRows {
						availableRows = maxFromHost
					}
					if availableRows <= 0 {
						continue
					}
				}
				renderData = constrainIterm2Width(renderData, viewportWidth, ip.cellPixelW)
				// Always set explicit height to prevent overflow.
				// Use the smaller of CellRows (estimated image height)
				// and availableRows (space to bottom of window).
				maxH := p.CellRows
				if availableRows < maxH {
					maxH = availableRows
				}
				if maxH <= 0 {
					continue
				}
				renderData = constrainIterm2Height(renderData, maxH)
			}

			// Render: position cursor, emit image. No save/restore —
			// BubbleTea does a full screen rewrite each render cycle
			// which repositions the cursor. Using DECSC/DECRC (\x1b7/\x1b8)
			// conflicts with BT's internal cursor state.
			pos := fmt.Sprintf("\x1b[%d;%dH", hostY+1, hostX+1)
			ip.pendingOutput = append(ip.pendingOutput, pos...)
			ip.pendingOutput = append(ip.pendingOutput, renderData...)
			rendered++
		}

		if len(kept) == 0 {
			delete(ip.placements, windowID)
		} else {
			ip.placements[windowID] = kept
		}
	}

	if rendered > 0 {
		imagePassDbg("RefreshAllImages: rendered %d placements, pendingLen=%d", rendered, len(ip.pendingOutput))
	}
}

// HasImagePlacements returns whether any image placements are stored.
func (ip *ImagePassthrough) HasImagePlacements() bool {
	ip.mu.Lock()
	defer ip.mu.Unlock()
	return len(ip.placements) > 0
}


// ForwardRawSequence forwards a raw image sequence (sixel DCS or iTerm2 OSC)
// directly to the host terminal without cursor positioning or placement storage.
func (ip *ImagePassthrough) ForwardRawSequence(rawData []byte) {
	ip.mu.Lock()
	defer ip.mu.Unlock()
	if len(rawData) > maxImageSize {
		return
	}
	if !ip.iterm2Enabled && !ip.sixelEnabled {
		return
	}
	imagePassDbg("ForwardRawSequence: len=%d", len(rawData))
	ip.pendingOutput = append(ip.pendingOutput, rawData...)
}

// ClearWindow removes all stored image placements for a window.
func (ip *ImagePassthrough) ClearWindow(windowID string) bool {
	ip.mu.Lock()
	defer ip.mu.Unlock()
	if _, ok := ip.placements[windowID]; ok {
		imagePassDbg("ClearWindow: removing placements for window=%s", windowID)
		delete(ip.placements, windowID)
		return true
	}
	return false
}

// estimateIterm2CellRows parses an iTerm2 File= OSC to estimate the image
// height in cell rows. Extracts pixel dimensions from the image header
// (PNG/JPEG/GIF) in the base64 data, then computes rows based on the
// terminal content width and cell pixel size.
// Returns 0 if dimensions cannot be determined.
func estimateIterm2CellRows(rawOSC []byte, contentCols, cellPixelW, cellPixelH int) int {
	if cellPixelH <= 0 {
		cellPixelH = 20
	}
	if cellPixelW <= 0 {
		cellPixelW = 10
	}
	if contentCols <= 0 {
		contentCols = 80
	}

	// Find the ':' separator between params and base64 data.
	colonIdx := bytes.IndexByte(rawOSC, ':')
	if colonIdx < 0 || colonIdx+1 >= len(rawOSC) {
		return 0
	}

	// Parse width/height params from File= header.
	// Params are between "File=" and ":"
	paramWidth := ""
	paramHeight := ""
	fileIdx := bytes.Index(rawOSC, []byte("File="))
	if fileIdx >= 0 {
		paramStr := string(rawOSC[fileIdx+5 : colonIdx])
		for _, param := range strings.Split(paramStr, ";") {
			k, v, ok := strings.Cut(param, "=")
			if !ok {
				continue
			}
			switch k {
			case "width":
				paramWidth = v
			case "height":
				paramHeight = v
			}
		}
	}

	// Check if explicit cell-based dimensions are given.
	if paramWidth != "" && paramWidth != "auto" && paramHeight != "" && paramHeight != "auto" {
		// Try to parse as plain number (character cells).
		h, err := strconv.Atoi(paramHeight)
		if err == nil && h > 0 {
			return h
		}
	}

	// Need pixel dimensions from the image header.
	// PNG/GIF need ~24 bytes; JPEG needs more (SOF marker can be at byte
	// 200+ after EXIF/APP segments). Decode up to 1024 raw bytes.
	b64Data := rawOSC[colonIdx+1:]
	// Strip trailing BEL (\x07) or ST (\x1b\\)
	if len(b64Data) > 0 && b64Data[len(b64Data)-1] == '\x07' {
		b64Data = b64Data[:len(b64Data)-1]
	} else if len(b64Data) > 1 && b64Data[len(b64Data)-2] == '\x1b' && b64Data[len(b64Data)-1] == '\\' {
		b64Data = b64Data[:len(b64Data)-2]
	}

	headerB64 := b64Data
	if len(headerB64) > 1368 { // 1368 base64 chars = 1024 raw bytes
		headerB64 = headerB64[:1368]
	}
	// Round down to multiple of 4 for valid base64
	headerB64 = headerB64[:len(headerB64)/4*4]
	header, err := base64.StdEncoding.DecodeString(string(headerB64))
	if err != nil {
		header, err = base64.RawStdEncoding.DecodeString(string(headerB64))
		if err != nil {
			return 0
		}
	}

	imgW, imgH := parseImageDimensions(header)
	imagePassDbg("estimateIterm2CellRows: headerLen=%d imgParsed=%dx%d magic=%x",
		len(header), imgW, imgH, func() byte { if len(header) > 0 { return header[0] }; return 0 }())
	if imgW <= 0 || imgH <= 0 {
		return 0
	}

	// Compute display dimensions.
	// Default (width=auto): iTerm2 renders at native pixel size, capped
	// to content width if the image overflows.
	displayW := imgW
	maxW := contentCols * cellPixelW
	if displayW > maxW {
		displayW = maxW
	}
	displayH := imgH * displayW / imgW

	// If explicit width is given, use it.
	if paramWidth != "" && paramWidth != "auto" {
		if pw, err := strconv.Atoi(paramWidth); err == nil && pw > 0 {
			displayW = pw * cellPixelW
			displayH = imgH * displayW / imgW
		} else if strings.HasSuffix(paramWidth, "px") {
			if pw, err := strconv.Atoi(paramWidth[:len(paramWidth)-2]); err == nil && pw > 0 {
				displayW = pw
				displayH = imgH * displayW / imgW
			}
		}
	}
	// If explicit height, use it.
	if paramHeight != "" && paramHeight != "auto" {
		if ph, err := strconv.Atoi(paramHeight); err == nil && ph > 0 {
			displayH = ph * cellPixelH
		} else if strings.HasSuffix(paramHeight, "px") {
			if ph, err := strconv.Atoi(paramHeight[:len(paramHeight)-2]); err == nil && ph > 0 {
				displayH = ph
			}
		}
	}

	cellRows := (displayH + cellPixelH - 1) / cellPixelH
	if cellRows <= 0 {
		cellRows = 1
	}

	imagePassDbg("estimateIterm2CellRows: imgPx=%dx%d displayPx=%dx%d cellRows=%d paramW=%q paramH=%q",
		imgW, imgH, displayW, displayH, cellRows, paramWidth, paramHeight)
	return cellRows
}

// parseImageDimensions extracts width and height from a PNG, JPEG, or GIF header.
func parseImageDimensions(header []byte) (width, height int) {
	if len(header) >= 24 && string(header[1:4]) == "PNG" {
		// PNG: IHDR chunk at offset 16, width at 16, height at 20 (big-endian uint32)
		w := binary.BigEndian.Uint32(header[16:20])
		h := binary.BigEndian.Uint32(header[20:24])
		return int(w), int(h)
	}
	if len(header) >= 10 && string(header[0:3]) == "GIF" {
		// GIF: width at 6, height at 8 (little-endian uint16)
		w := binary.LittleEndian.Uint16(header[6:8])
		h := binary.LittleEndian.Uint16(header[8:10])
		return int(w), int(h)
	}
	if len(header) >= 2 && header[0] == 0xFF && header[1] == 0xD8 {
		// JPEG: scan for SOF marker (0xFF 0xC0..0xCF, except 0xC4/0xC8/0xCC)
		for i := 2; i+9 < len(header); {
			if header[i] != 0xFF {
				i++
				continue
			}
			marker := header[i+1]
			if marker == 0xFF {
				i++
				continue
			}
			if marker == 0xD9 { // EOI
				break
			}
			if marker >= 0xC0 && marker <= 0xCF &&
				marker != 0xC4 && marker != 0xC8 && marker != 0xCC {
				// SOF: height at i+5, width at i+7 (big-endian uint16)
				h := binary.BigEndian.Uint16(header[i+5 : i+7])
				w := binary.BigEndian.Uint16(header[i+7 : i+9])
				return int(w), int(h)
			}
			// Skip to next marker
			if i+3 < len(header) {
				segLen := int(binary.BigEndian.Uint16(header[i+2 : i+4]))
				i += 2 + segLen
			} else {
				break
			}
		}
	}
	return 0, 0
}

// constrainIterm2Height injects a height=N param into an iTerm2 File= OSC
// sequence to prevent the image from extending past the available rows.
func constrainIterm2Height(rawData []byte, maxRows int) []byte {
	if maxRows <= 0 {
		return rawData
	}

	fileIdx := bytes.Index(rawData, []byte("File="))
	if fileIdx < 0 {
		fileIdx = bytes.Index(rawData, []byte("MultipartFile="))
		if fileIdx < 0 {
			return rawData
		}
	}
	colonIdx := bytes.IndexByte(rawData[fileIdx:], ':')
	if colonIdx < 0 {
		return rawData
	}
	colonIdx += fileIdx

	eqIdx := bytes.IndexByte(rawData[fileIdx:colonIdx], '=')
	if eqIdx < 0 {
		return rawData
	}
	paramStart := fileIdx + eqIdx + 1
	paramStr := string(rawData[paramStart:colonIdx])

	// Replace or add height param. With preserveAspectRatio=1 (default),
	// iTerm2 fits the image within both width and height constraints
	// while maintaining aspect ratio — no stretching.
	params := strings.Split(paramStr, ";")
	found := false
	h := fmt.Sprintf("%d", maxRows)
	for i, p := range params {
		k, _, ok := strings.Cut(p, "=")
		if ok && k == "height" {
			params[i] = "height=" + h
			found = true
		}
	}
	if !found {
		params = append(params, "height="+h)
	}

	var buf bytes.Buffer
	buf.Write(rawData[:paramStart])
	buf.WriteString(strings.Join(params, ";"))
	buf.Write(rawData[colonIdx:])
	return buf.Bytes()
}

// constrainIterm2Width rewrites an iTerm2 File= OSC sequence to cap the
// width to contentWidthCells if the image's native pixel width would
// overflow. If the image fits naturally, the data is returned unchanged
// so iTerm2 renders at native resolution (no upscaling).
func constrainIterm2Width(rawData []byte, contentWidthCells, cellPixelW int) []byte {
	if contentWidthCells <= 0 || cellPixelW <= 0 {
		return rawData
	}

	// Find the "File=" prefix and the ":" separator between params and data.
	fileIdx := bytes.Index(rawData, []byte("File="))
	if fileIdx < 0 {
		fileIdx = bytes.Index(rawData, []byte("MultipartFile="))
		if fileIdx < 0 {
			return rawData
		}
	}

	colonIdx := bytes.IndexByte(rawData[fileIdx:], ':')
	if colonIdx < 0 {
		return rawData
	}
	colonIdx += fileIdx // absolute index

	// Extract the params string (between File= or MultipartFile= and :)
	eqIdx := bytes.IndexByte(rawData[fileIdx:colonIdx], '=')
	if eqIdx < 0 {
		return rawData
	}
	paramStart := fileIdx + eqIdx + 1
	paramStr := string(rawData[paramStart:colonIdx])

	// Check if explicit width is already set by the sender.
	params := strings.Split(paramStr, ";")
	for _, p := range params {
		k, v, ok := strings.Cut(p, "=")
		if ok && k == "width" && v != "" && v != "auto" {
			// Sender specified explicit width — respect it.
			imagePassDbg("constrainIterm2Width: explicit width=%q, not modifying", v)
			return rawData
		}
	}

	// Decode image header to get native pixel width.
	// JPEG needs more bytes (SOF marker can be past byte 200).
	b64Data := rawData[colonIdx+1:]
	if len(b64Data) > 0 && b64Data[len(b64Data)-1] == '\x07' {
		b64Data = b64Data[:len(b64Data)-1]
	} else if len(b64Data) > 1 && b64Data[len(b64Data)-2] == '\x1b' && b64Data[len(b64Data)-1] == '\\' {
		b64Data = b64Data[:len(b64Data)-2]
	}
	headerB64 := b64Data
	if len(headerB64) > 1368 { // 1024 raw bytes
		headerB64 = headerB64[:1368]
	}
	headerB64 = headerB64[:len(headerB64)/4*4]
	header, err := base64.StdEncoding.DecodeString(string(headerB64))
	if err != nil {
		header, err = base64.RawStdEncoding.DecodeString(string(headerB64))
		if err != nil {
			return rawData
		}
	}
	imgW, _ := parseImageDimensions(header)
	if imgW <= 0 {
		return rawData
	}

	// Compute native width in cells. If it fits, don't constrain.
	nativeCols := (imgW + cellPixelW - 1) / cellPixelW
	if nativeCols <= contentWidthCells {
		imagePassDbg("constrainIterm2Width: nativeCols=%d fits in contentCols=%d, no constraint", nativeCols, contentWidthCells)
		return rawData
	}

	// Image overflows — cap to content width.
	newWidth := fmt.Sprintf("%d", contentWidthCells)
	params = append(params, "width="+newWidth)

	var buf bytes.Buffer
	buf.Write(rawData[:paramStart])
	buf.WriteString(strings.Join(params, ";"))
	buf.Write(rawData[colonIdx:])
	imagePassDbg("constrainIterm2Width: nativeCols=%d > contentCols=%d, set width=%s", nativeCols, contentWidthCells, newWidth)
	return buf.Bytes()
}

// --- Sixel DCS extraction (Terminal methods) ---

// sixelDCSSegment represents a sixel DCS sequence found in PTY output.
type sixelDCSSegment struct {
	DataBefore []byte
	RawDCS     []byte // complete ESC P ... ESC \ sequence
	PixelRows  int    // estimated from raster attributes or '-' count
}

// extractSixelDCS scans data for sixel DCS sequences.
// Supports both 7-bit DCS (ESC P) and 8-bit DCS (0x90).
// Format: DCS [params] q [raster-attrs] [sixel-data] ST
// Returns segments and trailing non-sixel data. Handles sequences that span
// buffer boundaries via sixelDCSBuf.
func (t *Terminal) extractSixelDCS(data []byte) (segments []sixelDCSSegment, trailing []byte) {
	if len(t.sixelDCSBuf) > 0 {
		combined := make([]byte, len(t.sixelDCSBuf)+len(data))
		copy(combined, t.sixelDCSBuf)
		copy(combined[len(t.sixelDCSBuf):], data)
		data = combined
		t.sixelDCSBuf = t.sixelDCSBuf[:0]
	}

	// Fast path: no ESC and no 8-bit DCS (0x90) means no DCS sequences.
	if bytes.IndexByte(data, '\x1b') < 0 && bytes.IndexByte(data, 0x90) < 0 {
		return nil, data
	}

	i := 0
	lastCopy := 0

	for i < len(data) {
		// Look for DCS introducer: ESC P (7-bit) or 0x90 (8-bit C1)
		isDCS7 := data[i] == '\x1b' && i+1 < len(data) && data[i+1] == 'P'
		isDCS8 := data[i] == 0x90
		if !isDCS7 && !isDCS8 {
			i++
			continue
		}

		dcsStart := i
		j := i + 2 // 7-bit: skip ESC P
		if isDCS8 {
			j = i + 1 // 8-bit: skip only 0x90
		}

		// Skip optional params (digits and semicolons)
		for j < len(data) && ((data[j] >= '0' && data[j] <= '9') || data[j] == ';') {
			j++
		}

		// Must have 'q' as sixel data introducer
		if j >= len(data) {
			// Incomplete — buffer remainder
			if len(data[dcsStart:]) <= maxImageSize {
				t.sixelDCSBuf = append(t.sixelDCSBuf[:0], data[dcsStart:]...)
			}
			trailing = data[lastCopy:dcsStart]
			return
		}
		if data[j] != 'q' {
			i = j
			continue
		}
		sixelDataStart := j + 1

		// Find ST: ESC \ (7-bit) or 0x9C (8-bit C1)
		found := false
		for k := sixelDataStart; k < len(data); k++ {
			var rawDCS []byte
			if data[k] == '\x1b' && k+1 < len(data) && data[k+1] == '\\' {
				rawDCS = data[dcsStart : k+2] // 7-bit ST
			} else if data[k] == 0x9c {
				rawDCS = data[dcsStart : k+1] // 8-bit ST
			}
			if rawDCS != nil {
				sixelData := data[sixelDataStart:k]
				pixelRows := parseSixelHeight(sixelData)

				imagePassDbg("extractSixelDCS: found sixel seq len=%d pixelRows=%d", len(rawDCS), pixelRows)

				seg := sixelDCSSegment{
					DataBefore: data[lastCopy:dcsStart],
					RawDCS:     rawDCS,
					PixelRows:  pixelRows,
				}
				segments = append(segments, seg)
				if data[k] == 0x9c {
					i = k + 1
				} else {
					i = k + 2
				}
				lastCopy = i
				found = true
				break
			}
		}
		if !found {
			// Incomplete DCS — buffer from start (if within size limit)
			if len(data[dcsStart:]) <= maxImageSize {
				t.sixelDCSBuf = append(t.sixelDCSBuf[:0], data[dcsStart:]...)
				imagePassDbg("extractSixelDCS: buffering incomplete DCS (%d bytes)", len(data[dcsStart:]))
			} else {
				imagePassDbg("extractSixelDCS: dropping oversized DCS (%d bytes > %d)", len(data[dcsStart:]), maxImageSize)
			}
			trailing = data[lastCopy:dcsStart]
			return
		}
	}

	trailing = data[lastCopy:]
	return
}

// parseSixelHeight extracts pixel height from sixel data.
// First tries raster attributes ("Pan;Pad;Ph;Pv), then falls back to counting
// '-' characters (each is a sixel newline = 6 pixel rows).
func parseSixelHeight(sixelData []byte) int {
	// Try raster attributes: starts with '"' after 'q'
	if len(sixelData) > 0 && sixelData[0] == '"' {
		rasterEnd := 1
		for rasterEnd < len(sixelData) {
			b := sixelData[rasterEnd]
			// Stop at first sixel data character (color #, repeat !, CR $, LF -, or pixel data ?-~)
			if b == '#' || b == '!' || b == '$' || b == '-' || (b >= '?' && b <= '~') {
				break
			}
			rasterEnd++
		}
		rasterStr := string(sixelData[1:rasterEnd])
		parts := strings.Split(rasterStr, ";")
		if len(parts) >= 4 {
			if pv, err := strconv.Atoi(parts[3]); err == nil && pv > 0 {
				return pv // exact height from raster attributes
			}
		}
	}

	// Fallback: count '-' (sixel newline = 6 pixel rows)
	dashes := bytes.Count(sixelData, []byte{'-'})
	return (dashes + 1) * 6
}

// truncateSixelBands truncates a sixel DCS sequence to show at most maxBands
// bands (each band is 6 pixels high). Returns the truncated DCS with proper
// ST terminator, or nil if truncation is not possible.
func truncateSixelBands(rawDCS []byte, maxBands int) []byte {
	if maxBands <= 0 || len(rawDCS) < 4 {
		return nil
	}

	// Find 'q' (sixel data start) — after DCS introducer and optional params
	start := 0
	if rawDCS[0] == '\x1b' && len(rawDCS) > 1 && rawDCS[1] == 'P' {
		start = 2 // 7-bit DCS
	} else if rawDCS[0] == 0x90 {
		start = 1 // 8-bit DCS
	} else {
		return nil
	}

	// Skip params (digits, semicolons)
	for start < len(rawDCS) && ((rawDCS[start] >= '0' && rawDCS[start] <= '9') || rawDCS[start] == ';') {
		start++
	}

	if start >= len(rawDCS) || rawDCS[start] != 'q' {
		return nil
	}

	// Determine where ST starts in the original data
	stStart := len(rawDCS)
	var st []byte
	if len(rawDCS) >= 2 && rawDCS[len(rawDCS)-1] == '\\' && rawDCS[len(rawDCS)-2] == '\x1b' {
		stStart = len(rawDCS) - 2
		st = []byte("\x1b\\")
	} else if rawDCS[len(rawDCS)-1] == 0x9c {
		stStart = len(rawDCS) - 1
		st = []byte{0x9c}
	} else {
		st = []byte("\x1b\\")
	}

	// Count '-' characters (sixel newlines) in the sixel data portion.
	// Band 1: data before first '-'. Band 2: after first '-'. Etc.
	// To keep maxBands bands, we need (maxBands - 1) '-' chars.
	// Truncate at the maxBands-th '-' (which would start band maxBands+1).
	sixelDataStart := start + 1
	dashes := 0
	for i := sixelDataStart; i < stStart; i++ {
		if rawDCS[i] == '-' {
			dashes++
			if dashes == maxBands {
				// Truncate here: keep everything before this dash, add ST
				result := make([]byte, 0, i+len(st))
				result = append(result, rawDCS[:i]...)
				result = append(result, st...)
				return result
			}
		}
	}

	// Image has ≤ maxBands bands — no truncation needed
	return rawDCS
}

// --- iTerm2 OSC extraction (Terminal methods) ---

// iterm2OSCSegment represents an iTerm2 inline image OSC found in PTY output.
type iterm2OSCSegment struct {
	DataBefore  []byte
	RawOSC      []byte // complete ESC ] 1337;File=... BEL/ST sequence
	IsMultipart bool   // true for FilePart/FileEnd (no cursor injection needed)
}

// iTerm2 file-related OSC prefixes. We intercept all of these:
// - File=          single-shot inline image
// - MultipartFile= chunked transfer start
// - FilePart=      continuation chunk
// - FileEnd        end marker
var iterm2FilePrefixes = [][]byte{
	[]byte("\x1b]1337;File="),
	[]byte("\x1b]1337;MultipartFile="),
	[]byte("\x1b]1337;FilePart="),
	[]byte("\x1b]1337;FileEnd"),
}

// extractIterm2OSC scans data for iTerm2 inline image OSC sequences.
// Handles single-shot (File=) and multipart (MultipartFile/FilePart/FileEnd).
func (t *Terminal) extractIterm2OSC(data []byte) (segments []iterm2OSCSegment, trailing []byte) {
	if len(t.iterm2OSCBuf) > 0 {
		combined := make([]byte, len(t.iterm2OSCBuf)+len(data))
		copy(combined, t.iterm2OSCBuf)
		copy(combined[len(t.iterm2OSCBuf):], data)
		data = combined
		t.iterm2OSCBuf = t.iterm2OSCBuf[:0]
	}

	// Fast path: check for the OSC 1337 prefix
	if !bytes.Contains(data, []byte("\x1b]1337")) {
		return nil, data
	}

	i := 0
	lastCopy := 0

	for i < len(data) {
		// Try each known iTerm2 file prefix
		bestIdx := -1
		bestPrefix := -1
		bestPrefixLen := 0
		for pidx, prefix := range iterm2FilePrefixes {
			idx := bytes.Index(data[i:], prefix)
			if idx >= 0 && (bestIdx < 0 || idx < bestIdx) {
				bestIdx = idx
				bestPrefix = pidx
				bestPrefixLen = len(prefix)
			}
		}
		if bestIdx < 0 {
			break
		}
		oscStart := i + bestIdx

		// Find terminator: BEL (\x07) or ST (ESC \)
		found := false
		for k := oscStart + bestPrefixLen; k < len(data); k++ {
			terminated := false
			end := k
			if data[k] == '\x07' {
				terminated = true
				end = k + 1
			} else if data[k] == '\x1b' && k+1 < len(data) && data[k+1] == '\\' {
				terminated = true
				end = k + 2
			}
			if terminated {
				rawOSC := data[oscStart:end]
				isMultipart := bestPrefix >= 2 // FilePart or FileEnd

				imagePassDbg("extractIterm2OSC: prefix=%d len=%d multipart=%v",
					bestPrefix, len(rawOSC), isMultipart)

				seg := iterm2OSCSegment{
					DataBefore:  data[lastCopy:oscStart],
					RawOSC:      rawOSC,
					IsMultipart: isMultipart,
				}
				segments = append(segments, seg)
				i = end
				lastCopy = end
				found = true
				break
			}
		}
		if !found {
			// Incomplete OSC — buffer (if within size limit)
			if len(data[oscStart:]) <= maxImageSize {
				t.iterm2OSCBuf = append(t.iterm2OSCBuf[:0], data[oscStart:]...)
				imagePassDbg("extractIterm2OSC: buffering incomplete OSC (%d bytes)", len(data[oscStart:]))
			} else {
				imagePassDbg("extractIterm2OSC: dropping oversized OSC (%d bytes)", len(data[oscStart:]))
			}
			trailing = data[lastCopy:oscStart]
			return
		}
	}

	trailing = data[lastCopy:]
	return
}

// --- Sixel DA1 Detection ---

// DetectSixelTTY sends a DA1 query to /dev/tty and checks for sixel support
// (attribute 4 in the response). Must be called before Setsid detaches the
// controlling terminal.
func DetectSixelTTY() bool {
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

	// Send DA1 (primary device attributes) query
	if _, err := tty.Write([]byte("\x1b[c")); err != nil {
		return false
	}

	done := make(chan bool, 1)
	go func() {
		buf := make([]byte, 256)
		total := 0
		for total < len(buf) {
			n, err := tty.Read(buf[total:])
			if n > 0 {
				total += n
				resp := string(buf[:total])
				if strings.Contains(resp, "c") {
					done <- strings.Contains(resp, ";4;") || strings.HasSuffix(resp, ";4c")
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
	case <-time.After(150 * time.Millisecond):
		// 150ms is generous: most terminals respond in <50ms,
		// but SSH latency can add ~100ms.
		return false
	}
}

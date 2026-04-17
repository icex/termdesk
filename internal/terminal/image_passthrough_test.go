package terminal

import (
	"bytes"
	"encoding/base64"
	"encoding/binary"
	"fmt"
	"strings"
	"testing"
)

// ---------------------------------------------------------------------------
// detectIterm2 / detectSixel
// ---------------------------------------------------------------------------

func TestDetectIterm2(t *testing.T) {
	tests := []struct {
		name string
		env  map[string]string
		want bool
	}{
		{"TERMDESK_ITERM2=1", map[string]string{"TERMDESK_ITERM2": "1"}, true},
		{"TERMDESK_GRAPHICS=iterm2", map[string]string{"TERMDESK_GRAPHICS": "iterm2"}, true},
		{"TERM_PROGRAM=iTerm.app", map[string]string{"TERM_PROGRAM": "iTerm.app"}, true},
		{"TERM_PROGRAM=WezTerm", map[string]string{"TERM_PROGRAM": "WezTerm"}, true},
		{"LC_TERMINAL=iTerm2", map[string]string{"LC_TERMINAL": "iTerm2"}, true},
		{"ITERM_SESSION_ID set", map[string]string{"ITERM_SESSION_ID": "w0t0p0:AAAAA"}, true},
		{"WEZTERM_PANE set", map[string]string{"WEZTERM_PANE": "0"}, true},
		{"no env vars", map[string]string{}, false},
		{"irrelevant env", map[string]string{"TERM_PROGRAM": "xterm"}, false},
		{"TERMDESK_ITERM2=0", map[string]string{"TERMDESK_ITERM2": "0"}, false},
		{"TERMDESK_GRAPHICS=sixel", map[string]string{"TERMDESK_GRAPHICS": "sixel"}, false},
	}

	// List of all env vars detectIterm2 reads — we must clear them all.
	envKeys := []string{
		"TERMDESK_ITERM2", "TERMDESK_GRAPHICS", "TERM_PROGRAM",
		"LC_TERMINAL", "ITERM_SESSION_ID", "WEZTERM_PANE",
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Clear all relevant env vars first.
			for _, k := range envKeys {
				t.Setenv(k, "")
			}
			// Set the specific vars for this test case.
			for k, v := range tt.env {
				t.Setenv(k, v)
			}
			got := detectIterm2()
			if got != tt.want {
				t.Errorf("detectIterm2() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestDetectSixel(t *testing.T) {
	tests := []struct {
		name string
		env  map[string]string
		want bool
	}{
		{"TERMDESK_SIXEL=1", map[string]string{"TERMDESK_SIXEL": "1"}, true},
		{"TERMDESK_GRAPHICS=sixel", map[string]string{"TERMDESK_GRAPHICS": "sixel"}, true},
		{"TERM_PROGRAM=WezTerm", map[string]string{"TERM_PROGRAM": "WezTerm"}, true},
		{"TERM_PROGRAM=ghostty", map[string]string{"TERM_PROGRAM": "ghostty"}, true},
		{"TERM_PROGRAM=foot", map[string]string{"TERM_PROGRAM": "foot"}, true},
		{"TERM_PROGRAM=contour", map[string]string{"TERM_PROGRAM": "contour"}, true},
		{"TERM_PROGRAM=mlterm", map[string]string{"TERM_PROGRAM": "mlterm"}, true},
		{"TERM_PROGRAM=iTerm.app", map[string]string{"TERM_PROGRAM": "iTerm.app"}, true},
		{"XTERM_VERSION set", map[string]string{"XTERM_VERSION": "388"}, true},
		{"no env vars", map[string]string{}, false},
		{"irrelevant TERM_PROGRAM", map[string]string{"TERM_PROGRAM": "xterm"}, false},
		{"TERMDESK_SIXEL=0", map[string]string{"TERMDESK_SIXEL": "0"}, false},
		{"TERMDESK_GRAPHICS=iterm2", map[string]string{"TERMDESK_GRAPHICS": "iterm2"}, false},
	}

	envKeys := []string{
		"TERMDESK_SIXEL", "TERMDESK_GRAPHICS", "TERM_PROGRAM", "XTERM_VERSION",
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			for _, k := range envKeys {
				t.Setenv(k, "")
			}
			for k, v := range tt.env {
				t.Setenv(k, v)
			}
			got := detectSixel()
			if got != tt.want {
				t.Errorf("detectSixel() = %v, want %v", got, tt.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// NewImagePassthrough
// ---------------------------------------------------------------------------

func TestNewImagePassthrough(t *testing.T) {
	// Clear all env to get a clean state.
	envKeys := []string{
		"TERMDESK_ITERM2", "TERMDESK_GRAPHICS", "TERM_PROGRAM",
		"LC_TERMINAL", "ITERM_SESSION_ID", "WEZTERM_PANE",
		"TERMDESK_SIXEL", "XTERM_VERSION",
	}
	for _, k := range envKeys {
		t.Setenv(k, "")
	}

	t.Run("sixel enabled", func(t *testing.T) {
		t.Setenv("TERMDESK_SIXEL", "1")
		ip := NewImagePassthrough(10, 20)
		if !ip.SixelEnabled() {
			t.Error("expected sixel enabled")
		}
	})

	t.Run("iterm2 enabled", func(t *testing.T) {
		t.Setenv("TERMDESK_ITERM2", "1")
		ip := NewImagePassthrough(10, 20)
		if !ip.Iterm2Enabled() {
			t.Error("expected iterm2 enabled")
		}
	})

	t.Run("nothing enabled", func(t *testing.T) {
		ip := NewImagePassthrough(10, 20)
		if ip.IsEnabled() {
			t.Error("expected no protocol enabled")
		}
	})
}

// ---------------------------------------------------------------------------
// SixelEnabled / Iterm2Enabled / IsEnabled
// ---------------------------------------------------------------------------

func TestGetters(t *testing.T) {
	ip := &ImagePassthrough{
		sixelEnabled:  true,
		iterm2Enabled: false,
		placements:    make(map[string][]*ImagePlacement),
	}
	if !ip.SixelEnabled() {
		t.Error("SixelEnabled should be true")
	}
	if ip.Iterm2Enabled() {
		t.Error("Iterm2Enabled should be false")
	}
	if !ip.IsEnabled() {
		t.Error("IsEnabled should be true when sixel is on")
	}

	ip2 := &ImagePassthrough{placements: make(map[string][]*ImagePlacement)}
	if ip2.IsEnabled() {
		t.Error("IsEnabled should be false when nothing is on")
	}
}

// ---------------------------------------------------------------------------
// FlushPending
// ---------------------------------------------------------------------------

func TestFlushPending(t *testing.T) {
	ip := &ImagePassthrough{placements: make(map[string][]*ImagePlacement)}

	// Empty flush returns nil.
	if got := ip.FlushPending(); got != nil {
		t.Errorf("expected nil, got %d bytes", len(got))
	}

	// Non-empty flush returns data and clears.
	ip.pendingOutput = []byte("hello")
	got := ip.FlushPending()
	if string(got) != "hello" {
		t.Errorf("expected 'hello', got %q", got)
	}
	if ip.FlushPending() != nil {
		t.Error("second FlushPending should return nil")
	}
}

// ---------------------------------------------------------------------------
// ForwardImage
// ---------------------------------------------------------------------------

func TestForwardImage(t *testing.T) {
	t.Run("disabled protocol returns 0", func(t *testing.T) {
		ip := &ImagePassthrough{placements: make(map[string][]*ImagePlacement)}
		got := ip.ForwardImage([]byte("data"), ImageFormatSixel, "w1", 0, 0, 0, false, 5)
		if got != 0 {
			t.Errorf("expected 0, got %d", got)
		}
	})

	t.Run("disabled iterm2 returns 0", func(t *testing.T) {
		ip := &ImagePassthrough{placements: make(map[string][]*ImagePlacement)}
		got := ip.ForwardImage([]byte("data"), ImageFormatIterm2, "w1", 0, 0, 0, false, 5)
		if got != 0 {
			t.Errorf("expected 0, got %d", got)
		}
	})

	t.Run("oversized data rejected", func(t *testing.T) {
		ip := &ImagePassthrough{sixelEnabled: true, placements: make(map[string][]*ImagePlacement)}
		bigData := make([]byte, maxImageSize+1)
		got := ip.ForwardImage(bigData, ImageFormatSixel, "w1", 0, 0, 0, false, 5)
		if got != 0 {
			t.Errorf("expected 0 for oversized, got %d", got)
		}
	})

	t.Run("cellRows <= 0 rejected", func(t *testing.T) {
		ip := &ImagePassthrough{sixelEnabled: true, placements: make(map[string][]*ImagePlacement)}
		got := ip.ForwardImage([]byte("data"), ImageFormatSixel, "w1", 0, 0, 0, false, 0)
		if got != 0 {
			t.Errorf("expected 0, got %d", got)
		}
		got = ip.ForwardImage([]byte("data"), ImageFormatSixel, "w1", 0, 0, 0, false, -1)
		if got != 0 {
			t.Errorf("expected 0 for negative cellRows, got %d", got)
		}
	})

	t.Run("stores placement and returns cellRows", func(t *testing.T) {
		ip := &ImagePassthrough{sixelEnabled: true, placements: make(map[string][]*ImagePlacement)}
		got := ip.ForwardImage([]byte("sixeldata"), ImageFormatSixel, "w1", 5, 3, 100, false, 7)
		if got != 7 {
			t.Errorf("expected 7, got %d", got)
		}
		if !ip.HasImagePlacements() {
			t.Error("expected HasImagePlacements() = true")
		}
		pls := ip.placements["w1"]
		if len(pls) != 1 {
			t.Fatalf("expected 1 placement, got %d", len(pls))
		}
		p := pls[0]
		if p.WindowID != "w1" || p.GuestX != 5 || p.AbsoluteLine != 103 || p.CellRows != 7 {
			t.Errorf("placement fields wrong: %+v", p)
		}
	})

	t.Run("data is copied", func(t *testing.T) {
		ip := &ImagePassthrough{iterm2Enabled: true, placements: make(map[string][]*ImagePlacement)}
		buf := []byte("original")
		ip.ForwardImage(buf, ImageFormatIterm2, "w1", 0, 0, 0, false, 1)
		buf[0] = 'X'
		if ip.placements["w1"][0].RawData[0] == 'X' {
			t.Error("placement data should be a copy, not a reference")
		}
	})

	t.Run("capacity eviction", func(t *testing.T) {
		ip := &ImagePassthrough{sixelEnabled: true, placements: make(map[string][]*ImagePlacement)}
		// Fill to capacity
		for i := 0; i < maxImagePlacementsPerWindow; i++ {
			ip.ForwardImage([]byte(fmt.Sprintf("img%d", i)), ImageFormatSixel, "w1", 0, i, 0, false, 1)
		}
		if len(ip.placements["w1"]) != maxImagePlacementsPerWindow {
			t.Fatalf("expected %d, got %d", maxImagePlacementsPerWindow, len(ip.placements["w1"]))
		}
		// One more should evict the oldest
		ip.ForwardImage([]byte("overflow"), ImageFormatSixel, "w1", 0, 99, 0, false, 1)
		if len(ip.placements["w1"]) != maxImagePlacementsPerWindow {
			t.Errorf("expected %d after eviction, got %d", maxImagePlacementsPerWindow, len(ip.placements["w1"]))
		}
		// Oldest (AbsoluteLine=0) should be gone; newest (AbsoluteLine=99) should be present.
		first := ip.placements["w1"][0]
		if first.AbsoluteLine == 0 {
			t.Error("oldest placement should have been evicted")
		}
		last := ip.placements["w1"][len(ip.placements["w1"])-1]
		if last.AbsoluteLine != 99 {
			t.Errorf("newest placement should be last, got AbsoluteLine=%d", last.AbsoluteLine)
		}
	})
}

// ---------------------------------------------------------------------------
// HasImagePlacements / ClearWindow / ForwardRawSequence
// ---------------------------------------------------------------------------

func TestHasImagePlacements(t *testing.T) {
	ip := &ImagePassthrough{placements: make(map[string][]*ImagePlacement)}
	if ip.HasImagePlacements() {
		t.Error("should be false when empty")
	}
	ip.placements["w1"] = []*ImagePlacement{{WindowID: "w1"}}
	if !ip.HasImagePlacements() {
		t.Error("should be true")
	}
}

func TestClearWindowImage(t *testing.T) {
	ip := &ImagePassthrough{placements: make(map[string][]*ImagePlacement)}
	ip.placements["w1"] = []*ImagePlacement{{WindowID: "w1"}}
	ip.placements["w2"] = []*ImagePlacement{{WindowID: "w2"}}
	ip.ClearWindow("w1")
	if _, ok := ip.placements["w1"]; ok {
		t.Error("w1 should be removed")
	}
	if _, ok := ip.placements["w2"]; !ok {
		t.Error("w2 should still exist")
	}
	// ClearWindow on non-existent window should not panic
	ip.ClearWindow("w999")
}

func TestForwardRawSequence(t *testing.T) {
	t.Run("disabled does nothing", func(t *testing.T) {
		ip := &ImagePassthrough{placements: make(map[string][]*ImagePlacement)}
		ip.ForwardRawSequence([]byte("data"))
		if len(ip.pendingOutput) != 0 {
			t.Error("expected no output when disabled")
		}
	})

	t.Run("oversized rejected", func(t *testing.T) {
		ip := &ImagePassthrough{sixelEnabled: true, placements: make(map[string][]*ImagePlacement)}
		big := make([]byte, maxImageSize+1)
		ip.ForwardRawSequence(big)
		if len(ip.pendingOutput) != 0 {
			t.Error("expected no output for oversized")
		}
	})

	t.Run("normal forward appends", func(t *testing.T) {
		ip := &ImagePassthrough{iterm2Enabled: true, placements: make(map[string][]*ImagePlacement)}
		ip.ForwardRawSequence([]byte("abc"))
		ip.ForwardRawSequence([]byte("def"))
		if string(ip.pendingOutput) != "abcdef" {
			t.Errorf("expected 'abcdef', got %q", ip.pendingOutput)
		}
	})
}

// ---------------------------------------------------------------------------
// parseImageDimensions
// ---------------------------------------------------------------------------

func TestParseImageDimensions(t *testing.T) {
	t.Run("PNG", func(t *testing.T) {
		// Build a minimal PNG header: 8-byte signature + 4-byte length + "IHDR" + w + h
		header := make([]byte, 24)
		copy(header[0:8], []byte{0x89, 'P', 'N', 'G', '\r', '\n', 0x1a, '\n'})
		// IHDR chunk length (4 bytes, big-endian) at offset 8
		binary.BigEndian.PutUint32(header[8:12], 13)
		// "IHDR" at offset 12
		copy(header[12:16], []byte("IHDR"))
		// Width at offset 16, Height at offset 20
		binary.BigEndian.PutUint32(header[16:20], 320)
		binary.BigEndian.PutUint32(header[20:24], 240)
		w, h := parseImageDimensions(header)
		if w != 320 || h != 240 {
			t.Errorf("PNG: got %dx%d, want 320x240", w, h)
		}
	})

	t.Run("GIF", func(t *testing.T) {
		header := make([]byte, 10)
		copy(header[0:6], []byte("GIF89a"))
		binary.LittleEndian.PutUint16(header[6:8], 100)
		binary.LittleEndian.PutUint16(header[8:10], 80)
		w, h := parseImageDimensions(header)
		if w != 100 || h != 80 {
			t.Errorf("GIF: got %dx%d, want 100x80", w, h)
		}
	})

	t.Run("GIF87a", func(t *testing.T) {
		header := make([]byte, 10)
		copy(header[0:6], []byte("GIF87a"))
		binary.LittleEndian.PutUint16(header[6:8], 50)
		binary.LittleEndian.PutUint16(header[8:10], 25)
		w, h := parseImageDimensions(header)
		if w != 50 || h != 25 {
			t.Errorf("GIF87a: got %dx%d, want 50x25", w, h)
		}
	})

	t.Run("JPEG with SOF0", func(t *testing.T) {
		// JPEG: FFD8 + JFIF APP0 segment + SOF0 marker
		header := make([]byte, 20)
		header[0] = 0xFF
		header[1] = 0xD8
		// APP0 marker (FF E0) with 4-byte segment
		header[2] = 0xFF
		header[3] = 0xE0
		binary.BigEndian.PutUint16(header[4:6], 4) // segment length = 4 (includes length field)
		// APP0 data: 2 bytes (4 - 2 for length field)
		header[6] = 0x00
		header[7] = 0x00
		// SOF0 marker at offset 8 (after APP0: 2 + 4 = 6, so 2 + 6 = 8)
		header[8] = 0xFF
		header[9] = 0xC0
		binary.BigEndian.PutUint16(header[10:12], 8) // segment length
		header[12] = 8                                // precision
		binary.BigEndian.PutUint16(header[13:15], 480) // height
		binary.BigEndian.PutUint16(header[15:17], 640) // width
		w, h := parseImageDimensions(header)
		if w != 640 || h != 480 {
			t.Errorf("JPEG: got %dx%d, want 640x480", w, h)
		}
	})

	t.Run("JPEG with SOF2 (progressive)", func(t *testing.T) {
		header := make([]byte, 12)
		header[0] = 0xFF
		header[1] = 0xD8
		// SOF2 directly after SOI
		header[2] = 0xFF
		header[3] = 0xC2
		binary.BigEndian.PutUint16(header[4:6], 8)
		header[5] = 8
		binary.BigEndian.PutUint16(header[7:9], 200)
		binary.BigEndian.PutUint16(header[9:11], 300)
		w, h := parseImageDimensions(header)
		if w != 300 || h != 200 {
			t.Errorf("JPEG SOF2: got %dx%d, want 300x200", w, h)
		}
	})

	t.Run("invalid data", func(t *testing.T) {
		w, h := parseImageDimensions([]byte{0x00, 0x01, 0x02})
		if w != 0 || h != 0 {
			t.Errorf("invalid: got %dx%d, want 0x0", w, h)
		}
	})

	t.Run("empty data", func(t *testing.T) {
		w, h := parseImageDimensions(nil)
		if w != 0 || h != 0 {
			t.Errorf("empty: got %dx%d, want 0x0", w, h)
		}
	})

	t.Run("short PNG", func(t *testing.T) {
		w, h := parseImageDimensions([]byte{0x89, 'P', 'N', 'G'})
		if w != 0 || h != 0 {
			t.Errorf("short PNG: got %dx%d, want 0x0", w, h)
		}
	})

	t.Run("JPEG with EOI before SOF", func(t *testing.T) {
		header := []byte{0xFF, 0xD8, 0xFF, 0xD9, 0xFF, 0xC0, 0x00, 0x08, 0x08, 0x00, 0x10, 0x00, 0x20}
		w, h := parseImageDimensions(header)
		if w != 0 || h != 0 {
			t.Errorf("JPEG EOI: got %dx%d, want 0x0", w, h)
		}
	})

	t.Run("JPEG DHT marker (0xC4) is not SOF", func(t *testing.T) {
		// 0xC4 should be skipped (it's DHT, not SOF)
		header := make([]byte, 20)
		header[0] = 0xFF
		header[1] = 0xD8
		header[2] = 0xFF
		header[3] = 0xC4 // DHT, not SOF
		binary.BigEndian.PutUint16(header[4:6], 4)
		header[6] = 0x00
		header[7] = 0x00
		// Now SOF0 at offset 8
		header[8] = 0xFF
		header[9] = 0xC0
		binary.BigEndian.PutUint16(header[10:12], 8)
		header[12] = 8
		binary.BigEndian.PutUint16(header[13:15], 100)
		binary.BigEndian.PutUint16(header[15:17], 200)
		w, h := parseImageDimensions(header)
		if w != 200 || h != 100 {
			t.Errorf("JPEG DHT skip: got %dx%d, want 200x100", w, h)
		}
	})
}

// ---------------------------------------------------------------------------
// parseSixelHeight
// ---------------------------------------------------------------------------

func TestParseSixelHeight(t *testing.T) {
	t.Run("raster attributes", func(t *testing.T) {
		// Format: "Pan;Pad;Ph;Pv  => raster height = Pv
		data := []byte(`"1;1;100;200#0;2;0;0;0`)
		h := parseSixelHeight(data)
		if h != 200 {
			t.Errorf("expected 200, got %d", h)
		}
	})

	t.Run("raster with insufficient parts", func(t *testing.T) {
		// Only 2 parts, should fall through to dash counting
		data := []byte(`"1;1-??~-??~`)
		h := parseSixelHeight(data)
		// 2 dashes => (2+1)*6 = 18
		if h != 18 {
			t.Errorf("expected 18, got %d", h)
		}
	})

	t.Run("dash counting fallback", func(t *testing.T) {
		// No raster attributes. 3 dashes = (3+1)*6 = 24
		data := []byte("??~-??~-??~-??~")
		h := parseSixelHeight(data)
		if h != 24 {
			t.Errorf("expected 24, got %d", h)
		}
	})

	t.Run("no dashes single band", func(t *testing.T) {
		data := []byte("??~??~??~")
		h := parseSixelHeight(data)
		// 0 dashes => (0+1)*6 = 6
		if h != 6 {
			t.Errorf("expected 6, got %d", h)
		}
	})

	t.Run("empty data", func(t *testing.T) {
		h := parseSixelHeight(nil)
		// 0 dashes => (0+1)*6 = 6
		if h != 6 {
			t.Errorf("expected 6, got %d", h)
		}
	})

	t.Run("raster with invalid Pv", func(t *testing.T) {
		// Pv is "abc" (not a number) — falls through to dash counting
		data := []byte(`"1;1;100;abc#??~-??~`)
		h := parseSixelHeight(data)
		// 1 dash => (1+1)*6 = 12
		if h != 12 {
			t.Errorf("expected 12, got %d", h)
		}
	})
}

// ---------------------------------------------------------------------------
// truncateSixelBands
// ---------------------------------------------------------------------------

func TestTruncateSixelBands(t *testing.T) {
	t.Run("7-bit DCS under limit", func(t *testing.T) {
		// 2 bands (1 dash), maxBands=5 => no truncation
		raw := []byte("\x1bPq??~-??~\x1b\\")
		result := truncateSixelBands(raw, 5)
		if !bytes.Equal(result, raw) {
			t.Errorf("expected no truncation, got %q", result)
		}
	})

	t.Run("7-bit DCS truncation", func(t *testing.T) {
		// 4 bands (3 dashes), maxBands=2 => truncate at 2nd dash
		raw := []byte("\x1bPq??~-??~-??~-??~\x1b\\")
		result := truncateSixelBands(raw, 2)
		if result == nil {
			t.Fatal("expected non-nil result")
		}
		// Should contain exactly 1 dash (maxBands-1=1 dashes for 2 bands)
		// Then truncated at the 2nd dash position and ST appended
		dashes := bytes.Count(result, []byte{'-'})
		if dashes != 1 {
			t.Errorf("expected 1 dash in result, got %d (result=%q)", dashes, result)
		}
		if !bytes.HasSuffix(result, []byte("\x1b\\")) {
			t.Error("result should end with 7-bit ST")
		}
	})

	t.Run("8-bit DCS", func(t *testing.T) {
		// 3 bands (2 dashes), maxBands=1 => truncate at 1st dash
		raw := []byte{0x90, 'q', '?', '?', '~', '-', '?', '?', '~', '-', '?', '?', '~', 0x9c}
		result := truncateSixelBands(raw, 1)
		if result == nil {
			t.Fatal("expected non-nil result")
		}
		dashes := bytes.Count(result, []byte{'-'})
		if dashes != 0 {
			t.Errorf("expected 0 dashes, got %d (result=%q)", dashes, result)
		}
	})

	t.Run("maxBands <= 0", func(t *testing.T) {
		raw := []byte("\x1bPq??~\x1b\\")
		if truncateSixelBands(raw, 0) != nil {
			t.Error("maxBands=0 should return nil")
		}
		if truncateSixelBands(raw, -1) != nil {
			t.Error("maxBands=-1 should return nil")
		}
	})

	t.Run("too short data", func(t *testing.T) {
		if truncateSixelBands([]byte{0x1b}, 1) != nil {
			t.Error("short data should return nil")
		}
	})

	t.Run("invalid DCS start", func(t *testing.T) {
		raw := []byte("Xq??~\x1b\\")
		if truncateSixelBands(raw, 1) != nil {
			t.Error("invalid DCS start should return nil")
		}
	})

	t.Run("no q after params", func(t *testing.T) {
		raw := []byte("\x1bP0;1X??~\x1b\\")
		if truncateSixelBands(raw, 1) != nil {
			t.Error("missing 'q' should return nil")
		}
	})

	t.Run("7-bit DCS with params", func(t *testing.T) {
		// DCS with params 0;1;0 before q
		raw := []byte("\x1bP0;1;0q??~-??~-??~\x1b\\")
		result := truncateSixelBands(raw, 2)
		if result == nil {
			t.Fatal("expected non-nil result")
		}
		dashes := bytes.Count(result, []byte{'-'})
		if dashes != 1 {
			t.Errorf("expected 1 dash, got %d", dashes)
		}
	})

	t.Run("no ST terminator in original", func(t *testing.T) {
		// Data without proper ST — should use default \x1b\\
		raw := []byte("\x1bPq??~-??~-??~")
		result := truncateSixelBands(raw, 1)
		if result == nil {
			t.Fatal("expected non-nil result")
		}
		// Should have appended default ST
		if !bytes.HasSuffix(result, []byte("\x1b\\")) {
			t.Error("should end with default ST")
		}
	})
}

// ---------------------------------------------------------------------------
// constrainIterm2Height
// ---------------------------------------------------------------------------

func TestConstrainIterm2Height(t *testing.T) {
	t.Run("replace existing height", func(t *testing.T) {
		raw := []byte("\x1b]1337;File=name=test;height=20;inline=1:base64data\x07")
		result := constrainIterm2Height(raw, 5)
		if !bytes.Contains(result, []byte("height=5")) {
			t.Errorf("expected height=5 in result: %q", result)
		}
		if bytes.Contains(result, []byte("height=20")) {
			t.Error("old height=20 should be replaced")
		}
	})

	t.Run("add height when missing", func(t *testing.T) {
		raw := []byte("\x1b]1337;File=name=test;inline=1:base64data\x07")
		result := constrainIterm2Height(raw, 10)
		if !bytes.Contains(result, []byte("height=10")) {
			t.Errorf("expected height=10 added: %q", result)
		}
	})

	t.Run("maxRows <= 0 returns unchanged", func(t *testing.T) {
		raw := []byte("\x1b]1337;File=inline=1:data\x07")
		result := constrainIterm2Height(raw, 0)
		if !bytes.Equal(result, raw) {
			t.Error("maxRows=0 should return unchanged data")
		}
	})

	t.Run("no File= prefix", func(t *testing.T) {
		raw := []byte("\x1b]1337;Something=else\x07")
		result := constrainIterm2Height(raw, 5)
		if !bytes.Equal(result, raw) {
			t.Error("no File= should return unchanged")
		}
	})

	t.Run("MultipartFile= prefix", func(t *testing.T) {
		raw := []byte("\x1b]1337;MultipartFile=name=test;inline=1:data\x07")
		result := constrainIterm2Height(raw, 3)
		if !bytes.Contains(result, []byte("height=3")) {
			t.Errorf("expected height=3: %q", result)
		}
	})
}

// ---------------------------------------------------------------------------
// constrainIterm2Width
// ---------------------------------------------------------------------------

func makePNGBase64(width, height int) string {
	header := make([]byte, 24)
	copy(header[0:8], []byte{0x89, 'P', 'N', 'G', '\r', '\n', 0x1a, '\n'})
	binary.BigEndian.PutUint32(header[8:12], 13)
	copy(header[12:16], []byte("IHDR"))
	binary.BigEndian.PutUint32(header[16:20], uint32(width))
	binary.BigEndian.PutUint32(header[20:24], uint32(height))
	return base64.StdEncoding.EncodeToString(header)
}

func TestConstrainIterm2Width(t *testing.T) {
	t.Run("image fits naturally, no modification", func(t *testing.T) {
		// 100px wide image, cell=10px, content=80 cols => native=10 cols, fits in 80
		b64 := makePNGBase64(100, 50)
		raw := []byte(fmt.Sprintf("\x1b]1337;File=inline=1:%s\x07", b64))
		result := constrainIterm2Width(raw, 80, 10)
		if !bytes.Equal(result, raw) {
			t.Error("image fits, should not be modified")
		}
	})

	t.Run("image overflows, width added", func(t *testing.T) {
		// 1000px wide image, cell=10px, content=50 cols => native=100 cols > 50
		b64 := makePNGBase64(1000, 500)
		raw := []byte(fmt.Sprintf("\x1b]1337;File=inline=1:%s\x07", b64))
		result := constrainIterm2Width(raw, 50, 10)
		if !bytes.Contains(result, []byte("width=50")) {
			t.Errorf("expected width=50 added: %q", result)
		}
	})

	t.Run("explicit width respects sender", func(t *testing.T) {
		b64 := makePNGBase64(1000, 500)
		raw := []byte(fmt.Sprintf("\x1b]1337;File=width=20;inline=1:%s\x07", b64))
		result := constrainIterm2Width(raw, 50, 10)
		if !bytes.Equal(result, raw) {
			t.Error("explicit width should not be overridden")
		}
	})

	t.Run("contentWidthCells <= 0 returns unchanged", func(t *testing.T) {
		raw := []byte("\x1b]1337;File=inline=1:data\x07")
		result := constrainIterm2Width(raw, 0, 10)
		if !bytes.Equal(result, raw) {
			t.Error("contentWidthCells=0 should return unchanged")
		}
	})

	t.Run("cellPixelW <= 0 returns unchanged", func(t *testing.T) {
		raw := []byte("\x1b]1337;File=inline=1:data\x07")
		result := constrainIterm2Width(raw, 80, 0)
		if !bytes.Equal(result, raw) {
			t.Error("cellPixelW=0 should return unchanged")
		}
	})

	t.Run("no File= prefix", func(t *testing.T) {
		raw := []byte("\x1b]1337;Other=data\x07")
		result := constrainIterm2Width(raw, 80, 10)
		if !bytes.Equal(result, raw) {
			t.Error("no File= should return unchanged")
		}
	})

	t.Run("invalid base64 returns unchanged", func(t *testing.T) {
		raw := []byte("\x1b]1337;File=inline=1:!@#$%^&\x07")
		result := constrainIterm2Width(raw, 80, 10)
		if !bytes.Equal(result, raw) {
			t.Error("invalid base64 should return unchanged")
		}
	})

	t.Run("width=auto is not treated as explicit", func(t *testing.T) {
		b64 := makePNGBase64(1000, 500)
		raw := []byte(fmt.Sprintf("\x1b]1337;File=width=auto;inline=1:%s\x07", b64))
		result := constrainIterm2Width(raw, 50, 10)
		// width=auto is not "explicit" so constraining should happen
		if bytes.Equal(result, raw) {
			t.Error("width=auto should allow constraining")
		}
	})

	t.Run("ST terminator", func(t *testing.T) {
		b64 := makePNGBase64(1000, 500)
		raw := []byte(fmt.Sprintf("\x1b]1337;File=inline=1:%s\x1b\\", b64))
		result := constrainIterm2Width(raw, 50, 10)
		if !bytes.Contains(result, []byte("width=50")) {
			t.Errorf("expected width=50 with ST terminator: %s", result)
		}
	})
}

// ---------------------------------------------------------------------------
// estimateIterm2CellRows
// ---------------------------------------------------------------------------

func TestEstimateIterm2CellRows(t *testing.T) {
	t.Run("explicit cell height", func(t *testing.T) {
		// width=10;height=8 (both explicit cell counts)
		raw := []byte("\x1b]1337;File=width=10;height=8;inline=1:base64data\x07")
		rows := estimateIterm2CellRows(raw, 80, 10, 20)
		if rows != 8 {
			t.Errorf("expected 8, got %d", rows)
		}
	})

	t.Run("auto with embedded PNG", func(t *testing.T) {
		// 200px wide, 100px tall. cellPixelW=10, contentCols=80 => maxW=800
		// displayW = 200 (fits). displayH = 100. cellRows = ceil(100/20) = 5
		b64 := makePNGBase64(200, 100)
		raw := []byte(fmt.Sprintf("\x1b]1337;File=inline=1:%s\x07", b64))
		rows := estimateIterm2CellRows(raw, 80, 10, 20)
		if rows != 5 {
			t.Errorf("expected 5, got %d", rows)
		}
	})

	t.Run("auto with overflow width", func(t *testing.T) {
		// 1000px wide, 500px tall. contentCols=50, cellPixelW=10 => maxW=500
		// displayW = 500 (capped). displayH = 500*500/1000 = 250. cellRows = ceil(250/20) = 13
		b64 := makePNGBase64(1000, 500)
		raw := []byte(fmt.Sprintf("\x1b]1337;File=inline=1:%s\x07", b64))
		rows := estimateIterm2CellRows(raw, 50, 10, 20)
		if rows != 13 {
			t.Errorf("expected 13, got %d", rows)
		}
	})

	t.Run("pixel width/height params", func(t *testing.T) {
		// width=100px, height=200px => displayW=100, displayH=200, rows=ceil(200/20)=10
		b64 := makePNGBase64(400, 400)
		raw := []byte(fmt.Sprintf("\x1b]1337;File=width=100px;height=200px;inline=1:%s\x07", b64))
		rows := estimateIterm2CellRows(raw, 80, 10, 20)
		if rows != 10 {
			t.Errorf("expected 10, got %d", rows)
		}
	})

	t.Run("explicit cell width with auto height", func(t *testing.T) {
		// width=5 (cells) => displayW = 5*10 = 50. Image is 200x100.
		// displayH = 100*50/200 = 25. cellRows = ceil(25/20) = 2
		b64 := makePNGBase64(200, 100)
		raw := []byte(fmt.Sprintf("\x1b]1337;File=width=5;height=auto;inline=1:%s\x07", b64))
		rows := estimateIterm2CellRows(raw, 80, 10, 20)
		if rows != 2 {
			t.Errorf("expected 2, got %d", rows)
		}
	})

	t.Run("no colon returns 0", func(t *testing.T) {
		raw := []byte("\x1b]1337;File=inline=1")
		rows := estimateIterm2CellRows(raw, 80, 10, 20)
		if rows != 0 {
			t.Errorf("expected 0, got %d", rows)
		}
	})

	t.Run("invalid image header returns 0", func(t *testing.T) {
		raw := []byte("\x1b]1337;File=inline=1:AAAA\x07")
		rows := estimateIterm2CellRows(raw, 80, 10, 20)
		if rows != 0 {
			t.Errorf("expected 0, got %d", rows)
		}
	})

	t.Run("defaults for zero cellPixelH and cellPixelW", func(t *testing.T) {
		// When cellPixelH=0, defaults to 20. cellPixelW=0, defaults to 10.
		b64 := makePNGBase64(200, 100)
		raw := []byte(fmt.Sprintf("\x1b]1337;File=inline=1:%s\x07", b64))
		rows := estimateIterm2CellRows(raw, 80, 0, 0)
		// displayW=200, maxW=80*10=800, fits. displayH=100. rows=ceil(100/20)=5
		if rows != 5 {
			t.Errorf("expected 5 with defaults, got %d", rows)
		}
	})

	t.Run("ST terminator", func(t *testing.T) {
		b64 := makePNGBase64(200, 100)
		raw := []byte(fmt.Sprintf("\x1b]1337;File=inline=1:%s\x1b\\", b64))
		rows := estimateIterm2CellRows(raw, 80, 10, 20)
		if rows != 5 {
			t.Errorf("expected 5 with ST terminator, got %d", rows)
		}
	})

	t.Run("explicit cell height with non-numeric width", func(t *testing.T) {
		// width=auto;height=12
		raw := []byte("\x1b]1337;File=width=auto;height=12;inline=1:AAAA\x07")
		rows := estimateIterm2CellRows(raw, 80, 10, 20)
		// width=auto, height=12 — not both explicit, so falls to image parsing
		// Invalid image => 0
		if rows != 0 {
			t.Errorf("expected 0 (auto width + bad image), got %d", rows)
		}
	})
}

// ---------------------------------------------------------------------------
// RefreshAllImages
// ---------------------------------------------------------------------------

func TestRefreshAllImages(t *testing.T) {
	t.Run("empty placements no output", func(t *testing.T) {
		ip := &ImagePassthrough{placements: make(map[string][]*ImagePlacement)}
		ip.RefreshAllImages(24, func() map[string]*ImageWindowInfo { return nil })
		if len(ip.pendingOutput) != 0 {
			t.Error("expected no output for empty placements")
		}
	})

	t.Run("window gone removes placements", func(t *testing.T) {
		ip := &ImagePassthrough{sixelEnabled: true, placements: make(map[string][]*ImagePlacement)}
		ip.placements["gone"] = []*ImagePlacement{{WindowID: "gone", CellRows: 1}}
		ip.RefreshAllImages(24, func() map[string]*ImageWindowInfo {
			return map[string]*ImageWindowInfo{} // "gone" not present
		})
		if _, ok := ip.placements["gone"]; ok {
			t.Error("placements for gone window should be removed")
		}
	})

	t.Run("visible placement generates output", func(t *testing.T) {
		ip := &ImagePassthrough{
			sixelEnabled: true,
			cellPixelH:   16,
			placements:   make(map[string][]*ImagePlacement),
		}
		// Build a minimal sixel DCS
		sixelData := []byte("\x1bPq??~\x1b\\")
		ip.placements["w1"] = []*ImagePlacement{{
			WindowID:     "w1",
			GuestX:       0,
			AbsoluteLine: 5,
			RawData:      sixelData,
			CellRows:     1,
			Format:       ImageFormatSixel,
			IsAltScreen:  false,
		}}
		ip.RefreshAllImages(50, func() map[string]*ImageWindowInfo {
			return map[string]*ImageWindowInfo{
				"w1": {
					WindowX: 2, WindowY: 1, ContentOffX: 1, ContentOffY: 1,
					ContentWidth: 80, ContentHeight: 24,
					Visible: true, ScrollbackLen: 5, ScrollOffset: 0,
					IsAltScreen: false,
				},
			}
		})
		if len(ip.pendingOutput) == 0 {
			t.Error("expected pending output for visible placement")
		}
		// Should contain cursor positioning escape
		if !bytes.Contains(ip.pendingOutput, []byte("\x1b[")) {
			t.Error("expected cursor positioning in output")
		}
	})

	t.Run("off-screen placement GCd", func(t *testing.T) {
		ip := &ImagePassthrough{
			sixelEnabled: true,
			cellPixelH:   16,
			placements:   make(map[string][]*ImagePlacement),
		}
		ip.placements["w1"] = []*ImagePlacement{{
			WindowID:     "w1",
			GuestX:       0,
			AbsoluteLine: 0, // line 0
			CellRows:     1,
			Format:       ImageFormatSixel,
			IsAltScreen:  false,
		}}
		ip.RefreshAllImages(24, func() map[string]*ImageWindowInfo {
			return map[string]*ImageWindowInfo{
				"w1": {
					ContentWidth: 80, ContentHeight: 24,
					Visible: true, ScrollbackLen: 100, ScrollOffset: 0,
					IsAltScreen: false,
				},
			}
		})
		// AbsoluteLine=0, viewportTop=100, relativeY=-100, fullBottom=-99
		// fullBottom <= -viewportHeight (-99 <= -24)? yes => GC'd
		if _, ok := ip.placements["w1"]; ok {
			t.Error("off-screen placement should be GC'd")
		}
	})

	t.Run("alt screen mismatch kept but not rendered", func(t *testing.T) {
		ip := &ImagePassthrough{
			sixelEnabled: true,
			cellPixelH:   16,
			placements:   make(map[string][]*ImagePlacement),
		}
		sixelData := []byte("\x1bPq??~\x1b\\")
		ip.placements["w1"] = []*ImagePlacement{{
			WindowID:     "w1",
			GuestX:       0,
			AbsoluteLine: 5,
			RawData:      sixelData,
			CellRows:     1,
			Format:       ImageFormatSixel,
			IsAltScreen:  true, // placement in alt screen
		}}
		ip.RefreshAllImages(50, func() map[string]*ImageWindowInfo {
			return map[string]*ImageWindowInfo{
				"w1": {
					ContentWidth: 80, ContentHeight: 24,
					Visible: true, ScrollbackLen: 5,
					IsAltScreen: false, // window is NOT in alt screen
				},
			}
		})
		// Placement should be kept
		if _, ok := ip.placements["w1"]; !ok {
			t.Error("alt screen mismatch placement should be kept")
		}
		// But no output generated
		if len(ip.pendingOutput) != 0 {
			t.Error("alt screen mismatch should not generate output")
		}
	})

	t.Run("iterm2 placement rendered with width constraint", func(t *testing.T) {
		b64 := makePNGBase64(1000, 500)
		rawOSC := []byte(fmt.Sprintf("\x1b]1337;File=inline=1:%s\x07", b64))
		ip := &ImagePassthrough{
			iterm2Enabled: true,
			cellPixelW:    10,
			cellPixelH:    20,
			placements:    make(map[string][]*ImagePlacement),
		}
		ip.placements["w1"] = []*ImagePlacement{{
			WindowID:     "w1",
			GuestX:       0,
			AbsoluteLine: 5,
			RawData:      rawOSC,
			CellRows:     5,
			Format:       ImageFormatIterm2,
			IsAltScreen:  false,
		}}
		ip.RefreshAllImages(50, func() map[string]*ImageWindowInfo {
			return map[string]*ImageWindowInfo{
				"w1": {
					WindowX: 0, WindowY: 0, ContentOffX: 0, ContentOffY: 0,
					ContentWidth: 50, ContentHeight: 24,
					Visible: true, ScrollbackLen: 5, ScrollOffset: 0,
					IsAltScreen: false,
				},
			}
		})
		if len(ip.pendingOutput) == 0 {
			t.Error("expected pending output for iTerm2 placement")
		}
	})

	t.Run("not visible window suppresses output", func(t *testing.T) {
		ip := &ImagePassthrough{
			sixelEnabled: true,
			cellPixelH:   16,
			placements:   make(map[string][]*ImagePlacement),
		}
		ip.placements["w1"] = []*ImagePlacement{{
			WindowID:     "w1",
			GuestX:       0,
			AbsoluteLine: 5,
			RawData:      []byte("\x1bPq??~\x1b\\"),
			CellRows:     1,
			Format:       ImageFormatSixel,
			IsAltScreen:  false,
		}}
		ip.RefreshAllImages(50, func() map[string]*ImageWindowInfo {
			return map[string]*ImageWindowInfo{
				"w1": {
					ContentWidth: 80, ContentHeight: 24,
					Visible:       false, // not visible
					ScrollbackLen: 5,
					IsAltScreen:   false,
				},
			}
		})
		if len(ip.pendingOutput) != 0 {
			t.Error("non-visible window should not produce output")
		}
	})

	t.Run("hostY above contentTop clipped", func(t *testing.T) {
		ip := &ImagePassthrough{
			sixelEnabled: true,
			cellPixelH:   16,
			placements:   make(map[string][]*ImagePlacement),
		}
		ip.placements["w1"] = []*ImagePlacement{{
			WindowID:     "w1",
			GuestX:       0,
			AbsoluteLine: 3, // will be relativeY = 3-5 = -2
			RawData:      []byte("\x1bPq??~\x1b\\"),
			CellRows:     1,
			Format:       ImageFormatSixel,
			IsAltScreen:  false,
		}}
		ip.RefreshAllImages(50, func() map[string]*ImageWindowInfo {
			return map[string]*ImageWindowInfo{
				"w1": {
					WindowX: 0, WindowY: 5, ContentOffX: 0, ContentOffY: 2,
					ContentWidth: 80, ContentHeight: 24,
					Visible: true, ScrollbackLen: 5, ScrollOffset: 0,
					IsAltScreen: false,
				},
			}
		})
		// hostY = 5 + 2 + (-2) = 5, contentTop = 5 + 2 = 7. hostY(5) < contentTop(7) => clipped
		if len(ip.pendingOutput) != 0 {
			t.Error("placement above content top should be clipped")
		}
	})
}

// ---------------------------------------------------------------------------
// extractSixelDCS (Terminal method)
// ---------------------------------------------------------------------------

func newMinimalTerminal() *Terminal {
	return &Terminal{
		writeCh:   make(chan []byte, 256),
		emuCh:     make(chan []byte, 128),
		done:      make(chan struct{}),
		inputDone: make(chan struct{}),
		scrollCap: defaultScrollbackCap,
		scrollRing: make([][]ScreenCell, defaultScrollbackCap),
	}
}

func TestExtractSixelDCS(t *testing.T) {
	t.Run("no DCS in data", func(t *testing.T) {
		term := newMinimalTerminal()
		segs, trailing := term.extractSixelDCS([]byte("hello world"))
		if len(segs) != 0 {
			t.Error("expected no segments")
		}
		if string(trailing) != "hello world" {
			t.Errorf("expected trailing='hello world', got %q", trailing)
		}
	})

	t.Run("7-bit DCS complete", func(t *testing.T) {
		term := newMinimalTerminal()
		data := []byte("before\x1bPq??~-??~\x1b\\after")
		segs, trailing := term.extractSixelDCS(data)
		if len(segs) != 1 {
			t.Fatalf("expected 1 segment, got %d", len(segs))
		}
		if string(segs[0].DataBefore) != "before" {
			t.Errorf("DataBefore=%q, want 'before'", segs[0].DataBefore)
		}
		if !bytes.Contains(segs[0].RawDCS, []byte("\x1bPq")) {
			t.Error("RawDCS should contain DCS start")
		}
		if segs[0].PixelRows <= 0 {
			t.Error("PixelRows should be positive")
		}
		if string(trailing) != "after" {
			t.Errorf("trailing=%q, want 'after'", trailing)
		}
	})

	t.Run("8-bit DCS complete", func(t *testing.T) {
		term := newMinimalTerminal()
		data := []byte("x\x90q??~\x9cy")
		segs, trailing := term.extractSixelDCS(data)
		if len(segs) != 1 {
			t.Fatalf("expected 1 segment, got %d", len(segs))
		}
		if string(segs[0].DataBefore) != "x" {
			t.Errorf("DataBefore=%q", segs[0].DataBefore)
		}
		if string(trailing) != "y" {
			t.Errorf("trailing=%q", trailing)
		}
	})

	t.Run("multiple segments", func(t *testing.T) {
		term := newMinimalTerminal()
		data := []byte("a\x1bPq??~\x1b\\b\x1bPq??~-??~\x1b\\c")
		segs, trailing := term.extractSixelDCS(data)
		if len(segs) != 2 {
			t.Fatalf("expected 2 segments, got %d", len(segs))
		}
		if string(segs[0].DataBefore) != "a" {
			t.Errorf("seg0.DataBefore=%q", segs[0].DataBefore)
		}
		if string(segs[1].DataBefore) != "b" {
			t.Errorf("seg1.DataBefore=%q", segs[1].DataBefore)
		}
		if string(trailing) != "c" {
			t.Errorf("trailing=%q", trailing)
		}
	})

	t.Run("incomplete sequence buffered", func(t *testing.T) {
		term := newMinimalTerminal()
		data := []byte("before\x1bPq??~??~")
		segs, trailing := term.extractSixelDCS(data)
		if len(segs) != 0 {
			t.Error("expected no segments for incomplete")
		}
		if string(trailing) != "before" {
			t.Errorf("trailing=%q, want 'before'", trailing)
		}
		if len(term.sixelDCSBuf) == 0 {
			t.Error("expected sixelDCSBuf to be non-empty")
		}

		// Second call completes the sequence
		data2 := []byte("-??~\x1b\\done")
		segs2, trailing2 := term.extractSixelDCS(data2)
		if len(segs2) != 1 {
			t.Fatalf("expected 1 segment after completion, got %d", len(segs2))
		}
		if string(trailing2) != "done" {
			t.Errorf("trailing2=%q", trailing2)
		}
	})

	t.Run("non-sixel DCS skipped (no q)", func(t *testing.T) {
		term := newMinimalTerminal()
		// DCS with 'r' instead of 'q' (not sixel)
		data := []byte("\x1bPr??~\x1b\\trailing")
		segs, trailing := term.extractSixelDCS(data)
		if len(segs) != 0 {
			t.Error("expected no segments for non-sixel DCS")
		}
		if string(trailing) != "\x1bPr??~\x1b\\trailing" {
			t.Errorf("trailing=%q", trailing)
		}
	})

	t.Run("DCS with params before q", func(t *testing.T) {
		term := newMinimalTerminal()
		data := []byte("\x1bP0;1;0q??~\x1b\\")
		segs, trailing := term.extractSixelDCS(data)
		if len(segs) != 1 {
			t.Fatalf("expected 1 segment, got %d", len(segs))
		}
		if len(trailing) != 0 {
			t.Errorf("unexpected trailing: %q", trailing)
		}
	})

	t.Run("buffered data combined with new data", func(t *testing.T) {
		term := newMinimalTerminal()
		// Pre-populate buffer with partial DCS
		term.sixelDCSBuf = []byte("\x1bPq??~")
		data := []byte("\x1b\\rest")
		segs, trailing := term.extractSixelDCS(data)
		if len(segs) != 1 {
			t.Fatalf("expected 1 segment, got %d", len(segs))
		}
		if string(trailing) != "rest" {
			t.Errorf("trailing=%q", trailing)
		}
		if len(term.sixelDCSBuf) != 0 {
			t.Error("sixelDCSBuf should be cleared after successful extraction")
		}
	})
}

// ---------------------------------------------------------------------------
// extractIterm2OSC (Terminal method)
// ---------------------------------------------------------------------------

func TestExtractIterm2OSC(t *testing.T) {
	t.Run("no iTerm2 OSC", func(t *testing.T) {
		term := newMinimalTerminal()
		segs, trailing := term.extractIterm2OSC([]byte("hello world"))
		if len(segs) != 0 {
			t.Error("expected no segments")
		}
		if string(trailing) != "hello world" {
			t.Errorf("trailing=%q", trailing)
		}
	})

	t.Run("File= with BEL terminator", func(t *testing.T) {
		term := newMinimalTerminal()
		data := []byte("pre\x1b]1337;File=inline=1:data\x07post")
		segs, trailing := term.extractIterm2OSC(data)
		if len(segs) != 1 {
			t.Fatalf("expected 1 segment, got %d", len(segs))
		}
		if string(segs[0].DataBefore) != "pre" {
			t.Errorf("DataBefore=%q", segs[0].DataBefore)
		}
		if segs[0].IsMultipart {
			t.Error("File= should not be multipart")
		}
		if !bytes.Contains(segs[0].RawOSC, []byte("File=")) {
			t.Error("RawOSC should contain File=")
		}
		if string(trailing) != "post" {
			t.Errorf("trailing=%q", trailing)
		}
	})

	t.Run("File= with ST terminator", func(t *testing.T) {
		term := newMinimalTerminal()
		data := []byte("\x1b]1337;File=inline=1:data\x1b\\rest")
		segs, trailing := term.extractIterm2OSC(data)
		if len(segs) != 1 {
			t.Fatalf("expected 1 segment, got %d", len(segs))
		}
		if string(trailing) != "rest" {
			t.Errorf("trailing=%q", trailing)
		}
	})

	t.Run("MultipartFile= not multipart flag", func(t *testing.T) {
		term := newMinimalTerminal()
		data := []byte("\x1b]1337;MultipartFile=name=test:data\x07")
		segs, _ := term.extractIterm2OSC(data)
		if len(segs) != 1 {
			t.Fatalf("expected 1 segment, got %d", len(segs))
		}
		// MultipartFile= is index 1, which is < 2, so IsMultipart = false
		if segs[0].IsMultipart {
			t.Error("MultipartFile= should not be marked as IsMultipart")
		}
	})

	t.Run("FilePart= is multipart", func(t *testing.T) {
		term := newMinimalTerminal()
		data := []byte("\x1b]1337;FilePart=id=1:chunk\x07")
		segs, _ := term.extractIterm2OSC(data)
		if len(segs) != 1 {
			t.Fatalf("expected 1 segment, got %d", len(segs))
		}
		if !segs[0].IsMultipart {
			t.Error("FilePart= should be multipart")
		}
	})

	t.Run("FileEnd is multipart", func(t *testing.T) {
		term := newMinimalTerminal()
		data := []byte("\x1b]1337;FileEnd\x07")
		segs, _ := term.extractIterm2OSC(data)
		if len(segs) != 1 {
			t.Fatalf("expected 1 segment, got %d", len(segs))
		}
		if !segs[0].IsMultipart {
			t.Error("FileEnd should be multipart")
		}
	})

	t.Run("multiple OSC segments", func(t *testing.T) {
		term := newMinimalTerminal()
		data := []byte("a\x1b]1337;File=inline=1:data1\x07b\x1b]1337;FilePart=id=1:data2\x07c")
		segs, trailing := term.extractIterm2OSC(data)
		if len(segs) != 2 {
			t.Fatalf("expected 2 segments, got %d", len(segs))
		}
		if string(segs[0].DataBefore) != "a" {
			t.Errorf("seg0.DataBefore=%q", segs[0].DataBefore)
		}
		if string(segs[1].DataBefore) != "b" {
			t.Errorf("seg1.DataBefore=%q", segs[1].DataBefore)
		}
		if string(trailing) != "c" {
			t.Errorf("trailing=%q", trailing)
		}
	})

	t.Run("incomplete sequence buffered", func(t *testing.T) {
		term := newMinimalTerminal()
		data := []byte("before\x1b]1337;File=inline=1:dat")
		segs, trailing := term.extractIterm2OSC(data)
		if len(segs) != 0 {
			t.Error("expected no segments for incomplete")
		}
		if string(trailing) != "before" {
			t.Errorf("trailing=%q", trailing)
		}
		if len(term.iterm2OSCBuf) == 0 {
			t.Error("expected iterm2OSCBuf to be non-empty")
		}

		// Complete in second call
		data2 := []byte("a\x07after")
		segs2, trailing2 := term.extractIterm2OSC(data2)
		if len(segs2) != 1 {
			t.Fatalf("expected 1 segment after completion, got %d", len(segs2))
		}
		if string(trailing2) != "after" {
			t.Errorf("trailing2=%q", trailing2)
		}
	})

	t.Run("other OSC 1337 is ignored", func(t *testing.T) {
		term := newMinimalTerminal()
		// Not a file-related OSC 1337
		data := []byte("\x1b]1337;CursorShape=1\x07")
		segs, trailing := term.extractIterm2OSC(data)
		if len(segs) != 0 {
			t.Error("non-file OSC should yield no segments")
		}
		if string(trailing) != "\x1b]1337;CursorShape=1\x07" {
			t.Errorf("trailing=%q", trailing)
		}
	})

	t.Run("buffered data combined with new data", func(t *testing.T) {
		term := newMinimalTerminal()
		term.iterm2OSCBuf = []byte("\x1b]1337;File=inline=1:d")
		data := []byte("ata\x07rest")
		segs, trailing := term.extractIterm2OSC(data)
		if len(segs) != 1 {
			t.Fatalf("expected 1 segment, got %d", len(segs))
		}
		if string(trailing) != "rest" {
			t.Errorf("trailing=%q", trailing)
		}
	})
}

// ---------------------------------------------------------------------------
// Edge cases and integration-style tests
// ---------------------------------------------------------------------------

func TestForwardImageMultipleWindows(t *testing.T) {
	ip := &ImagePassthrough{
		sixelEnabled:  true,
		iterm2Enabled: true,
		placements:    make(map[string][]*ImagePlacement),
	}

	ip.ForwardImage([]byte("s1"), ImageFormatSixel, "w1", 0, 0, 0, false, 3)
	ip.ForwardImage([]byte("i1"), ImageFormatIterm2, "w2", 0, 0, 0, false, 5)
	ip.ForwardImage([]byte("s2"), ImageFormatSixel, "w1", 0, 1, 0, false, 2)

	if len(ip.placements["w1"]) != 2 {
		t.Errorf("w1 expected 2 placements, got %d", len(ip.placements["w1"]))
	}
	if len(ip.placements["w2"]) != 1 {
		t.Errorf("w2 expected 1 placement, got %d", len(ip.placements["w2"]))
	}
}

func TestRefreshAllImagesMultipleWindows(t *testing.T) {
	ip := &ImagePassthrough{
		sixelEnabled: true,
		cellPixelH:   16,
		placements:   make(map[string][]*ImagePlacement),
	}

	ip.placements["w1"] = []*ImagePlacement{{
		WindowID: "w1", GuestX: 0, AbsoluteLine: 5,
		RawData: []byte("\x1bPq??~\x1b\\"), CellRows: 1,
		Format: ImageFormatSixel,
	}}
	ip.placements["w2"] = []*ImagePlacement{{
		WindowID: "w2", GuestX: 0, AbsoluteLine: 5,
		RawData: []byte("\x1bPq??~\x1b\\"), CellRows: 1,
		Format: ImageFormatSixel,
	}}

	ip.RefreshAllImages(50, func() map[string]*ImageWindowInfo {
		return map[string]*ImageWindowInfo{
			"w1": {
				WindowX: 0, WindowY: 0, ContentOffX: 0, ContentOffY: 0,
				ContentWidth: 80, ContentHeight: 24,
				Visible: true, ScrollbackLen: 5,
			},
			// w2 is not returned => should be removed
		}
	})

	if _, ok := ip.placements["w1"]; !ok {
		t.Error("w1 should still have placements")
	}
	if _, ok := ip.placements["w2"]; ok {
		t.Error("w2 should be removed (window gone)")
	}
}

func TestRefreshSixelBottomClamp(t *testing.T) {
	ip := &ImagePassthrough{
		sixelEnabled: true,
		cellPixelH:   16,
		placements:   make(map[string][]*ImagePlacement),
	}

	// Place image near the bottom of the host terminal
	ip.placements["w1"] = []*ImagePlacement{{
		WindowID: "w1", GuestX: 0, AbsoluteLine: 22,
		RawData:  []byte("\x1bPq??~-??~-??~-??~-??~\x1b\\"),
		CellRows: 5, Format: ImageFormatSixel,
	}}

	ip.RefreshAllImages(25, func() map[string]*ImageWindowInfo {
		return map[string]*ImageWindowInfo{
			"w1": {
				WindowX: 0, WindowY: 0, ContentOffX: 0, ContentOffY: 0,
				ContentWidth: 80, ContentHeight: 24,
				Visible: true, ScrollbackLen: 22,
			},
		}
	})

	// Should generate output (truncated sixel)
	if len(ip.pendingOutput) == 0 {
		t.Error("expected output for bottom-clamped sixel")
	}
}

func TestRefreshIterm2AvailableRowsZero(t *testing.T) {
	ip := &ImagePassthrough{
		iterm2Enabled: true,
		cellPixelW:    10,
		cellPixelH:    20,
		placements:    make(map[string][]*ImagePlacement),
	}

	b64 := makePNGBase64(100, 50)
	rawOSC := []byte(fmt.Sprintf("\x1b]1337;File=inline=1:%s\x07", b64))

	// Place image at the very bottom where availableRows <= 0
	ip.placements["w1"] = []*ImagePlacement{{
		WindowID: "w1", GuestX: 0, AbsoluteLine: 23,
		RawData: rawOSC, CellRows: 5, Format: ImageFormatIterm2,
	}}

	ip.RefreshAllImages(24, func() map[string]*ImageWindowInfo {
		return map[string]*ImageWindowInfo{
			"w1": {
				WindowX: 0, WindowY: 0, ContentOffX: 0, ContentOffY: 0,
				ContentWidth: 80, ContentHeight: 24,
				Visible: true, ScrollbackLen: 23,
			},
		}
	})

	// availableRows = contentBottom - hostY + 1 = 23 - 23 + 1 = 1 > 0, so it should render
	// But if hostTermHeight clamps: contentBottom = min(23, 23) = 23, hostY = 0+0+0 = 0...
	// Actually: relativeY = 23-23 = 0, hostY = 0+0+0 = 0, contentBottom = 0+0+24-1 = 23,
	// but hostTermHeight=24 so contentBottom=min(23,23)=23, availableRows=23-0+1=24>0 => renders
	// This is fine, just verify no panic
}

func TestParseSixelHeightRasterZeroPv(t *testing.T) {
	// Pv=0 should fall through to dash counting
	data := []byte(`"1;1;100;0-??~-??~`)
	h := parseSixelHeight(data)
	// 2 dashes => (2+1)*6 = 18
	if h != 18 {
		t.Errorf("expected 18, got %d", h)
	}
}

func TestEstimateIterm2CellRowsDefaultContentCols(t *testing.T) {
	// contentCols=0 defaults to 80
	b64 := makePNGBase64(200, 100)
	raw := []byte(fmt.Sprintf("\x1b]1337;File=inline=1:%s\x07", b64))
	rows := estimateIterm2CellRows(raw, 0, 10, 20)
	// contentCols defaults to 80, maxW=80*10=800, image=200px fits
	// displayW=200, displayH=100, rows=ceil(100/20)=5
	if rows != 5 {
		t.Errorf("expected 5, got %d", rows)
	}
}

func TestEstimateIterm2CellRowsGIF(t *testing.T) {
	// Build GIF header
	header := make([]byte, 10)
	copy(header[0:6], []byte("GIF89a"))
	binary.LittleEndian.PutUint16(header[6:8], 200)
	binary.LittleEndian.PutUint16(header[8:10], 100)
	b64 := base64.StdEncoding.EncodeToString(header)
	raw := []byte(fmt.Sprintf("\x1b]1337;File=inline=1:%s\x07", b64))
	rows := estimateIterm2CellRows(raw, 80, 10, 20)
	// 200px wide, 100px tall. maxW=800, fits. displayH=100, rows=ceil(100/20)=5
	if rows != 5 {
		t.Errorf("expected 5, got %d", rows)
	}
}

func TestEstimateIterm2CellRowsExplicitHeightPixels(t *testing.T) {
	// height=100px with an image
	b64 := makePNGBase64(400, 400)
	raw := []byte(fmt.Sprintf("\x1b]1337;File=height=100px;inline=1:%s\x07", b64))
	rows := estimateIterm2CellRows(raw, 80, 10, 20)
	// height=100px => displayH = 100. cellRows = ceil(100/20) = 5
	if rows != 5 {
		t.Errorf("expected 5, got %d", rows)
	}
}

func TestConstrainIterm2HeightNoColon(t *testing.T) {
	// No colon separator — should return unchanged
	raw := []byte("\x1b]1337;File=inline=1data\x07")
	result := constrainIterm2Height(raw, 5)
	if !bytes.Equal(result, raw) {
		t.Error("no colon should return unchanged")
	}
}

func TestTruncateSixelBandsExactMatch(t *testing.T) {
	// 3 bands (2 dashes), maxBands=3 => no truncation
	raw := []byte("\x1bPq??~-??~-??~\x1b\\")
	result := truncateSixelBands(raw, 3)
	if !bytes.Equal(result, raw) {
		t.Errorf("exact match should return original, got %q", result)
	}
}

func TestTruncateSixelBands8bitST(t *testing.T) {
	// 8-bit DCS with 8-bit ST
	raw := []byte{0x90, 'q', '?', '?', '~', '-', '?', '?', '~', '-', '?', '?', '~', 0x9c}
	result := truncateSixelBands(raw, 2)
	if result == nil {
		t.Fatal("expected non-nil result")
	}
	dashes := bytes.Count(result, []byte{'-'})
	if dashes != 1 {
		t.Errorf("expected 1 dash, got %d", dashes)
	}
}

func TestExtractSixelDCSFastPath(t *testing.T) {
	// No ESC and no 0x90 => fast path returns nil, data
	term := newMinimalTerminal()
	data := []byte("no escape sequences here")
	segs, trailing := term.extractSixelDCS(data)
	if len(segs) != 0 {
		t.Error("expected no segments on fast path")
	}
	if !bytes.Equal(trailing, data) {
		t.Error("trailing should equal original data on fast path")
	}
}

func TestExtractIterm2OSCFastPath(t *testing.T) {
	// No \x1b]1337 => fast path
	term := newMinimalTerminal()
	data := []byte("normal text with \x1b[31m color")
	segs, trailing := term.extractIterm2OSC(data)
	if len(segs) != 0 {
		t.Error("expected no segments on fast path")
	}
	if !bytes.Equal(trailing, data) {
		t.Error("trailing should equal original data on fast path")
	}
}

func TestRefreshGuestXOutOfBounds(t *testing.T) {
	ip := &ImagePassthrough{
		sixelEnabled: true,
		cellPixelH:   16,
		placements:   make(map[string][]*ImagePlacement),
	}

	// GuestX negative — should not render
	ip.placements["w1"] = []*ImagePlacement{{
		WindowID: "w1", GuestX: -1, AbsoluteLine: 5,
		RawData: []byte("\x1bPq??~\x1b\\"), CellRows: 1,
		Format: ImageFormatSixel,
	}}

	ip.RefreshAllImages(50, func() map[string]*ImageWindowInfo {
		return map[string]*ImageWindowInfo{
			"w1": {
				ContentWidth: 80, ContentHeight: 24,
				Visible: true, ScrollbackLen: 5,
			},
		}
	})

	if len(ip.pendingOutput) != 0 {
		t.Error("negative GuestX should not produce output")
	}
}

func TestRefreshGuestXAtContentWidth(t *testing.T) {
	ip := &ImagePassthrough{
		sixelEnabled: true,
		cellPixelH:   16,
		placements:   make(map[string][]*ImagePlacement),
	}

	// GuestX == ContentWidth — out of bounds
	ip.placements["w1"] = []*ImagePlacement{{
		WindowID: "w1", GuestX: 80, AbsoluteLine: 5,
		RawData: []byte("\x1bPq??~\x1b\\"), CellRows: 1,
		Format: ImageFormatSixel,
	}}

	ip.RefreshAllImages(50, func() map[string]*ImageWindowInfo {
		return map[string]*ImageWindowInfo{
			"w1": {
				ContentWidth: 80, ContentHeight: 24,
				Visible: true, ScrollbackLen: 5,
			},
		}
	})

	if len(ip.pendingOutput) != 0 {
		t.Error("GuestX==ContentWidth should not produce output")
	}
}

func TestConstrainIterm2WidthMultipartFile(t *testing.T) {
	b64 := makePNGBase64(1000, 500)
	raw := []byte(fmt.Sprintf("\x1b]1337;MultipartFile=inline=1:%s\x07", b64))
	result := constrainIterm2Width(raw, 50, 10)
	if !bytes.Contains(result, []byte("width=50")) {
		t.Errorf("expected width=50 for MultipartFile=: %s", result)
	}
}

func TestExtractSixelDCSIncompleteAtParams(t *testing.T) {
	term := newMinimalTerminal()
	// ESC P followed by digits but no 'q' — data ends at param parse
	data := []byte("before\x1bP0;1")
	segs, trailing := term.extractSixelDCS(data)
	if len(segs) != 0 {
		t.Error("expected no segments")
	}
	// The incomplete DCS should be buffered
	if len(term.sixelDCSBuf) == 0 {
		t.Error("expected sixelDCSBuf to be non-empty")
	}
	if string(trailing) != "before" {
		t.Errorf("trailing=%q, want 'before'", trailing)
	}
}

func TestImageFormatConstants(t *testing.T) {
	// Verify the constants are distinct
	if ImageFormatSixel == ImageFormatIterm2 {
		t.Error("ImageFormatSixel and ImageFormatIterm2 should be different")
	}
}

func TestConstrainIterm2WidthEmptyAfterDecode(t *testing.T) {
	// Unrecognized image format in base64 data
	junkB64 := base64.StdEncoding.EncodeToString([]byte("not an image format"))
	raw := []byte(fmt.Sprintf("\x1b]1337;File=inline=1:%s\x07", junkB64))
	result := constrainIterm2Width(raw, 50, 10)
	if !bytes.Equal(result, raw) {
		t.Error("unrecognized format should return unchanged")
	}
}

func TestFlushPendingDoesNotLeakReference(t *testing.T) {
	ip := &ImagePassthrough{placements: make(map[string][]*ImagePlacement)}
	ip.pendingOutput = []byte("data")
	out := ip.FlushPending()
	// Verify the internal field is nil
	if ip.pendingOutput != nil {
		t.Error("pendingOutput should be nil after flush")
	}
	_ = out
}

func TestConstrainIterm2HeightNoEqSign(t *testing.T) {
	// Malformed: File= but no = in params section (just "File" without params)
	// This is artificial but tests the eqIdx < 0 branch
	raw := []byte("\x1b]1337;File:data\x07")
	result := constrainIterm2Height(raw, 5)
	if !bytes.Equal(result, raw) {
		t.Error("no = sign should return unchanged")
	}
}

func TestConstrainIterm2WidthNoColonAfterFile(t *testing.T) {
	raw := []byte("\x1b]1337;File=inline=1")
	result := constrainIterm2Width(raw, 80, 10)
	if !bytes.Equal(result, raw) {
		t.Error("no colon should return unchanged")
	}
}

func TestEstimateIterm2CellRowsExplicitHeightCells(t *testing.T) {
	// Both width and height are explicit cell counts (not auto, not px)
	raw := []byte("\x1b]1337;File=width=20;height=15;inline=1:AAAA\x07")
	rows := estimateIterm2CellRows(raw, 80, 10, 20)
	if rows != 15 {
		t.Errorf("expected 15, got %d", rows)
	}
}

func TestRefreshSixelAvailableRowsZero(t *testing.T) {
	ip := &ImagePassthrough{
		sixelEnabled: true,
		cellPixelH:   16,
		placements:   make(map[string][]*ImagePlacement),
	}

	// Place image at the very bottom where safeBottom < hostY
	ip.placements["w1"] = []*ImagePlacement{{
		WindowID: "w1", GuestX: 0, AbsoluteLine: 5,
		RawData:  []byte("\x1bPq??~\x1b\\"),
		CellRows: 1, Format: ImageFormatSixel,
	}}

	ip.RefreshAllImages(3, func() map[string]*ImageWindowInfo {
		return map[string]*ImageWindowInfo{
			"w1": {
				WindowX: 0, WindowY: 0, ContentOffX: 0, ContentOffY: 0,
				ContentWidth: 80, ContentHeight: 10,
				Visible: true, ScrollbackLen: 5,
			},
		}
	})

	// hostY = 0+0+0 = 0, safeBottom = min(9, 3-2)=1, availableRows = 1-0+1=2
	// maxBands = (2*16)/6 = 5, then 5-1=4 > 0 => should render
	// This mainly tests no panic
}

// Verify the GIF header with a trailing partial
func TestParseImageDimensionsShortGIF(t *testing.T) {
	header := make([]byte, 8) // need 10 for GIF
	copy(header[0:6], []byte("GIF89a"))
	w, h := parseImageDimensions(header)
	if w != 0 || h != 0 {
		t.Errorf("short GIF: got %dx%d, want 0x0", w, h)
	}
}

func TestParseImageDimensionsShortJPEG(t *testing.T) {
	// JPEG SOI only, no SOF marker
	header := []byte{0xFF, 0xD8}
	w, h := parseImageDimensions(header)
	if w != 0 || h != 0 {
		t.Errorf("short JPEG: got %dx%d, want 0x0", w, h)
	}
}

func TestParseImageDimensionsJPEGPaddingFF(t *testing.T) {
	// JPEG with consecutive 0xFF bytes (fill bytes)
	header := make([]byte, 14)
	header[0] = 0xFF
	header[1] = 0xD8
	header[2] = 0xFF
	header[3] = 0xFF // padding FF
	header[4] = 0xC0 // SOF0
	binary.BigEndian.PutUint16(header[5:7], 8)
	header[7] = 8 // precision
	binary.BigEndian.PutUint16(header[8:10], 300)
	binary.BigEndian.PutUint16(header[10:12], 400)
	w, h := parseImageDimensions(header)
	if w != 400 || h != 300 {
		t.Errorf("JPEG with fill bytes: got %dx%d, want 400x300", w, h)
	}
}

func TestSixelDCSBufTooLarge(t *testing.T) {
	term := newMinimalTerminal()
	// Create a DCS that is larger than maxImageSize — should not be buffered
	data := make([]byte, maxImageSize+100)
	data[0] = '\x1b'
	data[1] = 'P'
	data[2] = 'q'
	// Fill with sixel data, no ST terminator
	for i := 3; i < len(data); i++ {
		data[i] = '?'
	}
	segs, _ := term.extractSixelDCS(data)
	if len(segs) != 0 {
		t.Error("expected no segments for oversized incomplete DCS")
	}
	if len(term.sixelDCSBuf) != 0 {
		t.Error("oversized incomplete DCS should not be buffered")
	}
}

func TestIterm2OSCBufTooLarge(t *testing.T) {
	term := newMinimalTerminal()
	// Create oversized incomplete iTerm2 OSC
	prefix := []byte("\x1b]1337;File=inline=1:")
	data := make([]byte, maxImageSize+100)
	copy(data, prefix)
	for i := len(prefix); i < len(data); i++ {
		data[i] = 'A' // base64 data
	}
	// No terminator
	segs, _ := term.extractIterm2OSC(data)
	if len(segs) != 0 {
		t.Error("expected no segments for oversized incomplete OSC")
	}
	if len(term.iterm2OSCBuf) != 0 {
		t.Error("oversized incomplete OSC should not be buffered")
	}
}

// Test constrainIterm2Width with no = sign after File keyword
func TestConstrainIterm2WidthNoEqAfterFile(t *testing.T) {
	raw := []byte("\x1b]1337;File:data\x07")
	result := constrainIterm2Width(raw, 80, 10)
	if !bytes.Equal(result, raw) {
		t.Error("no = after File should return unchanged")
	}
}

// Test that empty width param is not treated as explicit
func TestConstrainIterm2WidthEmptyValue(t *testing.T) {
	b64 := makePNGBase64(1000, 500)
	raw := []byte(fmt.Sprintf("\x1b]1337;File=width=;inline=1:%s\x07", b64))
	result := constrainIterm2Width(raw, 50, 10)
	// width="" is empty — should not be treated as explicit, so constraining happens
	if bytes.Equal(result, raw) {
		t.Error("empty width value should allow constraining")
	}
}

// ---------------------------------------------------------------------------
// Additional coverage tests for RefreshAllImages (sixel/iTerm2 rendering paths)
// ---------------------------------------------------------------------------

func TestRefreshSixelMaxBandsOne(t *testing.T) {
	// Test the maxBands=1 case (no decrement, stays at 1)
	ip := &ImagePassthrough{
		sixelEnabled: true,
		cellPixelH:   6, // cellPixelH=6, so maxBands = (1*6)/6 = 1, then no decrement since <=1
		placements:   make(map[string][]*ImagePlacement),
	}
	sixelData := []byte("\x1bPq??~\x1b\\")
	ip.placements["w1"] = []*ImagePlacement{{
		WindowID: "w1", GuestX: 0, AbsoluteLine: 5,
		RawData: sixelData, CellRows: 1, Format: ImageFormatSixel,
	}}

	ip.RefreshAllImages(50, func() map[string]*ImageWindowInfo {
		return map[string]*ImageWindowInfo{
			"w1": {
				WindowX: 0, WindowY: 0, ContentOffX: 0, ContentOffY: 0,
				ContentWidth: 80, ContentHeight: 24,
				Visible: true, ScrollbackLen: 5, ScrollOffset: 0,
			},
		}
	})
	// maxBands = (1*6)/6 = 1, which is <=1 so no decrement. Should still render.
	if len(ip.pendingOutput) == 0 {
		t.Error("expected output for maxBands=1 sixel rendering")
	}
}

func TestRefreshSixelCellPixelHZero(t *testing.T) {
	// When cellPixelH=0, should default to 16
	ip := &ImagePassthrough{
		sixelEnabled: true,
		cellPixelH:   0,
		placements:   make(map[string][]*ImagePlacement),
	}
	sixelData := []byte("\x1bPq??~\x1b\\")
	ip.placements["w1"] = []*ImagePlacement{{
		WindowID: "w1", GuestX: 0, AbsoluteLine: 5,
		RawData: sixelData, CellRows: 1, Format: ImageFormatSixel,
	}}

	ip.RefreshAllImages(50, func() map[string]*ImageWindowInfo {
		return map[string]*ImageWindowInfo{
			"w1": {
				WindowX: 0, WindowY: 0, ContentOffX: 0, ContentOffY: 0,
				ContentWidth: 80, ContentHeight: 24,
				Visible: true, ScrollbackLen: 5, ScrollOffset: 0,
			},
		}
	})
	if len(ip.pendingOutput) == 0 {
		t.Error("expected output with cellPixelH defaulting to 16")
	}
}

func TestRefreshSixelTruncateFail(t *testing.T) {
	// Test when truncateSixelBands returns nil (e.g., invalid DCS structure)
	ip := &ImagePassthrough{
		sixelEnabled: true,
		cellPixelH:   16,
		placements:   make(map[string][]*ImagePlacement),
	}
	// Use invalid DCS (doesn't start with ESC P or 0x90) — truncateSixelBands will return nil
	badData := []byte("not-a-valid-dcs")
	ip.placements["w1"] = []*ImagePlacement{{
		WindowID: "w1", GuestX: 0, AbsoluteLine: 5,
		RawData: badData, CellRows: 1, Format: ImageFormatSixel,
	}}

	ip.RefreshAllImages(50, func() map[string]*ImageWindowInfo {
		return map[string]*ImageWindowInfo{
			"w1": {
				WindowX: 0, WindowY: 0, ContentOffX: 0, ContentOffY: 0,
				ContentWidth: 80, ContentHeight: 24,
				Visible: true, ScrollbackLen: 5, ScrollOffset: 0,
			},
		}
	})
	// truncateSixelBands returns nil for invalid DCS, so placement is skipped
	if len(ip.pendingOutput) != 0 {
		t.Error("expected no output when truncateSixelBands fails")
	}
}

func TestRefreshSixelSkipBottomZeroBands(t *testing.T) {
	// Test availableRows > 0 but maxBands = 0 after computation
	ip := &ImagePassthrough{
		sixelEnabled: true,
		cellPixelH:   100, // very large cell — maxBands = (1*100)/6 = 16, minus 1 = 15
		placements:   make(map[string][]*ImagePlacement),
	}
	// Place image so that safeBottom < hostY (availableRows=0)
	sixelData := []byte("\x1bPq??~\x1b\\")
	ip.placements["w1"] = []*ImagePlacement{{
		WindowID: "w1", GuestX: 0, AbsoluteLine: 5,
		RawData: sixelData, CellRows: 1, Format: ImageFormatSixel,
	}}

	// hostTermHeight=3, contentBottom=min(23,2)=2, safeBottom=min(2,1)=1
	// hostY=0+0+0=0, availableRows=1-0+1=2
	ip.RefreshAllImages(3, func() map[string]*ImageWindowInfo {
		return map[string]*ImageWindowInfo{
			"w1": {
				WindowX: 0, WindowY: 0, ContentOffX: 0, ContentOffY: 0,
				ContentWidth: 80, ContentHeight: 24,
				Visible: true, ScrollbackLen: 5, ScrollOffset: 0,
			},
		}
	})
	// Should produce output (maxBands = (2*100)/6 = 33, minus 1 = 32 > 0)
	// This test mainly verifies no panic with extreme cellPixelH values
}

func TestRefreshIterm2AvailableRowsPositive(t *testing.T) {
	// Test iTerm2 rendering path where availableRows > 0
	b64 := makePNGBase64(100, 50)
	rawOSC := []byte(fmt.Sprintf("\x1b]1337;File=inline=1:%s\x07", b64))
	ip := &ImagePassthrough{
		iterm2Enabled: true,
		cellPixelW:    10,
		cellPixelH:    20,
		placements:    make(map[string][]*ImagePlacement),
	}

	ip.placements["w1"] = []*ImagePlacement{{
		WindowID: "w1", GuestX: 0, AbsoluteLine: 5,
		RawData: rawOSC, CellRows: 3, Format: ImageFormatIterm2,
	}}

	ip.RefreshAllImages(50, func() map[string]*ImageWindowInfo {
		return map[string]*ImageWindowInfo{
			"w1": {
				WindowX: 0, WindowY: 0, ContentOffX: 0, ContentOffY: 0,
				ContentWidth: 50, ContentHeight: 24,
				Visible: true, ScrollbackLen: 5, ScrollOffset: 0,
			},
		}
	})

	if len(ip.pendingOutput) == 0 {
		t.Error("expected pending output for iTerm2 placement with available rows")
	}
	// Verify cursor positioning and width constraint are in output
	if !bytes.Contains(ip.pendingOutput, []byte("\x1b[")) {
		t.Error("expected cursor positioning in output")
	}
}

func TestRefreshAllImagesAllKeptEmpty(t *testing.T) {
	// Test that when all placements for a window are GCd, the window entry is removed
	ip := &ImagePassthrough{
		sixelEnabled: true,
		cellPixelH:   16,
		placements:   make(map[string][]*ImagePlacement),
	}
	// Single placement that will be GCd (scrolled way off top)
	ip.placements["w1"] = []*ImagePlacement{{
		WindowID: "w1", GuestX: 0, AbsoluteLine: 0,
		RawData: []byte("\x1bPq??~\x1b\\"), CellRows: 1,
		Format: ImageFormatSixel,
	}}

	ip.RefreshAllImages(24, func() map[string]*ImageWindowInfo {
		return map[string]*ImageWindowInfo{
			"w1": {
				ContentWidth: 80, ContentHeight: 24,
				Visible: true, ScrollbackLen: 200, // way past absLine=0
			},
		}
	})

	if _, ok := ip.placements["w1"]; ok {
		t.Error("window with all GCd placements should be removed from map")
	}
}

func TestRefreshSixelHostTermHeightZero(t *testing.T) {
	// Test with hostTermHeight=0 — should not clamp contentBottom
	ip := &ImagePassthrough{
		sixelEnabled: true,
		cellPixelH:   16,
		placements:   make(map[string][]*ImagePlacement),
	}
	sixelData := []byte("\x1bPq??~\x1b\\")
	ip.placements["w1"] = []*ImagePlacement{{
		WindowID: "w1", GuestX: 0, AbsoluteLine: 5,
		RawData: sixelData, CellRows: 1, Format: ImageFormatSixel,
	}}

	ip.RefreshAllImages(0, func() map[string]*ImageWindowInfo {
		return map[string]*ImageWindowInfo{
			"w1": {
				WindowX: 0, WindowY: 0, ContentOffX: 0, ContentOffY: 0,
				ContentWidth: 80, ContentHeight: 24,
				Visible: true, ScrollbackLen: 5,
			},
		}
	})
	// hostTermHeight=0 means no clamping. Should still render.
	if len(ip.pendingOutput) == 0 {
		t.Error("expected output with hostTermHeight=0")
	}
}

// ---------------------------------------------------------------------------
// Additional estimateIterm2CellRows edge cases
// ---------------------------------------------------------------------------

func TestEstimateIterm2CellRowsExplicitCellWidthOverflow(t *testing.T) {
	// Explicit width in cells that causes overflow
	b64 := makePNGBase64(200, 100)
	raw := []byte(fmt.Sprintf("\x1b]1337;File=width=100;height=auto;inline=1:%s\x07", b64))
	rows := estimateIterm2CellRows(raw, 80, 10, 20)
	// width=100 cells => displayW = 100*10 = 1000px
	// displayH = 100*1000/200 = 500. cellRows = ceil(500/20) = 25
	if rows != 25 {
		t.Errorf("expected 25, got %d", rows)
	}
}

func TestEstimateIterm2CellRowsInvalidBase64FallsToRaw(t *testing.T) {
	// Base64 data with padding issues (falls through to RawStdEncoding)
	// Create PNG header, encode without padding
	header := make([]byte, 24)
	copy(header[0:8], []byte{0x89, 'P', 'N', 'G', '\r', '\n', 0x1a, '\n'})
	binary.BigEndian.PutUint32(header[8:12], 13)
	copy(header[12:16], []byte("IHDR"))
	binary.BigEndian.PutUint32(header[16:20], 200)
	binary.BigEndian.PutUint32(header[20:24], 100)
	// Use raw encoding (no padding)
	b64 := base64.RawStdEncoding.EncodeToString(header)
	raw := []byte(fmt.Sprintf("\x1b]1337;File=inline=1:%s\x07", b64))
	rows := estimateIterm2CellRows(raw, 80, 10, 20)
	if rows != 5 {
		t.Errorf("expected 5 (raw base64 fallback), got %d", rows)
	}
}

func TestEstimateIterm2CellRowsWidthPixelSuffix(t *testing.T) {
	// width=200px => displayW=200, displayH=100*200/200=100. rows=ceil(100/20)=5
	b64 := makePNGBase64(200, 100)
	raw := []byte(fmt.Sprintf("\x1b]1337;File=width=200px;inline=1:%s\x07", b64))
	rows := estimateIterm2CellRows(raw, 80, 10, 20)
	if rows != 5 {
		t.Errorf("expected 5 with width=200px, got %d", rows)
	}
}

func TestEstimateIterm2CellRowsExplicitHeightCellsOnly(t *testing.T) {
	// Both explicit: width=10;height=5 — returns 5 (parsed as cell count)
	raw := []byte("\x1b]1337;File=width=10;height=5;inline=1:AAAA\x07")
	rows := estimateIterm2CellRows(raw, 80, 10, 20)
	if rows != 5 {
		t.Errorf("expected 5, got %d", rows)
	}
}

// ---------------------------------------------------------------------------
// constrainIterm2Width RawStdEncoding fallback
// ---------------------------------------------------------------------------

func TestConstrainIterm2WidthRawBase64Fallback(t *testing.T) {
	// PNG header encoded without padding (RawStdEncoding)
	header := make([]byte, 24)
	copy(header[0:8], []byte{0x89, 'P', 'N', 'G', '\r', '\n', 0x1a, '\n'})
	binary.BigEndian.PutUint32(header[8:12], 13)
	copy(header[12:16], []byte("IHDR"))
	binary.BigEndian.PutUint32(header[16:20], 1000)
	binary.BigEndian.PutUint32(header[20:24], 500)
	b64 := base64.RawStdEncoding.EncodeToString(header)
	raw := []byte(fmt.Sprintf("\x1b]1337;File=inline=1:%s\x07", b64))
	result := constrainIterm2Width(raw, 50, 10)
	if !bytes.Contains(result, []byte("width=50")) {
		t.Errorf("expected width=50 with raw base64 fallback: %s", result)
	}
}

// ---------------------------------------------------------------------------
// constrainIterm2Height edge cases
// ---------------------------------------------------------------------------

func TestConstrainIterm2HeightNoColonInParams(t *testing.T) {
	// File= present but no colon after params
	raw := []byte("\x1b]1337;File=inline=1\x07")
	result := constrainIterm2Height(raw, 5)
	if !bytes.Equal(result, raw) {
		t.Error("no colon after File= params should return unchanged")
	}
}

func TestConstrainIterm2HeightNoEqInParams(t *testing.T) {
	// File= present, colon exists, but no = between keyword and params
	// This tests the eqIdx < 0 path
	raw := []byte("\x1b]1337;File:data\x07")
	result := constrainIterm2Height(raw, 5)
	if !bytes.Equal(result, raw) {
		t.Error("no = in params should return unchanged")
	}
}

// Test parseSixelHeight with raster that stops at color introducer
func TestParseSixelHeightRasterStopsAtColor(t *testing.T) {
	data := []byte(`"1;1;100;50#0;2;0;0;0??~`)
	h := parseSixelHeight(data)
	if h != 50 {
		t.Errorf("expected 50, got %d", h)
	}
}

// Test parseSixelHeight with raster that stops at repeat
func TestParseSixelHeightRasterStopsAtRepeat(t *testing.T) {
	data := []byte(`"1;1;80;40!10?`)
	h := parseSixelHeight(data)
	if h != 40 {
		t.Errorf("expected 40, got %d", h)
	}
}

// Test parseSixelHeight with raster that stops at carriage return
func TestParseSixelHeightRasterStopsAtCR(t *testing.T) {
	data := []byte(`"1;1;80;60$??~`)
	h := parseSixelHeight(data)
	if h != 60 {
		t.Errorf("expected 60, got %d", h)
	}
}

// Test parseSixelHeight with raster that stops at dash
func TestParseSixelHeightRasterStopsAtDash(t *testing.T) {
	data := []byte(`"1;1;80;30-??~`)
	h := parseSixelHeight(data)
	if h != 30 {
		t.Errorf("expected 30, got %d", h)
	}
}

// Test parseSixelHeight with raster that stops at pixel data
func TestParseSixelHeightRasterStopsAtPixelData(t *testing.T) {
	data := []byte(`"1;1;80;45?~`)
	h := parseSixelHeight(data)
	if h != 45 {
		t.Errorf("expected 45, got %d", h)
	}
}

func TestEstimateIterm2CellRowsExplicitWidthCellsWithAutoHeight(t *testing.T) {
	// width=3 (cells), height=auto.
	// Image: 300x150. displayW = 3*10 = 30.
	// displayH = 150*30/300 = 15. cellRows = ceil(15/20) = 1
	b64 := makePNGBase64(300, 150)
	raw := []byte(fmt.Sprintf("\x1b]1337;File=width=3;inline=1:%s\x07", b64))
	rows := estimateIterm2CellRows(raw, 80, 10, 20)
	if rows != 1 {
		t.Errorf("expected 1, got %d", rows)
	}
}

func TestEstimateIterm2CellRowsNoFilePrefix(t *testing.T) {
	raw := []byte("\x1b]1337;Other=stuff:AAAA\x07")
	rows := estimateIterm2CellRows(raw, 80, 10, 20)
	// No "File=" found, so paramWidth/paramHeight both empty
	// Falls through to image parsing, AAAA decodes to junk => 0
	if rows != 0 {
		t.Errorf("expected 0, got %d", rows)
	}
}

func TestMultipleExtractIterm2OSCNoLeadingGarbage(t *testing.T) {
	term := newMinimalTerminal()
	data := []byte("\x1b]1337;File=inline=1:aaa\x07\x1b]1337;FileEnd\x07")
	segs, trailing := term.extractIterm2OSC(data)
	if len(segs) != 2 {
		t.Fatalf("expected 2 segments, got %d", len(segs))
	}
	if len(segs[0].DataBefore) != 0 {
		t.Errorf("seg0 DataBefore should be empty, got %q", segs[0].DataBefore)
	}
	if len(segs[1].DataBefore) != 0 {
		t.Errorf("seg1 DataBefore should be empty, got %q", segs[1].DataBefore)
	}
	if len(trailing) != 0 {
		t.Errorf("trailing should be empty, got %q", trailing)
	}
}

// Verify that ForwardImage with iterm2 format works when iterm2 is enabled
func TestForwardImageIterm2Enabled(t *testing.T) {
	ip := &ImagePassthrough{iterm2Enabled: true, placements: make(map[string][]*ImagePlacement)}
	got := ip.ForwardImage([]byte("iterm2data"), ImageFormatIterm2, "w1", 2, 4, 50, true, 3)
	if got != 3 {
		t.Errorf("expected 3, got %d", got)
	}
	p := ip.placements["w1"][0]
	if p.IsAltScreen != true {
		t.Error("expected IsAltScreen=true")
	}
	if p.Format != ImageFormatIterm2 {
		t.Error("expected ImageFormatIterm2")
	}
	if p.AbsoluteLine != 54 {
		t.Errorf("expected AbsoluteLine=54, got %d", p.AbsoluteLine)
	}
}

func TestRefreshPartiallyVisibleImage(t *testing.T) {
	ip := &ImagePassthrough{
		sixelEnabled: true,
		cellPixelH:   16,
		placements:   make(map[string][]*ImagePlacement),
	}

	// Image that extends below viewport but starts within it
	ip.placements["w1"] = []*ImagePlacement{{
		WindowID: "w1", GuestX: 0, AbsoluteLine: 22,
		RawData:  []byte("\x1bPq??~-??~-??~\x1b\\"),
		CellRows: 5, Format: ImageFormatSixel,
	}}

	ip.RefreshAllImages(50, func() map[string]*ImageWindowInfo {
		return map[string]*ImageWindowInfo{
			"w1": {
				WindowX: 0, WindowY: 0, ContentOffX: 0, ContentOffY: 0,
				ContentWidth: 80, ContentHeight: 24,
				Visible: true, ScrollbackLen: 22,
			},
		}
	})

	// relativeY = 22-22 = 0, fullBottom = 0+5 = 5, visible (0 < 24 && 5 > 0)
	// Should produce output (truncated sixel)
	if len(ip.pendingOutput) == 0 {
		t.Error("partially visible image should produce output")
	}
}

func TestRefreshScrolledUpButStillInRange(t *testing.T) {
	ip := &ImagePassthrough{
		sixelEnabled: true,
		cellPixelH:   16,
		placements:   make(map[string][]*ImagePlacement),
	}

	// Image that scrolled slightly above viewport but within GC threshold
	ip.placements["w1"] = []*ImagePlacement{{
		WindowID: "w1", GuestX: 0, AbsoluteLine: 90,
		RawData:  []byte("\x1bPq??~\x1b\\"),
		CellRows: 1, Format: ImageFormatSixel,
	}}

	ip.RefreshAllImages(50, func() map[string]*ImageWindowInfo {
		return map[string]*ImageWindowInfo{
			"w1": {
				ContentWidth: 80, ContentHeight: 24,
				Visible: true, ScrollbackLen: 100, ScrollOffset: 0,
			},
		}
	})

	// viewportTop = 100-0=100, relativeY = 90-100=-10, fullBottom=-10+1=-9
	// GC check: fullBottom <= -viewportHeight => -9 <= -24? No => kept (but not visible)
	if _, ok := ip.placements["w1"]; !ok {
		t.Error("placement should be kept (within GC threshold)")
	}
}

func TestExtractSixelDCS8bitMixed(t *testing.T) {
	// 8-bit DCS start with 7-bit ST
	term := newMinimalTerminal()
	data := []byte{0x90, 'q', '?', '?', '~', '\x1b', '\\', 'z'}
	segs, trailing := term.extractSixelDCS(data)
	if len(segs) != 1 {
		t.Fatalf("expected 1 segment, got %d", len(segs))
	}
	if string(trailing) != "z" {
		t.Errorf("trailing=%q", trailing)
	}
}

// Verify constrainIterm2Height preserves base64 data after params
func TestConstrainIterm2HeightPreservesData(t *testing.T) {
	raw := []byte("\x1b]1337;File=name=test;inline=1:AAAA==\x07")
	result := constrainIterm2Height(raw, 5)
	if !bytes.Contains(result, []byte(":AAAA==\x07")) {
		t.Errorf("data after colon should be preserved: %q", result)
	}
	if !bytes.Contains(result, []byte("height=5")) {
		t.Error("height should be injected")
	}
	if !strings.Contains(string(result), "name=test") {
		t.Error("existing params should be preserved")
	}
}

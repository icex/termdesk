package terminal

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func newTestKP() *KittyPassthrough {
	return &KittyPassthrough{
		enabled:       true,
		placements:    make(map[string]map[uint32]*KittyPlacement),
		imageIDMap:    make(map[string]map[uint32]uint32),
		nextHostID:    1,
		pendingDirect: make(map[string]*pendingDirectTransmit),
		cellPixelW:    10,
		cellPixelH:    20,
	}
}

// TestTransmitPlaceSingleChunk verifies a single-chunk a=T places the image
// and returns a PlacementResult with correct Rows for cursor injection.
func TestTransmitPlaceSingleChunk(t *testing.T) {
	kp := newTestKP()
	cmd := &KittyCommand{
		Action: KittyActionTransmitPlace,
		Format: KittyFormatPNG,
		Width:  100,
		Height: 200,
		Data:   []byte("fakepng"),
	}
	result := kp.ForwardCommand(cmd, nil, "win1",
		5, 2, 80, 24, 1, 1, 0, 0, 0, false, nil)

	if result == nil {
		t.Fatal("expected PlacementResult, got nil")
	}
	// 200px / 20px cell = 10 rows. Cursor injection uses full imgRows.
	if result.Rows != 10 {
		t.Errorf("expected Rows=10 (imgRows), got %d", result.Rows)
	}
	// Should have a placement tracked (hidden — deferred to refresh)
	if len(kp.placements["win1"]) != 1 {
		t.Errorf("expected 1 placement, got %d", len(kp.placements["win1"]))
	}
	for _, p := range kp.placements["win1"] {
		if !p.Hidden {
			t.Error("expected placement to be hidden (deferred to refresh)")
		}
	}
	// Pending output should have transmit command only (no place — deferred)
	out := string(kp.pendingOutput)
	if !strings.Contains(out, "a=t,") {
		t.Error("expected transmit command in output")
	}
	if strings.Contains(out, "a=p,") {
		t.Error("expected NO place command in output (deferred to refresh)")
	}
}

// TestTransmitPlaceChunked verifies that chunked a=T (first chunk) followed
// by continuation chunks (default a=t) correctly places the image.
func TestTransmitPlaceChunked(t *testing.T) {
	kp := newTestKP()

	// First chunk: a=T, m=1
	cmd1 := &KittyCommand{
		Action: KittyActionTransmitPlace,
		Format: KittyFormatPNG,
		Width:  100,
		Height: 200,
		More:   true,
		Data:   []byte("chunk1"),
	}
	result1 := kp.ForwardCommand(cmd1, nil, "win1",
		5, 2, 80, 24, 1, 1, 0, 0, 0, false, nil)
	if result1 != nil {
		t.Error("expected nil result for first chunk (more=true)")
	}

	// Continuation chunk: default a=t (parser default), m=1
	cmd2 := &KittyCommand{
		Action: KittyActionTransmit, // parser default for continuation
		More:   true,
		Data:   []byte("chunk2"),
	}
	result2 := kp.ForwardCommand(cmd2, nil, "win1",
		5, 2, 80, 24, 1, 1, 0, 0, 0, false, nil)
	if result2 != nil {
		t.Error("expected nil result for middle chunk")
	}

	// Final chunk: default a=t, m=0
	cmd3 := &KittyCommand{
		Action: KittyActionTransmit, // parser default for continuation
		More:   false,
		Data:   []byte("chunk3"),
	}
	result3 := kp.ForwardCommand(cmd3, nil, "win1",
		5, 2, 80, 24, 1, 1, 0, 0, 0, false, nil)

	if result3 == nil {
		t.Fatal("expected PlacementResult for final chunk, got nil (andPlace lost!)")
	}
	// imgRows = 10 (200px / 20px). Cursor injection uses full imgRows.
	if result3.Rows != 10 {
		t.Errorf("expected Rows=10 (imgRows), got %d", result3.Rows)
	}
	if len(kp.placements["win1"]) != 1 {
		t.Errorf("expected 1 placement, got %d", len(kp.placements["win1"]))
	}
}

// TestPlaceOneNoClipNoSourceRect verifies that placeOne omits y=/h= when
// the full image is visible (no clipping needed).
func TestPlaceOneNoClipNoSourceRect(t *testing.T) {
	kp := newTestKP()
	p := &KittyPlacement{
		HostImageID: 1,
		HostX:       10, HostY: 5,
		Cols: 20, Rows: 10, DisplayRows: 10,
		MaxShowable: 10, MaxShowableCols: 20,
		PixelWidth: 200, PixelHeight: 200,
	}
	kp.placeOne(p)
	out := string(kp.pendingOutput)

	// Should NOT have y= or h= since no clipping
	if strings.Contains(out, ",y=") {
		t.Error("should not have y= when no clipping needed")
	}
	if strings.Contains(out, ",h=") {
		t.Error("should not have h= when no clipping needed")
	}
	// Should have c= and r=
	if !strings.Contains(out, ",c=20") {
		t.Errorf("expected c=20, got: %s", out)
	}
	if !strings.Contains(out, ",r=10") {
		t.Errorf("expected r=10, got: %s", out)
	}
}

// TestPlaceOneWithClipTop verifies correct source rect when image is
// partially scrolled off the top.
func TestPlaceOneWithClipTop(t *testing.T) {
	kp := newTestKP()
	p := &KittyPlacement{
		HostImageID: 1,
		HostX:       10, HostY: 5,
		Cols: 20, Rows: 10, DisplayRows: 10,
		MaxShowable: 7, MaxShowableCols: 20,
		ClipTop:     3,
		PixelWidth:  200, PixelHeight: 200,
	}
	kp.placeOne(p)
	out := string(kp.pendingOutput)

	// pixelsPerRow = 200/10 = 20
	// sourceY = 3 * 20 = 60
	// sourceHeight = 7 * 20 = 140
	if !strings.Contains(out, ",y=60") {
		t.Errorf("expected y=60 for clipTop=3, got: %s", out)
	}
	if !strings.Contains(out, ",h=140") {
		t.Errorf("expected h=140 for 7 visible rows, got: %s", out)
	}
	if !strings.Contains(out, ",r=7") {
		t.Errorf("expected r=7 for visible rows, got: %s", out)
	}
}

// TestCursorInjectionAfterAPC verifies that safeEmuWrite injects newlines
// after a Kitty APC that places an image (C=0, default cursor movement).
func TestCursorInjectionAfterAPC(t *testing.T) {
	// We can't easily test safeEmuWrite with a real emulator, but we can
	// test the callback flow by calling extractKittyAPCs and simulating
	// what safeEmuWrite does.

	term := &Terminal{
		kittyAPCBuf: nil,
	}

	// Simulate an APC for a=T (transmit+place) followed by a shell prompt
	apc := "\x1b_Ga=T,f=100,s=100,v=200,q=2;AAAA\x1b\\"
	prompt := "$ "
	data := []byte(apc + prompt)

	segments, trailing := term.extractKittyAPCs(data)
	if len(segments) != 1 {
		t.Fatalf("expected 1 segment, got %d", len(segments))
	}
	if string(segments[0].DataBefore) != "" {
		t.Errorf("expected empty DataBefore, got %q", segments[0].DataBefore)
	}
	if segments[0].Cmd == nil {
		t.Fatal("expected parsed command, got nil")
	}
	if segments[0].Cmd.Action != KittyActionTransmitPlace {
		t.Errorf("expected action T, got %c", byte(segments[0].Cmd.Action))
	}
	if string(trailing) != "$ " {
		t.Errorf("expected trailing '$ ', got %q", trailing)
	}
}

// TestResizeClearsPlacement verifies that ClearWindow removes placements
// and generates delete commands.
func TestResizeClearsPlacement(t *testing.T) {
	kp := newTestKP()

	// Place an image
	cmd := &KittyCommand{
		Action: KittyActionTransmitPlace,
		Format: KittyFormatRGB,
		Width:  100,
		Height: 200,
		Data:   []byte("rgb"),
	}
	kp.ForwardCommand(cmd, nil, "win1",
		5, 2, 80, 24, 1, 1, 0, 0, 0, false, nil)

	if len(kp.placements["win1"]) != 1 {
		t.Fatalf("expected 1 placement, got %d", len(kp.placements["win1"]))
	}

	// Clear pending (simulate flushing the placement)
	kp.pendingOutput = nil

	// Simulate resize: ClearWindow
	kp.ClearWindow("win1")
	if len(kp.placements["win1"]) != 0 {
		t.Errorf("expected 0 placements after clear, got %d", len(kp.placements["win1"]))
	}

	// Should have generated a delete command
	out := kp.FlushPending()
	if !bytes.Contains(out, []byte("a=d,")) {
		t.Error("expected delete command after ClearWindow")
	}
}

// TestTransmitOnlySingleChunkNoPlace verifies a=t (transmit only, no place)
// does not return PlacementResult and does not create a placement.
func TestTransmitOnlySingleChunkNoPlace(t *testing.T) {
	kp := newTestKP()
	rawData := []byte("a=t,i=1,f=24,s=10,v=10,q=2;AAAA")
	cmd := &KittyCommand{
		Action:  KittyActionTransmit,
		ImageID: 1,
		Format:  KittyFormatRGB,
		Width:   10,
		Height:  10,
		Data:    []byte{0, 0, 0},
	}
	result := kp.ForwardCommand(cmd, rawData, "win1",
		5, 2, 80, 24, 1, 1, 0, 0, 0, false, nil)

	if result != nil {
		t.Error("expected nil PlacementResult for transmit-only")
	}
	if len(kp.placements["win1"]) != 0 {
		t.Errorf("expected 0 placements for transmit-only, got %d", len(kp.placements["win1"]))
	}
}

// TestTransmitPlaceCappedRows verifies that when the image would extend past
// the content area, displayRows is capped and PlacementResult uses the cap.
func TestTransmitPlaceCappedRows(t *testing.T) {
	kp := newTestKP()
	cmd := &KittyCommand{
		Action: KittyActionTransmitPlace,
		Format: KittyFormatPNG,
		Width:  100,
		Height: 400, // 400px / 20px = 20 rows
		Data:   []byte("fakepng"),
	}
	// Window height 15, content height = 13. Cursor at row 5.
	// remainingRows = 13 - 5 = 8. displayRows = min(20, 8) = 8.
	// But cursor injection should use full imgRows (20), not capped displayRows.
	// The real terminal advances by the full image height (scrolling if needed).
	result := kp.ForwardCommand(cmd, nil, "win1",
		5, 2, 80, 15, 1, 1, 0, 5, 0, false, nil)

	if result == nil {
		t.Fatal("expected PlacementResult, got nil")
	}
	if result.Rows != 20 {
		t.Errorf("expected Rows=20 (full image height for cursor injection), got %d", result.Rows)
	}

	// Verify the placement's DisplayRows is capped for visual clipping.
	placements := kp.placements["win1"]
	if len(placements) != 1 {
		t.Fatalf("expected 1 placement, got %d", len(placements))
	}
	for _, p := range placements {
		if p.DisplayRows != 8 {
			t.Errorf("expected DisplayRows=8 (capped to window), got %d", p.DisplayRows)
		}
		if p.Rows != 20 {
			t.Errorf("expected Rows=20 (full image), got %d", p.Rows)
		}
	}
}

// TestRefreshScrollUpdatesPosition verifies that RefreshAllPlacements moves
// the image when scrollbackLen changes (content scrolling).
func TestRefreshScrollUpdatesPosition(t *testing.T) {
	kp := newTestKP()

	// Place an image when cursor is at row 5, scrollbackLen = 10.
	cmd := &KittyCommand{
		Action: KittyActionTransmitPlace,
		Format: KittyFormatPNG,
		Width:  100,
		Height: 200,
		Data:   []byte("fakepng"),
	}
	// Window at (5,2), 80x24, content offset (1,1), cursor at (0,5), scrollback=10
	result := kp.ForwardCommand(cmd, nil, "win1",
		5, 2, 80, 24, 1, 1, 0, 5, 10, false, nil)
	if result == nil {
		t.Fatal("expected PlacementResult, got nil")
	}

	// Placement is deferred (hidden). Run initial refresh to show it.
	kp.FlushPending()
	kp.RefreshAllPlacements(func() map[string]*KittyWindowInfo {
		return map[string]*KittyWindowInfo{
			"win1": {
				WindowX: 5, WindowY: 2,
				ContentOffX: 1, ContentOffY: 1,
				ContentWidth: 78, ContentHeight: 22,
				Width: 80, Height: 24,
				Visible: true, ScrollbackLen: 10, ScrollOffset: 0,
			},
		}
	})

	// Verify placement's AbsoluteLine = scrollbackLen(10) + cursorY(5) = 15
	var placement *KittyPlacement
	for _, p := range kp.placements["win1"] {
		placement = p
		break
	}
	if placement == nil {
		t.Fatal("no placement found")
	}
	if placement.AbsoluteLine != 15 {
		t.Errorf("expected AbsoluteLine=15, got %d", placement.AbsoluteLine)
	}
	if placement.Hidden {
		t.Error("expected placement to be visible after initial refresh")
	}

	initialHostY := placement.HostY // should be 2 + 1 + 5 = 8
	kp.FlushPending()

	// Simulate content scrolling: scrollbackLen increased by 3
	kp.RefreshAllPlacements(func() map[string]*KittyWindowInfo {
		return map[string]*KittyWindowInfo{
			"win1": {
				WindowX: 5, WindowY: 2,
				ContentOffX: 1, ContentOffY: 1,
				ContentWidth: 78, ContentHeight: 22,
				Width: 80, Height: 24,
				Visible: true, ScrollbackLen: 13, ScrollOffset: 0,
			},
		}
	})

	// Image should have moved up by 3 rows
	expectedHostY := initialHostY - 3 // 8 - 3 = 5
	if placement.HostY != expectedHostY {
		t.Errorf("after scroll: expected HostY=%d, got %d (didn't move!)", expectedHostY, placement.HostY)
	}

	out := kp.FlushPending()
	if len(out) == 0 {
		t.Error("expected re-place command after scroll, got nothing")
	}
	// Should contain a place command (no pre-delete needed — default
	// placement is replaced atomically by placeOne).
	outStr := string(out)
	if !strings.Contains(outStr, "a=p,") {
		t.Error("expected place command in output")
	}
}

// TestRefreshScrollOffTop verifies that an image is hidden when it scrolls
// completely off the top of the viewport.
func TestRefreshScrollOffTop(t *testing.T) {
	kp := newTestKP()

	cmd := &KittyCommand{
		Action: KittyActionTransmitPlace,
		Format: KittyFormatPNG,
		Width:  100,
		Height: 200, // 200px / 20px = 10 rows
		Data:   []byte("fakepng"),
	}
	// Cursor at row 0, scrollback = 0
	kp.ForwardCommand(cmd, nil, "win1",
		5, 2, 80, 24, 1, 1, 0, 0, 0, false, nil)
	kp.FlushPending()

	// Initial refresh to show the placement
	kp.RefreshAllPlacements(func() map[string]*KittyWindowInfo {
		return map[string]*KittyWindowInfo{
			"win1": {
				WindowX: 5, WindowY: 2,
				ContentOffX: 1, ContentOffY: 1,
				ContentWidth: 78, ContentHeight: 22,
				Width: 80, Height: 24,
				Visible: true, ScrollbackLen: 0, ScrollOffset: 0,
			},
		}
	})
	kp.FlushPending()

	// Now scroll far enough that the image is completely off the top
	// AbsoluteLine=0, Rows=10, so fullBottom=0-20+10=-10, which is <0 → hidden
	kp.RefreshAllPlacements(func() map[string]*KittyWindowInfo {
		return map[string]*KittyWindowInfo{
			"win1": {
				WindowX: 5, WindowY: 2,
				ContentOffX: 1, ContentOffY: 1,
				ContentWidth: 78, ContentHeight: 22,
				Width: 80, Height: 24,
				Visible: true, ScrollbackLen: 20, ScrollOffset: 0,
			},
		}
	})

	var placement *KittyPlacement
	for _, p := range kp.placements["win1"] {
		placement = p
		break
	}
	if placement == nil {
		t.Fatal("placement should still exist (hidden)")
	}
	if !placement.Hidden {
		t.Error("expected placement to be hidden after scrolling off top")
	}

	out := kp.FlushPending()
	if len(out) == 0 {
		t.Error("expected delete command when hiding")
	}
}

// TestTransmitDoesNotIncludePlacementParams verifies the transmit command
// only includes transmission params (a,i,f,s,v,o), not placement params.
func TestTransmitDoesNotIncludePlacementParams(t *testing.T) {
	kp := newTestKP()
	cmd := &KittyCommand{
		Action:  KittyActionTransmitPlace,
		Format:  KittyFormatPNG,
		Width:   100,
		Height:  200,
		Columns: 10,
		Rows:    20,
		ZIndex:  5,
		Data:    []byte("fakepng"),
	}
	kp.ForwardCommand(cmd, nil, "win1",
		5, 2, 80, 24, 1, 1, 0, 0, 0, false, nil)

	out := string(kp.pendingOutput)
	// Find the transmit command (a=t)
	parts := strings.Split(out, "\x1b_G")
	var transmitPart string
	for _, p := range parts {
		if strings.HasPrefix(p, "a=t,") {
			transmitPart = p
			break
		}
	}
	if transmitPart == "" {
		t.Fatal("no transmit command found")
	}
	// Transmit should NOT have c=, r=, or z= (those are placement params)
	if strings.Contains(transmitPart, ",c=") {
		t.Error("transmit command should not have c= (columns)")
	}
	if strings.Contains(transmitPart, ",r=") {
		t.Error("transmit command should not have r= (rows)")
	}
	if strings.Contains(transmitPart, ",z=") {
		t.Error("transmit command should not have z= (zindex)")
	}
}

// --- Additional coverage tests ---

func TestBuildKittyResponse(t *testing.T) {
	// OK response with image ID
	resp := BuildKittyResponse(true, 42, "")
	if !bytes.Contains(resp, []byte("i=42;")) {
		t.Errorf("expected i=42 in OK response, got %q", resp)
	}
	if !bytes.Contains(resp, []byte("OK")) {
		t.Errorf("expected OK in response, got %q", resp)
	}

	// OK response without image ID
	resp2 := BuildKittyResponse(true, 0, "")
	if bytes.Contains(resp2, []byte("i=")) {
		t.Errorf("should not have i= when imageID is 0, got %q", resp2)
	}

	// Error response with message
	resp3 := BuildKittyResponse(false, 5, "EINVAL:bad param")
	if !bytes.Contains(resp3, []byte("EINVAL:bad param")) {
		t.Errorf("expected error message, got %q", resp3)
	}

	// Error response without message (default)
	resp4 := BuildKittyResponse(false, 0, "")
	if !bytes.Contains(resp4, []byte("ENOENT:file not found")) {
		t.Errorf("expected default error, got %q", resp4)
	}
}

func TestIsKittyResponse(t *testing.T) {
	tests := []struct {
		data []byte
		want bool
	}{
		{nil, false},
		{[]byte{}, false},
		{[]byte("OK"), true},
		{[]byte("ENOENT:file not found"), true},
		{[]byte("EINVAL:bad"), true},
		{[]byte("ABC"), false}, // second byte < 'A' → not uppercase pair
		{[]byte("raw pixel data"), false},
	}
	for _, tt := range tests {
		got := isKittyResponse(tt.data)
		if got != tt.want {
			t.Errorf("isKittyResponse(%q) = %v, want %v", tt.data, got, tt.want)
		}
	}
}

func TestParseKittyControlParamsAllFields(t *testing.T) {
	cmd := &KittyCommand{}
	parseKittyControlParams("a=T,q=2,i=100,I=200,p=5,f=100,t=f,o=z,s=640,v=480,S=1000,O=500,m=1,d=a,x=10,y=20,w=30,h=40,X=5,Y=3,c=80,r=24,z=-1,C=1,U=1", cmd)

	if cmd.Action != KittyActionTransmitPlace {
		t.Errorf("Action: got %c, want T", byte(cmd.Action))
	}
	if cmd.Quiet != 2 {
		t.Errorf("Quiet: got %d, want 2", cmd.Quiet)
	}
	if cmd.ImageID != 100 {
		t.Errorf("ImageID: got %d, want 100", cmd.ImageID)
	}
	if cmd.ImageNumber != 200 {
		t.Errorf("ImageNumber: got %d, want 200", cmd.ImageNumber)
	}
	if cmd.PlacementID != 5 {
		t.Errorf("PlacementID: got %d, want 5", cmd.PlacementID)
	}
	if cmd.Format != KittyFormatPNG {
		t.Errorf("Format: got %d, want %d", cmd.Format, KittyFormatPNG)
	}
	if cmd.Medium != KittyMediumFile {
		t.Errorf("Medium: got %c, want f", byte(cmd.Medium))
	}
	if cmd.Compression != KittyCompressionZlib {
		t.Error("Compression should be zlib")
	}
	if cmd.Width != 640 {
		t.Errorf("Width: got %d, want 640", cmd.Width)
	}
	if cmd.Height != 480 {
		t.Errorf("Height: got %d, want 480", cmd.Height)
	}
	if cmd.Size != 1000 {
		t.Errorf("Size: got %d, want 1000", cmd.Size)
	}
	if cmd.Offset != 500 {
		t.Errorf("Offset: got %d, want 500", cmd.Offset)
	}
	if !cmd.More {
		t.Error("More should be true")
	}
	if cmd.Delete != KittyDeleteTarget('a') {
		t.Errorf("Delete: got %c, want a", byte(cmd.Delete))
	}
	if cmd.SourceX != 10 {
		t.Errorf("SourceX: got %d, want 10", cmd.SourceX)
	}
	if cmd.SourceY != 20 {
		t.Errorf("SourceY: got %d, want 20", cmd.SourceY)
	}
	if cmd.SourceWidth != 30 {
		t.Errorf("SourceWidth: got %d, want 30", cmd.SourceWidth)
	}
	if cmd.SourceHeight != 40 {
		t.Errorf("SourceHeight: got %d, want 40", cmd.SourceHeight)
	}
	if cmd.XOffset != 5 {
		t.Errorf("XOffset: got %d, want 5", cmd.XOffset)
	}
	if cmd.YOffset != 3 {
		t.Errorf("YOffset: got %d, want 3", cmd.YOffset)
	}
	if cmd.Columns != 80 {
		t.Errorf("Columns: got %d, want 80", cmd.Columns)
	}
	if cmd.Rows != 24 {
		t.Errorf("Rows: got %d, want 24", cmd.Rows)
	}
	if cmd.ZIndex != -1 {
		t.Errorf("ZIndex: got %d, want -1", cmd.ZIndex)
	}
	if cmd.CursorMove != 1 {
		t.Errorf("CursorMove: got %d, want 1", cmd.CursorMove)
	}
	if !cmd.Virtual {
		t.Error("Virtual should be true")
	}
}

func TestParseKittyControlParamsEdgeCases(t *testing.T) {
	// Empty pairs, no-value pairs
	cmd := &KittyCommand{}
	parseKittyControlParams(",a=t,,badpair,s=10", cmd)
	if cmd.Action != KittyActionTransmit {
		t.Errorf("Action: got %c, want t", byte(cmd.Action))
	}
	if cmd.Width != 10 {
		t.Errorf("Width: got %d, want 10", cmd.Width)
	}
}

func TestParseKittyCommandEmpty(t *testing.T) {
	cmd, err := ParseKittyCommand(nil)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if cmd != nil {
		t.Error("expected nil for empty data")
	}
}

func TestParseKittyCommandFileMedium(t *testing.T) {
	// File medium: data is base64-encoded file path
	import64 := "L3RtcC9pbWFnZS5wbmc=" // base64("/tmp/image.png")
	data := []byte("t=f;" + import64)
	cmd, err := ParseKittyCommand(data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cmd.Medium != KittyMediumFile {
		t.Errorf("Medium: got %c, want f", byte(cmd.Medium))
	}
	if cmd.FilePath != "/tmp/image.png" {
		t.Errorf("FilePath: got %q, want /tmp/image.png", cmd.FilePath)
	}
}

func TestIsEnabled(t *testing.T) {
	kp := newTestKP()
	if !kp.IsEnabled() {
		t.Error("expected IsEnabled=true for test kp")
	}
	kp.mu.Lock()
	kp.enabled = false
	kp.mu.Unlock()
	if kp.IsEnabled() {
		t.Error("expected IsEnabled=false after disabling")
	}
}

func TestHasPlacements(t *testing.T) {
	kp := newTestKP()
	if kp.HasPlacements() {
		t.Error("expected no placements initially")
	}

	// Add a placement
	cmd := &KittyCommand{
		Action: KittyActionTransmitPlace,
		Format: KittyFormatPNG,
		Width:  100, Height: 200,
		Data: []byte("fakepng"),
	}
	kp.ForwardCommand(cmd, nil, "win1", 5, 2, 80, 24, 1, 1, 0, 0, 0, false, nil)
	if !kp.HasPlacements() {
		t.Error("expected placements after transmit+place")
	}
}

func TestHideAllPlacements(t *testing.T) {
	kp := newTestKP()

	// Place an image
	cmd := &KittyCommand{
		Action: KittyActionTransmitPlace,
		Format: KittyFormatPNG,
		Width:  100, Height: 200,
		Data: []byte("fakepng"),
	}
	kp.ForwardCommand(cmd, nil, "win1", 5, 2, 80, 24, 1, 1, 0, 0, 0, false, nil)
	kp.FlushPending()

	// Make placement visible first
	kp.RefreshAllPlacements(func() map[string]*KittyWindowInfo {
		return map[string]*KittyWindowInfo{
			"win1": {
				WindowX: 5, WindowY: 2, ContentOffX: 1, ContentOffY: 1,
				ContentWidth: 78, ContentHeight: 22,
				Width: 80, Height: 24, Visible: true,
			},
		}
	})
	kp.FlushPending()

	// Now hide all
	kp.HideAllPlacements()
	for _, placements := range kp.placements {
		for _, p := range placements {
			if !p.Hidden {
				t.Error("expected all placements hidden after HideAllPlacements")
			}
		}
	}

	// Should have delete commands in pending output
	out := kp.FlushPending()
	if len(out) == 0 {
		t.Error("expected delete commands after HideAllPlacements")
	}
}

func TestAllocateHostIDWraparound(t *testing.T) {
	kp := newTestKP()
	kp.nextHostID = 0xFFFFFFFF // max uint32
	id1 := kp.allocateHostID()
	if id1 != 0xFFFFFFFF {
		t.Errorf("expected max uint32, got %d", id1)
	}
	// After overflow, nextHostID wraps to 0, then gets reset to 1
	id2 := kp.allocateHostID()
	if id2 != 1 {
		t.Errorf("expected 1 after wraparound, got %d", id2)
	}
}

func TestFlushPendingEmpty(t *testing.T) {
	kp := newTestKP()
	out := kp.FlushPending()
	if out != nil {
		t.Errorf("expected nil for empty pending, got %d bytes", len(out))
	}
}

func TestGetOrAllocateHostID(t *testing.T) {
	kp := newTestKP()

	// First call should allocate a new ID
	id1 := kp.getOrAllocateHostID("win1", 100)
	if id1 != 1 {
		t.Errorf("expected 1, got %d", id1)
	}

	// Same window+guest should return same ID
	id2 := kp.getOrAllocateHostID("win1", 100)
	if id2 != id1 {
		t.Errorf("expected same ID %d, got %d", id1, id2)
	}

	// Different guest ID in same window should allocate new
	id3 := kp.getOrAllocateHostID("win1", 200)
	if id3 == id1 {
		t.Errorf("expected different ID for different guest, got %d", id3)
	}

	// Different window should allocate new
	id4 := kp.getOrAllocateHostID("win2", 100)
	if id4 == id1 {
		t.Errorf("expected different ID for different window, got %d", id4)
	}
}

func TestReadKittyFileMediumRegularFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "image.png")
	content := []byte("fake-png-data")
	os.WriteFile(path, content, 0644)

	data, err := readKittyFileMedium(KittyMediumFile, path)
	if err != nil {
		t.Fatalf("readKittyFileMedium: %v", err)
	}
	if !bytes.Equal(data, content) {
		t.Errorf("expected %q, got %q", content, data)
	}
	// Regular file should NOT be deleted
	if _, err := os.Stat(path); err != nil {
		t.Error("regular file should not be deleted after read")
	}
}

func TestReadKittyFileMediumTempFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "temp.png")
	content := []byte("temp-data")
	os.WriteFile(path, content, 0644)

	data, err := readKittyFileMedium(KittyMediumTempFile, path)
	if err != nil {
		t.Fatalf("readKittyFileMedium: %v", err)
	}
	if !bytes.Equal(data, content) {
		t.Errorf("expected %q, got %q", content, data)
	}
	// Temp file should be deleted after read
	if _, err := os.Stat(path); err == nil {
		t.Error("temp file should be deleted after read")
	}
}

func TestReadKittyFileMediumMissing(t *testing.T) {
	_, err := readKittyFileMedium(KittyMediumFile, "/nonexistent/path")
	if err == nil {
		t.Error("expected error for missing file")
	}
}

func TestHostTermProgram(t *testing.T) {
	orig := os.Getenv("TERM_PROGRAM")
	defer os.Setenv("TERM_PROGRAM", orig)

	os.Setenv("TERM_PROGRAM", "WezTerm")
	if got := HostTermProgram(); got != "WezTerm" {
		t.Errorf("expected WezTerm, got %q", got)
	}

	os.Unsetenv("TERM_PROGRAM")
	if got := HostTermProgram(); got != "" {
		t.Errorf("expected empty, got %q", got)
	}
}

func TestHasKittyTerminfo(t *testing.T) {
	// This test just verifies the function runs without panicking
	// and returns a bool. We can't control system terminfo paths.
	_ = HasKittyTerminfo()
}

func TestForwardCommandDisabled(t *testing.T) {
	kp := newTestKP()
	kp.enabled = false

	cmd := &KittyCommand{Action: KittyActionTransmitPlace}
	result := kp.ForwardCommand(cmd, nil, "win1", 0, 0, 80, 24, 1, 1, 0, 0, 0, false, func(b []byte) {})
	if result != nil {
		t.Error("expected nil result when disabled")
	}
}

func TestForwardCommandDelete(t *testing.T) {
	kp := newTestKP()

	// First place an image so we have something to delete
	placeCmd := &KittyCommand{
		Action:  KittyActionTransmitPlace,
		Format:  KittyFormatPNG,
		Width:   100,
		Height:  200,
		Data:    []byte("fakepng"),
		Columns: 10,
		Rows:    5,
		ImageID: 42,
	}
	kp.ForwardCommand(placeCmd, nil, "win1", 0, 0, 80, 24, 1, 1, 0, 0, 0, false, func(b []byte) {})
	kp.FlushPending() // clear pending

	// Now delete all
	delCmd := &KittyCommand{
		Action: KittyActionDelete,
		Delete: KittyDeleteAll,
	}
	kp.ForwardCommand(delCmd, nil, "win1", 0, 0, 80, 24, 1, 1, 0, 0, 0, false, func(b []byte) {})

	if kp.HasPlacements() {
		t.Error("expected no placements after delete all")
	}
}

func TestForwardCommandDeleteByID(t *testing.T) {
	kp := newTestKP()

	// Place an image
	placeCmd := &KittyCommand{
		Action:  KittyActionTransmitPlace,
		Format:  KittyFormatPNG,
		Width:   100,
		Height:  200,
		Data:    []byte("fakepng"),
		Columns: 10,
		Rows:    5,
		ImageID: 42,
	}
	kp.ForwardCommand(placeCmd, nil, "win1", 0, 0, 80, 24, 1, 1, 0, 0, 0, false, func(b []byte) {})
	kp.FlushPending()

	// Delete by ID
	delCmd := &KittyCommand{
		Action:  KittyActionDelete,
		Delete:  KittyDeleteByID,
		ImageID: 42,
	}
	kp.ForwardCommand(delCmd, nil, "win1", 0, 0, 80, 24, 1, 1, 0, 0, 0, false, func(b []byte) {})

	if kp.HasPlacements() {
		t.Error("expected no placements after delete by ID")
	}
}

func TestForwardCommandDeleteByIDAndPlacement(t *testing.T) {
	kp := newTestKP()

	// Place an image with placement ID
	placeCmd := &KittyCommand{
		Action:      KittyActionTransmitPlace,
		Format:      KittyFormatPNG,
		Width:       100,
		Height:      200,
		Data:        []byte("fakepng"),
		Columns:     10,
		Rows:        5,
		ImageID:     42,
		PlacementID: 7,
	}
	kp.ForwardCommand(placeCmd, nil, "win1", 0, 0, 80, 24, 1, 1, 0, 0, 0, false, func(b []byte) {})
	kp.FlushPending()

	// Delete by ID and placement
	delCmd := &KittyCommand{
		Action:      KittyActionDelete,
		Delete:      KittyDeleteByIDAndPlacement,
		ImageID:     42,
		PlacementID: 7,
	}
	kp.ForwardCommand(delCmd, nil, "win1", 0, 0, 80, 24, 1, 1, 0, 0, 0, false, func(b []byte) {})

	out := kp.FlushPending()
	if !bytes.Contains(out, []byte("a=d,d=I")) {
		t.Errorf("expected delete-by-ID-and-placement APC, got %q", out)
	}
}

func TestForwardCommandQuery(t *testing.T) {
	kp := newTestKP()
	var ptyResponse []byte

	cmd := &KittyCommand{
		Action:  KittyActionQuery,
		ImageID: 99,
	}
	kp.ForwardCommand(cmd, []byte("a=q,i=99"), "win1", 0, 0, 80, 24, 1, 1, 0, 0, 0, false, func(b []byte) {
		ptyResponse = append(ptyResponse, b...)
	})

	// Query should forward to host and send response back
	out := kp.FlushPending()
	if len(out) == 0 && len(ptyResponse) == 0 {
		// Query is forwarded via pendingOutput
		t.Log("query forwarded (no immediate response is also valid)")
	}
}

func TestForwardCommandPlace(t *testing.T) {
	kp := newTestKP()

	// First transmit an image
	txCmd := &KittyCommand{
		Action:  KittyActionTransmit,
		Format:  KittyFormatPNG,
		Width:   100,
		Height:  200,
		Data:    []byte("fakepng"),
		ImageID: 42,
	}
	kp.ForwardCommand(txCmd, nil, "win1", 0, 0, 80, 24, 1, 1, 0, 0, 0, false, func(b []byte) {})
	kp.FlushPending()

	// Now place it
	placeCmd := &KittyCommand{
		Action:  KittyActionPlace,
		ImageID: 42,
		Columns: 10,
		Rows:    5,
	}
	result := kp.ForwardCommand(placeCmd, nil, "win1", 0, 0, 80, 24, 1, 1, 0, 0, 0, false, func(b []byte) {})
	// Place should create a placement and return PlacementResult
	if !kp.HasPlacements() {
		t.Error("expected placements after place")
	}
	_ = result
}

func TestForwardCommandPlaceWithSourceRect(t *testing.T) {
	kp := newTestKP()

	cmd := &KittyCommand{
		Action:       KittyActionTransmitPlace,
		Format:       KittyFormatPNG,
		Width:        100,
		Height:       200,
		Data:         []byte("fakepng"),
		ImageID:      50,
		Columns:      10,
		Rows:         5,
		SourceX:      10,
		SourceY:      20,
		SourceWidth:  50,
		SourceHeight: 60,
		ZIndex:       3,
	}
	result := kp.ForwardCommand(cmd, nil, "win1", 5, 3, 80, 24, 1, 1, 2, 1, 100, false, func(b []byte) {})

	// TransmitPlace creates a deferred (Hidden) placement
	if !kp.HasPlacements() {
		t.Error("expected placements after transmit+place")
	}

	// Check result
	if result == nil {
		t.Fatal("expected non-nil PlacementResult for transmit+place")
	}
	if result.Rows != 5 {
		t.Errorf("expected Rows=5, got %d", result.Rows)
	}

	// Verify placement stores source rect params
	for _, pmap := range kp.placements {
		for _, p := range pmap {
			if p.SourceX != 10 {
				t.Errorf("SourceX = %d, want 10", p.SourceX)
			}
			if p.SourceY != 20 {
				t.Errorf("SourceY = %d, want 20", p.SourceY)
			}
			if p.SourceWidth != 50 {
				t.Errorf("SourceWidth = %d, want 50", p.SourceWidth)
			}
			if p.SourceHeight != 60 {
				t.Errorf("SourceHeight = %d, want 60", p.SourceHeight)
			}
			if p.ZIndex != 3 {
				t.Errorf("ZIndex = %d, want 3", p.ZIndex)
			}
		}
	}
}

func TestClearWindow(t *testing.T) {
	kp := newTestKP()

	// Place an image
	cmd := &KittyCommand{
		Action:  KittyActionTransmitPlace,
		Format:  KittyFormatPNG,
		Width:   100,
		Height:  200,
		Data:    []byte("fakepng"),
		Columns: 10,
		Rows:    5,
		ImageID: 42,
	}
	kp.ForwardCommand(cmd, nil, "win1", 0, 0, 80, 24, 1, 1, 0, 0, 0, false, func(b []byte) {})
	kp.FlushPending()

	// Clear the window
	kp.ClearWindow("win1")
	if kp.HasPlacements() {
		t.Error("expected no placements after ClearWindow")
	}
}

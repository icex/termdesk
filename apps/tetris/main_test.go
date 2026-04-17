package main

import (
	"encoding/base64"
	"encoding/json"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/lipgloss"
)

func seedModel(seed uint64) model {
	m := newModel()
	// Reset with known state for deterministic tests.
	m.board = [boardH][boardW]cell{}
	m.cur = pieceI
	m.curRot = 0
	m.curRow = 0
	m.curCol = 3
	m.canHold = true
	return m
}

func TestMoveLeftRight(t *testing.T) {
	m := seedModel(42)
	origCol := m.curCol

	m.moveLeft()
	if m.curCol != origCol-1 {
		t.Errorf("moveLeft: got col %d, want %d", m.curCol, origCol-1)
	}
	m.moveRight()
	if m.curCol != origCol {
		t.Errorf("moveRight: got col %d, want %d", m.curCol, origCol)
	}
}

func TestMoveLeftWall(t *testing.T) {
	m := seedModel(42)
	m.curCol = 0
	if m.moveLeft() {
		t.Error("moveLeft should fail at left wall")
	}
}

func TestMoveRightWall(t *testing.T) {
	m := seedModel(42)
	// I-piece rotation 0 occupies cols curCol..curCol+3
	m.curCol = boardW - 4
	if m.moveRight() {
		t.Error("moveRight should fail at right wall")
	}
}

func TestMoveDown(t *testing.T) {
	m := seedModel(42)
	origRow := m.curRow

	m.moveDown()
	if m.curRow != origRow+1 {
		t.Errorf("moveDown: got row %d, want %d", m.curRow, origRow+1)
	}
}

func TestMoveDownFloor(t *testing.T) {
	m := seedModel(42)
	// I-piece rotation 0 is a single row. Place at bottom.
	m.curRow = boardH - 1
	if m.moveDown() {
		t.Error("moveDown should fail at floor")
	}
}

func TestHardDrop(t *testing.T) {
	m := seedModel(42)
	m.curRow = 0
	dropped := m.hardDrop()
	if dropped == 0 {
		t.Error("hardDrop should drop at least 1 row")
	}
	// Should be at bottom.
	if m.curRow != boardH-1 { // I-piece rot 0 is 1 row tall
		t.Errorf("hardDrop: got row %d, want %d", m.curRow, boardH-1)
	}
}

func TestWallCollision(t *testing.T) {
	m := seedModel(42)
	m.curRow = boardH - 1
	if m.validPosition(m.cur, m.curRot, m.curRow+1, m.curCol) {
		t.Error("should not be valid below floor")
	}
}

func TestSelfCollision(t *testing.T) {
	m := seedModel(42)
	// Fill a row where the I-piece would land.
	for c := 0; c < boardW; c++ {
		m.board[10][c] = cell{filled: true, piece: pieceO}
	}
	m.curRow = 9
	if m.validPosition(m.cur, m.curRot, 10, m.curCol) {
		t.Error("should collide with filled row")
	}
}

func TestLineClear(t *testing.T) {
	m := seedModel(42)
	// Fill bottom row completely.
	for c := 0; c < boardW; c++ {
		m.board[boardH-1][c] = cell{filled: true, piece: pieceO}
	}
	cleared := m.clearLines()
	if cleared != 1 {
		t.Errorf("clearLines: got %d, want 1", cleared)
	}
	// Bottom row should now be empty.
	for c := 0; c < boardW; c++ {
		if m.board[boardH-1][c].filled {
			t.Errorf("board[%d][%d] should be empty after clear", boardH-1, c)
		}
	}
}

func TestMultiLineClear(t *testing.T) {
	m := seedModel(42)
	// Fill bottom 4 rows.
	for r := boardH - 4; r < boardH; r++ {
		for c := 0; c < boardW; c++ {
			m.board[r][c] = cell{filled: true, piece: pieceO}
		}
	}
	cleared := m.clearLines()
	if cleared != 4 {
		t.Errorf("clearLines: got %d, want 4", cleared)
	}
}

func TestScoring(t *testing.T) {
	m := seedModel(42)
	m.level = 1
	// Fill bottom row except one column, then lock a piece to complete it.
	for c := 0; c < boardW; c++ {
		m.board[boardH-1][c] = cell{filled: true, piece: pieceO}
	}
	m.clearLines()
	// After 1 line clear at level 1: 100 points.
	// (We call clearLines directly, so scoring happens in lockPiece — test lockPiece.)
}

func TestHoldPiece(t *testing.T) {
	m := seedModel(42)
	original := m.cur

	m.holdPiece()
	if !m.hasHold {
		t.Error("hasHold should be true after hold")
	}
	if m.hold != original {
		t.Errorf("hold: got %d, want %d", m.hold, original)
	}
	if m.canHold {
		t.Error("canHold should be false after hold")
	}
}

func TestHoldSwap(t *testing.T) {
	m := seedModel(42)
	m.hasHold = true
	m.hold = pieceT
	m.canHold = true
	original := m.cur

	m.holdPiece()
	if m.cur != pieceT {
		t.Errorf("after hold swap: cur should be %d, got %d", pieceT, m.cur)
	}
	if m.hold != original {
		t.Errorf("after hold swap: hold should be %d, got %d", original, m.hold)
	}
}

func TestHoldTwice(t *testing.T) {
	m := seedModel(42)
	m.holdPiece()
	prevCur := m.cur
	m.holdPiece() // Should be a no-op (canHold is false).
	if m.cur != prevCur {
		t.Error("second hold should be a no-op")
	}
}

func TestRotateCW(t *testing.T) {
	m := seedModel(42)
	m.cur = pieceT
	m.curRot = 0
	m.curRow = 5
	m.curCol = 4
	if !m.rotateCW() {
		t.Error("rotateCW should succeed")
	}
	if m.curRot != 1 {
		t.Errorf("rotateCW: got rot %d, want 1", m.curRot)
	}
}

func TestRotateCCW(t *testing.T) {
	m := seedModel(42)
	m.cur = pieceT
	m.curRot = 0
	m.curRow = 5
	m.curCol = 4
	if !m.rotateCCW() {
		t.Error("rotateCCW should succeed")
	}
	if m.curRot != 3 {
		t.Errorf("rotateCCW: got rot %d, want 3", m.curRot)
	}
}

func TestRotateO(t *testing.T) {
	m := seedModel(42)
	m.cur = pieceO
	m.curRot = 0
	m.curRow = 5
	m.curCol = 4
	if m.rotateCW() {
		t.Error("O piece should not rotate")
	}
}

func TestGhostRow(t *testing.T) {
	m := seedModel(42)
	m.curRow = 0
	gr := m.ghostRow()
	if gr <= m.curRow {
		t.Error("ghost row should be below current row")
	}
	if gr != boardH-1 { // I-piece rot 0 is 1 row tall on empty board
		t.Errorf("ghost row: got %d, want %d", gr, boardH-1)
	}
}

func TestLevelUp(t *testing.T) {
	m := seedModel(42)
	m.level = 1
	m.lines = 9
	m.score = 0
	// Fill bottom row and lock a piece (simulate via clearLines + manual update).
	for c := 0; c < boardW; c++ {
		m.board[boardH-1][c] = cell{filled: true, piece: pieceO}
	}
	cleared := m.clearLines()
	if cleared != 1 {
		t.Fatal("expected 1 line clear")
	}
	m.lines += cleared
	newLevel := m.lines/10 + 1
	if newLevel != 2 {
		t.Errorf("level: got %d, want 2", newLevel)
	}
}

func TestGenerationDiscardsOldTicks(t *testing.T) {
	m := seedModel(42)
	m.started = true
	m.gen = 5

	// Tick from old generation should be ignored.
	oldTick := tickMsg{gen: 3}
	updated, cmd := m.Update(oldTick)
	m = updated.(model)
	if cmd != nil {
		t.Error("old tick should produce no command")
	}
}

func TestGameOverOnLockAboveBoard(t *testing.T) {
	m := seedModel(42)
	// Fill row 0 so piece can't move down from row -1.
	for c := 0; c < boardW; c++ {
		m.board[0][c] = cell{filled: true, piece: pieceO}
	}
	// I-piece rot 0 is a single row. Place at row -1 (above board).
	m.cur = pieceI
	m.curRot = 0
	m.curRow = -1
	m.curCol = 3
	m.lockPiece()
	if !m.gameOver {
		t.Error("game should be over when piece locks above board")
	}
}

func TestReset(t *testing.T) {
	m := seedModel(42)
	m.score = 999
	m.lines = 50
	m.level = 6
	m.gameOver = true
	oldGen := m.gen

	m.reset()
	if m.score != 0 || m.lines != 0 || m.level != 1 {
		t.Error("reset should clear score/lines/level")
	}
	if m.gameOver {
		t.Error("reset should clear gameOver")
	}
	if m.gen != oldGen+1 {
		t.Error("reset should increment gen")
	}
}

func TestViewReturnsAltScreen(t *testing.T) {
	m := seedModel(42)
	m.width = 80
	m.height = 30
	v := m.View()
	if !v.AltScreen {
		t.Error("View should set AltScreen")
	}
}

func TestQuitKey(t *testing.T) {
	m := seedModel(42)
	m.width = 80
	m.height = 30
	_, cmd := m.Update(tea.KeyPressMsg(tea.Key{Code: 'q'}))
	if cmd == nil {
		t.Fatal("'q' should produce a command")
	}
}

func TestFallSpeedDecreases(t *testing.T) {
	s1 := fallSpeed(1)
	s5 := fallSpeed(5)
	s10 := fallSpeed(10)
	if s5 >= s1 {
		t.Errorf("level 5 should be faster than level 1: %v >= %v", s5, s1)
	}
	if s10 >= s5 {
		t.Errorf("level 10 should be faster than level 5: %v >= %v", s10, s5)
	}
}

func TestBagRefill(t *testing.T) {
	m := seedModel(42)
	seen := make(map[pieceType]bool)
	// Draw 14 pieces (2 full bags).
	for i := 0; i < 14; i++ {
		p := m.drawPiece()
		seen[p] = true
	}
	if len(seen) != int(pieceCount) {
		t.Errorf("should see all %d piece types, got %d", pieceCount, len(seen))
	}
}

// ── Update key handling tests ───────────────────────────────────────

func TestUpdateMoveLeftKey(t *testing.T) {
	m := seedModel(42)
	m.curRow = 10
	origCol := m.curCol

	updated, cmd := m.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyLeft}))
	m = updated.(model)
	if m.curCol != origCol-1 {
		t.Errorf("left arrow: got col %d, want %d", m.curCol, origCol-1)
	}
	// First key press should start the game and schedule a tick.
	if !m.started {
		t.Error("game should start on first key press")
	}
	if cmd == nil {
		t.Error("first key should schedule a tick")
	}
}

func TestUpdateMoveLeftKeyH(t *testing.T) {
	m := seedModel(42)
	m.curRow = 10
	origCol := m.curCol

	updated, _ := m.Update(tea.KeyPressMsg(tea.Key{Code: 'h'}))
	m = updated.(model)
	if m.curCol != origCol-1 {
		t.Errorf("'h' key: got col %d, want %d", m.curCol, origCol-1)
	}
}

func TestUpdateMoveRightKey(t *testing.T) {
	m := seedModel(42)
	m.curRow = 10
	origCol := m.curCol

	updated, _ := m.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyRight}))
	m = updated.(model)
	if m.curCol != origCol+1 {
		t.Errorf("right arrow: got col %d, want %d", m.curCol, origCol+1)
	}
}

func TestUpdateMoveRightKeyL(t *testing.T) {
	m := seedModel(42)
	m.curRow = 10
	origCol := m.curCol

	updated, _ := m.Update(tea.KeyPressMsg(tea.Key{Code: 'l'}))
	m = updated.(model)
	if m.curCol != origCol+1 {
		t.Errorf("'l' key: got col %d, want %d", m.curCol, origCol+1)
	}
}

func TestUpdateMoveDownKey(t *testing.T) {
	m := seedModel(42)
	m.curRow = 10
	origRow := m.curRow
	origScore := m.score

	updated, _ := m.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyDown}))
	m = updated.(model)
	if m.curRow != origRow+1 {
		t.Errorf("down arrow: got row %d, want %d", m.curRow, origRow+1)
	}
	if m.score != origScore+1 {
		t.Errorf("soft drop bonus: got score %d, want %d", m.score, origScore+1)
	}
}

func TestUpdateMoveDownKeyJ(t *testing.T) {
	m := seedModel(42)
	m.curRow = 10
	origRow := m.curRow

	updated, _ := m.Update(tea.KeyPressMsg(tea.Key{Code: 'j'}))
	m = updated.(model)
	if m.curRow != origRow+1 {
		t.Errorf("'j' key: got row %d, want %d", m.curRow, origRow+1)
	}
}

func TestUpdateRotateCWKey(t *testing.T) {
	m := seedModel(42)
	m.cur = pieceT
	m.curRot = 0
	m.curRow = 10
	m.curCol = 4

	updated, _ := m.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyUp}))
	m = updated.(model)
	if m.curRot != 1 {
		t.Errorf("up arrow rotate: got rot %d, want 1", m.curRot)
	}
}

func TestUpdateRotateCWKeyW(t *testing.T) {
	m := seedModel(42)
	m.cur = pieceT
	m.curRot = 0
	m.curRow = 10
	m.curCol = 4

	updated, _ := m.Update(tea.KeyPressMsg(tea.Key{Code: 'w'}))
	m = updated.(model)
	if m.curRot != 1 {
		t.Errorf("'w' key rotate: got rot %d, want 1", m.curRot)
	}
}

func TestUpdateRotateCCWKeyZ(t *testing.T) {
	m := seedModel(42)
	m.cur = pieceT
	m.curRot = 0
	m.curRow = 10
	m.curCol = 4

	updated, _ := m.Update(tea.KeyPressMsg(tea.Key{Code: 'z'}))
	m = updated.(model)
	if m.curRot != 3 {
		t.Errorf("'z' key CCW rotate: got rot %d, want 3", m.curRot)
	}
}

func TestUpdateHardDropSpace(t *testing.T) {
	m := seedModel(42)
	m.cur = pieceI
	m.curRot = 0
	m.curRow = 0
	m.curCol = 3
	origScore := m.score

	updated, cmd := m.Update(tea.KeyPressMsg(tea.Key{Code: ' '}))
	m = updated.(model)
	// Hard drop should give score bonus (2 per row dropped).
	if m.score <= origScore {
		t.Errorf("hard drop should increase score: got %d, want > %d", m.score, origScore)
	}
	// Game should be started.
	if !m.started {
		t.Error("hard drop should start the game")
	}
	// Should schedule a tick (game not over on empty board).
	if cmd == nil {
		t.Error("hard drop should schedule a tick")
	}
}

func TestUpdateHoldKeyC(t *testing.T) {
	m := seedModel(42)
	m.curRow = 10
	original := m.cur

	updated, _ := m.Update(tea.KeyPressMsg(tea.Key{Code: 'c'}))
	m = updated.(model)
	if !m.hasHold {
		t.Error("'c' key should trigger hold")
	}
	if m.hold != original {
		t.Errorf("hold: got %d, want %d", m.hold, original)
	}
}

func TestUpdateHoldKeyTab(t *testing.T) {
	m := seedModel(42)
	m.curRow = 10
	original := m.cur

	updated, _ := m.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyTab}))
	m = updated.(model)
	if !m.hasHold {
		t.Error("tab key should trigger hold")
	}
	if m.hold != original {
		t.Errorf("hold: got %d, want %d", m.hold, original)
	}
}

func TestUpdatePauseToggle(t *testing.T) {
	m := seedModel(42)
	m.started = true
	m.gameOver = false

	// Pause.
	updated, cmd := m.Update(tea.KeyPressMsg(tea.Key{Code: 'p'}))
	m = updated.(model)
	if !m.paused {
		t.Error("'p' should pause the game")
	}
	if cmd != nil {
		t.Error("pausing should not schedule a tick")
	}

	// Unpause.
	updated, cmd = m.Update(tea.KeyPressMsg(tea.Key{Code: 'p'}))
	m = updated.(model)
	if m.paused {
		t.Error("second 'p' should unpause the game")
	}
	if cmd == nil {
		t.Error("unpausing should schedule a tick")
	}
}

func TestUpdatePauseNotStarted(t *testing.T) {
	m := seedModel(42)
	m.started = false

	updated, _ := m.Update(tea.KeyPressMsg(tea.Key{Code: 'p'}))
	m = updated.(model)
	if m.paused {
		t.Error("'p' should not pause when game not started")
	}
}

func TestUpdatePauseWhenGameOver(t *testing.T) {
	m := seedModel(42)
	m.started = true
	m.gameOver = true

	updated, _ := m.Update(tea.KeyPressMsg(tea.Key{Code: 'p'}))
	m = updated.(model)
	if m.paused {
		t.Error("'p' should not pause when game is over")
	}
}

func TestUpdateResetKey(t *testing.T) {
	m := seedModel(42)
	m.gameOver = true
	m.score = 500

	updated, _ := m.Update(tea.KeyPressMsg(tea.Key{Code: 'r'}))
	m = updated.(model)
	if m.gameOver {
		t.Error("'r' should reset when game is over")
	}
	if m.score != 0 {
		t.Error("'r' should reset score")
	}
}

func TestUpdateResetKeyNotGameOver(t *testing.T) {
	m := seedModel(42)
	m.gameOver = false
	m.score = 500

	updated, _ := m.Update(tea.KeyPressMsg(tea.Key{Code: 'r'}))
	m = updated.(model)
	if m.score != 500 {
		t.Error("'r' should not reset when game is not over")
	}
}

func TestUpdateQuitCtrlC(t *testing.T) {
	m := seedModel(42)
	_, cmd := m.Update(tea.KeyPressMsg(tea.Key{Code: 3}))
	if cmd == nil {
		t.Error("Ctrl+C should produce a quit command")
	}
}

func TestUpdateQuitUpperQ(t *testing.T) {
	m := seedModel(42)
	_, cmd := m.Update(tea.KeyPressMsg(tea.Key{Code: 'Q'}))
	if cmd == nil {
		t.Error("'Q' should produce a quit command")
	}
}

func TestUpdateIgnoresKeysWhenGameOver(t *testing.T) {
	m := seedModel(42)
	m.gameOver = true
	m.curRow = 10
	origCol := m.curCol

	updated, _ := m.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyLeft}))
	m = updated.(model)
	if m.curCol != origCol {
		t.Error("left arrow should be ignored when game is over")
	}
}

func TestUpdateIgnoresKeysWhenPaused(t *testing.T) {
	m := seedModel(42)
	m.started = true
	m.paused = true
	m.curRow = 10
	origCol := m.curCol

	updated, _ := m.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyLeft}))
	m = updated.(model)
	if m.curCol != origCol {
		t.Error("left arrow should be ignored when paused")
	}
}

func TestUpdateWindowSizeMsg(t *testing.T) {
	m := seedModel(42)
	updated, cmd := m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	m = updated.(model)
	if m.width != 120 {
		t.Errorf("width: got %d, want 120", m.width)
	}
	if m.height != 40 {
		t.Errorf("height: got %d, want 40", m.height)
	}
	if cmd != nil {
		t.Error("WindowSizeMsg should not produce a command")
	}
}

func TestUpdateUnknownMsg(t *testing.T) {
	m := seedModel(42)
	type unknownMsg struct{}
	updated, cmd := m.Update(unknownMsg{})
	m = updated.(model)
	if cmd != nil {
		t.Error("unknown message should not produce a command")
	}
	_ = m
}

// ── Tick handling tests ─────────────────────────────────────────────

func TestUpdateTickMovesDown(t *testing.T) {
	m := seedModel(42)
	m.started = true
	m.curRow = 10
	origRow := m.curRow

	updated, cmd := m.Update(tickMsg{gen: m.gen})
	m = updated.(model)
	if m.curRow != origRow+1 {
		t.Errorf("tick should move piece down: got row %d, want %d", m.curRow, origRow+1)
	}
	if cmd == nil {
		t.Error("tick should schedule next tick")
	}
}

func TestUpdateTickLocksAtBottom(t *testing.T) {
	m := seedModel(42)
	m.started = true
	m.cur = pieceI
	m.curRot = 0
	m.curRow = boardH - 1
	m.curCol = 3
	gen := m.gen

	updated, cmd := m.Update(tickMsg{gen: gen})
	m = updated.(model)
	// Piece should have been locked (gen increments in lockPiece).
	if m.gen == gen {
		t.Error("tick at bottom should lock piece and increment gen")
	}
	// Board row at bottom should have filled cells from the I-piece.
	filled := false
	for c := 3; c < 7; c++ {
		if m.board[boardH-1][c].filled {
			filled = true
			break
		}
	}
	if !filled {
		t.Error("locked piece should fill board cells")
	}
	// Should schedule next tick (game not over).
	if m.gameOver {
		if cmd != nil {
			t.Error("game over should not schedule a tick")
		}
	} else {
		if cmd == nil {
			t.Error("after locking, should schedule next tick")
		}
	}
}

func TestUpdateTickIgnoredWhenPaused(t *testing.T) {
	m := seedModel(42)
	m.started = true
	m.paused = true
	origRow := m.curRow

	updated, cmd := m.Update(tickMsg{gen: m.gen})
	m = updated.(model)
	if m.curRow != origRow {
		t.Error("tick should be ignored when paused")
	}
	if cmd != nil {
		t.Error("paused tick should not schedule next tick")
	}
}

func TestUpdateTickIgnoredWhenNotStarted(t *testing.T) {
	m := seedModel(42)
	m.started = false
	origRow := m.curRow

	updated, cmd := m.Update(tickMsg{gen: m.gen})
	m = updated.(model)
	if m.curRow != origRow {
		t.Error("tick should be ignored when not started")
	}
	if cmd != nil {
		t.Error("not-started tick should not schedule next tick")
	}
}

func TestUpdateTickIgnoredWhenGameOver(t *testing.T) {
	m := seedModel(42)
	m.started = true
	m.gameOver = true

	_, cmd := m.Update(tickMsg{gen: m.gen})
	if cmd != nil {
		t.Error("game over tick should not schedule next tick")
	}
}

// ── lockPiece scoring and line clear tests ──────────────────────────

func TestLockPieceScoring1Line(t *testing.T) {
	m := seedModel(42)
	m.level = 1
	m.score = 0
	m.lines = 0
	m.combos = 0

	// Fill the bottom row except cols 3-6 (where I-piece will land).
	for c := 0; c < boardW; c++ {
		if c < 3 || c > 6 {
			m.board[boardH-1][c] = cell{filled: true, piece: pieceO}
		}
	}
	// Place I-piece at bottom row to complete the line.
	m.cur = pieceI
	m.curRot = 0
	m.curRow = boardH - 1
	m.curCol = 3

	m.lockPiece()
	// 1 line at level 1 = 100 points.
	if m.score != 100 {
		t.Errorf("1-line score: got %d, want 100", m.score)
	}
	if m.lines != 1 {
		t.Errorf("lines: got %d, want 1", m.lines)
	}
	if m.combos != 1 {
		t.Errorf("combos: got %d, want 1", m.combos)
	}
}

func TestLockPieceScoring2Lines(t *testing.T) {
	m := seedModel(42)
	m.level = 1
	m.score = 0
	m.lines = 0

	// Fill 2 bottom rows except cols 3-6. I-piece vertical (rot 1) spans 4 rows,
	// so use a different approach: fill rows fully except one col, use O-piece.
	// Actually, let's fill 2 rows completely except col 0, place I-piece vertical
	// at col 0 to fill both rows.
	// Simpler: fill 2 rows except cols 3-6 and use horizontal I at each row.
	// Easiest approach: manually set up the board and lock piece.

	// Fill bottom 2 rows completely except cols 3-6.
	for r := boardH - 2; r < boardH; r++ {
		for c := 0; c < boardW; c++ {
			if c < 3 || c > 6 {
				m.board[r][c] = cell{filled: true, piece: pieceO}
			}
		}
	}
	// I-piece rotation 2 is also horizontal (same as 0). Place at row boardH-2
	// to fill cols 3-6 of that row.
	m.cur = pieceI
	m.curRot = 0
	m.curRow = boardH - 2
	m.curCol = 3

	// Lock fills row boardH-2 cols 3-6, completing that row.
	// But only 1 row is completed since I-piece is only 1 row tall.
	// For 2 lines we need to also fill the row below.
	// Fill row boardH-1 cols 3-6 too.
	for c := 3; c <= 6; c++ {
		m.board[boardH-1][c] = cell{filled: true, piece: pieceO}
	}

	m.lockPiece()
	// 2 lines at level 1 = 300 points.
	if m.score != 300 {
		t.Errorf("2-line score: got %d, want 300", m.score)
	}
	if m.lines != 2 {
		t.Errorf("lines: got %d, want 2", m.lines)
	}
}

func TestLockPieceCombosReset(t *testing.T) {
	m := seedModel(42)
	m.level = 1
	m.score = 0
	m.combos = 3

	// Lock piece without clearing any lines.
	m.cur = pieceI
	m.curRot = 0
	m.curRow = 10
	m.curCol = 3
	m.lockPiece()

	if m.combos != 0 {
		t.Errorf("combos should reset to 0 when no lines cleared: got %d", m.combos)
	}
}

func TestLockPieceComboBonus(t *testing.T) {
	m := seedModel(42)
	m.level = 1
	m.score = 0
	m.lines = 0
	m.combos = 1 // Already had 1 combo from a previous clear.

	// Fill bottom row except cols 3-6.
	for c := 0; c < boardW; c++ {
		if c < 3 || c > 6 {
			m.board[boardH-1][c] = cell{filled: true, piece: pieceO}
		}
	}
	m.cur = pieceI
	m.curRot = 0
	m.curRow = boardH - 1
	m.curCol = 3

	m.lockPiece()
	// combos was 1, now becomes 2. Score = base(100) + combo_bonus(50*(2-1)*1) = 150.
	if m.score != 150 {
		t.Errorf("combo bonus score: got %d, want 150", m.score)
	}
	if m.combos != 2 {
		t.Errorf("combos: got %d, want 2", m.combos)
	}
}

func TestLockPieceLevelUp(t *testing.T) {
	m := seedModel(42)
	m.level = 1
	m.lines = 9
	m.score = 0
	oldSpeed := m.speed

	// Fill bottom row except cols 3-6.
	for c := 0; c < boardW; c++ {
		if c < 3 || c > 6 {
			m.board[boardH-1][c] = cell{filled: true, piece: pieceO}
		}
	}
	m.cur = pieceI
	m.curRot = 0
	m.curRow = boardH - 1
	m.curCol = 3

	m.lockPiece()
	if m.level != 2 {
		t.Errorf("level: got %d, want 2", m.level)
	}
	if m.speed >= oldSpeed {
		t.Errorf("speed should decrease after level up: got %v, was %v", m.speed, oldSpeed)
	}
}

func TestLockPieceNoLevelUpWhenNotEnoughLines(t *testing.T) {
	m := seedModel(42)
	m.level = 1
	m.lines = 5
	m.score = 0

	// Fill bottom row except cols 3-6.
	for c := 0; c < boardW; c++ {
		if c < 3 || c > 6 {
			m.board[boardH-1][c] = cell{filled: true, piece: pieceO}
		}
	}
	m.cur = pieceI
	m.curRot = 0
	m.curRow = boardH - 1
	m.curCol = 3

	m.lockPiece()
	if m.level != 1 {
		t.Errorf("level should stay 1: got %d", m.level)
	}
}

func TestLockPieceIncrementsGen(t *testing.T) {
	m := seedModel(42)
	m.curRow = 10
	oldGen := m.gen

	m.lockPiece()
	if m.gen != oldGen+1 {
		t.Errorf("lockPiece should increment gen: got %d, want %d", m.gen, oldGen+1)
	}
}

// ── spawnPiece game over test ───────────────────────────────────────

func TestSpawnPieceGameOver(t *testing.T) {
	m := seedModel(42)
	// Fill the top rows so the next piece cannot spawn.
	for r := 0; r < 4; r++ {
		for c := 0; c < boardW; c++ {
			m.board[r][c] = cell{filled: true, piece: pieceO}
		}
	}
	m.spawnPiece()
	if !m.gameOver {
		t.Error("spawnPiece should set gameOver when spawn position is blocked")
	}
}

// ── tryRotate wall kick failure ─────────────────────────────────────

func TestTryRotateAllKicksFail(t *testing.T) {
	m := seedModel(42)
	m.cur = pieceT
	m.curRot = 0
	m.curRow = 0
	m.curCol = 0

	// Fill the board around the piece so all wall kick offsets fail.
	for r := 0; r < 5; r++ {
		for c := 0; c < boardW; c++ {
			if r == 0 && c >= 0 && c <= 2 {
				continue // Leave room for the T-piece in current position.
			}
			if r == 1 && c >= 0 && c <= 2 {
				continue
			}
			m.board[r][c] = cell{filled: true, piece: pieceO}
		}
	}

	if m.rotateCW() {
		t.Error("rotation should fail when all wall kicks are blocked")
	}
	if m.curRot != 0 {
		t.Error("rotation should not change when all kicks fail")
	}
}

// ── fallSpeed edge cases ────────────────────────────────────────────

func TestFallSpeedFloorAt50ms(t *testing.T) {
	// At very high levels the formula would go below 50ms, but it's clamped.
	s := fallSpeed(100)
	if s != 50*time.Millisecond {
		t.Errorf("fallSpeed(100): got %v, want 50ms", s)
	}
}

func TestFallSpeedLevel1(t *testing.T) {
	// Level 1: pow(0.8, 0) = 1.0 => 1000ms.
	s := fallSpeed(1)
	if s != 1000*time.Millisecond {
		t.Errorf("fallSpeed(1): got %v, want 1s", s)
	}
}

// ── newModel state restoration test ─────────────────────────────────

func TestNewModelRestoresState(t *testing.T) {
	ts := tetrisState{
		Cur:      int(pieceT),
		CurRot:   2,
		CurRow:   5,
		CurCol:   4,
		Bag:      []int{0, 1, 2},
		Next:     [3]int{int(pieceS), int(pieceZ), int(pieceJ)},
		Hold:     int(pieceL),
		HasHold:  true,
		CanHold:  false,
		Score:    9999,
		Lines:    42,
		Level:    5,
		Combos:   3,
		SpeedMs:  200,
		Gen:      7,
		GameOver: false,
		Paused:   true,
		Started:  true,
	}
	// Fill a cell in the board state.
	ts.Board[10][5] = boardCell{F: true, P: int(pieceI)}

	data, err := json.Marshal(ts)
	if err != nil {
		t.Fatal(err)
	}
	encoded := base64.StdEncoding.EncodeToString(data)
	t.Setenv("TERMDESK_APP_STATE", encoded)

	m := newModel()

	if m.cur != pieceT {
		t.Errorf("cur: got %d, want %d", m.cur, pieceT)
	}
	if m.curRot != 2 {
		t.Errorf("curRot: got %d, want 2", m.curRot)
	}
	if m.curRow != 5 {
		t.Errorf("curRow: got %d, want 5", m.curRow)
	}
	if m.curCol != 4 {
		t.Errorf("curCol: got %d, want 4", m.curCol)
	}
	if m.score != 9999 {
		t.Errorf("score: got %d, want 9999", m.score)
	}
	if m.lines != 42 {
		t.Errorf("lines: got %d, want 42", m.lines)
	}
	if m.level != 5 {
		t.Errorf("level: got %d, want 5", m.level)
	}
	if m.combos != 3 {
		t.Errorf("combos: got %d, want 3", m.combos)
	}
	if m.speed != 200*time.Millisecond {
		t.Errorf("speed: got %v, want 200ms", m.speed)
	}
	if m.gen != 7 {
		t.Errorf("gen: got %d, want 7", m.gen)
	}
	if m.gameOver {
		t.Error("gameOver should be false")
	}
	if !m.paused {
		t.Error("paused should be true")
	}
	if !m.started {
		t.Error("started should be true")
	}
	if m.hold != pieceL {
		t.Errorf("hold: got %d, want %d", m.hold, pieceL)
	}
	if !m.hasHold {
		t.Error("hasHold should be true")
	}
	if m.canHold {
		t.Error("canHold should be false")
	}
	if !m.board[10][5].filled {
		t.Error("board[10][5] should be filled")
	}
	if m.board[10][5].piece != pieceI {
		t.Errorf("board[10][5].piece: got %d, want %d", m.board[10][5].piece, pieceI)
	}
	if m.next[0] != pieceS || m.next[1] != pieceZ || m.next[2] != pieceJ {
		t.Errorf("next: got %v, want [S Z J]", m.next)
	}
	if len(m.bag) != 3 {
		t.Errorf("bag len: got %d, want 3", len(m.bag))
	}
}

func TestNewModelInvalidEnvIgnored(t *testing.T) {
	t.Setenv("TERMDESK_APP_STATE", "not-valid-base64!!!")
	m := newModel()
	// Should fall through to default initialization.
	if m.level != 1 {
		t.Errorf("level: got %d, want 1", m.level)
	}
}

func TestNewModelInvalidJSONIgnored(t *testing.T) {
	encoded := base64.StdEncoding.EncodeToString([]byte("not json"))
	t.Setenv("TERMDESK_APP_STATE", encoded)
	m := newModel()
	if m.level != 1 {
		t.Errorf("level: got %d, want 1", m.level)
	}
}

// ── View edge cases ─────────────────────────────────────────────────

func TestViewZeroSize(t *testing.T) {
	m := seedModel(42)
	m.width = 0
	m.height = 0
	v := m.View()
	if !v.AltScreen {
		t.Error("View should set AltScreen even at zero size")
	}
}

func TestViewPausedState(t *testing.T) {
	m := seedModel(42)
	m.width = 80
	m.height = 40
	m.started = true
	m.paused = true
	v := m.View()
	if !v.AltScreen {
		t.Error("View should set AltScreen")
	}
}

func TestViewGameOverState(t *testing.T) {
	m := seedModel(42)
	m.width = 80
	m.height = 40
	m.started = true
	m.gameOver = true
	m.score = 500
	v := m.View()
	if !v.AltScreen {
		t.Error("View should set AltScreen")
	}
}

func TestViewStartedState(t *testing.T) {
	m := seedModel(42)
	m.width = 80
	m.height = 40
	m.started = true
	v := m.View()
	if !v.AltScreen {
		t.Error("View should set AltScreen")
	}
}

// ── renderMiniPieceLines edge case ──────────────────────────────────

func TestRenderMiniPieceLinesInvalidPiece(t *testing.T) {
	lines := renderMiniPieceLines(-1, 0, "#1E1E2E")
	if len(lines) != 2 {
		t.Errorf("invalid piece should return 2 lines, got %d", len(lines))
	}
}

// ── renderLeftPanel without hold ─────────────────────────────────────

func TestRenderLeftPanelNoHold(t *testing.T) {
	m := seedModel(42)
	m.hasHold = false
	// Just ensure it doesn't panic and returns lines.
	bg := lipgloss.Color("#1E1E2E")
	textSt := lipgloss.NewStyle().Background(bg)
	dimSt := lipgloss.NewStyle().Background(bg)
	boldSt := lipgloss.NewStyle().Background(bg)
	borderSt := lipgloss.NewStyle().Background(bg)
	emptySt := lipgloss.NewStyle().Background(bg)
	lines := m.renderLeftPanel(textSt, dimSt, boldSt, borderSt, emptySt, bg)
	if len(lines) == 0 {
		t.Error("renderLeftPanel should return lines")
	}
}

// ── Hard drop game over test ────────────────────────────────────────

func TestUpdateHardDropGameOver(t *testing.T) {
	m := seedModel(42)
	m.started = true

	// Fill most of the board but leave last column empty to prevent line clears.
	// This ensures after locking, spawnPiece fails because row 0 + rows below are blocked.
	for r := 1; r < boardH; r++ {
		for c := 0; c < boardW-1; c++ {
			m.board[r][c] = cell{filled: true, piece: pieceO}
		}
	}
	m.cur = pieceI
	m.curRot = 0
	m.curRow = 0
	m.curCol = 3

	updated, cmd := m.Update(tea.KeyPressMsg(tea.Key{Code: ' '}))
	m = updated.(model)
	// The piece locks at row 0 and spawnPiece tries to spawn on a full board.
	if !m.gameOver {
		t.Error("hard drop on nearly full board should cause game over")
	}
	if cmd != nil {
		t.Error("game over after hard drop should not schedule a tick")
	}
}

// ── Tick that causes game over ──────────────────────────────────────

func TestUpdateTickGameOverOnLock(t *testing.T) {
	m := seedModel(42)
	m.started = true

	// Fill rows 1..boardH-1 but leave last column empty to prevent line clears.
	for r := 1; r < boardH; r++ {
		for c := 0; c < boardW-1; c++ {
			m.board[r][c] = cell{filled: true, piece: pieceO}
		}
	}
	m.cur = pieceI
	m.curRot = 0
	m.curRow = 0
	m.curCol = 3

	// Tick should fail to move down and lock.
	updated, cmd := m.Update(tickMsg{gen: m.gen})
	m = updated.(model)
	if !m.gameOver {
		t.Error("tick lock on full board should cause game over")
	}
	if cmd != nil {
		t.Error("game over should not schedule next tick")
	}
}

// ── Start game on first key (via startGame closure) ─────────────────

func TestUpdateFirstKeyStartsGame(t *testing.T) {
	m := seedModel(42)
	m.started = false
	m.curRow = 10

	updated, cmd := m.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyRight}))
	m = updated.(model)
	if !m.started {
		t.Error("first movement key should start the game")
	}
	if cmd == nil {
		t.Error("starting the game should schedule a tick")
	}
}

func TestUpdateAlreadyStartedNoExtraTick(t *testing.T) {
	m := seedModel(42)
	m.started = true
	m.curRow = 10

	updated, cmd := m.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyRight}))
	m = updated.(model)
	// When already started, startGame returns nil (no extra tick).
	if cmd != nil {
		t.Error("already-started move should not schedule extra tick")
	}
}

// ── Uppercase key variants ──────────────────────────────────────────

func TestUpdateUppercaseKeys(t *testing.T) {
	tests := []struct {
		name string
		code rune
	}{
		{"H", 'H'},
		{"L", 'L'},
		{"J", 'J'},
		{"W", 'W'},
		{"Z", 'Z'},
		{"C", 'C'},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			m := seedModel(42)
			m.curRow = 10
			m.curCol = 4
			m.cur = pieceT
			// Should not panic.
			updated, _ := m.Update(tea.KeyPressMsg(tea.Key{Code: tc.code}))
			_ = updated.(model)
		})
	}
}

// ── Soft drop at floor does not give bonus ──────────────────────────

func TestUpdateSoftDropAtFloorNoBonus(t *testing.T) {
	m := seedModel(42)
	m.curRow = boardH - 1
	m.score = 0

	updated, _ := m.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyDown}))
	m = updated.(model)
	// moveDown fails at floor, no soft drop bonus.
	if m.score != 0 {
		t.Errorf("soft drop at floor should not give bonus: got %d", m.score)
	}
}

// ── R key uppercase ─────────────────────────────────────────────────

func TestUpdateResetUpperR(t *testing.T) {
	m := seedModel(42)
	m.gameOver = true
	m.score = 500

	updated, _ := m.Update(tea.KeyPressMsg(tea.Key{Code: 'R'}))
	m = updated.(model)
	if m.gameOver {
		t.Error("'R' should reset when game is over")
	}
}

// ── P key uppercase ─────────────────────────────────────────────────

func TestUpdatePauseUpperP(t *testing.T) {
	m := seedModel(42)
	m.started = true

	updated, _ := m.Update(tea.KeyPressMsg(tea.Key{Code: 'P'}))
	m = updated.(model)
	if !m.paused {
		t.Error("'P' should pause the game")
	}
}

// ── View with hold piece set ────────────────────────────────────────

func TestViewWithHoldPiece(t *testing.T) {
	m := seedModel(42)
	m.width = 80
	m.height = 40
	m.hasHold = true
	m.hold = pieceT
	v := m.View()
	if !v.AltScreen {
		t.Error("View should set AltScreen")
	}
}

func TestUpdateStateDumpMsg(t *testing.T) {
	m := seedModel(42)
	m.started = true
	m.score = 1234
	m.lines = 10
	m.level = 2
	m.combos = 1
	m.hasHold = true
	m.hold = pieceT
	m.canHold = false
	m.paused = true

	// Fill a cell so we can verify board serialization
	m.board[5][3] = cell{filled: true, piece: pieceS}

	updated, cmd := m.Update(stateDumpMsg{})
	m = updated.(model)

	// stateDumpMsg should return a command (listenStateDump)
	if cmd == nil {
		t.Error("stateDumpMsg should return a command to listen for next dump")
	}

	// Verify state was not mutated by the dump
	if m.score != 1234 {
		t.Errorf("score should remain 1234, got %d", m.score)
	}
	if m.lines != 10 {
		t.Errorf("lines should remain 10, got %d", m.lines)
	}
	if !m.board[5][3].filled {
		t.Error("board cell should remain filled after dump")
	}
}

func TestInit(t *testing.T) {
	m := seedModel(42)
	m.started = false
	m.gameOver = false
	m.paused = false

	cmd := m.Init()
	// Init always returns a command (at least listenStateDump)
	if cmd == nil {
		t.Error("Init should return a non-nil command")
	}
}

func TestInitStartedGame(t *testing.T) {
	m := seedModel(42)
	m.started = true
	m.gameOver = false
	m.paused = false

	cmd := m.Init()
	// When started and not paused/game over, should return batch with tick + listenStateDump
	if cmd == nil {
		t.Error("Init for started game should return a non-nil command")
	}
}

func TestInitPausedGame(t *testing.T) {
	m := seedModel(42)
	m.started = true
	m.paused = true
	m.gameOver = false

	cmd := m.Init()
	// Paused game: should still return listenStateDump at minimum
	if cmd == nil {
		t.Error("Init for paused game should return a non-nil command")
	}
}

func TestScheduleTickReturnsCommand(t *testing.T) {
	cmd := scheduleTick(100*time.Millisecond, 1)
	if cmd == nil {
		t.Error("scheduleTick should return a non-nil command")
	}
}

func TestStateDumpMsgSerializesState(t *testing.T) {
	// Build a model with known state
	m := seedModel(42)
	m.score = 5555
	m.lines = 20
	m.level = 3
	m.combos = 2
	m.started = true
	m.paused = false
	m.gameOver = false
	m.hasHold = true
	m.hold = pieceL
	m.canHold = true
	m.board[15][7] = cell{filled: true, piece: pieceZ}

	// Capture the OSC output by examining that Update doesn't error
	// and returns the correct command (listenStateDump)
	updated, cmd := m.Update(stateDumpMsg{})
	m2 := updated.(model)

	// Model state should be preserved
	if m2.score != 5555 {
		t.Errorf("score after dump: got %d, want 5555", m2.score)
	}
	if m2.level != 3 {
		t.Errorf("level after dump: got %d, want 3", m2.level)
	}
	if !m2.board[15][7].filled {
		t.Error("board[15][7] should still be filled after dump")
	}
	if cmd == nil {
		t.Error("stateDumpMsg handler should return listenStateDump command")
	}
}

func TestStateDumpMsgSerializationRoundTrip(t *testing.T) {
	// Verify the tetrisState structure can round-trip through JSON
	m := seedModel(42)
	m.score = 42
	m.lines = 5
	m.level = 2
	m.combos = 1
	m.started = true
	m.hasHold = true
	m.hold = pieceJ
	m.canHold = false
	m.board[3][4] = cell{filled: true, piece: pieceI}

	var ts tetrisState
	for r := range boardH {
		for c := range boardW {
			ts.Board[r][c] = boardCell{F: m.board[r][c].filled, P: int(m.board[r][c].piece)}
		}
	}
	ts.Cur = int(m.cur)
	ts.CurRot = m.curRot
	ts.CurRow = m.curRow
	ts.CurCol = m.curCol
	ts.Bag = make([]int, len(m.bag))
	for i, p := range m.bag {
		ts.Bag[i] = int(p)
	}
	for i, p := range m.next {
		ts.Next[i] = int(p)
	}
	ts.Hold = int(m.hold)
	ts.HasHold = m.hasHold
	ts.CanHold = m.canHold
	ts.Score = m.score
	ts.Lines = m.lines
	ts.Level = m.level
	ts.Combos = m.combos
	ts.SpeedMs = m.speed.Milliseconds()
	ts.Gen = m.gen
	ts.GameOver = m.gameOver
	ts.Paused = m.paused
	ts.Started = m.started

	data, err := json.Marshal(ts)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}

	// Restore via env var
	encoded := base64.StdEncoding.EncodeToString(data)
	t.Setenv("TERMDESK_APP_STATE", encoded)
	restored := newModel()

	if restored.score != 42 {
		t.Errorf("restored score: got %d, want 42", restored.score)
	}
	if restored.hold != pieceJ {
		t.Errorf("restored hold: got %d, want %d", restored.hold, pieceJ)
	}
	if !restored.board[3][4].filled {
		t.Error("restored board[3][4] should be filled")
	}
}

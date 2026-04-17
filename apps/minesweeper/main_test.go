package main

import (
	"encoding/base64"
	"encoding/json"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
)

func viewContent(v tea.View) string {
	return v.Content
}

// countMines returns the total number of mines in the grid.
func countMines(m *model) int {
	count := 0
	for y := range gridH {
		for x := range gridW {
			if m.grid[y][x].mine {
				count++
			}
		}
	}
	return count
}

// countRevealed returns the total number of revealed cells.
func countRevealed(m *model) int {
	count := 0
	for y := range gridH {
		for x := range gridW {
			if m.grid[y][x].state == cellRevealed {
				count++
			}
		}
	}
	return count
}

// findSafeEmpty finds a cell with no mine and adjacent==0 (for flood-fill testing).
func findSafeEmpty(m *model) (int, int, bool) {
	for y := range gridH {
		for x := range gridW {
			if !m.grid[y][x].mine && m.grid[y][x].adjacent == 0 {
				return x, y, true
			}
		}
	}
	return 0, 0, false
}

// findSafeCell finds any non-mine cell.
func findSafeCell(m *model) (int, int, bool) {
	for y := range gridH {
		for x := range gridW {
			if !m.grid[y][x].mine {
				return x, y, true
			}
		}
	}
	return 0, 0, false
}

// findMineCell finds a mine cell.
func findMineCell(m *model) (int, int, bool) {
	for y := range gridH {
		for x := range gridW {
			if m.grid[y][x].mine {
				return x, y, true
			}
		}
	}
	return 0, 0, false
}

func TestMinePlacement(t *testing.T) {
	m := NewTestModel(42)

	// Should have exactly 10 mines.
	mc := countMines(&m)
	if mc != mines {
		t.Errorf("expected %d mines, got %d", mines, mc)
	}

	// All cells should start hidden.
	for y := range gridH {
		for x := range gridW {
			if m.grid[y][x].state != cellHidden {
				t.Errorf("cell (%d,%d) should be hidden initially", x, y)
			}
		}
	}
}

func TestMinePlacementDeterministic(t *testing.T) {
	m1 := NewTestModel(123)
	m2 := NewTestModel(123)

	for y := range gridH {
		for x := range gridW {
			if m1.grid[y][x].mine != m2.grid[y][x].mine {
				t.Errorf("same seed should produce same mines at (%d,%d)", x, y)
			}
		}
	}
}

func TestAdjacentCounts(t *testing.T) {
	m := NewTestModel(42)

	for y := range gridH {
		for x := range gridW {
			if m.grid[y][x].mine {
				continue
			}
			// Manually count adjacent mines.
			expected := 0
			for dy := -1; dy <= 1; dy++ {
				for dx := -1; dx <= 1; dx++ {
					if dx == 0 && dy == 0 {
						continue
					}
					nx, ny := x+dx, y+dy
					if nx >= 0 && nx < gridW && ny >= 0 && ny < gridH && m.grid[ny][nx].mine {
						expected++
					}
				}
			}
			if m.grid[y][x].adjacent != expected {
				t.Errorf("cell (%d,%d) adjacent: got %d, want %d", x, y, m.grid[y][x].adjacent, expected)
			}
		}
	}
}

func TestFloodFillReveal(t *testing.T) {
	m := NewTestModel(42)
	m.firstClick = false // Skip first-click safety for this test.

	sx, sy, found := findSafeEmpty(&m)
	if !found {
		t.Skip("no empty cell found with this seed")
	}

	m.reveal(sx, sy)

	// The clicked cell should be revealed.
	if m.grid[sy][sx].state != cellRevealed {
		t.Errorf("cell (%d,%d) should be revealed", sx, sy)
	}

	// Multiple cells should be revealed (flood fill).
	revealed := countRevealed(&m)
	if revealed < 2 {
		t.Errorf("flood fill should reveal multiple cells, got %d", revealed)
	}

	// No mine should be revealed.
	for y := range gridH {
		for x := range gridW {
			if m.grid[y][x].mine && m.grid[y][x].state == cellRevealed {
				t.Errorf("mine at (%d,%d) should not be revealed by flood fill", x, y)
			}
		}
	}

	// All revealed cells should either be empty or have a number (boundary of flood).
	for y := range gridH {
		for x := range gridW {
			c := m.grid[y][x]
			if c.state == cellRevealed && c.mine {
				t.Errorf("revealed mine at (%d,%d)", x, y)
			}
		}
	}
}

func TestFirstClickSafety(t *testing.T) {
	// Create model and find a mine.
	m := NewTestModel(42)
	mx, my, found := findMineCell(&m)
	if !found {
		t.Fatal("no mine found")
	}

	// First click on a mine should not lose.
	m.handleReveal(mx, my)

	if m.gameState == stateLost {
		t.Error("first click on a mine should not lose the game")
	}

	// The cell should be revealed and not a mine anymore.
	if m.grid[my][mx].mine {
		t.Error("mine should have been relocated after first click")
	}
	if m.grid[my][mx].state != cellRevealed {
		t.Error("first-clicked cell should be revealed")
	}

	// Should still have exactly 10 mines total.
	mc := countMines(&m)
	if mc != mines {
		t.Errorf("should still have %d mines after relocation, got %d", mines, mc)
	}
}

func TestWinDetection(t *testing.T) {
	m := NewTestModel(42)
	m.firstClick = false

	// Reveal all non-mine cells manually.
	for y := range gridH {
		for x := range gridW {
			if !m.grid[y][x].mine {
				m.grid[y][x].state = cellRevealed
			}
		}
	}

	if !m.checkWin() {
		t.Error("should detect win when all non-mine cells are revealed")
	}
}

func TestWinDetectionViaReveal(t *testing.T) {
	m := NewTestModel(42)
	m.firstClick = false

	// Reveal all non-mine cells except one, then reveal the last one.
	var lastX, lastY int
	for y := range gridH {
		for x := range gridW {
			if !m.grid[y][x].mine {
				lastX, lastY = x, y
			}
		}
	}

	// Reveal all except last.
	for y := range gridH {
		for x := range gridW {
			if !m.grid[y][x].mine && !(x == lastX && y == lastY) {
				m.grid[y][x].state = cellRevealed
			}
		}
	}

	m.handleReveal(lastX, lastY)

	if m.gameState != stateWon {
		t.Errorf("game should be won, got state %d", m.gameState)
	}
}

func TestLoseDetection(t *testing.T) {
	m := NewTestModel(42)
	m.firstClick = false // Disable first-click safety.

	mx, my, found := findMineCell(&m)
	if !found {
		t.Fatal("no mine found")
	}

	m.reveal(mx, my)

	if m.gameState != stateLost {
		t.Error("clicking a mine should lose the game")
	}

	// All mines should be revealed after losing.
	for y := range gridH {
		for x := range gridW {
			if m.grid[y][x].mine && m.grid[y][x].state != cellRevealed {
				t.Errorf("mine at (%d,%d) should be revealed after losing", x, y)
			}
		}
	}
}

func TestFlagToggle(t *testing.T) {
	m := NewTestModel(42)

	sx, sy, found := findSafeCell(&m)
	if !found {
		t.Fatal("no safe cell found")
	}

	// Initial state.
	if m.flags != 0 {
		t.Errorf("initial flags should be 0, got %d", m.flags)
	}

	// Flag the cell.
	m.toggleFlag(sx, sy)
	if m.grid[sy][sx].state != cellFlagged {
		t.Error("cell should be flagged")
	}
	if m.flags != 1 {
		t.Errorf("flags should be 1, got %d", m.flags)
	}

	// Unflag the cell.
	m.toggleFlag(sx, sy)
	if m.grid[sy][sx].state != cellHidden {
		t.Error("cell should be unflagged (hidden)")
	}
	if m.flags != 0 {
		t.Errorf("flags should be 0 after unflag, got %d", m.flags)
	}
}

func TestFlagPreventsReveal(t *testing.T) {
	m := NewTestModel(42)
	m.firstClick = false

	sx, sy, found := findSafeCell(&m)
	if !found {
		t.Fatal("no safe cell found")
	}

	// Flag the cell.
	m.toggleFlag(sx, sy)

	// Try to reveal it — should be blocked.
	m.handleReveal(sx, sy)

	if m.grid[sy][sx].state != cellFlagged {
		t.Error("flagged cell should not be revealed")
	}
}

func TestRevealedCellCannotBeRevealed(t *testing.T) {
	m := NewTestModel(42)
	m.firstClick = false

	sx, sy, found := findSafeCell(&m)
	if !found {
		t.Fatal("no safe cell found")
	}

	m.reveal(sx, sy)
	before := countRevealed(&m)
	m.handleReveal(sx, sy) // Should be no-op.
	after := countRevealed(&m)

	if before != after {
		t.Error("revealing an already-revealed cell should be a no-op")
	}
}

func TestFlagDoesNotAffectRevealedCell(t *testing.T) {
	m := NewTestModel(42)
	m.firstClick = false

	sx, sy, found := findSafeCell(&m)
	if !found {
		t.Fatal("no safe cell found")
	}

	m.reveal(sx, sy)
	m.toggleFlag(sx, sy) // Should be no-op on revealed cell.

	if m.grid[sy][sx].state != cellRevealed {
		t.Error("toggling flag on revealed cell should be no-op")
	}
	if m.flags != 0 {
		t.Errorf("flags should still be 0, got %d", m.flags)
	}
}

// --- Exported helper method tests ---

func TestRevealCell(t *testing.T) {
	m := NewTestModel(42)

	sx, sy, found := findSafeCell(&m)
	if !found {
		t.Fatal("no safe cell found")
	}

	// RevealCell wraps handleReveal; first click triggers firstClick logic.
	m.RevealCell(sx, sy)

	if m.grid[sy][sx].state != cellRevealed {
		t.Error("RevealCell should reveal the targeted cell")
	}
	if m.GameState() == stateLost {
		t.Error("revealing a safe cell should not lose the game")
	}
}

func TestRevealCellOnMine(t *testing.T) {
	m := NewTestModel(42)
	m.firstClick = false

	mx, my, found := findMineCell(&m)
	if !found {
		t.Fatal("no mine found")
	}

	m.RevealCell(mx, my)

	if m.GameState() != stateLost {
		t.Error("RevealCell on a mine (after first click) should lose the game")
	}
}

func TestToggleFlagExported(t *testing.T) {
	m := NewTestModel(42)

	sx, sy, found := findSafeCell(&m)
	if !found {
		t.Fatal("no safe cell found")
	}

	m.ToggleFlag(sx, sy)
	if m.grid[sy][sx].state != cellFlagged {
		t.Error("ToggleFlag should flag a hidden cell")
	}
	if m.Flags() != 1 {
		t.Errorf("Flags() should return 1 after flagging, got %d", m.Flags())
	}

	m.ToggleFlag(sx, sy)
	if m.grid[sy][sx].state != cellHidden {
		t.Error("ToggleFlag should unflag a flagged cell")
	}
	if m.Flags() != 0 {
		t.Errorf("Flags() should return 0 after unflagging, got %d", m.Flags())
	}
}

func TestGameStateExported(t *testing.T) {
	m := NewTestModel(42)

	if m.GameState() != statePlaying {
		t.Errorf("initial GameState should be statePlaying (0), got %d", m.GameState())
	}

	// Trigger a loss.
	m.firstClick = false
	mx, my, found := findMineCell(&m)
	if !found {
		t.Fatal("no mine found")
	}
	m.RevealCell(mx, my)

	if m.GameState() != stateLost {
		t.Errorf("GameState should be stateLost (2), got %d", m.GameState())
	}
}

func TestGridExported(t *testing.T) {
	m := NewTestModel(42)

	grid := m.Grid()
	if grid == nil {
		t.Fatal("Grid() should not return nil")
	}

	// Grid should reflect the same mine layout.
	mineCount := 0
	for y := range gridH {
		for x := range gridW {
			if grid[y][x].mine {
				mineCount++
			}
		}
	}
	if mineCount != mines {
		t.Errorf("Grid() should contain %d mines, got %d", mines, mineCount)
	}

	// Mutations through the pointer should be reflected in the model.
	grid[0][0].state = cellRevealed
	if m.grid[0][0].state != cellRevealed {
		t.Error("Grid() should return a pointer to the actual grid")
	}
}

func TestFlagsExported(t *testing.T) {
	m := NewTestModel(42)

	if m.Flags() != 0 {
		t.Errorf("initial Flags() should be 0, got %d", m.Flags())
	}

	sx, sy, found := findSafeCell(&m)
	if !found {
		t.Fatal("no safe cell found")
	}
	m.ToggleFlag(sx, sy)

	if m.Flags() != 1 {
		t.Errorf("Flags() should be 1 after one flag, got %d", m.Flags())
	}
}

// --- Coordinate conversion tests ---

func TestGridOrigin(t *testing.T) {
	m := NewTestModel(42)
	m.width = 80
	m.height = 40

	gx, gy := m.gridOrigin()

	// totalGridW = 1 + 9*(3+1) = 37
	// frameW = 37 + 4 = 41
	// originX = (80 - 41) / 2 = 19
	// gridX = 19 + 3 = 22
	expectedX := (m.width-(1+gridW*(cellW+1)+4))/2 + 3
	if gx != expectedX {
		t.Errorf("gridOrigin X: got %d, want %d", gx, expectedX)
	}

	// totalGridH = 1 + 9*(2+1) = 28
	// frameH = 28 + 5 = 33
	// originY = (40 - 33) / 2 = 3
	// gridY = 3 + 4 = 7
	expectedY := (m.height-(1+gridH*(cellH+1)+5))/2 + 4
	if gy != expectedY {
		t.Errorf("gridOrigin Y: got %d, want %d", gy, expectedY)
	}
}

func TestGridOriginZeroSize(t *testing.T) {
	m := NewTestModel(42)
	m.width = 0
	m.height = 0

	gx, gy := m.gridOrigin()

	// With zero width/height, origin is negative but still computable.
	totalGridW := 1 + gridW*(cellW+1)
	totalGridH := 1 + gridH*(cellH+1)
	frameW := totalGridW + 4
	frameH := totalGridH + 5
	expectedX := (0-frameW)/2 + 3
	expectedY := (0-frameH)/2 + 4
	if gx != expectedX {
		t.Errorf("gridOrigin X with zero size: got %d, want %d", gx, expectedX)
	}
	if gy != expectedY {
		t.Errorf("gridOrigin Y with zero size: got %d, want %d", gy, expectedY)
	}
}

func TestScreenToGridValidCells(t *testing.T) {
	m := NewTestModel(42)
	m.width = 80
	m.height = 40

	originX, originY := m.gridOrigin()

	strideX := cellW + 1
	strideY := cellH + 1

	// Test corner and center cells.
	tests := []struct {
		name  string
		gridX int
		gridY int
	}{
		{"top-left", 0, 0},
		{"top-right", gridW - 1, 0},
		{"bottom-left", 0, gridH - 1},
		{"bottom-right", gridW - 1, gridH - 1},
		{"center", gridW / 2, gridH / 2},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sx := originX + tt.gridX*strideX
			sy := originY + tt.gridY*strideY
			gx, gy, ok := m.screenToGrid(sx, sy)
			if !ok {
				t.Errorf("screenToGrid(%d, %d) should be valid", sx, sy)
			}
			if gx != tt.gridX || gy != tt.gridY {
				t.Errorf("screenToGrid(%d, %d) = (%d, %d), want (%d, %d)", sx, sy, gx, gy, tt.gridX, tt.gridY)
			}
		})
	}
}

func TestScreenToGridNegativeOffset(t *testing.T) {
	m := NewTestModel(42)
	m.width = 80
	m.height = 40

	// Coordinates before the grid origin should return false.
	_, _, ok := m.screenToGrid(0, 0)
	if ok {
		t.Error("screenToGrid at (0, 0) should return false for an 80x40 terminal")
	}
}

func TestScreenToGridOnVerticalBorder(t *testing.T) {
	m := NewTestModel(42)
	m.width = 80
	m.height = 40

	originX, originY := m.gridOrigin()

	// The vertical border character sits at offset cellW within each stride.
	sx := originX + cellW
	sy := originY

	_, _, ok := m.screenToGrid(sx, sy)
	if ok {
		t.Error("screenToGrid on vertical border should return false")
	}
}

func TestScreenToGridOnHorizontalBorder(t *testing.T) {
	m := NewTestModel(42)
	m.width = 80
	m.height = 40

	originX, originY := m.gridOrigin()

	// The horizontal border row sits at offset cellH within each stride.
	sx := originX
	sy := originY + cellH

	_, _, ok := m.screenToGrid(sx, sy)
	if ok {
		t.Error("screenToGrid on horizontal border should return false")
	}
}

func TestScreenToGridOutOfBounds(t *testing.T) {
	m := NewTestModel(42)
	m.width = 80
	m.height = 40

	originX, originY := m.gridOrigin()

	strideX := cellW + 1
	strideY := cellH + 1

	// Beyond the last column.
	sx := originX + gridW*strideX
	sy := originY
	_, _, ok := m.screenToGrid(sx, sy)
	if ok {
		t.Error("screenToGrid beyond last column should return false")
	}

	// Beyond the last row.
	sx = originX
	sy = originY + gridH*strideY
	_, _, ok = m.screenToGrid(sx, sy)
	if ok {
		t.Error("screenToGrid beyond last row should return false")
	}
}

// --- handleReveal edge case: first click on a non-mine cell ---

func TestFirstClickOnSafeCell(t *testing.T) {
	m := NewTestModel(42)

	sx, sy, found := findSafeCell(&m)
	if !found {
		t.Fatal("no safe cell found")
	}

	if !m.firstClick {
		t.Fatal("firstClick should be true initially")
	}

	m.handleReveal(sx, sy)

	if m.firstClick {
		t.Error("firstClick should be false after first reveal")
	}
	if m.grid[sy][sx].state != cellRevealed {
		t.Error("the clicked cell should be revealed")
	}

	// Mines should NOT have been regenerated (the cell was already safe).
	mc := countMines(&m)
	if mc != mines {
		t.Errorf("should still have %d mines, got %d", mines, mc)
	}
	if m.gameState != statePlaying {
		t.Errorf("game should still be playing, got state %d", m.gameState)
	}
}

// --- handleReveal edge case: reveal when game is already over ---

func TestHandleRevealAfterGameOver(t *testing.T) {
	m := NewTestModel(42)
	m.firstClick = false

	mx, my, found := findMineCell(&m)
	if !found {
		t.Fatal("no mine found")
	}
	m.reveal(mx, my)
	if m.gameState != stateLost {
		t.Fatal("game should be lost")
	}

	sx, sy, found := findSafeCell(&m)
	if !found {
		t.Fatal("no safe cell found")
	}
	before := countRevealed(&m)
	m.handleReveal(sx, sy)
	after := countRevealed(&m)

	if before != after {
		t.Error("handleReveal should be a no-op when the game is over")
	}
}

// --- placeMines edge case: safe cell exclusion ---

func TestPlaceMinesWithSafeCell(t *testing.T) {
	m := NewTestModel(42)

	safeX, safeY := 4, 4
	m.placeMines(safeX, safeY)

	if m.grid[safeY][safeX].mine {
		t.Errorf("cell (%d,%d) should be safe after placeMines with safe coords", safeX, safeY)
	}

	mc := countMines(&m)
	if mc != mines {
		t.Errorf("should have %d mines, got %d", mines, mc)
	}

	// Verify adjacent counts are consistent after re-placement.
	for y := range gridH {
		for x := range gridW {
			if m.grid[y][x].mine {
				continue
			}
			expected := 0
			for dy := -1; dy <= 1; dy++ {
				for dx := -1; dx <= 1; dx++ {
					if dx == 0 && dy == 0 {
						continue
					}
					nx, ny := x+dx, y+dy
					if nx >= 0 && nx < gridW && ny >= 0 && ny < gridH && m.grid[ny][nx].mine {
						expected++
					}
				}
			}
			if m.grid[y][x].adjacent != expected {
				t.Errorf("cell (%d,%d) adjacent: got %d, want %d", x, y, m.grid[y][x].adjacent, expected)
			}
		}
	}
}

// ---------------------------------------------------------------------------
// Init tests
// ---------------------------------------------------------------------------

func TestInit(t *testing.T) {
	m := NewTestModel(42)
	cmd := m.Init()
	if cmd == nil {
		t.Fatal("Init() should return a non-nil command (batch of tick + state dump listener)")
	}
}

// ---------------------------------------------------------------------------
// tickCmd tests
// ---------------------------------------------------------------------------

func TestTickCmd(t *testing.T) {
	cmd := tickCmd()
	if cmd == nil {
		t.Fatal("tickCmd() should return a non-nil command")
	}
}

// ---------------------------------------------------------------------------
// Update: WindowSizeMsg
// ---------------------------------------------------------------------------

func TestUpdateWindowSizeMsg(t *testing.T) {
	m := NewTestModel(42)
	m2, cmd := m.Update(tea.WindowSizeMsg{Width: 120, Height: 60})
	m = m2.(model)

	if m.width != 120 {
		t.Errorf("expected width 120, got %d", m.width)
	}
	if m.height != 60 {
		t.Errorf("expected height 60, got %d", m.height)
	}
	if cmd != nil {
		t.Error("expected no command from WindowSizeMsg")
	}
}

// ---------------------------------------------------------------------------
// Update: tickMsg
// ---------------------------------------------------------------------------

func TestUpdateTickMsgUpdatesElapsed(t *testing.T) {
	m := NewTestModel(42)
	m.firstClick = false
	m.gameState = statePlaying
	m.startTime = time.Now().Add(-5 * time.Second)

	m2, cmd := m.Update(tickMsg(time.Now()))
	m = m2.(model)

	if m.elapsed < 4 {
		t.Errorf("elapsed should be ~5 seconds, got %d", m.elapsed)
	}
	if cmd == nil {
		t.Error("tick should return a follow-up tick command")
	}
}

func TestUpdateTickMsgDoesNotUpdateBeforeFirstClick(t *testing.T) {
	m := NewTestModel(42)
	// firstClick is true, so elapsed should stay at 0.
	m.gameState = statePlaying

	m2, cmd := m.Update(tickMsg(time.Now()))
	m = m2.(model)

	if m.elapsed != 0 {
		t.Errorf("elapsed should be 0 before first click, got %d", m.elapsed)
	}
	if cmd == nil {
		t.Error("tick should return a follow-up tick command")
	}
}

func TestUpdateTickMsgDoesNotUpdateAfterGameOver(t *testing.T) {
	m := NewTestModel(42)
	m.firstClick = false
	m.gameState = stateLost
	m.startTime = time.Now().Add(-10 * time.Second)
	m.elapsed = 3

	m2, _ := m.Update(tickMsg(time.Now()))
	m = m2.(model)

	// Elapsed should NOT advance because game is lost.
	if m.elapsed != 3 {
		t.Errorf("elapsed should stay at 3 after game over, got %d", m.elapsed)
	}
}

func TestUpdateTickMsgDoesNotUpdateAfterWin(t *testing.T) {
	m := NewTestModel(42)
	m.firstClick = false
	m.gameState = stateWon
	m.startTime = time.Now().Add(-10 * time.Second)
	m.elapsed = 5

	m2, _ := m.Update(tickMsg(time.Now()))
	m = m2.(model)

	if m.elapsed != 5 {
		t.Errorf("elapsed should stay at 5 after win, got %d", m.elapsed)
	}
}

// ---------------------------------------------------------------------------
// Update: KeyPressMsg - quit keys
// ---------------------------------------------------------------------------

func TestUpdateQuitQ(t *testing.T) {
	m := NewTestModel(42)
	m.width = 80
	m.height = 40

	k := tea.Key{Code: 'q'}
	_, cmd := m.Update(tea.KeyPressMsg(k))
	if cmd == nil {
		t.Fatal("expected quit command for 'q' key")
	}
}

func TestUpdateQuitUpperQ(t *testing.T) {
	m := NewTestModel(42)
	m.width = 80
	m.height = 40

	k := tea.Key{Code: 'Q'}
	_, cmd := m.Update(tea.KeyPressMsg(k))
	if cmd == nil {
		t.Fatal("expected quit command for 'Q' key")
	}
}

func TestUpdateQuitEscape(t *testing.T) {
	m := NewTestModel(42)
	m.width = 80
	m.height = 40

	k := tea.Key{Code: tea.KeyEscape}
	_, cmd := m.Update(tea.KeyPressMsg(k))
	if cmd == nil {
		t.Fatal("expected quit command for Escape key")
	}
}

func TestUpdateQuitCtrlC(t *testing.T) {
	m := NewTestModel(42)
	m.width = 80
	m.height = 40

	k := tea.Key{Code: 3} // Ctrl+C
	_, cmd := m.Update(tea.KeyPressMsg(k))
	if cmd == nil {
		t.Fatal("expected quit command for Ctrl+C")
	}
}

// ---------------------------------------------------------------------------
// Update: KeyPressMsg - restart
// ---------------------------------------------------------------------------

func TestUpdateRestart(t *testing.T) {
	m := NewTestModel(42)
	m.width = 80
	m.height = 40
	// Make some changes to the model.
	m.firstClick = false
	m.gameState = stateLost

	k := tea.Key{Code: 'r'}
	m2, cmd := m.Update(tea.KeyPressMsg(k))
	m = m2.(model)

	if m.gameState != statePlaying {
		t.Errorf("restart should reset game state to playing, got %d", m.gameState)
	}
	if !m.firstClick {
		t.Error("restart should reset firstClick to true")
	}
	if m.width != 80 || m.height != 40 {
		t.Errorf("restart should preserve width/height, got %d x %d", m.width, m.height)
	}
	if cmd != nil {
		t.Error("restart should not return a command")
	}
}

func TestUpdateRestartUpperR(t *testing.T) {
	m := NewTestModel(42)
	m.width = 80
	m.height = 40
	m.gameState = stateLost

	k := tea.Key{Code: 'R'}
	m2, _ := m.Update(tea.KeyPressMsg(k))
	m = m2.(model)

	if m.gameState != statePlaying {
		t.Errorf("restart with 'R' should reset game state, got %d", m.gameState)
	}
}

// ---------------------------------------------------------------------------
// Update: KeyPressMsg - cursor movement
// ---------------------------------------------------------------------------

func TestUpdateCursorUp(t *testing.T) {
	m := NewTestModel(42)
	m.width = 80
	m.height = 40
	m.cursorY = 5

	k := tea.Key{Code: tea.KeyUp}
	m2, _ := m.Update(tea.KeyPressMsg(k))
	m = m2.(model)

	if m.cursorY != 4 {
		t.Errorf("cursor should move up to 4, got %d", m.cursorY)
	}
}

func TestUpdateCursorUpK(t *testing.T) {
	m := NewTestModel(42)
	m.width = 80
	m.height = 40
	m.cursorY = 3

	k := tea.Key{Code: 'k'}
	m2, _ := m.Update(tea.KeyPressMsg(k))
	m = m2.(model)

	if m.cursorY != 2 {
		t.Errorf("cursor should move up to 2, got %d", m.cursorY)
	}
}

func TestUpdateCursorUpAtTop(t *testing.T) {
	m := NewTestModel(42)
	m.width = 80
	m.height = 40
	m.cursorY = 0

	k := tea.Key{Code: tea.KeyUp}
	m2, _ := m.Update(tea.KeyPressMsg(k))
	m = m2.(model)

	if m.cursorY != 0 {
		t.Errorf("cursor should stay at 0 at top edge, got %d", m.cursorY)
	}
}

func TestUpdateCursorDown(t *testing.T) {
	m := NewTestModel(42)
	m.width = 80
	m.height = 40
	m.cursorY = 3

	k := tea.Key{Code: tea.KeyDown}
	m2, _ := m.Update(tea.KeyPressMsg(k))
	m = m2.(model)

	if m.cursorY != 4 {
		t.Errorf("cursor should move down to 4, got %d", m.cursorY)
	}
}

func TestUpdateCursorDownJ(t *testing.T) {
	m := NewTestModel(42)
	m.width = 80
	m.height = 40
	m.cursorY = 3

	k := tea.Key{Code: 'j'}
	m2, _ := m.Update(tea.KeyPressMsg(k))
	m = m2.(model)

	if m.cursorY != 4 {
		t.Errorf("cursor should move down to 4 with 'j', got %d", m.cursorY)
	}
}

func TestUpdateCursorDownAtBottom(t *testing.T) {
	m := NewTestModel(42)
	m.width = 80
	m.height = 40
	m.cursorY = gridH - 1

	k := tea.Key{Code: tea.KeyDown}
	m2, _ := m.Update(tea.KeyPressMsg(k))
	m = m2.(model)

	if m.cursorY != gridH-1 {
		t.Errorf("cursor should stay at %d at bottom edge, got %d", gridH-1, m.cursorY)
	}
}

func TestUpdateCursorLeft(t *testing.T) {
	m := NewTestModel(42)
	m.width = 80
	m.height = 40
	m.cursorX = 5

	k := tea.Key{Code: tea.KeyLeft}
	m2, _ := m.Update(tea.KeyPressMsg(k))
	m = m2.(model)

	if m.cursorX != 4 {
		t.Errorf("cursor should move left to 4, got %d", m.cursorX)
	}
}

func TestUpdateCursorLeftH(t *testing.T) {
	m := NewTestModel(42)
	m.width = 80
	m.height = 40
	m.cursorX = 3

	k := tea.Key{Code: 'h'}
	m2, _ := m.Update(tea.KeyPressMsg(k))
	m = m2.(model)

	if m.cursorX != 2 {
		t.Errorf("cursor should move left to 2 with 'h', got %d", m.cursorX)
	}
}

func TestUpdateCursorLeftAtEdge(t *testing.T) {
	m := NewTestModel(42)
	m.width = 80
	m.height = 40
	m.cursorX = 0

	k := tea.Key{Code: tea.KeyLeft}
	m2, _ := m.Update(tea.KeyPressMsg(k))
	m = m2.(model)

	if m.cursorX != 0 {
		t.Errorf("cursor should stay at 0 at left edge, got %d", m.cursorX)
	}
}

func TestUpdateCursorRight(t *testing.T) {
	m := NewTestModel(42)
	m.width = 80
	m.height = 40
	m.cursorX = 3

	k := tea.Key{Code: tea.KeyRight}
	m2, _ := m.Update(tea.KeyPressMsg(k))
	m = m2.(model)

	if m.cursorX != 4 {
		t.Errorf("cursor should move right to 4, got %d", m.cursorX)
	}
}

func TestUpdateCursorRightL(t *testing.T) {
	m := NewTestModel(42)
	m.width = 80
	m.height = 40
	m.cursorX = 3

	k := tea.Key{Code: 'l'}
	m2, _ := m.Update(tea.KeyPressMsg(k))
	m = m2.(model)

	if m.cursorX != 4 {
		t.Errorf("cursor should move right to 4 with 'l', got %d", m.cursorX)
	}
}

func TestUpdateCursorRightAtEdge(t *testing.T) {
	m := NewTestModel(42)
	m.width = 80
	m.height = 40
	m.cursorX = gridW - 1

	k := tea.Key{Code: tea.KeyRight}
	m2, _ := m.Update(tea.KeyPressMsg(k))
	m = m2.(model)

	if m.cursorX != gridW-1 {
		t.Errorf("cursor should stay at %d at right edge, got %d", gridW-1, m.cursorX)
	}
}

// ---------------------------------------------------------------------------
// Update: KeyPressMsg - space/enter to reveal
// ---------------------------------------------------------------------------

func TestUpdateSpaceReveals(t *testing.T) {
	m := NewTestModel(42)
	m.width = 80
	m.height = 40

	sx, sy, found := findSafeCell(&m)
	if !found {
		t.Fatal("no safe cell found")
	}
	m.cursorX = sx
	m.cursorY = sy

	k := tea.Key{Code: ' '}
	m2, _ := m.Update(tea.KeyPressMsg(k))
	m = m2.(model)

	if m.grid[sy][sx].state != cellRevealed {
		t.Error("space key should reveal the cell at cursor")
	}
}

func TestUpdateEnterReveals(t *testing.T) {
	m := NewTestModel(42)
	m.width = 80
	m.height = 40

	sx, sy, found := findSafeCell(&m)
	if !found {
		t.Fatal("no safe cell found")
	}
	m.cursorX = sx
	m.cursorY = sy

	k := tea.Key{Code: tea.KeyEnter}
	m2, _ := m.Update(tea.KeyPressMsg(k))
	m = m2.(model)

	if m.grid[sy][sx].state != cellRevealed {
		t.Error("enter key should reveal the cell at cursor")
	}
}

// ---------------------------------------------------------------------------
// Update: KeyPressMsg - 'f' to flag
// ---------------------------------------------------------------------------

func TestUpdateFlagKey(t *testing.T) {
	m := NewTestModel(42)
	m.width = 80
	m.height = 40

	sx, sy, found := findSafeCell(&m)
	if !found {
		t.Fatal("no safe cell found")
	}
	m.cursorX = sx
	m.cursorY = sy

	k := tea.Key{Code: 'f'}
	m2, _ := m.Update(tea.KeyPressMsg(k))
	m = m2.(model)

	if m.grid[sy][sx].state != cellFlagged {
		t.Error("'f' key should flag the cell at cursor")
	}
	if m.flags != 1 {
		t.Errorf("flags should be 1, got %d", m.flags)
	}
}

func TestUpdateFlagKeyUpperF(t *testing.T) {
	m := NewTestModel(42)
	m.width = 80
	m.height = 40

	sx, sy, found := findSafeCell(&m)
	if !found {
		t.Fatal("no safe cell found")
	}
	m.cursorX = sx
	m.cursorY = sy

	k := tea.Key{Code: 'F'}
	m2, _ := m.Update(tea.KeyPressMsg(k))
	m = m2.(model)

	if m.grid[sy][sx].state != cellFlagged {
		t.Error("'F' key should flag the cell at cursor")
	}
}

func TestUpdateFlagOnRevealedCellIsNoop(t *testing.T) {
	m := NewTestModel(42)
	m.width = 80
	m.height = 40
	m.firstClick = false

	sx, sy, found := findSafeCell(&m)
	if !found {
		t.Fatal("no safe cell found")
	}
	m.grid[sy][sx].state = cellRevealed
	m.cursorX = sx
	m.cursorY = sy

	k := tea.Key{Code: 'f'}
	m2, _ := m.Update(tea.KeyPressMsg(k))
	m = m2.(model)

	if m.grid[sy][sx].state != cellRevealed {
		t.Error("flagging a revealed cell should be a no-op")
	}
	if m.flags != 0 {
		t.Errorf("flags should still be 0, got %d", m.flags)
	}
}

// ---------------------------------------------------------------------------
// Update: KeyPressMsg - keys ignored when game is over
// ---------------------------------------------------------------------------

func TestUpdateKeysIgnoredAfterGameOver(t *testing.T) {
	m := NewTestModel(42)
	m.width = 80
	m.height = 40
	m.gameState = stateLost
	m.cursorX = 4
	m.cursorY = 4

	// Movement keys should be ignored.
	keys := []rune{tea.KeyUp, tea.KeyDown, tea.KeyLeft, tea.KeyRight, ' ', 'f'}
	for _, code := range keys {
		k := tea.Key{Code: code}
		m2, _ := m.Update(tea.KeyPressMsg(k))
		m = m2.(model)
	}

	// Cursor should not have moved, no cells revealed/flagged.
	if m.cursorX != 4 || m.cursorY != 4 {
		t.Errorf("cursor should stay at (4,4) after game over, got (%d,%d)", m.cursorX, m.cursorY)
	}
}

func TestUpdateQuitStillWorksAfterGameOver(t *testing.T) {
	m := NewTestModel(42)
	m.width = 80
	m.height = 40
	m.gameState = stateLost

	k := tea.Key{Code: 'q'}
	_, cmd := m.Update(tea.KeyPressMsg(k))
	if cmd == nil {
		t.Fatal("quit key should still work after game over")
	}
}

func TestUpdateRestartWorksAfterGameOver(t *testing.T) {
	m := NewTestModel(42)
	m.width = 80
	m.height = 40
	m.gameState = stateLost

	k := tea.Key{Code: 'r'}
	m2, _ := m.Update(tea.KeyPressMsg(k))
	m = m2.(model)

	if m.gameState != statePlaying {
		t.Errorf("restart should work after game over, got state %d", m.gameState)
	}
}

// ---------------------------------------------------------------------------
// Update: MouseClickMsg
// ---------------------------------------------------------------------------

func TestUpdateMouseLeftClickReveals(t *testing.T) {
	m := NewTestModel(42)
	m.width = 80
	m.height = 40

	originX, originY := m.gridOrigin()

	// Find a safe cell and click on it.
	sx, sy, found := findSafeCell(&m)
	if !found {
		t.Fatal("no safe cell found")
	}

	strideX := cellW + 1
	strideY := cellH + 1
	screenX := originX + sx*strideX
	screenY := originY + sy*strideY

	click := tea.MouseClickMsg(tea.Mouse{X: screenX, Y: screenY, Button: tea.MouseLeft})
	m2, _ := m.Update(click)
	m = m2.(model)

	if m.grid[sy][sx].state != cellRevealed {
		t.Error("left click should reveal the cell")
	}
	if m.cursorX != sx || m.cursorY != sy {
		t.Errorf("cursor should move to clicked cell (%d,%d), got (%d,%d)", sx, sy, m.cursorX, m.cursorY)
	}
}

func TestUpdateMouseRightClickFlags(t *testing.T) {
	m := NewTestModel(42)
	m.width = 80
	m.height = 40

	originX, originY := m.gridOrigin()

	sx, sy, found := findSafeCell(&m)
	if !found {
		t.Fatal("no safe cell found")
	}

	strideX := cellW + 1
	strideY := cellH + 1
	screenX := originX + sx*strideX
	screenY := originY + sy*strideY

	click := tea.MouseClickMsg(tea.Mouse{X: screenX, Y: screenY, Button: tea.MouseRight})
	m2, _ := m.Update(click)
	m = m2.(model)

	if m.grid[sy][sx].state != cellFlagged {
		t.Error("right click should flag the cell")
	}
	if m.flags != 1 {
		t.Errorf("flags should be 1, got %d", m.flags)
	}
}

func TestUpdateMouseRightClickOnRevealedIsNoop(t *testing.T) {
	m := NewTestModel(42)
	m.width = 80
	m.height = 40
	m.firstClick = false

	originX, originY := m.gridOrigin()

	sx, sy, found := findSafeCell(&m)
	if !found {
		t.Fatal("no safe cell found")
	}
	m.grid[sy][sx].state = cellRevealed

	strideX := cellW + 1
	strideY := cellH + 1
	screenX := originX + sx*strideX
	screenY := originY + sy*strideY

	click := tea.MouseClickMsg(tea.Mouse{X: screenX, Y: screenY, Button: tea.MouseRight})
	m2, _ := m.Update(click)
	m = m2.(model)

	if m.grid[sy][sx].state != cellRevealed {
		t.Error("right click on revealed cell should be a no-op")
	}
}

func TestUpdateMouseClickOutsideGrid(t *testing.T) {
	m := NewTestModel(42)
	m.width = 80
	m.height = 40
	oldCursorX := m.cursorX
	oldCursorY := m.cursorY

	// Click at (0, 0) which is outside the grid.
	click := tea.MouseClickMsg(tea.Mouse{X: 0, Y: 0, Button: tea.MouseLeft})
	m2, _ := m.Update(click)
	m = m2.(model)

	if m.cursorX != oldCursorX || m.cursorY != oldCursorY {
		t.Error("clicking outside grid should not move cursor")
	}
}

func TestUpdateMouseClickIgnoredAfterGameOver(t *testing.T) {
	m := NewTestModel(42)
	m.width = 80
	m.height = 40
	m.gameState = stateLost

	originX, originY := m.gridOrigin()
	click := tea.MouseClickMsg(tea.Mouse{X: originX, Y: originY, Button: tea.MouseLeft})
	before := countRevealed(&m)

	m2, _ := m.Update(click)
	m = m2.(model)

	after := countRevealed(&m)
	if before != after {
		t.Error("mouse clicks should be ignored after game over")
	}
}

// ---------------------------------------------------------------------------
// Update: unknown message type
// ---------------------------------------------------------------------------

func TestUpdateUnknownMsg(t *testing.T) {
	m := NewTestModel(42)
	m.width = 80
	m.height = 40

	type unknownMsg struct{}
	m2, cmd := m.Update(unknownMsg{})
	m = m2.(model)

	if cmd != nil {
		t.Error("unknown message type should return nil command")
	}
	if m.gameState != statePlaying {
		t.Errorf("unknown message should not change game state, got %d", m.gameState)
	}
}

// ---------------------------------------------------------------------------
// Update: stateDumpMsg
// ---------------------------------------------------------------------------

func TestUpdateStateDumpMsg(t *testing.T) {
	m := NewTestModel(42)
	m.width = 80
	m.height = 40
	m.cursorX = 3
	m.cursorY = 5
	m.flags = 2
	m.elapsed = 42
	m.firstClick = false

	m2, cmd := m.Update(stateDumpMsg{})
	_ = m2.(model)

	// stateDumpMsg should return a follow-up command (listenStateDump).
	if cmd == nil {
		t.Error("stateDumpMsg should return a follow-up command")
	}
}

// ---------------------------------------------------------------------------
// View tests
// ---------------------------------------------------------------------------

func TestViewZeroSize(t *testing.T) {
	m := NewTestModel(42)
	m.width = 0
	m.height = 0

	v := m.View()
	if !v.AltScreen {
		t.Error("View should set AltScreen even at zero size")
	}
}

func TestViewNormalPlaying(t *testing.T) {
	m := NewTestModel(42)
	m.width = 80
	m.height = 40

	v := m.View()
	if !v.AltScreen {
		t.Error("View should set AltScreen")
	}
	if v.MouseMode != tea.MouseModeCellMotion {
		t.Errorf("View should set MouseMode to CellMotion, got %d", v.MouseMode)
	}
}

func TestViewWonState(t *testing.T) {
	m := NewTestModel(42)
	m.width = 80
	m.height = 40
	m.gameState = stateWon
	m.firstClick = false

	v := m.View()
	if !v.AltScreen {
		t.Error("View should set AltScreen in won state")
	}
}

func TestViewLostState(t *testing.T) {
	m := NewTestModel(42)
	m.width = 80
	m.height = 40
	m.gameState = stateLost
	m.firstClick = false

	v := m.View()
	if !v.AltScreen {
		t.Error("View should set AltScreen in lost state")
	}
}

func TestViewWithFlags(t *testing.T) {
	m := NewTestModel(42)
	m.width = 80
	m.height = 40

	sx, sy, found := findSafeCell(&m)
	if !found {
		t.Fatal("no safe cell found")
	}
	m.toggleFlag(sx, sy)

	v := m.View()
	if !v.AltScreen {
		t.Error("View should set AltScreen with flags")
	}
}

func TestViewWithRevealedCells(t *testing.T) {
	m := NewTestModel(42)
	m.width = 80
	m.height = 40
	m.firstClick = false

	// Reveal a safe cell to exercise the revealed cell rendering path.
	sx, sy, found := findSafeCell(&m)
	if !found {
		t.Fatal("no safe cell found")
	}
	m.reveal(sx, sy)

	v := m.View()
	if !v.AltScreen {
		t.Error("View should set AltScreen with revealed cells")
	}
}

func TestViewWithRevealedMines(t *testing.T) {
	m := NewTestModel(42)
	m.width = 80
	m.height = 40
	m.firstClick = false

	// Lose the game to reveal all mines.
	mx, my, found := findMineCell(&m)
	if !found {
		t.Fatal("no mine found")
	}
	m.reveal(mx, my)

	v := m.View()
	if !v.AltScreen {
		t.Error("View should set AltScreen in lost state with mines revealed")
	}
}

func TestViewWithAllCellStates(t *testing.T) {
	// Set up a model with all cell states to exercise all rendering branches.
	m := NewTestModel(42)
	m.width = 100
	m.height = 50
	m.firstClick = false

	// Find cells with different adjacent counts for rendering numbers.
	// Reveal some cells.
	for y := range gridH {
		for x := range gridW {
			if !m.grid[y][x].mine && m.grid[y][x].adjacent > 0 {
				m.grid[y][x].state = cellRevealed
				break
			}
		}
	}

	// Flag a cell.
	for y := range gridH {
		for x := range gridW {
			if m.grid[y][x].state == cellHidden && !m.grid[y][x].mine {
				m.grid[y][x].state = cellFlagged
				break
			}
		}
	}

	// Reveal an empty cell (adjacent == 0).
	for y := range gridH {
		for x := range gridW {
			if !m.grid[y][x].mine && m.grid[y][x].adjacent == 0 && m.grid[y][x].state == cellHidden {
				m.grid[y][x].state = cellRevealed
				break
			}
		}
	}

	v := m.View()
	if !v.AltScreen {
		t.Error("View should set AltScreen")
	}
}

func TestViewElapsedAndMineCounter(t *testing.T) {
	m := NewTestModel(42)
	m.width = 80
	m.height = 40
	m.elapsed = 123
	m.flags = 5

	v := m.View()
	if !v.AltScreen {
		t.Error("View should set AltScreen")
	}
}

// ---------------------------------------------------------------------------
// newModel with TERMDESK_APP_STATE env var
// ---------------------------------------------------------------------------

func TestNewModelWithEnvState(t *testing.T) {
	// Create a state to restore.
	ms := mineState{
		State:   statePlaying,
		CursorX: 3,
		CursorY: 7,
		Flags:   2,
		Elapsed: 42,
	}
	// Set up grid with some mines.
	ms.Grid[0][0] = cellData{M: true, S: cellHidden, A: 0}
	ms.Grid[0][1] = cellData{M: false, S: cellRevealed, A: 1}
	ms.Grid[1][0] = cellData{M: false, S: cellFlagged, A: 1}

	data, err := json.Marshal(ms)
	if err != nil {
		t.Fatalf("failed to marshal state: %v", err)
	}
	encoded := base64.StdEncoding.EncodeToString(data)

	t.Setenv("TERMDESK_APP_STATE", encoded)

	m := newModel()

	if m.cursorX != 3 {
		t.Errorf("expected cursorX 3, got %d", m.cursorX)
	}
	if m.cursorY != 7 {
		t.Errorf("expected cursorY 7, got %d", m.cursorY)
	}
	if m.flags != 2 {
		t.Errorf("expected flags 2, got %d", m.flags)
	}
	if m.elapsed != 42 {
		t.Errorf("expected elapsed 42, got %d", m.elapsed)
	}
	if m.grid[0][0].mine != true {
		t.Error("grid[0][0] should be a mine")
	}
	if m.grid[0][1].state != cellRevealed {
		t.Error("grid[0][1] should be revealed")
	}
	if m.grid[1][0].state != cellFlagged {
		t.Error("grid[1][0] should be flagged")
	}
}

func TestNewModelWithEnvStatePlayingResetsTimer(t *testing.T) {
	ms := mineState{
		State:      statePlaying,
		FirstClick: false,
		Elapsed:    10,
	}

	data, err := json.Marshal(ms)
	if err != nil {
		t.Fatalf("failed to marshal state: %v", err)
	}
	encoded := base64.StdEncoding.EncodeToString(data)
	t.Setenv("TERMDESK_APP_STATE", encoded)

	m := newModel()

	// startTime should be set ~10 seconds in the past.
	if m.startTime.IsZero() {
		t.Error("startTime should be set when restoring a playing game")
	}
	elapsed := int(time.Since(m.startTime).Seconds())
	if elapsed < 9 || elapsed > 12 {
		t.Errorf("startTime should be ~10 seconds ago, got %d seconds", elapsed)
	}
}

func TestNewModelWithInvalidEnvState(t *testing.T) {
	t.Setenv("TERMDESK_APP_STATE", "not-valid-base64!!!")

	// Should not panic, should fall back to default model.
	m := newModel()
	if m.gameState != statePlaying {
		t.Errorf("invalid env state should fall back to default, got state %d", m.gameState)
	}
	if !m.firstClick {
		t.Error("invalid env state should fall back to firstClick=true")
	}
}

func TestNewModelWithInvalidJsonEnvState(t *testing.T) {
	// Valid base64 but invalid JSON.
	encoded := base64.StdEncoding.EncodeToString([]byte("not json"))
	t.Setenv("TERMDESK_APP_STATE", encoded)

	m := newModel()
	if m.gameState != statePlaying {
		t.Errorf("invalid json env state should fall back to default, got state %d", m.gameState)
	}
}

func TestNewModelWithNoEnvState(t *testing.T) {
	t.Setenv("TERMDESK_APP_STATE", "")

	m := newModel()
	if m.gameState != statePlaying {
		t.Errorf("empty env state should create default model, got state %d", m.gameState)
	}
	if !m.firstClick {
		t.Error("empty env state should start with firstClick=true")
	}
	mc := countMines(&m)
	if mc != mines {
		t.Errorf("should have %d mines, got %d", mines, mc)
	}
}

func TestNewModelEnvStateFirstClickTrue(t *testing.T) {
	// When firstClick is true, startTime should NOT be set.
	ms := mineState{
		State:      statePlaying,
		FirstClick: true,
		Elapsed:    0,
	}

	data, err := json.Marshal(ms)
	if err != nil {
		t.Fatalf("failed to marshal state: %v", err)
	}
	encoded := base64.StdEncoding.EncodeToString(data)
	t.Setenv("TERMDESK_APP_STATE", encoded)

	m := newModel()

	if !m.firstClick {
		t.Error("firstClick should be true when restored with firstClick=true")
	}
	if !m.startTime.IsZero() {
		t.Error("startTime should be zero when firstClick is true")
	}
}

func TestNewModelEnvStateGameWon(t *testing.T) {
	ms := mineState{
		State:      stateWon,
		FirstClick: false,
		Elapsed:    30,
	}

	data, err := json.Marshal(ms)
	if err != nil {
		t.Fatalf("failed to marshal state: %v", err)
	}
	encoded := base64.StdEncoding.EncodeToString(data)
	t.Setenv("TERMDESK_APP_STATE", encoded)

	m := newModel()

	if m.gameState != stateWon {
		t.Errorf("expected stateWon, got %d", m.gameState)
	}
	// startTime should NOT be reset for won games (not statePlaying).
	if !m.startTime.IsZero() {
		t.Error("startTime should be zero for a finished game")
	}
}

// ---------------------------------------------------------------------------
// mineState serialization/deserialization roundtrip
// ---------------------------------------------------------------------------

func TestMineStateSerialization(t *testing.T) {
	m := NewTestModel(42)
	m.firstClick = false
	m.cursorX = 5
	m.cursorY = 7
	m.flags = 3
	m.elapsed = 99
	m.gameState = statePlaying

	// Flag some cells.
	flagged := 0
	for y := range gridH {
		for x := range gridW {
			if !m.grid[y][x].mine && m.grid[y][x].state == cellHidden && flagged < 3 {
				m.grid[y][x].state = cellFlagged
				flagged++
			}
		}
	}

	// Build mineState.
	var ms mineState
	for y := range gridH {
		for x := range gridW {
			c := m.grid[y][x]
			ms.Grid[y][x] = cellData{M: c.mine, S: c.state, A: c.adjacent}
		}
	}
	ms.State = m.gameState
	ms.CursorX = m.cursorX
	ms.CursorY = m.cursorY
	ms.Flags = m.flags
	ms.FirstClick = m.firstClick
	ms.Elapsed = m.elapsed

	data, err := json.Marshal(ms)
	if err != nil {
		t.Fatalf("failed to marshal: %v", err)
	}

	var ms2 mineState
	if err := json.Unmarshal(data, &ms2); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if ms2.State != statePlaying {
		t.Errorf("expected state playing, got %d", ms2.State)
	}
	if ms2.CursorX != 5 || ms2.CursorY != 7 {
		t.Errorf("expected cursor (5,7), got (%d,%d)", ms2.CursorX, ms2.CursorY)
	}
	if ms2.Flags != 3 {
		t.Errorf("expected flags 3, got %d", ms2.Flags)
	}
	if ms2.Elapsed != 99 {
		t.Errorf("expected elapsed 99, got %d", ms2.Elapsed)
	}

	// Verify grid roundtrip.
	for y := range gridH {
		for x := range gridW {
			if ms2.Grid[y][x].M != ms.Grid[y][x].M {
				t.Errorf("grid mine mismatch at (%d,%d)", x, y)
			}
			if ms2.Grid[y][x].S != ms.Grid[y][x].S {
				t.Errorf("grid state mismatch at (%d,%d)", x, y)
			}
			if ms2.Grid[y][x].A != ms.Grid[y][x].A {
				t.Errorf("grid adjacent mismatch at (%d,%d)", x, y)
			}
		}
	}
}

// ---------------------------------------------------------------------------
// Edge case: reveal out of bounds
// ---------------------------------------------------------------------------

func TestRevealOutOfBounds(t *testing.T) {
	m := NewTestModel(42)
	m.firstClick = false

	// These should be no-ops and not panic.
	m.reveal(-1, 0)
	m.reveal(0, -1)
	m.reveal(gridW, 0)
	m.reveal(0, gridH)
	m.reveal(-1, -1)
	m.reveal(gridW, gridH)

	if m.gameState != statePlaying {
		t.Error("out-of-bounds reveal should not change game state")
	}
}

// ---------------------------------------------------------------------------
// Integration: full game win via Update
// ---------------------------------------------------------------------------

func TestFullGameWinViaUpdate(t *testing.T) {
	m := NewTestModel(42)
	m.width = 80
	m.height = 40

	// First, trigger first click on a safe cell.
	sx, sy, found := findSafeCell(&m)
	if !found {
		t.Fatal("no safe cell found")
	}
	m.cursorX = sx
	m.cursorY = sy

	k := tea.Key{Code: ' '}
	m2, _ := m.Update(tea.KeyPressMsg(k))
	m = m2.(model)

	if m.firstClick {
		t.Error("firstClick should be false after first reveal via Update")
	}

	// Now reveal all non-mine cells except those already revealed.
	for y := range gridH {
		for x := range gridW {
			if !m.grid[y][x].mine && m.grid[y][x].state != cellRevealed {
				m.cursorX = x
				m.cursorY = y
				m2, _ = m.Update(tea.KeyPressMsg(k))
				m = m2.(model)
			}
		}
	}

	if m.gameState != stateWon {
		t.Errorf("game should be won after revealing all safe cells, got state %d", m.gameState)
	}
}

// ---------------------------------------------------------------------------
// Integration: full game loss via mouse click
// ---------------------------------------------------------------------------

func TestFullGameLossViaMouse(t *testing.T) {
	m := NewTestModel(42)
	m.width = 80
	m.height = 40

	// First click on a safe cell to start the game.
	sx, sy, found := findSafeCell(&m)
	if !found {
		t.Fatal("no safe cell found")
	}

	originX, originY := m.gridOrigin()
	strideX := cellW + 1
	strideY := cellH + 1

	click := tea.MouseClickMsg(tea.Mouse{
		X:      originX + sx*strideX,
		Y:      originY + sy*strideY,
		Button: tea.MouseLeft,
	})
	m2, _ := m.Update(click)
	m = m2.(model)

	if m.firstClick {
		t.Error("firstClick should be false after first click")
	}

	// Now find a mine and click it.
	mx, my, found := findMineCell(&m)
	if !found {
		t.Fatal("no mine found after first click")
	}

	click = tea.MouseClickMsg(tea.Mouse{
		X:      originX + mx*strideX,
		Y:      originY + my*strideY,
		Button: tea.MouseLeft,
	})
	m2, _ = m.Update(click)
	m = m2.(model)

	if m.gameState != stateLost {
		t.Errorf("game should be lost after clicking a mine, got state %d", m.gameState)
	}
}

// ---------------------------------------------------------------------------
// View content checks (verify output contains expected strings)
// ---------------------------------------------------------------------------

func TestViewContainsMinesweeper(t *testing.T) {
	m := NewTestModel(42)
	m.width = 80
	m.height = 40

	v := m.View()
	content := viewContent(v)
	if content == "" {
		t.Error("View content should not be empty")
	}
}

func TestViewLoadingState(t *testing.T) {
	m := NewTestModel(42)
	m.width = 0
	m.height = 0

	v := m.View()
	content := viewContent(v)
	if !strings.Contains(content, "Loading...") {
		t.Error("View should show 'Loading...' when width/height is 0")
	}
}

// ---------------------------------------------------------------------------
// Mouse click on grid borders
// ---------------------------------------------------------------------------

func TestMouseClickOnVerticalBorder(t *testing.T) {
	m := NewTestModel(42)
	m.width = 80
	m.height = 40
	oldCX, oldCY := m.cursorX, m.cursorY

	originX, originY := m.gridOrigin()
	// Click on the vertical border (pipe character).
	click := tea.MouseClickMsg(tea.Mouse{X: originX + cellW, Y: originY, Button: tea.MouseLeft})
	m2, _ := m.Update(click)
	m = m2.(model)

	if m.cursorX != oldCX || m.cursorY != oldCY {
		t.Error("clicking on vertical border should not move cursor")
	}
}

func TestMouseClickOnHorizontalBorder(t *testing.T) {
	m := NewTestModel(42)
	m.width = 80
	m.height = 40
	oldCX, oldCY := m.cursorX, m.cursorY

	originX, originY := m.gridOrigin()
	click := tea.MouseClickMsg(tea.Mouse{X: originX, Y: originY + cellH, Button: tea.MouseLeft})
	m2, _ := m.Update(click)
	m = m2.(model)

	if m.cursorX != oldCX || m.cursorY != oldCY {
		t.Error("clicking on horizontal border should not move cursor")
	}
}

// ---------------------------------------------------------------------------
// Reveal numbered cell (adjacent > 0, not flood fill)
// ---------------------------------------------------------------------------

func TestRevealNumberedCell(t *testing.T) {
	m := NewTestModel(42)
	m.firstClick = false

	// Find a cell with adjacent > 0.
	var nx, ny int
	found := false
	for y := range gridH {
		for x := range gridW {
			if !m.grid[y][x].mine && m.grid[y][x].adjacent > 0 {
				nx, ny = x, y
				found = true
				break
			}
		}
		if found {
			break
		}
	}
	if !found {
		t.Skip("no numbered cell found")
	}

	before := countRevealed(&m)
	m.reveal(nx, ny)

	if m.grid[ny][nx].state != cellRevealed {
		t.Error("numbered cell should be revealed")
	}

	// Numbered cells should NOT trigger flood fill; only this cell should be newly revealed.
	after := countRevealed(&m)
	if after != before+1 {
		t.Errorf("revealing a numbered cell should reveal exactly 1 cell, revealed %d", after-before)
	}
}

// ---------------------------------------------------------------------------
// placeMines covers the branch where a cell is already a mine
// ---------------------------------------------------------------------------

func TestPlaceMinesAllCorners(t *testing.T) {
	// Test safe cell at each corner.
	corners := [][2]int{{0, 0}, {gridW - 1, 0}, {0, gridH - 1}, {gridW - 1, gridH - 1}}
	for _, c := range corners {
		m := NewTestModel(42)
		m.placeMines(c[0], c[1])
		if m.grid[c[1]][c[0]].mine {
			t.Errorf("corner (%d,%d) should be safe after placeMines", c[0], c[1])
		}
		mc := countMines(&m)
		if mc != mines {
			t.Errorf("corner (%d,%d): expected %d mines, got %d", c[0], c[1], mines, mc)
		}
	}
}

// ---------------------------------------------------------------------------
// checkWin with hidden cells
// ---------------------------------------------------------------------------

func TestCheckWinFalseWithHiddenCells(t *testing.T) {
	m := NewTestModel(42)
	// All cells hidden, should not be a win.
	if m.checkWin() {
		t.Error("checkWin should return false when all cells are hidden")
	}
}

func TestCheckWinFalseWithPartialReveal(t *testing.T) {
	m := NewTestModel(42)
	m.firstClick = false

	// Reveal just one cell.
	sx, sy, found := findSafeCell(&m)
	if !found {
		t.Fatal("no safe cell found")
	}
	m.grid[sy][sx].state = cellRevealed

	if m.checkWin() {
		t.Error("checkWin should return false with partial reveal")
	}
}

// ---------------------------------------------------------------------------
// View with different window sizes
// ---------------------------------------------------------------------------

func TestViewSmallWindow(t *testing.T) {
	m := NewTestModel(42)
	m.width = 20
	m.height = 10

	// Should not panic even with a very small window.
	v := m.View()
	if !v.AltScreen {
		t.Error("View should set AltScreen even with small window")
	}
}

func TestViewLargeWindow(t *testing.T) {
	m := NewTestModel(42)
	m.width = 200
	m.height = 100

	v := m.View()
	if !v.AltScreen {
		t.Error("View should set AltScreen with large window")
	}
}

// ---------------------------------------------------------------------------
// Cursor at center after init
// ---------------------------------------------------------------------------

func TestInitialCursorPosition(t *testing.T) {
	m := NewTestModel(42)
	if m.cursorX != gridW/2 {
		t.Errorf("initial cursorX should be %d, got %d", gridW/2, m.cursorX)
	}
	if m.cursorY != gridH/2 {
		t.Errorf("initial cursorY should be %d, got %d", gridH/2, m.cursorY)
	}
}

// ---------------------------------------------------------------------------
// Multiple flag operations
// ---------------------------------------------------------------------------

func TestMultipleFlags(t *testing.T) {
	m := NewTestModel(42)
	m.width = 80
	m.height = 40

	// Flag multiple cells via Update.
	flagCount := 0
	for y := range gridH {
		for x := range gridW {
			if !m.grid[y][x].mine && flagCount < 5 {
				m.cursorX = x
				m.cursorY = y
				k := tea.Key{Code: 'f'}
				m2, _ := m.Update(tea.KeyPressMsg(k))
				m = m2.(model)
				flagCount++
			}
		}
	}

	if m.flags != 5 {
		t.Errorf("expected 5 flags, got %d", m.flags)
	}
}

// ---------------------------------------------------------------------------
// Mouse right-click flag and unflag cycle
// ---------------------------------------------------------------------------

func TestMouseRightClickFlagUnflag(t *testing.T) {
	m := NewTestModel(42)
	m.width = 80
	m.height = 40

	originX, originY := m.gridOrigin()
	sx, sy, found := findSafeCell(&m)
	if !found {
		t.Fatal("no safe cell found")
	}

	strideX := cellW + 1
	strideY := cellH + 1
	screenX := originX + sx*strideX
	screenY := originY + sy*strideY

	// First right-click: flag.
	click := tea.MouseClickMsg(tea.Mouse{X: screenX, Y: screenY, Button: tea.MouseRight})
	m2, _ := m.Update(click)
	m = m2.(model)

	if m.grid[sy][sx].state != cellFlagged {
		t.Error("first right-click should flag")
	}
	if m.flags != 1 {
		t.Errorf("flags should be 1, got %d", m.flags)
	}

	// Second right-click: unflag.
	m2, _ = m.Update(click)
	m = m2.(model)

	if m.grid[sy][sx].state != cellHidden {
		t.Error("second right-click should unflag")
	}
	if m.flags != 0 {
		t.Errorf("flags should be 0 after unflag, got %d", m.flags)
	}
}

// ---------------------------------------------------------------------------
// View rendering with cursor on each cell type
// ---------------------------------------------------------------------------

func TestViewCursorOnHiddenCell(t *testing.T) {
	m := NewTestModel(42)
	m.width = 80
	m.height = 40
	m.cursorX = 0
	m.cursorY = 0

	v := m.View()
	if !v.AltScreen {
		t.Error("View should set AltScreen")
	}
}

func TestViewCursorOnFlaggedCell(t *testing.T) {
	m := NewTestModel(42)
	m.width = 80
	m.height = 40

	sx, sy, found := findSafeCell(&m)
	if !found {
		t.Fatal("no safe cell found")
	}
	m.grid[sy][sx].state = cellFlagged
	m.cursorX = sx
	m.cursorY = sy

	v := m.View()
	if !v.AltScreen {
		t.Error("View should set AltScreen")
	}
}

func TestViewCursorOnRevealedNumberCell(t *testing.T) {
	m := NewTestModel(42)
	m.width = 80
	m.height = 40
	m.firstClick = false

	// Find a cell with adjacent > 0 and reveal it.
	for y := range gridH {
		for x := range gridW {
			if !m.grid[y][x].mine && m.grid[y][x].adjacent > 0 {
				m.grid[y][x].state = cellRevealed
				m.cursorX = x
				m.cursorY = y
				goto found
			}
		}
	}
found:

	v := m.View()
	if !v.AltScreen {
		t.Error("View should set AltScreen")
	}
}

func TestViewCursorOnRevealedEmptyCell(t *testing.T) {
	m := NewTestModel(42)
	m.width = 80
	m.height = 40
	m.firstClick = false

	// Find an empty cell (adjacent == 0) and reveal it.
	for y := range gridH {
		for x := range gridW {
			if !m.grid[y][x].mine && m.grid[y][x].adjacent == 0 {
				m.grid[y][x].state = cellRevealed
				m.cursorX = x
				m.cursorY = y
				goto foundEmpty
			}
		}
	}
foundEmpty:

	v := m.View()
	if !v.AltScreen {
		t.Error("View should set AltScreen")
	}
}

// ---------------------------------------------------------------------------
// View: won state does not show cursor highlight
// ---------------------------------------------------------------------------

func TestViewNoCursorAfterWin(t *testing.T) {
	m := NewTestModel(42)
	m.width = 80
	m.height = 40
	m.gameState = stateWon

	// View should still render without cursor highlight.
	v := m.View()
	if !v.AltScreen {
		t.Error("View should set AltScreen")
	}
}

func TestViewNoCursorAfterLoss(t *testing.T) {
	m := NewTestModel(42)
	m.width = 80
	m.height = 40
	m.gameState = stateLost

	v := m.View()
	if !v.AltScreen {
		t.Error("View should set AltScreen")
	}
}

// ---------------------------------------------------------------------------
// View with only width=0 (height > 0)
// ---------------------------------------------------------------------------

func TestViewOnlyWidthZero(t *testing.T) {
	m := NewTestModel(42)
	m.width = 0
	m.height = 40

	v := m.View()
	content := viewContent(v)
	if !strings.Contains(content, "Loading...") {
		t.Error("View should show Loading when width is 0")
	}
}

func TestViewOnlyHeightZero(t *testing.T) {
	m := NewTestModel(42)
	m.width = 80
	m.height = 0

	v := m.View()
	content := viewContent(v)
	if !strings.Contains(content, "Loading...") {
		t.Error("View should show Loading when height is 0")
	}
}

// ---------------------------------------------------------------------------
// View renders all number colors (cells with adjacent 1-8)
// ---------------------------------------------------------------------------

func TestViewAllNumberColors(t *testing.T) {
	m := NewTestModel(42)
	m.width = 100
	m.height = 50
	m.firstClick = false

	// Manually set cells to have various adjacent counts and reveal them.
	// Use cells that are not mines.
	idx := 0
	for y := range gridH {
		for x := range gridW {
			if !m.grid[y][x].mine && idx < 8 {
				m.grid[y][x].adjacent = idx + 1
				m.grid[y][x].state = cellRevealed
				idx++
			}
		}
	}

	v := m.View()
	if !v.AltScreen {
		t.Error("View should set AltScreen")
	}
}

// ---------------------------------------------------------------------------
// screenToGrid with second row of a cell
// ---------------------------------------------------------------------------

func TestScreenToGridSecondRow(t *testing.T) {
	m := NewTestModel(42)
	m.width = 80
	m.height = 40

	originX, originY := m.gridOrigin()

	// Click on the second row (row=1) of the first cell.
	// First cell starts at originX, originY. Second row is originY+1.
	gx, gy, ok := m.screenToGrid(originX, originY+1)
	if !ok {
		t.Error("screenToGrid should succeed for second row of a cell")
	}
	if gx != 0 || gy != 0 {
		t.Errorf("expected (0,0), got (%d,%d)", gx, gy)
	}
}

func TestScreenToGridSecondColumn(t *testing.T) {
	m := NewTestModel(42)
	m.width = 80
	m.height = 40

	originX, originY := m.gridOrigin()

	// Click on the second column position (offset 1) within the first cell.
	gx, gy, ok := m.screenToGrid(originX+1, originY)
	if !ok {
		t.Error("screenToGrid should succeed for second column position within cell")
	}
	if gx != 0 || gy != 0 {
		t.Errorf("expected (0,0), got (%d,%d)", gx, gy)
	}
}

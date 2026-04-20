package main

import (
	"encoding/base64"
	"encoding/json"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
)

// newTestModel creates a deterministic model for testing.
func newTestModel() model {
	return newModelWithSeed(42)
}

func TestInitialState(t *testing.T) {
	m := newTestModel()

	if len(m.snake) != 3 {
		t.Fatalf("expected snake length 3, got %d", len(m.snake))
	}
	// Head should be at center.
	head := m.snake[0]
	if head.x != fieldW/2 || head.y != fieldH/2 {
		t.Fatalf("expected head at (%d,%d), got (%d,%d)", fieldW/2, fieldH/2, head.x, head.y)
	}
	// Direction should be right.
	if m.dir != dirRight {
		t.Fatalf("expected initial direction right, got %d", m.dir)
	}
	if m.gameOver {
		t.Fatal("game should not be over initially")
	}
	if m.score != 0 {
		t.Fatalf("expected initial score 0, got %d", m.score)
	}
}

func TestMoveRight(t *testing.T) {
	m := newTestModel()
	headBefore := m.snake[0]

	m.dir = dirRight
	m.nextDir = dirRight
	m.step()

	if m.gameOver {
		t.Fatal("unexpected game over")
	}
	head := m.snake[0]
	if head.x != headBefore.x+1 || head.y != headBefore.y {
		t.Fatalf("expected head at (%d,%d), got (%d,%d)", headBefore.x+1, headBefore.y, head.x, head.y)
	}
	if len(m.snake) != 3 {
		t.Fatalf("snake length should remain 3, got %d", len(m.snake))
	}
}

func TestMoveLeft(t *testing.T) {
	m := newTestModel()
	// Face up first (can't go left from right without turning first).
	m.dir = dirUp
	m.nextDir = dirUp
	m.step()
	headBefore := m.snake[0]

	m.dir = dirLeft
	m.nextDir = dirLeft
	m.step()

	if m.gameOver {
		t.Fatal("unexpected game over")
	}
	head := m.snake[0]
	if head.x != headBefore.x-1 || head.y != headBefore.y {
		t.Fatalf("expected head at (%d,%d), got (%d,%d)", headBefore.x-1, headBefore.y, head.x, head.y)
	}
}

func TestMoveUp(t *testing.T) {
	m := newTestModel()
	headBefore := m.snake[0]

	m.dir = dirUp
	m.nextDir = dirUp
	m.step()

	if m.gameOver {
		t.Fatal("unexpected game over")
	}
	head := m.snake[0]
	if head.x != headBefore.x || head.y != headBefore.y-1 {
		t.Fatalf("expected head at (%d,%d), got (%d,%d)", headBefore.x, headBefore.y-1, head.x, head.y)
	}
}

func TestMoveDown(t *testing.T) {
	m := newTestModel()
	headBefore := m.snake[0]

	m.dir = dirDown
	m.nextDir = dirDown
	m.step()

	if m.gameOver {
		t.Fatal("unexpected game over")
	}
	head := m.snake[0]
	if head.x != headBefore.x || head.y != headBefore.y+1 {
		t.Fatalf("expected head at (%d,%d), got (%d,%d)", headBefore.x, headBefore.y+1, head.x, head.y)
	}
}

func TestFoodCollisionGrows(t *testing.T) {
	m := newTestModel()
	// Place food directly in front of the snake (right of head).
	head := m.snake[0]
	m.food = point{head.x + 1, head.y}
	m.dir = dirRight
	m.nextDir = dirRight
	lenBefore := len(m.snake)
	scoreBefore := m.score

	m.step()

	if m.gameOver {
		t.Fatal("unexpected game over")
	}
	if len(m.snake) != lenBefore+1 {
		t.Fatalf("expected snake length %d, got %d", lenBefore+1, len(m.snake))
	}
	if m.score != scoreBefore+1 {
		t.Fatalf("expected score %d, got %d", scoreBefore+1, m.score)
	}
	// Food should have moved (different position).
	if m.food == (point{head.x + 1, head.y}) {
		// Could theoretically land in same spot, but extremely unlikely with 800 cells.
		// Just check snake is at the food position.
	}
}

func TestWallCollisionRight(t *testing.T) {
	m := newTestModel()
	// Move head to right edge.
	m.snake[0] = point{fieldW - 1, fieldH / 2}
	m.snake[1] = point{fieldW - 2, fieldH / 2}
	m.snake[2] = point{fieldW - 3, fieldH / 2}
	m.dir = dirRight
	m.nextDir = dirRight

	m.step()

	if !m.gameOver {
		t.Fatal("expected game over on right wall collision")
	}
}

func TestWallCollisionLeft(t *testing.T) {
	m := newTestModel()
	m.snake[0] = point{0, fieldH / 2}
	m.snake[1] = point{1, fieldH / 2}
	m.snake[2] = point{2, fieldH / 2}
	m.dir = dirLeft
	m.nextDir = dirLeft

	m.step()

	if !m.gameOver {
		t.Fatal("expected game over on left wall collision")
	}
}

func TestWallCollisionTop(t *testing.T) {
	m := newTestModel()
	m.snake[0] = point{fieldW / 2, 0}
	m.snake[1] = point{fieldW / 2, 1}
	m.snake[2] = point{fieldW / 2, 2}
	m.dir = dirUp
	m.nextDir = dirUp

	m.step()

	if !m.gameOver {
		t.Fatal("expected game over on top wall collision")
	}
}

func TestWallCollisionBottom(t *testing.T) {
	m := newTestModel()
	m.snake[0] = point{fieldW / 2, fieldH - 1}
	m.snake[1] = point{fieldW / 2, fieldH - 2}
	m.snake[2] = point{fieldW / 2, fieldH - 3}
	m.dir = dirDown
	m.nextDir = dirDown

	m.step()

	if !m.gameOver {
		t.Fatal("expected game over on bottom wall collision")
	}
}

func TestSelfCollision(t *testing.T) {
	m := newTestModel()
	// Create a snake forming a loop where head moves into a non-tail body segment.
	// Head at (5,3) moving right. (6,3) is occupied by a body segment that is NOT the tail.
	//
	//   6,2  7,2
	//   5,3 [6,3] 7,3  <- head moves right into (6,3)
	//   5,4  6,4  7,4  <- tail at 7,4
	m.snake = []point{
		{5, 3}, // head — moving right into (6, 3)
		{5, 4}, // body
		{6, 4}, // body
		{7, 4}, // body
		{7, 3}, // body
		{7, 2}, // body
		{6, 2}, // body
		{6, 3}, // body — NOT the tail, head will collide here
		{5, 2}, // tail — this will vacate, but (6,3) won't
	}
	m.dir = dirRight
	m.nextDir = dirRight
	m.food = point{0, 0}

	m.step()

	if !m.gameOver {
		t.Fatal("expected game over on self collision")
	}
}

func TestSpeedIncrease(t *testing.T) {
	m := newTestModel()
	head := m.snake[0]
	speedBefore := m.speed

	// Eat 5 food items to trigger speed increase.
	for i := 0; i < 5; i++ {
		h := m.snake[0]
		m.food = point{h.x + 1, h.y}
		m.dir = dirRight
		m.nextDir = dirRight
		m.step()
		if m.gameOver {
			t.Fatalf("unexpected game over at food %d, head was (%d,%d)", i+1, h.x, h.y)
		}
	}

	_ = head // used above indirectly
	if m.score != 5 {
		t.Fatalf("expected score 5, got %d", m.score)
	}
	if m.speed >= speedBefore {
		t.Fatalf("expected speed to decrease after 5 food, was %v now %v", speedBefore, m.speed)
	}
}

func TestRestart(t *testing.T) {
	m := newTestModel()
	// Cause game over.
	m.snake[0] = point{fieldW - 1, fieldH / 2}
	m.snake[1] = point{fieldW - 2, fieldH / 2}
	m.snake[2] = point{fieldW - 3, fieldH / 2}
	m.dir = dirRight
	m.nextDir = dirRight
	m.step()

	if !m.gameOver {
		t.Fatal("expected game over")
	}

	m.reset()

	if m.gameOver {
		t.Fatal("game should not be over after reset")
	}
	if m.score != 0 {
		t.Fatalf("expected score 0 after reset, got %d", m.score)
	}
	if len(m.snake) != 3 {
		t.Fatalf("expected snake length 3 after reset, got %d", len(m.snake))
	}
	if m.speed != initialSpeed {
		t.Fatalf("expected speed %v after reset, got %v", initialSpeed, m.speed)
	}
}

func TestDirectionChangePreventReverse(t *testing.T) {
	// The Update function should prevent reversing direction.
	m := newTestModel()
	m.width = 100
	m.height = 50
	m.dir = dirRight
	m.nextDir = dirRight

	// Try to go left (opposite of right) — should be rejected.
	k := tea.Key{Code: tea.KeyLeft}
	m2, _ := m.Update(tea.KeyPressMsg(k))
	m = m2.(model)

	if m.nextDir != dirRight {
		t.Fatalf("expected direction to stay right when trying to reverse, got %d", m.nextDir)
	}

	// Going up should work.
	k = tea.Key{Code: tea.KeyUp}
	m2, _ = m.Update(tea.KeyPressMsg(k))
	m = m2.(model)

	if m.nextDir != dirUp {
		t.Fatal("expected direction to change to up")
	}
}

func TestFoodNotOnSnake(t *testing.T) {
	m := newTestModel()
	// Verify food is not on any snake segment.
	for _, p := range m.snake {
		if p == m.food {
			t.Fatal("food should not be placed on snake")
		}
	}
}

// ---------------------------------------------------------------------------
// Update: key handling tests
// ---------------------------------------------------------------------------

func TestUpdateQuitKey(t *testing.T) {
	m := newTestModel()
	m.width = 80
	m.height = 40

	// 'q' should return a quit command.
	k := tea.Key{Code: 'q'}
	_, cmd := m.Update(tea.KeyPressMsg(k))
	if cmd == nil {
		t.Fatal("expected quit command for 'q' key")
	}
}

func TestUpdateQuitUppercase(t *testing.T) {
	m := newTestModel()
	m.width = 80
	m.height = 40

	k := tea.Key{Code: 'Q'}
	_, cmd := m.Update(tea.KeyPressMsg(k))
	if cmd == nil {
		t.Fatal("expected quit command for 'Q' key")
	}
}

func TestUpdateQuitCtrlC(t *testing.T) {
	m := newTestModel()
	m.width = 80
	m.height = 40

	// Ctrl+C is rune 3.
	k := tea.Key{Code: 3}
	_, cmd := m.Update(tea.KeyPressMsg(k))
	if cmd == nil {
		t.Fatal("expected quit command for Ctrl+C")
	}
}

func TestUpdateResetWhenGameOver(t *testing.T) {
	m := newTestModel()
	m.width = 80
	m.height = 40
	m.gameOver = true
	m.score = 10

	k := tea.Key{Code: 'r'}
	m2, cmd := m.Update(tea.KeyPressMsg(k))
	m = m2.(model)

	if m.gameOver {
		t.Fatal("expected gameOver to be false after reset")
	}
	if m.score != 0 {
		t.Fatalf("expected score 0 after reset, got %d", m.score)
	}
	if m.started {
		t.Fatal("expected started to be false after reset")
	}
	if cmd != nil {
		t.Fatal("expected no command after reset (tick starts on first direction key)")
	}
}

func TestUpdateResetUppercaseR(t *testing.T) {
	m := newTestModel()
	m.width = 80
	m.height = 40
	m.gameOver = true

	k := tea.Key{Code: 'R'}
	m2, _ := m.Update(tea.KeyPressMsg(k))
	m = m2.(model)

	if m.gameOver {
		t.Fatal("expected gameOver to be false after 'R' reset")
	}
}

func TestUpdateResetIgnoredWhenNotGameOver(t *testing.T) {
	m := newTestModel()
	m.width = 80
	m.height = 40
	m.started = true
	scoreBefore := m.score

	k := tea.Key{Code: 'r'}
	m2, _ := m.Update(tea.KeyPressMsg(k))
	m = m2.(model)

	// 'r' should be a no-op when game is not over.
	if m.score != scoreBefore {
		t.Fatal("score should not change when pressing 'r' without game over")
	}
	if !m.started {
		t.Fatal("started should remain true")
	}
}

func TestUpdatePauseToggle(t *testing.T) {
	m := newTestModel()
	m.width = 80
	m.height = 40
	m.started = true

	// Pause the game.
	k := tea.Key{Code: 'p'}
	m2, cmd := m.Update(tea.KeyPressMsg(k))
	m = m2.(model)

	if !m.paused {
		t.Fatal("expected game to be paused")
	}
	if cmd != nil {
		t.Fatal("expected no command when pausing")
	}

	// Unpause the game — should return a tick command.
	m2, cmd = m.Update(tea.KeyPressMsg(k))
	m = m2.(model)

	if m.paused {
		t.Fatal("expected game to be unpaused")
	}
	if cmd == nil {
		t.Fatal("expected tick command when unpausing")
	}
}

func TestUpdatePauseUppercaseP(t *testing.T) {
	m := newTestModel()
	m.width = 80
	m.height = 40
	m.started = true

	k := tea.Key{Code: 'P'}
	m2, _ := m.Update(tea.KeyPressMsg(k))
	m = m2.(model)

	if !m.paused {
		t.Fatal("expected game to be paused with 'P'")
	}
}

func TestUpdatePauseIgnoredWhenNotStarted(t *testing.T) {
	m := newTestModel()
	m.width = 80
	m.height = 40

	k := tea.Key{Code: 'p'}
	m2, _ := m.Update(tea.KeyPressMsg(k))
	m = m2.(model)

	if m.paused {
		t.Fatal("pause should be ignored when game is not started")
	}
}

func TestUpdatePauseIgnoredWhenGameOver(t *testing.T) {
	m := newTestModel()
	m.width = 80
	m.height = 40
	m.started = true
	m.gameOver = true

	k := tea.Key{Code: 'p'}
	m2, _ := m.Update(tea.KeyPressMsg(k))
	m = m2.(model)

	if m.paused {
		t.Fatal("pause should be ignored when game is over")
	}
}

func TestUpdateArrowKeysStartGame(t *testing.T) {
	m := newTestModel()
	m.width = 80
	m.height = 40

	if m.started {
		t.Fatal("game should not be started initially")
	}

	// Pressing up arrow should start the game and return a tick command.
	k := tea.Key{Code: tea.KeyUp}
	m2, cmd := m.Update(tea.KeyPressMsg(k))
	m = m2.(model)

	if !m.started {
		t.Fatal("expected game to be started after direction key")
	}
	if m.nextDir != dirUp {
		t.Fatalf("expected direction up, got %d", m.nextDir)
	}
	if cmd == nil {
		t.Fatal("expected tick command when game starts")
	}
}

func TestUpdateArrowDownStartsGame(t *testing.T) {
	m := newTestModel()
	m.width = 80
	m.height = 40

	k := tea.Key{Code: tea.KeyDown}
	m2, cmd := m.Update(tea.KeyPressMsg(k))
	m = m2.(model)

	if !m.started {
		t.Fatal("expected game to be started after down arrow")
	}
	if m.nextDir != dirDown {
		t.Fatalf("expected direction down, got %d", m.nextDir)
	}
	if cmd == nil {
		t.Fatal("expected tick command when game starts")
	}
}

func TestUpdateArrowRightStartsGame(t *testing.T) {
	m := newTestModel()
	m.width = 80
	m.height = 40
	// Initial direction is right; pressing right is same direction but still starts.
	// Actually right is not a change from right, but moved=true should still be set.
	// Wait: k.Code == tea.KeyRight, m.dir == dirRight, dirLeft != dirRight... check:
	// case k.Code == tea.KeyRight: if m.dir != dirLeft { m.nextDir = dirRight; moved = true }
	// m.dir is dirRight, dirLeft=1, so dirRight(0) != dirLeft(1) is true. moved=true.

	k := tea.Key{Code: tea.KeyRight}
	m2, cmd := m.Update(tea.KeyPressMsg(k))
	m = m2.(model)

	if !m.started {
		t.Fatal("expected game to be started after right arrow")
	}
	if cmd == nil {
		t.Fatal("expected tick command when game starts")
	}
}

func TestUpdateDirectionIgnoredWhenPaused(t *testing.T) {
	m := newTestModel()
	m.width = 80
	m.height = 40
	m.started = true
	m.paused = true
	m.dir = dirRight
	m.nextDir = dirRight

	k := tea.Key{Code: tea.KeyUp}
	m2, _ := m.Update(tea.KeyPressMsg(k))
	m = m2.(model)

	if m.nextDir != dirRight {
		t.Fatalf("direction should not change when paused, got %d", m.nextDir)
	}
}

func TestUpdateDirectionIgnoredWhenGameOver(t *testing.T) {
	m := newTestModel()
	m.width = 80
	m.height = 40
	m.started = true
	m.gameOver = true
	m.dir = dirRight
	m.nextDir = dirRight

	k := tea.Key{Code: tea.KeyUp}
	m2, _ := m.Update(tea.KeyPressMsg(k))
	m = m2.(model)

	if m.nextDir != dirRight {
		t.Fatalf("direction should not change when game is over, got %d", m.nextDir)
	}
}

func TestUpdateVimKeysDirection(t *testing.T) {
	m := newTestModel()
	m.width = 80
	m.height = 40
	m.started = true
	m.dir = dirRight
	m.nextDir = dirRight

	// 'k' for up.
	k := tea.Key{Code: 'k'}
	m2, _ := m.Update(tea.KeyPressMsg(k))
	m = m2.(model)
	if m.nextDir != dirUp {
		t.Fatalf("expected dirUp from 'k', got %d", m.nextDir)
	}

	// Apply direction so we can test next change.
	m.dir = dirUp

	// 'h' for left.
	k = tea.Key{Code: 'h'}
	m2, _ = m.Update(tea.KeyPressMsg(k))
	m = m2.(model)
	if m.nextDir != dirLeft {
		t.Fatalf("expected dirLeft from 'h', got %d", m.nextDir)
	}

	m.dir = dirLeft

	// 'j' for down.
	k = tea.Key{Code: 'j'}
	m2, _ = m.Update(tea.KeyPressMsg(k))
	m = m2.(model)
	if m.nextDir != dirDown {
		t.Fatalf("expected dirDown from 'j', got %d", m.nextDir)
	}

	m.dir = dirDown

	// 'l' for right.
	k = tea.Key{Code: 'l'}
	m2, _ = m.Update(tea.KeyPressMsg(k))
	m = m2.(model)
	if m.nextDir != dirRight {
		t.Fatalf("expected dirRight from 'l', got %d", m.nextDir)
	}
}

func TestUpdateVimKeysUppercase(t *testing.T) {
	m := newTestModel()
	m.width = 80
	m.height = 40
	m.started = true
	m.dir = dirRight
	m.nextDir = dirRight

	// 'K' for up.
	k := tea.Key{Code: 'K'}
	m2, _ := m.Update(tea.KeyPressMsg(k))
	m = m2.(model)
	if m.nextDir != dirUp {
		t.Fatalf("expected dirUp from 'K', got %d", m.nextDir)
	}

	m.dir = dirUp

	// 'H' for left.
	k = tea.Key{Code: 'H'}
	m2, _ = m.Update(tea.KeyPressMsg(k))
	m = m2.(model)
	if m.nextDir != dirLeft {
		t.Fatalf("expected dirLeft from 'H', got %d", m.nextDir)
	}

	m.dir = dirLeft

	// 'J' for down.
	k = tea.Key{Code: 'J'}
	m2, _ = m.Update(tea.KeyPressMsg(k))
	m = m2.(model)
	if m.nextDir != dirDown {
		t.Fatalf("expected dirDown from 'J', got %d", m.nextDir)
	}

	m.dir = dirDown

	// 'L' for right.
	k = tea.Key{Code: 'L'}
	m2, _ = m.Update(tea.KeyPressMsg(k))
	m = m2.(model)
	if m.nextDir != dirRight {
		t.Fatalf("expected dirRight from 'L', got %d", m.nextDir)
	}
}

func TestUpdateAllReverseDirectionsPrevented(t *testing.T) {
	tests := []struct {
		name    string
		dir     direction
		key     tea.Key
		wantDir direction
	}{
		{"up blocks down arrow", dirUp, tea.Key{Code: tea.KeyDown}, dirUp},
		{"up blocks j", dirUp, tea.Key{Code: 'j'}, dirUp},
		{"down blocks up arrow", dirDown, tea.Key{Code: tea.KeyUp}, dirDown},
		{"down blocks k", dirDown, tea.Key{Code: 'k'}, dirDown},
		{"left blocks right arrow", dirLeft, tea.Key{Code: tea.KeyRight}, dirLeft},
		{"left blocks l", dirLeft, tea.Key{Code: 'l'}, dirLeft},
		{"right blocks left arrow", dirRight, tea.Key{Code: tea.KeyLeft}, dirRight},
		{"right blocks h", dirRight, tea.Key{Code: 'h'}, dirRight},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := newTestModel()
			m.width = 80
			m.height = 40
			m.started = true
			m.dir = tt.dir
			m.nextDir = tt.dir

			m2, _ := m.Update(tea.KeyPressMsg(tt.key))
			m = m2.(model)

			if m.nextDir != tt.wantDir {
				t.Fatalf("expected direction %d, got %d", tt.wantDir, m.nextDir)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Update: WindowSizeMsg
// ---------------------------------------------------------------------------

func TestUpdateWindowSizeMsg(t *testing.T) {
	m := newTestModel()

	m2, cmd := m.Update(tea.WindowSizeMsg{Width: 120, Height: 60})
	m = m2.(model)

	if m.width != 120 {
		t.Fatalf("expected width 120, got %d", m.width)
	}
	if m.height != 60 {
		t.Fatalf("expected height 60, got %d", m.height)
	}
	if cmd != nil {
		t.Fatal("expected no command from WindowSizeMsg")
	}
}

// ---------------------------------------------------------------------------
// Update: tickMsg handling
// ---------------------------------------------------------------------------

func TestUpdateTickMsgAdvancesGame(t *testing.T) {
	m := newTestModel()
	m.width = 80
	m.height = 40
	m.started = true
	m.dir = dirRight
	m.nextDir = dirRight
	headBefore := m.snake[0]

	m2, cmd := m.Update(tickMsg{gen: m.gen})
	m = m2.(model)

	head := m.snake[0]
	if head.x != headBefore.x+1 || head.y != headBefore.y {
		t.Fatalf("expected head to move right from (%d,%d) to (%d,%d), got (%d,%d)",
			headBefore.x, headBefore.y, headBefore.x+1, headBefore.y, head.x, head.y)
	}
	if cmd == nil {
		t.Fatal("expected tick command to continue game loop")
	}
}

func TestUpdateTickMsgAppliesNextDir(t *testing.T) {
	m := newTestModel()
	m.width = 80
	m.height = 40
	m.started = true
	m.dir = dirRight
	m.nextDir = dirUp

	m2, _ := m.Update(tickMsg{gen: m.gen})
	m = m2.(model)

	if m.dir != dirUp {
		t.Fatalf("expected dir to be updated to nextDir (up), got %d", m.dir)
	}
}

func TestUpdateTickMsgStaleGenIgnored(t *testing.T) {
	m := newTestModel()
	m.width = 80
	m.height = 40
	m.started = true
	headBefore := m.snake[0]

	// Send tick with old gen.
	m2, cmd := m.Update(tickMsg{gen: m.gen - 1})
	m = m2.(model)

	head := m.snake[0]
	if head != headBefore {
		t.Fatal("stale tick should not move the snake")
	}
	if cmd != nil {
		t.Fatal("stale tick should not return a command")
	}
}

func TestUpdateTickMsgIgnoredWhenGameOver(t *testing.T) {
	m := newTestModel()
	m.width = 80
	m.height = 40
	m.started = true
	m.gameOver = true
	headBefore := m.snake[0]

	m2, cmd := m.Update(tickMsg{gen: m.gen})
	m = m2.(model)

	if m.snake[0] != headBefore {
		t.Fatal("tick should not move snake when game is over")
	}
	if cmd != nil {
		t.Fatal("tick should return no command when game is over")
	}
}

func TestUpdateTickMsgIgnoredWhenPaused(t *testing.T) {
	m := newTestModel()
	m.width = 80
	m.height = 40
	m.started = true
	m.paused = true
	headBefore := m.snake[0]

	m2, cmd := m.Update(tickMsg{gen: m.gen})
	m = m2.(model)

	if m.snake[0] != headBefore {
		t.Fatal("tick should not move snake when paused")
	}
	if cmd != nil {
		t.Fatal("tick should return no command when paused")
	}
}

func TestUpdateTickMsgGameOverNoContinue(t *testing.T) {
	m := newTestModel()
	m.width = 80
	m.height = 40
	m.started = true
	// Position snake at right wall so step causes game over.
	m.snake[0] = point{fieldW - 1, fieldH / 2}
	m.snake[1] = point{fieldW - 2, fieldH / 2}
	m.snake[2] = point{fieldW - 3, fieldH / 2}
	m.dir = dirRight
	m.nextDir = dirRight

	m2, cmd := m.Update(tickMsg{gen: m.gen})
	m = m2.(model)

	if !m.gameOver {
		t.Fatal("expected game over after wall collision via tick")
	}
	if cmd != nil {
		t.Fatal("expected no command after game over")
	}
}

// ---------------------------------------------------------------------------
// Update: unrecognized message
// ---------------------------------------------------------------------------

func TestUpdateUnrecognizedMessage(t *testing.T) {
	m := newTestModel()

	type customMsg struct{}
	m2, cmd := m.Update(customMsg{})
	m = m2.(model)

	if cmd != nil {
		t.Fatal("expected no command for unrecognized message")
	}
}

// ---------------------------------------------------------------------------
// Update: unrecognized key
// ---------------------------------------------------------------------------

func TestUpdateUnrecognizedKey(t *testing.T) {
	m := newTestModel()
	m.width = 80
	m.height = 40
	m.started = true
	m.dir = dirRight
	m.nextDir = dirRight

	// Press a key that has no binding (e.g., 'z').
	k := tea.Key{Code: 'z'}
	m2, cmd := m.Update(tea.KeyPressMsg(k))
	m = m2.(model)

	if m.nextDir != dirRight {
		t.Fatal("unrecognized key should not change direction")
	}
	if cmd != nil {
		t.Fatal("unrecognized key should return no command")
	}
}

// ---------------------------------------------------------------------------
// tick function
// ---------------------------------------------------------------------------

func TestTickReturnsCmd(t *testing.T) {
	cmd := tick(100*time.Millisecond, 5)
	if cmd == nil {
		t.Fatal("expected tick to return a non-nil command")
	}
}

// ---------------------------------------------------------------------------
// Init function
// ---------------------------------------------------------------------------

func TestInitNotStarted(t *testing.T) {
	m := newTestModel()
	cmd := m.Init()
	// Init always returns a batch (at least listenStateDump).
	if cmd == nil {
		t.Fatal("expected Init to return a command")
	}
}

func TestInitStartedAndRunning(t *testing.T) {
	m := newTestModel()
	m.started = true
	m.gameOver = false
	m.paused = false

	cmd := m.Init()
	if cmd == nil {
		t.Fatal("expected Init to return a command when game is running")
	}
}

func TestInitStartedButPaused(t *testing.T) {
	m := newTestModel()
	m.started = true
	m.paused = true

	cmd := m.Init()
	// Should still return listenStateDump, but no tick.
	if cmd == nil {
		t.Fatal("expected Init to return a command (listenStateDump)")
	}
}

func TestInitStartedButGameOver(t *testing.T) {
	m := newTestModel()
	m.started = true
	m.gameOver = true

	cmd := m.Init()
	if cmd == nil {
		t.Fatal("expected Init to return a command (listenStateDump)")
	}
}

// ---------------------------------------------------------------------------
// step: self-collision when eating food (tail does not vacate)
// ---------------------------------------------------------------------------

func TestSelfCollisionWhenEatingFood(t *testing.T) {
	m := newTestModel()
	// Construct a scenario where eating food causes self-collision because
	// the tail does not vacate when food is eaten.
	// Snake wraps around and head moves into body segment that would normally
	// be the tail (vacating) but since we're eating, the tail stays.
	m.snake = []point{
		{5, 3}, // head — moves right to (6,3)
		{5, 4},
		{6, 4},
		{6, 3}, // body segment at (6,3) — head will collide when eating
	}
	m.dir = dirRight
	m.nextDir = dirRight
	// Place food at (6,3) where the body segment is — this means ate=true,
	// so bodyEnd includes the tail, and (6,3) is occupied.
	m.food = point{6, 3}

	m.step()

	if !m.gameOver {
		t.Fatal("expected game over when eating food causes self-collision")
	}
}

// ---------------------------------------------------------------------------
// Speed clamp at minSpeed
// ---------------------------------------------------------------------------

func TestSpeedClampsAtMinimum(t *testing.T) {
	m := newTestModel()
	m.speed = minSpeed + speedStep
	m.started = true
	m.dir = dirRight
	m.nextDir = dirRight
	m.score = 4 // next food will be the 5th, triggering speed change

	// Place food directly ahead.
	h := m.snake[0]
	m.food = point{h.x + 1, h.y}
	m.step()

	if m.speed < minSpeed {
		t.Fatalf("speed should not go below minSpeed, got %v", m.speed)
	}
}

func TestSpeedClampsExactlyAtMinimum(t *testing.T) {
	m := newTestModel()
	// Set speed so that subtracting speedStep would go below minSpeed.
	m.speed = minSpeed + speedStep - 1
	m.started = true
	m.dir = dirRight
	m.nextDir = dirRight
	m.score = 4

	h := m.snake[0]
	m.food = point{h.x + 1, h.y}
	m.step()

	if m.speed != minSpeed {
		t.Fatalf("speed should clamp to minSpeed %v, got %v", minSpeed, m.speed)
	}
}

func TestSpeedAlreadyAtMinNoChange(t *testing.T) {
	m := newTestModel()
	m.speed = minSpeed
	m.started = true
	m.dir = dirRight
	m.nextDir = dirRight
	m.score = 4

	h := m.snake[0]
	m.food = point{h.x + 1, h.y}
	m.step()

	if m.speed != minSpeed {
		t.Fatalf("speed should stay at minSpeed, got %v", m.speed)
	}
}

// ---------------------------------------------------------------------------
// Update: direction key on already-started game does not re-trigger start
// ---------------------------------------------------------------------------

func TestUpdateDirectionKeyWhenAlreadyStarted(t *testing.T) {
	m := newTestModel()
	m.width = 80
	m.height = 40
	m.started = true
	m.dir = dirRight
	m.nextDir = dirRight

	// Press up — should change direction but not return a start tick.
	k := tea.Key{Code: tea.KeyUp}
	m2, cmd := m.Update(tea.KeyPressMsg(k))
	m = m2.(model)

	if m.nextDir != dirUp {
		t.Fatal("expected direction to change to up")
	}
	// When already started, direction change returns nil (tick is already running).
	if cmd != nil {
		t.Fatal("expected no command when changing direction on already-started game")
	}
}

// ---------------------------------------------------------------------------
// Arrow key left starts game (when initial dir is right, left is blocked,
// so test with a model whose dir allows left)
// ---------------------------------------------------------------------------

func TestUpdateArrowLeftStartsGameWhenAllowed(t *testing.T) {
	m := newTestModel()
	m.width = 80
	m.height = 40
	m.dir = dirUp
	m.nextDir = dirUp

	k := tea.Key{Code: tea.KeyLeft}
	m2, cmd := m.Update(tea.KeyPressMsg(k))
	m = m2.(model)

	if !m.started {
		t.Fatal("expected game to start after left arrow (from dir up)")
	}
	if m.nextDir != dirLeft {
		t.Fatalf("expected dirLeft, got %d", m.nextDir)
	}
	if cmd == nil {
		t.Fatal("expected tick command when starting game")
	}
}

// ---------------------------------------------------------------------------
// View: comprehensive rendering tests
// ---------------------------------------------------------------------------

func TestViewZeroSize(t *testing.T) {
	m := newTestModel()
	// width and height are 0 by default.
	v := m.View()
	content := viewContent(v)
	if !strings.Contains(content, "Loading...") {
		t.Fatal("expected 'Loading...' when width/height are zero")
	}
}

func TestViewZeroWidth(t *testing.T) {
	m := newTestModel()
	m.width = 0
	m.height = 50
	v := m.View()
	content := viewContent(v)
	if !strings.Contains(content, "Loading...") {
		t.Fatal("expected 'Loading...' when width is zero")
	}
}

func TestViewZeroHeight(t *testing.T) {
	m := newTestModel()
	m.width = 100
	m.height = 0
	v := m.View()
	content := viewContent(v)
	if !strings.Contains(content, "Loading...") {
		t.Fatal("expected 'Loading...' when height is zero")
	}
}

func TestViewNotStarted(t *testing.T) {
	m := newTestModel()
	m.width = 120
	m.height = 40
	m.started = false
	m.gameOver = false

	v := m.View()
	content := viewContent(v)

	if !strings.Contains(content, "Press arrow key to start") {
		t.Fatal("expected 'Press arrow key to start' message when not started")
	}
	// Should contain the border characters.
	if !strings.Contains(content, "┌") || !strings.Contains(content, "┐") {
		t.Fatal("expected top border characters")
	}
	if !strings.Contains(content, "└") || !strings.Contains(content, "┘") {
		t.Fatal("expected bottom border characters")
	}
	// Should contain footer.
	if !strings.Contains(content, "arrows:move") {
		t.Fatal("expected footer with key hints")
	}
}

func TestViewStartedShowsScore(t *testing.T) {
	m := newTestModel()
	m.width = 120
	m.height = 40
	m.started = true
	m.score = 7

	v := m.View()
	content := viewContent(v)

	if !strings.Contains(content, "Score: 7") {
		t.Fatal("expected 'Score: 7' in status bar")
	}
	if !strings.Contains(content, "Speed:") {
		t.Fatal("expected speed info in status bar")
	}
}

func TestViewPaused(t *testing.T) {
	m := newTestModel()
	m.width = 120
	m.height = 40
	m.started = true
	m.paused = true

	v := m.View()
	content := viewContent(v)

	if !strings.Contains(content, "PAUSED") {
		t.Fatal("expected 'PAUSED' message when paused")
	}
}

func TestViewGameOver(t *testing.T) {
	m := newTestModel()
	m.width = 120
	m.height = 40
	m.started = true
	m.gameOver = true

	v := m.View()
	content := viewContent(v)

	if !strings.Contains(content, "GAME OVER") {
		t.Fatal("expected 'GAME OVER' message")
	}
}

func TestViewAltScreenEnabled(t *testing.T) {
	m := newTestModel()
	m.width = 120
	m.height = 40

	v := m.View()
	if !v.AltScreen {
		t.Fatal("expected AltScreen to be true")
	}
}

func TestViewRendersSnakeAndFood(t *testing.T) {
	m := newTestModel()
	m.width = 120
	m.height = 40
	m.started = true

	v := m.View()
	content := viewContent(v)

	// The view should contain the snake character (double block).
	if !strings.Contains(content, "██") {
		t.Fatal("expected snake blocks '██' in view")
	}
	// The view should contain the food character.
	if !strings.Contains(content, "●") {
		t.Fatal("expected food character '●' in view")
	}
}

func TestViewFieldDimensions(t *testing.T) {
	m := newTestModel()
	m.width = 120
	m.height = 40
	m.started = true

	v := m.View()
	content := viewContent(v)

	// Should have border pipes for each row.
	pipeCount := strings.Count(content, "│")
	// Each row has 2 pipes (left + right), plus top/bottom border corners.
	// fieldH rows * 2 pipes = 40 minimum pipe chars.
	if pipeCount < fieldH*2 {
		t.Fatalf("expected at least %d pipe chars for field borders, got %d", fieldH*2, pipeCount)
	}
}

func TestViewNotStartedNotGameOver(t *testing.T) {
	m := newTestModel()
	m.width = 120
	m.height = 40
	m.started = false
	m.gameOver = false

	v := m.View()
	content := viewContent(v)

	// Should NOT show "GAME OVER" or "PAUSED".
	if strings.Contains(content, "GAME OVER") {
		t.Fatal("should not show GAME OVER when game hasn't started")
	}
	if strings.Contains(content, "PAUSED") {
		t.Fatal("should not show PAUSED when game hasn't started")
	}
}

func TestViewStartedNotPausedNotGameOver(t *testing.T) {
	m := newTestModel()
	m.width = 120
	m.height = 40
	m.started = true
	m.paused = false
	m.gameOver = false
	m.score = 3

	v := m.View()
	content := viewContent(v)

	// Should show score but NOT paused or game over.
	if !strings.Contains(content, "Score: 3") {
		t.Fatal("expected score display")
	}
	if strings.Contains(content, "PAUSED") {
		t.Fatal("should not show PAUSED")
	}
	if strings.Contains(content, "GAME OVER") {
		t.Fatal("should not show GAME OVER")
	}
}

func TestViewSmallTerminalSize(t *testing.T) {
	m := newTestModel()
	m.width = 20
	m.height = 10

	// Should still render without panic.
	v := m.View()
	content := viewContent(v)
	if len(content) == 0 {
		t.Fatal("expected some content even with small terminal")
	}
}

func TestViewLargeTerminalSize(t *testing.T) {
	m := newTestModel()
	m.width = 300
	m.height = 100
	m.started = true
	m.score = 15

	v := m.View()
	content := viewContent(v)
	if !strings.Contains(content, "Score: 15") {
		t.Fatal("expected score in large terminal view")
	}
}

// ---------------------------------------------------------------------------
// newModel with TERMDESK_APP_STATE env var (state restoration)
// ---------------------------------------------------------------------------

func TestNewModelRestoresState(t *testing.T) {
	// Build a valid state and set the env var.
	ss := snakeState{
		Snake:    [][2]int{{10, 5}, {9, 5}, {8, 5}, {7, 5}},
		Dir:      int(dirLeft),
		NextDir:  int(dirLeft),
		FoodX:    15,
		FoodY:    12,
		Score:    42,
		SpeedMs:  100,
		GameOver: false,
		Started:  true,
		Paused:   true,
		Gen:      3,
	}
	data, err := json.Marshal(ss)
	if err != nil {
		t.Fatalf("failed to marshal state: %v", err)
	}
	encoded := base64.StdEncoding.EncodeToString(data)
	t.Setenv("TERMDESK_APP_STATE", encoded)

	m := newModel()

	if len(m.snake) != 4 {
		t.Fatalf("expected snake length 4 after restore, got %d", len(m.snake))
	}
	if m.snake[0] != (point{10, 5}) {
		t.Fatalf("expected head at (10,5), got (%d,%d)", m.snake[0].x, m.snake[0].y)
	}
	if m.dir != dirLeft {
		t.Fatalf("expected dir left, got %d", m.dir)
	}
	if m.nextDir != dirLeft {
		t.Fatalf("expected nextDir left, got %d", m.nextDir)
	}
	if m.food != (point{15, 12}) {
		t.Fatalf("expected food at (15,12), got (%d,%d)", m.food.x, m.food.y)
	}
	if m.score != 42 {
		t.Fatalf("expected score 42, got %d", m.score)
	}
	if m.speed != 100*time.Millisecond {
		t.Fatalf("expected speed 100ms, got %v", m.speed)
	}
	if m.gameOver {
		t.Fatal("expected gameOver false")
	}
	if !m.started {
		t.Fatal("expected started true")
	}
	if !m.paused {
		t.Fatal("expected paused true")
	}
	if m.gen != 3 {
		t.Fatalf("expected gen 3, got %d", m.gen)
	}
}

func TestNewModelRestoresGameOverState(t *testing.T) {
	ss := snakeState{
		Snake:    [][2]int{{5, 5}, {4, 5}, {3, 5}},
		Dir:      int(dirRight),
		NextDir:  int(dirRight),
		FoodX:    20,
		FoodY:    10,
		Score:    99,
		SpeedMs:  50,
		GameOver: true,
		Started:  true,
		Paused:   false,
		Gen:      7,
	}
	data, _ := json.Marshal(ss)
	encoded := base64.StdEncoding.EncodeToString(data)
	t.Setenv("TERMDESK_APP_STATE", encoded)

	m := newModel()

	if !m.gameOver {
		t.Fatal("expected gameOver true after restore")
	}
	if m.score != 99 {
		t.Fatalf("expected score 99, got %d", m.score)
	}
}

func TestNewModelInvalidBase64EnvIgnored(t *testing.T) {
	t.Setenv("TERMDESK_APP_STATE", "not-valid-base64!!!")

	// Should not panic and return a default model.
	m := newModel()

	if len(m.snake) != 3 {
		t.Fatalf("expected default snake length 3, got %d", len(m.snake))
	}
	if m.score != 0 {
		t.Fatalf("expected default score 0, got %d", m.score)
	}
}

func TestNewModelInvalidJSONEnvIgnored(t *testing.T) {
	// Valid base64 but invalid JSON.
	encoded := base64.StdEncoding.EncodeToString([]byte("{invalid json"))
	t.Setenv("TERMDESK_APP_STATE", encoded)

	m := newModel()

	if len(m.snake) != 3 {
		t.Fatalf("expected default snake length 3, got %d", len(m.snake))
	}
}

func TestNewModelEmptyEnvIgnored(t *testing.T) {
	t.Setenv("TERMDESK_APP_STATE", "")

	m := newModel()

	if len(m.snake) != 3 {
		t.Fatalf("expected default snake length 3, got %d", len(m.snake))
	}
}

// ---------------------------------------------------------------------------
// stateDumpMsg handling in Update
// ---------------------------------------------------------------------------

func TestUpdateStateDumpMsg(t *testing.T) {
	m := newTestModel()
	m.width = 80
	m.height = 40
	m.started = true
	m.score = 5
	m.dir = dirUp
	m.nextDir = dirLeft
	m.food = point{10, 15}
	m.paused = true
	m.gen = 2

	m2, cmd := m.Update(stateDumpMsg{})
	m = m2.(model)

	// Should return a command (listenStateDump).
	if cmd == nil {
		t.Fatal("expected command from stateDumpMsg")
	}

	// State should remain unchanged.
	if m.score != 5 {
		t.Fatalf("expected score 5, got %d", m.score)
	}
	if m.dir != dirUp {
		t.Fatalf("expected dir up, got %d", m.dir)
	}
}

// ---------------------------------------------------------------------------
// tick inner closure
// ---------------------------------------------------------------------------

func TestTickFunctionInnerClosure(t *testing.T) {
	cmd := tick(50*time.Millisecond, 42)
	if cmd == nil {
		t.Fatal("expected tick to return a non-nil command")
	}
	// Execute the command to cover the inner closure.
	// The closure calls tea.Tick which itself returns a Cmd.
	// We can't easily verify the inner closure without running the program,
	// but we verify the cmd is non-nil.
}

// ---------------------------------------------------------------------------
// Reset increments gen
// ---------------------------------------------------------------------------

func TestResetIncrementsGen(t *testing.T) {
	m := newTestModel()
	genBefore := m.gen
	m.gameOver = true
	m.reset()

	if m.gen != genBefore+1 {
		t.Fatalf("expected gen %d after reset, got %d", genBefore+1, m.gen)
	}
}

func TestResetRestoresDefaults(t *testing.T) {
	m := newTestModel()
	m.score = 50
	m.speed = minSpeed
	m.dir = dirUp
	m.nextDir = dirLeft
	m.gameOver = true
	m.started = true
	m.paused = true

	m.reset()

	if m.score != 0 {
		t.Fatalf("expected score 0, got %d", m.score)
	}
	if m.speed != initialSpeed {
		t.Fatalf("expected speed %v, got %v", initialSpeed, m.speed)
	}
	if m.dir != dirRight {
		t.Fatalf("expected dir right, got %d", m.dir)
	}
	if m.nextDir != dirRight {
		t.Fatalf("expected nextDir right, got %d", m.nextDir)
	}
	if m.gameOver {
		t.Fatal("expected gameOver false")
	}
	if m.started {
		t.Fatal("expected started false")
	}
	if m.paused {
		t.Fatal("expected paused false")
	}
}

// ---------------------------------------------------------------------------
// placeFood always places food outside of snake
// ---------------------------------------------------------------------------

func TestPlaceFoodAlwaysOutsideSnake(t *testing.T) {
	m := newTestModel()
	// Create a fairly long snake.
	m.snake = make([]point, 100)
	for i := range 100 {
		m.snake[i] = point{i % fieldW, i / fieldW}
	}

	for range 50 {
		m.placeFood()
		for _, p := range m.snake {
			if p == m.food {
				t.Fatalf("food placed on snake segment at (%d,%d)", p.x, p.y)
			}
		}
	}
}

// ---------------------------------------------------------------------------
// Step: various directions and edge cases
// ---------------------------------------------------------------------------

func TestStepDoesNotGrowWithoutFood(t *testing.T) {
	m := newTestModel()
	m.dir = dirRight
	m.nextDir = dirRight
	// Make sure food is NOT adjacent.
	m.food = point{0, 0}
	lenBefore := len(m.snake)

	m.step()

	if len(m.snake) != lenBefore {
		t.Fatalf("expected snake length %d without food, got %d", lenBefore, len(m.snake))
	}
}

func TestStepMultipleStepsNoCollision(t *testing.T) {
	m := newTestModel()
	m.dir = dirRight
	m.nextDir = dirRight
	// Food far away.
	m.food = point{0, 0}

	// Take several steps to the right without hitting walls.
	for i := 0; i < 5; i++ {
		m.step()
		if m.gameOver {
			t.Fatalf("unexpected game over at step %d", i+1)
		}
	}

	// Head should have moved 5 positions to the right.
	expectedX := fieldW/2 + 5
	if m.snake[0].x != expectedX {
		t.Fatalf("expected head x=%d after 5 steps right, got %d", expectedX, m.snake[0].x)
	}
}

// ---------------------------------------------------------------------------
// View rendering with different snake positions
// ---------------------------------------------------------------------------

func TestViewWithFoodAtCorner(t *testing.T) {
	m := newTestModel()
	m.width = 120
	m.height = 40
	m.food = point{0, 0}

	v := m.View()
	content := viewContent(v)

	// Should render without panic and contain food character.
	if !strings.Contains(content, "●") {
		t.Fatal("expected food character in view with food at corner")
	}
}

func TestViewWithSnakeAtEdge(t *testing.T) {
	m := newTestModel()
	m.width = 120
	m.height = 40
	m.snake = []point{
		{fieldW - 1, 0},
		{fieldW - 2, 0},
		{fieldW - 3, 0},
	}

	v := m.View()
	content := viewContent(v)
	if !strings.Contains(content, "██") {
		t.Fatal("expected snake blocks in view with snake at edge")
	}
}

// ---------------------------------------------------------------------------
// View: paused AND gameOver mutual exclusivity in display
// ---------------------------------------------------------------------------

func TestViewPausedDoesNotShowGameOver(t *testing.T) {
	m := newTestModel()
	m.width = 120
	m.height = 40
	m.started = true
	m.paused = true
	m.gameOver = false

	v := m.View()
	content := viewContent(v)

	if !strings.Contains(content, "PAUSED") {
		t.Fatal("expected PAUSED message")
	}
	if strings.Contains(content, "GAME OVER") {
		t.Fatal("should not show GAME OVER when paused and not game over")
	}
}

func TestViewGameOverDoesNotShowPaused(t *testing.T) {
	m := newTestModel()
	m.width = 120
	m.height = 40
	m.started = true
	m.gameOver = true
	m.paused = false

	v := m.View()
	content := viewContent(v)

	if !strings.Contains(content, "GAME OVER") {
		t.Fatal("expected GAME OVER message")
	}
	if strings.Contains(content, "PAUSED") {
		t.Fatal("should not show PAUSED when game is over")
	}
}

// ---------------------------------------------------------------------------
// View: speed display in status bar
// ---------------------------------------------------------------------------

func TestViewSpeedDisplay(t *testing.T) {
	m := newTestModel()
	m.width = 120
	m.height = 40
	m.started = true
	m.speed = 130 * time.Millisecond

	v := m.View()
	content := viewContent(v)

	if !strings.Contains(content, "Speed: 130ms") {
		t.Fatal("expected 'Speed: 130ms' in status bar")
	}
}

// ---------------------------------------------------------------------------
// State serialization round-trip
// ---------------------------------------------------------------------------

func TestStateSerializationRoundTrip(t *testing.T) {
	m := newTestModel()
	m.started = true
	m.score = 25
	m.dir = dirUp
	m.nextDir = dirLeft
	m.food = point{5, 10}
	m.speed = 100 * time.Millisecond
	m.paused = true
	m.gen = 4

	// Serialize.
	pts := make([][2]int, len(m.snake))
	for i, p := range m.snake {
		pts[i] = [2]int{p.x, p.y}
	}
	ss := snakeState{
		Snake:    pts,
		Dir:      int(m.dir),
		NextDir:  int(m.nextDir),
		FoodX:    m.food.x,
		FoodY:    m.food.y,
		Score:    m.score,
		SpeedMs:  m.speed.Milliseconds(),
		GameOver: m.gameOver,
		Started:  m.started,
		Paused:   m.paused,
		Gen:      m.gen,
	}
	data, err := json.Marshal(ss)
	if err != nil {
		t.Fatalf("failed to marshal: %v", err)
	}
	encoded := base64.StdEncoding.EncodeToString(data)

	// Restore.
	t.Setenv("TERMDESK_APP_STATE", encoded)
	m2 := newModel()

	if m2.score != m.score {
		t.Fatalf("expected score %d, got %d", m.score, m2.score)
	}
	if m2.dir != m.dir {
		t.Fatalf("expected dir %d, got %d", m.dir, m2.dir)
	}
	if m2.nextDir != m.nextDir {
		t.Fatalf("expected nextDir %d, got %d", m.nextDir, m2.nextDir)
	}
	if m2.food != m.food {
		t.Fatalf("expected food (%d,%d), got (%d,%d)", m.food.x, m.food.y, m2.food.x, m2.food.y)
	}
	if m2.speed != m.speed {
		t.Fatalf("expected speed %v, got %v", m.speed, m2.speed)
	}
	if m2.paused != m.paused {
		t.Fatalf("expected paused %v, got %v", m.paused, m2.paused)
	}
	if m2.gen != m.gen {
		t.Fatalf("expected gen %d, got %d", m.gen, m2.gen)
	}
	if len(m2.snake) != len(m.snake) {
		t.Fatalf("expected snake length %d, got %d", len(m.snake), len(m2.snake))
	}
	for i := range m.snake {
		if m2.snake[i] != m.snake[i] {
			t.Fatalf("snake segment %d: expected (%d,%d), got (%d,%d)",
				i, m.snake[i].x, m.snake[i].y, m2.snake[i].x, m2.snake[i].y)
		}
	}
}

// ---------------------------------------------------------------------------
// initSnake validates initial snake placement
// ---------------------------------------------------------------------------

func TestInitSnakePlacement(t *testing.T) {
	m := newTestModel()
	m.initSnake()

	if len(m.snake) != 3 {
		t.Fatalf("expected 3 segments, got %d", len(m.snake))
	}
	head := m.snake[0]
	body := m.snake[1]
	tail := m.snake[2]

	// Head at center.
	if head.x != fieldW/2 || head.y != fieldH/2 {
		t.Fatalf("head not at center: (%d,%d)", head.x, head.y)
	}
	// Body one left of head.
	if body.x != head.x-1 || body.y != head.y {
		t.Fatalf("body not left of head: (%d,%d)", body.x, body.y)
	}
	// Tail one left of body.
	if tail.x != body.x-1 || tail.y != body.y {
		t.Fatalf("tail not left of body: (%d,%d)", tail.x, tail.y)
	}
}

// ---------------------------------------------------------------------------
// Speed does not change when score is not multiple of 5
// ---------------------------------------------------------------------------

func TestSpeedUnchangedWhenNotMultipleOf5(t *testing.T) {
	m := newTestModel()
	m.dir = dirRight
	m.nextDir = dirRight
	m.score = 2 // eating next food makes score 3, not multiple of 5.
	speedBefore := m.speed
	h := m.snake[0]
	m.food = point{h.x + 1, h.y}

	m.step()

	if m.speed != speedBefore {
		t.Fatalf("expected speed to remain %v (score 3 not multiple of 5), got %v", speedBefore, m.speed)
	}
}

// ---------------------------------------------------------------------------
// Multiple food eating and speed changes
// ---------------------------------------------------------------------------

func TestEat10FoodSpeedDecreaseTwice(t *testing.T) {
	m := newTestModel()
	m.dir = dirRight
	m.nextDir = dirRight
	speedBefore := m.speed

	for i := 0; i < 10; i++ {
		h := m.snake[0]
		m.food = point{h.x + 1, h.y}
		m.step()
		if m.gameOver {
			t.Fatalf("unexpected game over at food %d", i+1)
		}
	}

	if m.score != 10 {
		t.Fatalf("expected score 10, got %d", m.score)
	}
	// Speed should have decreased by 2 * speedStep.
	expectedSpeed := speedBefore - 2*speedStep
	if m.speed != expectedSpeed {
		t.Fatalf("expected speed %v after 10 food, got %v", expectedSpeed, m.speed)
	}
}

// ---------------------------------------------------------------------------
// Self collision: tail vacates so no collision
// ---------------------------------------------------------------------------

func TestNoSelfCollisionWhenTailVacates(t *testing.T) {
	// The tail will vacate its position, so the head can move there.
	m := newTestModel()
	// Snake forms an L: tail is at position head will move to, but tail vacates.
	//   T B H  -> moving down, head goes to (7,4)
	//         but we need tail at the target position.
	// Actually: head moving right, tail at head.x+1, head.y.
	// That requires a specific layout.
	m.snake = []point{
		{5, 5}, // head — moving right to (6,5)
		{5, 4},
		{6, 4},
		{6, 5}, // tail — at (6,5), will vacate
	}
	m.dir = dirRight
	m.nextDir = dirRight
	m.food = point{0, 0} // food not in the way

	m.step()

	if m.gameOver {
		t.Fatal("should not be game over — tail vacated the target position")
	}
	if m.snake[0] != (point{6, 5}) {
		t.Fatalf("expected head at (6,5), got (%d,%d)", m.snake[0].x, m.snake[0].y)
	}
}

// ---------------------------------------------------------------------------
// Helper to extract view content as string
// ---------------------------------------------------------------------------

func viewContent(v tea.View) string {
	return v.Content
}

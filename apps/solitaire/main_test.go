package main

import (
	"encoding/base64"
	"encoding/json"
	"testing"

	tea "charm.land/bubbletea/v2"
)

func viewContent(v tea.View) string {
	return v.Content
}

// ── Deal tests ──────────────────────────────────────────────────────

func TestDeal52Cards(t *testing.T) {
	m := NewTestModel(42)
	total := len(m.stock) + len(m.waste)
	for i := 0; i < 4; i++ {
		total += len(m.foundations[i])
	}
	for i := 0; i < 7; i++ {
		total += len(m.tableau[i])
	}
	if total != 52 {
		t.Fatalf("expected 52 cards, got %d", total)
	}
}

func TestDealTableauLayout(t *testing.T) {
	m := NewTestModel(42)
	// Stock should have 24 cards.
	if len(m.stock) != 24 {
		t.Errorf("stock: expected 24, got %d", len(m.stock))
	}
	// Tableau column i should have i+1 cards.
	for i := 0; i < 7; i++ {
		if len(m.tableau[i]) != i+1 {
			t.Errorf("tableau[%d]: expected %d cards, got %d", i, i+1, len(m.tableau[i]))
		}
		// Only the last card should be face up.
		for j, c := range m.tableau[i] {
			if j == i {
				if !c.faceUp {
					t.Errorf("tableau[%d][%d] should be face up", i, j)
				}
			} else {
				if c.faceUp {
					t.Errorf("tableau[%d][%d] should be face down", i, j)
				}
			}
		}
	}
}

func TestDealNoFoundations(t *testing.T) {
	m := NewTestModel(42)
	for i := 0; i < 4; i++ {
		if len(m.foundations[i]) != 0 {
			t.Errorf("foundation[%d] should be empty, has %d", i, len(m.foundations[i]))
		}
	}
}

// ── Move validation tests ───────────────────────────────────────────

func TestCanMoveToFoundationAceOnEmpty(t *testing.T) {
	m := NewTestModel(42)
	ace := card{rank: 1, suit: suitSpades, faceUp: true}
	if !m.canMoveToFoundation(ace, 0) {
		t.Error("ace should be placeable on empty foundation")
	}
}

func TestCanMoveToFoundationNonAceOnEmpty(t *testing.T) {
	m := NewTestModel(42)
	two := card{rank: 2, suit: suitSpades, faceUp: true}
	if m.canMoveToFoundation(two, 0) {
		t.Error("non-ace should not be placeable on empty foundation")
	}
}

func TestCanMoveToFoundationSequential(t *testing.T) {
	m := NewTestModel(42)
	m.foundations[0] = []card{{rank: 1, suit: suitHearts, faceUp: true}}
	two := card{rank: 2, suit: suitHearts, faceUp: true}
	if !m.canMoveToFoundation(two, 0) {
		t.Error("2♥ should be placeable on A♥")
	}
}

func TestCanMoveToFoundationWrongSuit(t *testing.T) {
	m := NewTestModel(42)
	m.foundations[0] = []card{{rank: 1, suit: suitHearts, faceUp: true}}
	two := card{rank: 2, suit: suitSpades, faceUp: true}
	if m.canMoveToFoundation(two, 0) {
		t.Error("2♠ should not be placeable on A♥")
	}
}

func TestCanMoveToTableauKingOnEmpty(t *testing.T) {
	m := NewTestModel(42)
	m.tableau[0] = nil
	king := card{rank: 13, suit: suitSpades, faceUp: true}
	if !m.canMoveToTableau(king, 0) {
		t.Error("king should be placeable on empty tableau")
	}
}

func TestCanMoveToTableauNonKingOnEmpty(t *testing.T) {
	m := NewTestModel(42)
	m.tableau[0] = nil
	queen := card{rank: 12, suit: suitSpades, faceUp: true}
	if m.canMoveToTableau(queen, 0) {
		t.Error("non-king should not be placeable on empty tableau")
	}
}

func TestCanMoveToTableauOppositeColor(t *testing.T) {
	m := NewTestModel(42)
	m.tableau[0] = []card{{rank: 5, suit: suitSpades, faceUp: true}}
	four := card{rank: 4, suit: suitHearts, faceUp: true}
	if !m.canMoveToTableau(four, 0) {
		t.Error("4♥ should be placeable on 5♠")
	}
}

func TestCanMoveToTableauSameColor(t *testing.T) {
	m := NewTestModel(42)
	m.tableau[0] = []card{{rank: 5, suit: suitSpades, faceUp: true}}
	four := card{rank: 4, suit: suitClubs, faceUp: true}
	if m.canMoveToTableau(four, 0) {
		t.Error("4♣ should not be placeable on 5♠ (same color)")
	}
}

// ── Draw tests ──────────────────────────────────────────────────────

func TestDrawFromStock(t *testing.T) {
	m := NewTestModel(42)
	stockLen := len(m.stock)
	m.drawFromStock()
	if len(m.stock) != stockLen-1 {
		t.Errorf("stock should decrease by 1, got %d", len(m.stock))
	}
	if len(m.waste) != 1 {
		t.Errorf("waste should have 1 card, got %d", len(m.waste))
	}
	if !m.waste[0].faceUp {
		t.Error("waste card should be face up")
	}
}

func TestDrawThree(t *testing.T) {
	m := NewTestModel(42)
	m.drawMode = drawThree
	stockLen := len(m.stock)
	m.drawFromStock()
	if len(m.stock) != stockLen-3 {
		t.Errorf("stock should decrease by 3, got %d", len(m.stock))
	}
	if len(m.waste) != 3 {
		t.Errorf("waste should have 3 cards, got %d", len(m.waste))
	}
}

func TestRecycleWaste(t *testing.T) {
	m := NewTestModel(42)
	// Draw all cards.
	for len(m.stock) > 0 {
		m.drawFromStock()
	}
	wasteLen := len(m.waste)
	// Recycle.
	m.drawFromStock()
	if len(m.stock) != wasteLen {
		t.Errorf("after recycle, stock should have %d, got %d", wasteLen, len(m.stock))
	}
	if len(m.waste) != 0 {
		t.Errorf("after recycle, waste should be empty, got %d", len(m.waste))
	}
	// All recycled cards should be face down.
	for i, c := range m.stock {
		if c.faceUp {
			t.Errorf("recycled stock[%d] should be face down", i)
		}
	}
}

// ── Scoring tests ───────────────────────────────────────────────────

func TestScoreWasteToTableau(t *testing.T) {
	m := NewTestModel(42)
	m.waste = []card{{rank: 4, suit: suitHearts, faceUp: true}}
	m.tableau[0] = []card{{rank: 5, suit: suitSpades, faceUp: true}}
	m.moveWasteToTableau(0)
	if m.score != scoreWasteToTab {
		t.Errorf("expected score %d, got %d", scoreWasteToTab, m.score)
	}
}

func TestScoreWasteToFoundation(t *testing.T) {
	m := NewTestModel(42)
	m.waste = []card{{rank: 1, suit: suitHearts, faceUp: true}}
	m.moveWasteToFoundation(0)
	if m.score != scoreWasteToFound {
		t.Errorf("expected score %d, got %d", scoreWasteToFound, m.score)
	}
}

func TestScoreTabToFoundation(t *testing.T) {
	m := NewTestModel(42)
	m.tableau[0] = []card{{rank: 1, suit: suitHearts, faceUp: true}}
	m.moveTableauToFoundation(0, 0)
	if m.score != scoreTabToFound {
		t.Errorf("expected score %d, got %d", scoreTabToFound, m.score)
	}
}

func TestScoreFoundToTab(t *testing.T) {
	m := NewTestModel(42)
	m.foundations[0] = []card{{rank: 1, suit: suitHearts, faceUp: true}}
	m.tableau[0] = []card{{rank: 2, suit: suitSpades, faceUp: true}}
	m.moveFoundationToTableau(0, 0)
	// Foundation to tab is -15 but clamped at 0.
	if m.score != 0 {
		t.Errorf("score should be clamped at 0, got %d", m.score)
	}
}

func TestScoreFloorAtZero(t *testing.T) {
	m := NewTestModel(42)
	m.addScore(-100)
	if m.score != 0 {
		t.Errorf("score should floor at 0, got %d", m.score)
	}
}

// ── Move execution tests ────────────────────────────────────────────

func TestMoveTableauToTableau(t *testing.T) {
	m := NewTestModel(42)
	m.tableau[0] = []card{{rank: 5, suit: suitSpades, faceUp: true}}
	m.tableau[1] = []card{{rank: 4, suit: suitHearts, faceUp: true}}
	ok := m.moveTableauToTableau(1, 0, 0)
	if !ok {
		t.Error("move should succeed")
	}
	if len(m.tableau[0]) != 2 {
		t.Errorf("dst should have 2 cards, got %d", len(m.tableau[0]))
	}
	if len(m.tableau[1]) != 0 {
		t.Errorf("src should be empty, got %d", len(m.tableau[1]))
	}
}

func TestMoveMultiCardRun(t *testing.T) {
	m := NewTestModel(42)
	m.tableau[0] = []card{
		{rank: 13, suit: suitSpades, faceUp: true},
	}
	m.tableau[1] = []card{
		{rank: 6, suit: suitClubs, faceUp: false},
		{rank: 5, suit: suitHearts, faceUp: true},
		{rank: 4, suit: suitSpades, faceUp: true},
	}
	// Move 5♥ + 4♠ run to the K♠.
	// First need a valid target. K doesn't accept 5. Let's set up properly.
	m.tableau[0] = []card{{rank: 6, suit: suitClubs, faceUp: true}}
	ok := m.moveTableauToTableau(1, 1, 0)
	if !ok {
		t.Error("multi-card run move should succeed")
	}
	if len(m.tableau[0]) != 3 {
		t.Errorf("dst should have 3 cards, got %d", len(m.tableau[0]))
	}
	// Source should flip the hidden card.
	if len(m.tableau[1]) != 1 {
		t.Fatalf("src should have 1 card, got %d", len(m.tableau[1]))
	}
	if !m.tableau[1][0].faceUp {
		t.Error("hidden card should flip face up")
	}
}

func TestFlipTopCard(t *testing.T) {
	m := NewTestModel(42)
	m.score = 0
	m.tableau[0] = []card{
		{rank: 3, suit: suitClubs, faceUp: false},
		{rank: 7, suit: suitHearts, faceUp: true},
	}
	// Move 7♥ to a valid destination.
	m.tableau[1] = []card{{rank: 8, suit: suitSpades, faceUp: true}}
	m.moveTableauToTableau(0, 1, 1)
	// The face-down 3♣ should be flipped.
	if !m.tableau[0][0].faceUp {
		t.Error("top card should be flipped face up")
	}
	if m.score != scoreTurnCard {
		t.Errorf("should earn %d for turning card, got %d", scoreTurnCard, m.score)
	}
}

// ── Auto-complete tests ─────────────────────────────────────────────

func TestCanAutoComplete(t *testing.T) {
	m := NewTestModel(42)
	if m.canAutoComplete() {
		t.Error("should not be able to auto-complete at start")
	}
}

func TestAutoCompleteAllFaceUp(t *testing.T) {
	m := NewTestModel(42)
	m.stock = nil
	m.waste = nil
	// Set up tableau with all face-up cards.
	for col := 0; col < 7; col++ {
		for i := range m.tableau[col] {
			m.tableau[col][i].faceUp = true
		}
	}
	if !m.canAutoComplete() {
		t.Error("should be able to auto-complete when all face-up and no stock")
	}
}

func TestBuildAutoCompleteSequence(t *testing.T) {
	m := NewTestModel(42)
	m.stock = nil
	m.waste = nil
	m.foundations = [4][]card{}
	// Simple setup: 4 aces in tableau.
	m.tableau = [7][]card{
		{{rank: 1, suit: suitClubs, faceUp: true}},
		{{rank: 1, suit: suitDiamonds, faceUp: true}},
		{{rank: 1, suit: suitHearts, faceUp: true}},
		{{rank: 1, suit: suitSpades, faceUp: true}},
		nil, nil, nil,
	}
	seq := m.buildAutoCompleteSequence()
	if len(seq) != 4 {
		t.Errorf("expected 4 auto-complete moves, got %d", len(seq))
	}
}

// ── Win detection ───────────────────────────────────────────────────

func TestCheckWin(t *testing.T) {
	m := NewTestModel(42)
	if m.checkWin() {
		t.Error("should not be won at start")
	}
}

func TestCheckWinAllFoundations(t *testing.T) {
	m := NewTestModel(42)
	m.stock = nil
	m.waste = nil
	m.tableau = [7][]card{}
	for s := 0; s < 4; s++ {
		m.foundations[s] = make([]card, 13)
		for r := 1; r <= 13; r++ {
			m.foundations[s][r-1] = card{rank: r, suit: suit(s), faceUp: true}
		}
	}
	if !m.checkWin() {
		t.Error("should detect win when all foundations full")
	}
}

// ── Hit testing ─────────────────────────────────────────────────────

func TestHitTestStock(t *testing.T) {
	m := NewTestModel(42)
	m.width = 80
	m.height = 50
	ox, oy := m.contentOrigin()
	hit := m.hitTest(ox+3, oy+topRowStart+2)
	if !hit.valid || hit.pile.ptype != pileStock {
		t.Errorf("expected stock hit, got valid=%v pile=%v", hit.valid, hit.pile)
	}
}

func TestHitTestWaste(t *testing.T) {
	m := NewTestModel(42)
	m.width = 80
	m.height = 50
	m.drawFromStock()
	ox, oy := m.contentOrigin()
	hit := m.hitTest(ox+colStride+3, oy+topRowStart+2)
	if !hit.valid || hit.pile.ptype != pileWaste {
		t.Errorf("expected waste hit, got valid=%v pile=%v", hit.valid, hit.pile)
	}
}

func TestHitTestFoundation(t *testing.T) {
	m := NewTestModel(42)
	m.width = 80
	m.height = 50
	ox, oy := m.contentOrigin()
	// Foundation 0 is at column 3.
	hit := m.hitTest(ox+3*colStride+3, oy+topRowStart+2)
	if !hit.valid || hit.pile.ptype != pileFoundation || hit.pile.index != 0 {
		t.Errorf("expected foundation 0, got valid=%v pile=%v", hit.valid, hit.pile)
	}
}

func TestHitTestTableau(t *testing.T) {
	m := NewTestModel(42)
	m.width = 80
	m.height = 50
	ox, oy := m.contentOrigin()
	// Tableau col 0 has 1 face-up card at row 0.
	hit := m.hitTest(ox+3, oy+tabStart+2)
	if !hit.valid || hit.pile.ptype != pileTableau || hit.pile.index != 0 {
		t.Errorf("expected tableau 0, got valid=%v pile=%v", hit.valid, hit.pile)
	}
	if hit.cardIdx != 0 {
		t.Errorf("expected card 0, got %d", hit.cardIdx)
	}
}

func TestHitTestGap(t *testing.T) {
	m := NewTestModel(42)
	m.width = 80
	m.height = 50
	ox, oy := m.contentOrigin()
	// Col 2 is the gap.
	hit := m.hitTest(ox+2*colStride+3, oy+topRowStart+2)
	if hit.valid {
		t.Error("gap column should not be a valid hit")
	}
}

func TestHitTestOutOfBounds(t *testing.T) {
	m := NewTestModel(42)
	m.width = 80
	m.height = 50
	hit := m.hitTest(0, 0)
	if hit.valid {
		t.Error("out of bounds should not be valid")
	}
}

// ── Keyboard navigation tests ───────────────────────────────────────

func TestTabCycling(t *testing.T) {
	m := NewTestModel(42)
	// Start at stock.
	if m.cursor.ptype != pileStock {
		t.Fatalf("expected stock cursor, got %v", m.cursor)
	}
	m.tabNext() // waste
	if m.cursor.ptype != pileWaste {
		t.Errorf("expected waste, got %v", m.cursor)
	}
	m.tabNext() // found0
	if m.cursor.ptype != pileFoundation || m.cursor.index != 0 {
		t.Errorf("expected foundation 0, got %v", m.cursor)
	}
	// Skip to tableau.
	m.tabNext() // found1
	m.tabNext() // found2
	m.tabNext() // found3
	m.tabNext() // tab0
	if m.cursor.ptype != pileTableau || m.cursor.index != 0 {
		t.Errorf("expected tableau 0, got %v", m.cursor)
	}
	// Cycle through all tableau.
	for i := 1; i <= 6; i++ {
		m.tabNext()
	}
	// Should wrap to stock.
	m.tabNext()
	if m.cursor.ptype != pileStock {
		t.Errorf("expected wrap to stock, got %v", m.cursor)
	}
}

func TestTabPrev(t *testing.T) {
	m := NewTestModel(42)
	m.tabPrev() // should go to tab6
	if m.cursor.ptype != pileTableau || m.cursor.index != 6 {
		t.Errorf("expected tableau 6, got %v", m.cursor)
	}
}

func TestCursorLeftRight(t *testing.T) {
	m := NewTestModel(42)
	m.cursor = pileID{ptype: pileTableau, index: 3}
	m.cursorLeft()
	if m.cursor.index != 2 {
		t.Errorf("expected tab 2, got %d", m.cursor.index)
	}
	m.cursorRight()
	if m.cursor.index != 3 {
		t.Errorf("expected tab 3, got %d", m.cursor.index)
	}
}

func TestCursorUpFromTableau(t *testing.T) {
	m := NewTestModel(42)
	m.cursor = pileID{ptype: pileTableau, index: 0}
	m.cursorCard = 0
	m.cursorUp() // should go to top row (stock).
	if m.cursor.ptype != pileStock {
		t.Errorf("expected stock, got %v", m.cursor)
	}
}

func TestCursorDownFromStock(t *testing.T) {
	m := NewTestModel(42)
	m.cursorDown()
	if m.cursor.ptype != pileTableau || m.cursor.index != 0 {
		t.Errorf("expected tableau 0, got %v", m.cursor)
	}
}

// ── Selection and placement tests ───────────────────────────────────

func TestSelectAndPlace(t *testing.T) {
	m := NewTestModel(42)
	m.tableau[0] = []card{{rank: 5, suit: suitSpades, faceUp: true}}
	m.tableau[1] = []card{{rank: 4, suit: suitHearts, faceUp: true}}

	m.selectCard(pileID{ptype: pileTableau, index: 1}, 0)
	if m.selected == nil {
		t.Fatal("expected selection")
	}

	ok := m.tryPlace(pileID{ptype: pileTableau, index: 0})
	if !ok {
		t.Error("placement should succeed")
	}
	if len(m.tableau[0]) != 2 {
		t.Errorf("expected 2 cards in dst, got %d", len(m.tableau[0]))
	}
}

func TestAutoSendToFoundation(t *testing.T) {
	m := NewTestModel(42)
	m.tableau[0] = []card{{rank: 1, suit: suitHearts, faceUp: true}}
	ok := m.autoSendToFoundation(pileID{ptype: pileTableau, index: 0}, 0)
	if !ok {
		t.Error("auto-send ace to foundation should succeed")
	}
	foundAce := false
	for i := 0; i < 4; i++ {
		if len(m.foundations[i]) > 0 && m.foundations[i][0].rank == 1 && m.foundations[i][0].suit == suitHearts {
			foundAce = true
		}
	}
	if !foundAce {
		t.Error("ace should be in a foundation")
	}
}

// ── Draw mode toggle ────────────────────────────────────────────────

func TestDrawModeToggle(t *testing.T) {
	m := NewTestModel(42)
	if m.drawMode != drawOne {
		t.Error("default should be draw 1")
	}
	m.drawMode = drawThree
	if m.drawMode != drawThree {
		t.Error("should be draw 3")
	}
}

// ── State persistence tests ─────────────────────────────────────────

func TestStatePersistenceRoundtrip(t *testing.T) {
	m := NewTestModel(42)
	m.drawFromStock()
	m.drawFromStock()
	m.score = 100
	m.moves = 5
	m.elapsed = 42

	// Serialize.
	js := solitaireJSON{
		Stock:   cardsToJSON(m.stock),
		Waste:   cardsToJSON(m.waste),
		Draw:    int(m.drawMode),
		Score:   m.score,
		Moves:   m.moves,
		Elapsed: m.elapsed,
		State:   int(m.state),
		Started: m.started,
	}
	for i := 0; i < 4; i++ {
		js.Found[i] = cardsToJSON(m.foundations[i])
	}
	for i := 0; i < 7; i++ {
		js.Tab[i] = cardsToJSON(m.tableau[i])
	}
	data, err := json.Marshal(js)
	if err != nil {
		t.Fatal(err)
	}

	// Deserialize into fresh model.
	m2 := NewTestModel(99)
	m2.restoreState(data)

	if m2.score != 100 {
		t.Errorf("score: expected 100, got %d", m2.score)
	}
	if m2.moves != 5 {
		t.Errorf("moves: expected 5, got %d", m2.moves)
	}
	if len(m2.stock) != len(m.stock) {
		t.Errorf("stock len: expected %d, got %d", len(m.stock), len(m2.stock))
	}
	if len(m2.waste) != len(m.waste) {
		t.Errorf("waste len: expected %d, got %d", len(m.waste), len(m2.waste))
	}
}

func TestRestoreInvalidJSON(t *testing.T) {
	m := NewTestModel(42)
	m.restoreState([]byte("invalid json"))
	// Should not panic, just ignore.
	if m.GetScore() != 0 {
		t.Error("invalid restore should not change score")
	}
}

func TestRestoreBase64(t *testing.T) {
	m := NewTestModel(42)
	js := solitaireJSON{Score: 999}
	data, _ := json.Marshal(js)
	encoded := base64.StdEncoding.EncodeToString(data)
	decoded, _ := base64.StdEncoding.DecodeString(encoded)
	m.restoreState(decoded)
	if m.score != 999 {
		t.Errorf("expected score 999 after base64 roundtrip, got %d", m.score)
	}
}

// ── View tests ──────────────────────────────────────────────────────

func TestViewZeroSize(t *testing.T) {
	m := NewTestModel(42)
	v := m.View()
	if !v.AltScreen {
		t.Error("AltScreen should be true")
	}
	s := viewContent(v)
	if s != "Loading..." {
		t.Errorf("zero size should show loading, got %q", s)
	}
}

func TestViewNormalSize(t *testing.T) {
	m := NewTestModel(42)
	m.width = 80
	m.height = 50
	v := m.View()
	s := viewContent(v)
	if len(s) == 0 {
		t.Error("view should not be empty")
	}
	if v.MouseMode != tea.MouseModeCellMotion {
		t.Error("mouse mode should be cell motion")
	}
}

func TestViewWon(t *testing.T) {
	m := NewTestModel(42)
	m.width = 80
	m.height = 50
	m.state = stateWon
	m.score = 500
	v := m.View()
	s := viewContent(v)
	if !containsStr(s, "WIN") {
		t.Error("won view should contain WIN")
	}
}

func TestViewAutoComplete(t *testing.T) {
	m := NewTestModel(42)
	m.width = 80
	m.height = 50
	m.state = stateAutoComplete
	v := m.View()
	s := viewContent(v)
	if !containsStr(s, "Auto") {
		t.Error("auto-complete view should contain Auto")
	}
}

func containsStr(s, sub string) bool {
	return len(s) >= len(sub) && searchString(s, sub)
}

func searchString(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}

// ── Update message tests ────────────────────────────────────────────

func TestUpdateQuit(t *testing.T) {
	m := NewTestModel(42)
	m.width = 80
	m.height = 50
	_, cmd := m.Update(tea.KeyPressMsg(tea.Key{Code: 'q'}))
	if cmd == nil {
		t.Error("q should produce quit cmd")
	}
}

func TestUpdateRestart(t *testing.T) {
	m := NewTestModel(42)
	m.width = 80
	m.height = 50
	m.score = 100
	m2, _ := m.Update(tea.KeyPressMsg(tea.Key{Code: 'r'}))
	nm := m2.(model)
	if nm.score != 0 {
		t.Errorf("reset should clear score, got %d", nm.score)
	}
}

func TestUpdateDraw(t *testing.T) {
	m := NewTestModel(42)
	m.width = 80
	m.height = 50
	stockLen := len(m.stock)
	m2, _ := m.Update(tea.KeyPressMsg(tea.Key{Code: 'd'}))
	nm := m2.(model)
	if len(nm.stock) != stockLen-1 {
		t.Errorf("d key should draw, stock: %d -> %d", stockLen, len(nm.stock))
	}
}

func TestUpdateWindowSize(t *testing.T) {
	m := NewTestModel(42)
	m2, _ := m.Update(tea.WindowSizeMsg{Width: 100, Height: 60})
	nm := m2.(model)
	if nm.width != 100 || nm.height != 60 {
		t.Errorf("expected 100x60, got %dx%d", nm.width, nm.height)
	}
}

// ── Utility tests ───────────────────────────────────────────────────

func TestRankString(t *testing.T) {
	cases := map[int]string{1: "A", 2: "2", 10: "10", 11: "J", 12: "Q", 13: "K"}
	for rank, expected := range cases {
		if got := rankString(rank); got != expected {
			t.Errorf("rankString(%d) = %q, want %q", rank, got, expected)
		}
	}
}

func TestSuitRune(t *testing.T) {
	if suitRune(suitClubs) != '♣' {
		t.Error("clubs")
	}
	if suitRune(suitDiamonds) != '♦' {
		t.Error("diamonds")
	}
	if suitRune(suitHearts) != '♥' {
		t.Error("hearts")
	}
	if suitRune(suitSpades) != '♠' {
		t.Error("spades")
	}
}

func TestIsRed(t *testing.T) {
	if !isRed(suitHearts) {
		t.Error("hearts should be red")
	}
	if !isRed(suitDiamonds) {
		t.Error("diamonds should be red")
	}
	if isRed(suitClubs) {
		t.Error("clubs should not be red")
	}
	if isRed(suitSpades) {
		t.Error("spades should not be red")
	}
}

func TestOppositeColor(t *testing.T) {
	if !oppositeColor(suitHearts, suitSpades) {
		t.Error("hearts and spades should be opposite")
	}
	if oppositeColor(suitHearts, suitDiamonds) {
		t.Error("hearts and diamonds should not be opposite")
	}
}

func TestTimedBonus(t *testing.T) {
	if timedBonus(0) != 0 {
		t.Error("0 elapsed should give 0 bonus")
	}
	bonus := timedBonus(100)
	if bonus != 7000 {
		t.Errorf("expected 7000 bonus for 100s, got %d", bonus)
	}
}

// ── Tableau column height ───────────────────────────────────────────

func TestTableauColHeight(t *testing.T) {
	// Empty.
	if h := tableauColHeight(nil); h != cardH {
		t.Errorf("empty col height should be %d, got %d", cardH, h)
	}
	// 1 face-up card.
	if h := tableauColHeight([]card{{faceUp: true}}); h != cardH {
		t.Errorf("1 face-up should be %d, got %d", cardH, h)
	}
	// 3 hidden + 2 face-up.
	tab := []card{
		{faceUp: false}, {faceUp: false}, {faceUp: false},
		{faceUp: true}, {faceUp: true},
	}
	// 3*1 + 1*2 + 5 = 10
	expected := 3 + 2 + cardH
	if h := tableauColHeight(tab); h != expected {
		t.Errorf("expected %d, got %d", expected, h)
	}
}

package main

import (
	"encoding/base64"
	"encoding/json"
	"math"
	"os"
	"testing"

	tea "charm.land/bubbletea/v2"
)

func TestInterleave(t *testing.T) {
	got := interleave([]string{"a", "b", "c"}, ",")
	want := []string{"a", ",", "b", ",", "c"}
	if len(got) != len(want) {
		t.Fatalf("len=%d want %d", len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("idx %d = %q want %q", i, got[i], want[i])
		}
	}

	if out := interleave([]string{}, ","); len(out) != 0 {
		t.Fatalf("empty parts should stay empty")
	}
	if out := interleave([]string{"a"}, ""); len(out) != 1 || out[0] != "a" {
		t.Fatalf("empty sep should return original slice")
	}
}

func TestOpSymbol(t *testing.T) {
	cases := map[byte]string{
		'+': "+",
		'-': "\u2212",
		'*': "\u00d7",
		'/': "\u00f7",
		'?': "?",
	}
	for op, want := range cases {
		if got := opSymbol(op); got != want {
			t.Fatalf("opSymbol(%q)=%q want %q", op, got, want)
		}
	}
}

func TestFormatNum(t *testing.T) {
	cases := []struct {
		in   float64
		want string
	}{
		{2, "2"},
		{2.5, "2.5"},
		{2.5000000000, "2.5"},
		{0.0001, "0.0001"},
		{1e16, "10000000000000000"},
	}
	for _, c := range cases {
		if got := formatNum(c.in); got != c.want {
			t.Fatalf("formatNum(%v)=%q want %q", c.in, got, c.want)
		}
	}
	if got := formatNum(math.NaN()); got != "Error" {
		t.Fatalf("NaN => %q", got)
	}
	if got := formatNum(math.Inf(1)); got != "Error" {
		t.Fatalf("Inf => %q", got)
	}
}

func TestCalculatorFlow(t *testing.T) {
	m := newModel()
	m.handleKey('1')
	m.handleKey('2')
	m.handleKey('+')
	m.handleKey('3')
	m.handleKey('=')
	if m.display != "15" {
		t.Fatalf("display=%q want %q", m.display, "15")
	}

	m.handleKey('c')
	if m.display != "0" || m.op != 0 || m.hasLeft {
		t.Fatalf("clear failed: %+v", m)
	}

	m.display = "12"
	m.newNum = false
	m.backspace()
	if m.display != "1" {
		t.Fatalf("backspace=%q want %q", m.display, "1")
	}
	m.backspace()
	if m.display != "0" || !m.newNum {
		t.Fatalf("backspace to zero: display=%q newNum=%v", m.display, m.newNum)
	}

	m.display = "50"
	m.newNum = false
	m.percent()
	if m.display != "0.5" {
		t.Fatalf("percent=%q want %q", m.display, "0.5")
	}

	m.clear()
	m.display = "9"
	m.newNum = false
	m.applyOp('/')
	m.display = "0"
	m.newNum = false
	m.evaluate()
	if m.err != "Div by 0" {
		t.Fatalf("div by 0 err=%q", m.err)
	}
}

func TestHitButton(t *testing.T) {
	totalW := gridCols*btnW + (gridCols-1) + padX*2
	totalH := displayH + gridRows*btnH + (gridRows-1) + padY*2 + 2
	m := newModel()
	m.width = totalW
	m.height = totalH

	gridStartX := padX
	gridStartY := displayH + 2 + padY

	r, c, ok := m.hitButton(gridStartX+1, gridStartY+0)
	if !ok || r != 0 || c != 0 {
		t.Fatalf("hitButton top-left = (%d,%d,%v)", r, c, ok)
	}

	// Wide "0" button spans two columns.
	r, c, ok = m.hitButton(gridStartX+btnW+1, gridStartY+(gridRows-1)*(btnH+1))
	if !ok || r != 4 || c != 0 {
		t.Fatalf("hitButton wide 0 = (%d,%d,%v)", r, c, ok)
	}
}

// --- appendDot tests ---

func TestAppendDotNewNum(t *testing.T) {
	m := newModel()
	m.newNum = true
	m.display = "5"
	m.appendDot()
	if m.display != "0." {
		t.Fatalf("appendDot with newNum: display=%q want %q", m.display, "0.")
	}
	if m.newNum {
		t.Fatalf("newNum should be false after appendDot")
	}
}

func TestAppendDotExistingNumber(t *testing.T) {
	m := newModel()
	m.newNum = false
	m.display = "42"
	m.appendDot()
	if m.display != "42." {
		t.Fatalf("appendDot to existing: display=%q want %q", m.display, "42.")
	}
}

func TestAppendDotDuplicateRejection(t *testing.T) {
	m := newModel()
	m.newNum = false
	m.display = "3.14"
	m.appendDot()
	if m.display != "3.14" {
		t.Fatalf("duplicate dot should be rejected: display=%q want %q", m.display, "3.14")
	}
}

// --- gridButton tests ---

func TestGridButtonValid(t *testing.T) {
	btn := gridButton(0, 0)
	if btn == nil {
		t.Fatal("gridButton(0,0) should not be nil")
	}
	if btn.label != "C" {
		t.Fatalf("gridButton(0,0).label=%q want %q", btn.label, "C")
	}

	btn = gridButton(1, 2)
	if btn == nil {
		t.Fatal("gridButton(1,2) should not be nil")
	}
	if btn.label != "9" {
		t.Fatalf("gridButton(1,2).label=%q want %q", btn.label, "9")
	}

	btn = gridButton(4, 0)
	if btn == nil {
		t.Fatal("gridButton(4,0) should not be nil")
	}
	if btn.label != "0" || !btn.wide {
		t.Fatalf("gridButton(4,0): label=%q wide=%v", btn.label, btn.wide)
	}
}

func TestGridButtonOutOfBounds(t *testing.T) {
	if btn := gridButton(-1, 0); btn != nil {
		t.Fatal("negative row should return nil")
	}
	if btn := gridButton(5, 0); btn != nil {
		t.Fatal("row past grid should return nil")
	}
	if btn := gridButton(0, -1); btn != nil {
		t.Fatal("negative col should return nil")
	}
	if btn := gridButton(0, 4); btn != nil {
		t.Fatal("col past row length should return nil")
	}
	// Last row only has 3 buttons (0 is wide + . + =)
	if btn := gridButton(4, 3); btn != nil {
		t.Fatal("col=3 on last row should return nil")
	}
}

// --- handleKey all branches ---

func TestHandleKeyDot(t *testing.T) {
	m := newModel()
	m.newNum = true
	m.handleKey('.')
	if m.display != "0." {
		t.Fatalf("handleKey('.') with newNum: display=%q want %q", m.display, "0.")
	}
}

func TestHandleKeyEnter(t *testing.T) {
	m := newModel()
	m.handleKey('5')
	m.handleKey('+')
	m.handleKey('3')
	m.handleKey('\r')
	if m.display != "8" {
		t.Fatalf("handleKey enter: display=%q want %q", m.display, "8")
	}
}

func TestHandleKeyNewline(t *testing.T) {
	m := newModel()
	m.handleKey('5')
	m.handleKey('+')
	m.handleKey('3')
	m.handleKey('\n')
	if m.display != "8" {
		t.Fatalf("handleKey newline: display=%q want %q", m.display, "8")
	}
}

func TestHandleKeyBackspace127(t *testing.T) {
	m := newModel()
	m.display = "123"
	m.newNum = false
	m.handleKey(127)
	if m.display != "12" {
		t.Fatalf("backspace 127: display=%q want %q", m.display, "12")
	}
}

func TestHandleKeyBackspace8(t *testing.T) {
	m := newModel()
	m.display = "45"
	m.newNum = false
	m.handleKey(8)
	if m.display != "4" {
		t.Fatalf("backspace 8: display=%q want %q", m.display, "4")
	}
}

func TestHandleKeyClearUpperCase(t *testing.T) {
	m := newModel()
	m.display = "99"
	m.newNum = false
	m.handleKey('C')
	if m.display != "0" {
		t.Fatalf("handleKey('C'): display=%q want %q", m.display, "0")
	}
}

func TestHandleKeyUnknownKey(t *testing.T) {
	m := newModel()
	m.display = "42"
	m.newNum = false
	m.handleKey('z') // unknown key
	if m.display != "42" {
		t.Fatalf("unknown key should not change display: got %q", m.display)
	}
}

func TestHandleKeyErrCleared(t *testing.T) {
	m := newModel()
	m.err = "Div by 0"
	m.handleKey('5')
	if m.err != "" {
		t.Fatalf("err should be cleared on new key entry: err=%q", m.err)
	}
}

// --- newModel with TERMDESK_APP_STATE ---

func TestNewModelWithValidState(t *testing.T) {
	cs := calcState{
		Display: "42",
		Result:  "42",
		Op:      '+',
		Left:    10,
		HasLeft: true,
		NewNum:  false,
	}
	data, _ := json.Marshal(cs)
	encoded := base64.StdEncoding.EncodeToString(data)
	os.Setenv("TERMDESK_APP_STATE", encoded)
	defer os.Unsetenv("TERMDESK_APP_STATE")

	m := newModel()
	if m.display != "42" {
		t.Fatalf("restored display=%q want %q", m.display, "42")
	}
	if m.result != "42" {
		t.Fatalf("restored result=%q want %q", m.result, "42")
	}
	if m.op != '+' {
		t.Fatalf("restored op=%d want %d", m.op, '+')
	}
	if m.left != 10 {
		t.Fatalf("restored left=%f want %f", m.left, 10.0)
	}
	if !m.hasLeft {
		t.Fatal("restored hasLeft should be true")
	}
	if m.newNum {
		t.Fatal("restored newNum should be false")
	}
}

func TestNewModelWithInvalidBase64(t *testing.T) {
	os.Setenv("TERMDESK_APP_STATE", "!!!invalid-base64!!!")
	defer os.Unsetenv("TERMDESK_APP_STATE")

	m := newModel()
	// Should fall back to default state
	if m.display != "0" {
		t.Fatalf("invalid base64 should fallback: display=%q want %q", m.display, "0")
	}
	if !m.newNum {
		t.Fatal("invalid base64 should fallback: newNum should be true")
	}
}

func TestNewModelWithInvalidJSON(t *testing.T) {
	encoded := base64.StdEncoding.EncodeToString([]byte("not json"))
	os.Setenv("TERMDESK_APP_STATE", encoded)
	defer os.Unsetenv("TERMDESK_APP_STATE")

	m := newModel()
	// Should fall back to default state
	if m.display != "0" {
		t.Fatalf("invalid json should fallback: display=%q want %q", m.display, "0")
	}
	if !m.newNum {
		t.Fatal("invalid json should fallback: newNum should be true")
	}
}

func TestNewModelWithNoEnvVar(t *testing.T) {
	os.Unsetenv("TERMDESK_APP_STATE")
	m := newModel()
	if m.display != "0" {
		t.Fatalf("no env: display=%q want %q", m.display, "0")
	}
	if !m.newNum {
		t.Fatal("no env: newNum should be true")
	}
	if m.hoverR != -1 || m.hoverC != -1 {
		t.Fatalf("no env: hover should be (-1,-1), got (%d,%d)", m.hoverR, m.hoverC)
	}
}

// --- applyOp tests ---

func TestApplyOpChainedOperations(t *testing.T) {
	m := newModel()
	m.handleKey('2')
	m.handleKey('+')
	m.handleKey('3')
	m.handleKey('*') // should evaluate 2+3=5, then set op to *
	if m.display != "5" {
		t.Fatalf("chained applyOp: display=%q want %q", m.display, "5")
	}
	if m.op != '*' {
		t.Fatalf("chained applyOp: op=%d want %d", m.op, '*')
	}
	if !m.hasLeft {
		t.Fatal("chained applyOp: hasLeft should be true")
	}
}

func TestApplyOpParseError(t *testing.T) {
	m := newModel()
	m.display = "abc"
	m.newNum = false
	m.applyOp('+')
	if m.err != "Error" {
		t.Fatalf("applyOp parse error: err=%q want %q", m.err, "Error")
	}
}

// --- evaluate tests ---

func TestEvaluateAddition(t *testing.T) {
	m := newModel()
	m.handleKey('7')
	m.handleKey('+')
	m.handleKey('3')
	m.handleKey('=')
	if m.display != "10" {
		t.Fatalf("7+3: display=%q want %q", m.display, "10")
	}
}

func TestEvaluateSubtraction(t *testing.T) {
	m := newModel()
	m.handleKey('9')
	m.handleKey('-')
	m.handleKey('4')
	m.handleKey('=')
	if m.display != "5" {
		t.Fatalf("9-4: display=%q want %q", m.display, "5")
	}
}

func TestEvaluateMultiplication(t *testing.T) {
	m := newModel()
	m.handleKey('6')
	m.handleKey('*')
	m.handleKey('7')
	m.handleKey('=')
	if m.display != "42" {
		t.Fatalf("6*7: display=%q want %q", m.display, "42")
	}
}

func TestEvaluateDivision(t *testing.T) {
	m := newModel()
	m.handleKey('8')
	m.handleKey('/')
	m.handleKey('4')
	m.handleKey('=')
	if m.display != "2" {
		t.Fatalf("8/4: display=%q want %q", m.display, "2")
	}
}

func TestEvaluateNoOp(t *testing.T) {
	m := newModel()
	m.display = "5"
	m.hasLeft = false
	m.op = 0
	m.evaluate()
	if m.display != "5" {
		t.Fatalf("evaluate no-op: display=%q want %q", m.display, "5")
	}
}

func TestEvaluateNoOpHasLeftNoOp(t *testing.T) {
	m := newModel()
	m.display = "5"
	m.hasLeft = true
	m.op = 0
	m.evaluate()
	if m.display != "5" {
		t.Fatalf("evaluate hasLeft but no op: display=%q want %q", m.display, "5")
	}
}

func TestEvaluateParseError(t *testing.T) {
	m := newModel()
	m.display = "xyz"
	m.hasLeft = true
	m.left = 10
	m.op = '+'
	m.newNum = false
	m.evaluate()
	if m.err != "Error" {
		t.Fatalf("evaluate parse error: err=%q want %q", m.err, "Error")
	}
}

func TestEvaluateDivisionByZero(t *testing.T) {
	m := newModel()
	m.handleKey('5')
	m.handleKey('/')
	m.handleKey('0')
	m.handleKey('=')
	if m.err != "Div by 0" {
		t.Fatalf("division by zero: err=%q want %q", m.err, "Div by 0")
	}
	if m.hasLeft {
		t.Fatal("division by zero should clear hasLeft")
	}
	if m.op != 0 {
		t.Fatalf("division by zero should clear op: op=%d", m.op)
	}
}

// --- appendDigit edge cases ---

func TestAppendDigitReplaceZero(t *testing.T) {
	m := newModel()
	m.display = "0"
	m.newNum = false
	m.appendDigit('5')
	if m.display != "5" {
		t.Fatalf("replace zero: display=%q want %q", m.display, "5")
	}
}

func TestAppendDigitNewNumTrue(t *testing.T) {
	m := newModel()
	m.display = "99"
	m.newNum = true
	m.appendDigit('3')
	if m.display != "3" {
		t.Fatalf("newNum=true digit: display=%q want %q", m.display, "3")
	}
	if m.newNum {
		t.Fatal("newNum should be false after appendDigit")
	}
}

func TestAppendDigitNormalConcat(t *testing.T) {
	m := newModel()
	m.display = "12"
	m.newNum = false
	m.appendDigit('3')
	if m.display != "123" {
		t.Fatalf("normal concat: display=%q want %q", m.display, "123")
	}
}

// --- backspace edge cases ---

func TestBackspaceWhenNewNum(t *testing.T) {
	m := newModel()
	m.display = "42"
	m.newNum = true
	m.backspace()
	if m.display != "42" {
		t.Fatalf("backspace with newNum should be no-op: display=%q", m.display)
	}
}

// --- percent edge cases ---

func TestPercentParseError(t *testing.T) {
	m := newModel()
	m.display = "abc"
	m.newNum = false
	m.percent()
	// Should silently return without changing display
	if m.display != "abc" {
		t.Fatalf("percent parse error: display=%q want %q", m.display, "abc")
	}
}

// --- hitButton miss cases ---

func TestHitButtonAboveGrid(t *testing.T) {
	totalW := gridCols*btnW + (gridCols-1) + padX*2
	totalH := displayH + gridRows*btnH + (gridRows-1) + padY*2 + 2
	m := newModel()
	m.width = totalW
	m.height = totalH

	// Click above the grid area (in the display region)
	_, _, ok := m.hitButton(padX+1, 0)
	if ok {
		t.Fatal("clicking above grid should miss")
	}
}

func TestHitButtonLeftOfGrid(t *testing.T) {
	totalW := gridCols*btnW + (gridCols-1) + padX*2
	totalH := displayH + gridRows*btnH + (gridRows-1) + padY*2 + 2
	m := newModel()
	m.width = totalW
	m.height = totalH

	gridStartY := displayH + 2 + padY
	// Click to the left of the grid
	_, _, ok := m.hitButton(-1, gridStartY)
	if ok {
		t.Fatal("clicking left of grid should miss")
	}
}

func TestHitButtonInRowGap(t *testing.T) {
	totalW := gridCols*btnW + (gridCols-1) + padX*2
	totalH := displayH + gridRows*btnH + (gridRows-1) + padY*2 + 2
	m := newModel()
	m.width = totalW
	m.height = totalH

	gridStartX := padX
	gridStartY := displayH + 2 + padY
	// Click in the gap between row 0 and row 1 (y = btnH, which is the gap)
	_, _, ok := m.hitButton(gridStartX+1, gridStartY+btnH)
	if ok {
		t.Fatal("clicking in row gap should miss")
	}
}

func TestHitButtonPastColumns(t *testing.T) {
	totalW := gridCols*btnW + (gridCols-1) + padX*2
	totalH := displayH + gridRows*btnH + (gridRows-1) + padY*2 + 2
	m := newModel()
	m.width = totalW
	m.height = totalH

	gridStartX := padX
	gridStartY := displayH + 2 + padY
	// Click past the last column
	_, _, ok := m.hitButton(gridStartX+totalW+50, gridStartY)
	if ok {
		t.Fatal("clicking past columns should miss")
	}
}

func TestHitButtonPastRows(t *testing.T) {
	totalW := gridCols*btnW + (gridCols-1) + padX*2
	totalH := displayH + gridRows*btnH + (gridRows-1) + padY*2 + 2
	m := newModel()
	m.width = totalW
	m.height = totalH

	gridStartX := padX
	gridStartY := displayH + 2 + padY
	// Click below the last row
	_, _, ok := m.hitButton(gridStartX+1, gridStartY+gridRows*(btnH+1)+10)
	if ok {
		t.Fatal("clicking past rows should miss")
	}
}

func TestHitButtonLastRowNonWideButtons(t *testing.T) {
	totalW := gridCols*btnW + (gridCols-1) + padX*2
	totalH := displayH + gridRows*btnH + (gridRows-1) + padY*2 + 2
	m := newModel()
	m.width = totalW
	m.height = totalH

	gridStartX := padX
	gridStartY := displayH + 2 + padY
	lastRowY := gridStartY + (gridRows-1)*(btnH+1)

	// "." button: after the wide "0" (btnW*2+1), there's a gap (+1), then "."
	dotX := gridStartX + (btnW*2 + 1) + 1
	r, c, ok := m.hitButton(dotX+1, lastRowY)
	if !ok || r != 4 || c != 1 {
		t.Fatalf("hitButton dot: r=%d c=%d ok=%v, want r=4 c=1 ok=true", r, c, ok)
	}

	// "=" button: after ".", there's a gap (+1), then "="
	eqX := dotX + btnW + 1
	r, c, ok = m.hitButton(eqX+1, lastRowY)
	if !ok || r != 4 || c != 2 {
		t.Fatalf("hitButton equals: r=%d c=%d ok=%v, want r=4 c=2 ok=true", r, c, ok)
	}

	// Past the "=" button
	pastEqX := eqX + btnW + 1
	_, _, ok = m.hitButton(pastEqX, lastRowY)
	if ok {
		t.Fatal("clicking past last button in last row should miss")
	}
}

// --- chained calculations end-to-end ---

func TestChainedCalculation(t *testing.T) {
	m := newModel()
	// 2 + 3 * 4 = (2+3=5, then 5*4=20)
	m.handleKey('2')
	m.handleKey('+')
	m.handleKey('3')
	m.handleKey('*')
	if m.display != "5" {
		t.Fatalf("chained 2+3: display=%q want %q", m.display, "5")
	}
	m.handleKey('4')
	m.handleKey('=')
	if m.display != "20" {
		t.Fatalf("chained 5*4: display=%q want %q", m.display, "20")
	}
}

// --- decimal arithmetic end-to-end ---

func TestDecimalArithmetic(t *testing.T) {
	m := newModel()
	m.handleKey('1')
	m.handleKey('.')
	m.handleKey('5')
	m.handleKey('+')
	m.handleKey('2')
	m.handleKey('.')
	m.handleKey('3')
	m.handleKey('=')
	if m.display != "3.8" {
		t.Fatalf("1.5+2.3: display=%q want %q", m.display, "3.8")
	}
}

// --- Update tests ---

func TestUpdateWindowSizeMsg(t *testing.T) {
	m := newModel()
	msg := tea.WindowSizeMsg{Width: 80, Height: 24}
	result, cmd := m.Update(msg)
	rm := result.(model)
	if rm.width != 80 || rm.height != 24 {
		t.Fatalf("window size: width=%d height=%d, want 80x24", rm.width, rm.height)
	}
	if cmd != nil {
		t.Fatal("WindowSizeMsg should return nil cmd")
	}
}

func TestUpdateKeyPressQuit(t *testing.T) {
	m := newModel()
	// Test 'q' key
	msg := tea.KeyPressMsg(tea.Key{Code: 'q'})
	_, cmd := m.Update(msg)
	if cmd == nil {
		t.Fatal("'q' key should return quit cmd")
	}

	// Test Escape key
	msg = tea.KeyPressMsg(tea.Key{Code: tea.KeyEscape})
	_, cmd = m.Update(msg)
	if cmd == nil {
		t.Fatal("escape key should return quit cmd")
	}

	// Test 'Q' key
	msg = tea.KeyPressMsg(tea.Key{Code: 'Q'})
	_, cmd = m.Update(msg)
	if cmd == nil {
		t.Fatal("'Q' key should return quit cmd")
	}

	// Test ctrl+c
	msg = tea.KeyPressMsg(tea.Key{Code: 3})
	_, cmd = m.Update(msg)
	if cmd == nil {
		t.Fatal("ctrl+c should return quit cmd")
	}
}

func TestUpdateKeyPressDigits(t *testing.T) {
	m := newModel()
	for _, d := range "0123456789" {
		msg := tea.KeyPressMsg(tea.Key{Code: rune(d)})
		result, _ := m.Update(msg)
		m = result.(model)
	}
	if m.display != "123456789" {
		t.Fatalf("digits: display=%q want %q", m.display, "123456789")
	}
}

func TestUpdateKeyPressOperators(t *testing.T) {
	m := newModel()
	// Type "5"
	msg := tea.KeyPressMsg(tea.Key{Code: '5'})
	result, _ := m.Update(msg)
	m = result.(model)

	// Type "+"
	msg = tea.KeyPressMsg(tea.Key{Code: '+'})
	result, _ = m.Update(msg)
	m = result.(model)
	if m.op != '+' {
		t.Fatalf("+ op: op=%d want %d", m.op, '+')
	}

	// Type "3", then "-"
	msg = tea.KeyPressMsg(tea.Key{Code: '3'})
	result, _ = m.Update(msg)
	m = result.(model)
	msg = tea.KeyPressMsg(tea.Key{Code: '-'})
	result, _ = m.Update(msg)
	m = result.(model)
	if m.op != '-' {
		t.Fatalf("- op: op=%d want %d", m.op, '-')
	}
}

func TestUpdateKeyPressMultiplyAliases(t *testing.T) {
	// Test 'x' maps to '*'
	m := newModel()
	msg := tea.KeyPressMsg(tea.Key{Code: '5'})
	result, _ := m.Update(msg)
	m = result.(model)
	msg = tea.KeyPressMsg(tea.Key{Code: 'x'})
	result, _ = m.Update(msg)
	m = result.(model)
	if m.op != '*' {
		t.Fatalf("x key op: op=%d want %d", m.op, '*')
	}

	// Test 'X' maps to '*'
	m = newModel()
	msg = tea.KeyPressMsg(tea.Key{Code: '5'})
	result, _ = m.Update(msg)
	m = result.(model)
	msg = tea.KeyPressMsg(tea.Key{Code: 'X'})
	result, _ = m.Update(msg)
	m = result.(model)
	if m.op != '*' {
		t.Fatalf("X key op: op=%d want %d", m.op, '*')
	}

	// Test '*' maps to '*'
	m = newModel()
	msg = tea.KeyPressMsg(tea.Key{Code: '5'})
	result, _ = m.Update(msg)
	m = result.(model)
	msg = tea.KeyPressMsg(tea.Key{Code: '*'})
	result, _ = m.Update(msg)
	m = result.(model)
	if m.op != '*' {
		t.Fatalf("* key op: op=%d want %d", m.op, '*')
	}
}

func TestUpdateKeyPressDivideAliases(t *testing.T) {
	// Test '/' maps to '/'
	m := newModel()
	msg := tea.KeyPressMsg(tea.Key{Code: '5'})
	result, _ := m.Update(msg)
	m = result.(model)
	msg = tea.KeyPressMsg(tea.Key{Code: '/'})
	result, _ = m.Update(msg)
	m = result.(model)
	if m.op != '/' {
		t.Fatalf("/ key op: op=%d want %d", m.op, '/')
	}

	// Test '\\' maps to '/'
	m = newModel()
	msg = tea.KeyPressMsg(tea.Key{Code: '5'})
	result, _ = m.Update(msg)
	m = result.(model)
	msg = tea.KeyPressMsg(tea.Key{Code: '\\'})
	result, _ = m.Update(msg)
	m = result.(model)
	if m.op != '/' {
		t.Fatalf("\\ key op: op=%d want %d", m.op, '/')
	}
}

func TestUpdateKeyPressPercent(t *testing.T) {
	m := newModel()
	m.display = "50"
	m.newNum = false
	msg := tea.KeyPressMsg(tea.Key{Code: '%'})
	result, _ := m.Update(msg)
	rm := result.(model)
	if rm.display != "0.5" {
		t.Fatalf("percent: display=%q want %q", rm.display, "0.5")
	}
}

func TestUpdateKeyPressEquals(t *testing.T) {
	m := newModel()
	// 4+6=
	for _, k := range []rune{'4', '+', '6', '='} {
		msg := tea.KeyPressMsg(tea.Key{Code: k})
		result, _ := m.Update(msg)
		m = result.(model)
	}
	if m.display != "10" {
		t.Fatalf("equals: display=%q want %q", m.display, "10")
	}
}

func TestUpdateKeyPressEnter(t *testing.T) {
	m := newModel()
	for _, k := range []rune{'4', '+', '6'} {
		msg := tea.KeyPressMsg(tea.Key{Code: k})
		result, _ := m.Update(msg)
		m = result.(model)
	}
	msg := tea.KeyPressMsg(tea.Key{Code: tea.KeyEnter})
	result, _ := m.Update(msg)
	rm := result.(model)
	if rm.display != "10" {
		t.Fatalf("enter key: display=%q want %q", rm.display, "10")
	}
}

func TestUpdateKeyPressClear(t *testing.T) {
	m := newModel()
	msg := tea.KeyPressMsg(tea.Key{Code: '5'})
	result, _ := m.Update(msg)
	m = result.(model)
	msg = tea.KeyPressMsg(tea.Key{Code: 'c'})
	result, _ = m.Update(msg)
	m = result.(model)
	if m.display != "0" {
		t.Fatalf("clear: display=%q want %q", m.display, "0")
	}

	// Test 'C'
	msg = tea.KeyPressMsg(tea.Key{Code: '5'})
	result, _ = m.Update(msg)
	m = result.(model)
	msg = tea.KeyPressMsg(tea.Key{Code: 'C'})
	result, _ = m.Update(msg)
	m = result.(model)
	if m.display != "0" {
		t.Fatalf("Clear C: display=%q want %q", m.display, "0")
	}
}

func TestUpdateKeyPressBackspace(t *testing.T) {
	m := newModel()
	m.display = "42"
	m.newNum = false
	msg := tea.KeyPressMsg(tea.Key{Code: tea.KeyBackspace})
	result, _ := m.Update(msg)
	rm := result.(model)
	if rm.display != "4" {
		t.Fatalf("backspace: display=%q want %q", rm.display, "4")
	}
}

func TestUpdateKeyPressDot(t *testing.T) {
	m := newModel()
	msg := tea.KeyPressMsg(tea.Key{Code: '.'})
	result, _ := m.Update(msg)
	rm := result.(model)
	if rm.display != "0." {
		t.Fatalf("dot: display=%q want %q", rm.display, "0.")
	}
}

func TestUpdateMouseClick(t *testing.T) {
	totalW := gridCols*btnW + (gridCols-1) + padX*2
	totalH := displayH + gridRows*btnH + (gridRows-1) + padY*2 + 2
	m := newModel()
	m.width = totalW
	m.height = totalH

	gridStartX := padX
	gridStartY := displayH + 2 + padY

	// Click on the "7" button (row 1, col 0)
	clickX := gridStartX + 1
	clickY := gridStartY + (btnH + 1) // row 1
	msg := tea.MouseClickMsg(tea.Mouse{Button: tea.MouseLeft, X: clickX, Y: clickY})
	result, _ := m.Update(msg)
	rm := result.(model)
	if rm.display != "7" {
		t.Fatalf("mouse click 7: display=%q want %q", rm.display, "7")
	}
}

func TestUpdateMouseClickMiss(t *testing.T) {
	totalW := gridCols*btnW + (gridCols-1) + padX*2
	totalH := displayH + gridRows*btnH + (gridRows-1) + padY*2 + 2
	m := newModel()
	m.width = totalW
	m.height = totalH

	// Click outside the grid
	msg := tea.MouseClickMsg(tea.Mouse{Button: tea.MouseLeft, X: -10, Y: -10})
	result, _ := m.Update(msg)
	rm := result.(model)
	if rm.display != "0" {
		t.Fatalf("mouse click miss: display=%q want %q", rm.display, "0")
	}
}

func TestUpdateMouseClickRightButton(t *testing.T) {
	totalW := gridCols*btnW + (gridCols-1) + padX*2
	totalH := displayH + gridRows*btnH + (gridRows-1) + padY*2 + 2
	m := newModel()
	m.width = totalW
	m.height = totalH

	gridStartX := padX
	gridStartY := displayH + 2 + padY

	// Right-click on a button should be ignored
	msg := tea.MouseClickMsg(tea.Mouse{Button: tea.MouseRight, X: gridStartX + 1, Y: gridStartY})
	result, _ := m.Update(msg)
	rm := result.(model)
	if rm.display != "0" {
		t.Fatalf("right click should be ignored: display=%q", rm.display)
	}
}

func TestUpdateMouseMotion(t *testing.T) {
	totalW := gridCols*btnW + (gridCols-1) + padX*2
	totalH := displayH + gridRows*btnH + (gridRows-1) + padY*2 + 2
	m := newModel()
	m.width = totalW
	m.height = totalH

	gridStartX := padX
	gridStartY := displayH + 2 + padY

	// Move mouse over "C" button (row 0, col 0)
	msg := tea.MouseMotionMsg(tea.Mouse{X: gridStartX + 1, Y: gridStartY})
	result, _ := m.Update(msg)
	rm := result.(model)
	if rm.hoverR != 0 || rm.hoverC != 0 {
		t.Fatalf("hover: r=%d c=%d, want 0,0", rm.hoverR, rm.hoverC)
	}

	// Move mouse outside grid
	msg = tea.MouseMotionMsg(tea.Mouse{X: -10, Y: -10})
	result, _ = rm.Update(msg)
	rm = result.(model)
	if rm.hoverR != -1 || rm.hoverC != -1 {
		t.Fatalf("hover outside: r=%d c=%d, want -1,-1", rm.hoverR, rm.hoverC)
	}
}

func TestUpdateStateDumpMsg(t *testing.T) {
	m := newModel()
	m.display = "42"
	m.op = '+'
	m.left = 10
	m.hasLeft = true

	// Send stateDumpMsg; it should return a command (listenStateDump)
	_, cmd := m.Update(stateDumpMsg{})
	if cmd == nil {
		t.Fatal("stateDumpMsg should return a cmd to re-listen")
	}
}

func TestUpdateUnknownMsg(t *testing.T) {
	m := newModel()
	type unknownMsg struct{}
	result, cmd := m.Update(unknownMsg{})
	rm := result.(model)
	if rm.display != "0" {
		t.Fatalf("unknown msg should not change state: display=%q", rm.display)
	}
	if cmd != nil {
		t.Fatal("unknown msg should return nil cmd")
	}
}

// --- View tests ---

func TestViewZeroSize(t *testing.T) {
	m := newModel()
	m.width = 0
	m.height = 0
	v := m.View()
	// When width/height is 0, View returns "Loading..."
	if !v.AltScreen {
		t.Fatal("View should set AltScreen")
	}
}

func TestViewNonZeroSize(t *testing.T) {
	m := newModel()
	m.width = 80
	m.height = 24
	m.display = "42"
	v := m.View()
	if !v.AltScreen {
		t.Fatal("View should set AltScreen")
	}
	if v.MouseMode != tea.MouseModeCellMotion {
		t.Fatalf("View should set MouseModeCellMotion, got %d", v.MouseMode)
	}
}

func TestViewWithError(t *testing.T) {
	m := newModel()
	m.width = 80
	m.height = 24
	m.err = "Div by 0"
	v := m.View()
	if !v.AltScreen {
		t.Fatal("View should set AltScreen")
	}
}

func TestViewWithPendingOp(t *testing.T) {
	m := newModel()
	m.width = 80
	m.height = 24
	m.op = '+'
	m.left = 10
	m.hasLeft = true
	m.display = "5"
	v := m.View()
	if !v.AltScreen {
		t.Fatal("View should set AltScreen")
	}
}

func TestViewWithHover(t *testing.T) {
	m := newModel()
	m.width = 80
	m.height = 24
	m.hoverR = 1
	m.hoverC = 2
	v := m.View()
	if !v.AltScreen {
		t.Fatal("View should set AltScreen")
	}
}

// --- Init test ---

func TestInit(t *testing.T) {
	m := newModel()
	cmd := m.Init()
	if cmd == nil {
		t.Fatal("Init should return listenStateDump cmd")
	}
}

// --- End-to-end complex calculations ---

func TestComplexCalculation(t *testing.T) {
	// 1.5 + 2.5 = 4
	m := newModel()
	for _, k := range []byte{'1', '.', '5', '+', '2', '.', '5', '='} {
		m.handleKey(k)
	}
	if m.display != "4" {
		t.Fatalf("1.5+2.5: display=%q want %q", m.display, "4")
	}
}

func TestCalculationAfterClear(t *testing.T) {
	m := newModel()
	m.handleKey('5')
	m.handleKey('+')
	m.handleKey('3')
	m.handleKey('=')
	m.handleKey('c')
	m.handleKey('9')
	m.handleKey('-')
	m.handleKey('4')
	m.handleKey('=')
	if m.display != "5" {
		t.Fatalf("after clear 9-4: display=%q want %q", m.display, "5")
	}
}

func TestResultAfterEvaluate(t *testing.T) {
	m := newModel()
	m.handleKey('3')
	m.handleKey('*')
	m.handleKey('7')
	m.handleKey('=')
	if m.result != "21" {
		t.Fatalf("result after evaluate: result=%q want %q", m.result, "21")
	}
	if !m.newNum {
		t.Fatal("newNum should be true after evaluate")
	}
}

func TestMultipleDecimalDots(t *testing.T) {
	m := newModel()
	m.handleKey('1')
	m.handleKey('.')
	m.handleKey('2')
	m.handleKey('.')  // should be rejected
	m.handleKey('3')
	if m.display != "1.23" {
		t.Fatalf("multiple dots: display=%q want %q", m.display, "1.23")
	}
}

func TestPercentAfterCalculation(t *testing.T) {
	m := newModel()
	m.handleKey('2')
	m.handleKey('0')
	m.handleKey('0')
	m.newNum = false
	m.handleKey('%')
	if m.display != "2" {
		t.Fatalf("200%%: display=%q want %q", m.display, "2")
	}
}

func TestAllButtonKinds(t *testing.T) {
	// Verify all button types exist in the grid
	kindsSeen := map[btnType]bool{}
	for _, row := range buttonGrid {
		for _, btn := range row {
			kindsSeen[btn.kind] = true
		}
	}
	for _, k := range []btnType{btnDigit, btnOp, btnFunc, btnEqual, btnClear} {
		if !kindsSeen[k] {
			t.Fatalf("button kind %d not found in grid", k)
		}
	}
}

func TestViewAllButtonStyles(t *testing.T) {
	// Exercise View with hover on each button type
	m := newModel()
	m.width = 80
	m.height = 30

	// Hover over digit (row 1, col 0 = "7")
	m.hoverR = 1
	m.hoverC = 0
	m.View()

	// Hover over op (row 1, col 3 = multiply)
	m.hoverR = 1
	m.hoverC = 3
	m.View()

	// Hover over func (row 0, col 1 = "%")
	m.hoverR = 0
	m.hoverC = 1
	m.View()

	// Hover over equal (row 4, col 2 = "=")
	m.hoverR = 4
	m.hoverC = 2
	m.View()

	// Hover over clear (row 0, col 0 = "C")
	m.hoverR = 0
	m.hoverC = 0
	m.View()
}

package main

import (
	"fmt"
	"math"
	"os"
	"strconv"
	"strings"
	"unicode"
)

func main() {
	c := &calc{}
	c.run()
	// Clear screen on exit
	fmt.Print("\x1b[2J\x1b[H")
	os.Exit(0)
}

type calc struct {
	display string // current input/display
	result  string // last computed result
	op      byte   // pending operator: +, -, *, /
	left    float64
	hasLeft bool
	newNum  bool // next digit starts a new number
	err     string
}

func (c *calc) run() {
	// Enable raw mode
	oldState, err := makeRaw(os.Stdin.Fd())
	if err != nil {
		fmt.Fprintf(os.Stderr, "cannot enter raw mode: %v\n", err)
		return
	}
	defer restore(os.Stdin.Fd(), oldState)

	// Hide cursor
	fmt.Print("\x1b[?25l")
	defer fmt.Print("\x1b[?25h")

	c.display = "0"
	c.newNum = true
	c.render()

	buf := make([]byte, 32)
	for {
		n, err := os.Stdin.Read(buf)
		if err != nil || n == 0 {
			return
		}
		for i := 0; i < n; i++ {
			b := buf[i]
			if c.handleKey(b) {
				return // quit
			}
		}
		c.render()
	}
}

func (c *calc) handleKey(b byte) bool {
	c.err = ""

	switch {
	case b == 'q' || b == 'Q' || b == 3: // q or Ctrl+C
		return true

	case b >= '0' && b <= '9':
		c.appendDigit(b)

	case b == '.':
		c.appendDot()

	case b == '+', b == '-', b == '*', b == '/', b == 'x', b == 'X':
		if b == 'x' || b == 'X' {
			b = '*'
		}
		c.applyOp(b)

	case b == '=' || b == '\r' || b == '\n':
		c.evaluate()

	case b == 'c' || b == 'C':
		c.clear()

	case b == 127 || b == 8: // backspace/delete
		c.backspace()

	case b == '%':
		c.percent()
	}
	return false
}

func (c *calc) appendDigit(b byte) {
	if c.newNum {
		c.display = string(b)
		c.newNum = false
	} else {
		if c.display == "0" {
			c.display = string(b)
		} else {
			c.display += string(b)
		}
	}
}

func (c *calc) appendDot() {
	if c.newNum {
		c.display = "0."
		c.newNum = false
	} else if !strings.Contains(c.display, ".") {
		c.display += "."
	}
}

func (c *calc) applyOp(op byte) {
	if c.hasLeft && !c.newNum {
		c.evaluate()
	}
	val, err := strconv.ParseFloat(c.display, 64)
	if err != nil {
		c.err = "Error"
		return
	}
	c.left = val
	c.hasLeft = true
	c.op = op
	c.newNum = true
}

func (c *calc) evaluate() {
	if !c.hasLeft || c.op == 0 {
		return
	}
	right, err := strconv.ParseFloat(c.display, 64)
	if err != nil {
		c.err = "Error"
		return
	}
	var res float64
	switch c.op {
	case '+':
		res = c.left + right
	case '-':
		res = c.left - right
	case '*':
		res = c.left * right
	case '/':
		if right == 0 {
			c.err = "Div by 0"
			c.hasLeft = false
			c.op = 0
			return
		}
		res = c.left / right
	}
	c.display = formatNum(res)
	c.result = c.display
	c.hasLeft = false
	c.op = 0
	c.newNum = true
}

func (c *calc) clear() {
	c.display = "0"
	c.result = ""
	c.op = 0
	c.left = 0
	c.hasLeft = false
	c.newNum = true
	c.err = ""
}

func (c *calc) backspace() {
	if c.newNum {
		return
	}
	if len(c.display) > 1 {
		c.display = c.display[:len(c.display)-1]
	} else {
		c.display = "0"
		c.newNum = true
	}
}

func (c *calc) percent() {
	val, err := strconv.ParseFloat(c.display, 64)
	if err != nil {
		return
	}
	c.display = formatNum(val / 100)
	c.newNum = true
}

func formatNum(f float64) string {
	if math.IsInf(f, 0) || math.IsNaN(f) {
		return "Error"
	}
	// Show integer form when possible
	if f == math.Trunc(f) && math.Abs(f) < 1e15 {
		return strconv.FormatFloat(f, 'f', 0, 64)
	}
	s := strconv.FormatFloat(f, 'f', 10, 64)
	// Trim trailing zeros
	s = strings.TrimRight(s, "0")
	s = strings.TrimRight(s, ".")
	return s
}

// UI rendering
const (
	boxW = 34
	boxH = 14
)

func (c *calc) render() {
	// Move cursor to top-left and draw
	fmt.Print("\x1b[H")

	// Colors
	borderColor := "\x1b[36m"  // cyan
	displayBg := "\x1b[44m"    // blue bg
	displayFg := "\x1b[97m"    // bright white
	btnColor := "\x1b[37m"     // white
	opColor := "\x1b[33m"      // yellow
	accentColor := "\x1b[32m"  // green
	errColor := "\x1b[31m"     // red
	dimColor := "\x1b[90m"     // gray
	reset := "\x1b[0m"

	hline := strings.Repeat("─", boxW-2)

	// Top border
	fmt.Printf("%s╭%s╮%s\r\n", borderColor, hline, reset)

	// Title
	title := " \x1b[1mCalculator\x1b[22m"
	titlePad := boxW - 2 - printWidth(title)
	fmt.Printf("%s│%s%s%s%s│%s\r\n", borderColor, reset, title, strings.Repeat(" ", titlePad), borderColor, reset)

	// Separator
	fmt.Printf("%s├%s┤%s\r\n", borderColor, hline, reset)

	// Display area - show pending operation
	opStr := ""
	if c.op != 0 {
		opStr = fmt.Sprintf("%.10g %c", c.left, c.op)
		// Trim to fit
		if len(opStr) > boxW-4 {
			opStr = opStr[:boxW-4]
		}
	}
	opPad := boxW - 4 - len(opStr)
	if opPad < 0 {
		opPad = 0
	}
	fmt.Printf("%s│%s %s%s%s %s│%s\r\n", borderColor, displayBg, dimColor, strings.Repeat(" ", opPad)+opStr, reset+displayBg, reset+borderColor, reset)

	// Main display line
	dispText := c.display
	if c.err != "" {
		dispText = c.err
	}
	if len(dispText) > boxW-4 {
		dispText = dispText[len(dispText)-(boxW-4):]
	}
	dpad := boxW - 4 - len(dispText)
	if dpad < 0 {
		dpad = 0
	}
	fg := displayFg
	if c.err != "" {
		fg = errColor
	}
	fmt.Printf("%s│%s %s\x1b[1m%s%s%s %s│%s\r\n",
		borderColor, displayBg, fg, strings.Repeat(" ", dpad)+dispText, "\x1b[22m", reset+displayBg, reset+borderColor, reset)

	// Separator
	fmt.Printf("%s├%s┤%s\r\n", borderColor, hline, reset)

	// Button rows
	type btn struct {
		label string
		color string
	}
	rows := [][]btn{
		{{" C ", errColor}, {" % ", opColor}, {" ← ", opColor}, {" ÷ ", opColor}},
		{{" 7 ", btnColor}, {" 8 ", btnColor}, {" 9 ", btnColor}, {" × ", opColor}},
		{{" 4 ", btnColor}, {" 5 ", btnColor}, {" 6 ", btnColor}, {" - ", opColor}},
		{{" 1 ", btnColor}, {" 2 ", btnColor}, {" 3 ", btnColor}, {" + ", opColor}},
		{{" 0     ", btnColor}, {" . ", btnColor}, {" = ", accentColor}},
	}

	for _, row := range rows {
		line := ""
		for j, b := range row {
			if j > 0 {
				line += "  "
			}
			line += b.color + b.label + reset
		}
		// Pad to fit box
		pad := boxW - 4 - printWidth(line)
		if pad < 0 {
			pad = 0
		}
		fmt.Printf("%s│%s  %s%s  %s│%s\r\n", borderColor, reset, line, strings.Repeat(" ", pad), borderColor, reset)
	}

	// Footer
	fmt.Printf("%s├%s┤%s\r\n", borderColor, hline, reset)
	hint := dimColor + " q:quit  c:clear  ←:back" + reset
	hpad := boxW - 2 - printWidth(hint)
	if hpad < 0 {
		hpad = 0
	}
	fmt.Printf("%s│%s%s%s│%s\r\n", borderColor, hint, strings.Repeat(" ", hpad), borderColor, reset)

	// Bottom border
	fmt.Printf("%s╰%s╯%s\r\n", borderColor, hline, reset)
}

// printWidth returns the visible character width of a string, ignoring ANSI sequences.
func printWidth(s string) int {
	w := 0
	inEsc := false
	for _, r := range s {
		if r == '\x1b' {
			inEsc = true
			continue
		}
		if inEsc {
			if unicode.IsLetter(r) {
				inEsc = false
			}
			continue
		}
		w++
	}
	return w
}

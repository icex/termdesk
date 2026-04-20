package main

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"

	"github.com/charmbracelet/lipgloss"
	tea "charm.land/bubbletea/v2"
)

// calcState is the serializable calculator state for workspace persistence.
type calcState struct {
	Display string  `json:"d"`
	Result  string  `json:"r"`
	Op      byte    `json:"o"`
	Left    float64 `json:"l"`
	HasLeft bool    `json:"h"`
	NewNum  bool    `json:"n"`
}

// stateDumpMsg signals that SIGUSR1 was received and state should be dumped.
type stateDumpMsg struct{}

// listenStateDump waits for SIGUSR1 and returns a stateDumpMsg.
func listenStateDump() tea.Cmd {
	return func() tea.Msg {
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, syscall.SIGUSR1)
		<-sigCh
		signal.Stop(sigCh)
		return stateDumpMsg{}
	}
}

func main() {
	p := tea.NewProgram(newModel())
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

// Button types for styling.
type btnType int

const (
	btnDigit btnType = iota
	btnOp
	btnFunc
	btnEqual
	btnClear
)

type button struct {
	label string
	key   byte // keyboard shortcut (0 = none)
	kind  btnType
	wide  bool // double-width button
}

// Calculator button grid (5 rows x 4 cols, bottom-left "0" spans 2 cols).
var buttonGrid = [][]button{
	{{"C", 'c', btnClear, false}, {"%", '%', btnFunc, false}, {"\u2190", 127, btnFunc, false}, {"\u00f7", '/', btnOp, false}},
	{{"7", '7', btnDigit, false}, {"8", '8', btnDigit, false}, {"9", '9', btnDigit, false}, {"\u00d7", '*', btnOp, false}},
	{{"4", '4', btnDigit, false}, {"5", '5', btnDigit, false}, {"6", '6', btnDigit, false}, {"\u2212", '-', btnOp, false}},
	{{"1", '1', btnDigit, false}, {"2", '2', btnDigit, false}, {"3", '3', btnDigit, false}, {"+", '+', btnOp, false}},
	{{"0", '0', btnDigit, true}, {".", '.', btnDigit, false}, {"=", '=', btnEqual, false}},
}

const (
	btnW      = 7 // button width in chars
	btnH      = 2 // button height in lines
	gridCols  = 4
	gridRows  = 5
	padX      = 1 // horizontal padding inside frame
	padY      = 0 // vertical padding inside frame
	displayH  = 3 // display area height
)

type model struct {
	display  string
	result   string
	op       byte
	left     float64
	hasLeft  bool
	newNum   bool
	err      string
	width    int
	height   int
	hoverR   int // hovered button row (-1 = none)
	hoverC   int // hovered button col (-1 = none)
}

func newModel() model {
	m := model{
		display: "0",
		newNum:  true,
		hoverR:  -1,
		hoverC:  -1,
	}
	// Restore state from TERMDESK_APP_STATE env var if present (workspace restore).
	if envState := os.Getenv("TERMDESK_APP_STATE"); envState != "" {
		if decoded, err := base64.StdEncoding.DecodeString(envState); err == nil {
			var cs calcState
			if err := json.Unmarshal(decoded, &cs); err == nil {
				m.display = cs.Display
				m.result = cs.Result
				m.op = cs.Op
				m.left = cs.Left
				m.hasLeft = cs.HasLeft
				m.newNum = cs.NewNum
			}
		}
	}
	return m
}

func (m model) Init() tea.Cmd {
	return listenStateDump()
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case stateDumpMsg:
		// Serialize state and send via OSC 667 protocol.
		cs := calcState{
			Display: m.display,
			Result:  m.result,
			Op:      m.op,
			Left:    m.left,
			HasLeft: m.hasLeft,
			NewNum:  m.newNum,
		}
		if data, err := json.Marshal(cs); err == nil {
			encoded := base64.StdEncoding.EncodeToString(data)
			fmt.Fprintf(os.Stdout, "\x1b]667;state-response;%s\x07", encoded)
		}
		return m, listenStateDump() // re-listen for next request

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case tea.KeyPressMsg:
		k := tea.Key(msg)
		switch {
		case k.Code == tea.KeyEscape || k.Code == 'q' || k.Code == 'Q':
			return m, tea.Quit
		case k.Code == 3: // ctrl+c
			return m, tea.Quit
		case k.Code >= '0' && k.Code <= '9':
			m.handleKey(byte(k.Code))
		case k.Code == '.':
			m.handleKey('.')
		case k.Code == '+':
			m.handleKey('+')
		case k.Code == '-':
			m.handleKey('-')
		case k.Code == '*' || k.Code == 'x' || k.Code == 'X':
			m.handleKey('*')
		case k.Code == '/' || k.Code == '\\':
			m.handleKey('/')
		case k.Code == '%':
			m.handleKey('%')
		case k.Code == '=' || k.Code == tea.KeyEnter:
			m.handleKey('=')
		case k.Code == 'c' || k.Code == 'C':
			m.handleKey('c')
		case k.Code == tea.KeyBackspace:
			m.handleKey(127)
		}
		return m, nil

	case tea.MouseClickMsg:
		mouse := tea.Mouse(msg)
		if mouse.Button == tea.MouseLeft {
			if r, c, ok := m.hitButton(mouse.X, mouse.Y); ok {
				btn := gridButton(r, c)
				if btn != nil && btn.key != 0 {
					m.handleKey(btn.key)
				}
			}
		}
		return m, nil

	case tea.MouseMotionMsg:
		mouse := tea.Mouse(msg)
		if r, c, ok := m.hitButton(mouse.X, mouse.Y); ok {
			m.hoverR = r
			m.hoverC = c
		} else {
			m.hoverR = -1
			m.hoverC = -1
		}
		return m, nil
	}
	return m, nil
}

// gridButton returns the button at grid position (r, c), or nil.
func gridButton(r, c int) *button {
	if r < 0 || r >= len(buttonGrid) {
		return nil
	}
	row := buttonGrid[r]
	if c < 0 || c >= len(row) {
		return nil
	}
	return &row[c]
}

// hitButton checks if screen coords (mx, my) fall on a button.
func (m model) hitButton(mx, my int) (int, int, bool) {
	// Compute frame origin
	totalW := gridCols*btnW + (gridCols-1) + padX*2
	totalH := displayH + gridRows*btnH + (gridRows-1) + padY*2 + 2 // +2 for display border
	originX := (m.width - totalW) / 2
	originY := (m.height - totalH) / 2

	// Button grid starts below display
	gridStartY := originY + displayH + 2 + padY
	gridStartX := originX + padX

	// Relative position in grid area
	relX := mx - gridStartX
	relY := my - gridStartY
	if relX < 0 || relY < 0 {
		return -1, -1, false
	}

	// Determine row
	row := -1
	y := 0
	for r := 0; r < gridRows; r++ {
		if relY >= y && relY < y+btnH {
			row = r
			break
		}
		y += btnH + 1 // +1 for gap
	}
	if row < 0 {
		return -1, -1, false
	}

	// Determine col — handle wide "0" button
	rowBtns := buttonGrid[row]
	x := 0
	for c, btn := range rowBtns {
		w := btnW
		if btn.wide {
			w = btnW*2 + 1 // spans 2 columns
		}
		if relX >= x && relX < x+w {
			return row, c, true
		}
		x += w + 1 // +1 for gap
	}
	return -1, -1, false
}

func (m model) View() tea.View {
	var v tea.View
	v.AltScreen = true
	v.MouseMode = tea.MouseModeCellMotion

	if m.width == 0 || m.height == 0 {
		v.SetContent("Loading...")
		return v
	}

	// Colors
	frameBg := lipgloss.Color("#1E1E2E")
	displayBg := lipgloss.Color("#181825")
	displayFg := lipgloss.Color("#CDD6F4")
	dimFg := lipgloss.Color("#6C7086")
	accentFg := lipgloss.Color("#89B4FA")

	digitBg := lipgloss.Color("#313244")
	digitFg := lipgloss.Color("#CDD6F4")
	opBg := lipgloss.Color("#45475A")
	opFg := lipgloss.Color("#F9E2AF")
	funcBg := lipgloss.Color("#45475A")
	funcFg := lipgloss.Color("#A6ADC8")
	equalBg := lipgloss.Color("#89B4FA")
	equalFg := lipgloss.Color("#1E1E2E")
	clearBg := lipgloss.Color("#F38BA8")
	clearFg := lipgloss.Color("#1E1E2E")
	hoverBg := lipgloss.Color("#585B70")

	totalW := gridCols*btnW + (gridCols - 1) + padX*2

	// Title
	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(accentFg).
		Background(frameBg).
		Width(totalW).
		Align(lipgloss.Center)
	titleStr := titleStyle.Render("\uf1ec Calculator")

	// Display: pending operation line
	opStr := ""
	if m.op != 0 {
		opStr = fmt.Sprintf("%.10g %s", m.left, opSymbol(m.op))
	}
	opLineStyle := lipgloss.NewStyle().
		Foreground(dimFg).
		Background(displayBg).
		Width(totalW - 2).
		Align(lipgloss.Right).
		PaddingRight(1)
	opLine := opLineStyle.Render(opStr)

	// Display: main value
	dispText := m.display
	if m.err != "" {
		dispText = m.err
	}
	dispStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(displayFg).
		Background(displayBg).
		Width(totalW - 2).
		Align(lipgloss.Right).
		PaddingRight(1)
	if m.err != "" {
		dispStyle = dispStyle.Foreground(lipgloss.Color("#F38BA8"))
	}
	dispLine := dispStyle.Render(dispText)

	displayBox := lipgloss.NewStyle().
		Background(displayBg).
		Width(totalW).
		Padding(0, 1).
		Render(opLine + "\n" + dispLine)

	// Buttons
	var gridLines []string
	for r, row := range buttonGrid {
		var rowParts []string
		for c, btn := range row {
			w := btnW
			if btn.wide {
				w = btnW*2 + 1
			}

			bg, fg := digitBg, digitFg
			switch btn.kind {
			case btnOp:
				bg, fg = opBg, opFg
			case btnFunc:
				bg, fg = funcBg, funcFg
			case btnEqual:
				bg, fg = equalBg, equalFg
			case btnClear:
				bg, fg = clearBg, clearFg
			}

			if r == m.hoverR && c == m.hoverC {
				bg = hoverBg
			}

			style := lipgloss.NewStyle().
				Bold(true).
				Foreground(fg).
				Background(bg).
				Width(w).
				Height(btnH).
				Align(lipgloss.Center, lipgloss.Center)

			rowParts = append(rowParts, style.Render(btn.label))
		}
		gridLines = append(gridLines, lipgloss.JoinHorizontal(lipgloss.Top, interleave(rowParts, " ")...))
	}
	// 1-line gap between button rows (matches hitButton's y += btnH + 1)
	gridGap := lipgloss.NewStyle().Background(frameBg).Width(totalW).Height(1).Render("")
	grid := lipgloss.JoinVertical(lipgloss.Left, interleave(gridLines, gridGap)...)

	// Compose frame
	frameStyle := lipgloss.NewStyle().
		Background(frameBg).
		Padding(0, padX).
		BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("#585B70"))

	content := lipgloss.JoinVertical(lipgloss.Left,
		titleStr,
		displayBox,
		"",
		grid,
	)

	frame := frameStyle.Render(content)

	// Center on screen
	centered := lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, frame,
		lipgloss.WithWhitespaceBackground(lipgloss.Color("#11111B")))

	v.SetContent(centered)
	return v
}

// interleave inserts separator strings between elements for JoinHorizontal/Vertical.
func interleave(parts []string, sep string) []string {
	if len(parts) == 0 || sep == "" {
		return parts
	}
	result := make([]string, 0, len(parts)*2-1)
	for i, p := range parts {
		if i > 0 {
			result = append(result, sep)
		}
		result = append(result, p)
	}
	return result
}

func opSymbol(op byte) string {
	switch op {
	case '+':
		return "+"
	case '-':
		return "\u2212"
	case '*':
		return "\u00d7"
	case '/':
		return "\u00f7"
	}
	return string(op)
}

// --- Calculator logic ---

func (m *model) handleKey(b byte) {
	m.err = ""
	switch {
	case b >= '0' && b <= '9':
		m.appendDigit(b)
	case b == '.':
		m.appendDot()
	case b == '+', b == '-', b == '*', b == '/':
		m.applyOp(b)
	case b == '=' || b == '\r' || b == '\n':
		m.evaluate()
	case b == 'c' || b == 'C':
		m.clear()
	case b == 127 || b == 8:
		m.backspace()
	case b == '%':
		m.percent()
	}
}

func (m *model) appendDigit(b byte) {
	if m.newNum {
		m.display = string(b)
		m.newNum = false
	} else {
		if m.display == "0" {
			m.display = string(b)
		} else {
			m.display += string(b)
		}
	}
}

func (m *model) appendDot() {
	if m.newNum {
		m.display = "0."
		m.newNum = false
	} else if !strings.Contains(m.display, ".") {
		m.display += "."
	}
}

func (m *model) applyOp(op byte) {
	if m.hasLeft && !m.newNum {
		m.evaluate()
	}
	val, err := strconv.ParseFloat(m.display, 64)
	if err != nil {
		m.err = "Error"
		return
	}
	m.left = val
	m.hasLeft = true
	m.op = op
	m.newNum = true
}

func (m *model) evaluate() {
	if !m.hasLeft || m.op == 0 {
		return
	}
	right, err := strconv.ParseFloat(m.display, 64)
	if err != nil {
		m.err = "Error"
		return
	}
	var res float64
	switch m.op {
	case '+':
		res = m.left + right
	case '-':
		res = m.left - right
	case '*':
		res = m.left * right
	case '/':
		if right == 0 {
			m.err = "Div by 0"
			m.hasLeft = false
			m.op = 0
			return
		}
		res = m.left / right
	}
	m.display = formatNum(res)
	m.result = m.display
	m.hasLeft = false
	m.op = 0
	m.newNum = true
}

func (m *model) clear() {
	m.display = "0"
	m.result = ""
	m.op = 0
	m.left = 0
	m.hasLeft = false
	m.newNum = true
	m.err = ""
}

func (m *model) backspace() {
	if m.newNum {
		return
	}
	if len(m.display) > 1 {
		m.display = m.display[:len(m.display)-1]
	} else {
		m.display = "0"
		m.newNum = true
	}
}

func (m *model) percent() {
	val, err := strconv.ParseFloat(m.display, 64)
	if err != nil {
		return
	}
	m.display = formatNum(val / 100)
	m.newNum = true
}

func formatNum(f float64) string {
	if math.IsInf(f, 0) || math.IsNaN(f) {
		return "Error"
	}
	if f == math.Trunc(f) && math.Abs(f) < 1e15 {
		return strconv.FormatFloat(f, 'f', 0, 64)
	}
	s := strconv.FormatFloat(f, 'f', 10, 64)
	s = strings.TrimRight(s, "0")
	s = strings.TrimRight(s, ".")
	return s
}

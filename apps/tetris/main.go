package main

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"math"
	"math/rand/v2"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/charmbracelet/colorprofile"
	"github.com/charmbracelet/lipgloss"
	tea "charm.land/bubbletea/v2"
)

func main() {
	p := tea.NewProgram(newModel(), tea.WithColorProfile(colorprofile.TrueColor))
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

// ── Board dimensions ────────────────────────────────────────────────

const (
	boardW = 10
	boardH = 20
	cellW  = 2 // each cell is 2 chars wide ("██")
)

// ── Piece types ─────────────────────────────────────────────────────

type pieceType int

const (
	pieceI pieceType = iota
	pieceO
	pieceT
	pieceS
	pieceZ
	pieceJ
	pieceL
	pieceCount // 7
)

// shape is a set of (row, col) offsets relative to pivot.
type shape [][2]int

// pieces defines rotation state 0 for each piece.
var pieces = [pieceCount]shape{
	// I
	{{0, 0}, {0, 1}, {0, 2}, {0, 3}},
	// O
	{{0, 0}, {0, 1}, {1, 0}, {1, 1}},
	// T
	{{0, 1}, {1, 0}, {1, 1}, {1, 2}},
	// S
	{{0, 1}, {0, 2}, {1, 0}, {1, 1}},
	// Z
	{{0, 0}, {0, 1}, {1, 1}, {1, 2}},
	// J
	{{0, 0}, {1, 0}, {1, 1}, {1, 2}},
	// L
	{{0, 2}, {1, 0}, {1, 1}, {1, 2}},
}

// rotations[piece][rotation] — precomputed 4 rotation states.
var rotations [pieceCount][4]shape

func init() {
	for p := pieceType(0); p < pieceCount; p++ {
		rotations[p][0] = pieces[p]
		for r := 1; r < 4; r++ {
			rotations[p][r] = rotateCW(rotations[p][r-1])
		}
	}
}

// rotateCW rotates a shape 90° clockwise around its bounding box center.
func rotateCW(s shape) shape {
	// Find bounding box.
	maxR, maxC := 0, 0
	for _, b := range s {
		if b[0] > maxR {
			maxR = b[0]
		}
		if b[1] > maxC {
			maxC = b[1]
		}
	}
	// Transpose + reverse rows → CW rotation.
	out := make(shape, len(s))
	for i, b := range s {
		out[i] = [2]int{b[1], maxR - b[0]}
	}
	return out
}

// ── Piece colors (Catppuccin Mocha) ─────────────────────────────────

var pieceColors = [pieceCount]lipgloss.Color{
	"#89B4FA", // I — blue
	"#F9E2AF", // O — yellow
	"#CBA6F7", // T — mauve
	"#A6E3A1", // S — green
	"#F38BA8", // Z — red
	"#89DCEB", // J — sky
	"#FAB387", // L — peach
}

// ── SRS wall kick data ──────────────────────────────────────────────

// kicksJLSTZ[fromRot][toRot] = list of (row, col) offsets to try.
var kicksJLSTZ = map[[2]int][][2]int{
	{0, 1}: {{0, 0}, {0, -1}, {-1, -1}, {2, 0}, {2, -1}},
	{1, 0}: {{0, 0}, {0, 1}, {1, 1}, {-2, 0}, {-2, 1}},
	{1, 2}: {{0, 0}, {0, 1}, {1, 1}, {-2, 0}, {-2, 1}},
	{2, 1}: {{0, 0}, {0, -1}, {-1, -1}, {2, 0}, {2, -1}},
	{2, 3}: {{0, 0}, {0, 1}, {-1, 1}, {2, 0}, {2, 1}},
	{3, 2}: {{0, 0}, {0, -1}, {1, -1}, {-2, 0}, {-2, -1}},
	{3, 0}: {{0, 0}, {0, -1}, {1, -1}, {-2, 0}, {-2, -1}},
	{0, 3}: {{0, 0}, {0, 1}, {-1, 1}, {2, 0}, {2, 1}},
}

var kicksI = map[[2]int][][2]int{
	{0, 1}: {{0, 0}, {0, -2}, {0, 1}, {1, -2}, {-2, 1}},
	{1, 0}: {{0, 0}, {0, 2}, {0, -1}, {-1, 2}, {2, -1}},
	{1, 2}: {{0, 0}, {0, -1}, {0, 2}, {-2, -1}, {1, 2}},
	{2, 1}: {{0, 0}, {0, 1}, {0, -2}, {2, 1}, {-1, -2}},
	{2, 3}: {{0, 0}, {0, 2}, {0, -1}, {1, 2}, {-2, -1}},
	{3, 2}: {{0, 0}, {0, -2}, {0, 1}, {-1, -2}, {2, 1}},
	{3, 0}: {{0, 0}, {0, 1}, {0, -2}, {-2, 1}, {1, -2}},
	{0, 3}: {{0, 0}, {0, -1}, {0, 2}, {2, -1}, {-1, 2}},
}

// ── State persistence ────────────────────────────────────────────────

// Serializable state for workspace persistence.
type boardCell struct {
	F bool `json:"f"` // filled
	P int  `json:"p"` // piece type
}

type tetrisState struct {
	Board    [boardH][boardW]boardCell `json:"b"`
	Cur      int                       `json:"c"`
	CurRot   int                       `json:"cr"`
	CurRow   int                       `json:"crw"`
	CurCol   int                       `json:"ccl"`
	Bag      []int                     `json:"bg"`
	Next     [3]int                    `json:"n"`
	Hold     int                       `json:"h"`
	HasHold  bool                      `json:"hh"`
	CanHold  bool                      `json:"ch"`
	Score    int                       `json:"s"`
	Lines    int                       `json:"l"`
	Level    int                       `json:"lv"`
	Combos   int                       `json:"co"`
	SpeedMs  int64                     `json:"sp"`
	Gen      int                       `json:"g"`
	GameOver bool                      `json:"go"`
	Paused   bool                      `json:"p"`
	Started  bool                      `json:"st"`
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

// ── Tick message ────────────────────────────────────────────────────

type tickMsg struct{ gen int }

func scheduleTick(d time.Duration, gen int) tea.Cmd {
	return tea.Tick(d, func(time.Time) tea.Msg {
		return tickMsg{gen: gen}
	})
}

// ── Board cell ──────────────────────────────────────────────────────

type cell struct {
	filled bool
	piece  pieceType // which piece left this cell
}

// ── Model ───────────────────────────────────────────────────────────

type model struct {
	board [boardH][boardW]cell

	// Current piece.
	cur    pieceType
	curRot int
	curRow int // top-left of bounding box (can be negative)
	curCol int

	// Next queue + bag.
	bag  []pieceType
	next [3]pieceType

	// Hold.
	hold    pieceType
	hasHold bool
	canHold bool

	// Scoring.
	score  int
	lines  int
	level  int
	combos int // consecutive line-clearing ticks

	// Timing.
	speed time.Duration
	gen   int

	// State.
	gameOver bool
	paused   bool
	started  bool

	// Terminal size.
	width  int
	height int

	rng *rand.Rand
}

func newModel() model {
	r := rand.New(rand.NewPCG(uint64(time.Now().UnixNano()), 0))
	m := model{
		level:   1,
		canHold: true,
		hold:    -1,
		rng:     r,
	}
	m.speed = fallSpeed(m.level)
	m.fillBag()
	// Fill next queue.
	for i := range m.next {
		m.next[i] = m.drawPiece()
	}
	m.spawnPiece()
	// Restore state from TERMDESK_APP_STATE env var if present (workspace restore).
	if envState := os.Getenv("TERMDESK_APP_STATE"); envState != "" {
		if decoded, err := base64.StdEncoding.DecodeString(envState); err == nil {
			var ts tetrisState
			if err := json.Unmarshal(decoded, &ts); err == nil {
				for r := range boardH {
					for c := range boardW {
						m.board[r][c] = cell{filled: ts.Board[r][c].F, piece: pieceType(ts.Board[r][c].P)}
					}
				}
				m.cur = pieceType(ts.Cur)
				m.curRot = ts.CurRot
				m.curRow = ts.CurRow
				m.curCol = ts.CurCol
				m.bag = make([]pieceType, len(ts.Bag))
				for i, p := range ts.Bag {
					m.bag[i] = pieceType(p)
				}
				for i, p := range ts.Next {
					m.next[i] = pieceType(p)
				}
				m.hold = pieceType(ts.Hold)
				m.hasHold = ts.HasHold
				m.canHold = ts.CanHold
				m.score = ts.Score
				m.lines = ts.Lines
				m.level = ts.Level
				m.combos = ts.Combos
				m.speed = time.Duration(ts.SpeedMs) * time.Millisecond
				m.gen = ts.Gen
				m.gameOver = ts.GameOver
				m.paused = ts.Paused
				m.started = ts.Started
			}
		}
	}
	return m
}

// fillBag creates a shuffled bag of all 7 pieces.
func (m *model) fillBag() {
	m.bag = make([]pieceType, pieceCount)
	for i := range m.bag {
		m.bag[i] = pieceType(i)
	}
	m.rng.Shuffle(len(m.bag), func(i, j int) {
		m.bag[i], m.bag[j] = m.bag[j], m.bag[i]
	})
}

// drawPiece takes the next piece from the bag, refilling if empty.
func (m *model) drawPiece() pieceType {
	if len(m.bag) == 0 {
		m.fillBag()
	}
	p := m.bag[0]
	m.bag = m.bag[1:]
	return p
}

// spawnPiece places the next piece at the top of the board.
func (m *model) spawnPiece() {
	m.cur = m.next[0]
	copy(m.next[:], m.next[1:])
	m.next[2] = m.drawPiece()
	m.curRot = 0
	m.curRow = 0 // start at top of visible area
	m.curCol = (boardW - 4) / 2
	m.canHold = true

	// Check if spawn position is blocked.
	if !m.validPosition(m.cur, m.curRot, m.curRow, m.curCol) {
		m.gameOver = true
	}
}

// fallSpeed returns the drop interval for a given level (Tetris guideline).
func fallSpeed(level int) time.Duration {
	seconds := math.Pow(0.8-float64(level-1)*0.007, float64(level-1))
	ms := int(seconds * 1000)
	if ms < 50 {
		ms = 50
	}
	return time.Duration(ms) * time.Millisecond
}

// ── Collision detection ─────────────────────────────────────────────

func (m *model) validPosition(p pieceType, rot, row, col int) bool {
	for _, b := range rotations[p][rot] {
		r, c := row+b[0], col+b[1]
		if c < 0 || c >= boardW || r >= boardH {
			return false
		}
		if r >= 0 && m.board[r][c].filled {
			return false
		}
	}
	return true
}

// ── Piece operations ────────────────────────────────────────────────

func (m *model) moveLeft() bool {
	if m.validPosition(m.cur, m.curRot, m.curRow, m.curCol-1) {
		m.curCol--
		return true
	}
	return false
}

func (m *model) moveRight() bool {
	if m.validPosition(m.cur, m.curRot, m.curRow, m.curCol+1) {
		m.curCol++
		return true
	}
	return false
}

func (m *model) moveDown() bool {
	if m.validPosition(m.cur, m.curRot, m.curRow+1, m.curCol) {
		m.curRow++
		return true
	}
	return false
}

func (m *model) rotateCW() bool {
	newRot := (m.curRot + 1) % 4
	return m.tryRotate(m.curRot, newRot)
}

func (m *model) rotateCCW() bool {
	newRot := (m.curRot + 3) % 4
	return m.tryRotate(m.curRot, newRot)
}

func (m *model) tryRotate(from, to int) bool {
	kicks := kicksJLSTZ
	if m.cur == pieceI {
		kicks = kicksI
	}
	if m.cur == pieceO {
		// O doesn't rotate.
		return false
	}
	offsets := kicks[[2]int{from, to}]
	for _, off := range offsets {
		nr, nc := m.curRow+off[0], m.curCol+off[1]
		if m.validPosition(m.cur, to, nr, nc) {
			m.curRow = nr
			m.curCol = nc
			m.curRot = to
			return true
		}
	}
	return false
}

func (m *model) hardDrop() int {
	dropped := 0
	for m.validPosition(m.cur, m.curRot, m.curRow+1, m.curCol) {
		m.curRow++
		dropped++
	}
	return dropped
}

// ghostRow returns the row where the ghost piece would land.
func (m *model) ghostRow() int {
	r := m.curRow
	for m.validPosition(m.cur, m.curRot, r+1, m.curCol) {
		r++
	}
	return r
}

func (m *model) holdPiece() {
	if !m.canHold {
		return
	}
	if !m.hasHold {
		m.hasHold = true
		m.hold = m.cur
		m.spawnPiece()
	} else {
		m.hold, m.cur = m.cur, m.hold
		m.curRot = 0
		m.curRow = 0
		m.curCol = (boardW - 4) / 2
	}
	m.canHold = false
}

// ── Lock + line clear ───────────────────────────────────────────────

func (m *model) lockPiece() {
	// Invalidate old ticks — hard drop schedules a new tick, so the old
	// game-loop tick must be killed to prevent parallel tick chains.
	m.gen++

	lockedAbove := false
	for _, b := range rotations[m.cur][m.curRot] {
		r, c := m.curRow+b[0], m.curCol+b[1]
		if r < 0 {
			lockedAbove = true
			continue
		}
		if r < boardH && c >= 0 && c < boardW {
			m.board[r][c] = cell{filled: true, piece: m.cur}
		}
	}
	if lockedAbove {
		m.gameOver = true
		return
	}
	cleared := m.clearLines()
	if cleared > 0 {
		m.combos++
		// Standard scoring.
		base := [5]int{0, 100, 300, 500, 800}
		if cleared > 4 {
			cleared = 4
		}
		m.score += base[cleared] * m.level
		// Combo bonus.
		if m.combos > 1 {
			m.score += 50 * (m.combos - 1) * m.level
		}
		m.lines += cleared
		// Level up every 10 lines.
		newLevel := m.lines/10 + 1
		if newLevel > m.level {
			m.level = newLevel
			m.speed = fallSpeed(m.level)
		}
	} else {
		m.combos = 0
	}
	m.spawnPiece()
}

func (m *model) clearLines() int {
	cleared := 0
	for r := boardH - 1; r >= 0; r-- {
		full := true
		for c := 0; c < boardW; c++ {
			if !m.board[r][c].filled {
				full = false
				break
			}
		}
		if full {
			cleared++
			// Shift everything above down.
			for rr := r; rr > 0; rr-- {
				m.board[rr] = m.board[rr-1]
			}
			m.board[0] = [boardW]cell{}
			r++ // re-check this row (it now has the row above)
		}
	}
	return cleared
}

// ── Reset ───────────────────────────────────────────────────────────

func (m *model) reset() {
	m.board = [boardH][boardW]cell{}
	m.score = 0
	m.lines = 0
	m.level = 1
	m.combos = 0
	m.speed = fallSpeed(1)
	m.gameOver = false
	m.paused = false
	m.started = false
	m.hold = -1
	m.hasHold = false
	m.canHold = true
	m.gen++
	m.fillBag()
	for i := range m.next {
		m.next[i] = m.drawPiece()
	}
	m.spawnPiece()
}

// ── Bubble Tea interface ────────────────────────────────────────────

func (m model) Init() tea.Cmd {
	cmds := []tea.Cmd{listenStateDump()}
	if m.started && !m.gameOver && !m.paused {
		cmds = append(cmds, scheduleTick(m.speed, m.gen))
	}
	return tea.Batch(cmds...)
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case stateDumpMsg:
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
		if data, err := json.Marshal(ts); err == nil {
			encoded := base64.StdEncoding.EncodeToString(data)
			fmt.Fprintf(os.Stdout, "\x1b]667;state-response;%s\x07", encoded)
		}
		return m, listenStateDump()

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case tea.KeyPressMsg:
		k := tea.Key(msg)
		switch {
		case k.Code == 'q' || k.Code == 'Q' || k.Code == 3: // q or Ctrl+C
			return m, tea.Quit

		case k.Code == 'r' || k.Code == 'R':
			if m.gameOver {
				m.reset()
				return m, nil
			}

		case k.Code == 'p' || k.Code == 'P':
			if m.started && !m.gameOver {
				m.paused = !m.paused
				if !m.paused {
					return m, scheduleTick(m.speed, m.gen)
				}
			}
			return m, nil
		}

		if m.gameOver || m.paused {
			return m, nil
		}

		startGame := func() tea.Cmd {
			if !m.started {
				m.started = true
				return scheduleTick(m.speed, m.gen)
			}
			return nil
		}

		switch {
		case k.Code == tea.KeyLeft || k.Code == 'h' || k.Code == 'H':
			m.moveLeft()
			return m, startGame()

		case k.Code == tea.KeyRight || k.Code == 'l' || k.Code == 'L':
			m.moveRight()
			return m, startGame()

		case k.Code == tea.KeyDown || k.Code == 'j' || k.Code == 'J':
			if m.moveDown() {
				m.score++ // soft drop bonus
			}
			return m, startGame()

		case k.Code == tea.KeyUp || k.Code == 'w' || k.Code == 'W':
			m.rotateCW()
			return m, startGame()

		case k.Code == 'z' || k.Code == 'Z':
			m.rotateCCW()
			return m, startGame()

		case k.Code == ' ': // space — hard drop
			dropped := m.hardDrop()
			m.score += dropped * 2
			m.lockPiece()
			if !m.started {
				m.started = true
			}
			if m.gameOver {
				return m, nil
			}
			return m, scheduleTick(m.speed, m.gen)

		case k.Code == 'c' || k.Code == 'C' || k.Code == tea.KeyTab:
			m.holdPiece()
			return m, startGame()
		}
		return m, nil

	case tickMsg:
		if msg.gen != m.gen || m.gameOver || m.paused || !m.started {
			return m, nil
		}
		if !m.moveDown() {
			m.lockPiece()
			if m.gameOver {
				return m, nil
			}
		}
		return m, scheduleTick(m.speed, m.gen)
	}
	return m, nil
}

// ── View ────────────────────────────────────────────────────────────

func (m model) View() tea.View {
	var v tea.View
	v.AltScreen = true

	if m.width == 0 || m.height == 0 {
		v.SetContent("Loading...")
		return v
	}

	// Color palette.
	bgColor := lipgloss.Color("#11111B")
	frameBg := lipgloss.Color("#1E1E2E")
	borderFg := lipgloss.Color("#585B70")
	textFg := lipgloss.Color("#CDD6F4")
	dimFg := lipgloss.Color("#6C7086")
	ghostFg := lipgloss.Color("#45475A")
	gameOverFg := lipgloss.Color("#F38BA8")
	pauseFg := lipgloss.Color("#F9E2AF")

	// Styles.
	borderSt := lipgloss.NewStyle().Foreground(borderFg).Background(frameBg)
	emptySt := lipgloss.NewStyle().Foreground(frameBg).Background(frameBg)
	ghostSt := lipgloss.NewStyle().Foreground(ghostFg).Background(frameBg)
	textSt := lipgloss.NewStyle().Foreground(textFg).Background(frameBg)
	dimSt := lipgloss.NewStyle().Foreground(dimFg).Background(frameBg)
	boldSt := lipgloss.NewStyle().Foreground(textFg).Background(frameBg).Bold(true)

	// Precompute current piece cell set (for board rendering).
	type pos struct{ r, c int }
	curCells := make(map[pos]bool)
	for _, b := range rotations[m.cur][m.curRot] {
		curCells[pos{m.curRow + b[0], m.curCol + b[1]}] = true
	}
	// Ghost cells.
	gr := m.ghostRow()
	ghostCells := make(map[pos]bool)
	if gr != m.curRow {
		for _, b := range rotations[m.cur][m.curRot] {
			p := pos{gr + b[0], m.curCol + b[1]}
			if !curCells[p] {
				ghostCells[p] = true
			}
		}
	}

	// Pre-render cell strings (avoids style allocation in hot loop).
	var pieceStrs [pieceCount]string
	for i := pieceType(0); i < pieceCount; i++ {
		pieceStrs[i] = lipgloss.NewStyle().Foreground(pieceColors[i]).Background(frameBg).Render("██")
	}
	ghostStr := ghostSt.Render("░░")
	emptyStr := emptySt.Render("  ")
	pipeStr := borderSt.Render("│")

	// ── Build the playfield ──
	var sb strings.Builder
	sb.Grow(4096)

	// Title.
	sb.WriteString(boldSt.Render("  TETRIS"))
	sb.WriteByte('\n')

	// Construct side panels and board row by row.
	hFill := strings.Repeat("─", boardW*cellW)
	topBorder := borderSt.Render("┌" + hFill + "┐")
	botBorder := borderSt.Render("└" + hFill + "┘")

	// Pre-render side panels.
	leftPanel := m.renderLeftPanel(textSt, dimSt, boldSt, borderSt, emptySt, frameBg)
	rightPanel := m.renderRightPanel(textSt, dimSt, boldSt, borderSt, emptySt, frameBg)

	// Each panel has boardH lines (padded).
	lp := padLines(leftPanel, boardH+2, panelW)
	rp := padLines(rightPanel, boardH+2, panelW)

	// Row 0: top border.
	sb.WriteString(lp[0])
	sb.WriteString(topBorder)
	sb.WriteString(rp[0])
	sb.WriteByte('\n')

	// Board rows.
	for r := 0; r < boardH; r++ {
		sb.WriteString(lp[r+1])
		sb.WriteString(pipeStr)
		for c := 0; c < boardW; c++ {
			p := pos{r, c}
			if curCells[p] {
				sb.WriteString(pieceStrs[m.cur])
			} else if ghostCells[p] {
				sb.WriteString(ghostStr)
			} else if m.board[r][c].filled {
				sb.WriteString(pieceStrs[m.board[r][c].piece])
			} else {
				sb.WriteString(emptyStr)
			}
		}
		sb.WriteString(pipeStr)
		sb.WriteString(rp[r+1])
		sb.WriteByte('\n')
	}

	// Bottom border.
	sb.WriteString(lp[boardH+1])
	sb.WriteString(botBorder)
	sb.WriteString(rp[boardH+1])
	sb.WriteByte('\n')

	// Status line.
	if !m.started && !m.gameOver {
		sb.WriteString(textSt.Render("  Press any arrow key to start"))
	} else if m.paused {
		pst := lipgloss.NewStyle().Foreground(pauseFg).Background(frameBg).Bold(true)
		sb.WriteString(pst.Render("  PAUSED — press 'p' to resume"))
	} else if m.gameOver {
		gst := lipgloss.NewStyle().Foreground(gameOverFg).Background(frameBg).Bold(true)
		sb.WriteString(gst.Render(fmt.Sprintf("  GAME OVER — Score: %d", m.score)))
		sb.WriteByte('\n')
		sb.WriteString(dimSt.Render("  Press 'r' to restart"))
	} else {
		sb.WriteString(dimSt.Render(fmt.Sprintf("  Score: %d  Level: %d  Lines: %d", m.score, m.level, m.lines)))
	}
	sb.WriteByte('\n')

	// Footer.
	sb.WriteString(dimSt.Render("  ←→:move  ↓:soft  ↑:rotate  z:CCW"))
	sb.WriteByte('\n')
	sb.WriteString(dimSt.Render("  space:drop  c:hold  p:pause  q:quit"))

	content := sb.String()

	centered := lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, content,
		lipgloss.WithWhitespaceBackground(bgColor))

	v.SetContent(centered)
	return v
}

// ── Side panel renderers ────────────────────────────────────────────

const panelW = 12 // visual width of each side panel

func (m model) renderLeftPanel(textSt, dimSt, boldSt, borderSt, emptySt lipgloss.Style, bg lipgloss.Color) []string {
	pl := func(st lipgloss.Style, text string) string {
		return st.Render(fmt.Sprintf("%-*s", panelW, text))
	}
	blank := pl(emptySt, "")
	var lines []string

	// Hold piece.
	lines = append(lines, pl(boldSt, "  HOLD"))
	if m.hasHold {
		lines = append(lines, renderMiniPieceLines(m.hold, 0, bg)...)
	} else {
		lines = append(lines, pl(dimSt, "  ----"))
		lines = append(lines, blank)
	}
	lines = append(lines, blank)

	// Info.
	lines = append(lines, pl(boldSt, "  SCORE"))
	lines = append(lines, pl(textSt, fmt.Sprintf("  %d", m.score)))
	lines = append(lines, blank)
	lines = append(lines, pl(boldSt, "  LEVEL"))
	lines = append(lines, pl(textSt, fmt.Sprintf("  %d", m.level)))
	lines = append(lines, blank)
	lines = append(lines, pl(boldSt, "  LINES"))
	lines = append(lines, pl(textSt, fmt.Sprintf("  %d", m.lines)))

	return lines
}

func (m model) renderRightPanel(textSt, dimSt, boldSt, borderSt, emptySt lipgloss.Style, bg lipgloss.Color) []string {
	pl := func(st lipgloss.Style, text string) string {
		return st.Render(fmt.Sprintf("%-*s", panelW, text))
	}
	blank := pl(emptySt, "")
	var lines []string

	lines = append(lines, pl(boldSt, "  NEXT"))
	for i := 0; i < 3; i++ {
		lines = append(lines, renderMiniPieceLines(m.next[i], 0, bg)...)
		lines = append(lines, blank)
	}

	return lines
}

// renderMiniPieceLines renders a piece in a 4×2 grid, padded to panelW.
func renderMiniPieceLines(p pieceType, rot int, bg lipgloss.Color) []string {
	padSt := lipgloss.NewStyle().Foreground(bg).Background(bg)
	if p < 0 || p >= pieceCount {
		pad := padSt.Render(strings.Repeat(" ", panelW))
		return []string{pad, pad}
	}
	grid := [2][4]bool{}
	for _, b := range rotations[p][rot] {
		r, c := b[0], b[1]
		if r < 2 && c < 4 {
			grid[r][c] = true
		}
	}
	st := lipgloss.NewStyle().Foreground(pieceColors[p]).Background(bg)
	eSt := padSt
	// Piece is 4 cells × 2 chars = 8 chars. Panel is 12. Pad: 2 left + 8 piece + 2 right.
	lPad := padSt.Render("  ")
	rPad := padSt.Render("  ")
	var lines []string
	for r := 0; r < 2; r++ {
		var row strings.Builder
		row.WriteString(lPad)
		for c := 0; c < 4; c++ {
			if grid[r][c] {
				row.WriteString(st.Render("██"))
			} else {
				row.WriteString(eSt.Render("  "))
			}
		}
		row.WriteString(rPad)
		lines = append(lines, row.String())
	}
	return lines
}

// padLines ensures a slice has exactly n entries, filling missing with styled blanks.
func padLines(lines []string, n, width int) []string {
	bg := lipgloss.Color("#1E1E2E")
	padSt := lipgloss.NewStyle().Foreground(bg).Background(bg)
	pad := padSt.Render(strings.Repeat(" ", width))
	result := make([]string, n)
	for i := 0; i < n; i++ {
		if i < len(lines) {
			result[i] = lines[i]
		} else {
			result[i] = pad
		}
	}
	return result
}

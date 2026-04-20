package main

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"math/rand"
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

// Game constants.
const (
	gridW = 9
	gridH = 9
	mines = 10
)

// Cell states.
const (
	cellHidden   = 0
	cellRevealed = 1
	cellFlagged  = 2
)

// Game states.
const (
	statePlaying = 0
	stateWon     = 1
	stateLost    = 2
)

// Cell display dimensions.
const (
	cellW = 3
	cellH = 2
)

type cell struct {
	mine    bool
	state   int // cellHidden, cellRevealed, cellFlagged
	adjacent int
}

// Serializable state for workspace persistence.
type cellData struct {
	M bool `json:"m"` // mine
	S int  `json:"s"` // state (hidden/revealed/flagged)
	A int  `json:"a"` // adjacent count
}

type mineState struct {
	Grid       [gridH][gridW]cellData `json:"g"`
	State      int                    `json:"st"`
	CursorX    int                    `json:"cx"`
	CursorY    int                    `json:"cy"`
	Flags      int                    `json:"f"`
	FirstClick bool                   `json:"fc"`
	Elapsed    int                    `json:"e"`
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

type tickMsg time.Time

type model struct {
	grid      [gridH][gridW]cell
	gameState int
	cursorX   int
	cursorY   int
	flags     int
	width     int
	height    int
	firstClick bool // true = first click hasn't happened yet
	startTime  time.Time
	elapsed    int // seconds elapsed
	rng        *rand.Rand
}

func newModel() model {
	m := newModelWithSeed(time.Now().UnixNano())
	// Restore state from TERMDESK_APP_STATE env var if present (workspace restore).
	if envState := os.Getenv("TERMDESK_APP_STATE"); envState != "" {
		if decoded, err := base64.StdEncoding.DecodeString(envState); err == nil {
			var ms mineState
			if err := json.Unmarshal(decoded, &ms); err == nil {
				for y := range gridH {
					for x := range gridW {
						m.grid[y][x] = cell{
							mine:     ms.Grid[y][x].M,
							state:    ms.Grid[y][x].S,
							adjacent: ms.Grid[y][x].A,
						}
					}
				}
				m.gameState = ms.State
				m.cursorX = ms.CursorX
				m.cursorY = ms.CursorY
				m.flags = ms.Flags
				m.firstClick = ms.FirstClick
				m.elapsed = ms.Elapsed
				if !m.firstClick && m.gameState == statePlaying {
					m.startTime = time.Now().Add(-time.Duration(ms.Elapsed) * time.Second)
				}
			}
		}
	}
	return m
}

func newModelWithSeed(seed int64) model {
	m := model{
		cursorX:    gridW / 2,
		cursorY:    gridH / 2,
		firstClick: true,
		rng:        rand.New(rand.NewSource(seed)),
	}
	m.placeMines(-1, -1)
	return m
}

// placeMines randomly places mines, avoiding safeX/safeY (use -1,-1 for initial placement).
func (m *model) placeMines(safeX, safeY int) {
	// Clear grid.
	for y := range gridH {
		for x := range gridW {
			m.grid[y][x] = cell{}
		}
	}

	// Place mines randomly.
	placed := 0
	for placed < mines {
		x := m.rng.Intn(gridW)
		y := m.rng.Intn(gridH)
		if m.grid[y][x].mine {
			continue
		}
		if x == safeX && y == safeY {
			continue
		}
		m.grid[y][x].mine = true
		placed++
	}

	// Compute adjacency counts.
	m.computeAdjacent()
}

func (m *model) computeAdjacent() {
	for y := range gridH {
		for x := range gridW {
			if m.grid[y][x].mine {
				continue
			}
			count := 0
			for dy := -1; dy <= 1; dy++ {
				for dx := -1; dx <= 1; dx++ {
					if dx == 0 && dy == 0 {
						continue
					}
					nx, ny := x+dx, y+dy
					if nx >= 0 && nx < gridW && ny >= 0 && ny < gridH && m.grid[ny][nx].mine {
						count++
					}
				}
			}
			m.grid[y][x].adjacent = count
		}
	}
}

// reveal reveals a cell and flood-fills if it's empty (adjacent == 0).
func (m *model) reveal(x, y int) {
	if x < 0 || x >= gridW || y < 0 || y >= gridH {
		return
	}
	c := &m.grid[y][x]
	if c.state != cellHidden {
		return
	}
	c.state = cellRevealed

	if c.mine {
		m.gameState = stateLost
		m.revealAllMines()
		return
	}

	// Flood fill if empty.
	if c.adjacent == 0 {
		for dy := -1; dy <= 1; dy++ {
			for dx := -1; dx <= 1; dx++ {
				if dx == 0 && dy == 0 {
					continue
				}
				m.reveal(x+dx, y+dy)
			}
		}
	}
}

func (m *model) revealAllMines() {
	for y := range gridH {
		for x := range gridW {
			if m.grid[y][x].mine {
				m.grid[y][x].state = cellRevealed
			}
		}
	}
}

// toggleFlag toggles flag on a hidden cell.
func (m *model) toggleFlag(x, y int) {
	c := &m.grid[y][x]
	switch c.state {
	case cellHidden:
		c.state = cellFlagged
		m.flags++
	case cellFlagged:
		c.state = cellHidden
		m.flags--
	}
}

// checkWin returns true if all non-mine cells are revealed.
func (m *model) checkWin() bool {
	for y := range gridH {
		for x := range gridW {
			c := &m.grid[y][x]
			if !c.mine && c.state != cellRevealed {
				return false
			}
		}
	}
	return true
}

// handleReveal processes a reveal action at (x, y), including first-click safety.
func (m *model) handleReveal(x, y int) {
	if m.gameState != statePlaying {
		return
	}
	if m.grid[y][x].state == cellFlagged {
		return
	}
	if m.grid[y][x].state == cellRevealed {
		return
	}

	// First click safety: regenerate if mine is hit.
	if m.firstClick {
		m.firstClick = false
		m.startTime = time.Now()
		if m.grid[y][x].mine {
			m.placeMines(x, y)
		}
	}

	m.reveal(x, y)

	if m.gameState == statePlaying && m.checkWin() {
		m.gameState = stateWon
	}
}

func (m model) Init() tea.Cmd {
	return tea.Batch(tickCmd(), listenStateDump())
}

func tickCmd() tea.Cmd {
	return tea.Tick(time.Second, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case stateDumpMsg:
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
		if data, err := json.Marshal(ms); err == nil {
			encoded := base64.StdEncoding.EncodeToString(data)
			fmt.Fprintf(os.Stdout, "\x1b]667;state-response;%s\x07", encoded)
		}
		return m, listenStateDump()

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case tickMsg:
		if m.gameState == statePlaying && !m.firstClick {
			m.elapsed = int(time.Since(m.startTime).Seconds())
		}
		return m, tickCmd()

	case tea.KeyPressMsg:
		k := tea.Key(msg)
		switch {
		case k.Code == 'q' || k.Code == 'Q' || k.Code == tea.KeyEscape:
			return m, tea.Quit
		case k.Code == 3: // ctrl+c
			return m, tea.Quit
		case k.Code == 'r' || k.Code == 'R':
			nm := newModelWithSeed(time.Now().UnixNano())
			nm.width = m.width
			nm.height = m.height
			return nm, nil
		}

		if m.gameState != statePlaying {
			return m, nil
		}

		switch {
		case k.Code == tea.KeyUp || k.Code == 'k':
			if m.cursorY > 0 {
				m.cursorY--
			}
		case k.Code == tea.KeyDown || k.Code == 'j':
			if m.cursorY < gridH-1 {
				m.cursorY++
			}
		case k.Code == tea.KeyLeft || k.Code == 'h':
			if m.cursorX > 0 {
				m.cursorX--
			}
		case k.Code == tea.KeyRight || k.Code == 'l':
			if m.cursorX < gridW-1 {
				m.cursorX++
			}
		case k.Code == ' ' || k.Code == tea.KeyEnter:
			m.handleReveal(m.cursorX, m.cursorY)
		case k.Code == 'f' || k.Code == 'F':
			if m.grid[m.cursorY][m.cursorX].state != cellRevealed {
				m.toggleFlag(m.cursorX, m.cursorY)
			}
		}
		return m, nil

	case tea.MouseClickMsg:
		mouse := tea.Mouse(msg)
		gx, gy, ok := m.screenToGrid(mouse.X, mouse.Y)
		if !ok || m.gameState != statePlaying {
			return m, nil
		}
		m.cursorX = gx
		m.cursorY = gy
		switch mouse.Button {
		case tea.MouseLeft:
			m.handleReveal(gx, gy)
		case tea.MouseRight:
			if m.grid[gy][gx].state != cellRevealed {
				m.toggleFlag(gx, gy)
			}
		}
		return m, nil
	}
	return m, nil
}

// screenToGrid converts screen coordinates to grid coordinates.
func (m model) screenToGrid(sx, sy int) (int, int, bool) {
	originX, originY := m.gridOrigin()
	dx := sx - originX
	dy := sy - originY

	if dx < 0 || dy < 0 {
		return 0, 0, false
	}

	// Each cell: cellW content + 1 border = cellW+1 stride.
	strideX := cellW + 1
	cellIdx := dx / strideX
	if dx%strideX >= cellW {
		return 0, 0, false // clicked on │ border
	}

	strideY := cellH + 1
	rowIdx := dy / strideY
	if dy%strideY >= cellH {
		return 0, 0, false // clicked on ─ border
	}

	if cellIdx < 0 || cellIdx >= gridW || rowIdx < 0 || rowIdx >= gridH {
		return 0, 0, false
	}

	return cellIdx, rowIdx, true
}

func (m model) gridOrigin() (int, int) {
	totalGridW := 1 + gridW*(cellW+1) // grid including box-drawing borders
	totalGridH := 1 + gridH*(cellH+1)

	frameW := totalGridW + 4 // border(2) + padding(2)
	frameH := totalGridH + 5 // border(2) + title(1) + status(1) + footer(1)

	originX := (m.width - frameW) / 2
	originY := (m.height - frameH) / 2

	// Grid cell content starts after: border(1) + padding(1) + │(1)
	gridX := originX + 3
	// Grid cell content starts after: border(1) + title(1) + status(1) + ┌──┐(1)
	gridY := originY + 4

	return gridX, gridY
}

// Number colors for adjacent mine counts.
var numberColors = [9]lipgloss.Color{
	"",       // 0 (unused)
	"#5B8DEF", // 1 blue
	"#50C878", // 2 green
	"#FF4444", // 3 red
	"#9B59B6", // 4 purple
	"#8B0000", // 5 maroon
	"#00CED1", // 6 cyan
	"#3C3C3C", // 7 dark
	"#808080", // 8 gray
}

func (m model) View() tea.View {
	var v tea.View
	v.AltScreen = true
	v.MouseMode = tea.MouseModeCellMotion

	if m.width == 0 || m.height == 0 {
		v.SetContent("Loading...")
		return v
	}

	// Colors.
	frameBg := lipgloss.Color("#1E1E2E")
	titleFg := lipgloss.Color("#89B4FA")
	statusFg := lipgloss.Color("#CDD6F4")
	hiddenFg := lipgloss.Color("#6C7086")
	flagFg := lipgloss.Color("#F38BA8")
	mineFg := lipgloss.Color("#F38BA8")
	cursorBg := lipgloss.Color("#585B70")
	emptyFg := lipgloss.Color("#45475A")
	footerFg := lipgloss.Color("#6C7086")
	wonFg := lipgloss.Color("#A6E3A1")
	lostFg := lipgloss.Color("#F38BA8")

	totalGridW := 1 + gridW*(cellW+1)
	innerW := totalGridW

	// Title.
	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(titleFg).
		Background(frameBg).
		Width(innerW).
		Align(lipgloss.Center)
	titleStr := titleStyle.Render("Minesweeper")

	// Status bar: mine counter + game state + timer.
	mineCount := mines - m.flags
	timerStr := fmt.Sprintf("%03d", m.elapsed)
	mineStr := fmt.Sprintf("%03d", mineCount)
	stateStr := ""
	switch m.gameState {
	case stateWon:
		stateStr = lipgloss.NewStyle().Foreground(wonFg).Bold(true).Render(" WIN! ")
	case stateLost:
		stateStr = lipgloss.NewStyle().Foreground(lostFg).Bold(true).Render("BOOM!")
	default:
		stateStr = lipgloss.NewStyle().Foreground(titleFg).Render("\uf11b")
	}

	// Pad status to fill width.
	stateW := lipgloss.Width(stateStr)
	mineW := lipgloss.Width(mineStr)
	timerW := lipgloss.Width(timerStr)
	padLen := innerW - mineW - stateW - timerW
	leftPad := padLen / 2
	rightPad := padLen - leftPad

	statusLine := lipgloss.NewStyle().Foreground(lipgloss.Color("#F9E2AF")).Background(frameBg).Render(mineStr) +
		lipgloss.NewStyle().Background(frameBg).Width(leftPad).Render("") +
		lipgloss.NewStyle().Background(frameBg).Render(stateStr) +
		lipgloss.NewStyle().Background(frameBg).Width(rightPad).Render("") +
		lipgloss.NewStyle().Foreground(statusFg).Background(frameBg).Render(timerStr)

	// Grid with box-drawing borders.
	gridLineFg := lipgloss.Color("#585B70")
	glStyle := lipgloss.NewStyle().Foreground(gridLineFg).Background(frameBg)

	hFill := strings.Repeat("─", cellW)
	buildHBorder := func(left, mid, right string) string {
		var b strings.Builder
		b.WriteString(left)
		for x := range gridW {
			b.WriteString(hFill)
			if x < gridW-1 {
				b.WriteString(mid)
			}
		}
		b.WriteString(right)
		return glStyle.Render(b.String())
	}

	topBorder := buildHBorder("┌", "┬", "┐")
	midBorder := buildHBorder("├", "┼", "┤")
	botBorder := buildHBorder("└", "┴", "┘")
	pipe := glStyle.Render("│")

	var numContent [9]string
	for i := 1; i <= 8; i++ {
		numContent[i] = fmt.Sprintf(" %d ", i)
	}

	gridRows := make([]string, 0, 2+gridH*(cellH+1))
	gridRows = append(gridRows, topBorder)

	for y := range gridH {
		for row := range cellH {
			var line strings.Builder
			line.WriteString(pipe)
			for x := range gridW {
				c := m.grid[y][x]
				isCursor := x == m.cursorX && y == m.cursorY && m.gameState == statePlaying

				fg := hiddenFg
				bg := frameBg
				if isCursor {
					bg = cursorBg
				}

				content := "   " // cellW spaces
				switch c.state {
				case cellHidden:
					content = "░░░"
					fg = hiddenFg
				case cellFlagged:
					fg = flagFg
					if row == 0 {
						content = " ⚑ "
					}
				case cellRevealed:
					if c.mine {
						fg = mineFg
						if row == 0 {
							content = " ✹ "
						}
					} else if c.adjacent > 0 {
						fg = numberColors[c.adjacent]
						if row == 0 {
							content = numContent[c.adjacent]
						}
					} else {
						fg = emptyFg
						if row == 0 {
							content = " · "
						}
					}
				}

				cellStyle := lipgloss.NewStyle().Foreground(fg).Background(bg)
				line.WriteString(cellStyle.Render(content))
				if x < gridW-1 {
					line.WriteString(pipe)
				}
			}
			line.WriteString(pipe)
			gridRows = append(gridRows, line.String())
		}
		if y < gridH-1 {
			gridRows = append(gridRows, midBorder)
		}
	}
	gridRows = append(gridRows, botBorder)
	grid := lipgloss.JoinVertical(lipgloss.Left, gridRows...)

	// Footer.
	footerStyle := lipgloss.NewStyle().
		Foreground(footerFg).
		Background(frameBg).
		Width(innerW).
		Align(lipgloss.Left)
	footerStr := footerStyle.Render("spc:reveal  f:flag  r:restart  q:quit")

	// Compose.
	content := lipgloss.JoinVertical(lipgloss.Left,
		titleStr,
		statusLine,
		grid,
		footerStr,
	)

	frameStyle := lipgloss.NewStyle().
		Background(frameBg).
		Padding(0, 1).
		BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("#585B70"))

	frame := frameStyle.Render(content)

	centered := lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, frame,
		lipgloss.WithWhitespaceBackground(lipgloss.Color("#11111B")))

	v.SetContent(centered)
	return v
}


// --- Exported functions for testing ---

// NewTestModel creates a model with a specific seed for deterministic testing.
func NewTestModel(seed int64) model {
	return newModelWithSeed(seed)
}

// RevealCell exposes reveal for testing.
func (m *model) RevealCell(x, y int) {
	m.handleReveal(x, y)
}

// ToggleFlag exposes toggleFlag for testing.
func (m *model) ToggleFlag(x, y int) {
	m.toggleFlag(x, y)
}

// GameState returns the game state.
func (m *model) GameState() int {
	return m.gameState
}

// Grid returns a pointer to the grid for testing.
func (m *model) Grid() *[gridH][gridW]cell {
	return &m.grid
}

// Flags returns the current flag count.
func (m *model) Flags() int {
	return m.flags
}

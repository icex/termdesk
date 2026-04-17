package main

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
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

// Direction constants.
type direction int

const (
	dirRight direction = iota
	dirLeft
	dirUp
	dirDown
)

// Game dimensions.
const (
	fieldW = 40 // playing field columns
	fieldH = 20 // playing field rows

	initialSpeed = 150 * time.Millisecond
	minSpeed     = 50 * time.Millisecond
	speedStep    = 20 * time.Millisecond // decrease per 5 food eaten
)

// point represents a grid coordinate.
type point struct {
	x, y int
}

// Serializable state for workspace persistence.
type snakeState struct {
	Snake    [][2]int `json:"s"`  // each point as [x,y]
	Dir      int      `json:"d"`
	NextDir  int      `json:"nd"`
	FoodX    int      `json:"fx"`
	FoodY    int      `json:"fy"`
	Score    int      `json:"sc"`
	SpeedMs  int64    `json:"sp"`
	GameOver bool     `json:"go"`
	Started  bool     `json:"st"`
	Paused   bool     `json:"p"`
	Gen      int      `json:"g"`
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

// tickMsg drives the game loop. gen tracks which game generation this tick belongs to.
type tickMsg struct {
	gen int
}

type model struct {
	snake    []point   // head is snake[0]
	dir      direction // current direction
	nextDir  direction // buffered next direction (applied on tick)
	food     point
	score    int
	speed    time.Duration
	gameOver bool
	started  bool // false until first directional input
	paused   bool // true when game is paused
	gen      int  // generation counter, incremented on reset to discard stale ticks
	width    int
	height   int
	rng      *rand.Rand
}

func newModel() model {
	m := newModelWithSeed(uint64(time.Now().UnixNano()))
	// Restore state from TERMDESK_APP_STATE env var if present (workspace restore).
	if envState := os.Getenv("TERMDESK_APP_STATE"); envState != "" {
		if decoded, err := base64.StdEncoding.DecodeString(envState); err == nil {
			var ss snakeState
			if err := json.Unmarshal(decoded, &ss); err == nil {
				m.snake = make([]point, len(ss.Snake))
				for i, p := range ss.Snake {
					m.snake[i] = point{p[0], p[1]}
				}
				m.dir = direction(ss.Dir)
				m.nextDir = direction(ss.NextDir)
				m.food = point{ss.FoodX, ss.FoodY}
				m.score = ss.Score
				m.speed = time.Duration(ss.SpeedMs) * time.Millisecond
				m.gameOver = ss.GameOver
				m.started = ss.Started
				m.paused = ss.Paused
				m.gen = ss.Gen
			}
		}
	}
	return m
}

func newModelWithSeed(seed uint64) model {
	r := rand.New(rand.NewPCG(seed, 0))
	m := model{
		dir:   dirRight,
		speed: initialSpeed,
		rng:   r,
	}
	m.initSnake()
	m.placeFood()
	return m
}

// initSnake places a 3-segment snake in the center of the field moving right.
func (m *model) initSnake() {
	cx, cy := fieldW/2, fieldH/2
	m.snake = []point{
		{cx, cy},     // head
		{cx - 1, cy}, // body
		{cx - 2, cy}, // tail
	}
}

// placeFood randomly places food on a cell not occupied by the snake.
func (m *model) placeFood() {
	occupied := make(map[point]bool, len(m.snake))
	for _, p := range m.snake {
		occupied[p] = true
	}
	for {
		p := point{m.rng.IntN(fieldW), m.rng.IntN(fieldH)}
		if !occupied[p] {
			m.food = p
			return
		}
	}
}

// reset restarts the game.
func (m *model) reset() {
	m.score = 0
	m.speed = initialSpeed
	m.dir = dirRight
	m.nextDir = dirRight
	m.gameOver = false
	m.started = false
	m.paused = false
	m.gen++
	m.initSnake()
	m.placeFood()
}

func (m model) Init() tea.Cmd {
	cmds := []tea.Cmd{listenStateDump()}
	if m.started && !m.gameOver && !m.paused {
		cmds = append(cmds, tick(m.speed, m.gen))
	}
	return tea.Batch(cmds...)
}

func tick(d time.Duration, gen int) tea.Cmd {
	return tea.Tick(d, func(time.Time) tea.Msg {
		return tickMsg{gen: gen}
	})
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case stateDumpMsg:
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
		if data, err := json.Marshal(ss); err == nil {
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
				// Don't start tick here — let first direction key start it
				// (same as initial game). Starting here would create a
				// duplicate tick chain when the user presses a direction.
			}
			return m, nil
		case k.Code == 'p' || k.Code == 'P':
			if m.started && !m.gameOver {
				m.paused = !m.paused
				if !m.paused {
					return m, tick(m.speed, m.gen)
				}
			}
			return m, nil
		}
		if !m.gameOver && !m.paused {
			moved := false
			switch {
			case k.Code == tea.KeyUp || k.Code == 'k' || k.Code == 'K':
				if m.dir != dirDown {
					m.nextDir = dirUp
					moved = true
				}
			case k.Code == tea.KeyDown || k.Code == 'j' || k.Code == 'J':
				if m.dir != dirUp {
					m.nextDir = dirDown
					moved = true
				}
			case k.Code == tea.KeyLeft || k.Code == 'h' || k.Code == 'H':
				if m.dir != dirRight {
					m.nextDir = dirLeft
					moved = true
				}
			case k.Code == tea.KeyRight || k.Code == 'l' || k.Code == 'L':
				if m.dir != dirLeft {
					m.nextDir = dirRight
					moved = true
				}
			}
			if moved && !m.started {
				m.started = true
				return m, tick(m.speed, m.gen)
			}
		}
		return m, nil

	case tickMsg:
		if msg.gen != m.gen || m.gameOver || m.paused {
			return m, nil // discard stale ticks from previous game
		}
		m.dir = m.nextDir
		m.step()
		if m.gameOver {
			return m, nil
		}
		return m, tick(m.speed, m.gen)
	}
	return m, nil
}

// step advances the snake one cell in the current direction.
func (m *model) step() {
	head := m.snake[0]
	var next point
	switch m.dir {
	case dirUp:
		next = point{head.x, head.y - 1}
	case dirDown:
		next = point{head.x, head.y + 1}
	case dirLeft:
		next = point{head.x - 1, head.y}
	case dirRight:
		next = point{head.x + 1, head.y}
	}

	// Wall collision.
	if next.x < 0 || next.x >= fieldW || next.y < 0 || next.y >= fieldH {
		m.gameOver = true
		return
	}

	// Self collision (check against body, excluding the tail which will move).
	// But if we're about to eat food, the tail won't move, so check all segments.
	ate := next == m.food
	bodyEnd := len(m.snake)
	if !ate {
		bodyEnd = len(m.snake) - 1 // tail will vacate
	}
	for i := 0; i < bodyEnd; i++ {
		if m.snake[i] == next {
			m.gameOver = true
			return
		}
	}

	// Move snake.
	newSnake := make([]point, 0, len(m.snake)+1)
	newSnake = append(newSnake, next)
	if ate {
		newSnake = append(newSnake, m.snake...) // keep tail
		m.snake = newSnake
		m.score++
		// Speed up every 5 food.
		if m.score%5 == 0 && m.speed > minSpeed {
			m.speed -= speedStep
			if m.speed < minSpeed {
				m.speed = minSpeed
			}
		}
		m.placeFood()
	} else {
		newSnake = append(newSnake, m.snake[:len(m.snake)-1]...) // drop tail
		m.snake = newSnake
	}
}

func (m model) View() tea.View {
	var v tea.View
	v.AltScreen = true

	if m.width == 0 || m.height == 0 {
		v.SetContent("Loading...")
		return v
	}

	// Colors.
	borderFg := lipgloss.Color("#CDD6F4")
	bgColor := lipgloss.Color("#11111B")
	snakeColor := lipgloss.Color("#A6E3A1")
	foodColor := lipgloss.Color("#F38BA8")
	statusFg := lipgloss.Color("#CDD6F4")
	fieldBg := lipgloss.Color("#1E1E2E")
	gameOverFg := lipgloss.Color("#F38BA8")

	// Build grid. Each cell is one character.
	// Create a set for fast snake lookup.
	snakeSet := make(map[point]bool, len(m.snake))
	for _, p := range m.snake {
		snakeSet[p] = true
	}

	snakeStyle := lipgloss.NewStyle().Foreground(snakeColor).Background(fieldBg)
	foodStyle := lipgloss.NewStyle().Foreground(foodColor).Background(fieldBg)
	emptyStyle := lipgloss.NewStyle().Foreground(fieldBg).Background(fieldBg)
	borderStyle := lipgloss.NewStyle().Foreground(borderFg).Background(fieldBg)

	// Pre-render cell strings once instead of per-cell per-frame.
	snakeStr := snakeStyle.Render("██")
	foodStr := foodStyle.Render("●ˑ")
	emptyStr := emptyStyle.Render("  ")

	// Pre-compute border strings once.
	hFill := strings.Repeat("──", fieldW)
	topBorder := borderStyle.Render("┌" + hFill + "┐")
	botBorder := borderStyle.Render("└" + hFill + "┘")
	pipe := borderStyle.Render("│")

	var sb strings.Builder
	sb.Grow(4096)
	// Top border.
	sb.WriteString(topBorder)
	sb.WriteByte('\n')

	for y := 0; y < fieldH; y++ {
		sb.WriteString(pipe)
		for x := 0; x < fieldW; x++ {
			p := point{x, y}
			if snakeSet[p] {
				sb.WriteString(snakeStr)
			} else if p == m.food {
				sb.WriteString(foodStr)
			} else {
				sb.WriteString(emptyStr)
			}
		}
		sb.WriteString(pipe)
		sb.WriteByte('\n')
	}

	// Bottom border.
	sb.WriteString(botBorder)
	sb.WriteByte('\n')

	// Status bar.
	statusStyle := lipgloss.NewStyle().Foreground(statusFg).Background(fieldBg).Bold(true)
	if !m.started && !m.gameOver {
		sb.WriteString(statusStyle.Render(" Press arrow key to start "))
	} else {
		speedMs := m.speed.Milliseconds()
		statusText := fmt.Sprintf(" Score: %d │ Speed: %dms ", m.score, speedMs)
		sb.WriteString(statusStyle.Render(statusText))
	}

	if m.paused {
		sb.WriteByte('\n')
		pauseStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#F9E2AF")).Background(fieldBg).Bold(true)
		sb.WriteString(pauseStyle.Render(" PAUSED — press 'p' to resume "))
	} else if m.gameOver {
		sb.WriteByte('\n')
		goStyle := lipgloss.NewStyle().Foreground(gameOverFg).Background(fieldBg).Bold(true)
		sb.WriteString(goStyle.Render(" GAME OVER — press 'r' to restart "))
	}

	// Footer with key hints.
	sb.WriteByte('\n')
	footerStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#6C7086")).Background(fieldBg)
	sb.WriteString(footerStyle.Render(" arrows:move  p:pause  r:restart  q:quit "))

	content := sb.String()

	// Center on screen.
	centered := lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, content,
		lipgloss.WithWhitespaceBackground(bgColor))

	v.SetContent(centered)
	return v
}

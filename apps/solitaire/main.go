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

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/colorprofile"
	"github.com/charmbracelet/lipgloss"
)

func main() {
	p := tea.NewProgram(newModel(), tea.WithColorProfile(colorprofile.TrueColor))
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

// ── Card types ──────────────────────────────────────────────────────

type suit int

const (
	suitClubs    suit = iota // black ♣
	suitDiamonds             // red ♦
	suitHearts               // red ♥
	suitSpades               // black ♠
)

func isRed(s suit) bool           { return s == suitDiamonds || s == suitHearts }
func oppositeColor(a, b suit) bool { return isRed(a) != isRed(b) }

func suitRune(s suit) rune {
	switch s {
	case suitClubs:
		return '♣'
	case suitDiamonds:
		return '♦'
	case suitHearts:
		return '♥'
	case suitSpades:
		return '♠'
	}
	return '?'
}

// suitDisplay returns the suit string for rendering.
// Hearts get a text variation selector to force filled rendering.
func suitDisplay(s suit) string {
	if s == suitHearts {
		return "❤"
	}
	return string(suitRune(s))
}

func rankString(rank int) string {
	switch rank {
	case 1:
		return "A"
	case 10:
		return "10"
	case 11:
		return "J"
	case 12:
		return "Q"
	case 13:
		return "K"
	default:
		return fmt.Sprintf("%d", rank)
	}
}

type card struct {
	rank   int
	suit   suit
	faceUp bool
}

// ── Pile identification ─────────────────────────────────────────────

type pileType int

const (
	pileStock      pileType = iota
	pileWaste
	pileFoundation
	pileTableau
)

type pileID struct {
	ptype pileType
	index int
}

type selection struct {
	pile    pileID
	cardIdx int
	count   int // number of cards in run (>=1)
}

// ── Game / draw modes ───────────────────────────────────────────────

type gameState int

const (
	statePlaying     gameState = iota
	stateWon
	stateAutoComplete
)

type drawMode int

const (
	drawOne   drawMode = iota
	drawThree
)

// ── Scoring constants ───────────────────────────────────────────────

const (
	scoreWasteToTab    = 5
	scoreWasteToFound  = 10
	scoreTabToFound    = 10
	scoreTurnCard      = 5
	scoreFoundToTab    = -15
)

// ── Tick / animation messages ───────────────────────────────────────

type tickMsg time.Time
type autoTickMsg struct{ gen int }
type stateDumpMsg struct{}

func tickCmd() tea.Cmd {
	return tea.Tick(time.Second, func(t time.Time) tea.Msg { return tickMsg(t) })
}

func scheduleAutoTick(gen int) tea.Cmd {
	return tea.Tick(80*time.Millisecond, func(time.Time) tea.Msg { return autoTickMsg{gen: gen} })
}

func listenStateDump() tea.Cmd {
	return func() tea.Msg {
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, syscall.SIGUSR1)
		<-sigCh
		signal.Stop(sigCh)
		return stateDumpMsg{}
	}
}

// ── Auto-complete step ──────────────────────────────────────────────

type autoMove struct {
	fromPile pileID
	cardIdx  int
	toFound  int
}

// ── Model ───────────────────────────────────────────────────────────

type model struct {
	stock       []card
	waste       []card
	foundations [4][]card
	tableau     [7][]card

	selected   *selection
	cursor     pileID
	cursorCard int

	hoverPile  pileID
	hoverCard  int
	hoverValid bool

	// Drag state.
	dragging      bool
	dragStartPile pileID
	dragStartCard int
	dragX, dragY  int // current mouse position during drag

	drawMode  drawMode
	score     int
	moves     int
	startTime time.Time
	elapsed   int
	started   bool

	state gameState
	gen   int

	// Auto-complete.
	autoStep  int
	autoCards []autoMove

	// Double-click detection.
	lastClickTime time.Time
	lastClickPile pileID
	lastClickCard int

	width  int
	height int
	rng    *rand.Rand
}

// ── Construction ────────────────────────────────────────────────────

func newDeck(rng *rand.Rand) []card {
	deck := make([]card, 52)
	i := 0
	for s := suit(0); s < 4; s++ {
		for r := 1; r <= 13; r++ {
			deck[i] = card{rank: r, suit: s}
			i++
		}
	}
	rng.Shuffle(len(deck), func(a, b int) { deck[a], deck[b] = deck[b], deck[a] })
	return deck
}

func deal(deck []card) ([]card, [7][]card) {
	var tableau [7][]card
	idx := 0
	for col := 0; col < 7; col++ {
		tableau[col] = make([]card, col+1)
		for row := 0; row <= col; row++ {
			c := deck[idx]
			c.faceUp = row == col
			tableau[col][row] = c
			idx++
		}
	}
	stock := make([]card, len(deck)-idx)
	copy(stock, deck[idx:])
	return stock, tableau
}

func newModel() model {
	m := newModelWithSeed(uint64(time.Now().UnixNano()))
	if envState := os.Getenv("TERMDESK_APP_STATE"); envState != "" {
		if decoded, err := base64.StdEncoding.DecodeString(envState); err == nil {
			m.restoreState(decoded)
		}
	}
	return m
}

func newModelWithSeed(seed uint64) model {
	rng := rand.New(rand.NewPCG(seed, 0))
	deck := newDeck(rng)
	stock, tableau := deal(deck)
	return model{
		stock:   stock,
		tableau: tableau,
		cursor:  pileID{ptype: pileStock},
		rng:     rng,
	}
}

// ── Move validation ─────────────────────────────────────────────────

func (m *model) canMoveToFoundation(c card, fi int) bool {
	f := m.foundations[fi]
	if len(f) == 0 {
		return c.rank == 1
	}
	top := f[len(f)-1]
	return c.suit == top.suit && c.rank == top.rank+1
}

func (m *model) canMoveToTableau(c card, ti int) bool {
	tab := m.tableau[ti]
	if len(tab) == 0 {
		return c.rank == 13
	}
	top := tab[len(tab)-1]
	return top.faceUp && oppositeColor(c.suit, top.suit) && c.rank == top.rank-1
}

func (m *model) findFoundationForCard(c card) int {
	for i := 0; i < 4; i++ {
		if m.canMoveToFoundation(c, i) {
			return i
		}
	}
	return -1
}

// ── Move execution ──────────────────────────────────────────────────

func (m *model) addScore(delta int) {
	m.score += delta
	if m.score < 0 {
		m.score = 0
	}
}

func (m *model) ensureStarted() {
	if !m.started {
		m.started = true
		m.startTime = time.Now()
	}
}

func (m *model) drawFromStock() {
	m.ensureStarted()
	if len(m.stock) == 0 {
		// Recycle waste back to stock (reversed, face-down).
		for i := len(m.waste) - 1; i >= 0; i-- {
			c := m.waste[i]
			c.faceUp = false
			m.stock = append(m.stock, c)
		}
		m.waste = m.waste[:0]
		return
	}
	n := 1
	if m.drawMode == drawThree {
		n = 3
	}
	if n > len(m.stock) {
		n = len(m.stock)
	}
	for i := 0; i < n; i++ {
		c := m.stock[len(m.stock)-1]
		m.stock = m.stock[:len(m.stock)-1]
		c.faceUp = true
		m.waste = append(m.waste, c)
	}
	m.moves++
}

func (m *model) flipTopTableau(ti int) {
	tab := m.tableau[ti]
	if len(tab) > 0 && !tab[len(tab)-1].faceUp {
		m.tableau[ti][len(tab)-1].faceUp = true
		m.addScore(scoreTurnCard)
	}
}

func (m *model) moveWasteToFoundation(fi int) bool {
	if len(m.waste) == 0 {
		return false
	}
	c := m.waste[len(m.waste)-1]
	if !m.canMoveToFoundation(c, fi) {
		return false
	}
	m.waste = m.waste[:len(m.waste)-1]
	m.foundations[fi] = append(m.foundations[fi], c)
	m.addScore(scoreWasteToFound)
	m.moves++
	return true
}

func (m *model) moveWasteToTableau(ti int) bool {
	if len(m.waste) == 0 {
		return false
	}
	c := m.waste[len(m.waste)-1]
	if !m.canMoveToTableau(c, ti) {
		return false
	}
	m.waste = m.waste[:len(m.waste)-1]
	m.tableau[ti] = append(m.tableau[ti], c)
	m.addScore(scoreWasteToTab)
	m.moves++
	return true
}

func (m *model) moveTableauToFoundation(srcTab, fi int) bool {
	tab := m.tableau[srcTab]
	if len(tab) == 0 {
		return false
	}
	c := tab[len(tab)-1]
	if !c.faceUp || !m.canMoveToFoundation(c, fi) {
		return false
	}
	m.tableau[srcTab] = tab[:len(tab)-1]
	m.foundations[fi] = append(m.foundations[fi], c)
	m.addScore(scoreTabToFound)
	m.flipTopTableau(srcTab)
	m.moves++
	return true
}

func (m *model) moveTableauToTableau(srcTab, srcCardIdx, dstTab int) bool {
	src := m.tableau[srcTab]
	if srcCardIdx < 0 || srcCardIdx >= len(src) {
		return false
	}
	c := src[srcCardIdx]
	if !c.faceUp {
		return false
	}
	if !m.canMoveToTableau(c, dstTab) {
		return false
	}
	run := make([]card, len(src)-srcCardIdx)
	copy(run, src[srcCardIdx:])
	m.tableau[srcTab] = src[:srcCardIdx]
	m.tableau[dstTab] = append(m.tableau[dstTab], run...)
	m.flipTopTableau(srcTab)
	m.moves++
	return true
}

func (m *model) moveFoundationToTableau(fi, ti int) bool {
	f := m.foundations[fi]
	if len(f) == 0 {
		return false
	}
	c := f[len(f)-1]
	if !m.canMoveToTableau(c, ti) {
		return false
	}
	m.foundations[fi] = f[:len(f)-1]
	m.tableau[ti] = append(m.tableau[ti], c)
	m.addScore(scoreFoundToTab)
	m.moves++
	return true
}

func (m *model) autoSendToFoundation(p pileID, cardIdx int) bool {
	switch p.ptype {
	case pileWaste:
		if len(m.waste) == 0 {
			return false
		}
		c := m.waste[len(m.waste)-1]
		fi := m.findFoundationForCard(c)
		if fi < 0 {
			return false
		}
		return m.moveWasteToFoundation(fi)
	case pileTableau:
		tab := m.tableau[p.index]
		if cardIdx != len(tab)-1 || len(tab) == 0 {
			return false
		}
		c := tab[cardIdx]
		if !c.faceUp {
			return false
		}
		fi := m.findFoundationForCard(c)
		if fi < 0 {
			return false
		}
		return m.moveTableauToFoundation(p.index, fi)
	case pileFoundation:
		return false
	}
	return false
}

// ── Selection logic ─────────────────────────────────────────────────

func (m *model) clearSelection() { m.selected = nil }

func (m *model) selectCard(p pileID, cardIdx int) {
	count := 1
	if p.ptype == pileTableau {
		count = len(m.tableau[p.index]) - cardIdx
	}
	m.selected = &selection{pile: p, cardIdx: cardIdx, count: count}
}

func (m *model) tryPlace(dst pileID) bool {
	if m.selected == nil {
		return false
	}
	sel := m.selected
	m.clearSelection()

	switch sel.pile.ptype {
	case pileWaste:
		if dst.ptype == pileFoundation {
			return m.moveWasteToFoundation(dst.index)
		}
		if dst.ptype == pileTableau {
			return m.moveWasteToTableau(dst.index)
		}
	case pileTableau:
		if dst.ptype == pileFoundation && sel.count == 1 {
			return m.moveTableauToFoundation(sel.pile.index, dst.index)
		}
		if dst.ptype == pileTableau && dst.index != sel.pile.index {
			return m.moveTableauToTableau(sel.pile.index, sel.cardIdx, dst.index)
		}
	case pileFoundation:
		if dst.ptype == pileTableau {
			return m.moveFoundationToTableau(sel.pile.index, dst.index)
		}
	}
	return false
}

// ── Win / auto-complete ─────────────────────────────────────────────

func (m *model) checkWin() bool {
	for i := 0; i < 4; i++ {
		if len(m.foundations[i]) != 13 {
			return false
		}
	}
	return true
}

func (m *model) canAutoComplete() bool {
	if len(m.stock) > 0 {
		return false
	}
	for col := 0; col < 7; col++ {
		for _, c := range m.tableau[col] {
			if !c.faceUp {
				return false
			}
		}
	}
	return true
}

func (m *model) buildAutoCompleteSequence() []autoMove {
	// Deep copy state for simulation.
	var simFound [4][]card
	for i := 0; i < 4; i++ {
		simFound[i] = make([]card, len(m.foundations[i]))
		copy(simFound[i], m.foundations[i])
	}
	var simTab [7][]card
	for i := 0; i < 7; i++ {
		simTab[i] = make([]card, len(m.tableau[i]))
		copy(simTab[i], m.tableau[i])
	}
	simWaste := make([]card, len(m.waste))
	copy(simWaste, m.waste)

	var moves []autoMove
	for len(moves) < 52 {
		moved := false
		// Try waste first.
		if len(simWaste) > 0 {
			c := simWaste[len(simWaste)-1]
			for fi := 0; fi < 4; fi++ {
				if canPlaceOnFound(c, simFound[fi]) {
					moves = append(moves, autoMove{
						fromPile: pileID{ptype: pileWaste},
						cardIdx:  len(simWaste) - 1,
						toFound:  fi,
					})
					simWaste = simWaste[:len(simWaste)-1]
					simFound[fi] = append(simFound[fi], c)
					moved = true
					break
				}
			}
			if moved {
				continue
			}
		}
		// Try each tableau column.
		for col := 0; col < 7; col++ {
			if len(simTab[col]) == 0 {
				continue
			}
			c := simTab[col][len(simTab[col])-1]
			for fi := 0; fi < 4; fi++ {
				if canPlaceOnFound(c, simFound[fi]) {
					moves = append(moves, autoMove{
						fromPile: pileID{ptype: pileTableau, index: col},
						cardIdx:  len(simTab[col]) - 1,
						toFound:  fi,
					})
					simTab[col] = simTab[col][:len(simTab[col])-1]
					simFound[fi] = append(simFound[fi], c)
					moved = true
					break
				}
			}
			if moved {
				break
			}
		}
		if !moved {
			break
		}
	}
	return moves
}

func canPlaceOnFound(c card, f []card) bool {
	if len(f) == 0 {
		return c.rank == 1
	}
	top := f[len(f)-1]
	return c.suit == top.suit && c.rank == top.rank+1
}

func (m *model) startAutoComplete() tea.Cmd {
	m.state = stateAutoComplete
	m.autoCards = m.buildAutoCompleteSequence()
	m.autoStep = 0
	m.gen++
	return scheduleAutoTick(m.gen)
}

func (m *model) stepAutoComplete() {
	if m.autoStep >= len(m.autoCards) {
		return
	}
	mv := m.autoCards[m.autoStep]
	switch mv.fromPile.ptype {
	case pileWaste:
		m.moveWasteToFoundation(mv.toFound)
	case pileTableau:
		m.moveTableauToFoundation(mv.fromPile.index, mv.toFound)
	}
	m.autoStep++
}

func timedBonus(elapsed int) int {
	if elapsed <= 0 {
		return 0
	}
	bonus := 700000 / elapsed
	if bonus < 0 {
		bonus = 0
	}
	return bonus
}

// ── Reset ───────────────────────────────────────────────────────────

func (m *model) reset() {
	deck := newDeck(m.rng)
	stock, tableau := deal(deck)
	m.stock = stock
	m.waste = nil
	m.foundations = [4][]card{}
	m.tableau = tableau
	m.selected = nil
	m.cursor = pileID{ptype: pileStock}
	m.cursorCard = 0
	m.score = 0
	m.moves = 0
	m.elapsed = 0
	m.started = false
	m.state = statePlaying
	m.gen++
	m.autoStep = 0
	m.autoCards = nil
}

// ── Bubble Tea interface ────────────────────────────────────────────

func (m model) Init() tea.Cmd {
	cmds := []tea.Cmd{tickCmd(), listenStateDump()}
	return tea.Batch(cmds...)
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case stateDumpMsg:
		m.dumpState()
		return m, listenStateDump()

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case tickMsg:
		if m.state == statePlaying && m.started {
			m.elapsed = int(time.Since(m.startTime).Seconds())
		}
		return m, tickCmd()

	case autoTickMsg:
		if msg.gen != m.gen || m.state != stateAutoComplete {
			return m, nil
		}
		m.stepAutoComplete()
		if m.autoStep >= len(m.autoCards) {
			m.state = stateWon
			m.addScore(timedBonus(m.elapsed))
			return m, nil
		}
		return m, scheduleAutoTick(m.gen)

	case tea.KeyPressMsg:
		return m.handleKey(tea.Key(msg))

	case tea.MouseClickMsg:
		mouse := tea.Mouse(msg)
		return m.handleMouseClick(mouse)

	case tea.MouseReleaseMsg:
		mouse := tea.Mouse(msg)
		return m.handleMouseRelease(mouse)

	case tea.MouseMotionMsg:
		mouse := tea.Mouse(msg)
		hit := m.hitTest(mouse.X, mouse.Y)
		m.hoverValid = hit.valid
		m.hoverPile = hit.pile
		m.hoverCard = hit.cardIdx

		if mouse.Button == tea.MouseLeft && m.state == statePlaying {
			if !m.dragging && m.selected == nil {
				// Start drag from motion with button held.
				if hit.valid && hit.cardIdx >= 0 {
					m.ensureStarted()
					m.cursor = hit.pile
					if hit.pile.ptype == pileTableau {
						m.cursorCard = hit.cardIdx
					}
					m.selectCard(hit.pile, hit.cardIdx)
					m.dragging = true
					m.dragStartPile = hit.pile
					m.dragStartCard = hit.cardIdx
				}
			}
			m.dragX = mouse.X
			m.dragY = mouse.Y
		} else if m.dragging && m.selected != nil {
			// Button released during motion — drop.
			return m.handleDrop(hit)
		}

		return m, nil
	}
	return m, nil
}

// ── Key handling ────────────────────────────────────────────────────

func (m model) handleKey(k tea.Key) (tea.Model, tea.Cmd) {
	switch {
	case k.Code == 'q' || k.Code == 'Q' || k.Code == tea.KeyEscape || k.Code == 3:
		return m, tea.Quit
	case k.Code == 'r' || k.Code == 'R':
		m.reset()
		return m, nil
	}

	if m.state != statePlaying {
		return m, nil
	}

	switch {
	case k.Code == 'm' || k.Code == 'M':
		if m.drawMode == drawOne {
			m.drawMode = drawThree
		} else {
			m.drawMode = drawOne
		}
		return m, nil

	case k.Code == 'd' || k.Code == 'D':
		m.clearSelection()
		m.drawFromStock()
		m.checkAutoCompleteOrWin()
		return m, nil

	case k.Code == 'a' || k.Code == 'A':
		if !m.autoSendToFoundation(m.cursor, m.lastCardIdx(m.cursor)) {
			return m, tea.Raw("\x07")
		}
		cmd := m.checkAutoCompleteOrWin()
		return m, cmd

	case k.Code == tea.KeyLeft || k.Code == 'h':
		m.cursorLeft()
	case k.Code == tea.KeyRight || k.Code == 'l':
		m.cursorRight()
	case k.Code == tea.KeyUp || k.Code == 'k':
		m.cursorUp()
	case k.Code == tea.KeyDown || k.Code == 'j':
		m.cursorDown()
	case k.Code == tea.KeyTab:
		if k.Mod == tea.ModShift {
			m.tabPrev()
		} else {
			m.tabNext()
		}

	case k.Code == ' ' || k.Code == tea.KeyEnter:
		m.ensureStarted()
		if m.cursor.ptype == pileStock {
			m.clearSelection()
			m.drawFromStock()
		} else if m.selected != nil {
			if !m.tryPlace(m.cursor) {
				return m, tea.Raw("\x07")
			}
		} else {
			ci := m.cursorCard
			if m.cursor.ptype != pileTableau {
				ci = m.lastCardIdx(m.cursor)
			}
			if ci >= 0 {
				m.selectCard(m.cursor, ci)
			} else {
				return m, tea.Raw("\x07")
			}
		}
		cmd := m.checkAutoCompleteOrWin()
		return m, cmd
	}

	return m, nil
}

func (m *model) checkAutoCompleteOrWin() tea.Cmd {
	if m.checkWin() {
		m.state = stateWon
		m.addScore(timedBonus(m.elapsed))
	} else if m.canAutoComplete() {
		return m.startAutoComplete()
	}
	return nil
}

func (m *model) lastCardIdx(p pileID) int {
	switch p.ptype {
	case pileWaste:
		if len(m.waste) > 0 {
			return len(m.waste) - 1
		}
	case pileFoundation:
		if len(m.foundations[p.index]) > 0 {
			return len(m.foundations[p.index]) - 1
		}
	case pileTableau:
		if len(m.tableau[p.index]) > 0 {
			return len(m.tableau[p.index]) - 1
		}
	}
	return -1
}

// ── Keyboard navigation ─────────────────────────────────────────────

// Pile order for left/right in top row: stock, waste, found0..3
// Pile order for left/right in tableau: tab0..6

func (m *model) cursorLeft() {
	switch m.cursor.ptype {
	case pileStock:
		// already leftmost
	case pileWaste:
		m.cursor = pileID{ptype: pileStock}
	case pileFoundation:
		if m.cursor.index > 0 {
			m.cursor.index--
		} else {
			m.cursor = pileID{ptype: pileWaste}
		}
	case pileTableau:
		if m.cursor.index > 0 {
			m.cursor.index--
			m.cursorCard = m.clampCursorCard(m.cursor)
		}
	}
}

func (m *model) cursorRight() {
	switch m.cursor.ptype {
	case pileStock:
		m.cursor = pileID{ptype: pileWaste}
	case pileWaste:
		m.cursor = pileID{ptype: pileFoundation, index: 0}
	case pileFoundation:
		if m.cursor.index < 3 {
			m.cursor.index++
		}
	case pileTableau:
		if m.cursor.index < 6 {
			m.cursor.index++
			m.cursorCard = m.clampCursorCard(m.cursor)
		}
	}
}

func (m *model) cursorUp() {
	if m.cursor.ptype == pileTableau {
		prev := m.cursorCard
		if m.cursorCard > 0 {
			m.cursorCard--
			// Ensure cursor is on a face-up card.
			tab := m.tableau[m.cursor.index]
			if m.cursorCard < len(tab) && !tab[m.cursorCard].faceUp {
				m.cursorCard = prev // bounce back
			}
		}
		if m.cursorCard == prev {
			// At topmost face-up card (or cursorCard was 0) — move to top row.
			col := m.cursor.index
			if col <= 0 {
				m.cursor = pileID{ptype: pileStock}
			} else if col == 1 {
				m.cursor = pileID{ptype: pileWaste}
			} else {
				m.cursor = pileID{ptype: pileFoundation, index: col - 3}
				if m.cursor.index < 0 {
					m.cursor.index = 0
				}
				if m.cursor.index > 3 {
					m.cursor.index = 3
				}
			}
		}
	}
}

func (m *model) cursorDown() {
	switch m.cursor.ptype {
	case pileStock:
		m.cursor = pileID{ptype: pileTableau, index: 0}
		m.cursorCard = m.clampCursorCard(m.cursor)
	case pileWaste:
		m.cursor = pileID{ptype: pileTableau, index: 1}
		m.cursorCard = m.clampCursorCard(m.cursor)
	case pileFoundation:
		m.cursor = pileID{ptype: pileTableau, index: m.cursor.index + 3}
		if m.cursor.index > 6 {
			m.cursor.index = 6
		}
		m.cursorCard = m.clampCursorCard(m.cursor)
	case pileTableau:
		tab := m.tableau[m.cursor.index]
		if m.cursorCard < len(tab)-1 {
			m.cursorCard++
		}
	}
}

func (m *model) clampCursorCard(p pileID) int {
	if p.ptype != pileTableau {
		return 0
	}
	tab := m.tableau[p.index]
	// Find first face-up card.
	for i, c := range tab {
		if c.faceUp {
			return i
		}
	}
	if len(tab) > 0 {
		return len(tab) - 1
	}
	return 0
}

func (m *model) tabNext() {
	// Order: stock, waste, found0-3, tab0-6
	switch m.cursor.ptype {
	case pileStock:
		m.cursor = pileID{ptype: pileWaste}
	case pileWaste:
		m.cursor = pileID{ptype: pileFoundation, index: 0}
	case pileFoundation:
		if m.cursor.index < 3 {
			m.cursor.index++
		} else {
			m.cursor = pileID{ptype: pileTableau, index: 0}
			m.cursorCard = m.clampCursorCard(m.cursor)
		}
	case pileTableau:
		if m.cursor.index < 6 {
			m.cursor.index++
			m.cursorCard = m.clampCursorCard(m.cursor)
		} else {
			m.cursor = pileID{ptype: pileStock}
		}
	}
}

func (m *model) tabPrev() {
	switch m.cursor.ptype {
	case pileStock:
		m.cursor = pileID{ptype: pileTableau, index: 6}
		m.cursorCard = m.clampCursorCard(m.cursor)
	case pileWaste:
		m.cursor = pileID{ptype: pileStock}
	case pileFoundation:
		if m.cursor.index > 0 {
			m.cursor.index--
		} else {
			m.cursor = pileID{ptype: pileWaste}
		}
	case pileTableau:
		if m.cursor.index > 0 {
			m.cursor.index--
			m.cursorCard = m.clampCursorCard(m.cursor)
		} else {
			m.cursor = pileID{ptype: pileFoundation, index: 3}
		}
	}
}

// ── Mouse handling ──────────────────────────────────────────────────

const doubleClickMs = 400

func (m model) handleMouseClick(mouse tea.Mouse) (tea.Model, tea.Cmd) {
	if m.state != statePlaying {
		return m, nil
	}

	hit := m.hitTest(mouse.X, mouse.Y)
	if !hit.valid {
		m.clearSelection()
		m.dragging = false
		return m, nil
	}

	m.ensureStarted()

	// Right-click: auto-send to foundation.
	if mouse.Button == tea.MouseRight {
		if !m.autoSendToFoundation(hit.pile, hit.cardIdx) {
			return m, tea.Raw("\x07")
		}
		cmd := m.checkAutoCompleteOrWin()
		return m, cmd
	}

	// Double-click: auto-send to foundation.
	isDouble := m.isDoubleClick(hit.pile, hit.cardIdx)
	if isDouble && hit.pile.ptype != pileStock {
		m.clearSelection()
		m.dragging = false
		if !m.autoSendToFoundation(hit.pile, hit.cardIdx) {
			return m, tea.Raw("\x07")
		}
		cmd := m.checkAutoCompleteOrWin()
		return m, cmd
	}

	// Stock click: draw (handled on click, not drag).
	if hit.pile.ptype == pileStock {
		m.clearSelection()
		m.dragging = false
		m.drawFromStock()
		cmd := m.checkAutoCompleteOrWin()
		return m, cmd
	}

	// If we already have a selection (from a previous click), try to place.
	if m.selected != nil {
		if m.tryPlace(hit.pile) {
			m.dragging = false
			cmd := m.checkAutoCompleteOrWin()
			return m, cmd
		}
		m.clearSelection()
		m.dragging = false
		// Fall through to re-select the clicked card below.
	}

	// Select card and start drag.
	if hit.cardIdx >= 0 {
		m.cursor = hit.pile
		if hit.pile.ptype == pileTableau {
			m.cursorCard = hit.cardIdx
		}
		m.selectCard(hit.pile, hit.cardIdx)
		m.dragging = true
		m.dragStartPile = hit.pile
		m.dragStartCard = hit.cardIdx
		m.dragX = mouse.X
		m.dragY = mouse.Y
	}

	return m, nil
}

func (m model) handleMouseRelease(mouse tea.Mouse) (tea.Model, tea.Cmd) {
	if !m.dragging || m.selected == nil {
		m.dragging = false
		return m, nil
	}
	hit := m.hitTest(mouse.X, mouse.Y)
	return m.handleDrop(hit)
}

func (m model) handleDrop(hit hitResult) (tea.Model, tea.Cmd) {
	m.dragging = false
	m.dragX = 0
	m.dragY = 0

	// Dropped on the same card we started on — keep selection (click-to-select).
	if hit.valid && hit.pile == m.dragStartPile && hit.cardIdx == m.dragStartCard {
		return m, nil
	}

	// Dropped on a valid target — try to place.
	if hit.valid {
		if m.tryPlace(hit.pile) {
			cmd := m.checkAutoCompleteOrWin()
			return m, cmd
		}
		// Invalid drop — bell and keep selection.
		return m, tea.Raw("\x07")
	}

	// Dropped outside — cancel selection.
	m.clearSelection()
	return m, nil
}

func (m *model) isDoubleClick(p pileID, cardIdx int) bool {
	now := time.Now()
	isDouble := now.Sub(m.lastClickTime).Milliseconds() < doubleClickMs &&
		m.lastClickPile == p &&
		m.lastClickCard == cardIdx
	m.lastClickTime = now
	m.lastClickPile = p
	m.lastClickCard = cardIdx
	return isDouble
}

// ── Hit testing ─────────────────────────────────────────────────────

const (
	cardW     = 7
	cardH     = 5
	colStride = 8 // cardW + 1 gap

	headerRows  = 2 // title + score line
	topRowStart = 2 // after header
	topRowH     = cardH
	tabStart    = topRowStart + topRowH + 1 // 1 gap row
)

type hitResult struct {
	pile    pileID
	cardIdx int
	valid   bool
}

func (m model) contentSize() (int, int) {
	w := 7 * colStride - 1 // 55
	h := tabStart + m.maxTableauHeight()
	if h < tabStart+cardH {
		h = tabStart + cardH
	}
	h += 3 // footer
	return w, h
}

func (m model) contentOrigin() (int, int) {
	cw, ch := m.contentSize()
	ox := (m.width - cw) / 2
	oy := (m.height - ch) / 2
	return ox, oy
}

func (m model) hitTest(mx, my int) hitResult {
	ox, oy := m.contentOrigin()
	rx, ry := mx-ox, my-oy

	if rx < 0 || ry < 0 {
		return hitResult{}
	}

	cw, _ := m.contentSize()
	if rx >= cw {
		return hitResult{}
	}

	col := rx / colStride
	inCard := rx%colStride < cardW
	if !inCard || col > 6 {
		return hitResult{}
	}

	// Top row (stock, waste, gap, foundations).
	if ry >= topRowStart && ry < topRowStart+topRowH {
		return m.topRowHit(col)
	}

	// Tableau.
	if ry >= tabStart && col < 7 {
		tabY := ry - tabStart
		cardIdx, ok := m.tableauHitTest(col, tabY)
		if ok {
			return hitResult{
				pile:    pileID{ptype: pileTableau, index: col},
				cardIdx: cardIdx,
				valid:   true,
			}
		}
		// Click on empty tableau column.
		if len(m.tableau[col]) == 0 {
			return hitResult{
				pile:    pileID{ptype: pileTableau, index: col},
				cardIdx: -1,
				valid:   true,
			}
		}
	}

	return hitResult{}
}

func (m model) topRowHit(col int) hitResult {
	switch col {
	case 0:
		return hitResult{pile: pileID{ptype: pileStock}, cardIdx: -1, valid: true}
	case 1:
		ci := -1
		if len(m.waste) > 0 {
			ci = len(m.waste) - 1
		}
		return hitResult{pile: pileID{ptype: pileWaste}, cardIdx: ci, valid: true}
	case 2:
		return hitResult{} // gap
	case 3, 4, 5, 6:
		fi := col - 3
		ci := -1
		if len(m.foundations[fi]) > 0 {
			ci = len(m.foundations[fi]) - 1
		}
		return hitResult{pile: pileID{ptype: pileFoundation, index: fi}, cardIdx: ci, valid: true}
	}
	return hitResult{}
}

func (m model) tableauHitTest(col, relY int) (int, bool) {
	tab := m.tableau[col]
	if len(tab) == 0 {
		return -1, false
	}

	y := 0
	for i, c := range tab {
		isLast := i == len(tab)-1
		var h int
		if !c.faceUp {
			h = 1
		} else if isLast {
			h = cardH
		} else {
			h = 2
		}
		if relY >= y && relY < y+h {
			if !c.faceUp {
				return -1, false // can't click face-down
			}
			return i, true
		}
		y += h
	}
	return -1, false
}

func (m model) maxTableauHeight() int {
	maxH := 0
	for col := 0; col < 7; col++ {
		h := tableauColHeight(m.tableau[col])
		if h > maxH {
			maxH = h
		}
	}
	return maxH
}

func tableauColHeight(tab []card) int {
	if len(tab) == 0 {
		return cardH
	}
	h := 0
	for i, c := range tab {
		if !c.faceUp {
			h++
		} else if i == len(tab)-1 {
			h += cardH
		} else {
			h += 2
		}
	}
	return h
}

// ── Rendering ───────────────────────────────────────────────────────

// Color palette (Catppuccin Mocha).
var (
	bgColor      = lipgloss.Color("#11111B")
	frameBg      = lipgloss.Color("#1E1E2E")
	cardFaceBg   = lipgloss.Color("#CDD6F4")
	cardFaceFg   = lipgloss.Color("#1E1E2E") // black text on white card
	cardBackBg   = lipgloss.Color("#1E3A5F")
	cardBackFg   = lipgloss.Color("#4A7DB5")
	redFg        = lipgloss.Color("#D20F39")
	blackFg      = lipgloss.Color("#1E1E2E")
	highlightBdr = lipgloss.Color("#6B0700")
	highlightBg  = lipgloss.Color("#A8B4C8")
	hoverBg      = lipgloss.Color("#B8C4DB")
	dropOkBg     = lipgloss.Color("#2E7D32") // green tint for valid drop
	dropOkFg     = lipgloss.Color("#A6E3A1") // green border for valid drop
	dropBadBg    = lipgloss.Color("#7D2E2E") // red tint for invalid drop
	dropBadFg    = lipgloss.Color("#F38BA8") // red border for invalid drop
	emptyFg      = lipgloss.Color("#45475A")
	titleFg      = lipgloss.Color("#89B4FA")
	textFg       = lipgloss.Color("#CDD6F4")
	dimFg        = lipgloss.Color("#6C7086")
	wonFg        = lipgloss.Color("#A6E3A1")
	cursorFg     = lipgloss.Color("#89B4FA")
)

func cardColor(c card) lipgloss.Color {
	if isRed(c.suit) {
		return redFg
	}
	return blackFg
}

func (m model) View() tea.View {
	var v tea.View
	v.AltScreen = true
	v.MouseMode = tea.MouseModeCellMotion

	if m.width == 0 || m.height == 0 {
		v.SetContent("Loading...")
		return v
	}

	var sb strings.Builder
	sb.Grow(m.width * m.height * 20)

	bst := lipgloss.NewStyle().Foreground(textFg).Background(frameBg)
	dst := lipgloss.NewStyle().Foreground(dimFg).Background(frameBg)
	tst := lipgloss.NewStyle().Foreground(titleFg).Background(frameBg).Bold(true)

	// Header.
	modeStr := "Draw 1"
	if m.drawMode == drawThree {
		modeStr = "Draw 3"
	}
	title := tst.Render("♠ Solitaire")
	sb.WriteString(title)
	sb.WriteByte('\n')

	scoreStr := fmt.Sprintf("Score: %d  Moves: %d  Time: %d:%02d  [%s]",
		m.score, m.moves, m.elapsed/60, m.elapsed%60, modeStr)
	sb.WriteString(dst.Render(scoreStr))
	sb.WriteByte('\n')

	// Top row: Stock | Waste | gap | Found0 | Found1 | Found2 | Found3.
	topRows := m.renderTopRow()
	for _, row := range topRows {
		sb.WriteString(row)
		sb.WriteByte('\n')
	}

	// Gap.
	sb.WriteString(bst.Render(strings.Repeat(" ", 7*colStride-1)))
	sb.WriteByte('\n')

	// Tableau.
	tabRows := m.renderTableau()
	for _, row := range tabRows {
		sb.WriteString(row)
		sb.WriteByte('\n')
	}

	// Win / status.
	if m.state == stateWon {
		wst := lipgloss.NewStyle().Foreground(wonFg).Background(frameBg).Bold(true)
		sb.WriteString(wst.Render(fmt.Sprintf("YOU WIN!  Score: %d", m.score)))
		sb.WriteByte('\n')
		sb.WriteString(dst.Render("Press 'r' for new game, 'q' to quit"))
	} else if m.state == stateAutoComplete {
		sb.WriteString(tst.Render("Auto-completing..."))
	} else {
		sb.WriteString(dst.Render("spc:select  d:draw  a:auto  m:mode  r:new  q:quit"))
	}

	content := sb.String()
	centered := lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, content,
		lipgloss.WithWhitespaceBackground(bgColor))

	// Overlay dragged card(s) at mouse position.
	if m.dragging && m.selected != nil {
		overlay := m.renderDragOverlay()
		if len(overlay) > 0 {
			lines := strings.Split(centered, "\n")
			col := m.dragX - cardW/2
			if col < 0 {
				col = 0
			}
			for i, oLine := range overlay {
				row := m.dragY + i
				if row >= 0 && row < len(lines) {
					lines[row] = spliceStyledLine(lines[row], col, cardW, oLine)
				}
			}
			v.SetContent(strings.Join(lines, "\n"))
			return v
		}
	}

	v.SetContent(centered)
	return v
}

// renderTopRow renders the 5-row top area (stock, waste, gap, foundations).
func (m model) renderTopRow() [cardH]string {
	var cols [7][cardH]string

	// Stock.
	stockPile := pileID{ptype: pileStock}
	stockCur := m.isCursorOn(stockPile, -1)
	stockHl := m.isCardHighlighted(stockPile, -1)
	stockHv := m.isCardHovered(stockPile, -1)
	if len(m.stock) > 0 {
		if (stockCur || stockHv) && !stockHl {
			cols[0] = renderCardBackHover()
		} else {
			cols[0] = renderCardBack(stockHl)
		}
	} else {
		if stockCur || stockHv {
			cols[0] = renderEmptyPileHover("↺")
		} else {
			cols[0] = renderEmptyPile("↺")
		}
	}

	// Waste.
	wp := pileID{ptype: pileWaste}
	if len(m.waste) > 0 {
		c := m.waste[len(m.waste)-1]
		ci := len(m.waste) - 1
		if m.isDraggedFrom(wp, ci) {
			cols[1] = renderCardGhost()
		} else {
			hl := m.isCardHighlighted(wp, ci)
			hv := m.isCardHovered(wp, ci)
			cur := m.isCursorOn(wp, ci)
			if cur && !hl {
				cols[1] = renderCardFullCursor(c)
			} else {
				cols[1] = renderCardFull(c, hl, hv)
			}
		}
	} else {
		if m.isCursorOn(wp, -1) || m.isCardHovered(wp, -1) {
			cols[1] = renderEmptyPileHover("")
		} else {
			cols[1] = renderEmptyPile("")
		}
	}

	// Gap (col 2).
	for row := 0; row < cardH; row++ {
		cols[2][row] = lipgloss.NewStyle().Background(frameBg).Render(strings.Repeat(" ", cardW))
	}

	// Foundations.
	suitLabels := [4]string{"♣", "♦", "♥", "♠"}
	for i := 0; i < 4; i++ {
		f := m.foundations[i]
		fp := pileID{ptype: pileFoundation, index: i}
		if len(f) > 0 {
			c := f[len(f)-1]
			ci := len(f) - 1
			if ds := m.dropTargetState(fp); ds != 0 {
				cols[3+i] = renderDropTargetCard(c, ds > 0)
			} else {
				hl := m.isCardHighlighted(fp, ci)
				hv := m.isCardHovered(fp, ci)
				cur := m.isCursorOn(fp, ci)
				if cur && !hl {
					cols[3+i] = renderCardFullCursor(c)
				} else {
					cols[3+i] = renderCardFull(c, hl, hv)
				}
			}
		} else {
			if ds := m.dropTargetState(fp); ds != 0 {
				cols[3+i] = renderDropTargetPile(suitLabels[i], ds > 0)
			} else if m.isCursorOn(fp, -1) || m.isCardHovered(fp, -1) {
				cols[3+i] = renderEmptyPileHover(suitLabels[i])
			} else {
				cols[3+i] = renderEmptyPile(suitLabels[i])
			}
		}
	}

	// Compose rows.
	gap := lipgloss.NewStyle().Background(frameBg).Render(" ")
	var rows [cardH]string
	for row := 0; row < cardH; row++ {
		var parts []string
		for col := 0; col < 7; col++ {
			parts = append(parts, cols[col][row])
			if col < 6 {
				parts = append(parts, gap)
			}
		}
		rows[row] = strings.Join(parts, "")
	}
	return rows
}

func (m model) isCardHighlighted(p pileID, cardIdx int) bool {
	if m.selected != nil {
		s := m.selected
		if s.pile == p {
			if p.ptype == pileTableau {
				return cardIdx >= s.cardIdx && cardIdx < s.cardIdx+s.count
			}
			return cardIdx == s.cardIdx
		}
	}
	return false
}

func (m model) isCursorOn(p pileID, cardIdx int) bool {
	if m.cursor == p {
		if p.ptype == pileTableau {
			return cardIdx == m.cursorCard
		}
		return true
	}
	return false
}

func (m model) isCardHovered(p pileID, cardIdx int) bool {
	if !m.hoverValid {
		return false
	}
	if m.hoverPile != p {
		return false
	}
	if p.ptype == pileTableau {
		return cardIdx >= m.hoverCard
	}
	return true
}

// dropTargetState returns 0 if not a drop target, 1 if valid drop, -1 if invalid drop.
func (m model) dropTargetState(p pileID) int {
	if !m.dragging || m.selected == nil || !m.hoverValid {
		return 0
	}
	if m.hoverPile != p || m.hoverPile == m.selected.pile {
		return 0
	}
	// Get the card being dragged.
	var dragCard card
	sel := m.selected
	switch sel.pile.ptype {
	case pileWaste:
		if len(m.waste) == 0 {
			return -1
		}
		dragCard = m.waste[len(m.waste)-1]
	case pileFoundation:
		f := m.foundations[sel.pile.index]
		if len(f) == 0 {
			return -1
		}
		dragCard = f[len(f)-1]
	case pileTableau:
		tab := m.tableau[sel.pile.index]
		if sel.cardIdx >= len(tab) {
			return -1
		}
		dragCard = tab[sel.cardIdx]
	default:
		return -1
	}
	// Check if the move is valid.
	switch p.ptype {
	case pileFoundation:
		if sel.count > 1 {
			return -1
		}
		if m.canMoveToFoundation(dragCard, p.index) {
			return 1
		}
		return -1
	case pileTableau:
		if m.canMoveToTableau(dragCard, p.index) {
			return 1
		}
		return -1
	}
	return -1
}

func (m model) isDraggedFrom(p pileID, cardIdx int) bool {
	if !m.dragging || m.selected == nil {
		return false
	}
	if m.selected.pile != p {
		return false
	}
	if p.ptype == pileTableau {
		return cardIdx >= m.selected.cardIdx
	}
	return cardIdx == m.selected.cardIdx
}

// renderTableau renders all 7 columns side by side.
func (m model) renderTableau() []string {
	maxH := m.maxTableauHeight()
	if maxH < cardH {
		maxH = cardH
	}

	// Pre-render each column as a slice of cardW-wide strings.
	var colLines [7][]string
	for col := 0; col < 7; col++ {
		colLines[col] = m.renderTableauCol(col, maxH)
	}

	gap := lipgloss.NewStyle().Background(frameBg).Render(" ")
	pad := lipgloss.NewStyle().Background(frameBg).Render(strings.Repeat(" ", cardW))

	rows := make([]string, maxH)
	for row := 0; row < maxH; row++ {
		var parts []string
		for col := 0; col < 7; col++ {
			if row < len(colLines[col]) {
				parts = append(parts, colLines[col][row])
			} else {
				parts = append(parts, pad)
			}
			if col < 6 {
				parts = append(parts, gap)
			}
		}
		rows[row] = strings.Join(parts, "")
	}
	return rows
}

func (m model) renderTableauCol(col, maxH int) []string {
	tab := m.tableau[col]
	pid := pileID{ptype: pileTableau, index: col}

	if len(tab) == 0 {
		if ds := m.dropTargetState(pid); ds != 0 {
			dt := renderDropTargetPile("K", ds > 0)
			return dt[:]
		}
		ep := renderEmptyPile("K")
		return ep[:]
	}

	var lines []string
	for i, c := range tab {
		isLast := i == len(tab)-1
		hl := m.isCardHighlighted(pid, i)
		hv := m.isCardHovered(pid, i)
		cur := m.isCursorOn(pid, i)
		dragged := m.isDraggedFrom(pid, i)

		if dragged {
			// Show ghost placeholder for dragged cards.
			if isLast {
				ghost := renderCardGhost()
				for _, r := range ghost {
					lines = append(lines, r)
				}
			} else {
				lines = append(lines, renderCardGhostPeek()...)
			}
			continue
		}

		if !c.faceUp {
			// Face-down: 1 row (top of card back).
			lines = append(lines, renderCardBackRow(cur))
		} else if isLast {
			// Top card: full 5 rows.
			if ds := m.dropTargetState(pid); isLast && ds != 0 {
				full := renderDropTargetCard(c, ds > 0)
				for _, r := range full {
					lines = append(lines, r)
				}
			} else {
				full := renderCardFull(c, hl, hv)
				if cur && !hl {
					full = renderCardFullCursor(c)
				}
				for _, r := range full {
					lines = append(lines, r)
				}
			}
		} else {
			// Peek: 2 rows.
			peek := renderCardPeek(c, hl, hv)
			if cur && !hl {
				peek = renderCardPeekCursor(c)
			}
			lines = append(lines, peek[0], peek[1])
		}
	}
	return lines
}

// ── Card rendering primitives ───────────────────────────────────────

func renderCardFull(c card, highlighted, hovered bool) [cardH]string {
	cc := cardColor(c)
	bdrFg := emptyFg
	bg := cardFaceBg
	if hovered && !highlighted {
		bg = hoverBg
	}
	if highlighted {
		bdrFg = highlightBdr
		bg = highlightBg
	}

	bdr := lipgloss.NewStyle().Foreground(bdrFg).Background(bg)
	txt := lipgloss.NewStyle().Foreground(cc).Background(bg)
	blank := lipgloss.NewStyle().Background(bg)

	r := rankString(c.rank)
	s := suitDisplay(c.suit)

	// 7 chars per row.
	// Row 0: ┌─────┐
	// Row 1: │A♠   │  or │10♠  │
	// Row 2: │  ♠  │
	// Row 3: │   ♠A│  or │  ♠10│
	// Row 4: └─────┘
	var rows [cardH]string
	rows[0] = bdr.Render("┌─────┐")
	rows[4] = bdr.Render("└─────┘")

	// Row 1: rank+suit, left-aligned within 5 chars.
	inner1 := r + s
	pad1 := 5 - len([]rune(r)) - 1
	rows[1] = bdr.Render("│") + txt.Render(inner1) + blank.Render(strings.Repeat(" ", pad1)) + bdr.Render("│")

	// Row 2: centered suit.
	rows[2] = bdr.Render("│") + blank.Render("  ") + txt.Render(s) + blank.Render("  ") + bdr.Render("│")

	// Row 3: suit+rank, right-aligned within 5 chars.
	inner3 := s + r
	pad3 := 5 - len([]rune(r)) - 1
	rows[3] = bdr.Render("│") + blank.Render(strings.Repeat(" ", pad3)) + txt.Render(inner3) + bdr.Render("│")

	return rows
}

func renderCardFullCursor(c card) [cardH]string {
	cc := cardColor(c)
	bg := hoverBg
	bdr := lipgloss.NewStyle().Foreground(emptyFg).Background(bg)
	txt := lipgloss.NewStyle().Foreground(cc).Background(bg)
	blank := lipgloss.NewStyle().Background(bg)

	r := rankString(c.rank)
	s := suitDisplay(c.suit)

	var rows [cardH]string
	rows[0] = bdr.Render("┌─────┐")
	rows[4] = bdr.Render("└─────┘")

	inner1 := r + s
	pad1 := 5 - len([]rune(r)) - 1
	rows[1] = bdr.Render("│") + txt.Render(inner1) + blank.Render(strings.Repeat(" ", pad1)) + bdr.Render("│")
	rows[2] = bdr.Render("│") + blank.Render("  ") + txt.Render(s) + blank.Render("  ") + bdr.Render("│")
	inner3 := s + r
	pad3 := 5 - len([]rune(r)) - 1
	rows[3] = bdr.Render("│") + blank.Render(strings.Repeat(" ", pad3)) + txt.Render(inner3) + bdr.Render("│")

	return rows
}

func renderCardPeek(c card, highlighted, hovered bool) [2]string {
	cc := cardColor(c)
	bdrFg := emptyFg
	bg := cardFaceBg
	if hovered && !highlighted {
		bg = hoverBg
	}
	if highlighted {
		bdrFg = highlightBdr
		bg = highlightBg
	}
	bdr := lipgloss.NewStyle().Foreground(bdrFg).Background(bg)
	txt := lipgloss.NewStyle().Foreground(cc).Background(bg)
	blank := lipgloss.NewStyle().Background(bg)

	r := rankString(c.rank)
	s := suitDisplay(c.suit)
	inner := r + s
	pad := 5 - len([]rune(r)) - 1

	return [2]string{
		bdr.Render("┌─────┐"),
		bdr.Render("│") + txt.Render(inner) + blank.Render(strings.Repeat(" ", pad)) + bdr.Render("│"),
	}
}

func renderCardPeekCursor(c card) [2]string {
	cc := cardColor(c)
	bg := hoverBg
	bdr := lipgloss.NewStyle().Foreground(emptyFg).Background(bg)
	txt := lipgloss.NewStyle().Foreground(cc).Background(bg)
	blank := lipgloss.NewStyle().Background(bg)

	r := rankString(c.rank)
	s := suitDisplay(c.suit)
	inner := r + s
	pad := 5 - len([]rune(r)) - 1

	return [2]string{
		bdr.Render("┌─────┐"),
		bdr.Render("│") + txt.Render(inner) + blank.Render(strings.Repeat(" ", pad)) + bdr.Render("│"),
	}
}

func renderCardBack(highlighted bool) [cardH]string {
	bdrFg := cardBackFg
	if highlighted {
		bdrFg = highlightBdr
	}
	bdr := lipgloss.NewStyle().Foreground(bdrFg).Background(cardBackBg)
	p1 := lipgloss.NewStyle().Foreground(lipgloss.Color("#5B9BD5")).Background(cardBackBg)
	p2 := lipgloss.NewStyle().Foreground(lipgloss.Color("#3A7CC0")).Background(cardBackBg)

	var rows [cardH]string
	rows[0] = bdr.Render("┌─────┐")
	rows[1] = bdr.Render("│") + p1.Render("▓░▓░▓") + bdr.Render("│")
	rows[2] = bdr.Render("│") + p2.Render("░▓░▓░") + bdr.Render("│")
	rows[3] = bdr.Render("│") + p1.Render("▓░▓░▓") + bdr.Render("│")
	rows[4] = bdr.Render("└─────┘")
	return rows
}

func renderCardBackHover() [cardH]string {
	bg := hoverBg
	bdr := lipgloss.NewStyle().Foreground(emptyFg).Background(bg)
	p1 := lipgloss.NewStyle().Foreground(lipgloss.Color("#5B9BD5")).Background(bg)
	p2 := lipgloss.NewStyle().Foreground(lipgloss.Color("#3A7CC0")).Background(bg)

	var rows [cardH]string
	rows[0] = bdr.Render("┌─────┐")
	rows[1] = bdr.Render("│") + p1.Render("▓░▓░▓") + bdr.Render("│")
	rows[2] = bdr.Render("│") + p2.Render("░▓░▓░") + bdr.Render("│")
	rows[3] = bdr.Render("│") + p1.Render("▓░▓░▓") + bdr.Render("│")
	rows[4] = bdr.Render("└─────┘")
	return rows
}

func renderCardBackRow(cursor bool) string {
	bdrFg := cardBackFg
	if cursor {
		bdrFg = cursorFg
	}
	bdr := lipgloss.NewStyle().Foreground(bdrFg).Background(cardBackBg)
	pat := lipgloss.NewStyle().Foreground(lipgloss.Color("#5B9BD5")).Background(cardBackBg)
	return bdr.Render("│") + pat.Render("▓░▓░▓") + bdr.Render("│")
}

// spliceStyledLine replaces visual columns [col, col+width) in an ANSI-styled
// line with the overlay string.
func spliceStyledLine(line string, col, width int, overlay string) string {
	runes := []rune(line)
	var prefix strings.Builder
	visualCol := 0
	i := 0

	// Phase 1: copy everything up to visual column `col`.
	for i < len(runes) {
		if runes[i] == '\x1b' && i+1 < len(runes) && runes[i+1] == '[' {
			// CSI sequence — copy until final byte (0x40-0x7E).
			for i < len(runes) {
				prefix.WriteRune(runes[i])
				ch := runes[i]
				i++
				if ch >= 0x40 && ch <= 0x7E && ch != '[' {
					break
				}
			}
			continue
		}
		if visualCol >= col {
			break
		}
		prefix.WriteRune(runes[i])
		visualCol++
		i++
	}

	// Phase 2: skip `width` visual characters in original line.
	skipped := 0
	for i < len(runes) && skipped < width {
		if runes[i] == '\x1b' && i+1 < len(runes) && runes[i+1] == '[' {
			for i < len(runes) {
				ch := runes[i]
				i++
				if ch >= 0x40 && ch <= 0x7E && ch != '[' {
					break
				}
			}
			continue
		}
		skipped++
		i++
	}

	// Phase 3: collect suffix.
	var suffix strings.Builder
	for i < len(runes) {
		suffix.WriteRune(runes[i])
		i++
	}

	return prefix.String() + "\x1b[0m" + overlay + "\x1b[0m" + suffix.String()
}

// renderDragOverlay returns the lines for the floating dragged card(s).
func (m model) renderDragOverlay() []string {
	if m.selected == nil {
		return nil
	}
	sel := m.selected
	var lines []string
	switch sel.pile.ptype {
	case pileWaste:
		if len(m.waste) > 0 {
			c := m.waste[len(m.waste)-1]
			full := renderCardFull(c, true, false)
			lines = append(lines, full[:]...)
		}
	case pileFoundation:
		f := m.foundations[sel.pile.index]
		if len(f) > 0 {
			c := f[len(f)-1]
			full := renderCardFull(c, true, false)
			lines = append(lines, full[:]...)
		}
	case pileTableau:
		tab := m.tableau[sel.pile.index]
		for i := sel.cardIdx; i < len(tab); i++ {
			c := tab[i]
			isLast := i == len(tab)-1
			if isLast {
				full := renderCardFull(c, true, false)
				lines = append(lines, full[:]...)
			} else {
				peek := renderCardPeek(c, true, false)
				lines = append(lines, peek[0], peek[1])
			}
		}
	}
	return lines
}

func renderCardGhost() [cardH]string {
	st := lipgloss.NewStyle().Foreground(dimFg).Background(frameBg)
	var rows [cardH]string
	rows[0] = st.Render("┌─────┐")
	rows[1] = st.Render("│     │")
	rows[2] = st.Render("│     │")
	rows[3] = st.Render("│     │")
	rows[4] = st.Render("└─────┘")
	return rows
}

func renderCardGhostPeek() []string {
	st := lipgloss.NewStyle().Foreground(dimFg).Background(frameBg)
	return []string{
		st.Render("┌─────┐"),
		st.Render("│     │"),
	}
}

func renderEmptyPile(label string) [cardH]string {
	st := lipgloss.NewStyle().Foreground(emptyFg).Background(frameBg)
	blank := lipgloss.NewStyle().Background(frameBg)
	var rows [cardH]string
	rows[0] = st.Render("┌─────┐")
	rows[4] = st.Render("└─────┘")

	// Center label.
	lw := len([]rune(label))
	if lw == 0 {
		rows[1] = st.Render("│") + blank.Render("     ") + st.Render("│")
		rows[2] = st.Render("│") + blank.Render("     ") + st.Render("│")
		rows[3] = st.Render("│") + blank.Render("     ") + st.Render("│")
	} else {
		lpad := (5 - lw) / 2
		rpad := 5 - lw - lpad
		rows[1] = st.Render("│") + blank.Render("     ") + st.Render("│")
		rows[2] = st.Render("│") + blank.Render(strings.Repeat(" ", lpad)) + st.Render(label) + blank.Render(strings.Repeat(" ", rpad)) + st.Render("│")
		rows[3] = st.Render("│") + blank.Render("     ") + st.Render("│")
	}
	return rows
}

func renderEmptyPileHover(label string) [cardH]string {
	bg := hoverBg
	st := lipgloss.NewStyle().Foreground(emptyFg).Background(bg)
	blank := lipgloss.NewStyle().Background(bg)
	var rows [cardH]string
	rows[0] = st.Render("┌─────┐")
	rows[4] = st.Render("└─────┘")

	lw := len([]rune(label))
	if lw == 0 {
		rows[1] = st.Render("│") + blank.Render("     ") + st.Render("│")
		rows[2] = st.Render("│") + blank.Render("     ") + st.Render("│")
		rows[3] = st.Render("│") + blank.Render("     ") + st.Render("│")
	} else {
		lpad := (5 - lw) / 2
		rpad := 5 - lw - lpad
		rows[1] = st.Render("│") + blank.Render("     ") + st.Render("│")
		rows[2] = st.Render("│") + blank.Render(strings.Repeat(" ", lpad)) + st.Render(label) + blank.Render(strings.Repeat(" ", rpad)) + st.Render("│")
		rows[3] = st.Render("│") + blank.Render("     ") + st.Render("│")
	}
	return rows
}

func renderDropTargetPile(label string, valid bool) [cardH]string {
	fg, bg := dropOkFg, dropOkBg
	if !valid {
		fg, bg = dropBadFg, dropBadBg
	}
	st := lipgloss.NewStyle().Foreground(fg).Background(bg)
	blank := lipgloss.NewStyle().Background(bg)
	var rows [cardH]string
	rows[0] = st.Render("┌─────┐")
	rows[4] = st.Render("└─────┘")

	lw := len([]rune(label))
	if lw == 0 {
		rows[1] = st.Render("│") + blank.Render("     ") + st.Render("│")
		rows[2] = st.Render("│") + blank.Render("     ") + st.Render("│")
		rows[3] = st.Render("│") + blank.Render("     ") + st.Render("│")
	} else {
		lpad := (5 - lw) / 2
		rpad := 5 - lw - lpad
		rows[1] = st.Render("│") + blank.Render("     ") + st.Render("│")
		rows[2] = st.Render("│") + blank.Render(strings.Repeat(" ", lpad)) + st.Render(label) + blank.Render(strings.Repeat(" ", rpad)) + st.Render("│")
		rows[3] = st.Render("│") + blank.Render("     ") + st.Render("│")
	}
	return rows
}

func renderDropTargetCard(c card, valid bool) [cardH]string {
	cc := cardColor(c)
	fg, bg := dropOkFg, dropOkBg
	if !valid {
		fg, bg = dropBadFg, dropBadBg
	}
	bdr := lipgloss.NewStyle().Foreground(fg).Background(bg)
	txt := lipgloss.NewStyle().Foreground(cc).Background(bg)
	blank := lipgloss.NewStyle().Background(bg)

	r := rankString(c.rank)
	s := suitDisplay(c.suit)

	var rows [cardH]string
	rows[0] = bdr.Render("┌─────┐")
	rows[4] = bdr.Render("└─────┘")

	inner1 := r + s
	pad1 := 5 - len([]rune(r)) - 1
	rows[1] = bdr.Render("│") + txt.Render(inner1) + blank.Render(strings.Repeat(" ", pad1)) + bdr.Render("│")
	rows[2] = bdr.Render("│") + blank.Render("  ") + txt.Render(s) + blank.Render("  ") + bdr.Render("│")
	inner3 := s + r
	pad3 := 5 - len([]rune(r)) - 1
	rows[3] = bdr.Render("│") + blank.Render(strings.Repeat(" ", pad3)) + txt.Render(inner3) + bdr.Render("│")

	return rows
}

// ── State persistence ───────────────────────────────────────────────

type cardJSON struct {
	R int  `json:"r"`
	S int  `json:"s"`
	F bool `json:"f"`
}

type solitaireJSON struct {
	Stock  []cardJSON    `json:"st"`
	Waste  []cardJSON    `json:"w"`
	Found  [4][]cardJSON `json:"fn"`
	Tab    [7][]cardJSON `json:"tb"`
	Draw   int           `json:"dm"`
	Score  int           `json:"sc"`
	Moves  int           `json:"mv"`
	Elapsed int          `json:"el"`
	State  int           `json:"gs"`
	CurP   int           `json:"cp"`
	CurI   int           `json:"ci"`
	CurC   int           `json:"cc"`
	Started bool         `json:"sd"`
}

func cardsToJSON(cards []card) []cardJSON {
	out := make([]cardJSON, len(cards))
	for i, c := range cards {
		out[i] = cardJSON{R: c.rank, S: int(c.suit), F: c.faceUp}
	}
	return out
}

func jsonToCards(jc []cardJSON) []card {
	out := make([]card, len(jc))
	for i, j := range jc {
		out[i] = card{rank: j.R, suit: suit(j.S), faceUp: j.F}
	}
	return out
}

func (m *model) dumpState() {
	js := solitaireJSON{
		Stock:   cardsToJSON(m.stock),
		Waste:   cardsToJSON(m.waste),
		Draw:    int(m.drawMode),
		Score:   m.score,
		Moves:   m.moves,
		Elapsed: m.elapsed,
		State:   int(m.state),
		CurP:    int(m.cursor.ptype),
		CurI:    m.cursor.index,
		CurC:    m.cursorCard,
		Started: m.started,
	}
	for i := 0; i < 4; i++ {
		js.Found[i] = cardsToJSON(m.foundations[i])
	}
	for i := 0; i < 7; i++ {
		js.Tab[i] = cardsToJSON(m.tableau[i])
	}
	if data, err := json.Marshal(js); err == nil {
		encoded := base64.StdEncoding.EncodeToString(data)
		fmt.Fprintf(os.Stdout, "\x1b]667;state-response;%s\x07", encoded)
	}
}

func (m *model) restoreState(data []byte) {
	var js solitaireJSON
	if err := json.Unmarshal(data, &js); err != nil {
		return
	}
	m.stock = jsonToCards(js.Stock)
	m.waste = jsonToCards(js.Waste)
	for i := 0; i < 4; i++ {
		m.foundations[i] = jsonToCards(js.Found[i])
	}
	for i := 0; i < 7; i++ {
		m.tableau[i] = jsonToCards(js.Tab[i])
	}
	m.drawMode = drawMode(js.Draw)
	m.score = js.Score
	m.moves = js.Moves
	m.elapsed = js.Elapsed
	m.state = gameState(js.State)
	m.cursor = pileID{ptype: pileType(js.CurP), index: js.CurI}
	m.cursorCard = js.CurC
	m.started = js.Started
	if m.started && m.state == statePlaying {
		m.startTime = time.Now().Add(-time.Duration(js.Elapsed) * time.Second)
	}
}

// ── Exported test helpers ───────────────────────────────────────────

func NewTestModel(seed uint64) model      { return newModelWithSeed(seed) }
func (m *model) GetStock() []card          { return m.stock }
func (m *model) GetWaste() []card          { return m.waste }
func (m *model) GetFoundations() [4][]card { return m.foundations }
func (m *model) GetTableau() [7][]card     { return m.tableau }
func (m *model) GetScore() int             { return m.score }
func (m *model) GetState() gameState       { return m.state }
func (m *model) GetDrawMode() drawMode     { return m.drawMode }

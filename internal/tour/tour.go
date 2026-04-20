package tour

// Step represents a single step in the guided tour.
type Step struct {
	Title string
	Body  string
}

// Tour manages the first-run guided tour state.
type Tour struct {
	Steps   []Step
	Current int
	Active  bool
}

var defaultSteps = []Step{
	{Title: "Welcome to Termdesk!", Body: "A retro TUI desktop environment.\nLet's take a quick tour."},
	{Title: "Open a Terminal", Body: "Press N to create a new terminal window."},
	{Title: "Enter Terminal Mode", Body: "Press I to type inside the terminal.\nYour keystrokes go to the shell."},
	{Title: "Return to Normal Mode", Body: "Press Ctrl+A then Esc to exit\nTerminal mode and manage windows."},
	{Title: "Try the Launcher", Body: "Press Ctrl+Space to open the\napp launcher and search for apps."},
	{Title: "Split Panes", Body: "In Terminal mode, press Ctrl+A then %\nto split horizontally, or \" vertically.\nUse o to cycle panes, x to close one."},
	{Title: "You're Ready!", Body: "Press F1 for help anytime.\nEnjoy Termdesk!"},
}

// New creates a new tour. If tourCompleted is true, the tour
// will not be active.
func New(tourCompleted bool) *Tour {
	return &Tour{
		Steps:   defaultSteps,
		Current: 0,
		Active:  !tourCompleted,
	}
}

// Next advances to the next step. Returns false if the tour is done.
func (t *Tour) Next() bool {
	if !t.Active {
		return false
	}
	t.Current++
	if t.Current >= len(t.Steps) {
		t.Active = false
		return false
	}
	return true
}

// Skip ends the tour immediately.
func (t *Tour) Skip() {
	t.Active = false
}

// CurrentStep returns the current step, or nil if the tour is not active.
func (t *Tour) CurrentStep() *Step {
	if !t.Active || t.Current < 0 || t.Current >= len(t.Steps) {
		return nil
	}
	return &t.Steps[t.Current]
}

// IsLast returns true if on the last step.
func (t *Tour) IsLast() bool {
	return t.Current == len(t.Steps)-1
}

// StepInfo returns "Step X/Y".
func (t *Tour) StepInfo() string {
	if !t.Active {
		return ""
	}
	return string(rune('0'+t.Current+1)) + "/" + string(rune('0'+len(t.Steps)))
}

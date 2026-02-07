package app

// WindowResizedMsg is sent when the terminal window is resized.
type WindowResizedMsg struct {
	Width  int
	Height int
}

// PtyOutputMsg signals that a PTY has produced output.
type PtyOutputMsg struct {
	WindowID string
}

// PtyClosedMsg signals that a PTY session has ended.
type PtyClosedMsg struct {
	WindowID string
	Err      error
}

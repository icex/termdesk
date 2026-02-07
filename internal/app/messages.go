package app

// WindowResizedMsg is sent when the terminal window is resized.
type WindowResizedMsg struct {
	Width  int
	Height int
}

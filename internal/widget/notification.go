package widget

import "fmt"

// NotificationWidget displays unread notification count with a bell icon.
type NotificationWidget struct {
	UnreadCount int
}

func (w *NotificationWidget) Name() string { return "notification" }

func (w *NotificationWidget) Render() string {
	if w.UnreadCount == 0 {
		return "\U000f009b" // nf-md-bell_outline
	}
	return fmt.Sprintf("\U000f009a %d", w.UnreadCount) // nf-md-bell + count
}

func (w *NotificationWidget) ColorLevel() string {
	switch {
	case w.UnreadCount >= 4:
		return "red"
	case w.UnreadCount >= 1:
		return "yellow"
	default:
		return ""
	}
}

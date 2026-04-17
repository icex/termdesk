package notification

// HistoryItems returns all notifications from newest to oldest.
func (m *Manager) HistoryItems() []Notification {
	out := make([]Notification, len(m.history))
	for i, n := range m.history {
		out[len(m.history)-1-i] = n
	}
	return out
}

// UnreadCount returns the number of unread notifications.
func (m *Manager) UnreadCount() int {
	count := 0
	for _, n := range m.history {
		if !n.Read {
			count++
		}
	}
	return count
}

// VisibleToasts returns the current visible toasts.
func (m *Manager) VisibleToasts() []Notification {
	out := make([]Notification, len(m.visible))
	copy(out, m.visible)
	return out
}

// CenterVisible returns whether the notification center is shown.
func (m *Manager) CenterVisible() bool {
	return m.centerVisible
}

// CenterIndex returns the current selection index in the center.
func (m *Manager) CenterIndex() int {
	return m.centerIdx
}

// History returns the notification history slice.
func (m *Manager) History() []Notification {
	return m.history
}

// String returns the string representation of a Severity.
func (s Severity) String() string {
	switch s {
	case Info:
		return "info"
	case Warning:
		return "warning"
	case Error:
		return "error"
	default:
		return "info"
	}
}

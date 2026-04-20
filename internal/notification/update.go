package notification

import "time"

// Push adds a notification to both visible toasts and history.
func (m *Manager) Push(title, body string, sev Severity) {
	n := Notification{
		ID:        m.nextID,
		Title:     title,
		Body:      body,
		Severity:  sev,
		CreatedAt: time.Now(),
	}
	m.nextID++

	// Add to history (capped)
	m.history = append(m.history, n)
	if len(m.history) > historyCapacity {
		m.history = m.history[len(m.history)-historyCapacity:]
	}

	// Add to visible (max 3, drop oldest if needed)
	m.visible = append(m.visible, n)
	if len(m.visible) > maxVisible {
		m.visible = m.visible[len(m.visible)-maxVisible:]
	}
}

// Tick checks for expired toasts and returns their IDs.
func (m *Manager) Tick() []int {
	now := time.Now()
	var dismissed []int
	var remaining []Notification
	for _, n := range m.visible {
		if now.Sub(n.CreatedAt) >= toastDuration {
			dismissed = append(dismissed, n.ID)
		} else {
			remaining = append(remaining, n)
		}
	}
	m.visible = remaining
	return dismissed
}

// Dismiss removes a notification from visible toasts by ID.
func (m *Manager) Dismiss(id int) {
	for i, n := range m.visible {
		if n.ID == id {
			m.visible = append(m.visible[:i], m.visible[i+1:]...)
			return
		}
	}
}

// Cleanup removes old notifications, keeping only the most recent maxKeep.
func (m *Manager) Cleanup(maxKeep int) {
	if len(m.history) > maxKeep {
		m.history = m.history[len(m.history)-maxKeep:]
	}
}

// MarkAllRead marks all history items as read.
func (m *Manager) MarkAllRead() {
	for i := range m.history {
		m.history[i].Read = true
	}
}

// ShowCenter opens the notification center.
func (m *Manager) ShowCenter() {
	m.centerVisible = true
	m.centerIdx = 0
	m.MarkAllRead()
}

// HideCenter closes the notification center.
func (m *Manager) HideCenter() {
	m.centerVisible = false
}

// ToggleCenter toggles the notification center visibility.
func (m *Manager) ToggleCenter() {
	if m.centerVisible {
		m.HideCenter()
	} else {
		m.ShowCenter()
	}
}

// MoveCenterSelection moves the center selection by delta, wrapping.
func (m *Manager) MoveCenterSelection(delta int) {
	count := len(m.history)
	if count == 0 {
		return
	}
	m.centerIdx += delta
	if m.centerIdx < 0 {
		m.centerIdx = count - 1
	}
	if m.centerIdx >= count {
		m.centerIdx = 0
	}
}

// DeleteFromHistory removes a notification from history by ID.
func (m *Manager) DeleteFromHistory(id int) {
	for i, n := range m.history {
		if n.ID == id {
			m.history = append(m.history[:i], m.history[i+1:]...)
			// Adjust center index if needed
			if m.centerIdx >= len(m.history) && m.centerIdx > 0 {
				m.centerIdx = len(m.history) - 1
			}
			return
		}
	}
}

// ClearHistory removes all history and visible toasts.
func (m *Manager) ClearHistory() {
	m.history = nil
	m.visible = nil
	m.centerIdx = 0
}

// RestoreNotification adds a notification to history (for workspace restore).
func (m *Manager) RestoreNotification(title, body, severity string, createdAt time.Time, read bool) {
	sev := Info
	switch severity {
	case "warning":
		sev = Warning
	case "error":
		sev = Error
	}

	n := Notification{
		ID:        m.nextID,
		Title:     title,
		Body:      body,
		Severity:  sev,
		CreatedAt: createdAt,
		Read:      read,
	}
	m.nextID++
	m.history = append(m.history, n)
	if len(m.history) > historyCapacity {
		m.history = m.history[len(m.history)-historyCapacity:]
	}
}

package notification

import "time"

const (
	historyCapacity = 20
	maxVisible      = 3
	toastDuration   = 4 * time.Second
)

// Severity levels for notifications.
type Severity int

const (
	Info Severity = iota
	Warning
	Error
)

// Notification is a single notification entry.
type Notification struct {
	ID        int
	Title     string
	Body      string
	Severity  Severity
	CreatedAt time.Time
	Read      bool
}

// Manager manages toast notifications and a history ring buffer.
type Manager struct {
	history       []Notification // oldest-first, capped at historyCapacity
	visible       []Notification // currently showing toasts (max 3)
	nextID        int
	centerVisible bool
	centerIdx     int
}

// New creates a new notification manager.
func New() *Manager {
	return &Manager{}
}

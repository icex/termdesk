package session

// Adapter abstracts session operations for UI/CLI wiring.
type Adapter interface {
	Attach(name string) error
	ListSessions() ([]SessionInfo, error)
	SessionExists(name string) bool
	NewServer(name string, cols, rows int, opts ServerOptions) (*Server, error)
	SocketDir() string
	EnsureSocketDir() error
	PidPath(name string) string
}

type defaultAdapter struct{}

// NewAdapter returns the default session adapter.
func NewAdapter() Adapter {
	return defaultAdapter{}
}

func (defaultAdapter) Attach(name string) error {
	return Attach(name)
}

func (defaultAdapter) ListSessions() ([]SessionInfo, error) {
	return ListSessions()
}

func (defaultAdapter) SessionExists(name string) bool {
	return SessionExists(name)
}

func (defaultAdapter) NewServer(name string, cols, rows int, opts ServerOptions) (*Server, error) {
	return NewServer(name, cols, rows, opts)
}

func (defaultAdapter) SocketDir() string {
	return SocketDir()
}

func (defaultAdapter) EnsureSocketDir() error {
	return EnsureSocketDir()
}

func (defaultAdapter) PidPath(name string) string {
	return PidPath(name)
}

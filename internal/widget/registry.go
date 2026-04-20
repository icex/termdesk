package widget

// WidgetMeta describes a registered widget type.
type WidgetMeta struct {
	Name    string // unique identifier: "cpu", "mem", "battery", etc.
	Label   string // human-readable: "CPU Monitor", "Memory", etc.
	Builtin bool   // true for built-in widgets
}

// Registry holds all known widget types.
type Registry struct {
	metas  []WidgetMeta
	byName map[string]WidgetMeta
}

// NewRegistry creates an empty widget registry.
func NewRegistry() *Registry {
	return &Registry{
		byName: make(map[string]WidgetMeta),
	}
}

// Register adds a widget type to the registry. Duplicates are ignored.
func (r *Registry) Register(meta WidgetMeta) {
	if _, exists := r.byName[meta.Name]; exists {
		return
	}
	r.metas = append(r.metas, meta)
	r.byName[meta.Name] = meta
}

// All returns all registered widget metadata in registration order.
func (r *Registry) All() []WidgetMeta {
	out := make([]WidgetMeta, len(r.metas))
	copy(out, r.metas)
	return out
}

// Has returns true if a widget with the given name is registered.
func (r *Registry) Has(name string) bool {
	_, ok := r.byName[name]
	return ok
}

// Get returns the metadata for a widget by name, or empty WidgetMeta if not found.
func (r *Registry) Get(name string) (WidgetMeta, bool) {
	m, ok := r.byName[name]
	return m, ok
}

// DefaultRegistry creates a registry with all built-in widgets.
func DefaultRegistry() *Registry {
	r := NewRegistry()
	r.Register(WidgetMeta{Name: "cpu", Label: "CPU Monitor", Builtin: true})
	r.Register(WidgetMeta{Name: "mem", Label: "Memory", Builtin: true})
	r.Register(WidgetMeta{Name: "battery", Label: "Battery", Builtin: true})
	r.Register(WidgetMeta{Name: "notification", Label: "Notifications", Builtin: true})
	r.Register(WidgetMeta{Name: "workspace", Label: "Workspace", Builtin: true})
	r.Register(WidgetMeta{Name: "hostname", Label: "Hostname", Builtin: true})
	r.Register(WidgetMeta{Name: "user", Label: "User", Builtin: true})
	r.Register(WidgetMeta{Name: "clock", Label: "Clock", Builtin: true})
	r.Register(WidgetMeta{Name: "disk", Label: "Disk Usage", Builtin: true})
	r.Register(WidgetMeta{Name: "load", Label: "Load Average", Builtin: true})
	r.Register(WidgetMeta{Name: "git", Label: "Git Branch", Builtin: true})
	r.Register(WidgetMeta{Name: "docker", Label: "Docker", Builtin: true})
	r.Register(WidgetMeta{Name: "weather", Label: "Weather", Builtin: true})
	r.Register(WidgetMeta{Name: "mail", Label: "Mail", Builtin: true})
	return r
}

// DefaultEnabledWidgets returns the default enabled widget names in order.
func DefaultEnabledWidgets() []string {
	return []string{"cpu", "mem", "battery", "notification", "workspace", "hostname", "user", "clock"}
}

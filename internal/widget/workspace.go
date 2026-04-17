package widget

// WorkspaceWidget displays the current workspace/project name in the menu bar.
type WorkspaceWidget struct {
	DisplayName string // workspace display name (e.g., "termdesk", "Default")
}

func (w *WorkspaceWidget) Name() string       { return "workspace" }
func (w *WorkspaceWidget) Render() string     { return "\uf07c " + w.DisplayName }
func (w *WorkspaceWidget) ColorLevel() string { return "" }

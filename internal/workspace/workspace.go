package workspace

import (
	"compress/gzip"
	"encoding/base64"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/icex/termdesk/pkg/geometry"
)

const envHomeDir = "TERMDESK_HOME"

func homeDir() string {
	if override := strings.TrimSpace(os.Getenv(envHomeDir)); override != "" {
		return override
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return home
}

// WorkspaceState represents the complete state of the workspace.
type WorkspaceState struct {
	Version       int                 // schema version (1)
	SavedAt       time.Time           // when this was saved
	Windows       []WindowState       // all windows
	FocusedID     string              // ID of focused window
	Clipboard     []string            // clipboard history
	Notifications []NotificationState // notification history
}

// WindowState represents a single window's serializable state.
type WindowState struct {
	ID         string
	Title      string
	Command    string
	Args       []string // command arguments
	WorkDir    string   // working directory
	X          int      // position
	Y          int
	Width      int
	Height     int
	ZIndex     int
	Minimized  bool
	Resizable  bool           // false for fixed-size windows (default true)
	PreMaxRect *geometry.Rect // nil if not maximized

	// Buffer stored externally as gzip file in ~/.config/termdesk/buffers/
	BufferFile string // window ID reference to .buf.gz file
	BufferRows int    // buffer dimensions at capture
	BufferCols int

	// App state persistence (SIGUSR1/OSC 667 protocol)
	AppStateData string // base64-encoded JSON app state

	// Split pane layout (empty for unsplit windows)
	SplitTree   string      // encoded split tree (window.EncodeSplitTree format)
	FocusedPane string      // focused pane terminal ID
	Panes       []PaneState // per-pane terminal data (only for split windows)
}

// PaneState represents a single pane's terminal state in a split window.
type PaneState struct {
	TermID     string
	Command    string
	Args       []string
	WorkDir    string
	BufferFile string // external gzip buffer file ID
	BufferRows int
	BufferCols int
}

// buffersDir returns the directory for external buffer files.
func buffersDir() string {
	home := homeDir()
	if home == "" {
		return ""
	}
	return filepath.Join(home, ".config", "termdesk", "buffers")
}

// SaveBuffer writes terminal buffer content to a gzip file.
// Returns the buffer file ID (window ID) or "" on error.
func SaveBuffer(windowID, content string) string {
	if content == "" {
		return ""
	}
	dir := buffersDir()
	if dir == "" {
		return ""
	}
	os.MkdirAll(dir, 0755)

	path := filepath.Join(dir, windowID+".buf.gz")
	f, err := os.Create(path)
	if err != nil {
		return ""
	}
	defer f.Close()

	gz := gzip.NewWriter(f)
	if _, err := gz.Write([]byte(content)); err != nil {
		gz.Close()
		os.Remove(path)
		return ""
	}
	if err := gz.Close(); err != nil {
		os.Remove(path)
		return ""
	}
	return windowID
}

// LoadBuffer reads a gzip buffer file by window ID.
// Returns "" if file doesn't exist or on error.
func LoadBuffer(windowID string) string {
	dir := buffersDir()
	if dir == "" {
		return ""
	}
	path := filepath.Join(dir, windowID+".buf.gz")
	f, err := os.Open(path)
	if err != nil {
		return ""
	}
	defer f.Close()

	gz, err := gzip.NewReader(f)
	if err != nil {
		return ""
	}
	defer gz.Close()

	data, err := io.ReadAll(io.LimitReader(gz, 2*1024*1024)) // 2MB limit
	if err != nil {
		return ""
	}
	return string(data)
}

// CleanupBuffers removes buffer files not referenced by any window ID in the set.
func CleanupBuffers(activeIDs map[string]bool) {
	dir := buffersDir()
	if dir == "" {
		return
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if !strings.HasSuffix(name, ".buf.gz") {
			continue
		}
		id := strings.TrimSuffix(name, ".buf.gz")
		if !activeIDs[id] {
			os.Remove(filepath.Join(dir, name))
		}
	}
}

// NotificationState represents a notification for persistence.
type NotificationState struct {
	Title     string
	Body      string
	Severity  string // "info", "warning", "error"
	CreatedAt time.Time
	Read      bool
}

// GetWorkspacePath returns the workspace file path for a project directory.
// If projectDir is "", returns global workspace path (~/.config/termdesk/workspace.toml).
// Otherwise returns {projectDir}/.termdesk-workspace.toml (workspace stored alongside project config)
func GetWorkspacePath(projectDir string) string {
	if projectDir == "" {
		// Global workspace (no project)
		home := homeDir()
		if home == "" {
			return ""
		}
		dir := filepath.Join(home, ".config", "termdesk")
		os.MkdirAll(dir, 0755)
		return filepath.Join(dir, "workspace.toml")
	}

	// Project-specific workspace stored in same directory as .termdesk.toml config
	return filepath.Join(projectDir, ".termdesk-workspace.toml")
}

// ProjectDirFromPath extracts the project directory from a workspace file path.
// Returns "" for the global workspace path (~/.config/termdesk/workspace.toml).
func ProjectDirFromPath(path string) string {
	base := filepath.Base(path)
	if base == "workspace.toml" {
		return "" // global workspace
	}
	// Project workspace: {dir}/.termdesk-workspace.toml
	return filepath.Dir(path)
}

// SaveWorkspace serializes and saves workspace state to disk.
func SaveWorkspace(state WorkspaceState, projectDir string) error {
	state.Version = 1
	state.SavedAt = time.Now()

	path := GetWorkspacePath(projectDir)
	if path == "" {
		return fmt.Errorf("could not determine workspace path")
	}

	content := serializeWorkspace(state)
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, []byte(content), 0644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

// LoadWorkspace loads workspace state from disk.
// Returns nil if file doesn't exist (first run).
// Automatically removes corrupted workspace files.
func LoadWorkspace(projectDir string) (*WorkspaceState, error) {
	path := GetWorkspacePath(projectDir)
	if path == "" {
		return nil, fmt.Errorf("could not determine workspace path")
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil // no saved workspace
		}
		return nil, err
	}

	// Check if file is corrupted (too large or contains invalid data)
	if len(data) > 10*1024*1024 { // 10MB limit
		// Corrupted - remove it
		os.Remove(path)
		return nil, fmt.Errorf("workspace file was corrupted (too large) and has been removed")
	}

	state := &WorkspaceState{}
	parseWorkspace(state, string(data))

	// Validate the parsed state
	if !isValidWorkspace(state) {
		// Corrupted - remove it
		os.Remove(path)
		return nil, fmt.Errorf("workspace file was corrupted (invalid structure) and has been removed")
	}

	return state, nil
}

// isValidWorkspace checks if a WorkspaceState is valid.
func isValidWorkspace(state *WorkspaceState) bool {
	if state == nil {
		return false
	}
	// Check version
	if state.Version != 1 {
		return false
	}
	// Check that saved time is reasonable (not zero, not too far in future)
	if state.SavedAt.IsZero() || state.SavedAt.After(time.Now().Add(24*time.Hour)) {
		return false
	}
	// All checks passed
	return true
}

// serializeWorkspace converts WorkspaceState to TOML format.
func serializeWorkspace(state WorkspaceState) string {
	var sb strings.Builder
	sb.WriteString("# Termdesk workspace state (auto-saved)\n\n")
	sb.WriteString(fmt.Sprintf("version = %d\n", state.Version))
	sb.WriteString(fmt.Sprintf("saved_at = \"%s\"\n", state.SavedAt.Format(time.RFC3339)))
	sb.WriteString(fmt.Sprintf("focused_id = \"%s\"\n\n", state.FocusedID))

	// Clipboard
	if len(state.Clipboard) > 0 {
		sb.WriteString("clipboard = [")
		for i, item := range state.Clipboard {
			if i > 0 {
				sb.WriteString(", ")
			}
			sb.WriteString(fmt.Sprintf("\"%s\"", escapeString(item)))
		}
		sb.WriteString("]\n\n")
	}

	// Windows
	for _, w := range state.Windows {
		sb.WriteString("[[window]]\n")
		sb.WriteString(fmt.Sprintf("id = \"%s\"\n", w.ID))
		sb.WriteString(fmt.Sprintf("title = \"%s\"\n", escapeString(w.Title)))
		sb.WriteString(fmt.Sprintf("command = \"%s\"\n", escapeString(w.Command)))
		if len(w.Args) > 0 {
			sb.WriteString("args = [")
			for i, arg := range w.Args {
				if i > 0 {
					sb.WriteString(", ")
				}
				sb.WriteString(fmt.Sprintf("\"%s\"", escapeString(arg)))
			}
			sb.WriteString("]\n")
		}
		sb.WriteString(fmt.Sprintf("workdir = \"%s\"\n", escapeString(w.WorkDir)))
		sb.WriteString(fmt.Sprintf("x = %d\n", w.X))
		sb.WriteString(fmt.Sprintf("y = %d\n", w.Y))
		sb.WriteString(fmt.Sprintf("width = %d\n", w.Width))
		sb.WriteString(fmt.Sprintf("height = %d\n", w.Height))
		sb.WriteString(fmt.Sprintf("zindex = %d\n", w.ZIndex))
		if w.Minimized {
			sb.WriteString("minimized = true\n")
		}
		if !w.Resizable {
			sb.WriteString("resizable = false\n")
		}
		if w.PreMaxRect != nil {
			sb.WriteString(fmt.Sprintf("premax_x = %d\n", w.PreMaxRect.X))
			sb.WriteString(fmt.Sprintf("premax_y = %d\n", w.PreMaxRect.Y))
			sb.WriteString(fmt.Sprintf("premax_width = %d\n", w.PreMaxRect.Width))
			sb.WriteString(fmt.Sprintf("premax_height = %d\n", w.PreMaxRect.Height))
		}
		// Buffer stored in external gzip file — just reference the ID here.
		if w.BufferFile != "" {
			sb.WriteString(fmt.Sprintf("buffer_file = \"%s\"\n", w.BufferFile))
			sb.WriteString(fmt.Sprintf("buffer_rows = %d\n", w.BufferRows))
			sb.WriteString(fmt.Sprintf("buffer_cols = %d\n", w.BufferCols))
		}
		// App state (already base64-encoded by the app)
		if w.AppStateData != "" {
			sb.WriteString(fmt.Sprintf("app_state = \"%s\"\n", w.AppStateData))
		}
		// Split pane layout
		if w.SplitTree != "" {
			sb.WriteString(fmt.Sprintf("split_tree = \"%s\"\n", escapeString(w.SplitTree)))
			sb.WriteString(fmt.Sprintf("focused_pane = \"%s\"\n", w.FocusedPane))
			sb.WriteString(fmt.Sprintf("pane_count = %d\n", len(w.Panes)))
			for i, p := range w.Panes {
				sb.WriteString(fmt.Sprintf("pane_%d_id = \"%s\"\n", i, p.TermID))
				sb.WriteString(fmt.Sprintf("pane_%d_command = \"%s\"\n", i, escapeString(p.Command)))
				if len(p.Args) > 0 {
					sb.WriteString(fmt.Sprintf("pane_%d_args = [", i))
					for j, arg := range p.Args {
						if j > 0 {
							sb.WriteString(", ")
						}
						sb.WriteString(fmt.Sprintf("\"%s\"", escapeString(arg)))
					}
					sb.WriteString("]\n")
				}
				sb.WriteString(fmt.Sprintf("pane_%d_workdir = \"%s\"\n", i, escapeString(p.WorkDir)))
				if p.BufferFile != "" {
					sb.WriteString(fmt.Sprintf("pane_%d_buffer_file = \"%s\"\n", i, p.BufferFile))
					sb.WriteString(fmt.Sprintf("pane_%d_buffer_rows = %d\n", i, p.BufferRows))
					sb.WriteString(fmt.Sprintf("pane_%d_buffer_cols = %d\n", i, p.BufferCols))
				}
			}
		}
		sb.WriteString("\n")
	}

	// Notifications
	for _, n := range state.Notifications {
		sb.WriteString("[[notification]]\n")
		sb.WriteString(fmt.Sprintf("title = \"%s\"\n", escapeString(n.Title)))
		sb.WriteString(fmt.Sprintf("body = \"%s\"\n", escapeString(n.Body)))
		sb.WriteString(fmt.Sprintf("severity = \"%s\"\n", n.Severity))
		sb.WriteString(fmt.Sprintf("created_at = \"%s\"\n", n.CreatedAt.Format(time.RFC3339)))
		if n.Read {
			sb.WriteString("read = true\n")
		}
		sb.WriteString("\n")
	}

	return sb.String()
}

// parseWorkspace parses TOML content into WorkspaceState.
// Uses simple line-by-line parsing similar to userconfig.go.
func parseWorkspace(state *WorkspaceState, content string) {
	lines := strings.Split(content, "\n")
	var currentWindow *WindowState
	var currentNotif *NotificationState

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		if line == "[[window]]" {
			if currentWindow != nil {
				state.Windows = append(state.Windows, *currentWindow)
			}
			currentWindow = &WindowState{Resizable: true}
			continue
		}

		if line == "[[notification]]" {
			if currentNotif != nil {
				state.Notifications = append(state.Notifications, *currentNotif)
			}
			currentNotif = &NotificationState{}
			continue
		}

		key, val, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		key = strings.TrimSpace(key)
		val = strings.TrimSpace(val)

		// Top-level fields
		if currentWindow == nil && currentNotif == nil {
			switch key {
			case "version":
				fmt.Sscanf(val, "%d", &state.Version)
			case "saved_at":
				state.SavedAt, _ = time.Parse(time.RFC3339, unescapeString(extractQuotedValue(val)))
			case "focused_id":
				state.FocusedID = unescapeString(extractQuotedValue(val))
			case "clipboard":
				state.Clipboard = parseStringArray(val)
			}
			continue
		}

		// Window fields
		if currentWindow != nil {
			switch key {
			case "id":
				currentWindow.ID = unescapeString(extractQuotedValue(val))
			case "title":
				currentWindow.Title = unescapeString(extractQuotedValue(val))
			case "command":
				currentWindow.Command = unescapeString(extractQuotedValue(val))
			case "args":
				currentWindow.Args = parseStringArray(val)
			case "workdir":
				currentWindow.WorkDir = unescapeString(extractQuotedValue(val))
			case "x":
				fmt.Sscanf(val, "%d", &currentWindow.X)
			case "y":
				fmt.Sscanf(val, "%d", &currentWindow.Y)
			case "width":
				fmt.Sscanf(val, "%d", &currentWindow.Width)
			case "height":
				fmt.Sscanf(val, "%d", &currentWindow.Height)
			case "zindex":
				fmt.Sscanf(val, "%d", &currentWindow.ZIndex)
			case "minimized":
				currentWindow.Minimized = (val == "true")
			case "resizable":
				currentWindow.Resizable = (val == "true")
			case "premax_x", "premax_y", "premax_width", "premax_height":
				if currentWindow.PreMaxRect == nil {
					currentWindow.PreMaxRect = &geometry.Rect{}
				}
				var v int
				fmt.Sscanf(val, "%d", &v)
				switch key {
				case "premax_x":
					currentWindow.PreMaxRect.X = v
				case "premax_y":
					currentWindow.PreMaxRect.Y = v
				case "premax_width":
					currentWindow.PreMaxRect.Width = v
				case "premax_height":
					currentWindow.PreMaxRect.Height = v
				}
			case "buffer_file":
				currentWindow.BufferFile = unescapeString(extractQuotedValue(val))
			case "buffer_content":
				// Legacy: inline base64 buffer content. Load it and
				// set BufferFile so it gets migrated to gzip on next save.
				bv := extractQuotedValue(val)
				if decoded, err := base64.StdEncoding.DecodeString(bv); err == nil {
					content := string(decoded)
					// Save as external file and store the reference.
					if id := SaveBuffer(currentWindow.ID, content); id != "" {
						currentWindow.BufferFile = id
					}
				}
			case "buffer_rows":
				fmt.Sscanf(val, "%d", &currentWindow.BufferRows)
			case "buffer_cols":
				fmt.Sscanf(val, "%d", &currentWindow.BufferCols)
			case "app_state":
				currentWindow.AppStateData = extractQuotedValue(val)
			case "split_tree":
				currentWindow.SplitTree = unescapeString(extractQuotedValue(val))
			case "focused_pane":
				currentWindow.FocusedPane = unescapeString(extractQuotedValue(val))
			case "pane_count":
				var n int
				fmt.Sscanf(val, "%d", &n)
				if n > 0 && currentWindow.Panes == nil {
					currentWindow.Panes = make([]PaneState, n)
				}
			default:
				// Parse pane_N_field keys for split pane data
				parsePaneField(currentWindow, key, val)
			}
		}

		// Notification fields
		if currentNotif != nil {
			switch key {
			case "title":
				currentNotif.Title = unescapeString(extractQuotedValue(val))
			case "body":
				currentNotif.Body = unescapeString(extractQuotedValue(val))
			case "severity":
				currentNotif.Severity = extractQuotedValue(val)
			case "created_at":
				currentNotif.CreatedAt, _ = time.Parse(time.RFC3339, extractQuotedValue(val))
			case "read":
				currentNotif.Read = (val == "true")
			}
		}
	}

	// Append last item
	if currentWindow != nil {
		state.Windows = append(state.Windows, *currentWindow)
	}
	if currentNotif != nil {
		state.Notifications = append(state.Notifications, *currentNotif)
	}
}

// extractQuotedValue extracts the first quoted string from a TOML value.
// Handles escaped quotes within the string. If the value is not quoted,
// returns it as-is (for numbers, bools, etc.).
func extractQuotedValue(s string) string {
	s = strings.TrimSpace(s)
	if !strings.HasPrefix(s, "\"") {
		return s
	}
	s = s[1:] // strip leading quote
	for i := 0; i < len(s); i++ {
		if s[i] == '\\' {
			i++ // skip escaped char
			continue
		}
		if s[i] == '"' {
			return s[:i]
		}
	}
	return s // no closing quote, return as-is
}

// extractArrayValue extracts the first bracketed array from a TOML value.
func extractArrayValue(s string) string {
	s = strings.TrimSpace(s)
	if !strings.HasPrefix(s, "[") {
		return s
	}
	depth := 0
	for i := 0; i < len(s); i++ {
		if s[i] == '[' {
			depth++
		}
		if s[i] == ']' {
			depth--
			if depth == 0 {
				return s[:i+1]
			}
		}
	}
	return s
}

// unescapeString reverses escapeString: \\->\ \"->\" \\n->\n
func unescapeString(s string) string {
	var sb strings.Builder
	sb.Grow(len(s))
	for i := 0; i < len(s); i++ {
		if s[i] == '\\' && i+1 < len(s) {
			switch s[i+1] {
			case '\\':
				sb.WriteByte('\\')
				i++
			case '"':
				sb.WriteByte('"')
				i++
			case 'n':
				sb.WriteByte('\n')
				i++
			default:
				sb.WriteByte(s[i])
			}
		} else {
			sb.WriteByte(s[i])
		}
	}
	return sb.String()
}

func parseStringArray(s string) []string {
	s = extractArrayValue(s)
	s = strings.TrimPrefix(s, "[")
	s = strings.TrimSuffix(s, "]")
	if s == "" {
		return nil
	}
	var result []string
	var current strings.Builder
	inQuote := false
	escaped := false
	for _, ch := range s {
		if escaped {
			current.WriteRune(ch)
			escaped = false
			continue
		}
		if ch == '\\' && inQuote {
			escaped = true
			continue
		}
		if ch == '"' {
			inQuote = !inQuote
			continue
		}
		if ch == ',' && !inQuote {
			result = append(result, strings.TrimSpace(current.String()))
			current.Reset()
			continue
		}
		current.WriteRune(ch)
	}
	if current.Len() > 0 || len(result) > 0 {
		result = append(result, strings.TrimSpace(current.String()))
	}
	return result
}

func escapeString(s string) string {
	s = strings.ReplaceAll(s, "\\", "\\\\")
	s = strings.ReplaceAll(s, "\"", "\\\"")
	s = strings.ReplaceAll(s, "\n", "\\n")
	return s
}

// parsePaneField parses keys like "pane_0_id", "pane_1_command", etc.
func parsePaneField(w *WindowState, key, val string) {
	if !strings.HasPrefix(key, "pane_") {
		return
	}
	rest := key[5:] // strip "pane_"
	// Parse index: "0_id", "1_command", etc.
	idxEnd := strings.Index(rest, "_")
	if idxEnd < 0 {
		return
	}
	var idx int
	fmt.Sscanf(rest[:idxEnd], "%d", &idx)
	field := rest[idxEnd+1:]

	// Grow panes slice if needed
	for len(w.Panes) <= idx {
		w.Panes = append(w.Panes, PaneState{})
	}

	switch field {
	case "id":
		w.Panes[idx].TermID = unescapeString(extractQuotedValue(val))
	case "command":
		w.Panes[idx].Command = unescapeString(extractQuotedValue(val))
	case "args":
		w.Panes[idx].Args = parseStringArray(val)
	case "workdir":
		w.Panes[idx].WorkDir = unescapeString(extractQuotedValue(val))
	case "buffer_file":
		w.Panes[idx].BufferFile = unescapeString(extractQuotedValue(val))
	case "buffer_rows":
		fmt.Sscanf(val, "%d", &w.Panes[idx].BufferRows)
	case "buffer_cols":
		fmt.Sscanf(val, "%d", &w.Panes[idx].BufferCols)
	}
}

package settings

import (
	"fmt"
	"strings"

	"github.com/icex/termdesk/internal/config"
	"github.com/icex/termdesk/internal/widget"
	"github.com/icex/termdesk/pkg/geometry"
)

// ItemType distinguishes setting control types.
type ItemType int

const (
	TypeToggle ItemType = iota
	TypeChoice
	TypeText // text input with optional preset choices (left/right cycle, Enter to edit)
)

// Layout constants for settings panel hit-testing and rendering.
const (
	PanelWidth   = 80 // default inner width
	TabRow       = 3  // relY of tab bar (spacer=0, title=1, spacer=2, tabs=3)
	ItemStartRow = 6  // relY where items begin (tabs=3, sep=4, spacer=5, items=6+)
	HeaderRows   = 6  // rows above items (spacer+title+spacer+tabs+sep+spacer)
	FooterRows   = 4  // rows below items (spacer+sep+footer+spacer)
)

// Item is a single setting entry.
type Item struct {
	Key     string
	Label   string
	Type    ItemType
	BoolVal bool
	StrVal  string
	Choices []string
}

// Section groups related settings under a tab.
type Section struct {
	Title string
	Items []Item
}

// Panel manages the settings UI state.
type Panel struct {
	Sections    []Section
	ActiveTab   int
	ActiveItem  int
	HoverTab    int // hovered tab index (-1 = none)
	Visible     bool
	TextEditing bool   // true when a TypeText item is in edit mode
	TextBuf     string // edit buffer for TypeText (committed on Enter)
	TextCursor  int    // cursor position within TextBuf (rune index)
}

// New creates a settings panel populated from the current config.
// widgetMetas lists all registered widgets; enabledWidgets controls toggle state.
// Pass nil for both to omit the Widgets tab.
func New(cfg config.UserConfig, wMetas []widget.WidgetMeta, wEnabled []string) *Panel {
	general := Section{
		Title: "\uf013  General",
		Items: []Item{
			{Key: "theme", Label: "Theme", Type: TypeChoice, StrVal: cfg.Theme, Choices: config.ThemeNames()},
			{Key: "animations", Label: "Animations", Type: TypeToggle, BoolVal: cfg.Animations},
			{Key: "animation_speed", Label: "Animation Speed", Type: TypeChoice, StrVal: cfg.AnimationSpeed, Choices: []string{"slow", "normal", "fast"}},
			{Key: "animation_style", Label: "Animation Style", Type: TypeChoice, StrVal: cfg.AnimationStyle, Choices: []string{"smooth", "snappy", "bouncy"}},
			{Key: "show_desk_clock", Label: "Desktop Clock", Type: TypeToggle, BoolVal: cfg.ShowDeskClock},
			{Key: "show_keys", Label: "Show Keys Overlay", Type: TypeToggle, BoolVal: cfg.ShowKeys},
			{Key: "tiling_mode", Label: "Tiling Mode", Type: TypeToggle, BoolVal: cfg.TilingMode},
			{Key: "default_terminal_mode", Label: "Default to Terminal Mode", Type: TypeToggle, BoolVal: cfg.DefaultTerminalMode},
			{Key: "focus_follows_mouse", Label: "Focus Follows Mouse", Type: TypeToggle, BoolVal: cfg.FocusFollowsMouse},
			{Key: "quake_height_percent", Label: "Quake Height (%)", Type: TypeChoice, StrVal: formatQuakeHeight(cfg.QuakeHeightPercent), Choices: []string{"20", "25", "30", "33", "40", "50", "60", "75"}},
		},
	}

	dockSection := Section{
		Title: "\uf2d2  Dock",
		Items: []Item{
			{Key: "icons_only", Label: "Icons Only", Type: TypeToggle, BoolVal: cfg.IconsOnly},
			{Key: "minimize_to_dock", Label: "Minimize to Dock icon", Type: TypeToggle, BoolVal: cfg.MinimizeToDock},
			{Key: "hide_dock_when_maximized", Label: "Hide When Maximized", Type: TypeToggle, BoolVal: cfg.HideDockWhenMaximized},
			{Key: "hide_dock_apps", Label: "Hide App Shortcuts", Type: TypeToggle, BoolVal: cfg.HideDockApps},
		},
	}

	kb := cfg.Keys
	workspace := Section{
		Title: "\uf07c  Workspace",
		Items: []Item{
			{Key: "workspace_auto_save", Label: "Auto-Save Workspace", Type: TypeToggle, BoolVal: cfg.WorkspaceAutoSave},
			{Key: "workspace_auto_save_min", Label: "Auto-Save Interval (min)", Type: TypeChoice, StrVal: formatAutoSaveInterval(cfg.WorkspaceAutoSaveMin), Choices: []string{"1", "2", "5", "10", "15"}},
		},
	}

	keybindings := Section{
		Title: "\uf11c  Keybindings",
		Items: []Item{
			{Key: "prefix", Label: "Prefix Key", Type: TypeChoice, StrVal: kb.Prefix, Choices: []string{"ctrl+a", "ctrl+g", "ctrl+b", "ctrl+t"}},
			{Key: "quit", Label: "Quit", Type: TypeChoice, StrVal: kb.Quit, Choices: []string{"q"}},
			{Key: "new_terminal", Label: "New Terminal", Type: TypeChoice, StrVal: kb.NewTerminal, Choices: []string{"n"}},
			{Key: "enter_terminal", Label: "Enter Terminal", Type: TypeChoice, StrVal: kb.EnterTerminal, Choices: []string{"i"}},
			{Key: "quake_terminal", Label: "Quake Terminal", Type: TypeChoice, StrVal: kb.QuakeTerminal, Choices: []string{"ctrl+`", "ctrl+~", "f12", "ctrl+f12", "f11"}},
		},
	}

	// Color palette presets for wallpaper color pickers
	colorPalette := []string{
		"#282C34", "#1E1E2E", "#2E3440", "#1A1B26",
		"#000000", "#1C1C1C", "#0D1117", "#002B36",
		"#FAFAFA", "#F5F5F5", "#EFF1F5", "#FDF6E3",
		"#E06C75", "#98C379", "#61AFEF", "#C678DD",
		"#E5C07B", "#56B6C2", "#BE5046", "#D19A66",
	}
	patternPresets := []string{"░", "▒", "▓", "▚", "⠿", "◇", "◆", "≈", "╳", "·", "•", "∘", "○"}

	wpMode := cfg.WallpaperMode
	if wpMode == "" {
		wpMode = "theme"
	}
	wpColor := cfg.WallpaperColor
	if wpColor == "" {
		wpColor = "#282C34"
	}
	wpPattern := cfg.WallpaperPattern
	if wpPattern == "" {
		wpPattern = "░"
	}
	wpPatFg := cfg.WallpaperPatternFg
	if wpPatFg == "" {
		wpPatFg = "#303844"
	}
	wpPatBg := cfg.WallpaperPatternBg
	if wpPatBg == "" {
		wpPatBg = "#282C34"
	}

	wallpaper := Section{
		Title: "\uf03e  Wallpaper",
		Items: []Item{
			{Key: "wallpaper_mode", Label: "Mode", Type: TypeChoice, StrVal: wpMode, Choices: []string{"theme", "color", "pattern", "program"}},
			{Key: "wallpaper_color", Label: "Color", Type: TypeText, StrVal: wpColor, Choices: colorPalette},
			{Key: "wallpaper_pattern", Label: "Pattern", Type: TypeText, StrVal: wpPattern, Choices: patternPresets},
			{Key: "wallpaper_pattern_fg", Label: "Pattern Fg", Type: TypeText, StrVal: wpPatFg, Choices: colorPalette},
			{Key: "wallpaper_pattern_bg", Label: "Pattern Bg", Type: TypeText, StrVal: wpPatBg, Choices: colorPalette},
			{Key: "wallpaper_program", Label: "Program", Type: TypeText, StrVal: cfg.WallpaperProgram},
		},
	}

	sections := []Section{general, dockSection, workspace, keybindings, wallpaper}

	// Build Widgets tab if widget metadata was provided.
	if len(wMetas) > 0 {
		enabledSet := make(map[string]bool)
		for _, name := range wEnabled {
			enabledSet[name] = true
		}
		var widgetItems []Item
		for _, meta := range wMetas {
			label := meta.Label
			if !meta.Builtin {
				label += " (custom)"
			}
			widgetItems = append(widgetItems, Item{
				Key:     "widget_" + meta.Name,
				Label:   label,
				Type:    TypeToggle,
				BoolVal: enabledSet[meta.Name],
			})
		}
		sections = append(sections, Section{
			Title: "\uf009  Widgets",
			Items: widgetItems,
		})
	}

	return &Panel{
		Sections:  sections,
		Visible:   false,
		ActiveTab: 0,
		HoverTab:  -1,
	}
}

func formatAutoSaveInterval(minutes int) string {
	if minutes <= 0 {
		return "1"
	}
	return fmt.Sprintf("%d", minutes)
}

func formatQuakeHeight(pct int) string {
	if pct <= 0 {
		return "40"
	}
	return fmt.Sprintf("%d", pct)
}

// InnerWidth returns the panel content width clamped to screen.
func (p *Panel) InnerWidth(screenW int) int {
	w := PanelWidth
	if w > screenW-4 {
		w = screenW - 4
	}
	return w
}

// Bounds returns the panel's screen rectangle (including border).
func (p *Panel) Bounds(screenW, screenH int) geometry.Rect {
	innerW := p.InnerWidth(screenW)
	sec := p.Sections[p.ActiveTab]
	innerH := HeaderRows + len(sec.Items) + FooterRows
	boxW := innerW + 2
	boxH := innerH + 2
	startX := (screenW - boxW) / 2
	startY := (screenH - boxH) / 2
	if startX < 0 {
		startX = 0
	}
	if startY < 1 {
		startY = 1
	}
	return geometry.Rect{X: startX, Y: startY, Width: boxW, Height: boxH}
}

// TabAtX returns which tab index is at the given relative X position
// within the tab row, or -1 if not on any tab. innerW is the panel content width.
func (p *Panel) TabAtX(relX, innerW int) int {
	totalW := 0
	for _, s := range p.Sections {
		totalW += len([]rune(s.Title)) + 2
	}
	offset := (innerW - totalW) / 2
	if offset < 0 {
		offset = 0
	}
	x := offset
	for i, s := range p.Sections {
		tabW := len([]rune(s.Title)) + 2
		if relX >= x && relX < x+tabW {
			return i
		}
		x += tabW
	}
	return -1
}

// ItemAtY returns which item index is at the given relative Y position
// inside the panel border, or -1 if not on any item.
func (p *Panel) ItemAtY(relY int) int {
	if p.ActiveTab >= len(p.Sections) {
		return -1
	}
	row := relY - ItemStartRow
	if row < 0 || row >= len(p.Sections[p.ActiveTab].Items) {
		return -1
	}
	return row
}

// CurrentItem returns a pointer to the currently selected item, or nil.
func (p *Panel) CurrentItem() *Item {
	if p.ActiveTab < 0 || p.ActiveTab >= len(p.Sections) {
		return nil
	}
	sec := &p.Sections[p.ActiveTab]
	if p.ActiveItem < 0 || p.ActiveItem >= len(sec.Items) {
		return nil
	}
	return &sec.Items[p.ActiveItem]
}

// Toggle flips the current item if it's a toggle.
func (p *Panel) Toggle() {
	item := p.CurrentItem()
	if item == nil || item.Type != TypeToggle {
		return
	}
	item.BoolVal = !item.BoolVal
}

// StartTextEdit enters text editing mode for the current TypeText item.
func (p *Panel) StartTextEdit() {
	item := p.CurrentItem()
	if item == nil || item.Type != TypeText {
		return
	}
	p.TextEditing = true
	p.TextBuf = item.StrVal
	p.TextCursor = len([]rune(item.StrVal))
}

// CommitTextEdit saves the text buffer to the item and exits editing mode.
func (p *Panel) CommitTextEdit() {
	if !p.TextEditing {
		return
	}
	item := p.CurrentItem()
	if item != nil && item.Type == TypeText {
		item.StrVal = p.TextBuf
	}
	p.TextEditing = false
	p.TextBuf = ""
	p.TextCursor = 0
}

// CancelTextEdit discards changes and exits editing mode.
func (p *Panel) CancelTextEdit() {
	p.TextEditing = false
	p.TextBuf = ""
	p.TextCursor = 0
}

// TextInsert inserts a string at the cursor position in the text buffer.
func (p *Panel) TextInsert(s string) {
	if !p.TextEditing {
		return
	}
	runes := []rune(p.TextBuf)
	ins := []rune(s)
	newRunes := make([]rune, 0, len(runes)+len(ins))
	newRunes = append(newRunes, runes[:p.TextCursor]...)
	newRunes = append(newRunes, ins...)
	newRunes = append(newRunes, runes[p.TextCursor:]...)
	p.TextBuf = string(newRunes)
	p.TextCursor += len(ins)
}

// TextBackspace deletes the character before the cursor.
func (p *Panel) TextBackspace() {
	if !p.TextEditing || p.TextCursor <= 0 {
		return
	}
	runes := []rune(p.TextBuf)
	p.TextCursor--
	p.TextBuf = string(append(runes[:p.TextCursor], runes[p.TextCursor+1:]...))
}

// TextDelete deletes the character at the cursor.
func (p *Panel) TextDelete() {
	if !p.TextEditing {
		return
	}
	runes := []rune(p.TextBuf)
	if p.TextCursor >= len(runes) {
		return
	}
	p.TextBuf = string(append(runes[:p.TextCursor], runes[p.TextCursor+1:]...))
}

// TextMoveCursor moves the text cursor by delta runes.
func (p *Panel) TextMoveCursor(delta int) {
	if !p.TextEditing {
		return
	}
	p.TextCursor += delta
	runes := []rune(p.TextBuf)
	if p.TextCursor < 0 {
		p.TextCursor = 0
	}
	if p.TextCursor > len(runes) {
		p.TextCursor = len(runes)
	}
}

// CycleText cycles a TypeText item through its preset Choices by delta.
// If the current value isn't in the preset list, jumps to the first preset.
func (p *Panel) CycleText(delta int) {
	item := p.CurrentItem()
	if item == nil || item.Type != TypeText || len(item.Choices) == 0 {
		return
	}
	idx := -1
	for i, c := range item.Choices {
		if c == item.StrVal {
			idx = i
			break
		}
	}
	if idx < 0 {
		idx = 0
	} else {
		idx += delta
	}
	if idx < 0 {
		idx = len(item.Choices) - 1
	}
	if idx >= len(item.Choices) {
		idx = 0
	}
	item.StrVal = item.Choices[idx]
}

// CycleChoice cycles the current choice item by delta.
func (p *Panel) CycleChoice(delta int) {
	item := p.CurrentItem()
	if item == nil || item.Type != TypeChoice || len(item.Choices) == 0 {
		return
	}
	idx := 0
	for i, c := range item.Choices {
		if c == item.StrVal {
			idx = i
			break
		}
	}
	idx += delta
	if idx < 0 {
		idx = len(item.Choices) - 1
	}
	if idx >= len(item.Choices) {
		idx = 0
	}
	item.StrVal = item.Choices[idx]
}

// NextItem moves selection down, wrapping.
func (p *Panel) NextItem() {
	if p.ActiveTab >= len(p.Sections) {
		return
	}
	count := len(p.Sections[p.ActiveTab].Items)
	if count == 0 {
		return
	}
	p.ActiveItem++
	if p.ActiveItem >= count {
		p.ActiveItem = 0
	}
}

// PrevItem moves selection up, wrapping.
func (p *Panel) PrevItem() {
	if p.ActiveTab >= len(p.Sections) {
		return
	}
	count := len(p.Sections[p.ActiveTab].Items)
	if count == 0 {
		return
	}
	p.ActiveItem--
	if p.ActiveItem < 0 {
		p.ActiveItem = count - 1
	}
}

// NextTab moves to the next section tab, wrapping.
func (p *Panel) NextTab() {
	p.ActiveTab++
	if p.ActiveTab >= len(p.Sections) {
		p.ActiveTab = 0
	}
	p.ActiveItem = 0
}

// PrevTab moves to the previous section tab, wrapping.
func (p *Panel) PrevTab() {
	p.ActiveTab--
	if p.ActiveTab < 0 {
		p.ActiveTab = len(p.Sections) - 1
	}
	p.ActiveItem = 0
}

// SetTab switches to the given tab index.
func (p *Panel) SetTab(idx int) {
	if idx >= 0 && idx < len(p.Sections) {
		p.ActiveTab = idx
		p.ActiveItem = 0
	}
}

// SetItem selects the given item index in the current tab.
func (p *Panel) SetItem(idx int) {
	if p.ActiveTab >= 0 && p.ActiveTab < len(p.Sections) {
		sec := p.Sections[p.ActiveTab]
		if idx >= 0 && idx < len(sec.Items) {
			p.ActiveItem = idx
		}
	}
}

// Show makes the panel visible.
func (p *Panel) Show() {
	p.Visible = true
	p.ActiveTab = 0
	p.ActiveItem = 0
	p.HoverTab = -1
}

// Hide hides the panel.
func (p *Panel) Hide() {
	p.Visible = false
}

// ApplyTo writes the panel values back to a UserConfig.
func (p *Panel) ApplyTo(cfg *config.UserConfig) {
	var widgetEnabled []string
	hasWidgetItems := false
	for _, sec := range p.Sections {
		for _, item := range sec.Items {
			if strings.HasPrefix(item.Key, "widget_") {
				hasWidgetItems = true
				if item.BoolVal {
					widgetEnabled = append(widgetEnabled, strings.TrimPrefix(item.Key, "widget_"))
				}
				continue
			}
			switch item.Key {
			case "theme":
				cfg.Theme = item.StrVal
			case "animations":
				cfg.Animations = item.BoolVal
			case "animation_speed":
				cfg.AnimationSpeed = item.StrVal
			case "animation_style":
				cfg.AnimationStyle = item.StrVal
			case "icons_only":
				cfg.IconsOnly = item.BoolVal
			case "show_desk_clock":
				cfg.ShowDeskClock = item.BoolVal
			case "show_keys":
				cfg.ShowKeys = item.BoolVal
			case "tiling_mode":
				cfg.TilingMode = item.BoolVal
			case "minimize_to_dock":
				cfg.MinimizeToDock = item.BoolVal
			case "hide_dock_when_maximized":
				cfg.HideDockWhenMaximized = item.BoolVal
			case "hide_dock_apps":
				cfg.HideDockApps = item.BoolVal
			case "default_terminal_mode":
				cfg.DefaultTerminalMode = item.BoolVal
			case "focus_follows_mouse":
				cfg.FocusFollowsMouse = item.BoolVal
			case "workspace_auto_save":
				cfg.WorkspaceAutoSave = item.BoolVal
			case "workspace_auto_save_min":
				var minutes int
				fmt.Sscanf(item.StrVal, "%d", &minutes)
				if minutes > 0 {
					cfg.WorkspaceAutoSaveMin = minutes
				}
			case "quake_height_percent":
				var pct int
				fmt.Sscanf(item.StrVal, "%d", &pct)
				if pct >= 10 && pct <= 90 {
					cfg.QuakeHeightPercent = pct
				}
			case "prefix":
				cfg.Keys.Prefix = item.StrVal
			case "quake_terminal":
				cfg.Keys.QuakeTerminal = item.StrVal
			case "wallpaper_mode":
				cfg.WallpaperMode = item.StrVal
			case "wallpaper_color":
				cfg.WallpaperColor = item.StrVal
			case "wallpaper_pattern":
				cfg.WallpaperPattern = item.StrVal
			case "wallpaper_pattern_fg":
				cfg.WallpaperPatternFg = item.StrVal
			case "wallpaper_pattern_bg":
				cfg.WallpaperPatternBg = item.StrVal
			case "wallpaper_program":
				cfg.WallpaperProgram = item.StrVal
			}
		}
	}
	if hasWidgetItems {
		cfg.EnabledWidgets = widgetEnabled
	}
}

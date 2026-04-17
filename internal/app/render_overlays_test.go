package app

import (
	"testing"

	"github.com/icex/termdesk/internal/clipboard"
	"github.com/icex/termdesk/internal/config"
	"github.com/icex/termdesk/internal/contextmenu"
	"github.com/icex/termdesk/internal/notification"
	"github.com/icex/termdesk/internal/settings"
	"github.com/icex/termdesk/internal/tour"
)

// bufHasNonSpaceInRegion checks whether any cell in the given rectangular
// region contains a character other than space.
func bufHasNonSpaceInRegion(buf *Buffer, x0, y0, x1, y1 int) bool {
	if x0 < 0 {
		x0 = 0
	}
	if y0 < 0 {
		y0 = 0
	}
	if x1 > buf.Width {
		x1 = buf.Width
	}
	if y1 > buf.Height {
		y1 = buf.Height
	}
	for y := y0; y < y1; y++ {
		for x := x0; x < x1; x++ {
			if buf.Cells[y][x].Char != ' ' {
				return true
			}
		}
	}
	return false
}

// bufHasAnyNonSpace checks whether any cell in the entire buffer has a
// non-space character.
func bufHasAnyNonSpace(buf *Buffer) bool {
	return bufHasNonSpaceInRegion(buf, 0, 0, buf.Width, buf.Height)
}

// ----------------------------------------------------------------------------
// RenderClipboardHistory
// ----------------------------------------------------------------------------

func TestRenderClipboardHistory_NilClipboard(t *testing.T) {
	theme := config.RetroTheme()
	buf := AcquireThemedBuffer(80, 30, theme)
	RenderClipboardHistory(buf, nil, theme)
	// Nothing should have been drawn beyond the desktop pattern
}

func TestRenderClipboardHistory_NotVisible(t *testing.T) {
	theme := config.RetroTheme()
	buf := AcquireThemedBuffer(80, 30, theme)
	clip := clipboard.New()
	clip.Copy("hello")
	// Visible is false by default
	RenderClipboardHistory(buf, clip, theme)
	// The clipboard overlay should not render when not visible
}

func TestRenderClipboardHistory_EmptyHistory(t *testing.T) {
	theme := config.RetroTheme()
	buf := NewBuffer(80, 30, theme.DesktopBg)
	clip := clipboard.New()
	clip.ShowHistory()
	RenderClipboardHistory(buf, clip, theme)

	// Should draw the overlay even with no entries (shows "No clipboard entries")
	if !bufHasAnyNonSpace(buf) {
		t.Error("expected clipboard overlay content for empty history")
	}
}

func TestRenderClipboardHistory_WithItems(t *testing.T) {
	theme := config.RetroTheme()
	buf := NewBuffer(80, 30, theme.DesktopBg)
	clip := clipboard.New()
	clip.Copy("first item")
	clip.Copy("second item")
	clip.Copy("third item")
	clip.ShowHistory()

	RenderClipboardHistory(buf, clip, theme)

	// Center region should contain overlay content
	cx, cy := buf.Width/2, buf.Height/2
	if !bufHasNonSpaceInRegion(buf, cx-25, cy-8, cx+25, cy+8) {
		t.Error("expected clipboard history content in center region")
	}
}

func TestRenderClipboardHistory_SelectedItem(t *testing.T) {
	theme := config.RetroTheme()
	buf := NewBuffer(80, 30, theme.DesktopBg)
	clip := clipboard.New()
	clip.Copy("item A")
	clip.Copy("item B")
	clip.ShowHistory()
	clip.MoveSelection(1) // select second item

	RenderClipboardHistory(buf, clip, theme)

	if !bufHasAnyNonSpace(buf) {
		t.Error("expected clipboard overlay with selected item")
	}
}

func TestRenderClipboardHistory_NarrowBuffer(t *testing.T) {
	theme := config.RetroTheme()
	buf := NewBuffer(30, 20, theme.DesktopBg)
	clip := clipboard.New()
	clip.Copy("test")
	clip.ShowHistory()

	// Should not panic on a narrow buffer
	RenderClipboardHistory(buf, clip, theme)

	if !bufHasAnyNonSpace(buf) {
		t.Error("expected clipboard overlay even on narrow buffer")
	}
}

// ----------------------------------------------------------------------------
// RenderContextMenu
// ----------------------------------------------------------------------------

func TestRenderContextMenu_NilMenu(t *testing.T) {
	theme := config.RetroTheme()
	buf := AcquireThemedBuffer(80, 30, theme)
	RenderContextMenu(buf, nil, theme)
}

func TestRenderContextMenu_NotVisible(t *testing.T) {
	theme := config.RetroTheme()
	buf := NewBuffer(80, 30, theme.DesktopBg)
	menu := &contextmenu.Menu{
		X: 10, Y: 10,
		Items:   []contextmenu.Item{{Label: "Paste", Action: "paste"}},
		Visible: false,
	}
	RenderContextMenu(buf, menu, theme)
	// Should not draw when not visible
}

func TestRenderContextMenu_DesktopMenu(t *testing.T) {
	theme := config.RetroTheme()
	buf := NewBuffer(80, 30, theme.DesktopBg)
	menu := contextmenu.DesktopMenu(10, 5, contextmenu.KeyBindings{})

	RenderContextMenu(buf, menu, theme)

	// Should have content near menu position
	if !bufHasNonSpaceInRegion(buf, 8, 4, 40, 20) {
		t.Error("expected context menu content near position (10, 5)")
	}
}

func TestRenderContextMenu_TitleBarMenu(t *testing.T) {
	theme := config.RetroTheme()
	buf := NewBuffer(80, 30, theme.DesktopBg)
	menu := contextmenu.TitleBarMenu(15, 3, true, false, contextmenu.KeyBindings{})

	RenderContextMenu(buf, menu, theme)

	if !bufHasAnyNonSpace(buf) {
		t.Error("expected title bar context menu content")
	}
}

func TestRenderContextMenu_WithDisabledItems(t *testing.T) {
	theme := config.RetroTheme()
	buf := NewBuffer(80, 30, theme.DesktopBg)
	menu := &contextmenu.Menu{
		X: 5, Y: 5,
		Visible: true,
		Items: []contextmenu.Item{
			{Label: "Copy", Action: "copy"},
			{Label: "---", Disabled: true},
			{Label: "Paste", Action: "paste"},
		},
	}

	RenderContextMenu(buf, menu, theme)

	if !bufHasAnyNonSpace(buf) {
		t.Error("expected context menu with disabled items")
	}
}

func TestRenderContextMenu_HoverIndex(t *testing.T) {
	theme := config.RetroTheme()
	buf := NewBuffer(80, 30, theme.DesktopBg)
	menu := contextmenu.DesktopMenu(10, 5, contextmenu.KeyBindings{})
	menu.HoverIndex = 2 // Tile All

	RenderContextMenu(buf, menu, theme)

	if !bufHasAnyNonSpace(buf) {
		t.Error("expected context menu with hovered item")
	}
}

func TestRenderContextMenu_ClampToBuffer(t *testing.T) {
	theme := config.RetroTheme()
	buf := NewBuffer(40, 15, theme.DesktopBg)
	// Position near bottom-right edge to trigger clamping
	menu := contextmenu.DesktopMenu(35, 12, contextmenu.KeyBindings{})

	RenderContextMenu(buf, menu, theme)

	if !bufHasAnyNonSpace(buf) {
		t.Error("expected context menu content even when clamped to buffer edge")
	}
}

// ----------------------------------------------------------------------------
// RenderModal
// ----------------------------------------------------------------------------

func TestRenderModal_Nil(t *testing.T) {
	theme := config.RetroTheme()
	buf := AcquireThemedBuffer(80, 30, theme)
	RenderModal(buf, nil, theme)
}

func TestRenderModal_SimpleContent(t *testing.T) {
	theme := config.RetroTheme()
	buf := NewBuffer(80, 30, theme.DesktopBg)
	modal := &ModalOverlay{
		Title: "Help",
		Lines: []string{
			"Welcome to Termdesk",
			"Press F1 for help",
			"Press N for new terminal",
		},
	}

	RenderModal(buf, modal, theme)

	cx, cy := buf.Width/2, buf.Height/2
	if !bufHasNonSpaceInRegion(buf, cx-25, cy-10, cx+25, cy+10) {
		t.Error("expected modal content in center of buffer")
	}
}

func TestRenderModal_TabbedContent(t *testing.T) {
	theme := config.RetroTheme()
	buf := NewBuffer(80, 30, theme.DesktopBg)
	modal := &ModalOverlay{
		Title: "Tabbed Help",
		Tabs: []HelpTab{
			{Title: "General", Lines: []string{"Line A1", "Line A2"}},
			{Title: "Keys", Lines: []string{"Line B1", "Line B2", "Line B3"}},
		},
		ActiveTab: 0,
	}

	RenderModal(buf, modal, theme)

	if !bufHasAnyNonSpace(buf) {
		t.Error("expected tabbed modal content")
	}
}

func TestRenderModal_TabSwitch(t *testing.T) {
	theme := config.RetroTheme()
	buf := NewBuffer(80, 30, theme.DesktopBg)
	modal := &ModalOverlay{
		Title: "Tabbed Help",
		Tabs: []HelpTab{
			{Title: "General", Lines: []string{"General info"}},
			{Title: "Keys", Lines: []string{"Key bindings"}},
		},
		ActiveTab: 1, // second tab active
	}

	RenderModal(buf, modal, theme)

	if !bufHasAnyNonSpace(buf) {
		t.Error("expected tabbed modal on second tab")
	}
}

func TestRenderModal_Scrollable(t *testing.T) {
	theme := config.RetroTheme()
	buf := NewBuffer(80, 30, theme.DesktopBg)

	// Create many lines that exceed visible area
	var lines []string
	for i := 0; i < 50; i++ {
		lines = append(lines, "This is a really long line of help content to test scrolling")
	}
	modal := &ModalOverlay{
		Title:   "Scrollable",
		Lines:   lines,
		ScrollY: 5,
	}

	RenderModal(buf, modal, theme)

	if !bufHasAnyNonSpace(buf) {
		t.Error("expected scrollable modal content")
	}
}

func TestRenderModal_ScrollYClamped(t *testing.T) {
	theme := config.RetroTheme()
	buf := NewBuffer(80, 30, theme.DesktopBg)
	modal := &ModalOverlay{
		Title:   "Clamped",
		Lines:   []string{"Only one line"},
		ScrollY: 999, // way beyond content
	}

	RenderModal(buf, modal, theme)

	// ScrollY should be clamped; should still render
	if !bufHasAnyNonSpace(buf) {
		t.Error("expected modal content even with clamped scroll")
	}
}

func TestRenderModal_EmptyLines(t *testing.T) {
	theme := config.RetroTheme()
	buf := NewBuffer(80, 30, theme.DesktopBg)
	modal := &ModalOverlay{
		Title: "Empty",
		Lines: []string{},
	}

	// Should not panic with empty lines
	RenderModal(buf, modal, theme)
}

func TestRenderModal_InvalidActiveTab(t *testing.T) {
	theme := config.RetroTheme()
	buf := NewBuffer(80, 30, theme.DesktopBg)
	modal := &ModalOverlay{
		Title: "Bad Tab",
		Tabs: []HelpTab{
			{Title: "Only Tab", Lines: []string{"content"}},
		},
		ActiveTab: 99, // out of range, should be clamped to 0
	}

	RenderModal(buf, modal, theme)

	if !bufHasAnyNonSpace(buf) {
		t.Error("expected modal content with clamped active tab")
	}
}

// ----------------------------------------------------------------------------
// RenderToasts
// ----------------------------------------------------------------------------

func TestRenderToasts_Empty(t *testing.T) {
	theme := config.RetroTheme()
	buf := NewBuffer(80, 30, theme.DesktopBg)
	nm := notification.New()

	RenderToasts(buf, nm, theme)

	// No toasts, nothing rendered
}

func TestRenderToasts_InfoToast(t *testing.T) {
	theme := config.RetroTheme()
	buf := NewBuffer(80, 30, theme.DesktopBg)
	nm := notification.New()
	nm.Push("Hello", "World", notification.Info)

	RenderToasts(buf, nm, theme)

	// Toast should appear near top-right
	if !bufHasNonSpaceInRegion(buf, buf.Width-55, 0, buf.Width, 10) {
		t.Error("expected toast content in top-right region")
	}
}

func TestRenderToasts_WarningToast(t *testing.T) {
	theme := config.RetroTheme()
	buf := NewBuffer(80, 30, theme.DesktopBg)
	nm := notification.New()
	nm.Push("Warning", "Something bad", notification.Warning)

	RenderToasts(buf, nm, theme)

	if !bufHasAnyNonSpace(buf) {
		t.Error("expected warning toast content")
	}
}

func TestRenderToasts_ErrorToast(t *testing.T) {
	theme := config.RetroTheme()
	buf := NewBuffer(80, 30, theme.DesktopBg)
	nm := notification.New()
	nm.Push("Error", "Failed badly", notification.Error)

	RenderToasts(buf, nm, theme)

	if !bufHasAnyNonSpace(buf) {
		t.Error("expected error toast content")
	}
}

func TestRenderToasts_MultipleToasts(t *testing.T) {
	theme := config.RetroTheme()
	buf := NewBuffer(80, 30, theme.DesktopBg)
	nm := notification.New()
	nm.Push("First", "Body 1", notification.Info)
	nm.Push("Second", "Body 2", notification.Warning)
	nm.Push("Third", "Body 3", notification.Error)

	RenderToasts(buf, nm, theme)

	if !bufHasAnyNonSpace(buf) {
		t.Error("expected multiple toast content")
	}
}

func TestRenderToasts_NarrowBuffer(t *testing.T) {
	theme := config.RetroTheme()
	buf := NewBuffer(30, 15, theme.DesktopBg)
	nm := notification.New()
	nm.Push("Title", "Body", notification.Info)

	// Should not panic on narrow buffer
	RenderToasts(buf, nm, theme)

	if !bufHasAnyNonSpace(buf) {
		t.Error("expected toast content on narrow buffer")
	}
}

// ----------------------------------------------------------------------------
// RenderNotificationCenter
// ----------------------------------------------------------------------------

func TestRenderNotificationCenter_NotVisible(t *testing.T) {
	theme := config.RetroTheme()
	buf := NewBuffer(80, 30, theme.DesktopBg)
	nm := notification.New()
	nm.Push("Test", "Body", notification.Info)
	// Center not shown

	RenderNotificationCenter(buf, nm, theme)

	// Nothing should be drawn for the center
}

func TestRenderNotificationCenter_EmptyHistory(t *testing.T) {
	theme := config.RetroTheme()
	buf := NewBuffer(80, 30, theme.DesktopBg)
	nm := notification.New()
	nm.ShowCenter()

	RenderNotificationCenter(buf, nm, theme)

	// Should show "No notifications" overlay
	if !bufHasAnyNonSpace(buf) {
		t.Error("expected notification center with empty state")
	}
}

func TestRenderNotificationCenter_WithItems(t *testing.T) {
	theme := config.RetroTheme()
	buf := NewBuffer(80, 30, theme.DesktopBg)
	nm := notification.New()
	nm.Push("Alert 1", "First alert", notification.Info)
	nm.Push("Alert 2", "Second alert", notification.Warning)
	nm.Push("Alert 3", "Third alert", notification.Error)
	nm.ShowCenter()

	RenderNotificationCenter(buf, nm, theme)

	// Should render on right side of buffer
	if !bufHasNonSpaceInRegion(buf, buf.Width-50, 0, buf.Width, buf.Height) {
		t.Error("expected notification center content on right side")
	}
}

func TestRenderNotificationCenter_SelectionIndex(t *testing.T) {
	theme := config.RetroTheme()
	buf := NewBuffer(80, 30, theme.DesktopBg)
	nm := notification.New()
	nm.Push("Item 1", "Body", notification.Info)
	nm.Push("Item 2", "Body", notification.Info)
	nm.ShowCenter()
	nm.MoveCenterSelection(1) // select second item

	RenderNotificationCenter(buf, nm, theme)

	if !bufHasAnyNonSpace(buf) {
		t.Error("expected notification center with selection")
	}
}

func TestRenderNotificationCenter_NarrowBuffer(t *testing.T) {
	theme := config.RetroTheme()
	buf := NewBuffer(30, 15, theme.DesktopBg)
	nm := notification.New()
	nm.Push("Test", "Body", notification.Info)
	nm.ShowCenter()

	// Should not panic on narrow buffer
	RenderNotificationCenter(buf, nm, theme)

	if !bufHasAnyNonSpace(buf) {
		t.Error("expected notification center on narrow buffer")
	}
}

// ----------------------------------------------------------------------------
// RenderSettingsPanel
// ----------------------------------------------------------------------------

func TestRenderSettingsPanel_Nil(t *testing.T) {
	theme := config.RetroTheme()
	buf := AcquireThemedBuffer(80, 30, theme)
	RenderSettingsPanel(buf, nil, theme)
}

func TestRenderSettingsPanel_NotVisible(t *testing.T) {
	theme := config.RetroTheme()
	buf := NewBuffer(80, 30, theme.DesktopBg)
	panel := settings.New(config.UserConfig{}, nil, nil)
	// panel.Visible is false by default

	RenderSettingsPanel(buf, panel, theme)
}

func TestRenderSettingsPanel_Visible(t *testing.T) {
	theme := config.RetroTheme()
	buf := NewBuffer(80, 30, theme.DesktopBg)
	panel := settings.New(config.UserConfig{
		Theme:      "retro",
		Animations: true,
	}, nil, nil)
	panel.Show()

	RenderSettingsPanel(buf, panel, theme)

	cx, cy := buf.Width/2, buf.Height/2
	if !bufHasNonSpaceInRegion(buf, cx-30, cy-12, cx+30, cy+12) {
		t.Error("expected settings panel content in center region")
	}
}

func TestRenderSettingsPanel_SecondTab(t *testing.T) {
	theme := config.RetroTheme()
	buf := NewBuffer(80, 30, theme.DesktopBg)
	panel := settings.New(config.UserConfig{}, nil, nil)
	panel.Show()
	panel.NextTab() // switch to Workspace tab

	RenderSettingsPanel(buf, panel, theme)

	if !bufHasAnyNonSpace(buf) {
		t.Error("expected settings panel on second tab")
	}
}

func TestRenderSettingsPanel_ThirdTab(t *testing.T) {
	theme := config.RetroTheme()
	buf := NewBuffer(80, 30, theme.DesktopBg)
	panel := settings.New(config.UserConfig{}, nil, nil)
	panel.Show()
	panel.NextTab()
	panel.NextTab() // switch to Keybindings tab

	RenderSettingsPanel(buf, panel, theme)

	if !bufHasAnyNonSpace(buf) {
		t.Error("expected settings panel on keybindings tab")
	}
}

func TestRenderSettingsPanel_ItemNavigation(t *testing.T) {
	theme := config.RetroTheme()
	buf := NewBuffer(80, 30, theme.DesktopBg)
	panel := settings.New(config.UserConfig{}, nil, nil)
	panel.Show()
	panel.NextItem()
	panel.NextItem() // third item selected

	RenderSettingsPanel(buf, panel, theme)

	if !bufHasAnyNonSpace(buf) {
		t.Error("expected settings panel with navigated item")
	}
}

func TestRenderSettingsPanel_NarrowBuffer(t *testing.T) {
	theme := config.RetroTheme()
	buf := NewBuffer(40, 20, theme.DesktopBg)
	panel := settings.New(config.UserConfig{}, nil, nil)
	panel.Show()

	// Should not panic on narrow buffer
	RenderSettingsPanel(buf, panel, theme)

	if !bufHasAnyNonSpace(buf) {
		t.Error("expected settings panel on narrow buffer")
	}
}

// ----------------------------------------------------------------------------
// RenderShowKeys
// ----------------------------------------------------------------------------

func TestRenderShowKeys(t *testing.T) {
	theme := config.RetroTheme()
	buf := NewBuffer(80, 30, theme.DesktopBg)

	events := []showKeyEvent{
		{Key: "ctrl+a", Action: "new_terminal"},
		{Key: "f9", Action: "toggle_expose"},
		{Key: "g", Action: ""},
	}

	RenderShowKeys(buf, events, theme)

	if !bufHasNonSpaceInRegion(buf, 0, buf.Height-12, 30, buf.Height-1) {
		t.Error("expected show-keys overlay in bottom-left region")
	}
}

// ----------------------------------------------------------------------------
// RenderTooltip
// ----------------------------------------------------------------------------

func TestRenderTooltip_EmptyText(t *testing.T) {
	theme := config.RetroTheme()
	buf := NewBuffer(80, 30, theme.DesktopBg)

	RenderTooltip(buf, "", 10, 5, theme)

	// Empty text should produce no output
}

func TestRenderTooltip_BasicText(t *testing.T) {
	theme := config.RetroTheme()
	buf := NewBuffer(80, 30, theme.DesktopBg)

	RenderTooltip(buf, "Hello Tooltip", 10, 5, theme)

	// Should have content near (11, 6) — offset by +1
	if !bufHasNonSpaceInRegion(buf, 9, 4, 30, 10) {
		t.Error("expected tooltip content near position (10, 5)")
	}
}

func TestRenderTooltip_ClampRight(t *testing.T) {
	theme := config.RetroTheme()
	buf := NewBuffer(40, 15, theme.DesktopBg)

	// Position near right edge to trigger clamping
	RenderTooltip(buf, "Long tooltip text here", 35, 5, theme)

	if !bufHasAnyNonSpace(buf) {
		t.Error("expected tooltip content when clamped to right edge")
	}
}

func TestRenderTooltip_ClampBottom(t *testing.T) {
	theme := config.RetroTheme()
	buf := NewBuffer(40, 15, theme.DesktopBg)

	// Position near bottom to trigger clamping (ty = y - boxH)
	RenderTooltip(buf, "Bottom tooltip", 5, 14, theme)

	if !bufHasAnyNonSpace(buf) {
		t.Error("expected tooltip content when clamped to bottom edge")
	}
}

func TestRenderTooltip_TopLeft(t *testing.T) {
	theme := config.RetroTheme()
	buf := NewBuffer(80, 30, theme.DesktopBg)

	RenderTooltip(buf, "Corner", 0, 0, theme)

	if !bufHasAnyNonSpace(buf) {
		t.Error("expected tooltip at top-left corner")
	}
}

// ----------------------------------------------------------------------------
// RenderTour
// ----------------------------------------------------------------------------

func TestRenderTour_InactiveTour(t *testing.T) {
	theme := config.RetroTheme()
	buf := NewBuffer(80, 30, theme.DesktopBg)
	tr := tour.New(true) // completed = inactive

	RenderTour(buf, tr, theme)

	// Inactive tour should not render (CurrentStep() returns nil)
}

func TestRenderTour_FirstStep(t *testing.T) {
	theme := config.RetroTheme()
	buf := NewBuffer(80, 30, theme.DesktopBg)
	tr := tour.New(false) // active tour

	RenderTour(buf, tr, theme)

	cx, cy := buf.Width/2, buf.Height/2
	if !bufHasNonSpaceInRegion(buf, cx-25, cy-10, cx+25, cy+10) {
		t.Error("expected tour overlay content in center")
	}
}

func TestRenderTour_MiddleStep(t *testing.T) {
	theme := config.RetroTheme()
	buf := NewBuffer(80, 30, theme.DesktopBg)
	tr := tour.New(false)
	tr.Next() // step 2
	tr.Next() // step 3

	RenderTour(buf, tr, theme)

	if !bufHasAnyNonSpace(buf) {
		t.Error("expected tour overlay on middle step")
	}
}

func TestRenderTour_LastStep(t *testing.T) {
	theme := config.RetroTheme()
	buf := NewBuffer(80, 30, theme.DesktopBg)
	tr := tour.New(false)
	// Advance to last step
	for tr.Next() {
	}
	// Tour should be inactive now; re-create to test last step rendering
	tr2 := tour.New(false)
	for i := 0; i < len(tr2.Steps)-1; i++ {
		tr2.Next()
	}
	if !tr2.IsLast() {
		t.Fatal("expected to be on last step")
	}

	RenderTour(buf, tr2, theme)

	if !bufHasAnyNonSpace(buf) {
		t.Error("expected tour overlay on last step (Finish button)")
	}
}

func TestRenderTour_SkippedTour(t *testing.T) {
	theme := config.RetroTheme()
	buf := NewBuffer(80, 30, theme.DesktopBg)
	tr := tour.New(false)
	tr.Skip()

	RenderTour(buf, tr, theme)

	// After skip, tour should not render
}

// ----------------------------------------------------------------------------
// Cross-theme tests (ensure overlays render with different themes)
// ----------------------------------------------------------------------------

func TestOverlays_DifferentThemes(t *testing.T) {
	themes := []config.Theme{
		config.RetroTheme(),
		config.ModernTheme(),
	}

	for _, theme := range themes {
		t.Run(theme.Name, func(t *testing.T) {
			buf := NewBuffer(80, 30, theme.DesktopBg)

			// Clipboard
			clip := clipboard.New()
			clip.Copy("data")
			clip.ShowHistory()
			RenderClipboardHistory(buf, clip, theme)
			if !bufHasAnyNonSpace(buf) {
				t.Error("clipboard overlay should render with theme " + theme.Name)
			}

			// Context menu
			buf2 := NewBuffer(80, 30, theme.DesktopBg)
			menu := contextmenu.DesktopMenu(10, 5, contextmenu.KeyBindings{})
			RenderContextMenu(buf2, menu, theme)
			if !bufHasAnyNonSpace(buf2) {
				t.Error("context menu should render with theme " + theme.Name)
			}

			// Modal
			buf3 := NewBuffer(80, 30, theme.DesktopBg)
			modal := &ModalOverlay{Title: "Test", Lines: []string{"line"}}
			RenderModal(buf3, modal, theme)
			if !bufHasAnyNonSpace(buf3) {
				t.Error("modal should render with theme " + theme.Name)
			}

			// Toast
			buf4 := NewBuffer(80, 30, theme.DesktopBg)
			nm := notification.New()
			nm.Push("T", "B", notification.Info)
			RenderToasts(buf4, nm, theme)
			if !bufHasAnyNonSpace(buf4) {
				t.Error("toast should render with theme " + theme.Name)
			}

			// Tooltip
			buf5 := NewBuffer(80, 30, theme.DesktopBg)
			RenderTooltip(buf5, "tip", 10, 10, theme)
			if !bufHasAnyNonSpace(buf5) {
				t.Error("tooltip should render with theme " + theme.Name)
			}

			// Tour
			buf6 := NewBuffer(80, 30, theme.DesktopBg)
			tr := tour.New(false)
			RenderTour(buf6, tr, theme)
			if !bufHasAnyNonSpace(buf6) {
				t.Error("tour should render with theme " + theme.Name)
			}
		})
	}
}

package settings

import (
	"fmt"
	"strings"
	"testing"

	"github.com/icex/termdesk/internal/config"
	"github.com/icex/termdesk/internal/widget"
)

func TestNewPanel(t *testing.T) {
	cfg := config.DefaultUserConfig()
	p := New(cfg, nil, nil)
	if p == nil {
		t.Fatal("New() returned nil")
	}
	if len(p.Sections) < 4 {
		t.Errorf("expected at least 4 sections, got %d", len(p.Sections))
	}
	if p.Visible {
		t.Error("panel should be hidden after New")
	}
}

func TestGeneralSectionItems(t *testing.T) {
	cfg := config.DefaultUserConfig()
	p := New(cfg, nil, nil)
	gen := p.Sections[0]
	if !strings.Contains(gen.Title, "General") {
		t.Errorf("expected first section to contain 'General', got %q", gen.Title)
	}
	if len(gen.Items) < 3 {
		t.Errorf("expected at least 3 items in General, got %d", len(gen.Items))
	}
	// Theme should be a choice
	theme := gen.Items[0]
	if theme.Type != TypeChoice {
		t.Errorf("theme should be TypeChoice, got %d", theme.Type)
	}
	if theme.StrVal != "modern" {
		t.Errorf("default theme should be 'modern', got %q", theme.StrVal)
	}
	if len(theme.Choices) != 14 {
		t.Errorf("expected 14 theme choices, got %d", len(theme.Choices))
	}
	// Animations should be a toggle
	anim := gen.Items[1]
	if anim.Type != TypeToggle {
		t.Errorf("animations should be TypeToggle")
	}
	if !anim.BoolVal {
		t.Error("animations should default to true")
	}
}

func TestToggle(t *testing.T) {
	cfg := config.DefaultUserConfig()
	p := New(cfg, nil, nil)
	// Navigate to animations (index 1)
	p.ActiveItem = 1
	if !p.CurrentItem().BoolVal {
		t.Fatal("animations should start true")
	}
	p.Toggle()
	if p.CurrentItem().BoolVal {
		t.Error("animations should be false after toggle")
	}
	p.Toggle()
	if !p.CurrentItem().BoolVal {
		t.Error("animations should be true after second toggle")
	}
}

func TestToggleOnChoiceDoesNothing(t *testing.T) {
	cfg := config.DefaultUserConfig()
	p := New(cfg, nil, nil)
	p.ActiveItem = 0 // theme (choice type)
	before := p.CurrentItem().StrVal
	p.Toggle()
	if p.CurrentItem().StrVal != before {
		t.Error("toggle on choice item should not change value")
	}
}

func TestCycleChoice(t *testing.T) {
	cfg := config.DefaultUserConfig()
	p := New(cfg, nil, nil)
	p.ActiveItem = 0 // theme
	original := p.CurrentItem().StrVal

	p.CycleChoice(1)
	if p.CurrentItem().StrVal == original {
		t.Error("cycle forward should change value")
	}

	// Cycle backward to get back
	p.CycleChoice(-1)
	if p.CurrentItem().StrVal != original {
		t.Error("cycle backward should restore original")
	}
}

func TestCycleChoiceWraps(t *testing.T) {
	cfg := config.DefaultUserConfig()
	p := New(cfg, nil, nil)
	p.ActiveItem = 0 // theme
	choices := p.CurrentItem().Choices

	// Cycle forward through all choices + 1 to wrap
	for range len(choices) {
		p.CycleChoice(1)
	}
	if p.CurrentItem().StrVal != choices[0] {
		t.Errorf("expected wrap to first choice %q, got %q", choices[0], p.CurrentItem().StrVal)
	}
}

func TestCycleChoiceOnToggleDoesNothing(t *testing.T) {
	cfg := config.DefaultUserConfig()
	p := New(cfg, nil, nil)
	p.ActiveItem = 1 // animations (toggle)
	if p.CurrentItem().Type != TypeToggle {
		t.Fatalf("expected item 1 to be TypeToggle, got %d", p.CurrentItem().Type)
	}
	before := p.CurrentItem().BoolVal
	p.CycleChoice(1)
	if p.CurrentItem().BoolVal != before {
		t.Error("cycle on toggle should not change value")
	}
}

func TestItemNavigation(t *testing.T) {
	cfg := config.DefaultUserConfig()
	p := New(cfg, nil, nil)
	if p.ActiveItem != 0 {
		t.Errorf("should start at item 0, got %d", p.ActiveItem)
	}
	count := len(p.Sections[0].Items)
	if count < 7 {
		t.Errorf("expected at least 7 items in General section, got %d", count)
	}

	p.NextItem()
	if p.ActiveItem != 1 {
		t.Errorf("expected item 1, got %d", p.ActiveItem)
	}

	p.PrevItem()
	if p.ActiveItem != 0 {
		t.Errorf("expected item 0, got %d", p.ActiveItem)
	}

	// Wrap backward
	p.PrevItem()
	if p.ActiveItem != count-1 {
		t.Errorf("expected wrap to %d, got %d", count-1, p.ActiveItem)
	}

	// Wrap forward
	p.NextItem()
	if p.ActiveItem != 0 {
		t.Errorf("expected wrap to 0, got %d", p.ActiveItem)
	}
}

func TestTabNavigation(t *testing.T) {
	cfg := config.DefaultUserConfig()
	p := New(cfg, nil, nil)
	if p.ActiveTab != 0 {
		t.Error("should start at tab 0")
	}

	p.NextTab()
	if p.ActiveTab != 1 {
		t.Errorf("expected tab 1, got %d", p.ActiveTab)
	}
	if p.ActiveItem != 0 {
		t.Error("tab switch should reset ActiveItem to 0")
	}

	// Wrap forward
	for range len(p.Sections) {
		p.NextTab()
	}
	if p.ActiveTab != 1 {
		t.Errorf("expected wrap, got tab %d", p.ActiveTab)
	}

	p.PrevTab()
	if p.ActiveTab != 0 {
		t.Errorf("expected tab 0, got %d", p.ActiveTab)
	}
}

func TestApplyTo(t *testing.T) {
	cfg := config.DefaultUserConfig()
	p := New(cfg, nil, nil)

	// Change theme (General[0])
	p.ActiveItem = 0
	p.CycleChoice(1)
	newTheme := p.CurrentItem().StrVal

	// Toggle animations (General[1])
	p.ActiveItem = 1
	p.Toggle()

	// Change animation speed (General[2])
	p.ActiveItem = 2
	p.CycleChoice(1)
	newSpeed := p.CurrentItem().StrVal

	// Change animation style (General[3])
	p.ActiveItem = 3
	p.CycleChoice(1)
	newStyle := p.CurrentItem().StrVal

	// Toggle icons only (Dock[0])
	p.ActiveTab = 1 // Dock section
	p.ActiveItem = 0
	p.Toggle()

	// Apply
	p.ApplyTo(&cfg)
	if cfg.Theme != newTheme {
		t.Errorf("expected theme %q, got %q", newTheme, cfg.Theme)
	}
	if cfg.Animations != false {
		t.Error("expected animations false")
	}
	if cfg.AnimationSpeed != newSpeed {
		t.Errorf("expected animation_speed %q, got %q", newSpeed, cfg.AnimationSpeed)
	}
	if cfg.AnimationStyle != newStyle {
		t.Errorf("expected animation_style %q, got %q", newStyle, cfg.AnimationStyle)
	}
	if cfg.IconsOnly != true {
		t.Error("expected icons_only true")
	}
}

func TestCurrentItem(t *testing.T) {
	cfg := config.DefaultUserConfig()
	p := New(cfg, nil, nil)
	item := p.CurrentItem()
	if item == nil {
		t.Fatal("CurrentItem() should not return nil")
	}
	if item.Key != "theme" {
		t.Errorf("expected first item key 'theme', got %q", item.Key)
	}
}

func TestShowHide(t *testing.T) {
	cfg := config.DefaultUserConfig()
	p := New(cfg, nil, nil)
	p.Hide()
	if p.Visible {
		t.Error("should be hidden")
	}
	p.Show()
	if !p.Visible {
		t.Error("should be visible")
	}
}

func TestFormatAutoSaveInterval(t *testing.T) {
	// formatAutoSaveInterval is unexported, so test via New with different config values.
	// Workspace section is now index 2
	cfg := config.DefaultUserConfig()
	cfg.WorkspaceAutoSaveMin = 0
	p := New(cfg, nil, nil)
	wsSection := p.Sections[2] // Workspace section
	intervalItem := wsSection.Items[1]
	if intervalItem.StrVal != "1" {
		t.Errorf("expected interval '1' for minutes=0, got %q", intervalItem.StrVal)
	}

	// Test negative value (returns "1")
	cfg.WorkspaceAutoSaveMin = -5
	p = New(cfg, nil, nil)
	intervalItem = p.Sections[2].Items[1]
	if intervalItem.StrVal != "1" {
		t.Errorf("expected interval '1' for minutes=-5, got %q", intervalItem.StrVal)
	}

	// Test positive value (returns formatted string)
	cfg.WorkspaceAutoSaveMin = 10
	p = New(cfg, nil, nil)
	intervalItem = p.Sections[2].Items[1]
	if intervalItem.StrVal != "10" {
		t.Errorf("expected interval '10' for minutes=10, got %q", intervalItem.StrVal)
	}
}

func TestCurrentItemInvalidTab(t *testing.T) {
	cfg := config.DefaultUserConfig()
	p := New(cfg, nil, nil)

	// Negative tab index
	p.ActiveTab = -1
	if item := p.CurrentItem(); item != nil {
		t.Error("expected nil for negative ActiveTab")
	}

	// Tab index beyond sections
	p.ActiveTab = 999
	if item := p.CurrentItem(); item != nil {
		t.Error("expected nil for out-of-range ActiveTab")
	}
}

func TestCurrentItemInvalidItem(t *testing.T) {
	cfg := config.DefaultUserConfig()
	p := New(cfg, nil, nil)

	// Valid tab, negative item index
	p.ActiveTab = 0
	p.ActiveItem = -1
	if item := p.CurrentItem(); item != nil {
		t.Error("expected nil for negative ActiveItem")
	}

	// Valid tab, item index beyond items
	p.ActiveItem = 999
	if item := p.CurrentItem(); item != nil {
		t.Error("expected nil for out-of-range ActiveItem")
	}
}

func TestToggleWithNilItem(t *testing.T) {
	cfg := config.DefaultUserConfig()
	p := New(cfg, nil, nil)
	// Set invalid indices so CurrentItem returns nil
	p.ActiveTab = -1
	// Should not panic
	p.Toggle()
}

func TestCycleChoiceWithNilItem(t *testing.T) {
	cfg := config.DefaultUserConfig()
	p := New(cfg, nil, nil)
	// Set invalid indices so CurrentItem returns nil
	p.ActiveTab = -1
	// Should not panic
	p.CycleChoice(1)
	p.CycleChoice(-1)
}

func TestCycleChoiceBackwardWrap(t *testing.T) {
	cfg := config.DefaultUserConfig()
	p := New(cfg, nil, nil)
	p.ActiveItem = 0 // theme (first choice)
	choices := p.CurrentItem().Choices

	// Set StrVal to the first choice explicitly
	p.CurrentItem().StrVal = choices[0]

	// Cycle backward should wrap to last choice
	p.CycleChoice(-1)
	if p.CurrentItem().StrVal != choices[len(choices)-1] {
		t.Errorf("expected wrap to last choice %q, got %q", choices[len(choices)-1], p.CurrentItem().StrVal)
	}
}

func TestNextItemInvalidTab(t *testing.T) {
	cfg := config.DefaultUserConfig()
	p := New(cfg, nil, nil)
	p.ActiveTab = 999 // beyond sections
	p.ActiveItem = 0
	p.NextItem() // should be a no-op, not panic
	if p.ActiveItem != 0 {
		t.Errorf("expected ActiveItem unchanged, got %d", p.ActiveItem)
	}
}

func TestPrevItemInvalidTab(t *testing.T) {
	cfg := config.DefaultUserConfig()
	p := New(cfg, nil, nil)
	p.ActiveTab = 999 // beyond sections
	p.ActiveItem = 0
	p.PrevItem() // should be a no-op, not panic
	if p.ActiveItem != 0 {
		t.Errorf("expected ActiveItem unchanged, got %d", p.ActiveItem)
	}
}

func TestNextItemEmptySection(t *testing.T) {
	p := &Panel{
		Sections:  []Section{{Title: "Empty", Items: []Item{}}},
		ActiveTab: 0,
	}
	p.NextItem() // should be a no-op, not panic
	if p.ActiveItem != 0 {
		t.Errorf("expected ActiveItem 0 for empty section, got %d", p.ActiveItem)
	}
}

func TestPrevItemEmptySection(t *testing.T) {
	p := &Panel{
		Sections:  []Section{{Title: "Empty", Items: []Item{}}},
		ActiveTab: 0,
	}
	p.PrevItem() // should be a no-op, not panic
	if p.ActiveItem != 0 {
		t.Errorf("expected ActiveItem 0 for empty section, got %d", p.ActiveItem)
	}
}

func TestPrevTabWrapFromZero(t *testing.T) {
	cfg := config.DefaultUserConfig()
	p := New(cfg, nil, nil)
	if p.ActiveTab != 0 {
		t.Fatalf("expected starting tab 0, got %d", p.ActiveTab)
	}
	// Move some item forward first to verify ActiveItem resets
	p.ActiveItem = 3
	// PrevTab from 0 should wrap to last section
	p.PrevTab()
	lastTab := len(p.Sections) - 1
	if p.ActiveTab != lastTab {
		t.Errorf("expected wrap to tab %d, got %d", lastTab, p.ActiveTab)
	}
	if p.ActiveItem != 0 {
		t.Errorf("expected ActiveItem reset to 0, got %d", p.ActiveItem)
	}
}

func TestSetTabValid(t *testing.T) {
	cfg := config.DefaultUserConfig()
	p := New(cfg, nil, nil)
	p.ActiveItem = 5 // some non-zero item

	p.SetTab(1)
	if p.ActiveTab != 1 {
		t.Errorf("expected tab 1, got %d", p.ActiveTab)
	}
	if p.ActiveItem != 0 {
		t.Errorf("expected ActiveItem reset to 0, got %d", p.ActiveItem)
	}

	p.SetTab(2)
	if p.ActiveTab != 2 {
		t.Errorf("expected tab 2, got %d", p.ActiveTab)
	}

	// Set to 0
	p.SetTab(0)
	if p.ActiveTab != 0 {
		t.Errorf("expected tab 0, got %d", p.ActiveTab)
	}
}

func TestSetTabInvalid(t *testing.T) {
	cfg := config.DefaultUserConfig()
	p := New(cfg, nil, nil)
	p.SetTab(1)

	// Negative index should be ignored
	p.SetTab(-1)
	if p.ActiveTab != 1 {
		t.Errorf("expected tab unchanged at 1, got %d", p.ActiveTab)
	}

	// Out-of-range index should be ignored
	p.SetTab(999)
	if p.ActiveTab != 1 {
		t.Errorf("expected tab unchanged at 1, got %d", p.ActiveTab)
	}
}

func TestSetItemValid(t *testing.T) {
	cfg := config.DefaultUserConfig()
	p := New(cfg, nil, nil)
	p.ActiveTab = 0

	p.SetItem(3)
	if p.ActiveItem != 3 {
		t.Errorf("expected item 3, got %d", p.ActiveItem)
	}

	p.SetItem(0)
	if p.ActiveItem != 0 {
		t.Errorf("expected item 0, got %d", p.ActiveItem)
	}

	// Last item in section
	lastIdx := len(p.Sections[0].Items) - 1
	p.SetItem(lastIdx)
	if p.ActiveItem != lastIdx {
		t.Errorf("expected item %d, got %d", lastIdx, p.ActiveItem)
	}
}

func TestSetItemInvalid(t *testing.T) {
	cfg := config.DefaultUserConfig()
	p := New(cfg, nil, nil)
	p.ActiveTab = 0
	p.ActiveItem = 2

	// Negative index should be ignored
	p.SetItem(-1)
	if p.ActiveItem != 2 {
		t.Errorf("expected item unchanged at 2, got %d", p.ActiveItem)
	}

	// Out-of-range index should be ignored
	p.SetItem(999)
	if p.ActiveItem != 2 {
		t.Errorf("expected item unchanged at 2, got %d", p.ActiveItem)
	}
}

func TestSetItemInvalidTab(t *testing.T) {
	cfg := config.DefaultUserConfig()
	p := New(cfg, nil, nil)
	// Set an invalid tab
	p.ActiveTab = -1
	p.ActiveItem = 0
	p.SetItem(1) // should be a no-op
	if p.ActiveItem != 0 {
		t.Errorf("expected item unchanged at 0, got %d", p.ActiveItem)
	}

	p.ActiveTab = 999
	p.SetItem(1) // should be a no-op
	if p.ActiveItem != 0 {
		t.Errorf("expected item unchanged at 0, got %d", p.ActiveItem)
	}
}

func TestApplyToWorkspaceSettings(t *testing.T) {
	cfg := config.DefaultUserConfig()
	p := New(cfg, nil, nil)

	// Navigate to Workspace tab (now index 2)
	p.ActiveTab = 2

	// Toggle workspace_auto_save (index 0)
	p.ActiveItem = 0
	originalAutoSave := p.CurrentItem().BoolVal
	p.Toggle()

	// Change auto-save interval (index 1)
	p.ActiveItem = 1
	p.CycleChoice(1) // cycle to next interval
	newInterval := p.CurrentItem().StrVal

	p.ApplyTo(&cfg)

	if cfg.WorkspaceAutoSave == originalAutoSave {
		t.Error("expected WorkspaceAutoSave to be toggled")
	}

	expectedInterval := newInterval
	if fmt.Sprintf("%d", cfg.WorkspaceAutoSaveMin) != expectedInterval {
		t.Errorf("expected WorkspaceAutoSaveMin %s, got %d", expectedInterval, cfg.WorkspaceAutoSaveMin)
	}
}

func TestApplyToPrefixKey(t *testing.T) {
	cfg := config.DefaultUserConfig()
	p := New(cfg, nil, nil)

	// Navigate to Keybindings tab (now index 3)
	p.ActiveTab = 3
	p.ActiveItem = 0 // prefix key
	p.CycleChoice(1) // change prefix key
	newPrefix := p.CurrentItem().StrVal

	p.ApplyTo(&cfg)
	if cfg.Keys.Prefix != newPrefix {
		t.Errorf("expected prefix %q, got %q", newPrefix, cfg.Keys.Prefix)
	}
}

func TestApplyToShowDeskClock(t *testing.T) {
	cfg := config.DefaultUserConfig()
	p := New(cfg, nil, nil)

	// show_desk_clock is at index 4 in General section
	p.ActiveTab = 0
	p.ActiveItem = 4
	if p.CurrentItem().Key != "show_desk_clock" {
		t.Fatalf("expected key 'show_desk_clock', got %q", p.CurrentItem().Key)
	}
	p.Toggle()

	p.ApplyTo(&cfg)
	if cfg.ShowDeskClock != true {
		t.Error("expected ShowDeskClock to be true after toggle")
	}
}

func TestApplyToShowKeys(t *testing.T) {
	cfg := config.DefaultUserConfig()
	p := New(cfg, nil, nil)

	// show_keys is at index 5 in General section
	p.ActiveTab = 0
	p.ActiveItem = 5
	if p.CurrentItem().Key != "show_keys" {
		t.Fatalf("expected key 'show_keys', got %q", p.CurrentItem().Key)
	}
	p.Toggle()

	p.ApplyTo(&cfg)
	if cfg.ShowKeys != true {
		t.Error("expected ShowKeys to be true after toggle")
	}
}

func TestApplyToTilingMode(t *testing.T) {
	cfg := config.DefaultUserConfig()
	p := New(cfg, nil, nil)

	// tiling_mode is at index 6 in General section
	p.ActiveTab = 0
	p.ActiveItem = 6
	if p.CurrentItem().Key != "tiling_mode" {
		t.Fatalf("expected key 'tiling_mode', got %q", p.CurrentItem().Key)
	}
	p.Toggle()

	p.ApplyTo(&cfg)
	if cfg.TilingMode != true {
		t.Error("expected TilingMode to be true after toggle")
	}
}

func TestApplyToMinimizeToDock(t *testing.T) {
	cfg := config.DefaultUserConfig()
	p := New(cfg, nil, nil)

	// minimize_to_dock is at index 1 in Dock section (tab 1)
	p.ActiveTab = 1
	p.ActiveItem = 1
	if p.CurrentItem().Key != "minimize_to_dock" {
		t.Fatalf("expected key 'minimize_to_dock', got %q", p.CurrentItem().Key)
	}
	// Default is true, toggle to false
	p.Toggle()

	p.ApplyTo(&cfg)
	if cfg.MinimizeToDock != false {
		t.Error("expected MinimizeToDock to be false after toggle")
	}
}

func TestApplyToHideDockWhenMaximized(t *testing.T) {
	cfg := config.DefaultUserConfig()
	p := New(cfg, nil, nil)

	// hide_dock_when_maximized is at index 2 in Dock section (tab 1)
	p.ActiveTab = 1
	p.ActiveItem = 2
	if p.CurrentItem().Key != "hide_dock_when_maximized" {
		t.Fatalf("expected key 'hide_dock_when_maximized', got %q", p.CurrentItem().Key)
	}
	p.Toggle()

	p.ApplyTo(&cfg)
	if cfg.HideDockWhenMaximized != true {
		t.Error("expected HideDockWhenMaximized to be true after toggle")
	}
}

func TestApplyToAutoSaveInvalidInterval(t *testing.T) {
	cfg := config.DefaultUserConfig()
	p := New(cfg, nil, nil)

	// Directly set the auto-save interval item to a non-numeric value
	// to test the fmt.Sscanf failure / minutes <= 0 path in ApplyTo
	p.Sections[2].Items[1].StrVal = "invalid"
	originalMin := cfg.WorkspaceAutoSaveMin
	p.ApplyTo(&cfg)
	// When Sscanf fails, minutes is 0, so the if minutes > 0 branch is skipped
	// and WorkspaceAutoSaveMin should remain unchanged
	if cfg.WorkspaceAutoSaveMin != originalMin {
		t.Errorf("expected WorkspaceAutoSaveMin unchanged at %d, got %d", originalMin, cfg.WorkspaceAutoSaveMin)
	}
}

func TestShowResetsState(t *testing.T) {
	cfg := config.DefaultUserConfig()
	p := New(cfg, nil, nil)

	// Move to non-zero tab and item
	p.ActiveTab = 2
	p.ActiveItem = 3

	// Show should reset state
	p.Show()
	if p.ActiveTab != 0 {
		t.Errorf("expected Show to reset ActiveTab to 0, got %d", p.ActiveTab)
	}
	if p.ActiveItem != 0 {
		t.Errorf("expected Show to reset ActiveItem to 0, got %d", p.ActiveItem)
	}
	if !p.Visible {
		t.Error("expected Visible to be true after Show")
	}
}

func TestDockSectionStructure(t *testing.T) {
	cfg := config.DefaultUserConfig()
	p := New(cfg, nil, nil)

	dock := p.Sections[1]
	if !strings.Contains(dock.Title, "Dock") {
		t.Errorf("expected second section to contain 'Dock', got %q", dock.Title)
	}
	if len(dock.Items) != 4 {
		t.Fatalf("expected 4 items in Dock, got %d", len(dock.Items))
	}
	expectedKeys := []string{"icons_only", "minimize_to_dock", "hide_dock_when_maximized", "hide_dock_apps"}
	for i, ek := range expectedKeys {
		if dock.Items[i].Key != ek {
			t.Errorf("expected dock item %d key %q, got %q", i, ek, dock.Items[i].Key)
		}
	}
}

func TestWorkspaceSectionStructure(t *testing.T) {
	cfg := config.DefaultUserConfig()
	p := New(cfg, nil, nil)

	ws := p.Sections[2]
	if !strings.Contains(ws.Title, "Workspace") {
		t.Errorf("expected third section to contain 'Workspace', got %q", ws.Title)
	}
	if len(ws.Items) != 2 {
		t.Fatalf("expected 2 items in Workspace, got %d", len(ws.Items))
	}
	if ws.Items[0].Key != "workspace_auto_save" {
		t.Errorf("expected first workspace item key 'workspace_auto_save', got %q", ws.Items[0].Key)
	}
	if ws.Items[0].Type != TypeToggle {
		t.Error("workspace_auto_save should be TypeToggle")
	}
	if ws.Items[1].Key != "workspace_auto_save_min" {
		t.Errorf("expected second workspace item key 'workspace_auto_save_min', got %q", ws.Items[1].Key)
	}
	if ws.Items[1].Type != TypeChoice {
		t.Error("workspace_auto_save_min should be TypeChoice")
	}
}

func TestKeybindingsSectionStructure(t *testing.T) {
	cfg := config.DefaultUserConfig()
	p := New(cfg, nil, nil)

	kb := p.Sections[3]
	if !strings.Contains(kb.Title, "Keybindings") {
		t.Errorf("expected fourth section to contain 'Keybindings', got %q", kb.Title)
	}
	if len(kb.Items) != 5 {
		t.Fatalf("expected 5 items in Keybindings, got %d", len(kb.Items))
	}
	expectedKeys := []string{"prefix", "quit", "new_terminal", "enter_terminal", "quake_terminal"}
	for i, ek := range expectedKeys {
		if kb.Items[i].Key != ek {
			t.Errorf("expected keybinding item %d key %q, got %q", i, ek, kb.Items[i].Key)
		}
	}
}

// --- Widgets tab tests ---

func TestWidgetsTabCreated(t *testing.T) {
	cfg := config.DefaultUserConfig()
	metas := widget.DefaultRegistry().All()
	enabled := widget.DefaultEnabledWidgets()
	p := New(cfg, metas, enabled)

	if len(p.Sections) != 6 {
		t.Fatalf("expected 6 sections with widgets tab, got %d", len(p.Sections))
	}
	ws := p.Sections[5]
	if !strings.Contains(ws.Title, "Widgets") {
		t.Errorf("expected 5th section to contain 'Widgets', got %q", ws.Title)
	}
	if len(ws.Items) != len(metas) {
		t.Errorf("expected %d widget items, got %d", len(metas), len(ws.Items))
	}
	// Default-enabled widgets should be enabled; others should be disabled.
	enabledSet := make(map[string]bool)
	for _, name := range enabled {
		enabledSet["widget_"+name] = true
	}
	for _, item := range ws.Items {
		if enabledSet[item.Key] && !item.BoolVal {
			t.Errorf("widget %q should be enabled by default", item.Key)
		}
		if !enabledSet[item.Key] && item.BoolVal {
			t.Errorf("widget %q should be disabled by default", item.Key)
		}
	}
}

func TestWidgetsTabNoMetasNoTab(t *testing.T) {
	cfg := config.DefaultUserConfig()
	p := New(cfg, nil, nil)
	if len(p.Sections) != 5 {
		t.Fatalf("expected 5 sections without widget metas, got %d", len(p.Sections))
	}
}

func TestWidgetsTabPartialEnabled(t *testing.T) {
	cfg := config.DefaultUserConfig()
	metas := widget.DefaultRegistry().All()
	enabled := []string{"cpu", "clock"}
	p := New(cfg, metas, enabled)

	ws := p.Sections[5]
	for _, item := range ws.Items {
		name := item.Key[len("widget_"):]
		shouldBeOn := name == "cpu" || name == "clock"
		if item.BoolVal != shouldBeOn {
			t.Errorf("widget %q: enabled=%v, want %v", name, item.BoolVal, shouldBeOn)
		}
	}
}

func TestWidgetsTabApplyTo(t *testing.T) {
	cfg := config.DefaultUserConfig()
	metas := widget.DefaultRegistry().All()
	enabled := widget.DefaultEnabledWidgets()
	p := New(cfg, metas, enabled)

	// Disable battery (find it in widgets tab)
	p.ActiveTab = 5
	for i, item := range p.Sections[5].Items {
		if item.Key == "widget_battery" {
			p.ActiveItem = i
			p.Toggle()
			break
		}
	}

	p.ApplyTo(&cfg)

	// Battery should not be in enabled list
	for _, name := range cfg.EnabledWidgets {
		if name == "battery" {
			t.Fatal("battery should be disabled after toggle")
		}
	}
	// Other widgets should still be present
	if len(cfg.EnabledWidgets) != len(enabled)-1 {
		t.Errorf("expected %d enabled widgets, got %d", len(enabled)-1, len(cfg.EnabledWidgets))
	}
}

func TestWidgetsTabCustomWidget(t *testing.T) {
	cfg := config.DefaultUserConfig()
	reg := widget.DefaultRegistry()
	reg.Register(widget.WidgetMeta{Name: "mytest", Label: "My Test Widget", Builtin: false})
	metas := reg.All()
	enabled := []string{"cpu", "clock", "mytest"}
	p := New(cfg, metas, enabled)

	ws := p.Sections[5]
	// Find the custom widget item
	var found bool
	for _, item := range ws.Items {
		if item.Key == "widget_mytest" {
			found = true
			if !item.BoolVal {
				t.Error("mytest widget should be enabled")
			}
			if item.Label != "My Test Widget (custom)" {
				t.Errorf("expected label with (custom) suffix, got %q", item.Label)
			}
		}
	}
	if !found {
		t.Fatal("mytest widget item not found in Widgets tab")
	}
}

// --- InnerWidth tests ---

func TestInnerWidthLargeScreen(t *testing.T) {
	cfg := config.DefaultUserConfig()
	p := New(cfg, nil, nil)

	// Screen much wider than PanelWidth: should return PanelWidth unchanged.
	w := p.InnerWidth(200)
	if w != PanelWidth {
		t.Errorf("expected InnerWidth=%d for large screen, got %d", PanelWidth, w)
	}
}

func TestInnerWidthExactFit(t *testing.T) {
	cfg := config.DefaultUserConfig()
	p := New(cfg, nil, nil)

	// Screen width = PanelWidth + 4: w = PanelWidth, and PanelWidth > screenW-4
	// is false (56 > 56 is false), so should return PanelWidth.
	w := p.InnerWidth(PanelWidth + 4)
	if w != PanelWidth {
		t.Errorf("expected InnerWidth=%d for exact fit, got %d", PanelWidth, w)
	}
}

func TestInnerWidthNarrowScreen(t *testing.T) {
	cfg := config.DefaultUserConfig()
	p := New(cfg, nil, nil)

	// Screen narrower than PanelWidth+4: should clamp.
	screenW := 30
	expected := screenW - 4 // 26
	w := p.InnerWidth(screenW)
	if w != expected {
		t.Errorf("expected InnerWidth=%d for narrow screen (screenW=%d), got %d", expected, screenW, w)
	}
}

func TestInnerWidthVeryNarrowScreen(t *testing.T) {
	cfg := config.DefaultUserConfig()
	p := New(cfg, nil, nil)

	// Very narrow screen: screenW-4 could be 0 or negative.
	w := p.InnerWidth(3)
	if w != -1 {
		t.Errorf("expected InnerWidth=-1 for screenW=3, got %d", w)
	}

	w = p.InnerWidth(4)
	if w != 0 {
		t.Errorf("expected InnerWidth=0 for screenW=4, got %d", w)
	}
}

// --- Bounds tests ---

func TestBoundsNormalScreen(t *testing.T) {
	cfg := config.DefaultUserConfig()
	p := New(cfg, nil, nil)

	// Use a large screen so panel is centered.
	screenW, screenH := 120, 40
	r := p.Bounds(screenW, screenH)

	// InnerWidth should be PanelWidth since screen is large.
	innerW := PanelWidth
	sec := p.Sections[p.ActiveTab]
	innerH := HeaderRows + len(sec.Items) + FooterRows
	boxW := innerW + 2
	boxH := innerH + 2
	expectedX := (screenW - boxW) / 2
	expectedY := (screenH - boxH) / 2

	if r.X != expectedX {
		t.Errorf("Bounds X: expected %d, got %d", expectedX, r.X)
	}
	if r.Y != expectedY {
		t.Errorf("Bounds Y: expected %d, got %d", expectedY, r.Y)
	}
	if r.Width != boxW {
		t.Errorf("Bounds Width: expected %d, got %d", boxW, r.Width)
	}
	if r.Height != boxH {
		t.Errorf("Bounds Height: expected %d, got %d", boxH, r.Height)
	}
}

func TestBoundsSmallScreen(t *testing.T) {
	cfg := config.DefaultUserConfig()
	p := New(cfg, nil, nil)

	// Small screen where startX would be negative and startY would be < 1.
	screenW, screenH := 10, 5
	r := p.Bounds(screenW, screenH)

	// startX and startY should be clamped.
	if r.X < 0 {
		t.Errorf("Bounds X should not be negative, got %d", r.X)
	}
	if r.Y < 1 {
		t.Errorf("Bounds Y should be at least 1, got %d", r.Y)
	}
}

func TestBoundsDifferentTabs(t *testing.T) {
	cfg := config.DefaultUserConfig()
	p := New(cfg, nil, nil)
	screenW, screenH := 120, 60

	// Bounds depends on ActiveTab because different sections have different item counts.
	r0 := p.Bounds(screenW, screenH)

	p.ActiveTab = 1 // Dock (4 items vs General's 8)
	r1 := p.Bounds(screenW, screenH)

	// Same width, but different heights.
	if r0.Width != r1.Width {
		t.Errorf("expected same width, got %d vs %d", r0.Width, r1.Width)
	}
	// General has more items than Dock, so General should be taller.
	if r0.Height <= r1.Height {
		t.Errorf("General panel (height=%d) should be taller than Dock panel (height=%d)", r0.Height, r1.Height)
	}
}

// --- TabAtX tests ---

func TestTabAtXHitsAllTabs(t *testing.T) {
	cfg := config.DefaultUserConfig()
	p := New(cfg, nil, nil)
	innerW := PanelWidth

	// Calculate tab positions to verify each tab is hittable.
	totalW := 0
	for _, s := range p.Sections {
		totalW += len([]rune(s.Title)) + 2
	}
	offset := (innerW - totalW) / 2

	x := offset
	for i, s := range p.Sections {
		tabW := len([]rune(s.Title)) + 2
		// First character of tab
		idx := p.TabAtX(x, innerW)
		if idx != i {
			t.Errorf("TabAtX(%d) at start of tab %d (%q): expected %d, got %d", x, i, s.Title, i, idx)
		}
		// Last character of tab
		idx = p.TabAtX(x+tabW-1, innerW)
		if idx != i {
			t.Errorf("TabAtX(%d) at end of tab %d (%q): expected %d, got %d", x+tabW-1, i, s.Title, i, idx)
		}
		x += tabW
	}
}

func TestTabAtXBeforeTabs(t *testing.T) {
	cfg := config.DefaultUserConfig()
	p := New(cfg, nil, nil)
	innerW := PanelWidth

	// X=0 is before the centered tabs, should return -1.
	idx := p.TabAtX(0, innerW)
	if idx != -1 {
		t.Errorf("TabAtX(0) should return -1 for area before tabs, got %d", idx)
	}
}

func TestTabAtXAfterTabs(t *testing.T) {
	cfg := config.DefaultUserConfig()
	p := New(cfg, nil, nil)
	innerW := PanelWidth

	// X at end of panel, should return -1.
	idx := p.TabAtX(innerW-1, innerW)
	if idx != -1 {
		t.Errorf("TabAtX(%d) should return -1 for area after tabs, got %d", innerW-1, idx)
	}
}

func TestTabAtXNarrowPanel(t *testing.T) {
	cfg := config.DefaultUserConfig()
	p := New(cfg, nil, nil)

	// Use a very narrow innerW where offset would be negative (clamped to 0).
	// Total tab width for 5 sections exceeds innerW.
	innerW := 10
	// First tab should still be hittable at x=0.
	idx := p.TabAtX(0, innerW)
	if idx != 0 {
		t.Errorf("TabAtX(0) with narrow panel: expected 0, got %d", idx)
	}
}

// --- ItemAtY tests ---

func TestItemAtYValidItems(t *testing.T) {
	cfg := config.DefaultUserConfig()
	p := New(cfg, nil, nil)

	// Items start at ItemStartRow (6).
	itemCount := len(p.Sections[0].Items)
	for i := 0; i < itemCount; i++ {
		relY := ItemStartRow + i
		idx := p.ItemAtY(relY)
		if idx != i {
			t.Errorf("ItemAtY(%d): expected item %d, got %d", relY, i, idx)
		}
	}
}

func TestItemAtYBelowItems(t *testing.T) {
	cfg := config.DefaultUserConfig()
	p := New(cfg, nil, nil)

	itemCount := len(p.Sections[0].Items)
	relY := ItemStartRow + itemCount // one past the last item
	idx := p.ItemAtY(relY)
	if idx != -1 {
		t.Errorf("ItemAtY(%d) below items: expected -1, got %d", relY, idx)
	}
}

func TestItemAtYAboveItems(t *testing.T) {
	cfg := config.DefaultUserConfig()
	p := New(cfg, nil, nil)

	// relY before ItemStartRow should return -1.
	for relY := 0; relY < ItemStartRow; relY++ {
		idx := p.ItemAtY(relY)
		if idx != -1 {
			t.Errorf("ItemAtY(%d) above items: expected -1, got %d", relY, idx)
		}
	}
}

func TestItemAtYNegativeRelY(t *testing.T) {
	cfg := config.DefaultUserConfig()
	p := New(cfg, nil, nil)

	idx := p.ItemAtY(-5)
	if idx != -1 {
		t.Errorf("ItemAtY(-5): expected -1, got %d", idx)
	}
}

func TestItemAtYInvalidTab(t *testing.T) {
	cfg := config.DefaultUserConfig()
	p := New(cfg, nil, nil)
	p.ActiveTab = 999 // out of range

	idx := p.ItemAtY(ItemStartRow)
	if idx != -1 {
		t.Errorf("ItemAtY with invalid tab: expected -1, got %d", idx)
	}
}

func TestItemAtYDifferentTabs(t *testing.T) {
	cfg := config.DefaultUserConfig()
	p := New(cfg, nil, nil)

	// Switch to Dock tab (fewer items).
	p.ActiveTab = 1
	dockItemCount := len(p.Sections[1].Items)

	// Last valid item in Dock.
	relY := ItemStartRow + dockItemCount - 1
	idx := p.ItemAtY(relY)
	if idx != dockItemCount-1 {
		t.Errorf("ItemAtY(%d) last Dock item: expected %d, got %d", relY, dockItemCount-1, idx)
	}

	// One past last Dock item.
	relY = ItemStartRow + dockItemCount
	idx = p.ItemAtY(relY)
	if idx != -1 {
		t.Errorf("ItemAtY(%d) past Dock items: expected -1, got %d", relY, idx)
	}
}

// --- ApplyTo tests for remaining keys ---

func TestApplyToDefaultTerminalMode(t *testing.T) {
	cfg := config.DefaultUserConfig()
	p := New(cfg, nil, nil)

	// default_terminal_mode is at index 7 in General section
	p.ActiveTab = 0
	p.ActiveItem = 7
	if p.CurrentItem().Key != "default_terminal_mode" {
		t.Fatalf("expected key 'default_terminal_mode', got %q", p.CurrentItem().Key)
	}
	p.Toggle()

	p.ApplyTo(&cfg)
	if cfg.DefaultTerminalMode != true {
		t.Error("expected DefaultTerminalMode to be true after toggle")
	}
}

func TestApplyToHideDockApps(t *testing.T) {
	cfg := config.DefaultUserConfig()
	p := New(cfg, nil, nil)

	// hide_dock_apps is at index 3 in Dock section (tab 1)
	p.ActiveTab = 1
	p.ActiveItem = 3
	if p.CurrentItem().Key != "hide_dock_apps" {
		t.Fatalf("expected key 'hide_dock_apps', got %q", p.CurrentItem().Key)
	}
	p.Toggle()

	p.ApplyTo(&cfg)
	if cfg.HideDockApps != true {
		t.Error("expected HideDockApps to be true after toggle")
	}
}

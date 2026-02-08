package config

import "testing"

func TestRetroTheme(t *testing.T) {
	theme := RetroTheme()
	if theme.Name != "retro" {
		t.Errorf("Name = %q, want retro", theme.Name)
	}
	if theme.BorderTopLeft != '┌' {
		t.Error("expected ┌ for top-left border")
	}
	if theme.CloseButton != "[X]" {
		t.Errorf("CloseButton = %q, want [X]", theme.CloseButton)
	}
	if theme.ActiveBorderFg == "" {
		t.Error("expected non-empty ActiveBorderFg")
	}
	if theme.DesktopBg == "" {
		t.Error("expected non-empty DesktopBg")
	}
}

func TestModernTheme(t *testing.T) {
	theme := ModernTheme()
	if theme.Name != "modern" {
		t.Errorf("Name = %q, want modern", theme.Name)
	}
	if theme.BorderTopLeft != '╭' {
		t.Error("expected ╭ for top-left border")
	}
	if theme.CloseButton != "[×]" {
		t.Errorf("CloseButton = %q, want [×]", theme.CloseButton)
	}
}

func TestGetThemeRetro(t *testing.T) {
	theme := GetTheme("retro")
	if theme.Name != "retro" {
		t.Errorf("GetTheme(retro) = %q", theme.Name)
	}
}

func TestGetThemeModern(t *testing.T) {
	theme := GetTheme("modern")
	if theme.Name != "modern" {
		t.Errorf("GetTheme(modern) = %q", theme.Name)
	}
}

func TestGetThemeDefault(t *testing.T) {
	theme := GetTheme("unknown")
	if theme.Name != "modern" {
		t.Errorf("GetTheme(unknown) = %q, want modern fallback", theme.Name)
	}
}

func TestTokyoNightTheme(t *testing.T) {
	theme := TokyoNightTheme()
	if theme.Name != "tokyonight" {
		t.Errorf("Name = %q, want tokyonight", theme.Name)
	}
	if theme.BorderTopLeft != '╭' {
		t.Error("expected rounded borders")
	}
}

func TestCatppuccinTheme(t *testing.T) {
	theme := CatppuccinTheme()
	if theme.Name != "catppuccin" {
		t.Errorf("Name = %q, want catppuccin", theme.Name)
	}
}

func TestThemeNames(t *testing.T) {
	names := ThemeNames()
	if len(names) != 4 {
		t.Errorf("ThemeNames() returned %d themes, want 4", len(names))
	}
	// Verify all theme names resolve
	for _, name := range names {
		theme := GetTheme(name)
		if theme.Name != name {
			t.Errorf("GetTheme(%q).Name = %q", name, theme.Name)
		}
	}
}

func TestThemeNewFields(t *testing.T) {
	for _, name := range ThemeNames() {
		theme := GetTheme(name)
		if theme.SnapLeftButton == "" {
			t.Errorf("theme %q: SnapLeftButton is empty", name)
		}
		if theme.SnapRightButton == "" {
			t.Errorf("theme %q: SnapRightButton is empty", name)
		}
		if theme.DockAccentBg == "" {
			t.Errorf("theme %q: DockAccentBg is empty", name)
		}
		if theme.ContentBg == "" {
			t.Errorf("theme %q: ContentBg is empty", name)
		}
		if theme.UnfocusedFade <= 0 {
			t.Errorf("theme %q: UnfocusedFade should be > 0", name)
		}
	}
}

func TestThemeColorsDiffer(t *testing.T) {
	theme := RetroTheme()
	// With transparent borders, backgrounds match desktop bg;
	// active vs inactive is distinguished by foreground colors.
	if theme.ActiveBorderFg == theme.InactiveBorderFg {
		t.Error("active and inactive border foregrounds should differ")
	}
	if theme.ActiveTitleFg == theme.InactiveTitleFg {
		t.Error("active and inactive title foregrounds should differ")
	}
}

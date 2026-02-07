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
	if theme.Name != "retro" {
		t.Errorf("GetTheme(unknown) = %q, want retro fallback", theme.Name)
	}
}

func TestThemeColorsDiffer(t *testing.T) {
	theme := RetroTheme()
	if theme.ActiveBorderBg == theme.InactiveBorderBg {
		t.Error("active and inactive border backgrounds should differ")
	}
	if theme.ActiveTitleFg == theme.InactiveTitleFg {
		t.Error("active and inactive title foregrounds should differ")
	}
}

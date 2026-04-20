package config

import (
	"image/color"
	"testing"
)

func TestRetroTheme(t *testing.T) {
	theme := RetroTheme()
	if theme.Name != "retro" {
		t.Errorf("Name = %q, want retro", theme.Name)
	}
	if theme.BorderTopLeft != '┌' {
		t.Error("expected ┌ for top-left border")
	}
	if theme.CloseButton != " ▼ " {
		t.Errorf("CloseButton = %q, want \" ▼ \"", theme.CloseButton)
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
	if theme.CloseButton != " \ueab8 " {
		t.Errorf("CloseButton = %q, want nf-cod-chrome_close", theme.CloseButton)
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
	if len(names) != 14 {
		t.Errorf("ThemeNames() returned %d themes, want 14", len(names))
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

func TestRedmondTheme(t *testing.T) {
	theme := RedmondTheme()
	if theme.Name != "redmond" {
		t.Errorf("Name = %q, want redmond", theme.Name)
	}
}

func TestPlatinumTheme(t *testing.T) {
	theme := PlatinumTheme()
	if theme.Name != "platinum" {
		t.Errorf("Name = %q, want platinum", theme.Name)
	}
}

func TestUbuntuTheme(t *testing.T) {
	theme := UbuntuTheme()
	if theme.Name != "ubuntu" {
		t.Errorf("Name = %q, want ubuntu", theme.Name)
	}
}

func TestAquaTheme(t *testing.T) {
	theme := AquaTheme()
	if theme.Name != "aqua" {
		t.Errorf("Name = %q, want aqua", theme.Name)
	}
	if theme.BorderTopLeft != '╭' {
		t.Error("expected rounded borders for aqua")
	}
}

func TestSpringboardTheme(t *testing.T) {
	theme := SpringboardTheme()
	if theme.Name != "springboard" {
		t.Errorf("Name = %q, want springboard", theme.Name)
	}
}

func TestNordTheme(t *testing.T) {
	theme := NordTheme()
	if theme.Name != "nord" {
		t.Errorf("Name = %q, want nord", theme.Name)
	}
}

func TestDraculaTheme(t *testing.T) {
	theme := DraculaTheme()
	if theme.Name != "dracula" {
		t.Errorf("Name = %q, want dracula", theme.Name)
	}
}

func TestSolarizedTheme(t *testing.T) {
	theme := SolarizedTheme()
	if theme.Name != "solarized" {
		t.Errorf("Name = %q, want solarized", theme.Name)
	}
}

func TestThemeAccentColors(t *testing.T) {
	for _, name := range ThemeNames() {
		theme := GetTheme(name)
		if theme.AccentColor == "" {
			t.Errorf("theme %q: AccentColor is empty", name)
		}
		if theme.AccentFg == "" {
			t.Errorf("theme %q: AccentFg is empty", name)
		}
		if theme.SubtleFg == "" {
			t.Errorf("theme %q: SubtleFg is empty", name)
		}
		if theme.ButtonYesBg == "" {
			t.Errorf("theme %q: ButtonYesBg is empty", name)
		}
		if theme.ButtonNoBg == "" {
			t.Errorf("theme %q: ButtonNoBg is empty", name)
		}
		if theme.ButtonFg == "" {
			t.Errorf("theme %q: ButtonFg is empty", name)
		}
	}
}

func TestThemeAccentColorsParsed(t *testing.T) {
	for _, name := range ThemeNames() {
		theme := GetTheme(name)
		theme.ParseColors()
		c := theme.C()
		if c.AccentColor == nil {
			t.Errorf("theme %q: parsed AccentColor is nil", name)
		}
		if c.AccentFg == nil {
			t.Errorf("theme %q: parsed AccentFg is nil", name)
		}
		if c.SubtleFg == nil {
			t.Errorf("theme %q: parsed SubtleFg is nil", name)
		}
		if c.ButtonYesBg == nil {
			t.Errorf("theme %q: parsed ButtonYesBg is nil", name)
		}
		if c.ButtonNoBg == nil {
			t.Errorf("theme %q: parsed ButtonNoBg is nil", name)
		}
		if c.ButtonFg == nil {
			t.Errorf("theme %q: parsed ButtonFg is nil", name)
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

func TestTitleBarRows(t *testing.T) {
	// Theme with no TitleBarHeight set (zero value) should return 1.
	theme := Theme{}
	if got := theme.TitleBarRows(); got != 1 {
		t.Errorf("TitleBarRows() with zero height = %d, want 1", got)
	}

	// Theme with TitleBarHeight = 3 should return 3.
	theme.TitleBarHeight = 3
	if got := theme.TitleBarRows(); got != 3 {
		t.Errorf("TitleBarRows() with height 3 = %d, want 3", got)
	}

	// Theme with TitleBarHeight = 1 should return 1.
	theme.TitleBarHeight = 1
	if got := theme.TitleBarRows(); got != 1 {
		t.Errorf("TitleBarRows() with height 1 = %d, want 1", got)
	}

	// Verify real themes return a valid value.
	for _, name := range ThemeNames() {
		th := GetTheme(name)
		rows := th.TitleBarRows()
		if rows < 1 {
			t.Errorf("theme %q: TitleBarRows() = %d, want >= 1", name, rows)
		}
	}
}

func TestCAutoParseColors(t *testing.T) {
	// C() should auto-call ParseColors if not yet parsed.
	theme := ModernTheme()
	// Do not call ParseColors manually.
	c := theme.C()
	if c == nil {
		t.Fatal("C() returned nil")
	}
	if c.ActiveBorderFg == nil {
		t.Error("C() auto-parse: ActiveBorderFg is nil")
	}
	if c.DesktopBg == nil {
		t.Error("C() auto-parse: DesktopBg is nil")
	}
	if c.DefaultFg == nil {
		t.Error("C() auto-parse: DefaultFg is nil")
	}
}

func TestCReturnsConsistentPointer(t *testing.T) {
	theme := NordTheme()
	theme.ParseColors()
	c1 := theme.C()
	c2 := theme.C()
	if c1 != c2 {
		t.Error("C() should return the same pointer on repeated calls")
	}
}

func TestHexToColor(t *testing.T) {
	tests := []struct {
		hex  string
		want color.Color
	}{
		{"#000000", color.RGBA{R: 0, G: 0, B: 0, A: 255}},
		{"#FFFFFF", color.RGBA{R: 255, G: 255, B: 255, A: 255}},
		{"#ffffff", color.RGBA{R: 255, G: 255, B: 255, A: 255}},
		{"#FF0000", color.RGBA{R: 255, G: 0, B: 0, A: 255}},
		{"#00FF00", color.RGBA{R: 0, G: 255, B: 0, A: 255}},
		{"#0000FF", color.RGBA{R: 0, G: 0, B: 255, A: 255}},
		{"#1a2b3c", color.RGBA{R: 0x1a, G: 0x2b, B: 0x3c, A: 255}},
	}
	for _, tt := range tests {
		got := hexToColor(tt.hex)
		if got != tt.want {
			t.Errorf("hexToColor(%q) = %v, want %v", tt.hex, got, tt.want)
		}
	}
}

func TestHexToColorInvalid(t *testing.T) {
	invalid := []string{
		"",
		"#",
		"#FFF",
		"#GGGGGG",
		"000000",
		"#12345",
		"#1234567",
	}
	for _, hex := range invalid {
		got := hexToColor(hex)
		// Short or malformed strings should return nil (length/prefix check).
		// Strings with valid length but invalid chars return a color with 0 nibbles.
		if len(hex) != 7 || hex[0] != '#' {
			if got != nil {
				t.Errorf("hexToColor(%q) = %v, want nil", hex, got)
			}
		}
	}
}

func TestHexNibble(t *testing.T) {
	tests := []struct {
		b    byte
		want uint8
	}{
		{'0', 0}, {'1', 1}, {'9', 9},
		{'a', 10}, {'f', 15},
		{'A', 10}, {'F', 15},
	}
	for _, tt := range tests {
		got := hexNibble(tt.b)
		if got != tt.want {
			t.Errorf("hexNibble(%c) = %d, want %d", tt.b, got, tt.want)
		}
	}
}

func TestHexNibbleInvalid(t *testing.T) {
	invalid := []byte{'g', 'G', 'z', '!', ' ', '@'}
	for _, b := range invalid {
		got := hexNibble(b)
		if got != 0 {
			t.Errorf("hexNibble(%c) = %d, want 0", b, got)
		}
	}
}

func TestHexByte(t *testing.T) {
	if got := hexByte('F', 'F'); got != 255 {
		t.Errorf("hexByte('F','F') = %d, want 255", got)
	}
	if got := hexByte('0', '0'); got != 0 {
		t.Errorf("hexByte('0','0') = %d, want 0", got)
	}
	if got := hexByte('1', 'A'); got != 0x1A {
		t.Errorf("hexByte('1','A') = %d, want %d", got, 0x1A)
	}
}

func TestParseColorsDefaultFg(t *testing.T) {
	// Theme with no DefaultFg set should use #C0C0C0.
	theme := Theme{
		ActiveBorderFg: "#FFFFFF",
		ActiveBorderBg: "#000000",
	}
	theme.ParseColors()
	c := theme.C()
	if c.DefaultFg == nil {
		t.Fatal("DefaultFg should not be nil when DefaultFg hex is empty")
	}
	want := color.RGBA{R: 0xC0, G: 0xC0, B: 0xC0, A: 255}
	if c.DefaultFg != want {
		t.Errorf("DefaultFg = %v, want %v", c.DefaultFg, want)
	}
}

func TestParseColorsCustomDefaultFg(t *testing.T) {
	theme := Theme{
		DefaultFg: "#FF0000",
	}
	theme.ParseColors()
	c := theme.C()
	want := color.RGBA{R: 255, G: 0, B: 0, A: 255}
	if c.DefaultFg != want {
		t.Errorf("DefaultFg = %v, want %v", c.DefaultFg, want)
	}
}

func TestParseColorsPerButtonFg(t *testing.T) {
	theme := Theme{
		CloseButtonFg: "#FF0000",
		MinButtonFg:   "#FFFF00",
		MaxButtonFg:   "#00FF00",
	}
	theme.ParseColors()
	c := theme.C()

	if c.CloseButtonFg == nil {
		t.Error("CloseButtonFg should not be nil")
	}
	if c.MinButtonFg == nil {
		t.Error("MinButtonFg should not be nil")
	}
	if c.MaxButtonFg == nil {
		t.Error("MaxButtonFg should not be nil")
	}
}

func TestSequoiaTheme(t *testing.T) {
	theme := SequoiaTheme()
	if theme.Name != "sequoia" {
		t.Errorf("Name = %q, want sequoia", theme.Name)
	}
}

func TestSleekTheme(t *testing.T) {
	theme := SleekTheme()
	if theme.Name != "sleek" {
		t.Errorf("Name = %q, want sleek", theme.Name)
	}
}

func TestIsLightThemes(t *testing.T) {
	lightThemes := []string{"redmond", "platinum", "aqua"}
	for _, name := range lightThemes {
		theme := GetTheme(name)
		if !theme.IsLight() {
			t.Errorf("theme %q should be light, but IsLight() returned false", name)
		}
	}
}

func TestIsDarkThemes(t *testing.T) {
	darkThemes := []string{"tokyonight", "dracula", "modern", "catppuccin", "nord"}
	for _, name := range darkThemes {
		theme := GetTheme(name)
		if theme.IsLight() {
			t.Errorf("theme %q should be dark, but IsLight() returned true", name)
		}
	}
}

func TestIsLightEmptyBg(t *testing.T) {
	// Theme with no MenuBarBg and no ContentBg — should return false.
	theme := Theme{}
	if theme.IsLight() {
		t.Error("theme with empty backgrounds should not be considered light")
	}
}

func TestIsLightFallsBackToContentBg(t *testing.T) {
	// Theme with no MenuBarBg but a light ContentBg — should use ContentBg as fallback.
	theme := Theme{
		ContentBg: "#FFFFFF",
	}
	if !theme.IsLight() {
		t.Error("theme with light ContentBg and no MenuBarBg should be considered light")
	}
}

func TestCMissingKey(t *testing.T) {
	// C() returns a parsedColors struct. Accessing a color from a field that was
	// empty in the theme should return nil (since hexToColor("") returns nil).
	theme := Theme{}
	theme.ParseColors()
	c := theme.C()
	if c.ActiveBorderFg != nil {
		t.Error("C().ActiveBorderFg should be nil for an empty theme")
	}
	if c.DesktopBg != nil {
		t.Error("C().DesktopBg should be nil for an empty theme")
	}
	if c.AccentColor != nil {
		t.Error("C().AccentColor should be nil for an empty theme")
	}
}

func TestIsLightBackgroundBoundary(t *testing.T) {
	// Test with exact boundary colors. Luminance threshold is > 128.
	// #808080 -> R=128, G=128, B=128 -> lum = (299*128 + 587*128 + 114*128)/1000 = 128 -> NOT light.
	if isLightBackground("#808080") {
		t.Error("#808080 should not be considered light (lum = 128)")
	}
	// #818181 -> R=129, G=129, B=129 -> lum = (299*129 + 587*129 + 114*129)/1000 = 129 -> light.
	if !isLightBackground("#818181") {
		t.Error("#818181 should be considered light (lum = 129)")
	}
}

func TestParseColorsANSIPaletteDarkTheme(t *testing.T) {
	// Dark theme with no custom ANSI palette should use darkANSIPalette defaults.
	theme := ModernTheme()
	theme.ParseColors()
	c := theme.C()

	// All 16 ANSI colors should be non-nil.
	for i := 0; i < 16; i++ {
		if c.ANSIPalette[i] == nil {
			t.Errorf("dark theme ANSIPalette[%d] is nil", i)
		}
	}
}

func TestParseColorsANSIPaletteLightTheme(t *testing.T) {
	// Light theme should use lightANSIPalette defaults for unfilled slots.
	theme := RedmondTheme()
	theme.ParseColors()
	c := theme.C()

	for i := 0; i < 16; i++ {
		if c.ANSIPalette[i] == nil {
			t.Errorf("light theme ANSIPalette[%d] is nil", i)
		}
	}
}

func TestParseColorsCustomANSIPalette(t *testing.T) {
	// Theme with one custom ANSI color should use that instead of the default.
	theme := ModernTheme()
	theme.ANSIPalette[0] = "#FF0000" // custom red for color 0
	theme.ParseColors()
	c := theme.C()

	want := color.RGBA{R: 255, G: 0, B: 0, A: 255}
	if c.ANSIPalette[0] != want {
		t.Errorf("custom ANSIPalette[0] = %v, want %v", c.ANSIPalette[0], want)
	}
	// Color 1 should still be the default.
	if c.ANSIPalette[1] == nil {
		t.Error("ANSIPalette[1] should be non-nil (from defaults)")
	}
}

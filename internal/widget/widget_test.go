package widget

import (
	"regexp"
	"strings"
	"testing"
	"unicode/utf8"
)

type testWidget struct {
	name   string
	render string
}

func (w testWidget) Name() string       { return w.name }
func (w testWidget) Render() string     { return w.render }
func (w testWidget) ColorLevel() string { return "" }

func TestBarRenderAndZones(t *testing.T) {
	b := &Bar{Widgets: []Widget{
		testWidget{name: "a", render: "AA"},
		testWidget{name: "b", render: ""},
		testWidget{name: "c", render: "CCC"},
	}}

	out := b.Render()
	if out == "" {
		t.Fatalf("expected render output")
	}

	zones := b.RenderZones(30)
	if len(zones) != 2 {
		t.Fatalf("zones=%d want 2", len(zones))
	}
	if zones[0].Type != "a" || zones[1].Type != "c" {
		t.Fatalf("zone types=%v", zones)
	}
	if zones[0].End <= zones[0].Start || zones[1].End <= zones[1].Start {
		t.Fatalf("invalid zone bounds")
	}
}

func TestBarRenderEmpty(t *testing.T) {
	b := &Bar{Widgets: []Widget{
		testWidget{name: "a", render: ""},
		testWidget{name: "b", render: ""},
	}}
	if got := b.Render(); got != "" {
		t.Fatalf("expected empty render for all-empty widgets, got %q", got)
	}
	zones := b.RenderZones(80)
	if len(zones) != 0 {
		t.Fatalf("expected no zones, got %d", len(zones))
	}
}

func TestNewDefaultBar(t *testing.T) {
	b := NewDefaultBar("testuser")
	if b == nil {
		t.Fatalf("expected non-nil bar")
	}
	if len(b.Widgets) != 8 {
		t.Fatalf("expected 8 widgets, got %d", len(b.Widgets))
	}

	// Verify widget types and order.
	names := make([]string, len(b.Widgets))
	for i, w := range b.Widgets {
		names[i] = w.Name()
	}
	expected := []string{"cpu", "mem", "battery", "notification", "workspace", "hostname", "user", "clock"}
	for i, want := range expected {
		if names[i] != want {
			t.Fatalf("widget[%d] name=%q, want %q", i, names[i], want)
		}
	}

	// Verify the user widget received the username.
	u, ok := b.Widgets[6].(*UserWidget)
	if !ok {
		t.Fatalf("widget[6] is not *UserWidget")
	}
	if u.Username != "testuser" {
		t.Fatalf("username=%q, want %q", u.Username, "testuser")
	}
}

func TestNewBarPartialEnabled(t *testing.T) {
	reg := DefaultRegistry()
	enabled := []string{"clock", "cpu", "user"}
	b := NewBar(reg, enabled, "alice", "alice@host", nil)

	if len(b.Widgets) != 3 {
		t.Fatalf("expected 3 widgets, got %d", len(b.Widgets))
	}
	// Verify order matches enabled list
	names := []string{b.Widgets[0].Name(), b.Widgets[1].Name(), b.Widgets[2].Name()}
	if names[0] != "clock" || names[1] != "cpu" || names[2] != "user" {
		t.Fatalf("wrong order: %v", names)
	}
	// Verify user widget got username
	u := b.Widgets[2].(*UserWidget)
	if u.Username != "alice" {
		t.Fatalf("username=%q, want %q", u.Username, "alice")
	}
}

func TestNewBarCustomWidget(t *testing.T) {
	reg := DefaultRegistry()
	reg.Register(WidgetMeta{Name: "myshell", Label: "My Shell", Builtin: false})

	cw := &ShellWidget{WidgetName: "myshell", Icon: ">"}
	cw.lastOutput = "test"

	enabled := []string{"clock", "myshell"}
	b := NewBar(reg, enabled, "bob", "bob", map[string]*ShellWidget{"myshell": cw})

	if len(b.Widgets) != 2 {
		t.Fatalf("expected 2 widgets, got %d", len(b.Widgets))
	}
	if b.Widgets[1].Name() != "myshell" {
		t.Fatalf("expected myshell, got %q", b.Widgets[1].Name())
	}
	if b.Widgets[1].Render() != "> test" {
		t.Fatalf("custom widget render=%q", b.Widgets[1].Render())
	}
}

func TestNewBarUnknownSkipped(t *testing.T) {
	reg := DefaultRegistry()
	enabled := []string{"cpu", "nonexistent", "clock"}
	b := NewBar(reg, enabled, "u", "u", nil)
	if len(b.Widgets) != 2 {
		t.Fatalf("expected 2, got %d", len(b.Widgets))
	}
}

func TestNewBarEmptyEnabled(t *testing.T) {
	reg := DefaultRegistry()
	b := NewBar(reg, nil, "u", "u", nil)
	if len(b.Widgets) != 0 {
		t.Fatalf("expected 0 widgets for nil enabled, got %d", len(b.Widgets))
	}
}

// --- Name() method tests ---

func TestBatteryWidgetName(t *testing.T) {
	w := &BatteryWidget{}
	if got := w.Name(); got != "battery" {
		t.Fatalf("Name()=%q, want %q", got, "battery")
	}
}

func TestClockWidgetName(t *testing.T) {
	w := &ClockWidget{}
	if got := w.Name(); got != "clock" {
		t.Fatalf("Name()=%q, want %q", got, "clock")
	}
}

func TestCPUWidgetName(t *testing.T) {
	w := &CPUWidget{}
	if got := w.Name(); got != "cpu" {
		t.Fatalf("Name()=%q, want %q", got, "cpu")
	}
}

func TestMemoryWidgetName(t *testing.T) {
	w := &MemoryWidget{}
	if got := w.Name(); got != "mem" {
		t.Fatalf("Name()=%q, want %q", got, "mem")
	}
}

func TestNotificationWidgetName(t *testing.T) {
	w := &NotificationWidget{}
	if got := w.Name(); got != "notification" {
		t.Fatalf("Name()=%q, want %q", got, "notification")
	}
}

func TestUserWidgetName(t *testing.T) {
	w := &UserWidget{}
	if got := w.Name(); got != "user" {
		t.Fatalf("Name()=%q, want %q", got, "user")
	}
}

func TestHostnameWidgetName(t *testing.T) {
	w := &HostnameWidget{}
	if got := w.Name(); got != "hostname" {
		t.Fatalf("Name()=%q, want %q", got, "hostname")
	}
}

// --- Battery widget tests ---

func TestBatteryWidget(t *testing.T) {
	w := &BatteryWidget{Pct: 90, Charging: true, Present: true}
	if got := w.Render(); got == "" {
		t.Fatalf("expected render output")
	}
	if w.ColorLevel() != "green" {
		t.Fatalf("expected green")
	}

	w.Pct = 10
	if w.ColorLevel() != "red" {
		t.Fatalf("expected red")
	}

	w.Present = false
	if w.Render() != "" || w.ColorLevel() != "" {
		t.Fatalf("absent battery should render empty")
	}
}

func TestBatteryRenderIconBranches(t *testing.T) {
	tests := []struct {
		pct      float64
		charging bool
		wantPct  string
		wantZap  bool
	}{
		{pct: 95, charging: false, wantPct: "95%", wantZap: false},
		{pct: 80, charging: false, wantPct: "80%", wantZap: false},
		{pct: 65, charging: true, wantPct: "65%", wantZap: true},
		{pct: 45, charging: false, wantPct: "45%", wantZap: false},
		{pct: 25, charging: true, wantPct: "25%", wantZap: true},
		{pct: 10, charging: false, wantPct: "10%", wantZap: false},
	}
	for _, tt := range tests {
		w := &BatteryWidget{Pct: tt.pct, Charging: tt.charging, Present: true}
		got := w.Render()
		if got == "" {
			t.Fatalf("pct=%.0f: empty render", tt.pct)
		}
		if !strings.Contains(got, tt.wantPct) {
			t.Fatalf("pct=%.0f: %q missing %q", tt.pct, got, tt.wantPct)
		}
		hasZap := strings.Contains(got, "\uf1e6")
		if hasZap != tt.wantZap {
			t.Fatalf("pct=%.0f charging=%v: zap=%v want %v", tt.pct, tt.charging, hasZap, tt.wantZap)
		}
	}
}

func TestBatteryColorLevelYellow(t *testing.T) {
	w := &BatteryWidget{Pct: 20, Present: true}
	if got := w.ColorLevel(); got != "yellow" {
		t.Fatalf("ColorLevel()=%q, want %q", got, "yellow")
	}
}

// --- Clock widget tests ---

func TestClockWidget(t *testing.T) {
	w := &ClockWidget{}
	out := w.Render()
	if !regexp.MustCompile(`^\d{2}:\d{2} [AP]M$`).MatchString(out) {
		t.Fatalf("unexpected time format: %q", out)
	}
}

func TestClockColorLevel(t *testing.T) {
	w := &ClockWidget{}
	if got := w.ColorLevel(); got != "" {
		t.Fatalf("ColorLevel()=%q, want empty", got)
	}
}

// --- CPU widget tests ---

func TestCPUWidget(t *testing.T) {
	w := &CPUWidget{}
	w.Update(10)
	w.Update(20)
	if w.Pct != 20 {
		t.Fatalf("pct=%v", w.Pct)
	}
	if len(w.History) != 2 {
		t.Fatalf("history=%d", len(w.History))
	}
	if w.ColorLevel() != "green" {
		t.Fatalf("expected green")
	}
}

func TestCPURender(t *testing.T) {
	w := &CPUWidget{Pct: 25, History: []float64{10, 20, 30}}
	got := w.Render()
	if got == "" {
		t.Fatalf("expected non-empty render")
	}
	if !strings.Contains(got, "25%") {
		t.Fatalf("render %q missing percentage", got)
	}
	if utf8.RuneCountInString(got) < cpuChartWidth {
		t.Fatalf("render too short: %q", got)
	}
}

func TestCPURenderHighUsage(t *testing.T) {
	w := &CPUWidget{Pct: 75, History: []float64{75}}
	got := w.Render()
	if !strings.Contains(got, "75%") {
		t.Fatalf("render %q missing percentage", got)
	}
}

func TestCPUColorLevelYellow(t *testing.T) {
	w := &CPUWidget{Pct: 60}
	if got := w.ColorLevel(); got != "yellow" {
		t.Fatalf("ColorLevel()=%q, want %q", got, "yellow")
	}
}

func TestCPUColorLevelRed(t *testing.T) {
	w := &CPUWidget{Pct: 90}
	if got := w.ColorLevel(); got != "red" {
		t.Fatalf("ColorLevel()=%q, want %q", got, "red")
	}
}

func TestCPUUpdateHistoryOverflow(t *testing.T) {
	w := &CPUWidget{}
	for i := 0; i < 25; i++ {
		w.Update(float64(i))
	}
	if len(w.History) != 20 {
		t.Fatalf("history len=%d, want 20", len(w.History))
	}
	if w.History[0] != 5 {
		t.Fatalf("history[0]=%v, want 5", w.History[0])
	}
}

func TestFormatCPU(t *testing.T) {
	low := formatCPU(10)
	if !strings.Contains(low, "10%") {
		t.Fatalf("formatCPU(10)=%q missing 10%%", low)
	}

	high := formatCPU(75)
	if !strings.Contains(high, "75%") {
		t.Fatalf("formatCPU(75)=%q missing 75%%", high)
	}

	if low == high {
		t.Fatalf("expected different output for low vs high CPU")
	}
}

func TestFormatCPUChart(t *testing.T) {
	// Empty history: all minimum blocks.
	empty := formatCPUChart(nil)
	if utf8.RuneCountInString(empty) != cpuChartWidth {
		t.Fatalf("empty chart width=%d, want %d", utf8.RuneCountInString(empty), cpuChartWidth)
	}

	// Full history at 100%.
	full := make([]float64, 20)
	for i := range full {
		full[i] = 100
	}
	chart := formatCPUChart(full)
	if utf8.RuneCountInString(chart) != cpuChartWidth {
		t.Fatalf("full chart width=%d, want %d", utf8.RuneCountInString(chart), cpuChartWidth)
	}
	for _, r := range chart {
		if r != '\u2588' {
			t.Fatalf("expected max block char, got %c", r)
		}
	}

	// Partial history shorter than chart width.
	partial := formatCPUChart([]float64{0, 50, 100})
	if utf8.RuneCountInString(partial) != cpuChartWidth {
		t.Fatalf("partial chart width=%d, want %d", utf8.RuneCountInString(partial), cpuChartWidth)
	}

	// History longer than chart width.
	long := make([]float64, 30)
	for i := range long {
		long[i] = float64(i * 4)
	}
	longChart := formatCPUChart(long)
	if utf8.RuneCountInString(longChart) != cpuChartWidth {
		t.Fatalf("long chart width=%d, want %d", utf8.RuneCountInString(longChart), cpuChartWidth)
	}
}

// --- Memory widget tests ---

func TestMemoryWidget(t *testing.T) {
	w := &MemoryWidget{UsedGB: 8, TotalGB: 16}
	if w.ColorLevel() != "green" {
		t.Fatalf("expected green")
	}
	w.UsedGB = 15
	if w.ColorLevel() != "red" {
		t.Fatalf("expected red")
	}
}

func TestMemoryRender(t *testing.T) {
	w := &MemoryWidget{UsedGB: 4.5, TotalGB: 16}
	got := w.Render()
	if got == "" {
		t.Fatalf("expected non-empty render")
	}
	if !strings.Contains(got, "4.5G") {
		t.Fatalf("render %q missing memory value", got)
	}
}

func TestMemoryColorLevelYellow(t *testing.T) {
	w := &MemoryWidget{UsedGB: 10.4, TotalGB: 16}
	if got := w.ColorLevel(); got != "yellow" {
		t.Fatalf("ColorLevel()=%q, want %q", got, "yellow")
	}
}

func TestMemoryColorLevelZeroTotal(t *testing.T) {
	w := &MemoryWidget{UsedGB: 5, TotalGB: 0}
	if got := w.ColorLevel(); got != "green" {
		t.Fatalf("ColorLevel()=%q, want %q for zero total", got, "green")
	}
}

// --- Notification widget tests ---

func TestNotificationWidget(t *testing.T) {
	w := &NotificationWidget{UnreadCount: 0}
	if w.Render() == "" {
		t.Fatalf("expected icon output")
	}
	if w.ColorLevel() != "" {
		t.Fatalf("expected default color")
	}
	w.UnreadCount = 2
	if w.ColorLevel() != "yellow" {
		t.Fatalf("expected yellow")
	}
	w.UnreadCount = 4
	if w.ColorLevel() != "red" {
		t.Fatalf("expected red")
	}
}

func TestNotificationRenderWithCount(t *testing.T) {
	w := &NotificationWidget{UnreadCount: 3}
	got := w.Render()
	if got == "" {
		t.Fatalf("expected non-empty render")
	}
	if !strings.Contains(got, "3") {
		t.Fatalf("render %q missing count", got)
	}
}

// --- User widget tests ---

func TestUserWidget(t *testing.T) {
	w := &UserWidget{}
	if w.Render() != "" {
		t.Fatalf("empty username should render empty")
	}
	w.Username = "alice"
	got := w.Render()
	if got == "" {
		t.Fatalf("expected render output")
	}
	if !strings.Contains(got, "alice") {
		t.Fatalf("render %q missing username", got)
	}
	// Should contain an OS icon (not the old  user icon)
	if strings.Contains(got, "\uf007") {
		t.Fatalf("should use OS icon, not generic user icon")
	}
}

func TestUserColorLevel(t *testing.T) {
	w := &UserWidget{Username: "bob"}
	if got := w.ColorLevel(); got != "" {
		t.Fatalf("ColorLevel()=%q, want empty", got)
	}
}

// --- Hostname widget tests ---

func TestHostnameWidget(t *testing.T) {
	w := &HostnameWidget{}
	if w.Render() != "" {
		t.Fatalf("empty hostname should render empty")
	}
	w.Hostname = "myhost"
	got := w.Render()
	if got == "" {
		t.Fatalf("expected render output")
	}
	if !strings.Contains(got, "myhost") {
		t.Fatalf("render %q missing hostname", got)
	}
}

func TestHostnameColorLevel(t *testing.T) {
	w := &HostnameWidget{Hostname: "host"}
	if got := w.ColorLevel(); got != "" {
		t.Fatalf("ColorLevel()=%q, want empty", got)
	}
}

func TestHostnameFromNewBar(t *testing.T) {
	reg := DefaultRegistry()
	enabled := []string{"hostname"}
	b := NewBar(reg, enabled, "alice", "alice@myhost", nil)
	if len(b.Widgets) != 1 {
		t.Fatalf("expected 1 widget, got %d", len(b.Widgets))
	}
	hw, ok := b.Widgets[0].(*HostnameWidget)
	if !ok {
		t.Fatalf("expected *HostnameWidget")
	}
	if hw.Hostname != "myhost" {
		t.Fatalf("hostname=%q, want %q", hw.Hostname, "myhost")
	}
}

// --- Workspace widget tests ---

func TestWorkspaceWidgetName(t *testing.T) {
	w := &WorkspaceWidget{DisplayName: "Default"}
	if got := w.Name(); got != "workspace" {
		t.Fatalf("Name()=%q, want %q", got, "workspace")
	}
}

func TestWorkspaceWidgetRender(t *testing.T) {
	w := &WorkspaceWidget{DisplayName: "myproject"}
	got := w.Render()
	if got == "" {
		t.Fatalf("expected non-empty render")
	}
	if !strings.Contains(got, "myproject") {
		t.Fatalf("render %q missing display name", got)
	}
	// Should contain the folder icon
	if !strings.Contains(got, "\uf07c") {
		t.Fatalf("render %q missing folder icon", got)
	}
}

func TestWorkspaceWidgetRenderDefault(t *testing.T) {
	w := &WorkspaceWidget{DisplayName: "Default"}
	got := w.Render()
	if !strings.Contains(got, "Default") {
		t.Fatalf("render %q missing 'Default'", got)
	}
}

func TestWorkspaceWidgetColorLevel(t *testing.T) {
	w := &WorkspaceWidget{DisplayName: "test"}
	if got := w.ColorLevel(); got != "" {
		t.Fatalf("ColorLevel()=%q, want empty", got)
	}
}

// --- osIcon tests ---

func TestOsIconDarwin(t *testing.T) {
	// On darwin, osIcon should return the Apple icon
	icon := osIcon()
	if icon == "" {
		t.Fatal("osIcon() returned empty string")
	}
	// On darwin, should be the Apple icon
	if icon != "\uf179" {
		t.Logf("osIcon()=%q (platform-specific, expected Apple icon on darwin)", icon)
	}
}

// --- isSSHSession tests ---

func TestIsSSHSessionWithSSHTTY(t *testing.T) {
	t.Setenv("SSH_TTY", "/dev/pts/0")
	t.Setenv("SSH_CLIENT", "")
	t.Setenv("SSH_CONNECTION", "")
	if !isSSHSession() {
		t.Error("expected SSH session with SSH_TTY set")
	}
}

func TestIsSSHSessionWithSSHClient(t *testing.T) {
	t.Setenv("SSH_TTY", "")
	t.Setenv("SSH_CLIENT", "192.168.1.1 12345 22")
	t.Setenv("SSH_CONNECTION", "")
	if !isSSHSession() {
		t.Error("expected SSH session with SSH_CLIENT set")
	}
}

func TestIsSSHSessionWithSSHConnection(t *testing.T) {
	t.Setenv("SSH_TTY", "")
	t.Setenv("SSH_CLIENT", "")
	t.Setenv("SSH_CONNECTION", "192.168.1.1 12345 192.168.1.2 22")
	if !isSSHSession() {
		t.Error("expected SSH session with SSH_CONNECTION set")
	}
}

func TestIsSSHSessionNoSSH(t *testing.T) {
	t.Setenv("SSH_TTY", "")
	t.Setenv("SSH_CLIENT", "")
	t.Setenv("SSH_CONNECTION", "")
	if isSSHSession() {
		t.Error("expected no SSH session when all SSH vars empty")
	}
}

// --- Custom widget SetOutput/MarkRun tests ---

func TestShellWidgetSetOutput(t *testing.T) {
	w := &ShellWidget{WidgetName: "test", Icon: "X"}
	w.SetOutput("hello world")

	if w.lastOutput != "hello world" {
		t.Fatalf("lastOutput=%q, want %q", w.lastOutput, "hello world")
	}
	if w.lastRun.IsZero() {
		t.Fatal("lastRun should be set after SetOutput")
	}
	// Should not need refresh immediately after SetOutput
	if w.NeedsRefresh() {
		t.Fatal("should not need refresh right after SetOutput")
	}
}

func TestShellWidgetMarkRun(t *testing.T) {
	w := &ShellWidget{WidgetName: "test", Interval: 60}
	if !w.NeedsRefresh() {
		t.Fatal("should need refresh before any run")
	}

	w.MarkRun()
	if w.lastRun.IsZero() {
		t.Fatal("lastRun should be set after MarkRun")
	}
	if w.NeedsRefresh() {
		t.Fatal("should not need refresh right after MarkRun")
	}
}

func TestShellWidgetNeedsRefreshDefaultInterval(t *testing.T) {
	w := &ShellWidget{WidgetName: "test"} // Interval 0 = use default (2s)
	if !w.NeedsRefresh() {
		t.Fatal("should need refresh initially")
	}
	w.MarkRun()
	if w.NeedsRefresh() {
		t.Fatal("should not need refresh right after MarkRun")
	}
}

// --- linuxDistroIcon test ---
// On darwin this will hit the error path (no /etc/os-release) and return generic Linux icon.
func TestLinuxDistroIconDefault(t *testing.T) {
	icon := linuxDistroIcon()
	if icon == "" {
		t.Fatal("linuxDistroIcon() returned empty string")
	}
	// On darwin, /etc/os-release doesn't exist, so should return generic Linux
	if icon != "\uf17c" {
		t.Logf("linuxDistroIcon()=%q (may vary on Linux)", icon)
	}
}

// --- DisplayWidth tests ---

func TestDisplayWidth(t *testing.T) {
	if got := DisplayWidth("hello"); got != 5 {
		t.Fatalf("DisplayWidth('hello') = %d, want 5", got)
	}
	if got := DisplayWidth(""); got != 0 {
		t.Fatalf("DisplayWidth('') = %d, want 0", got)
	}
}

// --- Bar.DisplayWidth test ---

func TestBarDisplayWidth(t *testing.T) {
	b := &Bar{Widgets: []Widget{
		testWidget{name: "a", render: "AA"},
	}}
	dw := b.DisplayWidth()
	if dw <= 0 {
		t.Fatalf("Bar.DisplayWidth() = %d, expected positive", dw)
	}
}

// --- ReadBattery functional test ---

func TestReadBatteryFunctional(t *testing.T) {
	// Should not panic on macOS. Result depends on hardware.
	info := ReadBattery()
	// On a Mac laptop, Present should be true. On a desktop Mac, false.
	// Either way, should not crash.
	t.Logf("ReadBattery: present=%v pct=%.0f charging=%v", info.Present, info.Percent, info.Charging)
}

// --- ReadCPUPercent functional test ---

func TestReadCPUPercentFunctional(t *testing.T) {
	// Should not panic. Returns a value >= 0.
	pct := ReadCPUPercent()
	if pct < 0 {
		t.Fatalf("ReadCPUPercent() = %v, should be >= 0", pct)
	}
	t.Logf("ReadCPUPercent: %.1f%%", pct)
}

// --- ReadMemoryInfo functional test ---

func TestReadMemoryInfoFunctional(t *testing.T) {
	// Should not panic. Returns values >= 0.
	used, total := ReadMemoryInfo()
	if total < 0 {
		t.Fatalf("ReadMemoryInfo total = %v, should be >= 0", total)
	}
	if used < 0 {
		t.Fatalf("ReadMemoryInfo used = %v, should be >= 0", used)
	}
	t.Logf("ReadMemoryInfo: used=%.1fGB total=%.1fGB", used, total)
}

func TestHostnameSSHIcon(t *testing.T) {
	w := &HostnameWidget{Hostname: "remote", IsSSH: true}
	got := w.Render()
	if !strings.Contains(got, "\uf023") {
		t.Fatalf("SSH hostname should have lock icon, got %q", got)
	}
	if !strings.Contains(got, "remote") {
		t.Fatalf("render %q missing hostname", got)
	}

	w2 := &HostnameWidget{Hostname: "local", IsSSH: false}
	got2 := w2.Render()
	if strings.Contains(got2, "\uf023") {
		t.Fatalf("non-SSH hostname should NOT have lock icon, got %q", got2)
	}
	if !strings.Contains(got2, "\uf108") {
		t.Fatalf("non-SSH hostname should have monitor icon, got %q", got2)
	}
}

// --- osIconForGOOS tests ---

func TestOsIconForGOOS(t *testing.T) {
	tests := []struct {
		goos string
		want string
	}{
		{"darwin", "\uf179"},
		{"android", "\uf17b"},
		{"windows", "\uf17a"},
		{"freebsd", "\uf30c"},
	}
	for _, tt := range tests {
		got := osIconForGOOS(tt.goos)
		if got != tt.want {
			t.Errorf("osIconForGOOS(%q)=%q, want %q", tt.goos, got, tt.want)
		}
	}
}

func TestOsIconForGOOSLinuxDefault(t *testing.T) {
	// "linux" falls through to linuxDistroIcon, which on darwin returns generic Tux
	got := osIconForGOOS("linux")
	if got == "" {
		t.Fatal("osIconForGOOS(linux) returned empty string")
	}
}

// --- matchDistroIcon tests ---

func TestMatchDistroIcon(t *testing.T) {
	tests := []struct {
		data string
		want string
	}{
		{`NAME="Ubuntu"`, "\uf31b"},
		{`ID=debian`, "\uf306"},
		{`NAME="Fedora Linux"`, "\uf30a"},
		{`ID=arch`, "\uf303"},
		{`NAME="CentOS Stream"`, "\uf304"},
		{`ID=manjaro`, "\uf312"},
		{`NAME="openSUSE Tumbleweed"`, "\uf314"},
		{`ID=suse`, "\uf314"},
		{`NAME="Linux Mint"`, "\uf30e"},
		{`ID=gentoo`, "\uf30d"},
		{`ID=alpine`, "\uf300"},
		{`ID=nixos`, "\uf313"},
		{`ID=void`, "\uf32e"},
		{`NAME="Pop!_OS"`, "\uf32a"},
		{`NAME="Raspberry Pi OS"`, "\uf315"},
		{`NAME="Unknown Distro"`, "\uf17c"},
		{``, "\uf17c"},
	}
	for _, tt := range tests {
		got := matchDistroIcon(tt.data)
		if got != tt.want {
			t.Errorf("matchDistroIcon(%q)=%q, want %q", tt.data, got, tt.want)
		}
	}
}

// --- formatCPUChart edge cases ---

func TestFormatCPUChartNegativeValues(t *testing.T) {
	// Negative values should clamp to index 0 (minimum block).
	chart := formatCPUChart([]float64{-10, -50, -100})
	if utf8.RuneCountInString(chart) != cpuChartWidth {
		t.Fatalf("chart width=%d, want %d", utf8.RuneCountInString(chart), cpuChartWidth)
	}
	// All placed runes should be the minimum block '▁'
	runes := []rune(chart)
	for i := cpuChartWidth - 3; i < cpuChartWidth; i++ {
		if runes[i] != '▁' {
			t.Errorf("chart[%d]=%c, want '▁' for negative input", i, runes[i])
		}
	}
}

func TestFormatCPUChartOverflow(t *testing.T) {
	// Values > 100 should clamp to index 7 (maximum block).
	chart := formatCPUChart([]float64{150, 200, 999})
	if utf8.RuneCountInString(chart) != cpuChartWidth {
		t.Fatalf("chart width=%d, want %d", utf8.RuneCountInString(chart), cpuChartWidth)
	}
	runes := []rune(chart)
	for i := cpuChartWidth - 3; i < cpuChartWidth; i++ {
		if runes[i] != '█' {
			t.Errorf("chart[%d]=%c, want '█' for overflow input", i, runes[i])
		}
	}
}

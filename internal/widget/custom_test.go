package widget

import (
	"testing"
	"time"
)

func TestShellWidgetName(t *testing.T) {
	w := &ShellWidget{WidgetName: "mywidget"}
	if got := w.Name(); got != "mywidget" {
		t.Fatalf("Name()=%q, want %q", got, "mywidget")
	}
}

func TestShellWidgetRenderEmpty(t *testing.T) {
	w := &ShellWidget{WidgetName: "test", Icon: "X"}
	if got := w.Render(); got != "" {
		t.Fatalf("expected empty render before refresh, got %q", got)
	}
}

func TestShellWidgetRenderWithOutput(t *testing.T) {
	w := &ShellWidget{WidgetName: "test", Icon: "X"}
	w.lastOutput = "hello"
	if got := w.Render(); got != "X hello" {
		t.Fatalf("Render()=%q, want %q", got, "X hello")
	}
}

func TestShellWidgetRenderNoIcon(t *testing.T) {
	w := &ShellWidget{WidgetName: "test"}
	w.lastOutput = "hello"
	if got := w.Render(); got != "hello" {
		t.Fatalf("Render()=%q, want %q", got, "hello")
	}
}

func TestShellWidgetColorLevel(t *testing.T) {
	w := &ShellWidget{WidgetName: "test"}
	if got := w.ColorLevel(); got != "" {
		t.Fatalf("ColorLevel()=%q, want empty", got)
	}
}

func TestShellWidgetRefresh(t *testing.T) {
	w := &ShellWidget{
		WidgetName: "test",
		Icon:       ">",
		Command:    "echo hello",
		Interval:   1,
	}
	w.Refresh()
	if w.lastOutput != "hello" {
		t.Fatalf("lastOutput=%q, want %q", w.lastOutput, "hello")
	}
	if got := w.Render(); got != "> hello" {
		t.Fatalf("Render()=%q, want %q", got, "> hello")
	}
}

func TestShellWidgetRefreshInterval(t *testing.T) {
	w := &ShellWidget{
		WidgetName: "test",
		Command:    "echo first",
		Interval:   60, // long interval
	}
	w.Refresh()
	if w.lastOutput != "first" {
		t.Fatalf("first refresh: lastOutput=%q", w.lastOutput)
	}

	// Change command but interval hasn't elapsed — should keep old output
	w.Command = "echo second"
	w.Refresh()
	if w.lastOutput != "first" {
		t.Fatalf("expected cached output, got %q", w.lastOutput)
	}

	// Force interval elapsed
	w.lastRun = time.Now().Add(-61 * time.Second)
	w.Refresh()
	if w.lastOutput != "second" {
		t.Fatalf("after interval: lastOutput=%q", w.lastOutput)
	}
}

func TestShellWidgetRefreshTruncation(t *testing.T) {
	w := &ShellWidget{
		WidgetName: "test",
		Command:    "echo 'this is a very long output string that should be truncated'",
	}
	w.Refresh()
	runes := []rune(w.lastOutput)
	if len(runes) > shellWidgetMaxOutput {
		t.Fatalf("output not truncated: %d runes > %d", len(runes), shellWidgetMaxOutput)
	}
}

func TestShellWidgetRefreshError(t *testing.T) {
	w := &ShellWidget{
		WidgetName: "test",
		Command:    "echo good",
	}
	w.Refresh()
	if w.lastOutput != "good" {
		t.Fatalf("expected 'good', got %q", w.lastOutput)
	}

	// Bad command — should keep previous output
	w.Command = "false"
	w.lastRun = time.Time{} // force re-run
	w.Refresh()
	if w.lastOutput != "good" {
		t.Fatalf("expected preserved output on error, got %q", w.lastOutput)
	}
}

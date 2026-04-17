package widget

import "testing"

func TestRegistryRegisterAndAll(t *testing.T) {
	r := NewRegistry()
	r.Register(WidgetMeta{Name: "a", Label: "Widget A", Builtin: true})
	r.Register(WidgetMeta{Name: "b", Label: "Widget B", Builtin: false})

	all := r.All()
	if len(all) != 2 {
		t.Fatalf("expected 2 widgets, got %d", len(all))
	}
	if all[0].Name != "a" || all[1].Name != "b" {
		t.Fatalf("wrong order: %v", all)
	}
}

func TestRegistryDuplicateIgnored(t *testing.T) {
	r := NewRegistry()
	r.Register(WidgetMeta{Name: "a", Label: "First"})
	r.Register(WidgetMeta{Name: "a", Label: "Second"})

	all := r.All()
	if len(all) != 1 {
		t.Fatalf("expected 1, got %d", len(all))
	}
	if all[0].Label != "First" {
		t.Fatalf("expected first registration to win, got %q", all[0].Label)
	}
}

func TestRegistryHas(t *testing.T) {
	r := NewRegistry()
	r.Register(WidgetMeta{Name: "cpu", Label: "CPU"})

	if !r.Has("cpu") {
		t.Fatal("expected Has(cpu) = true")
	}
	if r.Has("nonexistent") {
		t.Fatal("expected Has(nonexistent) = false")
	}
}

func TestRegistryGet(t *testing.T) {
	r := NewRegistry()
	r.Register(WidgetMeta{Name: "mem", Label: "Memory", Builtin: true})

	m, ok := r.Get("mem")
	if !ok {
		t.Fatal("expected Get(mem) to succeed")
	}
	if m.Label != "Memory" || !m.Builtin {
		t.Fatalf("unexpected meta: %+v", m)
	}

	_, ok = r.Get("missing")
	if ok {
		t.Fatal("expected Get(missing) to fail")
	}
}

func TestDefaultRegistry(t *testing.T) {
	r := DefaultRegistry()
	all := r.All()
	if len(all) != 14 {
		t.Fatalf("expected 14 built-in widgets, got %d", len(all))
	}

	// First 8 match the default enabled list.
	expectedNames := DefaultEnabledWidgets()
	for i, want := range expectedNames {
		if all[i].Name != want {
			t.Fatalf("widget[%d] = %q, want %q", i, all[i].Name, want)
		}
		if !all[i].Builtin {
			t.Fatalf("widget %q should be builtin", all[i].Name)
		}
	}
	// The remaining 6 are also built-in.
	for i := len(expectedNames); i < len(all); i++ {
		if !all[i].Builtin {
			t.Fatalf("widget %q should be builtin", all[i].Name)
		}
	}
}

func TestDefaultEnabledWidgets(t *testing.T) {
	enabled := DefaultEnabledWidgets()
	if len(enabled) != 8 {
		t.Fatalf("expected 8, got %d", len(enabled))
	}
	expected := []string{"cpu", "mem", "battery", "notification", "workspace", "hostname", "user", "clock"}
	for i, want := range expected {
		if enabled[i] != want {
			t.Fatalf("enabled[%d] = %q, want %q", i, enabled[i], want)
		}
	}
}

func TestRegistryAllReturnsCopy(t *testing.T) {
	r := NewRegistry()
	r.Register(WidgetMeta{Name: "a", Label: "A"})

	all := r.All()
	all[0].Name = "modified"

	all2 := r.All()
	if all2[0].Name != "a" {
		t.Fatal("All() should return a copy, not a reference")
	}
}

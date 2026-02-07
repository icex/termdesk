package ringbuf

import "testing"

func TestNewCapacity(t *testing.T) {
	r := New[int](5)
	if r.Cap() != 5 {
		t.Errorf("cap = %d, want 5", r.Cap())
	}
	if r.Len() != 0 {
		t.Errorf("len = %d, want 0", r.Len())
	}
}

func TestNewMinCapacity(t *testing.T) {
	r := New[int](0)
	if r.Cap() < 1 {
		t.Errorf("cap = %d, want >= 1", r.Cap())
	}
}

func TestPushAndItems(t *testing.T) {
	r := New[string](3)
	r.Push("a")
	r.Push("b")
	r.Push("c")

	items := r.Items()
	if len(items) != 3 {
		t.Fatalf("len = %d, want 3", len(items))
	}
	if items[0] != "a" || items[1] != "b" || items[2] != "c" {
		t.Errorf("items = %v, want [a b c]", items)
	}
}

func TestPushOverflow(t *testing.T) {
	r := New[int](3)
	r.Push(1)
	r.Push(2)
	r.Push(3)
	r.Push(4) // should drop 1

	items := r.Items()
	if len(items) != 3 {
		t.Fatalf("len = %d, want 3", len(items))
	}
	if items[0] != 2 || items[1] != 3 || items[2] != 4 {
		t.Errorf("items = %v, want [2 3 4]", items)
	}
}

func TestPushMultipleOverflow(t *testing.T) {
	r := New[int](2)
	for i := 1; i <= 5; i++ {
		r.Push(i)
	}

	items := r.Items()
	if len(items) != 2 {
		t.Fatalf("len = %d, want 2", len(items))
	}
	if items[0] != 4 || items[1] != 5 {
		t.Errorf("items = %v, want [4 5]", items)
	}
}

func TestLatest(t *testing.T) {
	r := New[string](5)

	_, ok := r.Latest()
	if ok {
		t.Error("expected no latest on empty buffer")
	}

	r.Push("hello")
	v, ok := r.Latest()
	if !ok || v != "hello" {
		t.Errorf("latest = %q, ok=%v, want hello/true", v, ok)
	}

	r.Push("world")
	v, ok = r.Latest()
	if !ok || v != "world" {
		t.Errorf("latest = %q, ok=%v, want world/true", v, ok)
	}
}

func TestLen(t *testing.T) {
	r := New[int](5)
	if r.Len() != 0 {
		t.Error("expected 0")
	}
	r.Push(1)
	if r.Len() != 1 {
		t.Error("expected 1")
	}
	r.Push(2)
	r.Push(3)
	if r.Len() != 3 {
		t.Error("expected 3")
	}
}

func TestClear(t *testing.T) {
	r := New[int](5)
	r.Push(1)
	r.Push(2)
	r.Clear()
	if r.Len() != 0 {
		t.Errorf("len = %d, want 0 after clear", r.Len())
	}
	_, ok := r.Latest()
	if ok {
		t.Error("expected no latest after clear")
	}
}

func TestItemsReturnsCopy(t *testing.T) {
	r := New[int](5)
	r.Push(1)
	r.Push(2)

	items := r.Items()
	items[0] = 99

	orig := r.Items()
	if orig[0] != 1 {
		t.Error("Items() should return a copy, not a reference")
	}
}

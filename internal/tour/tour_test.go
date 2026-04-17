package tour

import "testing"

func TestNewActive(t *testing.T) {
	tr := New(false)
	if !tr.Active {
		t.Error("expected tour to be active when not completed")
	}
	if tr.CurrentStep() == nil {
		t.Error("expected non-nil current step")
	}
	if tr.CurrentStep().Title != "Welcome to Termdesk!" {
		t.Errorf("unexpected title: %s", tr.CurrentStep().Title)
	}
}

func TestNewCompleted(t *testing.T) {
	tr := New(true)
	if tr.Active {
		t.Error("expected tour to be inactive when completed")
	}
	if tr.CurrentStep() != nil {
		t.Error("expected nil current step when inactive")
	}
}

func TestNext(t *testing.T) {
	tr := New(false)
	for i := 0; i < len(defaultSteps)-1; i++ {
		if !tr.Next() {
			t.Errorf("expected Next to return true at step %d", i)
		}
	}
	// Now on last step
	if !tr.IsLast() {
		t.Error("expected IsLast on final step")
	}
	// Next past the end
	if tr.Next() {
		t.Error("expected Next to return false past end")
	}
	if tr.Active {
		t.Error("expected tour to be inactive after finishing")
	}
}

func TestSkip(t *testing.T) {
	tr := New(false)
	tr.Skip()
	if tr.Active {
		t.Error("expected tour to be inactive after skip")
	}
}

func TestNextOnInactiveTour(t *testing.T) {
	tr := New(true)
	if tr.Next() {
		t.Error("expected Next to return false on inactive tour")
	}
}

func TestStepInfo(t *testing.T) {
	tr := New(false)
	got := tr.StepInfo()
	if got != "1/7" {
		t.Errorf("expected 1/7, got %s", got)
	}
}

func TestStepInfoInactive(t *testing.T) {
	tr := New(true)
	got := tr.StepInfo()
	if got != "" {
		t.Errorf("expected empty string for inactive tour, got %q", got)
	}
}

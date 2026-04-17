package notification

import (
	"testing"
	"time"
)

func TestNewManager(t *testing.T) {
	m := New()
	if m == nil {
		t.Fatal("New() returned nil")
	}
	if m.UnreadCount() != 0 {
		t.Errorf("new manager should have 0 unread, got %d", m.UnreadCount())
	}
	if len(m.HistoryItems()) != 0 {
		t.Errorf("new manager should have empty history")
	}
}

func TestPushAddsToVisibleAndHistory(t *testing.T) {
	m := New()
	m.Push("Title", "Body", Info)

	if len(m.visible) != 1 {
		t.Fatalf("expected 1 visible, got %d", len(m.visible))
	}
	if m.visible[0].Title != "Title" {
		t.Errorf("expected title 'Title', got %q", m.visible[0].Title)
	}
	if m.visible[0].Body != "Body" {
		t.Errorf("expected body 'Body', got %q", m.visible[0].Body)
	}
	if m.visible[0].Severity != Info {
		t.Errorf("expected severity Info, got %d", m.visible[0].Severity)
	}

	items := m.HistoryItems()
	if len(items) != 1 {
		t.Fatalf("expected 1 history item, got %d", len(items))
	}
	if items[0].Title != "Title" {
		t.Errorf("history item title mismatch")
	}
}

func TestVisibleMaxThree(t *testing.T) {
	m := New()
	m.Push("A", "", Info)
	m.Push("B", "", Warning)
	m.Push("C", "", Error)
	m.Push("D", "", Info)

	if len(m.visible) != 3 {
		t.Errorf("expected max 3 visible, got %d", len(m.visible))
	}
	// Oldest should be dropped from visible
	titles := make([]string, len(m.visible))
	for i, n := range m.visible {
		titles[i] = n.Title
	}
	if titles[0] != "B" || titles[1] != "C" || titles[2] != "D" {
		t.Errorf("expected visible [B,C,D], got %v", titles)
	}

	// But all 4 in history
	items := m.HistoryItems()
	if len(items) != 4 {
		t.Errorf("expected 4 history items, got %d", len(items))
	}
}

func TestHistoryItemsNewestFirst(t *testing.T) {
	m := New()
	m.Push("First", "", Info)
	m.Push("Second", "", Info)
	m.Push("Third", "", Info)

	items := m.HistoryItems()
	if len(items) != 3 {
		t.Fatalf("expected 3 items, got %d", len(items))
	}
	if items[0].Title != "Third" {
		t.Errorf("expected newest first, got %q", items[0].Title)
	}
	if items[2].Title != "First" {
		t.Errorf("expected oldest last, got %q", items[2].Title)
	}
}

func TestHistoryCapsAt20(t *testing.T) {
	m := New()
	for i := 0; i < 25; i++ {
		m.Push("N", "", Info)
	}
	items := m.HistoryItems()
	if len(items) != historyCapacity {
		t.Errorf("expected history capped at %d, got %d", historyCapacity, len(items))
	}
}

func TestDismiss(t *testing.T) {
	m := New()
	m.Push("A", "", Info)
	m.Push("B", "", Info)
	id := m.visible[0].ID

	m.Dismiss(id)
	if len(m.visible) != 1 {
		t.Errorf("expected 1 visible after dismiss, got %d", len(m.visible))
	}
	if m.visible[0].Title != "B" {
		t.Errorf("wrong notification remaining")
	}
}

func TestDismissNonExistent(t *testing.T) {
	m := New()
	m.Push("A", "", Info)
	m.Dismiss(999) // should not panic
	if len(m.visible) != 1 {
		t.Errorf("dismiss of non-existent ID should not remove anything")
	}
}

func TestUnreadCount(t *testing.T) {
	m := New()
	m.Push("A", "", Info)
	m.Push("B", "", Warning)
	if m.UnreadCount() != 2 {
		t.Errorf("expected 2 unread, got %d", m.UnreadCount())
	}

	m.MarkAllRead()
	if m.UnreadCount() != 0 {
		t.Errorf("expected 0 unread after MarkAllRead, got %d", m.UnreadCount())
	}

	m.Push("C", "", Error)
	if m.UnreadCount() != 1 {
		t.Errorf("expected 1 unread after new push, got %d", m.UnreadCount())
	}
}

func TestTick(t *testing.T) {
	m := New()
	m.Push("A", "", Info)
	// Manually set creation time to be old
	m.visible[0].CreatedAt = time.Now().Add(-5 * time.Second)

	m.Push("B", "", Info) // this one is fresh

	dismissed := m.Tick()
	if len(dismissed) != 1 {
		t.Fatalf("expected 1 dismissed, got %d", len(dismissed))
	}
	if len(m.visible) != 1 {
		t.Errorf("expected 1 visible after tick, got %d", len(m.visible))
	}
	if m.visible[0].Title != "B" {
		t.Errorf("wrong notification remaining after tick")
	}
}

func TestTickNoDismissForFresh(t *testing.T) {
	m := New()
	m.Push("Fresh", "", Info)

	dismissed := m.Tick()
	if len(dismissed) != 0 {
		t.Errorf("fresh notification should not be dismissed")
	}
	if len(m.visible) != 1 {
		t.Errorf("visible count should still be 1")
	}
}

func TestIDsAreUnique(t *testing.T) {
	m := New()
	m.Push("A", "", Info)
	m.Push("B", "", Info)
	if m.visible[0].ID == m.visible[1].ID {
		t.Errorf("IDs should be unique")
	}
}

func TestCenterVisibility(t *testing.T) {
	m := New()
	if m.CenterVisible() {
		t.Error("center should be hidden initially")
	}
	m.ShowCenter()
	if !m.CenterVisible() {
		t.Error("center should be visible after ShowCenter")
	}
	m.HideCenter()
	if m.CenterVisible() {
		t.Error("center should be hidden after HideCenter")
	}
}

func TestToggleCenter(t *testing.T) {
	m := New()
	m.ToggleCenter()
	if !m.CenterVisible() {
		t.Error("toggle should show center")
	}
	m.ToggleCenter()
	if m.CenterVisible() {
		t.Error("toggle should hide center")
	}
}

func TestCenterNavigation(t *testing.T) {
	m := New()
	m.Push("A", "", Info)
	m.Push("B", "", Info)
	m.Push("C", "", Info)
	m.ShowCenter()

	if m.CenterIndex() != 0 {
		t.Errorf("center should start at index 0")
	}

	m.MoveCenterSelection(1)
	if m.CenterIndex() != 1 {
		t.Errorf("expected index 1, got %d", m.CenterIndex())
	}

	m.MoveCenterSelection(1)
	m.MoveCenterSelection(1) // wraps
	if m.CenterIndex() != 0 {
		t.Errorf("expected wrap to 0, got %d", m.CenterIndex())
	}

	m.MoveCenterSelection(-1) // wraps backward
	if m.CenterIndex() != 2 {
		t.Errorf("expected wrap to 2, got %d", m.CenterIndex())
	}
}

func TestDeleteFromHistory(t *testing.T) {
	m := New()
	m.Push("A", "", Info)
	m.Push("B", "", Info)
	m.Push("C", "", Info)

	items := m.HistoryItems()
	idToDelete := items[1].ID // "B" (middle item, newest-first order)

	m.DeleteFromHistory(idToDelete)
	items = m.HistoryItems()
	if len(items) != 2 {
		t.Fatalf("expected 2 items after delete, got %d", len(items))
	}
	for _, item := range items {
		if item.Title == "B" {
			t.Error("deleted item should not appear in history")
		}
	}
}

func TestClearHistory(t *testing.T) {
	m := New()
	m.Push("A", "", Info)
	m.Push("B", "", Info)

	m.ClearHistory()
	if len(m.HistoryItems()) != 0 {
		t.Error("history should be empty after clear")
	}
	// Visible toasts should also be cleared
	if len(m.visible) != 0 {
		t.Error("visible should be empty after clear")
	}
}

func TestMarkAllReadSetsReadFlag(t *testing.T) {
	m := New()
	m.Push("A", "", Info)
	m.Push("B", "", Info)

	m.MarkAllRead()
	items := m.HistoryItems()
	for _, item := range items {
		if !item.Read {
			t.Errorf("item %q should be marked as read", item.Title)
		}
	}
}

// --- Cleanup tests ---

func TestCleanupTrimsHistory(t *testing.T) {
	m := New()
	for i := 0; i < 10; i++ {
		m.Push("N", "", Info)
	}
	if len(m.history) != 10 {
		t.Fatalf("expected 10 history items, got %d", len(m.history))
	}

	m.Cleanup(5)
	if len(m.history) != 5 {
		t.Errorf("expected 5 history items after cleanup, got %d", len(m.history))
	}
	// Should keep the most recent 5 (IDs 5..9)
	for i, n := range m.history {
		expectedID := 5 + i
		if n.ID != expectedID {
			t.Errorf("history[%d] expected ID %d, got %d", i, expectedID, n.ID)
		}
	}
}

func TestCleanupNoOpWhenUnderLimit(t *testing.T) {
	m := New()
	m.Push("A", "", Info)
	m.Push("B", "", Info)

	m.Cleanup(10) // maxKeep > len(history)
	if len(m.history) != 2 {
		t.Errorf("cleanup should not remove items when under limit, got %d", len(m.history))
	}
}

func TestCleanupExactLimit(t *testing.T) {
	m := New()
	for i := 0; i < 5; i++ {
		m.Push("N", "", Info)
	}

	m.Cleanup(5) // exactly at limit
	if len(m.history) != 5 {
		t.Errorf("cleanup at exact limit should keep all items, got %d", len(m.history))
	}
}

// --- VisibleToasts tests ---

func TestVisibleToastsReturnsCorrectOrder(t *testing.T) {
	m := New()
	m.Push("A", "body-a", Info)
	m.Push("B", "body-b", Warning)
	m.Push("C", "body-c", Error)

	toasts := m.VisibleToasts()
	if len(toasts) != 3 {
		t.Fatalf("expected 3 toasts, got %d", len(toasts))
	}
	if toasts[0].Title != "A" || toasts[1].Title != "B" || toasts[2].Title != "C" {
		t.Errorf("expected toasts [A,B,C], got [%s,%s,%s]", toasts[0].Title, toasts[1].Title, toasts[2].Title)
	}
}

func TestVisibleToastsEmpty(t *testing.T) {
	m := New()
	toasts := m.VisibleToasts()
	if len(toasts) != 0 {
		t.Errorf("expected 0 toasts for new manager, got %d", len(toasts))
	}
}

func TestVisibleToastsDefensiveCopy(t *testing.T) {
	m := New()
	m.Push("A", "", Info)

	toasts := m.VisibleToasts()
	toasts[0].Title = "Modified"

	// Original should be unchanged
	if m.visible[0].Title != "A" {
		t.Errorf("VisibleToasts should return a defensive copy; original was modified")
	}
}

// --- History (raw slice) tests ---

func TestHistoryReturnsOldestFirst(t *testing.T) {
	m := New()
	m.Push("First", "", Info)
	m.Push("Second", "", Warning)
	m.Push("Third", "", Error)

	h := m.History()
	if len(h) != 3 {
		t.Fatalf("expected 3 history items, got %d", len(h))
	}
	if h[0].Title != "First" {
		t.Errorf("expected oldest first in History(), got %q", h[0].Title)
	}
	if h[2].Title != "Third" {
		t.Errorf("expected newest last in History(), got %q", h[2].Title)
	}
}

func TestHistoryEmpty(t *testing.T) {
	m := New()
	h := m.History()
	if h != nil && len(h) != 0 {
		t.Errorf("expected nil or empty history for new manager, got len %d", len(h))
	}
}

// --- RestoreNotification tests ---

func TestRestoreNotificationAllFields(t *testing.T) {
	m := New()
	created := time.Date(2025, 1, 15, 10, 30, 0, 0, time.UTC)
	m.RestoreNotification("Restored Title", "Restored Body", "warning", created, true)

	h := m.History()
	if len(h) != 1 {
		t.Fatalf("expected 1 history item, got %d", len(h))
	}
	n := h[0]
	if n.Title != "Restored Title" {
		t.Errorf("expected title 'Restored Title', got %q", n.Title)
	}
	if n.Body != "Restored Body" {
		t.Errorf("expected body 'Restored Body', got %q", n.Body)
	}
	if n.Severity != Warning {
		t.Errorf("expected severity Warning, got %d", n.Severity)
	}
	if !n.CreatedAt.Equal(created) {
		t.Errorf("expected createdAt %v, got %v", created, n.CreatedAt)
	}
	if !n.Read {
		t.Error("expected Read to be true")
	}
}

func TestRestoreNotificationSeverityParsing(t *testing.T) {
	tests := []struct {
		input    string
		expected Severity
	}{
		{"info", Info},
		{"warning", Warning},
		{"error", Error},
		{"unknown", Info}, // unknown defaults to Info
		{"", Info},        // empty defaults to Info
		{"WARNING", Info}, // case-sensitive, uppercase not recognized -> defaults to Info
	}
	for _, tc := range tests {
		m := New()
		m.RestoreNotification("T", "B", tc.input, time.Now(), false)
		h := m.History()
		if len(h) != 1 {
			t.Fatalf("severity %q: expected 1 item, got %d", tc.input, len(h))
		}
		if h[0].Severity != tc.expected {
			t.Errorf("severity %q: expected %d, got %d", tc.input, tc.expected, h[0].Severity)
		}
	}
}

func TestRestoreNotificationCapsAt20(t *testing.T) {
	m := New()
	for i := 0; i < 25; i++ {
		m.RestoreNotification("R", "", "info", time.Now(), true)
	}
	h := m.History()
	if len(h) != historyCapacity {
		t.Errorf("expected history capped at %d, got %d", historyCapacity, len(h))
	}
}

func TestRestoreNotificationIncrementsID(t *testing.T) {
	m := New()
	m.RestoreNotification("A", "", "info", time.Now(), false)
	m.RestoreNotification("B", "", "info", time.Now(), false)

	h := m.History()
	if h[0].ID == h[1].ID {
		t.Error("restored notifications should have unique IDs")
	}
	if h[1].ID != h[0].ID+1 {
		t.Errorf("expected sequential IDs, got %d and %d", h[0].ID, h[1].ID)
	}
}

func TestRestoreNotificationNotInToasts(t *testing.T) {
	m := New()
	m.RestoreNotification("Restored", "", "info", time.Now(), true)

	toasts := m.VisibleToasts()
	if len(toasts) != 0 {
		t.Error("restored notifications should not appear in visible toasts")
	}
}

func TestRestoreNotificationUnreadFlag(t *testing.T) {
	m := New()
	m.RestoreNotification("Unread", "", "error", time.Now(), false)

	h := m.History()
	if h[0].Read {
		t.Error("expected Read to be false")
	}
	if m.UnreadCount() != 1 {
		t.Errorf("expected 1 unread, got %d", m.UnreadCount())
	}
}

// --- Severity.String tests ---

func TestSeverityString(t *testing.T) {
	tests := []struct {
		sev      Severity
		expected string
	}{
		{Info, "info"},
		{Warning, "warning"},
		{Error, "error"},
		{Severity(99), "info"}, // unknown defaults to "info"
	}
	for _, tc := range tests {
		got := tc.sev.String()
		if got != tc.expected {
			t.Errorf("Severity(%d).String() = %q, want %q", tc.sev, got, tc.expected)
		}
	}
}

// --- MoveCenterSelection edge cases ---

func TestMoveCenterSelectionEmptyHistory(t *testing.T) {
	m := New()
	// Should not panic on empty history
	m.MoveCenterSelection(1)
	if m.CenterIndex() != 0 {
		t.Errorf("expected centerIdx 0 on empty history, got %d", m.CenterIndex())
	}
	m.MoveCenterSelection(-1)
	if m.CenterIndex() != 0 {
		t.Errorf("expected centerIdx 0 on empty history after -1, got %d", m.CenterIndex())
	}
}

// --- DeleteFromHistory edge cases ---

func TestDeleteFromHistoryNonExistentID(t *testing.T) {
	m := New()
	m.Push("A", "", Info)
	m.Push("B", "", Info)

	m.DeleteFromHistory(999) // non-existent ID
	if len(m.history) != 2 {
		t.Errorf("deleting non-existent ID should not change history, got %d", len(m.history))
	}
}

func TestDeleteFromHistoryAdjustsCenterIndex(t *testing.T) {
	m := New()
	m.Push("A", "", Info)
	m.Push("B", "", Info)
	m.Push("C", "", Info)

	// Set centerIdx to last item (index 2)
	m.centerIdx = 2

	// Delete the last item (C has ID 2)
	m.DeleteFromHistory(2)

	// After deletion, history has 2 items (indices 0,1).
	// centerIdx was 2, which is now >= len(history), so it should be adjusted to 1.
	if m.centerIdx != 1 {
		t.Errorf("expected centerIdx adjusted to 1, got %d", m.centerIdx)
	}
}

func TestDeleteFromHistoryLastItemAdjustsCenterToZero(t *testing.T) {
	m := New()
	m.Push("A", "", Info)

	m.centerIdx = 0
	m.DeleteFromHistory(0) // delete the only item

	// centerIdx should remain 0 (guarded by > 0 check)
	if m.centerIdx != 0 {
		t.Errorf("expected centerIdx 0 after deleting last item, got %d", m.centerIdx)
	}
	if len(m.history) != 0 {
		t.Errorf("expected empty history, got %d", len(m.history))
	}
}

// --- ShowCenter tests ---

func TestShowCenterMarksAllReadAndResetsCenterIndex(t *testing.T) {
	m := New()
	m.Push("A", "", Info)
	m.Push("B", "", Warning)
	m.Push("C", "", Error)

	// Move center index somewhere
	m.centerIdx = 2

	if m.UnreadCount() != 3 {
		t.Fatalf("expected 3 unread, got %d", m.UnreadCount())
	}

	m.ShowCenter()

	if m.CenterIndex() != 0 {
		t.Errorf("ShowCenter should reset center index to 0, got %d", m.CenterIndex())
	}
	if m.UnreadCount() != 0 {
		t.Errorf("ShowCenter should mark all as read, got %d unread", m.UnreadCount())
	}
	if !m.CenterVisible() {
		t.Error("ShowCenter should set center visible")
	}
}

// --- Push + RestoreNotification ID interaction ---

func TestPushAndRestoreShareIDSequence(t *testing.T) {
	m := New()
	m.Push("Pushed", "", Info)
	m.RestoreNotification("Restored", "", "warning", time.Now(), false)
	m.Push("Pushed2", "", Error)

	h := m.History()
	if len(h) != 3 {
		t.Fatalf("expected 3 items, got %d", len(h))
	}
	// IDs should be 0, 1, 2 sequentially
	for i, n := range h {
		if n.ID != i {
			t.Errorf("history[%d] expected ID %d, got %d", i, i, n.ID)
		}
	}
}

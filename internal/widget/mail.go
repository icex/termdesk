package widget

import (
	"fmt"
	"os"
	"path/filepath"
	"time"
)

const mailRefreshInterval = 60 * time.Second

// MailWidget displays the count of unread emails in Maildir.
type MailWidget struct {
	Count    int
	lastRead time.Time
}

func (w *MailWidget) Name() string { return "mail" }

func (w *MailWidget) Render() string {
	if w.lastRead.IsZero() {
		return ""
	}
	if w.Count == 0 {
		return ""
	}
	return fmt.Sprintf("\xf3\xb0\x87\xae %d", w.Count) // 󰇮 nf-md-email
}

func (w *MailWidget) ColorLevel() string {
	if w.Count > 10 {
		return "red"
	}
	if w.Count > 0 {
		return "yellow"
	}
	return "green"
}

func (w *MailWidget) NeedsRefresh() bool {
	return w.lastRead.IsZero() || time.Since(w.lastRead) >= mailRefreshInterval
}

func (w *MailWidget) MarkRefreshed() {
	w.lastRead = time.Now()
}

// ReadMailCount returns the number of new messages in ~/Maildir/new/.
func ReadMailCount() int {
	home, err := os.UserHomeDir()
	if err != nil {
		return 0
	}
	entries, err := os.ReadDir(filepath.Join(home, "Maildir", "new"))
	if err != nil {
		return 0
	}
	return len(entries)
}

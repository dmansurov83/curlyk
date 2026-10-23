package gui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/user/curlyk/internal/i18n"
)

// loadTestFile writes content to a temporary .http file and opens it, which is
// the state autosave works on.
func loadTestFile(t *testing.T, u *ui, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "auto.http")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	u.loadInto(path, content)
	return path
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

// TestApplyAutosaveRespectsTheDebounce covers the decision function: nothing is
// written before the deadline, and the buffer lands on disk after it.
func TestApplyAutosaveRespectsTheDebounce(t *testing.T) {
	u := newTestUI(t)
	path := loadTestFile(t, u, "old\n")
	u.ed.SetText("new\n")
	u.saved = "old\n"

	u.autosaveDeadline = time.Now().Add(time.Minute)
	u.applyAutosave(time.Now())
	if got := readFile(t, path); got != "old\n" {
		t.Fatalf("file = %q, want it untouched inside the debounce window", got)
	}

	u.autosaveDeadline = time.Now().Add(-time.Second)
	u.applyAutosave(time.Now())
	if got := readFile(t, path); got != "new\n" {
		t.Fatalf("file = %q, want the autosaved buffer", got)
	}
	if u.saved != "new\n" {
		t.Fatalf("saved snapshot = %q, want the autosaved text", u.saved)
	}
	if strings.Contains(u.header.Text(), i18n.T("dirty.marker")) {
		t.Fatalf("header = %q, want the dirty marker gone", u.header.Text())
	}
	if !strings.Contains(u.StatusMessage(), i18n.T("status.autosaved", filepath.Base(path))) {
		t.Fatalf("status = %q, want the autosave message", u.StatusMessage())
	}
}

// TestAutosaveTimerWritesAfterTyping proves the timer path end to end with a
// shortened debounce.
func TestAutosaveTimerWritesAfterTyping(t *testing.T) {
	old := autosaveDelay
	autosaveDelay = 20 * time.Millisecond
	defer func() { autosaveDelay = old }()

	u := newTestUI(t)
	path := loadTestFile(t, u, "a\n")
	u.ed.doc.setCaret(0, 0)
	u.ed.TypedRune('x') // "xa\n", triggers onChange → scheduleAutosave

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if readFile(t, path) == "xa\n" {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("file = %q, want the autosaved edit", readFile(t, path))
}

// TestExplicitSaveCancelsAutosave makes sure Ctrl+S does not leave a pending
// timer that could overwrite a later state.
func TestExplicitSaveCancelsAutosave(t *testing.T) {
	u := newTestUI(t)
	path := loadTestFile(t, u, "a\n")
	u.ed.doc.setCaret(0, 0)
	u.ed.TypedRune('b')

	u.saveFile()

	if u.autosaveTimer != nil {
		t.Fatal("an autosave timer is still pending after an explicit save")
	}
	if got := readFile(t, path); got != "ba\n" {
		t.Fatalf("file = %q, want the explicit save", got)
	}
}

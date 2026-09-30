package tui

import (
	"os"
	"path/filepath"
	"testing"
)

// TestSessionSaveLoad verifies content is persisted on exit and restored on
// the next launch (open+save the last file).
func TestSessionSaveLoad(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "sess.http")
	os.Setenv("HTTPTOOL_SESSION_PATH", path)
	defer os.Unsetenv("HTTPTOOL_SESSION_PATH")

	// start with example, put our text, save like on exit
	m := New(Args{Width: 120, Height: 30}).(model)
	m.ed.SetText("GET http://example/save\nAccept: x\n")
	m.saveSession()

	if _, err := os.Stat(path); err != nil {
		t.Fatalf("session file not written: %v", err)
	}

	// simulate a fresh launch: file = empty args
	m2 := New(Args{Width: 120, Height: 30}).(model)
	if got := m2.ed.Text(); got != "GET http://example/save\nAccept: x\n" {
		t.Errorf("session not restored, got:\n%s", got)
	}
}
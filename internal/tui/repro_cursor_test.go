package tui

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/user/curlyk/internal/settings"
)

// TestCursorPersistOnAutosaveLoop verifies the cursor is written to settings by
// the periodic autosave loop (simulating a terminal close without clean quit).
func TestCursorPersistOnAutosaveLoop(t *testing.T) {
	dir := t.TempDir()
	ap := filepath.Join(dir, "appsettings.yml")
	settings.SetAppSettingsPath(ap)
	defer settings.SetAppSettingsPath("appsettings.yml")

	f := filepath.Join(dir, "b_restore.http")
	if err := os.WriteFile(f, []byte("a\nbb\nccc\ndddd\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	m := New(Args{FilePath: f, Width: 120, Height: 30}).(model)
	m.ed.curRow, m.ed.curCol, m.ed.scroll = 3, 1, 2

	// drive one autosave tick: cursor should be persisted without a clean quit
	m.autosaveDeadline = time.Now().Add(-time.Second)
	next, _ := m.Update(autosaveMsg{})
	m2 := next.(model)
	_ = m2

	s := settings.Load()
	abs := filepath.Join(dir, "b_restore.http")
	p, ok := s.FileCursors[abs]
	if !ok {
		t.Fatalf("cursor not persisted via autosave loop; got %+v", s.FileCursors)
	}
	if p.Row != 3 || p.Col != 1 || p.Scroll != 2 {
		t.Errorf("persisted %+v, want row=3 col=1 scroll=2", p)
	}
}
// TestStartupRestoresPerFileCursor simulates closing via the window's X button
// (no clean quit): only rememberCursor runs, which persists the per-file cursor
// but not the global one. On restart the per-file cursor should still be
// restored for the reopened file.
func TestStartupRestoresPerFileCursor(t *testing.T) {
	wd, _ := os.Getwd()
	a := filepath.Join(wd, "a_restore.http")
	if err := os.WriteFile(a, []byte("line0\nline1\nline2\nline3\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	defer os.Remove(a)

	dir := t.TempDir()
	ap := filepath.Join(dir, "appsettings.yml")
	settings.SetAppSettingsPath(ap)
	defer settings.SetAppSettingsPath("appsettings.yml")

	// session: user opens a.http, moves cursor, then the app is closed by the
	// window X button — only rememberCursor runs (no saveSession/quit).
	m := New(Args{Width: 120, Height: 30}).(model)
	m.filePath = a
	m.ed.SetText("line0\nline1\nline2\nline3\n")
	m.ed.curRow, m.ed.curCol, m.ed.scroll = 3, 5, 1
	m.rememberCursor(m.filePath)

	// restart with the same file as the last opened
	m2 := New(Args{FilePath: a, Width: 120, Height: 30}).(model)
	if m2.ed.curRow != 3 {
		t.Errorf("row after restart not restored: %d, want 3", m2.ed.curRow)
	}
	if m2.ed.curCol != 5 {
		t.Errorf("col after restart not restored: %d, want 5", m2.ed.curCol)
	}
	if m2.ed.scroll != 1 {
		t.Errorf("scroll after restart not restored: %d, want 1", m2.ed.scroll)
	}
}
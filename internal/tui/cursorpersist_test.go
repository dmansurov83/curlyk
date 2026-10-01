package tui

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/user/curlyk/internal/settings"
)

// TestCursorPersistAndRestore verifies the cursor position is saved on exit and
// restored on the next launch.
func TestCursorPersistAndRestore(t *testing.T) {
	dir := t.TempDir()
	sp := filepath.Join(dir, "sess.http")
	ap := filepath.Join(dir, "appsettings.yml")
	os.Setenv("HTTPTOOL_SESSION_PATH", sp)
	defer os.Unsetenv("HTTPTOOL_SESSION_PATH")
	settings.SetAppSettingsPath(ap)
	defer settings.SetAppSettingsPath("appsettings.yml")

	m := New(Args{Width: 120, Height: 30}).(model)
	m.ed.SetText("a\nbb\nccc\n")
	m.ed.curRow, m.ed.curCol = 2, 1
	m.ed.scroll = 2
	m.saveSession() // persists file + cursor settings

	// fresh launch: cursor should be restored
	m2 := New(Args{Width: 120, Height: 30}).(model)
	if m2.ed.curRow != 2 {
		t.Errorf("cursor row not restored: %d, want 2", m2.ed.curRow)
	}
	if m2.ed.curCol != 1 {
		t.Errorf("cursor col not restored: %d, want 1", m2.ed.curCol)
	}
}

// TestPerFileCursorRestore verifies the cursor position is remembered per file
// while switching between files and restored when a file is reopened.
func TestPerFileCursorRestore(t *testing.T) {
	m := New(Args{Width: 120, Height: 30}).(model)

	// fake the editor holding file A; move the cursor somewhere distinct
	m.filePath = "a.http"
	m.ed.SetText("line0\nline1\nline2\nline3\n")
	m.ed.curRow, m.ed.curCol, m.ed.scroll = 3, 5, 1
	m.rememberCursor(m.filePath)

	// switch to file B: opens it (resets cursor) then applies B's position
	m.ed.SetText("b0\nb1\n")
	m.filePath = "b.http"
	m.ed.curRow, m.ed.curCol, m.ed.scroll = 1, 2, 0
	m.rememberCursor(m.filePath)

	// switch back to file A: content replaced, cursor restored from memory
	m.ed.SetText("line0\nline1\nline2\nline3\n")
	m.filePath = "a.http"
	m.applyCursor(m.filePath)
	if m.ed.curRow != 3 {
		t.Errorf("file A row not restored: %d, want 3", m.ed.curRow)
	}
	if m.ed.curCol != 5 {
		t.Errorf("file A col not restored: %d, want 5", m.ed.curCol)
	}
	if m.ed.scroll != 1 {
		t.Errorf("file A scroll not restored: %d, want 1", m.ed.scroll)
	}

	// switching to a file never opened keeps cursor at its natural reset position
	m.ed.SetText("fresh\n")
	m.applyCursor("a.http")
	// content changed so the remembered row 3 is clamped to the last line
	if m.ed.curRow != 0 {
		t.Errorf("clamped row: %d, want 0 (buffer too short)", m.ed.curRow)
	}
}

// TestPerFileCursorPersistAcrossRestart verifies per-file cursors are written to
// appsettings.yml and restored on a fresh launch.
func TestPerFileCursorPersistAcrossRestart(t *testing.T) {
	dir := t.TempDir()
	ap := filepath.Join(dir, "appsettings.yml")
	settings.SetAppSettingsPath(ap)
	defer settings.SetAppSettingsPath("appsettings.yml")

	// record a cursor for file "a.http" as if the user moved it there
	m := New(Args{Width: 120, Height: 30}).(model)
	m.filePath = "a.http"
	m.ed.SetText("line0\nline1\nline2\nline3\n")
	m.ed.curRow, m.ed.curCol, m.ed.scroll = 3, 5, 1
	m.rememberCursor(m.filePath)

	// the settings file should have captured the per-file position
	s := settings.Load()
	abs := filepath.Join(mustAbs(t, "."), "a.http")
	pos, ok := s.FileCursors[abs]
	if !ok {
		t.Fatalf("per-file cursor not persisted to settings; got %+v", s.FileCursors)
	}
	if pos.Row != 3 || pos.Col != 5 || pos.Scroll != 1 {
		t.Errorf("persisted pos wrong: %+v", pos)
	}

	// fresh launch: opening a.http restores the remembered cursor
	m2 := New(Args{Width: 120, Height: 30}).(model)
	m2.filePath = "a.http"
	m2.ed.SetText("line0\nline1\nline2\nline3\n")
	m2.applyCursor(m2.filePath)
	if m2.ed.curRow != 3 {
		t.Errorf("row after restart: %d, want 3", m2.ed.curRow)
	}
	if m2.ed.curCol != 5 {
		t.Errorf("col after restart: %d, want 5", m2.ed.curCol)
	}
	if m2.ed.scroll != 1 {
		t.Errorf("scroll after restart: %d, want 1", m2.ed.scroll)
	}
}

func mustAbs(t *testing.T, p string) string {
	t.Helper()
	a, err := filepath.Abs(p)
	if err != nil {
		t.Fatalf("Abs(%q): %v", p, err)
	}
	return a
}
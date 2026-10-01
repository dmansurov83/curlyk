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
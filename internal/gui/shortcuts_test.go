package gui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/driver/desktop"

	"github.com/user/curlyk/internal/i18n"
)

// shortcut sends one custom shortcut with the control modifier.
func shortcut(e *Editor, key fyne.KeyName, extra fyne.KeyModifier) {
	e.TypedShortcut(&desktop.CustomShortcut{KeyName: key, Modifier: fyne.KeyModifierControl | extra})
}

// TestCtrlRIsARunAlias covers the extra run binding.
func TestCtrlRIsARunAlias(t *testing.T) {
	u := newTestUI(t)
	u.ed.SetText("# no request here\n")
	u.ed.doc.setCaret(0, 0)

	shortcut(u.ed, fyne.KeyR, 0)

	if !strings.Contains(u.StatusMessage(), i18n.T("err.noRequestCursor")) {
		t.Fatalf("status = %q, want the run path to report the missing request", u.StatusMessage())
	}
}

// TestRedoAliases covers Ctrl+Shift+Z and the platform Ctrl+Y.
func TestRedoAliases(t *testing.T) {
	for name, key := range map[string]struct {
		key   fyne.KeyName
		extra fyne.KeyModifier
	}{
		"Ctrl+Shift+Z": {key: fyne.KeyZ, extra: fyne.KeyModifierShift},
		"Ctrl+Y":       {key: fyne.KeyY},
	} {
		t.Run(name, func(t *testing.T) {
			u := newTestUI(t)
			u.ed.SetText("abc")
			u.ed.doc.setCaret(0, 3)
			u.ed.TypedRune('d')
			u.ed.Undo()
			if got := u.ed.Text(); got != "abc" {
				t.Fatalf("after undo text = %q", got)
			}

			shortcut(u.ed, key.key, key.extra)

			if got := u.ed.Text(); got != "abcd" {
				t.Fatalf("after %s text = %q, want the redo", name, got)
			}
		})
	}
}

// TestShortcutsWorkInTheCyrillicLayout keeps the desktop window usable with the
// ЙЦУКЕН layout: the physical key names are aliased to their Latin bindings.
func TestShortcutsWorkInTheCyrillicLayout(t *testing.T) {
	u := newTestUI(t)
	dir := t.TempDir()
	path := filepath.Join(dir, "ua.http")
	if err := os.WriteFile(path, []byte("old\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	u.loadInto(path, "old\n")
	u.ed.SetText("new\n")

	// Ctrl+Ы is the physical Ctrl+S key.
	shortcut(u.ed, "Ы", 0)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "new\n" {
		t.Fatalf("file = %q, want the Cyrillic Ctrl+S to save", data)
	}

	// Ctrl+Л is the physical Ctrl+K key (copy as cURL).
	u.ed.SetText("GET http://example.test/x\n")
	u.ed.doc.setCaret(0, 0)
	shortcut(u.ed, "Л", 0)
	if got := clipboard().Content(); !strings.Contains(got, "curl") {
		t.Fatalf("clipboard = %q, want the cURL command", got)
	}

	// Ctrl+А is the physical Ctrl+F key (find).
	shortcut(u.ed, "А", 0)
	if !u.searching {
		t.Fatal("Ctrl+А did not open the search row")
	}
}

// TestCtrlКIsTheCyrillicCtrlR documents the other half of the layout mapping:
// the physical R key is К, so it runs the request.
func TestCtrlКIsTheCyrillicCtrlR(t *testing.T) {
	u := newTestUI(t)
	u.ed.SetText("# no request here\n")
	u.ed.doc.setCaret(0, 0)

	shortcut(u.ed, "К", 0)

	if !strings.Contains(u.StatusMessage(), i18n.T("err.noRequestCursor")) {
		t.Fatalf("status = %q, want the run path", u.StatusMessage())
	}
}

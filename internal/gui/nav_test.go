package gui

import (
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/widget"

	"github.com/user/curlyk/internal/httpfile"
	"github.com/user/curlyk/internal/i18n"
)

// TestNavEntriesPreferNameThenTitleThenMethodURL covers the label precedence of
// the Ctrl+G list, exactly as the console builds it.
func TestNavEntriesPreferNameThenTitleThenMethodURL(t *testing.T) {
	src := "### Named\n@name create\nPOST http://x\n\n### Titled\nGET http://y\n\nGET http://z\n"
	entries := navEntriesFromRequests(httpfile.ParseFile(src))
	if len(entries) != 3 {
		t.Fatalf("entries = %d, want 3: %+v", len(entries), entries)
	}
	want := []string{"create", "Titled", "GET http://z"}
	for i, w := range want {
		if entries[i].label != w {
			t.Errorf("entry %d label = %q, want %q", i, entries[i].label, w)
		}
	}
}

// TestNavFilterNarrowsAndJumps covers typing a filter and accepting the single
// remaining entry: the caret lands on the request line and the editor takes the
// keyboard back.
func TestNavFilterNarrowsAndJumps(t *testing.T) {
	u := newTestUI(t)
	u.ed.SetText("### One\nGET http://one\n\n### Two\nGET http://two\n\n### Three\nGET http://three\n")
	u.openNav()
	if !u.navOpen() {
		t.Fatal("Ctrl+G popup is not open")
	}

	u.nav.filter.SetText("two")
	if got := len(u.nav.visible); got != 1 {
		t.Fatalf("visible entries = %d, want 1", got)
	}
	if !u.nav.key(fyne.KeyReturn) {
		t.Fatal("Enter was not consumed by the popup")
	}
	if u.navOpen() {
		t.Fatal("popup stayed open after the jump")
	}
	row, col := u.ed.Caret()
	if row != 4 || col != 0 {
		t.Fatalf("caret = %d:%d, want 4:0 (the GET http://two line)", row, col)
	}
	if u.active != paneEditor {
		t.Fatalf("active pane = %v, want the editor", u.active)
	}
	if focused := u.win.Canvas().Focused(); focused != u.ed {
		t.Fatalf("focus = %T, want the editor", focused)
	}
}

// TestNavArrowsMoveTheSelection keeps Up/Down inside the filtered list and clamps
// at both ends.
func TestNavArrowsMoveTheSelection(t *testing.T) {
	u := newTestUI(t)
	u.ed.SetText("### One\nGET http://one\n\n### Two\nGET http://two\n\n### Three\nGET http://three\n")
	u.openNav()

	u.nav.key(fyne.KeyDown)
	if u.nav.sel != 1 {
		t.Fatalf("sel after Down = %d, want 1", u.nav.sel)
	}
	u.nav.key(fyne.KeyUp)
	if u.nav.sel != 0 {
		t.Fatalf("sel after Up = %d, want 0", u.nav.sel)
	}
	u.nav.key(fyne.KeyUp)
	if u.nav.sel != 0 {
		t.Fatalf("sel after Up at the top = %d, want it clamped", u.nav.sel)
	}
}

// TestNavEmptyStates covers both empty messages: no requests in the file and no
// match for the filter.
func TestNavEmptyStates(t *testing.T) {
	u := newTestUI(t)
	u.ed.SetText("# just a comment\n")
	u.openNav()
	if got := firstRowText(u.nav.rows); got != i18n.T("nav.noRequests") {
		t.Fatalf("empty file row = %q, want the no-requests message", got)
	}
	u.closeNav()

	u.ed.SetText("### One\nGET http://one\n")
	u.openNav()
	u.nav.filter.SetText("zzz")
	if got := firstRowText(u.nav.rows); got != i18n.T("nav.noMatches") {
		t.Fatalf("filtered row = %q, want the no-matches message", got)
	}
}

// TestCtrlGOpensTheNavPopup checks the shortcut path from the editor.
func TestCtrlGOpensTheNavPopup(t *testing.T) {
	u := newTestUI(t)
	u.ed.SetText("GET http://one\n")
	u.ed.TypedShortcut(&desktop.CustomShortcut{KeyName: fyne.KeyG, Modifier: fyne.KeyModifierControl})
	if !u.navOpen() {
		t.Fatal("Ctrl+G did not open the navigation popup")
	}
}

// firstRowText returns the text of the first label row of the popup.
func firstRowText(rows *fyne.Container) string {
	if len(rows.Objects) == 0 {
		return ""
	}
	if l, ok := rows.Objects[0].(*widget.Label); ok {
		return l.Text
	}
	return ""
}

// TestCtrlGTwiceKeepsOnePopup makes the shortcut idempotent while the popup is
// already open.
func TestCtrlGTwiceKeepsOnePopup(t *testing.T) {
	u := newTestUI(t)
	u.ed.SetText("GET http://one\n")
	u.openNav()
	first := u.nav
	u.openNav()
	if u.nav != first {
		t.Fatal("a second Ctrl+G replaced the open popup")
	}
}

// TestNavPopupIsBigEnough covers the layout defect: the popup used to collapse to
// a tiny box, clipping even a single row. Its minimum size must fit a long label
// and several entries.
func TestNavPopupIsBigEnough(t *testing.T) {
	u := newTestUI(t)
	u.ed.SetText("### A long request title that must be fully readable\nGET http://example.test/a/very/long/path\n")
	u.openNav()

	if u.nav == nil || u.nav.popup == nil {
		t.Fatal("nav popup did not open")
	}
	got := u.nav.popup.MinSize()
	if got.Width < 500 {
		t.Fatalf("popup width = %v, want at least 500 to fit a row", got.Width)
	}
	if got.Height < 300 {
		t.Fatalf("popup height = %v, want room for several rows", got.Height)
	}
}

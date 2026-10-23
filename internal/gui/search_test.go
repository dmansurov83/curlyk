package gui

import (
	"strings"
	"testing"

	"fyne.io/fyne/v2"
)

// openResponseWithText loads a response whose body has two matches of "a".
func openResponseWithText(t *testing.T) *ui {
	t.Helper()
	u := newTestUI(t)
	u.setResponse("GET http://x\n\n{\"a\": \"a\"}", 2)
	return u
}

// TestTypingInResponseOpensTheSearch covers the read-only pane contract: any
// printable character starts a search over the response instead of being lost.
func TestTypingInResponseOpensTheSearch(t *testing.T) {
	u := openResponseWithText(t)
	u.focusPane(paneResp)

	u.resp.TypedRune('a')

	if !u.searching || u.searchPane != paneResp {
		t.Fatalf("searching=%v pane=%v, want a response search", u.searching, u.searchPane)
	}
	if !strings.Contains(u.searchEntry.Text, "a") {
		t.Fatalf("search text = %q, want the typed character", u.searchEntry.Text)
	}
	if u.resp.FindMatches() == 0 {
		t.Fatal("typing in the response found no matches")
	}
}

// TestSearchKeyNavigationWraps checks Enter/Down = next and Up/Shift+Tab =
// previous, wrapping at both ends.
func TestSearchKeyNavigationWraps(t *testing.T) {
	u := openResponseWithText(t)
	u.focusPane(paneResp)
	u.openResponseSearch("a")

	if got := u.resp.FindCurrent(); got != 1 {
		t.Fatalf("first match = %d, want 1", got)
	}
	u.searchEntry.TypedKey(&fyne.KeyEvent{Name: fyne.KeyDown})
	if got := u.resp.FindCurrent(); got != 2 {
		t.Fatalf("after Down = %d, want 2", got)
	}
	u.searchEntry.TypedKey(&fyne.KeyEvent{Name: fyne.KeyDown})
	if got := u.resp.FindCurrent(); got != 1 {
		t.Fatalf("Down past the end = %d, want the wrapped first match", got)
	}
	u.searchEntry.TypedKey(&fyne.KeyEvent{Name: fyne.KeyUp})
	if got := u.resp.FindCurrent(); got != 2 {
		t.Fatalf("Up from the first = %d, want the wrapped last match", got)
	}

	withModifiers(t, fyne.KeyModifierShift)
	u.searchEntry.TypedKey(&fyne.KeyEvent{Name: fyne.KeyTab})
	if got := u.resp.FindCurrent(); got != 1 {
		t.Fatalf("Shift+Tab = %d, want the previous match", got)
	}
}

// TestEscapeClosesResponseSearchAndClearsHighlights covers the dismissal path.
func TestEscapeClosesResponseSearchAndClearsHighlights(t *testing.T) {
	u := openResponseWithText(t)
	u.focusPane(paneResp)
	u.openResponseSearch("a")

	if !u.searchKey(fyne.KeyEscape) {
		t.Fatal("Escape was not consumed by the search field")
	}
	if u.searching {
		t.Fatal("search row is still open after Escape")
	}
	if u.resp.FindMatches() != 0 {
		t.Fatalf("response still has %d matches after closing", u.resp.FindMatches())
	}
}

// TestSearchRetargetsToTheResponsePane covers clicking into the response while a
// file search is open: the query moves to the response instead of closing.
func TestSearchRetargetsToTheResponsePane(t *testing.T) {
	u := newTestUI(t)
	u.ed.SetText("hit one\nhit two\n")
	u.focusPane(paneEditor)
	u.toggleSearch()
	u.searchEntry.SetText("hit")
	if u.ed.FindMatches() != 2 {
		t.Fatalf("editor matches = %d, want 2", u.ed.FindMatches())
	}

	u.setResponse("GET http://x\n\nhit hit", 2)
	u.setActive(paneResp)

	if !u.searching || u.searchPane != paneResp {
		t.Fatalf("searching=%v pane=%v, want the response", u.searching, u.searchPane)
	}
	if u.resp.FindMatches() != 2 {
		t.Fatalf("response matches = %d, want 2", u.resp.FindMatches())
	}
	if u.ed.FindMatches() != 0 {
		t.Fatalf("editor still highlights %d matches", u.ed.FindMatches())
	}
}

// TestTabCyclesPanesAndFocus checks Tab/Shift+Tab move through the pane order and
// hand the keyboard focus to the next widget.
func TestTabCyclesPanesAndFocus(t *testing.T) {
	u := newTestUI(t)
	u.focusPane(paneEditor)

	u.ed.TypedKey(&fyne.KeyEvent{Name: fyne.KeyTab})
	if u.active != paneResp {
		t.Fatalf("active pane = %v, want the response", u.active)
	}
	if focused := u.win.Canvas().Focused(); focused != u.resp {
		t.Fatalf("focus = %T, want the response editor", focused)
	}

	u.resp.TypedKey(&fyne.KeyEvent{Name: fyne.KeyTab})
	if u.active != paneFiles {
		t.Fatalf("active pane = %v, want the file panel", u.active)
	}

	u.focusPane(paneEditor)
	withModifiers(t, fyne.KeyModifierShift)
	u.ed.TypedKey(&fyne.KeyEvent{Name: fyne.KeyTab})
	if u.active != paneFiles {
		t.Fatalf("Shift+Tab active pane = %v, want the file panel", u.active)
	}
}

// TestTabDoesNotStealTheSearchFieldFocus keeps the search usable: while the row
// is open, Tab belongs to the input, not the pane cycle.
func TestTabDoesNotStealTheSearchFieldFocus(t *testing.T) {
	u := newTestUI(t)
	u.focusPane(paneEditor)
	u.toggleSearch()

	u.searchEntry.TypedKey(&fyne.KeyEvent{Name: fyne.KeyTab})

	if !u.searching || u.searchPane != paneEditor {
		t.Fatalf("searching=%v pane=%v, want the editor search to stay open", u.searching, u.searchPane)
	}
	if focused := u.win.Canvas().Focused(); focused != u.searchEntry {
		t.Fatalf("focus = %T, want the search field", focused)
	}
}

// TestTabFromTheFilePanelKeepsTheCycleGoing covers the last leg: the sidebar list
// also forwards Tab, so the pane cycle never dead-ends.
func TestTabFromTheFilePanelKeepsTheCycleGoing(t *testing.T) {
	u := newTestUI(t)
	u.focusPane(paneFiles)

	u.files.view.TypedKey(&fyne.KeyEvent{Name: fyne.KeyTab})
	if u.active != paneEditor {
		t.Fatalf("Tab from files = %v, want the editor", u.active)
	}

	u.focusPane(paneFiles)
	withModifiers(t, fyne.KeyModifierShift)
	u.files.view.TypedKey(&fyne.KeyEvent{Name: fyne.KeyTab})
	if u.active != paneResp {
		t.Fatalf("Shift+Tab from files = %v, want the response", u.active)
	}
}

// TestPanesCaptureTab guards the real key dispatch: Fyne moves focus itself on
// Tab unless the focused widget implements Tabbable and accepts it, so all three
// panes must claim the key.
func TestPanesCaptureTab(t *testing.T) {
	u := newTestUI(t)
	panes := map[string]fyne.CanvasObject{
		"editor":   u.ed,
		"response": u.resp,
		"files":    u.files.view,
	}
	for name, obj := range panes {
		tab, ok := obj.(fyne.Tabbable)
		if !ok {
			t.Errorf("%s does not implement fyne.Tabbable", name)
			continue
		}
		if !tab.AcceptsTab() {
			t.Errorf("%s does not accept Tab", name)
		}
	}
}

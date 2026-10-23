package gui

import (
	"strings"
	"testing"

	"github.com/user/curlyk/internal/i18n"
)

// TestPasteWithoutAHandlerInsertsText covers the plain editor contract: with no
// paste hook the clipboard is inserted exactly as it was.
func TestPasteWithoutAHandlerInsertsText(t *testing.T) {
	e := newTestEditor(t, "first")
	e.doc.setCaret(0, 5)
	cb := clipboard()
	if cb == nil {
		t.Fatal("no clipboard in the test app")
	}
	cb.SetContent(" second")

	if !e.Paste() {
		t.Fatal("Paste reported nothing inserted")
	}
	if got := e.Text(); got != "first second" {
		t.Fatalf("text = %q, want the clipboard appended", got)
	}
}

// TestPasteCurlConvertsToOneUndoStep covers the window hook: a curl command from
// the clipboard becomes an .http block, and one undo restores the buffer.
func TestPasteCurlConvertsToOneUndoStep(t *testing.T) {
	u := newTestUI(t)
	u.ed.SetText("first\n")
	u.ed.doc.setCaret(1, 0)

	clipboard().SetContent(`curl -X POST http://example.test/x -H "Content-Type: application/json" -d '{"a":1}'`)
	if !u.ed.Paste() {
		t.Fatal("Paste reported nothing inserted")
	}
	got := u.ed.Text()
	if !strings.Contains(got, "POST http://example.test/x") {
		t.Fatalf("pasted text is not a request block:\n%s", got)
	}
	if !strings.Contains(got, "Content-Type: application/json") {
		t.Fatalf("pasted text is missing the header:\n%s", got)
	}
	if !u.ed.CanUndo() {
		t.Fatal("the converted paste was not recorded as an undo step")
	}

	u.ed.Undo()
	if got := u.ed.Text(); got != "first\n" {
		t.Fatalf("after one undo text = %q, want the original buffer", got)
	}
}

// TestPasteBrokenCurlKeepsTheBuffer checks the failure path: an unparsable curl
// command is reported and nothing is inserted.
func TestPasteBrokenCurlKeepsTheBuffer(t *testing.T) {
	u := newTestUI(t)
	u.ed.SetText("first\n")
	u.ed.doc.setCaret(1, 0)

	// An unbalanced quote makes the shell splitter fail while the text still
	// looks like a curl command.
	clipboard().SetContent(`curl "http://example.test/x`)
	if u.ed.Paste() {
		t.Fatal("Paste reported an insertion for a broken curl command")
	}
	if got := u.ed.Text(); got != "first\n" {
		t.Fatalf("text = %q, want the buffer untouched", got)
	}
	if !strings.Contains(u.StatusMessage(), i18n.T("err.convertCurl", "")) {
		t.Fatalf("status = %q, want the conversion error", u.StatusMessage())
	}
}

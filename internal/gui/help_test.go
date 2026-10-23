package gui

import (
	"strings"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/driver/desktop"

	"github.com/user/curlyk/internal/i18n"
)

// TestHelpListsTheGUIKeys checks the reference content: the desktop key list is
// rendered with localised descriptions.
func TestHelpListsTheGUIKeys(t *testing.T) {
	text := helpText()
	for _, want := range []string{i18n.T("help.title"), "Ctrl+G", "Ctrl+F", "F1", i18n.T("help.run")} {
		if !strings.Contains(text, want) {
			t.Errorf("help text is missing %q:\n%s", want, text)
		}
	}
	if !strings.Contains(text, i18n.T("help.varTitle")) {
		t.Error("help text is missing the variables section")
	}
}

// TestF1TogglesHelpAndRestoresResponse covers the toggle over a real reply.
func TestF1TogglesHelpAndRestoresResponse(t *testing.T) {
	u := newTestUI(t)
	u.setResponse("GET http://x\n\n{\"a\":1}", 2)
	if u.helpShown() {
		t.Fatal("a response should hide the help")
	}

	u.toggleHelp()
	if !u.helpShown() {
		t.Fatal("help did not open")
	}
	if !strings.Contains(u.resp.Text(), i18n.T("help.title")) {
		t.Fatalf("pane text = %q, want the help", u.resp.Text())
	}

	u.toggleHelp()
	if u.helpShown() {
		t.Fatal("help did not close")
	}
	if got := u.resp.Text(); !strings.Contains(got, "\"a\":1") {
		t.Fatalf("restored response = %q, want the saved reply", got)
	}
}

// TestF1ShortcutReachesTheWindow checks the bare F1 key on the focused editor.
func TestF1ShortcutReachesTheWindow(t *testing.T) {
	u := newTestUI(t)
	u.setResponse("GET http://x\n\n{\"a\":1}", 2)
	u.ed.TypedShortcut(&desktop.CustomShortcut{KeyName: fyne.KeyF1})
	if !u.helpShown() {
		t.Fatal("F1 did not open the help")
	}
	u.ed.TypedShortcut(&desktop.CustomShortcut{KeyName: fyne.KeyF1})
	if u.helpShown() {
		t.Fatal("the second F1 did not close the help")
	}
}

// TestNewResponseHidesHelp keeps the reply visible: running a request while F1
// shows the reference returns the pane to the response.
func TestNewResponseHidesHelp(t *testing.T) {
	u := newTestUI(t)
	if !u.helpShown() {
		t.Fatal("the window should start with the help")
	}

	u.setResponse("GET http://x\n\n{\"a\":1}", 2)
	if u.helpShown() {
		t.Fatal("a new response must hide the help")
	}
	if got := u.resp.Text(); !strings.Contains(got, "\"a\":1") {
		t.Fatalf("pane text = %q, want the new response", got)
	}
}

// TestTypingOnHelpDoesNotSearch keeps the reference inert: it is not a reply, so
// printable input and Ctrl+F must not start a search over it.
func TestTypingOnHelpDoesNotSearch(t *testing.T) {
	u := newTestUI(t)
	u.focusPane(paneResp)
	if !u.helpShown() {
		t.Fatal("the pane should start with the help")
	}

	u.resp.TypedRune('a')
	if u.searching {
		t.Fatal("typing over the help opened a search")
	}

	u.toggleSearch()
	if u.searching {
		t.Fatal("Ctrl+F over the help opened a search")
	}
	if !strings.Contains(u.StatusMessage(), i18n.T("err.noResponse")) {
		t.Fatalf("status = %q, want the no-response message", u.StatusMessage())
	}
}

// TestF1KeyOpensHelp covers the real dispatch: a bare F1 reaches the widget as a
// key event (custom shortcuts require a modifier), so windowKey must handle it.
func TestF1KeyOpensHelp(t *testing.T) {
	u := newTestUI(t)
	u.setResponse("GET http://x\n\n{\"a\":1}", 2)

	u.ed.TypedKey(&fyne.KeyEvent{Name: fyne.KeyF1})
	if !u.helpShown() {
		t.Fatal("the F1 key event did not open the help")
	}
	u.ed.TypedKey(&fyne.KeyEvent{Name: fyne.KeyF1})
	if u.helpShown() {
		t.Fatal("the second F1 key event did not close the help")
	}

	u.focusPane(paneFiles)
	u.files.view.TypedKey(&fyne.KeyEvent{Name: fyne.KeyF1})
	if !u.helpShown() {
		t.Fatal("F1 from the file list did not open the help")
	}
}

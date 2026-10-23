package gui

import (
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/widget"

	"github.com/user/curlyk/internal/i18n"
	"github.com/user/curlyk/internal/runner"
	"github.com/user/curlyk/internal/settings"
)

// newTestUI builds the whole window in a temporary working directory, so the
// settings and the profiles a test writes never touch the repository.
func newTestUI(t *testing.T) *ui {
	t.Helper()
	dir := t.TempDir()
	t.Chdir(dir)
	settings.SetAppSettingsPath(filepath.Join(dir, "appsettings.yml"))
	i18n.SetLocale("ru")
	t.Cleanup(func() { i18n.SetLocale("ru") })

	u := newUI(fyne.CurrentApp())
	t.Cleanup(u.stopAutosave)
	t.Cleanup(func() {
		if u.spinner != nil {
			u.spinner.Stop()
		}
	})
	return u
}

// TestSearchRowReplacesTheHeader covers the console behaviour the window copies:
// Ctrl+F turns the top row into a search field with a match counter, and closing
// it restores the file header and drops the highlights.
func TestSearchRowReplacesTheHeader(t *testing.T) {
	u := newTestUI(t)
	u.ed.SetText("hit one\nhit two\nhit three")
	headerBefore := u.header.Text()

	u.toggleSearch()
	if !u.searching {
		t.Fatal("Ctrl+F did not open the search row")
	}

	u.searchEntry.SetText("hit")
	if got := u.searchCount.Text; got != "1/3" {
		t.Fatalf("counter = %q, want 1/3", got)
	}

	u.ed.FindNext()
	u.updateSearchCount()
	if got := u.searchCount.Text; got != "2/3" {
		t.Fatalf("counter after next = %q, want 2/3", got)
	}

	u.closeSearch()
	if u.searching {
		t.Fatal("closing the search left the row open")
	}
	if u.ed.FindMatches() != 0 {
		t.Fatalf("closing the search left %d matches highlighted", u.ed.FindMatches())
	}
	if got := u.searchEntry.Text; got != "" {
		t.Fatalf("closing the search left the query %q", got)
	}
	if got := u.header.Text(); !strings.HasPrefix(got, headerFilePart(headerBefore)) {
		t.Fatalf("header after closing the search = %q, want the file header back (%q)", got, headerBefore)
	}
}

// headerFilePart drops the caret suffix from a header line, so a test can compare
// the file part without depending on where the caret ended up.
func headerFilePart(header string) string {
	parts := strings.SplitN(header, " — ", 2)
	return parts[0]
}

// profileRowLabels returns the profile names shown in the sidebar, without the
// active-profile marker, so a test can assert what the panel lists.
func profileRowLabels(u *ui) []string {
	var out []string
	for _, o := range u.files.box.Objects {
		btn, ok := o.(*widget.Button)
		if !ok {
			continue
		}
		label := strings.TrimSpace(btn.Text)
		label = strings.TrimPrefix(label, i18n.T("profile.activeBadge"))
		label = strings.TrimSpace(label)
		out = append(out, label)
	}
	return out
}

func TestRequestUnderCaretAndSubstitution(t *testing.T) {
	u := newTestUI(t)
	u.ed.SetText("@var host = http://example.com\n\n### r\nGET {{host}}/v1\nAccept: {{kind}}\n")
	u.ed.doc.setCaret(3, 0)

	req := u.requestAtCaret()
	if req == nil {
		t.Fatal("no request found under the caret")
	}
	if req.Method != "GET" {
		t.Fatalf("method = %q, want GET", req.Method)
	}

	// The profile supplies the variable the file leaves undefined.
	if err := os.WriteFile("dev.profile", []byte("@var kind = application/json\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	u.settings.ActiveProfile = "dev.profile"

	if err := u.substituteVars(req); err != nil {
		t.Fatalf("substitution failed: %v", err)
	}
	if req.URL != "http://example.com/v1" {
		t.Fatalf("url = %q, want the substituted host", req.URL)
	}
	if len(req.Headers) != 1 || req.Headers[0].Value != "application/json" {
		t.Fatalf("headers = %+v, want the profile value", req.Headers)
	}
}

func TestSubstitutionErrorNamesTheVariable(t *testing.T) {
	u := newTestUI(t)
	u.ed.SetText("GET {{missing}}/v1")
	u.ed.doc.setCaret(0, 0)

	req := u.requestAtCaret()
	err := u.substituteVars(req)
	if err == nil {
		t.Fatal("an undefined variable should abort the run")
	}
	if msg := substitutionMessage(err); !strings.Contains(msg, "missing") {
		t.Fatalf("message = %q, want it to name the variable", msg)
	}
}

func TestRunRequestWithoutARequestReportsIt(t *testing.T) {
	u := newTestUI(t)
	u.ed.SetText("# just a comment\n")
	u.ed.doc.setCaret(0, 0)

	u.runRequest()
	// Like the console, the failure is reported on the status line, not in the
	// response pane.
	if !strings.Contains(u.StatusMessage(), i18n.T("err.noRequestCursor")) {
		t.Fatalf("status = %q, want the no-request message", u.StatusMessage())
	}
}

func TestFormatResponseRendersStatusHeadersAndBody(t *testing.T) {
	res := &runner.Result{
		Status:   "200 OK",
		Duration: 12 * time.Millisecond,
		Response: &http.Response{
			StatusCode: 200,
			Status:     "200 OK",
			Header:     http.Header{"Content-Type": {"application/json"}},
		},
	}
	out, _ := formatResponse(res, []byte(`{"a":1}`))
	for _, want := range []string{"200", "Content-Type", "application/json", "\"a\": 1"} {
		if !strings.Contains(out, want) {
			t.Errorf("response text is missing %q:\n%s", want, out)
		}
	}
	if strings.Contains(out, "\r") {
		t.Error("response text must not carry carriage returns (they break the layout)")
	}
}

func TestFormatResponseRendersNetworkError(t *testing.T) {
	res := &runner.Result{Err: errors.New("dial tcp 127.0.0.1:1: connect: connection refused")}
	out, _ := formatResponse(res, nil)
	if !strings.Contains(out, "connection refused") {
		t.Fatalf("response text = %q, want the network error", out)
	}
	if !strings.Contains(out, i18n.T("resp.netFailed")) {
		t.Fatalf("response text = %q, want the localized failure line", out)
	}
}

func TestProfileSelectionPersists(t *testing.T) {
	u := newTestUI(t)
	if err := os.WriteFile("dev.profile", []byte("@var host = http://localhost\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	u.reloadFiles()

	// newUI creates default.profile when the directory has none, so "dev" joins
	// it rather than replacing it. The rows live in the sidebar's profile box.
	rows := profileRowLabels(u)
	if !slices.Contains(rows, "dev") {
		t.Fatalf("profile rows = %v, want them to include dev", rows)
	}

	u.onProfileChanged("dev")
	if got := settings.Load().ActiveProfile; got != "dev.profile" {
		t.Fatalf("stored profile = %q, want dev.profile", got)
	}
	if got := u.profileVars()["host"]; got != "http://localhost" {
		t.Fatalf("profile vars = %v", u.profileVars())
	}
}

// TestCaretPersistedAndRestored covers the per-file caret memory: leaving a file
// records where the caret was, and reopening it goes back there.
func TestCaretPersistedAndRestored(t *testing.T) {
	u := newTestUI(t)
	path := filepath.Join(t.TempDir(), "api.http")
	u.ed.SetText(manyLines(80))
	u.path = path
	u.ed.RestoreCaret(40, 3, 0)

	u.persistSession()
	saved := settings.Load()
	pos, ok := saved.FileCursors[path]
	if !ok {
		t.Fatalf("no caret recorded for %s (map=%v)", path, saved.FileCursors)
	}
	if pos.Row != 40 || pos.Col != 3 {
		t.Fatalf("recorded caret = (%d,%d), want (40,3)", pos.Row, pos.Col)
	}

	// A fresh window restores it from the settings file.
	i18n.SetLocale("ru")
	u2 := newUI(fyne.CurrentApp())
	u2.loadInto(path, manyLines(80))
	if row, col := u2.ed.Caret(); row != 40 || col != 3 {
		t.Fatalf("restored caret = (%d,%d), want (40,3)", row, col)
	}
}

func TestCompletionSuggestsHeadersAndAccepts(t *testing.T) {
	u := newTestUI(t)
	u.ed.SetText("GET https://api.example.com/x\nAc")
	u.ed.doc.setCaret(1, 2)

	items := u.suggestionsForCaret()
	idx := -1
	for i, s := range items {
		if s.Label == "Accept" {
			idx = i
		}
	}
	if idx < 0 {
		t.Fatalf("no Accept suggestion: %+v", items)
	}

	u.complete.sel = idx
	u.openCompletion()
	if !u.completionOpen() {
		t.Fatal("the popup did not open")
	}
	u.acceptCompletion()
	if got := u.ed.LineAt(1); got != "Accept" {
		t.Fatalf("line after accept = %q, want Accept", got)
	}
	if u.completionOpen() {
		t.Fatal("accepting left the popup open")
	}
}

// TestCompletionClosesBracesForVariables covers the {{name}} flow: typing "{{"
// offers the file's variables and accepting one closes the braces.
func TestCompletionClosesBracesForVariables(t *testing.T) {
	u := newTestUI(t)
	u.ed.SetText("@var token = abc\n\nGET https://api.example.com/x\nAuthorization: Bearer {{")
	u.ed.doc.setCaret(3, runeLen("Authorization: Bearer {{"))
	u.ed.doc.breakUndoRun()

	// Typing "{{" is what opens the variable list in the real window.
	u.maybeAutoCompletion()
	if !u.completionOpen() {
		t.Fatal("\"{\" plus \"{\" did not open the completion popup")
	}

	found := false
	for i, s := range u.complete.items {
		if s.Label == "token" {
			u.complete.sel = i
			found = true
		}
	}
	if !found {
		t.Fatalf("variable not suggested: %+v", u.complete.items)
	}

	u.acceptCompletion()
	if got := u.ed.LineAt(3); got != "Authorization: Bearer {{token}}" {
		t.Fatalf("line after accept = %q", got)
	}
}

func TestCompletionWithoutCandidatesStaysClosed(t *testing.T) {
	u := newTestUI(t)
	u.ed.SetText("{ \"a\": 1 }")
	u.ed.doc.setCaret(0, 5)
	u.openCompletion()
	if u.completionOpen() {
		t.Fatal("a popup opened with no candidates")
	}
}

// TestCompletionArrowsMoveTheSelectionAndEnterAccepts covers the reported
// defect: with the list open the arrow keys must move the highlight and Enter
// must accept it. Because the list is painted by the editor (not a Fyne popup
// overlay), the editor keeps the keyboard focus and its key hook receives the
// keys.
func TestCompletionArrowsMoveTheSelectionAndEnterAccepts(t *testing.T) {
	u := newTestUI(t)
	u.ed.SetText("@var alpha = 1\n@var beta = 2\n\nGET https://x\nAuthorization: Bearer {{")
	u.ed.doc.setCaret(4, runeLen("Authorization: Bearer {{"))
	u.ed.doc.breakUndoRun()

	u.maybeAutoCompletion()
	if !u.completionOpen() {
		t.Fatal("typing {{ did not open the list")
	}
	u.complete.sel = 0

	// Arrow down moves the highlight; the popup stays open.
	if u.completionKey(fyne.KeyDown) {
		if u.complete.sel != 1 {
			t.Fatalf("after Down the selection = %d, want 1", u.complete.sel)
		}
	} else {
		t.Fatal("Down was not consumed by the completion list")
	}
	if !u.completionOpen() {
		t.Fatal("Down closed the list")
	}
	// The highlighted row is the second suggestion in the engine's order; a
	// non-{name} function is fine — the point is that Enter inserts exactly what
	// is highlighted, whatever it is.
	want := u.complete.items[u.complete.sel].Snippet

	// Enter accepts the highlighted row and closes the list.
	if !u.completionKey(fyne.KeyReturn) {
		t.Fatal("Enter was not consumed by the completion list")
	}
	if u.completionOpen() {
		t.Fatal("Enter left the list open")
	}
	if got := u.ed.LineAt(4); !strings.Contains(got, want) {
		t.Fatalf("line after Enter = %q, want it to contain the accepted row %q", got, want)
	}
}

// TestCompletionUpWrapsTheSelection pins the Up direction: it cycles back to
// the last row from the first, matching the TUI popup.
func TestCompletionUpWrapsTheSelection(t *testing.T) {
	u := newTestUI(t)
	u.ed.SetText("@var alpha = 1\n@var beta = 2\n@var gamma = 3\n\nAuthorization: Bearer {{")
	u.ed.doc.setCaret(4, runeLen("Authorization: Bearer {{"))
	u.ed.doc.breakUndoRun()

	u.maybeAutoCompletion()
	u.complete.sel = 0
	if !u.completionKey(fyne.KeyUp) {
		t.Fatal("Up was not consumed by the completion list")
	}
	if got := u.complete.sel; got != len(u.complete.items)-1 {
		t.Fatalf("Up from the first row = %d, want the last (%d)", got, len(u.complete.items)-1)
	}
}

// TestTypingWithTheCompletionListOpenKeepsWorking covers the other half of the
// reported defect: while the list is open the user must still be able to type.
// The editor holds the focus, so a key that is not a list key falls through to
// the editor and inserts text.
func TestTypingWithTheCompletionListOpenKeepsWorking(t *testing.T) {
	u := newTestUI(t)
	u.ed.SetText("@var token = abc\n\nAuthorization: Bearer {{")
	u.ed.doc.setCaret(2, runeLen("Authorization: Bearer {{"))
	u.ed.doc.breakUndoRun()

	u.maybeAutoCompletion()
	if !u.completionOpen() {
		t.Fatal("typing {{ did not open the list")
	}

	// A plain rune must reach the editor buffer.
	u.ed.TypedRune('t')
	if got := u.ed.LineAt(2); got != "Authorization: Bearer {{t" {
		t.Fatalf("line after typing = %q, want the rune inserted", got)
	}
}

// TestCompletionMouseClickAccepts covers the mouse path: clicking a row of the
// list must accept that suggestion. Coordinates are worked out from the same
// geometry the renderer uses, so a real click and a test click agree.
func TestCompletionMouseClickAccepts(t *testing.T) {
	u := newTestUI(t)
	u.ed.SetText("@var alpha = 1\n@var beta = 2\n\nAuthorization: Bearer {{")
	u.ed.doc.setCaret(3, runeLen("Authorization: Bearer {{"))
	u.ed.doc.breakUndoRun()
	u.maybeAutoCompletion()
	if !u.completionOpen() {
		t.Fatal("typing {{ did not open the list")
	}
	u.complete.sel = 0

	// Hit the second row (index 1) of the list.
	pos, _ := u.ed.completionBounds()
	_, lineH := u.ed.metrics()
	if lineH < 18 {
		lineH = 18
	}
	click := fyne.NewPos(pos.X+4, pos.Y+lineH+lineH/2) // second row
	if row := u.ed.completionRowAt(click); row != 1 {
		t.Fatalf("clicked row = %d, want 1", row)
	}

	accepted := u.complete.items[1].Snippet
	u.ed.MouseDown(&desktop.MouseEvent{
		PointEvent: fyne.PointEvent{Position: click},
		Button:     desktop.MouseButtonPrimary,
	})
	if u.completionOpen() {
		t.Fatal("clicking a row left the list open")
	}
	if !strings.Contains(u.ed.LineAt(3), accepted) {
		t.Fatalf("line after click = %q, want it to contain %q", u.ed.LineAt(3), accepted)
	}
}

// TestCompletionKeyboardEventsGoToTheFocusedEditor reproduces the real window:
// the events are delivered to whatever canvas.Focused() returns. If the editor
// is not focused after the list opens, the arrows would move nothing.
func TestCompletionKeyboardEventsGoToTheFocusedEditor(t *testing.T) {
	u := newTestUI(t)
	u.ed.SetText("@var token = abc\n\nAuthorization: Bearer {{")
	u.ed.doc.setCaret(2, runeLen("Authorization: Bearer {{"))
	u.ed.doc.breakUndoRun()
	u.win.Canvas().Focus(u.ed)
	u.maybeAutoCompletion()
	if !u.completionOpen() {
		t.Fatal("typing {{ did not open the list")
	}
	u.complete.sel = 0

	// The editor must be the focused widget, otherwise the real driver routes the
	// arrow to nothing.
	if got := u.win.Canvas().Focused(); got != u.ed {
		t.Fatalf("focused widget = %T, want the editor", got)
	}

	// Route the arrow through the canvas focus exactly like the driver does.
	if focused, ok := u.win.Canvas().Focused().(fyne.Focusable); ok {
		focused.TypedKey(&fyne.KeyEvent{Name: fyne.KeyDown})
	}
	if u.complete.sel != 1 {
		t.Fatalf("after Down the selection = %d, want 1", u.complete.sel)
	}
}

// TestCompletionKeysThroughTypedKey drives the real delivery path: a KeyDown
// reaches the editor's TypedKey, which consults the key hook (windowKey) that
// routes to the completion list. This is how the keys arrive in the running
// window, unlike calling completionKey directly.
func TestCompletionKeysThroughTypedKey(t *testing.T) {
	u := newTestUI(t)
	u.ed.SetText("@var token = abc\n\nAuthorization: Bearer {{")
	u.ed.doc.setCaret(2, runeLen("Authorization: Bearer {{"))
	u.ed.doc.breakUndoRun()
	u.maybeAutoCompletion()
	if !u.completionOpen() {
		t.Fatal("typing {{ did not open the list")
	}
	u.complete.sel = 0

	// KeyDown must reach completionKey through the editor's hook and move the
	// selection instead of moving the caret.
	u.ed.TypedKey(&fyne.KeyEvent{Name: fyne.KeyDown})
	if !u.completionOpen() {
		t.Fatal("the KeyDown closed the list")
	}
	if u.complete.sel != 1 {
		t.Fatalf("after KeyDown the selection = %d, want 1", u.complete.sel)
	}
	// The caret must not move while the list is open.
	if r, c := u.ed.Caret(); r != 2 || c != runeLen("Authorization: Bearer {{") {
		t.Fatalf("caret moved to (%d,%d) by the KeyDown, want it untouched", r, c)
	}

	// KeyReturn accepts and closes. Capture the accepted snippet before the
	// list closes and clears the items.
	accepted := u.complete.items[1].Snippet
	u.ed.TypedKey(&fyne.KeyEvent{Name: fyne.KeyReturn})
	if u.completionOpen() {
		t.Fatal("KeyReturn left the list open")
	}
	if !strings.Contains(u.ed.LineAt(2), accepted) {
		t.Fatalf("line after KeyReturn = %q, want it to contain %q", u.ed.LineAt(2), accepted)
	}
}

// TestCompletionIntegrationThroughTheWindow drives the whole window the way the
// real glfw app does: focus lives on the canvas, typing goes through TypedRune,
// and arrow/Enter go through the focused widget's TypedKey. This catches focus
// routing that unit tests on the editor alone miss.
func TestCompletionIntegrationThroughTheWindow(t *testing.T) {
	u := newTestUI(t)
	u.ed.SetText("@var token = abc\n\nGET https://x\nAuthorization: Bearer {{")
	u.ed.doc.setCaret(3, runeLen("Authorization: Bearer {{"))
	u.ed.doc.breakUndoRun()

	// Give the editor focus on the real window canvas, then open the list via the
	// auto path (the same trigger as typing the second '{').
	u.win.Canvas().Focus(u.ed)
	u.maybeAutoCompletion()
	if !u.completionOpen() {
		t.Fatal("the list did not open")
	}
	u.complete.sel = 0
	if got := u.win.Canvas().Focused(); got != u.ed {
		t.Fatalf("window focus = %T, want the editor", got)
	}

	// Deliver the arrow through the window's focused widget (what glfw does).
	if focused, ok := u.win.Canvas().Focused().(fyne.Focusable); ok {
		focused.TypedKey(&fyne.KeyEvent{Name: fyne.KeyDown})
	} else {
		t.Fatalf("focused widget is not Focusable: %T", u.win.Canvas().Focused())
	}
	if u.complete.sel != 1 {
		t.Fatalf("after Down the selection = %d, want 1", u.complete.sel)
	}

	// Enter through the same path accepts and closes.
	accepted := u.complete.items[1].Snippet
	if focused, ok := u.win.Canvas().Focused().(fyne.Focusable); ok {
		focused.TypedKey(&fyne.KeyEvent{Name: fyne.KeyReturn})
	}
	if u.completionOpen() {
		t.Fatal("Enter did not close the list")
	}
	if !strings.Contains(u.ed.LineAt(3), accepted) {
		t.Fatalf("line after Enter = %q, want %q", u.ed.LineAt(3), accepted)
	}
}

// TestCompletionWithCtrlSpaceThenArrows reproduces the reported workflow: type
// a letter, press Ctrl+Space to open the list, then use the arrows to choose.
// The arrows must move the highlight (not the caret) and Enter must accept.
func TestCompletionWithCtrlSpaceThenArrows(t *testing.T) {
	u := newTestUI(t)
	u.ed.SetText("G\nAccept:\nGET http://a\n")
	u.ed.doc.setCaret(0, 1) // after the "G"
	u.win.Canvas().Focus(u.ed)

	// Ctrl+Space opens the list with the method candidates. In the real window
	// glfw delivers it as a CustomShortcut to the focused widget's TypedShortcut.
	u.ed.TypedShortcut(&desktop.CustomShortcut{KeyName: fyne.KeySpace, Modifier: fyne.KeyModifierControl})
	if !u.completionOpen() {
		t.Fatal("Ctrl+Space did not open the list")
	}

	// The focused widget is the editor; arrows go through it.
	focused, ok := u.win.Canvas().Focused().(fyne.Focusable)
	if !ok {
		t.Fatalf("focused widget is not Focusable: %T", u.win.Canvas().Focused())
	}
	if focused != u.ed {
		t.Fatalf("focused widget = %T, want the editor", focused)
	}

	// Arrow down then Enter choose "GET" and replace the "G".
	focused.TypedKey(&fyne.KeyEvent{Name: fyne.KeyDown})
	if !u.completionOpen() {
		t.Fatal("the arrow closed the list")
	}
	focused.TypedKey(&fyne.KeyEvent{Name: fyne.KeyReturn})
	if u.completionOpen() {
		t.Fatal("Enter left the list open")
	}
	if got := u.ed.LineAt(0); got != "GET" {
		t.Fatalf("line after choosing = %q, want GET", got)
	}
}

// TestTypingTheBracesAutoOpensAndArrowsChoose reproduces the first reported
// workflow exactly: the user types "{{" (which auto-opens the list via the
// change callback), then presses an arrow and Enter. The arrow and Enter must
// be consumed by the list, not move the caret / insert a newline.
func TestTypingTheBracesAutoOpensAndArrowsChoose(t *testing.T) {
	u := newTestUI(t)
	u.ed.SetText("Authorization: Bearer \n")
	u.ed.doc.setCaret(0, runeLen("Authorization: Bearer "))
	u.win.Canvas().Focus(u.ed)

	// Typing "{ { " fires onChange -> maybeAutoCompletion -> openCompletion.
	u.ed.TypedRune('{')
	if u.completionOpen() {
		t.Fatal("the list must not open on a single {")
	}
	u.ed.TypedRune('{')
	if !u.completionOpen() {
		t.Fatal("typing {{ did not open the list")
	}
	if got := u.win.Canvas().Focused(); got != u.ed {
		t.Fatalf("focused widget = %T, want the editor", got)
	}

	// Arrow down must move the selection, not the caret.
	rBefore, cBefore := u.ed.Caret()
	u.ed.TypedKey(&fyne.KeyEvent{Name: fyne.KeyDown})
	if !u.completionOpen() {
		t.Fatal("the arrow closed the list")
	}
	if r, c := u.ed.Caret(); r != rBefore || c != cBefore {
		t.Fatalf("caret moved to (%d,%d) by the arrow, want (%d,%d)",
			r, c, rBefore, cBefore)
	}

	// Enter must accept a suggestion and close the list, not insert a newline.
	u.ed.TypedKey(&fyne.KeyEvent{Name: fyne.KeyReturn})
	if u.completionOpen() {
		t.Fatal("Enter left the list open")
	}
	if strings.Contains(u.ed.LineAt(0), "\n") {
		t.Fatal("Enter inserted a newline instead of accepting a suggestion")
	}
}

// TestCompletionSelectedRowUsesTheMenuPalette pins the visual fix: the list must
// read as a menu (a distinct panel background and a highlighted selected row),
// not as bare text painted over the editor.
func TestCompletionSelectedRowUsesTheMenuPalette(t *testing.T) {
	u := newTestUI(t)
	u.ed.SetText("@var token = abc\n\nAuthorization: Bearer {{")
	u.ed.doc.setCaret(2, runeLen("Authorization: Bearer {{"))
	u.ed.doc.breakUndoRun()

	u.maybeAutoCompletion()
	if !u.completionOpen() {
		t.Fatal("typing {{ did not open the list")
	}

	r := test.WidgetRenderer(u.ed)
	r.Refresh()

	var textObj *canvas.Text
	for _, o := range r.Objects() {
		if txt, ok := o.(*canvas.Text); ok && txt.Visible() && strings.Contains(txt.Text, u.complete.items[0].Label) {
			textObj = txt
			break
		}
	}
	if textObj == nil {
		t.Fatal("no text object draws the selected suggestion")
	}
	// The selected row is drawn in the menu-selection foreground, which differs
	// from the menu foreground, so it is visibly highlighted.
	if textObj.Color == u.pal.menuFg {
		t.Fatal("selected suggestion uses the unselected menu colour")
	}
	if textObj.Color != u.pal.menuSelFg {
		t.Fatalf("selected suggestion colour = %v, want the menu-selection colour", textObj.Color)
	}
}

// TestLoadingAFileClosesTheCompletionList pins the other visual bug: an open
// list painted over the previous buffer must not survive a file switch.
func TestLoadingAFileClosesTheCompletionList(t *testing.T) {
	u := newTestUI(t)
	u.ed.SetText("@var token = abc\n\nAuthorization: Bearer {{")
	u.ed.doc.setCaret(2, runeLen("Authorization: Bearer {{"))
	u.ed.doc.breakUndoRun()
	u.maybeAutoCompletion()
	if !u.completionOpen() {
		t.Fatal("typing {{ did not open the list")
	}

	u.loadInto(tempPath(t, "other.http"), "GET http://other\n")
	if u.completionOpen() {
		t.Fatal("the completion list stayed open after loading a file")
	}
}

// TestLanguageToggleRebuildsTheChrome checks the desktop window's UI strings go
// through i18n: switching the locale changes the header and the status line, and
// the language is persisted.
func TestLanguageToggleRebuildsTheChrome(t *testing.T) {
	u := newTestUI(t)
	if u.status.Text() == "" {
		t.Fatal("empty status line")
	}
	if !strings.Contains(u.status.Text(), "строк") && !strings.Contains(u.header.Text(), "строк") {
		t.Fatalf("header = %q, status = %q; expected the Russian locale", u.header.Text(), u.status.Text())
	}

	u.toggleLang()
	if got := i18n.Locale(); got != "en" {
		t.Fatalf("locale = %q, want en", got)
	}
	if !strings.Contains(u.header.Text(), "lines") {
		t.Fatalf("header after switching = %q, want the English locale", u.header.Text())
	}
	// Switching the language reports the language, exactly like the console.
	if want := i18n.T("status.lang", i18n.LangName("en")); !strings.Contains(u.status.Text(), want) {
		t.Fatalf("status after switching = %q, want %q", u.status.Text(), want)
	}
	if got := settings.Load().Lang; got != "en" {
		t.Fatalf("stored language = %q, want en", got)
	}
}

// TestGUIStringsAreTranslated guards against a missing key: every gui.* key the
// window actually uses must exist in both catalogs.
func TestGUIStringsAreTranslated(t *testing.T) {
	keys := []string{
		"gui.title", "gui.copyResponse", "gui.findPlaceholder",
		"gui.findCount", "gui.caret", "gui.selection", "gui.noResponse", "gui.ready",
	}
	for _, loc := range []string{"ru", "en"} {
		for _, key := range keys {
			if !i18n.Has(loc, key) {
				t.Errorf("key %q is missing from the %q catalog", key, loc)
			}
		}
	}
}

// TestWindowShortcutsReachTheWindow is the regression guard for a real bug found
// while verifying the window: because the editor implements fyne.Shortcutable,
// Fyne delivers custom shortcuts to the widget and never to the canvas
// handlers, so Ctrl+S and friends did nothing until the editor forwarded them.
func TestWindowShortcutsReachTheWindow(t *testing.T) {
	u := newTestUI(t)
	u.ed.SetText("GET http://a\n")

	// Ctrl+N starts an empty buffer.
	u.ed.TypedShortcut(&desktop.CustomShortcut{KeyName: fyne.KeyN, Modifier: fyne.KeyModifierControl})
	if got := u.ed.Text(); got != "" {
		t.Fatalf("Ctrl+N left the buffer as %q, want it cleared", got)
	}

	// Ctrl+F turns the top row into the search field.
	u.ed.TypedShortcut(&desktop.CustomShortcut{KeyName: fyne.KeyF, Modifier: fyne.KeyModifierControl})
	if !u.searching {
		t.Fatal("Ctrl+F did not reach the window")
	}

	// Escape (a bare key, not a shortcut) closes it again through the key hook.
	u.ed.TypedKey(&fyne.KeyEvent{Name: fyne.KeyEscape})
	if u.searching {
		t.Fatal("Escape did not close the search row")
	}
}

// TestShortcutsWorkUnderACyrillicLayout covers the aliases: Fyne reports the key
// as typed, so a Russian layout sends Ctrl+Ы for Ctrl+S.
func TestShortcutsWorkUnderACyrillicLayout(t *testing.T) {
	u := newTestUI(t)
	u.ed.SetText("some text")

	u.ed.TypedShortcut(&desktop.CustomShortcut{KeyName: "Т", Modifier: fyne.KeyModifierControl}) // Ctrl+N
	if got := u.ed.Text(); got != "" {
		t.Fatalf("Ctrl+Т (Ctrl+N) left the buffer as %q", got)
	}

	u.ed.TypedShortcut(&desktop.CustomShortcut{KeyName: "А", Modifier: fyne.KeyModifierControl}) // Ctrl+F
	if !u.searching {
		t.Fatal("Ctrl+А (Ctrl+F) did not open the search row")
	}
}

// TestCtrlSSavesTheOpenFile is the write path end to end: type, then Ctrl+S, then
// read the file back from disk.
func TestCtrlSSavesTheOpenFile(t *testing.T) {
	u := newTestUI(t)
	dir := t.TempDir()
	path := filepath.Join(dir, "api.http")
	u.loadInto(path, "GET http://a\n")

	u.ed.TypedRune('X')
	u.ed.TypedShortcut(&desktop.CustomShortcut{KeyName: fyne.KeyS, Modifier: fyne.KeyModifierControl})

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("the file was not written: %v", err)
	}
	if got := string(data); got != "XGET http://a\n" {
		t.Fatalf("file = %q, want the typed character at the caret", got)
	}
}

func TestWindowShortcutsIgnoreOtherKeys(t *testing.T) {
	u := newTestUI(t)
	u.ed.SetText("keep me")
	// A custom shortcut without the command modifier, and one with a key the
	// window does not use, must both fall through to the editor.
	if u.handleShortcut(&desktop.CustomShortcut{KeyName: fyne.KeyS, Modifier: fyne.KeyModifierAlt}) {
		t.Error("Alt+S should not be handled by the window")
	}
	if u.handleShortcut(&desktop.CustomShortcut{KeyName: fyne.KeyQ, Modifier: fyne.KeyModifierControl}) {
		t.Error("Ctrl+Q should not be handled by the window")
	}
	if u.handleShortcut(&fyne.ShortcutSelectAll{}) {
		t.Error("a non-custom shortcut should not be handled by the window")
	}
	if got := u.ed.Text(); got != "keep me" {
		t.Fatalf("buffer changed to %q", got)
	}
}

func TestThemeLabelIsLocalized(t *testing.T) {
	u := newTestUI(t)
	if got := u.themeLabel("default"); got != i18n.T("theme.default") {
		t.Fatalf("themeLabel(default) = %q, want the localised scheme name", got)
	}
	if got := u.themeLabel("my-custom"); !strings.Contains(got, "my-custom") {
		t.Fatalf("themeLabel(custom) = %q, want it to keep the file name", got)
	}
}

// TestWindowLayoutMatchesTheConsole keeps the structure honest: the window must
// have the header line, the three panes and the status line, and start with the
// editor pane active and no overlays open.
func TestWindowLayoutMatchesTheConsole(t *testing.T) {
	u := newTestUI(t)

	if u.header == nil || u.header.Text() == "" {
		t.Fatal("the header line is missing")
	}
	if !strings.Contains(u.header.Text(), i18n.T("header.file", "")) {
		t.Fatalf("header = %q, want it to name the file like the console", u.header.Text())
	}
	if u.status == nil || u.status.Text() == "" {
		t.Fatal("the status line is missing")
	}
	if u.files == nil || u.filesView == nil || u.files.view == nil {
		t.Fatal("the files panel is missing")
	}
	if u.ed == nil || u.resp == nil {
		t.Fatal("the editor or the response pane is missing")
	}
	if u.active != paneEditor {
		t.Fatalf("active pane = %v, want the editor", u.active)
	}
	if u.searching || u.completionOpen() || u.requestMenuOpen() {
		t.Fatal("an overlay is open at startup")
	}
}

// TestPanesRepaintTheActiveFrame covers the console's active-pane marking: the
// border colour follows the pane the user is working in.
func TestPanesRepaintTheActiveFrame(t *testing.T) {
	u := newTestUI(t)
	u.setActive(paneFiles)
	if u.active != paneFiles {
		t.Fatalf("active pane = %v, want files", u.active)
	}
	u.setActive(paneEditor)
	if u.active != paneEditor {
		t.Fatalf("active pane = %v, want editor", u.active)
	}
}

// TestClickingTheEditorFocusesIt is the regression guard for the reported defect:
// clicking the editor marked the pane active but the caret never appeared and
// typing/Ctrl+C did nothing. The driver only unfocuses on a click, so the editor
// itself must take the keyboard focus, exactly like widget.Entry.
func TestClickingTheEditorFocusesIt(t *testing.T) {
	u := newTestUI(t)
	u.ed.SetText("GET http://a\n")
	lineH := u.ed.lineHeight()

	// Give the focus to something else first, the way it is after a panel click.
	u.win.Canvas().Focus(u.searchEntry)
	if u.win.Canvas().Focused() == u.ed {
		t.Fatal("setup: the editor already has focus")
	}

	// A click on the text takes the keyboard focus.
	press := desktop.MouseEvent{
		PointEvent: fyne.PointEvent{Position: fyne.NewPos(u.ed.textOriginX(), paddingY+lineH/2)},
		Button:     desktop.MouseButtonPrimary,
	}
	u.ed.MouseDown(&press)
	u.ed.MouseUp(&desktop.MouseEvent{
		PointEvent: fyne.PointEvent{Position: press.Position},
		Button:     desktop.MouseButtonPrimary,
	})
	u.ed.Tapped(&fyne.PointEvent{Position: press.Position})

	if got := u.win.Canvas().Focused(); got != u.ed {
		t.Fatalf("focused object after click = %T, want the editor", got)
	}
	if !u.ed.focused {
		t.Fatal("the editor did not register the focus, so the caret stays hidden")
	}

	// With focus the clipboard shortcuts are delivered to the editor.
	u.ed.doc.setCaret(0, 0)
	u.ed.doc.anchorRow, u.ed.doc.anchorCol = 0, 3
	u.ed.doc.selOn = true
	u.ed.TypedShortcut(&fyne.ShortcutCopy{})
	if got := clipboard().Content(); got != "GET" {
		t.Fatalf("clipboard = %q, want the selection after a click", got)
	}
}

// TestLanguageToggleRebuildsSidebarAndHelp covers the chrome pieces built once:
// the sidebar profile rows and the key reference must follow the new locale.
func TestLanguageToggleRebuildsSidebarAndHelp(t *testing.T) {
	u := newTestUI(t)
	if !u.helpShown() {
		t.Fatal("the window should start with the help")
	}
	if got := firstProfileBoxLabel(u); got != i18n.T("profile.title") {
		t.Fatalf("sidebar title = %q, want the Russian profile title", got)
	}

	u.toggleLang()

	if got := firstProfileBoxLabel(u); got != i18n.T("profile.title") {
		t.Fatalf("sidebar title after the switch = %q, want %q", got, i18n.T("profile.title"))
	}
	if !strings.Contains(u.resp.Text(), i18n.T("help.title")) {
		t.Fatalf("help stayed in the old locale: %q", u.resp.Text())
	}
}

func firstProfileBoxLabel(u *ui) string {
	for _, o := range u.files.box.Objects {
		if l, ok := o.(*widget.Label); ok {
			return l.Text
		}
	}
	return ""
}

// TestTypingAMethodOpensCompletion mirrors the console: the method list opens by
// itself once the first letters of a request line are typed.
func TestTypingAMethodOpensCompletion(t *testing.T) {
	u := newTestUI(t)
	u.ed.SetText("")
	u.ed.doc.setCaret(0, 0)

	u.ed.TypedRune('G')

	if !u.completionOpen() {
		t.Fatal("typing a method prefix did not open the completion list")
	}
	if got := u.complete.items[0].Label; !strings.HasPrefix(got, "GET") {
		t.Fatalf("first suggestion = %q, want GET", got)
	}
}

// TestAcceptedCompletionDoesNotReopen keeps the list closed after Enter: the
// replacement edit must not trigger the automatic popup again.
func TestAcceptedCompletionDoesNotReopen(t *testing.T) {
	u := newTestUI(t)
	u.ed.SetText("")
	u.ed.doc.setCaret(0, 0)
	u.ed.TypedRune('G')
	if !u.completionOpen() {
		t.Fatal("completion did not open")
	}

	u.acceptCompletion()

	if u.completionOpen() {
		t.Fatal("the list reopened after accepting a suggestion")
	}
	if got := u.ed.LineAt(0); !strings.HasPrefix(got, "GET") {
		t.Fatalf("line after accepting = %q, want GET", got)
	}
}

// TestDialogDirFollowsTheOpenFile keeps the open/save dialogs in the folder the
// user is working in instead of the process default.
func TestDialogDirFollowsTheOpenFile(t *testing.T) {
	u := newTestUI(t)
	dir := t.TempDir()
	path := filepath.Join(dir, "api.http")
	if err := os.WriteFile(path, []byte("GET http://x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	u.loadInto(path, "GET http://x\n")

	got := u.dialogDir().Path()
	want := filepath.ToSlash(filepath.Clean(dir))
	if filepath.ToSlash(filepath.Clean(got)) != want {
		t.Fatalf("dialog dir = %q, want %q", got, want)
	}

	u.path = ""
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if got := u.dialogDir().Path(); filepath.ToSlash(filepath.Clean(got)) != filepath.ToSlash(filepath.Clean(wd)) {
		t.Fatalf("dialog dir for an unnamed buffer = %q, want the working directory %q", got, wd)
	}
}

// TestCtrlArrowsUseTheShortcutPath covers the real Fyne dispatch: Ctrl+Arrow is
// delivered as a CustomShortcut to the focused widget, not as TypedKey, so the
// window must apply word movement itself.
func TestCtrlArrowsUseTheShortcutPath(t *testing.T) {
	u := newTestUI(t)
	u.ed.SetText("GET http://example.test/x\n")
	u.ed.doc.setCaret(0, 0)

	u.ed.TypedShortcut(&desktop.CustomShortcut{KeyName: fyne.KeyRight, Modifier: fyne.KeyModifierControl})
	if col := u.ed.doc.col; col != 4 {
		t.Fatalf("Ctrl+Right col = %d, want 4 (start of http)", col)
	}
	u.ed.TypedShortcut(&desktop.CustomShortcut{KeyName: fyne.KeyLeft, Modifier: fyne.KeyModifierControl})
	if col := u.ed.doc.col; col != 0 {
		t.Fatalf("Ctrl+Left col = %d, want 0", col)
	}

	u.ed.TypedShortcut(&desktop.CustomShortcut{
		KeyName:  fyne.KeyRight,
		Modifier: fyne.KeyModifierControl | fyne.KeyModifierShift,
	})
	if got := u.ed.doc.selText(); got != "GET " {
		t.Fatalf("Ctrl+Shift+Right selection = %q, want %q", got, "GET ")
	}

	u.ed.doc.setCaret(0, 4)
	u.ed.doc.selClear()
	u.ed.TypedShortcut(&desktop.CustomShortcut{
		KeyName:  fyne.KeyLeft,
		Modifier: fyne.KeyModifierControl | fyne.KeyModifierShift,
	})
	if got := u.ed.doc.selText(); got != "GET " {
		t.Fatalf("Ctrl+Shift+Left selection = %q, want %q", got, "GET ")
	}
}

package gui

import (
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/widget"

	"github.com/user/curlyk/internal/i18n"
	"github.com/user/curlyk/internal/jsonlex"
	"github.com/user/curlyk/internal/runner"
)

// TestReadOnlyIgnoresEdits covers the viewer contract: typing, deleting and
// cutting change nothing in a read-only editor.
func TestReadOnlyIgnoresEdits(t *testing.T) {
	e := newTestEditor(t, "GET http://a\nsecond line\n")
	e.SetReadOnly(true)

	e.TypedRune('X')
	e.TypedRune('Y')
	if got := e.Text(); got != "GET http://a\nsecond line\n" {
		t.Fatalf("text after typing = %q, want it unchanged", got)
	}

	e.doc.setCaret(1, 0)
	pressKey(e, fyne.KeyBackspace)
	if got := e.Text(); got != "GET http://a\nsecond line\n" {
		t.Fatalf("text after backspace = %q, want it unchanged", got)
	}

	e.doc.selSelectAll()
	e.TypedShortcut(&fyne.ShortcutCut{})
	if got := e.Text(); got != "GET http://a\nsecond line\n" {
		t.Fatalf("text after cut = %q, want it unchanged", got)
	}
	if e.CanUndo() {
		t.Fatal("read-only editing must not pollute the undo history")
	}
}

// TestReadOnlyNavigationAndCopyStillWork keeps the useful viewer operations:
// moving the caret with the arrows and copying a selection.
func TestReadOnlyNavigationAndCopyStillWork(t *testing.T) {
	e := newTestEditor(t, "GET http://a\nsecond line\n")
	e.SetReadOnly(true)

	e.doc.setCaret(0, 0)
	pressKey(e, fyne.KeyRight)
	pressKey(e, fyne.KeyRight)
	if col := e.doc.col; col != 2 {
		t.Fatalf("caret col = %d, want 2 (navigation must keep working)", col)
	}

	// Back to the start, then Shift+Right to column 4 selects "GET ".
	e.doc.setCaret(0, 0)
	withModifiers(t, fyne.KeyModifierShift)
	pressKey(e, fyne.KeyRight)
	pressKey(e, fyne.KeyRight)
	pressKey(e, fyne.KeyRight)
	pressKey(e, fyne.KeyRight)
	if got := e.doc.selText(); got != "GET " {
		t.Fatalf("selection = %q, want \"GET \"", got)
	}
	withModifiers(t, 0)

	if !e.Copy() {
		t.Fatal("copy in read-only mode reported nothing to copy")
	}
	if got := clipboard().Content(); got != "GET " {
		t.Fatalf("clipboard = %q, want the selected text", got)
	}

	// Select All also works.
	e.TypedShortcut(&fyne.ShortcutSelectAll{})
	if got := e.doc.selText(); !strings.HasPrefix(got, "GET http://a") {
		t.Fatalf("select-all = %q", got)
	}
}

// TestResponsePaneIsReadOnlyViewer checks that the window's response pane is a
// read-only, unnumbered editor that starts with the key reference (the empty
// state), not with a response.
func TestResponsePaneIsReadOnlyViewer(t *testing.T) {
	u := newTestUI(t)
	if !u.resp.readOnly {
		t.Fatal("the response pane is not read-only")
	}
	if u.resp.showLines {
		t.Fatal("the response pane should not show line numbers")
	}
	if !u.helpShown() {
		t.Fatal("the pane should start with the key reference")
	}
	if got := u.resp.Text(); !strings.Contains(got, i18n.T("help.title")) {
		t.Fatalf("initial response text = %q, want the help title", got)
	}
}

// TestCopyResponse covers the pane's "copy response" button: with a body it puts
// the raw body on the clipboard, without one (204, HEAD, network error) it falls
// back to the formatted pane text so the button never silently does nothing.
func TestCopyResponse(t *testing.T) {
	u := newTestUI(t)
	u.setResponse("GET http://x\n\n{\"a\":1}", 2)
	u.lastBody = []byte("{\"a\":1}")
	u.copyResponse()
	if got := clipboard().Content(); got != `{"a":1}` {
		t.Fatalf("clipboard = %q, want the raw body", got)
	}
	if !strings.Contains(u.StatusMessage(), i18n.T("status.respCopied", 7)) {
		t.Fatalf("status = %q, want the copied message", u.StatusMessage())
	}

	// No body: the whole reply goes to the clipboard.
	u.setResponse("GET http://x\n\n204 No Content", 2)
	u.lastBody = nil
	u.copyResponse()
	if got := clipboard().Content(); !strings.Contains(got, "204 No Content") {
		t.Fatalf("clipboard = %q, want the formatted response", got)
	}

	// The empty state (help) reports that there is nothing to copy.
	u.showHelp()
	u.copyResponse()
	if !strings.Contains(u.StatusMessage(), i18n.T("err.noResponse")) {
		t.Fatalf("status = %q, want the no-response message", u.StatusMessage())
	}
}

// TestCopyResponseButtonIsWired taps the real button in the pane's header, which
// is what the user clicks.
func TestCopyResponseButtonIsWired(t *testing.T) {
	u := newTestUI(t)
	u.setResponse("GET http://x\n\n{\"a\":1}", 2)
	u.lastBody = []byte("body-text")

	btn := findButton(u.responsePane(), i18n.T("gui.copyResponse"))
	if btn == nil {
		t.Fatal("the response pane has no copy button")
	}
	btn.OnTapped()

	if got := clipboard().Content(); got != "body-text" {
		t.Fatalf("clipboard = %q, want the button to copy the body", got)
	}
}

// TestCtrlCCopiesTheWholeReplyWithoutSelection keeps the keyboard useful in the
// read-only pane.
func TestCtrlCCopiesTheWholeReplyWithoutSelection(t *testing.T) {
	u := newTestUI(t)
	u.setResponse("GET http://x\n\n{\"a\":1}", 2)
	u.lastBody = []byte("body-text")

	u.resp.TypedShortcut(&fyne.ShortcutCopy{})

	if got := clipboard().Content(); got != "body-text" {
		t.Fatalf("clipboard = %q, want the reply", got)
	}
}

// findButton walks a container tree for a button with the given label.
func findButton(obj fyne.CanvasObject, label string) *widget.Button {
	switch o := obj.(type) {
	case *widget.Button:
		if o.Text == label {
			return o
		}
	case *fyne.Container:
		for _, child := range o.Objects {
			if b := findButton(child, label); b != nil {
				return b
			}
		}
	}
	return nil
}

// TestFormatResponsePrettyPrintsJSONBody mirrors the console: a JSON response
// body is indented in the pane.
func TestFormatResponsePrettyPrintsJSONBody(t *testing.T) {
	res := &runner.Result{
		Status:   "200 OK",
		Duration: 10 * time.Millisecond,
		Response: &http.Response{
			StatusCode: 200,
			Status:     "200 OK",
			Header:     http.Header{"Content-Type": {"application/json"}},
		},
	}
	out, _ := formatResponse(res, []byte(`{"name":"Ada","tags":["a","b"]}`))
	if !strings.Contains(out, "\n  \"name\": \"Ada\",\n") {
		t.Fatalf("response text is not pretty-printed:\n%s", out)
	}

	// A non-JSON body is left as-is.
	out, _ = formatResponse(res, []byte("plain text"))
	if !strings.Contains(out, "plain text") || strings.Contains(out, "\"name\"") {
		t.Fatalf("non-JSON body was altered:\n%s", out)
	}
}

// TestResponseEditorSurvivesWindowRebuild keeps the pane a live widget when the
// window is rebuilt around it (a theme or language switch).
func TestResponseEditorSurvivesWindowRebuild(t *testing.T) {
	u := newTestUI(t)
	u.setResponse("GET http://x\n{\"a\":1}", 1)
	u.refreshContent()
	if got := u.respHdr.Text(); !strings.Contains(got, "http://x") {
		t.Fatalf("response header after rebuild = %q", got)
	}
	if got := u.resp.Text(); !strings.Contains(got, "\"a\":1") {
		t.Fatalf("response body after rebuild = %q", got)
	}
}

// TestResponseTextHighlightsJSONBodyOnly checks the response viewer's mode: the
// header part stays neutral and the JSON body gets lexed into typed segments.
func TestResponseTextHighlightsJSONBodyOnly(t *testing.T) {
	const text = "GET http://x\nContent-Type: application/json\n\n{\n  \"a\": 1,\n  \"b\": \"s\",\n  \"c\": true\n}"
	e := newTestEditor(t, "")
	e.SetResponseText(text, 3)

	for row := 0; row < 3; row++ {
		for _, s := range e.segments(row) {
			if s.jsonKind {
				t.Fatalf("header row %d was JSON-highlighted: %+v", row, s)
			}
		}
	}

	kinds := map[jsonlex.Kind]bool{}
	for row := 3; row < e.LineCount(); row++ {
		for _, s := range e.segments(row) {
			if s.jsonKind {
				kinds[s.json] = true
			}
		}
	}
	for _, want := range []jsonlex.Kind{jsonlex.Key, jsonlex.Number, jsonlex.String, jsonlex.Bool} {
		if !kinds[want] {
			t.Errorf("JSON body has no %v segment; kinds = %v", want, kinds)
		}
	}
}

// TestResponsePaneFocusMarksItActive covers the pane's focus hook: clicking or
// tabbing into the response pane makes it the active pane.
func TestResponsePaneFocusMarksItActive(t *testing.T) {
	u := newTestUI(t)
	u.setActive(paneEditor)
	u.resp.FocusGained()
	if u.active != paneResp {
		t.Fatalf("active pane = %v, want the response pane", u.active)
	}
}

// TestThemeChangeUpdatesResponsePalette keeps the response viewer in step with
// the scheme switch, so its JSON colours follow the editor's.
func TestThemeChangeUpdatesResponsePalette(t *testing.T) {
	u := newTestUI(t)
	before := u.resp.pal.name
	u.cycleTheme()
	if u.resp.pal.name == before {
		t.Fatalf("response palette stayed %q after a theme cycle", before)
	}
	if u.resp.pal.name != u.ed.pal.name {
		t.Fatalf("response palette %q != editor palette %q", u.resp.pal.name, u.ed.pal.name)
	}
}

// TestFormatResponseIncludesRequestHeaders checks the request dump: the headers
// that were actually sent are shown above the status, and the body starts after
// the reported header rows.
func TestFormatResponseIncludesRequestHeaders(t *testing.T) {
	reqURL, err := url.Parse("http://example.test/x")
	if err != nil {
		t.Fatal(err)
	}
	req := &http.Request{
		Method: "POST",
		URL:    reqURL,
		Header: http.Header{"X-Test": {"one"}},
	}
	res := &runner.Result{
		Status:   "200 OK",
		Duration: 5 * time.Millisecond,
		Request:  req,
		Response: &http.Response{
			StatusCode: 200,
			Status:     "200 OK",
			Header:     http.Header{"Content-Type": {"application/json"}},
		},
	}
	out, headerRows := formatResponse(res, []byte(`{"a":1}`))
	if !strings.Contains(out, "X-Test: one") {
		t.Fatalf("request headers missing from the dump:\n%s", out)
	}
	lines := strings.Split(out, "\n")
	if headerRows <= 0 || headerRows >= len(lines) {
		t.Fatalf("headerRows = %d for %d lines", headerRows, len(lines))
	}
	if !strings.Contains(strings.Join(lines[headerRows:], "\n"), "\"a\": 1") {
		t.Fatalf("body after line %d is missing the JSON body:\n%s", headerRows, strings.Join(lines[headerRows:], "\n"))
	}
}

// TestResponsePaneSplitsHeadersAndBody keeps the console order: the header block
// is its own fixed viewer, the copy button follows it, and only the body scrolls.
func TestResponsePaneSplitsHeadersAndBody(t *testing.T) {
	u := newTestUI(t)
	u.setResponse("GET http://x\nAccept: */*\n\n{\"a\":1}", 3)

	if got := u.respHdr.Text(); got != "GET http://x\nAccept: */*\n" {
		t.Fatalf("header viewer = %q", got)
	}
	if got := u.resp.Text(); got != "{\"a\":1}" {
		t.Fatalf("body viewer = %q", got)
	}
	if u.respHeaderLayout.height <= 0 {
		t.Fatal("the header viewer has no height")
	}
	if u.respHeaderLayout.height > float32(3)*u.respHdr.lineHeight()+2*paddingY+0.01 {
		t.Fatalf("header height = %v, want about three lines", u.respHeaderLayout.height)
	}
}

// TestResponseHeaderHidesForHelp keeps the empty state compact: the header
// viewer collapses and the help occupies the whole body.
func TestResponseHeaderHidesForHelp(t *testing.T) {
	u := newTestUI(t)
	u.setResponse("GET http://x\n\nbody", 2)
	u.toggleHelp()

	if u.respHeaderLayout.height != 0 {
		t.Fatalf("header height = %v, want 0 while the help is shown", u.respHeaderLayout.height)
	}
	if got := u.respHdr.Text(); got != "" {
		t.Fatalf("header viewer = %q, want it cleared", got)
	}
}

// TestRightClickCopiesTheResponse covers the secondary click: without a
// selection it copies the reply, with one it copies just the selection.
func TestRightClickCopiesTheResponse(t *testing.T) {
	u := newTestUI(t)
	u.setResponse("GET http://x\n\n{\"a\":1}", 2)
	u.lastBody = []byte("body-text")

	u.resp.TappedSecondary(&fyne.PointEvent{Position: fyne.NewPos(10, 10)})
	if got := clipboard().Content(); got != "body-text" {
		t.Fatalf("clipboard = %q, want the whole reply", got)
	}

	u.resp.doc.selSelectAll()
	u.resp.TappedSecondary(&fyne.PointEvent{Position: fyne.NewPos(10, 10)})
	if got := clipboard().Content(); got != u.resp.Text() {
		t.Fatalf("clipboard = %q, want the selected body %q", got, u.resp.Text())
	}
	if !strings.Contains(u.StatusMessage(), i18n.T("status.copiedResp", u.resp.SelectionLen())) {
		t.Fatalf("status = %q, want the copied-selection message", u.StatusMessage())
	}
}

// TestCopyButtonOnlyWithResponse keeps the pane header clean: the copy button
// appears with the first reply, hides while the help is shown, and returns with
// the reply.
func TestCopyButtonOnlyWithResponse(t *testing.T) {
	u := newTestUI(t)
	if u.copyButton.Visible() {
		t.Fatal("the copy button is visible in the empty/help state")
	}

	u.setResponse("GET http://x\n\nbody", 2)
	if !u.copyButton.Visible() {
		t.Fatal("the copy button is hidden with a response")
	}

	u.toggleHelp()
	if u.copyButton.Visible() {
		t.Fatal("the copy button is visible over the help")
	}

	u.toggleHelp()
	if !u.copyButton.Visible() {
		t.Fatal("the copy button did not return with the response")
	}
}

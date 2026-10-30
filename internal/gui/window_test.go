package gui

import (
	"image/color"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/widget"

	"github.com/user/curlyk/internal/i18n"
	"github.com/user/curlyk/internal/settings"
)

// TestFilesPanelListsAndOpensFiles covers the sidebar: it lists the working
// directory's .http files (the same set the console shows) and opening one loads
// it into the buffer.
func TestFilesPanelListsAndOpensFiles(t *testing.T) {
	u := newTestUI(t)
	if err := os.WriteFile("b.http", []byte("GET http://b\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile("a.http", []byte("GET http://a\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile("notes.txt", []byte("ignored"), 0o644); err != nil {
		t.Fatal(err)
	}

	u.reloadFiles()
	if got := u.files.files; len(got) != 2 || got[0] != "a.http" || got[1] != "b.http" {
		t.Fatalf("file list = %v, want [a.http b.http]", got)
	}

	u.openNamed("a.http")
	if !strings.HasSuffix(u.path, "a.http") {
		t.Fatalf("path = %q, want it to point at a.http", u.path)
	}
	if got := u.ed.Text(); got != "GET http://a\n" {
		t.Fatalf("buffer = %q", got)
	}
	if !strings.Contains(u.StatusMessage(), "a.http") {
		t.Fatalf("status = %q, want the opened file named", u.StatusMessage())
	}
}

func TestFilesPanelNewFile(t *testing.T) {
	u := newTestUI(t)
	u.ed.SetText("GET http://a\n")

	u.newFile()
	if got := u.ed.Text(); got != "" {
		t.Fatalf("buffer = %q, want it cleared", got)
	}
	if u.path != "" {
		t.Fatalf("path = %q, want an unnamed buffer", u.path)
	}
	if got := u.StatusMessage(); got != i18n.T("status.newFile") {
		t.Fatalf("status = %q, want %q", got, i18n.T("status.newFile"))
	}
}

// TestRightClickOpensTheRequestMenu covers the context menu the console offers
// on a request line.
func TestRightClickOpensTheRequestMenu(t *testing.T) {
	u := newTestUI(t)
	u.ed.SetText("### r\nGET http://a\nAccept: application/json\n")

	// Right click on the request line (row 1).
	d := u.ed.rowDisplay(1)
	at := fyne.NewPos(u.ed.textOriginX()+d.x(2), paddingY+u.ed.lineHeight())
	u.ed.TappedSecondary(&fyne.PointEvent{Position: at})

	if !u.requestMenuOpen() {
		t.Fatal("right click on a request line did not open the menu")
	}

	// Escape closes it again, before anything else.
	u.escape()
	if u.requestMenuOpen() {
		t.Fatal("Escape did not close the request menu")
	}
}

func TestRightClickOutsideARequestReportsIt(t *testing.T) {
	u := newTestUI(t)
	u.ed.SetText("# just a comment\n")

	d := u.ed.rowDisplay(0)
	u.ed.TappedSecondary(&fyne.PointEvent{Position: fyne.NewPos(u.ed.textOriginX()+d.x(1), paddingY)})

	if u.requestMenuOpen() {
		t.Fatal("a comment line should not open the request menu")
	}
	if !strings.Contains(u.StatusMessage(), i18n.T("err.noRequestCursor")) {
		t.Fatalf("status = %q, want the no-request message", u.StatusMessage())
	}
}

// TestRequestMenuFormatsJSONBody runs the menu's JSON action: the body is
// pretty-printed in the buffer.
func TestRequestMenuFormatsJSONBody(t *testing.T) {
	u := newTestUI(t)
	u.ed.SetText("POST http://a\nContent-Type: application/json\n\n{\"a\":1,\"b\":[2,3]}\n")

	u.formatRequestJSON(3)
	if got := u.ed.Text(); !strings.Contains(got, "\n  \"a\": 1,\n") {
		t.Fatalf("buffer after formatting =\n%s", got)
	}
	if got := u.StatusMessage(); got != i18n.T("status.jsonFormatted") {
		t.Fatalf("status = %q, want %q", got, i18n.T("status.jsonFormatted"))
	}
	if !u.ed.CanUndo() {
		t.Fatal("formatting should be undoable")
	}
}

func TestRequestMenuFormatsRejectsNonJSON(t *testing.T) {
	u := newTestUI(t)
	u.ed.SetText("POST http://a\nContent-Type: text/plain\n\nnot json\n")

	u.formatRequestJSON(3)
	if got := u.StatusMessage(); got != i18n.T("err.notJson") {
		t.Fatalf("status = %q, want %q", got, i18n.T("err.notJson"))
	}
	if got := u.ed.Text(); !strings.Contains(got, "not json") {
		t.Fatalf("buffer changed to %q", got)
	}
}

// TestRequestMenuCopiesCurl covers the menu's cURL action, including variable
// substitution.
func TestRequestMenuCopiesCurl(t *testing.T) {
	u := newTestUI(t)
	u.ed.SetText("@var host = http://example.com\n\n### r\nPOST {{host}}/v1\nAccept: application/json\n\n{\"a\":1}\n")
	u.ed.doc.setCaret(3, 0)

	u.copyAsCurl()
	cmd := clipboard().Content()
	if !strings.HasPrefix(cmd, "curl") {
		t.Fatalf("clipboard = %q, want a cURL command", cmd)
	}
	if !strings.Contains(cmd, "http://example.com/v1") {
		t.Fatalf("clipboard = %q, want the substituted URL", cmd)
	}
	if got := u.StatusMessage(); got != i18n.T("status.curlCopied") {
		t.Fatalf("status = %q, want %q", got, i18n.T("status.curlCopied"))
	}
}

// TestCompletionPopupIsReadable is the regression guard for the "tiny square"
// defect: the popup must be sized from its content, not left at a minimum Fyne
// cannot derive from a list of labels.
func TestCompletionPopupIsReadable(t *testing.T) {
	u := newTestUI(t)
	u.ed.SetText("@var token = abc\n\nGET https://api.example.com/x\nAuthorization: Bearer {{")
	u.ed.doc.setCaret(3, runeLen("Authorization: Bearer {{"))
	u.ed.doc.breakUndoRun()

	u.maybeAutoCompletion()
	if !u.completionOpen() {
		t.Fatal("typing {{ did not open the suggestion popup")
	}

	size := u.complete.popupSize()
	if size.Width < 60 {
		t.Fatalf("popup width = %v, want it wide enough to read a suggestion", size.Width)
	}
	_, lineH := u.ed.metrics()
	if want := float32(len(u.complete.items)) * lineH; size.Height < want {
		t.Fatalf("popup height = %v, want at least %v for %d rows", size.Height, want, len(u.complete.items))
	}
	if len(u.complete.items) == 0 {
		t.Fatal("no suggestions")
	}
}

// TestCompletionPopupFollowsTheCaret covers the other half of the defect: the
// popup opens below the caret and moves with it.
func TestCompletionPopupFollowsTheCaret(t *testing.T) {
	u := newTestUI(t)
	u.ed.SetText("GET https://api.example.com/x\n\nAc\n\nMore\n\nAuthorization: Bearer {{")
	u.ed.doc.setCaret(6, runeLen("Authorization: Bearer {{"))
	u.ed.doc.breakUndoRun()

	u.maybeAutoCompletion()
	if !u.completionOpen() {
		t.Fatal("the popup did not open")
	}
	first, _ := u.ed.completionBounds()

	// Move to an earlier line and refresh: the popup must follow the caret.
	u.ed.doc.setCaret(2, runeLen("Ac"))
	u.maybeAutoCompletion()
	if !u.completionOpen() {
		t.Fatal("the popup closed on refresh")
	}
	second, _ := u.ed.completionBounds()
	if first.Y == second.Y {
		t.Fatalf("popup y did not change with the caret (both %v)", first.Y)
	}
	if second.Y >= first.Y {
		t.Fatalf("popup y = %v on an earlier line, want less than %v", second.Y, first.Y)
	}
}

// TestCompletionPopupHiddenWithoutCandidates guards the opposite direction.
func TestCompletionPopupHiddenWithoutCandidates(t *testing.T) {
	u := newTestUI(t)
	u.ed.SetText("{ \"a\": 1 }")
	u.ed.doc.setCaret(0, 5)
	u.ed.doc.breakUndoRun()

	u.maybeAutoCompletion()
	if u.completionOpen() {
		t.Fatal("a popup opened with no candidates")
	}
}

// TestOpeningAFileFocusesTheEditor covers the behaviour after a panel click: the
// click leaves focus on the list, so without this the editor would not accept
// typing until the user clicked it again.
func TestOpeningAFileFocusesTheEditor(t *testing.T) {
	u := newTestUI(t)
	if err := os.WriteFile("a.http", []byte("GET http://a\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	u.reloadFiles()

	// Simulate the panel selection path.
	u.setActive(paneFiles)
	u.openNamed("a.http")

	if u.active != paneEditor {
		t.Fatalf("active pane = %v, want the editor after opening a file", u.active)
	}
	if focused := u.win.Canvas().Focused(); focused != u.ed {
		t.Fatalf("focused object is not the editor (%T)", focused)
	}

	u.ed.TypedRune('7')
	if got := u.ed.Text(); !strings.HasPrefix(got, "7") {
		t.Fatalf("buffer = %q, want the typed rune at the caret", got)
	}
}

// TestSwitchingFilesKeepsTheEditorPainted follows the reported symptom: after
// opening another file from the panel the editor showed nothing. Each opened file
// must be highlighted and painted straight away.
func TestSwitchingFilesKeepsTheEditorPainted(t *testing.T) {
	u := newTestUI(t)
	files := map[string]string{
		"a.http": "GET http://a\nAccept: application/json\n",
		"b.http": "POST http://b\nContent-Type: application/json\n\n{\"a\":1}\n",
	}
	for name, body := range files {
		if err := os.WriteFile(name, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	u.reloadFiles()

	for _, name := range []string{"a.http", "b.http", "a.http", "b.http"} {
		u.openNamed(name)
		if u.ed.LineCount() < 2 {
			t.Fatalf("%s: buffer has %d lines", name, u.ed.LineCount())
		}
		source := 0
		for row := 0; row < u.ed.LineCount(); row++ {
			// Blank lines have nothing to paint; every source line must be drawn.
			if strings.TrimSpace(u.ed.LineAt(row)) == "" {
				continue
			}
			source++
			if len(u.ed.rowPieces(row)) == 0 {
				t.Fatalf("%s: row %d is not painted after switching files", name, row)
			}
		}
		if source == 0 {
			t.Fatalf("%s: no source lines in the buffer", name)
		}
	}
}

// TestSwitchingFilesKeepsTheBufferText checks the text itself, so a future
// regression cannot hide behind an empty-but-consistent buffer.
func TestSwitchingFilesKeepsTheBufferText(t *testing.T) {
	u := newTestUI(t)
	if err := os.WriteFile("a.http", []byte("GET http://a\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile("b.http", []byte("POST http://b\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	u.reloadFiles()

	u.openNamed("a.http")
	if got := u.ed.Text(); !strings.Contains(got, "http://a") {
		t.Fatalf("buffer after opening a.http = %q", got)
	}
	u.openNamed("b.http")
	if got := u.ed.Text(); !strings.Contains(got, "http://b") {
		t.Fatalf("buffer after opening b.http = %q", got)
	}
	if !strings.HasSuffix(u.path, "b.http") {
		t.Fatalf("path = %q, want b.http", u.path)
	}
}

// TestFilesPanelPopulatedAtStartup is the guard for a real defect: the panel was
// built without loading the directory, so the window opened with an empty list
// and no profile section until the first file action.
func TestFilesPanelPopulatedAtStartup(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	settings.SetAppSettingsPath(filepath.Join(dir, "appsettings.yml"))
	i18n.SetLocale("ru")
	t.Cleanup(func() { i18n.SetLocale("ru") })

	if err := os.WriteFile("users.http", []byte("GET http://x\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	u := newUI(fyne.CurrentApp())

	if len(u.files.files) == 0 {
		t.Fatal("the file panel is empty at startup")
	}
	if got := u.files.rowLabel(1); got != "  users.http" {
		t.Fatalf("first file row = %q, want users.http", got)
	}
	// The profile section is filled as well, so the profile list is usable right
	// away.
	if len(u.files.box.Objects) == 0 {
		t.Fatal("the profile section is empty at startup")
	}
	if rows := profileRowLabels(u); len(rows) == 0 {
		t.Fatalf("no profile rows at startup (objects=%d)", len(u.files.box.Objects))
	}
}

// TestFilesPanelRows covers the sidebar rows: "+ Новый файл" first, then the
// files with the open one marked.
func TestFilesPanelRows(t *testing.T) {
	u := newTestUI(t)
	for _, name := range []string{"users.http", "b.http"} {
		if err := os.WriteFile(name, []byte("GET http://x\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	u.reloadFiles()

	if got := u.files.rowLabel(0); got != i18n.T("file.new") {
		t.Fatalf("row 0 = %q, want the new-file entry", got)
	}
	if got := u.files.rowLabel(1); got != "  b.http" {
		t.Fatalf("row 1 = %q, want b.http", got)
	}
	if got := u.files.rowLabel(2); got != "  users.http" {
		t.Fatalf("row 2 = %q, want users.http", got)
	}
	if got := u.files.rowLabel(3); got != "" {
		t.Fatalf("row 3 = %q, want nothing", got)
	}

	// The open file is marked like the console marks a selection.
	u.loadInto(filepath.Join(t.TempDir(), "users.http"), "GET http://x\n")
	if got := u.files.rowLabel(2); got != "▸ users.http" {
		t.Fatalf("row 2 with users.http open = %q, want it marked", got)
	}
	if got := u.files.rowLabel(1); got != "  b.http" {
		t.Fatalf("row 1 = %q, want it unmarked", got)
	}
}

// TestFilesPaneHasRoomForItsRows is the layout guard: the sidebar must be tall
// enough to show several rows, with the profile section below them. A pane that
// renders only its first row was a real defect, and it is invisible to the data
// tests above.
func TestFilesPaneHasRoomForItsRows(t *testing.T) {
	u := newTestUI(t)
	for _, name := range []string{"a.http", "b.http", "c.http"} {
		if err := os.WriteFile(name, []byte("GET http://x\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	u.reloadFiles()

	window := u.win.Content()
	window.Resize(fyne.NewSize(1200, 780))
	window.Refresh()

	listH := u.files.view.Size().Height
	if listH < 3*lineHeightOf(u) {
		t.Fatalf("file list height = %v, want room for at least three rows", listH)
	}
	if h := u.files.box.Size().Height; h <= 0 {
		t.Fatalf("profile section height = %v, want it laid out below the list", h)
	}
	if w := u.files.view.Size().Width; w < 80 {
		t.Fatalf("file list width = %v, want the pane to be wide enough for names", w)
	}
}

// lineHeightOf returns the editor's line height, the unit the sidebar rows use.
func lineHeightOf(u *ui) float32 { return u.ed.lineHeight() }

// TestRightClickKeepsEditorShortcutsWorking makes sure the new pointer handler
// did not break the tappable behaviour the editor already had.
func TestRightClickKeepsEditorShortcutsWorking(t *testing.T) {
	u := newTestUI(t)
	u.ed.SetText("GET http://a\n")
	u.ed.TypedShortcut(&desktop.CustomShortcut{KeyName: fyne.KeyN, Modifier: fyne.KeyModifierControl})
	if got := u.ed.Text(); got != "" {
		t.Fatalf("Ctrl+N left %q", got)
	}
}

// tempPath is a helper for tests that need a file outside the working directory.
func tempPath(t *testing.T, name string) string {
	t.Helper()
	return filepath.Join(t.TempDir(), name)
}

// TestEscapeClosesTheRequestMenu covers the overlay dismissal through the key
// hook: a popup does not take focus, so the editor receives Escape.
func TestEscapeClosesTheRequestMenu(t *testing.T) {
	u := newTestUI(t)
	u.ed.SetText("GET http://x\n")
	u.ed.doc.setCaret(0, 0)
	u.openRequestMenu(0, fyne.NewPos(10, 10))
	if !u.requestMenuOpen() {
		t.Fatal("the menu did not open")
	}

	u.ed.TypedKey(&fyne.KeyEvent{Name: fyne.KeyEscape})

	if u.requestMenuOpen() {
		t.Fatal("Escape did not close the request menu")
	}
}

// TestProfilesPanelIsTallAndSeparated covers the sidebar layout request: the
// profile section is taller than before and divided from the file list.
func TestProfilesPanelIsTallAndSeparated(t *testing.T) {
	u := newTestUI(t)
	if h := u.files.profileArea.MinSize().Height; h < 200 {
		t.Fatalf("profile area height = %v, want at least 200", h)
	}
	if !containsSeparator(u.filesView) {
		t.Fatal("the sidebar has no separator between files and profiles")
	}
}

func containsSeparator(obj fyne.CanvasObject) bool {
	switch o := obj.(type) {
	case *widget.Separator:
		return true
	case *fyne.Container:
		for _, child := range o.Objects {
			if containsSeparator(child) {
				return true
			}
		}
	}
	return false
}

// TestCompletionPopupStartsAtTheCaretColumn covers the placement defect: the
// list used to be pinned to the gutter and a full line too low. It must start at
// the caret column and immediately under the caret row.
func TestCompletionPopupStartsAtTheCaretColumn(t *testing.T) {
	u := newTestUI(t)
	u.ed.SetText("Ac")
	u.ed.doc.setCaret(0, 2)
	u.maybeAutoCompletion()
	if !u.completionOpen() {
		t.Fatal("the popup did not open")
	}

	pos, _ := u.ed.completionBounds()
	wantX := u.ed.caretOffsetX() - u.ed.scrollX
	if pos.X-wantX > 1 || wantX-pos.X > 1 {
		t.Fatalf("popup x = %v, want the caret column %v", pos.X, wantX)
	}
	if pos.X <= float32(paddingX) {
		t.Fatalf("popup x = %v, want it right of the gutter", pos.X)
	}
	_, lineH := u.ed.metrics()
	wantY := paddingY + lineH // caret row 0 ends here
	if pos.Y-wantY > 1 || wantY-pos.Y > 1 {
		t.Fatalf("popup y = %v, want just under the caret row %v", pos.Y, wantY)
	}
}

// TestOpeningAFileSelectsItsRow covers the sidebar state: the open file is the
// only highlighted row (also after switching), and an unnamed buffer clears it.
func TestOpeningAFileSelectsItsRow(t *testing.T) {
	u := newTestUI(t)
	for _, name := range []string{"a.http", "b.http"} {
		if err := os.WriteFile(name, []byte("GET http://x\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	u.reloadFiles()

	u.openNamed("a.http")
	if want := 1 + slices.Index(u.files.files, "a.http"); u.files.selected != want {
		t.Fatalf("selected row = %d, want %d (a.http)", u.files.selected, want)
	}

	u.openNamed("b.http")
	want := 1 + slices.Index(u.files.files, "b.http")
	if u.files.selected != want {
		t.Fatalf("selected row after switch = %d, want %d (b.http)", u.files.selected, want)
	}

	u.newFile()
	if u.files.selected != -1 {
		t.Fatalf("selected row for an unnamed buffer = %d, want none", u.files.selected)
	}
}

// fileRowBackground returns the highlight colour the sidebar actually paints on
// a row, so tests check what the user sees.
func fileRowBackground(u *ui, id int) color.Color {
	if id < 0 || id >= len(u.files.items) {
		return color.Transparent
	}
	return u.files.items[id].bg.FillColor
}

// TestOnlyTheOpenFileRowIsHighlighted covers the sidebar defect end to end: a
// row click moves the highlight to the opened file and clears every other row;
// a new buffer clears the highlight completely.
func TestOnlyTheOpenFileRowIsHighlighted(t *testing.T) {
	u := newTestUI(t)
	for _, name := range []string{"a.http", "b.http"} {
		if err := os.WriteFile(name, []byte("GET http://x\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	u.reloadFiles()
	aID := 1 + slices.Index(u.files.files, "a.http")
	bID := 1 + slices.Index(u.files.files, "b.http")

	u.files.clickRow(aID)
	if got := fileRowBackground(u, aID); got == color.Transparent {
		t.Fatal("the clicked row is not highlighted")
	}
	if got := fileRowBackground(u, bID); got != color.Transparent {
		t.Fatal("an unopened row is highlighted")
	}

	u.files.clickRow(bID)
	if got := fileRowBackground(u, bID); got == color.Transparent {
		t.Fatal("the second file row is not highlighted")
	}
	if got := fileRowBackground(u, aID); got != color.Transparent {
		t.Fatal("the previous file row stayed highlighted")
	}

	u.files.clickRow(0) // "+ Новый файл"
	if u.files.selected != -1 {
		t.Fatalf("selected row = %d, want none for a new buffer", u.files.selected)
	}
	for id := 0; id <= len(u.files.files); id++ {
		if got := fileRowBackground(u, id); got != color.Transparent {
			t.Fatalf("row %d is highlighted after starting a new buffer", id)
		}
	}
}

// TestFileRowHoverHighlight covers the hover tint: an inactive row gets a quiet
// background under the pointer, the open row keeps its stronger highlight, and
// leaving the row clears the tint.
func TestFileRowHoverHighlight(t *testing.T) {
	u := newTestUI(t)
	for _, name := range []string{"a.http", "b.http"} {
		if err := os.WriteFile(name, []byte("GET http://x\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	u.reloadFiles()
	u.openNamed("b.http")

	aID := 1 + slices.Index(u.files.files, "a.http")
	bID := 1 + slices.Index(u.files.files, "b.http")
	aRow := u.files.items[aID]
	if _, ok := any(aRow).(desktop.Hoverable); !ok {
		t.Fatal("fileRow does not implement desktop.Hoverable, so Fyne never sends MouseIn")
	}
	if got := aRow.bg.FillColor; got != color.Transparent {
		t.Fatalf("inactive row starts highlighted: %v", got)
	}

	aRow.MouseIn(&desktop.MouseEvent{})
	if got := aRow.bg.FillColor; got != u.pal.menuBg {
		t.Fatalf("hover background = %v, want %v", got, u.pal.menuBg)
	}
	aRow.MouseOut()
	if got := aRow.bg.FillColor; got != color.Transparent {
		t.Fatalf("hover left a background after MouseOut: %v", got)
	}

	// Hovering the open file must not replace its stronger highlight.
	bRow := u.files.items[bID]
	bRow.MouseIn(&desktop.MouseEvent{})
	if got := bRow.bg.FillColor; got != u.pal.menuSelBg {
		t.Fatalf("hovering the open file changed its highlight to %v", got)
	}
	bRow.MouseOut()
	if got := bRow.bg.FillColor; got != u.pal.menuSelBg {
		t.Fatalf("MouseOut removed the open-file highlight: %v", got)
	}
}

// TestWindowSizeIsRemembered covers the reported defect: the window always
// reopened at the hardcoded size instead of the one it was closed with.
func TestWindowSizeIsRemembered(t *testing.T) {
	u := newTestUI(t)
	u.win.Resize(fyne.NewSize(1000, 640))
	u.persistSession()

	saved := settings.Load()
	if saved.WindowWidth != 1000 || saved.WindowHeight != 640 {
		t.Fatalf("stored window size = %dx%d, want 1000x640", saved.WindowWidth, saved.WindowHeight)
	}

	u2 := newUI(fyne.CurrentApp())
	if got := u2.win.Canvas().Size(); got.Width != 1000 || got.Height != 640 {
		t.Fatalf("reopened window size = %v, want 1000x640", got)
	}
}

// TestWindowSizeFallsBackToTheDefaultWhenUnusable keeps a degenerate stored size
// (an unshown window reports zero) from shrinking the window to nothing.
func TestWindowSizeFallsBackToTheDefaultWhenUnusable(t *testing.T) {
	u := newTestUI(t)
	u.win.Resize(fyne.NewSize(100, 100))
	u.rememberWindowSize()
	if u.settings.WindowWidth != 0 || u.settings.WindowHeight != 0 {
		t.Fatalf("a tiny canvas size was stored: %dx%d", u.settings.WindowWidth, u.settings.WindowHeight)
	}

	got := windowSize(settings.Settings{WindowWidth: 10, WindowHeight: 10})
	if got.Width != defaultWindowWidth || got.Height != defaultWindowHeight {
		t.Fatalf("windowSize with a tiny remembered size = %v, want the default", got)
	}
}

// TestSplitSizesSurviveAPaneClick is the regression guard for the second
// reported defect: clicking a pane rebuilt the window content, which replaced
// the splitters and reset their dividers to the defaults.
func TestSplitSizesSurviveAPaneClick(t *testing.T) {
	u := newTestUI(t)
	u.outer.SetOffset(0.3)
	u.inner.SetOffset(0.5)

	u.setActive(paneFiles)
	u.setActive(paneResp)
	u.setActive(paneEditor)

	if got := u.outer.Offset; got < 0.299 || got > 0.301 {
		t.Fatalf("files|editor divider after a pane click = %v, want 0.3", got)
	}
	if got := u.inner.Offset; got < 0.499 || got > 0.501 {
		t.Fatalf("editor|response divider after a pane click = %v, want 0.5", got)
	}
}

// TestSplitSizesAreRemembered covers the persistence half: the divider
// positions are stored on close and restored in a fresh window.
func TestSplitSizesAreRemembered(t *testing.T) {
	u := newTestUI(t)
	u.outer.SetOffset(0.25)
	u.inner.SetOffset(0.7)
	u.persistSession()

	saved := settings.Load()
	if saved.SplitPane < 0.249 || saved.SplitPane > 0.251 {
		t.Fatalf("stored files|editor divider = %v, want 0.25", saved.SplitPane)
	}
	if saved.SplitResponse < 0.699 || saved.SplitResponse > 0.701 {
		t.Fatalf("stored editor|response divider = %v, want 0.7", saved.SplitResponse)
	}

	u2 := newUI(fyne.CurrentApp())
	if got := u2.outer.Offset; got < 0.249 || got > 0.251 {
		t.Fatalf("restored files|editor divider = %v, want 0.25", got)
	}
	if got := u2.inner.Offset; got < 0.699 || got > 0.701 {
		t.Fatalf("restored editor|response divider = %v, want 0.7", got)
	}
}

// fakeWindowState makes the platform maximize hooks test-controlled: it reports
// the given maximized state and records every request to apply one. A real OS
// window is not available on the headless test driver, so the state logic is
// checked through the hooks.
type fakeWindowState struct {
	maximized bool
	calls     []bool
}

func newFakeWindowState(t *testing.T, maximized bool) *fakeWindowState {
	t.Helper()
	f := &fakeWindowState{maximized: maximized}
	oldIs, oldSet := windowIsMaximized, windowSetMaximized
	windowIsMaximized = func(fyne.Window) bool { return f.maximized }
	windowSetMaximized = func(_ fyne.Window, max bool) { f.calls = append(f.calls, max) }
	t.Cleanup(func() { windowIsMaximized, windowSetMaximized = oldIs, oldSet })
	return f
}

// TestMaximizedStateIsRemembered covers the reported defect: a window closed
// maximized ("развёрнуто во весь экран") must reopen maximized.
func TestMaximizedStateIsRemembered(t *testing.T) {
	state := newFakeWindowState(t, true)
	u := newTestUI(t)
	u.persistSession()

	saved := settings.Load()
	if !saved.WindowMaximized {
		t.Fatal("the maximized state was not stored")
	}
	// The screen-sized canvas must not become the restored size.
	if saved.WindowWidth != 0 || saved.WindowHeight != 0 {
		t.Fatalf("maximized window stored size %dx%d, want the previous one kept",
			saved.WindowWidth, saved.WindowHeight)
	}
	if len(state.calls) != 0 {
		t.Fatalf("persisting applied a window state: %v", state.calls)
	}

	u2 := newUI(fyne.CurrentApp())
	if !u2.settings.WindowMaximized {
		t.Fatal("the maximized state was not loaded")
	}
	u2.restoreWindowState()
	if len(state.calls) != 1 || !state.calls[0] {
		t.Fatalf("restore applied %v, want a single maximize", state.calls)
	}
}

// TestWindowedStateIsRemembered is the other half: a normal window clears the
// flag and still stores its size.
func TestWindowedStateIsRemembered(t *testing.T) {
	state := newFakeWindowState(t, false)
	u := newTestUI(t)
	u.win.Resize(fyne.NewSize(1000, 640))
	u.persistSession()

	saved := settings.Load()
	if saved.WindowMaximized {
		t.Fatal("a normal window stored the maximized state")
	}
	if saved.WindowWidth != 1000 || saved.WindowHeight != 640 {
		t.Fatalf("stored window size = %dx%d, want 1000x640", saved.WindowWidth, saved.WindowHeight)
	}

	u2 := newUI(fyne.CurrentApp())
	u2.restoreWindowState()
	if len(state.calls) != 0 {
		t.Fatalf("restore applied %v for a normal window", state.calls)
	}
}

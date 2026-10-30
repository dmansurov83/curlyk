package gui

import (
	"os"
	"strings"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/test"
)

// TestMain installs a test app once: measuring text, driving keys and using the
// clipboard all need one.
func TestMain(m *testing.M) {
	test.NewApp()
	os.Exit(m.Run())
}

func newTestEditor(t *testing.T, src string) *Editor {
	t.Helper()
	e := NewEditor(src, nil)
	e.Resize(fyne.NewSize(600, 200))
	return e
}

// withModifiers makes the widget see the given modifiers for one test, standing
// in for the driver query that keyModifiersFn normally performs.
func withModifiers(t *testing.T, mods fyne.KeyModifier) {
	t.Helper()
	prev := keyModifiersFn
	keyModifiersFn = func() fyne.KeyModifier { return mods }
	t.Cleanup(func() { keyModifiersFn = prev })
}

func pressKey(e *Editor, name fyne.KeyName) { e.TypedKey(&fyne.KeyEvent{Name: name}) }

func typeString(e *Editor, s string) {
	for _, r := range s {
		e.TypedRune(r)
	}
}

// TestSetTextRehighlightsTheBuffer is the regression guard for a defect the
// window showed as an empty editor after switching files: SetText re-lexed an
// empty document, so every row painted nothing until the next keystroke.
func TestSetTextRehighlightsTheBuffer(t *testing.T) {
	e := newTestEditor(t, "GET http://old\n")
	e.SetText("POST http://new\nAccept: application/json\n")

	if got := e.Text(); !strings.Contains(got, "POST http://new") {
		t.Fatalf("buffer = %q", got)
	}
	// Blank lines legitimately have nothing to paint, so only the source lines are
	// checked.
	checked := 0
	for row := 0; row < e.LineCount(); row++ {
		if strings.TrimSpace(e.LineAt(row)) == "" {
			continue
		}
		checked++
		if len(e.lex.line(row)) == 0 {
			t.Fatalf("row %d has no highlight segments after SetText", row)
		}
		if len(e.rowPieces(row)) == 0 {
			t.Fatalf("row %d painted nothing after SetText", row)
		}
	}
	if checked != 2 {
		t.Fatalf("checked %d source rows, want 2", checked)
	}
}

func TestEditorTypingAndBackspace(t *testing.T) {
	e := newTestEditor(t, "")
	typeString(e, "GET")
	if got := e.Text(); got != "GET" {
		t.Fatalf("text = %q, want GET", got)
	}
	pressKey(e, fyne.KeyBackspace)
	if got := e.Text(); got != "GE" {
		t.Fatalf("text = %q, want GE", got)
	}
	// The caret sits at the end after a backspace, so step left before deleting
	// forward: that removes the 'E'.
	pressKey(e, fyne.KeyLeft)
	pressKey(e, fyne.KeyDelete)
	if got := e.Text(); got != "G" {
		t.Fatalf("text = %q, want G", got)
	}
}

// TestEditorNoOpKeysRecordNoUndoStep keeps Ctrl+Z meaningful: keys that cannot
// change the buffer must not fill the history.
func TestEditorNoOpKeysRecordNoUndoStep(t *testing.T) {
	e := newTestEditor(t, "one line")
	pressKey(e, fyne.KeyBackspace)
	if e.CanUndo() {
		t.Fatal("backspace at the buffer start should not record an undo step")
	}
	e.doc.moveDocEnd()
	pressKey(e, fyne.KeyDelete)
	if e.CanUndo() {
		t.Fatal("delete at the buffer end should not record an undo step")
	}
}

func TestShiftArrowsSelectAndCopy(t *testing.T) {
	e := newTestEditor(t, "GET http://a")
	withModifiers(t, fyne.KeyModifierShift)
	e.doc.setCaret(0, 0)
	for i := 0; i < 3; i++ {
		pressKey(e, fyne.KeyRight)
	}

	if got := e.doc.selText(); got != "GET" {
		t.Fatalf("selection = %q, want GET", got)
	}
	if !e.Copy() {
		t.Fatal("copy reported nothing to copy")
	}
	if got := clipboard().Content(); got != "GET" {
		t.Fatalf("clipboard = %q, want GET", got)
	}
}

// TestShiftThenPlainArrowCollapsesSelection covers the standard behaviour: an
// unmodified arrow after a selection drops it instead of moving past it.
func TestShiftThenPlainArrowCollapsesSelection(t *testing.T) {
	e := newTestEditor(t, "abcdef")
	withModifiers(t, fyne.KeyModifierShift)
	e.doc.setCaret(0, 1)
	pressKey(e, fyne.KeyRight)
	pressKey(e, fyne.KeyRight)
	if got := e.doc.selText(); got != "bc" {
		t.Fatalf("selection = %q, want bc", got)
	}

	withModifiers(t, 0)
	pressKey(e, fyne.KeyRight)
	if e.HasSelection() {
		t.Fatal("a plain arrow should clear the selection")
	}
	if e.doc.col != 3 {
		t.Fatalf("caret col = %d, want 3 (the selection end)", e.doc.col)
	}
}

func TestSelectAllCutAndPasteShortcuts(t *testing.T) {
	e := newTestEditor(t, "line one\nline two")
	e.TypedShortcut(&fyne.ShortcutSelectAll{})
	if !e.HasSelection() {
		t.Fatal("select all did not select")
	}

	e.TypedShortcut(&fyne.ShortcutCut{})
	if got := e.Text(); got != "" {
		t.Fatalf("text after cut = %q, want empty", got)
	}
	if got := clipboard().Content(); got != "line one\nline two" {
		t.Fatalf("clipboard = %q", got)
	}
	if !e.CanUndo() {
		t.Fatal("cut should be undoable")
	}

	e.TypedShortcut(&fyne.ShortcutPaste{})
	if got := e.Text(); got != "line one\nline two" {
		t.Fatalf("text after paste = %q", got)
	}
	if e.doc.lineCount() != 2 {
		t.Fatalf("pasted text split into %d lines, want 2", e.doc.lineCount())
	}
}

func TestUndoRedoShortcuts(t *testing.T) {
	e := newTestEditor(t, "")
	typeString(e, "abc")
	e.TypedShortcut(&fyne.ShortcutUndo{})
	if got := e.Text(); got != "" {
		t.Fatalf("after undo text = %q, want empty", got)
	}
	e.TypedShortcut(&fyne.ShortcutRedo{})
	if got := e.Text(); got != "abc" {
		t.Fatalf("after redo text = %q, want abc", got)
	}
}

// TestCellAtMapsClicksToRunes covers the click geometry, including a line with
// non-ASCII text where a byte-based mapping would land in the wrong column.
func TestCellAtMapsClicksToRunes(t *testing.T) {
	e := newTestEditor(t, "GET http://a\nпривет мир")
	origin := e.textOriginX()

	d := e.rowDisplay(0)
	if row, col := e.cellAt(fyne.NewPos(origin+d.x(3), paddingY)); row != 0 || col != 3 {
		t.Fatalf("click after GET mapped to (%d,%d), want (0,3)", row, col)
	}

	lineH := e.lineHeight()
	d = e.rowDisplay(1)
	if row, col := e.cellAt(fyne.NewPos(origin+d.x(6), paddingY+lineH)); row != 1 || col != 6 {
		t.Fatalf("click in the Cyrillic line mapped to (%d,%d), want (1,6)", row, col)
	}

	// A click past the end of a short line clamps to the line length.
	d = e.rowDisplay(0)
	if row, col := e.cellAt(fyne.NewPos(origin+d.x(200), paddingY)); row != 0 || col != runeLen("GET http://a") {
		t.Fatalf("click past the line end mapped to (%d,%d)", row, col)
	}
}

// TestVisibleRangeFollowsCaret pins the editor's own scrolling: the widget size
// stays fixed and the caret is scrolled into view instead.
func TestVisibleRangeFollowsCaret(t *testing.T) {
	e := newTestEditor(t, manyLines(200))
	lineH := e.lineHeight()

	first, last := e.visibleRange()
	if first != 0 {
		t.Fatalf("initial first row = %d, want 0", first)
	}
	if last >= 199 {
		t.Fatalf("initial last row = %d: the whole document is being drawn", last)
	}
	if got := float32(last-first+1) * lineH; got > e.Size().Height+2*lineH {
		t.Fatalf("visible window %v exceeds the widget height %v", got, e.Size().Height)
	}

	e.doc.setCaret(150, 0)
	e.ensureCaretVisible()
	if e.ScrollY() <= 0 {
		t.Fatalf("caret at row 150 did not scroll the view (offset %v)", e.ScrollY())
	}
	first, last = e.visibleRange()
	if first > 150 || last < 150 {
		t.Fatalf("row 150 is outside the visible window %d..%d", first, last)
	}

	// The wheel moves the viewport without moving the caret. Fyne hands the
	// scroll delta over in pixels (one notch is 25 px on Windows), so a notch
	// must move the view by exactly that many pixels, not by a fixed line count.
	const notch = 25
	before := e.ScrollY()
	e.Scrolled(&fyne.ScrollEvent{Scrolled: fyne.NewDelta(0, -notch)})
	if got := e.ScrollY() - before; got != notch {
		t.Fatalf("one wheel notch moved the view by %v px, want %v", got, notch)
	}
	if e.doc.row != 150 {
		t.Fatalf("scrolling moved the caret to row %d", e.doc.row)
	}
}

// TestRendererDrawsOnlyVisibleRows is the virtualisation guarantee: a large
// document must not produce a text object per line.
func TestRendererDrawsOnlyVisibleRows(t *testing.T) {
	e := newTestEditor(t, manyLines(500))
	r := test.WidgetRenderer(e)
	r.Refresh()

	visible := 0
	for _, o := range r.Objects() {
		if txt, ok := o.(*canvas.Text); ok && txt.Visible() && txt.Text != "" {
			visible++
		}
	}
	if visible == 0 {
		t.Fatal("nothing was drawn")
	}
	// 500 rows with several runs each would be well over a thousand objects.
	if visible > 300 {
		t.Fatalf("renderer drew %d text objects for a 500-line document", visible)
	}
}

func TestFindNavigatesAndHighlights(t *testing.T) {
	e := newTestEditor(t, "hit one\nnothing\nhit two\nhit three")
	e.SetFindQuery("hit")
	if got := e.FindMatches(); got != 3 {
		t.Fatalf("matches = %d, want 3", got)
	}
	if got := e.FindCurrent(); got != 1 {
		t.Fatalf("current match = %d, want 1", got)
	}

	e.FindNext()
	if got := e.FindCurrent(); got != 2 {
		t.Fatalf("after next current = %d, want 2", got)
	}
	if e.doc.row != 2 {
		t.Fatalf("caret row = %d, want 2 (second match line)", e.doc.row)
	}
	e.FindPrev()
	e.FindPrev()
	if got := e.FindCurrent(); got != 3 {
		t.Fatalf("wrapping back gives current = %d, want 3", got)
	}

	marks := e.findMarksRow(2)
	if len(marks) != 1 || marks[0].from != 0 || marks[0].to != 3 {
		t.Fatalf("marks on row 2 = %+v", marks)
	}
	var current, plain int
	for _, m := range e.findMarksRow(2) {
		if m.kind == markFindCurrent {
			current++
		}
	}
	for _, m := range e.findMarksRow(0) {
		if m.kind == markFind {
			plain++
		}
	}
	if current != 0 || plain != 1 {
		t.Fatalf("expected row 0 to hold a plain match, row 2 none (current is row 3): current=%d plain=%d", current, plain)
	}

	e.ClearFind()
	if e.FindMatches() != 0 || len(e.findMarksRow(0)) != 0 {
		t.Fatal("clearing the search left highlights behind")
	}
}

// TestSelectionPaintingClipsToMarks checks the renderer's run splitting: the
// selected part of a line keeps its syntax colour and gains an overlay.
func TestSelectionPaintingClipsToMarks(t *testing.T) {
	e := newTestEditor(t, "GET http://a")
	e.doc.anchorRow, e.doc.anchorCol = 0, 0
	e.doc.row, e.doc.col, e.doc.selOn = 0, 3, true

	var withMark, plain int
	for _, p := range e.rowPieces(0) {
		if p.mark != nil {
			withMark++
			if p.mark.kind != markSelection {
				t.Fatalf("unexpected mark kind %v", p.mark.kind)
			}
		} else {
			plain++
		}
	}
	if withMark == 0 {
		t.Fatal("the selection did not produce an overlay run")
	}
	if plain == 0 {
		t.Fatal("the unselected part of the line was not split out")
	}
}

// TestMouseClickDragAndDoubleTap covers the pointer paths: a click sets the
// caret, dragging from the press point selects, and a double click selects the
// word under the pointer.
func TestMouseClickDragAndDoubleTap(t *testing.T) {
	e := newTestEditor(t, "GET http://a\nHost: h")
	origin := e.textOriginX()
	d := e.rowDisplay(0)
	at := func(col int) fyne.Position { return fyne.NewPos(origin+d.x(col), paddingY+1) }

	e.MouseDown(&desktop.MouseEvent{
		PointEvent: fyne.PointEvent{Position: at(0)},
		Button:     desktop.MouseButtonPrimary,
	})
	if e.doc.col != 0 || e.doc.row != 0 {
		t.Fatalf("press moved the caret to (%d,%d), want (0,0)", e.doc.row, e.doc.col)
	}
	if e.HasSelection() {
		t.Fatal("a press alone should not select")
	}

	e.Dragged(&fyne.DragEvent{PointEvent: fyne.PointEvent{Position: at(3)}, Dragged: fyne.NewDelta(30, 0)})
	if got := e.doc.selText(); got != "GET" {
		t.Fatalf("drag selection = %q, want GET", got)
	}
	e.DragEnd()

	e.Tapped(&fyne.PointEvent{Position: at(5)})
	if e.HasSelection() {
		t.Fatal("a click should clear the selection")
	}
	if e.doc.col != 5 {
		t.Fatalf("click landed at col %d, want 5", e.doc.col)
	}

	e.DoubleTapped(&fyne.PointEvent{Position: at(1)})
	if got := e.doc.selText(); got != "GET" {
		t.Fatalf("double click selected %q, want GET", got)
	}
}

// TestDragSelectionRemembersThePressCell pins that a drag selects from where the
// button went down, not from where the pointer happened to be when the first
// drag event arrived.
func TestDragSelectionRemembersThePressCell(t *testing.T) {
	e := newTestEditor(t, "abcdef")
	origin := e.textOriginX()
	d := e.rowDisplay(0)

	e.MouseDown(&desktop.MouseEvent{
		PointEvent: fyne.PointEvent{Position: fyne.NewPos(origin+d.x(1), paddingY+1)},
		Button:     desktop.MouseButtonPrimary,
	})
	e.Dragged(&fyne.DragEvent{
		PointEvent: fyne.PointEvent{Position: fyne.NewPos(origin+d.x(4), paddingY+1)},
		Dragged:    fyne.NewDelta(40, 0),
	})
	if got := e.doc.selText(); got != "bcd" {
		t.Fatalf("drag selection = %q, want bcd", got)
	}
}

// TestPageKeysReachTheBufferEdges pins that paging down as far as it goes leaves
// the caret at the very end of the file, and paging up at its start. Before this
// guard the caret stopped a whole page above the end and PageDown then did
// nothing at all, so the file could never be reached with the keyboard.
func TestPageKeysReachTheBufferEdges(t *testing.T) {
	const last = "last line"
	e := newTestEditor(t, strings.Repeat("line\n", 199)+last)
	rows := e.visibleRows()
	if rows <= 0 || rows >= e.LineCount() {
		t.Fatalf("visibleRows = %d: the document must span several pages", rows)
	}

	e.doc.setCaret(0, 0)
	pressKey(e, fyne.KeyPageDown)
	if e.doc.row != rows {
		t.Fatalf("one PageDown moved the caret to row %d, want %d", e.doc.row, rows)
	}

	for i := 0; i < e.LineCount(); i++ {
		pressKey(e, fyne.KeyPageDown)
	}
	if got, want := e.doc.row, e.LineCount()-1; got != want {
		t.Fatalf("paging down stopped at row %d, want the last row %d", got, want)
	}
	if got, want := e.doc.col, runeLen(last); got != want {
		t.Fatalf("caret col after paging down = %d, want %d (end of the file)", got, want)
	}

	for i := 0; i < e.LineCount(); i++ {
		pressKey(e, fyne.KeyPageUp)
	}
	if e.doc.row != 0 || e.doc.col != 0 {
		t.Fatalf("paging up stopped at (%d,%d), want (0,0)", e.doc.row, e.doc.col)
	}
}

// manyLines builds n numbered .http-ish lines with a request every few lines.
func manyLines(n int) string {
	var b []byte
	for i := 0; i < n; i++ {
		switch i % 4 {
		case 0:
			b = append(b, "### block"...)
			b = append(b, '\n')
		case 1:
			b = append(b, "GET http://example.com/x"...)
			b = append(b, '\n')
		case 2:
			b = append(b, "Accept: application/json"...)
			b = append(b, '\n')
		default:
			b = append(b, '\n')
		}
	}
	return string(b)
}

// TestCtrlShiftArrowsSelectByWord covers the selection half of word navigation:
// Ctrl+Shift+Right extends to the next word boundary, Ctrl+Shift+Left back.
func TestCtrlShiftArrowsSelectByWord(t *testing.T) {
	e := newTestEditor(t, "GET http://example.test/x\n")
	e.doc.setCaret(0, 0)
	withModifiers(t, fyne.KeyModifierControl|fyne.KeyModifierShift)

	pressKey(e, fyne.KeyRight)
	if got := e.doc.selText(); got != "GET " {
		t.Fatalf("selection after Ctrl+Shift+Right = %q, want %q", got, "GET ")
	}

	withModifiers(t, 0)
	pressKey(e, fyne.KeyLeft)
	if e.doc.selActive() {
		t.Fatal("a plain arrow must collapse the selection")
	}
}

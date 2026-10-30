package gui

import (
	"strconv"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/widget"

	"github.com/user/curlyk/internal/httpfile"
)

// Layout constants of the editor, in pixels.
const (
	paddingX = 6
	paddingY = 4
	caretW   = 2
)

// keyModifiersFn reports the modifiers held right now. Fyne's key events carry
// no modifier field, so a widget that needs Shift/Ctrl (selection, word jumps)
// asks the driver, exactly as widget.Entry does. It is a variable so tests can
// drive those paths without a real driver.
var keyModifiersFn = func() fyne.KeyModifier {
	a := fyne.CurrentApp()
	if a == nil {
		return 0
	}
	dd, ok := a.Driver().(desktop.Driver)
	if !ok {
		return 0
	}
	return dd.CurrentKeyModifiers()
}

// Editor renders a .http source with curlyk's own syntax colours and handles
// typing, caret movement, selection and clicks. It scrolls itself: MinSize stays
// small (the widget never grows with the document), the visible window follows
// the caret and the wheel, and only the visible lines are rendered.
type Editor struct {
	widget.BaseWidget

	doc       *doc
	pal       *palette
	lex       *highlighter
	fontSize  float32
	tabSize   int
	showLines bool
	focused   bool

	// readOnly turns the editor into a viewer: navigation, selection, copying and
	// scrolling still work, every edit is ignored. The response pane uses it.
	readOnly bool

	scrollX float32
	scrollY float32

	// rows caches the measured geometry of every row drawn or touched so far;
	// dropMetrics invalidates it. Keeping it per row is what keeps caret and
	// click math off the hot path.
	rows map[int]*rowDisplay

	// find is the active search over the buffer, if any.
	find findState

	// The response pane reuses this editor as a read-only viewer. responseMode
	// switches the segment source from the .http lexer to the response segments
	// built by SetResponseText: header rows stay neutral, the body is JSON.
	responseMode   bool
	respHeaderRows int
	respSegs       [][]seg

	// complete is the autocomplete list, if any. It is drawn by this widget's own
	// renderer (not by a Fyne popup overlay) precisely so the editor keeps the
	// keyboard focus while a list is open. The window sets it when wiring the
	// completion.
	complete *completionPopup

	// requestRows is the set of buffer rows that open an HTTP request; the
	// renderer paints the run icon in their gutter. onRunIcon reports a click on
	// that icon to the window.
	requestRows map[int]bool
	onRunIcon   func(row int)
	iconRow     int

	// dragRow/dragCol are the anchor of a mouse drag; dragging is true from the
	// first Dragged event of that gesture until DragEnd.
	dragRow  int
	dragCol  int
	dragging bool

	onChange func()
	onCaret  func(y float32)
	keyHook  func(name fyne.KeyName) bool
	// runeHook reports printable input in a read-only viewer, so the window can
	// treat typing in the response pane as "start a search" (the pane itself
	// never accepts text).
	runeHook func(r rune) bool
	// shortcutHook lets the window handle its own shortcuts. It must be consulted
	// from TypedShortcut: once the focused widget is Shortcutable, Fyne delivers
	// every custom shortcut to the widget and never reaches the canvas handlers
	// registered with AddShortcut.
	shortcutHook func(s fyne.Shortcut) bool
	// secondaryHook reports right clicks (row, position) so the window can open
	// the request menu, and onFocus lets it mark the editor pane active.
	secondaryHook func(row int, pos fyne.Position)
	onFocus       func()
	// pasteHandler lets the window rewrite clipboard text before it is inserted
	// (a cURL command becomes an .http block). handled=false keeps the text as
	// pasted; a non-nil error aborts the paste so the window can report it.
	pasteHandler func(text string) (replacement string, handled bool, err error)
}

// NewEditor creates an editor over src. A nil palette means the default scheme.
func NewEditor(src string, pal *palette) *Editor {
	if pal == nil {
		pal = loadPalette("")
	}
	e := &Editor{
		doc:       newDoc(src),
		pal:       pal,
		lex:       newHighlighter(),
		fontSize:  14,
		tabSize:   4,
		showLines: true,
		rows:      map[int]*rowDisplay{},
		iconRow:   -1,
	}
	e.lex.setText(src)
	e.refreshRequestRows()
	e.ExtendBaseWidget(e)
	return e
}

// Text returns the current buffer.
func (e *Editor) Text() string { return e.doc.text() }

// SetText replaces the buffer, clears the history and moves the caret to the
// very beginning.
func (e *Editor) SetText(src string) {
	e.responseMode = false
	e.respSegs = nil
	e.doc.setText(src)
	// The highlight must follow the new buffer: relex("") would tokenize an empty
	// document and every row would paint nothing until the next keystroke.
	e.relex(src)
	e.refreshRequestRows()
	e.scrollX, e.scrollY = 0, 0
	e.Refresh()
	e.caretMoved()
}

// SetResponseText replaces the buffer with a response dump rendered as a
// read-only viewer: the first headerRows lines (the request dump, status, timing
// and response headers) are neutral, and the body below them is highlighted as
// JSON when it looks like JSON. The divider is passed in by the caller because
// only it knows which part of the formatted text is the body.
func (e *Editor) SetResponseText(text string, headerRows int) {
	e.doc.setText(text)
	lines := e.doc.lines
	headerRows = clampInt(headerRows, 0, len(lines))

	e.responseMode = true
	e.respHeaderRows = headerRows
	e.respSegs = responseSegments(lines, headerRows)
	e.requestRows = nil
	e.dropMetrics()
	e.scrollX, e.scrollY = 0, 0
	e.Refresh()
	e.caretMoved()
}

// ClearResponseMode returns the editor to .http source highlighting.
func (e *Editor) ClearResponseMode() {
	e.responseMode = false
	e.respSegs = nil
}

// SetPalette switches the colour scheme and repaints.
func (e *Editor) SetPalette(p *palette) {
	if p == nil {
		return
	}
	e.pal = p
	e.Refresh()
}

// SetFontSize changes the text size.
func (e *Editor) SetFontSize(size float32) {
	if size < 6 || size > 48 || size == e.fontSize {
		return
	}
	e.fontSize = size
	e.dropMetrics()
	e.ensureCaretVisible()
	e.Refresh()
	e.caretMoved()
}

// SetShowLineNumbers shows or hides the gutter.
func (e *Editor) SetShowLineNumbers(on bool) {
	if on == e.showLines {
		return
	}
	e.showLines = on
	e.Refresh()
}

// SetReadOnly makes the editor a viewer: the user can navigate, select and copy
// the text, but cannot change it. The response pane is a read-only editor.
func (e *Editor) SetReadOnly(on bool) { e.readOnly = on }

// SetOnChanged registers a callback fired after every edit.
func (e *Editor) SetOnChanged(fn func()) { e.onChange = fn }

// SetOnCaretMoved registers a callback fired whenever the caret moves, with the
// caret's top y in document pixels (not viewport pixels).
func (e *Editor) SetOnCaretMoved(fn func(y float32)) { e.onCaret = fn }

// SetKeyHook registers a handler consulted before the editor acts on a key, so
// an overlay such as the completion popup can consume navigation keys while the
// editor keeps focus. The handler reports whether it handled the key.
func (e *Editor) SetKeyHook(fn func(name fyne.KeyName) bool) { e.keyHook = fn }

// SetRuneHook registers the read-only viewer's printable-input handler: typing in
// the response pane opens its search instead of being dropped.
func (e *Editor) SetRuneHook(fn func(r rune) bool) { e.runeHook = fn }

// SetShortcutHandler registers the window's shortcut handler. It is consulted
// before the editor's own editing shortcuts and reports whether it handled the
// shortcut.
func (e *Editor) SetShortcutHandler(fn func(s fyne.Shortcut) bool) { e.shortcutHook = fn }

// SetSecondaryHook registers a handler for right clicks, called with the buffer
// row under the pointer and the click position inside the widget. The window
// uses it to open the request menu.
func (e *Editor) SetSecondaryHook(fn func(row int, pos fyne.Position)) { e.secondaryHook = fn }

// SetOnFocus registers a callback fired when the editor gains focus, so the
// window can mark the editor pane active.
func (e *Editor) SetOnFocus(fn func()) { e.onFocus = fn }

// SetPasteHandler registers the clipboard rewriter consulted by Paste.
func (e *Editor) SetPasteHandler(fn func(text string) (string, bool, error)) {
	e.pasteHandler = fn
}

// SetRunIconHandler registers the click handler of the gutter run icon, called
// with the buffer row whose icon was clicked.
func (e *Editor) SetRunIconHandler(fn func(row int)) { e.onRunIcon = fn }

// refreshRequestRows rebuilds the set of rows that start a request.
func (e *Editor) refreshRequestRows() {
	if e.responseMode {
		e.requestRows = nil
		return
	}
	reqs := httpfile.ParseFile(e.doc.text())
	rows := make(map[int]bool, len(reqs))
	for _, r := range reqs {
		if r.Line > 0 {
			rows[r.Line-1] = true
		}
	}
	e.requestRows = rows
}

// canvasPosition converts a position inside the widget into canvas coordinates,
// which is what overlays (the completion popup, the request menu) need.
func (e *Editor) canvasPosition(pos fyne.Position) fyne.Position {
	abs := fyne.CurrentApp().Driver().AbsolutePositionForObject(e)
	return fyne.NewPos(abs.X+pos.X, abs.Y+pos.Y)
}

// caretCanvasPosition is the canvas position just below the caret: where the
// completion popup opens.
func (e *Editor) caretCanvasPosition() fyne.Position {
	row, _ := e.Caret()
	lineH := e.lineHeight()
	return e.canvasPosition(fyne.NewPos(
		e.caretOffsetX()-e.scrollX,
		paddingY+float32(row)*lineH-e.scrollY+lineH,
	))
}

// ReplaceLines replaces the buffer lines [from,to) with text (split on newlines)
// as a single undo step, keeping the rest of the buffer. The request menu uses it
// to write a pretty-printed JSON body back.
func (e *Editor) ReplaceLines(from, to int, text string) bool {
	if from < 0 || to < from || from > e.doc.lineCount() {
		return false
	}
	if to > e.doc.lineCount() {
		to = e.doc.lineCount()
	}
	e.beginEdit(editOther)
	replacement := splitLines(text)
	lines := make([]string, 0, len(e.doc.lines)-(to-from)+len(replacement))
	lines = append(lines, e.doc.lines[:from]...)
	lines = append(lines, replacement...)
	lines = append(lines, e.doc.lines[to:]...)
	e.doc.lines = lines
	e.doc.setCaret(from, 0)
	e.doc.selClear()
	e.afterEdit()
	return true
}

// TappedSecondary implements fyne.SecondaryTappable: a right click reports the
// row under the pointer so the window can open the request menu there.
func (e *Editor) TappedSecondary(ev *fyne.PointEvent) {
	if e.secondaryHook == nil {
		return
	}
	row, col := e.cellAt(ev.Position)
	if !e.readOnly {
		e.doc.setCaret(row, col)
		e.doc.selClear()
		e.Refresh()
	}
	e.secondaryHook(row, ev.Position)
}

// Caret returns the current caret position as (row, rune column).
func (e *Editor) Caret() (row, col int) { return e.doc.row, e.doc.col }

// LineAt returns a source line ("" when the row is out of range).
func (e *Editor) LineAt(row int) string { return e.doc.line(row) }

// LineCount returns the number of lines in the buffer.
func (e *Editor) LineCount() int { return e.doc.lineCount() }

// RestoreCaret puts the caret at (row,col) and scrolls the given document offset
// into view, used to reopen a file where the user left it.
func (e *Editor) RestoreCaret(row, col int, scroll float32) {
	e.doc.setCaret(row, col)
	e.doc.selClear()
	e.scrollY = scroll
	e.clampScroll()
	e.ensureCaretVisible()
	e.Refresh()
	e.caretMoved()
}

// ReplaceRange replaces the runes [from,to) of row with text, as a single undo
// step. It is what an accepted completion uses.
func (e *Editor) ReplaceRange(row, from, to int, text string) bool {
	if row < 0 || row >= e.doc.lineCount() {
		return false
	}
	line := e.doc.line(row)
	from = clampInt(from, 0, runeLen(line))
	to = clampInt(to, from, runeLen(line))

	e.beginEdit(editOther)
	e.doc.lines[row] = runeSlice(line, 0, from) + text + runeSlice(line, to, runeLen(line))
	e.doc.setCaret(row, from+runeLen(text))
	e.doc.selClear()
	e.afterEdit()
	return true
}

// CaretY returns the caret's top offset inside the document.
func (e *Editor) CaretY() float32 { return paddingY + float32(e.doc.row)*e.lineHeight() }

// ScrollY returns the vertical scroll offset in pixels.
func (e *Editor) ScrollY() float32 { return e.scrollY }

// SelectionLen returns the number of selected runes.
func (e *Editor) SelectionLen() int { return runeLen(e.doc.selText()) }

// HasSelection reports whether something is selected.
func (e *Editor) HasSelection() bool { return e.doc.selActive() }

// CanUndo reports whether there is an edit to undo.
func (e *Editor) CanUndo() bool { return e.doc.canUndo() }

// CanRedo reports whether there is an undone edit to redo.
func (e *Editor) CanRedo() bool { return e.doc.canRedo() }

// TypedRune implements fyne.TypedRune.
func (e *Editor) TypedRune(r rune) {
	if e.readOnly {
		if e.runeHook != nil && r != 0 {
			e.runeHook(r)
		}
		return
	}
	e.beginEdit(editType)
	if !e.doc.insert(string(r)) {
		return
	}
	e.afterEdit()
}

// TypedKey implements fyne.TypedKey.
func (e *Editor) TypedKey(ev *fyne.KeyEvent) {
	// The key hook gets every key first: an overlay (the completion popup) or the
	// window (Escape) may consume it.
	if e.keyHook != nil && e.keyHook(ev.Name) {
		return
	}

	mods := keyModifiersFn()
	shift := mods&fyne.KeyModifierShift != 0
	ctrl := mods&(fyne.KeyModifierControl|fyne.KeyModifierSuper) != 0

	// In read-only mode the editing keys do nothing; navigation keys still move
	// the caret (and, with Shift, extend a selection).
	if e.readOnly {
		switch ev.Name {
		case fyne.KeyLeft, fyne.KeyRight, fyne.KeyUp, fyne.KeyDown,
			fyne.KeyHome, fyne.KeyEnd, fyne.KeyPageUp, fyne.KeyPageDown:
			e.moveKey(ev.Name, shift, ctrl)
		}
		return
	}

	switch ev.Name {
	case fyne.KeyBackspace:
		if !e.doc.canBackspace() {
			return
		}
		e.beginEdit(editOther)
		e.doc.backspace()
		e.afterEdit()
	case fyne.KeyDelete:
		if !e.doc.canDeleteForward() {
			return
		}
		e.beginEdit(editOther)
		e.doc.deleteForward()
		e.afterEdit()
	case fyne.KeyReturn, fyne.KeyEnter:
		e.beginEdit(editOther)
		e.doc.insert("\n")
		e.afterEdit()
	case fyne.KeyTab:
		e.beginEdit(editOther)
		e.doc.insert(strings.Repeat(" ", e.tabSize))
		e.afterEdit()
	case fyne.KeyLeft, fyne.KeyRight, fyne.KeyUp, fyne.KeyDown,
		fyne.KeyHome, fyne.KeyEnd, fyne.KeyPageUp, fyne.KeyPageDown:
		e.moveKey(ev.Name, shift, ctrl)
	default:
		return
	}
}

// MoveCaretKey applies a caret-navigation key coming from the window's shortcut
// handler (Fyne delivers Ctrl+Arrow as a custom shortcut, not as TypedKey).
func (e *Editor) MoveCaretKey(name fyne.KeyName, shift, ctrl bool) {
	e.moveKey(name, shift, ctrl)
}

// moveKey applies a caret-movement key. shift extends the selection; ctrl makes
// Home/End jump to the buffer edges and Up/Down a word at a time is not
// supported (a .http line is a whole field).
func (e *Editor) moveKey(name fyne.KeyName, shift, ctrl bool) {
	if !shift && e.doc.selActive() {
		// A plain movement collapses the selection to the edge it moves towards.
		e.collapseSelection(name)
		return
	}
	if shift && !e.doc.selOn {
		e.doc.selAnchor()
	}

	e.doc.breakUndoRun()
	moved := false
	switch name {
	case fyne.KeyLeft:
		if ctrl {
			moved = e.doc.moveWordLeft()
		} else {
			moved = e.doc.moveLeft()
		}
	case fyne.KeyRight:
		if ctrl {
			moved = e.doc.moveWordRight()
		} else {
			moved = e.doc.moveRight()
		}
	case fyne.KeyUp:
		moved = e.doc.moveVertical(-1)
	case fyne.KeyDown:
		moved = e.doc.moveVertical(1)
	case fyne.KeyHome:
		if ctrl {
			moved = e.doc.moveDocStart()
		} else {
			moved = e.doc.moveLineStart()
		}
	case fyne.KeyEnd:
		if ctrl {
			moved = e.doc.moveDocEnd()
		} else {
			moved = e.doc.moveLineEnd()
		}
	case fyne.KeyPageUp:
		moved = e.doc.pageVertical(-e.visibleRows())
	case fyne.KeyPageDown:
		moved = e.doc.pageVertical(e.visibleRows())
	}
	if !moved && !shift {
		e.doc.selClear()
	}
	e.ensureCaretVisible()
	e.Refresh()
	e.caretMoved()
}

// collapseSelection moves the caret to the selection edge the key moves towards
// and drops the selection.
func (e *Editor) collapseSelection(name fyne.KeyName) {
	r0, c0, r1, c1, _ := e.doc.selRange()
	switch name {
	case fyne.KeyRight, fyne.KeyDown, fyne.KeyEnd, fyne.KeyPageDown:
		e.doc.setCaret(r1, c1)
	default:
		e.doc.setCaret(r0, c0)
	}
	e.doc.selClear()
	e.ensureCaretVisible()
	e.Refresh()
	e.caretMoved()
}

// TypedShortcut implements fyne.Shortcutable: the clipboard, select all and
// undo/redo. Fyne maps Cmd on macOS and Ctrl elsewhere onto these shortcuts, so
// the same code path serves both.
func (e *Editor) TypedShortcut(s fyne.Shortcut) {
	// The window's own shortcuts are delivered here rather than to the canvas
	// while this widget has focus, so they are offered first.
	if e.shortcutHook != nil && e.shortcutHook(s) {
		return
	}

	switch s.(type) {
	case *fyne.ShortcutCopy:
		e.Copy()
	case *fyne.ShortcutCut:
		if !e.readOnly {
			e.Cut()
		}
	case *fyne.ShortcutPaste:
		if !e.readOnly {
			e.Paste()
		}
	case *fyne.ShortcutSelectAll:
		e.doc.selSelectAll()
		e.doc.breakUndoRun()
		e.ensureCaretVisible()
		e.Refresh()
		e.caretMoved()
	case *fyne.ShortcutUndo:
		if !e.readOnly {
			e.Undo()
		}
	case *fyne.ShortcutRedo:
		if !e.readOnly {
			e.Redo()
		}
	}
}

// Undo reverts the last edit and reports whether anything changed.
func (e *Editor) Undo() bool {
	if !e.doc.undo() {
		return false
	}
	e.relexAfterUndo()
	e.afterEdit()
	return true
}

// Redo re-applies an undone edit and reports whether anything changed.
func (e *Editor) Redo() bool {
	if !e.doc.redo() {
		return false
	}
	e.relexAfterUndo()
	e.afterEdit()
	return true
}

// requestFocus asks the driver for the keyboard focus, mirroring widget.Entry
// (whose own requestFocus calls CanvasForObject then Focus). The glfw driver
// does not auto-focus a widget on click — it only unfocuses the previous one —
// so without this call the caret never appears and the clipboard shortcuts
// (delivered only to the focused Shortcutable) never reach the editor.
func (e *Editor) requestFocus() {
	if c := fyne.CurrentApp().Driver().CanvasForObject(e); c != nil {
		c.Focus(e)
	}
}

// completionRowAt returns the index of the completion row under a widget
// position, or -1 when the list is closed or the point is outside it.
func (e *Editor) completionRowAt(pos fyne.Position) int {
	if e.complete == nil || !e.complete.open || len(e.complete.items) == 0 {
		return -1
	}
	p, s := e.completionBounds()
	if pos.X < p.X || pos.X >= p.X+s.Width || pos.Y < p.Y || pos.Y >= p.Y+s.Height {
		return -1
	}
	_, lineH := e.metrics()
	if lineH < 18 {
		lineH = 18
	}
	row := int((pos.Y - p.Y) / lineH)
	if row < 0 || row >= len(e.complete.items) {
		return -1
	}
	return row
}

// rowCanvasPosition is the canvas position of a row's gutter icon, where the
// request menu opens after a click on ▶.
func (e *Editor) rowCanvasPosition(row int) fyne.Position {
	_, lineH := e.metrics()
	return e.canvasPosition(fyne.NewPos(paddingX, paddingY+float32(row)*lineH-e.scrollY))
}

// runIconAt maps a click position to the request row whose ▶ icon it hits. The
// icon lives in the left part of the gutter, ahead of the line number.
func (e *Editor) runIconAt(pos fyne.Position) (int, bool) {
	if !e.showLines || len(e.requestRows) == 0 {
		return 0, false
	}
	charW, lineH := e.metrics()
	if lineH <= 0 {
		return 0, false
	}
	row := int((pos.Y + e.scrollY - paddingY) / lineH)
	if row < 0 || !e.requestRows[row] {
		return 0, false
	}
	if pos.X < 0 || pos.X > paddingX+charW {
		return 0, false
	}
	return row, true
}

// MouseDown implements desktop.Mouseable: it starts a potential drag at the
// pressed cell and drops any existing selection. It also takes the keyboard
// focus so the caret appears and typing works, like widget.Entry. A click on
// the completion list accepts that row instead of moving the caret.
func (e *Editor) MouseDown(ev *desktop.MouseEvent) {
	if ev.Button != desktop.MouseButtonPrimary {
		return
	}
	e.requestFocus()
	if row := e.completionRowAt(ev.Position); row >= 0 {
		if e.complete.u != nil {
			e.complete.u.acceptCompletionRow(row)
		}
		return
	}
	if row, ok := e.runIconAt(ev.Position); ok {
		// Remember the icon press: Tapped (which follows MouseDown) opens the
		// request menu instead of moving the caret to the gutter.
		e.iconRow = row
		return
	}
	e.iconRow = -1
	row, col := e.cellAt(ev.Position)
	e.dragRow, e.dragCol = row, col
	e.dragging = false
	e.doc.setCaret(row, col)
	e.doc.selClear()
	e.doc.breakUndoRun()
	e.Refresh()
	e.caretMoved()
}

// MouseUp implements desktop.Mouseable.
func (e *Editor) MouseUp(*desktop.MouseEvent) { e.dragging = false }

// Tapped implements fyne.Tappable: a click puts the caret where the pointer is,
// or opens the request menu when it hit a gutter run icon.
func (e *Editor) Tapped(ev *fyne.PointEvent) {
	e.requestFocus()
	if e.iconRow >= 0 {
		row := e.iconRow
		e.iconRow = -1
		if e.onRunIcon != nil {
			e.onRunIcon(row)
		}
		return
	}
	row, col := e.cellAt(ev.Position)
	e.doc.setCaret(row, col)
	e.doc.selClear()
	e.doc.breakUndoRun()
	e.Refresh()
	e.caretMoved()
}

// DoubleTapped implements fyne.DoubleTappable: select the word under the pointer.
func (e *Editor) DoubleTapped(ev *fyne.PointEvent) {
	e.iconRow = -1
	e.requestFocus()
	row, col := e.cellAt(ev.Position)
	e.doc.selWordAt(row, col)
	e.doc.breakUndoRun()
	e.Refresh()
	e.caretMoved()
}

// Dragged implements fyne.Draggable: dragging from the press point extends the
// selection, so text can be selected with the mouse.
func (e *Editor) Dragged(ev *fyne.DragEvent) {
	e.iconRow = -1
	e.dragging = true
	e.doc.anchorRow, e.doc.anchorCol = e.dragRow, e.dragCol
	e.doc.selOn = true

	row, col := e.cellAt(ev.Position)
	e.doc.setCaret(row, col)
	e.ensureCaretVisible()
	e.Refresh()
	e.caretMoved()
}

// DragEnd implements fyne.DragEndable.
func (e *Editor) DragEnd() { e.dragging = false }

// Scrolled implements fyne.Scrollable: the wheel scrolls the viewport. Fyne's
// driver already scales one notch to pixels (25 px on Windows, 10 on macOS), so
// the delta is applied as-is, exactly like the stock scroll containers do. The
// caret is not moved, so the view can be inspected away from the caret.
func (e *Editor) Scrolled(ev *fyne.ScrollEvent) {
	e.scrollY -= ev.Scrolled.DY
	e.scrollX -= ev.Scrolled.DX
	e.clampScroll()
	e.Refresh()
}

// Focusable implements fyne.Focusable.
func (e *Editor) Focusable() bool { return true }

// AcceptsTab implements fyne.Tabbable. The editor consumes Tab itself (the
// window switches panes), so Fyne must deliver the key instead of moving focus
// to the next widget.
func (e *Editor) AcceptsTab() bool { return true }

// FocusGained implements fyne.Focusable.
func (e *Editor) FocusGained() {
	e.focused = true
	e.Refresh()
	if e.onFocus != nil {
		e.onFocus()
	}
}

// FocusLost implements fyne.Focusable.
func (e *Editor) FocusLost() {
	e.focused = false
	e.Refresh()
}

// Resize tracks the viewport: the visible window, the scroll clamp and the caret
// reveal all depend on the widget's own size, since the editor scrolls itself.
func (e *Editor) Resize(size fyne.Size) {
	e.BaseWidget.Resize(size)
	e.clampScroll()
	if e.focused {
		e.ensureCaretVisible()
	}
}

// MinSize keeps the widget small: the editor scrolls itself, so its minimum size
// must never grow with the document (a growing minimum would make the parent
// layout scroll it too).
func (e *Editor) MinSize() fyne.Size {
	charW, lineH := e.metrics()
	return fyne.NewSize(20*charW, 8*lineH)
}

// beginEdit records an undo snapshot before a mutation.
func (e *Editor) beginEdit(kind string) { e.doc.pushUndo(kind) }

// afterEdit re-tokenizes the changed lines, repaints and notifies the owner.
func (e *Editor) afterEdit() {
	e.relexAfterUndo()
	e.refreshRequestRows()
	e.ensureCaretVisible()
	e.Refresh()
	if e.onChange != nil {
		e.onChange()
	}
	e.caretMoved()
}

// relexAfterUndo refreshes the highlight and the metric caches after any change
// to the buffer.
func (e *Editor) relexAfterUndo() {
	e.dropMetrics()
	e.lex.update(e.doc.lines)
}

// relex rebuilds the highlight for the whole buffer (used when the buffer is
// replaced wholesale) and refreshes the metric caches.
func (e *Editor) relex(src string) {
	e.dropMetrics()
	e.lex.setText(src)
}

func (e *Editor) caretMoved() {
	if e.onCaret != nil {
		e.onCaret(e.CaretY())
	}
}

// metrics returns the advance of the monospace face and the line height.
func (e *Editor) metrics() (charW, lineH float32) {
	style := fyne.TextStyle{Monospace: true}
	return fyne.MeasureText("M", e.fontSize, style).Width,
		fyne.MeasureText("Ag", e.fontSize, style).Height
}

func (e *Editor) lineHeight() float32 {
	_, lineH := e.metrics()
	return lineH
}

// textStyle returns the Fyne text style of a paint.
func (e *Editor) textStyle(p paint) fyne.TextStyle {
	return fyne.TextStyle{Monospace: true, Bold: p.bold, Italic: p.italic}
}

// gutterWidth is the width reserved for the run icon and the line numbers, 0
// when hidden.
func (e *Editor) gutterWidth() float32 {
	if !e.showLines {
		return 0
	}
	charW, _ := e.metrics()
	digits := len(strconv.Itoa(e.doc.lineCount()))
	return charW*float32(digits+2) + 2*paddingX
}

// textOriginX is the left edge of the source text in document coordinates.
func (e *Editor) textOriginX() float32 { return e.gutterWidth() + paddingX }

// contentHeight is the height of the whole document plus padding.
func (e *Editor) contentHeight() float32 {
	return float32(e.doc.lineCount())*e.lineHeight() + 2*paddingY
}

// visibleRows returns how many whole lines fit in the viewport.
func (e *Editor) visibleRows() int {
	_, lineH := e.metrics()
	if lineH <= 0 {
		return 1
	}
	h := e.Size().Height - 2*paddingY
	if h <= 0 {
		return 1
	}
	return max(1, int(h/lineH))
}

// visibleRange returns the row range the viewport shows, widened by one line so
// partially visible rows at both edges are drawn too.
func (e *Editor) visibleRange() (first, last int) {
	_, lineH := e.metrics()
	if lineH <= 0 {
		return 0, e.doc.lineCount() - 1
	}
	first = int((e.scrollY - paddingY) / lineH)
	if first < 0 {
		first = 0
	}
	last = int((e.scrollY + e.Size().Height - paddingY) / lineH)
	if last > e.doc.lineCount()-1 {
		last = e.doc.lineCount() - 1
	}
	return first, last
}

// clampScroll keeps the scroll offsets inside the document bounds.
func (e *Editor) clampScroll() {
	maxY := e.contentHeight() - e.Size().Height
	if maxY < 0 {
		maxY = 0
	}
	e.scrollY = clampFloat(e.scrollY, 0, maxY)

	maxX := e.visibleWidth() - e.Size().Width
	if maxX < 0 {
		maxX = 0
	}
	e.scrollX = clampFloat(e.scrollX, 0, maxX)
}

// ensureCaretVisible scrolls the minimum amount that brings the caret back into
// the viewport.
func (e *Editor) ensureCaretVisible() {
	_, lineH := e.metrics()
	caretTop := float32(e.doc.row) * lineH
	caretBottom := caretTop + lineH

	if caretTop-paddingY < e.scrollY {
		e.scrollY = caretTop - paddingY
	}
	if caretBottom+paddingY > e.scrollY+e.Size().Height {
		e.scrollY = caretBottom + paddingY - e.Size().Height
	}

	caretX := e.caretOffsetX()
	if caretX < e.scrollX {
		e.scrollX = caretX - e.textOriginX()
	}
	if caretX+caretW > e.scrollX+e.Size().Width-paddingX {
		e.scrollX = caretX + caretW + paddingX - e.Size().Width
	}
	e.clampScroll()
}

// caretOffsetX is the caret's x in document coordinates.
func (e *Editor) caretOffsetX() float32 {
	d := e.rowDisplay(e.doc.row)
	return e.textOriginX() + d.x(d.dispIndex(e.doc.col))
}

// visibleWidth is the widest visible line plus the gutter, used to clamp
// horizontal scrolling without measuring the whole document.
func (e *Editor) visibleWidth() float32 {
	first, last := e.visibleRange()
	width := e.textOriginX()
	for row := first; row <= last; row++ {
		d := e.rowDisplay(row)
		if w := e.textOriginX() + d.x(len(d.runes)); w > width {
			width = w
		}
	}
	return width + paddingX
}

// cellAt maps a click position inside the widget to a (row, rune column) cell.
func (e *Editor) cellAt(pos fyne.Position) (row, col int) {
	_, lineH := e.metrics()
	if lineH > 0 {
		row = int((pos.Y + e.scrollY - paddingY) / lineH)
	}
	row = clampInt(row, 0, e.doc.lineCount()-1)

	x := pos.X + e.scrollX - e.textOriginX()
	d := e.rowDisplay(row)
	col = d.rawIndex(d.nearestDisp(x))
	return row, clampInt(col, 0, runeLen(e.doc.line(row)))
}

// segments returns the styled byte ranges of a row: the .http lexer's segments
// for a source buffer, or the response segments for the read-only pane.
func (e *Editor) segments(row int) []seg {
	if e.responseMode {
		if row < 0 || row >= len(e.respSegs) {
			return nil
		}
		return e.respSegs[row]
	}
	return e.lex.line(row)
}

// rowDisplay returns the cached measured geometry of a row, building it on first
// use.
func (e *Editor) rowDisplay(row int) *rowDisplay {
	if d, ok := e.rows[row]; ok {
		return d
	}
	d := e.buildRowDisplay(e.doc.line(row))
	e.rows[row] = d
	return d
}

// runeWidth measures one rune in the monospace face, so wide glyphs (CJK,
// emoji) get their real advance instead of an assumed character width.
func (e *Editor) runeWidth(r rune) float32 {
	return fyne.MeasureText(string(r), e.fontSize, fyne.TextStyle{Monospace: true}).Width
}

// dropMetrics invalidates the per-row geometry cache.
func (e *Editor) dropMetrics() { clear(e.rows) }

func clampFloat(v, lo, hi float32) float32 {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

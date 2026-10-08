package tui

import (
	"strings"
	"unicode/utf8"

	"github.com/mattn/go-runewidth"
	"github.com/user/curlyk/internal/httpfile"
)

// normalizeNewlines rewrites CRLF and lone CR to LF so lines loaded from
// Windows-authored .http files never carry a stray carriage return into the
// renderer (a trailing \r would reset the terminal cursor to column 0 and
// corrupt the frame).
func normalizeNewlines(s string) string {
	if !strings.ContainsRune(s, '\r') {
		return s
	}
	s = strings.ReplaceAll(s, "\r\n", "\n")
	s = strings.ReplaceAll(s, "\r", "\n")
	return s
}

// editor is a minimal text editor: a buffer of lines, a cursor and viewport
// scroll. It renders lines through the .http syntax highlighter.
type editor struct {
	lines []string
	// cursor position
	curRow int // 0-based line index
	curCol int // column in runes (0-based)
	// onIcon means the cursor is on the run-icon gutter position (before col 0),
	// reachable by pressing Left at column 0 or Home. Enter runs the request there.
	onIcon bool
	// viewport
	scroll int // top visible line
	width  int
	height int // visible lines
	// wrapWidth is the display width (in cells) used to wrap long lines.
	wrapWidth int
	// hScroll is the horizontal scroll offset (in cells) for long lines.
	hScroll int
	// undo/redo stacks of editor snapshots.
	undo []editorSnap
	redo []editorSnap
	// undoDepth batches multiple primitive edits into a single undo step.
	undoDepth int
	// undoBase is the snapshot recorded when the current batch started.
	undoBase *editorSnap
}

// editorSnap is a full copy of the editor's undoable state (buffer + cursor).
type editorSnap struct {
	lines  []string
	curRow int
	curCol int
	onIcon bool
}

// snap returns a copy of the current mutable state.
func (e *editor) snap() editorSnap {
	cp := make([]string, len(e.lines))
	copy(cp, e.lines)
	return editorSnap{lines: cp, curRow: e.curRow, curCol: e.curCol, onIcon: e.onIcon}
}

// restore overwrites the editor state from a snapshot.
func (e *editor) restore(s editorSnap) {
	e.lines = s.lines
	e.curRow = s.curRow
	e.curCol = s.curCol
	e.onIcon = s.onIcon
}

// pushUndo records the current state before an edit, so Undo can return here.
// It clears the redo stack and skips duplicate consecutive snapshots. Edits
// inside a batch (undoDepth > 0) collapse onto the snapshot captured at batch
// start, so a compound edit (e.g. selection-replace or multi-line paste) undoes
// as a single step.
func (e *editor) pushUndo() {
	if e.undoDepth > 0 {
		// The very first mutation inside the batch records the batch-start
		// snapshot; later mutations in the same batch are absorbed.
		if e.undoBase != nil {
			e.recordSnapshot(*e.undoBase)
			e.undoBase = nil
		}
		return
	}
	if len(e.undo) > 0 {
		last := e.undo[len(e.undo)-1]
		if sameSnap(last, e.snap()) {
			return
		}
	}
	e.recordSnapshot(e.snap())
}

// recordSnapshot appends a snapshot to the undo stack, capping its size and
// clearing redo.
func (e *editor) recordSnapshot(s editorSnap) {
	e.undo = append(e.undo, s)
	const maxUndo = 200
	if len(e.undo) > maxUndo {
		e.undo = e.undo[len(e.undo)-maxUndo:]
	}
	e.redo = e.redo[:0]
}

// BeginUndo starts a batch: subsequent primitive edits coalesce into one undo
// step. It captures the current state but does not push it immediately.
func (e *editor) BeginUndo() {
	if e.undoDepth == 0 {
		e.undoBase = new(editorSnap)
		*e.undoBase = e.snap()
	}
	e.undoDepth++
}

// EndUndo closes an open undo batch. A batch that performed no edits leaves the
// history untouched.
func (e *editor) EndUndo() {
	if e.undoDepth > 0 {
		e.undoDepth--
	}
	if e.undoDepth == 0 {
		e.undoBase = nil
	}
}

// Undo reverts the most recent edit (one snapshot). Compound edits made through
// BeginUndo/EndUndo (paste, format, selection-replace, import) already collapse
// into a single step. Returns true when an action was undone.
func (e *editor) Undo() bool {
	if len(e.undo) == 0 {
		return false
	}
	// push current state onto redo, then restore the last snapshot.
	e.redo = append(e.redo, e.snap())
	s := e.undo[len(e.undo)-1]
	e.undo = e.undo[:len(e.undo)-1]
	e.restore(s)
	e.clampCol()
	return true
}

// Redo reapplies the most recently undone edit. Returns true when reapplied.
func (e *editor) Redo() bool {
	if len(e.redo) == 0 {
		return false
	}
	e.undo = append(e.undo, e.snap())
	s := e.redo[len(e.redo)-1]
	e.redo = e.redo[:len(e.redo)-1]
	e.restore(s)
	e.clampCol()
	return true
}

func sameSnap(a, b editorSnap) bool {
	if a.curRow != b.curRow || a.curCol != b.curCol || a.onIcon != b.onIcon || len(a.lines) != len(b.lines) {
		return false
	}
	for i := range a.lines {
		if a.lines[i] != b.lines[i] {
			return false
		}
	}
	return true
}

func newEditor(src string, width, height int) *editor {
	if src == "" {
		src = ""
	}
	lines := strings.Split(normalizeNewlines(src), "\n")
	return &editor{lines: lines, width: width, height: height}
}

// SetText replaces the whole content and resets the cursor. It records an undo
// snapshot of the previous buffer so Ctrl+Z can restore it.
func (e *editor) SetText(src string) {
	e.pushUndo()
	if src == "" {
		src = ""
	}
	e.lines = strings.Split(normalizeNewlines(src), "\n")
	e.curRow, e.curCol = 0, 0
	e.onIcon = false
	e.scroll = 0
}

// Text returns the full buffer content.
func (e *editor) Text() string {
	return strings.Join(e.lines, "\n")
}

// Lines exposes the buffer (read-only by convention).
func (e *editor) Lines() []string { return e.lines }

// MoveWord implements Ctrl+arrow word navigation.
func (e *editor) MoveWord(key string) {
	switch key {
	case "ctrl+left":
		e.moveWordLeft()
	case "ctrl+right":
		e.moveWordRight()
	case "ctrl+home":
		e.curRow, e.curCol = 0, 0
	case "ctrl+end":
		if len(e.lines) > 0 {
			e.curRow = len(e.lines) - 1
			e.curCol = e.lineLen(e.curRow)
		}
	}
}

func (e *editor) moveWordLeft() {
	for {
		runes := []rune(e.lines[e.curRow])
		// skip spaces to the left
		for e.curCol > 0 && (runes[e.curCol-1] == ' ' || runes[e.curCol-1] == '\t') {
			e.curCol--
		}
		if e.curCol > 0 {
			// step to start of the word
			for e.curCol > 0 {
				r := runes[e.curCol-1]
				if r == ' ' || r == '\t' {
					break
				}
				e.curCol--
			}
			return
		}
		// move to previous line end
		if e.curRow > 0 {
			e.curRow--
			e.curCol = e.lineLen(e.curRow)
		} else {
			return
		}
	}
}

func (e *editor) moveWordRight() {
	for {
		runes := []rune(e.lines[e.curRow])
		// skip spaces to the right
		for e.curCol < len(runes) && (runes[e.curCol] == ' ' || runes[e.curCol] == '\t') {
			e.curCol++
		}
		if e.curCol < len(runes) {
			// step to end of the word
			for e.curCol < len(runes) {
				r := runes[e.curCol]
				if r == ' ' || r == '\t' {
					break
				}
				e.curCol++
			}
			return
		}
		// move to next line start
		if e.curRow < len(e.lines)-1 {
			e.curRow++
			e.curCol = 0
		} else {
			return
		}
	}
}

// SelectedText returns the text covered by the range [anchor, cursor].
// The caller normalizes start/end. Returns "" if the selection is collapsed.
func (e *editor) SelectedText(anchorRow, anchorCol, curRow, curCol int) string {
	sr, sc, er, ec := anchorRow, anchorCol, curRow, curCol
	if er < sr || (er == sr && ec < sc) {
		sr, sc, er, ec = er, ec, sr, sc
	}
	if sr < 0 {
		sr = 0
	}
	if er >= len(e.lines) {
		er = len(e.lines) - 1
	}
	if er < sr {
		return ""
	}
	var b strings.Builder
	for r := sr; r <= er; r++ {
		line := e.lines[r]
		runes := []rune(line)
		cs, ce := 0, len(runes)
		if r == sr {
			cs = sc
			if cs > len(runes) {
				cs = len(runes)
			}
		}
		if r == er {
			ce = ec
			if ce > len(runes) {
				ce = len(runes)
			}
		}
		if ce < cs {
			ce = cs
		}
		b.WriteString(string(runes[cs:ce]))
		if r < er {
			b.WriteString("\n")
		}
	}
	return b.String()
}

// wordRangeInLine returns the rune-column [start,end) of the "word" at rune col c
// within a single line. Words are delimited by whitespace and .http punctuation
// characters. It is shared by the editor and the response pane so double-click
// word selection behaves identically in both.
func wordRangeInLine(line string, c int) (int, int) {
	runes := []rune(line)
	if len(runes) == 0 {
		return 0, 0
	}
	if c < 0 {
		c = 0
	}
	if c > len(runes) {
		c = len(runes)
	}
	// Define word chars: letters/digits/underscore/hyphen plus common URL
	// signs. Quotes, brackets, braces, commas etc. act as boundaries.
	isWord := func(r rune) bool {
		switch r {
		case ' ', '\t', '"', '\'', '(', ')', '[', ']', '{', '}', '<', '>', ',', ';', '\\':
			return false
		}
		return true
	}
	start := c
	for start > 0 && isWord(runes[start-1]) {
		start--
	}
	end := c
	for end < len(runes) && isWord(runes[end]) {
		end++
	}
	if start == end {
		// no word at cursor; still select one char
		if c < len(runes) {
			return c, c + 1
		}
		return c, c
	}
	return start, end
}

// wordRange returns the rune-column [start,end) of the "word" at rune col c.
// Words are delimited by whitespace and .http punctuation characters.
func (e *editor) wordRange(row, c int) (int, int) {
	if row < 0 || row >= len(e.lines) {
		return 0, 0
	}
	return wordRangeInLine(e.lines[row], c)
}

// DeleteRange removes the text in [anchor, cursor] and returns the removed text.
func (e *editor) DeleteRange(anchorRow, anchorCol, curRow, curCol int) string {
	sel := e.SelectedText(anchorRow, anchorCol, curRow, curCol)
	sr, sc, er, ec := anchorRow, anchorCol, curRow, curCol
	if er < sr || (er == sr && ec < sc) {
		sr, sc, er, ec = er, ec, sr, sc
	}
	if sel == "" {
		return ""
	}
	e.pushUndo()
	for r := er; r >= sr; r-- {
		if r < 0 || r >= len(e.lines) {
			continue
		}
		runes := []rune(e.lines[r])
		cs, ce := 0, len(runes)
		if r == sr {
			cs = sc
		}
		if r == er {
			ce = ec
		}
		if cs > len(runes) {
			cs = len(runes)
		}
		if ce > len(runes) {
			ce = len(runes)
		}
		if ce < cs {
			ce = cs
		}
		runes = append(runes[:cs], runes[ce:]...)
		e.lines[r] = string(runes)
	}
	// place cursor at start of deleted range
	e.curRow, e.curCol = sr, sc
	return sel
}
func (e *editor) lineLen(row int) int {
	if row < 0 || row >= len(e.lines) {
		return 0
	}
	return utf8.RuneCountInString(e.lines[row])
}

// clampCol ensures curCol is within the current line and exits icon mode.
func (e *editor) clampCol() {
	e.onIcon = false
	l := e.lineLen(e.curRow)
	if e.curCol > l {
		e.curCol = l
	}
	if e.curCol < 0 {
		e.curCol = 0
	}
}

// MoveCursor handles navigation keys.
func (e *editor) MoveCursor(key string) {
	switch key {
	case "up":
		if e.curRow > 0 {
			e.curRow--
			e.clampCol()
		}
	case "down":
		if e.curRow < len(e.lines)-1 {
			e.curRow++
			e.clampCol()
		}
	case "left":
		if e.onIcon {
			// already on the icon: on the first row there is nothing further left
			if e.curRow > 0 {
				// jump to end of the previous line
				e.curRow--
				e.curCol = e.lineLen(e.curRow)
				e.onIcon = false
			}
			// else stay on icon
		} else if e.curCol > 0 {
			e.curCol--
		} else if e.curRow > 0 {
			e.curRow--
			e.curCol = e.lineLen(e.curRow)
			e.onIcon = false
		} else {
			// at very start of buffer -> move to icon
			e.onIcon = true
			e.curCol = 0
		}
	case "right":
		if e.onIcon {
			e.onIcon = false
			e.curCol = 0
		} else if e.curCol < e.lineLen(e.curRow) {
			e.curCol++
		} else if e.curRow < len(e.lines)-1 {
			e.curRow++
			e.curCol = 0
		}
	case "home":
		if e.curCol > 0 {
			e.curCol = 0
		} else if !e.onIcon {
			e.onIcon = true
		}
	case "end":
		e.curCol = e.lineLen(e.curRow)
		e.onIcon = false
	}
}

// InsertRune inserts a rune at the cursor (handles multi-byte via string).
// sanitize removes NUL bytes from all lines (they corrupt rendering and break
// request detection). Call before lexing/rendering.
func (e *editor) sanitize() {
	for i, ln := range e.lines {
		if strings.IndexByte(ln, 0) >= 0 {
			e.lines[i] = strings.ReplaceAll(ln, "\x00", "")
		}
	}
}

func (e *editor) InsertString(s string) {
	// strip NUL bytes (they corrupt rendering and break request detection)
	if strings.IndexByte(s, 0) >= 0 {
		s = strings.ReplaceAll(s, "\x00", "")
	}
	// Strip stray carriage returns: InsertString is a single-line edit, so a
	// CR could not become a line break here; dropping it keeps CRLF pasted
	// content from leaking a \r into the rendered line.
	if strings.IndexByte(s, '\r') >= 0 {
		s = strings.ReplaceAll(s, "\r", "")
	}
	if s == "" {
		return
	}
	e.pushUndo()
	line := e.lines[e.curRow]
	// convert to runes
	runes := []rune(line)
	if e.curCol < 0 {
		e.curCol = 0
	}
	if e.curCol > len(runes) {
		e.curCol = len(runes)
	}
	insert := []rune(s)
	out := make([]rune, 0, len(runes)+len(insert))
	out = append(out, runes[:e.curCol]...)
	out = append(out, insert...)
	out = append(out, runes[e.curCol:]...)
	e.lines[e.curRow] = string(out)
	e.curCol += len(insert)
}

// Enter splits the line at the cursor.
func (e *editor) Enter() {
	e.pushUndo()
	line := e.lines[e.curRow]
	runes := []rune(line)
	if e.curCol > len(runes) {
		e.curCol = len(runes)
	}
	left := string(runes[:e.curCol])
	right := string(runes[e.curCol:])
	e.lines[e.curRow] = left
	// insert new line
	e.lines = append(e.lines, "")
	copy(e.lines[e.curRow+2:], e.lines[e.curRow+1:])
	e.lines[e.curRow+1] = right
	e.curRow++
	e.curCol = 0
}

// Backspace deletes the rune before the cursor.
func (e *editor) Backspace() {
	if e.curCol <= 0 && e.curRow <= 0 {
		return // nothing to delete
	}
	e.pushUndo()
	if e.curCol > 0 {
		line := e.lines[e.curRow]
		runes := []rune(line)
		e.lines[e.curRow] = string(append(runes[:e.curCol-1], runes[e.curCol:]...))
		e.curCol--
	} else if e.curRow > 0 {
		// join with previous line
		prev := e.lines[e.curRow-1]
		cur := e.lines[e.curRow]
		e.lines[e.curRow-1] = prev + cur
		e.curRow--
		e.curCol = len([]rune(prev))
		e.lines = append(e.lines[:e.curRow+1], e.lines[e.curRow+2:]...)
	}
}

// Delete removes the rune at the cursor.
func (e *editor) Delete() {
	// nothing to delete at the very end of the buffer
	if e.curCol >= e.lineLen(e.curRow) && e.curRow >= len(e.lines)-1 {
		return
	}
	e.pushUndo()
	line := e.lines[e.curRow]
	runes := []rune(line)
	if e.curCol < len(runes) {
		e.lines[e.curRow] = string(append(runes[:e.curCol], runes[e.curCol+1:]...))
	} else if e.curRow < len(e.lines)-1 {
		// join with next
		e.lines[e.curRow] = line + e.lines[e.curRow+1]
		e.lines = append(e.lines[:e.curRow+1], e.lines[e.curRow+2:]...)
	}
}

// DeleteLine removes the whole line under the cursor and moves the cursor to the
// start of the line that takes its place. If it was the last line the cursor
// moves to the end of the buffer. A no-op when the buffer has a single line.
func (e *editor) DeleteLine() {
	if len(e.lines) <= 1 {
		return
	}
	e.pushUndo()
	e.lines = append(e.lines[:e.curRow], e.lines[e.curRow+1:]...)
	if e.curRow >= len(e.lines) {
		e.curRow = len(e.lines) - 1
	}
	e.curCol = 0
}

// cursorLine returns the logical row = curRow - scroll offset.
func (e *editor) cursorLine() int { return e.curRow - e.scroll }

// contentWidth returns the visible line-text width in cells, accounting for the
// pane borders (2) and the run-icon + number gutter (5). It must match the
// contentW used in renderEditor so horizontal auto-scroll stays in sync with
// where lines are windowed.
func (e *editor) contentWidth() int {
	w := e.width - 2 - 5
	if w < 8 {
		w = 8
	}
	return w
}

// EnsureVisible scrolls the viewport so the cursor is visible both vertically
// and horizontally. For wide lines it auto-scrolls hScroll so the cursor column
// (in display cells) stays inside the visible content area, mirroring the way
// scroll keeps the cursor row visible.
func (e *editor) EnsureVisible() {
	if e.curRow < e.scroll {
		e.scroll = e.curRow
	}
	if e.curRow >= e.scroll+e.height {
		e.scroll = e.curRow - e.height + 1
	}
	if e.scroll < 0 {
		e.scroll = 0
	}
	e.hscrollToCursor()
}

// hscrollToCursor adjusts hScroll so the cursor column of the current line is
// visible within contentWidth cells. It is a no-op for lines shorter than the
// viewport.
func (e *editor) hscrollToCursor() {
	if e.curRow < 0 || e.curRow >= len(e.lines) {
		return
	}
	// Cursor cell column: display width of the line prefix before curCol.
	runes := []rune(e.lines[e.curRow])
	if e.curCol > len(runes) {
		e.curCol = len(runes)
	}
	if e.curCol < 0 {
		e.curCol = 0
	}
	cursor := runewidth.StringWidth(string(runes[:e.curCol]))

	cw := e.contentWidth()
	// Allow a small right margin so the cursor is not glued to the edge.
	margin := 2
	if cursor < e.hScroll {
		e.hScroll = cursor
		if e.hScroll < 0 {
			e.hScroll = 0
		}
	}
	if cursor >= e.hScroll+cw-margin {
		e.hScroll = cursor - cw + margin + 1
		if e.hScroll < 0 {
			e.hScroll = 0
		}
	}
}

// lineToks returns tokens for a 0-based line index.
func lineToksFor(idx int, toks []httpfile.Token) []httpfile.Token {
	var out []httpfile.Token
	for _, tk := range toks {
		if tk.Line-1 == idx {
			out = append(out, tk)
		}
	}
	return out
}

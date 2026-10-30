package gui

import (
	"strings"
)

// doc is the edited buffer plus the caret and the selection. Lines are stored
// without their terminators and columns are rune indexes, so the caret can never
// land inside a UTF-8 sequence and Cyrillic samples count as single characters.
//
// The selection is kept as an anchor plus the caret: the anchor stays where the
// selection started and the caret is the moving end, so extending a selection
// backwards works exactly like extending it forwards. selOn is false while
// nothing is selected, which makes a plain caret move clear the selection by
// construction.
type doc struct {
	lines   []string
	row     int
	col     int
	wantCol int // preferred column, kept while moving vertically

	anchorRow int
	anchorCol int
	selOn     bool

	history undoStack
}

func newDoc(src string) *doc {
	d := &doc{}
	d.setText(src)
	return d
}

func (d *doc) setText(src string) {
	d.lines = splitLines(src)
	d.row, d.col, d.wantCol = 0, 0, 0
	d.selClear()
	d.history.reset()
}

func (d *doc) text() string { return strings.Join(d.lines, "\n") }

func (d *doc) lineCount() int { return len(d.lines) }

func (d *doc) line(row int) string {
	if row < 0 || row >= len(d.lines) {
		return ""
	}
	return d.lines[row]
}

// setCaret places the caret at (row,col) without moving the selection anchor, so
// callers that extend a selection can reuse it.
func (d *doc) setCaret(row, col int) {
	d.row, d.col = row, col
	d.clamp()
	d.wantCol = d.col
}

// clamp pulls the caret into the buffer; every edit ends with it.
func (d *doc) clamp() {
	if d.row < 0 {
		d.row = 0
	}
	if d.row >= len(d.lines) {
		d.row = len(d.lines) - 1
	}
	if d.col < 0 {
		d.col = 0
	}
	if n := runeLen(d.lines[d.row]); d.col > n {
		d.col = n
	}
}

// insert places text at the caret, splitting the current line on newlines.
// It reports whether the buffer changed. Any selection is replaced by text.
func (d *doc) insert(text string) bool {
	if d.selActive() {
		d.selDelete()
	}
	if text == "" {
		return false
	}
	text = strings.ReplaceAll(text, "\r\n", "\n")
	text = strings.ReplaceAll(text, "\r", "\n")

	cur := []rune(d.lines[d.row])
	head := string(cur[:d.col])
	tail := string(cur[d.col:])

	merged := strings.Split(text, "\n")
	merged[0] = head + merged[0]
	merged[len(merged)-1] += tail

	if len(merged) > 1 {
		lines := make([]string, 0, len(d.lines)+len(merged)-1)
		lines = append(lines, d.lines[:d.row]...)
		lines = append(lines, merged...)
		lines = append(lines, d.lines[d.row+1:]...)
		d.lines = lines
		d.row += len(merged) - 1
		d.col = runeLen(merged[len(merged)-1]) - runeLen(tail)
	} else {
		d.lines[d.row] = merged[0]
		d.col += runeLen(text)
	}
	d.wantCol = d.col
	d.selClear()
	return true
}

// backspace deletes the selection, or the rune before the caret, joining lines
// at column 0.
func (d *doc) backspace() bool {
	if d.selDelete() {
		return true
	}
	if d.col > 0 {
		cur := []rune(d.lines[d.row])
		d.lines[d.row] = string(cur[:d.col-1]) + string(cur[d.col:])
		d.col--
		d.wantCol = d.col
		return true
	}
	if d.row == 0 {
		return false
	}
	d.col = runeLen(d.lines[d.row-1])
	d.lines[d.row-1] += d.lines[d.row]
	d.lines = append(d.lines[:d.row], d.lines[d.row+1:]...)
	d.row--
	d.wantCol = d.col
	return true
}

// deleteForward deletes the selection, or the rune after the caret, joining the
// next line at the end of the current one.
func (d *doc) deleteForward() bool {
	if d.selDelete() {
		return true
	}
	cur := []rune(d.lines[d.row])
	if d.col < len(cur) {
		d.lines[d.row] = string(cur[:d.col]) + string(cur[d.col+1:])
		return true
	}
	if d.row == len(d.lines)-1 {
		return false
	}
	d.lines[d.row] += d.lines[d.row+1]
	d.lines = append(d.lines[:d.row+1], d.lines[d.row+2:]...)
	return true
}

// canBackspace reports whether backspace would change the buffer, so the editor
// can avoid recording an undo step for a no-op.
func (d *doc) canBackspace() bool {
	return d.selActive() || d.col > 0 || d.row > 0
}

// canDeleteForward reports whether forward delete would change the buffer.
func (d *doc) canDeleteForward() bool {
	return d.selActive() || d.col < runeLen(d.lines[d.row]) || d.row < len(d.lines)-1
}

// moveLeft steps one rune back, wrapping to the previous line.
func (d *doc) moveLeft() bool {
	switch {
	case d.col > 0:
		d.col--
	case d.row > 0:
		d.row--
		d.col = runeLen(d.lines[d.row])
	default:
		return false
	}
	d.wantCol = d.col
	return true
}

// moveRight steps one rune forward, wrapping to the next line.
func (d *doc) moveRight() bool {
	end := runeLen(d.lines[d.row])
	switch {
	case d.col < end:
		d.col++
	case d.row < len(d.lines)-1:
		d.row++
		d.col = 0
	default:
		return false
	}
	d.wantCol = d.col
	return true
}

// moveVertical moves the caret a line up or down, keeping the preferred column.
func (d *doc) moveVertical(delta int) bool {
	row := d.row + delta
	if row < 0 || row >= len(d.lines) {
		return false
	}
	d.row = row
	d.col = d.wantCol
	d.clamp()
	return true
}

// pageVertical moves the caret a page up (negative delta) or down. A page that
// would run past the buffer lands on its edge instead of being refused: the very
// start upward, the very end downward. Without that, the caret would stay a page
// above the end of the file no matter how often PageDown is pressed.
func (d *doc) pageVertical(delta int) bool {
	if delta == 0 {
		return false
	}
	row := d.row + delta
	if row < 0 {
		return d.moveDocStart()
	}
	if row >= len(d.lines) {
		return d.moveDocEnd()
	}
	d.row = row
	d.col = d.wantCol
	d.clamp()
	return true
}

// moveLineStart moves the caret to the first rune of the line.
func (d *doc) moveLineStart() bool {
	if d.col == 0 {
		return false
	}
	d.col = 0
	d.wantCol = 0
	return true
}

// moveLineEnd moves the caret past the last rune of the line.
func (d *doc) moveLineEnd() bool {
	end := runeLen(d.lines[d.row])
	if d.col == end {
		return false
	}
	d.col = end
	d.wantCol = end
	return true
}

// moveWordLeft moves the caret to the start of the previous word, wrapping to
// the end of the previous line. Word characters are the ones selection treats as
// a word (letters, digits, underscore), so Ctrl+Left matches double-click.
func (d *doc) moveWordLeft() bool {
	if d.col == 0 {
		if d.row == 0 {
			return false
		}
		d.row--
		d.col = runeLen(d.lines[d.row])
		d.wantCol = d.col
		return true
	}
	runes := []rune(d.lines[d.row])
	i := d.col
	for i > 0 && !isWordRune(runes[i-1]) {
		i--
	}
	for i > 0 && isWordRune(runes[i-1]) {
		i--
	}
	if i == d.col {
		return false
	}
	d.col = i
	d.wantCol = i
	return true
}

// moveWordRight moves the caret past the end of the current word and then over
// the following separators to the start of the next word, wrapping to the next
// line.
func (d *doc) moveWordRight() bool {
	end := runeLen(d.lines[d.row])
	if d.col >= end {
		if d.row == len(d.lines)-1 {
			return false
		}
		d.row++
		d.col = 0
		d.wantCol = 0
		return true
	}
	runes := []rune(d.lines[d.row])
	i := d.col
	for i < end && isWordRune(runes[i]) {
		i++
	}
	for i < end && !isWordRune(runes[i]) {
		i++
	}
	if i == d.col {
		return false
	}
	d.col = i
	d.wantCol = i
	return true
}

// moveDocStart moves the caret to the very beginning.
func (d *doc) moveDocStart() bool {
	if d.row == 0 && d.col == 0 {
		return false
	}
	d.row, d.col, d.wantCol = 0, 0, 0
	return true
}

// moveDocEnd moves the caret to the very end.
func (d *doc) moveDocEnd() bool {
	last := len(d.lines) - 1
	end := runeLen(d.lines[last])
	if d.row == last && d.col == end {
		return false
	}
	d.row, d.col, d.wantCol = last, end, end
	return true
}

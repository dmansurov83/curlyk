package gui

import "strings"

// Selection helpers on doc. A selection is the range between the anchor (where
// it started) and the caret (the moving end); selOn records that a selection is
// in progress, which is what lets Shift+Home after Shift+Right collapse back to
// nothing instead of keeping a stale range.
//
// All ranges are inclusive-start / exclusive-end in (row, rune column).

// selClear drops the selection, keeping the caret where it is.
func (d *doc) selClear() { d.selOn = false }

// selAnchor marks the current caret as the selection anchor. Called on the first
// Shift+movement key, so the range grows from where the caret already was.
func (d *doc) selAnchor() {
	d.anchorRow, d.anchorCol = d.row, d.col
	d.selOn = true
}

// selSelectAll selects the whole buffer.
func (d *doc) selSelectAll() {
	d.anchorRow, d.anchorCol = 0, 0
	d.row = len(d.lines) - 1
	d.col = runeLen(d.lines[d.row])
	d.wantCol = d.col
	d.selOn = true
}

// selActive reports whether a non-empty range is selected.
func (d *doc) selActive() bool {
	return d.selOn && (d.anchorRow != d.row || d.anchorCol != d.col)
}

// selRange returns the normalized selection bounds (start before end). ok is
// false when nothing is selected.
func (d *doc) selRange() (r0, c0, r1, c1 int, ok bool) {
	if !d.selActive() {
		return 0, 0, 0, 0, false
	}
	if d.anchorRow < d.row || (d.anchorRow == d.row && d.anchorCol <= d.col) {
		return d.anchorRow, d.anchorCol, d.row, d.col, true
	}
	return d.row, d.col, d.anchorRow, d.anchorCol, true
}

// selText returns the selected text ("" when nothing is selected). Lines are
// joined with \n, so copying a multi-line selection yields the same text the
// file holds.
func (d *doc) selText() string {
	r0, c0, r1, c1, ok := d.selRange()
	if !ok {
		return ""
	}
	if r0 == r1 {
		return runeSlice(d.lines[r0], c0, c1)
	}
	var b strings.Builder
	b.WriteString(runeSlice(d.lines[r0], c0, runeLen(d.lines[r0])))
	b.WriteByte('\n')
	for r := r0 + 1; r < r1; r++ {
		b.WriteString(d.lines[r])
		b.WriteByte('\n')
	}
	b.WriteString(runeSlice(d.lines[r1], 0, c1))
	return b.String()
}

// selDelete removes the selected text and leaves the caret at its start. It
// reports whether anything was deleted.
func (d *doc) selDelete() bool {
	r0, c0, r1, c1, ok := d.selRange()
	if !ok {
		d.selClear()
		return false
	}
	tail := runeSlice(d.lines[r1], c1, runeLen(d.lines[r1]))
	head := runeSlice(d.lines[r0], 0, c0)

	rest := append([]string{head + tail}, d.lines[r1+1:]...)
	d.lines = append(d.lines[:r0], rest...)
	if len(d.lines) == 0 {
		d.lines = []string{""}
	}
	d.row, d.col = r0, c0
	d.clamp()
	d.wantCol = d.col
	d.selClear()
	return true
}

// selContainsCell reports whether a rune cell is inside the selection, used by
// the renderer to paint it.
func (d *doc) selContainsCell(row, col int) bool {
	r0, c0, r1, c1, ok := d.selRange()
	if !ok {
		return false
	}
	if row < r0 || row > r1 {
		return false
	}
	if row == r0 && col < c0 {
		return false
	}
	if row == r1 && col >= c1 {
		return false
	}
	return true
}

// selWordAt selects the word around (row,col). Word characters are letters,
// digits, '_' and '-'; anywhere else the single cell is selected, so a
// double-click on punctuation still selects something visible.
func (d *doc) selWordAt(row, col int) {
	if row < 0 || row >= len(d.lines) {
		return
	}
	runes := []rune(d.lines[row])
	if len(runes) == 0 {
		d.row, d.col = row, 0
		d.selClear()
		return
	}
	if col >= len(runes) {
		col = len(runes) - 1
	}

	if !isWordRune(runes[col]) {
		d.anchorRow, d.anchorCol = row, col
		d.row, d.col = row, col+1
		d.wantCol = d.col
		d.selOn = true
		return
	}
	start, end := col, col+1
	for start > 0 && isWordRune(runes[start-1]) {
		start--
	}
	for end < len(runes) && isWordRune(runes[end]) {
		end++
	}
	d.anchorRow, d.anchorCol = row, start
	d.row, d.col = row, end
	d.wantCol = d.col
	d.selOn = true
}

// selLineAt selects the whole line at row including its terminator, so copying
// a line and pasting it keeps line boundaries.
func (d *doc) selLineAt(row int) {
	if row < 0 || row >= len(d.lines) {
		return
	}
	d.anchorRow, d.anchorCol = row, 0
	d.row, d.col = row, runeLen(d.lines[row])
	if row < len(d.lines)-1 {
		d.row = row + 1
		d.col = 0
	}
	d.wantCol = d.col
	d.selOn = true
}

// isWordRune reports whether r belongs to a word for double-click selection.
func isWordRune(r rune) bool {
	switch {
	case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9':
		return true
	case r == '_' || r == '-' || r == '.':
		return true
	}
	// Treat any non-ASCII letter (Cyrillic included) as a word rune.
	return r > 127
}

// runeSlice returns the runes of s in [from,to), clamped to the string bounds.
func runeSlice(s string, from, to int) string {
	runes := []rune(s)
	if from < 0 {
		from = 0
	}
	if to > len(runes) {
		to = len(runes)
	}
	if from >= to {
		return ""
	}
	return string(runes[from:to])
}

// runePrefix returns the runes of s before col, clamped.
func runePrefix(s string, col int) string { return runeSlice(s, 0, col) }

// runeSuffix returns the runes of s from col onward, clamped.
func runeSuffix(s string, col int) string { return runeSlice(s, col, runeLen(s)) }

package gui

import (
	"slices"
	"strings"
)

// highlighter keeps the per-line syntax segments of the buffer and refreshes
// only what an edit can have changed.
//
// The .http lexer carries state along the file (inside headers / inside a body),
// but it resets that state on blank lines and on "###" separators, so those
// lines are block boundaries. A block delimited by boundaries tokenizes
// identically whether it is lexed alone or as part of the whole file. That is
// what makes an incremental update exact: re-lex the block around the edit and
// reuse the segments of every other line.
type highlighter struct {
	lines []string
	segs  [][]seg
}

func newHighlighter() *highlighter { return &highlighter{} }

// setText replaces the buffer and re-lexes everything.
func (h *highlighter) setText(src string) {
	h.lines = splitLines(src)
	h.segs = highlightRange(h.lines, 0, len(h.lines))
}

// remember stores the line slice being lexed. The slice is copied because the
// editor passes the document's own slice and mutates it in place: keeping a
// reference would make "what changed" compare the new text against itself and
// silently keep stale segments.
func (h *highlighter) remember(lines []string) {
	h.lines = slices.Clone(lines)
}

// line returns the segments of a row, or nil when the row is unknown.
func (h *highlighter) line(row int) []seg {
	if row < 0 || row >= len(h.segs) {
		return nil
	}
	return h.segs[row]
}

// update re-lexes the smallest block affected by a change from the previous
// buffer to lines. It is a plain call with the editor's current line slice, so
// the editor does not have to track what it changed.
func (h *highlighter) update(lines []string) {
	if h.lines == nil || len(h.segs) != len(h.lines) {
		h.remember(lines)
		h.segs = highlightRange(lines, 0, len(lines))
		return
	}
	if len(lines) == 0 {
		h.remember(lines)
		h.segs = nil
		return
	}

	first := 0
	for first < len(h.lines) && first < len(lines) && h.lines[first] == lines[first] {
		first++
	}
	if first == len(h.lines) && first == len(lines) {
		return // no change
	}

	// Common suffix, not counting the prefix already matched.
	last := 0
	for last < len(h.lines)-first && last < len(lines)-first &&
		h.lines[len(h.lines)-1-last] == lines[len(lines)-1-last] {
		last++
	}

	oldLines := h.lines
	oldSegs := h.segs
	h.remember(lines)

	segs := make([][]seg, len(lines))
	for i := 0; i < first; i++ {
		segs[i] = oldSegs[i]
	}
	for k := 0; k < last; k++ {
		segs[len(lines)-last+k] = oldSegs[len(oldLines)-last+k]
	}

	from := blockStart(lines, first)
	to := blockEnd(lines, len(lines)-last)
	fresh := highlightRange(lines, from, to)
	for i := from; i < to; i++ {
		segs[i] = fresh[i]
	}
	h.segs = segs
}

// isBlockBoundary reports whether a line resets the lexer state, which makes it
// safe to start a partial re-lex there.
func isBlockBoundary(line string) bool {
	if strings.TrimSpace(line) == "" {
		return true
	}
	return strings.HasPrefix(line, "###")
}

// blockStart returns the first row of the block containing row: the row after
// the previous boundary.
func blockStart(lines []string, row int) int {
	for i := row; i > 0; i-- {
		if isBlockBoundary(lines[i-1]) {
			return i
		}
	}
	return 0
}

// blockEnd returns the exclusive end of the block containing row: the index of
// the next boundary line.
func blockEnd(lines []string, row int) int {
	for i := row; i < len(lines); i++ {
		if isBlockBoundary(lines[i]) {
			return i
		}
	}
	return len(lines)
}

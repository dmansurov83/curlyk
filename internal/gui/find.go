package gui

import (
	"github.com/user/curlyk/internal/search"
)

// findState is the search over the buffer: the query, its matches in reading
// order and the index of the current one. The matching itself is the shared,
// UI-free internal/search model, so the GUI and the TUI find the same text.
type findState struct {
	query   string
	matches []search.Match
	cur     int
}

// SetFindQuery sets the search query and recomputes the matches. The current
// match snaps to the first one at or after the caret, so typing a query keeps
// the view where the user is working.
func (e *Editor) SetFindQuery(query string) {
	e.find.query = query
	e.find.matches = search.Find(e.doc.lines, query)
	e.find.cur = 0
	if query != "" {
		for i, m := range e.find.matches {
			if m.Line > e.doc.row || (m.Line == e.doc.row && m.Col >= e.doc.col) {
				e.find.cur = i
				break
			}
		}
		e.revealMatch()
	}
	e.Refresh()
}

// FindQuery returns the active query ("" when no search is running).
func (e *Editor) FindQuery() string { return e.find.query }

// FindMatches returns the number of matches of the active query.
func (e *Editor) FindMatches() int { return len(e.find.matches) }

// FindCurrent returns the 1-based index of the current match, or 0 when there
// are none.
func (e *Editor) FindCurrent() int {
	if len(e.find.matches) == 0 {
		return 0
	}
	return e.find.cur + 1
}

// ClearFind drops the search and its highlights.
func (e *Editor) ClearFind() {
	if e.find.query == "" && e.find.matches == nil {
		return
	}
	e.find = findState{}
	e.Refresh()
}

// FindNext moves to the next match, wrapping at the end, and scrolls it into
// view.
func (e *Editor) FindNext() {
	if len(e.find.matches) == 0 {
		return
	}
	e.find.cur = (e.find.cur + 1) % len(e.find.matches)
	e.revealMatch()
	e.Refresh()
}

// FindPrev moves to the previous match, wrapping at the start.
func (e *Editor) FindPrev() {
	if len(e.find.matches) == 0 {
		return
	}
	e.find.cur--
	if e.find.cur < 0 {
		e.find.cur = len(e.find.matches) - 1
	}
	e.revealMatch()
	e.Refresh()
}

// revealMatch puts the caret on the current match and scrolls it into view. The
// caret is moved without an undo step: searching is navigation, not editing.
func (e *Editor) revealMatch() {
	if e.find.cur < 0 || e.find.cur >= len(e.find.matches) {
		return
	}
	m := e.find.matches[e.find.cur]
	e.doc.setCaret(m.Line, m.Col)
	e.doc.selClear()
	e.doc.breakUndoRun()
	e.ensureCaretVisible()
	// A match past the right edge also needs a horizontal reveal.
	e.ensureMatchVisible(m)
	e.caretMoved()
}

// ensureMatchVisible scrolls horizontally when the match sits beyond the right
// edge of the viewport.
func (e *Editor) ensureMatchVisible(m search.Match) {
	d := e.rowDisplay(m.Line)
	startX := e.textOriginX() + d.x(d.dispIndex(m.Col))
	endX := e.textOriginX() + d.x(d.dispIndex(m.Col+runeLen(e.find.query)))
	view := e.Size().Width

	if endX-e.scrollX > view-paddingX {
		e.scrollX = endX + paddingX - view
	}
	if startX-e.scrollX < paddingX {
		e.scrollX = startX - paddingX
	}
	e.clampScroll()
}

// findMarksRow returns the search highlights of a row, with the current match
// marked distinctly.
func (e *Editor) findMarksRow(row int) []mark {
	if e.find.query == "" || len(e.find.matches) == 0 {
		return nil
	}
	width := runeLen(e.find.query)
	var out []mark
	for i, m := range e.find.matches {
		if m.Line != row {
			continue
		}
		kind := markFind
		if i == e.find.cur {
			kind = markFindCurrent
		}
		out = append(out, mark{from: m.Col, to: m.Col + width, kind: kind})
	}
	return out
}

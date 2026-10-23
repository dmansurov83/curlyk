package gui

import "slices"

// Undo/redo by snapshots of the buffer plus the caret and the selection. .http
// files are small enough that copying the line slice per edit is cheaper than
// maintaining an operation log, and it keeps undo exact: restoring a snapshot
// cannot drift from what the user saw.
//
// Consecutive typed characters are coalesced into a single step, so Ctrl+Z after
// typing a word removes the word rather than one letter at a time. Any other
// edit (paste, delete, Enter, Tab, cut) starts a new step.

// maxUndoSteps caps the history so a long editing session cannot grow without
// bound.
const maxUndoSteps = 500

// Kinds of edit used for coalescing.
const (
	editType  = "type"
	editOther = "edit"
)

// docState is one restorable snapshot of the buffer and the caret.
type docState struct {
	lines     []string
	row, col  int
	wantCol   int
	anchorRow int
	anchorCol int
	selOn     bool
}

// undoStack is the past/future pair of snapshots.
type undoStack struct {
	past    []docState
	future  []docState
	typedIn bool // a typing run is open; further typed runes fold into it
}

func (u *undoStack) reset() {
	u.past = nil
	u.future = nil
	u.typedIn = false
}

// state captures the current buffer and caret.
func (d *doc) state() docState {
	return docState{
		lines:     slices.Clone(d.lines),
		row:       d.row,
		col:       d.col,
		wantCol:   d.wantCol,
		anchorRow: d.anchorRow,
		anchorCol: d.anchorCol,
		selOn:     d.selOn,
	}
}

// restore puts a captured state back into the buffer.
func (d *doc) restore(s docState) {
	d.lines = slices.Clone(s.lines)
	d.row, d.col, d.wantCol = s.row, s.col, s.wantCol
	d.anchorRow, d.anchorCol, d.selOn = s.anchorRow, s.anchorCol, s.selOn
	d.clamp()
}

// pushUndo records the current state as an undo step. It must be called before
// the mutation it protects. kind is editType for typed characters, which are
// folded into one step, and editOther for everything else.
func (d *doc) pushUndo(kind string) {
	if kind == editType && d.history.typedIn {
		// The typing run is already covered by an earlier snapshot: keep that one
		// so the whole run undoes at once.
		d.history.future = nil
		return
	}
	d.history.past = append(d.history.past, d.state())
	if len(d.history.past) > maxUndoSteps {
		d.history.past = d.history.past[len(d.history.past)-maxUndoSteps:]
	}
	d.history.future = nil
	d.history.typedIn = kind == editType
}

// breakUndoRun ends an open typing run, so the next typed character starts a new
// undo step. Caret movement and non-typing edits both end it.
func (d *doc) breakUndoRun() { d.history.typedIn = false }

// canUndo reports whether there is something to undo.
func (d *doc) canUndo() bool { return len(d.history.past) > 0 }

// canRedo reports whether there is something to redo.
func (d *doc) canRedo() bool { return len(d.history.future) > 0 }

// undo restores the previous state and reports whether anything changed.
func (d *doc) undo() bool {
	if !d.canUndo() {
		return false
	}
	last := d.history.past[len(d.history.past)-1]
	d.history.past = d.history.past[:len(d.history.past)-1]
	d.history.future = append(d.history.future, d.state())
	d.history.typedIn = false
	d.restore(last)
	return true
}

// redo re-applies an undone state and reports whether anything changed.
func (d *doc) redo() bool {
	if !d.canRedo() {
		return false
	}
	next := d.history.future[len(d.history.future)-1]
	d.history.future = d.history.future[:len(d.history.future)-1]
	d.history.past = append(d.history.past, d.state())
	d.history.typedIn = false
	d.restore(next)
	return true
}

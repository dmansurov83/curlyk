package gui

import "testing"

func TestUndoRedoRoundTrip(t *testing.T) {
	d := newDoc("abc")
	d.pushUndo(editOther)
	d.insert("X")

	if !d.undo() {
		t.Fatal("undo reported no change")
	}
	if got := d.text(); got != "abc" {
		t.Fatalf("after undo text = %q, want abc", got)
	}
	if !d.redo() {
		t.Fatal("redo reported no change")
	}
	if got := d.text(); got != "Xabc" {
		t.Fatalf("after redo text = %q, want Xabc", got)
	}
	if d.redo() {
		t.Error("redo past the end should report no change")
	}
}

// TestTypingCoalescesIntoOneUndoStep covers the deliberate behaviour that a run
// of typed characters undoes as a word rather than letter by letter.
func TestTypingCoalescesIntoOneUndoStep(t *testing.T) {
	d := newDoc("")
	for _, r := range "GET" {
		d.pushUndo(editType)
		d.insert(string(r))
	}

	if !d.undo() {
		t.Fatal("undo reported no change")
	}
	if got := d.text(); got != "" {
		t.Fatalf("after undo text = %q, want the whole typing run removed", got)
	}
	if d.undo() {
		t.Error("the typing run should be a single step")
	}
}

func TestCaretMoveBreaksTypingRun(t *testing.T) {
	d := newDoc("")
	d.pushUndo(editType)
	d.insert("a")
	d.breakUndoRun() // what a caret move does
	d.pushUndo(editType)
	d.insert("b")

	d.undo()
	if got := d.text(); got != "a" {
		t.Fatalf("after first undo text = %q, want a", got)
	}
	d.undo()
	if got := d.text(); got != "" {
		t.Fatalf("after second undo text = %q, want empty", got)
	}
}

func TestNewEditClearsRedo(t *testing.T) {
	d := newDoc("a")
	d.pushUndo(editOther)
	d.insert("b")
	d.undo()
	if !d.canRedo() {
		t.Fatal("expected a redo step after undo")
	}
	d.pushUndo(editOther)
	d.insert("c")
	if d.canRedo() {
		t.Error("a new edit must clear the redo stack")
	}
}

// TestUndoRestoresSelectionAndCaret guards that an undo puts the caret (and any
// selection) back where the edit started, not merely the text.
func TestUndoRestoresSelectionAndCaret(t *testing.T) {
	d := newDoc("hello world")
	d.anchorRow, d.anchorCol = 0, 0
	d.row, d.col, d.selOn = 0, 5, true

	d.pushUndo(editOther)
	d.insert("X") // replaces the selection

	if got := d.text(); got != "X world" {
		t.Fatalf("text = %q, want \"X world\"", got)
	}
	d.undo()
	if got := d.text(); got != "hello world" {
		t.Fatalf("after undo text = %q", got)
	}
	if !d.selActive() || d.selText() != "hello" {
		t.Fatalf("selection not restored: active=%v text=%q", d.selActive(), d.selText())
	}
	if d.row != 0 || d.col != 5 {
		t.Fatalf("caret = (%d,%d), want (0,5)", d.row, d.col)
	}
}

func TestUndoStackIsBounded(t *testing.T) {
	d := newDoc("")
	for i := 0; i < maxUndoSteps+50; i++ {
		d.pushUndo(editOther)
		d.insert("x")
	}
	if len(d.history.past) > maxUndoSteps {
		t.Fatalf("history holds %d steps, want at most %d", len(d.history.past), maxUndoSteps)
	}
}

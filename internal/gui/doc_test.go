package gui

import "testing"

func TestDocInsertSplitsLines(t *testing.T) {
	d := newDoc("ab\ncd")
	d.row, d.col = 0, 1

	if !d.insert("X\nY") {
		t.Fatal("insert reported no change")
	}
	if got, want := d.text(), "aX\nYb\ncd"; got != want {
		t.Fatalf("text = %q, want %q", got, want)
	}
	if d.row != 1 || d.col != 1 {
		t.Fatalf("caret = (%d,%d), want (1,1)", d.row, d.col)
	}
}

func TestDocBackspaceJoinsLines(t *testing.T) {
	d := newDoc("Привет\nмир")
	d.row, d.col = 1, 0

	if !d.backspace() {
		t.Fatal("backspace reported no change")
	}
	if got, want := d.text(), "Приветмир"; got != want {
		t.Fatalf("text = %q, want %q", got, want)
	}
	if d.row != 0 || d.col != 6 {
		t.Fatalf("caret = (%d,%d), want (0,6)", d.row, d.col)
	}
}

func TestDocBackspaceAndDeleteAtBufferEdges(t *testing.T) {
	d := newDoc("one line")
	if d.backspace() {
		t.Error("backspace at the very start should report no change")
	}
	d.moveDocEnd()
	if d.deleteForward() {
		t.Error("delete at the very end should report no change")
	}
	if got, want := d.text(), "one line"; got != want {
		t.Fatalf("text = %q, want %q", got, want)
	}
}

func TestDocDeleteForwardJoinsLines(t *testing.T) {
	d := newDoc("ab\ncd")
	d.moveLineEnd()

	if !d.deleteForward() {
		t.Fatal("delete reported no change")
	}
	if got, want := d.text(), "abcd"; got != want {
		t.Fatalf("text = %q, want %q", got, want)
	}
	if d.row != 0 || d.col != 2 {
		t.Fatalf("caret = (%d,%d), want (0,2)", d.row, d.col)
	}
}

// TestDocVerticalKeepsColumn covers the usual editor behaviour: moving down a
// short line and back restores the original column.
func TestDocVerticalKeepsColumn(t *testing.T) {
	d := newDoc("long line\nx\nanother long line")
	d.row, d.col, d.wantCol = 0, 8, 8

	if !d.moveVertical(1) {
		t.Fatal("move down reported no change")
	}
	if d.col != 1 {
		t.Fatalf("col on the short line = %d, want 1", d.col)
	}
	if !d.moveVertical(1) {
		t.Fatal("second move down reported no change")
	}
	if d.col != 8 {
		t.Fatalf("col restored as %d, want 8", d.col)
	}
	d.moveVertical(-2)
	if d.row != 0 || d.col != 8 {
		t.Fatalf("caret = (%d,%d), want (0,8)", d.row, d.col)
	}
	if d.moveVertical(-1) {
		t.Error("moving above the first line should report no change")
	}
}

func TestDocHorizontalWrap(t *testing.T) {
	d := newDoc("ab\ncd")
	d.moveLineEnd()
	if !d.moveRight() {
		t.Fatal("move right at end of line should wrap")
	}
	if d.row != 1 || d.col != 0 {
		t.Fatalf("caret = (%d,%d), want (1,0)", d.row, d.col)
	}
	if !d.moveLeft() {
		t.Fatal("move left at start of line should wrap")
	}
	if d.row != 0 || d.col != 2 {
		t.Fatalf("caret = (%d,%d), want (0,2)", d.row, d.col)
	}
}

func TestDocCyrillicColumnsAreRunes(t *testing.T) {
	d := newDoc("Запрос")
	d.moveLineEnd()
	if d.col != 6 {
		t.Fatalf("col = %d, want 6 runes", d.col)
	}
	d.col = 3
	if !d.backspace() {
		t.Fatal("backspace reported no change")
	}
	if got, want := d.text(), "Зарос"; got != want {
		t.Fatalf("text = %q, want %q", got, want)
	}
}

func TestDocWordMovement(t *testing.T) {
	d := newDoc("GET http://example.test/x\nnext line\n")
	d.setCaret(0, 0)

	if !d.moveWordRight() || d.col != 4 {
		t.Fatalf("first Ctrl+Right = row %d col %d, want 0:4 (start of http)", d.row, d.col)
	}
	d.moveWordRight()
	if d.col != 11 {
		t.Fatalf("second Ctrl+Right col = %d, want 11 (example)", d.col)
	}
	d.moveWordRight()
	if d.col != 24 {
		t.Fatalf("third Ctrl+Right col = %d, want 24 (x)", d.col)
	}

	// "." is a word rune (double-click treats example.test as one word), so the
	// word starts at the host name.
	if !d.moveWordLeft() || d.col != 11 {
		t.Fatalf("Ctrl+Left col = %d, want 11 (start of example.test)", d.col)
	}
	d.setCaret(0, 0)
	if d.moveWordLeft() {
		t.Fatal("Ctrl+Left at the start of the file moved the caret")
	}
	d.setCaret(1, 0)
	if !d.moveWordLeft() || d.row != 0 || d.col != runeLen("GET http://example.test/x") {
		t.Fatalf("Ctrl+Left at a line start = %d:%d, want the end of the previous line", d.row, d.col)
	}
}

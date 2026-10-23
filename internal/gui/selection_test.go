package gui

import "testing"

func TestSelTextAcrossLines(t *testing.T) {
	d := newDoc("GET http://a\nHost: h\nbody")
	d.anchorRow, d.anchorCol = 0, 4
	d.row, d.col, d.selOn = 2, 2, true

	if got, want := d.selText(), "http://a\nHost: h\nbo"; got != want {
		t.Fatalf("selText = %q, want %q", got, want)
	}
}

// TestSelTextIsOrderIndependent covers a backwards selection (anchor below the
// caret), which happens whenever a selection is dragged upwards.
func TestSelTextIsOrderIndependent(t *testing.T) {
	d := newDoc("one\ntwo\nthree")
	d.anchorRow, d.anchorCol = 2, 5
	d.row, d.col, d.selOn = 0, 0, true

	if got, want := d.selText(), "one\ntwo\nthree"; got != want {
		t.Fatalf("selText = %q, want %q", got, want)
	}
}

func TestSelDeleteJoinsLines(t *testing.T) {
	d := newDoc("GET http://a\nHost: h\nbody")
	d.anchorRow, d.anchorCol = 0, 3
	d.row, d.col, d.selOn = 1, 4, true

	if !d.selDelete() {
		t.Fatal("selDelete reported no change")
	}
	if got, want := d.text(), "GET: h\nbody"; got != want {
		t.Fatalf("text = %q, want %q", got, want)
	}
	if d.row != 0 || d.col != 3 {
		t.Fatalf("caret = (%d,%d), want (0,3)", d.row, d.col)
	}
	if d.selActive() {
		t.Error("selection should be cleared after deletion")
	}
}

func TestSelContainsCell(t *testing.T) {
	d := newDoc("abcdef\nghijkl")
	d.anchorRow, d.anchorCol = 0, 2
	d.row, d.col, d.selOn = 1, 3, true

	cases := []struct {
		row, col int
		want     bool
	}{
		{0, 1, false},
		{0, 2, true},
		{0, 5, true},
		{1, 0, true},
		{1, 2, true},
		{1, 3, false},
		{2, 0, false},
	}
	for _, c := range cases {
		if got := d.selContainsCell(c.row, c.col); got != c.want {
			t.Errorf("selContainsCell(%d,%d) = %v, want %v", c.row, c.col, got, c.want)
		}
	}
}

func TestSelWordAt(t *testing.T) {
	d := newDoc("Authorization: Bearer {{token}}")
	d.selWordAt(0, 5)
	if got := d.selText(); got != "Authorization" {
		t.Fatalf("word = %q, want Authorization", got)
	}

	// Cyrillic text is one word as well.
	d = newDoc("Хост: сервер")
	d.selWordAt(0, 7)
	if got := d.selText(); got != "сервер" {
		t.Fatalf("word = %q, want сервер", got)
	}

	// A double-click on punctuation selects that one cell.
	d = newDoc("a: b")
	d.selWordAt(0, 1)
	if got := d.selText(); got != ":" {
		t.Fatalf("punctuation = %q, want :", got)
	}

	// Past the end of the line the last rune's word is selected.
	d = newDoc("GET x")
	d.selWordAt(0, 99)
	if got := d.selText(); got != "x" {
		t.Fatalf("word at end = %q, want x", got)
	}
}

func TestSelLineAtIncludesTerminator(t *testing.T) {
	d := newDoc("first\nsecond\nthird")
	d.selLineAt(1)
	if got, want := d.selText(), "second\n"; got != want {
		t.Fatalf("selText = %q, want %q", got, want)
	}

	// The last line has no terminator to include.
	d = newDoc("only")
	d.selLineAt(0)
	if got, want := d.selText(), "only"; got != want {
		t.Fatalf("selText = %q, want %q", got, want)
	}
}

func TestSelectAllAndInsertReplaces(t *testing.T) {
	d := newDoc("GET http://a\nHost: h")
	d.selSelectAll()
	if !d.selActive() {
		t.Fatal("select all produced no selection")
	}
	if !d.insert("new") {
		t.Fatal("insert reported no change")
	}
	if got, want := d.text(), "new"; got != want {
		t.Fatalf("text = %q, want %q", got, want)
	}
	if d.selActive() {
		t.Error("typing over a selection should clear it")
	}
}

// TestBackspaceDeletesSelectionFirst pins the usual editor contract: with a
// selection, backspace removes the selection and leaves the rest alone.
func TestBackspaceDeletesSelectionFirst(t *testing.T) {
	d := newDoc("GET http://a")
	d.anchorRow, d.anchorCol = 0, 4
	d.row, d.col, d.selOn = 0, 11, true

	if !d.backspace() {
		t.Fatal("backspace reported no change")
	}
	if got, want := d.text(), "GET a"; got != want {
		t.Fatalf("text = %q, want %q", got, want)
	}
}

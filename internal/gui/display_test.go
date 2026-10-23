package gui

import (
	"testing"
	"unicode/utf8"
)

// TestRowDisplayExpandsTabsAndMapsColumns covers the tab handling: the buffer
// keeps the tab, the drawing and the caret use the expanded columns, and the
// two mappings stay consistent.
func TestRowDisplayExpandsTabsAndMapsColumns(t *testing.T) {
	e := newTestEditor(t, "a\tb")
	d := e.rowDisplay(0)

	// "a" then a tab out to column 4, then "b".
	if got := string(d.runes); got != "a   b" {
		t.Fatalf("display runes = %q, want \"a   b\"", got)
	}
	if got := len(d.runes); got != 5 {
		t.Fatalf("display width = %d runes, want 5", got)
	}
	// Raw rune columns: 0='a', 1='\t', 2='b', 3=end.
	if got := d.dispIndex(0); got != 0 {
		t.Errorf("dispIndex(0) = %d, want 0", got)
	}
	if got := d.dispIndex(1); got != 1 {
		t.Errorf("dispIndex(tab) = %d, want 1", got)
	}
	if got := d.dispIndex(2); got != 4 {
		t.Errorf("dispIndex('b') = %d, want 4", got)
	}
	if got := d.rawIndex(4); got != 2 {
		t.Errorf("rawIndex(4) = %d, want 2 ('b')", got)
	}
	if got := d.rawIndex(len(d.runes)); got != 3 {
		t.Errorf("rawIndex(end) = %d, want 3", got)
	}
	// Offsets are monotonic, so a click can binary-search them.
	for i := 1; i < len(d.offsets); i++ {
		if d.offsets[i] < d.offsets[i-1] {
			t.Fatalf("offsets not monotonic at %d: %v", i, d.offsets)
		}
	}
}

// TestRowDisplayMapsByteOffsets pins the byte→display bridge used to convert the
// .http lexer's byte ranges into drawn rune ranges, with non-ASCII text.
func TestRowDisplayMapsByteOffsets(t *testing.T) {
	e := newTestEditor(t, "Заголовок: значение")
	d := e.rowDisplay(0)

	// Byte offset 0 is display column 0; the cyrillic run is one byte per two, so
	// a byte offset that lands mid-rune must not run ahead of its rune.
	if got := d.dispOfByte(0); got != 0 {
		t.Errorf("dispOfByte(0) = %d, want 0", got)
	}
	if got := d.dispOfByte(len("Заголовок")); got != runeLen("Заголовок") {
		t.Errorf("dispOfByte(end of the word) = %d, want %d", got, runeLen("Заголовок"))
	}
	if got := d.dispOfByte(len(e.doc.line(0))); got != runeLen(e.doc.line(0)) {
		t.Errorf("dispOfByte(line end) = %d, want %d", got, runeLen(e.doc.line(0)))
	}
	// Every byte offset inside a rune maps to that rune's column: a segment
	// boundary that lands mid-rune must not run ahead of the glyph it belongs to.
	word := "Заголовок"
	col := 0
	for bi := 0; bi < len(word); {
		_, size := utf8.DecodeRuneInString(word[bi:])
		if got, want := d.dispOfByte(bi), col; got != want {
			t.Fatalf("dispOfByte(%d) = %d, want %d", bi, got, want)
		}
		for k := bi + 1; k < bi+size; k++ {
			if got := d.dispOfByte(k); got != col {
				t.Fatalf("dispOfByte(%d) inside a rune = %d, want %d", k, got, col)
			}
		}
		bi += size
		col++
	}
}

func TestRowDisplayNearestDispSnapsToClosestBoundary(t *testing.T) {
	e := newTestEditor(t, "abcdef")
	d := e.rowDisplay(0)

	justInsideC := d.x(2) + (d.x(3)-d.x(2))/4
	if got := d.nearestDisp(justInsideC); got != 2 {
		t.Fatalf("click near the start of 'c' snapped to %d, want 2", got)
	}
	justBeforeD := d.x(3) - (d.x(3)-d.x(2))/4
	if got := d.nearestDisp(justBeforeD); got != 3 {
		t.Fatalf("click near the end of 'c' snapped to %d, want 3", got)
	}
	if got := d.nearestDisp(-10); got != 0 {
		t.Fatalf("click left of the text snapped to %d, want 0", got)
	}
	if got := d.nearestDisp(d.x(99)); got != len(d.runes) {
		t.Fatalf("click right of the text snapped to %d, want %d", got, len(d.runes))
	}
}

package tui

import "testing"

func TestSelectedTextMultiLine(t *testing.T) {
	ed := newEditor("GET /x\nAccept: json\n", 40, 10)
	// line0 = "GET /x" (7 runes). Select from (0,4)=" /x..." to (1,7)="Accept:"
	// - row0 col4..7 => "/x"
	// - "\n"
	// - row1 col0..7 => "Accept:"
	got := ed.SelectedText(0, 4, 1, 7)
	want := "/x\nAccept:"
	if got != want {
		t.Errorf("got %q want %q", got, want)
	}
}

func TestSelectedTextReverseOrder(t *testing.T) {
	ed := newEditor("hello world\n", 40, 10)
	// anchor after cursor
	got := ed.SelectedText(0, 5, 0, 0)
	if got != "hello" {
		t.Errorf("got %q want %q", got, "hello")
	}
}

func TestWordRange(t *testing.T) {
	ed := newEditor("GET https://x.com/1 Accept\n", 40, 10)
	// word at col 1 = "ET"? Actually wordRange includes only word chars (non-space).
	// "GET" spans runes 0..2.
	s, e := ed.wordRange(0, 0)
	if s != 0 || e != 3 {
		t.Errorf("GET range=%d..%d want 0..3", s, e)
	}
	// URL starts at col 4
	s, e = ed.wordRange(0, 7)
	if s != 4 {
		t.Errorf("URL start=%d want 4", s)
	}
	if e <= s {
		t.Errorf("URL empty range %d..%d", s, e)
	}
}

func TestWordRangeCyrillic(t *testing.T) {
	ed := newEditor(`{"title": "тест"}`+"\n", 40, 10)
	// click inside "тест" (rune index 11)
	s, e := ed.wordRange(0, 11)
	if e-s != 4 {
		t.Errorf("cyrillic word range=%d..%d want 4 chars", s, e)
	}
	text := ed.SelectedText(0, s, 0, e)
	if text != "тест" {
		t.Errorf("word=%q want %q", text, "тест")
	}
}

func TestDeleteRange(t *testing.T) {
	ed := newEditor("abcdef\n", 40, 10)
	sel := ed.DeleteRange(0, 1, 0, 4) // delete "bcd"
	if sel != "bcd" {
		t.Errorf("deleted=%q want bcd", sel)
	}
	if got := ed.Lines()[0]; got != "aef" {
		t.Errorf("line=%q want aef", got)
	}
	if ed.curRow != 0 || ed.curCol != 1 {
		t.Errorf("cursor=%d,%d want 0,1", ed.curRow, ed.curCol)
	}
}

func TestNavKey(t *testing.T) {
	if navKey("shift+up") != "up" {
		t.Errorf("navKey(shift+up)=%q", navKey("shift+up"))
	}
}
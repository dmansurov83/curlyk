package tui

import "testing"

// TestPasteCRLFDoesNotLeakCR verifies that pasting Windows CRLF text inserts line
// breaks but never leaves a carriage return in the buffer (which would reset the
// terminal cursor and corrupt the rendered frame).
func TestPasteCRLFDoesNotLeakCR(t *testing.T) {
	m := New(Args{Width: 120, Height: 30}).(model)
	m.active = paneEdit
	m.ed.SetText("GET /x\n")

	// Multi-line CRLF paste through the real paste path.
	m.pasteOrConvert("POST /api\r\nContent-Type: application/json\r\n\r\n{\r\n  \"a\": 1,\r\n}\r\n")

	text := m.ed.Text()
	for i, ln := range m.ed.Lines() {
		for _, r := range ln {
			if r == '\r' {
				t.Fatalf("buffer line %d contains CR after paste: %q\nfull text: %q", i, ln, text)
			}
		}
	}
	// The CRLF paste lands at cursor (0,0) and splits into 8 lines; the prior
	// buffer line "GET /x" is pushed to the end (empty cursor line). What
	// matters for the regression is that no \r survives and the lines are correct.
	if len(m.ed.Lines()) != 8 {
		t.Fatalf("expected 8 lines after CRLF paste, got %d: %q", len(m.ed.Lines()), m.ed.Lines())
	}
	want := "POST /api\nContent-Type: application/json\n\n{\n  \"a\": 1,\n}\nGET /x\n"
	if text != want {
		t.Errorf("paste result mismatch:\ngot  %q\nwant %q", text, want)
	}
}

// TestInsertStringStripsCR ensures a stray carriage return reaching the single-line
// insert path (e.g. char-by-char terminal paste) is dropped.
func TestInsertStringStripsCR(t *testing.T) {
	e := newEditor("a\n", 40, 10)
	e.curRow = 0
	e.curCol = 1
	e.InsertString("\r")
	for _, r := range e.lines[0] {
		if r == '\r' {
			t.Fatalf("InsertString left CR in line: %q", e.lines[0])
		}
	}
}
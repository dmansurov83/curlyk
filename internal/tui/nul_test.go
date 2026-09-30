package tui

import (
	"strings"
	"testing"
)

// TestSanitizeRemovesNUL verifies NUL bytes are stripped from the buffer so that
// rendering, request detection, and the run triangle work (issue report).
func TestSanitizeRemovesNUL(t *testing.T) {
	m := New(Args{Width: 120, Height: 30}).(model)
	m.active = paneEdit
	// simulate NUL bytes corrupting lines (e.g. bad paste)
	m.ed.SetText("\x00\x00POST https://x/publish\n\x00accept: a\n")
	m.ed.sanitize()
	lines := m.ed.Lines()
	if strings.IndexByte(lines[0], 0) >= 0 || strings.IndexByte(lines[1], 0) >= 0 {
		t.Fatal("NUL not removed")
	}
	if !httpfileIsRequestLine(lines[0]) {
		t.Errorf("after sanitize, first line should be a request line: %q", lines[0])
	}
}

// TestInsertStringStripsNUL verifies pasted NUL isn't inserted.
func TestInsertStringStripsNUL(t *testing.T) {
	ed := newEditor("abc\n", 40, 10)
	ed.curRow, ed.curCol = 0, 3
	ed.InsertString("\x00XYZ")
	if got := ed.Lines()[0]; got != "abcXYZ" {
		t.Errorf("NUL should be stripped, got %q", got)
	}
}

func httpfileIsRequestLine(s string) bool { return isRequestLine([]string{s}, 0) }
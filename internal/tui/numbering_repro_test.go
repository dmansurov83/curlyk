package tui

import "testing"

// TestRenderStartsAtLine1Repro reproduces the reported "numbering starts at 2"
// with the exact example content and initial cursor position from the dump.
func TestRenderStartsAtLine1Repro(t *testing.T) {
	m := New(Args{Width: 120, Height: 30}).(model)
	// mimic the dumped state
	m.ed.scroll = 0
	m.ed.curRow, m.ed.curCol, m.ed.onIcon = 0, 1, false
	out := m.renderEditor(60)
	plain := stripANSI(out)

	// line 1 label must precede the first content (GET block header line)
	if !indexOfBefore(plain, " 1 ", "GET") {
		t.Errorf("line 1 label must precede first content:\nSTART[%s]END", plain)
	}
	// line 2 label must precede GET
	if !indexOfBefore(plain, " 2 ", "GET") {
		t.Errorf("line 2 label must precede GET:\n%s", plain)
	}
}

func indexOfBefore(hay, needle, marker string) bool {
	i := indexOfSub(hay, needle)
	j := indexOfSub(hay, marker)
	if i < 0 || j < 0 {
		return false
	}
	return i < j
}

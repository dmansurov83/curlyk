package tui

import "testing"

// TestLineNumberingStartsAtOne verifies that the editor paints the first buffer
// line as line number 1 (no off-by-one from scroll or icon).
func TestLineNumberingStartsAtOne(t *testing.T) {
	m := New(Args{Width: 80, Height: 8}).(model)
	m.ed.SetText("alpha\nbeta\ngamma\n")
	m.ed.scroll = 0
	m.ed.curRow, m.ed.curCol = 0, 0
	out := m.renderEditor(40)
	plain := stripANSI(out)

	// The first rendered content row must carry line number 1.
	if !containsSub(plain, "alpha") {
		t.Fatalf("first line content missing:\n%s", plain)
	}
	if containsSub(plain, " 2 ") && !containsSub(plain, " 1 ") {
		t.Errorf("numbering skipped 1, starts at 2:\n%s", plain)
	}
	// ensure line 1 label precedes alpha
	idx1 := indexOfSub(plain, "  1")
	idxA := indexOfSub(plain, "alpha")
	if idx1 < 0 || idxA < 0 || idx1 > idxA {
		t.Errorf("line-1 label not before first line content:\n%s", plain)
	}
}

func indexOfSub(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}

func containsSub(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
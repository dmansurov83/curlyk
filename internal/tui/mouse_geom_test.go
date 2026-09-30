package tui

import "testing"

// TestMouseGeometryConsistency verifies that the editor text starts at screen
// column (left border + icon + number gutter) and that clicking there yields
// rune column 0, and that each screen column maps to the correct rune index.
func TestMouseGeometryConsistency(t *testing.T) {
	m := New(Args{Width: 120, Height: 30}).(model)
	el := m.layout().editorL
	// Simple ASCII line so display width == rune count.
	m.ed.SetText("GET      http\n") // col0..: G E T space...
	m.ed.curRow = 0

	// Text "GET      http": runes
	//  G(0) E(1) T(2) sp(3) sp(4) .. sp(8) h(9) t(10) t(11) p(12)
	// Editor x for rune col c = el+1(border) + 1(icon) + 4(num) + c = el+6+c
	cases := []struct {
		runeCol int
	}{
		{0}, {1}, {2}, {3}, {9}, {12},
	}
	for _, tc := range cases {
		x := el+6 + tc.runeCol
		y := headerHeight + 1 // row 0 is at screen y=headerHeight+1 (below header & top border)
		row, col, _ := mouseToEditorCell(&m, x, y)
		if row != 0 {
			t.Errorf("runeCol %d: row=%d want 0", tc.runeCol, row)
		}
		if col != tc.runeCol {
			t.Errorf("click x=%d want runeCol=%d, got col=%d", x, tc.runeCol, col)
		}
	}
}

// TestMouseIconAtScreenX_fw verifies the icon column is at x=el+1.
func TestMouseIconAtScreenX_fw(t *testing.T) {
	m := New(Args{Width: 120, Height: 30}).(model)
	el := m.layout().editorL
	m.ed.SetText("### x\nGET http://x/1\n")
	// request line is row 1 (0-based) -> screen y = headerHeight+2 (row1)
	row, col, onIcon := mouseToEditorCell(&m, el+1, headerHeight+2)
	if !onIcon {
		t.Errorf("expected icon at x=el+1, y=%d; got row=%d col=%d onIcon=%v", headerHeight+2, row, col, onIcon)
	}
	if row != 1 {
		t.Errorf("icon row=%d want 1", row)
	}
}

// TestMouseTextStartsAfterGutter: clicking just before text (line number area)
// clamps to column 0.
func TestMouseTextStartsAfterGutter(t *testing.T) {
	m := New(Args{Width: 120, Height: 30}).(model)
	el := m.layout().editorL
	m.ed.SetText("abcdef\n")
	// screen x=el+3 is within the line number gutter -> clamp to col 0
	row, col, onIcon := mouseToEditorCell(&m, el+3, headerHeight+1)
	if onIcon || col != 0 || row != 0 {
		t.Errorf("gutter click got row=%d col=%d onIcon=%v; want row=0 col=0 onIcon=false", row, col, onIcon)
	}
}

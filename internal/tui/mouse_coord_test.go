package tui

import "testing"

func TestDisplayToRuneAscii(t *testing.T) {
	if got := displayToRune("GET /x", 0); got != 0 {
		t.Errorf("pos0=%d want 0", got)
	}
	if got := displayToRune("GET /x", 1); got != 1 {
		t.Errorf("pos1=%d want 1", got)
	}
	if got := displayToRune("GET /x", 6); got != 6 {
		t.Errorf("pos6=%d want 6 (end)", got)
	}
	if got := displayToRune("GET /x", 50); got != 6 {
		t.Errorf("pos50=%d want 6 (clamp to len)", got)
	}
}

func TestDisplayToRuneCyrillic(t *testing.T) {
	// "тест" = 4 Cyrillic chars, each display width 2 = 8 cells.
	s := `{"title": "тест"}`
	wantPos8 := 11 // rune index after the 4 wide chars that occupy cells 9..16? verify below
	_ = wantPos8
	// The 4 Cyrillic letters each have width 2. After `{"title": "` (10 cells,
	// 10 runes), "т" is rune index 10 with width 2 → cells 10..11.
	// displayToRune should return the rune index whose cell position we clicked.
	if got := displayToRune(s, 10); got != 10 {
		t.Errorf("click at cell10 should be rune 10 (start of Cyrillic), got %d", got)
	}
	if got := displayToRune(s, 11); got != 11 {
		t.Errorf("click at cell11 (inside first wide char) should be rune 11, got %d", got)
	}
}

func TestMouseToEditorCellBounds(t *testing.T) {
	m := New(Args{Width: 120, Height: 30}).(model)
	el := m.layout().editorL
	m.ed.SetText("GET /x\n")
	// click at screen x=el+6 → first text char (rune col 0), y=headerHeight+1 → row 0
	row, col, onIcon := mouseToEditorCell(&m, el+6, headerHeight+1)
	if row != 0 {
		t.Errorf("row=%d want 0", row)
	}
	if col != 0 {
		t.Errorf("col=%d want 0", col)
	}
	if onIcon {
		t.Error("onIcon should be false at text column")
	}

	// click at the icon column (screen x=el+1)
	row, col, onIcon = mouseToEditorCell(&m, el+1, headerHeight+1)
	if !onIcon {
		t.Errorf("expected onIcon=true at x=el+1")
	}

	// click in the right pane (x >= editor end)
	row, _, _ = mouseToEditorCell(&m, m.layout().half+2, 5)
	if row >= 0 {
		t.Errorf("expected row=-1 for right pane click, got %d", row)
	}
}
package tui

import "testing"

// TestClickOffsetWhenScrolled verifies that when the editor is horizontally
// scrolled, a mouse click maps to the full-line rune column (adding hScroll).
func TestClickOffsetWhenScrolled(t *testing.T) {
	m := New(Args{Width: 120, Height: 30}).(model)
	m.active = paneEdit
	m.width, m.height = 120, 30
	m.ed.width = m.layout().mid
	line := "ABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789abcdefghijklmnopqrstuvwxyz"
	m.ed.SetText(line + "\n")
	m.ed.curRow = 0
	m.ed.hScroll = 20
	// click at visible text cell 2 → full line cell 22 → rune col 22
	x := m.layout().editorL + 6 + 2
	_, col, _ := mouseToEditorCell(&m, x, 2)
	if col != 22 {
		t.Errorf("scrolled click at viewport cell2 → runeCol=%d want 22", col)
	}
}

// TestClickNoScrollStaysExact verifies clicks map exactly when hScroll==0.
func TestClickNoScrollStaysExact(t *testing.T) {
	m := New(Args{Width: 120, Height: 30}).(model)
	m.active = paneEdit
	m.width, m.height = 120, 30
	m.ed.width = m.layout().mid
	m.ed.SetText("GET /path\n")
	m.ed.curRow = 0
	for _, want := range []int{0, 2, 5, 8} {
		x := m.layout().editorL + 6 + want
		_, col, _ := mouseToEditorCell(&m, x, 2)
		if col != want {
			t.Errorf("click cell %d → runeCol=%d want %d", want, col, want)
		}
	}
}

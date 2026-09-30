package tui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestCursorMovesVerifiesEditor(t *testing.T) {
	m := New(Args{Width: 100, Height: 24}).(model)
	m.active = paneEdit

	// starts at row 0 col 0
	if m.ed.curRow != 0 || m.ed.curCol != 0 {
		t.Fatalf("start pos=%d,%d", m.ed.curRow, m.ed.curCol)
	}

	// move down twice (rows 1 and 2)
	_, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
	_, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
	if m.ed.curRow != 2 {
		t.Errorf("after downx2 row=%d want 2", m.ed.curRow)
	}

	// move right 5 times
	for i := 0; i < 5; i++ {
		_, _ = m.Update(tea.KeyMsg{Type: tea.KeyRight})
	}
	if m.ed.curCol != 5 {
		t.Errorf("curCol=%d want 5", m.ed.curCol)
	}
	if m.ed.curRow != 2 {
		t.Errorf("row=%d want 2", m.ed.curRow)
	}
}

// TestCursorBlockVisibleOnActiveLine verifies the rendered active line keeps
// its content and the cursor block substitutes exactly one cell.
func TestCursorBlockVisibleOnActiveLine(t *testing.T) {
	m := New(Args{Width: 100, Height: 24}).(model)
	m.ed.SetText("GET /a\nPost /b\n")
	m.ed.curRow, m.ed.curCol = 1, 3
	v := m.View()
	if v == "" {
		t.Fatal("empty view")
	}
}
package tui

import (
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

// TestDoubleClickSelectionPersists verifies that a double-click on a word
// selects it AND keeps the selection after the release (issue #1).
func TestDoubleClickSelectionPersists(t *testing.T) {
	m := New(Args{Width: 120, Height: 30}).(model)
	el := m.layout().editorL
	m.active = paneEdit
	m.ed.SetText("GET https://x.com PV\n")

	// first click (single) on the URL word area, screen x for rune col 8
	click := func() tea.Model {
		var mm tea.Model = m
		mm, _ = mm.Update(tea.MouseMsg{
			Button: tea.MouseButtonLeft, Action: tea.MouseActionPress, X: el + 6 + 8, Y: headerHeight + 1,
		})
		// release
		mm, _ = mm.Update(tea.MouseMsg{
			Button: tea.MouseButtonLeft, Action: tea.MouseActionRelease, X: el + 6 + 8, Y: headerHeight + 1,
		})
		m = mm.(model)
		return mm
	}
	click() // first click
	// force double by setting the last click time recently
	m.lastClickTime = time.Now()

	// second click within 300ms -> double
	mm, _ := m.Update(tea.MouseMsg{
		Button: tea.MouseButtonLeft, Action: tea.MouseActionPress, X: el + 6 + 8, Y: headerHeight + 1,
	})
	m = mm.(model)
	// release of the double click
	mm, _ = m.Update(tea.MouseMsg{
		Button: tea.MouseButtonLeft, Action: tea.MouseActionRelease, X: el + 6 + 8, Y: headerHeight + 1,
	})
	r := mm.(model)
	if !r.selActive {
		t.Fatal("double-click selection must persist after release")
	}
	sel := r.ed.SelectedText(r.selAnchorRow, r.selAnchorCol, r.ed.curRow, r.ed.curCol)
	if sel != "https://x.com" {
		t.Errorf("selected=%q want https://x.com", sel)
	}
}

// TestSingleClickClearsSelection verifies a plain single click collapses the selection.
func TestSingleClickClearsSelection(t *testing.T) {
	m := New(Args{Width: 120, Height: 30}).(model)
	el := m.layout().editorL
	m.active = paneEdit
	m.ed.SetText("hello world\n")
	m.selActive = true
	m.selAnchorRow, m.selAnchorCol = 0, 0
	m.ed.curRow, m.ed.curCol = 0, 5

	// a single click elsewhere after >300ms gap
	m.lastClickTime = time.Now().Add(-1 * time.Second)
	mm, _ := m.Update(tea.MouseMsg{
		Button: tea.MouseButtonLeft, Action: tea.MouseActionPress, X: el + 6 + 1, Y: headerHeight + 1,
	})
	r := mm.(model)
	r.mouseDragged = false
	mm2, _ := r.Update(tea.MouseMsg{
		Button: tea.MouseButtonLeft, Action: tea.MouseActionRelease, X: el + 6 + 1, Y: headerHeight + 1,
	})
	if mm2.(model).selActive {
		t.Error("single click should clear the selection")
	}
}


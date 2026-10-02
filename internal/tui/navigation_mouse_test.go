package tui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

// TestNavMouseClickJumps verifies a left-click on a navigation popup entry
// jumps the cursor to that request and closes the popup.
func TestNavMouseClickJumps(t *testing.T) {
	m := New(Args{Width: 120, Height: 30}).(model)
	m.active = paneEdit
	m.ed.SetText("### a\nGET http://api.test/v1/first\n@name alpha\n\n### b\nPOST http://api.test/v1/second\n")
	m.beginNav()
	if m.nav == nil {
		t.Fatal("nav should be open")
	}
	// Title row at headerHeight+1; entries start at headerHeight+2.
	// Entry 0 = first request (GET /first).
	clickEntry := func(entryIdx, x int) model {
		m2, _ := m.Update(tea.MouseMsg{
			Button: tea.MouseButtonLeft, Action: tea.MouseActionPress,
			X: x, Y: headerHeight + 2 + entryIdx,
		})
		return m2.(model)
	}
	res := clickEntry(0, m.layout().editorL+10)
	if res.nav != nil {
		t.Error("clicking an entry must close the navigation popup")
	}
	// First request line is at source line 2 (0-based row 1).
	if res.ed.curRow != 1 {
		t.Errorf("cursor row=%d want 1 (first request line)", res.ed.curRow)
	}
	if res.active != paneEdit {
		t.Errorf("active pane=%d want paneEdit", res.active)
	}
}

// TestNavMouseClickSecondEntry verifies clicking a later visible entry selects
// the right request.
func TestNavMouseClickSecondEntry(t *testing.T) {
	m := New(Args{Width: 120, Height: 30}).(model)
	m.active = paneEdit
	m.ed.SetText("### a\nGET http://x/1\n\n### b\nPOST http://x/2\n")
	m.beginNav()
	// Entry rows: title(headerHeight+1), entry0(headerHeight+2), entry1(headerHeight+3).
	m2, _ := m.Update(tea.MouseMsg{
		Button: tea.MouseButtonLeft, Action: tea.MouseActionPress,
		X: m.layout().editorL + 10, Y: headerHeight + 3,
	})
	res := m2.(model)
	if res.nav != nil {
		t.Error("clicking entry must close popup")
	}
	// Second request line is at source line 5 (0-based row 4).
	if res.ed.curRow != 4 {
		t.Errorf("cursor row=%d want 4 (second request line)", res.ed.curRow)
	}
}

// TestNavClickOutsideDismisses verifies clicking elsewhere in the editor pane
// closes the navigation popup without jumping.
func TestNavClickOutsideDismisses(t *testing.T) {
	m := New(Args{Width: 120, Height: 30}).(model)
	m.active = paneEdit
	m.ed.SetText("### a\nGET http://x/1\n")
	m.beginNav()
	// Click in the response pane (right of the editor): should dismiss too.
	m2, _ := m.Update(tea.MouseMsg{
		Button: tea.MouseButtonLeft, Action: tea.MouseActionPress,
		X: m.layout().half + 5, Y: headerHeight + 3,
	})
	if m2.(model).nav != nil {
		t.Error("click outside the nav entries must dismiss the popup")
	}
	// Cursor must not have jumped (still at initial 0).
	if m2.(model).ed.curRow != 0 {
		t.Errorf("cursor moved to row=%d unexpectedly", m2.(model).ed.curRow)
	}
}

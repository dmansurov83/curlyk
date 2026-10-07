package tui

import (
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

func TestRespSelectionMousePressAndDrag(t *testing.T) {
	m := New(Args{Width: 120, Height: 30}).(model)
	rs := m.layout().half
	m.active = paneResp
	m.response = "alpha\nbeta\ngamma\n"
	m.respScroll = 0

	// click in response pane: x=rs+1 (content col 0), y=headerHeight+2 (content row 0,
	// below header + top border + copy button)
	bodyStart := headerHeight + 2
	m2, _ := m.Update(tea.MouseMsg{
		Button: tea.MouseButtonLeft, Action: tea.MouseActionPress, X: rs + 1, Y: bodyStart,
	})
	r := m2.(model)
	if !r.respSelActive {
		t.Fatal("response selection should start on click in response pane")
	}
	if r.respSelAnchorRow != 0 || r.respSelAnchorCol != 0 {
		t.Errorf("anchor=%d,%d want 0,0", r.respSelAnchorRow, r.respSelAnchorCol)
	}

	// drag to row 2 (y=bodyStart+2), content col 2 (x=rs+3)
	m3, _ := r.Update(tea.MouseMsg{
		Button: tea.MouseButtonLeft, Action: tea.MouseActionMotion, X: rs + 3, Y: bodyStart + 2,
	})
	r = m3.(model)
	sel := r.respSelectedText()
	// anchor (0,0) "alpha" + "\nbeta\n" + (2,0..2) "ga"
	if sel != "alpha\nbeta\nga" {
		t.Errorf("respSelectedText=%q", sel)
	}
}

func TestRespShiftSelectAndCopy(t *testing.T) {
	m := New(Args{Width: 120, Height: 30}).(model)
	m.active = paneResp
	m.response = "line one\nline two\n"
	m.respScroll = 0
	m.respSelActive = true
	m.respSelAnchorRow, m.respSelAnchorCol = 0, 0
	m.respSelCurRow, m.respSelCurCol = 0, 4

	sel := m.respSelectedText()
	if sel != "line" {
		t.Errorf("respSelectedText=%q want line", sel)
	}

	// Shift+Right extends selection
	m2, _ := m.handleKey(tea.KeyMsg{Type: tea.KeyShiftRight})
	r := m2.(model)
	if r.respSelCurCol != 5 {
		t.Errorf("curCol after shift+right=%d want 5", r.respSelCurCol)
	}
	// copy: Ctrl+C on response pane
	m3, cmd := r.handleKey(tea.KeyMsg{Type: tea.KeyCtrlC})
	if cmd != nil {
		t.Fatal("Ctrl+C on response should not quit")
	}
	r = m3.(model)
	if r.respSelActive {
		t.Error("selection should clear after copy")
	}
}

func TestMouseToRespCell(t *testing.T) {
	m := New(Args{Width: 120, Height: 30}).(model)
	rs := m.layout().half // response pane left border
	m.response = "abc\n"
	m.respScroll = 0
	row, col, ok := mouseToRespCell(&m, rs+1, headerHeight+2) // content col 0, row 0
	if !ok || row != 0 || col != 0 {
		t.Errorf("resp cell=%d,%d ok=%v want 0,0 true", row, col, ok)
	}
	// x=rs+3 -> content col 2
	row, col, ok = mouseToRespCell(&m, rs+3, headerHeight+2)
	if !ok || row != 0 || col != 2 {
		t.Errorf("resp cell=%d,%d ok=%v want 0,2 true", row, col, ok)
	}
	// click in left (editor) pane -> not ok
	_, _, ok = mouseToRespCell(&m, 30, 5)
	if ok {
		t.Error("editor pane click should not map to response cell")
	}
}

// TestRespDoubleClickSelectsWord verifies that a double-click in the response
// pane selects the word under the cursor and keeps it after the release.
func TestRespDoubleClickSelectsWord(t *testing.T) {
	m := New(Args{Width: 120, Height: 30}).(model)
	rs := m.layout().half
	m.active = paneResp
	m.response = "apple banana cherry\n"
	m.respScroll = 0
	m.respSelActive = false

	// double-click on the "banana" word: content col 7, body row 0.
	press := func() {
		mm, _ := m.Update(tea.MouseMsg{
			Button: tea.MouseButtonLeft, Action: tea.MouseActionPress, X: rs + 1 + 7, Y: headerHeight + 2,
		})
		m = mm.(model)
	}
	// first click
	press()
	// force double-click by setting the last click time recently
	m.lastClickTime = time.Now()
	// second click within 300ms -> double
	press()
	// release
	mm, _ := m.Update(tea.MouseMsg{
		Button: tea.MouseButtonLeft, Action: tea.MouseActionRelease, X: rs + 1 + 7, Y: headerHeight + 2,
	})
	r := mm.(model)
	if !r.respSelActive {
		t.Fatal("response double-click selection must be active")
	}
	sel := r.respSelectedText()
	if sel != "banana" {
		t.Errorf("resp double-click selected=%q want banana", sel)
	}
}
package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

// TestRightClickCopiesSelection verifies a right-click on an editor selection
// copies it and clears the selection.
func TestRightClickCopiesEditorSelection(t *testing.T) {
	m := New(Args{Width: 120, Height: 30}).(model)
	m.active = paneEdit
	m.ed.SetText("hello world\n")
	m.selActive = true
	m.selAnchorRow, m.selAnchorCol = 0, 0
	m.ed.curRow, m.ed.curCol = 0, 5

	// right-click anywhere in the editor (x < half)
	m2, cmd := m.Update(tea.MouseMsg{
		Button: tea.MouseButtonRight, Action: tea.MouseActionPress, X: 10, Y: 5,
	})
	if cmd != nil {
		t.Fatal("right-click copy should not produce a command")
	}
	r := m2.(model)
	if r.selActive {
		t.Error("selection should be cleared after right-click copy")
	}
	if r.status == "" || !strings.Contains(r.status, "Скопировано") {
		t.Errorf("expected copy status, got: %q", r.status)
	}
}

// TestRightClickCopiesResponseSelection verifies right-click in the response
// pane copies the response selection and clears it.
func TestRightClickCopiesResponseSelection(t *testing.T) {
	m := New(Args{Width: 120, Height: 30}).(model)
	m.active = paneResp
	m.response = "response body text\n"
	m.respSelActive = true
	m.respSelAnchorRow, m.respSelAnchorCol = 0, 0
	m.respSelCurRow, m.respSelCurCol = 0, 8

	m2, _ := m.Update(tea.MouseMsg{
		Button: tea.MouseButtonRight, Action: tea.MouseActionPress, X: 80, Y: 5,
	})
	r := m2.(model)
	if r.respSelActive {
		t.Error("response selection should be cleared after right-click copy")
	}
	if !strings.Contains(r.status, "Скопировано") {
		t.Errorf("expected copy status, got: %q", r.status)
	}
}

// TestRightClickNoSelectionNoCopy verifies right-click without a selection does nothing.
func TestRightClickNoSelection(t *testing.T) {
	m := New(Args{Width: 120, Height: 30}).(model)
	m.active = paneEdit
	m.selActive = false
	m.status = "prev"
	m2, _ := m.Update(tea.MouseMsg{
		Button: tea.MouseButtonRight, Action: tea.MouseActionPress, X: 10, Y: 5,
	})
	r := m2.(model)
	// right-click with no selection pastes from clipboard (like Ctrl+V), so the
	// status becomes either "Вставлено" or "Буфер обмена пуст".
	if r.status == "prev" {
		t.Errorf("expected paste attempt (status changed), got %q", r.status)
	}
	if r.selActive {
		t.Error("selActive should stay false")
	}
}
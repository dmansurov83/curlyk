package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

// TestHandleMouseClickEdit moves the cursor via a synthetic mouse click.
func TestHandleMouseClickEdit(t *testing.T) {
	m := New(Args{Width: 120, Height: 30}).(model)
	el := m.layout().editorL
	m.active = paneEdit
	m.ed.SetText("GET /path\n") // row0: "GET /path" (10 runes)
	m.ed.curRow, m.ed.curCol = 0, 0

	// Click on the text column. Editor pane starts after the files panel (fw).
	// Click at screen x = fw+6 (first text char of pane) → rune col 0.
	// Row 0 is at y = headerHeight+1 = 2 (below the header and top border).
	m2, _ := m.Update(tea.MouseMsg{
		Button: tea.MouseButtonLeft,
		Action: tea.MouseActionPress,
		X:      el + 6,
		Y:      headerHeight + 1,
	})
	r := m2.(model)
	if r.ed.curCol != 0 {
		t.Errorf("curCol=%d want 0", r.ed.curCol)
	}
	if r.active != paneEdit {
		t.Errorf("active=%v want paneEdit", r.active)
	}
}

// TestHandleMouseWheelResponse scrolls the response pane.
func TestHandleMouseWheelResponse(t *testing.T) {
	m := New(Args{Width: 120, Height: 30}).(model)
	m.active = paneResp
	// many lines so the response overflows the pane (visible height ~25)
	var many strings.Builder
	for i := 0; i < 100; i++ {
		many.WriteString("line" + itoa(i) + "\n")
	}
	m.response = many.String()
	m.respScroll = 10

	m2, _ := m.Update(tea.MouseMsg{Button: tea.MouseButtonWheelDown, Action: tea.MouseActionPress, X: 80})
	r := m2.(model)
	if r.respScroll != 13 {
		t.Errorf("respScroll=%d want 13", r.respScroll)
	}
	m3, _ := r.Update(tea.MouseMsg{Button: tea.MouseButtonWheelUp, Action: tea.MouseActionPress, X: 80})
	rr := m3.(model)
	if rr.respScroll != 10 {
		t.Errorf("respScroll after up=%d want 10", rr.respScroll)
	}
}

// TestWheelNoScrollWhenFits verifies wheeling a short response does nothing (issue #2).
func TestWheelNoScrollWhenFits(t *testing.T) {
	m := New(Args{Width: 120, Height: 30}).(model)
	m.active = paneResp
	m.response = "short\nbody\n"
	m.respScroll = 0
	m2, _ := m.Update(tea.MouseMsg{Button: tea.MouseButtonWheelDown, Action: tea.MouseActionPress, X: 80})
	if m2.(model).respScroll != 0 {
		t.Error("short response must not scroll")
	}
}

func itoa(i int) string {
	if i == 0 {
		return "0"
	}
	var b []byte
	for i > 0 {
		b = append([]byte{byte('0' + i%10)}, b...)
		i /= 10
	}
	return string(b)
}

// TestMouseClickEditorPaneSwitches tests that clicking in the right half focuses the response pane.
func TestMouseClickEditorPaneSwitches(t *testing.T) {
	m := New(Args{Width: 120, Height: 30}).(model)
	m.active = paneEdit
	m.response = "resp"

	// Right pane starts at half=60. Click at x=80 → response pane.
	m2, _ := m.Update(tea.MouseMsg{
		Button: tea.MouseButtonLeft,
		Action: tea.MouseActionPress,
		X:      80,
		Y:      10,
	})
	if m2.(model).active != paneResp {
		t.Errorf("active=%v want paneResp after right click", m2.(model).active)
	}
}

// TestClickRunIconOpensMenu verifies clicking the ▶ icon on a request line opens
// the action popup instead of running directly.
func TestClickRunIconOpensMenu(t *testing.T) {
	m := New(Args{Width: 120, Height: 30}).(model)
	el := m.layout().editorL
	m.active = paneEdit
	m.ed.SetText("### A\nGET http://x/1\n")
	m.ed.curRow, m.ed.curCol = 1, 0

	// Click at icon column (screen x=el+1) on request line (row index 1 → y=headerHeight+2).
	m2, cmd := m.Update(tea.MouseMsg{
		Button: tea.MouseButtonLeft,
		Action: tea.MouseActionPress,
		X:      el + 1,
		Y:      headerHeight + 2,
	})
	r := m2.(model)
	if cmd != nil {
		t.Fatal("clicking the run icon must open the menu, not run directly")
	}
	if r.actionMenu == nil {
		t.Fatal("expected action popup after clicking the run icon")
	}
	if r.actionMenu.anchorRow != 1 {
		t.Errorf("menu anchor=%d want 1", r.actionMenu.anchorRow)
	}
}

// TestClickRunIconNonRequestNoRun verifies clicking the icon column on a
// non-request line does NOT open the menu.
func TestClickRunIconNonRequestNoRun(t *testing.T) {
	m := New(Args{Width: 120, Height: 30}).(model)
	el := m.layout().editorL
	m.active = paneEdit
	// row 0 is "### ..." separator (not a request).
	m.ed.SetText("### only\n")
	_, cmd := m.Update(tea.MouseMsg{
		Button: tea.MouseButtonLeft,
		Action: tea.MouseActionPress,
		X:      el + 1,
		Y:      headerHeight + 1,
	})
	if cmd != nil {
		t.Fatal("did not expect a run Cmd (clicking separator icon)")
	}
	if m.actionMenu != nil {
		t.Fatal("must not open menu when clicking a non-request icon")
	}
}

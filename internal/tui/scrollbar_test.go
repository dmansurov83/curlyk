package tui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

// TestEditorScrollbarClick scrolls the editor by clicking its scrollbar column.
func TestEditorScrollbarClick(t *testing.T) {
	m := New(Args{Width: 120, Height: 30}).(model)
	m.active = paneEdit
	var b string
	for i := 0; i < 200; i++ {
		b += "line\n"
	}
	m.ed.SetText(b)
	m.ed.scroll = 0
	// scrollbar column is at the editor right edge interior = half-2.
	sb := m.layout().half - 2
	m2, _ := m.Update(tea.MouseMsg{
		Button: tea.MouseButtonLeft, Action: tea.MouseActionPress, X: sb, Y: m.height - 3,
	})
	r := m2.(model)
	if r.ed.scroll <= 0 {
		t.Errorf("editing scrollbar click should scroll, scroll=%d", r.ed.scroll)
	}
	// top click -> scroll 0
	r.ed.scroll = 100
	m3, _ := r.Update(tea.MouseMsg{
		Button: tea.MouseButtonLeft, Action: tea.MouseActionPress, X: sb, Y: 1,
	})
	if m3.(model).ed.scroll > 5 {
		t.Errorf("top scrollbar click should reset near 0, scroll=%d", m3.(model).ed.scroll)
	}
}

// TestRespScrollbarClick scrolls the response by clicking its scrollbar column.
func TestRespScrollbarClick(t *testing.T) {
	m := New(Args{Width: 120, Height: 30}).(model)
	m.active = paneResp
	var b string
	for i := 0; i < 150; i++ {
		b += "row\n"
	}
	m.response = b
	m.respScroll = 0
	// response scrollbar at x = width-2 = 118
	m2, _ := m.Update(tea.MouseMsg{
		Button: tea.MouseButtonLeft, Action: tea.MouseActionPress, X: 118, Y: m.height - 3,
	})
	r := m2.(model)
	if r.respScroll <= 0 {
		t.Errorf("resp scrollbar click should scroll, scroll=%d", r.respScroll)
	}
}

// TestScrollbarHiddenWhenFits verifies no scrollbar shows when content fits.
func TestScrollbarHiddenWhenFits(t *testing.T) {
	m := New(Args{Width: 120, Height: 30}).(model)
	m.active = paneEdit
	m.ed.SetText("short\n")
	m.ed.scroll = 0
	// scrollbar is hidden when content fits the pane
	if cell := m.editorScrollbarCell(0, m.ed.height); cell != "" {
		t.Errorf("scrollbar should be hidden when content fits, got %q", cell)
	}
}
package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

// TestMenuRenderShowsItems verifies the action popup draws its items inside the
// editor pane when open.
func TestMenuRenderShowsItems(t *testing.T) {
	m := New(Args{Width: 120, Height: 30}).(model)
	m.active = paneEdit
	m.ed.SetText("### x\nGET http://x/1\n")
	m.ed.curRow = 1

	res, _ := m.handleKey(tea.KeyMsg{Type: tea.KeyEnter})
	mm := res.(model)
	if mm.actionMenu == nil {
		t.Fatal("menu not open after Enter on request line")
	}

	v := mm.View()
	if !strings.Contains(v, "Выполнить") {
		t.Error("view missing Выполнить item")
	}
	if !strings.Contains(v, "Копировать как cURL") {
		t.Error("view missing Копировать как cURL item")
	}
}

// TestMenuEscCloses verifies Esc dismisses the popup.
func TestMenuEscCloses(t *testing.T) {
	m := New(Args{Width: 120, Height: 30}).(model)
	m.active = paneEdit
	m.ed.SetText("### x\nGET http://x/1\n")
	m.ed.curRow = 1
	m.actionMenu = &actionMenu{anchorRow: 1, items: []menuItem{{label: "Выполнить"}}}

	res, _ := m.handleKey(tea.KeyMsg{Type: tea.KeyEsc})
	if res.(model).actionMenu != nil {
		t.Error("Esc must close the action popup")
	}
}

// TestMenuNav verifies up/down move the selection within bounds.
func TestMenuNav(t *testing.T) {
	m := New(Args{Width: 120, Height: 30}).(model)
	m.actionMenu = &actionMenu{
		anchorRow: 1,
		items:     []menuItem{{label: "a"}, {label: "b"}, {label: "c"}},
	}
	if m.actionMenu.sel != 0 {
		t.Fatalf("initial sel=%d want 0", m.actionMenu.sel)
	}
	// down to 2
	for i := 0; i < 5; i++ {
		m.handleMenuKey(tea.KeyMsg{Type: tea.KeyDown})
	}
	if m.actionMenu.sel != 2 {
		t.Errorf("after downs sel=%d want 2 (clamped)", m.actionMenu.sel)
	}
	// up clamps to 0
	for i := 0; i < 5; i++ {
		m.handleMenuKey(tea.KeyMsg{Type: tea.KeyUp})
	}
	if m.actionMenu.sel != 0 {
		t.Errorf("after ups sel=%d want 0 (clamped)", m.actionMenu.sel)
	}
}
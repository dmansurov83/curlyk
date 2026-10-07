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

// TestMenuEnsuresBlankAfterRequest verifies that opening the action popup on a
// request line ensures a blank separator line ends the block (so a blank row is
// available under the request once the menu closes).
func TestMenuEnsuresBlankAfterRequest(t *testing.T) {
	m := New(Args{Width: 120, Height: 30}).(model)
	m.active = paneEdit
	// request line followed directly by another block's request line (no blank).
	m.ed.SetText("GET http://x/1\nPOST http://y/2\n")
	m.ed.curRow = 0

	m.beginActionMenu()
	lines := m.ed.Lines()
	if len(lines) < 3 {
		t.Fatalf("expected at least 3 lines, got %d: %q", len(lines), lines)
	}
	if strings.TrimSpace(lines[1]) != "" {
		t.Fatalf("expected blank separator on line 2, got %q", lines[1])
	}
	if !strings.Contains(lines[2], "POST") {
		t.Fatalf("expected next block preserved on line 3, got %q", lines[2])
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

// TestMouseMenuClickRunsItem verifies clicking a menu item with the mouse runs
// its action and closes the popup.
func TestMouseMenuClickRunsItem(t *testing.T) {
	m := New(Args{Width: 120, Height: 30}).(model)
	m.active = paneEdit
	m.ed.SetText("### a\nGET http://x/1\n")
	m.ed.curRow, m.ed.curCol = 1, 0
	m.beginActionMenu()
	if m.actionMenu == nil {
		t.Fatal("menu should be open")
	}
	// menu item 0 row: anchorRow=1, scroll=0 → menuRow() = headerHeight+2+(1)=4.
	rowY := m.menuRow()
	if m.menuItemAt(rowY) != 0 {
		t.Fatalf("menuItemAt(y=%d) want 0", rowY)
	}
	// Click item 0 → closes menu and runs → returns a cmd (runRequest).
	m2, cmd := m.Update(tea.MouseMsg{
		Button: tea.MouseButtonLeft, Action: tea.MouseActionPress,
		X: m.layout().editorL + 1, Y: rowY,
	})
	r := m2.(model)
	if r.actionMenu != nil {
		t.Error("menu must close after clicking an item")
	}
	if cmd == nil {
		t.Error("running Выполнить via mouse must return a cmd")
	}
}

// TestMouseRightClickRequestOpensMenu verifies right-clicking a request line
// opens its action popup.
func TestMouseRightClickRequestOpensMenu(t *testing.T) {
	m := New(Args{Width: 120, Height: 30}).(model)
	m.active = paneEdit
	m.ed.SetText("### a\nGET http://x/1\n")
	m.ed.curRow, m.ed.curCol = 1, 0
	// right-click on the request line text (row 1 → y=headerHeight+2)
	m2, _ := m.Update(tea.MouseMsg{
		Button: tea.MouseButtonRight, Action: tea.MouseActionPress,
		X: m.layout().editorL + 10, Y: headerHeight + 2,
	})
	if m2.(model).actionMenu == nil {
		t.Fatal("right-click on a request line should open the action popup")
	}
}

// TestFormatRequestJSON verifies formatting a JSON request body in place.
func TestFormatRequestJSON(t *testing.T) {
	m := New(Args{Width: 120, Height: 30}).(model)
	m.active = paneEdit
	m.ed.SetText("### a\nPOST http://x\nContent-Type: application/json\n\n{\"b\":1,\"a\":\"x\"}\n")
	m.ed.curRow = 1
	m.formatRequestJSON()
	got := m.ed.Text()
	wantFrag := "\n{\n  \"b\": 1,\n  \"a\": \"x\"\n}"
	if !strings.Contains(got, wantFrag) || !strings.Contains(got, "Content-Type: application/json") {
		t.Errorf("formatted body missing:\n%s", got)
	}
}

// TestFormatRequestJSONInvalid verifies non-JSON bodies are rejected.
func TestFormatRequestJSONInvalid(t *testing.T) {
	m := New(Args{Width: 120, Height: 30}).(model)
	m.active = paneEdit
	m.ed.SetText("### a\nPOST http://x\n\nnot json\n")
	m.ed.curRow = 1
	m.formatRequestJSON()
	if m.ed.Text() != "### a\nPOST http://x\n\nnot json\n" {
		t.Errorf("non-JSON body must be left untouched, got:\n%s", m.ed.Text())
	}
	if !strings.Contains(m.status, "JSON") {
		t.Errorf("status should mention invalid JSON, got %q", m.status)
	}
}

// TestFormatRequestJSONUndo verifies Ctrl+Z reverts an applied JSON format.
func TestFormatRequestJSONUndo(t *testing.T) {
	m := New(Args{Width: 120, Height: 30}).(model)
	m.active = paneEdit
	orig := "### a\nPOST http://x\nContent-Type: application/json\n\n{\"b\":1,\"a\":\"x\"}\n"
	m.ed.SetText(orig)
	m.ed.undo = m.ed.undo[:0] // clear SetText's snapshot
	m.ed.curRow = 1
	m.formatRequestJSON()
	if m.ed.Text() == orig {
		t.Fatal("format should change the body")
	}
	res, _ := m.handleKey(tea.KeyMsg{Type: tea.KeyCtrlZ})
	r := res.(model)
	if r.ed.Text() != orig {
		t.Errorf("Ctrl+Z should restore original, got:\n%s", r.ed.Text())
	}
}

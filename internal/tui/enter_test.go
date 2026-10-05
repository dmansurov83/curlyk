package tui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

// TestEnterOpensActionMenuOnRequestLine verifies plain Enter on a request line
// opens the action popup (Выполнить / Copy as cURL) instead of running directly.
func TestEnterOpensActionMenuOnRequestLine(t *testing.T) {
	m := New(Args{Width: 100, Height: 24}).(model)
	m.active = paneEdit
	m.ed.SetText("### x\nGET http://x/1\n")
	m.ed.curRow = 1

	res, cmd := m.handleKey(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd != nil {
		t.Fatalf("Enter on a request line must not run directly; got cmd")
	}
	mm := res.(model)
	if mm.actionMenu == nil {
		t.Fatal("Enter on a request line must open the action popup")
	}
	if mm.actionMenu.anchorRow != 1 {
		t.Errorf("menu anchor=%d want 1", mm.actionMenu.anchorRow)
	}
	if len(mm.actionMenu.items) != 3 {
		t.Errorf("menu items=%d want 3", len(mm.actionMenu.items))
	}
	// lines unchanged (open menu, not newline). "### x\nGET http://x/1\n" → 3
	if len(mm.ed.Lines()) != 3 {
		t.Errorf("lines should stay 3, got %d", len(mm.ed.Lines()))
	}
}

// TestEnterMenuSelRun verifies selecting "Выполнить" in the menu runs the request.
func TestEnterMenuSelRun(t *testing.T) {
	m := New(Args{Width: 100, Height: 24}).(model)
	m.active = paneEdit
	m.ed.SetText("### x\nGET http://x/1\n")
	m.ed.curRow = 1
	m.actionMenu = &actionMenu{
		anchorRow: 1,
		items: []menuItem{
			{label: "▶ Выполнить", run: func(mm *model) tea.Cmd { return mm.runRequest() }},
			{label: "⧉ Копировать как cURL", run: func(mm *model) tea.Cmd { mm.copyAsCurl(); return nil }},
		},
	}

	// "Выполнить" is item 0, already selected. Enter confirms it.
	res, cmd := m.handleKey(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatalf("confirming Выполнить must run; got nil cmd")
	}
	if res.(model).actionMenu != nil {
		t.Error("menu must close after confirming")
	}
}

// TestEnterInsertsNewlineOnBodyLine verifies plain Enter on a non-request line
// still inserts a newline.
func TestEnterInsertsNewlineOnBodyLine(t *testing.T) {
	m := New(Args{Width: 100, Height: 24}).(model)
	m.active = paneEdit
	m.ed.SetText("GET http://x\n\nbody\n")
	m.ed.curRow = 2 // body line
	before := len(m.ed.Lines())

	_, cmd := m.handleKey(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd != nil {
		t.Fatal("Enter on body must not run")
	}
	if len(m.ed.Lines()) != before+1 {
		t.Errorf("Enter should insert newline, lines %d->%d", before, len(m.ed.Lines()))
	}
}

// TestCtrlRRuns verifies Ctrl+R still runs the request under the cursor.
func TestCtrlRRuns(t *testing.T) {
	m := New(Args{Width: 100, Height: 24}).(model)
	m.active = paneEdit
	m.ed.SetText("### x\nGET http://x/1\n")
	m.ed.curRow = 1
	_, cmd := m.handleKey(tea.KeyMsg{Type: tea.KeyCtrlR})
	if cmd == nil {
		t.Fatal("expected run Cmd from Ctrl+R")
	}
}
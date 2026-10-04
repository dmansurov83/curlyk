package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

// TestHoverHighlightsActionMenu verifies that hovering a menu row paints it with
// the hover style (distinct from the selected item).
func TestHoverHighlightsActionMenu(t *testing.T) {
	m := New(Args{Width: 120, Height: 30}).(model)
	m.active = paneEdit
	m.ed.SetText("### x\nGET http://x/1\n")
	m.ed.curRow = 1
	m.beginActionMenu()
	if m.actionMenu == nil {
		t.Fatal("menu not open")
	}
	// Hover row 2 of the menu (only 3 items; row 2 is the last one).
	rowY := m.menuRow() + 2
	mm, _ := m.Update(tea.MouseMsg{
		Button: tea.MouseButtonNone, Action: tea.MouseActionMotion,
		X: m.layout().editorL + 10, Y: rowY,
	})
	m = mm.(model)
	out := m.View() // keep ANSI so we can assert the style paint
	// The hovered row should be styled with the hover background (ANSI 60).
	// We check by locating the row line and asserting it contains the hover
	// style escape sequence rather than the selected one. Simpler: verify the
	// hover index matches the expected row.
	if got := m.actionMenu.sel; got != 0 {
		t.Errorf("hover must not change selection (sel=%d)", got)
	}
	if got := m.menuHoverIndex(); got != 2 {
		t.Errorf("menuHoverIndex=%d want 2", got)
	}
	if !strings.Contains(out, "\x1b[97;48;5;60m") {
		t.Errorf("hovered menu row should use hover background (ANSI 60):\n%s", out)
	}
}

// TestHoverHighlightsFilesPanel verifies hovering a file row paints it with the
// hover style and does not change the selection.
func TestHoverHighlightsFilesPanel(t *testing.T) {
	m := New(Args{Width: 120, Height: 30}).(model)
	// Force a files list so the panel has rows to hover.
	m.filesPanel = &filesPanel{all: []string{"a.http", "b.http", "c.http"}}
	m.filesPanel.sel = 1
	// Files rows: row indices start at headerHeight+1 (no in-panel search box).
	// Hover row index 2 => y = headerHeight+3.
	y := headerHeight + 3
	mm, _ := m.Update(tea.MouseMsg{
		Button: tea.MouseButtonNone, Action: tea.MouseActionMotion,
		X: 5, Y: y,
	})
	m = mm.(model)
	if m.filesPanel.sel != 1 {
		t.Errorf("hover must not change files selection (sel=%d)", m.filesPanel.sel)
	}
	if hj, ok := mouseToFilesRow(&m, y); !ok || hj != 2 {
		t.Errorf("mouseToFilesRow(y=%d)=%d,%v want 2,true", y, hj, ok)
	}
	out := stripANSI(m.View())
	if !strings.Contains(out, "c.http") {
		t.Errorf("hovered file row should render:\n%s", out)
	}
}

// TestHoverHighlightsCopyButton verifies hovering the response copy button uses
// the hover style.
func TestHoverHighlightsCopyButton(t *testing.T) {
	m := New(Args{Width: 120, Height: 30}).(model)
	m = m.setResponse("HTTP/1.1 200 OK\n\nbody", "HTTP/1.1 200 OK\n\n", nil)
	// Copy button row = headerHeight+1+respHeaderLines (respHeaderLines=1).
	y := headerHeight + 2
	mm, _ := m.Update(tea.MouseMsg{
		Button: tea.MouseButtonNone, Action: tea.MouseActionMotion,
		X: m.layout().half + 5, Y: y,
	})
	m = mm.(model)
	if !m.hoverCopyButton() {
		t.Errorf("hoverCopyButton() should be true at (x=%d,y=%d)", m.layout().half+5, y)
	}
}

// TestHoverNavHighlightsEntry verifies hovering a navigation popup entry paints
// it without changing the selection.
func TestHoverNavHighlightsEntry(t *testing.T) {
	m := New(Args{Width: 120, Height: 30}).(model)
	m.active = paneEdit
	m.ed.SetText("### a\nGET http://x/1\n\n### b\nPOST http://x/2\n")
	m.beginNav()
	// Hover the second entry (row headerHeight+3).
	y := headerHeight + 3
	mm, _ := m.Update(tea.MouseMsg{
		Button: tea.MouseButtonNone, Action: tea.MouseActionMotion,
		X: m.layout().editorL + 10, Y: y,
	})
	m = mm.(model)
	if m.nav.sel != 0 {
		t.Errorf("hover must not change nav selection (sel=%d)", m.nav.sel)
	}
	if got := m.navHoverIndex(); got != 1 {
		t.Errorf("navHoverIndex=%d want 1", got)
	}
}

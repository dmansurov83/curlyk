package tui

import (
	"os"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

// TestFilesPanelMouseSelect verifies a left click on a file row selects it and
// focuses the files panel, without opening it.
func TestFilesPanelMouseSelect(t *testing.T) {
	dir := t.TempDir()
	old, _ := os.Getwd()
	defer os.Chdir(old)
	os.Chdir(dir)
	os.WriteFile("a.http", []byte("### a\nGET http://a\n"), 0o644)
	os.WriteFile("b.http", []byte("### b\nGET http://b\n"), 0o644)

	m := New(Args{Width: 120, Height: 30}).(model)
	m.active = paneEdit
	m.filesPanel = &filesPanel{all: []string{"a.http", "b.http"}}

	fw := m.filesWidth()
	// Row 0 = "+ Новый файл" (filter empty), row 1 = "a.http" at y=headerHeight+4.
	res, _ := m.Update(tea.MouseMsg{
		Button: tea.MouseButtonLeft, Action: tea.MouseActionPress,
		X: fw / 2, Y: headerHeight + 4,
	})
	r := res.(model)
	if r.active != paneFiles {
		t.Errorf("active=%v want paneFiles", r.active)
	}
	if r.filesPanel == nil || r.filesPanel.sel != 1 {
		t.Errorf("filesPanel.sel=%v want 1", r.filesPanel.sel)
	}
	// single click must not open a file (editor must not contain a.http's request)
	if strings.Contains(r.ed.Text(), "GET http://a") {
		t.Error("single click must not open a file")
	}
}

// TestFilesPanelMouseDoubleClickOpens verifies a double click on a file row
// opens that file into the editor.
func TestFilesPanelMouseDoubleClickOpens(t *testing.T) {
	dir := t.TempDir()
	old, _ := os.Getwd()
	defer os.Chdir(old)
	os.Chdir(dir)
	os.WriteFile("a.http", []byte("### a\nGET http://a\n"), 0o644)

	m := New(Args{Width: 120, Height: 30}).(model)
	m.active = paneEdit
	m.filesPanel = &filesPanel{all: []string{"a.http"}}

	fw := m.filesWidth()
	y := headerHeight + 4 // row 1 = "a.http"
	click := &tea.MouseMsg{Button: tea.MouseButtonLeft, Action: tea.MouseActionPress, X: fw / 2, Y: y}

	mm, _ := m.Update(*click) // first click (single)
	r := mm.(model)
	r.lastClickTime = time.Now()
	mm2, _ := r.Update(*click) // immediate second click = double
	r = mm2.(model)
	if r.active != paneEdit {
		t.Errorf("after double-click active=%v want paneEdit", r.active)
	}
	if !strings.Contains(r.ed.Text(), "GET http://a") {
		t.Errorf("double-click should open a.http, editor=%q", r.ed.Text())
	}
}
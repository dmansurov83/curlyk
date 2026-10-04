package tui

import (
	"os"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

// TestFilesPanelMouseSelect verifies a left click on a file row selects and
// opens it into the editor.
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
	// Row 1 = "a.http" at y=headerHeight+4 (row 0 is "+ Новый файл").
	res, _ := m.Update(tea.MouseMsg{
		Button: tea.MouseButtonLeft, Action: tea.MouseActionPress,
		X: fw / 2, Y: headerHeight + 2,
	})
	r := res.(model)
	if r.filesPanel == nil || r.filesPanel.sel != 1 {
		t.Errorf("filesPanel.sel=%v want 1", r.filesPanel.sel)
	}
	// single click must open a.http into the editor
	if !strings.Contains(r.ed.Text(), "GET http://a") {
		t.Errorf("single click should open a.http, editor=%q", r.ed.Text())
	}
}

// TestFilesPanelMouseSelectNew keeps verifying a click on the "+ Новый файл"
// pseudo-entry (row 0) creates a fresh buffer rather than opening a file.
func TestFilesPanelMouseSelectNew(t *testing.T) {
	dir := t.TempDir()
	old, _ := os.Getwd()
	defer os.Chdir(old)
	os.Chdir(dir)
	os.WriteFile("a.http", []byte("### a\nGET http://a\n"), 0o644)

	m := New(Args{Width: 120, Height: 30}).(model)
	m.active = paneEdit
	m.ed.SetText("old")
	m.filesPanel = &filesPanel{all: []string{"a.http"}}

	fw := m.filesWidth()
	res, _ := m.Update(tea.MouseMsg{
		Button: tea.MouseButtonLeft, Action: tea.MouseActionPress,
		X: fw / 2, Y: headerHeight + 1, // row 0 = "+ Новый файл"
	})
	r := res.(model)
	if r.filesPanel == nil || r.filesPanel.sel != 0 {
		t.Errorf("filesPanel.sel=%v want 0", r.filesPanel.sel)
	}
	if r.ed.Text() != "" {
		t.Errorf("click on new-file entry should clear editor, got %q", r.ed.Text())
	}
}

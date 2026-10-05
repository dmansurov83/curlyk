package tui

import (
	"os"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

// TestNewBufferThenSave verifies Ctrl+N clears the editor and Ctrl+S on an
// unnamed buffer asks for a file name and then saves to it.
func TestNewBufferThenSave(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)
	m := New(Args{Width: 120, Height: 30}).(model)
	m.active = paneEdit

	// Ctrl+N -> empty
	m2, _ := m.handleKey(tea.KeyMsg{Type: tea.KeyCtrlN})
	m = m2.(model)
	if m.ed.Text() != "" {
		t.Fatalf("new buffer should be empty, got %q", m.ed.Text())
	}

	// type something
	m.ed.SetText("POST http://y/2\nContent-Type: a\n\n")

	// Ctrl+S on unnamed buffer -> opens save-as dialog
	m3, _ := m.handleKey(tea.KeyMsg{Type: tea.KeyCtrlS})
	m = m3.(model)
	if m.dialog == nil || m.dialog.input == nil {
		t.Fatal("Ctrl+S on unnamed buffer should open save-as dialog")
	}

	// type a name and press Enter
	m.dialog.input.SetValue("newfile")
	m4, _ := m.handleKey(tea.KeyMsg{Type: tea.KeyEnter})
	m = m4.(model)
	if m.filePath != "newfile.http" {
		t.Fatalf("filePath=%q want newfile.http", m.filePath)
	}
	data, err := os.ReadFile(m.filePath)
	if err != nil {
		t.Fatalf("file not written: %v", err)
	}
	if !strings.Contains(string(data), "POST http://y/2") {
		t.Errorf("content not saved:\n%s", data)
	}
}

func chdir(t *testing.T, dir string) {
	t.Helper()
	wd, _ := os.Getwd()
	t.Cleanup(func() { os.Chdir(wd) })
	os.Chdir(dir)
}

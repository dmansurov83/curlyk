package tui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestFilePickerStartAndNavigate(t *testing.T) {
	dir := t.TempDir()
	wd, _ := os.Getwd()
	defer os.Chdir(wd)
	os.Chdir(dir)
	os.WriteFile(filepath.Join(dir, "a.http"), []byte("GET http://a/1\n"), 0o644)
	os.WriteFile(filepath.Join(dir, "b.http"), []byte("POST http://b/2\n"), 0o644)
	os.WriteFile(filepath.Join(dir, "notes.txt"), []byte("x"), 0o644)

	m := New(Args{Width: 100, Height: 24}).(model)
	m.startFilePicker()
	if m.filePicker == nil {
		t.Fatal("picker should start")
	}
	// only .http files listed
	if len(m.filePicker.files) != 2 {
		t.Fatalf("expected 2 .http files, got %d", len(m.filePicker.files))
	}
	out := m.View()
	if !strings.Contains(out, "a.http") || !strings.Contains(out, "b.http") {
		t.Errorf("picker view missing files:\n%s", out)
	}
	if strings.Contains(out, "notes.txt") {
		t.Error("non-.http file should not be listed")
	}

	// navigate down and open
	m2, _ := m.handleKey(tea.KeyMsg{Type: tea.KeyDown})
	m = m2.(model)
	if m.filePicker.sel != 1 {
		t.Errorf("sel=%d want 1", m.filePicker.sel)
	}
	m3, _ := m.handleKey(tea.KeyMsg{Type: tea.KeyEnter})
	m = m3.(model)
	if m.filePicker != nil {
		t.Fatal("picker should close after open")
	}
	if m.filePath != "b.http" {
		t.Errorf("filePath=%q want b.http", m.filePath)
	}
	if !strings.Contains(m.ed.Text(), "POST http://b/2") {
		t.Errorf("content of b.http not loaded:\n%s", m.ed.Text())
	}
}
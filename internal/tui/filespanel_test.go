package tui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

// TestFilesPanelStartupSelectsOpenFile verifies the sidebar highlights the
// file opened on startup.
func TestFilesPanelStartupSelectsOpenFile(t *testing.T) {
	dir := t.TempDir()
	wd, _ := os.Getwd()
	defer os.Chdir(wd)
	os.Chdir(dir)
	os.WriteFile(filepath.Join(dir, "open.http"), []byte("GET http://a/1\n"), 0o644)
	os.WriteFile(filepath.Join(dir, "other.http"), []byte("GET http://b/2\n"), 0o644)

	m := New(Args{FilePath: "open.http", Width: 120, Height: 30}).(model)
	if m.filePath != "open.http" {
		t.Fatalf("filePath=%q want open.http", m.filePath)
	}
	if m.filesPanel.sel != 1 {
		t.Errorf("startup sel=%d want 1 (open.http after the new-file row)", m.filesPanel.sel)
	}
	v := m.View()
	if !strings.Contains(v, "▸ open.http") {
		t.Errorf("open.http not highlighted in view:\n%s", v)
	}
}

// TestFilesPanelSelectFile verifies arrow navigation includes the "new file"
// row and selecting an actual file opens it (index offset).
func TestFilesPanelSelectFile(t *testing.T) {
	dir := t.TempDir()
	wd, _ := os.Getwd()
	defer os.Chdir(wd)
	os.Chdir(dir)
	os.WriteFile(filepath.Join(dir, "a.http"), []byte("GET http://a/1\n"), 0o644)

	m := New(Args{Width: 120, Height: 30}).(model)
	m.active = paneFiles
	m.filesPanel = &filesPanel{all: httpFilesInDir(".")}

	// items: [Новый файл, a.http]; sel=0 -> new file. Move down to a.http (sel=1).
	m2, _ := m.handleKey(tea.KeyMsg{Type: tea.KeyDown})
	m = m2.(model)
	if m.filesPanel.sel != 1 {
		t.Fatalf("sel=%d want 1", m.filesPanel.sel)
	}
	// Enter opens a.http (skipping the new-file row)
	m3, _ := m.handleKey(tea.KeyMsg{Type: tea.KeyEnter})
	m = m3.(model)
	if m.filePath != "a.http" {
		t.Errorf("filePath=%q want a.http", m.filePath)
	}
	if !strings.Contains(m.ed.Text(), "GET http://a/1") {
		t.Errorf("a.http content not loaded:\n%s", m.ed.Text())
	}
}

// TestFilesPanelFilter verifies typing filters and removes the new-file row.
func TestFilesPanelFilter(t *testing.T) {
	m := New(Args{Width: 120, Height: 30}).(model)
	m.active = paneFiles
	m.filesPanel = &filesPanel{all: []string{"alpha.http", "beta.http"}}

	// type "be"
	m2, _ := m.handleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("be")})
	m = m2.(model)
	if m.filesPanel.filter != "be" {
		t.Fatalf("filter=%q want be", m.filesPanel.filter)
	}
	// with a filter there is no new-file row, so itemCount == filtered files
	if c := m.filesPanel.itemCount(); c != 1 {
		t.Errorf("itemCount with filter=%d want 1", c)
	}
}

// TestFilesPanelSwitchSavesCurrent verifies that switching to another file
// persists the current buffer before loading the new one.
func TestFilesPanelSwitchSavesCurrent(t *testing.T) {
	dir := t.TempDir()
	wd, _ := os.Getwd()
	defer os.Chdir(wd)
	os.Chdir(dir)
	os.WriteFile(filepath.Join(dir, "a.http"), []byte("GET http://a/1\n"), 0o644)
	os.WriteFile(filepath.Join(dir, "b.http"), []byte("GET http://b/2\n"), 0o644)

	m := New(Args{Width: 120, Height: 30}).(model)
	m.active = paneFiles
	m.filesPanel = &filesPanel{all: httpFilesInDir(".")}

	// open a.http
	m.filesPanel.sel = 1
	m.openPanelFile()
	if m.filePath != "a.http" || !strings.Contains(m.ed.Text(), "GET http://a/1") {
		t.Fatalf("failed to open a.http: filePath=%q", m.filePath)
	}

	// edit a.http and mark dirty
	m.ed.SetText("GET http://a/EDITED\n")
	m.markDirty()
	if !m.dirty {
		t.Fatal("expected dirty after edit")
	}

	// switch to b.http
	m.filesPanel.sel = 2
	m.openPanelFile()
	if m.filePath != "b.http" {
		t.Fatalf("filePath=%q want b.http", m.filePath)
	}
	// the edited a.http content must have been persisted to disk
	onDisk, _ := os.ReadFile(filepath.Join(dir, "a.http"))
	if !strings.Contains(string(onDisk), "GET http://a/EDITED") {
		t.Errorf("a.http was not saved on switch; on disk: %q", onDisk)
	}
}
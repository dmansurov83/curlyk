package tui

import (
	"os"
	"path/filepath"
	"slices"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/bubbles/textinput"
)

// filesPanel is the left sidebar listing .http files with a search/filter box.
type filesPanel struct {
	all    []string // all .http files in the working dir
	filter string   // search text
	sel    int      // selected index into the filtered list
}

// filtered returns files matching the current filter.
func (p *filesPanel) filtered() []string {
	if p.filter == "" {
		return p.all
	}
	f := strings.ToLower(p.filter)
	var out []string
	for _, name := range p.all {
		if strings.Contains(strings.ToLower(name), f) {
			out = append(out, name)
		}
	}
	return out
}

// httpFilesInDir lists .http filenames in path (sorted).
func httpFilesInDir(path string) []string {
	entries, err := os.ReadDir(path)
	if err != nil {
		return nil
	}
	var out []string
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		if strings.HasSuffix(strings.ToLower(e.Name()), ".http") {
			out = append(out, e.Name())
		}
	}
	slices.Sort(out)
	return out
}

// panelKey handles keys when the left files panel has focus.
func (m *model) panelKey(msg tea.KeyMsg) tea.Cmd {
	p := m.filesPanel
	if p == nil {
		m.filesPanel = &filesPanel{}
		p = m.filesPanel
	}
	if p.all == nil {
		p.all = httpFilesInDir(".")
	}
	key := msg.String()
	total := p.itemCount()
	switch key {
	case "up":
		if p.sel > 0 {
			p.sel--
		}
	case "down":
		if p.sel < total-1 {
			p.sel++
		}
	case "enter":
		m.openPanelFile()
	case "backspace":
		if len(p.filter) > 0 {
			p.filter = p.filter[:len(p.filter)-1]
			p.sel = 0
		}
	case "esc":
		if p.filter != "" {
			p.filter = ""
			p.sel = 0
		} else {
			m.active = paneEdit
		}
	case "tab":
		m.active = paneEdit
	default:
		// typing filters the list
		if rs := msg.Runes; len(rs) > 0 && key != " " {
			p.filter += string(rs)
			p.sel = 0
		}
	}
	return nil
}

// itemCount returns the number of selectable rows: files, plus the "new file"
// pseudo-entry when the filter is empty.
func (p *filesPanel) itemCount() int {
	n := len(p.filtered())
	if p.filter == "" {
		n++ // the "+ Новый файл" entry
	}
	return n
}

// openPanelFile opens the currently selected file, or creates a new one if the
// "new file" pseudo-entry is selected.
func (m *model) openPanelFile() {
	p := m.filesPanel
	if p == nil {
		return
	}
	// When the filter is empty, row 0 is "+ Новый файл".
	if p.filter == "" && p.sel == 0 {
		m.rememberCursor(m.filePath)
		m.newBuffer()
		m.active = paneEdit
		return
	}
	listed := p.filtered()
	fileIdx := p.sel
	if p.filter == "" {
		fileIdx = p.sel - 1 // skip the "new file" row
	}
	if fileIdx < 0 || fileIdx >= len(listed) {
		return
	}
	name := listed[fileIdx]
	data, err := os.ReadFile(name)
	if err != nil {
		m.status = "Не удалось открыть " + name + ": " + err.Error()
		return
	}
	// remember where the cursor was in the file being left
	m.rememberCursor(m.filePath)
	m.ed.SetText(string(data))
	m.filePath = name
	m.active = paneEdit
	m.status = "Открыт " + name
	rememberLastOpened(name)
	// restore the cursor position this file was left at
	m.applyCursor(name)
}

// refreshFilesPanel rescans the working directory for .http files.
func (m *model) refreshFilesPanel() {
	entries, err := os.ReadDir(".")
	if err != nil {
		return
	}
	var files []string
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		if strings.HasSuffix(strings.ToLower(e.Name()), ".http") {
			files = append(files, e.Name())
		}
	}
	if m.filesPanel == nil {
		m.filesPanel = &filesPanel{}
	}
	p := m.filesPanel
	p.all = files
	// keep selection within bounds
	if p.sel >= len(p.filtered()) {
		p.sel = 0
	}
}

// newBuffer clears the editor into a fresh unnamed buffer (Ctrl+N).
func (m *model) newBuffer() {
	m.ed.SetText("")
	m.filePath = ""
	m.active = paneEdit
	m.status = "Новый файл (Ctrl+S — сохранить)"
}

// saveBuffer writes the editor to its file, or to the session file when no
// explicit file is open (Ctrl+S).
func (m *model) saveBuffer() {
	target := m.filePath
	if target == "" {
		target = sessionPath()
	}
	m.ed.sanitize()
	if err := os.WriteFile(target, []byte(m.ed.Text()), 0o644); err != nil {
		m.status = "Не удалось сохранить: " + err.Error()
		return
	}
	if m.filePath == "" {
		m.filePath = sessionPath()
	}
	m.status = "Сохранено в " + filepath.Base(target)
}

// beginSaveAs opens an input to type a file name for an unnamed buffer.
func (m *model) beginSaveAs() {
	ti := textinput.New()
	ti.Placeholder = "имя файла.http"
	ti.Focus()
	ti.Width = 40
	m.saveAs = &ti
	m.active = paneEdit
}

// saveBufferAs saves the editor to the given file name and binds it.
func (m *model) saveBufferAs(name string) {
	if !strings.HasSuffix(strings.ToLower(name), ".http") {
		name += ".http"
	}
	m.ed.sanitize()
	if err := os.WriteFile(name, []byte(m.ed.Text()), 0o644); err != nil {
		m.status = "Не удалось сохранить: " + err.Error()
		return
	}
	m.filePath = name
	m.status = "Сохранено в " + name
	rememberLastOpened(name)
	m.refreshFilesPanel()
}
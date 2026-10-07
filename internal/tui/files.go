package tui

import (
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/user/curlyk/internal/i18n"
)

// filesPanel is the left sidebar listing .http files with a search/filter box
// and an environment-profile section at the bottom.
type filesPanel struct {
	all    []string // all .http files in the working dir
	filter string   // search text
	sel    int      // selected index into the filtered list
	// profiles lists *.profile environment files in the working dir.
	profiles []string
	// onProfiles is true when the sidebar cursor is on the profile section
	// (bottom of the panel) rather than the file list.
	onProfiles bool
	// profSel is the selected index into profiles.
	profSel int
	// profNew is true when the "+ Новый профиль" action row is focused (only
	// meaningful while onProfiles is true).
	profNew bool
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

// loadProfiles rescans the working directory for *.profile files into the panel.
func (p *filesPanel) loadProfiles() {
	p.profiles = profileFilesInDir(".")
	if p.profSel >= len(p.profiles) {
		p.profSel = 0
	}
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
	if p.profiles == nil {
		p.loadProfiles()
	}
	key := msg.String()
	switch key {
	case "up", "down":
		m.movePanelSelection(key)
	case "enter":
		if p.onProfiles && p.profNew {
			m.beginProfileAs()
		} else if p.onProfiles {
			// open in the editor AND activate, exactly like opening a file
			m.openProfile(p.profiles[p.profSel])
		} else {
			m.openPanelFile()
		}
	case "backspace":
		if len(p.filter) > 0 {
			p.filter = p.filter[:len(p.filter)-1]
			p.sel = 0
			p.onProfiles = false
		}
	case "esc":
		if p.filter != "" {
			p.filter = ""
			p.sel = 0
			p.onProfiles = false
		} else if p.onProfiles {
			p.onProfiles = false
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
			p.onProfiles = false
		}
	}
	return nil
}

// movePanelSelection moves the sidebar cursor between the file rows and the
// profile section. Item selection (files + "New file") is counted by
// itemCount(); once past it, the cursor enters the profile section.
func (m *model) movePanelSelection(dir string) {
	p := m.filesPanel
	if p == nil {
		return
	}
	if dir == "up" {
		if p.onProfiles && p.profNew {
			// from new-profile row to the last profile (or first file if none)
			p.profNew = false
			if len(p.profiles) > 0 {
				p.profSel = len(p.profiles) - 1
			} else if p.itemCount() > 0 {
				p.onProfiles = false
				p.sel = p.itemCount() - 1
			}
			return
		}
		if p.onProfiles && p.profSel > 0 {
			p.profSel--
			return
		}
		if p.onProfiles && p.profSel == 0 {
			// jump back into the file list at its last row
			p.onProfiles = false
			p.profNew = false
			p.sel = p.itemCount() - 1
			return
		}
		if p.sel > 0 {
			p.sel--
		}
		return
	}
	// down
	if !p.onProfiles {
		if p.sel < p.itemCount()-1 {
			p.sel++
			return
		}
		// past the last file row: enter the profile section (first profile, or
		// straight to the new-profile action when none exist)
		p.onProfiles = true
		p.profSel = 0
		p.profNew = len(p.profiles) == 0
		return
	}
	if p.profNew {
		return // already at the bottom
	}
	if p.profSel < len(p.profiles)-1 {
		p.profSel++
		return
	}
	// last profile -> new-profile action row
	p.profNew = true
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
		m.saveCurrentBeforeSwitch()
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
		m.status = i18n.T("err.openFile", name, err.Error())
		return
	}
	// persist the current buffer before switching away to it
	m.saveCurrentBeforeSwitch()
	// remember where the cursor was in the file being left
	m.rememberCursor(m.filePath)
	m.ed.SetText(string(data))
	m.filePath = name
	m.dirty = false
	m.active = paneEdit
	m.complete = nil
	m.status = i18n.T("status.opened", name)
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
	p.loadProfiles()
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
	m.dirty = false
	m.active = paneEdit
	m.complete = nil
	m.status = i18n.T("status.newFile")
}

// saveCurrentBeforeSwitch persists the current editor buffer to its file (or
// the session file when unnamed) before switching to another file, so unsaved
// changes are not lost when the user opens a different file.
func (m *model) saveCurrentBeforeSwitch() {
	if m.ed == nil || !m.dirty {
		return
	}
	saved := m.filePath
	m.saveBuffer()
	if m.filePath == "" && saved != "" {
		m.filePath = saved
	}
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
		m.status = i18n.T("err.save", err.Error())
		return
	}
	if m.filePath == "" {
		m.filePath = sessionPath()
	}
	m.dirty = false
	m.status = i18n.T("status.saved", filepath.Base(target))
}

// beginSaveAs opens a modal dialog to type a file name for an unnamed buffer.
func (m *model) beginSaveAs() {
	ti := textinput.New()
	ti.Placeholder = i18n.T("placeholder.filename")
	ti.Focus()
	ti.Width = 40
	m.dialog = m.newInputDialog(i18n.T("saveAs.title"), &ti, []dialogButton{
		{label: i18n.T("saveAs.cancel"), action: dialogCancel},
		{label: i18n.T("saveAs.save"), action: dialogConfirm, defaultBtn: true},
	}, func(mm *model, act dialogAction, input string) tea.Cmd {
		if act != dialogConfirm {
			return nil
		}
		name := strings.TrimSpace(input)
		if name != "" {
			mm.saveBufferAs(name)
		}
		return nil
	})
}

// saveBufferAs saves the editor to the given file name and binds it.
func (m *model) saveBufferAs(name string) {
	if !strings.HasSuffix(strings.ToLower(name), ".http") {
		name += ".http"
	}
	m.ed.sanitize()
	if err := os.WriteFile(name, []byte(m.ed.Text()), 0o644); err != nil {
		m.status = i18n.T("err.save", err.Error())
		return
	}
	m.filePath = name
	m.dirty = false
	m.status = i18n.T("status.saved", name)
	rememberLastOpened(name)
	m.refreshFilesPanel()
}

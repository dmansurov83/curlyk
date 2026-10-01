package tui

import (
	"os"
	"path/filepath"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/user/curlyk/internal/i18n"
	"github.com/user/curlyk/internal/settings"
)

// autosaveDelay is how long to wait (debounce) after the last edit before
// silently persisting the editor.
const autosaveDelay = 5 * time.Second

// autosaveTick is how often the autosave loop wakes to check for work.
const autosaveTick = 500 * time.Millisecond

// autosaveMsg triggers the debounced background save check.
type autosaveMsg struct{}

// markDirty records that the editor changed and resets the debounce deadline.
// Called once per user edit; the autosave loop persists once edits pause.
func (m *model) markDirty() {
	m.dirty = true
	m.autosaveDeadline = time.Now().Add(autosaveDelay)
}

// autosaveLoop is the periodic command driving autosave.
func autosaveLoop() tea.Cmd {
	return tea.Tick(autosaveTick, func(time.Time) tea.Msg { return autosaveMsg{} })
}

// applyAutosave saves the buffer when dirty and the debounce window elapsed.
func (m *model) applyAutosave(now time.Time) {
	if !m.dirty {
		return
	}
	if now.Before(m.autosaveDeadline) {
		return // still in the debounce window; keep waiting
	}
	m.saveBuffer()
	m.dirty = false
}

// sessionPath returns the session file configured in appsettings.yml, falling
// back to the HTTPTOOL_SESSION_PATH env var (used by tests) or the default.
func sessionPath() string {
	if p := os.Getenv("HTTPTOOL_SESSION_PATH"); p != "" {
		return p
	}
	s := settings.Load()
	if s.SessionFile != "" {
		return s.SessionFile
	}
	return "last.session.http"
}

// lastOpenedPath returns the last explicitly opened file from settings.
func lastOpenedPath() string {
	s := settings.Load()
	return s.LastOpenedFile
}

// rememberLastOpened persists the given path as the last opened file.
func rememberLastOpened(path string) {
	s := settings.Load()
	if s.LastOpenedFile == path {
		return
	}
	s.LastOpenedFile = path
	_ = s.Save()
}

// quitWithSave saves the editor content and returns a Quit command.
func (m *model) quitWithSave() tea.Cmd {
	m.saveSession()
	return tea.Quit
}

// saveSession persists the editor: if an explicit file is open it is saved in
// place; otherwise the content is written to last.session.http in the current
// folder (visible next to the program).
func (m *model) saveSession() {
	m.ed.sanitize()
	target := m.filePath
	if target == "" {
		target = sessionPath()
	}
	if err := os.WriteFile(target, []byte(m.ed.Text()), 0o644); err != nil {
		m.status = i18n.T("err.save", err.Error())
		return
	}
	if m.filePath == "" {
		m.filePath = sessionPath()
	}
	// persist cursor position and other UI state
	s := settings.Load()
	s.CursorRow = m.ed.curRow
	s.CursorCol = m.ed.curCol
	s.EditorScroll = m.ed.scroll
	s.ActivePane = int(m.active)
	// also remember the current file's cursor in the per-file map
	if m.filePath != "" {
		if abs, err := filepath.Abs(m.filePath); err == nil {
			if s.FileCursors == nil {
				s.FileCursors = make(map[string]settings.CursorPos)
			}
			s.FileCursors[abs] = settings.CursorPos{Row: m.ed.curRow, Col: m.ed.curCol, Scroll: m.ed.scroll}
		}
	}
	_ = s.Save()
}

// persistCursorIfChanged writes the current editor cursor position for the
// open file into appsettings.yml when it differs from the last-persisted value.
// Called from the periodic autosave loop so the position survives even if the
// terminal/process is closed without a clean quit (Esc Esc / F10).
func (m *model) persistCursorIfChanged() {
	if m.filePath == "" || m.ed == nil {
		return
	}
	cur := editorPos{row: m.ed.curRow, col: m.ed.curCol, scroll: m.ed.scroll}
	if cur == m.lastPersisted {
		return
	}
	// keep the in-memory per-file map and go through rememberCursor so both the
	// map and the yml stay consistent.
	m.rememberCursor(m.filePath)
	m.lastPersisted = cur
}

// rememberCursor records the current editor position under the given file path
// so it can be restored when the file is opened again. It also persists the
// position to appsettings.yml so it survives a restart.
func (m *model) rememberCursor(path string) {
	if path == "" || m.ed == nil {
		return
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		abs = path
	}
	if m.perFileCursor == nil {
		m.perFileCursor = make(map[string]editorPos)
	}
	p := editorPos{row: m.ed.curRow, col: m.ed.curCol, scroll: m.ed.scroll}
	m.perFileCursor[abs] = p
	// persist to settings so the position survives a restart
	s := settings.Load()
	if s.FileCursors == nil {
		s.FileCursors = make(map[string]settings.CursorPos)
	}
	s.FileCursors[abs] = settings.CursorPos{Row: p.row, Col: p.col, Scroll: p.scroll}
	_ = s.Save()
}

// applyCursor restores a previously remembered cursor position for the given
// file onto the editor, clamping to the current buffer size. Returns true if a
// position was applied.
func (m *model) applyCursor(path string) {
	if m.ed == nil || path == "" {
		return
	}
	if m.perFileCursor == nil {
		m.perFileCursor = make(map[string]editorPos)
		// seed from settings so positions survive a restart
		s := settings.Load()
		for k, v := range s.FileCursors {
			m.perFileCursor[k] = editorPos{row: v.Row, col: v.Col, scroll: v.Scroll}
		}
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		abs = path
	}
	p, ok := m.perFileCursor[abs]
	if !ok {
		return
	}
	if p.row >= 0 && p.row < len(m.ed.Lines()) {
		m.ed.curRow = p.row
	}
	if p.col >= 0 {
		m.ed.curCol = p.col
	}
	if p.scroll >= 0 {
		m.ed.scroll = p.scroll
	}
	m.ed.clampCol()
}

package tui

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/user/curlyk/httptool/internal/curl"
	"github.com/user/curlyk/httptool/internal/httpfile"
	"github.com/user/curlyk/httptool/internal/runner"
	"github.com/user/curlyk/httptool/internal/settings"
)

type pane int

type importMode struct {
	input textinput.Model
}

const (
	paneEdit pane = iota
	paneResp
	paneFiles
)

type runState int

const (
	stateIdle runState = iota
	stateRunning
)

// model is the top-level bubbletea model.
type model struct {
	ed         *editor
	width      int
	height     int
	active     pane
	response   string
	respScroll int
	status     string
	state      runState
	filePath   string
	lastBody   []byte
	importing  *importMode
	// selection (mouse drag) anchor and active flag
	selAnchorRow, selAnchorCol int
	selActive                  bool
	// last-left-click tracking for double-click word selection
	lastClickRow, lastClickCol int
	lastClickTime              time.Time
	// last-left-click in the files panel (for double-click to open a file)
	lastClickFilesRow, lastClickFilesY int
	// last Esc press time (double-Esc quits)
	lastEscTime time.Time
	// response-pane text selection
	respSelActive                              bool
	respSelAnchorRow, respSelAnchorCol         int
	respSelCurRow, respSelCurCol               int
	// mouse session: true when a drag happened between press and release
	mouseDragged bool
	// true when the most recent press was a double-click (keeps the word selection)
	lastWasDouble bool
	// response meta block (HTTP status + timing + headers) shown above the body
	respHeader string
	// number of fixed header rows rendered above the scrollable body
	respHeaderLines int
	// horizontal scroll offset (in cells) for wide response content
	respHScroll int
	// left-hand file explorer panel (nil when not initialised)
	filesPanel *filesPanel
	// save-as input state (non-nil while asking for a file name)
	saveAs *textinput.Model
	// action popup shown on Enter over a request line (nil when inactive)
	actionMenu *actionMenu
	// dirty tracks whether the editor has unsaved changes since the last save.
	dirty bool
	// autosaveDeadline is the time until which edits keep postponing the save.
	autosaveDeadline time.Time
	// editBatchOpen marks that a character-edit batch is open, coalescing a
	// fast run of single-char edits (e.g. a paste delivered character-by-char on
	// Windows Terminal) into a single undo step. It closes after a short gap.
	editBatchOpen    bool
	editBatchDeadline time.Time
	// burstText accumulates the characters of the current burst (including
	// newlines) so a char-by-char pasted cURL can be detected and converted when
	// the batch closes. Stored as []byte: the model is copied by value in
	// bubbletea, and a strings.Builder must not be copied.
	burstText []byte
}

// editBatchGap is how long a burst of characters may pause before its undo
// batch closes.
const editBatchGap = 400 * time.Millisecond

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

// startEditBatch opens (or keeps open) the character-edit batch that turns a
// fast run of single-char edits into a single undo step. It is called for each
// printed character; the deadline is extended so a paste burst stays together.
func (m *model) startEditBatch() {
	if !m.editBatchOpen {
		m.ed.BeginUndo()
		m.editBatchOpen = true
		m.burstText = m.burstText[:0]
	}
	m.editBatchDeadline = time.Now().Add(editBatchGap)
}

// appendBurst records a character typed/pasted in the current burst. Newlines
// are kept as '\n' so a multi-line pasted cURL is detected and converted.
func (m *model) appendBurst(text string) {
	if m.editBatchOpen {
		m.burstText = append(m.burstText, text...)
	}
}

// closeEditBatchIfExpired closes the edit batch once the burst has paused past
// editBatchGap. Called from the periodic autosave loop. If the burst turns out
// to be a pasted cURL command, it is converted in place.
func (m *model) closeEditBatchIfExpired(now time.Time) {
	if !m.editBatchOpen || !now.After(m.editBatchDeadline) {
		return
	}
	m.ed.EndUndo()
	m.editBatchOpen = false
	text := string(m.burstText)
	m.burstText = m.burstText[:0]
	m.convertBurstCurl(text)
}

// convertBurstCurl checks whether the accumulated burst is a cURL command and,
// if so, replaces the raw burst text with the converted .http block. It uses
// Undo to revert the raw pasted text reliably (no manual range math), then
// inserts the converted block. Returns true when conversion happened.
func (m *model) convertBurstCurl(text string) bool {
	if text == "" || !isCurlCommand(text) {
		return false
	}
	block, err := curl.ImportLine(strings.TrimSpace(text))
	if err != nil {
		return false
	}
	// Revert the raw pasted burst: the batch captured the pre-paste state as an
	// undo snapshot, so one Undo restores it.
	if !m.ed.Undo() {
		return false
	}
	m.ed.BeginUndo() // group the conversion into one undo step
	defer m.ed.EndUndo()
	m.insertText(block)
	m.ed.clampCol()
	m.ed.EnsureVisible()
	m.status = "cURL конвертирован в запрос"
	return true
}

// forceCloseEditBatch closes any open character-edit batch immediately, so a
// fresh undo group can begin.
func (m *model) forceCloseEditBatch() {
	if m.editBatchOpen {
		m.ed.EndUndo()
		m.editBatchOpen = false
		m.burstText = m.burstText[:0]
	}
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

func initialModel() model {
	return model{
		ed:     newEditor(exampleHTTP, 80, 20),
		active: paneEdit,
		status: "Готов. Ctrl+Enter — выполнить, ^Y — импорт cURL, ^K — copy as cURL, Tab — панель, ^D — дамп",
	}
}

// Args configures the initial UI state from the CLI.
type Args struct {
	FilePath string
	Width    int // optional; 0 = default 100
	Height   int // optional; 0 = default 30
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
		m.status = "Не удалось сохранить: " + err.Error()
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
	_ = s.Save()
}

// New builds the start model. If filePath is set, its content is loaded;
// otherwise the last session file is opened if present.
func New(args Args) tea.Model {
	content := exampleHTTP
	openPath := ""
	switch {
	case args.FilePath != "":
		if data, err := os.ReadFile(args.FilePath); err == nil {
			content = string(data)
			openPath = args.FilePath
			rememberLastOpened(args.FilePath)
		}
	default:
		// reopen the last explicitly opened file if it still exists
		if last := lastOpenedPath(); last != "" {
			if data, err := os.ReadFile(last); err == nil {
				content = string(data)
				openPath = last
				break
			}
		}
		// otherwise restore the last session content
		if data, err := os.ReadFile(sessionPath()); err == nil {
			content = string(data)
			openPath = sessionPath()
		}
	}
	ed := newEditor(content, 80, 20)
	w, h := args.Width, args.Height
	if w == 0 {
		w = 100
	}
	if h == 0 {
		h = 30
	}
	ed.width, ed.height = w/2, h-6
	// restore cursor / scroll / pane from settings
	s := settings.Load()
	if s.CursorRow >= 0 && s.CursorRow < len(ed.Lines()) {
		ed.curRow = s.CursorRow
	}
	if s.CursorCol >= 0 {
		ed.curCol = s.CursorCol
		ed.clampCol()
	}
	if s.EditorScroll >= 0 {
		ed.scroll = s.EditorScroll
	}
	pane := paneEdit
	if s.ActivePane == 1 {
		pane = paneResp
	}
	// left file panel: list .http files in the working directory
	fp := &filesPanel{all: httpFilesInDir(".")}
	return model{
		ed:         ed,
		active:     pane,
		filesPanel: fp,
		status:     "Готов. Ctrl+Enter — выполнить, 2xEsc/F10 — выход, ^C/^X/^V — буфер, ^+стрелки — слова",
		filePath:   openPath,
		width:      w,
		height:     h,
	}
}

// Snapshot returns a rendered frame (for tests/snapshots without a terminal).
func Snapshot(args Args) string {
	m := New(args).(model)
	return m.View()
}

// runResultMsg carries the finished request.
type runResultMsg struct {
	res  *runner.Result
	body []byte
}

func (m model) Init() tea.Cmd {
	// Hide the system cursor; we draw our own block cursor in the editor.
	return tea.Batch(tea.HideCursor, autosaveLoop())
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		m.ed.width = m.layout().mid
		m.ed.height = msg.Height - 6
	case tea.KeyMsg:
		return m.handleKey(msg)
	case tea.MouseMsg:
		m, cmd := m.handleMouse(msg)
		return m, cmd
	case runResultMsg:
		return m.applyResponse(msg), nil
	case autosaveMsg:
		now := time.Now()
		m.closeEditBatchIfExpired(now)
		m.applyAutosave(now)
		return m, autosaveLoop()
	}
	return m, nil
}

// editorMaxScroll returns the maximum allowed scroll offset for the editor
// given its visible height, or 0 when the content fits (no scrolling).
func (m *model) editorMaxScroll() int {
	n := len(m.ed.Lines())
	vis := m.ed.height
	if vis < 0 {
		vis = 0
	}
	if n <= vis {
		return 0
	}
	return n - vis
}

// respMaxScroll returns the maximum scroll offset for the response body pane.
func (m *model) respMaxScroll() int {
	lines := respBodyLines(m)
	// visible scrollable rows: content height minus copy button and header
	vis := m.height - 6 - m.respHeaderLines
	if vis < 0 {
		vis = 0
	}
	if len(lines) <= vis {
		return 0
	}
	return len(lines) - vis
}

// clampEditorScroll keeps editor scroll within valid bounds.
func (m *model) clampEditorScroll() {
	mx := m.editorMaxScroll()
	if m.ed.scroll < 0 {
		m.ed.scroll = 0
	}
	if m.ed.scroll > mx {
		m.ed.scroll = mx
	}
}

// screen content layout inside the editor pane (0-based, relative to pane):
//   col 0        : left border (drawn by lipgloss)
//   col 1        : run icon (▶) for request lines, else ' '
//   col 2..5     : line number "%3d "
//   col 6..      : source text
const (
	paneContentX = 1 // first content column after left border
	textColAbs   = 6 // absolute pane column where source text starts
	// headerHeight is the number of fixed rows drawn above the panes (the
	// file-name header). Mouse and scrollbar coordinate math must account for it.
	headerHeight = 1
)

func (m model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// When the files panel is focused, keyboard drives it.
	if m.active == paneFiles {
		return m, m.panelKey(msg)
	}
	// Save-as input: capture a file name.
	if m.saveAs != nil {
		switch msg.String() {
		case "ctrl+c", "esc":
			m.saveAs = nil
		case "enter":
			name := strings.TrimSpace(m.saveAs.Value())
			m.saveAs = nil
			if name != "" {
				m.saveBufferAs(name)
			}
		default:
			updated, _ := m.saveAs.Update(msg)
			m.saveAs = &updated
		}
		return m, nil
	}
	// When the import input is active, forward keys to it.
	if m.importing != nil {
		switch msg.String() {
		case "ctrl+c", "esc":
			m.importing = nil
			return m, nil
		case "enter":
			val := m.importing.input.Value()
			m.importing = nil
			m.performImport(val)
			return m, nil
		case "backspace":
			m.importing.input, _ = m.importing.input.Update(msg)
		case "delete":
			m.importing.input, _ = m.importing.input.Update(msg)
		case "left", "right", "home", "end":
			m.importing.input, _ = m.importing.input.Update(msg)
		default:
			m.importing.input, _ = m.importing.input.Update(msg)
		}
		return m, nil
	}

	// Intercept bracketed paste (e.g. Ctrl+Shift+V / terminal context menu) so
	// a pasted cURL command is auto-converted instead of inserted as raw text.
	if msg.Paste {
		m.active = paneEdit
		text := string(msg.Runes)
		m.pasteOrConvert(text)
		return m, nil
	}

	// Request action popup: while open it consumes all keys.
	if cmd, handled := m.handleMenuKey(msg); handled {
		return m, cmd
	}

	// Ignore unrecognised Ctrl/Alt combinations. On Windows a bare Ctrl (or a
	// modifier read alone) can arrive as e.g. "ctrl+@" with no text payload;
	// without this guard it would fall through to handleEditKey and delete the
	// active selection. Only the known shortcuts below reach the switch.
	key := msg.String()
	if (strings.HasPrefix(key, "ctrl") || strings.HasPrefix(key, "alt")) && !knownModifierKey(key) {
		return m, nil
	}

	switch msg.String() {
	case "ctrl+c":
		// Always copy on Windows: Ctrl+C is captured here (never quits).
		if m.active == paneEdit {
			m.copySelection()
		} else if m.active == paneResp {
			m.copyRespSelection()
		}
	case "ctrl+x":
		if m.active == paneEdit {
			m.cutSelection()
		}
	case "ctrl+v":
		// always paste into the editor (focus editor first)
		m.active = paneEdit
		m.pasteClipboard()
	case "ctrl+z":
		if m.active == paneEdit {
			m.ed.Undo()
		}
	case "ctrl+shift+z":
		if m.active == paneEdit {
			m.ed.Redo()
		}
	case "ctrl+d":
		m.dumpDebug()
		return m, nil
	case "ctrl+k":
		if m.active == paneEdit {
			m.copyAsCurl()
		}
		return m, nil
	case "f10":
		return m, m.quitWithSave()
	case "esc":
		// clear selection if active; a second Esc shortly after quits.
		if m.selActive || m.respSelActive {
			m.selActive = false
			m.respSelActive = false
			m.lastEscTime = time.Time{}
			return m, nil
		}
		now := time.Now()
		if !m.lastEscTime.IsZero() && now.Sub(m.lastEscTime) < 800*time.Millisecond {
			return m, m.quitWithSave()
		}
		m.lastEscTime = now
	case "ctrl+enter", "ctrl+r":
		// On Windows coninput, Ctrl+Enter is indistinguishable from Enter, so we
		// also accept Ctrl+R (reliable modifier key) to run.
		if cmd := m.runRequest(); cmd != nil {
			return m, cmd
		}
	case "enter":
		// Enter on a request line opens the action popup (Выполнить / Copy as
		// cURL); elsewhere it inserts a newline (replacing any selection).
		if m.active == paneEdit && (m.ed.onIcon || isRequestLine(m.ed.Lines(), m.ed.curRow)) {
			m.beginActionMenu()
		} else if m.active == paneEdit {
			if m.selActive && m.hasSelection() {
				m.ed.DeleteRange(m.selAnchorRow, m.selAnchorCol, m.ed.curRow, m.ed.curCol)
				m.selActive = false
				m.selAnchorRow, m.selAnchorCol = m.ed.curRow, m.ed.curCol
			}
			m.ed.Enter()
			m.ed.clampCol()
			m.ed.EnsureVisible()
			m.markDirty()
		}
	case "ctrl+y":
		m.importing = &importMode{
			input: newImportInput(),
		}
	case "tab":
		switch m.active {
		case paneEdit:
			m.active = paneResp
		case paneResp:
			m.active = paneFiles
		case paneFiles:
			m.active = paneEdit
		}
	case "ctrl+s":
		if m.filePath == "" || m.filePath == sessionPath() {
			// unnamed buffer -> ask for a file name
			m.beginSaveAs()
		} else {
			m.saveBuffer()
		}
	case "ctrl+n":
		m.newBuffer()
	case "shift+up", "shift+down", "shift+left", "shift+right",
		"shift+home", "shift+end":
		if m.active == paneEdit {
			m.shiftSelect(msg.String())
		} else if m.active == paneResp {
			m.respShiftSelect(msg.String())
		}
	case "ctrl+left", "ctrl+right", "ctrl+home", "ctrl+end":
		if m.active == paneEdit {
			m.ed.MoveWord(msg.String())
			m.ed.EnsureVisible()
		}
	case "alt+left", "alt+right":
		m.hScrollBy(4 * signOf(msg.String()))
		m.ed.clampCol()
		m.ed.EnsureVisible()
	case "alt+home":
		if m.active == paneEdit {
			m.ed.hScroll = 0
		} else {
			m.respHScroll = 0
		}
	case "up", "down", "left", "right", "home", "end":
		if m.active == paneEdit {
			m.selActive = false
			m.ed.MoveCursor(msg.String())
			m.ed.EnsureVisible()
		} else {
			m.respMoveCursor(msg.String())
		}
	case "pageup", "pgup":
		if m.active == paneResp {
			m.scrollBy(-10)
		} else if m.active == paneEdit {
			m.ed.scroll -= m.ed.height
			m.clampEditorScroll()
		}
	case "pagedown", "pgdown":
		if m.active == paneResp {
			m.scrollBy(10)
		} else if m.active == paneEdit {
			m.ed.scroll += m.ed.height
			m.clampEditorScroll()
		}
	default:
		if m.active == paneEdit {
			m.handleEditKey(msg)
		}
	}
	return m, nil
}

// selectionForLine returns the rune-column selection range on a given 0-based
// line, and whether the line is part of the active drag selection.
func (m *model) selectionForLine(row int) (selStart, selEnd int, hasSel bool) {
	if !m.selActive {
		return 0, 0, false
	}
	aRow, aCol := m.selAnchorRow, m.selAnchorCol
	cRow, cCol := m.ed.curRow, m.ed.curCol
	// Normalize start/end rows.
	startRow, startCol, endRow, endCol := aRow, aCol, cRow, cCol
	if (endRow < startRow) || (endRow == startRow && endCol < startCol) {
		startRow, startCol, endRow, endCol = cRow, cCol, aRow, aCol
	}
	if row < startRow || row > endRow {
		return 0, 0, false
	}
	switch {
	case startRow == endRow:
		if row != startRow {
			return 0, 0, false
		}
		return startCol, endCol, true
	case row == startRow:
		return startCol, m.ed.lineLen(row), true
	case row == endRow:
		return 0, endCol, true
	default:
		return 0, m.ed.lineLen(row), true
	}
}

// isRequestLine reports whether the line at 0-based row is an HTTP request line.
func isRequestLine(lines []string, row int) bool {
	if row < 0 || row >= len(lines) {
		return false
	}
	return httpfile.IsRequestLine(lines[row])
}


// knownModifierKey reports whether key is one of the modifier shortcuts the app
// actually handles. Any other ctrl/alt combo is ignored so it never reaches the
// edit path (which would disturb the active selection on a bare modifier press).
func knownModifierKey(key string) bool {
	switch key {
	case "ctrl+c", "ctrl+x", "ctrl+v", "ctrl+z", "ctrl+shift+z",
		"ctrl+d", "ctrl+k", "ctrl+enter", "ctrl+r", "ctrl+y",
		"ctrl+s", "ctrl+n",
		"ctrl+left", "ctrl+right", "ctrl+home", "ctrl+end",
		"alt+left", "alt+right", "alt+home":
		return true
	}
	return false
}

// handleEditKey processes an editing key in the editor pane. If a text selection
// is active, an editing key (typing a rune, Enter, Backspace, Delete) replaces
// it: the selection is deleted before the action applies. Keys that perform no
// edit (e.g. a lone Ctrl, or an unrecognised modifier combo) leave the selection
// untouched.
func (m *model) handleEditKey(msg tea.KeyMsg) {
	key := msg.String()
	// A "text" edit requires a printable rune. Control characters (what a bare
	// Ctrl or Ctrl+letter often arrives as on Windows, e.g. ctrl+@ = NUL/0x00)
	// are not edits and must not replace the selection.
	hasPrintable := false
	for _, r := range msg.Runes {
		if r >= 0x20 && r != 0x7f {
			hasPrintable = true
			break
		}
	}
	isEdit := key == "enter" || key == "backspace" || key == "delete" || hasPrintable
	if !isEdit {
		return
	}
	m.markDirty()
	// If a selection is active, an editing key replaces it with the edit.
	if m.selActive && m.hasSelection() {
		// this starts a fresh undo group
		m.forceCloseEditBatch()
		m.ed.BeginUndo()
		defer m.ed.EndUndo()
		m.ed.DeleteRange(m.selAnchorRow, m.selAnchorCol, m.ed.curRow, m.ed.curCol)
		m.selActive = false
		m.selAnchorRow, m.selAnchorCol = m.ed.curRow, m.ed.curCol
		switch key {
		case "enter":
			m.ed.Enter()
		case "backspace", "delete":
			// The selection deletion already removed the text; no extra delete.
		default:
			if rs := msg.Runes; len(rs) > 0 {
				m.ed.InsertString(string(rs))
			}
		}
		m.ed.clampCol()
		m.ed.EnsureVisible()
		return
	}
	switch key {
	case "enter", "backspace", "delete":
		// If a paste burst is in progress, an Enter is a pasted newline (part of
		// a multi-line cURL), so keep the batch open and record '\n' for later
		// curl detection. Otherwise a manual Enter/Backspace/Delete starts a
		// fresh undo group.
		if m.editBatchOpen && key == "enter" {
			m.ed.Enter()
			m.appendBurst("\n")
		} else {
			m.forceCloseEditBatch()
			m.ed.BeginUndo()
			defer m.ed.EndUndo()
			switch key {
			case "enter":
				m.ed.Enter()
			case "backspace":
				m.ed.Backspace()
			case "delete":
				m.ed.Delete()
			}
		}
	default:
		// Printable characters: coalesce a fast burst (paste/typing) into one
		// undo step via the timed batch.
		if hasPrintable {
			m.startEditBatch()
			if rs := msg.Runes; len(rs) > 0 {
				m.ed.InsertString(string(rs))
				m.appendBurst(string(rs))
			}
		}
	}
	m.ed.clampCol()
	m.ed.EnsureVisible()
}

// hScrollBy adjusts the active pane's horizontal scroll offset.
func (m *model) hScrollBy(delta int) {
	if m.active == paneResp {
		m.respHScroll += delta
		if m.respHScroll < 0 {
			m.respHScroll = 0
		}
	} else {
		m.ed.hScroll += delta
		if m.ed.hScroll < 0 {
			m.ed.hScroll = 0
		}
	}
}

// signOf returns -1 for "left", +1 otherwise.
func signOf(key string) int {
	if key == "alt+left" {
		return -1
	}
	return 1
}

// dumpDebug writes a snapshot of the UI state to debug.txt for diagnostics.
// Triggered with Ctrl+D. Includes editor text, cursor, scroll, pane layout and
// geometry constants so rendering/mouse bugs can be reproduced offline.
func (m *model) dumpDebug() {
	var b strings.Builder
	lines := m.ed.Lines()
	b.WriteString("== HTTP Tool debug dump ==\n")
	b.WriteString(fmt.Sprintf("time:         %s\n", time.Now().Format("15:04:05")))
	b.WriteString(fmt.Sprintf("width:        %d\n", m.width))
	b.WriteString(fmt.Sprintf("height:       %d\n", m.height))
	b.WriteString(fmt.Sprintf("half:         %d\n", m.width/2))
	b.WriteString(fmt.Sprintf("active pane:  %s\n", paneName(m.active)))
	b.WriteString(fmt.Sprintf("state:        %s\n", stateName(m.state)))
	b.WriteString(fmt.Sprintf("status:       %s\n", m.status))
	b.WriteString(fmt.Sprintf("filePath:     %s\n", m.filePath))
	b.WriteString("\n-- editor --\n")
	b.WriteString(fmt.Sprintf("curRow:       %d (0-based)\n", m.ed.curRow))
	b.WriteString(fmt.Sprintf("curCol:       %d (0-based rune)\n", m.ed.curCol))
	b.WriteString(fmt.Sprintf("onIcon:       %v\n", m.ed.onIcon))
	b.WriteString(fmt.Sprintf("scroll:       %d\n", m.ed.scroll))
	b.WriteString(fmt.Sprintf("height:       %d (visible lines)\n", m.ed.height))
	b.WriteString(fmt.Sprintf("width:        %d\n", m.ed.width))
	b.WriteString(fmt.Sprintf("lineCount:    %d\n", len(lines)))
	b.WriteString("-- editor lines (index : content) --\n")
	for i, ln := range lines {
		b.WriteString(fmt.Sprintf("%3d : %s\n", i+1, ln))
	}
	b.WriteString("\n-- selection --\n")
	b.WriteString(fmt.Sprintf("selActive:    %v\n", m.selActive))
	b.WriteString(fmt.Sprintf("anchor:       (%d,%d)\n", m.selAnchorRow, m.selAnchorCol))
	b.WriteString("\n-- response pane --\n")
	b.WriteString(fmt.Sprintf("respScroll:   %d\n", m.respScroll))
	b.WriteString(fmt.Sprintf("respLen:      %d\n", len(m.response)))
	b.WriteString("\n-- geometry constants --\n")
	b.WriteString(fmt.Sprintf("paneContentX: %d\n", paneContentX))
	b.WriteString(fmt.Sprintf("textColAbs:   %d\n", textColAbs))

	// Actual rendered frame (what would be on screen right now).
	b.WriteString("\n-- RENDERED VIEW (strip ANSI) --\n")
	b.WriteString(stripANSI(m.View()))
	b.WriteString("\n-- END RENDERED VIEW --\n")

	if err := os.WriteFile("debug.txt", []byte(b.String()), 0o644); err != nil {
		m.status = "Ошибка записи debug.txt: " + err.Error()
		return
	}
	m.status = "Дамп записан в debug.txt"
}

func paneName(p pane) string {
	if p == paneEdit {
		return "edit"
	}
	return "response"
}

func stateName(s runState) string {
	if s == stateRunning {
		return "running"
	}
	return "idle"
}

// shiftSelect expands the selection with Shift+arrow navigation.
func (m *model) shiftSelect(key string) {
	if !m.selActive {
		// start a new selection anchored at current cursor
		m.selAnchorRow, m.selAnchorCol = m.ed.curRow, m.ed.curCol
		m.selActive = true
	}
	m.ed.MoveCursor(navKey(key))
	m.ed.EnsureVisible()
}

// navKey maps a shift+* key string to the plain navigation key.
func navKey(key string) string {
	return strings.TrimPrefix(key, "shift+")
}

func (m *model) scrollBy(n int) {
	m.respScroll += n
	if m.respScroll < 0 {
		m.respScroll = 0
	}
	mx := m.respMaxScroll()
	if m.respScroll > mx {
		m.respScroll = mx
	}
}

// runRequest executes the request block under the cursor.
func (m *model) runRequest() tea.Cmd {
	if m.state == stateRunning {
		return nil
	}
	m.ed.sanitize()
	reqs := httpfile.ParseFile(m.ed.Text())
	req := httpfile.GetRequestAtLine(reqs, m.ed.curRow+1)
	if req == nil {
		m.status = "Нет HTTP-запроса под курсором"
		m.active = paneEdit
		return nil
	}
	m.state = stateRunning
	m.status = "Выполняется " + req.Method + " " + req.URL + " ..."
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		res := runner.Run(ctx, *req, runner.Options{FollowRedirects: false})
		if res.Err != nil {
			return runResultMsg{res: res}
		}
		body, err := runner.BodyBytes(res.Response)
		if err != nil {
			return runResultMsg{res: res}
		}
		return runResultMsg{res: res, body: body}
	}
}

// applyResponse renders the response into the right pane.
func (m model) applyResponse(msg runResultMsg) tea.Model {
	m.state = stateIdle
	res := msg.res
	if res.Err != nil {
		m.status = "Ошибка: " + res.Err.Error()
		m.response = ""
		m.active = paneResp
		return m
	}
	var b strings.Builder
	var hdr strings.Builder
	hdr.WriteString("HTTP/" + res.Request.Proto + " " + res.Status + "\n")
	hdr.WriteString("Время: " + res.Duration.Round(time.Millisecond).String() + "\n")
	res.Response.Header.Write(&hdr)
	b.WriteString(hdr.String())
	b.WriteString("\n")
	if len(msg.body) > 0 {
		b.WriteString(formatBody(msg.body))
	}
	m.response = b.String()
	m.respHeader = hdr.String()
	m.respHeaderLines = 0
	if m.respHeader != "" {
		m.respHeaderLines = len(strings.Split(strings.TrimRight(m.respHeader, "\n"), "\n"))
	}
	m.lastBody = msg.body
	m.respScroll = 0
	m.respHScroll = 0
	if res.Response != nil {
		m.status = fmt.Sprintf("Ответ %d", res.Response.StatusCode)
	}
	m.active = paneResp
	return m
}

// performImport parses a pasted cURL command and inserts the result.
func (m *model) performImport(raw string) {
	block, err := curl.ImportLine(raw)
	if err != nil {
		m.status = "Ошибка импорта cURL: " + err.Error()
		return
	}
	cur := m.ed.Text()
	var sb strings.Builder
	if cur != "" && !strings.HasSuffix(cur, "\n") {
		sb.WriteString(cur)
		sb.WriteString("\n")
	}
	sb.WriteString("\n")
	sb.WriteString(block)
	m.ed.SetText(sb.String())
	m.status = "Импортирован запрос из cURL"
	m.active = paneEdit
	m.markDirty()
}

func newImportInput() textinput.Model {
	ti := textinput.New()
	ti.Placeholder = "curl -X POST https://api.example.com -H 'Content-Type: application/json' -d '{...}'"
	ti.Focus()
	ti.CharLimit = 0
	ti.Width = 60
	return ti
}


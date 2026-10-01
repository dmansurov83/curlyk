package tui

import (
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/user/curlyk/internal/curl"
)

// editBatchGap is how long a burst of characters may pause before its undo
// batch closes.
const editBatchGap = 400 * time.Millisecond

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
	case "ctrl+a":
		// Select all: entire editor buffer or entire response body.
		m.selectAll()
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

// knownModifierKey reports whether key is one of the modifier shortcuts the app
// actually handles. Any other ctrl/alt combo is ignored so it never reaches the
// edit path (which would disturb the active selection on a bare modifier press).
func knownModifierKey(key string) bool {
	switch key {
	case "ctrl+c", "ctrl+x", "ctrl+v", "ctrl+z", "ctrl+shift+z",
		"ctrl+d", "ctrl+k", "ctrl+enter", "ctrl+r", "ctrl+y",
		"ctrl+s", "ctrl+n", "ctrl+a",
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

// navKey maps a shift+* key string to the plain navigation key.
func navKey(key string) string {
	return strings.TrimPrefix(key, "shift+")
}

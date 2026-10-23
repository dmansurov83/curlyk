package gui

import "fyne.io/fyne/v2"

// Clipboard support: the editor implements fyne.Shortcutable, so Ctrl+C/X/V/A
// (Cmd on macOS) reach these methods. Fyne delivers the predefined shortcuts to
// the focused widget before its own canvas handlers, so the editor behaves like
// a text widget whenever it has focus.

// clipboard returns the system clipboard, or nil when there is no app yet.
func clipboard() fyne.Clipboard {
	a := fyne.CurrentApp()
	if a == nil {
		return nil
	}
	return a.Clipboard()
}

// Copy puts the selection on the clipboard. It reports whether there was
// something to copy.
func (e *Editor) Copy() bool {
	text := e.doc.selText()
	if text == "" {
		return false
	}
	cb := clipboard()
	if cb == nil {
		return false
	}
	cb.SetContent(text)
	return true
}

// Cut copies the selection to the clipboard and removes it, as one undo step.
func (e *Editor) Cut() bool {
	text := e.doc.selText()
	if text == "" {
		return false
	}
	if cb := clipboard(); cb != nil {
		cb.SetContent(text)
	}
	e.beginEdit(editOther)
	e.doc.selDelete()
	e.afterEdit()
	return true
}

// Paste inserts the clipboard content at the caret, replacing the selection. A
// multi-line paste is split into lines exactly as typed text is.
func (e *Editor) Paste() bool {
	cb := clipboard()
	if cb == nil {
		return false
	}
	text := cb.Content()
	if text == "" {
		return false
	}
	if e.pasteHandler != nil {
		replacement, handled, err := e.pasteHandler(text)
		if err != nil {
			return false
		}
		if handled {
			text = replacement
		}
	}
	e.beginEdit(editOther)
	if !e.doc.insert(text) {
		return false
	}
	e.afterEdit()
	return true
}

// InsertText inserts text as a single undo step (used by autocomplete accepts
// and by the cURL import path).
func (e *Editor) InsertText(text string) bool {
	if text == "" {
		return false
	}
	e.beginEdit(editOther)
	if !e.doc.insert(text) {
		return false
	}
	e.afterEdit()
	return true
}

// SelectAll selects the whole buffer.
func (e *Editor) SelectAll() {
	e.doc.selSelectAll()
	e.ensureCaretVisible()
	e.Refresh()
	e.caretMoved()
}

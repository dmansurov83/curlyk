package gui

import (
	"strings"

	"fyne.io/fyne/v2"

	"github.com/user/curlyk/internal/completion"
	"github.com/user/curlyk/internal/i18n"
)

// maxCompletionItems caps the suggestion list, matching the TUI popup.
const maxCompletionItems = 20

// completionPopup is the autocomplete list model. It is intentionally NOT a
// Fyne popup overlay: showing an overlay redirects the canvas' focus manager to
// the overlay, so the editor would lose the keyboard and arrows/Enter/typing
// would stop reaching it. Instead the list is painted by the editor's own
// renderer (below the caret) while the editor keeps focus and forwards the keys
// it does not use through its key hook — exactly the TUI's behaviour.
type completionPopup struct {
	u *ui

	items []completion.Suggestion
	sel   int
	// open marks a list currently shown; it is what the renderer and the
	// controller check instead of a widget's Visible flag.
	open bool
}

// suggestionsForCaret asks the shared engine what fits the caret's context.
func (u *ui) suggestionsForCaret() []completion.Suggestion {
	row, col := u.ed.Caret()
	line := u.ed.LineAt(row)
	sug := completion.Suggest(line, col, u.completionContext())
	if len(sug) > maxCompletionItems {
		sug = sug[:maxCompletionItems]
	}
	return sug
}

// completionContext collects the candidate words from the buffer and the active
// profile through the shared collector.
func (u *ui) completionContext() completion.Context {
	return completion.ContextForSource(u.ed.Text(), u.profileVars())
}

// openCompletion shows the suggestion list when the caret's context offers any
// candidate. It is a no-op otherwise, so Ctrl+Space on a plain line does nothing
// instead of flashing an empty box. The editor stays focused, so the keys that
// follow (arrows, Enter, typing) keep reaching it.
func (u *ui) openCompletion() {
	if u.complete == nil || u.win == nil {
		return
	}
	items := u.suggestionsForCaret()
	if len(items) == 0 {
		u.closeCompletion()
		return
	}
	u.complete.items = items
	if u.complete.sel >= len(items) {
		u.complete.sel = 0
	}
	u.complete.open = true
	// The editor must hold the keyboard while the list is up.
	u.win.Canvas().Focus(u.ed)
	u.ed.Refresh()
}

// closeCompletion hides the list and returns focus to wherever it belongs.
func (u *ui) closeCompletion() {
	if u.complete == nil {
		return
	}
	u.complete.open = false
	u.complete.items = nil
	u.complete.sel = 0
	if u.win != nil && u.ed != nil {
		u.ed.Refresh()
	}
}

// completionOpen reports whether the list is currently shown.
func (u *ui) completionOpen() bool {
	return u.complete != nil && u.complete.open && len(u.complete.items) > 0
}

// show repositions and repaints an open list after the caret (or the whole
// window) moved. It is a no-op when nothing is shown.
func (u *ui) showCompletion() {
	if u.complete == nil || !u.complete.open {
		return
	}
	u.complete.open = len(u.complete.items) > 0
	u.ed.Refresh()
}

// popupSize returns the list size: wide enough for the longest suggestion and
// tall enough for every row, in editor viewport units.
func (c *completionPopup) popupSize() fyne.Size {
	charW, lineH := c.u.ed.metrics()
	if lineH < 18 {
		lineH = 18
	}
	if charW <= 0 {
		charW = 8
	}

	width := 8 * charW
	for _, item := range c.items {
		w := fyne.MeasureText("▸ "+item.Label, c.u.ed.fontSize, fyne.TextStyle{Monospace: true}).Width
		if w > width {
			width = w
		}
	}
	width += 3 * charW

	rows := len(c.items)
	if rows == 0 {
		rows = 1
	}
	return fyne.NewSize(width, float32(rows)*lineH)
}

// completionKey is the editor key hook: while the list is open it consumes the
// navigation and accept keys, and every other key falls through to the editor
// (which then re-filters the list through maybeAutoCompletion). Because the list
// is drawn by the editor (not an overlay), this hook actually fires. The editor
// is (re)focused on every intercepted key so that even if the focus was lost to
// a refresh in the meantime, the next arrow keeps working.
func (u *ui) completionKey(name fyne.KeyName) bool {
	if !u.completionOpen() {
		return false
	}
	switch name {
	case fyne.KeyEscape:
		u.closeCompletion()
		return true
	case fyne.KeyDown:
		u.win.Canvas().Focus(u.ed)
		u.complete.sel = (u.complete.sel + 1) % len(u.complete.items)
		u.ed.Refresh()
		return true
	case fyne.KeyUp:
		u.win.Canvas().Focus(u.ed)
		u.complete.sel--
		if u.complete.sel < 0 {
			u.complete.sel = len(u.complete.items) - 1
		}
		u.ed.Refresh()
		return true
	case fyne.KeyReturn, fyne.KeyEnter, fyne.KeyTab:
		u.win.Canvas().Focus(u.ed)
		u.acceptCompletion()
		return true
	}
	return false
}

// acceptCompletion inserts the selected suggestion, replacing the range the
// engine asked for. Variable suggestions get their closing braces appended
// unless the line already has them.
func (u *ui) acceptCompletion() {
	if !u.completionOpen() {
		return
	}
	sel := u.complete.sel
	if sel < 0 || sel >= len(u.complete.items) {
		sel = 0
	}
	item := u.complete.items[sel]

	row, _ := u.ed.Caret()
	text := item.Snippet
	if item.Var {
		line := u.ed.LineAt(row)
		if !strings.HasPrefix(runeSlice(line, item.End, runeLen(line)), "}}") {
			text += "}}"
		}
	}
	u.closeCompletion()
	u.noAutoComplete = true
	u.ed.ReplaceRange(row, item.Start, item.End, text)
	u.noAutoComplete = false
	u.setStatus(i18n.T("completion.accepted"))
}

// acceptRow accepts the suggestion under a mouse click inside the list.
func (u *ui) acceptCompletionRow(row int) {
	if !u.completionOpen() || row < 0 || row >= len(u.complete.items) {
		return
	}
	u.complete.sel = row
	u.acceptCompletion()
}

// maybeAutoCompletion keeps the list in step with typing. An open list is
// re-filtered; otherwise the shared engine decides whether the caret sits in a
// context with candidates (a method or URL prefix, a header name, a "{{"
// variable) and the list opens by itself, exactly as in the console. Typing
// "{{" always opens it, even before the engine sees a prefix.
func (u *ui) maybeAutoCompletion() {
	if u.noAutoComplete {
		return
	}
	if u.completionOpen() {
		items := u.suggestionsForCaret()
		if len(items) == 0 {
			u.closeCompletion()
			return
		}
		u.complete.items = items
		if u.complete.sel >= len(items) {
			u.complete.sel = 0
		}
		u.showCompletion()
		return
	}
	row, col := u.ed.Caret()
	line := u.ed.LineAt(row)
	if strings.HasSuffix(runeSlice(line, 0, col), "{{") {
		u.complete.sel = 0
		u.openCompletion()
		return
	}
	if len(u.suggestionsForCaret()) > 0 {
		u.complete.sel = 0
		u.openCompletion()
	}
}

package gui

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/widget"
)

// searchField is the top-row search entry. It intercepts the keys that move
// between matches (Up/Down and Shift+Tab) and Escape, and passes everything else
// to the regular entry, so typing and editing keep working.
type searchField struct {
	widget.Entry
	onKey func(name fyne.KeyName) bool
}

func newSearchField() *searchField {
	f := &searchField{}
	f.ExtendBaseWidget(f)
	return f
}

// TypedKey implements fyne.TypedKey.
func (f *searchField) TypedKey(ev *fyne.KeyEvent) {
	if f.onKey != nil && f.onKey(ev.Name) {
		return
	}
	f.Entry.TypedKey(ev)
}

// findEditorFor returns the editor a search targets. The response pane is a
// read-only Editor, so the same findState serves both panes.
func (u *ui) findEditorFor(p pane) *Editor {
	if p == paneResp {
		return u.resp
	}
	return u.ed
}

// findEditor returns the editor the open (or next) search targets.
func (u *ui) findEditor() *Editor {
	if u.searching {
		return u.findEditorFor(u.searchPane)
	}
	return u.findEditorFor(u.active)
}

// searchKey handles the keys the search field intercepts. It reports whether the
// key was consumed.
func (u *ui) searchKey(name fyne.KeyName) bool {
	if !u.searching {
		return false
	}
	shift := keyModifiersFn()&fyne.KeyModifierShift != 0
	switch name {
	case fyne.KeyEscape:
		u.closeSearch()
		return true
	case fyne.KeyUp:
		u.prevMatch()
		return true
	case fyne.KeyDown:
		u.nextMatch()
		return true
	case fyne.KeyTab:
		if shift {
			u.prevMatch()
			return true
		}
	}
	return false
}

// nextMatch moves to the next match of the target pane.
func (u *ui) nextMatch() {
	u.findEditor().FindNext()
	u.updateSearchCount()
}

// prevMatch moves to the previous match of the target pane.
func (u *ui) prevMatch() {
	u.findEditor().FindPrev()
	u.updateSearchCount()
}

// retargetSearch moves an open search onto another pane. The editor is not a
// searchable pane (typing edits it), so activating it closes the search; the
// response pane takes the query over and keeps the same matches model.
func (u *ui) retargetSearch(p pane) {
	if !u.searching {
		return
	}
	if p != paneResp {
		u.closeSearch()
		return
	}
	if u.searchPane == paneResp {
		return
	}
	u.ed.ClearFind()
	u.searchPane = paneResp
	u.resp.SetFindQuery(u.searchEntry.Text)
	u.updateSearchCount()
}

// cyclePane moves the active pane and the keyboard focus one step through
// files → editor → response (backwards with Shift).
func (u *ui) cyclePane(back bool) bool {
	order := []pane{paneFiles, paneEditor, paneResp}
	idx := 0
	for i, p := range order {
		if p == u.active {
			idx = i
			break
		}
	}
	step := 1
	if back {
		step = -1
	}
	next := order[(idx+step+len(order))%len(order)]
	u.focusPane(next)
	return true
}

// focusPane makes a pane active and gives its widget the keyboard focus.
func (u *ui) focusPane(p pane) {
	u.setActive(p)
	var target fyne.Focusable
	switch p {
	case paneFiles:
		if u.files != nil && u.files.view != nil {
			target = u.files.view
		}
	case paneEditor:
		target = u.ed
	case paneResp:
		target = u.resp
	}
	if target != nil {
		u.win.Canvas().Focus(target)
	}
}

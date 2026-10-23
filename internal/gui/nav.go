package gui

import (
	"image/color"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"

	"github.com/user/curlyk/internal/httpfile"
	"github.com/user/curlyk/internal/i18n"
)

// navPopupSize is the minimum size of the "go to request" popup. It is generous
// enough to show a full request label and several rows; a plain Resize on the
// popup is ignored (it sizes itself from the content's MinSize), so the content
// gets an invisible background with this MinSize.
var navPopupSize = fyne.NewSize(560, 380)

// navEntry is one request in the Ctrl+G popup: what the row shows, what the
// filter matches against (lowercased) and the 1-based source line to jump to.
type navEntry struct {
	label  string
	search string
	line   int
}

// navFilter is the popup's filter field. It intercepts the keys that move the
// selection (Up/Down), accept it (Enter) and close the popup (Escape), so the
// caret stays in the input while the list is navigated.
type navFilter struct {
	widget.Entry
	onKey func(name fyne.KeyName) bool
}

func newNavFilter() *navFilter {
	f := &navFilter{}
	f.ExtendBaseWidget(f)
	return f
}

// TypedKey implements fyne.TypedKey.
func (f *navFilter) TypedKey(ev *fyne.KeyEvent) {
	if f.onKey != nil && f.onKey(ev.Name) {
		return
	}
	f.Entry.TypedKey(ev)
}

// navPopup is the "go to request" overlay opened by Ctrl+G.
type navPopup struct {
	u       *ui
	popup   *widget.PopUp
	title   *widget.Label
	filter  *navFilter
	rows    *fyne.Container
	entries []navEntry
	visible []int
	sel     int
}

// navEntriesFromRequests maps parsed requests to popup entries, preferring the
// @name, then the "### ..." title, then "METHOD URL".
func navEntriesFromRequests(reqs []httpfile.Request) []navEntry {
	entries := make([]navEntry, 0, len(reqs))
	for _, r := range reqs {
		label := r.Name
		if label == "" {
			label = r.Title
		}
		if label == "" {
			label = r.Method + " " + r.URL
		}
		entries = append(entries, navEntry{
			label:  label,
			search: strings.ToLower(label),
			line:   r.Line,
		})
	}
	return entries
}

// openNav shows the request list for the current buffer and focuses its filter.
func (u *ui) openNav() {
	if u.navOpen() {
		return
	}
	reqs := httpfile.ParseFile(u.ed.Text())
	entries := navEntriesFromRequests(reqs)

	n := &navPopup{u: u, entries: entries}
	n.title = widget.NewLabel("")
	n.filter = newNavFilter()
	n.filter.onKey = n.key
	n.filter.SetPlaceHolder(i18n.T("nav.filterPlaceholder"))
	n.filter.OnChanged = func(string) { n.refresh(true) }
	n.rows = container.NewVBox()

	body := container.NewBorder(
		container.NewVBox(n.title, n.filter),
		nil, nil, nil,
		container.NewVScroll(n.rows),
	)
	// The popup sizes itself from the content's MinSize; an invisible backdrop
	// gives it a usable width, so even a long request label fits.
	bg := canvas.NewRectangle(color.Transparent)
	bg.SetMinSize(navPopupSize)
	content := container.NewStack(bg, body)

	u.nav = n
	n.refresh(true)
	n.popup = widget.NewPopUp(content, u.win.Canvas())
	n.popup.ShowAtPosition(u.ed.caretCanvasPosition())
	u.win.Canvas().Focus(n.filter)
}

// closeNav hides the popup.
func (u *ui) closeNav() {
	if u.nav == nil || u.nav.popup == nil {
		return
	}
	u.nav.popup.Hide()
}

// navOpen reports whether the popup is showing.
func (u *ui) navOpen() bool {
	return u.nav != nil && u.nav.popup != nil && u.nav.popup.Visible()
}

// refresh rebuilds the visible rows for the current filter. reset puts the
// selection back on the first match, which is what typing a new query does.
func (n *navPopup) refresh(reset bool) {
	q := strings.ToLower(n.filter.Text)
	n.visible = n.visible[:0]
	for i, e := range n.entries {
		if q == "" || strings.Contains(e.search, q) {
			n.visible = append(n.visible, i)
		}
	}
	if reset {
		n.sel = 0
	}
	if n.sel >= len(n.visible) {
		n.sel = len(n.visible) - 1
	}
	if n.sel < 0 {
		n.sel = 0
	}

	n.title.SetText(i18n.T("nav.title", n.filter.Text))
	n.rows.RemoveAll()
	switch {
	case len(n.entries) == 0:
		n.rows.Add(widget.NewLabel(i18n.T("nav.noRequests")))
		return
	case len(n.visible) == 0:
		n.rows.Add(widget.NewLabel(i18n.T("nav.noMatches")))
		return
	}
	for pos, idx := range n.visible {
		e := n.entries[idx]
		label := "  " + e.label
		if pos == n.sel {
			label = "▸ " + e.label
		}
		pos := pos
		b := widget.NewButton(label, func() { n.accept(pos) })
		b.Alignment = widget.ButtonAlignLeading
		if pos != n.sel {
			b.Importance = widget.LowImportance
		}
		n.rows.Add(b)
	}
}

// move shifts the selection by delta inside the filtered list.
func (n *navPopup) move(delta int) {
	if len(n.visible) == 0 {
		return
	}
	n.sel = clampInt(n.sel+delta, 0, len(n.visible)-1)
	n.refresh(false)
}

// accept jumps to the entry at position pos of the filtered list.
func (n *navPopup) accept(pos int) {
	if pos < 0 || pos >= len(n.visible) {
		return
	}
	e := n.entries[n.visible[pos]]
	n.u.closeNav()
	n.u.jumpToRequest(e)
}

// key handles the keys the filter field intercepts.
func (n *navPopup) key(name fyne.KeyName) bool {
	switch name {
	case fyne.KeyEscape:
		n.u.closeNav()
		n.u.focusEditor()
		return true
	case fyne.KeyUp:
		n.move(-1)
		return true
	case fyne.KeyDown:
		n.move(1)
		return true
	case fyne.KeyReturn, fyne.KeyEnter:
		n.accept(n.sel)
		return true
	}
	return false
}

// jumpToRequest moves the caret to a request's line, reports it and returns the
// keyboard to the editor.
func (u *ui) jumpToRequest(e navEntry) {
	u.ed.RestoreCaret(e.line-1, 0, 0)
	u.focusEditor()
	u.setStatus(i18n.T("nav.jumped", e.label, e.line))
}

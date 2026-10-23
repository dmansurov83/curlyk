package gui

import (
	"image/color"
	"path/filepath"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/widget"

	"github.com/user/curlyk/internal/httpfile"
	"github.com/user/curlyk/internal/i18n"
	"github.com/user/curlyk/internal/profiles"
)

// filesPanel is the desktop window's left sidebar. It mirrors the console: the
// .http files of the working directory with a "+ Новый файл" entry first, and the
// environment-profile section underneath. Clicking a row opens the file or
// switches the active profile.
//
// The file rows are plain widgets with a background painted by this panel, not a
// widget.List: the list draws its own selection and focus highlight, which
// stacked a second (dark blue) mark on clicked rows and could leave old rows
// looking selected. Owning the paint keeps exactly one row highlighted — the
// open file.
type filesPanel struct {
	u     *ui
	view  *filesPane
	rows  *fyne.Container
	items []*fileRow
	files []string
	box   *fyne.Container // profile rows
	// profileArea is the scrollable profile section, kept for sizing/tests.
	profileArea *container.Scroll
	// selected is the highlighted row (-1 for none); it mirrors the open file.
	selected int
}

// fileRow is one clickable line of the file panel: a background plus a
// monospaced label. The background is the only highlight in the sidebar.
type fileRow struct {
	widget.BaseWidget
	p       *filesPanel
	bg      *canvas.Rectangle
	label   *widget.Label
	active  bool
	hovered bool
	onTap   func()
}

func newFileRow(p *filesPanel, text string, onTap func()) *fileRow {
	r := &fileRow{p: p, onTap: onTap}
	r.ExtendBaseWidget(r)
	r.label = widget.NewLabel(text)
	r.label.TextStyle = fyne.TextStyle{Monospace: true}
	r.bg = canvas.NewRectangle(color.Transparent)
	return r
}

// CreateRenderer implements fyne.Widget.
func (r *fileRow) CreateRenderer() fyne.WidgetRenderer {
	return widget.NewSimpleRenderer(container.NewStack(r.bg, r.label))
}

// Tapped implements fyne.Tappable: the whole row is the hit target.
func (r *fileRow) Tapped(*fyne.PointEvent) {
	if r.onTap != nil {
		r.onTap()
	}
}

// MouseIn implements desktop.Hoverable: hovering paints a quiet background so
// the row reads as clickable.
func (r *fileRow) MouseIn(*desktop.MouseEvent) {
	r.hovered = true
	r.paint()
}

// MouseOut implements desktop.Hoverable.
func (r *fileRow) MouseOut() {
	r.hovered = false
	r.paint()
}

// MouseMoved implements desktop.Hoverable. Fyne's interface requires all three
// methods; without this one the row is not Hoverable at all and MouseIn is never
// dispatched.
func (r *fileRow) MouseMoved(*desktop.MouseEvent) {}

var _ desktop.Hoverable = (*fileRow)(nil)

// setActive paints or clears the row highlight with the current scheme.
func (r *fileRow) setActive(on bool) {
	r.active = on
	r.paint()
}

// paint draws the row background: the active (open) file wins over the hover
// tint, so hovering the open file never hides which file is open.
func (r *fileRow) paint() {
	var col color.Color = color.Transparent
	switch {
	case r.active:
		col = r.p.u.pal.menuSelBg
	case r.hovered:
		col = r.p.u.pal.menuBg
	}
	if r.bg.FillColor != col {
		r.bg.FillColor = col
		r.bg.Refresh()
	}
}

// setText updates the row label (the open file gets the "▸" marker).
func (r *fileRow) setText(text string) { r.label.SetText(text) }

// filesPane makes the sidebar focusable so Tab can enter it and leave it, while
// still showing the plain row list rather than a selectable widget.List.
type filesPane struct {
	widget.BaseWidget
	p      *filesPanel
	scroll *container.Scroll
}

func newFilesPane(p *filesPanel) *filesPane {
	v := &filesPane{p: p}
	v.scroll = container.NewVScroll(p.rows)
	v.ExtendBaseWidget(v)
	return v
}

// CreateRenderer implements fyne.Widget.
func (v *filesPane) CreateRenderer() fyne.WidgetRenderer {
	return widget.NewSimpleRenderer(v.scroll)
}

// Focusable implements fyne.Focusable.
func (v *filesPane) Focusable() bool { return true }

// FocusGained implements fyne.Focusable.
func (v *filesPane) FocusGained() {}

// FocusLost implements fyne.Focusable.
func (v *filesPane) FocusLost() {}

// AcceptsTab implements fyne.Tabbable so the window's pane cycle owns Tab.
func (v *filesPane) AcceptsTab() bool { return true }

// TypedRune implements fyne.Focusable.
func (v *filesPane) TypedRune(rune) {}

// TypedKey implements fyne.TypedKey.
func (v *filesPane) TypedKey(ev *fyne.KeyEvent) {
	if ev.Name == fyne.KeyTab || ev.Name == fyne.KeyF1 {
		v.p.u.windowKey(ev.Name)
	}
}

// newFilesPanel builds the sidebar onto the window's panel and loads the current
// directory listing. It is called once, from newUI: the panel keeps its widget
// tree so window rebuilds (a changed pane frame, a language switch) reuse it.
func (u *ui) newFilesPanel() fyne.CanvasObject {
	p := u.files
	if p == nil {
		p = &filesPanel{u: u, selected: -1}
		u.files = p
	}
	p.rows = container.NewVBox()
	p.view = newFilesPane(p)

	// The profile section keeps its own scroll area, so the file list above it
	// always keeps the remaining height. It is taller than the console's minimum
	// because the desktop rows are larger, and a separator makes the two
	// sections read as separate lists.
	p.box = container.NewVBox()
	p.profileArea = container.NewVScroll(p.box)
	p.profileArea.SetMinSize(fyne.NewSize(160, 220))
	bottom := container.NewVBox(widget.NewSeparator(), p.profileArea)

	// Load the listing now: the panel must show the working directory's files and
	// profiles as soon as the window opens, not only after a file action.
	p.reload()

	return container.NewBorder(nil, bottom, nil, nil, p.view)
}

// rowLabel is the text of one sidebar row: the "+ Новый файл" entry first, then
// the files, with the open one marked the way the console marks a selection.
func (p *filesPanel) rowLabel(id widget.ListItemID) string {
	if id <= 0 {
		return i18n.T("file.new")
	}
	if id-1 >= len(p.files) {
		return ""
	}
	name := p.files[id-1]
	if name == filepath.Base(p.u.path) {
		return "▸ " + name
	}
	return "  " + name
}

// rebuildRows recreates the row widgets for the current file list.
func (p *filesPanel) rebuildRows() {
	p.rows.RemoveAll()
	p.items = p.items[:0]
	for id := 0; id <= len(p.files); id++ {
		id := id
		row := newFileRow(p, p.rowLabel(widget.ListItemID(id)), func() { p.clickRow(id) })
		row.setActive(id == p.selected)
		p.items = append(p.items, row)
		p.rows.Add(row)
	}
	p.rows.Refresh()
}

// paintRows repaints every row from p.selected.
func (p *filesPanel) paintRows() {
	for id, row := range p.items {
		row.setActive(id == p.selected)
		row.setText(p.rowLabel(widget.ListItemID(id)))
	}
	p.rows.Refresh()
}

// clickRow handles a click on a sidebar row: highlight it, then open the file
// (row 0 starts a new buffer).
func (p *filesPanel) clickRow(id int) {
	p.selected = id
	p.paintRows()
	p.u.setActive(paneFiles)
	if id <= 0 {
		p.u.newFile()
		return
	}
	p.u.openNamed(p.files[id-1])
}

// selectPath highlights the row of the open file and clears every other row.
// An empty path (a new buffer or a profile) leaves the list unselected.
func (p *filesPanel) selectPath(path string) {
	p.selected = -1
	if path != "" {
		base := filepath.Base(path)
		for i, name := range p.files {
			if name == base {
				p.selected = i + 1 // row 0 is "+ Новый файл"
				break
			}
		}
	}
	p.paintRows()
}

// reload rescans the working directory for files and profiles.
func (p *filesPanel) reload() {
	p.files = httpfile.FilesInDir(".")
	p.rebuildRows()
	p.reloadProfiles()
}

// reloadProfiles rebuilds the profile section: a title row and one row per
// profile, the active one marked with the badge the TUI uses.
func (p *filesPanel) reloadProfiles() {
	if p.box == nil {
		return
	}
	p.box.RemoveAll()

	title := widget.NewLabel(i18n.T("profile.title"))
	title.TextStyle = fyne.TextStyle{Bold: true}
	p.box.Add(title)

	names := profiles.List(".")
	if len(names) == 0 {
		hint := widget.NewLabel(i18n.T("profile.noneConfigured"))
		hint.TextStyle = fyne.TextStyle{Monospace: true}
		p.box.Add(hint)
		p.box.Refresh()
		return
	}
	for _, name := range names {
		file := name
		active := file == p.u.settings.ActiveProfile
		label := "  " + profiles.DisplayName(file)
		if active {
			label = "▸ " + profiles.DisplayName(file) + " " + i18n.T("profile.activeBadge")
		}
		btn := widget.NewButton(label, func() { p.u.onProfileChanged(profiles.DisplayName(file)) })
		btn.Alignment = widget.ButtonAlignLeading
		if !active {
			btn.Importance = widget.LowImportance
		}
		p.box.Add(btn)
	}
	// A "+ Новый профиль" action row always ends the section, so the window can
	// create a profile the way the console's profile picker can.
	btn := widget.NewButton(i18n.T("profile.new"), func() { p.u.askNewProfile() })
	btn.Alignment = widget.ButtonAlignLeading
	btn.Importance = widget.LowImportance
	p.box.Add(btn)
	p.box.Refresh()
}

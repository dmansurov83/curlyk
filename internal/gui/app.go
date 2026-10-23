package gui

import (
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/storage"
	"fyne.io/fyne/v2/widget"

	"github.com/user/curlyk/internal/i18n"
	"github.com/user/curlyk/internal/profiles"
	"github.com/user/curlyk/internal/settings"
	"github.com/user/curlyk/internal/theme"
)

// sampleDocument is shown when there is no file to open.
const sampleDocument = `### Список пользователей
# комментарий в стиле .http
GET https://api.example.com/v1/users?limit=20 HTTP/1.1
Accept: application/json
Authorization: Bearer {{token}}

### Создать пользователя
@name create-user
@timeout 5s
POST https://api.example.com/v1/users
Content-Type: application/json

{
  "name": "Ada",
  "active": true
}
`

// pane identifies which of the three panes is active, so its frame can be marked
// the way the TUI colours the active pane border.
type pane int

const (
	paneFiles pane = iota
	paneEditor
	paneResp
)

// ui is the whole desktop window. It mirrors the console layout: a header line
// with the file and the active profile, three panes (files | editor | response),
// and a status line. Everything it shows comes from the shared packages, so the
// window and the TUI open the same files, run the same requests and share
// appsettings.yml.
type ui struct {
	app fyne.App
	win fyne.Window

	ed   *Editor
	resp *Editor
	// respHdr renders the fixed header part of a reply (request dump, status,
	// timing, response headers) above the copy button; resp holds the body.
	respHdr          *Editor
	respHeaderLayout *fixedHeight
	respHeaderCanvas *fyne.Container
	copyButton       *widget.Button
	// respTop is the response header stack (title, header viewer, copy button),
	// kept so hiding the button re-lays it out.
	respTop *fyne.Container
	// respBody stacks the response editor with the activity spinner.
	respBody *fyne.Container
	spinner  *spinner
	files    *filesPanel
	// filesView is the sidebar widget tree, built once and reused by every
	// window rebuild.
	filesView fyne.CanvasObject

	// lastBody is the raw body of the most recent response, kept so the pane's
	// "copy body" button can put it on the clipboard independently of the
	// formatted text shown.
	lastBody []byte

	// respText/respHeaderRows keep the formatted reply so F1 can put the help
	// over it and restore it afterwards. helpVisible is the empty state (no
	// response yet) or the F1 toggle.
	respText       string
	respHeaderRows int
	helpVisible    bool

	header *strip
	status *strip

	searchEntry *searchField
	searchCount *widget.Label
	searching   bool
	// searchPane is the pane the open find targets (editor or response).
	searchPane pane

	complete *completionPopup
	menu     *requestMenu
	nav      *navPopup

	settings settings.Settings
	pal      *palette

	path      string
	saved     string
	statusMsg string
	active    pane
	running   bool
	// caret tracks the caret position of every file touched this session, so
	// switching files and closing the window keeps each one's place.
	caret map[string]settings.CursorPos
	// updating suppresses the profile list's callbacks while it is being
	// refreshed programmatically; building guards refreshContent against
	// re-entrancy; restoring suppresses caret recording while a file loads.
	updating  bool
	building  bool
	restoring bool
	// autosaveDeadline/autosaveTimer implement the debounced save after an edit.
	autosaveDeadline time.Time
	autosaveTimer    *time.Timer
	// noAutoComplete suppresses the automatic suggestion list while the window
	// itself rewrites the buffer (accepted suggestion, formatted JSON), so the
	// list does not pop back up over the change.
	noAutoComplete bool
}

// Run starts the desktop UI and blocks until the window is closed.
func Run() error {
	u := newUI(app.New())

	u.win.SetCloseIntercept(func() {
		u.persistSession()
		u.win.Close()
	})

	u.win.Show()
	u.win.Canvas().Focus(u.ed)
	u.app.Run()
	return nil
}

// newUI builds the window and every widget. Keeping construction separate from
// the event loop lets tests drive the whole window without running an app.
func newUI(a fyne.App) *ui {
	s := settings.Load()
	i18n.SetLocale(i18n.Resolve(s.Lang))

	u := &ui{
		settings: s,
		pal:      loadPalette(s.Theme),
		path:     s.LastOpenedFile,
		caret:    map[string]settings.CursorPos{},
		active:   paneEditor,
	}

	u.app = a
	u.win = a.NewWindow(i18n.T("gui.title", i18n.T("file.unnamed")))
	u.win.Resize(fyne.NewSize(1200, 780))

	u.header = newStrip(u.pal.headerBg, u.pal.headerFg, 14)
	u.status = newStrip(u.pal.statusBg, u.pal.statusFg, 14)

	u.ed = NewEditor(u.initialText(), u.pal)
	u.resp = NewEditor(i18n.T("gui.noResponse"), u.pal)
	u.resp.SetReadOnly(true)
	u.resp.SetShowLineNumbers(false)
	u.spinner = newSpinner(u.pal.borderActive)
	u.spinner.Hide()
	u.respBody = container.NewStack(u.resp, container.NewCenter(u.spinner))
	u.respHdr = NewEditor("", u.pal)
	u.respHdr.SetReadOnly(true)
	u.respHdr.SetShowLineNumbers(false)
	u.respHeaderLayout = &fixedHeight{}
	u.respHeaderCanvas = container.New(u.respHeaderLayout, u.respHdr)
	u.copyButton = widget.NewButton(i18n.T("gui.copyResponse"), u.copyResponse)
	u.copyButton.Alignment = widget.ButtonAlignLeading
	u.copyButton.Importance = widget.LowImportance
	// The response pane is a viewer, but the window's own shortcuts (Escape to
	// close an overlay, Ctrl+S to save the file under edit) still apply when it
	// has focus.
	// Ctrl+C with no selection in the response pane copies the whole reply: the
	// pane is read-only, so there is no "copy what is selected" ambiguity there.
	u.resp.SetShortcutHandler(func(s fyne.Shortcut) bool {
		if _, ok := s.(*fyne.ShortcutCopy); ok && !u.resp.HasSelection() {
			u.copyResponse()
			return true
		}
		return u.handleShortcut(s)
	})
	u.resp.SetKeyHook(u.windowKey)
	u.resp.SetOnFocus(func() { u.setActive(paneResp) })
	u.resp.SetSecondaryHook(func(_ int, _ fyne.Position) {
		if u.resp.HasSelection() {
			n := u.resp.SelectionLen()
			if u.resp.Copy() {
				u.setStatus(i18n.T("status.copiedResp", n))
			}
			return
		}
		u.copyResponse()
	})
	u.resp.SetRuneHook(func(r rune) bool {
		if u.helpShown() {
			return false
		}
		u.openResponseSearch(string(r))
		return true
	})
	u.complete = &completionPopup{u: u}
	// The list is painted by the editor's renderer, so the editor must know about
	// it (and keeps the keyboard focus while the list is up).
	u.ed.complete = u.complete

	u.ed.SetOnChanged(func() {
		u.updateHeader()
		u.maybeAutoCompletion()
		u.scheduleAutosave()
	})
	// The editor scrolls itself, so a caret move only refreshes the header and
	// records the position to persist.
	u.ed.SetOnCaretMoved(func(float32) {
		u.updateHeader()
		u.rememberCaret()
	})
	u.ed.SetKeyHook(u.windowKey)
	u.ed.SetShortcutHandler(u.handleShortcut)
	u.ed.SetSecondaryHook(u.openRequestMenu)
	u.ed.SetPasteHandler(u.convertPaste)
	u.ed.SetRunIconHandler(func(row int) { u.openRequestMenu(row, u.ed.rowCanvasPosition(row)) })
	u.ed.SetOnFocus(func() { u.setActive(paneEditor) })

	u.buildSearchRow()
	u.ensureProfiles()
	u.filesView = u.newFilesPanel()
	u.files.selectPath(u.path)
	u.win.SetContent(u.buildContent())
	u.registerShortcuts()
	u.restoreCaret()
	u.updateHeader()
	u.setStatus(i18n.T("gui.ready"))
	// The right pane starts with the key reference; the first reply replaces it.
	u.toggleHelp()

	return u
}

// buildContent assembles the window: the header line, the three panes and the
// status line, exactly the row order the console uses.
func (u *ui) buildContent() fyne.CanvasObject {
	return container.NewBorder(u.topRow(), u.status, nil, nil, u.panes())
}

// topRow is the header line, or the search bar while a search is open — the TUI
// draws the search on the same top row, so nothing below it moves.
func (u *ui) topRow() fyne.CanvasObject {
	if u.searching {
		return container.NewStack(
			canvas.NewRectangle(u.pal.headerBg),
			container.NewBorder(nil, nil, nil,
				container.NewHBox(u.searchCount, widget.NewButton("×", u.closeSearch)),
				u.searchEntry,
			),
		)
	}
	return u.header
}

// panes builds the three panes: files, editor and response, each in a frame that
// marks the active one.
func (u *ui) panes() fyne.CanvasObject {
	files := u.framedPane(paneFiles, u.filesView)
	editor := u.framedPane(paneEditor, u.ed)
	resp := u.framedPane(paneResp, u.responsePane())

	inner := container.NewHSplit(editor, resp)
	inner.SetOffset(0.64)
	outer := container.NewHSplit(files, inner)
	outer.SetOffset(0.18)
	return outer
}

// framedPane wraps a pane in a 2px border: the active pane gets the accent
// colour, the way the console marks the focused pane.
func (u *ui) framedPane(p pane, content fyne.CanvasObject) fyne.CanvasObject {
	col := u.pal.borderIdle
	if u.active == p {
		col = u.pal.borderActive
	}
	return newPanelFrame(col, content)
}

// responsePane builds the response pane in the console's order: the fixed header
// part of the reply, the copy button and then the scrollable body. There is no
// title row above it — the pane is identified by its content.
func (u *ui) responsePane() fyne.CanvasObject {
	top := container.NewVBox(u.respHeaderCanvas, u.copyButton)
	u.respTop = top
	return container.NewBorder(top, nil, nil, nil, u.respBody)
}

// refreshContent rebuilds the window content after a widget was added or removed
// (a changed pane frame, the search row, a profile click), keeping the keyboard
// focus on the editor. The guard stops the rebuild from re-entering itself
// through the widget callbacks it triggers.
func (u *ui) refreshContent() {
	if u.building {
		return
	}
	u.building = true
	u.win.SetContent(u.buildContent())
	// Only the active pane takes the keyboard: focusing the editor after every
	// rebuild would immediately steal the active mark from the files pane.
	if !u.searching && u.active == paneEditor {
		u.win.Canvas().Focus(u.ed)
	}
	u.building = false
}

// setActive marks a pane active and repaints the frames when that changed. An
// open search follows the activated pane: the response takes it over, the editor
// and the file list close it (the editor accepts text directly).
func (u *ui) setActive(p pane) {
	if u.active == p {
		return
	}
	u.active = p
	u.retargetSearch(p)
	u.refreshContent()
}

// updateHeader refreshes the header line: the file name with the dirty marker,
// the active profile and the caret position, in the console's wording.
func (u *ui) updateHeader() {
	name := i18n.T("file.unnamed")
	if u.path != "" {
		name = filepath.Base(u.path)
	}
	if u.ed.Text() != u.saved {
		name += " " + i18n.T("dirty.marker")
	}

	text := i18n.T("header.file", name)
	if u.settings.ActiveProfile != "" {
		text += i18n.T("header.profile", profiles.DisplayName(u.settings.ActiveProfile))
	}
	row, col := u.ed.Caret()
	text += " — " + i18n.T("gui.caret", row+1, col+1, u.ed.LineCount())
	if n := u.ed.SelectionLen(); n > 0 {
		text += " · " + i18n.T("gui.selection", n)
	}

	u.header.SetText(text)
	u.win.SetTitle(i18n.T("gui.title", name))
}

// setStatus shows a message on the status line, the way the console reports what
// just happened.
func (u *ui) setStatus(msg string) {
	u.statusMsg = msg
	u.status.SetText(" " + msg)
}

// StatusMessage returns what the status line currently shows (tests).
func (u *ui) StatusMessage() string { return u.statusMsg }

// buildSearchRow creates the search widgets used on the top row.
func (u *ui) buildSearchRow() {
	u.searchEntry = newSearchField()
	u.searchEntry.onKey = u.searchKey
	u.searchEntry.SetPlaceHolder(i18n.T("gui.findPlaceholder"))
	u.searchEntry.OnChanged = func(q string) {
		u.findEditor().SetFindQuery(q)
		u.updateSearchCount()
	}
	u.searchEntry.OnSubmitted = func(string) {
		u.nextMatch()
	}
	u.searchCount = widget.NewLabel("")
}

// openResponseSearch opens the top-row search targeted at the response pane and
// seeds it with the first typed character.
func (u *ui) openResponseSearch(initial string) {
	if !u.searching || u.searchPane != paneResp {
		u.searching = true
		u.searchPane = paneResp
		u.refreshContent()
		u.win.Canvas().Focus(u.searchEntry)
	}
	u.searchEntry.SetText(u.searchEntry.Text + initial)
}

// toggleSearch opens or closes the top-row search for the active pane.
func (u *ui) toggleSearch() {
	if u.searching {
		u.closeSearch()
		return
	}
	if u.active == paneResp && u.helpShown() {
		// The key reference is not a reply: there is nothing to search there.
		u.setStatus(i18n.T("err.noResponse"))
		return
	}
	u.searching = true
	u.searchPane = paneEditor
	if u.active == paneResp {
		u.searchPane = paneResp
	}
	u.refreshContent()

	target := u.findEditor()
	if q := target.FindQuery(); q != "" {
		u.searchEntry.SetText(q)
		target.SetFindQuery(q)
	} else {
		u.searchEntry.SetText("")
	}
	u.updateSearchCount()
	u.win.Canvas().Focus(u.searchEntry)
}

// closeSearch closes the search row and drops the highlights of its pane.
func (u *ui) closeSearch() {
	if !u.searching {
		return
	}
	target := u.findEditor()
	u.searching = false
	target.ClearFind()
	u.searchEntry.SetText("")
	u.searchPane = paneEditor
	u.updateSearchCount()
	u.refreshContent()
}

// updateSearchCount refreshes the "current/total" counter on the search row.
func (u *ui) updateSearchCount() {
	if u.searchCount == nil {
		return
	}
	target := u.findEditor()
	u.searchCount.SetText(i18n.T("gui.findCount", target.FindCurrent(), target.FindMatches()))
}

// escape closes the transient overlays, most recent first.
func (u *ui) escape() {
	if u.navOpen() {
		u.closeNav()
		u.focusEditor()
		return
	}
	if u.requestMenuOpen() {
		u.closeRequestMenu()
		return
	}
	if u.completionOpen() {
		u.closeCompletion()
		return
	}
	if u.searching {
		u.closeSearch()
	}
}

// windowKey is the editor's key hook: it lets the overlays and the window consume
// keys the editor would otherwise swallow (Escape, Tab) and lets the completion
// popup take the navigation keys while the editor keeps focus.
func (u *ui) windowKey(name fyne.KeyName) bool {
	if u.completionKey(name) {
		return true
	}
	if name == fyne.KeyF1 {
		// Fyne delivers a bare F1 as a key, not as a shortcut (custom shortcuts
		// need a modifier), so it must be handled here.
		u.toggleHelp()
		return true
	}
	if name == fyne.KeyEscape {
		if u.requestMenuOpen() {
			u.closeRequestMenu()
			return true
		}
		if u.navOpen() {
			u.closeNav()
			u.focusEditor()
			return true
		}
		if u.searching {
			u.closeSearch()
			return true
		}
	}
	if u.searching {
		// The search field owns the keyboard while it is open; Tab must stay with
		// it so the user can leave the input without the search closing.
		return false
	}
	if name == fyne.KeyTab {
		return u.cyclePane(keyModifiersFn()&fyne.KeyModifierShift != 0)
	}
	return false
}

// handleShortcut runs the window's own keyboard shortcuts. Fyne delivers custom
// shortcuts to the focused widget, so while the editor has focus this is the
// path that actually fires (the canvas registrations cover the other widgets);
// without it Ctrl+S, Ctrl+O, Ctrl+Enter and friends would silently do nothing.
func (u *ui) handleShortcut(s fyne.Shortcut) bool {
	sc, ok := s.(*desktop.CustomShortcut)
	if !ok {
		return false
	}
	if sc.KeyName == fyne.KeyF1 {
		u.toggleHelp()
		return true
	}
	if sc.Modifier&(fyne.KeyModifierControl|fyne.KeyModifierSuper) == 0 {
		return false
	}
	shift := sc.Modifier&fyne.KeyModifierShift != 0

	key := normalizeShortcutKey(sc.KeyName)
	// Fyne turns Ctrl+Arrow (and Ctrl+Shift+Arrow) into a custom shortcut, not
	// into TypedKey, so word navigation must be handled here or it never fires.
	if isNavigationKey(key) {
		u.moveCaretShortcut(key, shift)
		return true
	}
	if key == fyne.KeyZ && shift {
		u.ed.Redo()
		return true
	}

	switch key {
	case fyne.KeyN:
		u.newFile()
	case fyne.KeyO:
		u.openFile()
	case fyne.KeyS:
		u.saveFile()
	case fyne.KeyT:
		u.cycleTheme()
	case fyne.KeyK:
		u.copyAsCurl()
	case fyne.KeyG:
		u.openNav()
	case fyne.KeyL:
		u.toggleLang()
	case fyne.KeyR:
		u.runRequest()
	case fyne.KeyY:
		u.ed.Redo()
	case fyne.KeyReturn, fyne.KeyEnter:
		u.runRequest()
	case fyne.KeySpace:
		u.complete.sel = 0
		u.openCompletion()
	case fyne.KeyF:
		if shift {
			u.ed.FindPrev()
			u.updateSearchCount()
		} else {
			u.toggleSearch()
		}
	default:
		return false
	}
	return true
}

// isNavigationKey reports whether a key is a caret-movement key that Fyne may
// deliver as a modifier shortcut.
func isNavigationKey(k fyne.KeyName) bool {
	switch k {
	case fyne.KeyLeft, fyne.KeyRight, fyne.KeyUp, fyne.KeyDown,
		fyne.KeyHome, fyne.KeyEnd, fyne.KeyPageUp, fyne.KeyPageDown:
		return true
	}
	return false
}

// moveCaretShortcut applies a Ctrl+Arrow (with optional Shift) to whichever pane
// has the keyboard: the editor, or the read-only response body.
func (u *ui) moveCaretShortcut(name fyne.KeyName, shift bool) {
	target := u.ed
	if focused := u.win.Canvas().Focused(); focused == u.resp {
		target = u.resp
	}
	target.MoveCaretKey(name, shift, true)
}

// shortcutKeyAliases maps the command keys onto the same physical keys of a
// Cyrillic (ЙЦУКЕН) layout. Fyne reports the key name as it is typed, so with a
// Russian layout Ctrl+S can arrive as Ctrl+Ы; without these aliases the desktop
// window's shortcuts would stop working for Russian users.
var shortcutKeyAliases = map[fyne.KeyName]fyne.KeyName{
	"Ы": fyne.KeyS, "ы": fyne.KeyS,
	"Щ": fyne.KeyO, "щ": fyne.KeyO,
	"Т": fyne.KeyN, "т": fyne.KeyN,
	"Е": fyne.KeyT, "е": fyne.KeyT,
	"А": fyne.KeyF, "а": fyne.KeyF,
	"Л": fyne.KeyK, "л": fyne.KeyK,
	"П": fyne.KeyG, "п": fyne.KeyG,
	"К": fyne.KeyR, "к": fyne.KeyR,
	"Я": fyne.KeyZ, "я": fyne.KeyZ,
	"Д": fyne.KeyL, "д": fyne.KeyL,
}

// normalizeShortcutKey translates a layout-specific key name to the Latin one.
func normalizeShortcutKey(k fyne.KeyName) fyne.KeyName {
	if latin, ok := shortcutKeyAliases[k]; ok {
		return latin
	}
	return k
}

// initialText reads the last opened file, falling back to the sample.
func (u *ui) initialText() string {
	text := sampleDocument
	if u.settings.DefaultFile != "" {
		if data, err := os.ReadFile(u.settings.DefaultFile); err == nil {
			text = normalizeNewlines(string(data))
		}
	}
	if u.path != "" {
		if data, err := os.ReadFile(u.path); err == nil {
			text = normalizeNewlines(string(data))
		} else {
			u.path = ""
		}
	}
	u.saved = text
	return text
}

// newFile starts an empty buffer.
func (u *ui) newFile() {
	u.rememberCaret()
	u.stopAutosave()
	u.clearFileSelection()
	u.ed.SetText("")
	u.path = ""
	u.saved = ""
	u.reloadFiles()
	u.files.selectPath("")
	u.updateHeader()
	u.setStatus(i18n.T("status.newFile"))
	u.focusEditor()
}

// focusEditor makes the editor the active pane and gives it the keyboard, so a
// file opened from the panel can be edited straight away (the click leaves focus
// on the list otherwise).
func (u *ui) focusEditor() {
	u.setActive(paneEditor)
	u.win.Canvas().Focus(u.ed)
}

// openNamed opens a file from the working directory (the file panel's click
// path).
func (u *ui) openNamed(name string) {
	data, err := os.ReadFile(name)
	if err != nil {
		u.setStatus(i18n.T("err.openFile", name, err.Error()))
		return
	}
	u.rememberCaret()
	abs, aerr := filepath.Abs(name)
	if aerr != nil {
		abs = name
	}
	u.loadInto(abs, normalizeNewlines(string(data)))
	u.rememberFile()
	u.reloadFiles()
	u.files.selectPath(abs)
	u.updateHeader()
	u.setStatus(i18n.T("status.opened", name))
	u.focusEditor()
}

// clearFileSelection drops the sidebar's click highlight; the open file stays
// marked by the "▸" in its row label.
func (u *ui) clearFileSelection() {
	if u.files != nil {
		u.files.selectPath("")
	}
}

// reloadFiles rescans the sidebar after the file set changed.
func (u *ui) reloadFiles() {
	if u.files != nil {
		u.files.reload()
	}
}

// dialogDir returns the folder the open/save dialogs start in: the directory of
// the current file, or the working directory for an unnamed buffer.
func (u *ui) dialogDir() fyne.ListableURI {
	dir := ""
	if u.path != "" {
		dir = filepath.Dir(u.path)
	}
	if dir == "" || dir == "." {
		if wd, err := os.Getwd(); err == nil {
			dir = wd
		}
	}
	loc, err := storage.ListerForURI(storage.NewFileURI(dir))
	if err != nil {
		return nil
	}
	return loc
}

// openFile asks for a .http file and loads it into the editor.
func (u *ui) openFile() {
	fd := dialog.NewFileOpen(func(rc fyne.URIReadCloser, err error) {
		if err != nil {
			dialog.ShowError(err, u.win)
			return
		}
		if rc == nil {
			return
		}
		defer rc.Close()

		data, err := io.ReadAll(rc)
		if err != nil {
			dialog.ShowError(err, u.win)
			return
		}
		u.rememberCaret()
		path := rc.URI().Path()
		u.loadInto(path, normalizeNewlines(string(data)))
		u.rememberFile()
		u.reloadFiles()
		u.files.selectPath(path)
		u.updateHeader()
		u.setStatus(i18n.T("status.opened", filepath.Base(path)))
		u.focusEditor()
	}, u.win)
	fd.SetFilter(storage.NewExtensionFileFilter([]string{".http"}))
	if loc := u.dialogDir(); loc != nil {
		fd.SetLocation(loc)
	}
	fd.Show()
}

// saveFile writes the buffer back to its file, asking for a name when the buffer
// has no file yet.
func (u *ui) saveFile() {
	if u.path == "" {
		u.saveFileAs()
		return
	}
	if err := os.WriteFile(u.path, []byte(u.ed.Text()), 0o644); err != nil {
		u.setStatus(i18n.T("err.save", err.Error()))
		return
	}
	u.stopAutosave()
	u.saved = u.ed.Text()
	u.persistSession()
	u.updateHeader()
	u.setStatus(i18n.T("status.saved", u.path))
}

// saveFileAs asks for a destination path and saves there.
func (u *ui) saveFileAs() {
	fd := dialog.NewFileSave(func(wc fyne.URIWriteCloser, err error) {
		if err != nil {
			dialog.ShowError(err, u.win)
			return
		}
		if wc == nil {
			return
		}
		if _, err := wc.Write([]byte(u.ed.Text())); err != nil {
			_ = wc.Close()
			dialog.ShowError(err, u.win)
			return
		}
		if err := wc.Close(); err != nil {
			dialog.ShowError(err, u.win)
			return
		}
		u.rememberCaret()
		u.path = wc.URI().Path()
		u.stopAutosave()
		u.saved = u.ed.Text()
		u.rememberFile()
		u.reloadFiles()
		u.files.selectPath(u.path)
		u.updateHeader()
		u.setStatus(i18n.T("status.saved", u.path))
	}, u.win)
	fd.SetFilter(storage.NewExtensionFileFilter([]string{".http"}))
	if loc := u.dialogDir(); loc != nil {
		fd.SetLocation(loc)
	}
	fd.Show()
}

// cycleTheme steps to the next scheme (builtins, then themes/*.theme) and
// persists the choice.
func (u *ui) cycleTheme() {
	names := schemeNames()
	idx := 0
	for i, n := range names {
		if n == u.pal.name {
			idx = i
			break
		}
	}
	next := names[(idx+1)%len(names)]

	u.pal = loadPalette(next)
	u.ed.SetPalette(u.pal)
	u.resp.SetPalette(u.pal)
	u.respHdr.SetPalette(u.pal)
	u.spinner.SetColor(u.pal.borderActive)
	u.recolour()

	u.settings.Theme = next
	_ = u.settings.Save()
	u.setStatus(i18n.T("status.theme", u.themeLabel(u.pal.name)))
}

// recolour restyles the chrome from the palette after a scheme change.
func (u *ui) recolour() {
	u.header.SetColors(u.pal.headerBg, u.pal.headerFg)
	u.status.SetColors(u.pal.statusBg, u.pal.statusFg)
	u.refreshContent()
}

// toggleLang switches the interface language and rebuilds the chrome, so every
// label follows the new locale.
func (u *ui) toggleLang() {
	i18n.SetLocale(i18n.Toggle())
	u.settings.Lang = i18n.Locale()
	_ = u.settings.Save()

	u.searchEntry.SetPlaceHolder(i18n.T("gui.findPlaceholder"))
	// The sidebar is built once and reused, so it needs an explicit rebuild to
	// pick up the profile title and row wording.
	u.filesView = u.newFilesPanel()
	u.files.selectPath(u.path)
	if u.helpShown() {
		u.showHelp()
	}
	u.refreshContent()
	u.updateHeader()
	u.setStatus(i18n.T("status.lang", i18n.LangName(i18n.Locale())))
}

// setResponse replaces the response pane with a reply: the header part goes into
// the fixed header viewer, the body into the scrollable one. It must run on the
// Fyne main thread; runRequest hands it over with fyne.Do.
func (u *ui) setResponse(text string, headerRows int) {
	u.respText = text
	u.respHeaderRows = headerRows
	u.helpVisible = false
	u.applyResponseText(text, headerRows)
}

// applyResponseText splits a formatted reply at headerRows and loads both parts.
func (u *ui) applyResponseText(text string, headerRows int) {
	lines := strings.Split(text, "\n")
	headerRows = clampInt(headerRows, 0, len(lines))
	head := strings.Join(lines[:headerRows], "\n")
	body := strings.Join(lines[headerRows:], "\n")

	u.respHdr.SetResponseText(head, len(strings.Split(head, "\n")))
	u.resp.SetResponseText(body, 0)
	u.updateResponseLayout(headerRows)
	// Each new response is read from the top.
	u.resp.scrollY = 0
	u.resp.Refresh()
}

// maxResponseHeaderRows caps how much of the pane the fixed header block may
// take; more headers than that scroll inside their own viewer.
const maxResponseHeaderRows = 12

// updateResponseLayout sizes the fixed header viewer to the header rows and
// hides it when there are none (the empty state / help).
func (u *ui) updateResponseLayout(headerRows int) {
	height := float32(0)
	if headerRows > 0 {
		_, lineH := u.respHdr.metrics()
		shown := min(headerRows, maxResponseHeaderRows)
		height = float32(shown)*lineH + 2*paddingY
	}
	u.respHeaderLayout.height = height
	if height == 0 {
		u.respHdr.Hide()
	} else {
		u.respHdr.Show()
	}
	u.respHeaderCanvas.Refresh()
	u.respHeaderCanvas.Resize(u.respHeaderCanvas.Size())
	u.syncCopyButton()
}

// setRunning flips the in-flight state and shows the spinner over the response
// body while the request runs.
func (u *ui) setRunning(on bool) {
	u.running = on
	if u.spinner == nil {
		return
	}
	if on {
		u.spinner.Show()
		u.spinner.Start()
	} else {
		u.spinner.Stop()
		u.spinner.Hide()
	}
	if u.respBody != nil {
		u.respBody.Refresh()
	}
}

// syncCopyButton shows the copy button only when the pane holds a reply. The
// empty state (help) and a body-less placeholder keep it hidden, so there is
// nothing to click before the first request.
func (u *ui) syncCopyButton() {
	if u.copyButton == nil {
		return
	}
	if u.respText != "" && !u.helpVisible {
		u.copyButton.Show()
	} else {
		u.copyButton.Hide()
	}
	if u.respTop != nil {
		u.respTop.Refresh()
	}
}

// fixedHeight lays its only child out at a fixed height and full width; the
// response header viewer uses it, so the copy button always sits just below the
// last header line and above the body.
type fixedHeight struct {
	height float32
}

func (f *fixedHeight) Layout(objs []fyne.CanvasObject, size fyne.Size) {
	if len(objs) == 0 {
		return
	}
	h := f.height
	if h > size.Height {
		h = size.Height
	}
	objs[0].Resize(fyne.NewSize(size.Width, h))
	objs[0].Move(fyne.NewPos(0, 0))
}

func (f *fixedHeight) MinSize([]fyne.CanvasObject) fyne.Size {
	return fyne.NewSize(0, f.height)
}

// registerShortcuts wires the window-level keys. Fyne's predefined clipboard and
// undo shortcuts already follow the platform's command modifier (Cmd on macOS,
// Ctrl elsewhere) and are delivered to the focused widget, so only the custom
// ones are registered here — with the same modifier.
func (u *ui) registerShortcuts() {
	c := u.win.Canvas()
	mod := primaryModifier()

	add := func(key fyne.KeyName, extra fyne.KeyModifier, fn func()) {
		c.AddShortcut(&desktop.CustomShortcut{KeyName: key, Modifier: mod | extra},
			func(fyne.Shortcut) { fn() })
	}

	add(fyne.KeyN, 0, u.newFile)
	add(fyne.KeyO, 0, u.openFile)
	add(fyne.KeyS, 0, u.saveFile)
	add(fyne.KeyT, 0, u.cycleTheme)
	add(fyne.KeyK, 0, u.copyAsCurl)
	add(fyne.KeyG, 0, u.openNav)
	add(fyne.KeyR, 0, u.runRequest)
	add(fyne.KeyY, 0, func() { u.ed.Redo() })
	add(fyne.KeyZ, fyne.KeyModifierShift, func() { u.ed.Redo() })
	add(fyne.KeyL, 0, u.toggleLang)
	add(fyne.KeyF, 0, u.toggleSearch)
	add(fyne.KeyF, fyne.KeyModifierShift, func() {
		u.ed.FindPrev()
		u.updateSearchCount()
	})
	add(fyne.KeyReturn, 0, u.runRequest)
	add(fyne.KeyEnter, 0, u.runRequest)
	add(fyne.KeySpace, 0, func() {
		u.complete.sel = 0
		u.openCompletion()
	})

	c.AddShortcut(&desktop.CustomShortcut{KeyName: fyne.KeyEscape}, func(fyne.Shortcut) { u.escape() })
	c.AddShortcut(&desktop.CustomShortcut{KeyName: fyne.KeyF1}, func(fyne.Shortcut) { u.toggleHelp() })
}

// primaryModifier is the platform's command modifier: Cmd on macOS, Ctrl
// elsewhere.
func primaryModifier() fyne.KeyModifier {
	if runtime.GOOS == "darwin" {
		return fyne.KeyModifierSuper
	}
	return fyne.KeyModifierControl
}

// ensureProfiles creates the default profile when none exists and activates the
// configured one (falling back to the first available), so the sidebar always
// shows a usable environment.
func (u *ui) ensureProfiles() {
	if _, err := profiles.EnsureDefault("."); err != nil {
		// A read-only working directory must not stop the editor.
		_ = err
	}
	u.settings.ActiveProfile = profiles.Pick(profiles.List("."), u.settings.ActiveProfile)
	_ = u.settings.Save()
}

// askNewProfile prompts for a profile name and creates it, the way the console's
// "+ Новый профиль" row opens a name input. The new profile is activated and its
// file opened in the editor so its @var lines can be filled in.
func (u *ui) askNewProfile() {
	entry := widget.NewEntry()
	entry.SetPlaceHolder(i18n.T("placeholder.profileName"))

	form := dialog.NewForm(
		i18n.T("profile.titleDialog"),
		i18n.T("profile.create"),
		i18n.T("profile.cancel"),
		[]*widget.FormItem{widget.NewFormItem(i18n.T("profile.titleDialog"), entry)},
		func(ok bool) {
			if ok {
				u.createProfile(entry.Text)
			}
		},
		u.win,
	)
	form.Resize(fyne.NewSize(360, form.MinSize().Height))
	form.Show()
}

// createProfile creates a profile file from the typed name, refreshes the panel
// and activates it, mirroring the TUI's createProfile.
func (u *ui) createProfile(name string) {
	name = strings.TrimSpace(name)
	if name == "" {
		u.setStatus(i18n.T("profile.noneConfigured"))
		return
	}
	if u.updating {
		return
	}
	file, err := profiles.Create(".", name)
	if err != nil {
		u.setStatus(i18n.T("err.save", err.Error()))
		return
	}
	u.settings.ActiveProfile = file
	_ = u.settings.Save()
	u.reloadFiles()
	u.updateHeader()
	u.setStatus(i18n.T("profile.created", profiles.DisplayName(file)))
	u.openProfileFile(file)
}

// onProfileChanged activates a profile, persists the choice and opens the
// profile file in the editor so its @var lines can be edited in place — the same
// behaviour as the console's profile row.
func (u *ui) onProfileChanged(display string) {
	if u.updating {
		return
	}
	name := profiles.Normalize(display)
	u.settings.ActiveProfile = name
	_ = u.settings.Save()
	if u.files != nil {
		u.files.reloadProfiles()
	}
	u.updateHeader()
	u.setStatus(i18n.T("profile.activated", profiles.DisplayName(name)))
	u.openProfileFile(name)
}

// openProfileFile loads a .profile file into the editor. It is a regular buffer:
// Ctrl+S writes it back, and profileVars reads the saved values on the next run.
func (u *ui) openProfileFile(file string) {
	data, err := os.ReadFile(file)
	if err != nil {
		u.setStatus(i18n.T("err.openFile", file, err.Error()))
		return
	}
	u.rememberCaret()
	abs, aerr := filepath.Abs(file)
	if aerr != nil {
		abs = file
	}
	u.loadInto(abs, normalizeNewlines(string(data)))
	u.rememberFile()
	u.reloadFiles()
	u.files.selectPath(abs)
	u.updateHeader()
	u.setStatus(i18n.T("status.opened", file))
	u.focusEditor()
}

// themeLabel is the localised name of a scheme (custom schemes keep their file
// name).
func (u *ui) themeLabel(name string) string {
	switch name {
	case theme.DefaultName, theme.DarkulaName, theme.LightName:
		return i18n.T("theme." + name)
	}
	return name
}

// rememberFile stores the current file as the one to reopen next time.
func (u *ui) rememberFile() {
	u.settings.LastOpenedFile = u.path
	_ = u.settings.Save()
}

// rememberCaret records the caret of the current file in memory; persistSession
// writes the pending positions to appsettings.yml.
func (u *ui) rememberCaret() {
	if u.path == "" || u.ed == nil || u.restoring {
		return
	}
	row, col := u.ed.Caret()
	u.caret[u.path] = settings.CursorPos{Row: row, Col: col, Scroll: int(u.ed.ScrollY())}
}

// cursorFor returns the remembered caret of a file: this session's value first,
// then the one appsettings.yml holds from an earlier run.
func (u *ui) cursorFor(path string) (settings.CursorPos, bool) {
	if path == "" {
		return settings.CursorPos{}, false
	}
	if pos, ok := u.caret[path]; ok {
		return pos, true
	}
	if u.settings.FileCursors == nil {
		return settings.CursorPos{}, false
	}
	pos, ok := u.settings.FileCursors[path]
	return pos, ok
}

// loadInto replaces the buffer with a file's text and puts the caret back where
// that file was left. Every caret move made while loading is ignored, so the
// reset SetText performs neither wipes the remembered position nor pollutes the
// file being left.
func (u *ui) loadInto(path, text string) {
	// A switched file invalidates the open completion list (the caret and the
	// context are gone), so it is closed here rather than painted over the new
	// buffer.
	u.closeCompletion()
	u.stopAutosave()
	u.clearFileSelection()
	u.restoring = true
	u.path = path
	u.ed.SetText(text)
	u.saved = text

	if pos, ok := u.cursorFor(path); ok {
		u.ed.RestoreCaret(pos.Row, pos.Col, float32(pos.Scroll))
	}
	u.restoring = false
	// Record what the caret actually ended up at, so the file has an entry even
	// when it had none before.
	u.rememberCaret()
}

// restoreCaret reopens the current file where the caret was left.
func (u *ui) restoreCaret() {
	pos, ok := u.cursorFor(u.path)
	if !ok {
		return
	}
	u.restoring = true
	u.ed.RestoreCaret(pos.Row, pos.Col, float32(pos.Scroll))
	u.restoring = false
}

// persistSession writes the pending caret positions and the settings to disk.
func (u *ui) persistSession() {
	u.stopAutosave()
	if u.spinner != nil {
		u.spinner.Stop()
	}
	u.rememberCaret()
	if len(u.caret) > 0 {
		if u.settings.FileCursors == nil {
			u.settings.FileCursors = map[string]settings.CursorPos{}
		}
		for path, pos := range u.caret {
			u.settings.FileCursors[path] = pos
		}
	}
	_ = u.settings.Save()
}

// schemeNames lists the builtin schemes followed by custom theme files, in the
// order the TUI cycles through them.
func schemeNames() []string {
	names := []string{theme.DefaultName, theme.DarkulaName, theme.LightName}
	return append(names, theme.ListFiles()...)
}

func normalizeNewlines(s string) string {
	return strings.ReplaceAll(s, "\r\n", "\n")
}

package gui

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"

	"github.com/user/curlyk/internal/curl"
	"github.com/user/curlyk/internal/httpfile"
	"github.com/user/curlyk/internal/i18n"
)

// requestMenu is the popup opened by right-clicking a request line. It offers
// the same actions as the console's request menu (Enter on a request line in the
// TUI), so the two front ends stay equivalent.
type requestMenu struct {
	u     *ui
	popup *widget.PopUp
}

// openRequestMenu shows the menu for the request containing row, anchored at the
// click position inside the editor.
func (u *ui) openRequestMenu(row int, at fyne.Position) {
	block := u.requestAtRow(row)
	if block == nil {
		u.setStatus(i18n.T("err.noRequestCursor"))
		return
	}

	if u.menu == nil {
		u.menu = &requestMenu{u: u}
	}
	items := []fyne.CanvasObject{
		menuButton(i18n.T("menu.run"), func() { u.closeRequestMenu(); u.runRequest() }),
		menuButton(i18n.T("menu.copyCurl"), func() { u.closeRequestMenu(); u.copyAsCurl() }),
		menuButton(i18n.T("menu.formatJson"), func() { u.closeRequestMenu(); u.formatRequestJSON(row) }),
	}
	box := container.NewVBox(items...)
	box.Resize(box.MinSize())

	u.menu.popup = widget.NewPopUp(box, u.win.Canvas())
	u.menu.popup.ShowAtPosition(u.ed.canvasPosition(at))
}

// closeRequestMenu hides the menu.
func (u *ui) closeRequestMenu() {
	if u.menu != nil && u.menu.popup != nil {
		u.menu.popup.Hide()
	}
}

// requestMenuOpen reports whether the request menu is showing.
func (u *ui) requestMenuOpen() bool {
	return u.menu != nil && u.menu.popup != nil && u.menu.popup.Visible()
}

// menuButton builds one menu row (left aligned, so it reads like a list).
func menuButton(label string, tapped func()) fyne.CanvasObject {
	b := widget.NewButton(label, tapped)
	b.Alignment = widget.ButtonAlignLeading
	b.Importance = widget.LowImportance
	return b
}

// requestAtRow returns the request block containing a buffer row, or nil.
func (u *ui) requestAtRow(row int) *httpfile.Request {
	reqs := httpfile.ParseFile(u.ed.Text())
	return httpfile.GetRequestAtLine(reqs, row+1)
}

// copyAsCurl puts the request under the caret on the clipboard as a cURL
// command, with {{variables}} resolved exactly as a run would resolve them
// (Ctrl+K, like the console).
func (u *ui) copyAsCurl() {
	req := u.requestAtCaret()
	if req == nil {
		u.setStatus(i18n.T("err.noRequest"))
		return
	}
	if err := u.substituteVars(req); err != nil {
		u.setStatus(substitutionMessage(err))
		return
	}
	headers := make([]string, 0, len(req.Headers))
	for _, h := range req.Headers {
		headers = append(headers, h.Name+": "+h.Value)
	}
	cmd := curl.ExportRequest(req.Method, req.URL, headers, req.Body)

	cb := clipboard()
	if cb == nil {
		u.setStatus(i18n.T("err.copyCurl", "clipboard unavailable"))
		return
	}
	cb.SetContent(cmd)
	if got := cb.Content(); got != cmd {
		u.setStatus(i18n.T("err.copyCurl", "clipboard write failed"))
		return
	}
	u.setStatus(i18n.T("status.curlCopied"))
}

// formatRequestJSON pretty-prints the JSON body of the request containing row,
// replacing its lines in the buffer as one undo step.
func (u *ui) formatRequestJSON(row int) {
	req := u.requestAtRow(row)
	if req == nil {
		u.setStatus(i18n.T("err.noBodyFormat"))
		return
	}
	formatted, start, end, err := httpfile.FormatJSONBody(*req)
	switch {
	case err == httpfile.ErrNoJSONBody:
		u.setStatus(i18n.T("err.noBodyFormat"))
		return
	case err == httpfile.ErrNotJSON:
		u.setStatus(i18n.T("err.notJson"))
		return
	case err == httpfile.ErrBodyRange:
		u.setStatus(i18n.T("err.bodyRange"))
		return
	case err != nil:
		u.setStatus(i18n.T("err.formatJson", err.Error()))
		return
	}
	u.noAutoComplete = true
	u.ed.ReplaceLines(start, end, formatted)
	u.noAutoComplete = false
	u.setStatus(i18n.T("status.jsonFormatted"))
}

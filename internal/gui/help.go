package gui

import (
	"strings"

	"github.com/user/curlyk/internal/help"
	"github.com/user/curlyk/internal/i18n"
)

// helpText renders the key reference of the desktop window: the shared GUI key
// list from internal/help plus the variables section, aligned into two columns.
func helpText() string {
	lines := []string{i18n.T("help.title"), "", i18n.T("help.summary"), ""}

	width := 0
	for _, e := range help.GUIKeys {
		if w := len([]rune(e.Key)); w > width {
			width = w
		}
	}
	for _, e := range help.GUIKeys {
		pad := width - len([]rune(e.Key))
		lines = append(lines, e.Key+strings.Repeat(" ", pad)+"  "+i18n.T(e.DescKey))
	}

	lines = append(lines, "", i18n.T("help.varTitle"), "", i18n.T("help.varHint"), "")
	for _, v := range help.VarLines {
		lines = append(lines, i18n.T(v.Key))
	}
	return strings.Join(lines, "\n")
}

// showHelp puts the key reference into the response body. The header viewer is
// hidden and the copy button then reports that there is nothing to copy.
func (u *ui) showHelp() {
	u.helpVisible = true
	text := helpText()
	u.respHdr.SetResponseText("", 0)
	u.resp.SetResponseText(text, 0)
	u.updateResponseLayout(0)
	u.resp.scrollY = 0
	u.resp.Refresh()
}

// toggleHelp shows or hides the key reference. F1 shows it over the current
// response; pressing F1 again returns to that response (or to the empty-state
// placeholder when nothing has been run yet).
func (u *ui) toggleHelp() {
	if u.helpVisible {
		u.helpVisible = false
		if u.respText != "" {
			u.applyResponseText(u.respText, u.respHeaderRows)
		} else {
			u.respHdr.SetResponseText("", 0)
			u.resp.SetText(i18n.T("gui.noResponse"))
			u.updateResponseLayout(0)
		}
		u.resp.scrollY = 0
		u.resp.Refresh()
		return
	}
	u.showHelp()
}

// helpShown reports whether the pane currently renders the key reference (the
// empty state or F1).
func (u *ui) helpShown() bool { return u.helpVisible }

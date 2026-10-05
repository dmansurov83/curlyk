package tui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/user/curlyk/internal/i18n"
)

// fileSelStyle highlights the selected file row in the sidebar.
var fileSelStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("212")).Bold(true)

// fileHoverStyle highlights the file row under the mouse pointer in the sidebar.
var fileHoverStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("212")).Background(lipgloss.Color("238"))

func (m model) View() string {
	if m.width == 0 {
		return i18n.T("loading.editor") + "\n"
	}
	// left file panel width
	lay := m.layout()
	fw := lay.files
	mid := lay.mid
	if mid < 15 {
		mid = fw
	}
	respH := m.height - 6

	filesView := m.renderFilesPanel(fw, respH)
	edView := m.renderEditor(mid)
	// Each pane's lipgloss border adds 2 cells (left + right) on top of its
	// interior Width, and the files + editor panes already occupy their full
	// bordered width. The response pane gets the remaining width so the three
	// joined panes fit exactly in m.width: fw + mid + respW + 6 == m.width.
	respW := m.width - fw - mid - 6
	if respW < 10 {
		respW = 10
	}
	respView := m.renderResponse(respW, respH)

	pan := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color(borderColor(m.active == paneFiles))).
		Width(fw).Height(m.height - 4).
		Render(filesView)

	left := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color(borderColor(m.active == paneEdit))).
		Width(mid).Height(m.height - 4).
		Render(edView)

	right := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color(borderColor(m.active == paneResp))).
		Width(respW).Height(m.height - 4).
		Render(respView)

	bar := m.renderStatus(m.width)
	row1 := lipgloss.JoinHorizontal(lipgloss.Top, pan, left)
	paneRow := lipgloss.JoinHorizontal(lipgloss.Top, row1, right)
	// A modal dialog overlays the pane row, centred on top of it. The header
	// and status bar stay on screen so the dialog is clearly a floating window.
	if m.dialog != nil {
		paneRow = m.renderDialogOverlay(paneRow)
	}
	header := m.renderHeader(m.width)
	// The unified search bar (files or response) is drawn on the top header row
	// (in place of the file-name bar) so no pane row moves — the pane content,
	// and thus mouse and selection coordinates, stay exactly as when it is
	// closed.
	if m.searchActive() {
		header = m.searchBarRow(m.width)
	}
	return header + "\n" + paneRow + "\n" + bar
}

// renderFilesPanel draws the left sidebar: a search box on top, the file list
// with a "new file" entry, and an environment-profile section at the bottom.
func (m *model) renderFilesPanel(width, height int) string {
	if m.filesPanel == nil {
		m.filesPanel = &filesPanel{}
	}
	p := m.filesPanel
	if p.all == nil {
		p.all = httpFilesInDir(".")
	}
	if p.profiles == nil {
		p.loadProfiles()
	}
	var sb strings.Builder
	// The file filter is drawn on the top frame row when the files panel is
	// active (see View), so the panel itself starts directly at the file rows.
	// rows the file area fills so the profile section sits at a predictable
	// position for mouse hit-testing
	totalRows := m.filesPaneH()
	areaRows := m.fileListMaxRows()
	rowsWritten := 0
	selStyle := fileSelStyle
	// hovered file row index (same space as the render idx counter).
	hover := -1
	if m.hoverY >= 0 && m.paneAtX(m.hoverX) == paneFiles {
		if hj, ok := mouseToFilesRow(m, m.hoverY); ok {
			hover = hj
		}
	}
	hoverStyle := fileHoverStyle
	writeRow := func(s string) {
		if rowsWritten >= areaRows {
			return
		}
		sb.WriteString(s + "\n")
		rowsWritten++
	}
	// "new file" pseudo-entry (row index 0 when the filter is empty)
	idx := 0
	if p.filter == "" {
		switch {
		case p.sel == 0:
			writeRow(selStyle.Render("▸ " + i18n.T("file.new")))
		case idx == hover:
			writeRow(hoverStyle.Render("  " + i18n.T("file.new")))
		default:
			writeRow("  " + i18n.T("file.new"))
		}
		idx++
	}
	// file list, capped at the reserved file area rows
	listed := p.filtered()
	for _, f := range listed {
		switch {
		case idx == p.sel:
			writeRow(selStyle.Render("▸ " + f))
		case idx == hover:
			writeRow(hoverStyle.Render("  " + f))
		default:
			writeRow("  " + f)
		}
		idx++
	}
	// pad the file area to its reserved height so the profile section lands on
	// the rows the mouse mapping expects
	for rowsWritten < areaRows {
		sb.WriteString("\n")
		rowsWritten++
	}
	// environment profile section pinned to the bottom of the sidebar. Always
	// rendered so the user can discover profiles; a hint line shows when none
	// exist yet, and a "+ Новый профиль" action is always available at the very
	// bottom to create one.
	sb.WriteString(profileSepStyle.Render(strings.Repeat("─", width-2)) + "\n")
	sb.WriteString(profileTitleStyle.Render(i18n.T("profile.title")) + "\n")
	// hovered profile row (like files): for mouse hover highlighting
	profHover := -1
	if m.hoverY >= 0 && m.paneAtX(m.hoverX) == paneFiles {
		if hj, ok := m.mouseToProfileRow(m.hoverY); ok {
			profHover = hj
		}
	}
	if len(p.profiles) == 0 {
		sb.WriteString("  " + i18n.T("profile.noneConfigured") + "\n")
	} else {
		for pi := range p.profiles {
			sb.WriteString(m.profileRowLine(width, pi, pi == profHover) + "\n")
		}
	}
	// "new profile" pseudo-entry: always last, activates profile-name input.
	// Hover highlighting, like the file "new file" row and profile rows.
	newHover := m.hoverY >= 0 && m.paneAtX(m.hoverX) == paneFiles && m.mouseToProfileNewRow(m.hoverY)
	newLabel := i18n.T("profile.new")
	switch {
	case p.onProfiles && p.profNew:
		sb.WriteString(profileNewSelStyle.Render("▸ "+newLabel) + "\n")
	case newHover:
		sb.WriteString(fileHoverStyle.Render("  "+newLabel) + "\n")
	default:
		sb.WriteString("  " + newLabel + "\n")
	}
	// Cap the whole panel to its interior height so the frame never overflows
	// the terminal on small windows.
	out := sb.String()
	lines := strings.Split(strings.TrimRight(out, "\n"), "\n")
	if len(lines) > totalRows {
		return strings.Join(lines[:totalRows], "\n")
	}
	return out
}

var exampleHTTP = `
GET https://httpbin.org/get?x=1
Accept: application/json

### Создать запись
POST https://httpbin.org/post
Content-Type: application/json

{"title": "тест", "value": 42}
`

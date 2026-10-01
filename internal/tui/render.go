package tui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// fileSelStyle highlights the selected file row in the sidebar.
var fileSelStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("212")).Bold(true)

func (m model) View() string {
	if m.width == 0 {
		return "Загрузка редактора...\n"
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
	header := m.renderHeader(m.width)
	// save-as prompt for naming a new file
	if m.saveAs != nil {
		prompt := lipgloss.NewStyle().
			Background(lipgloss.Color("235")).
			Foreground(lipgloss.Color("222")).
			Width(m.width).
			Render("Сохранить как: " + m.saveAs.View())
		return header + "\n" + paneRow + "\n" + prompt + "\n" + bar
	}
	return header + "\n" + paneRow + "\n" + bar
}

// renderFilesPanel draws the left sidebar: a search box on top, then the file
// list with a "new file" entry.
func (m *model) renderFilesPanel(width, height int) string {
	if m.filesPanel == nil {
		m.filesPanel = &filesPanel{}
	}
	p := m.filesPanel
	if p.all == nil {
		p.all = httpFilesInDir(".")
	}
	var sb strings.Builder
	// search box
	search := "поиск: " + p.filter + "|"
	sb.WriteString(truncateWidth(search, width-2))
	sb.WriteString("\n\n")
	selStyle := fileSelStyle
	// "new file" pseudo-entry (row index 0 when the filter is empty)
	idx := 0
	if p.filter == "" {
		if p.sel == 0 {
			sb.WriteString(selStyle.Render("▸ + Новый файл") + "\n")
		} else {
			sb.WriteString("  + Новый файл\n")
		}
		idx++
	}
	// file list
	listed := p.filtered()
	for _, f := range listed {
		if idx == p.sel {
			sb.WriteString(selStyle.Render("▸ "+f) + "\n")
		} else {
			sb.WriteString("  " + f + "\n")
		}
		idx++
	}
	return sb.String()
}

var exampleHTTP = `

### Получить пользователя
GET https://httpbin.org/get?x=1
Accept: application/json

### Создать запись
POST https://httpbin.org/post
Content-Type: application/json

{"title": "тест", "value": 42}
`

package tui

import (
	"bytes"
	"encoding/json"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/user/curlyk/internal/httpfile"
)

// menuItem is one selectable entry in the request action popup.
type menuItem struct {
	label string
	// run performs the action and returns an optional tea.Cmd (e.g. to run a
	// request). It is called only once, when the item is confirmed with Enter.
	run func(m *model) tea.Cmd
}

// actionMenu is the popup shown when Enter is pressed on a request line.
// It is rendered as an inline block inside the editor pane, just below the
// anchor request line.
type actionMenu struct {
	items []menuItem
	sel   int
	// anchorRow is the editor line index the menu is attached to.
	anchorRow int
}

// menuSelStyle highlights the currently selected menu item.
var menuSelStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("15")).Background(lipgloss.Color("63"))

// menuNormalStyle is the base item style.
var menuNormalStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("252")).Background(lipgloss.Color("237"))

// menuItems builds the selectable actions for the request popup.
func menuItems() []menuItem {
	return []menuItem{
		{label: "▶ Выполнить", run: func(mm *model) tea.Cmd { return mm.runRequest() }},
		{label: "⧉ Копировать как cURL", run: func(mm *model) tea.Cmd { mm.copyAsCurl(); return nil }},
		{label: "{} Форматировать JSON", run: func(mm *model) tea.Cmd { mm.formatRequestJSON(); return nil }},
	}
}

// beginActionMenu opens the action popup for the request under the cursor.
// Returns nil when the cursor is not on a request line.
func (m *model) beginActionMenu() tea.Cmd {
	if !(m.ed.onIcon || isRequestLine(m.ed.Lines(), m.ed.curRow)) {
		return nil
	}
	m.actionMenu = &actionMenu{
		anchorRow: m.ed.curRow,
		items:     menuItems(),
	}
	// Keep the anchor request line and the whole popup inside the visible
	// window. The popup is drawn inline below the anchor, so it consumes the
	// same rows as the source would; reserve one editor row per menu item. If
	// the anchor sits too close to the bottom edge, scroll up so that the last
	// source row is the anchor line and the popup ends exactly at the bottom.
	effVis := m.effEditorVisible()
	if anchor := m.ed.curRow; anchor >= m.ed.scroll+effVis {
		m.ed.scroll = anchor + 1 - effVis
		if m.ed.scroll < 0 {
			m.ed.scroll = 0
		}
	}
	return nil
}

// handleMenuKey processes keys while the action popup is open. It returns the
// tea.Cmd to run (may be nil) and reports whether the key was consumed by the
// menu. When it returns true, the caller must not process the key further.
func (m *model) handleMenuKey(msg tea.KeyMsg) (tea.Cmd, bool) {
	menu := m.actionMenu
	if menu == nil {
		return nil, false
	}
	switch msg.String() {
	case "up":
		if menu.sel > 0 {
			menu.sel--
		}
	case "down":
		if menu.sel < len(menu.items)-1 {
			menu.sel++
		}
	case "esc", "ctrl+c", "q":
		m.actionMenu = nil
	case "enter":
		it := menu.items[menu.sel]
		m.actionMenu = nil
		if it.run != nil {
			return it.run(m), true
		}
	}
	return nil, true
}

// menuLines renders the popup block (one line per item) for the editor pane.
// contentW is the pane content width in cells.
func (m *model) menuLines(contentW int) []string {
	menu := m.actionMenu
	if menu == nil {
		return nil
	}
	out := make([]string, 0, len(menu.items))
	for i, it := range menu.items {
		line := padToWidth("  "+it.label, contentW)
		if i == menu.sel {
			line = menuSelStyle.Render(line)
		} else {
			line = menuNormalStyle.Render(line)
		}
		out = append(out, line)
	}
	return out
}

// menuAnchorRow returns the editor line the menu is attached to, or -1 when the
// menu is not open.
func (m *model) menuAnchorRow() int {
	if m.actionMenu == nil {
		return -1
	}
	return m.actionMenu.anchorRow
}

// menuItemAt maps an absolute screen y to the menu item index under it, or -1
// when the click is outside the open popup. The popup is drawn immediately
// below its anchor request line (one line per item).
func (m *model) menuItemAt(y int) int {
	menu := m.actionMenu
	if menu == nil {
		return -1
	}
	firstRow := headerHeight + 2 + (menu.anchorRow - m.ed.scroll)
	idx := y - firstRow
	if idx < 0 || idx >= len(menu.items) {
		return -1
	}
	return idx
}

// menuRow returns the screen y of the first menu row when the popup is open,
// or -1 otherwise.
func (m *model) menuRow() int {
	menu := m.actionMenu
	if menu == nil {
		return -1
	}
	return headerHeight + 2 + (menu.anchorRow - m.ed.scroll)
}

// activateMenuItem selects and runs the menu item at index i (used by mouse
// clicks). Returns the tea.Cmd produced by the action, if any.
func (m *model) activateMenuItem(i int) tea.Cmd {
	menu := m.actionMenu
	if menu == nil || i < 0 || i >= len(menu.items) {
		return nil
	}
	it := menu.items[i]
	m.actionMenu = nil
	if it.run != nil {
		return it.run(m)
	}
	return nil
}

// formatRequestJSON pretty-prints the JSON body of the request under the cursor,
// replacing its lines in the editor. It reports an error in the status bar when
// there is no body or the body is not valid JSON.
func (m *model) formatRequestJSON() {
	m.ed.sanitize()
	reqs := httpfile.ParseFile(m.ed.Text())
	req := httpfile.GetRequestAtLine(reqs, m.ed.curRow+1)
	if req == nil || req.Body == "" {
		m.status = "Нет тела запроса для форматирования"
		return
	}
	trimmed := strings.TrimSpace(req.Body)
	if trimmed == "" || !json.Valid([]byte(trimmed)) {
		m.status = "Тело запроса не является валидным JSON"
		return
	}
	var out bytes.Buffer
	if err := json.Indent(&out, []byte(trimmed), "", "  "); err != nil {
		m.status = "Ошибка форматирования JSON: " + err.Error()
		return
	}
	if req.BodyStart <= 0 || req.BodyEnd < req.BodyStart {
		m.status = "Не удалось определить строки тела запроса"
		return
	}
	// Replace the body line range [BodyStart, BodyEnd] (1-based) with the
	// formatted JSON. The editor works with 0-based lines.
	start := req.BodyStart - 1
	end := req.BodyEnd // exclusive
	formatted := out.String()
	m.ed.pushUndo() // snapshot current state (no batch active => recorded)
	m.markDirty()
	newLines := make([]string, 0, len(m.ed.lines)-(end-start)+1)
	newLines = append(newLines, m.ed.lines[:start]...)
	for _, ln := range strings.Split(formatted, "\n") {
		newLines = append(newLines, ln)
	}
	newLines = append(newLines, m.ed.lines[end:]...)
	m.ed.lines = newLines
	m.ed.clampCol()
	m.status = "JSON тела отформатирован"
}

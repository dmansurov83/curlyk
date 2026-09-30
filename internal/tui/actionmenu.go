package tui

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
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
var menuNormalStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("252"))

// beginActionMenu opens the action popup for the request under the cursor.
// Returns nil when the cursor is not on a request line.
func (m *model) beginActionMenu() tea.Cmd {
	if !(m.ed.onIcon || isRequestLine(m.ed.Lines(), m.ed.curRow)) {
		return nil
	}
	// Ensure the request line and its popup fit within the visible window:
	// scroll up if the anchor is too close to the bottom edge.
	menuH := 2 // two items
	if anchor := m.ed.curRow; anchor >= m.ed.scroll+m.ed.height-menuH {
		m.ed.scroll = anchor - (m.ed.height - menuH)
		if m.ed.scroll < 0 {
			m.ed.scroll = 0
		}
	}
	m.actionMenu = &actionMenu{
		anchorRow: m.ed.curRow,
		items: []menuItem{
			{label: "▶ Выполнить", run: func(mm *model) tea.Cmd { return mm.runRequest() }},
			{label: "⧉ Копировать как cURL", run: func(mm *model) tea.Cmd { mm.copyAsCurl(); return nil }},
		},
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
		var line string
		if i == menu.sel {
			line = menuSelStyle.Render("  " + it.label)
		} else {
			line = menuNormalStyle.Render("  " + it.label)
		}
		out = append(out, padToWidth(line, contentW))
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
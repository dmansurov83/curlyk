package tui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/user/curlyk/internal/httpfile"
	"github.com/user/curlyk/internal/i18n"
)

// maxNavRows is the maximum number of request entries the navigation popup
// lists before truncating with an ellipsis row.
const maxNavRows = 8

// navEntry is one selectable request in the "navigate by name" popup.
type navEntry struct {
	// name is the @name annotation (empty for unnamed requests).
	name string
	// label is the text shown in the list: the @name, or "METHOD URL".
	label string
	// search is the lowercased text matched against the filter.
	search string
	// line is the 1-based source line of the request line to jump to.
	line int
}

// navMenu is the "goto request" popup opened by Ctrl+G: a list of every request
// in the file with an incremental filter.
type navMenu struct {
	entries []navEntry
	// sel is the selected index within the visible (filtered) list.
	sel int
	// filter is the live query typed by the user.
	filter string
}

// beginNav opens the navigation popup, collecting the current file's requests.
func (m *model) beginNav() {
	m.nav = &navMenu{
		entries: m.buildNavEntries(),
		sel:     0,
		filter:  "",
	}
}

// buildNavEntries parses the editor text and returns one entry per request.
func (m *model) buildNavEntries() []navEntry {
	m.ed.sanitize()
	reqs := httpfile.ParseFile(m.ed.Text())
	entries := make([]navEntry, 0, len(reqs))
	for _, r := range reqs {
		label := r.Name
		search := r.Name
		if r.Name == "" {
			// Unnamed requests are shown as "METHOD URL" (filterable and searchable
			// by the full target, not only the truncated label).
			label = r.Method + " " + r.URL
			search = label
		}
		entries = append(entries, navEntry{
			name:   r.Name,
			label:  label,
			search: strings.ToLower(search),
			line:   r.Line,
		})
	}
	return entries
}

// visible returns the indices (into entries) that match the current filter, in
// source order.
func (n *navMenu) visible() []int {
	if n.filter == "" {
		out := make([]int, len(n.entries))
		for i := range n.entries {
			out[i] = i
		}
		return out
	}
	q := strings.ToLower(n.filter)
	var out []int
	for i, e := range n.entries {
		if strings.Contains(e.search, q) {
			out = append(out, i)
		}
	}
	return out
}

// handleNavKey processes keys while the navigation popup is open. It returns the
// tea.Cmd to run (nil for all navigation actions) and consumes every key.
func (m *model) handleNavKey(msg tea.KeyMsg) tea.Cmd {
	nav := m.nav
	if nav == nil {
		return nil
	}
	switch msg.String() {
	case "esc", "ctrl+c":
		m.nav = nil
	case "enter":
		vis := nav.visible()
		if nav.sel >= 0 && nav.sel < len(vis) {
			e := nav.entries[vis[nav.sel]]
			m.nav = nil
			m.jumpToRequest(e)
		}
	case "up":
		if nav.sel > 0 {
			nav.sel--
		}
	case "down":
		if nav.sel < len(nav.visible())-1 {
			nav.sel++
		}
	case "backspace":
		if r := []rune(nav.filter); len(r) > 0 {
			nav.filter = string(r[:len(r)-1])
			if nav.sel > 0 && nav.sel >= len(nav.visible()) {
				nav.sel = len(nav.visible()) - 1
			}
		}
	default:
		// Printable runes extend the filter; control keys (e.g. a lone Ctrl
		// arriving as NUL) are ignored.
		changed := false
		for _, r := range msg.Runes {
			if r >= 0x20 && r != 0x7f {
				nav.filter += string(r)
				changed = true
			}
		}
		if changed {
			// Keep the selection within the freshly filtered list.
			if nav.sel >= len(nav.visible()) {
				nav.sel = 0
			}
		}
	}
	return nil
}

// jumpToRequest moves the editor cursor onto a request's line and reports the
// jump in the status bar.
func (m *model) jumpToRequest(e navEntry) {
	m.active = paneEdit
	m.ed.curRow = e.line - 1
	m.ed.curCol = 0
	m.ed.onIcon = false
	m.selActive = false
	if m.ed.curRow < 0 {
		m.ed.curRow = 0
	}
	if m.ed.curRow >= len(m.ed.Lines()) {
		m.ed.curRow = len(m.ed.Lines()) - 1
	}
	m.ed.EnsureVisible()
	m.status = i18n.T("nav.jumped", e.label, e.line)
}

// navReserve returns the number of editor rows the open navigation popup
// consumes, or 0 when it is closed.
func (m *model) navReserve() int {
	if m.nav == nil {
		return 0
	}
	return m.navHeight()
}

// navHeight returns the display height (in rows) of the open navigation popup,
// matching exactly what navLines renders. It is the filter title row plus the
// entries: up to maxNavRows request rows (with an extra ellipsis row when the
// list is truncated) or a single "no matches" row when empty. Capped so a tiny
// window cannot overflow the frame.
func (m *model) navHeight() int {
	nav := m.nav
	if nav == nil {
		return 0
	}
	v := len(nav.visible())
	rows := 1 // filter title row
	if v == 0 {
		rows++ // "no matches" row
	} else if v > maxNavRows {
		rows += maxNavRows + 1 // entries + ellipsis row
	} else {
		rows += v
	}
	cap := m.height - 4
	if rows > cap {
		rows = cap
	}
	if rows < 0 {
		rows = 0
	}
	return rows
}

// navLines renders the navigation popup block (title row + entries) for the
// editor pane. contentW is the pane content width in cells.
func (m *model) navLines(contentW int) []string {
	nav := m.nav
	if nav == nil {
		return nil
	}
	var out []string
	title := padToWidth("  "+i18n.T("nav.title", nav.filter), contentW)
	out = append(out, menuSelStyle.Render(title))
	vis := nav.visible()
	if len(vis) == 0 {
		line := padToWidth("  "+i18n.T(navEmptyKey(nav)), contentW)
		out = append(out, menuNormalStyle.Render(line))
		return out
	}
	shown := 0
	for _, vi := range vis {
		if shown >= maxNavRows {
			break
		}
		e := nav.entries[vi]
		label := truncateWidth("  "+e.label, contentW)
		label = padToWidth(label, contentW)
		if shown == nav.sel {
			label = menuSelStyle.Render(label)
		} else {
			label = menuNormalStyle.Render(label)
		}
		out = append(out, label)
		shown++
	}
	if shown < len(vis) {
		out = append(out, menuNormalStyle.Render(padToWidth("  …", contentW)))
	}
	return out
}

// navEmptyKey picks the "no entries" message: the file has no requests at all
// versus the filter matched nothing.
func navEmptyKey(nav *navMenu) string {
	if len(nav.entries) == 0 {
		return "nav.noRequests"
	}
	return "nav.noMatches"
}

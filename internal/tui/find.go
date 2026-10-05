package tui

import (
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/mattn/go-runewidth"
	"github.com/user/curlyk/internal/i18n"
)

// searchBox is the unified top-row search bar opened over a non-editable panel
// (the file list or the response body). Both panels cannot be typed into, so any
// printable input there means "search". While open it consumes all keys and the
// search bar is drawn on the top frame row (in place of the file-name header),
// so the pane content below keeps its exact coordinates.
type searchBox struct {
	input *textinput.Model
	// target is the panel this search filters: paneFiles or paneResp.
	target pane
}

// findMatch is one occurrence of the search query on a response body line. col
// is the rune index where the match starts; the match covers
// [col, col+queryLen) in runes.
type findMatch struct {
	col int
}

// findStyle highlights all non-current matches; findCurStyle highlights the
// currently selected one distinctly.
var findStyle lipgloss.Style
var findCurStyle lipgloss.Style

// openSearch opens the unified search box targeting `target` (the file list or
// the response body), pre-filled with initial text.
func (m *model) openSearch(target pane, initial string) {
	ti := textinput.New()
	ti.Placeholder = searchPlaceholder(target)
	ti.Focus()
	ti.Width = 40
	ti.SetValue(initial)
	ti.CursorEnd()
	m.search = &searchBox{input: &ti, target: target}
	m.findMatches = nil
	m.findCur = 0
	if target == paneResp {
		m.refreshFind()
	} else {
		m.applyFileFilter(initial)
	}
	m.active = target
}

// searchPlaceholder returns the i18n placeholder for a search targeting target.
func searchPlaceholder(target pane) string {
	if target == paneFiles {
		return i18n.T("find.placeholderFile")
	}
	return i18n.T("find.placeholder")
}

// searchQuery returns the current search query text.
func (m *model) searchQuery() string {
	if m.search == nil {
		return ""
	}
	return m.search.input.Value()
}

// searchLabel returns the label of the active search box (files or response).
func (m *model) searchLabel() string {
	if m.search != nil && m.search.target == paneFiles {
		return i18n.T("search.label", m.searchQuery())
	}
	return i18n.T("find.bar", m.search.input.View(), m.findCountText())
}

// applyFileFilter feeds the current search query into the files panel filter.
func (m *model) applyFileFilter(query string) {
	if m.filesPanel == nil {
		m.filesPanel = &filesPanel{}
	}
	p := m.filesPanel
	p.filter = query
	p.sel = 0
	p.onProfiles = false
}

// findMatchesFor returns every (case-insensitive) match of query on the response
// body lines, in reading order (line-major, then column). findMatch carries only
// the column; the length is always the query length in runes.
func (m *model) findMatchesFor(query string) [][]findMatch {
	if query == "" || m.response == "" {
		return nil
	}
	q := []rune(strings.ToLower(query))
	mr := len(q)
	perLine := make([][]findMatch, len(respBodyLines(m)))
	for li, ln := range respBodyLines(m) {
		low := []rune(strings.ToLower(ln))
		var lineMatches []findMatch
		for i := 0; i+mr <= len(low); i++ {
			if string(low[i:i+mr]) == string(q) {
				lineMatches = append(lineMatches, findMatch{col: i})
			}
		}
		perLine[li] = lineMatches
	}
	return perLine
}

// findFlat is the flattened, sequential view of findMatches used for the counter
// and navigation ("match N of M").
type findFlat struct {
	line int // body-line index
	col  int // rune index within the line
}

// flattenFind flattens perLine matches into sequential order.
func flattenFind(per [][]findMatch) []findFlat {
	var out []findFlat
	for li, lineMatches := range per {
		for _, mm := range lineMatches {
			out = append(out, findFlat{line: li, col: mm.col})
		}
	}
	return out
}

// refreshFind recomputes response matches after the query changed and snaps the
// current match to the first match on/after the current viewport.
func (m *model) refreshFind() {
	m.findMatches = m.findMatchesFor(m.searchQuery())
	if m.findCur >= len(m.findMatches) {
		m.findCur = 0
	}
	m.scrollToCurrentMatch()
	m.active = paneResp
}

// findGlobalIndex returns the flat (sequential) index of the match on body line
// `line` at rune column `col`, or -1 when it is not among the current matches.
func (m *model) findGlobalIndex(line, col int) int {
	idx := 0
	for li := 0; li < line; li++ {
		idx += len(m.findMatches[li])
	}
	for _, mm := range m.findMatches[line] {
		if mm.col == col {
			return idx
		}
		idx++
	}
	return -1
}

// nextFind moves to the next response match (wrap) and scrolls it into view.
func (m *model) nextFind() {
	if len(m.findMatches) == 0 {
		return
	}
	flat := flattenFind(m.findMatches)
	n := len(flat)
	if n == 0 {
		return
	}
	nc := m.findCur + 1
	if nc >= n {
		nc = 0
	}
	m.findCur = nc
	m.scrollToCurrentMatch()
	m.active = paneResp
}

// prevFind moves to the previous response match (wrap) and scrolls it into view.
func (m *model) prevFind() {
	if len(m.findMatches) == 0 {
		return
	}
	n := len(flattenFind(m.findMatches))
	if n == 0 {
		return
	}
	nc := m.findCur - 1
	if nc < 0 {
		nc = n - 1
	}
	m.findCur = nc
	m.scrollToCurrentMatch()
	m.active = paneResp
}

// scrollToCurrentMatch brings the match at flat index findCur into view: the
// body line is centred vertically, and the match column is scrolled horizontally
// so it is visible (and centred when the line is wider than the pane).
func (m *model) scrollToCurrentMatch() {
	flat := flattenFind(m.findMatches)
	if len(flat) == 0 {
		return
	}
	if m.findCur < 0 || m.findCur >= len(flat) {
		m.findCur = 0
	}
	fm := flat[m.findCur]
	bodyLine := fm.line
	// respScroll is body-relative. Visible body rows = pane height minus the
	// fixed header + copy rows. Centre the matched body line within the
	// remaining viewport; clamp so the top of the body (scroll 0) and the last
	// line are still reachable.
	vis := m.height - 6 - m.respHeaderLines
	if vis < 3 {
		// Too small to centre meaningfully: keep the simple reveal.
		if vis < 1 {
			vis = 1
		}
		if bodyLine < m.respScroll {
			m.respScroll = bodyLine
		}
		if bodyLine >= m.respScroll+vis {
			m.respScroll = bodyLine - vis + 1
		}
	} else {
		m.respScroll = bodyLine - vis/2
	}
	if m.respScroll < 0 {
		m.respScroll = 0
	}
	mx := m.respMaxScroll()
	if m.respScroll > mx {
		m.respScroll = mx
	}
	m.scrollToMatchColumn(fm.line, fm.col)
}

// scrollToMatchColumn scrolls the response pane horizontally so the match at
// body line `line`, rune column `col`, is brought into view. The match is
// centred when the surrounding line is wider than the pane; otherwise the
// horizontal scroll is reset to 0.
func (m *model) scrollToMatchColumn(line, col int) {
	lines := respBodyLines(m)
	if line < 0 || line >= len(lines) {
		m.respHScroll = 0
		return
	}
	contentW := m.respContentWidth()
	lineRunes := []rune(lines[line])
	// Display offset (in cells) of the match start, accounting for wide runes.
	matchCells := 0
	for i := 0; i < col && i < len(lineRunes); i++ {
		matchCells += runewidth.RuneWidth(lineRunes[i])
	}
	totalCells := runewidth.StringWidth(lines[line])
	if totalCells <= contentW {
		// The whole line fits: show it from the left edge.
		m.respHScroll = 0
		return
	}
	// Centre the match in the viewport: put the match start at the horizontal
	// middle of the pane.
	want := matchCells - contentW/2
	if want < 0 {
		want = 0
	}
	if want > totalCells-contentW {
		want = totalCells - contentW
	}
	m.respHScroll = want
}

// findCountText returns the "current/total" counter for the response search.
func (m *model) findCountText() string {
	total := len(flattenFind(m.findMatches))
	if total == 0 {
		return "0/0"
	}
	return i18n.T("find.counter", m.findCur+1, total)
}

// closeSearch closes the search box and clears all response highlight state.
func (m *model) closeSearch() {
	if m.search != nil && m.search.target == paneFiles && m.filesPanel != nil {
		m.filesPanel.filter = ""
		m.filesPanel.sel = 0
		m.filesPanel.onProfiles = false
	}
	m.search = nil
	m.findMatches = nil
	m.findCur = 0
}

// handleSearchKey processes keys while the search box is open. It returns the
// tea.Cmd (always nil) and consumes every key.
func (m *model) handleSearchKey(msg tea.KeyMsg) tea.Cmd {
	if m.search == nil {
		return nil
	}
	switch msg.String() {
	case "esc", "ctrl+c":
		m.closeSearch()
		return nil
	case "enter", "down":
		if m.search.target == paneResp {
			m.nextFind()
		}
		return nil
	case "shift+tab", "shift+enter", "up":
		// Shift+Enter has no dedicated bubbletea KeyType (the Key struct has no
		// Shift field), so backward navigation also accepts Shift+Tab / Up.
		if m.search.target == paneResp {
			m.prevFind()
		}
		return nil
	case "tab":
		// Tab cycles to the next searchable pane and retargets the open search,
		// so typing keeps filtering whatever pane is now active. When it cycles
		// onto the editor, the search closes (the editor accepts text directly).
		m.searchTabCycle()
		return nil
	default:
		updated, _ := m.search.input.Update(msg)
		changed := updated.Value() != m.search.input.Value()
		m.search.input = &updated
		if changed {
			m.refreshSearch()
		}
		return nil
	}
}

// refreshSearch recomputes the active search after its query changed.
func (m *model) refreshSearch() {
	if m.search == nil {
		return
	}
	if m.search.target == paneResp {
		m.refreshFind()
	} else {
		m.applyFileFilter(m.searchQuery())
		m.active = paneFiles
	}
}

// searchTabCycle cycles the active pane from a searchable pane to the next one
// in the pane order, retargeting the open search. When the cycle reaches the
// editor (paneEdit), the search closes because the editor takes text directly.
func (m *model) searchTabCycle() {
	var next pane
	switch m.active {
	case paneEdit, paneResp:
		next = paneFiles
	case paneFiles:
		next = paneEdit
	default:
		next = paneResp
	}
	if next == paneEdit {
		// The editor consumes printable input; drop the search overlay.
		m.closeSearch()
		m.active = paneEdit
		return
	}
	m.setActivePane(next)
}

// searchActive reports whether the unified search box is open.
func (m *model) searchActive() bool {
	return m.search != nil
}

// respSearchActive reports whether the search box is open and targets the
// response pane (i.e. response highlighting is in effect).
func (m *model) respSearchActive() bool {
	return m.search != nil && m.search.target == paneResp
}

// setActivePane switches focus to pane p. When the unified search is open and
// the new pane is a searchable one (files or response), the search retargets to
// it so typing always searches the ACTIVE pane — a user can open a files search,
// click into the response pane, and keep typing to search the response instead.
// A search never targets the editor, since the editor accepts text directly.
func (m *model) setActivePane(p pane) {
	m.active = p
	if m.search == nil {
		return
	}
	switch p {
	case paneFiles, paneResp:
		m.search.target = p
		if p == paneResp {
			m.refreshFind()
		} else {
			m.applyFileFilter(m.searchQuery())
		}
	case paneEdit:
		// The editor consumes printable input directly; drop the search overlay
		// so it does not keep swallowing keys.
		m.closeSearch()
	}
}

// searchBarRow returns the ANSI-rendered top search bar row (fitted to width).
func (m *model) searchBarRow(width int) string {
	label := m.searchLabel()
	return lipgloss.NewStyle().
		Background(lipgloss.Color(searchBarBgColor)).
		Foreground(lipgloss.Color(searchBarFgColor)).
		Width(width).
		Render(truncateWidth(label, width))
}

// colorizeFindLine applies the find highlight (all matches on this body line,
// with the current one distinct) plus the active text-selection background.
// hi is the pane-space row; origLn is the full (untruncated) response body line
// and visLn is the already-truncated visible substring for this row; contentW is
// the visible width in cells. Match columns in findMatches are rune offsets into
// the FULL line, so when respHScroll > 0 they are rebased onto the visible
// window before painting.
func (m *model) colorizeFindLine(hi int, origLn, visLn string, contentW int, selStart, selEnd int, hasSel bool) string {
	bodyIdx := hi - m.respHeaderLineCount()
	if bodyIdx < 0 || bodyIdx >= len(m.findMatches) {
		// Header row or out of range: JSON/selection only.
		return jsonColorizeLine(visLn, selStart, selEnd, hasSel)
	}
	mr := len([]rune(m.searchQuery()))
	if mr == 0 {
		return jsonColorizeLine(visLn, selStart, selEnd, hasSel)
	}
	lineMatches := m.findMatches[bodyIdx]
	if len(lineMatches) == 0 {
		return jsonColorizeLine(visLn, selStart, selEnd, hasSel)
	}

	// Rune bounds of the visible window within the full line.
	winStart, _ := visibleRuneRange(origLn, contentW, m.respHScroll)

	runes := []rune(visLn)
	type runDec struct {
		findKind int // 0 none, 1 normal, 2 current
		inSel    bool
	}
	dec := make([]runDec, len(runes))
	for i := range dec {
		dec[i].inSel = hasSel && selStart <= i && i < selEnd
	}
	for _, mm := range lineMatches {
		// Rebases the full-line match column into the visible window.
		local := mm.col - winStart
		if local+mr <= 0 || local >= len(dec) {
			continue // match lies entirely outside the visible window
		}
		k := 1
		if m.findGlobalIndex(bodyIdx, mm.col) == m.findCur {
			k = 2
		}
		from := max(0, local)
		to := min(len(dec), local+mr)
		for i := from; i < to; i++ {
			dec[i].findKind = k
		}
	}

	// Paint contiguous runs with a single style: find current > find normal >
	// selection > plain (then the rest is JSON-coloured below only when no find).
	var b strings.Builder
	i := 0
	n := len(dec)
	for i < n {
		j := i
		for j < n && dec[j] == dec[i] {
			j++
		}
		seg := string(runes[i:j])
		switch {
		case dec[i].findKind == 2:
			b.WriteString(findCurStyle.Render(seg))
		case dec[i].findKind == 1:
			b.WriteString(findStyle.Render(seg))
		case dec[i].inSel:
			b.WriteString(selStyle.Render(seg))
		default:
			b.WriteString(seg)
		}
		i = j
	}
	return b.String()
}

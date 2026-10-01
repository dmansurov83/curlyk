package tui

import (
	"bytes"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/mattn/go-runewidth"
	"github.com/user/curlyk/internal/httpfile"
)

// runIconStyle styles the run ▶ icon in the editor gutter.
var runIconStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("212"))

// iconBlockStyle is the run icon rendered as a block when the cursor is on it.
var iconBlockStyle = lipgloss.NewStyle().Background(lipgloss.Color("63")).Foreground(lipgloss.Color("15")).Bold(true)

// numCurStyle styles the line number on the current cursor line.
var numCurStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("212")).Bold(true)

// numMutedStyle styles the line number on non-cursor lines.
var numMutedStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))

// fileSelStyle highlights the selected file row in the sidebar.
var fileSelStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("212")).Bold(true)

// scrollbarThumbStyle draws the scrollbar thumb.
var scrollbarThumbStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("250"))

// scrollbarTrackStyle draws the scrollbar track.
var scrollbarTrackStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))

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
	respW := m.width - fw - mid - 1
	if respW < 10 {
		respW = m.width - fw - mid
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
	// import prompt overlay (drawn above the status bar)
	if m.importing != nil {
		importBar := lipgloss.NewStyle().
			Background(lipgloss.Color("235")).
			Foreground(lipgloss.Color("222")).
			Width(m.width).
			Render("curl> " + m.importing.input.View())
		return header + "\n" + paneRow + "\n" + importBar + "\n" + bar
	}
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

// renderHeader draws the top bar with the current file name.
func (m *model) renderHeader(width int) string {
	name := m.currentFileName()
	txt := "Файл: " + name
	st := lipgloss.NewStyle().
		Background(lipgloss.Color("235")).
		Foreground(lipgloss.Color("252")).
		Width(width).
		Render(txt)
	return st
}

// currentFileName returns the base name of the open file, or a marker for the
// last-session buffer.
func (m *model) currentFileName() string {
	if m.filePath == "" {
		return "новый файл"
	}
	return filepath.Base(m.filePath)
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
			sb.WriteString(selStyle.Render("▸ " + f) + "\n")
		} else {
			sb.WriteString("  " + f + "\n")
		}
		idx++
	}
	return sb.String()
}

func borderColor(active bool) string {
	if active {
		return "212"
	}
	return "240"
}

func (m *model) renderEditor(width int) string {
	m.ed.sanitize()
	lines := m.ed.Lines()
	toks := httpfile.Lex(m.ed.Text())
	visible := m.ed.height
	scroll := m.ed.scroll
	end := scroll + visible
	if end > len(lines) {
		end = len(lines)
	}
	var sb strings.Builder
	for i := scroll; i < end; i++ {
		// run icon column
		icon := " "
		if isRequestLine(lines, i) {
			icon = ">" // ASCII-safe run indicator (▶ renders as '?' on some consoles)
		}
		iconStyle := runIconStyle
		// number gutter
		num := fmt.Sprintf("%3d ", i+1)
		if i == m.ed.curRow {
			if m.ed.onIcon {
				// cursor is on the run icon: render it as a block
				sb.WriteString(iconBlockStyle.Render(icon))
			} else {
				sb.WriteString(iconStyle.Render(icon))
			}
			sb.WriteString(numCurStyle.Render(num))
		} else {
			sb.WriteString(numMutedStyle.Render(icon))
			sb.WriteString(numMutedStyle.Render(num))
		}

		full := lines[i]
		// Visible content width: pane width minus borders (2) minus gutter (5).
		// contentW is the number of cells the line text may occupy.
		contentW := width - 2 - 5
		if contentW < 8 {
			contentW = 8
		}
		hs := m.ed.hScroll
		overflow := truncateWidthIsOverflow(full, contentW)

		// Window the line: take the rune range that fits in the viewport at the
		// current horizontal scroll offset. Reserve one cell for the '…' marker
		// when the line overflows and we are at the left edge (hScroll==0).
		winW := contentW
		showEllipsis := hs == 0 && overflow
		if showEllipsis {
			winW = contentW - 1
		}
		winStart, winEnd := visibleRuneRange(full, winW, hs)
		winLine := string([]rune(full)[winStart:winEnd])
		// Byte offset of the window start in the original line, for token re-basing.
		winStartB := byteLenOfRunes(full, winStart)
		winEndB := byteLenOfRunes(full, winEnd)
		toks := shiftTokensForWindow(lineToksFor(i, toks), winStartB, winEndB)

		// Apply syntax highlight and (on the cursor line) a visible block cursor,
		// all on the windowed substring, with offsets shifted by winStart.
		var rendered string
		selStart, selEnd, hasSel := m.selectionForLine(i)
		if i == m.ed.curRow {
			cw := m.ed.curCol - winStart
			if cw < 0 {
				cw = 0
			}
			// Clamp any selection on this line to the visible window and pass it
			// so both the block cursor and the selection highlight stay visible.
			var ws, we int
			if hasSel {
				ws = selStart - winStart
				we = selEnd - winStart
				if ws < 0 {
					ws = 0
				}
				if we > winEnd-winStart {
					we = winEnd - winStart
				}
				if we <= ws {
					hasSel = false
				}
			}
			rendered = renderLineWithCursorSel(winLine, cw, toks, ws, we, hasSel)
		} else {
			if hasSel {
				// Clamp the selection range to the visible window.
				ws := selStart - winStart
				we := selEnd - winStart
				if ws < 0 {
					ws = 0
				}
				if we > winEnd-winStart {
					we = winEnd - winStart
				}
				lts := toks
				rendered = renderLineSel(winLine, lts, ws, we)
			} else if lts := toks; len(lts) > 0 {
				rendered = highlightLine(winLine, lts)
			} else {
				rendered = winLine
			}
		}

		if showEllipsis {
			rendered += "…"
		}
		sb.WriteString(padToWidth(rendered, contentW))
		sb.WriteString("\n")
		// Request action popup: draw it right under its anchor request line.
		if i == m.menuAnchorRow() {
			for _, ml := range m.menuLines(contentW) {
				sb.WriteString(ml)
				sb.WriteString("\n")
			}
		}
	}
	return strings.TrimRight(sb.String(), "\n")
}

// editorScrollbarCell returns the scrollbar character for an editor line index
// (relative to the visible window). Returns "" (no scrollbar) when the editor
// content fits, or a blank cell when this line is not the thumb.
func (m *model) editorScrollbarCell(lineIdx, visible int) string {
	return m.scrollbarCell(len(m.ed.Lines()), visible, m.ed.scroll, lineIdx)
}

// scrollbarCell renders a 1-column scrollbar cell for a pane.
// total = total lines, visible = visible lines, scroll = offset,
// rel = 0-based row within the visible window.
// Hidden when content fits; only shows a neutral grey bar on overflow.
func (m *model) scrollbarCell(total, visible, scroll, rel int) string {
	if total <= visible || visible <= 0 {
		return ""
	}
	track := visible
	thumb := 3
	if thumb > track {
		thumb = track
	}
	frac := float64(scroll) / float64(total-visible)
	thumbPos := int(frac * float64(track-thumb))
	if thumbPos < 0 {
		thumbPos = 0
	}
	if rel >= thumbPos && rel < thumbPos+thumb {
		return scrollbarThumbStyle.Render("█")
	}
	return scrollbarTrackStyle.Render("│")
}

func (m *model) renderResponse(width, height int) string {
	if m.state == stateRunning {
		return lipgloss.NewStyle().Foreground(lipgloss.Color("214")).Render("Выполняется...")
	}
	if m.response == "" {
		return "\n  Выполните запрос: Ctrl+Enter\n  Импорт cURL: Ctrl+Y\n  Переключение панелей: Tab"
	}
	lines := respBodyLines(m)
	contentW := width - 2 // left border + scrollbar column
	if contentW < 8 {
		contentW = 8
	}
	var sb strings.Builder
	// Fixed header rows (status/time/headers), then the copy button, then the
	// scrollable body.
	m.respHeaderLines = 0
	if m.respHeader != "" {
		hdr := strings.Split(strings.TrimRight(m.respHeader, "\n"), "\n")
		m.respHeaderLines = len(hdr)
		for hi, h := range hdr {
			sb.WriteString(padToWidth(m.renderHeaderSelLine(hi, h, contentW), contentW))
			sb.WriteString("\n")
		}
	}
	// Fixed copy button row right after the headers (does not scroll).
	sb.WriteString(copyButtonRow(contentW))
	sb.WriteString("\n")
	// Body display lines (raw response text; JSON stays plain).
	body := lines
	if m.respScroll >= len(body) {
		m.respScroll = len(body) - 1
	}
	if m.respScroll < 0 {
		m.respScroll = 0
	}
	contentHeight := height - 1 - m.respHeaderLines
	if contentHeight < 0 {
		contentHeight = 0
	}
	start := m.respScroll
	end := start + contentHeight
	if end > len(body) {
		end = len(body)
	}
	for i := start; i < end; i++ {
		sb.WriteString(padToWidth(m.renderRespSelLine(m.respHeaderLineCount()+i, body[i], contentW), contentW))
		if i < end-1 {
			sb.WriteString("\n")
		}
	}
	return sb.String()
}

// renderRespSelLine truncates a response BODY line to width and applies JSON
// syntax coloring plus the active selection highlight when the given pane-space
// row falls inside it. Body rows are offset from the header count by respScroll;
// the line is truncated first so highlighting never re-measures an ANSI string.
func (m *model) renderRespSelLine(hi int, ln string, contentW int) string {
	if m.respHScroll > 0 {
		ln = truncateWidthOffset(ln, contentW, m.respHScroll)
	} else if truncateWidthIsOverflow(ln, contentW) {
		ln = truncateWidth(ln, contentW-1) + "…"
	} else {
		ln = truncateWidth(ln, contentW)
	}
	var hasSel bool
	var selStart, selEnd int
	if m.respSelActive && m.respHScroll == 0 {
		selStart, selEnd, hasSel = m.respSelectionForLine(hi)
	}
	return jsonColorizeLine(ln, selStart, selEnd, hasSel)
}

// renderHeaderSelLine truncates a fixed response-header row to width and applies
// the active selection highlight directly. Header lines are NOT JSON, so they
// are never run through the JSON colorizer (which would fragment each letter
// into its own ANSI span); selection is the only paint applied here.
func (m *model) renderHeaderSelLine(hi int, ln string, contentW int) string {
	if m.respHScroll > 0 {
		ln = truncateWidthOffset(ln, contentW, m.respHScroll)
	} else if truncateWidthIsOverflow(ln, contentW) {
		ln = truncateWidth(ln, contentW-1) + "…"
	} else {
		ln = truncateWidth(ln, contentW)
	}
	if m.respSelActive && m.respHScroll == 0 {
		s, e, has := m.respSelectionForLine(hi)
		if has {
			ln = renderRespLineSel(ln, s, e)
		}
	}
	return ln
}

// renderRespLineSel highlights the rune range [s,e) in a plain (non-highlighted)
// response line using the selection background.
func renderRespLineSel(line string, s, e int) string {
	runes := []rune(line)
	if s < 0 {
		s = 0
	}
	if e > len(runes) {
		e = len(runes)
	}
	if e < s {
		e = s
	}
	sb := []rune{}
	sb = append(sb, runes[:s]...)
	sb = append(sb, []rune(selStyle.Render(string(runes[s:e])))...)
	sb = append(sb, runes[e:]...)
	return string(sb)
}

// respHeaderLineCount returns the number of fixed header rows rendered above the
// body (from m.respHeader), matching the selection space used by respPaneLines.
func (m *model) respHeaderLineCount() int {
	return respHeaderCount(m)
}

// copyButtonRow renders the clickable "copy response" button line. It is shown
// in the response pane on a grey background so it reads as a clickable control.
func copyButtonRow(contentW int) string {
	label := "⧉  Скопировать ответ"
	full := label + strings.Repeat(" ", max(0, contentW-len([]rune(label))))
	btn := lipgloss.NewStyle().
		Foreground(lipgloss.Color("255")).
		Bold(true).
		Background(lipgloss.Color("240")).
		Render(full)
	return btn
}

// padToWidth pads s (possibly ANSI-coloured) with trailing spaces so its total
// display width is at least w cells.
func padToWidth(s string, w int) string {
	clean := stripANSI(s)
	cur := runewidth.StringWidth(clean)
	if cur >= w {
		return s
	}
	return s + strings.Repeat(" ", w-cur)
}

// respSelectionForLine returns the rune-col range selected on response line row.
func (m *model) respSelectionForLine(row int) (s, e int, has bool) {
	if !m.respSelActive {
		return 0, 0, false
	}
	ar, ac := m.respSelAnchorRow, m.respSelAnchorCol
	cr, cc := m.respSelCurRow, m.respSelCurCol
	sr, sc, er, ec := ar, ac, cr, cc
	if er < sr || (er == sr && ec < sc) {
		sr, sc, er, ec = cr, cc, ar, ac
	}
	if row < sr || row > er {
		return 0, 0, false
	}
	lines := respPaneLines(m)
	maxc := 0
	if row < len(lines) {
		maxc = len([]rune(lines[row]))
	}
	switch {
	case sr == er:
		if row != sr {
			return 0, 0, false
		}
		return sc, ec, true
	case row == sr:
		return sc, maxc, true
	case row == er:
		return 0, ec, true
	default:
		return 0, maxc, true
	}
}

func (m model) renderStatus(width int) string {
	st := m.status
	r := []rune(st)
	if len(r) > width {
		st = string(r[:width])
	}
	return lipgloss.NewStyle().
		Background(lipgloss.Color("236")).
		Foreground(lipgloss.Color("252")).
		Width(width).
		Render(" " + st)
}

// respBodyLines returns the raw response body lines (excluding the header).
func respBodyLines(m *model) []string {
	lines := strings.Split(m.response, "\n")
	hdr := strings.TrimRight(m.respHeader, "\n")
	if hdr != "" {
		hdrCount := len(strings.Split(hdr, "\n"))
		if hdrCount+1 < len(lines) {
			return lines[hdrCount+1:]
		}
		return nil
	}
	return lines
}

func truncateWidth(s string, max int) string {
	if max <= 0 {
		return ""
	}
	r := []rune(s)
	var w int
	var out []rune
	for _, ch := range r {
		rw := runewidth.RuneWidth(ch)
		if w+rw > max {
			break
		}
		out = append(out, ch)
		w += rw
	}
	return string(out)
}

// truncateWidthIsOverflow reports whether s is wider than max display cells and
// therefore would be visually truncated (candidates for an overflow indicator).
func truncateWidthIsOverflow(s string, max int) bool {
	if max <= 0 {
		return s != ""
	}
	return runewidth.StringWidth(s) > max
}

// visibleRuneRange returns the rune index range [start, end) of s that fits
// within `max` display cells starting at a horizontal `offset` in cells. This
// mirrors truncateWidthOffset but yields rune bounds instead of a substring, so
// the caller can slice the original line and still place a cursor correctly.
func visibleRuneRange(s string, max, offset int) (start, end int) {
	runes := []rune(s)
	var w int
	start = 0
	for start < len(runes) {
		rw := runewidth.RuneWidth(runes[start])
		if w+rw > offset {
			break
		}
		w += rw
		start++
	}
	end = start
	w = 0
	for i := start; i < len(runes); i++ {
		rw := runewidth.RuneWidth(runes[i])
		if w+rw > max && i > start {
			break
		}
		w += rw
		end++
	}
	return start, end
}

// truncateWidthOffset returns up to max display-width cells of s, starting at a
// horizontal offset (in cells). Used for horizontal scrolling of long lines.
func truncateWidthOffset(s string, max, offset int) string {
	if max <= 0 {
		return ""
	}
	start, end := visibleRuneRange(s, max, offset)
	return string([]rune(s)[start:end])
}

// displayToRune converts a display-width column into a rune index in s.
func displayToRune(s string, displayPos int) int {
	if displayPos <= 0 {
		return 0
	}
	runes := []rune(s)
	var w int
	for i, r := range runes {
		rw := runewidth.RuneWidth(r)
		if w+rw > displayPos {
			return i
		}
		w += rw
		if w == displayPos {
			return i + 1
		}
	}
	return len(runes)
}

// formatBody pretty-prints JSON response bodies; non-JSON is returned verbatim.
// JSON object-key order is preserved (uses json.Indent rather than MarshalIndent).
func formatBody(body []byte) string {
	s := string(body)
	trimmed := strings.TrimSpace(s)
	if trimmed == "" {
		return s
	}
	if !json.Valid([]byte(trimmed)) {
		return s
	}
	var out bytes.Buffer
	if err := json.Indent(&out, []byte(trimmed), "", "  "); err != nil {
		return s
	}
	return out.String()
}

// shiftTokensForWindow re-bases syntax tokens (byte offsets into the full line)
// onto a windowed substring starting at byte offset `byteStart`, dropping tokens
// that lie entirely outside the window. Variable spans are likewise shifted.
func shiftTokensForWindow(toks []httpfile.Token, byteStart, byteEnd int) []httpfile.Token {
	if byteStart == 0 {
		return toks
	}
	var out []httpfile.Token
	for _, tk := range toks {
		st, en := tk.Start, tk.End
		if en <= byteStart || st >= byteEnd {
			continue // outside window
		}
		if st < byteStart {
			st = byteStart
		}
		if en > byteEnd {
			en = byteEnd
		}
		nt := tk
		nt.Start = st - byteStart
		nt.End = en - byteStart
		out = append(out, nt)
	}
	return out
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
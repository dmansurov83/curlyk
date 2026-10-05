package tui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/user/curlyk/internal/i18n"
)

func (m *model) renderResponse(width, height int) string {
	if m.state == stateRunning {
		return lipgloss.NewStyle().Foreground(lipgloss.Color(executingColor)).Render(i18n.T("status.executing"))
	}
	if m.response == "" || m.helpVisible {
		// Show the hotkey reference inside the right pane: automatically when
		// there is no response yet, or manually toggled with F1. The pane's
		// interior height is exactly `height` rows, so the help fills it fully
		// (padded to a full frame) to avoid trailing blank rows.
		lines := helpPanelLines(width-2, height, m.helpScroll)
		return strings.Join(lines, "\n")
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
	var copyBtn string
	if m.hoverCopyButton() {
		copyBtn = copyButtonRowHover(contentW)
	} else {
		copyBtn = copyButtonRow(contentW)
	}
	sb.WriteString(copyBtn)
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
	origLn := ln
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
	// When the response search is open, paint the find matches over the line
	// (find only targets the body, so header rows keep JSON/selection only).
	if m.respSearchActive() {
		return m.colorizeFindLine(hi, origLn, ln, contentW, selStart, selEnd, hasSel)
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
	label := i18n.T("resp.copyButton")
	full := label + strings.Repeat(" ", max(0, contentW-len([]rune(label))))
	btn := lipgloss.NewStyle().
		Foreground(lipgloss.Color(copyBtnFgColor)).
		Bold(true).
		Background(lipgloss.Color(copyBtnBgColor)).
		Render(full)
	return btn
}

// copyButtonRowHover renders the copy-button row with a hover highlight (lighter
// background) so it reads as the pointer being over the button.
func copyButtonRowHover(contentW int) string {
	label := i18n.T("resp.copyButton")
	full := label + strings.Repeat(" ", max(0, contentW-len([]rune(label))))
	btn := lipgloss.NewStyle().
		Foreground(lipgloss.Color(copyHoverFgColor)).
		Bold(true).
		Background(lipgloss.Color(copyHoverBgColor)).
		Render(full)
	return btn
}

// hoverCopyButton reports whether the mouse pointer is over the response copy
// button row.
func (m *model) hoverCopyButton() bool {
	if m.hoverY < 0 || m.response == "" {
		return false
	}
	if m.paneAtX(m.hoverX) != paneResp {
		return false
	}
	return m.hoverY == headerHeight+1+m.respHeaderLines
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

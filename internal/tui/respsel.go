package tui

import "strings"

// respSelectedText returns the response text covered by the current selection.
func (m *model) respSelectedText() string {
	if !m.respSelActive {
		return ""
	}
	lines := respBodyLines(m)
	ar, ac := m.respSelAnchorRow, m.respSelAnchorCol
	cr, cc := m.respSelCurRow, m.respSelCurCol
	sr, sc, er, ec := ar, ac, cr, cc
	if er < sr || (er == sr && ec < sc) {
		sr, sc, er, ec = cr, cc, ar, ac
	}
	if sr < 0 {
		sr = 0
	}
	if er >= len(lines) {
		er = len(lines) - 1
	}
	if er < sr {
		return ""
	}
	var b strings.Builder
	for r := sr; r <= er; r++ {
		runes := []rune(lines[r])
		cs, ce := 0, len(runes)
		if r == sr {
			cs = sc
			if cs > len(runes) {
				cs = len(runes)
			}
		}
		if r == er {
			ce = ec
			if ce > len(runes) {
				ce = len(runes)
			}
		}
		if ce < cs {
			ce = cs
		}
		b.WriteString(string(runes[cs:ce]))
		if r < er {
			b.WriteString("\n")
		}
	}
	return b.String()
}

// respMoveCursor moves the response-selection cursor without modifying the
// active selection (plain arrows collapse it).
func (m *model) respMoveCursor(key string) {
	if !m.respSelActive {
		// start a collapsed cursor
		m.respSelAnchorRow, m.respSelAnchorCol = m.respSelCurRow, m.respSelCurCol
		m.respSelActive = true
		m.respSelCurRow, m.respSelCurCol = m.respSelAnchorRow, m.respSelAnchorCol
	}
	m.respMoveStep(key, false)
}

// respShiftSelect extends the response selection with Shift+arrows.
func (m *model) respShiftSelect(key string) {
	if !m.respSelActive {
		m.respSelActive = true
	}
	m.respMoveStep(key, true)
}

// respMoveStep moves the response selection cursor by key, optionally extending.
func (m *model) respMoveStep(key string, extend bool) {
	lines := respBodyLines(m)
	maxRow := len(lines) - 1
	if maxRow < 0 {
		return
	}
	if !extend && !m.respSelActive {
		return
	}
	row, col := m.respSelCurRow, m.respSelCurCol
	key = strings.TrimPrefix(key, "shift+")
	switch key {
	case "up":
		if row > 0 {
			row--
		}
		col = min(col, len([]rune(lines[row])))
	case "down":
		if row < maxRow {
			row++
		}
		col = min(col, len([]rune(lines[row])))
	case "left":
		if col > 0 {
			col--
		} else if row > 0 {
			row--
			col = len([]rune(lines[row]))
		}
	case "right":
		if col < len([]rune(lines[row])) {
			col++
		} else if row < maxRow {
			row++
			col = 0
		}
	case "home":
		col = 0
	case "end":
		col = len([]rune(lines[row]))
	}
	m.respSelCurRow, m.respSelCurCol = row, col
	// auto-scroll to keep cursor visible
	if row < m.respScroll {
		m.respScroll = row
	}
	if row >= m.respScroll+m.ed.height {
		m.respScroll = row - m.ed.height + 1
	}
	if m.respScroll < 0 {
		m.respScroll = 0
	}
	mx := m.respMaxScroll()
	if m.respScroll > mx {
		m.respScroll = mx
	}
}
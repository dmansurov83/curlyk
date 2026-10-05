package tui

import (
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

// paneLayout describes the horizontal split of the screen into the three
// panes in ABSOLUTE screen columns (borders included): the files sidebar, the
// editor and the response pane.
type paneLayout struct {
	files    int // files sidebar interior width (as rendered incl. borders)
	filesEnd int // absolute col just past the files pane (its right border + 1)
	mid      int // editor pane interior width
	editorL  int // absolute col of the editor pane's left border
	editorR  int // absolute col of the editor pane's right border (inclusive)
	half     int // absolute col of the response pane's left border = editorR + 1
	respR    int // response pane right border (inclusive) = width-1
}

// layout computes the current pane geometry from the model dimensions.
func (m *model) layout() paneLayout {
	fw := m.filesWidth()
	mid := (m.width - fw) / 2
	filesEnd := fw + 2 // files pane: interior fw + left/right borders
	editorL := filesEnd
	editorR := editorL + mid + 1 // editor interior mid + left/right borders
	half := editorR + 1          // response pane left border
	respR := m.width - 1
	return paneLayout{
		files:    fw,
		filesEnd: filesEnd,
		mid:      mid,
		editorL:  editorL,
		editorR:  editorR,
		half:     half,
		respR:    respR,
	}
}

// handleMouse processes mouse clicks and wheel scroll.
// It returns the updated model and an optional run command.
func (m model) handleMouse(msg tea.MouseMsg) (model, tea.Cmd) {
	lay := m.layout()
	half := lay.half
	if half < 1 {
		half = m.width
	}

	// Start handling mouse gestures.
	switch {
	case msg.Button == tea.MouseButtonLeft && msg.Action == tea.MouseActionPress:
		// A modal dialog owns the mouse while it is open: a click on one of its
		// buttons confirms it, any click outside the frame dismisses the dialog.
		if m.dialog != nil {
			if i := m.dialogButtonAt(msg.X, msg.Y); i >= 0 {
				cmd := m.activateDialog(m.dialog.buttons[i].action)
				return m, cmd
			}
			if !m.dialogContains(msg.X, msg.Y) {
				m.dialog = nil
			}
			return m, nil
		}
	case msg.Button == tea.MouseButtonRight && msg.Action == tea.MouseActionPress:
		// A modal dialog dismisses on any right-click outside the frame.
		if m.dialog != nil {
			if !m.dialogContains(msg.X, msg.Y) {
				m.dialog = nil
			}
			return m, nil
		}
	case msg.Button == tea.MouseButtonNone && msg.Action == tea.MouseActionMotion:
		// Hover: track the position even over a dialog so a freshly hovered
		// button can be highlighted.
		m.hoverX, m.hoverY = msg.X, msg.Y
		if m.dialog != nil {
			m.dialog.hovered = m.dialogButtonAt(msg.X, msg.Y)
		}
		return m, nil
	}

	// Start handling mouse gestures (dialogs dismissed above).
	switch {
	case msg.Button == tea.MouseButtonRight && msg.Action == tea.MouseActionPress:
		// Right-clicking a request line opens its action popup.
		if msg.X < half {
			row, _, onIcon := mouseToEditorCell(&m, msg.X, msg.Y)
			if row >= 0 && (onIcon || isRequestLine(m.ed.Lines(), row)) {
				m.setActivePane(paneEdit)
				m.ed.curRow, m.ed.curCol = row, 0
				m.ed.EnsureVisible()
				m.selActive = false
				m.beginActionMenu()
				return m, nil
			}
		}
		// Right-click on a selection copies it; without a selection in the
		// editor it pastes (like Ctrl+V) at the clicked position.
		if msg.X >= half {
			m.setActivePane(paneResp)
			if m.respSelActive && m.respSelectedText() != "" {
				m.copyRespSelection()
			}
		} else {
			m.setActivePane(paneEdit)
			if m.selActive && m.hasSelection() {
				m.copySelection()
				return m, nil
			}
			// place the cursor where clicked, then paste from clipboard
			row, col, onIcon := mouseToEditorCell(&m, msg.X, msg.Y)
			if row >= 0 && !onIcon {
				m.ed.curRow, m.ed.curCol = row, col
				m.ed.EnsureVisible()
			}
			m.pasteClipboard()
		}
	case msg.Button == tea.MouseButtonLeft && msg.Action == tea.MouseActionPress:
		x, y := msg.X, msg.Y
		m.mouseDragged = false
		// If the navigation popup (Ctrl+G) is open, a click on one of its entry
		// rows jumps to that request; a click elsewhere dismisses the popup.
		if m.nav != nil {
			if i := m.navItemAt(y); i >= 0 && x < lay.half {
				m.setActivePane(paneEdit)
				m.activateNavItem(i)
				return m, nil
			}
			m.nav = nil
		}
		// If the action popup is open, a click on one of its item rows runs it;
		// any click elsewhere in the editor pane dismisses the popup.
		if m.actionMenu != nil {
			if i := m.menuItemAt(y); i >= 0 && x < lay.half {
				m.setActivePane(paneEdit)
				cmd := m.activateMenuItem(i)
				return m, cmd
			}
			m.actionMenu = nil
		}
		// files panel click (incl. its borders)
		if x < lay.filesEnd {
			m.setActivePane(paneFiles)
			if row, ok := m.mouseToProfileRow(y); ok {
				m.openProfile(m.filesPanel.profiles[row])
				return m, nil
			}
			if m.mouseToProfileNewRow(y) {
				m.filesPanel.onProfiles = true
				m.filesPanel.profNew = true
				m.beginProfileAs()
				return m, nil
			}
			if row, ok := mouseToFilesRow(&m, y); ok {
				m.filesPanel.sel = row
				m.openPanelFile()
			}
			return m, nil
		}
		// scrollbar clicks
		if x == half-2 {
			// editor scrollbar column
			m.setActivePane(paneEdit)
			m.clickEditorScrollbar(y)
			return m, nil
		}
		if x == m.width-2 {
			// response scrollbar column
			m.setActivePane(paneResp)
			m.clickRespScrollbar(y)
			return m, nil
		}
		// copy-button row in the response pane: directly below the header rows.
		if x >= half && y == headerHeight+1+m.respHeaderLines && m.response != "" {
			m.setActivePane(paneResp)
			m.copyAllResponse()
			return m, nil
		}
		if x < half {
			m.setActivePane(paneEdit)
			row, col, onIcon := mouseToEditorCell(&m, x, y)
			if row >= 0 {
				// double-click detection (within 300ms and same cell)
				isDouble := !m.lastClickTime.IsZero() &&
					time.Since(m.lastClickTime) < 300*time.Millisecond &&
					row == m.lastClickRow && col == m.lastClickCol
				if !isDouble && time.Since(m.lastClickTime) > 300*time.Millisecond {
					// a fresh single-click sequence
					m.lastWasDouble = false
				}
				m.lastClickRow, m.lastClickCol, m.lastClickTime = row, col, time.Now()
				if isDouble {
					m.lastWasDouble = true
				}

				m.selAnchorRow, m.selAnchorCol = row, col
				m.selActive = true
				m.ed.curRow, m.ed.curCol = row, col
				m.ed.EnsureVisible()

				if onIcon && isRequestLine(m.ed.Lines(), row) {
					// Left-click on the ▶ run icon opens the request action popup
					// (Выполнить / Copy as cURL) instead of running directly.
					m.ed.curRow, m.ed.curCol = row, 0
					m.ed.EnsureVisible()
					m.beginActionMenu()
					m.selActive = false
					return m, nil
				}
				if isDouble {
					// select the word under the cursor; keep this selection
					ws, we := m.ed.wordRange(row, col)
					m.selAnchorRow, m.selAnchorCol = row, ws
					m.ed.curRow, m.ed.curCol = row, we
				}
			}
		} else {
			m.setActivePane(paneResp)
			m.selActive = false
			// click in response pane: start a text selection there
			if rrow, rcol, ok := mouseToRespCell(&m, x, y); ok {
				m.respSelActive = true
				m.respSelAnchorRow, m.respSelAnchorCol = rrow, rcol
				m.respSelCurRow, m.respSelCurCol = rrow, rcol
			}
		}
	case msg.Button == tea.MouseButtonNone && msg.Action == tea.MouseActionMotion:
		// Hover: the cursor moved with no button held. Track the position so
		// popups, the files panel and the copy button can highlight the row
		// under the pointer. With no change since the last event we still return,
		// forcing a repaint so a freshly opened popup highlights the hovered row.
		m.hoverX, m.hoverY = msg.X, msg.Y
	case msg.Button == tea.MouseButtonLeft && msg.Action == tea.MouseActionMotion:
		m.mouseDragged = true
		// drag while holding left button
		if m.active == paneResp {
			if rrow, rcol, ok := mouseToRespCell(&m, msg.X, msg.Y); ok {
				m.respSelCurRow, m.respSelCurCol = rrow, rcol
			}
		} else {
			row, col, _ := mouseToEditorCell(&m, msg.X, msg.Y)
			if row >= 0 {
				m.ed.curRow, m.ed.curCol = row, col
				m.ed.EnsureVisible()
			}
		}
	case msg.Button == tea.MouseButtonLeft && msg.Action == tea.MouseActionRelease:
		// Clear hover so a popup opened right after a click doesn't keep an
		// obsolete highlight; the next motion event re-establishes it.
		m.hoverX, m.hoverY = -1, -1
		if m.dialog != nil {
			m.dialog.hovered = -1
		}
		// finish selection. Keep it if it was a drag or a double-click word
		// selection; only collapse the standalone single-click.
		if m.active == paneResp {
			if rrow, rcol, ok := mouseToRespCell(&m, msg.X, msg.Y); ok {
				m.respSelCurRow, m.respSelCurCol = rrow, rcol
			}
			m.respSelActive = true // keep selection on release
		} else {
			// In the editor, only a drag-release moves the selection cursor end;
			// a plain click / double-click already positioned everything in the
			// press handler, so leave the cursor where the press put it.
			if m.mouseDragged {
				row, col, _ := mouseToEditorCell(&m, msg.X, msg.Y)
				if row >= 0 {
					m.ed.curRow, m.ed.curCol = row, col
					m.ed.EnsureVisible()
				}
			}
			if !m.mouseDragged {
				// A standalone single click (press→release with no motion and no
				// prior double-click) collapses the selection so the cursor just
				// moves. Double-click word selections and drags are kept.
				if !m.lastWasDouble {
					m.selActive = false
				}
			}
			// if mouseDragged, keep the drag selection active
		}
	case msg.Button == tea.MouseButtonWheelUp && msg.Action == tea.MouseActionPress:
		if m.paneAtX(msg.X) == paneResp && m.helpShown() {
			m.helpScroll -= 3
			m.clampHelpScroll()
		} else if m.paneAtX(msg.X) == paneResp {
			m.scrollBy(-3)
		} else {
			m.ed.scroll--
			if m.ed.scroll < 0 {
				m.ed.scroll = 0
			}
			m.clampEditorScroll()
		}
	case msg.Button == tea.MouseButtonWheelDown && msg.Action == tea.MouseActionPress:
		if m.paneAtX(msg.X) == paneResp && m.helpShown() {
			m.helpScroll += 3
			m.clampHelpScroll()
		} else if m.paneAtX(msg.X) == paneResp {
			m.scrollBy(3)
		} else {
			m.ed.scroll++
			m.clampEditorScroll()
		}
	case msg.Button == tea.MouseButtonWheelLeft && msg.Action == tea.MouseActionPress:
		// horizontal scroll left (tilt wheel / shift+wheel), over the hovered pane
		if m.paneAtX(msg.X) == paneResp {
			m.respHScroll -= 4
			if m.respHScroll < 0 {
				m.respHScroll = 0
			}
		} else {
			m.ed.hScroll -= 4
			if m.ed.hScroll < 0 {
				m.ed.hScroll = 0
			}
		}
	case msg.Button == tea.MouseButtonWheelRight && msg.Action == tea.MouseActionPress:
		if m.paneAtX(msg.X) == paneResp {
			m.respHScroll += 4
		} else {
			m.ed.hScroll += 4
		}
	}
	return m, nil
}

// filesWidth returns the left sidebar width.
func (m *model) filesWidth() int {
	fw := 20
	if fw >= m.width {
		fw = m.width / 4
	}
	return fw
}

// paneAtX returns which pane the horizontal cursor position x is over.
func (m *model) paneAtX(x int) pane {
	if x <= 0 {
		return m.active
	}
	lay := m.layout()
	if x < lay.filesEnd {
		return paneFiles
	}
	if x <= lay.editorR {
		return paneEdit
	}
	return paneResp
}

// mouseToEditorCell converts an absolute screen (x,y) to a 0-based (row, runeCol)
// within the editor buffer, or -1 if the click is outside the text area.
// y is the visible line (before scroll); the caller adds e.scroll.
// Returns also whether the click landed on the run-icon column.
func mouseToEditorCell(m *model, x, y int) (row int, runeCol int, onIcon bool) {
	lay := m.layout()
	left := lay.editorL
	right := lay.editorR

	// Outside the editor pane horizontally?
	if x < left || x > right {
		return -1, -1, false
	}
	// Vertical: editor pane is the full height (bordered). Top border y=headerHeight,
	// content y=headerHeight+1.., bottom border y=m.height-1.
	if y <= headerHeight || y >= m.height-1 {
		return -1, -1, false
	}

	row = y - headerHeight - paneContentX + m.ed.scroll // y-headerHeight-1 + scroll
	if row < 0 || row >= len(m.ed.Lines()) {
		if row < 0 {
			row = 0
		} else {
			row = len(m.ed.Lines()) - 1
		}
	}

	// pane-relative content column (0 = left border)
	pcol := x - left
	px := pcol - paneContentX // 0-based column inside the contrast area: icon at 0, num at 1..4, text at 5..
	if px < 0 {
		px = 0
	}

	// icon column?
	if px == 0 {
		return row, 0, true
	}
	// line-number area → clamp to col 0 (text start)
	if px < textColAbs-paneContentX {
		return row, 0, false
	}

	// text area: px - (textColAbs - paneContentX) is the display-width offset
	// within the visible window. The line may be horizontally scrolled, so add
	// hScroll to map back to the full-line display offset.
	textOffset := px - (textColAbs - paneContentX) + m.ed.hScroll
	line := ""
	if row < len(m.ed.Lines()) {
		line = m.ed.Lines()[row]
	}
	runeCol = displayToRune(line, textOffset)
	return row, runeCol, false
}

// mouseToRespCell converts an absolute screen (x,y) in the right (response)
// pane to a (row, runeCol) into the response pane's display lines (0-based,
// header rows first, then the scrollable body) and whether it hit inside the
// pane. Header rows are fixed and counted as-is; body rows are offset by
// respScroll. The copy button (directly below the headers) and borders are skipped.
func mouseToRespCell(m *model, x, y int) (row int, col int, ok bool) {
	half := m.layout().half
	if x < half || y <= headerHeight || y >= m.height-1 {
		return 0, 0, false
	}
	pcol := x - half
	content := pcol - paneContentX
	if content < 0 {
		content = 0
	}
	paneRow := y - (headerHeight + 1) // row within the pane interior (0 = first content row below top border)
	switch {
	case paneRow < 0:
		return 0, 0, false
	case paneRow < m.respHeaderLines:
		// a fixed header line: pane-space row is its 0-based header index.
		row = paneRow
	case paneRow == m.respHeaderLines:
		// the copy-button row: not selectable text.
		return 0, 0, false
	default:
		// a scrollable body line: headerCount + bodyIndex(visible + respScroll).
		bodyVis := paneRow - (1 + m.respHeaderLines)
		row = m.respHeaderLineCount() + bodyVis + m.respScroll
	}

	body := respPaneLines(m)
	if len(body) == 0 {
		return 0, col, true
	}
	if row < 0 {
		row = 0
	}
	if row >= len(body) {
		row = len(body) - 1
	}
	col = displayToRune(body[row], content+m.respHScroll)
	return row, col, true
}

// mouseToFilesRow maps an absolute screen y to a selectable row index in the
// files panel (0-based, including the "+ Новый файл" pseudo-entry when the
// filter is empty), mirroring renderFilesPanel. Returns ok=false when the click
// is on the search box, the panel border, or beyond the listed rows.
func mouseToFilesRow(m *model, y int) (row int, ok bool) {
	// Top border at y=headerHeight; search row + blank row follow, then rows.
	firstRow := headerHeight + 1
	idx := y - firstRow
	if idx < 0 {
		return 0, false
	}
	total := 0
	p := m.filesPanel
	if p != nil {
		if p.filter == "" {
			total = 1 // "+ Новый файл"
		}
		total += len(p.filtered())
	}
	// The file list area may be capped by the profile section at the bottom of
	// the sidebar; a click past the visible file rows is not a file row.
	if max := m.fileListMaxRows(); total > max {
		total = max
	}
	if idx >= total {
		return 0, false
	}
	return idx, true
}

// clickEditorScrollbar sets the editor scroll from a click on its scrollbar.
func (m *model) clickEditorScrollbar(y int) {
	total := len(m.ed.Lines())
	vis := m.ed.height
	mx := total - vis
	if mx < 0 {
		mx = 0
	}
	if vis <= 0 || total <= vis {
		return
	}
	// visible range y in [headerHeight+1, m.height-2] (content rows inside pane)
	rel := y - headerHeight - 1
	if rel < 0 {
		rel = 0
	}
	if rel >= vis {
		rel = vis - 1
	}
	frac := float64(rel) / float64(vis)
	m.ed.scroll = int(frac * float64(mx))
	m.clampEditorScroll()
}

// clickRespScrollbar sets the response scroll from a click on its scrollbar.
func (m *model) clickRespScrollbar(y int) {
	lines := respBodyLines(m)
	vis := m.height - 6 - m.respHeaderLines // scrollable body rows
	total := len(lines)
	mx := total - vis
	if mx < 0 {
		mx = 0
	}
	if vis <= 0 || total <= vis {
		return
	}
	bodyStart := headerHeight + 1 + 1 + m.respHeaderLines
	rel := y - bodyStart
	if rel < 0 {
		rel = 0
	}
	if rel >= vis {
		rel = vis - 1
	}
	frac := float64(rel) / float64(vis)
	m.respScroll = int(frac * float64(mx))
	if m.respScroll < 0 {
		m.respScroll = 0
	}
}

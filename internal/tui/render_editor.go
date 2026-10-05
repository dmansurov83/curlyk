package tui

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/user/curlyk/internal/httpfile"
	"github.com/user/curlyk/internal/i18n"
)

// runIconStyle styles the run ▶ icon in the editor gutter.
var runIconStyle lipgloss.Style

// iconBlockStyle is the run icon rendered as a block when the cursor is on it.
var iconBlockStyle lipgloss.Style

// numCurStyle styles the line number on the current cursor line.
var numCurStyle lipgloss.Style

// numMutedStyle styles the line number on non-cursor lines.
var numMutedStyle lipgloss.Style

// scrollbarThumbStyle draws the scrollbar thumb.
var scrollbarThumbStyle lipgloss.Style

// scrollbarTrackStyle draws the scrollbar track.
var scrollbarTrackStyle lipgloss.Style

// renderHeader draws the top bar with the current file name. A dirty marker is
// appended to the name when the buffer has unsaved changes.
func (m *model) renderHeader(width int) string {
	name := m.currentFileName()
	if m.dirty {
		name += " " + i18n.T("dirty.marker")
	}
	txt := i18n.T("header.file", name)
	if m.profile != "" {
		txt += i18n.T("header.profile", strings.TrimSuffix(filepath.Base(m.profile), ".profile"))
	}
	st := lipgloss.NewStyle().
		Background(lipgloss.Color(headerBgColor)).
		Foreground(lipgloss.Color(headerFgColor)).
		Width(width).
		Render(txt)
	return st
}

// currentFileName returns the base name of the open file, or a marker for the
// last-session buffer.
func (m *model) currentFileName() string {
	if m.filePath == "" {
		return i18n.T("file.unnamed")
	}
	return filepath.Base(m.filePath)
}

// borderColor returns the pane border color depending on whether the pane is
// active. The colors are resolved from the current scheme on each call.
func borderColor(active bool) string {
	if active {
		return borderActiveColor
	}
	return borderIdleColor
}

// menuReserve returns the number of editor content rows consumed by the open
// request action popup (one per menu item), or 0 when the popup is closed.
func (m *model) menuReserve() int {
	if m.actionMenu == nil {
		return 0
	}
	return len(m.actionMenu.items)
}

// popupRows returns the total number of editor content rows the currently open
// popup (request action menu, form editor or navigation) consumes, or 0 when
// none is open.
func (m *model) popupRows() int {
	if m.form != nil {
		return m.formReserve()
	}
	if m.nav != nil {
		return m.navReserve()
	}
	return m.menuReserve()
}

// effEditorVisible returns the number of source rows to render in the editor
// pane when the request action popup is open. The popup is drawn inline under
// its anchor request line, so it trades place with source rows: the source
// lines plus the popup must fit within the pane's content area (m.height-4
// rows). Without this the editor box would grow taller than the sibling panes,
// the whole frame would overrun the terminal height and the top file-name
// header would be pushed off-screen.
func (m *model) effEditorVisible() int {
	visible := m.ed.height
	if menuH := m.popupRows(); menuH > 0 {
		eff := m.height - 4 - menuH
		if eff > visible {
			eff = visible
		}
		if eff < 0 {
			eff = 0
		}
		visible = eff
	}
	return visible
}

func (m *model) renderEditor(width int) string {
	m.ed.sanitize()
	lines := m.ed.Lines()
	toks := httpfile.Lex(m.ed.Text())
	visible := m.effEditorVisible()
	scroll := m.ed.scroll
	end := scroll + visible
	if end > len(lines) {
		end = len(lines)
	}
	var sb strings.Builder
	// Visible content width: pane width minus borders (2) minus gutter (5).
	contentW := width - 2 - 5
	if contentW < 8 {
		contentW = 8
	}
	// Form editor popup (key=value body): a floating block pinned to the top of
	// the editor content area. It trades place with the source rows via the
	// reduced effEditorVisible so the pane never grows.
	if m.form != nil {
		for _, fl := range m.formLines(contentW) {
			sb.WriteString(fl)
			sb.WriteString("\n")
		}
	}
	// Navigation popup (Ctrl+G): a floating block pinned to the top of the
	// editor content area. It trades place with the source rows via the reduced
	// effEditorVisible so the pane never grows.
	if m.nav != nil {
		for _, nl := range m.navLines(contentW) {
			sb.WriteString(nl)
			sb.WriteString("\n")
		}
	}
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
		lineToks := lineToksFor(i, toks)
		// If this line is a JSON request body, lex it once and shift the JSON
		// cols into the window so cursor/selection splits keep the colors.
		var jsonCols []col
		if isBodyTokenLine(lineToks) {
			if jc, ok := jsonColsForLine(full); ok {
				jsonCols = shiftJSONColsForWindow(jc, winStartB, winEndB)
			}
		}
		toks := shiftTokensForWindow(lineToks, winStartB, winEndB)

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
			rendered = renderLineWithCursorSelJSON(winLine, cw, toks, ws, we, hasSel, jsonCols)
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
				rendered = renderLineSelJSON(winLine, lts, ws, we, jsonCols)
			} else if lts := toks; len(lts) > 0 {
				rendered = highlightLineJSON(winLine, lts, jsonCols)
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

func (m model) renderStatus(width int) string {
	st := m.status
	r := []rune(st)
	if len(r) > width {
		st = string(r[:width])
	}
	return lipgloss.NewStyle().
		Background(lipgloss.Color(statusBgColor)).
		Foreground(lipgloss.Color(statusFgColor)).
		Width(width).
		Render(" " + st)
}

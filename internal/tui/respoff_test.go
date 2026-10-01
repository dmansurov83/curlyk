package tui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

// TestRespClickWithHeaderOffset verifies mouse clicks in the response body are
// correctly offset past the fixed header rows (issue F/G). The mapped row counts
// header lines first: the first visible body line is at pane row == header count.
func TestRespClickWithHeaderOffset(t *testing.T) {
	m := New(Args{Width: 120, Height: 30}).(model)
	m.active = paneResp
	m.respHeader = "HTTP/1.1 200 OK\nВремя: 5ms\nX-H: 1\n"
	m.respHeaderLines = 3
	m.response = "HTTP/1.1 200 OK\nВремя: 5ms\nX-H: 1\n\nbodyline0\nbodyline1\n"
	m.respScroll = 0
	rs := m.layout().half
	// body lines start on the pane row right after the copy button and headers:
	// top border + copy + 3 header rows.
	bodyStart := headerHeight + 2 + m.respHeaderLines // = 6 with 3 header rows
	// clicking the first body line maps to pane row 3 (== header count), before
	// scrolling to bodyline0.
	row, col, ok := mouseToRespCell(&m, rs+1, bodyStart)
	if !ok || row != 3 {
		t.Errorf("body row0 should map to pane row %d at y=%d, got row=%d ok=%v", 3, bodyStart, row, ok)
	}
	row, col, _ = mouseToRespCell(&m, rs+5, bodyStart+1)
	if row != 4 {
		t.Errorf("body row1 at y=%d should map to pane row %d, got row=%d", bodyStart+1, 4, row)
	}
	// a click directly on a header line maps to that header's pane row (0-based).
	// with the copy button now below the headers, header row0 is at y=headerHeight+1.
	row, col, ok = mouseToRespCell(&m, rs+1, headerHeight+1) // first header row
	if !ok || row != 0 {
		t.Errorf("header row0 at y=%d should map to pane row 0, got row=%d ok=%v", headerHeight+1, row, ok)
	}
	row, col, ok = mouseToRespCell(&m, rs+1, headerHeight+2) // second header row
	if !ok || row != 1 {
		t.Errorf("header row1 at y=%d should map to pane row 1, got row=%d ok=%v", headerHeight+2, row, ok)
	}
	_ = col
}

// TestRespSelectionWithHeader verifies drag selection in the body works with header offset.
func TestRespSelectionWithHeader(t *testing.T) {
	m := New(Args{Width: 120, Height: 30}).(model)
	rs := m.layout().half
	m.active = paneResp
	m.respHeader = "HTTP/1.1 200 OK\nВремя: 1ms\n"
	m.respHeaderLines = 2
	m.response = "HTTP/1.1 200 OK\nВремя: 1ms\n\nalpha\nbeta\n"
	m.respScroll = 0
	// click body row0 col0 (x=rs+1 → content col0; y=bodyStart → body row0)
	bodyStart := headerHeight + 2 + m.respHeaderLines // headerHeight+copy+headers
	m2, _ := m.Update(tea.MouseMsg{Button: tea.MouseButtonLeft, Action: tea.MouseActionPress, X: rs + 1, Y: bodyStart})
	r := m2.(model)
	// drag to body row1 col3 (x=rs+4 → content col3; y=bodyStart+1 → body row1)
	m3, _ := r.Update(tea.MouseMsg{Button: tea.MouseButtonLeft, Action: tea.MouseActionMotion, X: rs + 4, Y: bodyStart + 1})
	r = m3.(model)
	sel := r.respSelectedText()
	// expected: alpha\nbet
	if sel != "alpha\nbet" {
		t.Errorf("respSelectedText=%q want alpha\\nbet", sel)
	}
}

// TestRespDragSelectHeader verifies dragging across the fixed header rows actually
// selects them (the bug: header clicks were clamped onto the body).
func TestRespDragSelectHeader(t *testing.T) {
	m := New(Args{Width: 120, Height: 30}).(model)
	rs := m.layout().half
	m.active = paneResp
	m.respHeader = "HTTP/1.1 200 OK\nВремя: 1ms\nX-Auth: token\n"
	m.respHeaderLines = 3
	m.response = "HTTP/1.1 200 OK\nВремя: 1ms\nX-Auth: token\n\nbody\n"
	m.respScroll = 0
	// press on the first header row (pane row 0), col 5. With the copy button
	// below the headers, header row0 is at y=headerHeight+1.
	headY := headerHeight + 1
	m2, _ := m.Update(tea.MouseMsg{Button: tea.MouseButtonLeft, Action: tea.MouseActionPress, X: rs + 6, Y: headY})
	r := m2.(model)
	if !r.respSelActive {
		t.Fatal("selection should start when clicking a header row")
	}
	if r.respSelAnchorRow != 0 || r.respSelAnchorCol != 5 {
		t.Fatalf("header anchor=%d,%d want 0,5", r.respSelAnchorRow, r.respSelAnchorCol)
	}
	// drag to the third header row (pane row 2), col 4
	m3, _ := r.Update(tea.MouseMsg{Button: tea.MouseButtonLeft, Action: tea.MouseActionMotion, X: rs + 5, Y: headY + 2})
	r = m3.(model)
	sel := r.respSelectedText()
	// row0 col5..end "1.1 200 OK" + "\nВремя: 1ms\n" + row2 col0..4 "X-Au"
	if sel != "1.1 200 OK\nВремя: 1ms\nX-Au" {
		t.Errorf("respSelectedText=%q\nwant %q", sel, "1.1 200 OK\nВремя: 1ms\nX-Au")
	}
	if r.respSelCurRow != 2 || r.respSelCurCol != 4 {
		t.Errorf("cursor after drag=%d,%d want 2,4", r.respSelCurRow, r.respSelCurCol)
	}
}

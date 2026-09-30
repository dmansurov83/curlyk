package tui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

// TestRespClickWithHeaderOffset verifies mouse clicks in the response body are
// correctly offset past the fixed header rows (issue F/G).
func TestRespClickWithHeaderOffset(t *testing.T) {
	m := New(Args{Width: 120, Height: 30}).(model)
	m.active = paneResp
	m.respHeader = "HTTP/1.1 200 OK\nВремя: 5ms\nX-H: 1\n"
	m.respHeaderLines = 3
	m.response = "HTTP/1.1 200 OK\nВремя: 5ms\nX-H: 1\n\nbodyline0\nbodyline1\n"
m.respScroll = 0
	rs := m.layout().half
	// body starts after header + copy button + top border + app header
	bodyStart := headerHeight + 2 + m.respHeaderLines // = 6 with 3 header rows
	row, col, ok := mouseToRespCell(&m, rs+1, bodyStart)
	if !ok || row != 0 {
		t.Errorf("body row0 should map at y=%d, got row=%d ok=%v", bodyStart, row, ok)
	}
	row, col, _ = mouseToRespCell(&m, rs+5, bodyStart+1)
	if row != 1 {
		t.Errorf("body row1 at y=%d, got row=%d", bodyStart+1, row)
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

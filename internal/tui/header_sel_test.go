package tui

import (
	"strings"
	"testing"
)

// TestHeaderSelectionSingleSpan ensures a selected header line is painted as one
// contiguous ANSI fragment, not fragmented per letter by the JSON colorizer
// (post-conflict regression that produced tons of ANSI switches on each header).
func TestHeaderSelectionSingleSpan(t *testing.T) {
	m := New(Args{Width: 120, Height: 30}).(model)
	m.active = paneResp
	m.respHeader = "HTTP/1.1 200 OK\nВремя: 800ms\n"
	m.response = "HTTP/1.1 200 OK\nВремя: 800ms\n\n{\"a\": 1}\n"
	// select the entire first header row (pane row 0)
	m.respSelActive = true
	m.respSelAnchorRow, m.respSelAnchorCol = 0, 0
	m.respSelCurRow, m.respSelCurCol = 0, len("HTTP/1.1 200 OK")

	out := m.renderResponse(57, 24)
	firstLine := strings.SplitN(out, "\n", 2)[0]

	// The whole header line inside one background-24 span: exactly two escape
	// sequences for that segment (open + reset), so the full "HTTP/1.1 200 OK"
	// text sits between a single \x1b[48;5;24m and \x1b[0m.
	if !strings.Contains(firstLine, "\x1b[48;5;24mHTTP/1.1 200 OK\x1b[0m") {
		t.Errorf("header should be one contiguous selection span, got %q", firstLine)
	}
}

// TestHeaderNotJSONColorized guards that undisrupted headers stay plain text
// (no JSON colors injected into a "Content-Length: 123" header while selected).
func TestHeaderNotJSONColorized(t *testing.T) {
	m := New(Args{Width: 120, Height: 30}).(model)
	m.active = paneResp
	m.respHeader = "HTTP/1.1 200 OK\nContent-Length: 123\n"
	m.response = "HTTP/1.1 200 OK\nContent-Length: 123\n\n{}\n"
	m.respSelActive = true
	m.respSelAnchorRow, m.respSelAnchorCol = 1, 0
	m.respSelCurRow, m.respSelCurCol = 1, 9

	out := m.renderResponse(57, 24)
	lines := strings.SplitN(out, "\n", 3)
	second := lines[1]
	// the selected header line must contain NO json number/key color (214/81) —
	// only the selection background 24.
	if strings.Contains(second, "38;5;214") || strings.Contains(second, "38;5;81") {
		t.Errorf("header line must not carry JSON colors, got %q", second)
	}
	if !strings.Contains(second, "\x1b[48;5;24mContent-L\x1b[0m") {
		t.Errorf("header line should be selection-painted contiguously, got %q", second)
	}
}
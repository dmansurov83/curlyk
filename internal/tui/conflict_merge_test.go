package tui

import (
	"strings"
	"testing"
)

// TestStatusLineNoDoubleHTTP guards against the two-agent conflict regression:
// renderRespSelLine must still compile and paint selection, and applyResponse's
// status line must not duplicate the HTTP/ prefix.
func TestStatusLineNoDoubleHTTP(t *testing.T) {
	// Build the header exactly as applyResponse does for a success response.
	var hdr strings.Builder
	hdr.WriteString("HTTP/1.1 200 OK\n")
	hdr.WriteString("Время: 5ms\n")
	hdr.WriteString("X-H: 1\n")
	m := New(Args{Width: 120, Height: 30}).(model)
	m.active = paneResp
	m.respHeader = hdr.String()
	m.response = hdr.String() + "\n{\"a\": 1}\n"
	m.respSelActive = true
	m.respSelAnchorRow, m.respSelAnchorCol = 0, 0
	m.respSelCurRow, m.respSelCurCol = 1, 0

	out := m.renderResponse(57, 24)
	plain := stripANSI(out)
	first := strings.SplitN(plain, "\n", 2)[0]
	if strings.HasPrefix(first, "HTTP/HTTP/") {
		t.Errorf("status line duplicates HTTP/ prefix: %q", first)
	}
	if !strings.Contains(first, "HTTP/1.1 200 OK") {
		t.Errorf("expected status header present, got %q", first)
	}
}
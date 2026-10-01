package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

// TestCopyButtonClickCopiesResponse verifies clicking the copy button at the
// top of the response pane starts a copy (status reflects it).
func TestCopyButtonClickCopiesResponse(t *testing.T) {
	m := New(Args{Width: 120, Height: 30}).(model)
	m.active = paneResp
	m.response = "HTTP/1.1 200 OK\n\n{\"a\":1}\n"
	m.lastBody = []byte("body")

	// click at x>half (right pane), y=headerHeight+1 (the copy button row)
	m2, _ := m.Update(tea.MouseMsg{
		Button: tea.MouseButtonLeft, Action: tea.MouseActionPress, X: 80, Y: headerHeight + 1,
	})
	r := m2.(model)
	if !strings.Contains(r.status, "Ответ скопирован") {
		t.Errorf("expected copy status, got %q", r.status)
	}
}

// TestCopyButtonRendered verifies renderResponse contains the copy button label
// and that it is drawn with a grey background so it reads as clickable.
func TestCopyButtonRendered(t *testing.T) {
	m := New(Args{Width: 120, Height: 30}).(model)
	m.response = "HTTP/1.1 200 OK\n\n{\"a\":1}\n"
	out := m.renderResponse(60, 10)
	plain := stripANSI(out)
	firstLine := strings.Split(plain, "\n")[0]
	if !strings.Contains(firstLine, "Скопировать") {
		t.Errorf("copy button row missing on first line: %q", firstLine)
	}
	// grey background (color 240) must appear in the copy-button row
	if !strings.Contains(out, "240") {
		t.Errorf("copy button should have grey background (48;5;240), out=%q", out)
	}
}

// TestCopyButtonBelowHeaders verifies the copy button is rendered below the
// header rows (status/time/headers), not above them.
func TestCopyButtonBelowHeaders(t *testing.T) {
	m := New(Args{Width: 120, Height: 30}).(model)
	m.response = "HTTP/1.1 200 OK\nВремя: 1ms\nX-A: b\n\nbody\n"
	m.respHeader = "HTTP/1.1 200 OK\nВремя: 1ms\nX-A: b\n"
	out := m.renderResponse(60, 10)
	plain := stripANSI(out)
	lines := strings.Split(plain, "\n")
	// the copy-button row must come after all three header lines.
	foundBtn := -1
	lastHeaderReq := 2
	for i, ln := range lines {
		switch {
		case strings.Contains(ln, "Скопировать"):
			foundBtn = i
		case strings.HasPrefix(ln, "X-A"):
			if i > lastHeaderReq {
				lastHeaderReq = i
			}
		}
	}
	if foundBtn < 0 {
		t.Fatalf("copy button missing in %q", plain)
	}
	if foundBtn < lastHeaderReq {
		t.Errorf("copy button at line %d must be below header line %d", foundBtn, lastHeaderReq)
	}
	if !strings.HasPrefix(lines[0], "HTTP/1.1") {
		t.Errorf("first rendered line should be the status header, got %q", lines[0])
	}
}
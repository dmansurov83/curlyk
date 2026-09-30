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

// TestCopyButtonRendered verifies renderResponse contains the copy button label.
func TestCopyButtonRendered(t *testing.T) {
	m := New(Args{Width: 120, Height: 30}).(model)
	m.response = "HTTP/1.1 200 OK\n\n{\"a\":1}\n"
	out := m.renderResponse(60, 10)
	plain := stripANSI(out)
	firstLine := strings.Split(plain, "\n")[0]
	if !strings.Contains(firstLine, "Скопировать") {
		t.Errorf("copy button row missing on first line: %q", firstLine)
	}
}
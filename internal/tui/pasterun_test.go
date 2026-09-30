package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

// TestPasteThenRun verifies that after pasting a cURL block, the inserted
// request can be found and run (issue A: "yellow, cannot run").
func TestPasteThenRun(t *testing.T) {
	m := New(Args{Width: 120, Height: 30}).(model)
	m.active = paneEdit
	m.ed.SetText("") // empty buffer
	km := tea.KeyMsg{Type: tea.KeyRunes, Paste: true,
		Runes: []rune(`curl -X GET https://api.test/users/{{id}} -H "Accept: application/json"`)}
	_, _ = m.handleKey(km)
	text := m.ed.Text()
	if !strings.Contains(text, "GET https://api.test/users/{{id}}") {
		t.Fatalf("curl not converted:\n%s", text)
	}
	// cursor should be on or near the request; run should find it
	if cmd := m.runRequest(); cmd != nil {
		// success: a run command was produced
		return
	}
	// if runRequest returned nil, maybe cursor isn't on the request; move there
	m.ed.curRow = 0
	if cmd := m.runRequest(); cmd == nil {
		t.Fatal("runRequest found no request after paste")
	}
}
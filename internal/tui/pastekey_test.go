package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

// TestPasteKeyMsgConvertsCurl verifies that a bracketed-paste KeyMsg containing
// a cURL command is auto-converted (issue #5 fix) rather than inserted raw.
func TestPasteKeyMsgConvertsCurl(t *testing.T) {
	m := New(Args{Width: 120, Height: 30}).(model)
	m.active = paneEdit
	// simulate a bracketed paste of a curl command
	km := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(`curl -X POST https://api.test/x -H "Content-Type: application/json" -d '{"a":1}'`), Paste: true}
	_, cmd := m.handleKey(km)
	if cmd != nil {
		t.Fatal("paste cmd unexpected")
	}
	text := m.ed.Text()
	if strings.Contains(text, "curl -X") {
		t.Errorf("curl pasted raw, not converted:\n%s", text)
	}
	if !strings.Contains(text, "POST https://api.test/x") {
		t.Errorf("curl not converted to request:\n%s", text)
	}
}

// TestPasteKeyMsgPlainInserts verifies a non-curl paste inserts as-is.
func TestPasteKeyMsgPlainInserts(t *testing.T) {
	m := New(Args{Width: 120, Height: 30}).(model)
	m.active = paneEdit
	m.ed.SetText("before\n")
	m.ed.curRow, m.ed.curCol = 0, 6
	km := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("hello world"), Paste: true}
	_, _ = m.handleKey(km)
	if !strings.Contains(m.ed.Text(), "hello world") {
		t.Errorf("plain paste not inserted:\n%s", m.ed.Text())
	}
}
package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

// TestEditorPgUpPgDn verifies PageUp/PageDown scroll the editor when it is the
// active pane.
func TestEditorPgUpPgDn(t *testing.T) {
	m := New(Args{Width: 120, Height: 30}).(model)
	m.active = paneEdit
	var b strings.Builder
	for i := 0; i < 300; i++ {
		b.WriteString("line" + itoa(i) + "\n")
	}
	m.ed.SetText(b.String())
	m.ed.scroll = 0
	m.ed.height = 20

	// PgDn scrolls down by roughly one page
	m2, _ := m.handleKey(tea.KeyMsg{Type: tea.KeyPgDown})
	m = m2.(model)
	t.Logf("after PgDown: scroll=%d height=%d lines=%d", m.ed.scroll, m.ed.height, len(m.ed.Lines()))
	if m.ed.scroll <= 0 {
		t.Errorf("PgDn should scroll editor, scroll=%d", m.ed.scroll)
	}
	d := m.ed.scroll

	// PgUp scrolls back up
	m3, _ := m.handleKey(tea.KeyMsg{Type: tea.KeyPgUp})
	m = m3.(model)
	if m.ed.scroll >= d {
		t.Errorf("PgUp should scroll back up, scroll=%d (was %d)", m.ed.scroll, d)
	}
}

// TestEditorPgDnClamps verifies PgDn at the bottom does not overflow.
func TestEditorPgDnClamps(t *testing.T) {
	m := New(Args{Width: 120, Height: 30}).(model)
	m.active = paneEdit
	m.ed.SetText("short\n")
	m.ed.scroll = 0
	m.ed.height = 20
	m2, _ := m.handleKey(tea.KeyMsg{Type: tea.KeyPgDown})
	if m2.(model).ed.scroll < 0 {
		t.Error("scroll should not go negative")
	}
}
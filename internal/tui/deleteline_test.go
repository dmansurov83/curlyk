package tui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestEditorDeleteLine(t *testing.T) {
	m := New(Args{Width: 120, Height: 30}).(model)
	m.active = paneEdit
	m.ed.SetText("one\ntwo\nthree\n")

	// Delete the middle line.
	m.ed.curRow, m.ed.curCol = 1, 1
	m.ed.DeleteLine()
	if got := m.ed.Text(); got != "one\nthree\n" {
		t.Fatalf("after delete middle=%q", got)
	}
	if m.ed.curRow != 1 || m.ed.curCol != 0 {
		t.Fatalf("cursor after middle=%d,%d", m.ed.curRow, m.ed.curCol)
	}

	// Delete the last line: the trailing "\n" leaves a final empty buffer line,
	// so the cursor stays on the line that now occupies the same index.
	m.ed.curRow, m.ed.curCol = 1, 2
	m.ed.DeleteLine()
	if got := m.ed.Text(); got != "one\n" {
		t.Fatalf("after delete last=%q", got)
	}
	if m.ed.curRow != 1 {
		t.Fatalf("cursor row after last delete=%d", m.ed.curRow)
	}

	// Deleting a single-line buffer is a no-op.
	m.ed.SetText("one")
	m.ed.curRow = 0
	m.ed.DeleteLine()
	if got := m.ed.Text(); got != "one" {
		t.Fatalf("single line changed=%q", got)
	}
}

func TestCtrlYDeleteLine(t *testing.T) {
	m := New(Args{Width: 120, Height: 30}).(model)
	m.active = paneEdit
	m.ed.SetText("one\ntwo\nthree\n")
	m.ed.curRow, m.ed.curCol = 1, 0

	m2, _ := m.handleKey(tea.KeyMsg{Type: tea.KeyCtrlY})
	mm, _ := m2.(model)
	if got := mm.ed.Text(); got != "one\nthree\n" {
		t.Fatalf("ctrl+y text=%q", got)
	}
	if mm.ed.curRow != 1 {
		t.Fatalf("ctrl+y cursor=%d", mm.ed.curRow)
	}
}
package tui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

// setupSel builds a model with a selected range "world" in "hello world\n".
func setupSel(t *testing.T) model {
	m := New(Args{Width: 120, Height: 30}).(model)
	m.active = paneEdit
	m.ed.SetText("hello world\n")
	m.selActive = true
	m.selAnchorRow, m.selAnchorCol = 0, 6
	m.ed.curRow, m.ed.curCol = 0, 11 // world spans cols 6..11
	return m
}

// TestTypeReplacesSelection verifies typing over a selection replaces it.
func TestTypeReplacesSelection(t *testing.T) {
	m := setupSel(t)
	res, _ := m.handleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("X")})
	r := res.(model)
	if got := r.ed.Text(); got != "hello X\n" {
		t.Errorf("type over selection = %q, want %q", got, "hello X\n")
	}
	if r.selActive {
		t.Error("selection must clear after replacing type")
	}
	// after deleting "world" (6..11) cursor at 6, then "X" inserted → col 7
	if r.ed.curRow != 0 || r.ed.curCol != 7 {
		t.Errorf("cursor=%d,%d want 0,7", r.ed.curRow, r.ed.curCol)
	}
}

// TestDeleteReplacesSelection verifies Delete removes the selection.
func TestDeleteReplacesSelection(t *testing.T) {
	m := setupSel(t)
	res, _ := m.handleKey(tea.KeyMsg{Type: tea.KeyDelete})
	r := res.(model)
	if got := r.ed.Text(); got != "hello \n" {
		t.Errorf("delete over selection = %q, want %q", got, "hello \n")
	}
	if r.selActive {
		t.Error("selection must clear after deleting")
	}
}

// TestBackspaceReplacesSelection verifies Backspace removes the selection.
func TestBackspaceReplacesSelection(t *testing.T) {
	m := setupSel(t)
	res, _ := m.handleKey(tea.KeyMsg{Type: tea.KeyBackspace})
	r := res.(model)
	if got := r.ed.Text(); got != "hello \n" {
		t.Errorf("backspace over selection = %q, want %q", got, "hello \n")
	}
	if r.selActive {
		t.Error("selection must clear after backspace")
	}
}

// TestEnterReplacesSelection verifies Enter replaces the selection with a newline.
func TestEnterReplacesSelection(t *testing.T) {
	m := setupSel(t)
	res, _ := m.handleKey(tea.KeyMsg{Type: tea.KeyEnter})
	r := res.(model)
	// "hello world\n" -> delete "world" leaves "hello \n" (line0 "hello "), then
	// Enter at col 6 inserts a newline: line0="hello ", line1="", line2="".
	if got := r.ed.Text(); got != "hello \n\n" {
		t.Errorf("enter over selection = %q, want %q", got, "hello \n\n")
	}
	if r.selActive {
		t.Error("selection must clear after enter")
	}
}
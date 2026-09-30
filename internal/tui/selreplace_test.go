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

// TestModifierKeyKeepsSelection verifies pressing a bare modifier key (e.g. Ctrl)
// with an active selection does NOT delete it.
func TestModifierKeyKeepsSelection(t *testing.T) {
	m := setupSel(t)
	// A zero KeyMsg models an unrecognised/modifier press with no text payload.
	// It must leave the selection and text untouched.
	res, _ := m.handleKey(tea.KeyMsg{})
	r := res.(model)
	if got := r.ed.Text(); got != "hello world\n" {
		t.Errorf("bare modifier must not change text, got %q", got)
	}
	if !r.selActive {
		t.Error("selection must survive a bare modifier press")
	}
}

// TestCtrlAtBareDoesNotDelete simulates how a bare Ctrl arrives on Windows
// (Ctrl+@ / zero KeyMsg) with an active selection: text and selection survive.
func TestCtrlAtBareDoesNotDelete(t *testing.T) {
	m := setupSel(t)
	res, _ := m.handleKey(tea.KeyMsg{Type: tea.KeyCtrlAt})
	r := res.(model)
	if got := r.ed.Text(); got != "hello world\n" {
		t.Errorf("bare Ctrl (ctrl+@) must not delete text, got %q", got)
	}
	if !r.selActive {
		t.Error("selection must survive bare Ctrl (ctrl+@)")
	}
}

// TestUnknownCtrlComboIgnored verifies an unknown Ctrl combination (e.g.
// Ctrl+H) is ignored rather than falling through to edit/delete.
func TestUnknownCtrlComboIgnored(t *testing.T) {
	m := setupSel(t)
	res, _ := m.handleKey(tea.KeyMsg{Type: tea.KeyCtrlH})
	r := res.(model)
	if got := r.ed.Text(); got != "hello world\n" {
		t.Errorf("unknown Ctrl combo must not delete text, got %q", got)
	}
	if !r.selActive {
		t.Error("selection must survive unknown Ctrl combo")
	}
}

// TestBareCtrlRuneKeepsSelection simulates a bare Ctrl arriving as a control
// rune (Ctrl+@ → NUL, 0x00) on Windows, with an active selection.
func TestBareCtrlRuneKeepsSelection(t *testing.T) {
	m := setupSel(t)
	res, _ := m.handleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{0x00}})
	r := res.(model)
	if got := r.ed.Text(); got != "hello world\n" {
		t.Errorf("bare Ctrl (NUL rune) must not delete text, got %q", got)
	}
	if !r.selActive {
		t.Error("selection must survive bare Ctrl (NUL rune)")
	}
}

// TestCtrlLetterRuneKeepsSelection: Ctrl+A arrives as control rune 0x01 and
// must not replace the selection.
func TestCtrlLetterRuneKeepsSelection(t *testing.T) {
	m := setupSel(t)
	res, _ := m.handleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{0x01}})
	r := res.(model)
	if got := r.ed.Text(); got != "hello world\n" {
		t.Errorf("Ctrl letter (0x01) must not delete text, got %q", got)
	}
	if !r.selActive {
		t.Error("selection must survive Ctrl letter (0x01)")
	}
}
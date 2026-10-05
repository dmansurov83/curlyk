package tui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
)

func teaKeyTab() tea.KeyMsg { return tea.KeyMsg{Type: tea.KeyTab} }
func teaKeyEnter() tea.KeyMsg {
	return tea.KeyMsg{Type: tea.KeyEnter}
}

// testDialog builds a small generic framed dialog for exercising the popup
// abstraction, independent of any production call site.
func testDialog(m *model) *dialogBox {
	return m.newDialog("Тест", []string{"Строка 1", "Строка 2"}, []dialogButton{
		{label: "Cancel", action: dialogCancel},
		{label: "Confirm", action: dialogConfirm, defaultBtn: true},
	}, func(m *model, act dialogAction, input string) tea.Cmd {
		return nil // just dismiss
	})
}

// TestDialogFrameFit verifies the popup centres over the pane row without
// growing the frame taller than the terminal and keeps the header.
func TestDialogFrameFit(t *testing.T) {
	for _, h := range []int{10, 15, 20} {
		m := New(Args{Width: 100, Height: h}).(model)
		m.dialog = testDialog(&m)
		out := m.View()
		lines := strings.Split(stripANSI(out), "\n")
		if len(lines) != h {
			t.Errorf("height=%d: frame height=%d, want %d", h, len(lines), h)
		}
		first := lines[0]
		if !strings.Contains(first, "Файл:") {
			t.Errorf("h=%d: header lost; first=%q", h, first)
		}
		if !strings.Contains(out, "Тест") {
			t.Errorf("h=%d: dialog title missing", h)
		}
		if !strings.Contains(out, "Confirm") || !strings.Contains(out, "Cancel") {
			t.Errorf("h=%d: buttons missing", h)
		}
	}
}

// TestDialogKeepsPanesVisible verifies the dialog is a true overlay: the pane
// content around the frame is still present in the rendered output (not blanked
// out by the dialog background).
func TestDialogKeepsPanesVisible(t *testing.T) {
	m := New(Args{Width: 100, Height: 20}).(model)
	m.dialog = testDialog(&m)
	strip := stripANSI(m.View())
	// The example HTTP text lives in the editor pane and must survive the
	// overlay (only the frame's own cells are overwritten).
	if !strings.Contains(strip, "httpbin.org/get") {
		t.Fatalf("editor content behind the dialog was lost (overlay blanks the pane)")
	}
	// The left file panel ("+ Новый файл") must also remain visible.
	if !strings.Contains(strip, "+ Новый файл") {
		t.Fatalf("files panel content behind the dialog was lost")
	}
}

// TestDialogKeys verifies keyboard handling: the default is Confirm, Tab cycles
// to Cancel, Enter closes with the selected action.
func TestDialogKeys(t *testing.T) {
	m := New(Args{Width: 100, Height: 20}).(model)
	m.dialog = testDialog(&m)
	if m.dialog.sel != 1 {
		t.Fatalf("default sel=%d want 1 (Confirm)", m.dialog.sel)
	}
	// Tab cycles to Cancel (index 0).
	res, _ := m.handleKey(teaKeyTab())
	if res.(model).dialog == nil {
		t.Fatalf("dialog should still be open after Tab")
	}
	if res.(model).dialog.sel != 0 {
		t.Fatalf("after Tab sel=%d want 0", res.(model).dialog.sel)
	}
	// Enter on the selected (Cancel) dismisses the dialog.
	res2, _ := res.(model).handleKey(teaKeyEnter())
	if res2.(model).dialog != nil {
		t.Fatalf("Enter should dismiss the dialog")
	}
}

// TestDialogBlocksKeys verifies the dialog consumes keys (a printable rune must
// NOT reach the editor while the popup is open).
func TestDialogBlocksKeys(t *testing.T) {
	m := New(Args{Width: 100, Height: 20}).(model)
	m.dialog = testDialog(&m)
	before := m.ed.Text()
	_, _ = m.handleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("XYZ")})
	if m.ed.Text() != before {
		t.Fatalf("dialog should block editing; text changed to %q", m.ed.Text())
	}
}

// testInputDialog builds a framed dialog with a text input, mirroring how the
// save-as / new-profile flows open one.
func testInputDialog(m *model, handler dialogHandler) *dialogBox {
	ti := new(textinput.Model)
	*ti = textinput.New()
	ti.Placeholder = "name"
	ti.Focus()
	return m.newInputDialog("Ввод", ti, []dialogButton{
		{label: "Cancel", action: dialogCancel},
		{label: "OK", action: dialogConfirm, defaultBtn: true},
	}, handler)
}

// TestInputDialogRoutesKeys verifies printable and editing keys go into the
// dialog's text input (reaching the committed value), not the editor, and that
// Enter confirms with the typed text.
func TestInputDialogRoutesKeys(t *testing.T) {
	var got string
	var gotAct dialogAction
	m := New(Args{Width: 100, Height: 20}).(model)
	m.dialog = testInputDialog(&m, func(mm *model, act dialogAction, input string) tea.Cmd {
		got, gotAct = input, act
		return nil
	})
	// Printable dir goes into the input; the editor text must not change.
	before := m.ed.Text()
	m2, _ := m.handleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("catalog")})
	if m2.(model).ed.Text() != before {
		t.Fatalf("input dialog must not edit the editor; got %q", m2.(model).ed.Text())
	}
	d := m2.(model).dialog
	if d == nil || d.input == nil || d.input.Value() != "catalog" {
		t.Fatalf("input value=%q want catalog", d.input.Value())
	}
	// Enter confirms with the typed value.
	m3, _ := m2.(model).handleKey(teaKeyEnter())
	if m3.(model).dialog != nil {
		t.Fatalf("dialog should close on Enter")
	}
	if gotAct != dialogConfirm || got != "catalog" {
		t.Fatalf("onConfirm act=%v input=%q want dialogConfirm/catalog", gotAct, got)
	}
}

// TestInputDialogEscCancels verifies Esc cancels the input dialog: the handler
// runs with dialogCancel (so a save/profile closure can skip the action) and the
// input value confirms nothing.
func TestInputDialogEscCancels(t *testing.T) {
	var gotAct dialogAction
	var ran bool
	m := New(Args{Width: 100, Height: 20}).(model)
	m.dialog = testInputDialog(&m, func(mm *model, act dialogAction, input string) tea.Cmd {
		ran, gotAct = true, act
		return nil
	})
	m.dialog.input.SetValue("draft")
	m2, _ := m.handleKey(tea.KeyMsg{Type: tea.KeyEscape})
	if m2.(model).dialog != nil {
		t.Fatalf("dialog should close on Esc")
	}
	if !ran || gotAct != dialogCancel {
		t.Fatalf("handler ran=%v act=%v want ran=true dialogCancel", ran, gotAct)
	}
}

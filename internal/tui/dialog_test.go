package tui

import (
	"strings"
	"testing"

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
	}, func(m *model, act dialogAction) tea.Cmd {
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

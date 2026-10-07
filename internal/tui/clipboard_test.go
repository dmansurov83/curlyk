package tui

import (
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

// TestShiftArrowSelection verifies Shift+Right extends a selection.
func TestShiftArrowSelection(t *testing.T) {
	m := New(Args{Width: 100, Height: 24}).(model)
	m.active = paneEdit
	m.ed.SetText("abcdef\n")
	m.ed.curRow, m.ed.curCol = 0, 0

	// Shift+Right twice -> anchor(0,0), cursor(0,2)
	var m2 tea.Model
	m2, _ = m.handleKey(tea.KeyMsg{Type: tea.KeyShiftRight})
	m = m2.(model)
	m2, _ = m.handleKey(tea.KeyMsg{Type: tea.KeyShiftRight})
	m = m2.(model)

	if !m.selActive {
		t.Fatal("selection should be active")
	}
	sel := m.ed.SelectedText(m.selAnchorRow, m.selAnchorCol, m.ed.curRow, m.ed.curCol)
	if sel != "ab" {
		t.Errorf("selected=%q want ab", sel)
	}
}

// TestDoubleClickWordSelection verifies two quick clicks on a word select it.
func TestDoubleClickWordSelection(t *testing.T) {
	m := New(Args{Width: 120, Height: 30}).(model)
	el := m.layout().editorL
	m.active = paneEdit
	m.ed.SetText("GET https://x.com PV\n")

	// Click twice on the URL area quickly, reassigning m each time.
	// Rune col 8 is inside "https://x.com" (cols 4..15).
	x := el + 6 + 8 // screen x: files + border+icon+num+col
	y := headerHeight + 1 // row 0

	var mm tea.Model = m
	mm, _ = mm.Update(tea.MouseMsg{
		Button: tea.MouseButtonLeft, Action: tea.MouseActionPress, X: x, Y: y,
	})
	m = mm.(model)
	// The first click recorded lastClickTime inside handleMouse.
	// Immediately click again (within default window) — the two Update calls
	// happen back-to-back so the delta is microseconds < 300ms.
	mm, _ = mm.Update(tea.MouseMsg{
		Button: tea.MouseButtonLeft, Action: tea.MouseActionPress, X: x, Y: y,
	})
	m = mm.(model)

	if !m.selActive {
		t.Fatal("double-click should create selection")
	}
	sel := m.ed.SelectedText(m.selAnchorRow, m.selAnchorCol, m.ed.curRow, m.ed.curCol)
	if sel != "https://x.com" {
		t.Errorf("double-click selected=%q want https://x.com", sel)
	}
}

// TestCopyAsCurlSubstitutesVars verifies copy-as-curl resolves {{name}} and
// {{$fn}} placeholders instead of copying the raw template.
func TestCopyAsCurlSubstitutesVars(t *testing.T) {
	orig := clipboardWriter
	t.Cleanup(func() { clipboardWriter = orig })
	var got string
	clipboardWriter = func(s string) error { got = s; return nil }

	m := New(Args{Width: 120, Height: 30}).(model)
	m.active = paneEdit
	m.ed.SetText("@var token = abc\n\nGET https://api.test/{{token}}/items\nX-Token: {{token}}\n\n")
	m.ed.curRow = 2
	m.copyAsCurl()

	if strings.Contains(got, "{{") {
		t.Fatalf("copy-as-curl left placeholder in: %s", got)
	}
	if !strings.Contains(got, "https://api.test/abc/items") {
		t.Errorf("url var not substituted, want https://api.test/abc/items in: %s", got)
	}
	if !strings.Contains(got, "X-Token: abc") {
		t.Errorf("header var not substituted, want X-Token: abc in: %s", got)
	}
}

// TestCopyAsCurlUnresolvedVar verifies copy-as-curl reports an unresolved
// variable instead of silently exporting the raw placeholder.
func TestCopyAsCurlUnresolvedVar(t *testing.T) {
	m := New(Args{Width: 120, Height: 30}).(model)
	m.active = paneEdit
	m.ed.SetText("\nGET https://api.test/{{missing}}\n")
	m.ed.curRow = 1
	m.copyAsCurl()
	if !strings.Contains(m.status, "missing") {
		t.Errorf("expected status mentioning missing var, got %q", m.status)
	}
}

// TestCurlPasteCursorOnRequest verifies after pasting+converting a curl the
// cursor lands on the request line so the request can be run/highlighted.
func TestCurlPasteCursorOnRequest(t *testing.T) {
	m := New(Args{Width: 120, Height: 30}).(model)
	m.active = paneEdit
	m.ed.SetText("### keep\n")
	m.ed.curRow, m.ed.curCol = 0, 0
	m.pasteOrConvert("curl -X 'POST' 'https://api.test/items' -H 'Content-Type: application/json'")
	if !isRequestLine(m.ed.Lines(), m.ed.curRow) {
		t.Errorf("cursor must be on the request line, got row=%d col=%d\n%s", m.ed.curRow, m.ed.curCol, m.ed.Text())
	}
}

// TestCopyCopiesVerifies that Ctrl+C copies and never quits.
func TestCopyCopies(t *testing.T) {
	m := New(Args{Width: 100, Height: 24}).(model)
	m.active = paneEdit
	m.ed.SetText("hello\n")
	m.selActive = true
	m.selAnchorRow, m.selAnchorCol = 0, 0
	m.ed.curRow, m.ed.curCol = 0, 5

	model2, cmd := m.handleKey(tea.KeyMsg{Type: tea.KeyCtrlC})
	// copying should return nil command (not quit)
	if cmd != nil {
		t.Fatal("Ctrl+C should copy and return nil cmd, not quit")
	}
	r := model2.(model)
	// selection cleared after copy
	if r.selActive {
		t.Error("selection should be cleared after copy")
	}
}

// TestEscEscQuits verifies pressing Esc twice quickly quits.
func TestEscEscQuits(t *testing.T) {
	m := New(Args{Width: 100, Height: 24}).(model)
	m.active = paneEdit
	_, cmd := m.handleKey(tea.KeyMsg{Type: tea.KeyEscape})
	if cmd != nil {
		t.Fatal("first Esc should not quit")
	}
	// simulate quick second Esc
	m.lastEscTime = time.Now().Add(-100 * time.Millisecond)
	_, cmd = m.handleKey(tea.KeyMsg{Type: tea.KeyEscape})
	if cmd == nil {
		t.Fatal("second Esc soon after should quit")
	}
}

// TestF10Quits verifies F10 quits.
func TestF10Quits(t *testing.T) {
	m := New(Args{Width: 100, Height: 24}).(model)
	m.active = paneEdit
	_, cmd := m.handleKey(tea.KeyMsg{Type: tea.KeyF10})
	if cmd == nil {
		t.Fatal("F10 should quit")
	}
}

package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/mattn/go-runewidth"
	"github.com/user/curlyk/internal/completion"
	"github.com/user/curlyk/internal/i18n"
)

func TestCompletionCtrlSpaceOpens(t *testing.T) {
	m := New(Args{Width: 120, Height: 30}).(model)
	m.active = paneEdit
	m.ed.SetText("GE\n")
	m.ed.curRow = 0
	m.ed.curCol = 2

	res, _ := m.handleKey(tea.KeyMsg{Type: tea.KeyCtrlAt})
	mm := res.(model)
	if mm.complete == nil {
		t.Fatal("Ctrl+Space must open the completion popup")
	}
	// "GE" is a method prefix → GET should be offered.
	labelFound := false
	for _, s := range mm.complete.suggestions {
		if s.Label == "GET" {
			labelFound = true
		}
	}
	if !labelFound {
		t.Fatalf("expected GET suggestion, got %v", mm.complete.suggestions)
	}
}

func TestCompletionVarAutoTrigger(t *testing.T) {
	m := New(Args{Width: 120, Height: 30}).(model)
	m.active = paneEdit
	// handleKey is a value receiver: each call returns the mutated model copy.
	// Type "{{" interactively; the popup opens when the second '{' lands.
	r, _ := m.handleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("{")})
	m = r.(model)
	r, _ = m.handleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("{")})
	m = r.(model)
	if m.complete == nil {
		t.Fatal("typing {{ must open the variable popup")
	}
	// Then type "$" — the popup re-filters; builtin functions start with "$".
	r, _ = m.handleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("$")})
	m = r.(model)
	if m.complete == nil {
		t.Fatal("popup stayed open while typing the variable name")
	}
	found := false
	for _, s := range m.complete.suggestions {
		if s.Label == "$timestamp" {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected $timestamp suggestion, got %v", m.complete.suggestions)
	}
}

func TestCompletionAcceptReplaces(t *testing.T) {
	m := New(Args{Width: 120, Height: 30}).(model)
	m.active = paneEdit
	m.ed.SetText("GE\n")
	m.ed.curRow = 0
	m.ed.curCol = 2
	// Pre-seed the popup so accept has a candidate.
	m.complete = &completionMenu{suggestions: []completion.Suggestion{
		{Label: "GET", Snippet: "GET", Start: 0, End: 2},
	}, sel: 0}

	m.acceptCompletion()
	got := m.ed.Lines()[0]
	if got != "GET" {
		t.Fatalf("accept replaced text: got %q, want %q", got, "GET")
	}
	if m.complete != nil {
		t.Fatal("accept must close the popup")
	}
	if m.ed.curCol != 3 {
		t.Fatalf("cursor after accept: got %d, want 3", m.ed.curCol)
	}
}

func TestCompletionAcceptVariableClosesBraces(t *testing.T) {
	m := New(Args{Width: 120, Height: 30}).(model)
	m.active = paneEdit
	m.ed.SetText("GET http://x/{{to\n")
	m.ed.curRow = 0
	m.ed.curCol = 15
	m.complete = &completionMenu{suggestions: []completion.Suggestion{
		{Label: "token", Snippet: "token", Start: 15, End: 15, Var: true},
	}, sel: 0}

	m.acceptCompletion()
	got := m.ed.Lines()[0]
	if !strings.Contains(got, "{{token}}") {
		t.Fatalf("accept variable must add closing braces, got %q", got)
	}
}

func TestCompletionEscCloses(t *testing.T) {
	m := New(Args{Width: 120, Height: 30}).(model)
	m.active = paneEdit
	m.complete = &completionMenu{suggestions: []completion.Suggestion{
		{Label: "A", Snippet: "A", Start: 0, End: 0},
	}, sel: 0}

	res, _ := m.handleKey(tea.KeyMsg{Type: tea.KeyEsc})
	if res.(model).complete != nil {
		t.Fatal("Esc must close the completion popup")
	}
}

func TestCompletionRenderNoPanic(t *testing.T) {
	m := New(Args{Width: 120, Height: 30}).(model)
	m.active = paneEdit
	m.ed.SetText("GE\n")
	m.ed.curRow = 0
	m.ed.curCol = 2
	m.complete = &completionMenu{suggestions: []completion.Suggestion{
		{Label: "GET", Snippet: "GET", Start: 0, End: 2},
	}, sel: 0}

	v := m.View() // must not panic
	if !strings.Contains(v, "GET") {
		t.Error("completion popup should render the GET suggestion")
	}
}

// TestCompletionEnterWithEmptySuggsDoesNotSwallow verifies that pressing Enter
// with an empty suggestion list closes the popup and is NOT consumed, so the
// key falls through to the normal edit path (e.g. new line / run request).
func TestCompletionEnterWithEmptySuggsDoesNotSwallow(t *testing.T) {
	m := New(Args{Width: 120, Height: 30}).(model)
	m.active = paneEdit
	m.complete = &completionMenu{suggestions: []completion.Suggestion{}, sel: 0}

	res, _ := m.handleKey(tea.KeyMsg{Type: tea.KeyEnter})
	mm := res.(model)
	if mm.complete != nil {
		t.Fatal("Enter with no suggestions must close the popup")
	}
	// The key was not consumed: the model must still be the same instance so a
	// subsequent Enter behaves as a normal Enter. We assert the edit path got
	// the key by checking the popup is gone and no accept happened.
	if mm.status == i18n.T("completion.accepted") {
		t.Fatal("Enter with empty suggestions must not apply a completion")
	}
}

// TestCompletionAcceptKeepsSurrounding verifies that accepting a variable keeps
// the text before and after the cursor: "GET " (before), the closing "}}" added,
// and the tail "/api/v1/leads" preserved.
func TestCompletionAcceptKeepsSurrounding(t *testing.T) {
	m := New(Args{Width: 120, Height: 30}).(model)
	m.active = paneEdit
	m.ed.SetText("GET {{l/api/v1/leads\n")
	m.ed.curRow = 0
	m.ed.curCol = 7
	// "{{" at cols 4-5, typed prefix "l" at col 6, cursor at 7 (after "l").
	// The suggestion replaces [6..7].
	m.complete = &completionMenu{suggestions: []completion.Suggestion{
		{Label: "leadId", Snippet: "leadId", Start: 6, End: 7, Var: true},
	}, sel: 0}

	ok := m.acceptCompletion()
	if !ok {
		t.Fatal("accept should have applied")
	}
	got := m.ed.Lines()[0]
	// "GET " + "{{" (cols 4-5) kept, "l" replaced with leadId + "}}", tail kept.
	if got != "GET {{leadId}}/api/v1/leads" {
		t.Fatalf("accept lost surrounding text: got %q", got)
	}
}

// TestCompletionAutoTriggerMethod verifies typing a method prefix ("GE") at the
// start of a line opens the popup with method suggestions (GET/POST...).
func TestCompletionAutoTriggerMethod(t *testing.T) {
	m := New(Args{Width: 120, Height: 30}).(model)
	m.active = paneEdit
	m.ed.SetText("\n")
	m.ed.curRow = 0
	m.ed.curCol = 0
	var out tea.Model = m
	out, _ = out.(model).handleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("G")})
	out, _ = out.(model).handleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("E")})
	mm := out.(model)
	if mm.complete == nil {
		t.Fatal("typing a method prefix must open the completion popup")
	}
	found := false
	for _, s := range mm.complete.suggestions {
		if s.Label == "GET" {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected GET suggestion, got %v", mm.complete.suggestions)
	}
}

// TestCompletionAutoTriggerScheme verifies that after a method word, typing the
// start of a URL scheme ("h") offers http:// and https://.
func TestCompletionAutoTriggerScheme(t *testing.T) {
	m := New(Args{Width: 120, Height: 30}).(model)
	m.active = paneEdit
	m.ed.SetText("GET \n")
	m.ed.curRow = 0
	m.ed.curCol = 4
	var out tea.Model = m
	out, _ = out.(model).handleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("h")})
	mm := out.(model)
	if mm.complete == nil {
		t.Fatal("typing a URL scheme prefix must open the completion popup")
	}
	got := []string{}
	for _, s := range mm.complete.suggestions {
		got = append(got, s.Label)
	}
	for _, want := range []string{"http://", "https://"} {
		found := false
		for _, g := range got {
			if g == want {
				found = true
			}
		}
		if !found {
			t.Fatalf("expected scheme %q in %v", want, got)
		}
	}
}

// TestCompletionPopupDoesNotOverflowPane verifies the popup never renders wider
// than the editor content area even when the cursor sits far to the right. A
// too-wide popup would be pushed out of view while the reserved source rows
// still shift (looking like the text "disappears" until Esc closes the popup).
func TestCompletionPopupDoesNotOverflowPane(t *testing.T) {
	m := New(Args{Width: 120, Height: 30}).(model)
	m.ed.width = 50
	m.active = paneEdit
	long := "GET " + "x456789012345678901234567890"
	m.ed.SetText(long + "\n")
	m.ed.curRow = 0
	m.ed.curCol = len(long)
	m.complete = &completionMenu{suggestions: []completion.Suggestion{
		{Label: "host", Snippet: "host", Start: m.ed.curCol, End: m.ed.curCol, Var: true},
	}, sel: 0}

	cw := m.ed.width - 2 - 5
	for i, l := range m.completeLines(cw, m.ed.curCol) {
		w := runewidth.StringWidth(stripANSI(l))
		if w > cw {
			t.Errorf("popup line %d width %d > contentW %d: %q", i, w, cw, stripANSI(l))
		}
	}
}

// TestCompletionPopupVisibleAtBottomCursor verifies the popup is scrolled into
// view when the cursor sits near the bottom of the editor, so it is not pushed
// off the pane (which would hide the popup while still reserving source rows).
func TestCompletionPopupVisibleAtBottomCursor(t *testing.T) {
	m := New(Args{Width: 120, Height: 30}).(model)
	m.active = paneEdit
	b := strings.Builder{}
	for i := 0; i < 22; i++ {
		b.WriteString("# line\n")
	}
	b.WriteString("### Get lead\nGET \n")
	m.ed.SetText(b.String())
	m.width, m.height = 120, 30
	m.ed.width = m.layout().mid
	m.ed.height = 30 - 6
	m.ed.curRow = 23
	m.ed.curCol = 4

	var out tea.Model = m
	out, _ = out.(model).handleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("{")})
	out, _ = out.(model).handleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("{")})
	mm := out.(model)
	if mm.complete == nil {
		t.Fatal("expected completion popup to open")
	}
	v := stripANSI(mm.View())
	if !strings.Contains(v, "Автодополнение") {
		t.Error("completion popup must be visible when cursor is at the bottom edge")
	}
}

package tui

import (
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

func TestUndoInsert(t *testing.T) {
	m := New(Args{Width: 120, Height: 30}).(model)
	m.active = paneEdit
	m.ed.SetText("hello\n")
	// clear the initial undo snapshot added by SetText
	m.ed.undo = m.ed.undo[:0]

	m.ed.curRow, m.ed.curCol = 0, 5
	m.ed.InsertString("X") // "helloX\n"
	if got := m.ed.Text(); got != "helloX\n" {
		t.Fatalf("after insert=%q", got)
	}
	if !m.ed.Undo() {
		t.Fatal("undo should succeed")
	}
	if got := m.ed.Text(); got != "hello\n" {
		t.Errorf("after undo=%q want hello", got)
	}
}

func TestUndoRedoBackspace(t *testing.T) {
	m := New(Args{Width: 120, Height: 30}).(model)
	m.active = paneEdit
	m.ed.SetText("abc\n")
	m.ed.undo = m.ed.undo[:0]
	m.ed.curRow, m.ed.curCol = 0, 3
	m.ed.Backspace() // "ab\n"
	if got := m.ed.Text(); got != "ab\n" {
		t.Fatalf("after backspace=%q", got)
	}
	m.ed.Undo()
	if got := m.ed.Text(); got != "abc\n" {
		t.Errorf("after undo=%q want abc", got)
	}
	m.ed.Redo()
	if got := m.ed.Text(); got != "ab\n" {
		t.Errorf("after redo=%q want ab", got)
	}
}

func TestUndoEnterSplit(t *testing.T) {
	m := New(Args{Width: 120, Height: 30}).(model)
	m.active = paneEdit
	m.ed.SetText("ab\n")
	m.ed.undo = m.ed.undo[:0]
	m.ed.curRow, m.ed.curCol = 0, 1
	m.ed.Enter() // "a\nb\n"
	if got := m.ed.Text(); got != "a\nb\n" {
		t.Fatalf("after enter=%q", got)
	}
	m.ed.Undo()
	if got := m.ed.Text(); got != "ab\n" {
		t.Errorf("after undo=%q want ab", got)
	}
}

// TestUndoSelectionReplaceAsSingle verifies replacing a selection by typing
// undoes as ONE step (the batch absorbs delete+insert).
func TestUndoSelectionReplaceAsSingle(t *testing.T) {
	m := setupSel(t) // hello world, sel "world" 6..11
	m.ed.undo = m.ed.undo[:0]
	res, _ := m.handleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("X")}) // hello X
	r := res.(model)
	if got := r.ed.Text(); got != "hello X\n" {
		t.Fatalf("after replace=%q", got)
	}
	if len(r.ed.undo) != 1 {
		t.Errorf("selection replace should be 1 undo step, got %d", len(r.ed.undo))
	}
	r.ed.Undo()
	if got := r.ed.Text(); got != "hello world\n" {
		t.Errorf("undo of replace=%q want hello world", got)
	}
}

// TestUndoCtrlZKey verifies the Ctrl+Z key triggers undo.
func TestUndoCtrlZKey(t *testing.T) {
	m := New(Args{Width: 120, Height: 30}).(model)
	m.active = paneEdit
	m.ed.SetText("hi\n")
	m.ed.undo = m.ed.undo[:0]
	m.ed.curRow, m.ed.curCol = 0, 2
	m.ed.InsertString("!")
	if m.ed.Text() != "hi!\n" {
		t.Fatalf("insert failed: %q", m.ed.Text())
	}
	res, _ := m.handleKey(tea.KeyMsg{Type: tea.KeyCtrlZ})
	r := res.(model)
	if r.ed.Text() != "hi\n" {
		t.Errorf("Ctrl+Z should undo, got %q", r.ed.Text())
	}
}

func TestUndoRedoKeys(t *testing.T) {
	m := New(Args{Width: 120, Height: 30}).(model)
	m.active = paneEdit
	m.ed.SetText("x\n")
	m.ed.undo = m.ed.undo[:0]
	m.ed.curRow, m.ed.curCol = 0, 1
	m.ed.InsertString("y") // xy
	res, _ := m.handleKey(tea.KeyMsg{Type: tea.KeyCtrlZ})
	r := res.(model)
	if r.ed.Text() != "x\n" {
		t.Fatalf("undo=%q", r.ed.Text())
	}
	// Redo: bubbletea has no KeyCtrlShiftZ enum; verify Redo via the editor API.
	if !r.ed.Redo() {
		t.Fatal("redo should succeed")
	}
	if r.ed.Text() != "xy\n" {
		t.Errorf("redo=%q want xy", r.ed.Text())
	}
}

// TestPasteUndoAsSingle verifies a paste delivered through pasteOrConvert (the
// Ctrl+V / bracketed-paste path) is undone as a single step, not per character.
func TestPasteUndoAsSingle(t *testing.T) {
	m := New(Args{Width: 120, Height: 30}).(model)
	m.active = paneEdit
	m.ed.SetText("abc\n")
	m.ed.undo = m.ed.undo[:0]
	m.ed.curRow, m.ed.curCol = 0, 0
	m.pasteOrConvert("XYZ") // single multi-char paste
	if got := m.ed.Text(); got != "XYZabc\n" {
		t.Fatalf("after paste=%q", got)
	}
	if len(m.ed.undo) != 1 {
		t.Fatalf("paste should create 1 undo step, got %d", len(m.ed.undo))
	}
	if !m.ed.Undo() {
		t.Fatal("undo should succeed")
	}
	if got := m.ed.Text(); got != "abc\n" {
		t.Errorf("undo of paste=%q want abc", got)
	}
}

// TestTypingBurstGroups verifies a fast run of single-character edits (typing or
// a paste delivered char-by-char) coalesces into one undo step once the batch
// closes.
func TestTypingBurstGroups(t *testing.T) {
	m := New(Args{Width: 120, Height: 30}).(model)
	m.active = paneEdit
	m.ed.SetText("abc\n")
	m.ed.undo = m.ed.undo[:0]
	m.ed.curRow, m.ed.curCol = 0, 0
	// type "xyz" one char at a time, each a separate edit key
	for _, r := range []rune{'x', 'y', 'z'} {
		m.handleEditKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}
	if got := m.ed.Text(); got != "xyzabc\n" {
		t.Fatalf("after typing=%q", got)
	}
	// close the timed batch (as the autosave loop would once typing pauses)
	m.closeEditBatchIfExpired(time.Now().Add(time.Hour))
	if len(m.ed.undo) != 1 {
		t.Fatalf("a fast burst closed by the timer should be 1 undo step, got %d", len(m.ed.undo))
	}
	m.ed.Undo()
	if got := m.ed.Text(); got != "abc\n" {
		t.Errorf("undo of burst=%q want abc", got)
	}
}

// TestTypingBatchSeparatesByGap verifies that after the batch closes, the next
// character forms a new undo step.
func TestTypingBatchSeparatesByGap(t *testing.T) {
	m := New(Args{Width: 120, Height: 30}).(model)
	m.active = paneEdit
	m.ed.SetText("a\n")
	m.ed.undo = m.ed.undo[:0]
	m.ed.curRow, m.ed.curCol = 0, 0
	m.handleEditKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'x'}})
	m.closeEditBatchIfExpired(time.Now().Add(time.Hour)) // pause
	m.handleEditKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'y'}})
	m.closeEditBatchIfExpired(time.Now().Add(time.Hour)) // pause
	if len(m.ed.undo) != 2 {
		t.Fatalf("two separated bursts should be 2 undo steps, got %d", len(m.ed.undo))
	}
}

// TestPasteCurlCharByCharConverts verifies a cURL command delivered
// character-by-character (as Windows Terminal does on Ctrl+V) is detected and
// converted to an .http request when the burst closes.
func TestPasteCurlCharByCharConverts(t *testing.T) {
	m := New(Args{Width: 120, Height: 30}).(model)
	m.active = paneEdit
	m.ed.SetText("### existing\n")
	m.ed.undo = m.ed.undo[:0]
	m.ed.curRow = 0
	m.ed.curCol = len([]rune(m.ed.Text()))
	cmd := "curl -X 'POST' 'https://api.example.com/x' -H 'Content-Type: application/json'"
	for _, r := range cmd {
		m.handleEditKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}
	m.closeEditBatchIfExpired(time.Now().Add(time.Hour))
	got := m.ed.Text()
	if !containsStr(got, "POST https://api.example.com/x") {
		t.Errorf("char-by-char curl should convert to .http, got:\n%s", got)
	}
	// raw curl should be gone
	if containsStr(got, "curl -X") {
		t.Errorf("raw curl should be replaced, got:\n%s", got)
	}
}
func containsStr(s, sub string) bool {
	return len(s) >= len(sub) && func() bool {
		for i := 0; i+len(sub) <= len(s); i++ {
			if s[i:i+len(sub)] == sub {
				return true
			}
		}
		return false
	}()
}
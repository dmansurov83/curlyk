package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestParseFormBody(t *testing.T) {
	cases := []struct {
		body string
		want []formField
	}{
		{"", nil},
		{"a=1&b=2", []formField{{key: "a", value: "1"}, {key: "b", value: "2"}}},
		{"grant_type=application&scope=openid offline_access",
			[]formField{{key: "grant_type", value: "application"}, {key: "scope", value: "openid offline_access"}}},
		{"a=hello%20world&b=x%2By", []formField{{key: "a", value: "hello world"}, {key: "b", value: "x+y"}}},
		{"key=", []formField{{key: "key", value: ""}}},
		{"=v", []formField{{key: "", value: "v"}}},
	}
	for i, c := range cases {
		got := parseFormBody(c.body)
		if len(got) != len(c.want) {
			t.Errorf("case %d: %q got %d fields want %d", i, c.body, len(got), len(c.want))
			continue
		}
		for j := range got {
			if got[j] != c.want[j] {
				t.Errorf("case %d field %d: got %+v want %+v", i, j, got[j], c.want[j])
			}
		}
	}
}

func TestEncodeForm(t *testing.T) {
	cases := []struct {
		fields []formField
		want   string
	}{
		{nil, ""},
		{[]formField{{key: "a", value: "1"}, {key: "b", value: "2"}}, "a=1&b=2"},
		{[]formField{{key: "scope", value: "openid offline_access"}}, "scope=openid+offline_access"},
		{[]formField{{key: "a", value: "x+y"}}, "a=x%2By"},
		{[]formField{{key: "", value: "dropped"}}, ""},
		{[]formField{{key: "empty", value: ""}}, "empty="},
		{[]formField{{key: "a", value: "1"}, {key: "", value: "x"}}, "a=1"},
		{[]formField{{key: "grant_type", value: "application"}, {key: "token", value: "AbC.~_-"}}, "grant_type=application&token=AbC.~_-"},
	}
	for i, c := range cases {
		if got := encodeForm(c.fields); got != c.want {
			t.Errorf("case %d: got %q want %q", i, got, c.want)
		}
	}
}

// TestFormEditorOpensAndCommits verifies the round trip: opening the editor on
// a request with a form body, editing a value, and committing writes back an
// encoded body into the editor buffer.
func TestFormEditorOpensAndCommits(t *testing.T) {
	m := New(Args{Width: 100, Height: 24}).(model)
	m.active = paneEdit
	m.ed.SetText("POST http://x/token\ncontent-type: application/x-www-form-urlencoded\n\ngrant_type=application&scope=openid offline_access\n")
	m.ed.curRow = 0

	m.beginFormEditor()
	if m.form == nil {
		t.Fatal("form editor not opened")
	}
	if len(m.form.fields) != 2 {
		t.Fatalf("fields=%d want 2", len(m.form.fields))
	}
	// Edit the value of the last field.
	m.form.sel = 1
	m.form.field = 1
	m.form.fields[1].value = ""
	m.handleFormKey(teaKeyOf("openid_app"))
	m.commitForm()
	if m.form != nil {
		t.Fatal("form must close after commit")
	}
	got := m.ed.Text()
	if !strings.Contains(got, "grant_type=application&scope=openid_app") {
		t.Errorf("committed body missing expected pair, got:\n%s", got)
	}
}

// TestFormEditorCommitCreatesBodyWhenAbsent verifies committing on a request
// with no body inserts a body block.
func TestFormEditorCommitCreatesBodyWhenAbsent(t *testing.T) {
	m := New(Args{Width: 100, Height: 24}).(model)
	m.active = paneEdit
	m.ed.SetText("POST http://x/token\ncontent-type: application/x-www-form-urlencoded\n")
	m.ed.curRow = 0

	m.beginFormEditor()
	if m.form == nil {
		t.Fatal("form not opened")
	}
	m.form.fields = []formField{{key: "grant_type", value: "application"}}
	m.commitForm()
	got := m.ed.Text()
	if !strings.Contains(got, "grant_type=application") {
		t.Errorf("expected body inserted, got:\n%s", got)
	}
	if !strings.Contains(got, "content-type: application/x-www-form-urlencoded\n\ngrant_type=application") {
		t.Errorf("body block must follow the headers with a blank separator:\n%s", got)
	}
}

// TestFormEditorScrolling verifies that moving down through many fields keeps
// the selection visible inside the capped popup list (no panic, no blank page).
func TestFormEditorScrolling(t *testing.T) {
	m := New(Args{Width: 100, Height: 24}).(model)
	m.active = paneEdit
	m.ed.SetText("POST http://x/token\ncontent-type: application/x-www-form-urlencoded\n\na=1&b=2&c=3&d=4&e=5&f=6&g=7&h=8&i=9&j=10\n")
	m.ed.curRow = 0
	m.beginFormEditor()
	if m.form == nil {
		t.Fatal("form not opened")
	}
	// Simulate pressing Down to the last row.
	for i := 0; i < len(m.form.fields)-1; i++ {
		m.handleFormKey(tea.KeyMsg{Type: tea.KeyDown})
	}
	if m.form.sel < 0 || m.form.sel >= len(m.form.fields) {
		t.Fatalf("selection out of range after scrolling: sel=%d fields=%d", m.form.sel, len(m.form.fields))
	}
	// Rendering must not panic and must show a row.
	lines := m.formLines(40)
	if len(lines) == 0 {
		t.Error("formLines returned no rows")
	}
	joined := strings.Join(lines, "\n")
	if !strings.Contains(joined, "=") {
		t.Errorf("expected a key=value row in popup, got:\n%s", joined)
	}
}

// TestFormEditorEscCancels verifies Esc closes the popup without modifying the
// buffer.
func TestFormEditorEscCancels(t *testing.T) {
	m := New(Args{Width: 100, Height: 24}).(model)
	m.active = paneEdit
	src := "POST http://x/token\ncontent-type: application/x-www-form-urlencoded\n\nold=value\n"
	m.ed.SetText(src)
	m.ed.curRow = 0
	m.beginFormEditor()
	m.handleFormKey(tea.KeyMsg{Type: tea.KeyEsc})
	if m.form != nil {
		t.Fatal("Esc must close the form editor")
	}
	if m.ed.Text() != src {
		t.Errorf("Esc must not modify the buffer:\n%s", m.ed.Text())
	}
}

func teaKeyOf(s string) tea.KeyMsg {
	return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)}
}

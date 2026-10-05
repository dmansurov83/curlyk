package tui

import (
	"strings"
	"testing"

	"github.com/user/curlyk/internal/i18n"
	"github.com/user/curlyk/internal/theme"
)

// TestEditorJSONBodyColored verifies that a JSON request body is rendered with
// JSON syntax colours (distinct per token kind) instead of the generic faint
// body style.
func TestEditorJSONBodyColored(t *testing.T) {
	i18n.SetLocale("ru")
	applyTheme(theme.DefaultName)

	m := New(Args{Width: 120, Height: 30}).(model)
	m.active = paneEdit
	// Request line + blank line ends headers, then an indented JSON body.
	m.ed.SetText("POST /x HTTP/1.1\nContent-Type: application/json\n\n{\n  \"name\": \"a\",\n  \"count\": 3,\n  \"ok\": true,\n  \"nil\": null\n}\n")

	out := m.renderEditor(120)
	plain := stripANSI(out)

	// Sanity: the body text is present in the plain output.
	for _, want := range []string{`"name"`, `"a"`, "3", "true", "null"} {
		if !strings.Contains(plain, want) {
			t.Errorf("body text %q missing from editor render:\n%s", want, plain)
		}
	}

	// A JSON body must NOT be painted with the faint body style on the key line.
	if strings.Contains(out, "38;5;186") {
		t.Errorf("JSON body wrongly rendered with body style 186:\n%q", out)
	}

	// Expect the JSON key colour (81) on the "name" key line, and the string
	// colour (114) for the value.
	if !strings.Contains(out, "38;5;81") {
		t.Errorf("expected JSON key colour 81 in editor render:\n%q", out)
	}
	if !strings.Contains(out, "38;5;114") {
		t.Errorf("expected JSON string colour 114 in editor render:\n%q", out)
	}
}

// TestEditorJSONBodyNoColorForNonJSONLine verifies a non-JSON body line keeps the
// generic faint body style (no JSON tokens applied).
func TestEditorJSONBodyNoColorForNonJSONLine(t *testing.T) {
	i18n.SetLocale("ru")
	applyTheme(theme.DefaultName)

	m := New(Args{Width: 120, Height: 30}).(model)
	m.active = paneEdit
	m.ed.SetText("POST /x HTTP/1.1\n\nhello plain body\n")

	out := m.renderEditor(120)
	if !strings.Contains(out, "38;5;186") {
		t.Errorf("expected faint body style for non-JSON body, got:\n%q", out)
	}
}

// TestEditorJSONBodyColoredAcrossCursor verifies JSON coloring is NOT lost on the
// part of a JSON body line after the block cursor. The cursor splits the line;
// both halves must keep their JSON token colors rather than falling back to the
// faint body style (body colour 186).
func TestEditorJSONBodyColoredAcrossCursor(t *testing.T) {
	i18n.SetLocale("ru")
	applyTheme(theme.DefaultName)

	m := New(Args{Width: 120, Height: 30}).(model)
	m.active = paneEdit
	// Body line: {"name":"a","count":3}
	m.ed.SetText("POST /x HTTP/1.1\n\n{\"name\":\"a\",\"count\":3}\n")
	m.ed.curRow = 2
	m.ed.curCol = 2 // cursor on 'n' of "name": splits the line

	out := m.renderEditor(120)

	// The line still shows JSON key color (81) on both halves and never the faint
	// body style (186).
	if !strings.Contains(out, "38;5;81") {
		t.Errorf("expected JSON key colour 81 with cursor present, got:\n%q", out)
	}
	if strings.Contains(out, "38;5;186") {
		t.Errorf("faint body style leaked into a JSON line with a cursor, got:\n%q", out)
	}
}

// TestEditorJSONBodyColoredAcrossSelection verifies JSON coloring is preserved on
// a JSON body line that is partly covered by a selection. The selected span keeps
// the JSON foreground merged with the selection background everywhere, not just
// where the span happens to begin at a token boundary.
func TestEditorJSONBodyColoredAcrossSelection(t *testing.T) {
	i18n.SetLocale("ru")
	applyTheme(theme.DefaultName)

	m := New(Args{Width: 120, Height: 30}).(model)
	m.active = paneEdit
	m.ed.SetText("POST /x HTTP/1.1\n\n{\"name\":\"a\",\"count\":3}\n")
	// Put the cursor away from the body line so it does not interfere.
	m.ed.curRow = 0
	// Select the whole body line (row 2) from col 0 to col 6.
	m.selActive = true
	m.selAnchorRow, m.selAnchorCol = 2, 0
	m.ed.curRow, m.ed.curCol = 2, 6

	out := m.renderEditor(120)

	// The whole "name" key selector span should be painted with the selection
	// background (48;5;24) merged with the JSON key foreground (38;5;81), even
	// though the selection starts mid-key after the '{'.
	if !strings.Contains(out, "48;5;24;38;5;81") && !strings.Contains(out, "38;5;81;48;5;24") {
		t.Errorf("expected selection background merged with JSON key colour on selected key, got:\n%q", out)
	}
	if strings.Contains(out, "38;5;186") {
		t.Errorf("faint body style leaked into a selected JSON line, got:\n%q", out)
	}
}
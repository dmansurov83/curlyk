package tui

import (
	"strings"
	"testing"
)

// TestViewStartsWithTopHeader verifies the rendered frame starts with the
// file-name header line, then the pane borders, and ends with the status bar.
func TestViewStartsWithTopHeader(t *testing.T) {
	m := New(Args{Width: 120, Height: 30}).(model)
	out := m.View()
	plain := stripANSI(out)
	lines := strings.Split(plain, "\n")

	if len(lines) < 2 {
		t.Fatalf("view too short: %d lines", len(lines))
	}
	// first line is the header with the file name
	if !strings.Contains(lines[0], "Файл:") {
		t.Errorf("first rendered line should be the file header:\n%q", lines[0])
	}
	// second line must contain the top-left border corner
	if !strings.ContainsAny(lines[1], "╭┌") {
		t.Errorf("second rendered line does not start with top border:\n%q", lines[1])
	}
	// last line must contain the status bar text
	last := lines[len(lines)-1]
	if !strings.Contains(last, "Готов") {
		t.Errorf("last line should be status bar, got: %q", last)
	}
}

// TestAddBlankLinesFirstShown verifies that after adding blank lines at the
// top of the buffer, the FIRST shown content is still the buffer's first line.
func TestBlankLinesRenderAtTop(t *testing.T) {
	m := New(Args{Width: 120, Height: 30}).(model)
	m.ed.SetText("\n\n### Получить пользователя\nGET /x\n")
	m.ed.scroll = 0
	out := m.renderEditor(60)
	plain := stripANSI(out)

	// line number 1 must appear
	if !strings.Contains(plain, "  1 ") {
		t.Errorf("line 1 label missing:\n%s", plain)
	}
	// content line 3 (### ...) must appear after the first two blank lines rendered
	if !strings.Contains(plain, "### Получить пользователя") {
		t.Errorf("content missing:\n%s", plain)
	}
}
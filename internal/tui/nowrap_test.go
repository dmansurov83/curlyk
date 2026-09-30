package tui

import (
	"strings"
	"testing"
)

// TestNoWrapLongLine verifies a long editor line is truncated, not wrapped
// (issue: wrapping broke vertical scrolling).
func TestNoWrapLongLine(t *testing.T) {
	m := New(Args{Width: 60, Height: 12}).(model) // half=30
	m.ed.SetText("short\n" + strings.Repeat("A", 200) + "\n")
	m.ed.scroll = 0
	out := m.renderEditor(30)
	plain := stripANSI(out)
	lines := strings.Split(plain, "\n")
	// Find the line with 'A's — it must be a single physical line, not wrapped.
	for i, ln := range lines {
		if strings.Contains(ln, "AAAA") {
			// count how many of the following "lines" are pure continuation (wrap)
			_ = i
		}
	}
	// Total physical lines must equal the 3 logical lines (plus nothing):
	// "short", the long A-line, and nothing else.
	if len(lines) > 3 {
		t.Errorf("long line was wrapped: got %d physical lines, want <=3:\n%q", len(lines), firstPart(lines))
	}
}

func firstPart(lines []string) string {
	if len(lines) > 3 {
		return strings.Join(lines[:3], "|")
	}
	return strings.Join(lines, "|")
}

// TestTruncateWidth verifies truncation by display width.
func TestTruncateWidth(t *testing.T) {
	if got := truncateWidth("abcdef", 3); got != "abc" {
		t.Errorf("got %q", got)
	}
	// Cyrillic is width 1 under default runewidth; all 4 fit in max 4.
	if got := truncateWidth("тест", 4); got != "тест" {
		t.Errorf("cyrillic truncate got %q want тест", got)
	}
	if got := truncateWidth("тест", 3); got != "тес" {
		t.Errorf("cyrillic truncate max3 got %q want тес", got)
	}
}
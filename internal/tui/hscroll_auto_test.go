package tui

import (
	"strings"
	"testing"

	"github.com/mattn/go-runewidth"
)

// TestHScrollToCursorScrollsWideLine verifies EnsureVisible auto-scrolls the
// horizontal offset so the cursor becomes visible on a long line.
func TestHScrollToCursorScrollsWideLine(t *testing.T) {
	e := newEditor("", 120, 30)
	e.width = 60
	// line of many 'a' chars, each 1 cell wide.
	long := ""
	for i := 0; i < 200; i++ {
		long += "a"
	}
	e.lines = []string{long, ""}
	// cursor near the end of the long line.
	e.curRow, e.curCol = 0, 190
	e.hScroll = 0
	e.EnsureVisible()
	cw := e.contentWidth()
	// hScroll should have advanced so that the cursor (cell 190) is visible
	// within [hScroll, hScroll+cw].
	if e.hScroll == 0 {
		t.Errorf("expected hScroll to advance, got 0")
	}
	if !(e.hScroll <= 190 && 190 < e.hScroll+cw) {
		t.Errorf("cursor cell 190 not visible: hScroll=%d cw=%d (range %d..%d)",
			e.hScroll, cw, e.hScroll, e.hScroll+cw)
	}
}

// TestHScrollToCursorNoScrollShortLine verifies EnsureVisible does not scroll
// horizontally for a line shorter than the viewport.
func TestHScrollToCursorNoScrollShortLine(t *testing.T) {
	e := newEditor("", 120, 30)
	e.width = 60
	e.lines = []string{"short", ""}
	e.curRow, e.curCol = 0, 3
	e.hScroll = 0
	e.EnsureVisible()
	if e.hScroll != 0 {
		t.Errorf("short line must not hscroll, got %d", e.hScroll)
	}
}

// TestHScrollOverflowIndicator verifies editor truncates overflowed lines to
// the content width without a marker, and truncateWidthIsOverflow behaves.
func TestHScrollOverflowIndicator(t *testing.T) {
	if !truncateWidthIsOverflow("abcdef", 4) {
		t.Error("abcdef wider than 4 should overflow")
	}
	if truncateWidthIsOverflow("abc", 4) {
		t.Error("abc not wider than 4")
	}
	m := New(Args{Width: 100, Height: 24}).(model)
	m.active = paneEdit
	long := ""
	for i := 0; i < 300; i++ {
		long += "x"
	}
	m.ed.SetText(long + "\n")
	m.ed.width = 50
	out := stripANSI(m.renderEditor(50))
	// Full line = gutter (5 cells) + padded content (<= contentW = width-7).
	// contentW = 50-2-5 = 43; gutter 5 → total <= 48.
	if runewidth.StringWidth(firstLineOf(out)) > 5+(50-7) {
		t.Errorf("renderEditor line too wide: got %d max %d",
			runewidth.StringWidth(firstLineOf(out)), 5+(50-7))
	}
	// Overflowed line at hScroll=0 must show the '…' marker.
	if !strings.Contains(out, "…") {
		t.Errorf("editor overflowed line should show '…', got %q", firstLineOf(out))
	}
}

// TestCursorVisibleWhenScrolled verifies the block cursor is still rendered
// (present in the ANSI output) once horizontal auto-scroll kicks in.
func TestCursorVisibleWhenScrolled(t *testing.T) {
	m := New(Args{Width: 100, Height: 24}).(model)
	m.active = paneEdit
	m.width, m.height = 100, 24
	// Editor width must track the real panel width (files sidebar accounted).
	m.ed.width = m.layout().mid
	var long string
	for i := 0; i < 200; i++ {
		long += "a"
	}
	m.ed.SetText(long + "\n")
	// walk the cursor far right so auto-scroll engages.
	for col := 0; col < 70; col++ {
		m.ed.curCol = col
		m.ed.EnsureVisible()
	}
	if m.ed.hScroll <= 0 {
		t.Fatalf("expected hScroll>0, got %d", m.ed.hScroll)
	}
	raw := m.renderEditor(m.layout().mid)
	// cursorStyle uses background colour 63; the rendered cursor emits it.
	if !strings.Contains(raw, "\x1b[") || !containsAnsiBg(raw, "63") {
		t.Errorf("cursor background (63) missing in scrolled render: %.40q", raw)
	}
	// windowed line must not exceed the pane content width.
	w := m.ed.contentWidth()
	if runewidth.StringWidth(stripANSI(firstLineOf(raw))) > 6+w {
		t.Errorf("rendered line too wide: %d cells (gutter 6 + content %d)",
			runewidth.StringWidth(stripANSI(firstLineOf(raw))), w)
	}
}

func containsAnsiBg(s, code string) bool {
	// match an SGR sequence containing the given colour code.
	for i := 0; i+1 < len(s); i++ {
		if s[i] == '\x1b' && s[i+1] == '[' {
			j := i + 2
			for j < len(s) && (s[j] >= '0' && s[j] <= '9' || s[j] == ';') {
				j++
			}
			seq := s[i : j+1]
			if strings.Contains(seq, code) && strings.HasSuffix(seq, "m") {
				return true
			}
		}
	}
	return false
}

func firstLineOf(s string) string {
	for i := 0; i < len(s); i++ {
		if s[i] == '\n' {
			return s[:i]
		}
	}
	return s
}

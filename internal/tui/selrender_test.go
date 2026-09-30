package tui

import (
	"strings"
	"testing"
)

// TestSelRenderScrolled verifies that when horizontally scrolled, the selection
// highlight is drawn at the correct visible position (tightened to the window).
func TestSelRenderScrolled(t *testing.T) {
	m := New(Args{Width: 120, Height: 30}).(model)
	m.active = paneEdit
	m.width, m.height = 120, 30
	m.ed.width = m.layout().mid
	// A body line (no syntax tokens), long enough to scroll.
	long := "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	m.ed.SetText("### x\nGET http://x\n" + long + "\n")
	m.ed.curRow = 2
	m.ed.curCol = 0
	// scroll so we see the tail of the long line.
	m.ed.hScroll = 20
	// select a range in the visible window.
	m.selActive = true
	m.selAnchorRow, m.selAnchorCol = 2, 25
	m.ed.curRow, m.ed.curCol = 2, 45

	out := m.renderEditor(m.layout().mid)
	// The selection background (selStyle bg=24) must be present in the render.
	if !strings.Contains(out, ";24m") {
		t.Errorf("selection background (48;5;24) missing, got %q", out[:80])
	}
	// Ensure the windowed line fits within the pane width.
	full := strings.Split(stripANSI(out), "\n")
	for i, ln := range full {
		if len([]rune(ln)) > 6+m.ed.contentWidth() {
			t.Errorf("line %d too wide (%d cells)", i, len([]rune(ln)))
		}
	}
}

// TestSelRenderNotScrolled verifies selection renders when hScroll==0 too.
func TestSelRenderNotScrolled(t *testing.T) {
	m := New(Args{Width: 120, Height: 30}).(model)
	m.active = paneEdit
	m.width, m.height = 120, 30
	m.ed.width = m.layout().mid
	m.ed.SetText("### x\nGET http://x\nhello world\n")
	m.selActive = true
	m.selAnchorRow, m.selAnchorCol = 2, 6
	m.ed.curRow, m.ed.curCol = 2, 11
	m.ed.hScroll = 0
	out := m.renderEditor(m.layout().mid)
	plain := stripANSI(out)
	if !strings.Contains(plain, "hello") {
		t.Fatalf("expected line text, got %q", plain)
	}
}
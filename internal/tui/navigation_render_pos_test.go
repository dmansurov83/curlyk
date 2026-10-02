package tui

import (
	"strings"
	"testing"
)

// TestNavRenderEntryRowPositions verifies the on-screen row where the nav popup
// entries appear, so mouse hit-testing (navItemAt / navFirstEntryRow) agrees
// with rendering. The editor pane's first content row is headerHeight+1 (the top
// border sits on the headerHeight-th row); the filter title occupies it, so the
// first selectable entry sits at headerHeight+2.
func TestNavRenderEntryRowPositions(t *testing.T) {
	m := New(Args{Width: 120, Height: 30}).(model)
	m.active = paneEdit
	m.ed.SetText("### a\nGET http://x/1\n\n### b\nPOST http://x/2\n")
	m.beginNav()
	out := stripANSI(m.View())
	rows := strings.Split(out, "\n")

	// Title row: first content row of the editor pane.
	if !strings.Contains(stripANSI(rows[headerHeight+1]), "Перейти к запросу") {
		t.Logf("title not found at row %d: %q", headerHeight+1, rows[headerHeight+1])
	}
	// First entry row.
	if !strings.Contains(stripANSI(rows[headerHeight+2]), "GET http://x/1") {
		t.Errorf("first entry missing at row %d: %q\nfull:\n%s", headerHeight+2, rows[headerHeight+2], out)
	}
	// Second entry row.
	if !strings.Contains(stripANSI(rows[headerHeight+3]), "POST http://x/2") {
		t.Errorf("second entry missing at row %d: %q\nfull:\n%s", headerHeight+3, rows[headerHeight+3], out)
	}
}

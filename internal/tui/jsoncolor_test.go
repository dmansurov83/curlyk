package tui

import (
	"strings"
	"testing"
)

// TestRespJSONSelectionNoArtifacts guards against ANSI-remainder artifacts in
// the response pane when a JSON body line is partially inside the selection.
// The raw rendered output must not contain a reset immediately followed by a
// colour-open with an empty span (the signature of a broken splice like
// "\x1b[38;5;81m\x1b[0m\x1b[48;5;24m..." that the terminal paints as a stray mark).
func TestRespJSONSelectionNoArtifacts(t *testing.T) {
	m := New(Args{Width: 120, Height: 30}).(model)
	m.active = paneResp
	m.response = "HTTP/1.1 200 OK\n\n{\n  \"args\": {},\n  \"data\": \"x\"\n}\n"
	m.respHeader = "HTTP/1.1 200 OK\n"
	// select partway through the "args" key line (pane row 2), from line start.
	m.respSelActive = true
	m.respSelAnchorRow, m.respSelAnchorCol = 2, 0
	m.respSelCurRow, m.respSelCurCol = 2, 5

	out := m.renderResponse(57, 24)

	// Artifact signature: an empty styled fragment, i.e. a colour/attribute-open
	// immediately followed by a reset with no text between. Lipgloss emits these
	// (e.g. "\x1b[38;5;81m\x1b[0m") when a zero-length span is rendered; the
	// terminal can draw them as stray marks. A reset then a new open ("...0m\x1b[38...")
	// is a legitimate boundary between two painted fragments and must NOT match.
	for i := 0; i+4 < len(out); i++ {
		if out[i] == 'm' && strings.HasPrefix(out[i+1:], "\x1b[0m") {
			t.Fatalf("empty ANSI fragment (style open then reset with no text) at offset %d: %q", i+1, out[i:i+12])
		}
	}

	// The selection must appear on the selected line ("args" key painted with
	// background 24 while keeping its foreground 81 for the unselected tail).
	if !strings.Contains(out, "38;5;81;48;5;24") {
		t.Errorf("expected merged foreground+selection on partially selected token, got %q", out)
	}
}
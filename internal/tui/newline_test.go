package tui

import "testing"

func TestNormalizeNewlines(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{"", ""},
		{"GET /x\nAccept: a\n", "GET /x\nAccept: a\n"},
		{"GET /x\r\nAccept: a\r\n", "GET /x\nAccept: a\n"},
		{"a\rb", "a\nb"},
		{"a\r\nb\rc\r\n", "a\nb\nc\n"},
	}
	for _, c := range cases {
		if got := normalizeNewlines(c.in); got != c.want {
			t.Errorf("normalizeNewlines(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

// TestEditorLoadsCRLFFile verifies that editing content loaded from a Windows
// CRLF file never keeps a trailing carriage return on any buffer line (which
// would silently corrupt the rendered frame on a terminal).
func TestEditorLoadsCRLFFile(t *testing.T) {
	e := newEditor("GET /x\r\nAccept: a\r\n\r\n{\r\n  \"k\": 1,\r\n}\r\n", 40, 10)
	lines := e.Lines()
	for i, ln := range lines {
		for _, r := range ln {
			if r == '\r' {
				t.Fatalf("line %d still contains CR: %q\nall lines: %q", i, ln, lines)
			}
		}
	}
	// The CRLF line endings must all have become line boundaries; the trailing
	// CRLF yields one trailing empty line, so 7 lines from 6 CRLF pairs.
	want := []string{"GET /x", "Accept: a", "", "{", `  "k": 1,`, "}", ""}
	if len(lines) != len(want) {
		t.Fatalf("expected %d lines after CRLF normalization, got %d: %q", len(want), len(lines), lines)
	}
	for i := range want {
		if lines[i] != want[i] {
			t.Errorf("line %d = %q, want %q", i, lines[i], want[i])
		}
	}
}
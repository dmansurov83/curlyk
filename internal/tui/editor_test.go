package tui

import (
	"strings"
	"testing"

	"github.com/user/curlyk/httptool/internal/httpfile"
)

func TestEditorTextRoundtrip(t *testing.T) {
	src := "GET http://x/1\nAccept: */*\n"
	ed := newEditor(src, 40, 10)
	if ed.Text() != src {
		t.Errorf("roundtrip: got %q", ed.Text())
	}
}

func TestEditorInsert(t *testing.T) {
	ed := newEditor("GET http://x", 40, 10)
	ed.curCol = 3 // after "GET"
	ed.InsertString(" ")
	got := ed.Lines()[0]
	if got != "GET  http://x" {
		t.Errorf("insert got %q", got)
	}
}

func TestHighlightLine_Comment(t *testing.T) {
	lts := []httpfile.Token{
		{Type: httpfile.TokMethod, Start: 0, End: 3, Value: "GET"},
		{Type: httpfile.TokURL, Start: 4, End: 22, Value: "http://example.com"},
	}
	out := highlightLine("GET http://example.com", lts)
	t.Logf("out=%q", out)
	if !strings.Contains(out, "http://example.com") {
		t.Error("highlight dropped text")
	}
	if !strings.Contains(out, "GET") {
		t.Error("highlight dropped method")
	}
}

func TestRenderEditorNoPanic(t *testing.T) {
	out := Snapshot(Args{Width: 100, Height: 24})
	if out == "" {
		t.Error("empty render")
	}
	if !strings.Contains(out, "GET https://httpbin.org/get") {
		t.Error("editor content missing from render")
	}
}

func TestRenderLineWithCursorPreservesText(t *testing.T) {
	raw := "GET /x"
	// cursor at end of line -> trailing block (space)
	out := renderLineWithCursor(raw, 6, nil)
	plain := stripANSI(out)
	if !strings.HasPrefix(plain, "GET /x") {
		t.Errorf("prefix broken: %q", plain)
	}
	// total visible width should equal len(raw)+1 (one extra cursor spacer at EOL)
	if len([]rune(plain)) != len([]rune(raw))+1 {
		t.Errorf("width=%d want %d\n%s", len([]rune(plain)), len([]rune(raw))+1, out)
	}

	// cursor in the middle: char under cursor is preserved once
	out2 := renderLineWithCursor("ABCD", 2, nil)
	plain2 := stripANSI(out2)
	// stripANSI keeps the cursor char: "AB" + "C"(block) + "D" = "ABCD"
	if len([]rune(plain2)) != 4 {
		t.Errorf("middle cursor width=%d want 4: %q", len([]rune(plain2)), plain2)
	}
}
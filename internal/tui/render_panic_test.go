package tui

import (
	"testing"

	"github.com/user/curlyk/httptool/internal/httpfile"
)

// TestRenderLineWithCursorSelNoPanicAtEnd reproduces a panic where the cursor
// column exceeds the line length (e.g. after horizontal scrolling or long
// selection), causing the segment slice to go out of bounds.
func TestRenderLineWithCursorSelNoPanicAtEnd(t *testing.T) {
	raw := "GET https://example.com very long request line that exceeds cursor bounds"
	// cursor column far beyond the actual rune count, selection also oversized.
	col := 1196
	selStart, selEnd := 22, 1196
	var toks []httpfile.Token
	_ = renderLineWithCursorSel(raw, col, toks, selStart, selEnd, true)

	// Cursor exactly at end of line.
	runes := []rune(raw)
	_ = renderLineWithCursorSel(raw, len(runes), toks, 0, len(runes), true)
}

// TestRenderLineWithCursorNoPanicAtEnd exercises the non-selection path with an
// oversized cursor column too.
func TestRenderLineWithCursorNoPanicAtEnd(t *testing.T) {
	raw := "POST http://x/1"
	col := 500
	var toks []httpfile.Token
	_ = renderLineWithCursor(raw, col, toks)
}
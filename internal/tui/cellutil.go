package tui

import (
	"bytes"
	"encoding/json"
	"strings"

	"github.com/mattn/go-runewidth"
	"github.com/user/curlyk/internal/httpfile"
)

// padToWidth pads s (possibly ANSI-coloured) with trailing spaces so its total
// display width is at least w cells.
func padToWidth(s string, w int) string {
	clean := stripANSI(s)
	cur := runewidth.StringWidth(clean)
	if cur >= w {
		return s
	}
	return s + strings.Repeat(" ", w-cur)
}

func truncateWidth(s string, max int) string {
	if max <= 0 {
		return ""
	}
	r := []rune(s)
	var w int
	var out []rune
	for _, ch := range r {
		rw := runewidth.RuneWidth(ch)
		if w+rw > max {
			break
		}
		out = append(out, ch)
		w += rw
	}
	return string(out)
}

// truncateWidthIsOverflow reports whether s is wider than max display cells and
// therefore would be visually truncated (candidates for an overflow indicator).
func truncateWidthIsOverflow(s string, max int) bool {
	if max <= 0 {
		return s != ""
	}
	return runewidth.StringWidth(s) > max
}

// visibleRuneRange returns the rune index range [start, end) of s that fits
// within `max` display cells starting at a horizontal `offset` in cells. This
// mirrors truncateWidthOffset but yields rune bounds instead of a substring, so
// the caller can slice the original line and still place a cursor correctly.
func visibleRuneRange(s string, max, offset int) (start, end int) {
	runes := []rune(s)
	var w int
	start = 0
	for start < len(runes) {
		rw := runewidth.RuneWidth(runes[start])
		if w+rw > offset {
			break
		}
		w += rw
		start++
	}
	end = start
	w = 0
	for i := start; i < len(runes); i++ {
		rw := runewidth.RuneWidth(runes[i])
		if w+rw > max && i > start {
			break
		}
		w += rw
		end++
	}
	return start, end
}

// truncateWidthOffset returns up to max display-width cells of s, starting at a
// horizontal offset (in cells). Used for horizontal scrolling of long lines.
func truncateWidthOffset(s string, max, offset int) string {
	if max <= 0 {
		return ""
	}
	start, end := visibleRuneRange(s, max, offset)
	return string([]rune(s)[start:end])
}

// displayToRune converts a display-width column into a rune index in s.
func displayToRune(s string, displayPos int) int {
	if displayPos <= 0 {
		return 0
	}
	runes := []rune(s)
	var w int
	for i, r := range runes {
		rw := runewidth.RuneWidth(r)
		if w+rw > displayPos {
			return i
		}
		w += rw
		if w == displayPos {
			return i + 1
		}
	}
	return len(runes)
}

// formatBody pretty-prints JSON response bodies; non-JSON is returned verbatim.
// JSON object-key order is preserved (uses json.Indent rather than MarshalIndent).
func formatBody(body []byte) string {
	s := string(body)
	trimmed := strings.TrimSpace(s)
	if trimmed == "" {
		return s
	}
	if !json.Valid([]byte(trimmed)) {
		return s
	}
	var out bytes.Buffer
	if err := json.Indent(&out, []byte(trimmed), "", "  "); err != nil {
		return s
	}
	return out.String()
}

// shiftTokensForWindow re-bases syntax tokens (byte offsets into the full line)
// onto a windowed substring starting at byte offset `byteStart`, dropping tokens
// that lie entirely outside the window. Variable spans are likewise shifted.
func shiftTokensForWindow(toks []httpfile.Token, byteStart, byteEnd int) []httpfile.Token {
	if byteStart == 0 {
		return toks
	}
	var out []httpfile.Token
	for _, tk := range toks {
		st, en := tk.Start, tk.End
		if en <= byteStart || st >= byteEnd {
			continue // outside window
		}
		if st < byteStart {
			st = byteStart
		}
		if en > byteEnd {
			en = byteEnd
		}
		nt := tk
		nt.Start = st - byteStart
		nt.End = en - byteStart
		out = append(out, nt)
	}
	return out
}

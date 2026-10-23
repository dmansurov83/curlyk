package gui

import (
	"sort"
	"strings"

	"github.com/user/curlyk/internal/httpfile"
	"github.com/user/curlyk/internal/jsonlex"
)

// seg is a styled byte range [start,end) inside one source line.
//
// kind is an .http lexer token type. Two extra meanings are carried by kind:
// TokOther marks unstyled text (indent, the gap around a token) and TokVariable
// marks a {{placeholder}} span, which the lexer reports as a VarSpan inside its
// enclosing token rather than as a token. jsonKind is set for the pieces of a
// request-body line that the shared JSON lexer recognises, so a JSON body is
// painted with the scheme's json_* colours instead of one flat colour.
type seg struct {
	start, end int
	kind       httpfile.TokenType
	json       jsonlex.Kind
	jsonKind   bool
}

// highlight returns the per-line segments of a whole document.
func highlight(src string) [][]seg {
	lines := splitLines(src)
	return highlightRange(lines, 0, len(lines))
}

// highlightRange lexes only lines[from:to) and returns segments for the whole
// document, with the untouched lines left nil. The .http lexer resets its state
// on blank lines and on "###" separators, so a sub-range that starts at such a
// boundary tokenizes exactly as the whole file does — this is what lets the
// editor re-lex a single request block on every keystroke instead of the file.
func highlightRange(lines []string, from, to int) [][]seg {
	if from < 0 {
		from = 0
	}
	if to > len(lines) {
		to = len(lines)
	}
	out := make([][]seg, len(lines))
	if from >= to {
		return out
	}

	sub := strings.Join(lines[from:to], "\n")
	byLine := make(map[int][]httpfile.Token)
	for _, tk := range httpfile.Lex(sub) {
		byLine[tk.Line] = append(byLine[tk.Line], tk)
	}
	for i := from; i < to; i++ {
		out[i] = lineSegments(lines[i], byLine[i-from+1])
	}
	return out
}

// lineSegments builds the full-coverage segment list of a single line.
func lineSegments(line string, toks []httpfile.Token) []seg {
	var segs []seg
	for _, tk := range toks {
		if tk.Type == httpfile.TokOther || tk.Value == "" {
			continue
		}
		start, end := clampInt(tk.Start, 0, len(line)), clampInt(tk.End, 0, len(line))
		if end <= start {
			continue
		}
		segs = append(segs, tokenSegments(tk, start, end)...)
	}
	return fillGaps(line, splitJSONSegments(line, segs))
}

// tokenSegments converts one token into segments, splitting out the
// {{placeholder}} spans it carries.
func tokenSegments(tk httpfile.Token, start, end int) []seg {
	if len(tk.Vars) == 0 {
		return []seg{{start: start, end: end, kind: tk.Type}}
	}
	var out []seg
	last := 0
	for _, v := range tk.Vars {
		if v.Start < last || v.End > end-start {
			continue
		}
		if v.Start > last {
			out = append(out, seg{start: start + last, end: start + v.Start, kind: tk.Type})
		}
		out = append(out, seg{start: start + v.Start, end: start + v.End, kind: httpfile.TokVariable})
		last = v.End
	}
	if last < end-start {
		out = append(out, seg{start: start + last, end: end, kind: tk.Type})
	}
	return out
}

// splitJSONSegments replaces every body-text segment of a JSON body line with
// the JSON lexer's tokens, so the body is highlighted as JSON. Non-body segments
// ({{variables}} inside the body, blanks, punctuation) are kept as they are, and
// a line that is not a JSON body start is returned unchanged.
func splitJSONSegments(line string, segs []seg) []seg {
	if !jsonlex.IsBodyStart(line) {
		return segs
	}
	offs := runeByteOffsets(line)
	var out []seg
	changed := false
	for _, s := range segs {
		if s.kind != httpfile.TokBodyText {
			out = append(out, s)
			continue
		}
		inner := splitJSONRange(line[s.start:s.end], s.start, offs, s)
		if inner == nil {
			out = append(out, s)
			continue
		}
		changed = true
		out = append(out, inner...)
	}
	if !changed {
		return segs
	}
	return out
}

// splitJSONRange lexes one body-text span (source line[start:end]) as JSON and
// returns segments with byte offsets relative to the whole line. Whitespace and
// unrecognised fragments keep the body's own kind. nil means "nothing to
// change".
func splitJSONRange(text string, base int, offs []int, orig seg) []seg {
	toks := jsonlex.Lex(text)
	if len(toks) == 0 {
		return nil
	}
	var out []seg
	appendRange := func(from, to int, kind httpfile.TokenType, jk jsonlex.Kind, isJSON bool) {
		if to <= from {
			return
		}
		out = append(out, seg{
			start:    base + byteOffset(offs, from),
			end:      base + byteOffset(offs, to),
			kind:     kind,
			json:     jk,
			jsonKind: isJSON,
		})
	}
	for _, t := range toks {
		switch t.Kind {
		case jsonlex.WS:
			// Keep the body's faint colour for the indentation.
			appendRange(t.Start, t.End, orig.kind, 0, false)
		case jsonlex.Other:
			appendRange(t.Start, t.End, orig.kind, 0, false)
		default:
			appendRange(t.Start, t.End, httpfile.TokBodyText, t.Kind, true)
		}
	}
	return out
}

// runeByteOffsets returns the byte offset of every rune of s, plus len(s) for
// the end position. The .http lexer works in bytes while the JSON lexer works in
// runes, so this is the bridge between the two.
func runeByteOffsets(s string) []int {
	offs := make([]int, 0, len(s)+1)
	for i := range s {
		offs = append(offs, i)
	}
	return append(offs, len(s))
}

// byteOffset maps a rune index to a byte offset using a table from
// runeByteOffsets, clamping out-of-range indexes to the string bounds.
func byteOffset(offs []int, runeIdx int) int {
	if runeIdx <= 0 {
		return 0
	}
	if runeIdx >= len(offs) {
		return offs[len(offs)-1]
	}
	return offs[runeIdx]
}

// fillGaps sorts segments, drops overlaps (the earlier segment wins) and fills
// the remaining holes with unstyled ones.
func fillGaps(line string, segs []seg) []seg {
	sort.SliceStable(segs, func(a, b int) bool { return segs[a].start < segs[b].start })

	var out []seg
	pos := 0
	for _, s := range segs {
		if s.start < pos || s.end > len(line) {
			continue
		}
		if s.start > pos {
			out = append(out, seg{start: pos, end: s.start, kind: httpfile.TokOther})
		}
		out = append(out, s)
		pos = s.end
	}
	if pos < len(line) {
		out = append(out, seg{start: pos, end: len(line), kind: httpfile.TokOther})
	}
	return out
}

// splitLines splits src into lines as the lexer numbers them (1-based, CR
// stripped) so line indexes stay in step with token.Line.
func splitLines(src string) []string {
	lines := strings.Split(src, "\n")
	for i, l := range lines {
		lines[i] = strings.TrimSuffix(l, "\r")
	}
	return lines
}

// runeLen returns the number of runes in line.
func runeLen(line string) int {
	return len([]rune(line))
}

func clampInt(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

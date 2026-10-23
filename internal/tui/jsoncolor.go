package tui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/user/curlyk/internal/httpfile"
	"github.com/user/curlyk/internal/jsonlex"
)

// JSON syntax coloring for the response body pane and for request bodies. The
// tokenizer itself lives in internal/jsonlex (shared with the Fyne GUI); this
// file only maps token kinds onto the active colour scheme.
//
// The integration point is jsonColorizeLine, applied to a response body line
// AFTER it has been truncated to its visible width (truncateWidth /
// truncateWidthOffset). It reads plain text and returns an ANSI-coloured
// string, so callers must never re-measure its output with runewidth:
// truncate first, colorize last.

var (
	jsonKeyStyle    lipgloss.Style
	jsonStringStyle lipgloss.Style
	jsonNumberStyle lipgloss.Style
	jsonBoolStyle   lipgloss.Style
	jsonNullStyle   lipgloss.Style
	jsonPunctStyle  lipgloss.Style
)

// jsonTokStyle maps a token kind to its lipgloss style.
func jsonTokStyle(kind jsonlex.Kind) lipgloss.Style {
	switch kind {
	case jsonlex.Key:
		return jsonKeyStyle
	case jsonlex.String:
		return jsonStringStyle
	case jsonlex.Number:
		return jsonNumberStyle
	case jsonlex.Bool:
		return jsonBoolStyle
	case jsonlex.Null:
		return jsonNullStyle
	case jsonlex.Punct:
		return jsonPunctStyle
	default:
		return lipgloss.NewStyle()
	}
}

// jsonColsForLine lexes a whole raw body line as JSON and returns syntax-colored
// cols with byte offsets relative to line, or ok=false when the line is not
// recognised as JSON so the caller can fall back to the generic body style.
// Lexing the full line once keeps JSON coloring correct when the line is later
// split by the cursor or a selection: clipping full-line cols is position-safe,
// whereas re-lexing an arbitrary fragment would mislabel it. Whitespace and
// unrecognised "other" fragments are left uncolored.
func jsonColsForLine(line string) ([]col, bool) {
	if !jsonlex.IsBodyStart(line) {
		return nil, false
	}
	toks := jsonlex.Lex(line)
	if len(toks) == 0 {
		return nil, false
	}
	var cols []col
	for _, t := range toks {
		if t.Kind == jsonlex.WS || t.Kind == jsonlex.Other {
			continue
		}
		cols = append(cols, col{
			start: byteLenOfRunes(line, t.Start),
			end:   byteLenOfRunes(line, t.End),
			style: jsonTokStyle(t.Kind),
		})
	}
	return cols, len(cols) > 0
}

// shiftJSONColsForWindow re-bases whole-line JSON cols (byte offsets into the
// full line) onto a windowed substring starting at byte offset byteStart,
// keeping only cols that intersect the window [byteStart, byteEnd).
func shiftJSONColsForWindow(cols []col, byteStart, byteEnd int) []col {
	var out []col
	for _, c := range cols {
		st, en := c.start, c.end
		if en <= byteStart || st >= byteEnd {
			continue
		}
		if st < byteStart {
			st = byteStart
		}
		if en > byteEnd {
			en = byteEnd
		}
		if en <= st {
			continue
		}
		out = append(out, col{start: st - byteStart, end: en - byteStart, style: c.style})
	}
	return out
}

// isBodyTokenLine reports whether a line's (unshifted) tokens include a body
// text token, i.e. the line belongs to a request body and is eligible for JSON
// syntax coloring.
func isBodyTokenLine(toks []httpfile.Token) bool {
	for _, tk := range toks {
		if tk.Type == httpfile.TokBodyText {
			return true
		}
	}
	return false
}

// jsonColorizeLine renders a single plain response-body line with JSON syntax
// coloring. If hasSel is true, the rune range [selStart, selEnd) is additionally
// painted with the selection background. Non-JSON text simply renders with no
// foreground colour (the line is returned ANSI-wrapped only where tokens
// matched).
//
// Fragments that fall inside the selection reuse the token's own style plus the
// selection background, so the output stays ANSI-contiguous across segment
// boundaries (a plain selStyle.Render splice would leave reset gaps that the
// terminal shows as visual artifacts).
func jsonColorizeLine(line string, selStart, selEnd int, hasSel bool) string {
	toks := jsonlex.Lex(line)
	if len(toks) == 0 {
		return line
	}
	var b strings.Builder
	for _, t := range toks {
		seg := line[t.Start:t.End]
		if t.Kind == jsonlex.WS {
			b.WriteString(seg)
			continue
		}
		st := jsonTokStyle(t.Kind)
		if !hasSel || t.End <= selStart || t.Start >= selEnd {
			b.WriteString(st.Render(seg))
			continue
		}
		runes := []rune(seg)
		relS := max(0, selStart-t.Start)
		relE := min(len(runes), selEnd-t.Start)
		if relS > 0 {
			b.WriteString(st.Render(string(runes[:relS])))
		}
		if relE > relS {
			b.WriteString(st.Background(lipgloss.Color(curScheme.JSONSelBg)).Render(string(runes[relS:relE])))
		}
		if relE < len(runes) {
			b.WriteString(st.Render(string(runes[relE:])))
		}
	}
	return b.String()
}

package gui

import (
	"strings"

	"github.com/user/curlyk/internal/httpfile"
	"github.com/user/curlyk/internal/jsonlex"
)

// responseSegments builds the per-line styling of a response dump: the header
// part (request dump, status, timing, response headers) stays neutral, and the
// body is highlighted as JSON when its first non-empty line opens an object or
// an array. A non-JSON body keeps one neutral segment per line, so HTML or plain
// text is never painted with JSON colours.
func responseSegments(lines []string, headerRows int) [][]seg {
	out := make([][]seg, len(lines))
	jsonBody := detectJSONBody(lines, headerRows)
	for i, line := range lines {
		if i < headerRows || !jsonBody {
			out[i] = fillGaps(line, nil)
			continue
		}
		out[i] = jsonLineSegments(line)
	}
	return out
}

// detectJSONBody reports whether the first non-empty line after the header block
// starts a JSON object or array.
func detectJSONBody(lines []string, headerRows int) bool {
	for i := headerRows; i < len(lines); i++ {
		t := strings.TrimSpace(lines[i])
		if t == "" {
			continue
		}
		return t[0] == '{' || t[0] == '['
	}
	return false
}

// jsonLineSegments lexes one body line with the shared JSON lexer. Whitespace and
// unrecognised fragments keep the plain body kind; tokens get their JSON colour.
func jsonLineSegments(line string) []seg {
	toks := jsonlex.Lex(line)
	if len(toks) == 0 {
		return fillGaps(line, nil)
	}
	offs := runeByteOffsets(line)
	segs := make([]seg, 0, len(toks))
	for _, t := range toks {
		from := byteOffset(offs, t.Start)
		to := byteOffset(offs, t.End)
		if to <= from {
			continue
		}
		switch t.Kind {
		case jsonlex.WS, jsonlex.Other:
			segs = append(segs, seg{start: from, end: to, kind: httpfile.TokBodyText})
		default:
			segs = append(segs, seg{
				start:    from,
				end:      to,
				kind:     httpfile.TokBodyText,
				json:     t.Kind,
				jsonKind: true,
			})
		}
	}
	return fillGaps(line, segs)
}

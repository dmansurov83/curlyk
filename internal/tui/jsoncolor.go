package tui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// JSON syntax coloring for the response body pane. Kept in its own file so
// other agents working on render.go/highlight.go don't collide.
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

type jsonTokKind int

const (
	jsonTokWS     jsonTokKind = iota // whitespace / indentation
	jsonTokKey                       // quoted string followed by ':'
	jsonTokString                    // quoted string value
	jsonTokNumber                    // -12.3e+4
	jsonTokBool                      // true / false
	jsonTokNull                      // null
	jsonTokPunct                     // { } [ ] , :
	jsonTokOther                     // stray text, "…" marker
)

type jsonTok struct {
	kind  jsonTokKind
	start int // rune index in the source line
	end   int // exclusive rune index
}

func isJSONSpace(r rune) bool {
	return r == ' ' || r == '\t'
}

// lexJSONLine splits line into JSON tokens (rune-indexed). Strings honour
// backslash escapes, so a \" inside a value does not terminate the token.
func lexJSONLine(line string) []jsonTok {
	runes := []rune(line)
	var toks []jsonTok
	i, n := 0, len(runes)
	for i < n {
		r := runes[i]
		switch {
		case isJSONSpace(r):
			j := i
			for j < n && isJSONSpace(runes[j]) {
				j++
			}
			toks = append(toks, jsonTok{jsonTokWS, i, j})
			i = j
		case r == '"':
			j := i + 1
			for j < n {
				if runes[j] == '\\' && j+1 < n {
					j += 2
					continue
				}
				if runes[j] == '"' {
					j++
					break
				}
				j++
			}
			toks = append(toks, jsonTok{jsonTokKey, i, j}) // demoted to String unless it's a key
			i = j
		case r == '{' || r == '}' || r == '[' || r == ']' || r == ',' || r == ':':
			toks = append(toks, jsonTok{jsonTokPunct, i, i + 1})
			i++
		case r == '-' || (r >= '0' && r <= '9'):
			j := i
			for j < n {
				c := runes[j]
				if (c >= '0' && c <= '9') || c == '-' || c == '+' || c == '.' || c == 'e' || c == 'E' {
					j++
					continue
				}
				break
			}
			toks = append(toks, jsonTok{jsonTokNumber, i, j})
			i = j
		case r >= 'a' && r <= 'z':
			j := i
			for j < n && runes[j] >= 'a' && runes[j] <= 'z' {
				j++
			}
			kind := jsonTokOther
			switch string(runes[i:j]) {
			case "true", "false":
				kind = jsonTokBool
			case "null":
				kind = jsonTokNull
			}
			toks = append(toks, jsonTok{kind, i, j})
			i = j
		default:
			toks = append(toks, jsonTok{jsonTokOther, i, i + 1})
			i++
		}
	}
	return toks
}

// markJSONKeys demotes key-position strings to jsonTokString and upgrades those
// directly followed by a ':' punct to jsonTokKey.
func markJSONKeys(line string, toks []jsonTok) []jsonTok {
	for i := range toks {
		if toks[i].kind != jsonTokKey {
			continue
		}
		isKey := false
		for j := i + 1; j < len(toks); j++ {
			if toks[j].kind == jsonTokWS {
				continue
			}
			if toks[j].kind == jsonTokPunct && line[toks[j].start] == ':' {
				isKey = true
			}
			break
		}
		if isKey {
			toks[i].kind = jsonTokKey
		} else {
			toks[i].kind = jsonTokString
		}
	}
	return toks
}

// jsonTokStyle maps a resolved token kind to its lipgloss style.
func jsonTokStyle(kind jsonTokKind) lipgloss.Style {
	switch kind {
	case jsonTokKey:
		return jsonKeyStyle
	case jsonTokString:
		return jsonStringStyle
	case jsonTokNumber:
		return jsonNumberStyle
	case jsonTokBool:
		return jsonBoolStyle
	case jsonTokNull:
		return jsonNullStyle
	case jsonTokPunct:
		return jsonPunctStyle
	default:
		return lipgloss.NewStyle()
	}
}

// jsonColorizeLine renders a single plain response-body line with JSON syntax
// coloring. If hasSel is true, the rune range [selStart, selEnd) is additionally
// wrapped in the selection background style. Non-JSON text simply renders with
// no foreground colour (the line is returned ANSI-wrapped only where tokens
// matched).
// jsonColorizeLine renders a single plain response-body line with JSON syntax
// coloring. If hasSel is true, the rune range [selStart, selEnd) is additionally
// painted with the selection background. Non-JSON text simply renders with
// no foreground colour (the line is returned ANSI-wrapped only where tokens
// matched).
//
// Fragments that fall inside the selection reuse the token's own style plus the
// selection background, so the output stays ANSI-contiguous across segment
// boundaries (a plain selStyle.Render splice would leave reset gaps that the
// terminal shows as visual artifacts).
func jsonColorizeLine(line string, selStart, selEnd int, hasSel bool) string {
	toks := markJSONKeys(line, lexJSONLine(line))
	if len(toks) == 0 {
		return line
	}
	var b strings.Builder
	for _, t := range toks {
		seg := line[t.start:t.end]
		if t.kind == jsonTokWS {
			b.WriteString(seg)
			continue
		}
		st := jsonTokStyle(t.kind)
		if !hasSel || t.end <= selStart || t.start >= selEnd {
			b.WriteString(st.Render(seg))
			continue
		}
		runes := []rune(seg)
		relS := max(0, selStart-t.start)
		relE := min(len(runes), selEnd-t.start)
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

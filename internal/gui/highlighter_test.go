package gui

import (
	"reflect"
	"strings"
	"testing"

	"github.com/user/curlyk/internal/httpfile"
	"github.com/user/curlyk/internal/jsonlex"
)

// TestHighlighterIncrementalMatchesFullLex is the core guarantee of the
// incremental highlighter: after every edit, the segments it kept must be
// identical to a full re-lex of the whole document. Any drift would show up as
// wrong colours around an edit.
func TestHighlighterIncrementalMatchesFullLex(t *testing.T) {
	src := strings.Join([]string{
		"### Create user",
		"# a comment",
		"POST http://api.example.com/v1/users HTTP/1.1",
		"Content-Type: application/json",
		"Authorization: Bearer {{token}}",
		"",
		"{",
		`  "name": "Ada",`,
		`  "active": true`,
		"}",
		"",
		"### Get users",
		"GET http://api.example.com/v1/users?limit=20",
		"Accept: application/json",
		"",
		"### Delete",
		"DELETE http://api.example.com/v1/users/1",
		"",
	}, "\n")

	h := newHighlighter()
	h.setText(src)
	lines := splitLines(src)

	edits := []struct {
		name   string
		change func(lines []string) []string
	}{
		{"type in a header", func(l []string) []string { l[3] = "Content-Type: application/xml"; return l }},
		{"type in the next header", func(l []string) []string { l[4] = "Authorization: Bearer {{tok}}"; return l }},
		{"extend a body line", func(l []string) []string { l[7] = `  "name": "Ada Lovelace",`; return l }},
		{"insert a line in the middle", func(l []string) []string {
			return append(l[:5], append([]string{"X-Trace: 1"}, l[5:]...)...)
		}},
		{"insert a blank line separator", func(l []string) []string {
			return append(l[:6], append([]string{""}, l[6:]...)...)
		}},
		{"delete that separator", func(l []string) []string {
			return append(l[:6], l[7:]...)
		}},
		{"turn a comment into a request line", func(l []string) []string { l[1] = "HEAD http://api.example.com/health"; return l }},
		{"delete a whole block", func(l []string) []string { return append(l[:1], l[4:]...) }},
		{"add a separator", func(l []string) []string {
			return append(l[:3], append([]string{"### new"}, l[3:]...)...)
		}},
		{"edit the first line", func(l []string) []string { l[0] = "### Renamed"; return l }},
		{"edit the last line", func(l []string) []string { l[len(l)-1] = "DELETE http://api.example.com/v1/users/2"; return l }},
	}

	for _, ed := range edits {
		lines = ed.change(lines)
		h.update(lines)

		want := highlight(strings.Join(lines, "\n"))
		if !reflect.DeepEqual(h.segs, want) {
			t.Fatalf("after %q the incremental highlight differs from a full lex\n in %d lines\n%s",
				ed.name, len(lines), diffLines(h.segs, want))
		}
	}
}

// diffLines describes the first rows whose segments differ, so a failure is
// readable.
func diffLines(got, want [][]seg) string {
	var b strings.Builder
	shown := 0
	for i := range want {
		var g []seg
		if i < len(got) {
			g = got[i]
		}
		if reflect.DeepEqual(g, want[i]) {
			continue
		}
		b.WriteString(strings.Join([]string{"row " + itoa(i) + ":", "  got  " + segsText(g), "  want " + segsText(want[i]), ""}, "\n"))
		shown++
		if shown == 4 {
			break
		}
	}
	if shown == 0 {
		b.WriteString("(no differing rows: the difference is in the row count)")
	}
	return b.String()
}

func segsText(segs []seg) string {
	var parts []string
	for _, s := range segs {
		parts = append(parts, "["+itoa(s.start)+","+itoa(s.end)+")")
	}
	return strings.Join(parts, " ")
}

func itoa(v int) string {
	if v == 0 {
		return "0"
	}
	neg := v < 0
	if neg {
		v = -v
	}
	var digits []byte
	for v > 0 {
		digits = append([]byte{byte('0' + v%10)}, digits...)
		v /= 10
	}
	if neg {
		return "-" + string(digits)
	}
	return string(digits)
}

// TestHighlighterKeepsBlocksIsolated checks the property the incremental update
// relies on: a block lexes the same alone as it does inside a file, because the
// lexer resets at blank lines and separators.
func TestHighlighterKeepsBlocksIsolated(t *testing.T) {
	whole := splitLines(strings.Join([]string{
		"### one",
		"POST http://a/b",
		"Content-Type: application/json",
		"",
		"{",
		`  "a": 1`,
		"}",
		"",
		"### two",
		"GET http://c/d",
	}, "\n"))

	full := highlight(strings.Join(whole, "\n"))
	// Block 2 only: the rows 8..9.
	part := highlightRange(whole, 8, len(whole))
	for i := 8; i < len(whole); i++ {
		if !reflect.DeepEqual(full[i], part[i]) {
			t.Fatalf("row %d differs when lexed as its own block", i)
		}
	}
}

// TestRequestLineIsHighlighted guards the lexer fix mentioned in the plan: the
// method, URL and version of a request line must carry their own token kinds
// (they were emitted with Line 0 and never reached any per-line highlighter).
func TestRequestLineIsHighlighted(t *testing.T) {
	segs := highlight("GET http://example.com/x HTTP/1.1")
	if len(segs) != 1 {
		t.Fatalf("expected one row, got %d", len(segs))
	}
	kinds := map[string]bool{}
	line := "GET http://example.com/x HTTP/1.1"
	for _, s := range segs[0] {
		kinds[line[s.start:s.end]] = true
	}
	for _, want := range []string{"GET", "http://example.com/x", "HTTP/1.1"} {
		if !kinds[want] {
			t.Errorf("token %q missing from the highlighted request line: %v", want, kinds)
		}
	}
}

// TestHighlightJSONBodySplitsSegments covers the ported JSON colouring: a JSON
// body line is no longer one flat run.
func TestHighlightJSONBodySplitsSegments(t *testing.T) {
	segs := highlight("POST http://a\nContent-Type: application/json\n\n{ \"name\": \"Ada\", \"n\": 1 }")
	if len(segs) != 4 {
		t.Fatalf("expected 4 rows, got %d", len(segs))
	}
	body := segs[3]
	if len(body) < 4 {
		t.Fatalf("JSON body line produced %d segments: %+v", len(body), body)
	}

	kinds := map[jsonlex.Kind]bool{}
	for _, s := range body {
		if s.jsonKind {
			kinds[s.json] = true
		}
	}
	for _, want := range []jsonlex.Kind{jsonlex.Key, jsonlex.String, jsonlex.Number} {
		if !kinds[want] {
			t.Errorf("JSON kind %v missing: %v", want, kinds)
		}
	}
}

// TestJSONBodyKeepsVariablesHighlighted makes sure splitting a body line for JSON
// does not lose the {{variable}} highlighting inside it.
func TestJSONBodyKeepsVariablesHighlighted(t *testing.T) {
	segs := highlight("POST http://a\nContent-Type: application/json\n\n{ \"token\": \"{{token}}\" }")
	var vars int
	for _, s := range segs[3] {
		if !s.jsonKind && s.kind == httpfile.TokVariable {
			vars++
		}
	}
	if vars != 1 {
		t.Fatalf("expected the {{token}} span to stay a variable, found %d", vars)
	}
}

// TestNonJSONBodyStaysFlat keeps the fallback: a body that is not JSON is drawn
// with the plain body colour.
func TestNonJSONBodyStaysFlat(t *testing.T) {
	segs := highlight("POST http://a\nContent-Type: text/plain\n\nnot json at all")
	for _, s := range segs[3] {
		if s.jsonKind {
			t.Fatalf("a non-JSON body line was JSON-coloured: %+v", s)
		}
	}
}

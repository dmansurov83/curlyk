package httpfile

import "testing"

// TestRequestLineTokensCarryLineNumber pins the fix for the request line being
// tokenized with Line 0. Consumers pick tokens per line by tk.Line-1, so those
// tokens used to match no line at all and lost their highlighting.
func TestRequestLineTokensCarryLineNumber(t *testing.T) {
	src := "### Demo\n" +
		"GET https://example.com/a HTTP/1.1\n" +
		"Accept: text/plain\n"

	want := map[TokenType]int{
		TokSeparator:   1,
		TokMethod:      2,
		TokURL:         2,
		TokHTTPVersion: 2,
		TokHeaderName:  3,
		TokHeaderValue: 3,
	}

	seen := map[TokenType]int{}
	for _, tk := range Lex(src) {
		if _, tracked := want[tk.Type]; tracked {
			seen[tk.Type] = tk.Line
		}
	}

	for typ, line := range want {
		if got := seen[typ]; got != line {
			t.Errorf("%v: Line=%d, want %d", typ, got, line)
		}
	}
}

// TestEveryTokenHasALine checks that no token escapes line numbering, on a
// source that exercises every branch of Lex.
func TestEveryTokenHasALine(t *testing.T) {
	src := "### Demo\n" +
		"# comment\n" +
		"@timeout 5s\n" +
		"POST https://example.com/a\n" +
		"Content-Type: application/json\n" +
		"Authorization: Bearer {{token}}\n" +
		"\n" +
		"{\"active\": true}\n" +
		"GET https://example.com/b\n" +
		"    ?group=day HTTP/1.1\n"

	for _, tk := range Lex(src) {
		if tk.Line < 1 {
			t.Errorf("token %v %q has Line=%d", tk.Type, tk.Value, tk.Line)
		}
	}
}

// TestTokenLinesMatchSourceLines verifies each reported line really contains the
// token value, which catches off-by-one errors in line numbering.
func TestTokenLinesMatchSourceLines(t *testing.T) {
	src := "### Demo\n" +
		"GET https://example.com/a HTTP/1.1\n" +
		"Accept: text/plain\n" +
		"\n" +
		"body\n"

	srcLines := splitTestLines(src)
	for _, tk := range Lex(src) {
		if tk.Type == TokOther || tk.Value == "" {
			continue
		}
		if tk.Line > len(srcLines) {
			t.Fatalf("token %q points at line %d, source has %d", tk.Value, tk.Line, len(srcLines))
		}
		line := srcLines[tk.Line-1]
		if tk.End > len(line) || line[tk.Start:tk.End] != tk.Value {
			t.Errorf("token %q at %d..%d does not match line %d (%q)",
				tk.Value, tk.Start, tk.End, tk.Line, line)
		}
	}
}

func splitTestLines(src string) []string {
	var out []string
	start := 0
	for i := 0; i < len(src); i++ {
		if src[i] == '\n' {
			out = append(out, src[start:i])
			start = i + 1
		}
	}
	return append(out, src[start:])
}

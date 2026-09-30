package httpfile

import (
	"strings"
)

// TokenType classifies a lexeme in a .http file.
type TokenType int

const (
	TokComment    TokenType = iota // # or // comment line
	TokSeparator                   // ### request separator
	TokMethod                      // GET, POST, ...
	TokURL                         // request target on the request line
	TokHTTPVersion                 // trailing HTTP/1.1
	TokHeaderName                  // header name before ':'
	TokHeaderValue                 // header value after ':'
	TokBodyText                    // raw body line
	TokVariable                    // a {{ ... }} placeholder (may be embedded)
	TokOther                       // anything not classified
)

// VarSpan locates a {{...}} variable inside a token's Value.
type VarSpan struct{ Start, End int }

// Token is a single lexeme with its source location (line 1-based, byte offsets within line).
type Token struct {
	Type  TokenType
	Value string
	Line  int       // 1-based
	Start int       // byte offset within the line
	End   int       // byte offset past the match
	Vars  []VarSpan // {{...}} spans relative to Value
}

// lexLine is a lazily chunked token stream; we produce one slice per file.
// Line holds both body-state and the parsed pieces.

// Lex splits file content into a flat token list, one set of tokens per line.
// Used by both the parser (event reconstruction) and the highlighter (style map).
func Lex(src string) []Token {
	var toks []Token
	lines := strings.Split(src, "\n")
	inBody := false

	for li, raw := range lines {
		line := strings.TrimSuffix(raw, "\r")
		lineNo := li + 1
		var lineToks []Token

		// Blank line: ends body. Special-case: also ends headers.
		if strings.TrimSpace(line) == "" {
			inBody = false
			lineToks = append(lineToks, Token{Type: TokOther, Value: "", Line: lineNo})
			toks = append(toks, lineToks...)
			continue
		}

		// Separator ### ...
		if strings.HasPrefix(line, "###") {
			inBody = false
			toks = append(toks, Token{Type: TokSeparator, Value: line, Line: lineNo, Start: 0, End: len(line)})
			continue
		}

		// Comment
		if strings.HasPrefix(line, "#") || strings.HasPrefix(line, "//") {
			if inBody {
				// a comment inside body is still body text
				toks = append(toks, bodyToken(line, lineNo, nil))
				continue
			}
			toks = append(toks, Token{Type: TokComment, Value: line, Line: lineNo, Start: 0, End: len(line)})
			continue
		}

		// If we're inside a body, everything is body text.
		if inBody {
			toks = append(toks, bodyToken(line, lineNo, nil))
			continue
		}

		// Request line: METHOD URL [HTTP/x.y]
		if m, urlStr, httpVer, ok := parseRequestLine(line); ok {
			lt := tokenizeRequestLine(line, m, urlStr, httpVer)
			for _, t := range lt {
				toks = append(toks, t)
			}
			// After a request line: headers next, body not yet started.
			continue
		}

		// Header line: Name: value
		if name, _, ok := splitHeaderLine(line); ok {
			nameEnd := len(name)
			// name token
			nt := Token{Type: TokHeaderName, Value: name, Line: lineNo, Start: 0, End: nameEnd,
				Vars: findVars(name)}
			// value token (starts after ": ")
			valStart := nameEnd + 1
			for valStart < len(line) && line[valStart] == ' ' {
				valStart++
			}
			vt := Token{Type: TokHeaderValue, Value: line[valStart:], Line: lineNo, Start: valStart, End: len(line),
				Vars: findVars(line[valStart:])}
			toks = append(toks, nt, vt)
			continue
		}

		// Bare line with no request/header seen yet: treat as body.
		toks = append(toks, bodyToken(line, lineNo, nil))
		inBody = true
	}

	return toks
}

func bodyToken(line string, lineNo int, vars []VarSpan) Token {
	t := Token{Type: TokBodyText, Value: line, Line: lineNo, Start: 0, End: len(line)}
	if vars == nil {
		t.Vars = findVars(line)
	}
	return t
}

// IsRequestLine reports whether the given line is an HTTP request line
// ("METHOD URL [HTTP/1.1]").
func IsRequestLine(line string) bool {
	_, _, _, ok := parseRequestLine(line)
	return ok
}

// parseRequestLine extracts METHOD, URL and optional HTTP version from a line.
func parseRequestLine(line string) (method, urlStr, httpVer string, ok bool) {
	i := 0
	for i < len(line) && (line[i] == ' ' || line[i] == '\t') {
		i++
	}
	start := i
	for i < len(line) && isMethodChar(line[i]) {
		i++
	}
	if i == start {
		return "", "", "", false
	}
	m := line[start:i]
	if !isKnownMethod(m) {
		return "", "", "", false
	}
	// skip spaces
	for i < len(line) && (line[i] == ' ' || line[i] == '\t') {
		i++
	}
	if i >= len(line) {
		return "", "", "", false
	}
	rest := line[i:]
	// optional trailing HTTP version
	httpVer = ""
	if vi := findHTTPVersion(rest); vi >= 0 {
		httpVer = strings.TrimSpace(rest[vi:])
		rest = strings.TrimRight(rest[:vi], " ")
	}
	if rest == "" {
		return "", "", "", false
	}
	return m, rest, httpVer, true
}

func findHTTPVersion(s string) int {
	idx := strings.Index(s, "HTTP/")
	if idx < 0 {
		return -1
	}
	// ensure it starts at a word boundary
	if idx > 0 && isMethodChar(s[idx-1]) {
		return -1
	}
	return idx
}

// tokenizeRequestLine builds method / URL / version tokens given parsed pieces.
func tokenizeRequestLine(line, method, urlStr, httpVer string) []Token {
	lineNo := 0 // filled by caller via append; we set Line later is irrelevant
	// We recompute offsets by scanning.
	mIdx := strings.Index(line, method)
	uIdx := mIdx + len(method)
	for uIdx < len(line) && (line[uIdx] == ' ' || line[uIdx] == '\t') {
		uIdx++
	}
	toks := []Token{
		{Type: TokMethod, Value: method, Line: lineNo, Start: mIdx, End: mIdx + len(method)},
		{Type: TokURL, Value: urlStr, Line: lineNo, Start: uIdx, End: uIdx + len(urlStr), Vars: findVars(urlStr)},
	}
	if httpVer != "" {
		vEnd := len(line)
		for vEnd > uIdx && (line[vEnd-1] == ' ' || line[vEnd-1] == '\r') {
			vEnd--
		}
		// locate the start of the version token
		vStart := findHTTPVersion(line) // absolute
		if vStart < 0 {
			vStart = vEnd
		}
		toks = append(toks, Token{Type: TokHTTPVersion, Value: httpVer, Line: lineNo, Start: vStart, End: vEnd})
	}
	return toks
}

// splitHeaderLine splits "Name: value".
func splitHeaderLine(line string) (string, string, bool) {
	idx := strings.IndexRune(line, ':')
	if idx <= 0 {
		return "", "", false
	}
	name := line[:idx]
	if strings.ContainsAny(name, " \t") {
		return "", "", false
	}
	val := line[idx+1:]
	val = strings.TrimLeft(val, " ")
	return name, val, true
}

func isMethodChar(c byte) bool {
	return (c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z') || (c >= '0' && c <= '9')
}

var knownMethods = map[string]bool{
	"GET": true, "POST": true, "PUT": true, "DELETE": true, "PATCH": true,
	"HEAD": true, "OPTIONS": true, "CONNECT": true, "TRACE": true,
}

func isKnownMethod(s string) bool { return knownMethods[strings.ToUpper(s)] }

// findVars locates {{...}} spans in a string, relative to s.
func findVars(s string) []VarSpan {
	var spans []VarSpan
	for i := 0; i < len(s); {
		open := strings.Index(s[i:], "{{")
		if open < 0 {
			break
		}
		start := i + open
		closeIdx := strings.Index(s[start+2:], "}}")
		if closeIdx < 0 {
			break
		}
		end := start + 2 + closeIdx + 2
		spans = append(spans, VarSpan{Start: start, End: end})
		i = end
	}
	return spans
}
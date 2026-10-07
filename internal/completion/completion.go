// Package completion provides a pure, UI-free suggestion engine for .http
// source lines. Given the current editor line and the cursor column (in runes)
// it classifies what the user is typing and returns replacement suggestions
// (HTTP methods, URL schemes/hosts, header names, media types and variables).
//
// The engine never touches the editor buffer; it only inspects the single line
// and returns ranges describing what an accept should replace. Callers (the TUI)
// collect candidate words into a Context and apply accepted suggestions.
package completion

import "strings"

// Suggestion is one completion candidate.
//
// Start and End are rune columns (0-based, relative to the line passed to
// Suggest) of the text an accept replaces with Snippet. Usually End is the
// cursor column and Start is the start of the typed prefix.
//
// Var marks a variable completion: the surrounding "{{" is open but the closing
// "}}" may or may not be typed yet. The TUI appends "}}" after inserting the
// snippet unless one already follows.
type Suggestion struct {
	Label   string // text shown in the list
	Snippet string // text inserted on accept
	Start   int    // rune col where replacement begins
	End     int    // rune col where replacement ends
	Var     bool   // true for {{name}} variable completions
}

// Context carries the candidate words the engine filters against. Each slice is
// collected by the caller (TUI) from the current file, the active profile and
// built-in function names.
type Context struct {
	Methods []string // HTTP method names (uppercase)
	Hosts   []string // host/domain strings gathered from request URLs and @var host
	Headers []string // header field names
	Vars    []string // variable names usable inside {{...}}
}

// maxResults caps how many suggestions Suggest returns to keep the popup small.
const maxResults = 20

// knownMethodNames is the default method set used when Context.Methods is empty.
var knownMethodNames = []string{"GET", "POST", "PUT", "DELETE", "PATCH", "HEAD", "OPTIONS", "CONNECT", "TRACE"}

// schemeCandidates are the scheme/prefix suggestions offered at the start of a
// URL before a "://" is present.
var schemeCandidates = []string{"http://", "https://", "www."}

// tldSuffixes complete a typed host prefix that already ends with a dot.
var tldSuffixes = []string{"com", "ru", "org", "net", "io", "dev"}

// mediaTypes are offered after a "Content-Type:" header.
var mediaTypes = []string{
	"application/json",
	"application/xml",
	"application/x-www-form-urlencoded",
	"application/octet-stream",
	"multipart/form-data",
	"text/plain",
	"text/html",
}

// defaultHeaders is the standard header-name set used when Context.Headers is
// empty.
var defaultHeaders = []string{
	"Accept", "Accept-Encoding", "Accept-Language", "Authorization",
	"Cache-Control", "Connection", "Content-Length", "Content-Type",
	"Cookie", "Host", "Origin", "Referer", "User-Agent", "X-Api-Key",
}

// Suggest classifies the .http line at cursor column col (rune-based) and
// returns matching suggestions. Multiple contexts can match at once (e.g. a
// single letter at the start of a line is a method prefix and an URL prefix);
// their candidates are merged and de-duplicated by Label.
func Suggest(line string, col int, ctx Context) []Suggestion {
	runes := []rune(line)
	if col < 0 {
		col = 0
	}
	if col > len(runes) {
		col = len(runes)
	}
	ctx = normalize(ctx)

	var out []Suggestion
	add := func(s Suggestion) {
		for _, e := range out {
			if e.Label == s.Label {
				return
			}
		}
		out = append(out, s)
	}

	// Variable context always applies whenever an open "{{" precedes the cursor.
	if vs, ok := varSuggest(runes, col, ctx); ok {
		for _, s := range vs {
			add(s)
		}
	}

	// Method context: the cursor sits on a letter-only word at the line start
	// that looks like a method being typed. Independent of whether a full
	// request line is recognised yet, so "ge" and "POST" both offer methods.
	if isMethodPrefix(runes, col) {
		for _, s := range methodSuggest(runes, col, ctx) {
			add(s)
		}
	}

	// URL context on a request line; otherwise header context. When a method
	// word is being typed the header branch is not reached (urlStart spuriously
	// matches nothing and header has no candidates for bare GET/POST).
	if us, ok := urlStart(runes); ok {
		for _, s := range urlSuggest(runes, col, us, ctx) {
			add(s)
		}
	} else if hc, ok := headerSuggest(runes, col, ctx); ok {
		for _, s := range hc {
			add(s)
		}
	}

	if len(out) > maxResults {
		out = out[:maxResults]
	}
	// Drop suggestions that would insert no new text: when the current text
	// from Start up to the cursor already equals the Snippet, accepting it would
	// change nothing, and showing it just "duplicates" the typed text. Variable
	// completion always appends "}}" so its snippet genuinely adds text.
	filtered := out[:0]
	for _, s := range out {
		if s.Var {
			filtered = append(filtered, s)
			continue
		}
		if s.End > len(runes) {
			s.End = len(runes)
		}
		if s.Start <= col && s.End >= col && string(runes[s.Start:col]) == s.Snippet {
			continue
		}
		filtered = append(filtered, s)
	}
	return filtered
}

// normalize fills empty slices with their built-in defaults and lowercases the
// host list for prefix matching.
func normalize(ctx Context) Context {
	if len(ctx.Methods) == 0 {
		ctx.Methods = knownMethodNames
	}
	if len(ctx.Headers) == 0 {
		ctx.Headers = defaultHeaders
	}
	return ctx
}

// varSuggest handles the "{{name"" context: an open "{{" before the cursor with
// no closing "}}" typed yet. Start is the column just after "{{", End is the
// cursor. Snippets carry the variable name without braces (Var=true).
func varSuggest(runes []rune, col int, ctx Context) ([]Suggestion, bool) {
	open := -1
	for i := 0; i+1 < col && i+1 <= len(runes); i++ {
		if runes[i] == '{' && i+1 < len(runes) && runes[i+1] == '{' {
			open = i
		}
	}
	if open < 0 {
		return nil, false
	}
	// A "}}" between the open brace and the cursor closes the variable.
	for j := open + 2; j+1 < col; j++ {
		if runes[j] == '}' && (j+1 >= len(runes) || runes[j+1] == '}') {
			return nil, false
		}
	}
	start := open + 2
	prefix := string(runes[start:col])
	var out []Suggestion
	for _, v := range ctx.Vars {
		if !strings.HasPrefix(v, prefix) {
			continue
		}
		out = append(out, Suggestion{Label: v, Snippet: v, Start: start, End: col, Var: true})
	}
	return out, true
}

// isMethodPrefix reports whether the text from the first non-space column up to
// the cursor looks like a method word being typed (letters only).
func isMethodPrefix(runes []rune, col int) bool {
	start := leadingSpaces(runes)
	if col <= start {
		return false
	}
	if col > len(runes) {
		col = len(runes)
	}
	for i := start; i < col; i++ {
		if !isLetter(runes[i]) {
			return false
		}
	}
	return col > start
}

// methodSuggest returns method candidates filtered by the typed prefix
// (case-insensitive), replacing the typed prefix.
func methodSuggest(runes []rune, col int, ctx Context) []Suggestion {
	start := leadingSpaces(runes)
	prefix := strings.ToUpper(string(runes[start:col]))
	var out []Suggestion
	for _, m := range ctx.Methods {
		up := strings.ToUpper(m)
		if !strings.HasPrefix(up, prefix) {
			continue
		}
		out = append(out, Suggestion{Label: up, Snippet: up, Start: start, End: col})
	}
	return out
}

// urlStart reports the rune column where the URL begins, when the line is a
// request line ("METHOD URL") or starts directly with a URL prefix ("http...",
// "www..."). Returns the rune index of the first URL character.
func urlStart(runes []rune) (int, bool) {
	start := leadingSpaces(runes)
	if len(runes) == 0 {
		return 0, false
	}
	// Line starts directly with a URL prefix.
	rest := string(runes[start:])
	low := strings.ToLower(rest)
	if strings.HasPrefix(low, "http") || strings.HasPrefix(low, "www") {
		return start, true
	}
	// Request line: known method followed by whitespace.
	end := start
	for end < len(runes) && runes[end] != ' ' && runes[end] != '\t' {
		end++
	}
	if end == start {
		return 0, false
	}
	method := strings.ToUpper(string(runes[start:end]))
	if !isKnownMethodName(method) {
		return 0, false
	}
	// Skip spaces to the URL.
	url := end
	for url < len(runes) && (runes[url] == ' ' || runes[url] == '\t') {
		url++
	}
	return url, true
}

// urlSuggest handles the URL context: scheme/prefix completion before "://" and
// host/domain completion inside the authority. us is the URL start rune column.
func urlSuggest(runes []rune, col, us int, ctx Context) []Suggestion {
	if col < us {
		col = us
	}
	if col > len(runes) {
		col = len(runes)
	}
	// Find "://" within the URL.
	sc := -1
	for i := us; i+2 < len(runes); i++ {
		if runes[i] == ':' && runes[i+1] == '/' && runes[i+2] == '/' {
			sc = i
			break
		}
	}

	// Host context: cursor is inside the authority (after "://", before the path).
	if sc >= 0 {
		authStart := sc + 3
		// End of the authority: first "/", "?", "#" or ":" (the scheme's own
		// colon is at sc, so scanning from authStart finds a port colon only).
		authEnd := len(runes)
		for i := authStart; i < len(runes); i++ {
			switch runes[i] {
			case '/', '?', '#', ':':
				authEnd = i
				i = len(runes)
			}
		}
		if col >= authStart && col <= authEnd {
			return hostSuggest(runes, col, authStart, authEnd, ctx)
		}
		return nil
	}

	// Scheme/prefix context: before "://" is present. Only fire when the cursor
	// is actually past the method word (a space or URL text follows), so a bare
	// "POST" with no URL yields only method suggestions.
	if us >= len(runes) {
		return nil
	}
	prefix := string(runes[us:col])
	if prefix == "" || allLetters(prefix) {
		var out []Suggestion
		for _, c := range schemeCandidates {
			if strings.HasPrefix(c, prefix) {
				out = append(out, Suggestion{Label: c, Snippet: c, Start: us, End: col})
			}
		}
		for _, h := range ctx.Hosts {
			if strings.HasPrefix(h, prefix) {
				out = append(out, Suggestion{Label: h, Snippet: h, Start: us, End: col})
			}
		}
		if len(out) == 0 && len(ctx.Hosts) > 0 {
			for _, h := range ctx.Hosts {
				out = append(out, Suggestion{Label: h, Snippet: h, Start: us, End: col})
			}
		}
		return out
	}
	return nil
}

// hostSuggest suggests hosts and domain suffixes for the typed authority prefix.
func hostSuggest(runes []rune, col, authStart, authEnd int, ctx Context) []Suggestion {
	if col < authStart {
		col = authStart
	}
	if col > authEnd {
		col = authEnd
	}
	prefix := string(runes[authStart:col])
	var out []Suggestion
	for _, h := range ctx.Hosts {
		if strings.HasPrefix(h, prefix) {
			out = append(out, Suggestion{Label: h, Snippet: h, Start: authStart, End: col})
		}
	}
	// Domain suffix: the typed host so far ends with a dot (e.g. "api.").
	// Only offer suffixes when the part before the dot does not already carry a
	// well-known TLD — otherwise "api.example.com." would spuriously suggest
	// "api.example.com.com".
	if strings.HasSuffix(prefix, ".") && len(prefix) > 1 {
		base := strings.TrimSuffix(prefix, ".")
		if lastDot := strings.LastIndexByte(base, '.'); lastDot >= 0 {
			if isKnownTLD(base[lastDot+1:]) {
				return out
			}
		}
		for _, t := range tldSuffixes {
			label := base + "." + t
			out = append(out, Suggestion{Label: label, Snippet: label, Start: authStart, End: col})
		}
	}
	return out
}

// isKnownTLD reports whether s looks like a top-level domain (com, ru, ...).
func isKnownTLD(s string) bool {
	for _, t := range tldSuffixes {
		if s == t {
			return true
		}
	}
	return false
}

// headerSuggest handles header-name completion (cursor in the field name before
// the ":" or typing the name) and media-type completion in a "Content-Type"
// value. It returns ok=false when the line is not plausibly a header line (e.g.
// a JSON body, or a bare method word that the method context already covers).
func headerSuggest(runes []rune, col int, ctx Context) ([]Suggestion, bool) {
	if len(runes) == 0 {
		return nil, false
	}
	start := leadingSpaces(runes)
	// Reject obvious body/JSON starts.
	if start < len(runes) {
		switch runes[start] {
		case '{', '[', '"', '/', '@':
			return nil, false
		}
	}

	idx := strings.IndexRune(string(runes), ':')
	// If no colon yet, the whole non-space word is the candidate header name.
	// Only fire when the cursor is within that word.
	if idx < 0 {
		if col <= start || col > len(runes) {
			return nil, false
		}
		word := runes[start:col]
		for _, r := range word {
			if !isHeaderNameRune(r) {
				return nil, false
			}
		}
		prefix := strings.ToLower(string(word))
		var out []Suggestion
		for _, h := range ctx.Headers {
			if !strings.HasPrefix(strings.ToLower(h), prefix) {
				continue
			}
			out = append(out, Suggestion{Label: h, Snippet: h, Start: start, End: col})
		}
		return out, col > start
	}

	nameR := runes[start:idx]
	if len(nameR) == 0 {
		return nil, false
	}
	for _, r := range nameR {
		if !isHeaderNameRune(r) {
			return nil, false
		}
	}
	name := string(nameR)

	// Cursor in the value: only the Content-Type value gets media suggestions.
	if col > idx {
		if strings.EqualFold(name, "Content-Type") {
			valStart := idx + 1
			for valStart < len(runes) && runes[valStart] == ' ' {
				valStart++
			}
			if col < valStart {
				col = valStart
			}
			prefix := string(runes[valStart:col])
			var out []Suggestion
			for _, m := range mediaTypes {
				if strings.HasPrefix(m, prefix) {
					out = append(out, Suggestion{Label: m, Snippet: m, Start: valStart, End: col})
				}
			}
			return out, true
		}
		return nil, false
	}

	// Cursor in the header name.
	count := col - start
	if count <= 0 {
		return nil, false
	}
	prefix := strings.ToLower(string(runes[start:col]))
	var out []Suggestion
	for _, h := range ctx.Headers {
		if !strings.HasPrefix(strings.ToLower(h), prefix) {
			continue
		}
		out = append(out, Suggestion{Label: h, Snippet: h, Start: start, End: col})
	}
	return out, count > 0
}

func isHeaderNameRune(r rune) bool {
	return (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') ||
		r == '-' || r == '_'
}

// leadingSpaces returns the number of leading space/tab runes in runes.
func leadingSpaces(runes []rune) int {
	i := 0
	for i < len(runes) && (runes[i] == ' ' || runes[i] == '\t') {
		i++
	}
	return i
}

func isLetter(r rune) bool {
	return (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z')
}

func allLetters(s string) bool {
	for _, r := range s {
		if !isLetter(r) {
			return false
		}
	}
	return s != ""
}

// methodSet is a small case-insensitive method set used to recognise known
// methods during line classification.
func isKnownMethodName(m string) bool {
	for _, k := range knownMethodNames {
		if k == m {
			return true
		}
	}
	return false
}

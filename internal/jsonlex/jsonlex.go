// Package jsonlex tokenizes a single line of JSON text for syntax
// highlighting. It is UI-free: it reports rune ranges and token kinds, and each
// front end (the TUI's lipgloss styler, the Fyne GUI's palette) decides how to
// paint them.
//
// The lexer is deliberately line-local: .http request bodies and response
// bodies are highlighted per line, so a token never spans a line boundary and
// callers can clip a line to a visible window without re-lexing fragments (see
// the TUI's shiftJSONColsForWindow).
package jsonlex

import (
	"bytes"
	"encoding/json"
	"strings"
)

// Kind classifies a JSON lexeme.
type Kind int

const (
	WS     Kind = iota // whitespace / indentation
	Key                // quoted string followed by ':'
	String             // quoted string value
	Number             // -12.3e+4
	Bool               // true / false
	Null               // null
	Punct              // { } [ ] , :
	Other              // stray text, the "…" marker
)

// Token is one lexeme of a line. Start/End are rune indexes into that line,
// End exclusive.
type Token struct {
	Kind  Kind
	Start int
	End   int
}

func isSpace(r rune) bool { return r == ' ' || r == '\t' }

// IsBodyStart reports whether a line plausibly starts a JSON value, so that it
// is coloured with JSON syntax instead of a generic body colour.
func IsBodyStart(line string) bool {
	t := strings.TrimLeft(line, " \t")
	if t == "" {
		return false
	}
	switch t[0] {
	case '{', '[', '"', '}', ']', ',':
		return true
	}
	return false
}

// Lex splits line into JSON tokens with key/value roles resolved: quoted
// strings that are followed by ':' become Kind Key, every other string is
// Kind String.
func Lex(line string) []Token {
	return MarkKeys(line, Tokens(line))
}

// Tokens splits line into raw JSON tokens. Strings honour backslash escapes, so
// a \" inside a value does not terminate the token; each string is reported as
// Kind Key until MarkKeys decides whether it really is one.
func Tokens(line string) []Token {
	runes := []rune(line)
	var toks []Token
	i, n := 0, len(runes)
	for i < n {
		r := runes[i]
		switch {
		case isSpace(r):
			j := i
			for j < n && isSpace(runes[j]) {
				j++
			}
			toks = append(toks, Token{WS, i, j})
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
			toks = append(toks, Token{Key, i, j}) // demoted to String unless it's a key
			i = j
		case r == '{' || r == '}' || r == '[' || r == ']' || r == ',' || r == ':':
			toks = append(toks, Token{Punct, i, i + 1})
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
			toks = append(toks, Token{Number, i, j})
			i = j
		case r >= 'a' && r <= 'z':
			j := i
			for j < n && runes[j] >= 'a' && runes[j] <= 'z' {
				j++
			}
			kind := Other
			switch string(runes[i:j]) {
			case "true", "false":
				kind = Bool
			case "null":
				kind = Null
			}
			toks = append(toks, Token{kind, i, j})
			i = j
		default:
			toks = append(toks, Token{Other, i, i + 1})
			i++
		}
	}
	return toks
}

// MarkKeys demotes key-position strings to Kind String and upgrades those
// directly followed by a ':' punctuation token to Kind Key. It is idempotent,
// so callers may run it on tokens that Lex already marked.
func MarkKeys(line string, toks []Token) []Token {
	runes := []rune(line)
	for i := range toks {
		if toks[i].Kind != Key {
			continue
		}
		isKey := false
		for j := i + 1; j < len(toks); j++ {
			if toks[j].Kind == WS {
				continue
			}
			if toks[j].Kind == Punct && toks[j].Start < len(runes) && runes[toks[j].Start] == ':' {
				isKey = true
			}
			break
		}
		if isKey {
			toks[i].Kind = Key
		} else {
			toks[i].Kind = String
		}
	}
	return toks
}

// Pretty reformats body as JSON indented with two spaces and returns it as text.
// Anything that is not valid JSON (an empty body, plain text, HTML) is returned
// unchanged, so callers can show a response body with it unconditionally.
//
// Both front ends format response bodies through it, so a payload looks the same
// in the console and in the desktop window.
func Pretty(body []byte) string {
	text := string(body)
	trimmed := strings.TrimSpace(text)
	if trimmed == "" || !json.Valid([]byte(trimmed)) {
		return text
	}
	var out bytes.Buffer
	if err := json.Indent(&out, []byte(trimmed), "", "  "); err != nil {
		return text
	}
	return out.String()
}

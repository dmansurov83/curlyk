package curl

import (
	"strings"
)

// ShellSplit splits a shell-like command line into argv tokens,
// honoring single quotes, double quotes and backslash escapes.
// It does NOT perform variable expansion (we keep them verbatim).
func ShellSplit(s string) ([]string, error) {
	var args []string
	var cur strings.Builder
	inSingle := false
	inDouble := false
	started := false

	flush := func(force bool) {
		if started || force {
			args = append(args, cur.String())
			cur.Reset()
			started = false
		}
	}

	runes := []rune(s)
	for i := 0; i < len(runes); i++ {
		c := runes[i]
		switch {
		case inSingle:
			if c == '\'' {
				inSingle = false
			} else {
				cur.WriteRune(c)
				started = true
			}
		case inDouble:
			switch {
			case c == '"':
				inDouble = false
			case c == '\\':
				if i+1 < len(runes) {
					nc := runes[i+1]
					if nc == '"' || nc == '\\' || nc == '$' || nc == '`' {
						cur.WriteRune(nc)
						i++
						started = true
					} else {
						cur.WriteRune(c)
						started = true
					}
				} else {
					cur.WriteRune(c)
				}
			default:
				cur.WriteRune(c)
				started = true
			}
		case c == '\'':
			inSingle = true
		case c == '"':
			inDouble = true
		case c == '\\':
			// A backslash immediately before a newline is a line continuation:
			// it joins physical lines into one command, so the backslash and the
			// following newline are dropped (and pending token is kept).
			if i+1 < len(runes) && (runes[i+1] == '\n' || runes[i+1] == '\r') {
				i++
				if i+1 < len(runes) && runes[i+1] == '\n' {
					i++
				}
				continue
			}
			if i+1 < len(runes) {
				cur.WriteRune(runes[i+1])
				i++
				started = true
			} else {
				cur.WriteRune(c)
			}
		case c == ' ' || c == '\t' || c == '\n' || c == '\r':
			flush(false)
		default:
			cur.WriteRune(c)
			started = true
		}
	}
	flush(true)
	if inSingle || inDouble {
		return args, errUnterminatedQuote
	}
	return args, nil
}

var errUnterminatedQuote = &parseError{msg: "unterminated quote in command line"}

type parseError struct{ msg string }

func (e *parseError) Error() string { return e.msg }

package httpfile

import "strings"

type parseMode int

const (
	modeNone    parseMode = iota // before any request line
	modeHeaders                  // collecting headers after a request line
	modeBody                     // collecting body after headers/blank line
)

// ParseFile parses .http source and returns all request blocks.
func ParseFile(src string) []Request {
	var reqs []Request
	var cur Request
	mode := parseMode(modeNone)
	var bodyLines []string
	seenRequest := false
	bodyStartLine := 0
	bodyEndLine := 0
	// pendingName is an @name annotation seen before its request line
	// (the JetBrains convention places @name above the request line). It is
	// attached to the next request line and cleared on "###" separators.
	pendingName := ""
	// pendingTitle is the text after the "###" separator; it is attached to
	// the next request line as the block title (cleared by flush, overwritten
	// by the next separator).
	pendingTitle := ""
	// fileVars collects "@var name = value" declarations across the whole file.
	// Variables are file-scoped and available to every request block.
	fileVars := map[string]string{}

	// urlCont is dropped in favour of a simpler rule: a blank line in header
	// mode is ignored until the first header is seen (see sawHeader).
	bodyURLEncoded := false

	// sawHeader is true once a header (Name: value) has been seen in the
	// current request block. Leading blank lines before the first header are
	// ignored so the user may leave an empty line after the request line / URL
	// continuation; a blank line after the first header ends the header block.
	sawHeader := false

	// blankInHeaders is true once a blank line was seen in header mode before
	// any header (i.e. the user left an empty line after the request line /
	// URL continuation). Used to stop a following "@name" from overwriting the
	// block name: an @name that appears after such a blank line is deferred as
	// a pending name for the next request instead.
	blankInHeaders := false

	flush := func() {
		if seenRequest {
			if bodyURLEncoded {
				cur.Body = joinURLEncodedBody(bodyLines)
			} else {
				cur.Body = joinBody(bodyLines)
			}
			if bodyStartLine > 0 {
				cur.BodyStart = bodyStartLine
				cur.BodyEnd = bodyEndLine
			}
			cur.Vars = fileVars
			reqs = append(reqs, cur)
		}
		cur = Request{}
		mode = modeNone
		bodyLines = nil
		seenRequest = false
		bodyStartLine = 0
		bodyEndLine = 0
		pendingName = ""
		pendingTitle = ""
		bodyURLEncoded = false
		sawHeader = false
		blankInHeaders = false
	}

	lines := strings.Split(src, "\n")
	for li, raw := range lines {
		line := strings.TrimSuffix(raw, "\r")
		lineNo := li + 1
		trim := strings.TrimSpace(line)

		// Separator closes the current request block. The text after "###" is
		// kept as the title of the NEXT request block (flush clears any
		// previous pending values, so set it after).
		if strings.HasPrefix(line, "###") {
			flush()
			pendingTitle = separatorTitle(line)
			continue
		}

		// Request line.
		if m, urlStr, _, ok := parseRequestLine(line); ok {
			// Capture pending @name / ### title before flush clears them.
			attachName := pendingName
			attachTitle := pendingTitle
			flush()
			cur.Method = m
			cur.URL = urlStr
			cur.Line = lineNo
			cur.URLEnd = lineNo
			// Attach an @name written above the request line.
			if attachName != "" {
				cur.Name = attachName
			}
			// Attach the "### ..." title of this block.
			cur.Title = attachTitle
			seenRequest = true
			mode = modeHeaders
			continue
		}

		// @name annotation. Accepted both after a request line (as a header-mode
		// annotation) and before the next one (anonymous-request annotation).
		if strings.HasPrefix(trim, "@name") {
			name := strings.TrimSpace(trim[len("@name"):])
			if seenRequest && mode != modeBody && !blankInHeaders {
				cur.Name = name
			} else {
				// Before a request line: defer to the next request line.
				pendingName = name
			}
			continue
		}

		// @var declaration. File-scoped: applies to any request block, wherever
		// it appears. Skipped in body mode so a JSON body containing
		// "@var ..." (e.g. "@var": 1) is not misread as a variable.
		if strings.HasPrefix(trim, "@var") {
			key, val := parseVarLine(trim)
			if key != "" {
				fileVars[key] = val
			}
			continue
		}

		// Processing option, e.g. "@timeout 5s". Only valid in header mode
		// (after the request line, before a blank line / header / body).
		if seenRequest && mode == modeHeaders {
			if opt, ok := parseOptionLine(line); ok {
				cur.Options = append(cur.Options, opt)
				continue
			}
		}

		// Comment handling depends on mode. A blank line in header mode before the
		// first header is ignored (not a body separator), so an empty line may be
		// left after the request line / URL continuation and before the headers.
		if trim == "" {
			// Blank line ends the header block (if in headers mode).
			if mode == modeHeaders && sawHeader {
				mode = modeBody
			} else if mode == modeHeaders && !sawHeader {
				// A leading blank line in header mode is tolerated (no body yet);
				// record it so a following "@name" is not attached to this block.
				blankInHeaders = true
			} else if mode == modeBody {
				bodyLines = append(bodyLines, "") // preserve blank lines inside body
				bodyEndLine = lineNo
			}
			continue
		}

		if mode == modeBody {
			// Everything in body mode is body text.
			if bodyStartLine == 0 {
				bodyStartLine = lineNo
			}
			bodyEndLine = lineNo
			bodyLines = append(bodyLines, line)
			continue
		}

		// Comment lines (outside body) are skipped.
		if strings.HasPrefix(line, "#") || strings.HasPrefix(line, "//") {
			continue
		}

		if mode == modeHeaders {
			// Try a header first.
			if name, val, ok := splitHeaderLine(line); ok {
				cur.Headers = append(cur.Headers, Header{Name: name, Value: val})
				sawHeader = true
				if isURLEncodedContentType(name, val) {
					bodyURLEncoded = true
				}
				continue
			}
			// An indented non-header line is a URL continuation: it concatenates
			// (without a space) into the request line's URL. Comments/options
			// written with indentation are skipped, not treated as URL parts.
			if isURLContinuationLine(line) {
				t := strings.TrimSpace(line)
				if _, ok := parseOptionLine(t); ok {
					continue
				}
				cur.URL += t
				cur.URLEnd = lineNo
				continue
			}
			// Not a header: fall through to body (line with no colon => body start).
			mode = modeBody
			bodyStartLine = lineNo
			bodyEndLine = lineNo
			bodyLines = append(bodyLines, line)
			continue
		}

		// modeNone: ignore stray non-request, non-header lines.
	}
	flush()

	return reqs
}

// parseVarLine parses a "@var name = value" declaration line. The "@var"
// prefix is already trimmed by the caller. It returns the variable name and
// its value. A missing value is allowed (empty string). Names follow the same
// rules as option names (letters, digits, hyphens). Returns ("", "") when the
// line carries no valid name.
func parseVarLine(trim string) (string, string) {
	rest := strings.TrimSpace(trim[len("@var"):])
	if rest == "" {
		return "", ""
	}
	name := rest
	val := ""
	for i := 0; i < len(rest); i++ {
		if rest[i] == '=' {
			name = strings.TrimSpace(rest[:i])
			val = strings.TrimSpace(rest[i+1:])
			break
		}
	}
	if name == "" {
		return "", ""
	}
	if !validAnnotationName(name) {
		return "", ""
	}
	return name, val
}

// parseOptionLine parses a "@name value" processing-option line. It returns the
// option (offset name/value) and true on success. Names must be non-empty and
// made entirely of lower-case ASCII letters and hyphens (timeout, no-redirect,
// insecure). Anything else, including a stray "@" that is not a known option
// line, yields ok=false so the parser can fall back to header/body handling.
func parseOptionLine(line string) (Option, bool) {
	if !strings.HasPrefix(line, "@") {
		return Option{}, false
	}
	rest := strings.TrimLeft(line[1:], " \t")
	name := rest
	value := ""
	if i := strings.IndexRune(rest, ' '); i >= 0 {
		name = rest[:i]
		value = strings.TrimSpace(rest[i+1:])
	}
	if name == "" {
		return Option{}, false
	}
	if !validAnnotationName(name) {
		return Option{}, false
	}
	return Option{Name: name, Value: value}, true
}

// validAnnotationName reports whether s is a valid name for an "@name"/"@var"
// annotation or a processing option: non-empty and made of ASCII letters, digits
// and hyphens.
func validAnnotationName(s string) bool {
	if s == "" {
		return false
	}
	for i := 0; i < len(s); i++ {
		c := s[i]
		ok := (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '-'
		if !ok {
			return false
		}
	}
	return true
}

// joinBody trims leading blank lines of the body and returns it joined.
func joinBody(lines []string) string {
	// trim empty lines from the top only
	start := 0
	for start < len(lines) && strings.TrimSpace(lines[start]) == "" {
		start++
	}
	joined := strings.Join(lines[start:], "\n")
	// trim trailing whitespace
	return strings.TrimRight(joined, " \t\r\n")
}

// isURLEncodedContentType reports whether an HTTP header line declares a
// application/x-www-form-urlencoded content type. Used to decide whether the
// body is joined from multi-line urlencoded parts.
func isURLEncodedContentType(name, val string) bool {
	if !strings.EqualFold(name, "Content-Type") {
		return false
	}
	return strings.Contains(strings.ToLower(val), "application/x-www-form-urlencoded")
}

// joinURLEncodedBody flattens a multi-line urlencoded body into a single line,
// normalising the JetBrains-style layout: whitespace around a key/value
// separator and around a leading "&" is dropped, and separate "&" lines fold
// into the previous value. The result contains no newlines and no stray spaces
// around separators, so it can be sent as-is.
func joinURLEncodedBody(lines []string) string {
	var out string
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		// A leading separator on its own attaches to the previous part.
		leadingAmp := strings.HasPrefix(line, "&")
		if leadingAmp {
			line = strings.TrimLeft(line, "&")
			line = strings.TrimSpace(line)
		}
		// Normalise "key = value" to "key=value".
		line = normalizeURLEncodedPart(line)
		if line == "" {
			continue
		}
		if out != "" {
			out += "&"
		}
		out += line
	}
	out = strings.TrimLeft(out, "&")
	return out
}

// normalizeURLEncodedPart trims whitespace around a single "=" in a
// key=value urlencoded part. Parts without "=" are returned trimmed.
func normalizeURLEncodedPart(part string) string {
	eq := strings.IndexByte(part, '=')
	if eq < 0 {
		return strings.TrimSpace(part)
	}
	return strings.TrimSpace(part[:eq]) + "=" + strings.TrimSpace(part[eq+1:])
}

// separatorTitle extracts the human-readable title from a "###" separator
// line: the text after the leading "#"s, trimmed of surrounding whitespace.
// "###" alone or "###   " yields an empty title.
func separatorTitle(line string) string {
	return strings.TrimSpace(strings.TrimLeft(line, "#"))
}

// GetRequestAtLine finds the request block whose Line <= targetLine,
// preferring the most recent request that starts at or before the cursor line.
func GetRequestAtLine(reqs []Request, targetLine int) *Request {
	best := -1
	for i := range reqs {
		if reqs[i].Line <= targetLine {
			best = i
		} else {
			break
		}
	}
	if best < 0 {
		return nil
	}
	return &reqs[best]
}

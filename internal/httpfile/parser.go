package httpfile

import "strings"

type parseMode int

const (
	modeNone parseMode = iota // before any request line
	modeHeaders               // collecting headers after a request line
	modeBody                  // collecting body after headers/blank line
)

// ParseFile parses .http source and returns all request blocks.
func ParseFile(src string) []Request {
var reqs []Request
	var cur Request
	mode := modeNone
	var bodyLines []string
	seenRequest := false
	bodyStartLine := 0
	bodyEndLine := 0

	flush := func() {
		if seenRequest {
			cur.Body = joinBody(bodyLines)
			if bodyStartLine > 0 {
				cur.BodyStart = bodyStartLine
				cur.BodyEnd = bodyEndLine
			}
			reqs = append(reqs, cur)
		}
		cur = Request{}
		mode = modeNone
		bodyLines = nil
		seenRequest = false
		bodyStartLine = 0
		bodyEndLine = 0
	}

	lines := strings.Split(src, "\n")
	for li, raw := range lines {
		line := strings.TrimSuffix(raw, "\r")
		lineNo := li + 1
		trim := strings.TrimSpace(line)

		// Separator closes the current request block.
		if strings.HasPrefix(line, "###") {
			flush()
			continue
		}

		// Request line.
		if m, urlStr, _, ok := parseRequestLine(line); ok {
			flush()
			cur.Method = m
			cur.URL = urlStr
			cur.Line = lineNo
			seenRequest = true
			mode = modeHeaders
			continue
		}

		// @name annotation.
		if strings.HasPrefix(trim, "@name") {
			if !seenRequest || mode == modeBody {
				// annotation before a request: may attach later; keep simple: skip
				// (JetBrains puts @name before the request line)
			}
			cur.Name = strings.TrimSpace(trim[len("@name"):])
			continue
		}

		// Comment handling depends on mode.
		if trim == "" {
			// Blank line ends the header block (if in headers mode).
			if mode == modeHeaders {
				mode = modeBody
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
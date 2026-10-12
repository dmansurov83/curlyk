package tui

import (
	"strings"

	"github.com/user/curlyk/internal/httpfile"
)

// viewCache memoizes the derived views of the two big pieces of text the UI
// works from: the editor buffer (its token stream and its parsed request blocks)
// and the response (its header rows and body lines).
//
// All four are pure functions of that text, and all four are rebuilt on every
// frame — or, for the buffer, on every keystroke. They allocate proportionally
// to the text, so without memoization a large .http file or a large response made
// painting and typing noticeably more expensive than the edit itself.
//
// The cache key is the source text itself rather than a mutation counter: joining
// the buffer lines and comparing strings is far cheaper than lexing or parsing
// them, and it stays correct however the text was changed (the editor buffer,
// the response and the header block are all assigned directly in places).
//
// The cache lives behind a pointer because bubbletea copies the model by value on
// every update: a plain field would be discarded with the copy. The editor buffer
// itself is shared the same way.
type viewCache struct {
	// bufText is the buffer text the entries below were derived from.
	bufText string
	toks    []httpfile.Token
	reqs    []httpfile.Request
	// reqsOK marks a finished parse, so an empty request list is not re-parsed.
	reqsOK bool

	// respKey is respHeader + "\x00" + response, the input of the split below.
	respKey  string
	respHdr  []string
	respBody []string
	respOK   bool
}

// cacheRef returns the model's view cache, creating it on first use. A model
// built by hand (not through New) simply gets a fresh cache per call: the values
// stay correct, only the reuse is lost.
func (m *model) cacheRef() *viewCache {
	if m.cache == nil {
		m.cache = &viewCache{}
	}
	return m.cache
}

// lexedTokens returns the token stream of the current editor buffer, reusing the
// previous result while the buffer text is unchanged.
func (m *model) lexedTokens() []httpfile.Token {
	c := m.cacheRef()
	text := m.ed.Text()
	if c.toks != nil && c.bufText == text {
		return c.toks
	}
	c.bufText = text
	c.toks = httpfile.Lex(text)
	c.reqs, c.reqsOK = nil, false
	return c.toks
}

// parsedRequests returns the request blocks of the current editor buffer,
// reusing the previous parse while the buffer text is unchanged. The returned
// slice is shared: callers that need to mutate a request must use
// requestForCursor, which hands out a private copy.
func (m *model) parsedRequests() []httpfile.Request {
	c := m.cacheRef()
	text := m.ed.Text()
	if c.bufText != text {
		c.bufText = text
		c.toks = nil // re-lexed on demand
		c.reqs, c.reqsOK = nil, false
	}
	if !c.reqsOK {
		c.reqs = httpfile.ParseFile(text)
		c.reqsOK = true
	}
	return c.reqs
}

// requestForCursor returns the request block containing the cursor line, or nil
// when the cursor is not inside one. The block is a private copy (headers
// included), so resolving {{...}} placeholders in it never writes back into the
// cached parse — a second run of the same request must resolve a fresh
// {{$random.uuid}} instead of reusing the substituted text.
func (m *model) requestForCursor() *httpfile.Request {
	req := httpfile.GetRequestAtLine(m.parsedRequests(), m.ed.curRow+1)
	if req == nil {
		return nil
	}
	clone := *req
	clone.Headers = append([]httpfile.Header(nil), req.Headers...)
	return &clone
}

// respLines splits the response pane content into its fixed header rows and its
// scrollable body lines, reusing the previous split while the response and its
// header block are unchanged. The header block is prefixed to the response text
// when it was rendered, so the body starts after the header rows and the blank
// separator that follows them.
func (m *model) respLines() (hdr, body []string) {
	c := m.cacheRef()
	key := m.respHeader + "\x00" + m.response
	if c.respOK && c.respKey == key {
		return c.respHdr, c.respBody
	}
	if h := strings.TrimRight(m.respHeader, "\n"); h != "" {
		c.respHdr = strings.Split(h, "\n")
	} else {
		c.respHdr = nil
	}
	lines := strings.Split(m.response, "\n")
	switch {
	case len(c.respHdr) == 0:
		c.respBody = lines
	case len(c.respHdr)+1 < len(lines):
		c.respBody = lines[len(c.respHdr)+1:]
	default:
		c.respBody = nil
	}
	c.respKey = key
	c.respOK = true
	return c.respHdr, c.respBody
}

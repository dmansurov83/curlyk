package tui

import (
	"strings"
	"testing"

	"github.com/user/curlyk/internal/httpfile"
)

// TestRequestForCursorLeavesCachedParseIntact verifies that resolving variables
// in the request handed to "run" / "copy as cURL" never writes back into the
// cached parse. Otherwise a second run of the same request would reuse the
// already-substituted text — sending the same {{$random.uuid}} twice and losing
// every other generated value.
func TestRequestForCursorLeavesCachedParseIntact(t *testing.T) {
	m := New(Args{Width: 120, Height: 30}).(model)
	m.ed.SetText("@var host = example.com\nGET http://{{host}}/x\nAccept: text/plain\n")
	m.ed.curRow = 1

	first := m.requestForCursor()
	if first == nil {
		t.Fatal("request under the cursor not found")
	}
	if first.URL != "http://{{host}}/x" {
		t.Fatalf("URL before resolution=%q", first.URL)
	}
	if err := m.resolveRequestVars(first); err != nil {
		t.Fatalf("resolveRequestVars: %v", err)
	}
	if first.URL != "http://example.com/x" {
		t.Errorf("resolved URL=%q want http://example.com/x", first.URL)
	}

	// The cached parse must still hold the raw placeholders.
	second := m.requestForCursor()
	if second == nil {
		t.Fatal("request under the cursor not found on the second lookup")
	}
	if second.URL != "http://{{host}}/x" {
		t.Errorf("cached parse was mutated: URL=%q", second.URL)
	}
	if got := second.Headers[0].Value; got != "text/plain" {
		t.Errorf("cached header mutated: %q", got)
	}
	if got := m.parsedRequests()[0].URL; got != "http://{{host}}/x" {
		t.Errorf("parsedRequests was mutated: %q", got)
	}
}

// TestParsedRequestsFollowBufferEdits verifies the request cache is invalidated
// when the buffer text changes, however the change was made (an editor method or
// a direct lines assignment, as the action menu and the form editor do).
func TestParsedRequestsFollowBufferEdits(t *testing.T) {
	m := New(Args{Width: 120, Height: 30}).(model)
	m.ed.SetText("GET http://a/1\n")
	if reqs := m.parsedRequests(); len(reqs) != 1 || reqs[0].Method != "GET" {
		t.Fatalf("initial parse=%+v", reqs)
	}

	m.ed.lines = []string{"POST http://a/2", ""}

	reqs := m.parsedRequests()
	if len(reqs) != 1 || reqs[0].Method != "POST" {
		t.Errorf("request cache went stale after a direct buffer edit: %+v", reqs)
	}
	if !hasTokenValue(m.lexedTokens(), "POST") {
		t.Error("token cache went stale after a direct buffer edit")
	}
}

// hasTokenValue reports whether the token stream contains a token with value.
func hasTokenValue(toks []httpfile.Token, value string) bool {
	for _, tk := range toks {
		if tk.Value == value {
			return true
		}
	}
	return false
}

// TestRespLinesCacheFollowsDirectAssignment verifies the response split is
// invalidated when the response or its header block is assigned directly (as the
// tests and the fixed-header rendering do), not only through setResponse.
func TestRespLinesCacheFollowsDirectAssignment(t *testing.T) {
	m := New(Args{Width: 120, Height: 30}).(model)
	m.respHeader = "HTTP/1.1 200 OK\n"
	m.response = "HTTP/1.1 200 OK\n\nbody one\n"
	if got := strings.Join(respBodyLines(&m), "|"); got != "body one|" {
		t.Fatalf("body=%q", got)
	}
	if got := respHeaderCount(&m); got != 1 {
		t.Fatalf("header rows=%d want 1", got)
	}

	m.response = "HTTP/1.1 200 OK\n\nbody two\nmore\n"
	if got := strings.Join(respBodyLines(&m), "|"); got != "body two|more|" {
		t.Errorf("stale body after assignment: %q", got)
	}

	m.respHeader = ""
	if got := strings.Join(respPaneLines(&m), "|"); got != "HTTP/1.1 200 OK||body two|more|" {
		t.Errorf("pane lines without a header block: %q", got)
	}
}

package tui

import (
	"net/http"
	"strings"
	"testing"
)

// TestDumpRequest verifies dumpRequest renders the assembled request line and
// its headers deterministically (sorted), so the resolved multi-line URL and
// headers are visible above the response.
func TestDumpRequest(t *testing.T) {
	urlStr := "http://example.com:8080/api/items?id=1"
	req, err := http.NewRequest("GET", urlStr, nil)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("X-Test", "abc")

	got := dumpRequest(req, "")
	if !strings.Contains(got, "GET http://example.com:8080/api/items?id=1") {
		t.Errorf("dump missing request line:\n%s", got)
	}
	if !strings.Contains(got, "Accept: application/json") {
		t.Errorf("dump missing Accept header:\n%s", got)
	}
	if !strings.Contains(got, "X-Test: abc") {
		t.Errorf("dump missing X-Test header:\n%s", got)
	}
}

// TestDumpRequestBody verifies the request body is included after a blank line.
func TestDumpRequestBody(t *testing.T) {
	req, err := http.NewRequest("POST", "http://example.com/x", nil)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	got := dumpRequest(req, "{\"a\":1}")
	if !strings.Contains(got, "\n\n{\"a\":1}") {
		t.Errorf("dump missing body after blank line:\n%q", got)
	}
}

// TestDumpRequestNil verifies a nil request yields an empty dump (no panic).
func TestDumpRequestNil(t *testing.T) {
	if got := dumpRequest(nil, ""); got != "" {
		t.Errorf("nil request dump=%q want empty", got)
	}
}

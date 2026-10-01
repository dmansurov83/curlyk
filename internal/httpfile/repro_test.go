package httpfile

import "testing"

// TestJSONBodyNotParsedAsHeader guards against a JSON object line being
// misread as a header (RFC 7230 field-name token validation). A JSON body
// like {"title": ...} must stay in the request body, otherwise formatting
// reports "invalid JSON" because Body is empty.
func TestJSONBodyNotParsedAsHeader(t *testing.T) {
	src := `POST https://httpbin.org/post
Content-Type: application/json
{"title": "тест", "value": 42}
`
	reqs := ParseFile(src)
	if len(reqs) != 1 {
		t.Fatalf("want 1 request, got %d", len(reqs))
	}
	r := reqs[0]
	if r.Body == "" {
		t.Fatalf("JSON body was parsed as a header; Body=%q", r.Body)
	}
	if len(r.Headers) != 1 {
		t.Fatalf("want 1 header, got %#v", r.Headers)
	}
}
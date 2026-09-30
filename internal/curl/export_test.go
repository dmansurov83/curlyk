package curl

import (
	"strings"
	"testing"
)

func TestExportRequest(t *testing.T) {
	out := ExportRequest("POST", "https://x/api", []string{"Content-Type: application/json"}, `{"a":1}`)
	if !strings.Contains(out, "curl") {
		t.Errorf("missing curl: %s", out)
	}
	if !strings.Contains(out, "-X POST") {
		t.Errorf("missing -X POST: %s", out)
	}
	if !strings.Contains(out, "https://x/api") {
		t.Errorf("missing url: %s", out)
	}
	if !strings.Contains(out, `-d '{"a":1}'`) {
		t.Errorf("missing body: %s", out)
	}
}

func TestExportRequestQuoting(t *testing.T) {
	out := ExportRequest("GET", "https://x/a?b=1&c=2", nil, "")
	if !strings.Contains(out, "https://x/a?b=1&c=2") {
		t.Errorf("url should be unquoted when simple: %s", out)
	}
	out2 := ExportRequest("GET", "https://x/a b", nil, "")
	if !strings.Contains(out2, "'https://x/a b'") {
		t.Errorf("url with space should be quoted: %s", out2)
	}
}

func TestExportRoundTrip(t *testing.T) {
	// Export then ImportLine should reproduce the request.
	cmd := ExportRequest("POST", "https://x/api", []string{"X-A: 1"}, `{"k":"v"}`)
	block, err := ImportLine(cmd)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(block, "POST https://x/api") {
		t.Errorf("block missing request line: %s", block)
	}
	if !strings.Contains(block, "X-A: 1") {
		t.Errorf("block missing header: %s", block)
	}
	if !strings.Contains(block, `{"k":"v"}`) {
		t.Errorf("block missing body: %s", block)
	}
}
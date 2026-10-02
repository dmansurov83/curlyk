package httpfile

import (
	"strings"
	"testing"
)

func TestParseVarSection(t *testing.T) {
	src := `@var host = http://api.example.com
@var apiKey = secret
### get
GET {{host}}/users
Authorization: Bearer {{apiKey}}

{"u": "{{apiKey}}"}
`
	reqs := ParseFile(src)
	if len(reqs) != 1 {
		t.Fatalf("want 1 request, got %d", len(reqs))
	}
	r := reqs[0]
	if r.Vars["host"] != "http://api.example.com" {
		t.Errorf("host=%q", r.Vars["host"])
	}
	if r.Vars["apiKey"] != "secret" {
		t.Errorf("apiKey=%q", r.Vars["apiKey"])
	}
}

// TestParseVarScopedToFile verifies @var declarations collected above the first
// request are attached to later blocks too (file scope).
func TestParseVarScopedToFile(t *testing.T) {
	src := `@var base = https://x.test`
	reqs := ParseFile(src + "\n### a\nGET {{base}}/1\n### b\nGET {{base}}/2\n")
	if len(reqs) != 2 {
		t.Fatalf("want 2 requests, got %d", len(reqs))
	}
	for i := 0; i < 2; i++ {
		if reqs[i].Vars["base"] != "https://x.test" {
			t.Errorf("req%d base=%q", i, reqs[i].Vars["base"])
		}
	}
}

// TestParseVarInsideBodyNotMangled verifies a JSON body key like "@var" is left
// untouched (variable parsing only fires in non-body mode).
func TestParseVarInsideBodyNotMangled(t *testing.T) {
	src := `POST http://x/
Content-Type: application/json

{"@var": 1}
`
	reqs := ParseFile(src)
	if len(reqs) != 1 {
		t.Fatalf("want 1 request, got %d", len(reqs))
	}
	if reqs[0].Vars != nil && len(reqs[0].Vars) != 0 {
		t.Errorf("body @var leaked into vars: %+v", reqs[0].Vars)
	}
	if !strings.Contains(reqs[0].Body, `"@var"`) {
		t.Errorf("body mangled: %q", reqs[0].Body)
	}
}

func TestSubstituteURLHeaderBody(t *testing.T) {
	vars := map[string]string{"host": "http://api.example.com", "id": "42"}
	u, err := Substitute("{{host}}/users/{{id}}", vars)
	if err != nil {
		t.Fatalf("substitute URL: %v", err)
	}
	if u != "http://api.example.com/users/42" {
		t.Errorf("URL=%q", u)
	}
	h, err := Substitute("Bearer {{id}}{{id}}", vars)
	if err != nil {
		t.Fatalf("substitute header: %v", err)
	}
	if h != "Bearer 4242" {
		t.Errorf("header=%q", h)
	}
}

func TestSubstituteUndefined(t *testing.T) {
	vars := map[string]string{"host": "http://x"}
	s, err := Substitute("{{host}}/{{missing}}/{{host}}", vars)
	se, ok := err.(*SubstitutionError)
	if !ok {
		t.Fatalf("want *SubstitutionError, got %T (%v)", err, err)
	}
	if se.First() != "missing" {
		t.Errorf("First()=%q", se.First())
	}
	if !strings.Contains(s, "{{missing}}") {
		t.Errorf("unresolved placeholder dropped: %q", s)
	}
	if !strings.Contains(s, "http://x") {
		t.Errorf("resolved part lost: %q", s)
	}
}

func TestSubstituteBuiltins(t *testing.T) {
	vars := map[string]string{}
	// timestamp format check (10 digits)
	ts, err := Substitute("{{$timestamp}}", vars)
	if err != nil {
		t.Fatalf("timestamp: %v", err)
	}
	if len(ts) != 10 {
		t.Errorf("timestamp=%q not 10 digits", ts)
	}
	iso, err := Substitute("{{$isoTimestamp}}", vars)
	if err != nil {
		t.Fatalf("iso: %v", err)
	}
	if !strings.Contains(iso, "T") {
		t.Errorf("iso=%q", iso)
	}
	// uuid format: 8-4-4-4-12 hex digits
	uuid, err := Substitute("{{$random.uuid}}", vars)
	if err != nil {
		t.Fatalf("uuid: %v", err)
	}
	if len(uuid) != 36 {
		t.Errorf("uuid=%q not 36 chars", uuid)
	}
	// guid alias
	guid, _ := Substitute("{{$guid}}", vars)
	if len(guid) != 36 {
		t.Errorf("guid=%q", guid)
	}
	// random.int is numeric
	n, err := Substitute("{{$random.int}}", vars)
	if err != nil {
		t.Fatalf("random.int: %v", err)
	}
	for _, c := range n {
		if c < '0' || c > '9' {
			t.Errorf("random.int=%q not numeric", n)
			break
		}
	}
	// unknown function reports undefined
	if _, err := Substitute("{{$nope}}", vars); err == nil {
		t.Error("unknown $-function should be undefined")
	}
}

// TestSubstituteNoRescan verifies a resolved value containing "{{" is inserted
// verbatim and not substituted again.
func TestSubstituteNoRescan(t *testing.T) {
	vars := map[string]string{"a": "x {{y}}", "y": "z"}
	out, err := Substitute("{{a}}", vars)
	if err != nil {
		t.Fatal(err)
	}
	if out != "x {{y}}" {
		t.Errorf("value re-scanned: %q", out)
	}
}

// TestSubstituteNoVarsUnchanged verifies that a string without placeholders is
// returned byte-for-byte even with an empty variable map.
func TestSubstituteNoVarsUnchanged(t *testing.T) {
	plain := "GET /plain?q=1\nX: y\n"
	out, err := Substitute(plain, nil)
	if err != nil {
		t.Fatal(err)
	}
	if out != plain {
		t.Errorf("plain text changed: %q", out)
	}
}

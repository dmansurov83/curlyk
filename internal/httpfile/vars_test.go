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

// TestSubstituteMissingNestedVar verifies an undefined variable referenced from
// inside another variable's value surfaces as a missing placeholder.
func TestSubstituteMissingNestedVar(t *testing.T) {
	vars := map[string]string{"auth": "Bearer {{token}}"}
	s, err := Substitute("X: {{auth}}", vars)
	se, ok := err.(*SubstitutionError)
	if !ok {
		t.Fatalf("want *SubstitutionError, got %T (%v)", err, err)
	}
	if se.First() != "token" {
		t.Errorf("First()=%q", se.First())
	}
	if !strings.Contains(s, "{{token}}") {
		t.Errorf("nested missing placeholder dropped: %q", s)
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

// TestSubstituteNestedVars verifies that a variable whose value references
// another variable (or built-in) is fully resolved.
func TestSubstituteNestedVars(t *testing.T) {
	vars := map[string]string{"apiKey": "e7e83986-b544-4c0b-bd62-b4d588406ced", "auth": "Api-Key {{apiKey}}"}
	out, err := Substitute("{{auth}}", vars)
	if err != nil {
		t.Fatal(err)
	}
	if out != "Api-Key e7e83986-b544-4c0b-bd62-b4d588406ced" {
		t.Errorf("out=%q", out)
	}
}

// TestSubstituteNestedChain verifies a chain of indirection resolves fully.
func TestSubstituteNestedChain(t *testing.T) {
	vars := map[string]string{"a": "{{b}}", "b": "{{c}}", "c": "root"}
	out, err := Substitute("{{a}}", vars)
	if err != nil {
		t.Fatal(err)
	}
	if out != "root" {
		t.Errorf("chain out=%q", out)
	}
}

// TestSubstituteCycle verifies a self-referencing variable is detected instead
// of looping forever.
func TestSubstituteCycle(t *testing.T) {
	vars := map[string]string{"a": "x {{a}}", "b": "{{c}}", "c": "{{b}}"}
	out, err := Substitute("{{a}}", vars)
	if err == nil {
		t.Fatalf("want cycle error, got %q", out)
	}
	if !strings.Contains(out, "{{a}}") {
		t.Errorf("cyclic placeholder dropped: %q", out)
	}
	// Mutual cycle b <-> c.
	out, err = Substitute("{{b}}", vars)
	if err == nil {
		t.Fatalf("want mutual cycle error, got %q", out)
	}
	if !strings.Contains(out, "{{b}}") {
		t.Errorf("mutual cycle placeholder dropped: %q", out)
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

func TestParseProfileVars(t *testing.T) {
	src := "@var host = http://dev\n@var token = abc\nGET http://ignored\n\n@var empty =\n"
	vars := ParseProfileVars(src)
	if vars["host"] != "http://dev" || vars["token"] != "abc" {
		t.Errorf("ParseProfileVars=%v", vars)
	}
	if _, ok := vars["empty"]; !ok {
		t.Errorf("expected empty var to be present")
	}
}

func TestMergeVarsOverrideLayersLast(t *testing.T) {
	base := map[string]string{"host": "http://local", "only": "base"}
	override := map[string]string{"host": "http://profile", "extra": "x"}
	merged := MergeVars(base, override)
	if merged["host"] != "http://profile" {
		t.Errorf("host=%q want profile (override wins)", merged["host"])
	}
	if merged["extra"] != "x" {
		t.Errorf("extra=%q", merged["extra"])
	}
	if merged["only"] != "base" {
		t.Errorf("only=%q", merged["only"])
	}
	// base must not be mutated
	if base["host"] != "http://local" {
		t.Errorf("base mutated: %v", base)
	}
}

func TestMergeVarsLocalOverProfile(t *testing.T) {
	profile := map[string]string{"host": "http://profile", "only": "p"}
	local := map[string]string{"host": "http://local"}
	// MergeVars(profile, local): local overrides profile => host=local
	merged := MergeVars(profile, local)
	if merged["host"] != "http://local" {
		t.Errorf("host=%q want local", merged["host"])
	}
	if merged["only"] != "p" {
		t.Errorf("only=%q want p (fallback from profile)", merged["only"])
	}
}

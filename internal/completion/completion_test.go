package completion

import (
	"reflect"
	"testing"
)

func ctx() Context {
	return Context{
		Methods: []string{"GET", "POST", "PUT", "DELETE"},
		Hosts:   []string{"api.example.com", "web.example.org"},
		Headers: []string{"Content-Type", "Authorization", "Accept"},
		Vars:    []string{"host", "token", "userId"},
	}
}

func labels(ss []Suggestion) []string {
	var out []string
	for _, s := range ss {
		out = append(out, s.Label)
	}
	return out
}

func contains(list []string, s string) bool {
	for _, e := range list {
		if e == s {
			return true
		}
	}
	return false
}

func TestSuggestMethodPrefix(t *testing.T) {
	// "ge" at start of line → GET, unaffected by case.
	ss := Suggest("ge", 2, ctx())
	got := labels(ss)
	if !reflect.DeepEqual(got, []string{"GET"}) {
		t.Fatalf("method prefix: got %v, want [GET]", got)
	}
}

func TestSuggestMethodCaseInsensitive(t *testing.T) {
	// A fully-typed method adds no new text, so it must NOT be suggested again
	// (that would just duplicate the typed word).
	ss := Suggest("POST", 4, ctx())
	if len(ss) != 0 {
		t.Fatalf("fully-typed method must not be re-suggested, got %v", labels(ss))
	}
}

func TestSuggestSchemePrefix(t *testing.T) {
	// After a known method, "h" starts the URL scheme → http:// and https://.
	line := "GET h"
	ss := Suggest(line, 5, ctx())
	got := labels(ss)
	want := []string{"http://", "https://"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("scheme prefix: got %v, want %v", got, want)
	}
}

func TestSuggestSchemeHostPrefix(t *testing.T) {
	// "GET a" → host candidates starting with a.
	ss := Suggest("GET a", 5, ctx())
	got := labels(ss)
	want := []string{"api.example.com"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("host prefix: got %v, want %v", got, want)
	}
}

func TestSuggestHostAuthority(t *testing.T) {
	// Cursor inside the authority after "://".
	line := "GET http://api"
	col := len(line) // after "api"
	ss := Suggest(line, col, ctx())
	got := labels(ss)
	want := []string{"api.example.com"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("authority: got %v, want %v", got, want)
	}
	// Replacement range covers the typed "api" (authStart=11).
	if len(ss) != 1 {
		t.Fatalf("expected 1 suggestion, got %d", len(ss))
	}
	if ss[0].Start != 11 || ss[0].End != col {
		t.Fatalf("range: got %d..%d, want 11..%d", ss[0].Start, ss[0].End, col)
	}
}

func TestSuggestHostSuffix(t *testing.T) {
	// Typed "api." → "api.com", "api.ru", ... suffixes.
	line := "GET http://api."
	col := len(line)
	ss := Suggest(line, col, ctx())
	var got []string
	for _, s := range ss {
		if s.Start == 11 { // suffix suggestions replace the typed "api."
			got = append(got, s.Label)
		}
	}
	for _, suf := range []string{"api.com", "api.ru", "api.org", "api.net", "api.io", "api.dev"} {
		if !contains(got, suf) {
			t.Fatalf("suffix: missing %q in %v", suf, got)
		}
	}
}

func TestSuggestHeaderName(t *testing.T) {
	// "Con" before the colon → Content-Type.
	line := "Con"
	ss := Suggest(line, 3, ctx())
	got := labels(ss)
	want := []string{"Content-Type"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("header name: got %v, want %v", got, want)
	}
}

func TestSuggestHeaderNameAfterColonNoSuggest(t *testing.T) {
	// Cursor in the value of an unrelated header → no suggestions.
	line := "Authorization: bea"
	ss := Suggest(line, len(line), ctx())
	if len(ss) != 0 {
		t.Fatalf("non-content-type value: got %v, want none", labels(ss))
	}
}

func TestSuggestContentTypeValue(t *testing.T) {
	line := "Content-Type: app"
	ss := Suggest(line, len(line), ctx())
	got := labels(ss)
	want := []string{"application/json", "application/xml", "application/x-www-form-urlencoded", "application/octet-stream"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("media type: got %v, want %v", got, want)
	}
}

func TestSuggestVar(t *testing.T) {
	// Open "{{" with typed prefix "to" → token variable, Var flag set.
	line := "GET http://x/{{to"
	col := len(line)
	ss := Suggest(line, col, ctx())
	if len(ss) != 1 {
		t.Fatalf("var: got %v, want [token]", labels(ss))
	}
	s := ss[0]
	if s.Label != "token" || !s.Var {
		t.Fatalf("var suggestion: %+v", s)
	}
	if s.Start != 15 || s.End != col {
		t.Fatalf("var range: got %d..%d, want 15..%d", s.Start, s.End, col)
	}
}

func TestSuggestVarNoClose(t *testing.T) {
	// A closing "}}" already present → no variable suggestions.
	line := "{{token}}"
	ss := Suggest(line, len(line), ctx())
	if len(ss) != 0 {
		t.Fatalf("closed var: got %v, want none", labels(ss))
	}
}

func TestSuggestDedupAcrossContexts(t *testing.T) {
	// "GE" at start matches the method context. It should not crash and the
	// method suggestion stands alone (no host/scheme noise in method position).
	ss := Suggest("GE", 2, ctx())
	if len(ss) == 0 {
		t.Fatalf("expected at least GET suggestion")
	}
	// The first (method) suggestion must be exact GET with range 0..2.
	if ss[0].Label != "GET" || ss[0].Start != 0 || ss[0].End != 2 {
		t.Fatalf("method suggestion: %+v", ss[0])
	}
}

func TestSuggestEmptyLine(t *testing.T) {
	ss := Suggest("", 0, ctx())
	if len(ss) != 0 {
		t.Fatalf("empty line: got %v, want none", labels(ss))
	}
}

func TestSuggestJsonBodyNotHeader(t *testing.T) {
	// A JSON body line must not produce header suggestions from the ':' inside.
	line := `{"key": "value"}`
	ss := Suggest(line, len(line), ctx())
	if len(ss) != 0 {
		t.Fatalf("json body: got %v, want none", labels(ss))
	}
}

// TestSuggestHostDuplicateSuppressed verifies a host suggestion identical to the
// typed authority is not shown (it would just duplicate the entered text).
func TestSuggestHostDuplicateSuppressed(t *testing.T) {
	line := "GET http://api.example.com"
	col := len(line)
	ss := Suggest(line, col, ctx())
	for _, s := range ss {
		if s.Label == "api.example.com" {
			t.Fatalf("fully-typed host must not be re-suggested, got %v", labels(ss))
		}
	}
}

// TestSuggestNoTLDSpam verifies typing a dot after a well-known TLD does not
// append meaningless suffixes ("api.example.com." must not become "api.example.com.com").
func TestSuggestNoTLDSpam(t *testing.T) {
	line := "GET http://api.example.com."
	col := len(line)
	ss := Suggest(line, col, ctx())
	for _, s := range ss {
		if s.Label == "api.example.com.com" || s.Label == "api.example.com.ru" {
			t.Fatalf("must not suggest dangling TLD spam: %v", labels(ss))
		}
	}
}

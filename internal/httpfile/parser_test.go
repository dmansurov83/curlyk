package httpfile

import "testing"

func TestParseSimple(t *testing.T) {
	src := `### Get todos
GET https://api.example.com/todos/1
Accept: application/json

### Create
POST https://api.example.com/todos
Content-Type: application/json

{"title": "test"}
`
	reqs := ParseFile(src)
	if len(reqs) != 2 {
		t.Fatalf("expected 2 requests, got %d", len(reqs))
	}
	r0 := reqs[0]
	if r0.Method != "GET" || r0.URL != "https://api.example.com/todos/1" {
		t.Errorf("bad req0: %s %s", r0.Method, r0.URL)
	}
	if len(r0.Headers) != 1 || r0.Headers[0].Name != "Accept" {
		t.Errorf("bad headers: %+v", r0.Headers)
	}
	r1 := reqs[1]
	if r1.Method != "POST" {
		t.Errorf("bad method: %s", r1.Method)
	}
	wantBody := `{"title": "test"}`
	if r1.Body != wantBody {
		t.Errorf("bad body: %q", r1.Body)
	}
}

func TestLexHighlight(t *testing.T) {
	src := "GET http://x/api/{{id}}\nContent-Type: application/json\n\n# comment\n"
	toks := Lex(src)
	var types []TokenType
	for _, tk := range toks {
		types = append(types, tk.Type)
	}
	// method, url, headerName, headerValue, then a comment
	found := map[TokenType]bool{}
	for _, tk := range toks {
		found[tk.Type] = true
	}
	if !found[TokMethod] || !found[TokURL] || !found[TokHeaderName] || !found[TokHeaderValue] || !found[TokComment] {
		t.Errorf("missing expected token types: %v", found)
	}
	// vars inside URL token
	for _, tk := range toks {
		if tk.Type == TokURL {
			if len(tk.Vars) != 1 {
				t.Errorf("expected 1 var in URL, got %d", len(tk.Vars))
			}
		}
	}
}

func TestGetRequestAtLine(t *testing.T) {
	src := `### first
GET http://a/1

### second
POST http://a/2
`
	reqs := ParseFile(src)
	if GetRequestAtLine(reqs, 1) != nil {
		t.Error("line 1 should have no request (separator)")
	}
	got := GetRequestAtLine(reqs, 2)
	if got == nil || got.URL != "http://a/1" {
		t.Errorf("line 2 should map to req0, got %+v", got)
	}
	got = GetRequestAtLine(reqs, 5)
	if got == nil || got.URL != "http://a/2" {
		t.Errorf("line 5 should map to req1, got %+v", got)
	}
}

// TestNameAnnotationBeforeRequest verifies the JetBrains convention where @name
// appears above the request line: the annotation attaches to the next request
// and does not leak across a "###" separator.
func TestNameAnnotationBeforeRequest(t *testing.T) {
	src := `### get
@name fetch todos
GET http://a/1

@name orphan
### other
POST http://a/2
`
	reqs := ParseFile(src)
	if len(reqs) != 2 {
		t.Fatalf("want 2 requests, got %d", len(reqs))
	}
	if reqs[0].Name != "fetch todos" {
		t.Errorf("req0 name=%q want 'fetch todos'", reqs[0].Name)
	}
	if reqs[1].Name != "" {
		t.Errorf("req1 name=%q want '' (orphan @name before separator must not attach)", reqs[1].Name)
	}
}

// TestSeparatorTitle verifies the "### ..." text becomes Request.Title of the
// next request block: trimmed of the leading "#"s and whitespace, empty for a
// bare separator, and not leaking across blocks.
func TestSeparatorTitle(t *testing.T) {
	src := `### Создать запись
POST https://api.example.com/todos
Content-Type: application/json

{"title": "test"}

###   
GET https://api.example.com/ping

###
DELETE https://api.example.com/todos/1

GET https://api.example.com/no-title
`
	reqs := ParseFile(src)
	if len(reqs) != 4 {
		t.Fatalf("want 4 requests, got %d", len(reqs))
	}
	if reqs[0].Title != "Создать запись" {
		t.Errorf("req0 title=%q want 'Создать запись'", reqs[0].Title)
	}
	if reqs[1].Title != "" {
		t.Errorf("req1 title=%q want '' (whitespace-only separator)", reqs[1].Title)
	}
	if reqs[2].Title != "" {
		t.Errorf("req2 title=%q want '' (bare separator)", reqs[2].Title)
	}
	// A request without any preceding separator must have an empty title.
	if reqs[3].Title != "" {
		t.Errorf("req3 title=%q want '' (no separator)", reqs[3].Title)
	}
}

// TestNameAnnotationAfterRequest verifies @name after the request line still
// attaches (original placement).
func TestNameAnnotationAfterRequest(t *testing.T) {
	src := "GET http://a/1\n@name inline\n"
	reqs := ParseFile(src)
	if len(reqs) != 1 {
		t.Fatalf("want 1 request, got %d", len(reqs))
	}
	if reqs[0].Name != "inline" {
		t.Errorf("req0 name=%q want 'inline'", reqs[0].Name)
	}
}

// TestParseOptions verifies processing options after the request line are
// collected into Request.Options and headers/body still parse correctly.
func TestParseOptions(t *testing.T) {
	src := `### get
GET https://api.test/users/1
@timeout 5s
@no-redirect
@insecure
Accept: application/json

{"a":1}
`
	reqs := ParseFile(src)
	if len(reqs) != 1 {
		t.Fatalf("want 1 request, got %d", len(reqs))
	}
	r := reqs[0]
	want := []Option{
		{Name: "timeout", Value: "5s"},
		{Name: "no-redirect", Value: ""},
		{Name: "insecure", Value: ""},
	}
	if len(r.Options) != len(want) {
		t.Fatalf("options=%+v want %+v", r.Options, want)
	}
	for i := range want {
		if r.Options[i] != want[i] {
			t.Errorf("option[%d]=%+v want %+v", i, r.Options[i], want[i])
		}
	}
	if len(r.Headers) != 1 || r.Headers[0].Name != "Accept" {
		t.Errorf("headers=%+v", r.Headers)
	}
	if r.Body != `{"a":1}` {
		t.Errorf("body=%q", r.Body)
	}
}

// TestParseOptionNotHeader verifies a bare "@" line or an unknown @-prefixed
// line is not swallowed as an option (so a stray '@' behaves as before).
func TestParseOptionRejectsMalformed(t *testing.T) {
	src := "GET http://a/1\n@\n"
	reqs := ParseFile(src)
	if len(reqs) != 1 {
		t.Fatalf("want 1 request, got %d", len(reqs))
	}
	if len(reqs[0].Options) != 0 {
		t.Errorf("options=%+v want none", reqs[0].Options)
	}
}

// TestLexOption verifies option lines lex to TokOption and are not treated as
// body text.
func TestLexOption(t *testing.T) {
	src := "GET http://x/\n@timeout 5s\n@no-redirect\nAccept: text/plain\n"
	toks := Lex(src)
	var optVals []string
	var hasMethod, hasHeader bool
	for _, tk := range toks {
		switch tk.Type {
		case TokOption:
			optVals = append(optVals, tk.Value)
		case TokMethod:
			hasMethod = true
		case TokHeaderName:
			hasHeader = true
		}
	}
	if len(optVals) != 2 {
		t.Errorf("expected 2 option tokens, got %v", optVals)
	}
	if !hasMethod || !hasHeader {
		t.Errorf("method=%v header=%v", hasMethod, hasHeader)
	}
}

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
package tui

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/user/curlyk/httptool/internal/curl"
	"github.com/user/curlyk/httptool/internal/httpfile"
	"github.com/user/curlyk/httptool/internal/runner"
)

// TestImportThenRun simulates: import cURL -> get .http block -> run it against a local server.
func TestImportThenRun(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"echo":"ok"}`))
	}))
	defer srv.Close()

	block, err := curl.ImportLine(`curl -X GET ` + srv.URL + `/ping`)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(block, "GET "+srv.URL+"/ping") {
		t.Errorf("import block wrong: %s", block)
	}

	reqs := httpfile.ParseFile(block)
	if len(reqs) != 1 || reqs[0].Method != "GET" {
		t.Fatalf("parse failed: %+v", reqs)
	}
	res := runner.Run(context.Background(), reqs[0], runner.Options{})
	if res.Err != nil {
		t.Fatal(res.Err)
	}
	body, _ := runner.BodyBytes(res.Response)
	if string(body) != `{"echo":"ok"}` {
		t.Errorf("body=%q", body)
	}
}

// TestPerformImport verifies the model-level import writes the block into the editor.
func TestPerformImport(t *testing.T) {
	m := New(Args{Width: 100, Height: 24}).(model)
	m.performImport(`curl -X POST https://httpbin.org/post -H "Content-Type: application/json" -d '{"a":1}'`)
	text := m.ed.Text()
	if !strings.Contains(text, "POST https://httpbin.org/post") {
		t.Errorf("import not inserted: %s", text)
	}
	if !strings.Contains(text, "Content-Type: application/json") {
		t.Errorf("header missing: %s", text)
	}
	if !strings.Contains(text, `{"a":1}`) {
		t.Errorf("body missing: %s", text)
	}
}
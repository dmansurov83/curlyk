package runner

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/user/curlyk/httptool/internal/httpfile"
)

func TestRunPOST(t *testing.T) {
	var gotBody string
	var gotCT string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		buf := make([]byte, 1024)
		n, _ := r.Body.Read(buf)
		gotBody = string(buf[:n])
		gotCT = r.Header.Get("Content-Type")
		w.Header().Set("X-Test", "yes")
		w.WriteHeader(http.StatusCreated)
		w.Write([]byte(`{"ok":true}`))
	}))
	defer srv.Close()

	req := httpfile.Request{
		Method:  "POST",
		URL:     srv.URL + "/e",
		Headers: []httpfile.Header{{Name: "Content-Type", Value: "application/json"}},
		Body:    `{"a":1}`,
	}
	res := Run(context.Background(), req, Options{})
	if res.Err != nil {
		t.Fatal(res.Err)
	}
	if res.Response.StatusCode != 201 {
		t.Errorf("status=%d", res.Response.StatusCode)
	}
	if res.Response.Header.Get("X-Test") != "yes" {
		t.Error("header not set")
	}
	body, _ := BodyBytes(res.Response)
	if string(body) != `{"ok":true}` {
		t.Errorf("body=%q", body)
	}
	if gotBody != `{"a":1}` {
		t.Errorf("received body=%q", gotBody)
	}
	if gotCT != "application/json" {
		t.Errorf("content-type=%q", gotCT)
	}
}

func TestRunRedirectNotFollowedDefault(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/final", http.StatusFound)
	}))
	defer srv.Close()

	req := httpfile.Request{Method: "GET", URL: srv.URL + "/start"}
	res := Run(context.Background(), req, Options{})
	if res.Err != nil {
		t.Fatal(res.Err)
	}
	if res.Response.StatusCode != 302 {
		t.Errorf("expected 302, got %d", res.Response.StatusCode)
	}
}

func TestRunError(t *testing.T) {
	req := httpfile.Request{Method: "GET", URL: "http://127.0.0.1:1/nope"}
	res := Run(context.Background(), req, Options{Timeout: 1})
	if res.Err == nil {
		t.Error("expected connection error")
	}
	_ = strings.Builder{}
}
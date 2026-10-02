package runner

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/user/curlyk/internal/httpfile"
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

// TestFollowRedirects verifies @follow-redirects / @redirects turns redirect
// following on, while the default stays off.
func TestFollowRedirects(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/start" {
			http.Redirect(w, r, "/final", http.StatusFound)
			return
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	}))
	defer srv.Close()

	cases := []struct {
		name    string
		options []httpfile.Option
		want    int
	}{
		{"no-option-default", nil, 302},
		{"no-redirect", []httpfile.Option{{Name: "no-redirect"}}, 302},
		{"follow-on", []httpfile.Option{{Name: "follow-redirects", Value: "on"}}, 200},
		{"follow-true", []httpfile.Option{{Name: "followredirects", Value: "true"}}, 200},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := httpfile.Request{Method: "GET", URL: srv.URL + "/start", Options: tc.options}
			opts, _ := ApplyOptions(req, Options{})
			res := Run(context.Background(), req, opts)
			if res.Err != nil {
				t.Fatal(res.Err)
			}
			if res.Response.StatusCode != tc.want {
				t.Errorf("status=%d want %d", res.Response.StatusCode, tc.want)
			}
		})
	}
}

// TestTimeoutOption verifies @timeout is applied and an invalid value surfaces
// an error while leaving the default timeout untouched.
func TestTimeoutOption(t *testing.T) {
	req := httpfile.Request{Options: []httpfile.Option{{Name: "timeout", Value: "5s"}}}
	opts, err := ApplyOptions(req, Options{})
	if err != nil {
		t.Fatal(err)
	}
	if opts.Timeout != 5*time.Second {
		t.Errorf("timeout=%v want 5s", opts.Timeout)
	}
	if opts.TimeoutOrDefault() != 5*time.Second {
		t.Errorf("TimeoutOrDefault=%v want 5s", opts.TimeoutOrDefault())
	}

	req.Options = []httpfile.Option{{Name: "timeout", Value: "abc"}}
	if _, err := ApplyOptions(req, Options{}); err == nil {
		t.Error("expected error for invalid timeout")
	}

	_, err = ApplyOptions(httpfile.Request{}, Options{})
	if err != nil {
		t.Errorf("no options should not error: %v", err)
	}
}

// TestInsecureOption verifies @insecure disables TLS verification without
// touching other fields.
func TestInsecureOption(t *testing.T) {
	req := httpfile.Request{Options: []httpfile.Option{
		{Name: "insecure"},
		{Name: "timeout", Value: "1500ms"},
	}}
	opts, err := ApplyOptions(req, Options{})
	if err != nil {
		t.Fatal(err)
	}
	if !opts.Insecure {
		t.Error("Insecure not set")
	}
	if opts.Timeout != 1500*time.Millisecond {
		t.Errorf("timeout=%v want 1500ms", opts.Timeout)
	}
}

// TestDefaultTimeout verifies TimeoutOrDefault returns the 30s default.
func TestDefaultTimeout(t *testing.T) {
	var opts Options
	if d := opts.TimeoutOrDefault(); d != 30*time.Second {
		t.Errorf("TimeoutOrDefault=%v want 30s", d)
	}
}

package gui

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// TestRunRequestFetchesAndCopies is the end-to-end path the user exercises:
// Ctrl+Enter runs the request, the reply lands in the pane, and the copy button
// puts its body on the clipboard.
func TestRunRequestFetchesAndCopies(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer srv.Close()

	u := newTestUI(t)
	u.ed.SetText("GET " + srv.URL + "/x HTTP/1.1\n")
	u.ed.doc.setCaret(0, 0)

	u.runRequest()
	deadline := time.Now().Add(5 * time.Second)
	for u.running && time.Now().Before(deadline) {
		time.Sleep(5 * time.Millisecond)
	}
	if u.running {
		t.Fatal("the request did not finish in time")
	}

	text := u.respHdr.Text() + "\n" + u.resp.Text()
	if !strings.Contains(text, "200") {
		t.Fatalf("response pane is missing the status:\n%s", text)
	}
	if !strings.Contains(text, `"ok": true`) {
		t.Fatalf("response pane is missing the JSON body:\n%s", text)
	}
	if u.helpShown() {
		t.Fatal("the reply should replace the help")
	}

	u.copyResponse()
	if got := clipboard().Content(); got != `{"ok":true}` {
		t.Fatalf("clipboard = %q, want the raw body", got)
	}
}

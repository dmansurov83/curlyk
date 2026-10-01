package tui

import (
	"errors"
	"strings"
	"testing"

	"github.com/mattn/go-runewidth"
	"github.com/user/curlyk/internal/runner"
)

// TestNetworkErrorShownInResponsePane verifies that a network error (not a
// server HTTP response) is rendered into the response pane as a readable body,
// with a matching status line, while leaving the active pane unchanged.
func TestNetworkErrorShownInResponsePane(t *testing.T) {
	m := New(Args{Width: 120, Height: 30}).(model)
	m.active = paneEdit

	res := &runner.Result{Err: errors.New("connection refused: dial tcp 127.0.0.1:1: connectex: no connection could be made")}

	got := m.applyResponse(runResultMsg{res: res}).(model)

	if !strings.Contains(got.response, "connection refused") {
		t.Errorf("response pane should contain the network error text, got %q", got.response)
	}
	if !strings.Contains(got.status, "Сетевая ошибка") {
		t.Errorf("status should mark a network error, got %q", got.status)
	}
	if got.active == paneResp {
		t.Errorf("right pane should not become active after a network error, got %d", got.active)
	}
	if got.respHeaderLines == 0 {
		t.Errorf("respHeaderLines should count the error header rows, got %d", got.respHeaderLines)
	}
	if got.response != "" && !strings.Contains(got.response, "Сетевой запрос не выполнен") {
		t.Errorf("response pane should contain a header title, got %q", got.response)
	}
}

// TestWrapToWidth verifies that a long unbroken network error is word-wrapped
// so every resulting line fits within the pane width.
func TestWrapToWidth(t *testing.T) {
	long := "POST https://example.com failed: dial tcp 203.0.113.7:443: connectex: A connection attempt failed because the connected party did not properly respond"
	wrapped := wrapToWidth(long, 30)
	lines := strings.Split(wrapped, "\n")
	if len(lines) < 2 {
		t.Fatalf("expected wrapping into multiple lines, got %q", wrapped)
	}
	for i, ln := range lines {
		if runewidth.StringWidth(ln) > 30 {
			t.Errorf("line %d width %d exceeds 30: %q", i, runewidth.StringWidth(ln), ln)
		}
	}
}

// TestWrapToWidthKeepsWords verifies wrapping joins words back and preserves
// the full text content (ignoring spaces across line breaks).
func TestWrapToWidthKeepsWords(t *testing.T) {
	long := "alpha beta gamma delta epsilon zeta eta theta"
	wrapped := wrapToWidth(long, 12)
	words := strings.Fields(strings.ReplaceAll(wrapped, "\n", " "))
	if strings.Join(words, " ") != long {
		t.Errorf("wrapping lost text: %q", wrapped)
	}
}
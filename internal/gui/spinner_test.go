package gui

import (
	"image/color"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// TestSpinnerStartsAndStops covers the animation lifecycle.
func TestSpinnerStartsAndStops(t *testing.T) {
	s := newSpinner(color.White)
	s.Start()
	if !s.running {
		t.Fatal("spinner did not start")
	}
	s.Start() // idempotent
	s.Stop()
	if s.running {
		t.Fatal("spinner did not stop")
	}
}

// TestSpinnerChaseBrightness checks the head of the chase: the first dot is
// brighter than the one opposite it.
func TestSpinnerChaseBrightness(t *testing.T) {
	s := newSpinner(color.White)
	s.phase = 0
	if s.opacity(0) <= s.opacity(spinnerDots/2) {
		t.Fatalf("head opacity %v is not brighter than the tail %v", s.opacity(0), s.opacity(spinnerDots/2))
	}
}

// TestSpinnerOnlyWhileRunning covers the window wiring: hidden at rest, shown
// and animated during a request, hidden again afterwards.
func TestSpinnerOnlyWhileRunning(t *testing.T) {
	u := newTestUI(t)
	if u.spinner.Visible() {
		t.Fatal("the spinner is visible at rest")
	}

	u.setRunning(true)
	if !u.spinner.Visible() || !u.spinner.running {
		t.Fatal("the spinner did not start with the request")
	}

	u.setRunning(false)
	if u.spinner.Visible() || u.spinner.running {
		t.Fatal("the spinner is still visible after the request")
	}
}

// TestRunRequestShowsSpinner is the end-to-end check: the spinner is on screen
// while the server is still answering and gone once the reply lands.
func TestRunRequestShowsSpinner(t *testing.T) {
	release := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-release
		_, _ = w.Write([]byte("ok"))
	}))
	defer srv.Close()

	u := newTestUI(t)
	u.ed.SetText("GET " + srv.URL + "\n")
	u.ed.doc.setCaret(0, 0)

	u.runRequest()
	if !u.spinner.Visible() {
		t.Fatal("the spinner is not visible while the request runs")
	}

	close(release)
	deadline := time.Now().Add(5 * time.Second)
	for u.running && time.Now().Before(deadline) {
		time.Sleep(5 * time.Millisecond)
	}
	if u.running {
		t.Fatal("the request did not finish in time")
	}
	if u.spinner.Visible() {
		t.Fatal("the spinner is still visible after the reply")
	}
}

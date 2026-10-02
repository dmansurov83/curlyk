package tui

import (
	"errors"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/mattn/go-runewidth"
	"github.com/user/curlyk/internal/i18n"
	"github.com/user/curlyk/internal/runner"
)

// TestHelpShownAtStartup verifies the right pane renders the hotkey reference
// while there is no response yet.
func TestHelpShownAtStartup(t *testing.T) {
	defer i18n.SetLocale("ru")
	i18n.SetLocale("ru")
	m := New(Args{Width: 120, Height: 40}).(model)
	if m.response != "" {
		t.Fatal("fresh model should have an empty response")
	}
	out := stripANSI(m.View())
	if !strings.Contains(out, "Горячие клавиши") {
		t.Errorf("startup right pane should show the hotkey title:\n%s", out)
	}
	if !strings.Contains(out, "Ctrl+G") {
		t.Errorf("help should list navigation hotkey Ctrl+G:\n%s", out)
	}
	for _, key := range []string{"Ctrl+Enter", "Ctrl+L", "Ctrl+S", "Ctrl+Z", "F10"} {
		if !strings.Contains(out, key) {
			t.Errorf("help missing key %q:\n%s", key, out)
		}
	}
	if !strings.Contains(out, "Переменные") {
		t.Errorf("help should show the variables section:\n%s", out)
	}
	if !strings.Contains(out, "@var") {
		t.Errorf("help should mention @var syntax:\n%s", out)
	}
	if !strings.Contains(out, "{{$random.uuid}}") {
		t.Errorf("help should list built-in functions:\n%s", out)
	}
}

// TestHelpTitleTranslates verifies the help panel follows the active locale.
func TestHelpTitleTranslates(t *testing.T) {
	defer i18n.SetLocale("ru")
	i18n.SetLocale("ru")
	lines := helpPanelLines(40, 40, 0)
	title := stripANSI(firstLine(strings.Join(lines, "\n")))
	if !strings.Contains(title, "Горячие клавиши") {
		t.Errorf("ru title mismatch: %q", title)
	}
	i18n.SetLocale("en")
	lines = helpPanelLines(40, 40, 0)
	title = stripANSI(firstLine(strings.Join(lines, "\n")))
	if !strings.Contains(title, "Hotkeys") {
		t.Errorf("en title mismatch: %q", title)
	}
}

// TestHelpHiddenAfterResponse verifies the hotkey panel is replaced by the real
// response once a request has run.
func TestHelpHiddenAfterResponse(t *testing.T) {
	m := New(Args{Width: 120, Height: 30}).(model)
	// setResponse is a value method returning the updated model; capture it.
	m = m.setResponse("HTTP/1.1 200 OK\n\nbody", "HTTP/1.1 200 OK\n\n", nil)
	out := stripANSI(m.View())
	if strings.Contains(out, "Горячие клавиши") {
		t.Errorf("help should be hidden after a response:\n%s", out)
	}
	if !strings.Contains(out, "body") {
		t.Errorf("response body should be shown:\n%s", out)
	}
}

// TestHelpF1TogglesOverResponse verifies F1 can force-show the hotkey panel
// even when a response is present, and that a fresh response dismisses it.
func TestHelpF1TogglesOverResponse(t *testing.T) {
	defer i18n.SetLocale("ru")
	i18n.SetLocale("ru")
	m := New(Args{Width: 120, Height: 30}).(model)
	m = m.setResponse("HTTP/1.1 200 OK\n\nbody", "HTTP/1.1 200 OK\n\n", nil)
	out := stripANSI(m.View())
	if strings.Contains(out, "Горячие клавиши") {
		t.Fatal("help must be hidden after a response")
	}
	// F1 shows help over the response.
	res, _ := m.handleKey(tea.KeyMsg{Type: tea.KeyF1})
	m = res.(model)
	if !m.helpVisible {
		t.Fatal("F1 should set helpVisible")
	}
	out = stripANSI(m.View())
	if !strings.Contains(out, "Горячие клавиши") {
		t.Errorf("help should be visible after F1:\n%s", out)
	}
	// F1 again hides it.
	res, _ = m.handleKey(tea.KeyMsg{Type: tea.KeyF1})
	m = res.(model)
	if m.helpVisible {
		t.Fatal("F1 should toggle helpVisible off")
	}
	// A fresh response dismisses help even if it was forced on.
	m.helpVisible = true
	m = m.applyResponse(runResultMsg{
		res: &runner.Result{Err: errors.New("connection refused")},
	}).(model)
	if m.helpVisible {
		t.Error("a fresh response must clear helpVisible")
	}
}
func TestHelpFitsPane(t *testing.T) {
	height := 15
	for width := 30; width <= 60; width += 10 {
		lines := helpPanelLines(width, height, 0)
		if len(lines) > height {
			t.Errorf("width=%d: %d lines > height %d", width, len(lines), height)
		}
		for i, ln := range lines {
			if w := runewidth.StringWidth(stripANSI(ln)); w > width {
				t.Errorf("width=%d line %d wider than pane: %d cells %q", width, i, w, stripANSI(ln))
			}
		}
	}
}

// TestHelpScrollRevealsVariables verifies that on a short window the variables
// section is not visible at scroll 0 but becomes visible once the panel scrolls.
func TestHelpScrollRevealsVariables(t *testing.T) {
	defer i18n.SetLocale("ru")
	i18n.SetLocale("ru")
	width, height := 40, 8
	top := strings.Join(helpPanelLines(width, height, 0), "\n")
	if strings.Contains(top, "@var") {
		t.Fatalf("@var already visible at scroll 0; test premise broken:\n%s", top)
	}
	mx := len(helpFullLines(width)) - height
	if mx < 1 {
		t.Fatal("expected the help to exceed the pane height")
	}
	bottom := strings.Join(helpPanelLines(width, height, mx), "\n")
	if !strings.Contains(bottom, "@var") {
		t.Errorf("@var not visible at max scroll %d:\n%s", mx, bottom)
	}
}

// TestHelpMaxScrollReachesBottom verifies that the scroll-clamp used by the
// model lets the very last help row reach the visible window. This regressed
// when helpScrollMax used a pane height larger than the actual render window,
// leaving the last rows unreachable.
func TestHelpMaxScrollReachesBottom(t *testing.T) {
	defer i18n.SetLocale("ru")
	i18n.SetLocale("ru")
	// Height 25 from the debug dump: paneHeight = height-6 = 19, window handed
	// to helpPanelLines is 19-2 = 17 rows.
	m := New(Args{Width: 120, Height: 25}).(model)
	m.active = paneResp
	mx := m.helpScrollMax()
	if mx <= 0 {
		t.Fatal("expected help to overflow a 25-row window")
	}
	lay := m.layout()
	width := lay.respR - lay.half - 1
	win := m.height - 6 // rows renderResponse passes as the help window height
	last := strings.Join(helpPanelLines(width-2, win, mx), "\n")
	// The last lines of the full reference must be present at max scroll.
	for _, frag := range []string{"{{$random.uuid}}", "{{$random.int}}", "{{$timestamp}}"} {
		if !strings.Contains(last, frag) {
			t.Errorf("max scroll window missing %q:\n%s", frag, last)
		}
	}
}

// TestHelpScrollKeys verifies arrows and Page keys move the help scroll window.
func TestHelpScrollKeys(t *testing.T) {
	defer i18n.SetLocale("ru")
	i18n.SetLocale("ru")
	m := New(Args{Width: 120, Height: 20}).(model)
	m.active = paneResp
	// 20-high window: help taller than pane, so scrolling is possible.
	maxBefore := m.helpScrollMax()
	if maxBefore <= 0 {
		t.Skip("pane too tall for help scrolling in this size")
	}
	res, _ := m.handleKey(tea.KeyMsg{Type: tea.KeyDown})
	m = res.(model)
	if m.helpScroll != 1 {
		t.Errorf("after Down helpScroll=%d want 1", m.helpScroll)
	}
	for i := 0; i < maxBefore+5; i++ {
		res, _ = m.handleKey(tea.KeyMsg{Type: tea.KeyDown})
		m = res.(model)
	}
	if m.helpScroll != maxBefore {
		t.Errorf("scrolled past bottom: helpScroll=%d want %d", m.helpScroll, maxBefore)
	}
	res, _ = m.handleKey(tea.KeyMsg{Type: tea.KeyUp})
	m = res.(model)
	if m.helpScroll != maxBefore-1 {
		t.Errorf("after Up helpScroll=%d want %d", m.helpScroll, maxBefore-1)
	}
}

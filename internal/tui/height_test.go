package tui

import (
	"strings"
	"testing"
)

// TestViewHeightFitsWindow ensures the rendered frame does not exceed the UI
// height (overflow would push content and could hide the top border on some
// terminals).
func TestViewHeightFitsWindow(t *testing.T) {
	for _, h := range []int{12, 20, 30, 40} {
		m := New(Args{Width: 120, Height: h}).(model)
		out := m.View()
		n := len(strings.Split(out, "\n"))
		// allow at most height lines (top-anchored bubbletea draws from row 0)
		if n > h {
			t.Errorf("height=%d: View returned %d lines (overflow)\nfirst=%q", h, n, firstLine(out))
		}
	}
}

func firstLine(s string) string {
	i := strings.IndexByte(s, '\n')
	if i < 0 {
		return s
	}
	return s[:i]
}
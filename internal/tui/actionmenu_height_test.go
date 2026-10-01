package tui

import (
	"strings"
	"testing"
)

// TestMenuOpenKeepsFrameFit verifies that opening the request action popup in a
// small window does not grow the rendered frame taller than the terminal height.
// The popup is drawn inline inside the editor pane, so it must trade place with
// source rows. If it overflowed the frame, bubbletea's top-anchored render would
// scroll the top file-name header off-screen.
func TestMenuOpenKeepsFrameFit(t *testing.T) {
	for _, h := range []int{9, 10, 12, 15, 20} {
		m := New(Args{Width: 100, Height: h}).(model)
		m.ed.curRow = 3 // request line "GET https://httpbin.org/get?x=1"
		m.beginActionMenu()
		if m.actionMenu == nil {
			t.Fatalf("height=%d: menu not opened", h)
		}
		out := m.View()
		n := len(strings.Split(out, "\n"))
		if n > h {
			t.Errorf("height=%d: frame with menu overflowed (%d lines > %d)", h, n, h)
		}
		// The top file-name header must survive.
		first := firstLine(out)
		if !strings.Contains(stripANSI(first), "Файл:") {
			t.Errorf("height=%d: top header lost when menu open; first=%q", h, first)
		}
		// The menu items must be visible inside the editor pane.
		if !strings.Contains(stripANSI(out), "Выполнить") {
			t.Errorf("height=%d: menu items not visible in frame", h)
		}
	}
}
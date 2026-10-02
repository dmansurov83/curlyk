package tui

import (
	"strings"
	"testing"
)

// TestFormOpenKeepsFrameFit verifies that opening the form editor (key=value
// body popup) in a small window does not grow the rendered frame taller than
// the terminal height, and that the top file-name header survives.
func TestFormOpenKeepsFrameFit(t *testing.T) {
	for _, h := range []int{9, 10, 12, 15, 20} {
		m := New(Args{Width: 100, Height: h}).(model)
		m.active = paneEdit
		m.ed.SetText("POST http://x/token\ncontent-type: application/x-www-form-urlencoded\n\ngrant_type=application&scope=openid offline_access\n")
		m.ed.curRow = 0
		m.beginFormEditor()
		if m.form == nil {
			t.Fatalf("height=%d: form not opened", h)
		}
		out := m.View()
		n := len(strings.Split(out, "\n"))
		if n > h {
			t.Errorf("height=%d: frame with form overflowed (%d lines > %d)", h, n, h)
		}
		first := firstLine(out)
		if !strings.Contains(stripANSI(first), "Файл:") {
			t.Errorf("height=%d: top header lost when form open; first=%q", h, first)
		}
	}
}

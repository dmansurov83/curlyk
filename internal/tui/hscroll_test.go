package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestTruncateWidthOffset(t *testing.T) {
	// "abcdefgh", max 4, offset 3 -> "defg"
	if got := truncateWidthOffset("abcdefgh", 4, 3); got != "defg" {
		t.Errorf("got %q want defg", got)
	}
	// offset beyond content -> ""
	if got := truncateWidthOffset("abc", 4, 10); got != "" {
		t.Errorf("got %q want empty", got)
	}
}

// TestWheelHorizontalScroll verifies tilt-wheel changes the horizontal offset.
func TestWheelHorizontalScroll(t *testing.T) {
	m := New(Args{Width: 120, Height: 30}).(model)
	m.active = paneResp
	m.response = strings.Repeat("x", 200) + "\n"
	m.respHScroll = 0
	m2, _ := m.Update(tea.MouseMsg{Button: tea.MouseButtonWheelRight, Action: tea.MouseActionPress, X: 80})
	if m2.(model).respHScroll == 0 {
		t.Error("wheel right should increase horizontal scroll")
	}
	m = m2.(model)
	m3, _ := m.Update(tea.MouseMsg{Button: tea.MouseButtonWheelLeft, Action: tea.MouseActionPress, X: 80})
	if m3.(model).respHScroll != 0 {
		t.Errorf("wheel left should decrease to 0, got %d", m3.(model).respHScroll)
	}
}
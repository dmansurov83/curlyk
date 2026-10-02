package tui

import (
	"os"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

// TestProfileHoverHighlight verifies that moving the mouse over a non-active
// profile row in the left sidebar renders it with the hover background, exactly
// like a file row.
func TestProfileHoverHighlight(t *testing.T) {
	dir := t.TempDir()
	old, _ := os.Getwd()
	t.Cleanup(func() { os.Chdir(old) })
	os.Chdir(dir)
	os.WriteFile("dev.profile", []byte("@var host = x\n"), 0o644)
	os.WriteFile("prod.profile", []byte("@var host = y\n"), 0o644)

	m := New(Args{Width: 120, Height: 40}).(model)
	m.filesPanel = &filesPanel{all: httpFilesInDir(".")}
	m.filesPanel.loadProfiles()

	// prod is non-active; hover over its row (index 1).
	y := m.profileRowAbs(1)
	x := m.filesWidth() / 2

	mm := m
	// dumpDebug paths aside, drive a real motion event through Update.
	r, _ := m.Update(tea.MouseMsg{
		Button: tea.MouseButtonNone, Action: tea.MouseActionMotion, X: x, Y: y,
	})
	mm = r.(model)
	if mm.hoverY != y {
		t.Fatalf("hoverY=%d want %d", mm.hoverY, y)
	}
	if mm.paneAtX(mm.hoverX) != paneFiles {
		t.Fatalf("paneAtX(%d)=%v want paneFiles (filesEnd=%d)", mm.hoverX, mm.paneAtX(mm.hoverX), mm.layout().filesEnd)
	}
	raw := mm.View()
	// fileHoverStyle uses background 238; present only for the hovered row.
	if !strings.Contains(raw, "238") {
		t.Errorf("hover background (238) not applied for non-active profile row (y=%d)", y)
	}
}

// TestProfileNewRowHover verifies the "+ Новый профиль" row highlights on hover,
// like the profile rows and the "new file" row.
func TestProfileNewRowHover(t *testing.T) {
	dir := t.TempDir()
	old, _ := os.Getwd()
	t.Cleanup(func() { os.Chdir(old) })
	os.Chdir(dir)
	os.WriteFile("dev.profile", []byte("@var host = x\n"), 0o644)

	m := New(Args{Width: 120, Height: 40}).(model)
	m.filesPanel = &filesPanel{all: httpFilesInDir(".")}
	m.filesPanel.loadProfiles()

	y := m.profileNewRowAbs()
	x := m.filesWidth() / 2
	r, _ := m.Update(tea.MouseMsg{
		Button: tea.MouseButtonNone, Action: tea.MouseActionMotion, X: x, Y: y,
	})
	mm := r.(model)
	if !mm.mouseToProfileNewRow(mm.hoverY) {
		t.Fatalf("mouseToProfileNewRow(%d)=false want true", mm.hoverY)
	}
	raw := mm.View()
	// The hovered "new profile" row must render with the hover background (238).
	if !strings.Contains(raw, "238") {
		t.Errorf("new-profile row not highlighted on hover (y=%d)", y)
	}
}

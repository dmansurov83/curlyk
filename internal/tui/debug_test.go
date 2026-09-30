package tui

import (
	"os"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestDumpDebugWritesFile(t *testing.T) {
	m := New(Args{Width: 120, Height: 30}).(model)
	m.ed.SetText("### sample\nGET http://x/1\n")
	m.ed.curRow, m.ed.curCol, m.ed.onIcon = 1, 3, false
	m.dumpDebug()

	data, err := os.ReadFile("debug.txt")
	if err != nil {
		t.Fatalf("debug.txt not written: %v", err)
	}
	s := string(data)
	for _, want := range []string{"curRow:", "1 : ### sample", "2 : GET http://x/1", "curCol:", "scroll:"} {
		if !strings.Contains(s, want) {
			t.Errorf("debug.txt missing %q:\n%s", want, s)
		}
	}
	os.Remove("debug.txt")
}

func TestDumpDebugCtrlDPath(t *testing.T) {
	m := New(Args{Width: 120, Height: 30}).(model)
	m.active = paneEdit
	_, cmd := m.handleKey(tea.KeyMsg{Type: tea.KeyCtrlD})
	if cmd != nil {
		t.Fatal("ctrl+d should not produce a command")
	}
	defer os.Remove("debug.txt")
	if _, err := os.Stat("debug.txt"); err != nil {
		t.Fatalf("debug.txt missing after ctrl+d: %v", err)
	}
}
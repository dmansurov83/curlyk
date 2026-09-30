package tui

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// TestAutosaveDebounce verifies edits mark dirty and are persisted only after
// the debounce window elapses without further edits.
func TestAutosaveDebounce(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "auto.http")
	os.Setenv("HTTPTOOL_SESSION_PATH", path)
	defer os.Unsetenv("HTTPTOOL_SESSION_PATH")

	m := New(Args{Width: 120, Height: 30}).(model)
	m.active = paneEdit
	m.ed.SetText("a\n")
	m.ed.undo = nil

	// An edit marks the model dirty and arms the deadline.
	m.markDirty()
	if !m.dirty {
		t.Fatal("markDirty should set dirty")
	}
	// Another edit within the window resets the deadline (debounce extend).
	m.markDirty()

	// Before the deadline elapses, applyAutosave must not save yet.
	before := time.Now()
	m.applyAutosave(before)
	if !m.dirty {
		t.Fatal("autosave must not fire during the debounce window")
	}
	if _, err := os.Stat(path); err == nil {
		t.Fatal("file must not be written during debounce window")
	}

	// After the window, autosave persists.
	after := time.Now().Add(autosaveDelay + time.Millisecond)
	m.applyAutosave(after)
	if m.dirty {
		t.Error("dirty should be false after autosave fires")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("session/autosave file not written: %v", err)
	}
	if string(data) != "a\n" {
		t.Errorf("autosaved content=%q want a", string(data))
	}
}

// TestUpdateAutosaveMsgProvesLoop verifies Update handles autosaveMsg and does
// not save when not dirty.
func TestUpdateAutosaveMsgProvesLoop(t *testing.T) {
	m := New(Args{Width: 120, Height: 30}).(model)
	m.active = paneEdit
	m.ed.SetText("x\n")
	m.dirty = false
	m.autosaveDeadline = time.Now().Add(-time.Second)

	res, cmd := m.Update(autosaveMsg{})
	if res.(model).dirty {
		t.Error("not-dirty model should stay not-dirty after autosaveMsg")
	}
	if cmd == nil {
		t.Fatal("autosaveMsg must re-arm the loop (return a cmd)")
	}
}
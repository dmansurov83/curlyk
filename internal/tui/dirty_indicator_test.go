package tui

import (
	"path/filepath"
	"strings"
	"testing"
)

// TestHeaderNoDirtyMarker verifies the header shows no marker when the buffer
// is clean.
func TestHeaderNoDirtyMarker(t *testing.T) {
	m := New(Args{Width: 120, Height: 30}).(model)
	m.dirty = false
	out := stripANSI(m.renderHeader(120))
	if strings.Contains(out, "*") {
		t.Errorf("clean header should not contain the dirty marker:\n%q", out)
	}
	if !strings.Contains(out, "Файл:") {
		t.Errorf("header should still show the file label:\n%q", out)
	}
}

// TestHeaderDirtyMarker verifies the header shows the marker when the buffer
// is dirty.
func TestHeaderDirtyMarker(t *testing.T) {
	m := New(Args{Width: 120, Height: 30}).(model)
	m.filePath = "request.http"
	m.dirty = true
	out := stripANSI(m.renderHeader(120))
	if !strings.Contains(out, "*") {
		t.Errorf("dirty header should contain the dirty marker:\n%q", out)
	}
	if !strings.Contains(out, "request.http") {
		t.Errorf("header should still show the file name:\n%q", out)
	}
}

// TestSaveClearsDirtyMarker verifies the marker disappears after a manual save.
// The save target lives in a temp dir so the test does not drop .http files
// into the package working directory (which the files panel scans).
func TestSaveClearsDirtyMarker(t *testing.T) {
	m := New(Args{Width: 120, Height: 30}).(model)
	m.filePath = filepath.Join(t.TempDir(), "request.http")
	m.ed.SetText("GET /x\n")
	m.dirty = true
	m.saveBuffer()
	if m.dirty {
		t.Error("saveBuffer should clear dirty")
	}
	out := stripANSI(m.renderHeader(120))
	if strings.Contains(out, "*") {
		t.Errorf("header should not show marker after save:\n%q", out)
	}
}
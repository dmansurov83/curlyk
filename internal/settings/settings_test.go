package settings

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSettingsRoundTrip(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "appsettings.yml")
	SetAppSettingsPath(p)
	defer SetAppSettingsPath("appsettings.yml")

	s := Settings{LastOpenedFile: "req.http", SessionFile: "sess.http", CursorRow: 12, CursorCol: 4, EditorScroll: 5}
	if err := s.Save(); err != nil {
		t.Fatal(err)
	}
	got := Load()
	if got.LastOpenedFile != "req.http" || got.SessionFile != "sess.http" {
		t.Errorf("roundtrip mismatch: %+v", got)
	}
	if got.CursorRow != 12 || got.CursorCol != 4 || got.EditorScroll != 5 {
		t.Errorf("cursor not persisted: row=%d col=%d scroll=%d", got.CursorRow, got.CursorCol, got.EditorScroll)
	}
}

func TestLoadMissingReturnsDefault(t *testing.T) {
	dir := t.TempDir()
	SetAppSettingsPath(filepath.Join(dir, "nope.yml"))
	defer SetAppSettingsPath("appsettings.yml")

	got := Load()
	if got.SessionFile != "last.session.http" {
		t.Errorf("expected default session file, got %q", got.SessionFile)
	}
}

func TestRememberLastOpenedWritesFile(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "appsettings.yml")
	SetAppSettingsPath(p)
	defer SetAppSettingsPath("appsettings.yml")

	// write via the helper-style save
	s := Settings{LastOpenedFile: "a.http"}
	if err := s.Save(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(p); err != nil {
		t.Fatalf("settings file not created: %v", err)
	}
}
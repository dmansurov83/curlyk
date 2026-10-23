package profiles

import (
	"os"
	"path/filepath"
	"testing"
)

func TestListFindsOnlyProfilesSorted(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{"b.profile", "a.profile", "notes.txt", "c.PROFILE"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("@var x = 1\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Mkdir(filepath.Join(dir, "nested.profile"), 0o755); err != nil {
		t.Fatal(err)
	}

	got := List(dir)
	want := []string{"a.profile", "b.profile", "c.PROFILE"}
	if len(got) != len(want) {
		t.Fatalf("List = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("List = %v, want %v", got, want)
		}
	}
}

func TestListMissingDirIsEmpty(t *testing.T) {
	if got := List(filepath.Join(t.TempDir(), "nope")); got != nil {
		t.Fatalf("List of a missing dir = %v, want nil", got)
	}
}

func TestLoadParsesVars(t *testing.T) {
	dir := t.TempDir()
	body := "# profile\n@var host = http://localhost:8080\n@var token = abc\n"
	if err := os.WriteFile(filepath.Join(dir, "dev.profile"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}

	vars := Load(dir, "dev.profile")
	if vars["host"] != "http://localhost:8080" || vars["token"] != "abc" {
		t.Fatalf("Load = %v", vars)
	}
	if Load(dir, "missing.profile") != nil {
		t.Error("Load of a missing profile should be nil")
	}
	if Load(dir, "") != nil {
		t.Error("Load with an empty name should be nil")
	}
}

func TestNormalizeAndDisplayName(t *testing.T) {
	if got := Normalize("dev"); got != "dev.profile" {
		t.Errorf("Normalize(dev) = %q", got)
	}
	if got := Normalize("dev.profile"); got != "dev.profile" {
		t.Errorf("Normalize(dev.profile) = %q", got)
	}
	if got := DisplayName("dev.profile"); got != "dev" {
		t.Errorf("DisplayName(dev.profile) = %q", got)
	}
}

// TestCreateWritesProfileWithSuffix covers the extracted behaviour: the suffix
// is appended when missing and the skeleton is written, exactly as the TUI's
// "new profile" action did before the extraction (including rewriting a file of
// the same name).
func TestCreateWritesProfileWithSuffix(t *testing.T) {
	dir := t.TempDir()
	file, err := Create(dir, "stage")
	if err != nil {
		t.Fatal(err)
	}
	if file != "stage.profile" {
		t.Fatalf("Create returned %q, want stage.profile", file)
	}
	if vars := Load(dir, file); vars["host"] != "http://localhost" {
		t.Fatalf("created profile vars = %v", vars)
	}
	if file2, err := Create(dir, "stage.profile"); err != nil || file2 != "stage.profile" {
		t.Fatalf("Create(stage.profile) = (%q, %v), want (stage.profile, nil)", file2, err)
	}
}

func TestEnsureDefaultCreatesAndKeeps(t *testing.T) {
	dir := t.TempDir()
	created, err := EnsureDefault(dir)
	if err != nil || !created {
		t.Fatalf("EnsureDefault = (%v, %v), want (true, nil)", created, err)
	}
	if _, err := os.Stat(filepath.Join(dir, DefaultName)); err != nil {
		t.Fatalf("default profile missing: %v", err)
	}

	// A second call must leave the file alone.
	if err := os.WriteFile(filepath.Join(dir, DefaultName), []byte("edited"), 0o644); err != nil {
		t.Fatal(err)
	}
	created, err = EnsureDefault(dir)
	if err != nil || created {
		t.Fatalf("second EnsureDefault = (%v, %v), want (false, nil)", created, err)
	}
	data, _ := os.ReadFile(filepath.Join(dir, DefaultName))
	if string(data) != "edited" {
		t.Fatalf("EnsureDefault overwrote the existing file: %q", data)
	}
}

func TestEnsureDefaultSkipsWhenProfilesExist(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "dev.profile"), []byte("@var a = 1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	created, err := EnsureDefault(dir)
	if err != nil || created {
		t.Fatalf("EnsureDefault = (%v, %v), want (false, nil)", created, err)
	}
	if _, err := os.Stat(filepath.Join(dir, DefaultName)); err == nil {
		t.Error("EnsureDefault should not create a default when profiles already exist")
	}
}

// TestPickNeverReturnsEmpty pins the invariant the TUI relies on: a profile is
// always active, so Pick falls back to the first available file and then to the
// default name.
func TestPickNeverReturnsEmpty(t *testing.T) {
	if got := Pick([]string{"dev.profile", "prod.profile"}, "prod.profile"); got != "prod.profile" {
		t.Errorf("Pick kept name = %q", got)
	}
	if got := Pick([]string{"dev.profile", "prod.profile"}, "gone.profile"); got != "dev.profile" {
		t.Errorf("Pick fallback = %q, want dev.profile", got)
	}
	if got := Pick(nil, ""); got != DefaultName {
		t.Errorf("Pick with nothing available = %q, want %q", got, DefaultName)
	}
}

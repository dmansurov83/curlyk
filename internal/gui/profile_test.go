package gui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"fyne.io/fyne/v2/widget"

	"github.com/user/curlyk/internal/httpfile"
	"github.com/user/curlyk/internal/i18n"
)

// clickProfileRow finds the sidebar button for a profile and taps it, so the
// test exercises the wiring rather than calling the handler directly.
func clickProfileRow(t *testing.T, u *ui, display string) {
	t.Helper()
	for _, o := range u.files.box.Objects {
		b, ok := o.(*widget.Button)
		if !ok {
			continue
		}
		if strings.Contains(b.Text, display) {
			b.OnTapped()
			return
		}
	}
	t.Fatalf("no sidebar row for profile %q", display)
}

// TestProfileClickOpensAndActivates covers the console behaviour: clicking a
// profile row acts it and loads its file into the editor.
func TestProfileClickOpensAndActivates(t *testing.T) {
	u := newTestUI(t)
	if err := os.WriteFile("dev.profile", []byte("@var token = old\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	u.reloadFiles()

	clickProfileRow(t, u, "dev")

	if u.settings.ActiveProfile != "dev.profile" {
		t.Fatalf("active profile = %q, want dev.profile", u.settings.ActiveProfile)
	}
	if filepath.Base(u.path) != "dev.profile" {
		t.Fatalf("open file = %q, want dev.profile", u.path)
	}
	if !strings.Contains(u.ed.Text(), "@var token = old") {
		t.Fatalf("editor text = %q, want the profile body", u.ed.Text())
	}
}

// TestProfileNewRowCreatesAndActivates covers the console's "+ Новый профиль"
// action: the sidebar carries a create row whose callback opens the editor with
// the new profile and activates it.
func TestProfileNewRowCreatesAndActivates(t *testing.T) {
	u := newTestUI(t)
	u.reloadFiles()

	// Drive the same helper the create row's button calls.
	u.createProfile("qa")

	if u.settings.ActiveProfile != "qa.profile" {
		t.Fatalf("active profile = %q, want qa.profile", u.settings.ActiveProfile)
	}
	if filepath.Base(u.path) != "qa.profile" {
		t.Fatalf("open file = %q, want a new qa.profile", u.path)
	}
	if _, err := os.Stat("qa.profile"); err != nil {
		t.Fatalf("profile file not written: %v", err)
	}
}

// TestProfileSectionCarriesNewRow covers the TUI difference the fix adds: the
// sidebar's profile section ends with a "+ Новый профиль" row, so the window can
// create a profile without leaving to the console.
func TestProfileSectionCarriesNewRow(t *testing.T) {
	u := newTestUI(t)
	u.reloadFiles()

	found := false
	for _, o := range u.files.box.Objects {
		b, ok := o.(*widget.Button)
		if !ok {
			continue
		}
		if b.Text == i18n.T("profile.new") {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("sidebar has no %q row; profile section object count = %d", i18n.T("profile.new"), len(u.files.box.Objects))
	}
}

// TestProfileSaveFeedsSubstitution keeps the edited profile live: Ctrl+S writes
// it back and the next request substitutes the new value.
func TestProfileSaveFeedsSubstitution(t *testing.T) {
	u := newTestUI(t)
	if err := os.WriteFile("dev.profile", []byte("@var token = old\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	u.reloadFiles()

	clickProfileRow(t, u, "dev")
	u.ed.SetText("@var token = new\n")
	u.saveFile()

	if got := u.profileVars()["token"]; got != "new" {
		t.Fatalf("profile token = %q, want the saved value", got)
	}
	req := &httpfile.Request{URL: "http://example.test/{{token}}"}
	if err := u.substituteVars(req); err != nil {
		t.Fatalf("substitution failed: %v", err)
	}
	if req.URL != "http://example.test/new" {
		t.Fatalf("substituted URL = %q, want the profile value", req.URL)
	}
}

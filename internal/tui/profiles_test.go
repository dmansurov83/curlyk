package tui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/user/curlyk/internal/i18n"
	"github.com/user/curlyk/internal/settings"
)

// writeProfileSample creates the given files in the working directory and
// changes into it, restoring the CWD on test end.
func writeProfileSample(t *testing.T, files map[string]string) {
	t.Helper()
	dir := t.TempDir()
	wd, _ := os.Getwd()
	t.Cleanup(func() { os.Chdir(wd) })
	os.Chdir(dir)
	for name, content := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

// TestProfileFilesInDir discovers *.profile files and ignores other types.
func TestProfileFilesInDir(t *testing.T) {
	writeProfileSample(t, map[string]string{
		"dev.profile":   "@var host = http://dev\n",
		"prod.profile":  "@var host = http://prod\n",
		"req.http":      "GET http://x\n",
		"notes.txt":     "no\n",
		"empty.profile": "",
	})
	got := profileFilesInDir(".")
	if len(got) != 3 {
		t.Fatalf("want 3 profile files, got %v", got)
	}
	if got[0] != "dev.profile" || got[1] != "empty.profile" || got[2] != "prod.profile" {
		t.Errorf("unexpected sorted listing: %v", got)
	}
}

func panelDown(m model) model {
	r, _ := m.handleKey(tea.KeyMsg{Type: tea.KeyDown})
	if rr, ok := r.(model); ok {
		return rr
	}
	return m
}

func panelEnter(m model) model {
	r, _ := m.handleKey(tea.KeyMsg{Type: tea.KeyEnter})
	if rr, ok := r.(model); ok {
		return rr
	}
	return m
}

// TestProfileActivateViaKeyboard drives the sidebar cursor into the profile
// section and activates a profile with Enter.
func TestProfileActivateViaKeyboard(t *testing.T) {
	writeProfileSample(t, map[string]string{
		"a.http":       "GET http://a\n",
		"dev.profile":  "@var host = http://dev\n",
		"prod.profile": "@var host = http://prod\n",
	})
	m := New(Args{Width: 120, Height: 40}).(model)
	m.active = paneFiles
	m.filesPanel = &filesPanel{all: httpFilesInDir(".")}
	m.filesPanel.loadProfiles()
	if len(m.filesPanel.profiles) != 2 {
		t.Fatalf("profiles=%v", m.filesPanel.profiles)
	}
	// items: [new-file, a.http]. Move down past the file list into profiles.
	m = panelDown(m) // new-file -> a.http
	m = panelDown(m) // last file -> first profile
	if !m.filesPanel.onProfiles || m.filesPanel.profSel != 0 {
		t.Fatalf("expected profile section focus, onProfiles=%v profSel=%d", m.filesPanel.onProfiles, m.filesPanel.profSel)
	}
	m = panelEnter(m)
	if m.profile != "dev.profile" {
		t.Fatalf("profile=%q want dev.profile", m.profile)
	}
}

// TestProfileClickOpensAndActivates verifies a mouse click on a profile row
// opens it in the editor AND activates it (like opening a file).
func TestProfileClickOpensAndActivates(t *testing.T) {
	writeProfileSample(t, map[string]string{
		"a.http":      "GET http://a\n",
		"dev.profile": "@var host = http://dev\n",
	})
	m := New(Args{Width: 120, Height: 40}).(model)
	m.active = paneEdit
	m.filesPanel = &filesPanel{all: httpFilesInDir(".")}
	m.filesPanel.loadProfiles()

	fw := m.filesWidth()
	y := m.profileRowAbs(0)
	r, _ := m.Update(tea.MouseMsg{
		Button: tea.MouseButtonLeft, Action: tea.MouseActionPress,
		X: fw / 2, Y: y,
	})
	mm := r.(model)
	if mm.profile != "dev.profile" {
		t.Errorf("profile=%q want dev.profile", mm.profile)
	}
	if mm.filePath != "dev.profile" {
		t.Errorf("filePath=%q want dev.profile (opened in editor)", mm.filePath)
	}
	if mm.active != paneEdit {
		t.Errorf("active=%v want paneEdit", mm.active)
	}
}

// TestProfileHeaderShown verifies the active profile appears in the top header.
func TestProfileHeaderShown(t *testing.T) {
	writeProfileSample(t, map[string]string{
		"dev.profile": "@var host = http://dev\n",
		"req.http":    "GET {{host}}/x\n",
	})
	m := New(Args{Width: 120, Height: 40}).(model)
	m.selectProfile("dev.profile")
	out := stripANSI(m.renderHeader(120))
	if !strings.Contains(out, "dev") {
		t.Errorf("header does not mention profile dev: %q", out)
	}
}

// TestProfilePersistedToSettings verifies selecting a profile writes it to
// appsettings.yml.
func TestProfilePersistedToSettings(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "appsettings.yml")
	settings.SetAppSettingsPath(p)
	defer settings.SetAppSettingsPath("appsettings.yml")
	writeProfileSample(t, map[string]string{
		"dev.profile": "@var host = http://dev\n",
	})
	m := New(Args{Width: 120, Height: 40}).(model)
	m.selectProfile("dev.profile")
	s := settings.Load()
	if s.ActiveProfile != "dev.profile" {
		t.Errorf("settings.ActiveProfile=%q want dev.profile", s.ActiveProfile)
	}
}

// TestProfileAlwaysActive verifies one profile is always active: selecting the
// active profile keeps it, and clearing (empty) falls back to the first profile.
func TestProfileAlwaysActive(t *testing.T) {
	writeProfileSample(t, map[string]string{
		"dev.profile":  "@var host = http://dev\n",
		"prod.profile": "@var host = http://prod\n",
	})
	m := New(Args{Width: 120, Height: 40}).(model)
	m.filesPanel = &filesPanel{all: httpFilesInDir(".")}
	m.filesPanel.loadProfiles()
	m.selectProfile("dev.profile")
	if m.profile != "dev.profile" {
		t.Fatalf("profile=%q want dev.profile", m.profile)
	}
	// re-selecting the same profile keeps it active (no toggle-off)
	m.selectProfile("dev.profile")
	if m.profile != "dev.profile" {
		t.Errorf("re-select must keep dev.profile, got %q", m.profile)
	}
	// an empty selection falls back to the first available profile
	m.setActiveProfile("")
	if m.profile != "dev.profile" {
		t.Errorf("empty selection must keep one active (dev.profile), got %q", m.profile)
	}
}

// TestActiveProfileVars verifies activeProfileVars parses a profile's @var lines.
func TestActiveProfileVars(t *testing.T) {
	writeProfileSample(t, map[string]string{
		"dev.profile": "@var host = http://dev\n@var token = abc\nGET http://ignored\n",
	})
	m := New(Args{Width: 120, Height: 40}).(model)
	m.profile = "dev.profile"
	vars := m.activeProfileVars()
	if vars["host"] != "http://dev" || vars["token"] != "abc" {
		t.Errorf("activeProfileVars=%v", vars)
	}
}

// TestProfileRenderingVisible verifies the profile section renders in the
// sidebar when profile files exist.
func TestProfileRenderingVisible(t *testing.T) {
	writeProfileSample(t, map[string]string{
		"dev.profile": "@var host = http://dev\n",
		"a.http":      "GET http://a\n",
	})
	m := New(Args{Width: 120, Height: 40}).(model)
	m.filesPanel = &filesPanel{all: httpFilesInDir(".")}
	m.filesPanel.loadProfiles()
	v := stripANSI(m.View())
	if !strings.Contains(v, "dev") {
		t.Errorf("profile dev not rendered in sidebar")
	}
}

// TestProfileFrameFitsWindow verifies that rendering with profile sections
// present never exceeds the terminal height across small windows.
func TestProfileFrameFitsWindow(t *testing.T) {
	for _, h := range []int{12, 15, 20, 30, 40} {
		writeProfileSample(t, map[string]string{
			"dev.profile":  "@var host = http://dev\n",
			"prod.profile": "@var host = http://prod\n",
			"a.http":       "GET http://a\n",
		})
		m := New(Args{Width: 100, Height: h}).(model)
		if m.filesPanel == nil {
			m.filesPanel = &filesPanel{}
		}
		m.filesPanel.loadProfiles()
		out := m.View()
		n := len(strings.Split(out, "\n"))
		if n > h {
			t.Errorf("height=%d: frame with profiles overflowed (%d lines > %d)", h, n, h)
		}
	}
}

// TestProfileSectionAlwaysVisible verifies the profile section (title + new
// action) renders in the sidebar even on a fresh working dir, thanks to the
// auto-created default.profile.
func TestProfileSectionAlwaysVisible(t *testing.T) {
	writeProfileSample(t, map[string]string{})
	m := New(Args{Width: 120, Height: 40}).(model)
	if m.filesPanel == nil {
		m.filesPanel = &filesPanel{}
	}
	// New auto-creates default.profile, so the section always has content.
	if len(m.filesPanel.profiles) == 0 {
		t.Fatal("expected default.profile to be auto-created on startup")
	}
	v := stripANSI(m.View())
	if !strings.Contains(v, i18n.T("profile.title")) {
		t.Errorf("profile section title not visible:\n%s", v)
	}
	if !strings.Contains(v, i18n.T("profile.new")) {
		t.Errorf("new-profile action not visible:\n%s", v)
	}
}

// TestEnsureDefaultProfile creates default.profile when none exist.
func TestEnsureDefaultProfile(t *testing.T) {
	writeProfileSample(t, map[string]string{})
	ensureDefaultProfile()
	if _, err := os.Stat(defaultProfileName); err != nil {
		t.Fatalf("default.profile not created: %v", err)
	}
	// second call must not overwrite
	if err := os.WriteFile(defaultProfileName, []byte("edited"), 0o644); err != nil {
		t.Fatal(err)
	}
	ensureDefaultProfile()
	data, _ := os.ReadFile(defaultProfileName)
	if string(data) != "edited" {
		t.Errorf("ensureDefaultProfile overwrote existing file: %q", data)
	}
}

// TestCreateProfileFromInput drives the new-profile flow: navigating to the
// "+ Новый профиль" row and entering a name creates and activates a profile.
func TestCreateProfileFromInput(t *testing.T) {
	writeProfileSample(t, map[string]string{})
	m := New(Args{Width: 120, Height: 40}).(model)
	m.active = paneFiles
	if m.filesPanel == nil {
		m.filesPanel = &filesPanel{}
	}
	m.filesPanel = &filesPanel{all: nil}
	m.filesPanel.loadProfiles()
	m.createProfile("staging")
	if _, err := os.Stat("staging.profile"); err != nil {
		t.Fatalf("staging.profile not created: %v", err)
	}
	if m.profile != "staging.profile" {
		t.Errorf("profile=%q want staging.profile", m.profile)
	}
	found := false
	for _, p := range m.filesPanel.profiles {
		if p == "staging.profile" {
			found = true
		}
	}
	if !found {
		t.Errorf("staging.profile not in panel list: %v", m.filesPanel.profiles)
	}
}

// TestProfileNewButtonMouse verifies a mouse click on the "+ Новый профиль" row
// opens the profile-name input.
func TestProfileNewButtonMouse(t *testing.T) {
	writeProfileSample(t, map[string]string{
		"dev.profile": "@var host = http://dev\n",
	})
	m := New(Args{Width: 120, Height: 40}).(model)
	m.active = paneEdit
	m.filesPanel = &filesPanel{all: httpFilesInDir(".")}
	m.filesPanel.loadProfiles()
	m.width = 120
	m.height = 40

	y := m.profileNewRowAbs()
	r, _ := m.Update(tea.MouseMsg{
		Button: tea.MouseButtonLeft, Action: tea.MouseActionPress,
		X: m.filesWidth() / 2, Y: y,
	})
	mm := r.(model)
	if mm.profileAs == nil {
		t.Fatalf("new-profile input not opened on click (profileAs=nil)")
	}
}

// TestProfileEditInEditor verifies a profile opens into the main editor pane
// (like a .http file) with its @var content loaded and the correct file path
// bound, so Ctrl+S / autosave write back to the profile.
func TestProfileEditInEditor(t *testing.T) {
	writeProfileSample(t, map[string]string{
		"dev.profile": "@var host = http://dev\n@var token = abc\n",
	})
	m := New(Args{Width: 120, Height: 40}).(model)
	m.filesPanel = &filesPanel{all: httpFilesInDir(".")}
	m.filesPanel.loadProfiles()
	m.editProfile("dev.profile")
	if m.filePath != "dev.profile" {
		t.Fatalf("filePath=%q want dev.profile", m.filePath)
	}
	if m.active != paneEdit {
		t.Errorf("active=%v want paneEdit (profile edited in main editor)", m.active)
	}
	if !strings.Contains(m.ed.Text(), "@var host = http://dev") {
		t.Errorf("profile content not loaded into editor:\n%s", m.ed.Text())
	}
	// edit and save via the normal editor path
	m.ed.SetText("@var host = http://new\n")
	m.markDirty()
	m.saveBuffer()
	data, _ := os.ReadFile("dev.profile")
	if !strings.Contains(string(data), "@var host = http://new") {
		t.Errorf("profile not saved via editor:\n%s", data)
	}
}

func (m model) pressTab() (model, bool) {
	r, _ := m.handleKey(tea.KeyMsg{Type: tea.KeyTab})
	if rr, ok := r.(model); ok {
		return rr, true
	}
	return m, false
}

func (m model) typeRunes(s string) (model, bool) {
	r, _ := m.handleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)})
	if rr, ok := r.(model); ok {
		return rr, true
	}
	return m, false
}

func (m model) pressDelete() (model, bool) {
	r, _ := m.handleKey(tea.KeyMsg{Type: tea.KeyDelete})
	if rr, ok := r.(model); ok {
		return rr, true
	}
	return m, false
}

func (m model) pressEnter() (model, bool) {
	r, _ := m.handleKey(tea.KeyMsg{Type: tea.KeyEnter})
	if rr, ok := r.(model); ok {
		return rr, true
	}
	return m, false
}

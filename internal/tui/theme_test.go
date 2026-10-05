package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/user/curlyk/internal/i18n"
	"github.com/user/curlyk/internal/settings"
	"github.com/user/curlyk/internal/theme"
)

// TestCycleThemeSwitchesSchemeAndPersists verifies that Ctrl+] wraps the active
// scheme to the next one, updates the status bar, and writes the choice to
// appsettings.yml.
func TestCycleThemeSwitchesSchemeAndPersists(t *testing.T) {
	settings.SetAppSettingsPath(t.TempDir() + "/appsettings.yml")
	i18n.SetLocale("ru")

	m := New(Args{Width: 120, Height: 30}).(model)

	// Start at default.
	applyTheme(theme.DefaultName)
	if curScheme != theme.Default() {
		t.Fatal("expected default scheme at start")
	}

	// Ctrl+] -> darkula.
	m2, _ := m.Update(tea.KeyMsg{Type: tea.KeyCtrlCloseBracket, Runes: []rune{}})
	r := m2.(model)
	if cn := currentSchemeName(); cn != theme.DarkulaName {
		t.Errorf("scheme after first step = %q, want darkula", cn)
	}
	if r.status == "" || !strings.Contains(r.status, "Тема:") {
		t.Errorf("status should mention theme, got %q", r.status)
	}
	if s := settings.Load(); s.Theme != theme.DarkulaName {
		t.Errorf("theme persisted = %q, want darkula", s.Theme)
	}
}

// TestCycleThemeWrapsLastToFirst verifies Ctrl+] from the last builtin scheme
// wraps back to the first.
func TestCycleThemeWrapsToFirst(t *testing.T) {
	settings.SetAppSettingsPath(t.TempDir() + "/appsettings.yml")
	i18n.SetLocale("ru")

	m := New(Args{Width: 120, Height: 30}).(model)

	// Jump straight to light (last builtin), then step forward -> default.
	applyTheme(theme.LightName)
	m2, _ := m.Update(tea.KeyMsg{Type: tea.KeyCtrlCloseBracket, Runes: []rune{}})
	_ = m2
	if cn := currentSchemeName(); cn != theme.DefaultName {
		t.Errorf("scheme after wrap = %q, want default", cn)
	}
}

// TestCustomThemeFromFileApplies verifies a scheme from themes/*.theme is found
// and applied by name.
func TestCustomThemeFromFileApplies(t *testing.T) {
	settings.SetAppSettingsPath(t.TempDir() + "/appsettings.yml")
	i18n.SetLocale("ru")

	oldThemeDir := theme.ThemesDir()
	t.Cleanup(func() { theme.SetThemesDir(oldThemeDir) })

	dir := t.TempDir()
	theme.SetThemesDir(dir)
	name := "mypalette"
	f := theme.ThemeFilePath(name)
	if err := theme.WriteTemplate(name, theme.WriteTemplateOptions{Base: theme.Darkula()}, f); err != nil {
		t.Fatal(err)
	}

	names := theme.ListFilesInDir(dir)
	found := false
	for _, n := range names {
		if n == name {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("custom scheme %q not listed: %v", name, names)
	}

	applyTheme(name)
	if curScheme != theme.Darkula() {
		t.Error("custom theme file should load the Darkula values it was based on")
	}
}

// TestCtrlBackslashStepsBack verifies Ctrl+\ steps the scheme backwards.
func TestCtrlBackslashStepsBack(t *testing.T) {
	settings.SetAppSettingsPath(t.TempDir() + "/appsettings.yml")
	i18n.SetLocale("ru")

	m := New(Args{Width: 120, Height: 30}).(model)
	applyTheme(theme.DarkulaName)

	m2, _ := m.Update(tea.KeyMsg{Type: tea.KeyCtrlBackslash, Runes: []rune{}})
	_ = m2
	if cn := currentSchemeName(); cn != theme.DefaultName {
		t.Errorf("scheme after Ctrl+\\ from darkula = %q, want default", cn)
	}
}

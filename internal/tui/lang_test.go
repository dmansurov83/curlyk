package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/user/curlyk/internal/i18n"
	"github.com/user/curlyk/internal/settings"
)

// TestToggleLanguageSwitchesUIAndPersists verifies that Ctrl+L switches the
// interface between ru and en, updates the status bar, and writes the choice
// to appsettings.yml.
func TestToggleLanguageSwitchesUIAndPersists(t *testing.T) {
	settings.SetAppSettingsPath(t.TempDir() + "/appsettings.yml")
	// Start explicitly in Russian.
	i18n.SetLocale("ru")

	m := New(Args{Width: 120, Height: 30}).(model)

	// Toggle ru -> en.
	m2, _ := m.Update(tea.KeyMsg{Type: tea.KeyCtrlL, Runes: []rune{}})
	r := m2.(model)
	if i18n.Locale() != "en" {
		t.Errorf("locale after first toggle = %q, want en", i18n.Locale())
	}
	if !strings.Contains(r.status, "Language:") {
		t.Errorf("en status should mention language, got %q", r.status)
	}
	out := stripANSI(r.View())
	if !strings.Contains(out, "File:") {
		t.Errorf("en rendering missing 'File:', got:\n%s", out)
	}
	if s := settings.Load(); s.Lang != "en" {
		t.Errorf("lang persisted = %q, want en", s.Lang)
	}

	// Toggle back en -> ru.
	m3, _ := r.Update(tea.KeyMsg{Type: tea.KeyCtrlL, Runes: []rune{}})
	r2 := m3.(model)
	if i18n.Locale() != "ru" {
		t.Errorf("locale after second toggle = %q, want ru", i18n.Locale())
	}
	if !strings.Contains(r2.status, "Язык:") {
		t.Errorf("ru status should mention language, got %q", r2.status)
	}
	if s := settings.Load(); s.Lang != "ru" {
		t.Errorf("lang persisted = %q, want ru", s.Lang)
	}
}

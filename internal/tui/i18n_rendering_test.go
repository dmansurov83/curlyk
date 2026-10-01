package tui

import (
	"os"
	"strings"
	"testing"
)

// TestEnglishLocaleRendering verifies that forcing CURLYK_LANG=en makes the
// rendered UI use English strings (header, status, copy button scope), which is
// the Definition of Done for i18n.
func TestEnglishLocaleRendering(t *testing.T) {
	os.Setenv("CURLYK_LANG", "en")
	defer os.Unsetenv("CURLYK_LANG")

	m := New(Args{Width: 120, Height: 30}).(model)
	out := m.View()
	plain := stripANSI(out)

	if !strings.Contains(plain, "File:") {
		t.Errorf("en header should contain 'File:', got:\n%s", plain)
	}
	if !strings.Contains(plain, "Ready.") {
		t.Errorf("en status bar should contain 'Ready.', got:\n%s", plain)
	}
	if strings.Contains(plain, "Файл:") {
		t.Errorf("en rendering leaked a Russian string 'Файл:':\n%s", plain)
	}
}

// TestRussianLocaleRendering is the default: without forcing a locale the UI
// stays Russian (kept by the existing suite).
func TestRussianLocaleRendering(t *testing.T) {
	os.Unsetenv("CURLYK_LANG")

	m := New(Args{Width: 120, Height: 30}).(model)
	out := stripANSI(m.View())
	if !strings.Contains(out, "Файл:") {
		t.Errorf("ru header should contain 'Файл:', got:\n%s", out)
	}
	if !strings.Contains(out, "Готов") {
		t.Errorf("ru status bar should contain 'Готов', got:\n%s", out)
	}
}

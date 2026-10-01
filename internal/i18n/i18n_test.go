package i18n

import (
	"os"
	"testing"
)

func TestDefaultLocaleRU(t *testing.T) {
	// Ensure the compiler default is ru, not a previously mutated global.
	SetLocale("")
	if Locale() != "ru" {
		t.Fatalf("default locale = %q, want ru", Locale())
	}
}

func TestLookupRU(t *testing.T) {
	SetLocale("ru")
	if got := T("menu.run"); got != "▶ Выполнить" {
		t.Errorf("T menu.run = %q, want ru label", got)
	}
}

func TestSetLocaleEN(t *testing.T) {
	SetLocale("en")
	if Locale() != "en" {
		t.Fatalf("locale after SetLocale(en) = %q", Locale())
	}
	if got := T("menu.run"); got != "▶ Run" {
		t.Errorf("T menu.run = %q, want en label", got)
	}
}

func TestLookupSwitchesBackToRU(t *testing.T) {
	SetLocale("en")
	_ = T("menu.run")
	SetLocale("ru")
	if got := T("menu.run"); got != "▶ Выполнить" {
		t.Errorf("after switching back to ru, got %q", got)
	}
}

func TestFormatArgs(t *testing.T) {
	SetLocale("ru")
	if got := T("resp.duration", "5ms"); got != "Время: 5ms" {
		t.Errorf("T resp.duration = %q, want substitution", got)
	}
	SetLocale("en")
	if got := T("resp.duration", "5ms"); got != "Time: 5ms" {
		t.Errorf("T resp.duration (en) = %q, want substitution", got)
	}
}

func TestFormatMultipleArgs(t *testing.T) {
	SetLocale("ru")
	if got := T("err.openFile", "a.http", "perm denied"); got != "Не удалось открыть a.http: perm denied" {
		t.Errorf("T err.openFile = %q", got)
	}
}

func TestFormatDoesNotInjectIntoFormatString(t *testing.T) {
	// The user-controlled error text is passed as the %s argument, never as the
	// format string, so a %!v or %d inside it must stay literal text.
	SetLocale("ru")
	malicious := "boom %!v(MISSING) %d"
	if got := T("err.save", malicious); got != "Не удалось сохранить: "+malicious {
		t.Errorf("T err.save did not keep argument literal: %q", got)
	}
}

func TestReadyMainFitsStatusBar(t *testing.T) {
	// renderStatus draws the help line with a " " + text into Width(width).
	// At the smallest default window width (100) the text plus the leading
	// space must fit on one line, otherwise it wraps onto a second row and
	// breaks the frame-height invariant (TestMenuOpenKeepsFrameFit).
	for _, lang := range []string{"ru", "en"} {
		SetLocale(lang)
		text := T("ready.main")
		if len([]rune(text)) > 99 {
			t.Errorf("%s: ready.main %d runes > 99 (will overflow a 100-cell status bar): %q", lang, len([]rune(text)), text)
		}
	}
	SetLocale("ru")
}

func TestUnknownKeyFallsBack(t *testing.T) {
	SetLocale("en")
	if got := T("not.real.key"); got != "not.real.key" {
		t.Errorf("unknown key should return the key itself, got %q", got)
	}
}

func TestUnknownLocaleKeepsCurrent(t *testing.T) {
	SetLocale("ru")
	SetLocale("xx")
	if Locale() != "ru" {
		t.Errorf("SetLocale(xx) should keep current ru, got %q", Locale())
	}
}

func TestResolveEnvWins(t *testing.T) {
	os.Setenv("CURLYK_LANG", "en")
	defer os.Unsetenv("CURLYK_LANG")
	if got := Resolve("ru"); got != "en" {
		t.Errorf("Resolve with env en + pref ru = %q, want en", got)
	}
}

func TestResolvePrefWhenNoEnv(t *testing.T) {
	os.Unsetenv("CURLYK_LANG")
	if got := Resolve("en"); got != "en" {
		t.Errorf("Resolve(en) without env = %q, want en", got)
	}
}

func TestResolveDefaultsRU(t *testing.T) {
	os.Unsetenv("CURLYK_LANG")
	if got := Resolve(""); got != "ru" {
		t.Errorf("Resolve() = %q, want ru", got)
	}
	if got := Resolve("xx"); got != "ru" {
		t.Errorf("Resolve(xx) = %q, want ru", got)
	}
}

func TestResolveIgnoresUnknownEnv(t *testing.T) {
	os.Setenv("CURLYK_LANG", "fr")
	defer os.Unsetenv("CURLYK_LANG")
	if got := Resolve(""); got != "ru" {
		t.Errorf("Resolve with unsupported env fr = %q, want ru", got)
	}
}

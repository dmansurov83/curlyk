package tui

import (
	"strings"
	"testing"
)

func TestFormatBodyJSON(t *testing.T) {
	got := formatBody([]byte(`{"a":1,"b":[1,2]}`))
	if !strings.Contains(got, "\n  \"a\"") {
		t.Errorf("json not indented:\n%q", got)
	}
	if !strings.Contains(got, "\"b\"") || !strings.Contains(got, "[") {
		t.Errorf("cats missing:\n%q", got)
	}
}

func TestFormatBodyNonJSON(t *testing.T) {
	got := formatBody([]byte("hello world"))
	if got != "hello world" {
		t.Errorf("non-json should be verbatim: %q", got)
	}
}

func TestIsCurlCommandExe(t *testing.T) {
	if !isCurlCommand(`curl.exe "https://api.test/x" -H "A: b"`) {
		t.Error("curl.exe should be detected")
	}
	if !isCurlCommand("curl https://x") {
		t.Error("curl should be detected")
	}
}
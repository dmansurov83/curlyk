package tui

import (
	"strings"
	"testing"

	"github.com/user/curlyk/httptool/internal/curl"
)

func TestIsCurlCommand(t *testing.T) {
	cases := []struct {
		in   string
		want bool
	}{
		{"curl -X POST http://x", true},
		{"curl http://x", true},
		{"CURL -X GET http://x", true},
		{"  curl -d 'a=1' http://x", true},
		{"", false},
		{"hello world", false},
		{"GET http://x", false},
	}
	for _, c := range cases {
		if got := isCurlCommand(c.in); got != c.want {
			t.Errorf("isCurlCommand(%q)=%v want %v", c.in, got, c.want)
		}
	}
}

func TestPasteCurlConverts(t *testing.T) {
	m := New(Args{Width: 100, Height: 24}).(model)
	m.active = paneEdit
	// We cannot easily mock the OS clipboard, but we can test the conversion
	// path through the same ImportLine + insertText used by curlPaste.
	block, err := curl.ImportLine(`curl -X POST https://api.test/x -H "Content-Type: application/json" -d '{"a":1}'`)
	if err != nil {
		t.Fatal(err)
	}
	m.insertText(block)
	text := m.ed.Text()
	if !strings.Contains(text, "POST https://api.test/x") {
		t.Errorf("curl not converted to request:\n%s", text)
	}
	if !strings.Contains(text, `{"a":1}`) {
		t.Errorf("body missing:\n%s", text)
	}
}
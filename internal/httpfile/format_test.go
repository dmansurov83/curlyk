package httpfile

import (
	"errors"
	"strings"
	"testing"
)

// TestFormatJSONBodyIndentsAndReportsRange covers the shared formatter used by
// the TUI action menu and the desktop window's request menu.
func TestFormatJSONBodyIndentsAndReportsRange(t *testing.T) {
	req := Request{
		Method:    "POST",
		URL:       "http://a",
		Body:      `{"a":1,"b":[2,3]}`,
		BodyStart: 5,
		BodyEnd:   5,
	}
	formatted, start, end, err := FormatJSONBody(req)
	if err != nil {
		t.Fatalf("FormatJSONBody: %v", err)
	}
	want := "{\n  \"a\": 1,\n  \"b\": [\n    2,\n    3\n  ]\n}"
	if formatted != want {
		t.Fatalf("formatted =\n%s\nwant\n%s", formatted, want)
	}
	if start != 4 || end != 5 {
		t.Fatalf("range = [%d,%d), want [4,5)", start, end)
	}
}

func TestFormatJSONBodyErrors(t *testing.T) {
	cases := []struct {
		name string
		req  Request
		want error
	}{
		{"no body", Request{BodyStart: 3, BodyEnd: 3}, ErrNoJSONBody},
		{"blank body", Request{Body: "   \n", BodyStart: 3, BodyEnd: 3}, ErrNotJSON},
		{"invalid json", Request{Body: "{oops", BodyStart: 3, BodyEnd: 3}, ErrNotJSON},
		{"unknown range", Request{Body: `{"a":1}`, BodyStart: 0, BodyEnd: 0}, ErrBodyRange},
	}
	for _, c := range cases {
		_, _, _, err := FormatJSONBody(c.req)
		if !errors.Is(err, c.want) {
			t.Errorf("%s: err = %v, want %v", c.name, err, c.want)
		}
	}
}

// TestFormatJSONBodyAcceptsPaddedBody keeps the leniency the TUI had: a body
// padded with spaces or newlines still formats.
func TestFormatJSONBodyAcceptsPaddedBody(t *testing.T) {
	req := Request{Body: "  {\"a\":1}  \n", BodyStart: 2, BodyEnd: 2}
	formatted, _, _, err := FormatJSONBody(req)
	if err != nil {
		t.Fatalf("FormatJSONBody: %v", err)
	}
	if !strings.Contains(formatted, "\"a\": 1") {
		t.Fatalf("formatted = %q", formatted)
	}
}

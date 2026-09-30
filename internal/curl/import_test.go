package curl

import (
	"testing"
)

func TestShellSplit(t *testing.T) {
	got, err := ShellSplit(`-H "Content-Type: application/json" -d '{"a":1}' http://x`)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"-H", "Content-Type: application/json", "-d", `{"a":1}`, "http://x"}
	if len(got) != len(want) {
		t.Fatalf("got %v want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("arg[%d]=%q want %q", i, got[i], want[i])
		}
	}
}

func TestImportBasic(t *testing.T) {
	argv, _ := ShellSplit(`-X POST -H "Content-Type: application/json" -d '{"title":"x"}' https://api.test/items`)
	p, err := ImportCommand(argv)
	if err != nil {
		t.Fatal(err)
	}
	if p.Method != "POST" {
		t.Errorf("method=%s", p.Method)
	}
	if p.URL != "https://api.test/items" {
		t.Errorf("url=%s", p.URL)
	}
	if len(p.Headers) != 1 || p.Headers[0].Name != "Content-Type" {
		t.Errorf("headers=%+v", p.Headers)
	}
	if p.Data != `{"title":"x"}` {
		t.Errorf("data=%q", p.Data)
	}
}

func TestToHTTPString(t *testing.T) {
	argv, _ := ShellSplit(`curl -X POST https://api.test/items -H "X-Token: abc" -d '{"a":1}'`)
	p, err := ImportCommand(argv[1:]) // drop "curl"
	if err != nil {
		t.Fatal(err)
	}
	out := ToHTTPString(p)
	t.Logf("argv=%v\np=%+v\nout:\n%s", argv[1:], p, out)
	wantSub := "POST https://api.test/items\nX-Token: abc\nContent-Type: application/json\n\n{\"a\":1}"
	_ = wantSub
	if !contains(out, "POST https://api.test/items") {
		t.Errorf("missing request line:\n%s", out)
	}
	if !contains(out, "X-Token: abc") {
		t.Errorf("missing header:\n%s", out)
	}
	if !contains(out, "Content-Type: application/json") {
		t.Errorf("missing inferred content-type:\n%s", out)
	}
	if !contains(out, `{"a":1}`) {
		t.Errorf("missing body:\n%s", out)
	}
	_ = wantSub
}

func contains(s, sub string) bool {
	return len(sub) == 0 || (len(sub) <= len(s) && indexOf(s, sub) >= 0)
}

func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}
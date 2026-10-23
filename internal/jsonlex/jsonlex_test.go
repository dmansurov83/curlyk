package jsonlex

import (
	"strings"
	"testing"
)

func TestLexMarksKeysAndValues(t *testing.T) {
	toks := Lex(`  "name": "Ada", "n": 1`)
	want := []Token{
		{WS, 0, 2},
		{Key, 2, 8},
		{Punct, 8, 9},
		{WS, 9, 10},
		{String, 10, 15},
		{Punct, 15, 16},
		{WS, 16, 17},
		{Key, 17, 20},
		{Punct, 20, 21},
		{WS, 21, 22},
		{Number, 22, 23},
	}
	if len(toks) != len(want) {
		t.Fatalf("got %d tokens, want %d: %+v", len(toks), len(want), toks)
	}
	for i := range want {
		if toks[i] != want[i] {
			t.Errorf("token %d = %+v, want %+v", i, toks[i], want[i])
		}
	}
}

// TestLexEscapedQuoteKeepsStringOneToken guards the escape handling: a \" inside
// a value must not end the token early.
func TestLexEscapedQuoteKeepsStringOneToken(t *testing.T) {
	toks := Lex(`"a\"b"`)
	if len(toks) != 1 {
		t.Fatalf("got %d tokens, want 1: %+v", len(toks), toks)
	}
	if toks[0].Kind != String || toks[0].Start != 0 || toks[0].End != 6 {
		t.Fatalf("token = %+v, want String [0,6)", toks[0])
	}
}

func TestLexClassifiesLiteralsAndPunct(t *testing.T) {
	toks := Lex(`{"a": true, "b": null, "c": -1.5e3}`)
	got := map[Kind]int{}
	for _, tk := range toks {
		got[tk.Kind]++
	}
	if got[Bool] != 1 || got[Null] != 1 || got[Number] != 1 {
		t.Fatalf("literal kinds = %+v", got)
	}
	if got[Punct] != 7 { // { } , , : : :
		t.Fatalf("punct count = %d, want 7", got[Punct])
	}
}

// TestLexNonASCIIUsesRuneIndexes covers the rune/byte bridge: a key carrying
// non-ASCII runes must report rune indexes, and MarkKeys must compare against
// the right character (the pre-extraction code indexed bytes with a rune index).
func TestLexNonASCIIUsesRuneIndexes(t *testing.T) {
	const line = `{"имя": "значение"}`
	toks := Lex(line)
	runes := []rune(line)
	for _, tk := range toks {
		if tk.Start < 0 || tk.End > len(runes) {
			t.Fatalf("token %+v out of rune range (line has %d runes)", tk, len(runes))
		}
	}
	var keys, strings int
	for _, tk := range toks {
		switch tk.Kind {
		case Key:
			keys++
		case String:
			strings++
		}
	}
	if keys != 1 || strings != 1 {
		t.Fatalf("keys=%d strings=%d, want 1 and 1: %+v", keys, strings, toks)
	}
}

// TestMarkKeysIsIdempotent allows callers to run it on already-marked tokens.
func TestMarkKeysIsIdempotent(t *testing.T) {
	line := `"a": 1`
	once := Lex(line)
	twice := MarkKeys(line, Lex(line))
	for i := range once {
		if once[i] != twice[i] {
			t.Fatalf("token %d changed on second pass: %+v vs %+v", i, once[i], twice[i])
		}
	}
}

func TestIsBodyStart(t *testing.T) {
	for _, line := range []string{"{", "  [", `"x"`, "}", "]", ", more"} {
		if !IsBodyStart(line) {
			t.Errorf("IsBodyStart(%q) = false, want true", line)
		}
	}
	for _, line := range []string{"", "   ", "GET http://x", "# c", "name: v"} {
		if IsBodyStart(line) {
			t.Errorf("IsBodyStart(%q) = true, want false", line)
		}
	}
}

func TestPrettyFormatsJSON(t *testing.T) {
	in := []byte(`{"a":1,"b":[2,3],"s":"x"}`)
	got := Pretty(in)
	want := "{\n  \"a\": 1,\n  \"b\": [\n    2,\n    3\n  ],\n  \"s\": \"x\"\n}"
	if got != want {
		t.Fatalf("Pretty =\n%s\nwant\n%s", got, want)
	}
}

func TestPrettyLeavesNonJSONUntouched(t *testing.T) {
	for _, in := range []string{"", "   ", "not json", "<html>hi</html>", "hello world"} {
		if got := Pretty([]byte(in)); got != in {
			t.Errorf("Pretty(%q) = %q, want it unchanged", in, got)
		}
	}
}

// TestPrettyWorksHeadlessly keeps the formatter UI-free: it must not require a
// display, only the standard library.
func TestPrettyWorksHeadlessly(t *testing.T) {
	got := Pretty([]byte(`[1, 2, 3]`))
	if !strings.Contains(got, "\n  2,\n") {
		t.Fatalf("Pretty = %q, want an indented array", got)
	}
}

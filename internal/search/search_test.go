package search

import "testing"

func TestFindReadsLinesInOrder(t *testing.T) {
	lines := []string{"foo bar", "foofoo", "abc"}
	got := Find(lines, "foo")
	want := []Match{{0, 0}, {1, 0}, {1, 3}}
	if len(got) != len(want) {
		t.Fatalf("got %d matches, want %d: %+v", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("match %d = %+v, want %+v", i, got[i], want[i])
		}
	}
	if n := Count(lines, "foo"); n != 3 {
		t.Errorf("Count = %d, want 3", n)
	}
}

// TestFindIsCaseInsensitiveAndOverlapping pins the behaviour the TUI relied on
// before the extraction: matching ignores case and overlapping occurrences are
// all reported.
func TestFindIsCaseInsensitiveAndOverlapping(t *testing.T) {
	if got := FindLine("aaa", "aa"); len(got) != 2 || got[0] != 0 || got[1] != 1 {
		t.Fatalf("FindLine(aaa, aa) = %v, want [0 1]", got)
	}
	if got := FindLine("Hello World", "WORLD"); len(got) != 1 || got[0] != 6 {
		t.Fatalf("FindLine = %v, want [6]", got)
	}
}

func TestFindUsesRuneColumns(t *testing.T) {
	got := FindLine("привет мир", "мир")
	if len(got) != 1 || got[0] != 7 {
		t.Fatalf("FindLine = %v, want [7] runes", got)
	}
}

func TestFindEmptyQueryMatchesNothing(t *testing.T) {
	if got := Find([]string{"a", "b"}, ""); got != nil {
		t.Errorf("Find with empty query = %v, want nil", got)
	}
	if got := FindLine("abc", ""); got != nil {
		t.Errorf("FindLine with empty query = %v, want nil", got)
	}
}

func TestFindLongerQueryThanLine(t *testing.T) {
	if got := FindLine("ab", "abc"); got != nil {
		t.Errorf("FindLine = %v, want nil", got)
	}
}

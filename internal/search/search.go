// Package search is the UI-free substring search shared by the TUI's find bar
// and the GUI's editor search. It only knows about plain lines and query text:
// matching is case-insensitive, works in runes (so Cyrillic text matches by
// character, not by byte) and reports every occurrence, including overlapping
// ones ("aa" occurs twice in "aaa").
package search

import "strings"

// Match is one occurrence of a query: the 0-based line index and the 0-based
// rune column where the match starts. Its length is always the query length in
// runes.
type Match struct {
	Line int
	Col  int
}

// Find returns every case-insensitive occurrence of query across lines, in
// reading order (line-major, then column). An empty query matches nothing.
func Find(lines []string, query string) []Match {
	cols := queryRunes(query)
	if len(cols) == 0 {
		return nil
	}
	var out []Match
	for li, line := range lines {
		for _, col := range findInRunes([]rune(strings.ToLower(line)), cols) {
			out = append(out, Match{Line: li, Col: col})
		}
	}
	return out
}

// FindLine returns the rune columns of every case-insensitive occurrence of
// query in a single line, in ascending order. An empty query matches nothing.
func FindLine(line, query string) []int {
	cols := queryRunes(query)
	if len(cols) == 0 {
		return nil
	}
	return findInRunes([]rune(strings.ToLower(line)), cols)
}

// Count returns the number of matches of query across lines.
func Count(lines []string, query string) int { return len(Find(lines, query)) }

// queryRunes lowercases the query and returns it as runes.
func queryRunes(query string) []rune {
	if query == "" {
		return nil
	}
	return []rune(strings.ToLower(query))
}

// findInRunes scans a lowercased line for every occurrence of the lowercased
// query, stepping one rune at a time so overlapping matches are all reported.
func findInRunes(low, query []rune) []int {
	if len(query) == 0 || len(low) < len(query) {
		return nil
	}
	var out []int
	for i := 0; i+len(query) <= len(low); i++ {
		if string(low[i:i+len(query)]) == string(query) {
			out = append(out, i)
		}
	}
	return out
}

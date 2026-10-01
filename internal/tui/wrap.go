package tui

import (
	"strings"

	"github.com/mattn/go-runewidth"
)

// wrapToWidth breaks a single long line of text at word boundaries so each
// resulting line fits within max cells (in rune-aware display width, so wide
// runes and spaces are handled). Existing newlines are preserved and continue
// the wrapping independently.
func wrapToWidth(s string, max int) string {
	if max <= 0 {
		return s
	}
	var out []string
	for _, line := range strings.Split(s, "\n") {
		out = append(out, wrapLine(line, max)...)
	}
	return strings.Join(out, "\n")
}

// wrapLine word-wraps one line (no embedded newlines) to max display cells,
// breaking at spaces and hard-splitting any single word wider than max.
func wrapLine(line string, max int) []string {
	runes := []rune(line)
	var result []string
	cur := []rune{}
	curW := 0
	writeWord := func(word []rune, wordW int) {
		if wordW <= max {
			// fits on its own
			if curW > 0 && curW+1+wordW > max {
				result = append(result, string(cur))
				cur = nil
				curW = 0
			} else if curW > 0 {
				cur = append(cur, ' ')
				curW++
			}
			cur = append(cur, word...)
			curW += wordW
			return
		}
		// over-wide word: if the line isn't empty, wrap first, then hard-split.
		if curW > 0 {
			result = append(result, string(cur))
			cur = nil
			curW = 0
		}
		for _, r := range word {
			rw := runewidth.RuneWidth(r)
			if curW > 0 && curW+rw > max {
				result = append(result, string(cur))
				cur = nil
				curW = 0
			}
			cur = append(cur, r)
			curW += rw
		}
	}
	word := []rune{}
	wordW := 0
	flushWord := func() {
		if len(word) > 0 {
			writeWord(word, wordW)
			word = nil
			wordW = 0
		}
	}
	for _, r := range runes {
		if r == ' ' {
			flushWord()
			continue
		}
		word = append(word, r)
		wordW += runewidth.RuneWidth(r)
	}
	flushWord()
	if len(cur) > 0 {
		result = append(result, string(cur))
	}
	return result
}

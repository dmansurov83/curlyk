package httpfile

import (
	"crypto/rand"
	"fmt"
	"math/big"
	"strings"
	"time"
)

// SubstitutionError reports unresolved {{...}} placeholders found while
// substituting variables, so the caller can abort instead of silently sending
// a raw "{{x}}" to the wire.
type SubstitutionError struct {
	// Missing holds the unresolved names, in order of first appearance.
	Missing []string
}

// Error implements the error interface.
func (e *SubstitutionError) Error() string {
	if len(e.Missing) == 0 {
		return "undefined variable"
	}
	return fmt.Sprintf("undefined variable: %s", e.Missing[0])
}

// First returns the first unresolved variable/function name, or "" if none.
func (e *SubstitutionError) First() string {
	if len(e.Missing) == 0 {
		return ""
	}
	return e.Missing[0]
}

// Substitute replaces {{name}} and {{$fn}} placeholders in s against the file
// variables in vars and the built-in functions. Placeholders are scanned
// left-to-right and the replacement text is not re-scanned, so a value that
// itself contains "{{" is inserted verbatim.
//
// The returned substitution and error are meaningful together: when err is
// nil every placeholder was resolved; otherwise the string is partially
// substituted (resolved parts replaced, unresolved kept) and err.(
// *SubstitutionError).Missing lists what could not be resolved.
func Substitute(s string, vars map[string]string) (string, error) {
	var miss []string
	var b strings.Builder
	i := 0
	for i < len(s) {
		open := strings.Index(s[i:], "{{")
		if open < 0 {
			b.WriteString(s[i:])
			break
		}
		start := i + open
		// copy everything before the placeholder
		b.WriteString(s[i:start])
		closeIdx := strings.Index(s[start+2:], "}}")
		if closeIdx < 0 {
			// unmatched "{{": treat the rest literally
			b.WriteString(s[start:])
			break
		}
		end := start + 2 + closeIdx + 2
		name := s[start+2 : end-2]
		val, ok := resolveVar(name, vars)
		if ok {
			b.WriteString(val)
		} else {
			miss = append(miss, name)
			// keep the placeholder intact so callers can show exactly what
			// was unresolved
			b.WriteString(s[start:end])
		}
		i = end
	}
	if len(miss) > 0 {
		return b.String(), &SubstitutionError{Missing: miss}
	}
	return b.String(), nil
}

// resolveVar resolves a single placeholder body. A leading "$" selects a
// built-in function; anything else is looked up in vars.
func resolveVar(name string, vars map[string]string) (string, bool) {
	if strings.HasPrefix(name, "$") {
		return builtinValue(name)
	}
	v, ok := vars[name]
	return v, ok
}

// builtinValue computes the value of a built-in $-function. Unknown functions
// report false.
func builtinValue(fn string) (string, bool) {
	switch fn {
	case "$timestamp":
		return fmt.Sprintf("%d", time.Now().Unix()), true
	case "$isoTimestamp":
		return time.Now().Format(time.RFC3339), true
	case "$random.uuid", "$guid":
		return randomUUID(), true
	case "$random.int":
		n, err := randomInt(1000000)
		if err != nil {
			return "", false
		}
		return fmt.Sprintf("%d", n), true
	}
	return "", false
}

// randomUUID returns a random version-4 UUID, or a deterministic fallback if
// the crypto source fails.
func randomUUID() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "00000000-0000-4000-8000-000000000000"
	}
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}

// randomInt returns a secure random integer in [0, n).
func randomInt(n int) (int, error) {
	bi, err := rand.Int(rand.Reader, big.NewInt(int64(n)))
	if err != nil {
		return 0, err
	}
	return int(bi.Int64()), nil
}

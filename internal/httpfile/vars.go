package httpfile

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"math/big"
	"net/url"
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
// left-to-right. A variable value may itself contain {{...}} placeholders
// referencing other variables (or built-ins); those are resolved recursively
// up to the depth of the reference chain. Cycles (a variable that transitively
// references itself) are detected and the cyclic placeholder is kept intact.
//
// The returned substitution and error are meaningful together: when err is
// nil every placeholder was resolved; otherwise the string is partially
// substituted (resolved parts replaced, unresolved kept) and err.(
// *SubstitutionError).Missing lists what could not be resolved.
func Substitute(s string, vars map[string]string) (string, error) {
	s, miss := substitute(s, vars, map[string]bool{})
	if len(miss) > 0 {
		return s, &SubstitutionError{Missing: miss}
	}
	return s, nil
}

// substitute replaces placeholders in s, resolving variable values
// recursively. seen tracks the variables currently being resolved so a
// reference cycle is not followed forever; a value hit again while on the
// current resolution stack keeps its placeholder and is reported as missing.
func substitute(s string, vars map[string]string, seen map[string]bool) (string, []string) {
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
		// Find the "}}" that closes this placeholder, skipping over any nested
		// "{{...}}" groups inside (e.g. a function argument that itself carries
		// placeholders like {{b64({{a}}:{{b}})}}). Without this, the inner "}}"
		// would terminate the outer placeholder prematurely.
		depth := 1
		end := -1
		for j := start + 2; j < len(s)-1; {
			if s[j] == '}' && s[j+1] == '}' {
				depth--
				if depth == 0 {
					end = j + 2
					break
				}
				j += 2
			} else if s[j] == '{' && s[j+1] == '{' {
				depth++
				j += 2
			} else {
				j++
			}
		}
		if end < 0 {
			// unmatched "{{": treat the rest literally
			b.WriteString(s[start:])
			break
		}
		name := s[start+2 : end-2]
		// Built-in functions with arguments: "{{fn(arg)}}" where arg may itself
		// contain nested {{...}} placeholders. These resolve by substituting the
		// argument first, then applying the function to the result.
		if fnCall, ok := splitFnCall(name); ok {
			argSub, argMiss := outerSubstitute(fnCall.Arg, vars)
			if len(argMiss) > 0 {
				miss = append(miss, argMiss...)
			}
			val, fok := builtinFnValue(fnCall.Name, argSub)
			if fok {
				b.WriteString(val)
			} else {
				miss = append(miss, fnCall.Name)
				b.WriteString(s[start:end])
			}
			i = end
			continue
		}
		// Built-in functions with no arguments ({{$fn}}) resolve directly
		// without participating in cycle detection.
		if strings.HasPrefix(name, "$") {
			val, ok := builtinValue(name)
			if ok {
				b.WriteString(val)
			} else {
				miss = append(miss, name)
				b.WriteString(s[start:end])
			}
			i = end
			continue
		}
		if seen[name] {
			// Cyclic reference: keep the placeholder and bail out of this chain.
			miss = append(miss, name)
			b.WriteString(s[start:end])
			i = end
			continue
		}
		val, ok := vars[name]
		if !ok {
			miss = append(miss, name)
			b.WriteString(s[start:end])
			i = end
			continue
		}
		// Resolve references inside the variable's own value. The resolved
		// value is inserted at this position; references within it are
		// substituted by direct recursion (not a re-scan of the whole output).
		seen[name] = true
		sub, cycle := substitute(val, vars, seen)
		delete(seen, name)
		if len(cycle) > 0 {
			miss = append(miss, cycle...)
		}
		b.WriteString(sub)
		i = end
	}
	return b.String(), miss
}

// ParseProfileVars parses the @var declarations of a profile file (a *.profile
// file whose lines use the same "@var name = value" syntax as .http files). It
// returns the parsed name->value map. The file needs no request blocks; only
// declarations are read. Both the explicit "@var name = value" form and the
// shorthand "@name = value" form (any line beginning with "@" that contains an
// "=") are accepted, so a profile can be written either way; lines that match
// neither are ignored.
func ParseProfileVars(src string) map[string]string {
	vars := map[string]string{}
	for _, raw := range strings.Split(src, "\n") {
		line := strings.TrimSpace(strings.TrimSuffix(raw, "\r"))
		var key, val string
		if strings.HasPrefix(line, "@var") {
			key, val = parseVarLine(line)
		} else if strings.HasPrefix(line, "@") && strings.Contains(line, "=") {
			key, val = parseBareProfileVar(line)
		}
		if key != "" {
			vars[key] = val
		}
	}
	return vars
}

// parseBareProfileVar parses a shorthand "@name = value" line (the "=" is
// already known to be present). A line like "@host = http://x" yields the
// variable host. Returns ("", "") when the "@" line carries no valid name.
func parseBareProfileVar(trim string) (string, string) {
	rest := strings.TrimSpace(trim[1:])
	if rest == "" {
		return "", ""
	}
	name := rest
	val := ""
	for i := 0; i < len(rest); i++ {
		if rest[i] == '=' {
			name = strings.TrimSpace(rest[:i])
			val = strings.TrimSpace(rest[i+1:])
			break
		}
	}
	if name == "" || !validAnnotationName(name) {
		return "", ""
	}
	return name, val
}

// MergeVars returns a copy of base with the entries of override layered on top.
// Profile variables win over file variables when the caller chooses then as the
// override layer. The base map is not mutated.
func MergeVars(base, override map[string]string) map[string]string {
	merged := make(map[string]string, len(base)+len(override))
	for k, v := range base {
		merged[k] = v
	}
	for k, v := range override {
		merged[k] = v
	}
	return merged
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

// fnCall is a parsed function invocation "name(arg)" or "name()".
type fnCall struct {
	Name string
	Arg  string
}

// splitFnCall reports whether name looks like a function call "fn(arg)". The
// name must be a known argument-taking function; the argument may be empty
// ("fn()") or any text delimited by balanced parentheses. This is called only
// on a name already extracted from a {{...}} placeholder.
func splitFnCall(name string) (fnCall, bool) {
	rest := name
	if !strings.HasPrefix(rest, "$") {
		// arg-taking functions are written without the "$" prefix, e.g.
		// {{b64(user:pass)}}; a call must open a parenthesis right after the name.
		open := strings.IndexByte(rest, '(')
		if open <= 0 || !strings.HasSuffix(rest, ")") {
			return fnCall{}, false
		}
		fn := rest[:open]
		// Only known functions are treated as calls; anything else is left
		// literally so it can fall through to variable lookup / be reported.
		switch fn {
		case "b64", "base64", "urlencode", "urldecode":
		default:
			return fnCall{}, false
		}
		return fnCall{Name: fn, Arg: rest[open+1 : len(rest)-1]}, true
	}
	return fnCall{}, false
}

// outerSubstitute substitutes placeholders in a function argument without cycle
// tracking (arguments never reference themselves through a value chain) and
// without the outer nesting wrapper. It reuses the same engine.
func outerSubstitute(arg string, vars map[string]string) (string, []string) {
	return substitute(arg, vars, map[string]bool{})
}

// builtinFnValue applies a known argument-taking built-in function to arg and
// returns the result. Unknown functions report false. The argument is already
// fully substituted by the caller.
func builtinFnValue(fn, arg string) (string, bool) {
	switch fn {
	case "b64", "base64":
		return base64.StdEncoding.EncodeToString([]byte(arg)), true
	case "urlencode":
		return urlQueryEscape(arg), true
	case "urldecode":
		out, err := urlQueryUnescape(arg)
		if err != nil {
			return "", false
		}
		return out, true
	}
	return "", false
}

// urlQueryEscape percent-encodes s for use in a query/FORM body ("+" for space).
func urlQueryEscape(s string) string {
	return url.QueryEscape(s)
}

// urlQueryUnescape decodes a percent-encoded string.
func urlQueryUnescape(s string) (string, error) {
	return url.QueryUnescape(s)
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

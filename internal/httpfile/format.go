package httpfile

import (
	"bytes"
	"encoding/json"
	"errors"
	"strings"
)

// Errors reported by FormatJSONBody so each front end can phrase them its own
// way (the TUI and the desktop window keep their own status lines).
var (
	// ErrNoJSONBody means the request has no body to format.
	ErrNoJSONBody = errors.New("request has no body")
	// ErrNotJSON means the body is not valid JSON.
	ErrNotJSON = errors.New("body is not valid JSON")
	// ErrBodyRange means the body line range is unusable.
	ErrBodyRange = errors.New("request body line range is unknown")
)

// FormatJSONBody pretty-prints the JSON body of a request with two-space
// indentation and returns the formatted text together with the 0-based,
// end-exclusive line range [start,end) the formatted body replaces.
//
// It is shared by the TUI's action menu and the desktop window's request menu,
// so both format a body identically.
func FormatJSONBody(req Request) (formatted string, start, end int, err error) {
	if req.Body == "" {
		return "", 0, 0, ErrNoJSONBody
	}
	trimmed := strings.TrimSpace(req.Body)
	if trimmed == "" || !json.Valid([]byte(trimmed)) {
		return "", 0, 0, ErrNotJSON
	}

	var out bytes.Buffer
	if err := json.Indent(&out, []byte(trimmed), "", "  "); err != nil {
		return "", 0, 0, err
	}
	if req.BodyStart <= 0 || req.BodyEnd < req.BodyStart {
		return "", 0, 0, ErrBodyRange
	}
	// The editor works with 0-based lines; BodyEnd is inclusive, so it becomes
	// the exclusive end as-is.
	return out.String(), req.BodyStart - 1, req.BodyEnd, nil
}

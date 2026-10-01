package curl

import "strings"

// ExportRequest renders a cURL command line from an HTTP request.
// method/url are required; headers is a list of "Name: value"; body may be "".
// Output is a single-line-ish cURL with proper shell quoting.
func ExportRequest(method, url string, headers []string, body string) string {
	var parts []string
	parts = append(parts, "curl")

	hasBody := body != ""
	// Method: emit -X <METHOD> when it isn't implied (GET with body is POST by curl default).
	if method != "" {
		if method == "POST" && hasBody {
			// curl infers POST from -d; -X still fine to be explicit
			parts = append(parts, "-X", method)
		} else if method != "GET" {
			parts = append(parts, "-X", method)
		}
	}

	if url != "" {
		parts = append(parts, quote(url))
	}

	for _, h := range headers {
		h = strings.TrimSpace(h)
		if h == "" {
			continue
		}
		parts = append(parts, "-H", quote(h))
	}

	if hasBody {
		parts = append(parts, "-d", quote(body))
	}

	return strings.Join(parts, " ")
}

// quote wraps a shell argument in single quotes, escaping embedded single quotes.
func quote(s string) string {
	if !strings.ContainsAny(s, " '\\\n\"{}[],:;=`)") {
		return s
	}
	// single-quote style: replace ' with '\''
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

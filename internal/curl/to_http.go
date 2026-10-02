package curl

import (
	"encoding/base64"
	"strings"
)

// ToHTTPString renders a Parsed cURL result as an .http request block source.
func ToHTTPString(p *Parsed) string {
	var b strings.Builder

	method := p.Method
	if method == "" {
		method = "GET"
	}

	// Request line with explicit HTTP/1.1
	b.WriteString(method)
	b.WriteString(" ")
	b.WriteString(p.URL)
	b.WriteString("\n")

	// Authentication handled via headers.
	if p.User != "" && !hasHeader(p.Headers, "Authorization") {
		b.WriteString("Authorization: Basic ")
		b.WriteString(base64.StdEncoding.EncodeToString([]byte(p.User)))
		b.WriteString("\n")
	}

	// Explicit headers.
	for _, h := range p.Headers {
		b.WriteString(h.Name)
		b.WriteString(": ")
		b.WriteString(h.Value)
		b.WriteString("\n")
	}

	// Content-Type inferred.
	ct := inferContentType(p)
	if ct != "" && !hasHeader(p.Headers, "Content-Type") {
		b.WriteString("Content-Type: ")
		b.WriteString(ct)
		b.WriteString("\n")
	}

	// Body.
	if p.Data != "" {
		b.WriteString("\n")
		b.WriteString(p.Data)
		b.WriteString("\n")
	}

	// Form fields as body text block.
	if p.Form && p.Data == "" {
		b.WriteString("\n")
		for _, f := range p.FormFields {
			b.WriteString("--form ")
			b.WriteString(f.Name)
			b.WriteString("=")
			if f.HasFile {
				b.WriteString("@")
				b.WriteString(f.Filename)
			} else {
				b.WriteString(f.Value)
			}
			b.WriteString("\n")
		}
	}

	return b.String()
}

// inferContentType guesses the Content-Type for a request that came from a
// cURL command. The body is treated as application/x-www-form-urlencoded when
// its shape is a flat set of key=value pairs joined by & (the typical
// OAuth2 token request), otherwise it is JSON. Multipart (-F) wins over both.
// Returns "" when there is no body.
func inferContentType(p *Parsed) string {
	if p.Form {
		return "multipart/form-data"
	}
	if p.Data == "" {
		return ""
	}
	if looksLikeForm(p.Data) {
		return "application/x-www-form-urlencoded"
	}
	return "application/json"
}

// looksLikeForm reports whether a body string has the shape of an
// application/x-www-form-urlencoded payload: a single line made of at least
// two key=value pairs separated by '&', with an '=' present in every group.
// It deliberately avoids heuristics that would misclassify JSON.
func looksLikeForm(body string) bool {
	if strings.ContainsAny(body, "{\n") {
		return false
	}
	s := strings.TrimSpace(body)
	if s == "" || strings.IndexByte(s, '&') < 0 {
		return false
	}
	for _, pair := range strings.Split(s, "&") {
		pair = strings.TrimSpace(pair)
		if pair == "" || strings.IndexByte(pair, '=') < 0 {
			return false
		}
	}
	return true
}

func hasHeader(hs []Header, name string) bool {
	for _, h := range hs {
		if strings.EqualFold(h.Name, name) {
			return true
		}
	}
	return false
}

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
	if p.Data != "" && !hasHeader(p.Headers, "Content-Type") {
		b.WriteString("Content-Type: application/json\n")
	}
	if p.Form && !hasHeader(p.Headers, "Content-Type") {
		b.WriteString("Content-Type: multipart/form-data\n")
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

func hasHeader(hs []Header, name string) bool {
	for _, h := range hs {
		if strings.EqualFold(h.Name, name) {
			return true
		}
	}
	return false
}

package curl

import "strings"

// IsCurlCommand reports whether text looks like a cURL command line, so a paste
// can be auto-converted into an .http request. It accepts the shapes the
// DevTools "Copy as cURL" produces: "curl", "curl.exe" and a multi-line command
// continued with a backslash, optionally behind a "$ " or "> " shell prompt.
func IsCurlCommand(text string) bool {
	t := strings.TrimSpace(text)
	if t == "" {
		return false
	}
	t = strings.TrimLeft(t, "$> ")
	low := strings.ToLower(t)
	if strings.HasPrefix(low, "curl.exe") || strings.HasPrefix(low, "curl ") || strings.EqualFold(strings.TrimSpace(t), "curl") {
		return true
	}
	return strings.HasPrefix(low, "curl\\") ||
		strings.HasPrefix(low, "curl\n") ||
		strings.HasPrefix(low, "curl\r\n")
}

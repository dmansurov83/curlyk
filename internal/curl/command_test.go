package curl

import "testing"

func TestIsCurlCommand(t *testing.T) {
	cases := []struct {
		name string
		text string
		want bool
	}{
		{"curl", "curl https://example.com", true},
		{"curl.exe", `curl.exe "https://example.com"`, true},
		{"bare curl", "curl", true},
		{"leading prompt", "$ curl https://example.com", true},
		{"continuation", "curl \\\n  -H 'A: b' \\\n  https://example.com", true},
		{"multiline", "curl\nhttps://example.com", true},
		{"empty", "", false},
		{"plain text", "GET https://example.com", false},
		{"curl inside a word", "notcurl https://example.com", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := IsCurlCommand(tc.text); got != tc.want {
				t.Fatalf("IsCurlCommand(%q) = %v, want %v", tc.text, got, tc.want)
			}
		})
	}
}

package curl

import (
	"strings"
	"testing"
)

// TestImportMultipleDataURLEncode verifies that several --data-urlencode flags
// are joined with '&' and URL-encoded (the OAuth2 token-request shape).
func TestImportMultipleDataURLEncode(t *testing.T) {
	cmd := `curl -X POST 'https://id-test.example.com/connect/token' \
  -H 'authorization: Basic QXBpQ2xpZW50' \
  -H 'content-type: application/x-www-form-urlencoded' \
  --data-urlencode 'grant_type=application' \
  --data-urlencode 'scope=openid offline_access ' \
  --data-urlencode 'app=Users' \
  --data-urlencode 'app_auth_type=Impersonation' \
  --data-urlencode 'token=eyJhbGciOiJSUzI1NiIsInR5cCI6IkpXVCJ9.abc-DEF_ghi' \
  --data-urlencode 'account_Id=2' \
  --data-urlencode 'user_Id=1329725'`
	out, err := ImportLine(cmd)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("out:\n%s", out)
	// The explicit cURL header is preserved (content-type stays lowercase, as
	// given on the command line).
	if !containsLower(out, "content-type: application/x-www-form-urlencoded") {
		t.Errorf("missing form content-type:\n%s", out)
	}
	// Fields must be joined with '&' — the original bug glued them together.
	if !contains(out, "grant_type=application&scope=openid%20offline_access%20&app=Users&app_auth_type=Impersonation") {
		t.Errorf("fields not joined/encoded correctly:\n%s", out)
	}
	if !contains(out, "&token=eyJhbGciOiJSUzI1NiIsInR5cCI6IkpXVCJ9.abc-DEF_ghi&account_Id=2&user_Id=1329725") {
		t.Errorf("token/account fields missing or mangled:\n%s", out)
	}
	// Space must be %20, not '+'.
	if strings.Contains(out, "openid+offline") {
		t.Errorf("space should be %%%%20, got '+':\n%s", out)
	}
}

// TestImportDataURLEncodeNoEq verifies a --data-urlencode with no '=' encodes
// the whole value.
func TestImportDataURLEncodeNoEq(t *testing.T) {
	out, err := ImportLine(`curl -X POST http://x --data-urlencode 'hello world'`)
	if err != nil {
		t.Fatal(err)
	}
	if !contains(out, "hello%20world") {
		t.Errorf("expected encoded value, got:\n%s", out)
	}
}

// TestImportDataURLEncodeFileRef verifies a name=@file reference is left
// untouched.
func TestImportDataURLEncodeFileRef(t *testing.T) {
	out, err := ImportLine(`curl -X POST http://x --data-urlencode 'payload=@data.json'`)
	if err != nil {
		t.Fatal(err)
	}
	if !contains(out, "payload=@data.json") {
		t.Errorf("file reference mangled:\n%s", out)
	}
}

// TestImportMultipleData keeps prior behaviour for plain -d chunks: joined
// with '&' like curl does.
func TestImportMultipleData(t *testing.T) {
	out, err := ImportLine(`curl -X POST http://x -d 'a=1' -d 'b=2'`)
	if err != nil {
		t.Fatal(err)
	}
	if !contains(out, "a=1&b=2") {
		t.Errorf("multiple -d chunks should be joined with '&':\n%s", out)
	}
}

// TestImportSingleData verifies a single -d chunk is used verbatim (no
// surrounding '&').
func TestImportSingleData(t *testing.T) {
	out, err := ImportLine(`curl -X POST http://x -d '{"a":1}'`)
	if err != nil {
		t.Fatal(err)
	}
	if !contains(out, "{\"a\":1}") {
		t.Errorf("single -d chunk mangled:\n%s", out)
	}
	if strings.Contains(out, "&{") {
		t.Errorf("unexpected '&' around single chunk:\n%s", out)
	}
}

func containsLower(s, sub string) bool {
	return contains(strings.ToLower(s), strings.ToLower(sub))
}

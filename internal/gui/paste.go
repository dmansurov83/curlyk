package gui

import (
	"strings"

	"github.com/user/curlyk/internal/curl"
	"github.com/user/curlyk/internal/i18n"
)

// convertPaste rewrites a cURL command from the clipboard into an .http block.
// It is the editor's paste hook: ordinary text is returned unchanged with
// handled=false, while a curl command that cannot be parsed aborts the paste and
// reports the error on the status line.
func (u *ui) convertPaste(text string) (string, bool, error) {
	if !curl.IsCurlCommand(text) {
		return text, false, nil
	}
	block, err := curl.ImportLine(strings.TrimSpace(text))
	if err != nil {
		u.setStatus(i18n.T("err.convertCurl", err.Error()))
		return "", true, err
	}
	u.setStatus(i18n.T("status.curlConverted"))
	return block, true, nil
}

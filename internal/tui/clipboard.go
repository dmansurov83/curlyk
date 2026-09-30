package tui

import (
	"fmt"
	"strings"

	"github.com/atotto/clipboard"
	"github.com/user/curlyk/httptool/internal/curl"
	"github.com/user/curlyk/httptool/internal/httpfile"
)

// hasSelection reports whether there is a non-empty active selection.
func (m *model) hasSelection() bool {
	if !m.selActive {
		return false
	}
	aRow, aCol := m.selAnchorRow, m.selAnchorCol
	text := m.ed.SelectedText(aRow, aCol, m.ed.curRow, m.ed.curCol)
	return text != ""
}

// copySelection copies the selected text to the OS clipboard.
func (m *model) copySelection() {
	if !m.selActive {
		// no active selection: nothing to copy
		m.status = "Нет выделения для копирования"
		return
	}
	aRow, aCol := m.selAnchorRow, m.selAnchorCol
	text := m.ed.SelectedText(aRow, aCol, m.ed.curRow, m.ed.curCol)
	if text == "" {
		return
	}
	if err := writeClipboardSafe(text); err != nil {
		m.status = "Не удалось скопировать: " + err.Error()
		return
	}
	m.status = "Скопировано (" + fmt.Sprintf("%d", len([]rune(text))) + " симв.)"
	m.selActive = false
}

// writeClipboardSafe strips NUL bytes (which panic Windows clipboard) before write.
func writeClipboardSafe(s string) error {
	if strings.IndexByte(s, 0) >= 0 {
		s = strings.ReplaceAll(s, "\x00", "")
	}
	return clipboard.WriteAll(s)
}

// copyAsCurl constructs a cURL command from the request under the cursor and
// copies it to the clipboard (Ctrl+K).
func (m *model) copyAsCurl() {
	reqs := httpfile.ParseFile(m.ed.Text())
	req := httpfile.GetRequestAtLine(reqs, m.ed.curRow+1)
	if req == nil {
		m.status = "Нет запроса под курсором"
		return
	}
	var headers []string
	for _, h := range req.Headers {
		headers = append(headers, h.Name+": "+h.Value)
	}
	cmd := curl.ExportRequest(req.Method, req.URL, headers, req.Body)
	if err := writeClipboardSafe(cmd); err != nil {
		m.status = "Не удалось скопировать cURL: " + err.Error()
		return
	}
	m.status = "cURL скопирован"
}

// copyRespSelection copies the selected response text to the clipboard.
func (m *model) copyRespSelection() {
	text := m.respSelectedText()
	if text == "" {
		m.status = "Нет выделения в ответе для копирования"
		return
	}
	if err := writeClipboardSafe(text); err != nil {
		m.status = "Не удалось копировать: " + err.Error()
		return
	}
	m.status = fmt.Sprintf("Скопировано из ответа (%d симв.)", len([]rune(text)))
	m.respSelActive = false
}

// copyAllResponse copies the last response (body) to the clipboard. Used by the
// copy button at the top of the response pane.
func (m *model) copyAllResponse() {
	if m.response == "" {
		m.status = "Нет ответа для копирования"
		return
	}
	text := m.response
	if len(m.lastBody) > 0 {
		text = string(m.lastBody)
	}
	if err := writeClipboardSafe(text); err != nil {
		m.status = "Не удалось копировать: " + err.Error()
		return
	}
	m.status = fmt.Sprintf("Ответ скопирован (%d симв.)", len([]rune(text)))
	m.respSelActive = false
}

// cutSelection copies and deletes the selected text.
func (m *model) cutSelection() {
	if !m.selActive {
		return
	}
	aRow, aCol := m.selAnchorRow, m.selAnchorCol
	sel := m.ed.DeleteRange(aRow, aCol, m.ed.curRow, m.ed.curCol)
	if sel == "" {
		return
	}
	if err := clipboard.WriteAll(sel); err != nil {
		m.status = "Не удалось вырезать: " + err.Error()
		return
	}
	m.selActive = false
	m.status = "Вырезано"
}

// pasteClipboard inserts clipboard contents at the cursor.
func (m *model) pasteClipboard() {
	text, err := clipboard.ReadAll()
	if err != nil || text == "" {
		m.status = "Буфер обмена пуст"
		return
	}
	m.pasteOrConvert(text)
}

// pasteOrConvert inserts text at the cursor; if it looks like a cURL command it
// is auto-converted into an .http request (issue #5).
func (m *model) pasteOrConvert(text string) {
	// Pasting replaces any active selection.
	if m.selActive && m.hasSelection() {
		m.ed.DeleteRange(m.selAnchorRow, m.selAnchorCol, m.ed.curRow, m.ed.curCol)
		m.selActive = false
		m.selAnchorRow, m.selAnchorCol = m.ed.curRow, m.ed.curCol
	}

	// Strip NUL bytes: Windows clipboard often appends them and they crash
	// clipboard.WriteAll later (syscall NUL panic).
	if strings.IndexByte(text, 0) >= 0 {
		text = strings.ReplaceAll(text, "\x00", "")
	}

	// If the pasted text looks like a cURL command, convert it into an .http
	// request automatically (issue #5).
	if isCurlCommand(text) {
		block, err := curl.ImportLine(strings.TrimSpace(text))
		if err != nil {
			m.status = "Не удалось конвертировать cURL: " + err.Error()
			return
		}
		m.insertText(block)
		m.status = "cURL конвертирован в запрос"
		return
	}

	// insert across lines
	lines := strings.Split(text, "\n")
	if len(lines) == 1 {
		m.ed.InsertString(lines[0])
		m.ed.clampCol()
		m.ed.EnsureVisible()
	} else {
		// multi-line paste: insert on current line, then new lines
		first := lines[0]
		m.ed.InsertString(first)
		// insert remaining lines
		for li := 1; li < len(lines); li++ {
			m.ed.Enter()
			m.ed.InsertString(lines[li])
		}
		m.ed.EnsureVisible()
	}
	m.status = "Вставлено"
}

// insertText inserts a block of text at the current cursor (newline-separated).
func (m *model) insertText(text string) {
	lines := strings.Split(text, "\n")
	if len(lines) == 1 {
		m.ed.InsertString(lines[0])
		m.ed.clampCol()
		m.ed.EnsureVisible()
		return
	}
	first := lines[0]
	m.ed.InsertString(first)
	for li := 1; li < len(lines); li++ {
		m.ed.Enter()
		m.ed.InsertString(lines[li])
	}
	m.ed.EnsureVisible()
}

// isCurlCommand reports whether text looks like a cURL command line.
func isCurlCommand(text string) bool {
	t := strings.TrimSpace(text)
	if t == "" {
		return false
	}
	// strip a leading shell prompt "$ " or "> "
	t = strings.TrimLeft(t, "$> ")
	low := strings.ToLower(t)
	// accept "curl", "curl ", "curl.exe " (common from DevTools copy-as-curl)
	if strings.HasPrefix(low, "curl.exe") || strings.HasPrefix(low, "curl ") || strings.EqualFold(strings.TrimSpace(t), "curl") {
		return true
	}
	// a multi-line curl with line continuations begins with "curl"
	return strings.HasPrefix(low, "curl\\") ||
		strings.HasPrefix(low, "curl\n") ||
		strings.HasPrefix(low, "curl\r\n")
}
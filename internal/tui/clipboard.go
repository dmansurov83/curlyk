package tui

import (
	"strings"

	"github.com/atotto/clipboard"
	"github.com/user/curlyk/internal/curl"
	"github.com/user/curlyk/internal/httpfile"
	"github.com/user/curlyk/internal/i18n"
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
		m.status = i18n.T("err.noSelection")
		return
	}
	aRow, aCol := m.selAnchorRow, m.selAnchorCol
	text := m.ed.SelectedText(aRow, aCol, m.ed.curRow, m.ed.curCol)
	if text == "" {
		return
	}
	if err := writeClipboardSafe(text); err != nil {
		m.status = i18n.T("err.copy", err.Error())
		return
	}
	m.status = i18n.T("status.copiedChars", len([]rune(text)))
	m.selActive = false
}

// clipboardWriter is the OS clipboard write hook; tests override it.
var clipboardWriter = clipboard.WriteAll

// writeClipboardSafe strips NUL bytes (which panic Windows clipboard) before write.
func writeClipboardSafe(s string) error {
	if strings.IndexByte(s, 0) >= 0 {
		s = strings.ReplaceAll(s, "\x00", "")
	}
	return clipboardWriter(s)
}

// copyAsCurl constructs a cURL command from the request under the cursor and
// copies it to the clipboard (Ctrl+K). {{name}} / {{$fn}} placeholders are
// resolved exactly like when running the request, so the copied command
// contains concrete values (falls back to the active profile's @var).
func (m *model) copyAsCurl() {
	reqs := httpfile.ParseFile(m.ed.Text())
	req := httpfile.GetRequestAtLine(reqs, m.ed.curRow+1)
	if req == nil {
		m.status = i18n.T("err.noRequest")
		return
	}
	vars := req.Vars
	if pv := m.activeProfileVars(); len(pv) > 0 {
		vars = httpfile.MergeVars(pv, vars) // local (vars) overrides profile (pv)
	}
	sub, subErr := httpfile.Substitute(req.URL, vars)
	if subErr != nil {
		m.status = i18n.T("err.variable", subErr.(*httpfile.SubstitutionError).First())
		return
	}
	req.URL = sub
	for i := range req.Headers {
		if hv, herr := httpfile.Substitute(req.Headers[i].Value, vars); herr != nil {
			m.status = i18n.T("err.variable", herr.(*httpfile.SubstitutionError).First())
			return
		} else {
			req.Headers[i].Value = hv
		}
	}
	if req.Body != "" {
		if bv, berr := httpfile.Substitute(req.Body, vars); berr != nil {
			m.status = i18n.T("err.variable", berr.(*httpfile.SubstitutionError).First())
			return
		} else {
			req.Body = bv
		}
	}
	var headers []string
	for _, h := range req.Headers {
		headers = append(headers, h.Name+": "+h.Value)
	}
	cmd := curl.ExportRequest(req.Method, req.URL, headers, req.Body)
	if err := writeClipboardSafe(cmd); err != nil {
		m.status = i18n.T("err.copyCurl", err.Error())
		return
	}
	m.status = i18n.T("status.curlCopied")
}

// copyRespSelection copies the selected response text to the clipboard.
func (m *model) copyRespSelection() {
	text := m.respSelectedText()
	if text == "" {
		m.status = i18n.T("err.noRespSelection")
		return
	}
	if err := writeClipboardSafe(text); err != nil {
		m.status = i18n.T("err.copyGeneric", err.Error())
		return
	}
	m.status = i18n.T("status.copiedResp", len([]rune(text)))
	m.respSelActive = false
}

// copyAllResponse copies the last response (body) to the clipboard. Used by the
// copy button at the top of the response pane.
func (m *model) copyAllResponse() {
	if m.response == "" {
		m.status = i18n.T("err.noResponse")
		return
	}
	text := m.response
	if len(m.lastBody) > 0 {
		text = string(m.lastBody)
	}
	if err := writeClipboardSafe(text); err != nil {
		m.status = i18n.T("err.copyGeneric", err.Error())
		return
	}
	m.status = i18n.T("status.respCopied", len([]rune(text)))
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
	m.markDirty()
	if err := clipboard.WriteAll(sel); err != nil {
		m.status = i18n.T("err.cut", err.Error())
		return
	}
	m.selActive = false
	m.status = i18n.T("status.cut")
}

// deleteLine removes the line under the cursor. If a selection is active, it is
// cleared first (the action targets the current line, not the selection).
func (m *model) deleteLine() {
	m.forceCloseEditBatch()
	m.ed.BeginUndo()
	defer m.ed.EndUndo()
	m.selActive = false
	m.selAnchorRow, m.selAnchorCol = m.ed.curRow, m.ed.curCol
	m.ed.DeleteLine()
	m.markDirty()
	m.status = i18n.T("status.lineDeleted")
}

// pasteClipboard inserts clipboard contents at the cursor.
func (m *model) pasteClipboard() {
	text, err := clipboard.ReadAll()
	if err != nil || text == "" {
		m.status = i18n.T("err.clipboardEmpty")
		return
	}
	m.pasteOrConvert(text)
}

// pasteOrConvert inserts text at the cursor; if it looks like a cURL command it
// is auto-converted into an .http request (issue #5).
func (m *model) pasteOrConvert(text string) {
	m.ed.BeginUndo()
	defer m.ed.EndUndo()
	m.markDirty()
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
			m.status = i18n.T("err.convertCurl", err.Error())
			return
		}
		// Remember where the block will start so we can put the cursor back on
		// the request line (insertText leaves the cursor at the block's end).
		startRow, startCol := m.ed.curRow, m.ed.curCol
		m.insertText(block)
		// Move the cursor to the request line (the first line of the block).
		if m.ed.curRow >= startRow {
			m.ed.curRow = startRow
			m.ed.curCol = 0
			if startCol == 0 {
				m.ed.curCol = 0
			} else {
				// block was appended mid-line; put cursor at block start
				m.ed.curCol = startCol
			}
		}
		m.ed.clampCol()
		m.ed.EnsureVisible()
		m.status = i18n.T("status.curlConverted")
		return
	}

	// insert across lines
	text = normalizeNewlines(text)
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
	m.status = i18n.T("status.pasted")
}

// insertText inserts a block of text at the current cursor (newline-separated).
func (m *model) insertText(text string) {
	text = normalizeNewlines(text)
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

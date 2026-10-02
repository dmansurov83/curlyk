package tui

import (
	"net/url"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/user/curlyk/internal/httpfile"
	"github.com/user/curlyk/internal/i18n"
)

// maxFormRows is the maximum number of form-field rows the editor popup lists
// before truncating with an ellipsis row.
const maxFormRows = 8

// maxFormFieldWidth caps each rendered field name/value so a long JWT or scope
// value cannot push the key and value cells off the pane.
const maxFormFieldWidth = 60

// formField is one key=value pair of an x-www-form-urlencoded body.
type formField struct {
	key   string
	value string
}

// formEditor is the "key=value body" popup opened from the actions menu. It
// shows the current form fields of the request under the cursor as editable
// key/value rows and writes the encoded body back into the editor buffer. The
// last row is always editable; moving past it creates a new empty row.
type formEditor struct {
	fields []formField
	// sel is the selected row index (0-based).
	sel int
	// field is the active cell of the selected row: 0 = key, 1 = value.
	field int
	// original is the request body captured when the popup opened.
	original  string
	bodyStart int
	bodyEnd   int
	// anchorLine is the 1-based request line the popup belongs to.
	anchorLine int
}

// parseFormBody splits a raw x-www-form-urlencoded body string into key/value
// pairs. It is the single normalized source used to populate the editor on
// open (and, through encodeForm, to rebuild the body on commit).
func parseFormBody(body string) []formField {
	body = strings.TrimSpace(body)
	if body == "" {
		return nil
	}
	var out []formField
	for _, pair := range strings.Split(body, "&") {
		if pair == "" {
			continue
		}
		k, v, _ := strings.Cut(pair, "=")
		dk, err := url.QueryUnescape(k)
		if err != nil {
			dk = k
		}
		dv, err := url.QueryUnescape(v)
		if err != nil {
			dv = v
		}
		out = append(out, formField{key: dk, value: dv})
	}
	return out
}

// encodeForm builds an x-www-form-urlencoded body from fields. Rows with an
// empty key are dropped; a row with a key but empty value yields "key=".
func encodeForm(fields []formField) string {
	var pairs []string
	for _, f := range fields {
		if strings.TrimSpace(f.key) == "" {
			continue
		}
		pairs = append(pairs, url.QueryEscape(f.key)+"="+url.QueryEscape(f.value))
	}
	return strings.Join(pairs, "&")
}

// beginFormEditor opens the key/value body editor for the request under the
// cursor. It reports an error in the status bar when the cursor is not on a
// request line.
func (m *model) beginFormEditor() {
	m.ed.sanitize()
	reqs := httpfile.ParseFile(m.ed.Text())
	req := httpfile.GetRequestAtLine(reqs, m.ed.curRow+1)
	if req == nil {
		m.status = i18n.T("err.noRequestCursor")
		return
	}
	fields := parseFormBody(req.Body)
	if len(fields) == 0 {
		fields = []formField{{}}
	}
	m.form = &formEditor{
		fields:     fields,
		sel:        0,
		field:      0,
		original:   req.Body,
		bodyStart:  req.BodyStart,
		bodyEnd:    req.BodyEnd,
		anchorLine: req.Line,
	}
}

// handleFormKey processes keys while the form editor popup is open. It returns
// true and a j/run command is not used; all keys are consumed. The caller is
// expected to handle Enter/commit separately via commitForm.
func (m *model) handleFormKey(msg tea.KeyMsg) tea.Cmd {
	f := m.form
	if f == nil {
		return nil
	}
	switch msg.String() {
	case "esc", "ctrl+c":
		m.form = nil
	case "enter":
		// Enter on a blank final row finishes editing; on a filled row it moves
		// to the next row (creating one when needed).
		if f.isLastRowBlank() {
			m.commitForm()
		} else {
			f.moveDown()
		}
	case "tab", "shift+tab":
		f.field = 1 - f.field
	case "down":
		f.moveDown()
	case "up":
		if f.sel > 0 {
			f.sel--
			f.field = 0
		}
	case "backspace", "delete":
		f.deleteCell()
	case "ctrl+d":
		f.deleteRow()
	default:
		var printable []rune
		for _, r := range msg.Runes {
			if r >= 0x20 && r != 0x7f {
				printable = append(printable, r)
			}
		}
		if len(printable) > 0 {
			f.typeInto(printable)
		}
	}
	return nil
}

// isLastRowBlank reports whether the final row (the add-row the cursor rests
// on) is empty, which Enter uses as the "finish editing" signal.
func (f *formEditor) isLastRowBlank() bool {
	if len(f.fields) == 0 {
		return true
	}
	last := f.fields[len(f.fields)-1]
	return strings.TrimSpace(last.key) == "" && strings.TrimSpace(last.value) == ""
}

// typeInto appends printable runes to the active cell of the selected row.
func (f *formEditor) typeInto(rs []rune) {
	if f.sel < 0 || f.sel >= len(f.fields) {
		return
	}
	txt := string(rs)
	if f.field == 0 {
		f.fields[f.sel].key += txt
	} else {
		f.fields[f.sel].value += txt
	}
}

// deleteCell clears the active cell of the selected row.
func (f *formEditor) deleteCell() {
	if f.sel < 0 || f.sel >= len(f.fields) {
		return
	}
	if f.field == 0 {
		f.fields[f.sel].key = ""
	} else {
		f.fields[f.sel].value = ""
	}
}

// deleteRow removes the selected row, keeping at least one row present.
func (f *formEditor) deleteRow() {
	if f.sel < 0 || f.sel >= len(f.fields) {
		return
	}
	f.fields = append(f.fields[:f.sel], f.fields[f.sel+1:]...)
	if len(f.fields) == 0 {
		f.fields = []formField{{}}
	}
	if f.sel >= len(f.fields) {
		f.sel = len(f.fields) - 1
	}
}

// moveDown moves to the next row, appending a blank one when already at the
// bottom so the user always has an editable row to add a new field.
func (f *formEditor) moveDown() {
	if f.sel >= len(f.fields)-1 {
		f.fields = append(f.fields, formField{})
	}
	if f.sel < len(f.fields)-1 {
		f.sel++
	}
	f.field = 0
}

// commitForm writes the encoded body back into the editor buffer, replacing
// the request's body block, or creating one if the request had no body.
// Returns true when the buffer changed.
func (m *model) commitForm() bool {
	f := m.form
	if f == nil {
		return false
	}
	body := encodeForm(f.fields)
	if body == f.original {
		m.form = nil
		return false
	}
	m.ed.pushUndo()
	if f.bodyStart > 0 {
		// Replace the existing body line range [BodyStart, BodyEnd] (1-based,
		// inclusive) with the encoded body.
		start := f.bodyStart - 1
		end := f.bodyEnd
		block := []string{}
		if body != "" {
			block = []string{body}
		}
		m.ed.lines = replaceLines(m.ed.lines, start, end, block)
		m.markDirty()
	} else if body != "" {
		// No body yet: insert a body block after the request line and its
		// headers. If a blank separator already separates headers from the next
		// block, reuse it (insert only the body); otherwise add the separator.
		insertAt := m.formBodyInsertIndex(f.anchorLine - 1)
		block := []string{body}
		// Reuse an existing blank separator only when it is not the final
		// (trailing) buffer line, which would be a file-ending newline, not a
		// real header/body delimiter.
		if !(insertAt < len(m.ed.lines)-1 && strings.TrimSpace(m.ed.lines[insertAt]) == "") {
			block = append([]string{""}, block...)
		}
		m.ed.lines = insertLines(m.ed.lines, insertAt, block)
		m.markDirty()
	}
	m.ed.clampCol()
	m.form = nil
	return true
}

// formBodyInsertIndex returns the 0-based line index where a new body block
// should be inserted: right after the request line and its headers. The caller
// inserts ["", body], so the blank separator it carries becomes the header/body
// delimiter (or a no-op when a separator already sits on that line).
func (m *model) formBodyInsertIndex(anchorRow int) int {
	lines := m.ed.Lines()
	i := anchorRow + 1
	for i < len(lines) {
		if _, _, ok := looksLikeHeaderLine(lines[i]); ok {
			i++
			continue
		}
		break
	}
	return i
}

// looksLikeHeaderLine mirrors httpfile.splitHeaderLine for the editor insertion
// logic: a line "Name: value" where Name is a valid token.
func looksLikeHeaderLine(line string) (string, string, bool) {
	idx := strings.IndexRune(line, ':')
	if idx <= 0 {
		return "", "", false
	}
	name := line[:idx]
	for i := 0; i < len(name); i++ {
		c := name[i]
		if !(c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' ||
			c == '-' || c == '_' || c == '!' || c == '#' || c == '$' || c == '%' ||
			c == '&' || c == '\'' || c == '*' || c == '+' || c == '.' || c == '^' ||
			c == '`' || c == '|' || c == '~') {
			return "", "", false
		}
	}
	return name, strings.TrimLeft(line[idx+1:], " "), true
}

// replaceLines returns lines with the half-open [start, end) range (0-based)
// replaced by block.
func replaceLines(lines []string, start, end int, block []string) []string {
	if start < 0 {
		start = 0
	}
	if end > len(lines) {
		end = len(lines)
	}
	if start > end {
		start = end
	}
	out := make([]string, 0, len(lines)-(end-start)+len(block))
	out = append(out, lines[:start]...)
	out = append(out, block...)
	out = append(out, lines[end:]...)
	return out
}

// insertLines returns lines with block inserted at index pos (0-based).
func insertLines(lines []string, pos int, block []string) []string {
	if pos < 0 {
		pos = 0
	}
	if pos > len(lines) {
		pos = len(lines)
	}
	var out []string
	out = append(out, lines[:pos]...)
	out = append(out, block...)
	out = append(out, lines[pos:]...)
	return out
}

// formReserve returns the number of editor content rows the open form-editor
// popup consumes, or 0 when it is closed.
func (m *model) formReserve() int {
	if m.form == nil {
		return 0
	}
	return m.formHeight()
}

// formHeight returns the display height (in rows) of the open form-editor
// popup, matching exactly what formLines renders.
func (m *model) formHeight() int {
	f := m.form
	if f == nil {
		return 0
	}
	rows := 2 // title + hint
	if len(f.fields) > 0 {
		rows += len(f.fields)
	} else {
		rows++
	}
	if cap := m.height - 4; rows > cap {
		rows = cap
	}
	if rows < 0 {
		rows = 0
	}
	return rows
}

// formLines renders the form-editor popup block for the editor pane. contentW
// is the pane content width in cells. It renders at most maxRows rows.
func (m *model) formLines(contentW int) []string {
	f := m.form
	if f == nil {
		return nil
	}
	maxRows := m.formHeight()
	var out []string
	if maxRows > 0 {
		title := truncateWidth("  "+i18n.T("form.title"), contentW)
		out = append(out, menuSelStyle.Render(title))
	}
	if maxRows > 1 {
		hint := truncateWidth("  "+i18n.T("form.hint"), contentW)
		out = append(out, menuNormalStyle.Render(hint))
	}
	fieldRows := maxRows - 2 // rows remaining for fields
	if fieldRows <= 0 {
		return out
	}

	// Keep the selection visible in the (capped) scroll window.
	vis := len(f.fields)
	if vis > maxFormRows {
		vis = maxFormRows
	}
	if vis > fieldRows {
		vis = fieldRows
	}
	first := f.sel
	if first > vis-1 {
		first = vis - 1
	}
	if first < 0 {
		first = 0
	}
	last := first + vis
	if last > len(f.fields) {
		last = len(f.fields)
	}
	for i := first; i < last; i++ {
		text := formatFormRow(f.fields[i], i == f.sel, f.field)
		label := padToWidth("    "+text, contentW)
		if i == f.sel {
			out = append(out, menuSelStyle.Render(label))
		} else {
			out = append(out, menuNormalStyle.Render(label))
		}
	}
	return out
}

// formatFormRow renders one field row as "key=value". On the selected row a
// cursor marker "▐" is appended so the user sees which row is active.
func formatFormRow(ff formField, selected bool, field int) string {
	line := ff.key
	if ff.value != "" {
		line += "=" + ff.value
	} else if ff.key != "" {
		line += "="
	}
	if len([]rune(line)) > maxFormFieldWidth {
		line = truncateWidth(line, maxFormFieldWidth)
	}
	if selected {
		line += "▐"
	}
	return line
}

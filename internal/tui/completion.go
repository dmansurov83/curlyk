package tui

import (
	"net/url"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/user/curlyk/internal/completion"
	"github.com/user/curlyk/internal/httpfile"
	"github.com/user/curlyk/internal/i18n"
)

// maxCompletionRows is the maximum number of suggestion rows the completion
// popup lists before truncating with an ellipsis row.
const maxCompletionRows = 8

// completionMenu is the autocomplete popup opened by Ctrl+Space (or when "{{"
// is typed). It holds the suggestion list computed for the cursor context at
// open time and refreshed as the user keeps typing.
type completionMenu struct {
	// suggestions is the candidate list for the current cursor context.
	suggestions []completion.Suggestion
	// sel is the selected index within suggestions.
	sel int
}

// completeReserve returns the number of editor content rows the open completion
// popup consumes, or 0 when it is closed.
func (m *model) completeReserve() int {
	if m.complete == nil {
		return 0
	}
	return m.completeHeight()
}

// completeHeight returns the display height (in rows) of the open completion
// popup, matching exactly what completeLines renders: up to maxCompletionRows
// suggestions (an ellipsis row when truncated). The popup only ever opens with
// at least one suggestion. Capped so a tiny window cannot overflow the frame.
func (m *model) completeHeight() int {
	if m.complete == nil {
		return 0
	}
	n := len(m.complete.suggestions)
	var rows int
	if n > maxCompletionRows {
		rows = maxCompletionRows + 1 // suggestions + ellipsis
	} else {
		rows = n
	}
	cap := m.height - 4
	if rows > cap {
		rows = cap
	}
	if rows < 0 {
		rows = 0
	}
	return rows
}

// beginCompletion opens the completion popup for the cursor context. It is a
// no-op when the engine offers no candidates.
func (m *model) beginCompletion() {
	suggs := m.suggestForCursor()
	if len(suggs) == 0 {
		m.complete = nil
		return
	}
	m.complete = &completionMenu{suggestions: suggs, sel: 0}
	m.fitCompletionScroll()
}

// fitCompletionScroll scrolls the editor up so the open completion popup fits
// inside the visible window: it is drawn inline under the cursor line and
// consumes popupRows source rows, so if the cursor sits too close to the bottom
// edge the popup would be pushed off the pane (and the reserved rows would hide
// text without showing anything). Mirrors beginActionMenu.
func (m *model) fitCompletionScroll() {
	popH := m.completeHeight()
	if popH <= 0 {
		return
	}
	effVis := m.effEditorVisible()
	if row := m.ed.curRow; row >= m.ed.scroll+effVis {
		m.ed.scroll = row + 1 - effVis
		if m.ed.scroll < 0 {
			m.ed.scroll = 0
		}
	}
}

// afterEditMaybeCompletion is called after every printable edit. When the
// completion popup is already open it re-filters against the new text;
// otherwise it triggers the popup for the current context: typing "{{" opens
// the variable list, and typing at the start of a request line (a method
// prefix) or a URL scheme opens the method/scheme-list. So GET/POST/http etc.
// are suggested as the user types, not only on an explicit Ctrl+Space.
func (m *model) afterEditMaybeCompletion() {
	m.ed.clampCol()
	if m.complete != nil {
		m.refreshCompletion()
		return
	}
	line := m.ed.Lines()[m.ed.curRow]
	runes := []rune(line)
	if len(runes) >= 2 && m.ed.curCol >= 2 &&
		runes[m.ed.curCol-2] == '{' && runes[m.ed.curCol-1] == '{' {
		m.beginCompletion()
		return
	}
	suggs := completion.Suggest(line, m.ed.curCol, m.buildCompletionContext())
	if len(suggs) > 0 {
		m.complete = &completionMenu{suggestions: suggs, sel: 0}
		m.fitCompletionScroll()
	}
}

// suggestForCursor computes the suggestions for the current cursor context.
func (m *model) suggestForCursor() []completion.Suggestion {
	m.ed.sanitize()
	if m.ed.curRow < 0 || m.ed.curRow >= len(m.ed.Lines()) {
		return nil
	}
	line := m.ed.Lines()[m.ed.curRow]
	return completion.Suggest(line, m.ed.curCol, m.buildCompletionContext())
}

// refreshCompletion recomputes the suggestion list for the current cursor.
// Called after the user edits while the popup is open. When no candidates
// remain the popup is hidden entirely (instead of showing "no matches"), so a
// stale popup never lingers.
func (m *model) refreshCompletion() {
	if m.complete == nil {
		return
	}
	line := m.ed.Lines()[m.ed.curRow]
	m.complete.suggestions = completion.Suggest(line, m.ed.curCol, m.buildCompletionContext())
	if len(m.complete.suggestions) == 0 {
		m.complete = nil
		return
	}
	if m.complete.sel >= len(m.complete.suggestions) {
		m.complete.sel = 0
	}
}

// buildCompletionContext collects the candidate words for autocomplete from the
// current buffer, the active profile and the built-in functions.
func (m *model) buildCompletionContext() completion.Context {
	reqs := httpfile.ParseFile(m.ed.Text())
	ctx := completion.Context{}
	seenHost := map[string]bool{}

	addHost := func(h string) {
		if h == "" {
			return
		}
		host := hostOf(h)
		if host == "" || seenHost[host] {
			return
		}
		seenHost[host] = true
		ctx.Hosts = append(ctx.Hosts, host)
	}

	// File-level @var map (from the last parsed request).
	var fileVars map[string]string
	for _, r := range reqs {
		for _, h := range r.Headers {
			ctx.Headers = appendUnique(ctx.Headers, h.Name)
		}
		addHost(r.URL)
		if r.Vars != nil {
			fileVars = r.Vars
		}
	}
	if fileVars != nil {
		for name, val := range fileVars {
			ctx.Vars = appendUnique(ctx.Vars, name)
			addHost(val)
		}
	}

	// Active profile vars and their hosts.
	if pv := m.activeProfileVars(); len(pv) > 0 {
		for name, val := range pv {
			ctx.Vars = appendUnique(ctx.Vars, name)
			addHost(val)
		}
	}

	ctx.Methods = httpMethods()
	for _, fn := range builtinFuncNames {
		ctx.Vars = appendUnique(ctx.Vars, fn)
	}
	return ctx
}

// hostOf extracts a bare hostname from a URL string or a raw host value.
func hostOf(s string) string {
	u, err := url.Parse(s)
	if err != nil {
		return ""
	}
	return u.Hostname()
}

// httpMethods returns the known HTTP method names for completion.
func httpMethods() []string {
	return []string{"GET", "POST", "PUT", "DELETE", "PATCH", "HEAD", "OPTIONS", "CONNECT", "TRACE"}
}

// builtinFuncNames are the {{$fn}} built-in variable functions offered for
// variable completion.
var builtinFuncNames = []string{
	"$timestamp",
	"$isoTimestamp",
	"$random.uuid",
	"$random.int",
	"$guid",
}

// appendUnique appends s to list when not already present.
func appendUnique(list []string, s string) []string {
	for _, e := range list {
		if e == s {
			return list
		}
	}
	return append(list, s)
}

// handleCompletionKey processes keys while the completion popup is open. It
// returns the tea.Cmd to run and whether the key was consumed by the popup.
// Only arrows / Enter / Tab / Esc / the toggle key are consumed; any other key
// (e.g. a printable char) is reported as not handled so the caller routes it to
// normal editing and then refreshes the popup.
func (m *model) handleCompletionKey(msg tea.KeyMsg) (tea.Cmd, bool) {
	c := m.complete
	if c == nil {
		return nil, false
	}
	switch msg.String() {
	case "esc", "ctrl+c":
		m.complete = nil
		return nil, true
	case "enter", "tab":
		// Accept and apply the selected suggestion. When nothing is applicable
		// (no suggestions, or the selection is invalid) we close the popup and
		// report the key as NOT handled so Enter/Tab falls through to the normal
		// edit/run path instead of being silently swallowed.
		if m.acceptCompletion() {
			return nil, true
		}
		m.complete = nil
		return nil, false
	case "up":
		if c.sel > 0 {
			c.sel--
		}
		return nil, true
	case "down":
		if c.sel < len(c.suggestions)-1 {
			c.sel++
		}
		return nil, true
	case "ctrl+space", "ctrl+@":
		// re-open/recompute in place
		m.beginCompletion()
		return nil, true
	}
	return nil, false
}

// acceptCompletion replaces the suggestion's range with its snippet and closes
// the popup. For variable suggestions the closing "}}" is appended when not
// already present. The edit is grouped into one undo step.
// acceptCompletion replaces the suggestion's range with its snippet and closes
// the popup. For variable suggestions the closing "}}" is appended when not
// already present. The edit is grouped into one undo step. It returns true when
// a suggestion was actually applied, false otherwise (no applicable candidate).
func (m *model) acceptCompletion() bool {
	c := m.complete
	if c == nil {
		return false
	}
	if c.sel < 0 || c.sel >= len(c.suggestions) {
		m.complete = nil
		return false
	}
	sug := c.suggestions[c.sel]

	row := m.ed.curRow
	runes := []rune(m.ed.Lines()[row])
	start := sug.Start
	end := sug.End
	if start < 0 {
		start = 0
	}
	if end < start {
		start, end = end, start
	}
	if start > len(runes) {
		start = len(runes)
	}
	if end > len(runes) {
		end = len(runes)
	}

	snippet := sug.Snippet
	if sug.Var {
		// Close the "{{}}" pair unless a "}}" already follows the snippet.
		if !strings.HasPrefix(string(runes[end:]), "}}") {
			snippet += "}}"
		}
	}

	m.forceCloseEditBatch()
	m.ed.pushUndo()
	m.markDirty()

	m.ed.lines[row] = string(runes[:start]) + snippet + string(runes[end:])
	// Place the cursor after the inserted snippet.
	m.ed.curRow = row
	m.ed.curCol = start + len([]rune(snippet))
	m.complete = nil
	m.ed.clampCol()
	m.ed.EnsureVisible()
	m.status = i18n.T("completion.accepted")
	return true
}

// completeLines renders the completion popup block (title row + suggestions)
// for the editor pane. contentW is the pane content width in cells. indent is
// the number of leading cells the popup is moved to the right (the cursor's
// visible column), so it opens under the cursor instead of at the pane edge.
// The indent is clamped so the popup never overflows the pane's right edge —
// otherwise on a far-right cursor the popup would be pushed out of view and the
// reserved source rows would "swallow" text without showing anything.
func (m *model) completeLines(contentW, indent int) []string {
	c := m.complete
	if c == nil {
		return nil
	}
	if indent < 0 {
		indent = 0
	}
	// Reserve at least a couple of cells for the popup itself.
	maxIndent := contentW - 6
	if contentW-6 < 3 {
		// tiny pane: no room to indent at all, clamp to left edge
		maxIndent = 0
	}
	if indent > maxIndent {
		indent = maxIndent
	}
	pad := strings.Repeat(" ", indent)
	bodyW := contentW - indent
	if bodyW < 3 {
		bodyW = 3
	}
	var out []string
	shown := 0
	for i := 0; i < len(c.suggestions) && shown < maxCompletionRows; i++ {
		sug := c.suggestions[i]
		label := padToWidth(pad+truncateWidth(sug.Label, bodyW), contentW)
		if i == c.sel {
			label = menuSelStyle.Render(label)
		} else {
			label = menuNormalStyle.Render(label)
		}
		out = append(out, label)
		shown++
	}
	if shown < len(c.suggestions) {
		out = append(out, menuNormalStyle.Render(padToWidth(pad+truncateWidth("…", bodyW), contentW)))
	}
	return out
}

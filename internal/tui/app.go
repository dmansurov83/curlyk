package tui

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"
	"github.com/user/curlyk/internal/httpfile"
	"github.com/user/curlyk/internal/i18n"
	"github.com/user/curlyk/internal/runner"
	"github.com/user/curlyk/internal/settings"
)

type pane int

const (
	paneEdit pane = iota
	paneResp
	paneFiles
)

type runState int

const (
	stateIdle runState = iota
	stateRunning
)

// model is the top-level bubbletea model.
type model struct {
	ed         *editor
	width      int
	height     int
	active     pane
	response   string
	respScroll int
	status     string
	state      runState
	filePath   string
	lastBody   []byte
	// selection (mouse drag) anchor and active flag
	selAnchorRow, selAnchorCol int
	selActive                  bool
	// last-left-click tracking for double-click word selection
	lastClickRow, lastClickCol int
	lastClickTime              time.Time
	// last Esc press time (double-Esc quits)
	lastEscTime time.Time
	// response-pane text selection
	respSelActive                      bool
	respSelAnchorRow, respSelAnchorCol int
	respSelCurRow, respSelCurCol       int
	// mouse session: true when a drag happened between press and release
	mouseDragged bool
	// true when the most recent press was a double-click (keeps the word selection)
	lastWasDouble bool
	// response meta block (HTTP status + timing + headers) shown above the body
	respHeader string
	// number of fixed header rows rendered above the scrollable body
	respHeaderLines int
	// horizontal scroll offset (in cells) for wide response content
	respHScroll int
	// left-hand file explorer panel (nil when not initialised)
	filesPanel *filesPanel
	// action popup shown on Enter over a request line (nil when inactive)
	actionMenu *actionMenu
	// navigation popup shown on Ctrl+G (nil when closed)
	nav *navMenu
	// form editor popup (key=value body editor) opened from the actions menu
	// (nil when closed)
	form *formEditor
	// search is the unified top-row search box open over a non-edit panel (the
	// file list or the response). It consumes all keys while non-nil and is
	// rendered on the top frame row. target says which panel it filters.
	search *searchBox
	// findMatches holds every (case-insensitive) match of the query on the
	// response body lines: one slice per body line, matches at rune columns.
	// Only meaningful while search targets the response pane.
	findMatches [][]findMatch
	// findCur is the flat index into the response matches of the currently
	// selected match.
	findCur int
	// dirty tracks whether the editor has unsaved changes since the last save.
	dirty bool
	// autosaveDeadline is the time until which edits keep postponing the save.
	autosaveDeadline time.Time
	// editBatchOpen marks that a character-edit batch is open, coalescing a
	// fast run of single-char edits (e.g. a paste delivered character-by-char on
	// Windows Terminal) into a single undo step. It closes after a short gap.
	editBatchOpen     bool
	editBatchDeadline time.Time
	// burstText accumulates the characters of the current burst (including
	// newlines) so a char-by-char pasted cURL can be detected and converted when
	// the batch closes. Stored as []byte: the model is copied by value in
	// bubbletea, and a strings.Builder must not be copied.
	burstText []byte
	// perFileCursor remembers the cursor row/col/scroll for each opened file,
	// keyed by absolute file path, so switching between files restores the
	// position each file was left at.
	perFileCursor map[string]editorPos
	// lastPersisted is the last cursor position written to settings, used by
	// the periodic autosave loop to avoid rewriting appsettings.yml on every
	// tick when nothing moved.
	lastPersisted editorPos
	// active profile name (a *.profile file whose @var variables are merged
	// into request substitution); "" when no profile is active
	profile string
	// helpVisible forces the hotkey reference to show in the right pane (F1),
	// regardless of whether a response is present. It is cleared automatically
	// by the next response so the result pane is not covered.
	helpVisible bool
	// helpScroll is the vertical scroll offset of the help reference panel,
	// used once the reference is taller than the pane. It lets the variables
	// section scroll into view on short windows.
	helpScroll int
	// hover tracks the mouse cursor position for hover highlighting in popups
	// (action menu, navigation), the files panel and the copy button. Only
	// meaningful while mouse motion events are flowing; reset to -1 on release
	// or when the cursor leaves the pane.
	hoverX, hoverY int
	// dialog is a modal framed popup (confirmations, prompts) drawn centred
	// over the panes. When non-nil it owns all keyboard and mouse input until
	// dismissed.
	dialog *dialogBox
}

// editorPos captures where the cursor was left in a file (canonical path key).
type editorPos struct {
	row, col, scroll int
}

func initialModel() model {
	return model{
		ed:     newEditor(exampleHTTP, 80, 20),
		active: paneEdit,
		status: i18n.T("ready.initial"),
		hoverX: -1,
		hoverY: -1,
	}
}

// Args configures the initial UI state from the CLI.
type Args struct {
	FilePath string
	Width    int // optional; 0 = default 100
	Height   int // optional; 0 = default 30
}

// New builds the start model. If filePath is set, its content is loaded;
// otherwise the last session file is opened if present.
func New(args Args) tea.Model {
	// Force 24-bit ANSI color rendering so syntax highlighting and the block
	// cursor are visible even when stdout is not detected as a terminal.
	lipgloss.SetColorProfile(termenv.TrueColor)

	content := exampleHTTP
	openPath := ""
	switch {
	case args.FilePath != "":
		if data, err := os.ReadFile(args.FilePath); err == nil {
			content = string(data)
			openPath = args.FilePath
			rememberLastOpened(args.FilePath)
		}
	default:
		// reopen the last explicitly opened file if it still exists
		if last := lastOpenedPath(); last != "" {
			if data, err := os.ReadFile(last); err == nil {
				content = string(data)
				openPath = last
				break
			}
		}
		// otherwise restore the last session content
		if data, err := os.ReadFile(sessionPath()); err == nil {
			content = string(data)
			openPath = sessionPath()
		}
	}
	ed := newEditor(content, 80, 20)
	w, h := args.Width, args.Height
	if w == 0 {
		w = 100
	}
	if h == 0 {
		h = 30
	}
	ed.width, ed.height = w/2, h-6
	// restore cursor / scroll / pane from settings
	s := settings.Load()
	i18n.SetLocale(i18n.Resolve(s.Lang))
	if s.CursorRow >= 0 && s.CursorRow < len(ed.Lines()) {
		ed.curRow = s.CursorRow
	}
	if s.CursorCol >= 0 {
		ed.curCol = s.CursorCol
		ed.clampCol()
	}
	if s.EditorScroll >= 0 {
		ed.scroll = s.EditorScroll
	}
	pane := paneEdit
	if s.ActivePane == 1 {
		pane = paneResp
	}
	// restore the per-file cursor for the opened file (freshly saved on the fly,
	// unlike the global cursor which is only written on a clean quit). This
	// keeps the position correct even when the app was closed via the window's
	// close button.
	m := &model{ed: ed}
	m.applyCursor(openPath)
	ed = m.ed
	// left file panel: list .http files in the working directory
	fp := &filesPanel{all: httpFilesInDir(".")}
	// ensure at least one profile exists so the profile section always has
	// something usable; create default.profile when none exist yet
	ensureDefaultProfile()
	// load environment profiles and restore the active one from settings
	fp.loadProfiles()
	profile := s.ActiveProfile
	if profile == "" {
		profile = defaultProfileName
	}
	if _, err := os.Stat(profile); err != nil {
		// fall back to the first available profile so exactly one is always
		// active
		if len(fp.profiles) > 0 {
			profile = fp.profiles[0]
		} else {
			profile = ""
		}
	}
	// highlight the file actually open on startup: with an empty filter row 0
	// is the "+ Новый файл" pseudo-entry, so the file index is offset by one.
	if openPath != "" {
		openBase := filepath.Base(openPath)
		for i, f := range fp.all {
			if f == openBase {
				fp.sel = i + 1
				break
			}
		}
	}
	return model{
		ed:         ed,
		active:     pane,
		filesPanel: fp,
		profile:    profile,
		status:     i18n.T("ready.main"),
		filePath:   openPath,
		width:      w,
		height:     h,
		hoverX:     -1,
		hoverY:     -1,
	}
}

// Snapshot returns a rendered frame (for tests/snapshots without a terminal).
func Snapshot(args Args) string {
	m := New(args).(model)
	return m.View()
}

// runResultMsg carries the finished request.
type runResultMsg struct {
	res  *runner.Result
	body []byte
}

func (m model) Init() tea.Cmd {
	// Hide the system cursor; we draw our own block cursor in the editor.
	return tea.Batch(tea.HideCursor, autosaveLoop())
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		m.ed.width = m.layout().mid
		m.ed.height = msg.Height - 6
	case tea.KeyMsg:
		return m.handleKey(msg)
	case tea.MouseMsg:
		m, cmd := m.handleMouse(msg)
		return m, cmd
	case runResultMsg:
		return m.applyResponse(msg), nil
	case autosaveMsg:
		now := time.Now()
		m.closeEditBatchIfExpired(now)
		m.applyAutosave(now)
		m.persistCursorIfChanged()
		return m, autosaveLoop()
	}
	return m, nil
}

// editorMaxScroll returns the maximum allowed scroll offset for the editor
// given its visible height, or 0 when the content fits (no scrolling).
func (m *model) editorMaxScroll() int {
	n := len(m.ed.Lines())
	vis := m.ed.height
	if vis < 0 {
		vis = 0
	}
	if n <= vis {
		return 0
	}
	return n - vis
}

// respMaxScroll returns the maximum scroll offset for the response body pane.
func (m *model) respMaxScroll() int {
	lines := respBodyLines(m)
	// visible scrollable rows: content height minus copy button and header
	vis := m.height - 6 - m.respHeaderLines
	if vis < 0 {
		vis = 0
	}
	if len(lines) <= vis {
		return 0
	}
	return len(lines) - vis
}

// clampEditorScroll keeps editor scroll within valid bounds.
func (m *model) clampEditorScroll() {
	mx := m.editorMaxScroll()
	if m.ed.scroll < 0 {
		m.ed.scroll = 0
	}
	if m.ed.scroll > mx {
		m.ed.scroll = mx
	}
}

// screen content layout inside the editor pane (0-based, relative to pane):
//
//	col 0        : left border (drawn by lipgloss)
//	col 1        : run icon (▶) for request lines, else ' '
//	col 2..5     : line number "%3d "
//	col 6..      : source text
const (
	paneContentX = 1 // first content column after left border
	textColAbs   = 6 // absolute pane column where source text starts
	// headerHeight is the number of fixed rows drawn above the panes (the
	// file-name header). Mouse and scrollbar coordinate math must account for it.
	headerHeight = 1
)

// selectionForLine returns the rune-column selection range on a given 0-based
// line, and whether the line is part of the active drag selection.
func (m *model) selectionForLine(row int) (selStart, selEnd int, hasSel bool) {
	if !m.selActive {
		return 0, 0, false
	}
	aRow, aCol := m.selAnchorRow, m.selAnchorCol
	cRow, cCol := m.ed.curRow, m.ed.curCol
	// Normalize start/end rows.
	startRow, startCol, endRow, endCol := aRow, aCol, cRow, cCol
	if (endRow < startRow) || (endRow == startRow && endCol < startCol) {
		startRow, startCol, endRow, endCol = cRow, cCol, aRow, aCol
	}
	if row < startRow || row > endRow {
		return 0, 0, false
	}
	switch {
	case startRow == endRow:
		if row != startRow {
			return 0, 0, false
		}
		return startCol, endCol, true
	case row == startRow:
		return startCol, m.ed.lineLen(row), true
	case row == endRow:
		return 0, endCol, true
	default:
		return 0, m.ed.lineLen(row), true
	}
}

// isRequestLine reports whether the line at 0-based row is an HTTP request line.
func isRequestLine(lines []string, row int) bool {
	if row < 0 || row >= len(lines) {
		return false
	}
	return httpfile.IsRequestLine(lines[row])
}

// dumpDebug writes a snapshot of the UI state to debug.txt for diagnostics.
// Triggered with Ctrl+D. Includes editor text, cursor, scroll, pane layout and
// geometry constants so rendering/mouse bugs can be reproduced offline.
func (m *model) dumpDebug() {
	var b strings.Builder
	lines := m.ed.Lines()
	req := httpfile.GetRequestAtLine(httpfile.ParseFile(m.ed.Text()), m.ed.curRow+1)
	b.WriteString("== HTTP Tool debug dump ==\n")
	b.WriteString(fmt.Sprintf("time:         %s\n", time.Now().Format("15:04:05")))
	b.WriteString(fmt.Sprintf("width:        %d\n", m.width))
	b.WriteString(fmt.Sprintf("height:       %d\n", m.height))
	b.WriteString(fmt.Sprintf("half:         %d\n", m.width/2))
	b.WriteString(fmt.Sprintf("active pane:  %s\n", paneName(m.active)))
	b.WriteString(fmt.Sprintf("state:        %s\n", stateName(m.state)))
	b.WriteString(fmt.Sprintf("status:       %s\n", m.status))
	b.WriteString(fmt.Sprintf("filePath:     %s\n", m.filePath))
	b.WriteString("\n-- editor --\n")
	b.WriteString(fmt.Sprintf("curRow:       %d (0-based)\n", m.ed.curRow))
	b.WriteString(fmt.Sprintf("curCol:       %d (0-based rune)\n", m.ed.curCol))
	b.WriteString(fmt.Sprintf("onIcon:       %v\n", m.ed.onIcon))
	b.WriteString(fmt.Sprintf("scroll:       %d\n", m.ed.scroll))
	b.WriteString(fmt.Sprintf("height:       %d (visible lines)\n", m.ed.height))
	b.WriteString(fmt.Sprintf("width:        %d\n", m.ed.width))
	b.WriteString(fmt.Sprintf("lineCount:    %d\n", len(lines)))
	if req != nil {
		b.WriteString(fmt.Sprintf("varCount:     %d\n", len(req.Vars)))
		b.WriteString("-- vars (name : value) --\n")
		for name, val := range req.Vars {
			b.WriteString(fmt.Sprintf("%s : %s\n", name, val))
		}
	} else {
		b.WriteString("varCount:     -\n")
	}
	b.WriteString("-- editor lines (index : content) --\n")
	for i, ln := range lines {
		b.WriteString(fmt.Sprintf("%3d : %s\n", i+1, ln))
	}
	b.WriteString("\n-- selection --\n")
	b.WriteString(fmt.Sprintf("selActive:    %v\n", m.selActive))
	b.WriteString(fmt.Sprintf("anchor:       (%d,%d)\n", m.selAnchorRow, m.selAnchorCol))
	b.WriteString("\n-- response pane --\n")
	b.WriteString(fmt.Sprintf("respScroll:   %d\n", m.respScroll))
	b.WriteString(fmt.Sprintf("respLen:      %d\n", len(m.response)))
	b.WriteString("\n-- geometry constants --\n")
	b.WriteString(fmt.Sprintf("paneContentX: %d\n", paneContentX))
	b.WriteString(fmt.Sprintf("textColAbs:   %d\n", textColAbs))

	// Actual rendered frame (what would be on screen right now).
	b.WriteString("\n-- RENDERED VIEW (strip ANSI) --\n")
	b.WriteString(stripANSI(m.View()))
	b.WriteString("\n-- END RENDERED VIEW --\n")

	if err := os.WriteFile("debug.txt", []byte(b.String()), 0o644); err != nil {
		m.status = i18n.T("err.debugWrite", err.Error())
		return
	}
	m.status = i18n.T("status.debugDumped")
}

func paneName(p pane) string {
	if p == paneEdit {
		return "edit"
	}
	return "response"
}

func stateName(s runState) string {
	if s == stateRunning {
		return "running"
	}
	return "idle"
}

// selectAll selects the entire editor buffer or the entire response body
// depending on the active pane.
func (m *model) selectAll() {
	if m.active == paneEdit {
		m.selActive = true
		m.selAnchorRow, m.selAnchorCol = 0, 0
		m.ed.curRow = len(m.ed.Lines()) - 1
		m.ed.curCol = m.ed.lineLen(m.ed.curRow)
		m.ed.EnsureVisible()
	} else if m.active == paneResp {
		lines := respPaneLines(m)
		if len(lines) == 0 {
			return
		}
		m.respSelActive = true
		m.respSelAnchorRow, m.respSelAnchorCol = 0, 0
		m.respSelCurRow = len(lines) - 1
		m.respSelCurCol = len([]rune(lines[m.respSelCurRow]))
	}
}

// shiftSelect expands the selection with Shift+arrow navigation.
func (m *model) shiftSelect(key string) {
	if !m.selActive {
		// start a new selection anchored at current cursor
		m.selAnchorRow, m.selAnchorCol = m.ed.curRow, m.ed.curCol
		m.selActive = true
	}
	m.ed.MoveCursor(navKey(key))
	m.ed.EnsureVisible()
}

func (m *model) scrollBy(n int) {
	m.respScroll += n
	if m.respScroll < 0 {
		m.respScroll = 0
	}
	mx := m.respMaxScroll()
	if m.respScroll > mx {
		m.respScroll = mx
	}
}

// runRequest executes the request block under the cursor.
func (m *model) runRequest() tea.Cmd {
	if m.state == stateRunning {
		return nil
	}
	m.ed.sanitize()
	reqs := httpfile.ParseFile(m.ed.Text())
	req := httpfile.GetRequestAtLine(reqs, m.ed.curRow+1)
	if req == nil {
		m.status = i18n.T("err.noRequestCursor")
		m.active = paneEdit
		return nil
	}
	m.state = stateRunning
	m.status = i18n.T("status.running", req.Method, req.URL)
	// Resolve {{name}} / {{$fn}} placeholders before building the request.
	// An unresolved placeholder aborts the run with a clear message instead of
	// silently sending a raw "{{x}}" on the wire.
	// The active profile, if any, supplies fallback @var values. File-level
	// (@var in the .http) values win over the profile; the profile fills in
	// anything the file does not define.
	vars := req.Vars
	if pv := m.activeProfileVars(); len(pv) > 0 {
		vars = httpfile.MergeVars(pv, vars) // local (vars) overrides profile (pv)
	}
	sub, subErr := httpfile.Substitute(req.URL, vars)
	if subErr != nil {
		m.state = stateIdle
		m.status = i18n.T("err.variable", subErr.(*httpfile.SubstitutionError).First())
		m.active = paneEdit
		return nil
	}
	req.URL = sub
	for i := range req.Headers {
		hv, herr := httpfile.Substitute(req.Headers[i].Value, vars)
		if herr != nil {
			m.state = stateIdle
			m.status = i18n.T("err.variable", herr.(*httpfile.SubstitutionError).First())
			m.active = paneEdit
			return nil
		}
		req.Headers[i].Value = hv
	}
	if req.Body != "" {
		bv, berr := httpfile.Substitute(req.Body, vars)
		if berr != nil {
			m.state = stateIdle
			m.status = i18n.T("err.variable", berr.(*httpfile.SubstitutionError).First())
			m.active = paneEdit
			return nil
		}
		req.Body = bv
	}
	opts, optErr := runner.ApplyOptions(*req, runner.Options{FollowRedirects: false})
	if optErr != nil {
		m.state = stateIdle
		m.status = i18n.T("err.option", optErr.Error())
		m.active = paneEdit
		return nil
	}
	clientTimeout := opts.TimeoutOrDefault()
	return func() tea.Msg {
		dur := clientTimeout
		if dur < 0 {
			dur = 0
		}
		ctx, cancel := context.WithTimeout(context.Background(), dur)
		defer cancel()
		res := runner.Run(ctx, *req, opts)
		if res.Err != nil {
			return runResultMsg{res: res}
		}
		body, err := runner.BodyBytes(res.Response)
		if err != nil {
			return runResultMsg{res: res}
		}
		return runResultMsg{res: res, body: body}
	}
}

// applyResponse renders the response into the right pane.
func (m model) applyResponse(msg runResultMsg) tea.Model {
	m.state = stateIdle
	// A fresh response always brings the result pane back: never let the F1
	// help cover the output of a request.
	m.helpVisible = false
	res := msg.res
	if res.Err != nil {
		// Network errors (connection refused, DNS, timeout, TLS, etc.) are not
		// HTTP responses, but they still belong in the response pane so the user
		// can read, scroll and copy the failure just like a real response.
		m.status = i18n.T("err.network", res.Err.Error())
		var hdr strings.Builder
		hdr.WriteString(i18n.T("resp.netFailed") + "\n")
		if res.Request != nil {
			hdr.WriteString(res.Request.Method + " " + res.Request.URL.String() + "\n")
		}
		if res.Duration > 0 {
			hdr.WriteString(i18n.T("resp.duration", res.Duration.Round(time.Millisecond).String()) + "\n")
		}
		var b strings.Builder
		b.WriteString(hdr.String())
		b.WriteString("\n")
		// The error is often one long unbroken line; wrap it to the response
		// pane width so the whole message is visible instead of being cut off
		// at the right edge of the pane.
		b.WriteString(wrapToWidth(res.Err.Error(), m.respContentWidth()))
		b.WriteString("\n")
		return m.setResponse(b.String(), hdr.String(), nil)
	}
	var b strings.Builder
	var hdr strings.Builder
	hdr.WriteString(res.Request.Proto + " " + res.Status + "\n")
	hdr.WriteString(i18n.T("resp.duration", res.Duration.Round(time.Millisecond).String()) + "\n")
	res.Response.Header.Write(&hdr)
	// http.Header.Write terminates each line with CRLF; the trailing \r is a
	// literal carriage return that in a terminal rewinds the cursor to column 0,
	// so the padded tail of each header row would overwrite the pane on the left.
	// Normalize to \n so header rows render as plain lines.
	hdrStr := strings.ReplaceAll(hdr.String(), "\r\n", "\n")
	b.WriteString(hdrStr)
	b.WriteString("\n")
	if len(msg.body) > 0 {
		b.WriteString(formatBody(msg.body))
	}
	if res.Response != nil {
		m.status = fmt.Sprintf(i18n.T("resp.statusCode"), res.Response.StatusCode)
	}
	return m.setResponse(b.String(), hdrStr, msg.body)
}

// respContentWidth returns the display width of the response pane body in
// cells, mirroring how renderResponse computes contentW.
func (m *model) respContentWidth() int {
	fw := m.filesWidth()
	mid := (m.width - fw) / 2
	respW := m.width - fw - mid - 6
	if respW < 10 {
		respW = 10
	}
	w := respW - 2 // left border + scrollbar column
	if w < 8 {
		w = 8
	}
	return w
}

// setResponse stores a rendered response (header + body) in the response pane
// and resets its scroll state. Used for both real HTTP responses and network
// errors, so the failure text behaves like a normal response body.
func (m model) setResponse(response, header string, body []byte) model {
	m.response = response
	m.respHeader = header
	m.respHeaderLines = 0
	if m.respHeader != "" {
		m.respHeaderLines = len(strings.Split(strings.TrimRight(m.respHeader, "\n"), "\n"))
	}
	m.lastBody = body
	m.respScroll = 0
	m.respHScroll = 0
	return m
}

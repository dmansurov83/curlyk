package tui

import (
	"fmt"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

// modelWithRespSearch returns a model with a response body set and the unified
// search box open targeting the response pane.
func modelWithRespSearch(t *testing.T, resp string) model {
	t.Helper()
	m := New(Args{Width: 120, Height: 30}).(model)
	m.response = "HTTP/1.1 200 OK\nContent-Type: application/json\n\n" + resp
	m.respHeader = "HTTP/1.1 200 OK\nContent-Type: application/json"
	m.openSearch(paneResp, "")
	if m.search == nil {
		t.Fatal("openSearch(paneResp) should open the search box")
	}
	return m
}

func (m *model) searchType(s string) {
	m.handleSearchKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)})
}

func (m *model) searchEnter() {
	m.handleSearchKey(tea.KeyMsg{Type: tea.KeyEnter})
}

func (m *model) searchPrev() {
	m.handleSearchKey(tea.KeyMsg{Type: tea.KeyShiftTab})
}

// TestFindMatchesAllOccurrences verifies findMatchesFor locates every match
// across lines, including multiple on one line.
func TestFindMatchesAllOccurrences(t *testing.T) {
	m := modelWithRespSearch(t, "foo bar\nfoofoo\nabc")
	per := m.findMatchesFor("foo")
	// Body lines: [0]="foo bar", [1]="foofoo", [2]="abc".
	if len(per) != 3 {
		t.Fatalf("expected 3 body lines, got %d", len(per))
	}
	if len(per[0]) != 1 || len(per[1]) != 2 || len(per[2]) != 0 {
		t.Fatalf("matches per line = %d/%d/%d, want 1/2/0",
			len(per[0]), len(per[1]), len(per[2]))
	}
	if got := len(flattenFind(per)); got != 3 {
		t.Fatalf("flattened total=%d, want 3", got)
	}
}

// TestFindScrollCentres verifies scrolling centres the matched body line in the
// visible viewport (for a reasonably tall window), clamping near the edges.
func TestFindScrollCentres(t *testing.T) {
	m := mockFindScrollModel(t, 30)
	m.searchType("needle")
	flat := flattenFind(m.findMatches)
	if len(flat) < 6 {
		t.Fatalf("expected multiple matches, got %d", len(flat))
	}
	vis := m.height - 6 - m.respHeaderLines
	mx := m.respMaxScroll()
	// Pick a match in the middle of the body, away from both edges, and verify
	// it lands centred: scroll = line - vis/2 (within the max-scroll clamp).
	mid := flat[len(flat)/2].line
	m.findCur = len(flat) / 2
	m.scrollToCurrentMatch()
	want := mid - vis/2
	if want < 0 {
		want = 0
	}
	if want > mx {
		want = mx
	}
	if m.respScroll != want {
		t.Fatalf("middle match scroll=%d want %d (centred)", m.respScroll, want)
	}
	// The very last match, near the bottom, must clamp to the max scroll (it
	// cannot be centred past the end of the body).
	last := flat[len(flat)-1].line
	m.findCur = len(flat) - 1
	m.scrollToCurrentMatch()
	if m.respScroll != mx {
		t.Fatalf("last match scroll=%d want max=%d (clamped), line=%d", m.respScroll, mx, last)
	}
}

// mockFindScrollModel builds a model with a tall response body and repeated
// "needle" matches spread across it.
func mockFindScrollModel(t *testing.T, height int) model {
	t.Helper()
	// One header line; body has many lines, several with "needle".
	var b strings.Builder
	for i := 0; i < 40; i++ {
		b.WriteString("line ")
		b.WriteString(strings.Repeat(fmt.Sprintf("%d ", i), 1))
		if i%7 == 1 || i%7 == 3 {
			b.WriteString("needle")
		}
		b.WriteString("\n")
	}
	m := New(Args{Width: 120, Height: height}).(model)
	m.response = "HTTP/1.1 200 OK\n\n" + b.String()
	m.respHeader = "HTTP/1.1 200 OK\n"
	m.openSearch(paneResp, "")
	return m
}

// TestFindLiveCounter verifies the counter tracks current/total as the query
// changes and Enter walks forward (wrapping).
func TestFindLiveCounter(t *testing.T) {
	m := modelWithRespSearch(t, "aa aa\nbbb")
	m.searchType("aa")
	if got := m.findCountText(); got != "1/2" {
		t.Fatalf("counter after typing 'aa' = %q, want 1/2", got)
	}
	m.searchEnter()
	if got := m.findCountText(); got != "2/2" {
		t.Fatalf("counter after next = %q, want 2/2", got)
	}
	m.searchEnter()
	if got := m.findCountText(); got != "1/2" {
		t.Fatalf("counter after wrap = %q, want 1/2", got)
	}
}

// TestFindShiftEnterPrev verifies backward navigation works. With a query that
// matches once, both directions keep the counter stable at 1/1.
func TestFindShiftEnterPrev(t *testing.T) {
	m := modelWithRespSearch(t, "here\nxx")
	m.searchType("her")
	if got := m.findCountText(); got != "1/1" {
		t.Fatalf("counter after typing 'her' = %q, want 1/1", got)
	}
	m.searchPrev()
	if got := m.findCountText(); got != "1/1" {
		t.Fatalf("after shift+tab (prev) = %q, want 1/1", got)
	}
}

// TestFindNoMatches verifies no matches yields "0/0" and Enter is a no-op.
func TestFindNoMatches(t *testing.T) {
	m := modelWithRespSearch(t, "alpha\nbeta")
	m.searchType("zzz")
	if got := m.findCountText(); got != "0/0" {
		t.Fatalf("counter no matches = %q, want 0/0", got)
	}
	m.searchEnter()
	if got := m.findCountText(); got != "0/0" {
		t.Fatalf("counter after Enter no matches = %q, want 0/0", got)
	}
}

// TestSearchEscCloses verifies Esc closes the search box and clears response
// highlight state.
func TestSearchEscCloses(t *testing.T) {
	m := modelWithRespSearch(t, "alpha\nbeta")
	m.searchType("a")
	if m.search == nil {
		t.Fatal("search box should be open")
	}
	m.handleSearchKey(tea.KeyMsg{Type: tea.KeyEsc})
	if m.search != nil {
		t.Fatal("Esc should close the search box")
	}
	if m.findMatches != nil || m.findCur != 0 {
		t.Fatal("find highlight state should reset on close")
	}
}

// TestTypingOverResponseOpensSearch verifies typing a printable key while the
// response pane is active opens the search box pre-filled with that text and
// immediately matches.
func TestTypingOverResponseOpensSearch(t *testing.T) {
	m := New(Args{Width: 120, Height: 30}).(model)
	m.response = "HTTP/1.1 200 OK\n\nneedle needle\n"
	m.respHeader = "HTTP/1.1 200 OK\n"
	m.active = paneResp
	res, _ := m.handleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("needle")})
	r := res.(model)
	if r.search == nil {
		t.Fatal("typing over the response pane should open the search box")
	}
	if got := r.searchQuery(); got != "needle" {
		t.Fatalf("search prefilled = %q, want 'needle'", got)
	}
	if got := r.findCountText(); got != "1/2" {
		t.Fatalf("counter after typing over response = %q, want 1/2", got)
	}
}

// TestTypingControlOverResponseDoesNotOpenSearch verifies a control key over the
// response pane does not open the search box.
func TestTypingControlOverResponseDoesNotOpenSearch(t *testing.T) {
	m := New(Args{Width: 120, Height: 30}).(model)
	m.response = "HTTP/1.1 200 OK\n\nbody\n"
	m.respHeader = "HTTP/1.1 200 OK\n"
	m.active = paneResp
	res, _ := m.handleKey(tea.KeyMsg{Type: tea.KeyCtrlZ})
	r := res.(model)
	if r.search != nil {
		t.Fatal("a control key over the response pane should not open the search box")
	}
}

// TestFindRendersHighlight verifies the response view contains the styled
// highlight for the current match (findCurStyle background 196) once a query is
// entered and the pane re-renders.
func TestFindRendersHighlight(t *testing.T) {
	m := modelWithRespSearch(t, "hello\nworld\n")
	m.searchType("hello")
	m.height = 30
	out := m.renderResponse(57, 24)
	if !strings.Contains(out, "48;5;196") {
		t.Fatalf("expected current-match highlight (background 196) in rendered response:\n%s", out)
	}
	if !strings.Contains(out, "hello") {
		t.Fatalf("expected query text present in rendered response:\n%s", out)
	}
}

// TestSearchBarPinnedToTop verifies the search bar renders on the top header row
// of the full frame (in place of the file-name bar), so the pane content and its
// coordinates are unchanged.
func TestSearchBarPinnedToTop(t *testing.T) {
	m := modelWithRespSearch(t, "match here\n")
	m.searchType("match")
	out := m.View()
	rows := strings.Split(strings.TrimRight(out, "\n"), "\n")
	if len(rows) < 1 {
		t.Fatalf("expected rendered frame rows, got none")
	}
	bars := stripANSI(rows[0])
	if !strings.Contains(bars, "Найти:") && !strings.Contains(bars, "Find:") {
		t.Fatalf("top row should be the search bar, got %q", bars)
	}
	found := false
	for _, r := range rows {
		if strings.Contains(stripANSI(r), "match") {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("body should still appear somewhere in the frame, got %q", stripANSI(out))
	}
}

// TestSearchDoesNotShiftSelection verifies that opening the search bar does not
// change where a mouse click maps in the response pane — the regression where an
// in-pane search row shifted selection by one line.
func TestSearchDoesNotShiftSelection(t *testing.T) {
	body := "HTTP/1.1 200 OK\n\n" + strings.Repeat("line x\n", 10)
	closed := New(Args{Width: 120, Height: 30}).(model)
	closed.response = body
	closed.respHeader = "HTTP/1.1 200 OK\n"

	open := closed
	open.openSearch(paneResp, "")
	open.searchType("line")

	half := closed.layout().half
	y := headerHeight + 2 + closed.respHeaderLines
	_, _, okClosed := mouseToRespCell(&closed, half+1, y+1)
	rowO, _, okOpen := mouseToRespCell(&open, half+1, y+1)
	if !okClosed || !okOpen {
		t.Fatalf("click should map in both states: closed=%v open=%v", okClosed, okOpen)
	}
	if rowO != 2 {
		t.Fatalf("open search row mapping=%d want 2 (unshifted)", rowO)
	}
}

// TestSearchBarKeepsFrame verifies the full View with the search box open shows
// the search bar and does not overflow the terminal height.
func TestSearchBarKeepsFrame(t *testing.T) {
	for _, h := range []int{12, 16, 20, 30} {
		m := New(Args{Width: 120, Height: h}).(model)
		m.response = "HTTP/1.1 200 OK\n\nmatch me\n"
		m.respHeader = "HTTP/1.1 200 OK\n"
		m.openSearch(paneResp, "")
		m.searchType("match")
		out := m.View()
		if n := len(strings.Split(out, "\n")); n > h {
			t.Fatalf("height=%d: frame with search bar overflowed (%d lines)", h, n)
		}
		if !strings.Contains(stripANSI(out), "Найти:") && !strings.Contains(stripANSI(out), "Find:") {
			t.Fatalf("height=%d: search bar not rendered in frame", h)
		}
	}
}

// TestTypingOverFilesOpensSearch verifies typing a printable key while the files
// panel is active opens the unified search box targeting files, pre-filled and
// applied to the file filter.
func TestTypingOverFilesOpensSearch(t *testing.T) {
	m := New(Args{Width: 120, Height: 30}).(model)
	m.filesPanel = &filesPanel{all: []string{"alpha.http", "beta.http", "gamma.http"}}
	m.active = paneFiles
	res, _ := m.handleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("alp")})
	r := res.(model)
	if r.search == nil {
		t.Fatal("typing over files panel should open the search box")
	}
	if r.search.target != paneFiles {
		t.Fatalf("search target=%d want paneFiles", r.search.target)
	}
	if got := r.searchQuery(); got != "alp" {
		t.Fatalf("search prefilled = %q, want 'alp'", got)
	}
	if got := len(r.filesPanel.filtered()); got != 1 {
		t.Fatalf("filtered files = %d want 1", got)
	}
}

// TestFileSearchEscClearsFilter verifies closing the files search restores the
// full file list.
func TestFileSearchEscClearsFilter(t *testing.T) {
	m := New(Args{Width: 120, Height: 30}).(model)
	m.filesPanel = &filesPanel{all: []string{"alpha.http", "beta.http"}}
	m.openSearch(paneFiles, "")
	m.searchType("beta")
	if got := len(m.filesPanel.filtered()); got != 1 {
		t.Fatalf("filtered = %d want 1", got)
	}
	m.handleSearchKey(tea.KeyMsg{Type: tea.KeyEsc})
	if m.search != nil {
		t.Fatal("Esc should close the files search")
	}
	if m.filesPanel.filter != "" {
		t.Fatalf("files filter should reset on close, got %q", m.filesPanel.filter)
	}
	if got := len(m.filesPanel.filtered()); got != 2 {
		t.Fatalf("after close filtered = %d want 2", got)
	}
}

// TestFileSearchFiltersTypingAhead verifies continued typing narrows the list.
func TestFileSearchFiltersTypingAhead(t *testing.T) {
	m := New(Args{Width: 120, Height: 30}).(model)
	m.filesPanel = &filesPanel{all: []string{"b.http", "ba.http", "c.http"}}
	m.openSearch(paneFiles, "")
	m.searchType("b")
	if got := len(m.filesPanel.filtered()); got != 2 {
		t.Fatalf("filter 'b' = %d want 2", got)
	}
	m.searchType("a")
	if got := len(m.filesPanel.filtered()); got != 1 {
		t.Fatalf("filter 'ba' = %d want 1", got)
	}
}

// TestSearchRetargetsToActivePane verifies the key fix: when the user opens a
// files search and then switches to the response pane (click or Tab), typing
// searches the response — never the files panel, regardless of which search was
// opened first.
func TestSearchRetargetsToActivePane(t *testing.T) {
	m := New(Args{Width: 120, Height: 30}).(model)
	m.response = "HTTP/1.1 200 OK\n\nneedle needle\n"
	m.respHeader = "HTTP/1.1 200 OK\n"
	m.filesPanel = &filesPanel{all: []string{"a.http", "b.http"}}

	// open a files search, then click into the response pane (retarget)
	m.active = paneFiles
	r1, _ := m.handleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("a")})
	m = r1.(model)
	if m.search == nil || m.search.target != paneFiles {
		t.Fatalf("expected files search open, got target=%d", targetOf(m))
	}
	half := m.layout().half
	r2, _ := m.Update(tea.MouseMsg{
		Button: tea.MouseButtonLeft, Action: tea.MouseActionPress,
		X: half + 1, Y: headerHeight + 2,
	})
	m = r2.(model)
	if m.search == nil {
		t.Fatal("click into response should keep the search box open")
	}
	if m.search.target != paneResp {
		t.Fatalf("search should retarget to response, got target=%d", targetOf(m))
	}
	if m.active != paneResp {
		t.Fatalf("active=%d want paneResp", m.active)
	}
	// typing now filters the response, not files: the query carries typed text
	// matched against the response body (findCur advanced) rather than the files
	// panel's filter.
	r3, _ := m.handleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("a")})
	m = r3.(model)
	if m.searchQuery() == "" {
		t.Fatal("search query should carry typed text")
	}
	if m.findCur < 0 {
		t.Fatalf("response search should be active over the body, findCur=%d", m.findCur)
	}
}

// TestSearchTypingAppendsInFiles verifies more than one letter can be typed in
// the files search (the duplication regression).
func TestSearchTypingAppendsInFiles(t *testing.T) {
	m := New(Args{Width: 120, Height: 30}).(model)
	m.filesPanel = &filesPanel{all: []string{"ba.http", "b.http", "c.http"}}
	m.active = paneFiles
	r1, _ := m.handleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("b")})
	m = r1.(model)
	r2, _ := m.handleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("a")})
	m = r2.(model)
	if m.searchQuery() != "ba" {
		t.Fatalf("query=%q want 'ba' (multi-letter typing)", m.searchQuery())
	}
	if n := len(m.filesPanel.filtered()); n != 1 {
		t.Fatalf("filtered=%d want 1", n)
	}
}

// TestSearchTabCycles verifies Tab from a searchable pane cycles toward the
// editor and closes the search there (the editor takes text directly).
func TestSearchTabCycles(t *testing.T) {
	m := New(Args{Width: 120, Height: 30}).(model)
	m.filesPanel = &filesPanel{all: []string{"a.http"}}
	m.active = paneFiles
	r1, _ := m.handleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("a")})
	m = r1.(model)
	if m.search == nil {
		t.Fatal("files search should be open")
	}
	r2, _ := m.handleKey(tea.KeyMsg{Type: tea.KeyTab})
	m = r2.(model)
	if m.search != nil {
		t.Fatal("Tab from files search should close the search (cycle reaches editor)")
	}
	if m.active != paneEdit {
		t.Fatalf("active=%d want paneEdit", m.active)
	}
}

func targetOf(m model) int {
	if m.search == nil {
		return -1
	}
	return int(m.search.target)
}

package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/mattn/go-runewidth"
)

// buildNavSrc returns a small multi-request source with mixed named/unnamed
// requests.
func buildNavSrc() string {
	return `### Первый
GET https://api.test/v1/users
Accept: application/json

### Поиск
@name findUser
POST https://api.test/v1/find

{"q":"x"}

GET https://api.test/v1/list
`
}

// TestNavCollect verifies buildNavEntries gathers every request with its name,
// METHOD URL fallback for unnamed ones, and the 1-based source line.
func TestNavCollect(t *testing.T) {
	m := New(Args{Width: 120, Height: 30}).(model)
	m.active = paneEdit
	m.ed.SetText(buildNavSrc())

	entries := m.buildNavEntries()
	if len(entries) != 3 {
		t.Fatalf("want 3 entries, got %d", len(entries))
	}
	// line 2 = "GET https://api.test/v1/users", named? no.
	if entries[0].name != "" || entries[0].line != 2 {
		t.Errorf("entry0 name=%q line=%d want name '' line 2", entries[0].name, entries[0].line)
	}
	if !strings.HasPrefix(entries[0].label, "GET ") {
		t.Errorf("entry0 unnamed label should start with METHOD, got %q", entries[0].label)
	}
	// line 7 = "POST https://api.test/v1/find", annotated @name findUser above it
	if entries[1].name != "findUser" || entries[1].line != 7 {
		t.Errorf("entry1 name=%q line=%d want findUser line 7", entries[1].name, entries[1].line)
	}
	if !strings.HasPrefix(entries[1].label, "findUser") {
		t.Errorf("entry1 named label should be the @name, got %q", entries[1].label)
	}
	// line 11 = "GET https://api.test/v1/list"
	if entries[2].line != 11 {
		t.Errorf("entry2 line=%d want 11", entries[2].line)
	}
}

// TestNavFilter verifies the live filter narrows the visible list by name and by
// the METHOD URL fallback for unnamed requests.
func TestNavFilter(t *testing.T) {
	m := New(Args{Width: 120, Height: 30}).(model)
	m.active = paneEdit
	m.ed.SetText(buildNavSrc())
	m.nav = &navMenu{entries: m.buildNavEntries(), sel: 0}

	// Empty filter: all visible.
	if got := len(m.nav.visible()); got != 3 {
		t.Fatalf("empty filter visible=%d want 3", got)
	}
	// Filter by part of @name (case-insensitive).
	m.handleNavKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("find")})
	if got := m.nav.visible(); len(got) != 1 || m.nav.entries[got[0]].name != "findUser" {
		t.Fatalf("filter 'find' visible=%v want only findUser", got)
	}
	// Filter by URL fragment (unnamed requests are matched via METHOD URL).
	m.nav.filter = ""
	m.nav.sel = 0
	m.handleNavKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("list")})
	got := m.nav.visible()
	if len(got) != 1 || strings.HasPrefix(m.nav.entries[got[0]].name, "find") {
		t.Fatalf("filter 'list' visible=%v want the unnamed GET /list", got)
	}
}

// TestNavBackspace verifies Backspace trims the filter and the selection stays
// clamped to the visible list.
func TestNavBackspace(t *testing.T) {
	m := New(Args{Width: 120, Height: 30}).(model)
	m.nav = &navMenu{entries: m.buildNavEntries(), sel: 0, filter: "find"}
	m.handleNavKey(tea.KeyMsg{Type: tea.KeyBackspace})
	if m.nav.filter != "fin" {
		t.Fatalf("after backspace filter=%q want 'fin'", m.nav.filter)
	}
}

// TestNavJumpToRequest verifies Ctrl+G → Enter moves the cursor onto the target
// request line, restores focus to the editor and reports the jump in status.
func TestNavJumpToRequest(t *testing.T) {
	m := New(Args{Width: 120, Height: 30}).(model)
	m.active = paneEdit
	m.ed.SetText(buildNavSrc())
	m.ed.curRow = 0 // cursor at the top

	res, _ := m.handleKey(tea.KeyMsg{Type: tea.KeyCtrlG})
	mm := res.(model)
	if mm.nav == nil {
		t.Fatal("Ctrl+G should open the navigation popup")
	}
	// Jump to the first matched request (findUser at line 7).
	mm.handleNavKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("find")})
	mm.handleNavKey(tea.KeyMsg{Type: tea.KeyEnter})
	if mm.nav != nil {
		t.Fatal("Enter should close the navigation popup")
	}
	if mm.active != paneEdit {
		t.Errorf("focus should return to the editor, active=%d", mm.active)
	}
	if mm.ed.curRow != 6 {
		t.Errorf("cursor row=%d want 6 (line 7, 0-based)", mm.ed.curRow)
	}
	if mm.ed.onIcon {
		t.Error("cursor should not be on the run icon")
	}
	if !strings.Contains(mm.status, "findUser") {
		t.Errorf("status should mention the jumped request, got %q", mm.status)
	}
}

// TestNavEscCloses verifies Esc dismisses the navigation popup.
func TestNavEscCloses(t *testing.T) {
	m := New(Args{Width: 120, Height: 30}).(model)
	m.nav = &navMenu{entries: m.buildNavEntries(), sel: 0}
	m.handleNavKey(tea.KeyMsg{Type: tea.KeyEsc})
	if m.nav != nil {
		t.Error("Esc must close the navigation popup")
	}
}

// TestNavRenderShowsEntries verifies the popup draws a title row and the request
// entries inside the editor pane.
func TestNavRenderShowsEntries(t *testing.T) {
	m := New(Args{Width: 120, Height: 30}).(model)
	m.active = paneEdit
	m.ed.SetText(buildNavSrc())
	res, _ := m.handleKey(tea.KeyMsg{Type: tea.KeyCtrlG})
	mm := res.(model)
	if mm.nav == nil {
		t.Fatal("Ctrl+G should open the navigation popup")
	}
	v := stripANSI(mm.View())
	for _, frag := range []string{"findUser", "GET", "POST"} {
		if !strings.Contains(v, frag) {
			t.Errorf("view missing %q", frag)
		}
	}
}

// TestNavWideUnnamedTruncated verifies a very long unnamed request target is
// truncated to the pane width (rune-safe, no panic) instead of overflowing.
func TestNavWideUnnamedTruncated(t *testing.T) {
	long := strings.Repeat("a", 500)
	m := New(Args{Width: 80, Height: 30}).(model)
	m.active = paneEdit
	m.ed.SetText("### x\nGET http://x/" + long + "\n")
	m.beginNav()
	m.nav.filter = "http"
	lines := m.navLines(30) // small contentW
	if len(lines) == 0 {
		t.Fatal("navLines must render at least the title")
	}
	for _, ln := range lines {
		if runewidth.StringWidth(stripANSI(ln)) > 32 {
			t.Errorf("entry line wider than pane bounds (%d cells): %q", runewidth.StringWidth(stripANSI(ln)), ln)
		}
	}
}

// TestNavOpenKeepsFrameFit verifies opening the navigation popup in a small
// window does not grow the frame taller than the terminal height and keeps the
// top file-name header visible.
func TestNavOpenKeepsFrameFit(t *testing.T) {
	for _, h := range []int{9, 10, 12, 15, 20} {
		m := New(Args{Width: 100, Height: h}).(model)
		m.ed.SetText(buildNavSrc())
		m.active = paneEdit
		res, _ := m.handleKey(tea.KeyMsg{Type: tea.KeyCtrlG})
		mm := res.(model)
		if mm.nav == nil {
			t.Fatalf("height=%d: nav not opened", h)
		}
		out := mm.View()
		n := len(strings.Split(out, "\n"))
		if n > h {
			t.Errorf("height=%d: frame with nav overflowed (%d lines > %d)", h, n, h)
		}
		first := firstLine(out)
		if !strings.Contains(stripANSI(first), "Файл:") {
			t.Errorf("height=%d: top header lost when nav open; first=%q", h, first)
		}
	}
}

// TestNavManyRequests verifies the Terms of Done scenario: in a file with many
// requests, Ctrl+G → filter → Enter lands the cursor on the right request even
// when the on-screen list is truncated with an ellipsis.
func TestNavManyRequests(t *testing.T) {
	// Build 50 request blocks; each block is 4 lines:
	//   "### req\n" / "GET http://host/itemN\n" / "@name itemN\n" / "".
	const perBlock = 4
	var b strings.Builder
	for i := 0; i < 50; i++ {
		b.WriteString("### req\n")
		b.WriteString("GET http://host/item\n")
		b.WriteString("@name item\n")
		b.WriteString("\n")
	}
	m := New(Args{Width: 120, Height: 30}).(model)
	m.active = paneEdit
	m.ed.SetText(b.String())
	res, _ := m.handleKey(tea.KeyMsg{Type: tea.KeyCtrlG})
	mm := res.(model)
	if mm.nav == nil {
		t.Fatal("Ctrl+G should open the navigation popup")
	}
	if len(mm.nav.entries) != 50 {
		t.Fatalf("expect 50 entries, got %d", len(mm.nav.entries))
	}
	// With 50 matches the rendered popup is capped (maxNavRows entries + title +
	// ellipsis row).
	if h := mm.navHeight(); h > maxNavRows+2 {
		t.Errorf("navHeight=%d exceeds view cap %d", h, maxNavRows+2)
	}
	// Give only one request a unique @name, then filter down to it. Blocks are
	// independent here, so re-add a unique marker to the LAST block's name.
	last := 49
	unique := "uniquetarget"
	mm.nav.entries[last].name = unique
	mm.nav.entries[last].label = unique
	mm.nav.entries[last].search = strings.ToLower(unique)
	mm.nav.sel = 0
	mm.nav.filter = ""
	mm.handleNavKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("unique")})
	vis := mm.nav.visible()
	if len(vis) != 1 {
		t.Fatalf("filter 'unique' visible=%v want exactly 1 match", vis)
	}
	mm.handleNavKey(tea.KeyMsg{Type: tea.KeyEnter})
	if mm.nav != nil {
		t.Fatal("Enter should close the navigation popup")
	}
	// Request line of the last block is 0-based row `last*perBlock + 1`.
	wantRow := last*perBlock + 1
	if got := mm.ed.curRow; got != wantRow {
		t.Errorf("cursor row=%d want %d (last unique request line)", got, wantRow)
	}
}
func TestNavNoRequestsMessage(t *testing.T) {
	m := New(Args{Width: 120, Height: 30}).(model)
	m.active = paneEdit
	m.ed.SetText("just a comment\n\n# no requests here\n")
	res, _ := m.handleKey(tea.KeyMsg{Type: tea.KeyCtrlG})
	mm := res.(model)
	if mm.nav == nil {
		t.Fatal("Ctrl+G should still open the popup")
	}
	if len(mm.nav.entries) != 0 {
		t.Fatalf("expected 0 entries, got %d", len(mm.nav.entries))
	}
	v := stripANSI(mm.View())
	if !strings.Contains(v, "нет запросов") {
		t.Errorf("view should show the no-requests message, got:\n%s", v)
	}
	if got := mm.nav.visible(); len(got) != 0 {
		t.Errorf("visible() with no requests should be empty, got %v", got)
	}
}

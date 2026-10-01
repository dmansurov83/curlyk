package tui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/mattn/go-runewidth"
	"github.com/user/curlyk/internal/i18n"
)

// displayCellWidth returns the number of terminal cells s occupies once ANSI
// escape sequences are stripped (runewidth is rune-aware for narrow/wide glyphs).
func displayCellWidth(s string) int {
	return runewidth.StringWidth(stripANSI(s))
}

// helpEntry is one row of the key-reference panel shown in the right pane while
// there is no response yet. key is the literal key combination (never
// localised); descKey is the i18n key for the action description.
type helpEntry struct {
	key     string
	descKey string
}

// helpKeyCombos is the ordered list of hotkeys shown at startup. It mirrors the
// README table and the bindings in keys.go.
var helpKeyCombos = []helpEntry{
	{key: "Ctrl+Enter / Ctrl+R", descKey: "help.run"},
	{key: "Enter", descKey: "help.runMenu"},
	{key: "Ctrl+K", descKey: "help.copyCurl"},
	{key: "Ctrl+G", descKey: "help.nav"},
	{key: "Ctrl+Z", descKey: "help.undo"},
	{key: "Ctrl+Shift+Z", descKey: "help.redo"},
	{key: "Ctrl+Y", descKey: "help.delLine"},
	{key: "Ctrl+C / Ctrl+X / Ctrl+V", descKey: "help.copy"},
	{key: "Ctrl+S", descKey: "help.save"},
	{key: "Ctrl+N", descKey: "help.new"},
	{key: "Tab", descKey: "help.pane"},
	{key: "Ctrl+L", descKey: "help.lang"},
	{key: "Ctrl+D", descKey: "help.debug"},
	{key: "Ctrl+←/→", descKey: "help.words"},
	{key: "Home", descKey: "help.home"},
	{key: "Alt+←/→", descKey: "help.hscroll"},
	{key: "F1", descKey: "help.panel"},
	{key: "Esc Esc / F10", descKey: "help.quit"},
}

// helpKeyStyle highlights the key combination; helpDescStyle renders the muted
// description to its right.
var helpKeyStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("212")).Bold(true)
var helpDescStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("245"))
var helpTitleStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("255"))

// helpPanelLines renders the key-reference block for the right pane. width/height
// are the pane interior dimensions; lines are padded to width so lipgloss border
// math stays exact (the pane interior is exactly Width x Height).
func helpPanelLines(width, height int) []string {
	var out []string
	title := padToWidth(helpTitleStyle.Render(i18n.T("help.title")), width)
	out = append(out, title)
	out = append(out, "")
	// Wrap the summary to width so it never overflows a narrow pane, then split
	// the wrapped text back into display lines.
	summary := wrapToWidth(i18n.T("help.summary"), width-2)
	for _, ln := range strings.Split(summary, "\n") {
		out = append(out, padToWidth(ln, width))
	}
	out = append(out, "")

	for _, e := range helpKeyCombos {
		// Reserve room for the key plus two spaces, giving the description what
		// remains so a narrow pane truncates the description rather than
		// corrupting non-ASCII/wide key glyphs.
		key := e.key
		keyW := displayCellWidth(key)
		descMax := width - keyW - 2
		desc := i18n.T(e.descKey)
		if descMax < 0 {
			// The key alone is already wider than the pane; truncate it.
			descMax = 0
			if keyW > width {
				key = truncateWidth(key, width)
			}
		}
		if displayCellWidth(desc) > descMax {
			desc = truncateWidth(desc, descMax)
		}
		row := helpKeyStyle.Render(key) + helpDescStyle.Render("  "+desc)
		out = append(out, padToWidth(row, width))
	}

	// Trim to the available height (pad with a stray blank row on the first
	// overflow only; lipgloss handles the rest).
	if height > 0 {
		if len(out) > height {
			out = out[:height]
		}
	}
	return out
}

package tui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/mattn/go-runewidth"
	"github.com/user/curlyk/internal/help"
	"github.com/user/curlyk/internal/i18n"
)

// displayCellWidth returns the number of terminal cells s occupies once ANSI
// escape sequences are stripped (runewidth is rune-aware for narrow/wide glyphs).
func displayCellWidth(s string) int {
	return runewidth.StringWidth(stripANSI(s))
}

// helpKeyStyle highlights the key combination; helpDescStyle renders the muted
// description to its right.
var helpKeyStyle lipgloss.Style
var helpDescStyle lipgloss.Style
var helpTitleStyle lipgloss.Style

// helpPanelLines renders the key-reference block for the right pane. width/height
// are the pane interior dimensions (excluding borders); scroll is the vertical
// offset. When the full reference is taller than height, a scroll window of the
// last height lines is rendered so the user can scroll to the variables section.
// Lines are padded to width so lipgloss border math stays exact.
// helpPanelLines renders the key-reference block for the right pane. width/height
// are the pane interior dimensions (excluding borders); scroll is the vertical
// offset into the full reference. When the full reference is taller than height,
// a window of height lines at 'scroll' is returned so the user can reach the
// variables section on short windows. Lines are padded to width so lipgloss
// border math stays exact.
func helpPanelLines(width, height, scroll int) []string {
	full := helpFullLines(width)
	if height > 0 && len(full) > height {
		if scroll > len(full)-height {
			scroll = len(full) - height
		}
		if scroll < 0 {
			scroll = 0
		}
		return full[scroll : scroll+height]
	}
	if height > 0 && len(full) < height {
		// Pad to the exact pane height so the rendered frame has no ragged tail
		// of missing rows (the bordered pane would otherwise insert blank lines
		// itself, indistinguishable from this padding).
		pad := make([]string, height-len(full))
		for i := range pad {
			pad[i] = padToWidth("", width)
		}
		return append(full, pad...)
	}
	return full
}

// helpFullLines renders the complete key-reference panel (hotkeys + variables
// section), one padded string per display row.
func helpFullLines(width int) []string {
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

	for _, e := range help.TUIKeys {
		// Reserve room for the key plus two spaces, giving the description what
		// remains so a narrow pane truncates the description rather than
		// corrupting non-ASCII/wide key glyphs.
		key := e.Key
		keyW := displayCellWidth(key)
		descMax := width - keyW - 2
		desc := i18n.T(e.DescKey)
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

	// Variables section: a short reference to the {{...}} substitution syntax.
	out = append(out,
		padToWidth(helpTitleStyle.Render(i18n.T("help.varTitle")), width),
		"",
		padToWidth(helpDescStyle.Render(i18n.T("help.varHint")), width),
		"",
	)
	for _, v := range help.VarLines {
		text := i18n.T(v.Key)
		if v.Literal {
			out = append(out, padToWidth(helpKeyStyle.Render(text), width))
			continue
		}
		out = append(out, padToWidth(helpDescStyle.Render(text), width))
	}

	return out
}

// helpShown reports whether the help reference is what the right pane renders
// right now (either no response yet, or forced on via F1).
func (m *model) helpShown() bool {
	return m.response == "" || m.helpVisible
}

// helpScrollMax returns the maximum meaningful helpScroll offset (0 when the
// full reference already fits the pane). It mirrors exactly how many rows
// renderResponse hands to helpPanelLines so the last rows are reachable.
func (m *model) helpScrollMax() int {
	lay := m.layout()
	width := lay.respR - lay.half - 1
	if width < 10 {
		width = 10
	}
	full := helpFullLines(width - 2)
	paneH := m.paneContentH()
	if paneH < 1 {
		paneH = 1
	}
	if len(full) <= paneH {
		return 0
	}
	return len(full) - paneH
}

// clampHelpScroll keeps helpScroll within [0, helpScrollMax].
func (m *model) clampHelpScroll() {
	mx := m.helpScrollMax()
	if m.helpScroll < 0 {
		m.helpScroll = 0
	}
	if m.helpScroll > mx {
		m.helpScroll = mx
	}
}

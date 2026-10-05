package tui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/cellbuf"
	"github.com/user/curlyk/internal/i18n"
)

// dialogAction identifies which dialog button was confirmed.
type dialogAction int

const (
	// dialogCancel dismisses the dialog without running the intended action.
	dialogCancel dialogAction = iota
	// dialogConfirm runs the dialog's intended action.
	dialogConfirm
)

// dialogHandler is the callback run when a button is confirmed. The dialog is
// already closed before the handler runs. It returns an optional tea.Cmd.
type dialogHandler func(m *model, act dialogAction) tea.Cmd

// dialogButton is one selectable action in a framed popup dialog.
type dialogButton struct {
	// label is the button text (already translated by the caller).
	label string
	// action is the semantic result of confirming the button.
	action dialogAction
	// defaultBtn marks the button highlighted on open (e.g. Confirm).
	defaultBtn bool
}

// dialogBox is a modal popup with a frame border, drawn centred over the panes
// with the underlying content still visible behind it. While open it owns all
// keyboard and mouse input: Tab/arrows cycle the selected button, Enter runs
// the handler for that action, Esc cancels. A click on a button confirms it; a
// click outside the frame dismisses the dialog.
type dialogBox struct {
	// title is rendered as the first interior row of the frame.
	title string
	// body is the paragraph shown below the title (one line per row).
	body []string
	// buttons is the horizontal button row.
	buttons []dialogButton
	// sel is the index of the highlighted (selected) button.
	sel int
	// onAction is the dialog's handler, invoked with the confirmed action.
	onAction dialogHandler
}

// dialogPad is the horizontal padding between the frame border and the content.
const dialogPad = 1

// dialogInteriorPad is the space reserved on each side of a button label so an
// inverted button reads as a block.
const dialogInteriorPad = 2

// newDialog builds a generic framed dialog with the given translated title,
// body lines and buttons, closing the popup with the handler on confirm.
func (m *model) newDialog(title string, body []string, buttons []dialogButton, onDone dialogHandler) *dialogBox {
	d := &dialogBox{
		title:    title,
		body:     body,
		buttons:  buttons,
		onAction: onDone,
	}
	d.sel = d.defaultIndex()
	return d
}

// defaultIndex returns the index of the button marked defaultBtn, or 0 when none
// is marked.
func (d *dialogBox) defaultIndex() int {
	for i, b := range d.buttons {
		if b.defaultBtn {
			return i
		}
	}
	return 0
}

// buttonRow returns the single-line button row: each label wrapped in interior
// padding and joined by a space, the selected one inverted.
func (d *dialogBox) buttonRow() string {
	var sb strings.Builder
	for i, b := range d.buttons {
		if i > 0 {
			sb.WriteString(" ")
		}
		label := padToWidth(b.label, displayCellWidth(b.label)+dialogInteriorPad)
		if i == d.sel {
			sb.WriteString(menuSelStyle.Render(label))
		} else {
			sb.WriteString(menuNormalStyle.Render(label))
		}
	}
	return sb.String()
}

// rows returns the dialog interior rows: the title row, the body lines and the
// button row.
func (d *dialogBox) rows() []string {
	out := make([]string, 0, len(d.body)+2)
	out = append(out, d.buttonTitleRow())
	out = append(out, d.body...)
	out = append(out, d.buttonRow())
	return out
}

// buttonTitleRow renders the dialog title line (the first interior row).
func (d *dialogBox) buttonTitleRow() string {
	return dialogTitleStyle.Render(d.title)
}

// dialogTitleStyle renders the dialog title line.
var dialogTitleStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("255"))

// dialogFrame renders the bordered frame block for the open dialog (without
// centering). Returns "" when no dialog is open.
func (m *model) dialogFrame() string {
	d := m.dialog
	if d == nil {
		return ""
	}
	contentW := m.dialogContentWidth()
	inner := strings.Join(d.rows(), "\n")
	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("212")).
		Width(contentW+2*dialogPad).
		Padding(0, dialogPad).
		Render(inner)
}

// dialogContentWidth returns the widest interior row (body/buttons) capped at
// three quarters of the terminal width so the frame never overruns a narrow
// window.
func (m *model) dialogContentWidth() int {
	d := m.dialog
	if d == nil {
		return 0
	}
	w := 0
	for _, r := range d.rows() {
		if cw := displayCellWidth(r); cw > w {
			w = cw
		}
	}
	if maxW := m.width * 3 / 4; w > maxW {
		w = maxW
	}
	if w < 5 {
		w = 5
	}
	return w
}

// dialogFrameSize returns the framed block's (width, height) in cells.
func (m *model) dialogFrameSize() (int, int) {
	frame := m.dialogFrame()
	fl := strings.Split(frame, "\n")
	fw := 0
	for _, ln := range fl {
		if cw := displayCellWidth(ln); cw > fw {
			fw = cw
		}
	}
	return fw, len(fl)
}

// dialogGeom returns the absolute screen (x0, y0) of the dialog frame's top-left
// corner within the pane row: centred horizontally over the full width and
// vertically within the pane block (below the header, above the status bar).
func (m *model) dialogGeom() (int, int) {
	fw, fh := m.dialogFrameSize()
	x0 := (m.width - fw) / 2
	if x0 < 0 {
		x0 = 0
	}
	paneH := m.height - 3 // headerHeight(1) above, status bar(1) below
	if paneH < 1 {
		paneH = 1
	}
	y0 := headerHeight + (paneH-fh)/2
	if y0 < headerHeight+1 {
		y0 = headerHeight + 1
	}
	return x0, y0
}

// renderDialogOverlay draws the dialog's framed box directly over the already
// built pane row: the pane content around the frame stays visible, and the
// frame (border + padding + content) overwrites only its own cells. Implemented
// with a cell buffer so both the base panes and the overlay keep their styles.
// The total row count is kept identical to the input so the frame height never
// changes.
func (m *model) renderDialogOverlay(paneRow string) string {
	if m.dialog == nil {
		return paneRow
	}
	lines := strings.Split(paneRow, "\n")
	paneW := m.width
	paneH := len(lines)
	if paneH < 1 {
		paneH = 1
	}
	buf := cellbuf.NewBuffer(paneW, paneH)
	cellbuf.SetContent(buf, paneRow)
	// Burn the frame on top.
	x0, y0 := m.dialogGeom()
	frame := m.dialogFrame()
	cellbuf.SetContentRect(buf, frame, cellbuf.Rect(x0, y0, dialogFrameWidth(frame), dialogFrameHeight(frame)))
	// cellbuf.Render writes CRLF line endings; the rest of the app joins frames
	// with bare LF, so normalise the overlay output.
	return strings.ReplaceAll(cellbuf.Render(buf), "\r\n", "\n")
}

// dialogFrameWidth returns the display width of a rendered frame block.
func dialogFrameWidth(frame string) int {
	fw := 0
	for _, ln := range strings.Split(frame, "\n") {
		if cw := displayCellWidth(ln); cw > fw {
			fw = cw
		}
	}
	return fw
}

// dialogFrameHeight returns the line count of a rendered frame block.
func dialogFrameHeight(frame string) int {
	return len(strings.Split(frame, "\n"))
}

// dialogContains reports whether an absolute (x, y) is inside the dialog frame.
func (m *model) dialogContains(x, y int) bool {
	if m.dialog == nil {
		return false
	}
	x0, y0 := m.dialogGeom()
	fw, fh := m.dialogFrameSize()
	return x >= x0 && x < x0+fw && y >= y0 && y < y0+fh
}

// dialogButtonAt maps an absolute screen (x, y) to the button index under it, in
// terms of the button row geometry (the last interior row of the frame). Returns
// -1 when the point is not on a button.
func (m *model) dialogButtonAt(x, y int) int {
	d := m.dialog
	if d == nil {
		return -1
	}
	x0, y0 := m.dialogGeom()
	_, fh := m.dialogFrameSize()
	if y != y0+fh-1 {
		return -1
	}
	rel := x - (x0 + dialogPad)
	if rel < 0 {
		return -1
	}
	// walk the buttons like buttonRow does
	col := 0
	for i, b := range d.buttons {
		w := displayCellWidth(b.label) + 2*dialogInteriorPad
		if rel >= col && rel < col+w {
			return i
		}
		col += w + 1
	}
	return -1
}

// handleDialogKey processes a key while the dialog is open. It consumes every
// key and returns the tea.Cmd to run (may be nil).
func (m *model) handleDialogKey(msg tea.KeyMsg) tea.Cmd {
	d := m.dialog
	if d == nil {
		return nil
	}
	switch msg.String() {
	case "esc", "ctrl+c":
		return m.activateDialog(dialogCancel)
	case "enter":
		if d.sel < 0 || d.sel >= len(d.buttons) {
			return nil
		}
		return m.activateDialog(d.buttons[d.sel].action)
	case "tab", "right":
		if n := len(d.buttons); n > 0 {
			d.sel = (d.sel + 1) % n
		}
	case "left", "shift+tab":
		if n := len(d.buttons); n > 0 {
			d.sel = (d.sel - 1 + n) % n
		}
	}
	return nil
}

// activateDialog closes the dialog and runs its handler with the given action.
// A nil onAction simply dismisses the popup.
func (m *model) activateDialog(act dialogAction) tea.Cmd {
	d := m.dialog
	if d == nil {
		return nil
	}
	handler := d.onAction
	m.dialog = nil
	if handler == nil {
		m.status = i18n.T("dialog.dismissed")
		return nil
	}
	return handler(m, act)
}

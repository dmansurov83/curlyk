package tui

// Frame geometry, in terminal rows. The screen is exactly three bands: a header
// row (file name / search bar), the pane row holding the three panes side by
// side, and the status bar. Every pane is drawn with a rounded border, so a
// pane's block is two rows taller than the content box inside it.
//
// These numbers used to be spelled out at every call site (m.height-6,
// m.height-4, m.height-3); they live here so the whole vertical layout can be
// reasoned about — and changed — in one place.
const (
	// headerHeight is the number of fixed rows drawn above the panes.
	headerHeight = 1
	// statusHeight is the number of fixed rows drawn below the panes.
	statusHeight = 1
	// paneBoxRows is how many rows a pane's border adds to its interior box.
	paneBoxRows = 2
	// paneSlackRows is the blank tail the content renderers leave at the bottom of
	// a pane's interior box (they fill paneContentH rows of the paneBoxH box).
	paneSlackRows = 2
)

// paneBoxH is the height of a pane's bordered block.
func (m *model) paneBoxH() int { return m.paneRowH() }

// paneInteriorH is the height of a pane's content box (borders excluded): what
// lipgloss is asked to draw.
func (m *model) paneInteriorH() int { return m.paneBoxH() - paneBoxRows }

// paneContentH is the number of rows the pane content renderers fill. It is
// paneSlackRows short of the interior box, so both the rendered frame and the
// mouse/scroll coordinate math agree on where a pane's content ends.
func (m *model) paneContentH() int { return m.paneInteriorH() - paneSlackRows }

// paneRowH is the height of the pane row: the terminal minus the header row and
// the status bar.
func (m *model) paneRowH() int { return m.height - headerHeight - statusHeight }

package gui

import (
	"image/color"
	"strconv"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
)

// markKind selects the overlay background painted behind a run of text.
type markKind int

const (
	markSelection markKind = iota
	markFind
	markFindCurrent
)

// mark is one overlay range of a row, in rune columns.
type mark struct {
	from, to int
	kind     markKind
}

// editorRenderer draws the visible part of the document: a background, the
// selection and search overlays, the caret, one text object per styled run and a
// line number per gutter row.
//
// Only the rows the viewport shows are built (the editor scrolls itself), so the
// cost of a repaint depends on the window size rather than on the length of the
// file. Objects are pooled and reused between frames: a keystroke usually
// changes a handful of them instead of allocating a fresh canvas.
type editorRenderer struct {
	ed    *Editor
	bg    *canvas.Rectangle
	caret *canvas.Rectangle
	objs  []fyne.CanvasObject

	rects []*canvas.Rectangle
	texts []*canvas.Text
	nums  []*canvas.Text
	icons []*canvas.Text

	// compRect/compTexts/compMarker draw the autocomplete list above the source.
	compRect   *canvas.Rectangle
	compTexts  []*canvas.Text
	compMarks  []*canvas.Rectangle
	nCompTexts int
	nCompMarks int

	nRects, nTexts, nNums, nIcons int
}

func (e *Editor) CreateRenderer() fyne.WidgetRenderer {
	return &editorRenderer{
		ed:       e,
		bg:       canvas.NewRectangle(e.pal.background),
		caret:    canvas.NewRectangle(e.pal.cursorBg),
		compRect: canvas.NewRectangle(e.pal.selection),
	}
}

func (r *editorRenderer) Layout(size fyne.Size) { r.bg.Resize(size) }

func (r *editorRenderer) MinSize() fyne.Size { return r.ed.MinSize() }

func (r *editorRenderer) Refresh() { r.rebuild() }

func (r *editorRenderer) Destroy() {}

func (r *editorRenderer) Objects() []fyne.CanvasObject { return r.objs }

// rebuild repaints the visible rows.
func (r *editorRenderer) rebuild() {
	e := r.ed
	_, lineH := e.metrics()
	origin := e.textOriginX()
	gutter := e.gutterWidth()

	r.nRects, r.nTexts, r.nNums, r.nIcons = 0, 0, 0, 0

	r.bg.FillColor = e.pal.background
	r.bg.Resize(e.Size())

	first, last := e.visibleRange()
	caretRowY := paddingY + float32(e.doc.row)*lineH - e.scrollY

	for row := first; row <= last; row++ {
		y := paddingY + float32(row)*lineH - e.scrollY

		if gutter > 0 {
			if e.requestRows[row] {
				icon := r.nextIcon()
				setText(icon, "▶", e.pal.gutterCur, e.fontSize, fyne.TextStyle{Monospace: true})
				icon.Move(fyne.NewPos(paddingX, y))
			}
			num := r.nextNum()
			col := e.pal.gutter
			if row == e.doc.row {
				col = e.pal.gutterCur
			}
			setText(num, strconv.Itoa(row+1), col, e.fontSize, fyne.TextStyle{Monospace: true})
			// Right-align the number against the gutter edge.
			width := fyne.MeasureText(num.Text, e.fontSize, num.TextStyle).Width
			num.Move(fyne.NewPos(gutter-paddingX-width-e.scrollX, y))
		}

		d := e.rowDisplay(row)
		for _, piece := range e.rowPieces(row) {
			width := d.x(piece.to) - d.x(piece.from)
			if width <= 0 {
				continue
			}
			x := origin + d.x(piece.from) - e.scrollX

			if bg := piece.mark; bg != nil {
				rect := r.nextRect()
				rect.FillColor = e.markColor(bg.kind)
				rect.Move(fyne.NewPos(x, y))
				rect.Resize(fyne.NewSize(width, lineH))
			}

			if piece.text == "" {
				continue
			}
			obj := r.nextText()
			setText(obj, piece.text, piece.paint.color, e.fontSize, e.textStyle(piece.paint))
			obj.Move(fyne.NewPos(x, y))
		}
	}

	// The caret is drawn over the overlays but under the glyphs, so it never
	// hides a character.
	r.caret.FillColor = e.pal.cursorBg
	r.caret.Move(fyne.NewPos(e.caretOffsetX()-e.scrollX, caretRowY))
	r.caret.Resize(fyne.NewSize(caretW, lineH))
	if e.focused {
		r.caret.Show()
	} else {
		r.caret.Hide()
	}

	r.rebuildCompletion()
	r.hideUnused()
	r.assemble()
}

// completionBounds is the autocomplete list area in viewport coordinates.
func (e *Editor) completionBounds() (fyne.Position, fyne.Size) {
	charW, lineH := e.metrics()
	realLineH := lineH
	if lineH < 18 {
		lineH = 18
	}
	if charW <= 0 {
		charW = 8
	}
	var size fyne.Size
	if e.complete != nil {
		size = e.complete.popupSize()
	}
	size.Width = max(size.Width, 8*charW)
	// The list sits just below the caret row and starts at the caret column, so
	// it reads as attached to what is being typed instead of pinned to the
	// gutter.
	x := e.caretOffsetX() - e.scrollX
	maxX := e.Size().Width - paddingX - size.Width
	if maxX < paddingX {
		maxX = paddingX
		size.Width = e.Size().Width - 2*paddingX
	}
	x = clampFloat(x, paddingX, maxX)
	y := paddingY + float32(e.doc.row)*realLineH - e.scrollY + realLineH
	if y < paddingY {
		y = paddingY
	}
	if y+size.Height > e.Size().Height-paddingY {
		size.Height = e.Size().Height - paddingY - y
		if size.Height < lineH {
			size.Height = lineH
		}
	}
	if x+size.Width > e.Size().Width-paddingX {
		size.Width = e.Size().Width - paddingX - x
		if size.Width < charW {
			size.Width = charW
		}
	}
	return fyne.NewPos(x, y), size
}

// rebuildCompletion draws the autocomplete list, if any, over the source pane.
// It is part of the editor's own renderer so the editor keeps the keyboard focus
// while the list is open (a Fyne popup overlay would steal it).
func (r *editorRenderer) rebuildCompletion() {
	e := r.ed
	_, lineH := e.metrics()
	if lineH < 18 {
		lineH = 18
	}

	pos, size := fyne.Position{}, fyne.Size{}
	open := e.complete != nil && e.complete.open && len(e.complete.items) > 0

	r.nCompTexts, r.nCompMarks = 0, 0
	if !open {
		r.compRect.Hide()
		// Hide the pooled label and highlight objects left over from an open list:
		// without this a stale menu stays painted over the source after the list is
		// dismissed.
		for i := range r.compTexts {
			r.compTexts[i].Hide()
		}
		for i := range r.compMarks {
			r.compMarks[i].Hide()
		}
		return
	}

	pos, size = e.completionBounds()
	// A solid panel background so the list reads as a menu, not as text floating
	// over the source.
	r.compRect.FillColor = e.pal.menuBg
	r.compRect.Move(pos)
	r.compRect.Resize(size)
	r.compRect.Show()

	textX := pos.X + 2*paddingX

	for i, item := range e.complete.items {
		rowY := pos.Y + float32(i)*lineH
		if rowY >= pos.Y+size.Height {
			break
		}

		// The selected row gets a full-width highlight, so the highlight is obvious
		// even when the list covers a lot of text.
		col := e.pal.menuFg
		label := "  " + item.Label
		if i == e.complete.sel {
			m := r.nextCompMark()
			m.FillColor = e.pal.menuSelBg
			m.Move(fyne.NewPos(pos.X, rowY))
			m.Resize(fyne.NewSize(size.Width, lineH))
			col = e.pal.menuSelFg
			label = "▸ " + item.Label
		}

		t := r.nextCompText()
		setText(t, label, col, e.fontSize, fyne.TextStyle{Monospace: true})
		t.Move(fyne.NewPos(textX, rowY))
	}

	// Hide any pooled texts past the visible rows.
	for i := r.nCompTexts; i < len(r.compTexts); i++ {
		r.compTexts[i].Hide()
	}
	for i := r.nCompMarks; i < len(r.compMarks); i++ {
		r.compMarks[i].Hide()
	}
}

func (r *editorRenderer) nextCompText() *canvas.Text {
	if r.nCompTexts == len(r.compTexts) {
		r.compTexts = append(r.compTexts, canvas.NewText("", color.Transparent))
	}
	t := r.compTexts[r.nCompTexts]
	r.nCompTexts++
	if !t.Visible() {
		t.Show()
	}
	return t
}

func (r *editorRenderer) nextCompMark() *canvas.Rectangle {
	if r.nCompMarks == len(r.compMarks) {
		r.compMarks = append(r.compMarks, canvas.NewRectangle(color.Transparent))
	}
	m := r.compMarks[r.nCompMarks]
	r.nCompMarks++
	if !m.Visible() {
		m.Show()
	}
	return m
}

// piece is a paintable run of one row: a rune range, the text to draw, its
// colour and the overlay (if any) painted behind it.
type piece struct {
	from, to int
	text     string
	paint    paint
	mark     *mark
}

// rowPieces splits a row into paintable runs: styled runs from the highlighter,
// each clipped by the selection and the active search matches.
func (e *Editor) rowPieces(row int) []piece {
	segs := e.segments(row)
	if len(segs) == 0 {
		return nil
	}
	d := e.rowDisplay(row)
	marks := e.marksForRow(row)

	var out []piece
	for _, s := range segs {
		from := d.dispOfByte(s.start)
		to := d.dispOfByte(s.end)
		if to <= from || from >= len(d.runes) {
			continue
		}
		if to > len(d.runes) {
			to = len(d.runes)
		}

		draw := e.pal.ofSeg(s)
		for _, cut := range cutByMarks(from, to, marks) {
			out = append(out, piece{
				from:  cut.from,
				to:    cut.to,
				text:  string(d.runes[cut.from:cut.to]),
				paint: draw,
				mark:  cut.mark,
			})
		}
	}
	return out
}

// cutRange is a run of a row restricted to one overlay kind.
type cutRange struct {
	from, to int
	mark     *mark
}

// cutByMarks splits [from,to) at the overlay boundaries, attaching the overlay
// that covers each part (nil for plain text). Overlapping overlays are resolved
// by mark priority, so a search hit inside the selection keeps the search colour.
func cutByMarks(from, to int, marks []mark) []cutRange {
	bounds := []int{from, to}
	for _, m := range marks {
		if m.to <= from || m.from >= to {
			continue
		}
		bounds = append(bounds, clampInt(m.from, from, to), clampInt(m.to, from, to))
	}
	sortInts(bounds)
	uniq := bounds[:0]
	for i, b := range bounds {
		if i == 0 || b != uniq[len(uniq)-1] {
			uniq = append(uniq, b)
		}
	}

	var out []cutRange
	for i := 0; i+1 < len(uniq); i++ {
		a, b := uniq[i], uniq[i+1]
		if b <= a {
			continue
		}
		out = append(out, cutRange{from: a, to: b, mark: markAt(a, b, marks)})
	}
	return out
}

// markAt returns the highest-priority overlay covering [from,to).
func markAt(from, to int, marks []mark) *mark {
	var best *mark
	for i := range marks {
		m := &marks[i]
		if m.to <= from || m.from >= to {
			continue
		}
		if best == nil || m.kind > best.kind {
			best = m
		}
	}
	return best
}

// marksForRow returns the overlays of a row: the text selection and the search
// matches (with the current match distinct).
func (e *Editor) marksForRow(row int) []mark {
	var marks []mark
	if e.doc.selActive() {
		r0, c0, r1, c1, _ := e.doc.selRange()
		if row >= r0 && row <= r1 {
			from, to := 0, runeLen(e.doc.line(row))
			if row == r0 {
				from = c0
			}
			if row == r1 {
				to = c1
			}
			marks = append(marks, mark{from: from, to: to, kind: markSelection})
		}
	}
	for _, m := range e.findMarksRow(row) {
		marks = append(marks, m)
	}
	return marks
}

// markColor maps an overlay kind to its background colour.
func (e *Editor) markColor(kind markKind) color.Color {
	switch kind {
	case markFind:
		return e.pal.find
	case markFindCurrent:
		return e.pal.findCur
	default:
		return e.pal.selection
	}
}

// nextRect returns a pooled overlay rectangle.
func (r *editorRenderer) nextRect() *canvas.Rectangle {
	if r.nRects == len(r.rects) {
		rc := canvas.NewRectangle(color.Transparent)
		r.rects = append(r.rects, rc)
	}
	rc := r.rects[r.nRects]
	r.nRects++
	if !rc.Visible() {
		rc.Show()
	}
	return rc
}

// nextText returns a pooled text object in the source rows' pool.
func (r *editorRenderer) nextText() *canvas.Text {
	if r.nTexts == len(r.texts) {
		r.texts = append(r.texts, canvas.NewText("", color.Transparent))
	}
	t := r.texts[r.nTexts]
	r.nTexts++
	if !t.Visible() {
		t.Show()
	}
	return t
}

// nextNum returns a pooled gutter number object.
func (r *editorRenderer) nextNum() *canvas.Text {
	if r.nNums == len(r.nums) {
		r.nums = append(r.nums, canvas.NewText("", color.Transparent))
	}
	n := r.nums[r.nNums]
	r.nNums++
	if !n.Visible() {
		n.Show()
	}
	return n
}

// nextIcon returns a pooled gutter run-icon object.
func (r *editorRenderer) nextIcon() *canvas.Text {
	if r.nIcons == len(r.icons) {
		r.icons = append(r.icons, canvas.NewText("", color.Transparent))
	}
	ic := r.icons[r.nIcons]
	r.nIcons++
	if !ic.Visible() {
		ic.Show()
	}
	return ic
}

// hideUnused hides the pooled objects this frame did not need.
func (r *editorRenderer) hideUnused() {
	for i := r.nRects; i < len(r.rects); i++ {
		r.rects[i].Hide()
	}
	for i := r.nTexts; i < len(r.texts); i++ {
		r.texts[i].Hide()
	}
	for i := r.nNums; i < len(r.nums); i++ {
		r.nums[i].Hide()
	}
	for i := r.nIcons; i < len(r.icons); i++ {
		r.icons[i].Hide()
	}
}

// assemble puts the objects in painting order: background, overlay rectangles,
// caret, text runs, then the gutter numbers, then the completion list on top.
func (r *editorRenderer) assemble() {
	r.objs = r.objs[:0]
	r.objs = append(r.objs, r.bg)
	for _, rc := range r.rects {
		r.objs = append(r.objs, rc)
	}
	r.objs = append(r.objs, r.caret)
	for _, t := range r.texts {
		r.objs = append(r.objs, t)
	}
	for _, n := range r.nums {
		r.objs = append(r.objs, n)
	}
	for _, ic := range r.icons {
		r.objs = append(r.objs, ic)
	}
	// The completion list paints last, over the source and the gutter.
	r.objs = append(r.objs, r.compRect)
	for _, m := range r.compMarks {
		r.objs = append(r.objs, m)
	}
	for _, t := range r.compTexts {
		r.objs = append(r.objs, t)
	}
}

// setText updates a pooled text object, refreshing it only when something
// actually changed so a repaint does not dirty every visible run.
func setText(obj *canvas.Text, text string, col color.Color, size float32, style fyne.TextStyle) {
	changed := obj.Text != text || obj.TextSize != size || obj.TextStyle != style || obj.Color != col
	obj.Text = text
	obj.TextSize = size
	obj.TextStyle = style
	obj.Color = col
	if changed {
		obj.Refresh()
	}
}

// sortInts sorts a small slice of ints in place (insertion sort: the slice is a
// handful of mark boundaries).
func sortInts(v []int) {
	for i := 1; i < len(v); i++ {
		for j := i; j > 0 && v[j] < v[j-1]; j-- {
			v[j], v[j-1] = v[j-1], v[j]
		}
	}
}

// byteRuneIndex returns a table mapping every byte offset of s to its rune
// index, so segment ranges (bytes, from the .http lexer) can be turned into rune
// ranges (what the renderer and the caret use).
func byteRuneIndex(s string) []int {
	table := make([]int, len(s)+1)
	ri := 0
	for bi := range s {
		for k := 0; k <= bi; k++ {
			if table[k] == 0 && k > 0 {
				continue
			}
		}
		table[bi] = ri
		ri++
	}
	table[len(s)] = ri
	return table
}

// byteRuneIndexLookup maps one byte offset through a table from byteRuneIndex.
func byteRuneIndexLookup(table []int, byteOff int) int {
	if byteOff <= 0 {
		return 0
	}
	if byteOff >= len(table) {
		return table[len(table)-1]
	}
	return table[byteOff]
}

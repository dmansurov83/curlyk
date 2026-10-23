package gui

import "unicode/utf8"

// rowDisplay is one source line prepared for painting. Tabs are expanded to
// spaces, so the drawn glyphs, the caret and mouse clicks all agree on the
// geometry, while the buffer keeps the tabs the user typed.
//
// Three coordinate systems meet here and this type is the bridge between them:
// byte offsets (what the .http lexer reports), raw rune columns (what the caret
// and the selection use) and display rune indexes (what is drawn and measured).
type rowDisplay struct {
	// runes is the line with tabs replaced by spaces up to the next tab stop.
	runes []rune
	// offsets holds the x offset of every display rune boundary, relative to the
	// text origin.
	offsets []float32
	// rawToDisp maps a raw rune column to a display rune index.
	rawToDisp []int
	// dispToRaw maps a display rune index back to a raw rune column.
	dispToRaw []int
	// byteToDisp maps a byte offset in the source line to a display rune index.
	byteToDisp []int
}

// buildRowDisplay measures a line and builds the mappings above.
func (e *Editor) buildRowDisplay(line string) *rowDisplay {
	d := &rowDisplay{}

	rawRunes := []rune(line)
	d.rawToDisp = make([]int, len(rawRunes)+1)
	d.dispToRaw = make([]int, 0, len(rawRunes)+1)

	for i, r := range rawRunes {
		d.rawToDisp[i] = len(d.runes)
		if r == '\t' && e.tabSize > 0 {
			pad := e.tabSize - len(d.runes)%e.tabSize
			for k := 0; k < pad; k++ {
				d.dispToRaw = append(d.dispToRaw, i)
				d.runes = append(d.runes, ' ')
			}
			continue
		}
		d.dispToRaw = append(d.dispToRaw, i)
		d.runes = append(d.runes, r)
	}
	d.rawToDisp[len(rawRunes)] = len(d.runes)
	d.dispToRaw = append(d.dispToRaw, len(rawRunes))

	d.offsets = make([]float32, len(d.runes)+1)
	var x float32
	for i, r := range d.runes {
		d.offsets[i] = x
		x += e.runeWidth(r)
	}
	d.offsets[len(d.runes)] = x

	d.byteToDisp = make([]int, len(line)+1)
	for bi, ri := 0, 0; bi < len(line); {
		_, size := utf8.DecodeRuneInString(line[bi:])
		d.byteToDisp[bi] = d.rawToDisp[ri]
		for k := bi + 1; k < bi+size && k < len(line); k++ {
			d.byteToDisp[k] = d.rawToDisp[ri]
		}
		bi += size
		ri++
	}
	d.byteToDisp[len(line)] = len(d.runes)
	return d
}

// dispIndex returns the display rune index of a raw rune column.
func (d *rowDisplay) dispIndex(rawCol int) int {
	if rawCol <= 0 {
		return 0
	}
	if rawCol >= len(d.rawToDisp) {
		return len(d.runes)
	}
	return d.rawToDisp[rawCol]
}

// rawIndex returns the raw rune column of a display rune index.
func (d *rowDisplay) rawIndex(disp int) int {
	if disp <= 0 {
		return 0
	}
	if disp >= len(d.dispToRaw) {
		return d.dispToRaw[len(d.dispToRaw)-1]
	}
	return d.dispToRaw[disp]
}

// dispOfByte returns the display rune index of a byte offset in the source line.
func (d *rowDisplay) dispOfByte(byteOff int) int {
	if byteOff <= 0 {
		return 0
	}
	if byteOff >= len(d.byteToDisp) {
		return len(d.runes)
	}
	return d.byteToDisp[byteOff]
}

// x returns the x offset of a display rune index, relative to the text origin.
func (d *rowDisplay) x(disp int) float32 {
	return offsetAt(d.offsets, disp)
}

// nearestDisp returns the display rune index a click at x snaps to: the nearer
// boundary of the rune the pointer is inside.
func (d *rowDisplay) nearestDisp(x float32) int {
	offs := d.offsets
	if len(offs) == 0 || x <= 0 {
		return 0
	}
	if x >= offs[len(offs)-1] {
		return len(offs) - 1
	}
	lo, hi := 0, len(offs)-1
	for lo < hi {
		mid := (lo + hi) / 2
		if offs[mid] < x {
			lo = mid + 1
		} else {
			hi = mid
		}
	}
	if lo > 0 && x-offs[lo-1] < offs[lo]-x {
		return lo - 1
	}
	return lo
}

// offsetAt returns the offset at an index, clamped to the cached range.
func offsetAt(offs []float32, idx int) float32 {
	if len(offs) == 0 {
		return 0
	}
	if idx < 0 {
		idx = 0
	}
	if idx >= len(offs) {
		idx = len(offs) - 1
	}
	return offs[idx]
}

package tui

import (
	"slices"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/user/curlyk/internal/httpfile"
)

// stripANSI removes ANSI escape sequences from a string (used by the debug dump).
func stripANSI(s string) string {
	if !strings.ContainsRune(s, '\x1b') {
		return s
	}
	var b strings.Builder
	i := 0
	for i < len(s) {
		if s[i] == '\x1b' {
			// consume until 'm' (SGR) or a letter terminator
			for i < len(s) && !(s[i] >= 'a' && s[i] <= 'z') && !(s[i] >= 'A' && s[i] <= 'Z') {
				i++
			}
			if i < len(s) {
				i++ // skip letter
			}
			continue
		}
		b.WriteByte(s[i])
		i++
	}
	return b.String()
}

var (
	methodStyle     lipgloss.Style
	urlStyle        lipgloss.Style
	httpVerStyle    lipgloss.Style
	headerNameStyle lipgloss.Style
	headerValStyle  lipgloss.Style
	bodyStyle       lipgloss.Style
	commentStyle    lipgloss.Style
	varStyle        lipgloss.Style
	separatorStyle  lipgloss.Style
	optionStyle     lipgloss.Style
	otherStyle      lipgloss.Style
	// cursorStyle is the visible block cursor (reverse video, high contrast).
	cursorStyle lipgloss.Style
	// selStyle highlights a selected range (dark blue background).
	selStyle lipgloss.Style
	// selStyleOnlyBg is the selection background alone (no foreground), used to
	// fill JSON body regions between colored tokens during selection.
	selStyleOnlyBg lipgloss.Style
)

// col is a styled segment over a byte range of the source line.
type col struct {
	start, end int
	style      lipgloss.Style
	// applyVars if true lets {{var}} spans inside get extra highlighting.
	applyVars bool
	vars      []httpfile.VarSpan
}

// highlightLine produces ANSI-styled text for one source line given its tokens.
// The raw line is preserved; tokens only describe regions to color.
func highlightLine(raw string, toks []httpfile.Token) string {
	return highlightLineJSON(raw, toks, nil)
}

// highlightLineJSON renders a line with token highlighting plus optional
// whole-line JSON cols for a body line. When jsonCols is non-nil it replaces
// the single faint body-token col with per-token JSON syntax coloring.
func highlightLineJSON(raw string, toks []httpfile.Token, jsonCols []col) string {
	var cols []col
	for _, tk := range toks {
		if tk.Type == httpfile.TokOther {
			continue
		}
		if len(jsonCols) > 0 && tk.Type == httpfile.TokBodyText {
			continue
		}
		start, end := tk.Start, tk.End
		if start < 0 {
			start = 0
		}
		if end > len(raw) {
			end = len(raw)
		}
		if end <= start {
			continue
		}
		c := col{start: start, end: end, style: baseStyle(tk.Type)}
		if len(tk.Vars) > 0 {
			c.applyVars = true
			c.vars = tk.Vars
		}
		cols = append(cols, c)
	}
	cols = append(cols, jsonCols...)
	if len(cols) == 0 {
		return raw
	}
	// sort by start, drop overlaps (keep first)
	slices.SortStableFunc(cols, func(a, b col) int { return a.start - b.start })
	filtered := cols[:0]
	for _, c := range cols {
		if len(filtered) > 0 && c.start < filtered[len(filtered)-1].end {
			continue // overlap: skip
		}
		filtered = append(filtered, c)
	}
	cols = filtered

	var sb strings.Builder
	pos := 0
	for _, c := range cols {
		if c.start > pos {
			sb.WriteString(raw[pos:c.start])
		}
		seg := raw[c.start:c.end]
		sb.WriteString(renderSegment(c, seg))
		pos = c.end
	}
	if pos < len(raw) {
		sb.WriteString(raw[pos:])
	}
	return sb.String()
}

// renderSegment colors a segment, inlining {{var}} spans if requested.
func renderSegment(c col, text string) string {
	if !c.applyVars || len(c.vars) == 0 {
		return c.style.Render(text)
	}
	var sb strings.Builder
	last := 0
	for _, vs := range c.vars {
		// vs offsets are relative to the token Value, which equals the segment text here.
		if vs.Start < last || vs.Start > len(text) || vs.End > len(text) {
			continue
		}
		if vs.Start > last {
			sb.WriteString(c.style.Render(text[last:vs.Start]))
		}
		sb.WriteString(varStyle.Render(text[vs.Start:vs.End]))
		last = vs.End
	}
	if last < len(text) {
		sb.WriteString(c.style.Render(text[last:]))
	}
	return sb.String()
}

// renderLineWithCursor renders a source line, inlining an explicit block cursor.
// col is the cursor's rune index within the line (0-based).
// The line is first syntax-highlighted WITHOUT ANSI markers being broken by the
// cursor: we place the cursor block on the raw text at the right rune boundary.
func renderLineWithCursor(raw string, col int, toks []httpfile.Token) string {
	return renderLineWithCursorJSON(raw, col, toks, nil)
}

// renderLineWithCursorJSON is renderLineWithCursor with precomputed whole-line
// JSON cols (nil when the line is not JSON / not a body line). Passing the cols
// from the full line keeps JSON coloring correct across the cursor split.
func renderLineWithCursorJSON(raw string, col int, toks []httpfile.Token, jsonCols []col) string {
	runes := []rune(raw)
	if col < 0 {
		col = 0
	}
	if col > len(runes) {
		col = len(runes)
	}
	// Byte offset where the cursor cell starts.
	cursorRuneStart := byteLenOfRunes(raw, col)

	cursorCh := " "
	if col < len(runes) {
		cursorCh = string(runes[col])
	}

	// left part: raw[:cursorRuneStart]
	leftStyled := renderRegionJSON(raw, 0, cursorRuneStart, toks, jsonCols)
	// cursor cell: visible block (on a character or at EOL)
	cursorCell := cursorStyle.Render(cursorCh)
	// right part after the cursor rune
	rightStart := cursorRuneStart + len([]byte(cursorCh))
	rightStyled := renderRegionJSON(raw, rightStart, len(raw), toks, jsonCols)

	return leftStyled + cursorCell + rightStyled
}

// byteLenOfRunes returns the byte length of the first n runes of s.
func byteLenOfRunes(s string, n int) int {
	if n <= 0 {
		return 0
	}
	r := []rune(s)
	if n >= len(r) {
		return len(s)
	}
	return len(string(r[:n]))
}

// renderRegion renders raw[p1:p2] (byte range) with token highlighting,
// clipping tokens that overlap the region. p1/p2 are byte offsets into raw.
func renderRegion(raw string, p1, p2 int, toks []httpfile.Token) string {
	return renderRegionJSON(raw, p1, p2, toks, nil)
}

// renderRegionJSON renders raw[p1:p2] with token highlighting plus optional
// whole-line JSON cols. When jsonCols is non-nil (precomputed from the full
// line), the JSON cols are clipped to [p1,p2) and painted instead of the body
// style, keeping JSON coloring correct across cursor/selection splits. p1/p2
// are byte offsets into raw.
func renderRegionJSON(raw string, p1, p2 int, toks []httpfile.Token, jsonCols []col) string {
	if p2 <= p1 {
		return ""
	}
	seg := raw[p1:p2]
	// Clip tokens to [p1,p2).
	var cols []col
	for _, tk := range toks {
		if tk.Type == httpfile.TokOther {
			continue
		}
		// When whole-line JSON cols are present they take over the body line, so
		// the single body-token col must not be added (it spans the whole line
		// and would win the overlap dedup, suppressing the JSON colors).
		if len(jsonCols) > 0 && tk.Type == httpfile.TokBodyText {
			continue
		}
		st := tk.Start
		en := tk.End
		// clip
		if en <= p1 || st >= p2 {
			continue
		}
		if st < p1 {
			st = p1
		}
		if en > p2 {
			en = p2
		}
		if en <= st {
			continue
		}
		c := col{start: st - p1, end: en - p1, style: baseStyle(tk.Type)}
		if len(tk.Vars) > 0 {
			c.applyVars = true
			c.vars = tk.Vars
		}
		cols = append(cols, c)
	}
	// Fold in whole-line JSON cols (clipped to the region), which replace the
	// single body-token col with per-token syntax coloring.
	for _, jc := range jsonCols {
		st := jc.start
		en := jc.end
		if en <= p1 || st >= p2 {
			continue
		}
		if st < p1 {
			st = p1
		}
		if en > p2 {
			en = p2
		}
		if en <= st {
			continue
		}
		cols = append(cols, col{start: st - p1, end: en - p1, style: jc.style})
	}
	if len(cols) == 0 {
		return seg
	}
	// sort + drop overlaps
	slices.SortStableFunc(cols, func(a, b col) int { return a.start - b.start })
	var filtered []col
	for _, c := range cols {
		if len(filtered) > 0 && c.start < filtered[len(filtered)-1].end {
			continue
		}
		filtered = append(filtered, c)
	}
	var sb strings.Builder
	pos := 0
	for _, c := range filtered {
		if c.start > pos {
			sb.WriteString(seg[pos:c.start])
		}
		segText := seg[c.start:c.end]
		sb.WriteString(renderSegment(c, segText))
		pos = c.end
	}
	if pos < len(seg) {
		sb.WriteString(seg[pos:])
	}
	return sb.String()
}

// renderLineSelJSON renders a source line with a selection range highlighted.
// selStart/selEnd are rune indices into the line; sel indicates whether the
// range applies to this line at all.
func renderLineSel(raw string, toks []httpfile.Token, selStart, selEnd int) string {
	return renderLineSelJSON(raw, toks, selStart, selEnd, nil)
}

// renderRegionJSONSel renders raw[p1:p2] as a selection: every JSON token in the
// region keeps its foreground color merged with the selection background, and
// gaps between tokens get the selection background alone. Merging the background
// into each token's style (instead of wrapping the whole region in selStyle)
// avoids the mid-region reset that would otherwise drop the selection at every
// JSON token boundary. p1/p2 are byte offsets into raw.
func renderRegionJSONSel(raw string, p1, p2 int, toks []httpfile.Token, jsonCols []col) string {
	if p2 <= p1 {
		return ""
	}
	seg := raw[p1:p2]
	selBg := lipgloss.Color(curScheme.JSONSelBg)
	// Collect JSON cols clipped to the region.
	var cols []col
	for _, jc := range jsonCols {
		st := jc.start
		en := jc.end
		if en <= p1 || st >= p2 {
			continue
		}
		if st < p1 {
			st = p1
		}
		if en > p2 {
			en = p2
		}
		if en <= st {
			continue
		}
		// Merge the selection background into the token's own style so the fg and
		// bg are emitted as a single SGR with no intervening reset.
		merged := jc.style.Background(selBg)
		cols = append(cols, col{start: st - p1, end: en - p1, style: merged})
	}
	slices.SortStableFunc(cols, func(a, b col) int { return a.start - b.start })
	var sb strings.Builder
	pos := 0
	for _, c := range cols {
		if c.start > pos {
			// gap: selection background alone
			sb.WriteString(selStyleOnlyBg.Render(seg[pos:c.start]))
		}
		if c.start >= c.end || c.end > len(seg) {
			continue
		}
		sb.WriteString(c.style.Render(seg[c.start:c.end]))
		pos = c.end
	}
	if pos < len(seg) {
		sb.WriteString(selStyleOnlyBg.Render(seg[pos:]))
	}
	return sb.String()
}

func renderLineSelJSON(raw string, toks []httpfile.Token, selStart, selEnd int, jsonCols []col) string {
	runes := []rune(raw)
	if selStart < 0 {
		selStart = 0
	}
	if selEnd > len(runes) {
		selEnd = len(runes)
	}
	if selEnd <= selStart {
		return highlightLineJSON(raw, toks, jsonCols)
	}

	sStartB := byteLenOfRunes(raw, selStart)
	sEndB := byteLenOfRunes(raw, selEnd)

	// before selection
	before := renderRegionJSON(raw, 0, sStartB, toks, jsonCols)
	// selection
	sel := renderRegionJSONSel(raw, sStartB, sEndB, toks, jsonCols)
	// after
	after := renderRegionJSON(raw, sEndB, len(raw), toks, jsonCols)

	return before + sel + after
}

// renderLineWithCursorSel renders the active line with the block cursor and,
// when selStart<selEnd, a selection range. The cursor block sits at col.
// We build the line by walking rune indices and choosing a background per cell.
func renderLineWithCursorSel(raw string, col int, toks []httpfile.Token, selStart, selEnd int, hasSel bool) string {
	return renderLineWithCursorSelJSON(raw, col, toks, selStart, selEnd, hasSel, nil)
}

func renderLineWithCursorSelJSON(raw string, col int, toks []httpfile.Token, selStart, selEnd int, hasSel bool, jsonCols []col) string {
	if !hasSel {
		return renderLineWithCursorJSON(raw, col, toks, jsonCols)
	}
	runes := []rune(raw)
	if selStart < 0 {
		selStart = 0
	}
	if selEnd > len(runes) {
		selEnd = len(runes)
	}
	if selEnd < selStart {
		selStart, selEnd = selEnd, selStart
	}
	c := col
	if c < 0 {
		c = 0
	}
	if c > len(runes) {
		c = len(runes)
	}

	// Partition the line into typed segments by (selection, cursor) membership.
	type region struct {
		start, end int // rune indices
		kind       int // 0=plain,1=sel,2=cursor,3=sel&cursor
	}
	var segs []region
	// Build boundaries set.
	bounds := []int{0, selStart, selEnd, c, c + 1, len(runes)}
	// sort & dedupe
	bounds = sortInts(bounds)
	for i := 1; i < len(bounds); i++ {
		s, e := bounds[i-1], bounds[i]
		// Clamp both boundaries to the line length: when the cursor sits at the
		// end of the line (c == len(runes)), the c+1 boundary exceeds the
		// buffer and would panic on the slices below.
		if s > len(runes) {
			s = len(runes)
		}
		if e > len(runes) {
			e = len(runes)
		}
		if e <= s {
			continue
		}
		kind := 0
		if s >= selStart && e <= selEnd && selStart < selEnd {
			kind = 1
		}
		// cursor occupies exactly the rune index c..c+1
		if s >= c && e <= c+1 {
			kind |= 2
		}
		segs = append(segs, region{s, e, kind})
	}

	var b strings.Builder
	for _, rg := range segs {
		segText := string(runes[rg.start:rg.end])
		p1 := byteLenOfRunes(raw, rg.start)
		p2 := byteLenOfRunes(raw, rg.end)
		switch rg.kind {
		case 0:
			b.WriteString(renderRegionJSON(raw, p1, p2, toks, jsonCols))
		case 1:
			// Selection: merge the selection bg into each JSON token so no reset
			// drops the highlight mid-way; fall back to wrapping in selStyle for
			// non-JSON lines.
			if len(jsonCols) > 0 {
				b.WriteString(renderRegionJSONSel(raw, p1, p2, toks, jsonCols))
			} else {
				b.WriteString(selStyle.Render(renderRegionJSON(raw, p1, p2, toks, jsonCols)))
			}
		case 2, 3:
			b.WriteString(cursorStyle.Render(segText))
		}
	}
	return b.String()
}

func sortInts(a []int) []int {
	slices.Sort(a)
	return a
}

func baseStyle(t httpfile.TokenType) lipgloss.Style {
	switch t {
	case httpfile.TokMethod:
		return methodStyle
	case httpfile.TokURL:
		return urlStyle
	case httpfile.TokHTTPVersion:
		return httpVerStyle
	case httpfile.TokHeaderName:
		return headerNameStyle
	case httpfile.TokHeaderValue:
		return headerValStyle
	case httpfile.TokBodyText:
		return bodyStyle
	case httpfile.TokComment:
		return commentStyle
	case httpfile.TokSeparator:
		return separatorStyle
	case httpfile.TokOption:
		return optionStyle
	default:
		return otherStyle
	}
}

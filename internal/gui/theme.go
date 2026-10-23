package gui

import (
	"image/color"

	"github.com/user/curlyk/internal/httpfile"
	"github.com/user/curlyk/internal/jsonlex"
	"github.com/user/curlyk/internal/theme"
)

// paint is a colour plus the emphasis the TUI gives a token.
type paint struct {
	color  color.Color
	bold   bool
	italic bool
}

// palette is the RGB form of a theme.Scheme: syntax colours as the TUI resolves
// them, plus the editor chrome the TUI inherits from the terminal (background,
// foreground, cursor, gutter).
type palette struct {
	name string

	background color.Color
	foreground color.Color
	selection  color.Color
	cursorBg   color.Color
	cursorFg   color.Color
	gutter     color.Color
	gutterCur  color.Color
	find       color.Color
	findCur    color.Color

	// Chrome of the window, mirroring the TUI: the header and status strips and
	// the pane borders (active/inactive), so the desktop layout reads like the
	// console.
	headerBg     color.Color
	headerFg     color.Color
	statusBg     color.Color
	statusFg     color.Color
	borderActive color.Color
	borderIdle   color.Color

	// Menu chrome, the completion list's colours: the panel background and the
	// selected row.
	menuBg    color.Color
	menuFg    color.Color
	menuSelBg color.Color
	menuSelFg color.Color

	syntax map[httpfile.TokenType]paint
	json   map[jsonlex.Kind]paint
}

// loadPalette resolves a scheme name (a builtin, or themes/<name>.theme) and
// falls back to the default scheme for an unknown name.
func loadPalette(name string) *palette {
	scheme, ok := theme.ByNameOrFile(name)
	if !ok {
		scheme = theme.Default()
		name = theme.DefaultName
	}
	p := newPalette(scheme)
	p.name = name
	return p
}

func newPalette(s theme.Scheme) *palette {
	bg := ansi256(s.HeaderBg)
	fg := ansi256(s.StatusFg)
	body := ansi256(s.Body)
	if s.Body == "" {
		body = fg
	}
	return &palette{
		background: bg,
		foreground: fg,
		selection:  ansi256(s.SelectionBg),
		cursorBg:   ansi256(s.CursorBg),
		cursorFg:   ansi256(s.CursorFg),
		gutter:     ansi256(s.NumMuted),
		gutterCur:  ansi256(s.NumCurrent),
		find:       ansi256(s.FindBg),
		findCur:    ansi256(s.FindCurBg),

		headerBg:     ansi256(s.HeaderBg),
		headerFg:     ansi256(s.HeaderFg),
		statusBg:     ansi256(s.StatusBg),
		statusFg:     ansi256(s.StatusFg),
		borderActive: ansi256(s.BorderActive),
		borderIdle:   ansi256(s.BorderIdle),
		menuBg:       ansi256(s.MenuBg),
		menuFg:       ansi256(s.MenuFg),
		menuSelBg:    ansi256(s.MenuSelBg),
		menuSelFg:    ansi256(s.MenuSelFg),
		syntax: map[httpfile.TokenType]paint{
			httpfile.TokOther:       {color: fg},
			httpfile.TokMethod:      {color: ansi256(s.Method), bold: true},
			httpfile.TokURL:         {color: ansi256(s.URL)},
			httpfile.TokHTTPVersion: {color: ansi256(s.HTTPVer)},
			httpfile.TokHeaderName:  {color: ansi256(s.HeaderName)},
			httpfile.TokHeaderValue: {color: ansi256(s.HeaderValue)},
			httpfile.TokBodyText:    {color: mix(body, bg, 0.3)},
			httpfile.TokComment:     {color: ansi256(s.Comment), italic: true},
			httpfile.TokVariable:    {color: ansi256(s.Variable), bold: true},
			httpfile.TokSeparator:   {color: ansi256(s.Separator), bold: true},
			httpfile.TokOption:      {color: ansi256(s.Option), bold: true},
		},
		json: map[jsonlex.Kind]paint{
			jsonlex.Key:    {color: ansi256(s.JSONKey)},
			jsonlex.String: {color: ansi256(s.JSONString)},
			jsonlex.Number: {color: ansi256(s.JSONNumber)},
			jsonlex.Bool:   {color: ansi256(s.JSONBool)},
			jsonlex.Null:   {color: ansi256(s.JSONNull)},
			jsonlex.Punct:  {color: ansi256(s.JSONPunct)},
		},
	}
}

// of resolves the paint for a token kind; anything unknown is plain text.
func (p *palette) of(kind httpfile.TokenType) paint {
	if pt, ok := p.syntax[kind]; ok {
		return pt
	}
	return paint{color: p.foreground}
}

// ofJSON resolves the paint for a JSON lexeme kind.
func (p *palette) ofJSON(kind jsonlex.Kind) paint {
	if pt, ok := p.json[kind]; ok {
		return pt
	}
	return paint{color: p.foreground}
}

// ofSeg resolves the paint of one highlighted segment, JSON pieces included.
func (p *palette) ofSeg(s seg) paint {
	if s.jsonKind {
		return p.ofJSON(s.json)
	}
	return p.of(s.kind)
}

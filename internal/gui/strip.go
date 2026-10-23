package gui

import (
	"image/color"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/widget"
)

// strip is one coloured row of the window chrome — the header line above the
// panes or the status line below them. It is the desktop counterpart of the
// TUI's header and status bars: a themed background with left-aligned text, so
// the window reads like the console instead of a toolbar.
type strip struct {
	widget.BaseWidget

	text *canvas.Text
	bg   *canvas.Rectangle
}

func newStrip(bg, fg color.Color, size float32) *strip {
	s := &strip{
		text: canvas.NewText("", fg),
		bg:   canvas.NewRectangle(bg),
	}
	s.text.TextSize = size
	s.text.TextStyle = fyne.TextStyle{Monospace: true}
	s.ExtendBaseWidget(s)
	return s
}

// SetText replaces the row text.
func (s *strip) SetText(text string) {
	if s.text.Text == text {
		return
	}
	s.text.Text = text
	s.text.Refresh()
}

// Text returns the current row text.
func (s *strip) Text() string { return s.text.Text }

// SetColors restyles the row, used when the colour scheme changes.
func (s *strip) SetColors(bg, fg color.Color) {
	s.bg.FillColor = bg
	s.bg.Refresh()
	s.text.Color = fg
	s.text.Refresh()
}

// SetFontSize changes the text size.
func (s *strip) SetFontSize(size float32) {
	if s.text.TextSize == size {
		return
	}
	s.text.TextSize = size
	s.text.Refresh()
}

func (s *strip) CreateRenderer() fyne.WidgetRenderer {
	return &stripRenderer{s: s, objs: []fyne.CanvasObject{s.bg, s.text}}
}

type stripRenderer struct {
	s    *strip
	objs []fyne.CanvasObject
}

func (r *stripRenderer) Layout(size fyne.Size) {
	r.s.bg.Resize(size)
	_, textH := r.s.textSize()
	r.s.text.Move(fyne.NewPos(paddingX, (size.Height-textH)/2))
}

func (r *stripRenderer) MinSize() fyne.Size {
	_, textH := r.s.textSize()
	return fyne.NewSize(1, textH+2*paddingY)
}

func (r *stripRenderer) Refresh() {
	r.Layout(r.s.Size())
}

func (r *stripRenderer) Objects() []fyne.CanvasObject { return r.objs }

func (r *stripRenderer) Destroy() {}

// textSize measures the row's text so the strip reserves the right height.
func (s *strip) textSize() (w, h float32) {
	return fyne.MeasureText("Ag", s.text.TextSize, s.text.TextStyle).Width,
		fyne.MeasureText("Ag", s.text.TextSize, s.text.TextStyle).Height
}

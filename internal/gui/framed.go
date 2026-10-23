package gui

import (
	"image/color"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/widget"
)

// frameThickness is the width of a pane border in pixels. A single hairline is
// easy to miss on a large window, and the console's panes are clearly framed, so
// the desktop frame is drawn two pixels wide.
const frameThickness = 2

// panelFrame draws a border of frameThickness pixels around its content. The
// colour marks the active pane, the desktop counterpart of the TUI's pane
// borders. It exists because Fyne has no bordered container: canvas rectangles
// as Border edges are stuck at one pixel, which reads as no frame at all.
type panelFrame struct {
	widget.BaseWidget

	content fyne.CanvasObject
	col     color.Color

	top    *canvas.Rectangle
	bottom *canvas.Rectangle
	left   *canvas.Rectangle
	right  *canvas.Rectangle
}

// newPanelFrame wraps content in a frame of the given colour.
func newPanelFrame(col color.Color, content fyne.CanvasObject) *panelFrame {
	f := &panelFrame{
		content: content,
		col:     col,
		top:     canvas.NewRectangle(col),
		bottom:  canvas.NewRectangle(col),
		left:    canvas.NewRectangle(col),
		right:   canvas.NewRectangle(col),
	}
	f.ExtendBaseWidget(f)
	return f
}

// SetColor recolours the frame (the pane became active or idle).
func (f *panelFrame) SetColor(col color.Color) {
	f.col = col
	for _, r := range []*canvas.Rectangle{f.top, f.bottom, f.left, f.right} {
		r.FillColor = col
		r.Refresh()
	}
}

func (f *panelFrame) CreateRenderer() fyne.WidgetRenderer {
	return &panelFrameRenderer{
		f:    f,
		objs: []fyne.CanvasObject{f.top, f.bottom, f.left, f.right, f.content},
	}
}

type panelFrameRenderer struct {
	f    *panelFrame
	objs []fyne.CanvasObject
}

func (r *panelFrameRenderer) Layout(size fyne.Size) {
	t := float32(frameThickness)
	inner := fyne.NewSize(size.Width-2*t, size.Height-2*t)

	r.f.top.Move(fyne.NewPos(0, 0))
	r.f.top.Resize(fyne.NewSize(size.Width, t))
	r.f.bottom.Move(fyne.NewPos(0, size.Height-t))
	r.f.bottom.Resize(fyne.NewSize(size.Width, t))
	r.f.left.Move(fyne.NewPos(0, 0))
	r.f.left.Resize(fyne.NewSize(t, size.Height))
	r.f.right.Move(fyne.NewPos(size.Width-t, 0))
	r.f.right.Resize(fyne.NewSize(t, size.Height))

	r.f.content.Move(fyne.NewPos(t, t))
	r.f.content.Resize(inner)
}

func (r *panelFrameRenderer) MinSize() fyne.Size {
	t := float32(frameThickness)
	min := r.f.content.MinSize()
	return fyne.NewSize(min.Width+2*t, min.Height+2*t)
}

func (r *panelFrameRenderer) Refresh() {
	r.Layout(r.f.Size())
	canvas.Refresh(r.f)
}

func (r *panelFrameRenderer) Objects() []fyne.CanvasObject { return r.objs }

func (r *panelFrameRenderer) Destroy() {}

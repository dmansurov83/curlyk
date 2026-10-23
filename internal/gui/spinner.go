package gui

import (
	"image/color"
	"math"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/widget"
)

// spinnerSize is the square reserved for the activity spinner.
const spinnerSize = 44

// spinnerDots is the number of dots in the chase.
const spinnerDots = 10

// spinner is the activity indicator shown over the response body while a
// request is in flight: a ring of dots with a bright head running around it.
// It animates with a Fyne animation, so the ticks arrive on the UI thread.
type spinner struct {
	widget.BaseWidget

	phase   float32
	color   color.Color
	anim    *fyne.Animation
	running bool
	dots    []*canvas.Circle
}

func newSpinner(col color.Color) *spinner {
	s := &spinner{color: col}
	s.ExtendBaseWidget(s)
	for i := 0; i < spinnerDots; i++ {
		s.dots = append(s.dots, canvas.NewCircle(col))
	}
	return s
}

// CreateRenderer implements fyne.Widget.
func (s *spinner) CreateRenderer() fyne.WidgetRenderer {
	return &spinnerRenderer{s: s}
}

// SetColor changes the spinner colour (theme switch).
func (s *spinner) SetColor(col color.Color) {
	if col == nil {
		return
	}
	s.color = col
	s.Refresh()
}

// Start begins the chase. It is idempotent.
func (s *spinner) Start() {
	if s.running {
		return
	}
	s.running = true
	s.anim = fyne.NewAnimation(900*time.Millisecond, func(f float32) {
		s.phase = f
		s.Refresh()
	})
	s.anim.RepeatCount = fyne.AnimationRepeatForever
	s.anim.Start()
}

// Stop ends the chase and leaves the widget ready to start again.
func (s *spinner) Stop() {
	s.running = false
	if s.anim != nil {
		s.anim.Stop()
		s.anim = nil
	}
}

// MinSize implements fyne.Widget.
func (s *spinner) MinSize() fyne.Size {
	s.ExtendBaseWidget(s)
	return fyne.NewSize(spinnerSize, spinnerSize)
}

// opacity is the brightness of dot i for the current phase: the head is opaque
// and the tail fades around the ring.
func (s *spinner) opacity(i int) float32 {
	n := float64(len(s.dots))
	head := float64(s.phase) * n
	d := math.Mod(float64(i)-head+n, n)
	return float32(0.12 + 0.88*(1-d/n))
}

// spinnerRenderer draws the ring of dots.
type spinnerRenderer struct {
	s *spinner
}

func (r *spinnerRenderer) Layout(size fyne.Size) {
	n := len(r.s.dots)
	if n == 0 {
		return
	}
	cx, cy := size.Width/2, size.Height/2
	dot := float32(6)
	radius := min(size.Width, size.Height)/2 - dot/2 - 2
	if radius < 0 {
		radius = 0
	}
	for i, d := range r.s.dots {
		ang := -math.Pi/2 + 2*math.Pi*float64(i)/float64(n)
		x := cx + radius*float32(math.Cos(ang)) - dot/2
		y := cy + radius*float32(math.Sin(ang)) - dot/2
		d.Move(fyne.NewPos(x, y))
		d.Resize(fyne.NewSize(dot, dot))
	}
}

func (r *spinnerRenderer) MinSize() fyne.Size {
	return fyne.NewSize(spinnerSize, spinnerSize)
}

func (r *spinnerRenderer) Refresh() {
	for i, d := range r.s.dots {
		d.FillColor = withAlpha(r.s.color, uint8(255*r.s.opacity(i)))
		d.Refresh()
	}
	r.Layout(r.s.Size())
}

func (r *spinnerRenderer) Objects() []fyne.CanvasObject {
	objs := make([]fyne.CanvasObject, 0, len(r.s.dots))
	for _, d := range r.s.dots {
		objs = append(objs, d)
	}
	return objs
}

func (r *spinnerRenderer) Destroy() {}

// withAlpha keeps the colour's RGB and applies a new alpha.
func withAlpha(c color.Color, a uint8) color.Color {
	if c == nil {
		return color.NRGBA{A: a}
	}
	rgba := color.NRGBAModel.Convert(c).(color.NRGBA)
	rgba.A = a
	return rgba
}

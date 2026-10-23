package gui

import (
	"image/color"
	"strconv"
)

// ansi256 converts an ANSI-256 palette number ("0"–"255", the form used by
// internal/theme) into an RGB colour. Unparsable input yields black.
func ansi256(num string) color.Color {
	n, err := strconv.Atoi(num)
	if err != nil || n < 0 || n > 255 {
		return color.NRGBA{A: 255}
	}
	switch {
	case n < 16:
		return ansi16[n]
	case n < 232:
		n -= 16
		return color.NRGBA{
			R: cubeLevel(n / 36),
			G: cubeLevel((n % 36) / 6),
			B: cubeLevel(n % 6),
			A: 255,
		}
	default:
		v := uint8(8 + (n-232)*10)
		return color.NRGBA{R: v, G: v, B: v, A: 255}
	}
}

func cubeLevel(v int) uint8 {
	if v == 0 {
		return 0
	}
	return uint8(55 + v*40)
}

// mix blends b into a by t (0 keeps a, 1 returns b).
func mix(a, b color.Color, t float64) color.Color {
	ar, ag, ab, _ := a.RGBA()
	br, bg, bb, _ := b.RGBA()
	blend := func(x, y uint32) uint8 {
		return uint8((float64(x)*(1-t) + float64(y)*t) / 257)
	}
	return color.NRGBA{
		R: blend(ar, br),
		G: blend(ag, bg),
		B: blend(ab, bb),
		A: 255,
	}
}

var ansi16 = []color.Color{
	color.NRGBA{R: 0x00, G: 0x00, B: 0x00, A: 255},
	color.NRGBA{R: 0xCD, G: 0x00, B: 0x00, A: 255},
	color.NRGBA{R: 0x00, G: 0xCD, B: 0x00, A: 255},
	color.NRGBA{R: 0xCD, G: 0xCD, B: 0x00, A: 255},
	color.NRGBA{R: 0x00, G: 0x00, B: 0xEE, A: 255},
	color.NRGBA{R: 0xCD, G: 0x00, B: 0xCD, A: 255},
	color.NRGBA{R: 0x00, G: 0xCD, B: 0xCD, A: 255},
	color.NRGBA{R: 0xE5, G: 0xE5, B: 0xE5, A: 255},
	color.NRGBA{R: 0x7F, G: 0x7F, B: 0x7F, A: 255},
	color.NRGBA{R: 0xFF, G: 0x00, B: 0x00, A: 255},
	color.NRGBA{R: 0x00, G: 0xFF, B: 0x00, A: 255},
	color.NRGBA{R: 0xFF, G: 0xFF, B: 0x00, A: 255},
	color.NRGBA{R: 0x5C, G: 0x5C, B: 0xFF, A: 255},
	color.NRGBA{R: 0xFF, G: 0x00, B: 0xFF, A: 255},
	color.NRGBA{R: 0x00, G: 0xFF, B: 0xFF, A: 255},
	color.NRGBA{R: 0xFF, G: 0xFF, B: 0xFF, A: 255},
}

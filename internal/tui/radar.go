package tui

import (
	"math"

	"github.com/siddham/synch/internal/profile"
)

// radarOpts controls interactive/animated rendering of the radar.
type radarOpts struct {
	fraction float64 // 0..1 scale on the data polygon, for the boot "inflate" sweep
	selected int     // index of the highlighted axis (-1 for none)
}

// radarBlock renders the static five-dimension spider chart (used by the static
// fallback and tests).
func radarBlock(dims []profile.DimensionResult, wDots, hDots int) string {
	return radarBlockWith(dims, wDots, hDots, radarOpts{fraction: 1, selected: -1})
}

// radarBlockWith renders the braille spider chart: faint concentric grid + spokes,
// the amber data polygon (each vertex at score/10 × fraction of its axis), and an
// optional highlighted axis drawn in the data layer so it reads as "active".
func radarBlockWith(dims []profile.DimensionResult, wDots, hDots int, opt radarOpts) string {
	c := newCanvas(wDots, hDots)
	cx, cy := wDots/2, hDots/2
	// Braille dots are 2 wide × 4 tall per cell and terminal cells are ~2× taller
	// than wide, so a dot is roughly square. Keep rx==ry in dot space and the shape
	// reads as a regular pentagon.
	r := math.Min(float64(wDots)/2, float64(hDots)/2) - 1
	n := len(dims)
	if n == 0 {
		return c.render()
	}

	angle := func(i int) float64 { return -math.Pi/2 + 2*math.Pi*float64(i)/float64(n) }
	point := func(i int, frac float64) (int, int) {
		a := angle(i)
		return int(math.Round(float64(cx) + math.Cos(a)*r*frac)),
			int(math.Round(float64(cy) + math.Sin(a)*r*frac))
	}

	// Grid rings.
	for _, ring := range []float64{0.25, 0.5, 0.75, 1.0} {
		var px, py int
		for i := 0; i <= n; i++ {
			x, y := point(i%n, ring)
			if i > 0 {
				c.line(px, py, x, y, 1)
			}
			px, py = x, y
		}
	}
	// Spokes (highlight the selected axis by drawing it on the data layer).
	for i := 0; i < n; i++ {
		x, y := point(i, 1.0)
		layer := uint8(1)
		if i == opt.selected {
			layer = 2
		}
		c.line(cx, cy, x, y, layer)
	}

	// Data polygon, scaled by the animation fraction.
	frac := func(d profile.DimensionResult) float64 {
		if !d.Signal.Sufficient {
			return 0
		}
		return d.Signal.Score / 10.0 * opt.fraction
	}
	var fx, fy, px, py int
	for i := 0; i < n; i++ {
		x, y := point(i, frac(dims[i]))
		if i == 0 {
			fx, fy = x, y
		} else {
			c.line(px, py, x, y, 2)
		}
		px, py = x, y
	}
	c.line(px, py, fx, fy, 2)
	return c.render()
}

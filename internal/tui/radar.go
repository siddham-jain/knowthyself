package tui

import (
	"math"

	"github.com/siddham/synch/internal/profile"
)

// radarBlock renders the five-dimension profile as a braille spider chart: faint
// concentric grid + spokes, with the data polygon (each vertex at score/10 of the
// axis length) drawn in the amber accent. Insufficient dimensions plot at 0.
func radarBlock(dims []profile.DimensionResult, wDots, hDots int) string {
	c := newCanvas(wDots, hDots)
	cx, cy := wDots/2, hDots/2
	// Terminal cells are ~twice as tall as wide; braille dots are 2×4, so a dot is
	// roughly square-ish. Use separate radii and correct for aspect so the chart
	// reads as a regular pentagon rather than a squashed one.
	rx := float64(wDots)/2 - 1
	ry := float64(hDots)/2 - 1
	n := len(dims)
	if n == 0 {
		return c.render()
	}

	angle := func(i int) float64 {
		// Start at top (−90°), go clockwise, evenly spaced.
		return -math.Pi/2 + 2*math.Pi*float64(i)/float64(n)
	}
	point := func(i int, frac float64) (int, int) {
		a := angle(i)
		x := float64(cx) + math.Cos(a)*rx*frac
		y := float64(cy) + math.Sin(a)*ry*frac
		return int(math.Round(x)), int(math.Round(y))
	}

	// Grid: concentric rings at 0.25/0.5/0.75/1.0 and spokes to each axis.
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
	for i := 0; i < n; i++ {
		x, y := point(i, 1.0)
		c.line(cx, cy, x, y, 1)
	}

	// Data polygon.
	frac := func(d profile.DimensionResult) float64 {
		if !d.Signal.Sufficient {
			return 0
		}
		return d.Signal.Score / 10.0
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
	c.line(px, py, fx, fy, 2) // close the polygon
	return c.render()
}

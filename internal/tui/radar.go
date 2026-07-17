package tui

import (
	"math"

	"github.com/charmbracelet/lipgloss"
	"github.com/siddham/reflect/internal/design"
	"github.com/siddham/reflect/internal/profile"
)

// radarOpts controls interactive/animated rendering of the radar.
type radarOpts struct {
	fraction float64 // 0..1 scale on the data polygon, for the boot "inflate" sweep
	selected int     // index of the highlighted axis (-1 for none)
	numbers  bool    // stamp axis numbers on the chart perimeter
	maxRows  int     // cap on rendered radar rows (0 = width-driven only), for short terminals
}

// radarBlock renders the static chart (used by the static fallback and tests).
func radarBlock(dims []profile.DimensionResult, wDots, hDots int) string {
	return radarBlockWith(dims, wDots, hDots, radarOpts{fraction: 1, selected: -1, numbers: true})
}

// radarBlockWith renders the braille spider chart, tuned for legibility:
//   - a decluttered grid: only the 5/10 reference rings + faint spokes;
//   - the data polygon FILLED (muted amber) with a bright edge, so area reads as
//     "overall" at a glance and the shape is unambiguous;
//   - a marker at each data point;
//   - the axis number stamped on the perimeter (bright for the selected axis), so a
//     vertex maps to the legend without counting around the chart.
func radarBlockWith(dims []profile.DimensionResult, wDots, hDots int, opt radarOpts) string {
	c := newCanvas(wDots, hDots)
	cx, cy := wDots/2, hDots/2
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

	// Grid: just the mid (5) and outer (10) reference rings — enough to judge scale
	// without burying the data shape.
	for _, ring := range []float64{0.5, 1.0} {
		var px, py int
		for i := 0; i <= n; i++ {
			x, y := point(i%n, ring)
			if i > 0 {
				c.line(px, py, x, y, layerGrid)
			}
			px, py = x, y
		}
	}
	// Spokes; the selected axis is drawn bright so it's easy to track.
	for i := 0; i < n; i++ {
		x, y := point(i, 1.0)
		layer := layerGrid
		if i == opt.selected {
			layer = layerEdge
		}
		c.line(cx, cy, x, y, layer)
	}

	// Data polygon: fill, then bright outline, then vertex markers.
	frac := func(d profile.DimensionResult) float64 {
		if !d.Signal.Sufficient {
			return 0
		}
		return d.Signal.Score / 10.0 * opt.fraction
	}
	verts := make([][2]int, n)
	for i := 0; i < n; i++ {
		x, y := point(i, frac(dims[i]))
		verts[i] = [2]int{x, y}
	}
	c.fillPolygon(verts, layerFill)
	for i := 0; i < n; i++ {
		a, b := verts[i], verts[(i+1)%n]
		c.line(a[0], a[1], b[0], b[1], layerEdge)
	}
	for i := 0; i < n; i++ {
		if dims[i].Signal.Sufficient {
			c.marker(verts[i][0], verts[i][1], layerVertex)
		}
	}

	// Numbered axis labels on the perimeter.
	if opt.numbers {
		normal := lipgloss.NewStyle().Foreground(design.Muted).Bold(true)
		hot := lipgloss.NewStyle().Foreground(design.Accent).Bold(true).Reverse(true)
		for i := 0; i < n; i++ {
			x, y := point(i, 1.0)
			style := normal
			if i == opt.selected {
				style = hot
			}
			c.stamp(x, y, rune('1'+i), style)
		}
	}
	return c.render()
}

// scaleCaption is a one-line legend for the grid rings, shown next to the title.
func scaleCaption() string {
	return design.Dim.Render("· rings 5·10")
}

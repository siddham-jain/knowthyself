package tui

import (
	"sort"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/siddham/reflect/internal/design"
)

// Layer identifies what a lit cell represents, so the renderer can color it. Higher
// layers win when pixels share a cell.
const (
	layerGrid   uint8 = 1 // faint reference rings + spokes
	layerFill   uint8 = 2 // muted amber polygon interior
	layerEdge   uint8 = 3 // bright amber polygon outline
	layerVertex uint8 = 4 // bright amber data-point markers
)

// overlayCell is a text glyph stamped over the braille grid (e.g. an axis number).
type overlayCell struct {
	r     rune
	style lipgloss.Style
}

// canvas is a braille pixel buffer: each terminal cell packs 2×4 dots (Unicode
// braille, base U+2800). A parallel layer grid records what each cell holds so the
// renderer colors grid, fill, edge, and markers distinctly; an overlay map stamps
// text glyphs (axis labels) on top.
type canvas struct {
	wDots, hDots   int
	wCells, hCells int
	dots           []uint8
	layer          []uint8
	overlay        map[int]overlayCell
}

// braille dot bit for a pixel at (col,row) within a cell: col∈{0,1}, row∈{0,1,2,3}.
var brailleBit = [2][4]uint8{
	{0x01, 0x02, 0x04, 0x40}, // col 0: dots 1,2,3,7
	{0x08, 0x10, 0x20, 0x80}, // col 1: dots 4,5,6,8
}

func newCanvas(wDots, hDots int) *canvas {
	wc := (wDots + 1) / 2
	hc := (hDots + 3) / 4
	return &canvas{
		wDots: wDots, hDots: hDots, wCells: wc, hCells: hc,
		dots:    make([]uint8, wc*hc),
		layer:   make([]uint8, wc*hc),
		overlay: map[int]overlayCell{},
	}
}

func (c *canvas) inBounds(x, y int) bool { return x >= 0 && y >= 0 && x < c.wDots && y < c.hDots }

// set lights the pixel at (x,y) on the given layer (higher layer wins per cell).
func (c *canvas) set(x, y int, layer uint8) {
	if !c.inBounds(x, y) {
		return
	}
	idx := (y/4)*c.wCells + (x / 2)
	c.dots[idx] |= brailleBit[x%2][y%4]
	if layer > c.layer[idx] {
		c.layer[idx] = layer
	}
}

// line draws a straight line (Bresenham) between two dot coordinates.
func (c *canvas) line(x0, y0, x1, y1 int, layer uint8) {
	dx, dy := abs(x1-x0), -abs(y1-y0)
	sx, sy := sign(x1-x0), sign(y1-y0)
	err := dx + dy
	for {
		c.set(x0, y0, layer)
		if x0 == x1 && y0 == y1 {
			return
		}
		e2 := 2 * err
		if e2 >= dy {
			err += dy
			x0 += sx
		}
		if e2 <= dx {
			err += dx
			y0 += sy
		}
	}
}

// dot draws a small filled marker (a plus) at (x,y) so data vertices stand out.
func (c *canvas) marker(x, y int, layer uint8) {
	for _, d := range [][2]int{{0, 0}, {1, 0}, {-1, 0}, {0, 1}, {0, -1}} {
		c.set(x+d[0], y+d[1], layer)
	}
}

// fillPolygon scanline-fills the interior of a closed polygon on the given layer, so
// the radar reads as a solid area rather than a thin, ambiguous outline.
func (c *canvas) fillPolygon(pts [][2]int, layer uint8) {
	if len(pts) < 3 {
		return
	}
	minY, maxY := pts[0][1], pts[0][1]
	for _, p := range pts {
		if p[1] < minY {
			minY = p[1]
		}
		if p[1] > maxY {
			maxY = p[1]
		}
	}
	if minY < 0 {
		minY = 0
	}
	if maxY >= c.hDots {
		maxY = c.hDots - 1
	}
	for y := minY; y <= maxY; y++ {
		var xs []int
		for i := 0; i < len(pts); i++ {
			a, b := pts[i], pts[(i+1)%len(pts)]
			y0, y1 := a[1], b[1]
			if y0 == y1 {
				continue
			}
			if (y >= y0 && y < y1) || (y >= y1 && y < y0) {
				x := a[0] + (y-y0)*(b[0]-a[0])/(y1-y0)
				xs = append(xs, x)
			}
		}
		sort.Ints(xs)
		for i := 0; i+1 < len(xs); i += 2 {
			for x := xs[i]; x <= xs[i+1]; x++ {
				c.set(x, y, layer)
			}
		}
	}
}

// stamp places a text glyph at the cell containing dot (x,y), replacing the braille
// there. Used for the numbered axis labels.
func (c *canvas) stamp(x, y int, r rune, style lipgloss.Style) {
	if !c.inBounds(x, y) {
		return
	}
	c.overlay[(y/4)*c.wCells+(x/2)] = overlayCell{r: r, style: style}
}

// render returns the colorized braille block.
func (c *canvas) render() string {
	styles := map[uint8]lipgloss.Style{
		layerGrid:   lipgloss.NewStyle().Foreground(design.Faint),
		layerFill:   lipgloss.NewStyle().Foreground(design.AccentDim),
		layerEdge:   lipgloss.NewStyle().Foreground(design.Accent).Bold(true),
		layerVertex: lipgloss.NewStyle().Foreground(design.Accent).Bold(true),
	}
	var b strings.Builder
	for row := 0; row < c.hCells; row++ {
		for col := 0; col < c.wCells; col++ {
			idx := row*c.wCells + col
			if ov, ok := c.overlay[idx]; ok {
				b.WriteString(ov.style.Render(string(ov.r)))
				continue
			}
			if c.dots[idx] == 0 {
				b.WriteByte(' ')
				continue
			}
			r := rune(0x2800 + int(c.dots[idx]))
			b.WriteString(styles[c.layer[idx]].Render(string(r)))
		}
		if row < c.hCells-1 {
			b.WriteByte('\n')
		}
	}
	return b.String()
}

func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}

func sign(x int) int {
	switch {
	case x > 0:
		return 1
	case x < 0:
		return -1
	default:
		return 0
	}
}
